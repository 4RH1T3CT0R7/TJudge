package handlers

import (
	"net"
	"net/http"
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
// к нашему ws от его имени (CSWSH)
//
// правила:
//   - в проде wildcard "*" и пустой список запрещены, Origin должен точно
//     совпасть с одним из разрешённых. пустой Origin пропускается только для
//     не-браузерных клиентов (нет Sec-Fetch-Site)
//   - в dev wildcard/пустой список разрешают всё, для локалки
func checkWebSocketOrigin(r *http.Request) bool {
	allowedOrigins := os.Getenv("WEBSOCKET_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = os.Getenv("CORS_ALLOWED_ORIGINS")
	}
	trimmed := strings.TrimSpace(allowedOrigins)

	prod := isProductionEnvLookup()

	if trimmed == "" || trimmed == "*" {
		return !prod
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
// @Summary WebSocket подключение к турниру
// @Description Устанавливает WebSocket соединение для получения real-time обновлений турнира
// @Tags websocket
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 101 "WebSocket соединение установлено"
// @Failure 401 {object} object{error=string}
// @Router /ws/tournaments/{id} [get]
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
// @Summary Статистика WebSocket
// @Description Возвращает количество активных WebSocket подключений
// @Tags websocket
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{total_clients=int,tournaments=int}
// @Failure 401 {object} object{error=string}
// @Router /ws/stats [get]
func (h *WebSocketHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetStats()
	writeJSON(w, http.StatusOK, stats)
}
