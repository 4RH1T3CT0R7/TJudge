package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// MaxLoginAttempts максимальное количество попыток входа
	MaxLoginAttempts = 5

	// LockoutDuration время блокировки аккаунта
	LockoutDuration = 15 * time.Minute

	// AttemptWindow окно времени для подсчёта попыток
	AttemptWindow = 5 * time.Minute
)

// LoginAttempt информация о попытке входа
type LoginAttempt struct {
	Timestamp time.Time
	Success   bool
	IP        string
}

// LoginTracker отслеживает попытки входа для защиты от brute-force
type LoginTracker interface {
	// RecordAttempt записывает попытку входа
	RecordAttempt(ctx context.Context, username, ip string, success bool) error

	// IsLocked проверяет, заблокирован ли аккаунт
	IsLocked(ctx context.Context, username string) (bool, time.Duration, error)

	// GetRecentAttempts возвращает количество недавних неудачных попыток
	GetRecentAttempts(ctx context.Context, username string) (int, error)

	// ClearAttempts очищает счётчик попыток (после успешного входа)
	ClearAttempts(ctx context.Context, username string) error
}

// InMemoryLoginTracker простая реализация в памяти (для production использовать Redis)
type InMemoryLoginTracker struct {
	attempts map[string][]LoginAttempt
	lockouts map[string]time.Time
	mu       sync.RWMutex
}

// NewInMemoryLoginTracker создаёт новый tracker
func NewInMemoryLoginTracker() *InMemoryLoginTracker {
	tracker := &InMemoryLoginTracker{
		attempts: make(map[string][]LoginAttempt),
		lockouts: make(map[string]time.Time),
	}

	// Запускаем cleanup горутину
	go tracker.cleanupLoop()

	return tracker
}

// RecordAttempt записывает попытку входа
func (t *InMemoryLoginTracker) RecordAttempt(ctx context.Context, username, ip string, success bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	attempt := LoginAttempt{
		Timestamp: time.Now(),
		Success:   success,
		IP:        ip,
	}

	t.attempts[username] = append(t.attempts[username], attempt)

	// Если достигнут лимит неудачных попыток - блокируем
	if !success {
		failedCount := t.countRecentFailedAttempts(username)
		if failedCount >= MaxLoginAttempts {
			t.lockouts[username] = time.Now().Add(LockoutDuration)
		}
	}

	return nil
}

// IsLocked проверяет блокировку
func (t *InMemoryLoginTracker) IsLocked(ctx context.Context, username string) (bool, time.Duration, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	lockoutTime, exists := t.lockouts[username]
	if !exists {
		return false, 0, nil
	}

	remaining := time.Until(lockoutTime)
	if remaining <= 0 {
		return false, 0, nil
	}

	return true, remaining, nil
}

// GetRecentAttempts возвращает количество недавних неудачных попыток
func (t *InMemoryLoginTracker) GetRecentAttempts(ctx context.Context, username string) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.countRecentFailedAttempts(username), nil
}

// ClearAttempts очищает попытки после успешного входа
func (t *InMemoryLoginTracker) ClearAttempts(ctx context.Context, username string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.attempts, username)
	delete(t.lockouts, username)

	return nil
}

// countRecentFailedAttempts подсчитывает неудачные попытки за окно времени
func (t *InMemoryLoginTracker) countRecentFailedAttempts(username string) int {
	attempts, exists := t.attempts[username]
	if !exists {
		return 0
	}

	count := 0
	cutoff := time.Now().Add(-AttemptWindow)

	for _, a := range attempts {
		if !a.Success && a.Timestamp.After(cutoff) {
			count++
		}
	}

	return count
}

// cleanupLoop периодически очищает старые записи
func (t *InMemoryLoginTracker) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.cleanup()
	}
}

// cleanup удаляет устаревшие записи
func (t *InMemoryLoginTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-AttemptWindow)

	// Очищаем старые попытки
	for username, attempts := range t.attempts {
		var recent []LoginAttempt
		for _, a := range attempts {
			if a.Timestamp.After(cutoff) {
				recent = append(recent, a)
			}
		}
		if len(recent) == 0 {
			delete(t.attempts, username)
		} else {
			t.attempts[username] = recent
		}
	}

	// Очищаем истёкшие блокировки
	for username, lockoutTime := range t.lockouts {
		if now.After(lockoutTime) {
			delete(t.lockouts, username)
		}
	}
}

// RedisLoginTracker реализация на Redis (для production)
type RedisLoginTracker struct {
	client RedisClient
	prefix string
}

// RedisClient интерфейс для Redis клиента
type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// NewRedisLoginTracker создаёт tracker на Redis
func NewRedisLoginTracker(client RedisClient) *RedisLoginTracker {
	return &RedisLoginTracker{
		client: client,
		prefix: "login_tracker:",
	}
}

// RecordAttempt записывает попытку входа в Redis
func (t *RedisLoginTracker) RecordAttempt(ctx context.Context, username, ip string, success bool) error {
	if success {
		// При успешном входе очищаем счётчик
		return t.ClearAttempts(ctx, username)
	}

	// Увеличиваем счётчик неудачных попыток
	key := t.prefix + "attempts:" + username
	count, err := t.client.Incr(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to increment attempts: %w", err)
	}

	// Устанавливаем TTL
	if count == 1 {
		if err := t.client.Expire(ctx, key, AttemptWindow); err != nil {
			return fmt.Errorf("failed to set TTL: %w", err)
		}
	}

	// Проверяем, нужно ли блокировать
	if count >= MaxLoginAttempts {
		lockKey := t.prefix + "lockout:" + username
		if err := t.client.Set(ctx, lockKey, "locked", LockoutDuration); err != nil {
			return fmt.Errorf("failed to set lockout: %w", err)
		}
	}

	return nil
}

// IsLocked проверяет блокировку в Redis
func (t *RedisLoginTracker) IsLocked(ctx context.Context, username string) (bool, time.Duration, error) {
	lockKey := t.prefix + "lockout:" + username
	_, err := t.client.Get(ctx, lockKey)
	if err != nil {
		// Ключ не существует = не заблокирован
		return false, 0, nil
	}

	// Возвращаем максимальное время блокировки (точное время требует расширения RedisClient интерфейса)
	return true, LockoutDuration, nil
}

// GetRecentAttempts возвращает количество попыток из Redis
func (t *RedisLoginTracker) GetRecentAttempts(ctx context.Context, username string) (int, error) {
	key := t.prefix + "attempts:" + username
	val, err := t.client.Get(ctx, key)
	if err != nil {
		return 0, nil
	}

	var count int
	_, _ = fmt.Sscanf(val, "%d", &count)
	return count, nil
}

// ClearAttempts очищает счётчик в Redis
func (t *RedisLoginTracker) ClearAttempts(ctx context.Context, username string) error {
	attemptsKey := t.prefix + "attempts:" + username
	lockoutKey := t.prefix + "lockout:" + username

	return t.client.Del(ctx, attemptsKey, lockoutKey)
}
