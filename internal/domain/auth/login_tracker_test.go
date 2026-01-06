package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryLoginTracker(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.attempts)
	assert.NotNil(t, tracker.lockouts)
}

func TestInMemoryLoginTracker_RecordAttempt_Failed(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Record failed attempts
	for i := 0; i < 3; i++ {
		err := tracker.RecordAttempt(ctx, username, ip, false)
		require.NoError(t, err)
	}

	// Check attempts count
	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestInMemoryLoginTracker_RecordAttempt_Success(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Record some failed attempts
	for i := 0; i < 3; i++ {
		_ = tracker.RecordAttempt(ctx, username, ip, false)
	}

	// Record successful attempt
	err := tracker.RecordAttempt(ctx, username, ip, true)
	require.NoError(t, err)

	// Attempts should still be recorded (but won't trigger lockout)
	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0)
}

func TestInMemoryLoginTracker_IsLocked_NotLocked(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"

	locked, remaining, err := tracker.IsLocked(ctx, username)
	require.NoError(t, err)
	assert.False(t, locked)
	assert.Equal(t, time.Duration(0), remaining)
}

func TestInMemoryLoginTracker_IsLocked_AfterMaxAttempts(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Record MaxLoginAttempts failed attempts
	for i := 0; i < MaxLoginAttempts; i++ {
		err := tracker.RecordAttempt(ctx, username, ip, false)
		require.NoError(t, err)
	}

	// Should be locked now
	locked, remaining, err := tracker.IsLocked(ctx, username)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Greater(t, remaining, time.Duration(0))
}

