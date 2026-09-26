package handlers

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/ws"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// прод определяется на каждом вызове, а не кэшируется в переменную пакета -
// тесты переключают окружение через t.Setenv
func isProductionEnvLookup() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	return env == "production" || env == "prod"
}

// checkWebSocketOrigin - проверка Origin на ws-хендшейке, в проде fail-closed.
// без неё чужой сайт, открытый у залогиненного юзера, мог бы подключиться
// к этому ws от его имени (CSWSH)
//
// правила:
//   - при явном списке Origin должен точно совпасть с одним из разрешённых
//   - в проде при wildcard "*" или пустом списке пускается только свой origin
//     (хост из Origin равен Host запроса)
//   - пустой Origin в проде пропускается только для не-браузерных клиентов
//     (нет Sec-Fetch-Site)
//   - в dev wildcard/пустой список разрешают всё, для локалки
func checkWebSocketOrigin(r *http.Request) bool {
	allowedOrigins := os.Getenv("WEBSOCKET_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = os.Getenv("CORS_ALLOWED_ORIGINS")
	}
	trimmed := strings.TrimSpace(allowedOrigins)
	wildcard := trimmed == "" || trimmed == "*"

	prod := isProductionEnvLookup()

	if wildcard && !prod {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// браузер при cross-origin всегда шлёт Origin, так что пустой - это
		// same-origin или curl/бот. в проде отсекается случай когда заголовок
		// Sec-Fetch-Site есть (значит браузер), иначе пропуск
		if prod && r.Header.Get("Sec-Fetch-Site") != "" {
			return false
		}
		return true
	}

	if wildcard {
		// сравниваются имена хостов без порта: nginx проксирует Host как $host,
		// а он порт отбрасывает
		u, err := url.Parse(origin)
		return err == nil && u.Hostname() != "" &&
			strings.EqualFold(u.Hostname(), (&url.URL{Host: r.Host}).Hostname())
	}

	for allowed := range strings.SplitSeq(allowedOrigins, ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWebSocketOrigin,
}

// WebSocketHandler - ws-подключения к турнирам
type WebSocketHandler struct {
	hub *ws.Hub
	log *logger.Logger
}

func NewWebSocketHandler(hub *ws.Hub, log *logger.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		log: log,
	}
}

// HandleTournament обрабатывает подключение к турниру
func (h *WebSocketHandler) HandleTournament(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// юзера в контекст положила auth-мидлварь
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized.WithMessage("authentication required"))
		return
	}

	// клиентский сабпротокол с токеном отражается обратно, так требует rfc 6455
	// (иначе некоторые браузеры рвут соединение)
	responseHeader := http.Header{}
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		for p := range strings.SplitSeq(proto, ",") {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "access_token.") {
				responseHeader.Set("Sec-WebSocket-Protocol", p)
				break
			}
		}
	}

	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		h.log.Warn("Failed to upgrade WebSocket connection",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("user_id", userID.String()),
			zap.String("origin", r.Header.Get("Origin")),
			zap.String("host", r.Host),
			zap.Error(err),
		)
		return
	}

	// tcp keepalive чтобы быстрее замечать молча отвалившихся клиентов
	// (заснувший ноутбук и тп) - os-пробы ходят чаще чем свой ws-пинг
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}

	h.log.Info("WebSocket connection established",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("user_id", userID.String()),
	)

	client := ws.NewClient(h.hub, conn, tournamentID, userID, h.log)

	client.Register()

	go client.WritePump()
	go client.ReadPump()
}

// GetStats возвращает статистику WebSocket подключений
func (h *WebSocketHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetStats()
	writeJSON(w, http.StatusOK, stats)
}
