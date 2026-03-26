package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	ws "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		allowedOrigins := os.Getenv("WEBSOCKET_ALLOWED_ORIGINS")
		if allowedOrigins == "" {
			allowedOrigins = os.Getenv("CORS_ALLOWED_ORIGINS")
		}

		// Wildcard или не задано — разрешить все (фронтенд встроен в API, same-origin)
		if allowedOrigins == "" || strings.TrimSpace(allowedOrigins) == "*" {
			return true
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header = same-origin request or non-browser client — allow
			return true
		}

		for _, allowed := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(allowed) == origin {
				return true
			}
		}
		return false
	},
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

	// Echo the exact offered subprotocol per RFC 6455 Section 4.2.2
	responseHeader := http.Header{}
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		for _, p := range strings.Split(proto, ",") {
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
