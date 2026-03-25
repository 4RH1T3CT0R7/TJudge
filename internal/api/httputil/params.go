package httputil

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// ParseUUIDParam extracts a URL parameter by name and parses it as UUID.
// On failure, writes a 400 error response with the given resource name and returns false.
func ParseUUIDParam(w http.ResponseWriter, r *http.Request, paramName, resourceName string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, paramName)
	id, err := uuid.Parse(raw)
	if err != nil {
		WriteError(w, errors.ErrInvalidInput.WithMessage("invalid "+resourceName+" ID"))
		return uuid.Nil, false
	}
	return id, true
}

// ParseQueryUUID parses a UUID from a query parameter.
// On failure, writes a 400 error response and returns false.
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
