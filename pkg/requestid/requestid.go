package requestid

import "context"

type ctxKey struct{}

// FromContext извлекает request ID из контекста. Возвращает пустую строку, если не установлен.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// WithContext возвращает новый контекст с заданным request ID.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}
