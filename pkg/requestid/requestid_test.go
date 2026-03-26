package requestid

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromContext_NoID(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", FromContext(ctx))
}

func TestWithContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithContext(ctx, "req-12345")
	assert.Equal(t, "req-12345", FromContext(ctx))
}

func TestDifferentContexts_DifferentIDs(t *testing.T) {
	ctx1 := WithContext(context.Background(), "id-1")
	ctx2 := WithContext(context.Background(), "id-2")

	assert.Equal(t, "id-1", FromContext(ctx1))
	assert.Equal(t, "id-2", FromContext(ctx2))
	assert.NotEqual(t, FromContext(ctx1), FromContext(ctx2))
}

func TestFromContext_EmptyString(t *testing.T) {
	ctx := WithContext(context.Background(), "")
	assert.Equal(t, "", FromContext(ctx))
}

type otherKey struct{}

func TestFromContext_WrongType(t *testing.T) {
	// If someone stores a non-string value with a different key type,
	// FromContext should return empty string.
	ctx := context.WithValue(context.Background(), otherKey{}, 12345)
	assert.Equal(t, "", FromContext(ctx))
}