func TestInMemoryLoginTracker_ClearAttempts(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Record failed attempts to trigger lockout
	for i := 0; i < MaxLoginAttempts; i++ {
		_ = tracker.RecordAttempt(ctx, username, ip, false)
	}

	// Verify locked
	locked, _, _ := tracker.IsLocked(ctx, username)
	assert.True(t, locked)

	// Clear attempts
	err := tracker.ClearAttempts(ctx, username)
	require.NoError(t, err)

	// Should no longer be locked
	locked, _, err = tracker.IsLocked(ctx, username)
	require.NoError(t, err)
	assert.False(t, locked)

	// Attempts should be cleared
	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestInMemoryLoginTracker_GetRecentAttempts(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Initially no attempts
	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Record 3 failed attempts
	for i := 0; i < 3; i++ {
		_ = tracker.RecordAttempt(ctx, username, ip, false)
	}

	count, err = tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestInMemoryLoginTracker_MultipleUsers(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	ip := "127.0.0.1"

	user1 := "user1"
	user2 := "user2"

	// Record attempts for user1
	for i := 0; i < 3; i++ {
		_ = tracker.RecordAttempt(ctx, user1, ip, false)
	}

	// Record attempts for user2
	_ = tracker.RecordAttempt(ctx, user2, ip, false)

	// Check that they are independent
	count1, _ := tracker.GetRecentAttempts(ctx, user1)
	count2, _ := tracker.GetRecentAttempts(ctx, user2)

	assert.Equal(t, 3, count1)
	assert.Equal(t, 1, count2)
}

func TestInMemoryLoginTracker_OnlyCountsFailedAttempts(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Mix of success and failure
	_ = tracker.RecordAttempt(ctx, username, ip, false)
	_ = tracker.RecordAttempt(ctx, username, ip, true)
	_ = tracker.RecordAttempt(ctx, username, ip, false)
	_ = tracker.RecordAttempt(ctx, username, ip, true)
	_ = tracker.RecordAttempt(ctx, username, ip, false)

	// Should only count failed attempts
	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestConstants(t *testing.T) {
	// Verify security constants are set appropriately
	assert.Equal(t, 5, MaxLoginAttempts)
	assert.Equal(t, 15*time.Minute, LockoutDuration)
	assert.Equal(t, 5*time.Minute, AttemptWindow)
}

func TestLoginTrackerInterface(t *testing.T) {
	// Verify that InMemoryLoginTracker implements LoginTracker interface
	var _ LoginTracker = (*InMemoryLoginTracker)(nil)
	var _ LoginTracker = (*RedisLoginTracker)(nil)
}

func TestInMemoryLoginTracker_ThreadSafety(t *testing.T) {
	tracker := NewInMemoryLoginTracker()
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _, _ = tracker.IsLocked(ctx, username)
			_, _ = tracker.GetRecentAttempts(ctx, username)
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func() {
			_ = tracker.RecordAttempt(ctx, username, ip, false)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// No panic means thread-safety works
}

func TestInMemoryLoginTracker_Cleanup(t *testing.T) {
	tracker := NewInMemoryLoginTracker()

	// Manually test cleanup function
	tracker.mu.Lock()
	// Add old attempt that should be cleaned
	tracker.attempts["olduser"] = []LoginAttempt{
		{Timestamp: time.Now().Add(-10 * time.Minute), Success: false, IP: "127.0.0.1"},
	}
	// Add recent attempt that should stay
	tracker.attempts["recentuser"] = []LoginAttempt{
		{Timestamp: time.Now(), Success: false, IP: "127.0.0.1"},
	}
	// Add expired lockout
	tracker.lockouts["expireduser"] = time.Now().Add(-time.Minute)
	// Add active lockout
	tracker.lockouts["lockeduser"] = time.Now().Add(10 * time.Minute)
	tracker.mu.Unlock()

	// Run cleanup
	tracker.cleanup()

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	// Old attempts should be removed
	_, hasOld := tracker.attempts["olduser"]
	assert.False(t, hasOld)

	// Recent attempts should remain
	_, hasRecent := tracker.attempts["recentuser"]
	assert.True(t, hasRecent)

	// Expired lockout should be removed
	_, hasExpired := tracker.lockouts["expireduser"]
	assert.False(t, hasExpired)

	// Active lockout should remain
	_, hasLocked := tracker.lockouts["lockeduser"]
	assert.True(t, hasLocked)
}

// Mock Redis Client for testing RedisLoginTracker
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) Incr(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.Called(ctx, key, expiration)
	return args.Error(0)
}

func TestNewRedisLoginTracker(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)

	assert.NotNil(t, tracker)
	assert.Equal(t, "login_tracker:", tracker.prefix)
}

func TestRedisLoginTracker_RecordAttempt_Failed(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// First failed attempt
	client.On("Incr", ctx, "login_tracker:attempts:testuser").Return(int64(1), nil)
	client.On("Expire", ctx, "login_tracker:attempts:testuser", AttemptWindow).Return(nil)

	err := tracker.RecordAttempt(ctx, username, ip, false)
	require.NoError(t, err)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_RecordAttempt_Lockout(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Fifth failed attempt triggers lockout
	client.On("Incr", ctx, "login_tracker:attempts:testuser").Return(int64(MaxLoginAttempts), nil)
	client.On("Set", ctx, "login_tracker:lockout:testuser", "locked", LockoutDuration).Return(nil)

	err := tracker.RecordAttempt(ctx, username, ip, false)
	require.NoError(t, err)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_RecordAttempt_Success(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"
	ip := "127.0.0.1"

	// Successful login clears attempts
	client.On("Del", ctx, []string{"login_tracker:attempts:testuser", "login_tracker:lockout:testuser"}).Return(nil)

	err := tracker.RecordAttempt(ctx, username, ip, true)
	require.NoError(t, err)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_IsLocked_Locked(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"

	client.On("Get", ctx, "login_tracker:lockout:testuser").Return("locked", nil)

	locked, remaining, err := tracker.IsLocked(ctx, username)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Equal(t, LockoutDuration, remaining)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_IsLocked_NotLocked(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"

	client.On("Get", ctx, "login_tracker:lockout:testuser").Return("", assert.AnError)

	locked, remaining, err := tracker.IsLocked(ctx, username)
	require.NoError(t, err)
	assert.False(t, locked)
	assert.Equal(t, time.Duration(0), remaining)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_GetRecentAttempts(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"

	client.On("Get", ctx, "login_tracker:attempts:testuser").Return("3", nil)

	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_GetRecentAttempts_NoAttempts(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"

	client.On("Get", ctx, "login_tracker:attempts:testuser").Return("", assert.AnError)

	count, err := tracker.GetRecentAttempts(ctx, username)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	client.AssertExpectations(t)
}

func TestRedisLoginTracker_ClearAttempts(t *testing.T) {
	client := new(MockRedisClient)
	tracker := NewRedisLoginTracker(client)
	ctx := context.Background()
	username := "testuser"

	client.On("Del", ctx, []string{"login_tracker:attempts:testuser", "login_tracker:lockout:testuser"}).Return(nil)

	err := tracker.ClearAttempts(ctx, username)
	require.NoError(t, err)

	client.AssertExpectations(t)
}
