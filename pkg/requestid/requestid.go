package requestid

import "context"

type ctxKey struct{}

// FromContext extracts request ID from context. Returns empty string if not set.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// WithContext returns a new context with the given request ID.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}
