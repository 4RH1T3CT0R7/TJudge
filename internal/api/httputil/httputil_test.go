package httputil_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

func TestWriteJSON_Success(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSON(rr, 200, map[string]string{"key": "val"})

	assert.Equal(t, 200, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "data should be a JSON object")
	assert.Equal(t, "val", data["key"])
}

func TestWriteJSON_NilData(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSON(rr, 200, nil)

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	assert.Nil(t, body["data"], "data should be null when nil is passed")
}

func TestWriteJSON_NilSliceNormalizedToEmptyArray(t *testing.T) {
	rr := httptest.NewRecorder()

	var items []string // typed-nil slice -- naive json marshalling = "null"
	httputil.WriteJSON(rr, 200, items)

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].([]any)
	require.True(t, ok, "expected []interface{} for typed-nil slice, got %T (%v)", body["data"], body["data"])
	assert.Empty(t, data, "expected []")
}

func TestWriteJSON_NilMapNormalizedToEmptyObject(t *testing.T) {
	rr := httptest.NewRecorder()

	var items map[string]int // typed-nil map -- naive json marshalling = "null"
	httputil.WriteJSON(rr, 200, items)

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "expected map[string]interface{} for typed-nil map, got %T", body["data"])
	assert.Empty(t, data)
}

func TestWriteJSON_NonNilSliceUnchanged(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSON(rr, 200, []string{"a", "b"})

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"a", "b"}, data)
}

func TestWriteJSON_StatusCode(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSON(rr, 201, map[string]string{"created": "true"})

	assert.Equal(t, 201, rr.Code)
}

func TestWriteMessage_NoDataField(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteMessage(rr, 200, "ok")

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "ok", body["message"])
	_, hasData := body["data"]
	assert.False(t, hasData, "message-only response should not contain a data field")
}

func TestWriteError_AppError(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteError(rr, errors.ErrNotFound)

	assert.Equal(t, 404, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	errMsg, ok := body["error"].(string)
	require.True(t, ok, "error field should be a string")
	assert.Equal(t, errors.ErrNotFound.Message, errMsg)
}

func TestWriteError_PlainError(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteError(rr, fmt.Errorf("boom"))

	assert.Equal(t, 500, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	errMsg, ok := body["error"].(string)
	require.True(t, ok, "error field should be a string")
	assert.Equal(t, "Internal server error", errMsg)
}
