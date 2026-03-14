package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
		if allowedOrigins == "" {
			allowedOrigins = "http://localhost:3000,http://localhost:5173"
		}

		// Wildcard — разрешить все (как CORS)
		if strings.TrimSpace(allowedOrigins) == "*" {
			return true
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
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
// WS /api/v1/ws/tournaments/:id
func (h *WebSocketHandler) HandleTournament(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
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
		h.log.LogError("Failed to upgrade connection", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("user_id", userID.String()),
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
// GET /api/v1/ws/stats
func (h *WebSocketHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetStats()
	writeJSON(w, http.StatusOK, stats)
}
