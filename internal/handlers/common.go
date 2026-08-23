package handlers

import (
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
)

// writeJSON записывает JSON ответ (делегирует в httputil)
func writeJSON(w http.ResponseWriter, status int, v any) {
	httputil.WriteJSON(w, status, v)
}

// writeError пишет ошибку в ответ (делегирует в httputil)
func writeError(w http.ResponseWriter, err error) {
	httputil.WriteError(w, err)
}
