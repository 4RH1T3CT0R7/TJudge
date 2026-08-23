package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// parseUUIDParam достаёт url-параметр и парсит как uuid,
// при ошибке сам пишет 400 и возвращает false
func parseUUIDParam(w http.ResponseWriter, r *http.Request, paramName, resourceName string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, paramName)
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid "+resourceName+" ID"))
		return uuid.Nil, false
	}
	return id, true
}

// parseQueryUUID - то же для query-параметра
func parseQueryUUID(w http.ResponseWriter, r *http.Request, paramName string) (uuid.UUID, bool) {
	raw := r.URL.Query().Get(paramName)
	if raw == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("missing "+paramName))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid "+paramName))
		return uuid.Nil, false
	}
	return id, true
}
