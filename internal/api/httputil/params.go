package httputil

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// ParseUUIDParam извлекает URL-параметр по имени и парсит его как UUID.
// При ошибке пишет 400 с указанным resourceName и возвращает false.
func ParseUUIDParam(w http.ResponseWriter, r *http.Request, paramName, resourceName string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, paramName)
	id, err := uuid.Parse(raw)
	if err != nil {
		WriteError(w, errors.ErrInvalidInput.WithMessage("invalid "+resourceName+" ID"))
		return uuid.Nil, false
	}
	return id, true
}

// ParseQueryUUID парсит UUID из query-параметра.
// При ошибке пишет 400 и возвращает false.
func ParseQueryUUID(w http.ResponseWriter, r *http.Request, paramName string) (uuid.UUID, bool) {
	raw := r.URL.Query().Get(paramName)
	if raw == "" {
		WriteError(w, errors.ErrInvalidInput.WithMessage("missing "+paramName))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		WriteError(w, errors.ErrInvalidInput.WithMessage("invalid "+paramName))
		return uuid.Nil, false
	}
	return id, true
}
