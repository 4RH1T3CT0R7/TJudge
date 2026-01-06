package handlers

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

// writeJSON записывает JSON ответ с оптимизацией
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	// Получаем буфер из пула
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	// Сериализуем в буфер
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"failed to encode response"}`))
		return
	}

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Пишем из буфера в ответ
	_, _ = buf.WriteTo(w)
}

// writeError пишет ошибку в ответ
func writeError(w http.ResponseWriter, err error) {
	appErr := errors.ToAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)

	// Используем статический JSON для простых ошибок (избегаем аллокаций)
	response := []byte(`{"error":"` + appErr.Message + `"}`)
	_, _ = w.Write(response)
}
