package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestServer_Close_Idempotent - регрессия на идемпотентность Close().
// Close() безопасен для повторного вызова и закрывает stopCh для cleanup-
// горутины fallback-лимитера. Тест работает на минимальной структуре без
// поднятия полного NewServer (handlers nil).
func TestServer_Close_Idempotent(t *testing.T) {
	s := &Server{
		rateLimitStopCh: make(chan struct{}),
	}

	assert.NotPanics(t, func() { s.Close() }, "первый Close не должен паниковать")
	assert.NotPanics(t, func() { s.Close() }, "повторный Close не должен паниковать")
	assert.NotPanics(t, func() { s.Close() }, "третий Close тоже OK")

	// После Close канал должен быть закрыт - чтение возвращает zero-value без блокировки.
	select {
	case <-s.rateLimitStopCh:
		// ok - канал закрыт
	default:
		t.Fatal("rateLimitStopCh должен быть закрыт после Close")
	}
}

// TestServer_Close_NilStopCh - Close на только что созданном Server (без вызовов WithXxx)
// не должен паниковать, даже если rateLimitStopCh не инициализирован.
func TestServer_Close_NilStopCh(t *testing.T) {
	s := &Server{}
	assert.NotPanics(t, func() { s.Close() })
}
