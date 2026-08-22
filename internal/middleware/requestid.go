package middleware

import "context"

// ключ для request id в контексте (свой тип чтобы не пересекаться с чужими строками)
type requestIDKey struct{}

// RequestIDFromContext достаёт request id из контекста, пустая строка если нет
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithRequestID кладёт request id в контекст
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}
