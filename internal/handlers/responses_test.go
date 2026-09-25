package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeJSONData unwraps the standard API envelope and decodes the "data" field
// into the target. Use this for all handler tests that read successful responses.
func decodeJSONData(t *testing.T, body *bytes.Buffer, target any) {
	t.Helper()
	var envelope Response
	err := json.NewDecoder(body).Decode(&envelope)
	require.NoError(t, err, "failed to decode response envelope")
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err, "failed to re-marshal envelope data")
	require.NoError(t, json.Unmarshal(raw, target), "failed to unmarshal data into target")
}

func TestWriteJSON_StatusOK_Map(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var result map[string]string
	decodeJSONData(t, rr.Body, &result)
	assert.Equal(t, "value", result["key"])
}

func TestWriteJSON_NilValue(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	// nil data => envelope with data:null
	assert.Contains(t, rr.Body.String(), `"data":null`)
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

// детали валидации полей доходят до клиента, а не голое «Validation failed»
func TestWriteError_ValidationDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	errs := models.ValidationErrors{}
	errs.Add("name", "name is required")
	errs.Add("game_type", "game_type is required")
	writeError(rr, errors.ErrValidation.WithError(errs))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Validation failed: name: name is required; game_type: game_type is required")

	rr = httptest.NewRecorder()
	writeError(rr, errors.ErrValidation.WithError(models.ValidateEmail("bad")))
	assert.Contains(t, rr.Body.String(), "email: invalid email format")
}

func TestWriteError_PlainError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, assert.AnError)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestWriteJSON_NilSliceNormalizedToEmptyArray(t *testing.T) {
	rr := httptest.NewRecorder()

	var items []string // typed-nil слайс, наивный marshal дал бы null
	writeJSON(rr, 200, items)

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].([]any)
	require.True(t, ok, "ждали []interface{} для typed-nil слайса, получили %T (%v)", body["data"], body["data"])
	assert.Empty(t, data)
}

func TestWriteJSON_NilMapNormalizedToEmptyObject(t *testing.T) {
	rr := httptest.NewRecorder()

	var items map[string]int
	writeJSON(rr, 200, items)

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "ждали map для typed-nil мапы, получили %T", body["data"])
	assert.Empty(t, data)
}

func TestWriteJSON_NonNilSliceUnchanged(t *testing.T) {
	rr := httptest.NewRecorder()

	writeJSON(rr, 200, []string{"a", "b"})

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"a", "b"}, data)
}
