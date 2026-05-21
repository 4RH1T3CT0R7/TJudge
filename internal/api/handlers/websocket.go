package handlers

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	ws "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// isProductionEnvLookup проверяет ENVIRONMENT=production|prod (lookup на каждом вызове,
// чтобы тесты могли изменять env через t.Setenv).
func isProductionEnvLookup() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	return env == "production" || env == "prod"
}

// checkWebSocketOrigin реализует fail-closed проверку Origin для WebSocket handshake.
//
// Правила:
//   - В production (ENVIRONMENT=production|prod) wildcard "*" и пустой origin-list
//     запрещены. Origin-заголовок должен точно совпадать с одним из allowed.
//     Пустой Origin разрешается только для не-браузерных клиентов (без Sec-Fetch-Site).
//   - В development wildcard/пустой origin-list разрешает любой origin.
//
// Это закрывает CSWSH (Cross-Site WebSocket Hijacking), когда чужой сайт,
// открытый в браузере авторизованного пользователя, подключается к WS.
func checkWebSocketOrigin(r *http.Request) bool {
	allowedOrigins := os.Getenv("WEBSOCKET_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = os.Getenv("CORS_ALLOWED_ORIGINS")
	}
	trimmed := strings.TrimSpace(allowedOrigins)

	prod := isProductionEnvLookup()

	// В dev wildcard и пустой список разрешают всё (legacy-поведение для локалки).
	// В prod оба режима - fail-closed.
	if trimmed == "" || trimmed == "*" {
		return !prod
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// Пустой Origin в браузерах не бывает при cross-origin; это либо same-origin,
		// либо не-браузерный клиент (curl, bot). В prod пропускаем только если это
		// явно не-браузерный клиент (нет Sec-Fetch-Site), иначе fail-closed.
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

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWebSocketOrigin,
}

// WebSocketHandler обрабатывает WebSocket подключения
type WebSocketHandler struct {
	hub *websocket.Hub
	log *logger.Logger
}

// NewWebSocketHandler создаёт новый WebSocket handler
func NewWebSocketHandler(hub *websocket.Hub, log *logger.Logger) *WebSocketHandler {
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
	// Извлекаем ID турнира из URL
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Извлекаем user ID из контекста (должен быть установлен auth middleware)
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized.WithMessage("authentication required"))
		return
	}

	// Отражаем предложенный клиентом subprotocol дословно (RFC 6455, секция 4.2.2).
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

	// Upgrade HTTP соединения в WebSocket
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

	// Включаем TCP keepalive для ранней детекции "мертвых" клиентов,
	// которые не отвечают на WebSocket ping (например, замороженный laptop).
	// OS-level probes отправляются чаще, чем WS ping, что уменьшает latency
	// обнаружения разрыва с ~35s до ~10s.
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}

	h.log.Info("WebSocket connection established",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("user_id", userID.String()),
	)

	// Создаём клиента
	client := websocket.NewClient(h.hub, conn, tournamentID, userID, h.log)

	// Регистрируем клиента в hub
	client.Register()

	// Запускаем горутины для чтения и записи
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
