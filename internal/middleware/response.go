package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

type errorResponse struct {
	Error string `json:"error"`
}

// writeError - своя копия хелпера из handlers (импортировать оттуда нельзя,
// получился бы цикл: handlers сам зависит от middleware). формат тот же:
// {"error": msg}, статус из AppError
func writeError(w http.ResponseWriter, err error) {
	appErr := errors.ToAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)

	data, marshalErr := json.Marshal(errorResponse{Error: appErr.Message})
	if marshalErr != nil {
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	_, _ = w.Write(data)
}
