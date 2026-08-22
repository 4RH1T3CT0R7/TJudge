package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDFromContext_NoID(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", RequestIDFromContext(ctx))
}

func TestWithRequestID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-12345")
	assert.Equal(t, "req-12345", RequestIDFromContext(ctx))
}

type otherCtxKey struct{}

func TestRequestIDFromContext_WrongType(t *testing.T) {
	// чужой ключ/тип в контексте - должны получить пустую строку, а не панику
	ctx := context.WithValue(context.Background(), otherCtxKey{}, 12345)
	assert.Equal(t, "", RequestIDFromContext(ctx))
}
