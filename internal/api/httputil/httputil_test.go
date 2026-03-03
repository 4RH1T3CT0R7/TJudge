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

	var body map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "data should be a JSON object")
	assert.Equal(t, "val", data["key"])
}

func TestWriteJSON_NilData(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSON(rr, 200, nil)

	var body map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	assert.Nil(t, body["data"], "data should be null when nil is passed")
}

func TestWriteJSON_StatusCode(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSON(rr, 201, map[string]string{"created": "true"})

	assert.Equal(t, 201, rr.Code)
}

func TestWriteJSONWithMeta_IncludesMeta(t *testing.T) {
	rr := httptest.NewRecorder()

	meta := &httputil.Meta{Total: 10, Limit: 5, Offset: 0}
	httputil.WriteJSONWithMeta(rr, 200, []string{"a", "b"}, meta)

	var body map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	metaObj, ok := body["meta"].(map[string]interface{})
	require.True(t, ok, "meta should be a JSON object")
	assert.Equal(t, float64(10), metaObj["total"])
	assert.Equal(t, float64(5), metaObj["limit"])
}

func TestWriteJSONWithMeta_NilMeta(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteJSONWithMeta(rr, 200, []string{"x"}, nil)

	var body map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	// Meta is omitempty, so it should be absent from the response when nil.
	_, exists := body["meta"]
	assert.False(t, exists, "meta key should be absent when nil meta is passed")
}

func TestWriteMessage_NoDataField(t *testing.T) {
	rr := httptest.NewRecorder()

	httputil.WriteMessage(rr, 200, "ok")

	var body map[string]interface{}
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

	var body map[string]interface{}
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

	var body map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	errMsg, ok := body["error"].(string)
	require.True(t, ok, "error field should be a string")
	assert.Equal(t, "Internal server error", errMsg)
}
