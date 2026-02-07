package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIDCursor(t *testing.T) {
	id := uuid.New()
	cursor := NewIDCursor(id)

	require.NotNil(t, cursor)
	assert.Equal(t, CursorTypeID, cursor.Type)
	require.NotNil(t, cursor.ID)
	assert.Equal(t, id, *cursor.ID)
	assert.Nil(t, cursor.Timestamp)
	assert.Nil(t, cursor.Fields)
}

func TestNewTimestampCursor(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Second)
	cursor := NewTimestampCursor(ts)

	require.NotNil(t, cursor)
	assert.Equal(t, CursorTypeTimestamp, cursor.Type)
	require.NotNil(t, cursor.Timestamp)
	assert.Equal(t, ts, *cursor.Timestamp)
	assert.Nil(t, cursor.ID)
}

func TestNewCompositeCursor(t *testing.T) {
	fields := map[string]interface{}{"rating": 1500, "name": "test"}
	cursor := NewCompositeCursor(fields)

	require.NotNil(t, cursor)
	assert.Equal(t, CursorTypeComposite, cursor.Type)
	assert.Equal(t, fields, cursor.Fields)
	assert.Nil(t, cursor.ID)
	assert.Nil(t, cursor.Timestamp)
}

func TestCursor_Encode_Nil(t *testing.T) {
	var cursor *Cursor
	encoded, err := cursor.Encode()

	require.NoError(t, err)
	assert.Equal(t, "", encoded)
}

func TestCursor_EncodeDecode_RoundTrip_ID(t *testing.T) {
	id := uuid.New()
	original := NewIDCursor(id)

	encoded, err := original.Encode()
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := DecodeCursor(encoded)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	assert.Equal(t, CursorTypeID, decoded.Type)
	require.NotNil(t, decoded.ID)
	assert.Equal(t, id, *decoded.ID)
}

func TestCursor_EncodeDecode_RoundTrip_Timestamp(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Second)
	original := NewTimestampCursor(ts)

	encoded, err := original.Encode()
	require.NoError(t, err)

	decoded, err := DecodeCursor(encoded)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	assert.Equal(t, CursorTypeTimestamp, decoded.Type)
	require.NotNil(t, decoded.Timestamp)
	assert.Equal(t, ts, decoded.Timestamp.UTC().Truncate(time.Second))
}

func TestDecodeCursor_EmptyString(t *testing.T) {
	decoded, err := DecodeCursor("")

	require.NoError(t, err)
	assert.Nil(t, decoded)
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("not-valid-base64!!!")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode cursor")
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON
	_, err := DecodeCursor("bm90LWpzb24=") // "not-json" in base64

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal cursor")
}

// --- PageRequest.Validate ---

func TestPageRequest_Validate_Valid_First(t *testing.T) {
	first := 10
	pr := &PageRequest{First: &first}

	err := pr.Validate()
	assert.NoError(t, err)
}

func TestPageRequest_Validate_Valid_Last(t *testing.T) {
	last := 20
	pr := &PageRequest{Last: &last}

	err := pr.Validate()
	assert.NoError(t, err)
}

func TestPageRequest_Validate_FirstAndLast_Conflict(t *testing.T) {
	first, last := 10, 10
	pr := &PageRequest{First: &first, Last: &last}

	err := pr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "first")
	assert.Contains(t, err.Error(), "last")
}

func TestPageRequest_Validate_AfterAndBefore_Conflict(t *testing.T) {
	after, before := "cursor1", "cursor2"
	pr := &PageRequest{After: &after, Before: &before}

	err := pr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "after")
	assert.Contains(t, err.Error(), "before")
}

func TestPageRequest_Validate_NegativeFirst(t *testing.T) {
	first := -1
	pr := &PageRequest{First: &first}

	err := pr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestPageRequest_Validate_ZeroFirst(t *testing.T) {
	first := 0
	pr := &PageRequest{First: &first}

	err := pr.Validate()
	assert.Error(t, err)
}

func TestPageRequest_Validate_NegativeLast(t *testing.T) {
	last := -5
	pr := &PageRequest{Last: &last}

	err := pr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestPageRequest_Validate_FirstExceedsMax(t *testing.T) {
	first := 101
	pr := &PageRequest{First: &first}

	err := pr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "100")
}

func TestPageRequest_Validate_LastExceedsMax(t *testing.T) {
	last := 200
	pr := &PageRequest{Last: &last}

	err := pr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "100")
}

func TestPageRequest_Validate_Empty(t *testing.T) {
	pr := &PageRequest{}

	err := pr.Validate()
	assert.NoError(t, err)
}

// --- GetLimit ---

func TestPageRequest_GetLimit_WithFirst(t *testing.T) {
	first := 15
	pr := &PageRequest{First: &first}

	assert.Equal(t, 15, pr.GetLimit())
}

func TestPageRequest_GetLimit_WithLast(t *testing.T) {
	last := 25
	pr := &PageRequest{Last: &last}

	assert.Equal(t, 25, pr.GetLimit())
}

func TestPageRequest_GetLimit_Default(t *testing.T) {
	pr := &PageRequest{}

	assert.Equal(t, 20, pr.GetLimit())
}

// --- IsForward / IsBackward ---

func TestPageRequest_IsForward_WithFirst(t *testing.T) {
	first := 10
	pr := &PageRequest{First: &first}

	assert.True(t, pr.IsForward())
	assert.False(t, pr.IsBackward())
}

func TestPageRequest_IsForward_WithAfter(t *testing.T) {
	after := "cursor"
	pr := &PageRequest{After: &after}

	assert.True(t, pr.IsForward())
}

func TestPageRequest_IsBackward_WithLast(t *testing.T) {
	last := 10
	pr := &PageRequest{Last: &last}

	assert.True(t, pr.IsBackward())
	assert.False(t, pr.IsForward())
}

func TestPageRequest_IsBackward_WithBefore(t *testing.T) {
	before := "cursor"
	pr := &PageRequest{Before: &before}

	assert.True(t, pr.IsBackward())
}

// --- GetCursor ---

func TestPageRequest_GetCursor_After(t *testing.T) {
	id := uuid.New()
	cursor := NewIDCursor(id)
	encoded, err := cursor.Encode()
	require.NoError(t, err)

	pr := &PageRequest{After: &encoded}
	decoded, err := pr.GetCursor()

	require.NoError(t, err)
	require.NotNil(t, decoded)
	assert.Equal(t, CursorTypeID, decoded.Type)
}

func TestPageRequest_GetCursor_Before(t *testing.T) {
	id := uuid.New()
	cursor := NewIDCursor(id)
	encoded, err := cursor.Encode()
	require.NoError(t, err)

	pr := &PageRequest{Before: &encoded}
	decoded, err := pr.GetCursor()

	require.NoError(t, err)
	require.NotNil(t, decoded)
}

func TestPageRequest_GetCursor_NoCursor(t *testing.T) {
	pr := &PageRequest{}
	decoded, err := pr.GetCursor()

	require.NoError(t, err)
	assert.Nil(t, decoded)
}
