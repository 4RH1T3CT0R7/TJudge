package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// bufferPool пул буферов для JSON сериализации
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Response - стандартный API-конверт для всех успешных ответов.
type Response struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta содержит pagination-метаданные для list-эндпоинтов.
type Meta struct {
	Total  int `json:"total,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ErrorResponse структура для JSON ответа с ошибкой
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON оборачивает payload в стандартный Response-конверт и пишет его.
//
// Нормализует typed-nil slice/map в пустые коллекции, чтобы list-эндпоинты
// отдавали `[]` / `{}` вместо `null`. Schema-валидаторы на фронте (см.
// web/src/api/schema.ts) ожидают массив, и `null` им ломает контракт.
// Untyped nil (writeJSON(w, ..., nil)) и nil-pointer оставляем как null
// -- это семантика "ресурс отсутствует".
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	v = normalizeNilCollections(v)
	envelope := Response{Data: v}
	writeRawJSON(w, status, envelope)
}

// normalizeNilCollections заменяет typed-nil slice/map на пустую коллекцию
// того же типа. Остальные значения возвращает без изменений.
func normalizeNilCollections(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return v
	}
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
		}
	case reflect.Map:
		if rv.IsNil() {
			return reflect.MakeMap(rv.Type()).Interface()
		}
	}
	return v
}

// messageResponse используется WriteMessage, чтобы не отдавать "data":null.
type messageResponse struct {
	Message string `json:"message"`
}

// WriteMessage пишет ответ только с сообщением (без payload в data).
func WriteMessage(w http.ResponseWriter, status int, message string) {
	writeRawJSON(w, status, messageResponse{Message: message})
}

// writeRawJSON кодирует любое значение в JSON и пишет его в ответ.
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
