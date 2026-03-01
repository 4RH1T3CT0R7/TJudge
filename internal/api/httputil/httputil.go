package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// bufferPool пул буферов для JSON сериализации
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Response is the standard API envelope for all successful responses.
type Response struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination metadata for list endpoints.
type Meta struct {
	Total  int `json:"total,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ErrorResponse структура для JSON ответа с ошибкой
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON wraps the payload in a standard Response envelope and writes it.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	envelope := Response{Data: v}
	writeRawJSON(w, status, envelope)
}

// WriteJSONWithMeta wraps the payload with pagination metadata.
func WriteJSONWithMeta(w http.ResponseWriter, status int, v interface{}, meta *Meta) {
	envelope := Response{Data: v, Meta: meta}
	writeRawJSON(w, status, envelope)
}

// messageResponse is used by WriteMessage to avoid emitting "data":null.
type messageResponse struct {
	Message string `json:"message"`
}

// WriteMessage writes a message-only response (no data payload).
func WriteMessage(w http.ResponseWriter, status int, message string) {
	writeRawJSON(w, status, messageResponse{Message: message})
}

// writeRawJSON encodes any value to JSON and writes it to the response.
func writeRawJSON(w http.ResponseWriter, status int, v interface{}) {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"failed to encode response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// WriteError пишет ошибку в ответ, конвертируя в AppError
func WriteError(w http.ResponseWriter, err error) {
	appErr := errors.ToAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)

	data, marshalErr := json.Marshal(ErrorResponse{Error: appErr.Message})
	if marshalErr != nil {
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	_, _ = w.Write(data)
}
