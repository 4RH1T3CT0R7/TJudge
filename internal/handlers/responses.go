package handlers

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// пул буферов чтобы не аллоцировать на каждый ответ
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// Response - стандартный конверт всех успешных ответов, фронт читает поле data
type Response struct {
	Data    any    `json:"data"`
	Message string `json:"message,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
}

// Meta - пагинация для списков
type Meta struct {
	Total  int `json:"total,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ErrorResponse - конверт ошибки
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeJSON заворачивает payload в конверт {"data":...} и пишет его.
// nil-слайсы и nil-мапы приводятся к []/{} - фронтовая схема ждёт массив,
// и null ей ломает разбор. голый nil оставляется как null (значит «ресурса нет»)
func writeJSON(w http.ResponseWriter, status int, v any) {
	v = normalizeNilCollections(v)
	envelope := Response{Data: v}
	writeRawJSON(w, status, envelope)
}

// normalizeNilCollections подменяет typed-nil slice/map на пустую коллекцию.
// без рефлексии тут никак - тип известен только в рантайме
func normalizeNilCollections(v any) any {
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

func writeRawJSON(w http.ResponseWriter, status int, v any) {
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

// writeError отдаёт ошибку как {"error": msg}, статус берёт из AppError
func writeError(w http.ResponseWriter, err error) {
	appErr := errors.ToAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)

	data, marshalErr := json.Marshal(ErrorResponse{Error: publicMessage(appErr)})
	if marshalErr != nil {
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	_, _ = w.Write(data)
}

// publicMessage - текст ошибки для клиента. внутренняя ошибка наружу не отдаётся,
// кроме ошибок валидации полей: в них нет внутренностей, а без них клиент видит
// голое «Validation failed» и не знает, что исправить
func publicMessage(appErr *errors.AppError) string {
	var fieldErrs models.ValidationErrors
	var fieldErr *models.ValidationError
	switch {
	case stderrors.As(appErr.Err, &fieldErrs):
		details := make([]string, 0, len(fieldErrs))
		for _, e := range fieldErrs {
			details = append(details, e.Error())
		}
		return appErr.Message + ": " + strings.Join(details, "; ")
	case stderrors.As(appErr.Err, &fieldErr):
		return appErr.Message + ": " + fieldErr.Error()
	}
	return appErr.Message
}
