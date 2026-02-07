package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON_StatusOK_Map(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var result map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestWriteJSON_StatusCreated_Struct(t *testing.T) {
	type Response struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusCreated, Response{ID: 1, Name: "test"})

	assert.Equal(t, http.StatusCreated, rr.Code)

	var result Response
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestWriteJSON_NilValue(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	// nil encodes as "null\n"
	assert.Contains(t, rr.Body.String(), "null")
}

func TestWriteJSON_EmptySlice(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, []string{})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "[]")
}

func TestWriteError_NotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, errors.ErrNotFound)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), "Resource not found")
}

func TestWriteError_ValidationWithMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, errors.ErrValidation.WithMessage("invalid email format"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid email format")
}

func TestWriteError_PlainError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, assert.AnError)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
