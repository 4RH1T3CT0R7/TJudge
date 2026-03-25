package httputil_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
)

func TestParseUUIDParam_ValidUUID(t *testing.T) {
	expected := uuid.New()

	r := httptest.NewRequest(http.MethodGet, "/test/"+expected.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", expected.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")

	assert.True(t, ok)
	assert.Equal(t, expected, id)
	assert.Equal(t, 200, w.Code)
}

func TestParseUUIDParam_InvalidUUID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test/not-a-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, 400, w.Code)

	var body map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "invalid tournament ID", body["error"])
}

func TestParseUUIDParam_EmptyParam(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	id, ok := httputil.ParseUUIDParam(w, r, "id", "game")

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, 400, w.Code)

	var body map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "invalid game ID", body["error"])
}

func TestParseQueryUUID_ValidUUID(t *testing.T) {
	expected := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/test?team_id="+expected.String(), nil)
	w := httptest.NewRecorder()

	id, ok := httputil.ParseQueryUUID(w, r, "team_id")

	assert.True(t, ok)
	assert.Equal(t, expected, id)
}

func TestParseQueryUUID_MissingParam(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	id, ok := httputil.ParseQueryUUID(w, r, "team_id")

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, 400, w.Code)

	var body map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "missing team_id", body["error"])
}

func TestParseQueryUUID_InvalidUUID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?game_id=invalid", nil)
	w := httptest.NewRecorder()

	id, ok := httputil.ParseQueryUUID(w, r, "game_id")

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, 400, w.Code)

	var body map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "invalid game_id", body["error"])
}
