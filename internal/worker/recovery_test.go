package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRecoveryMatchRepository is a mock for RecoveryMatchRepository
type MockRecoveryMatchRepository struct {
	mock.Mock
}

func (m *MockRecoveryMatchRepository) GetPending(ctx context.Context, limit int) ([]*domain.Match, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockRecoveryMatchRepository) GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*domain.Match, error) {
	args := m.Called(ctx, stuckDuration, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockRecoveryMatchRepository) BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status domain.MatchStatus) error {
	args := m.Called(ctx, matchIDs, status)
	return args.Error(0)
}

// MockRecoveryQueueManager is a mock for RecoveryQueueManager
type MockRecoveryQueueManager struct {
	mock.Mock
}

func (m *MockRecoveryQueueManager) Enqueue(ctx context.Context, match *domain.Match) error {
	args := m.Called(ctx, match)
	return args.Error(0)
}

func (m *MockRecoveryQueueManager) GetTotalQueueSize(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func newTestRecoveryService(t *testing.T, cfg RecoveryConfig) (*RecoveryService, *MockRecoveryMatchRepository, *MockRecoveryQueueManager) {
	t.Helper()
	matchRepo := new(MockRecoveryMatchRepository)
	queueMgr := new(MockRecoveryQueueManager)
	log, _ := logger.New("error", "json")
	svc := NewRecoveryService(matchRepo, queueMgr, log, cfg)
	return svc, matchRepo, queueMgr
}

// --- NewRecoveryService ---

func TestNewRecoveryService_Defaults(t *testing.T) {
	svc, _, _ := newTestRecoveryService(t, RecoveryConfig{})

	assert.Equal(t, 10*time.Minute, svc.stuckDuration)
	assert.Equal(t, 1000, svc.batchSize)
	assert.Equal(t, 5*time.Minute, svc.periodicInterval)
}

func TestNewRecoveryService_CustomConfig(t *testing.T) {
	cfg := RecoveryConfig{
		StuckDuration:    20 * time.Minute,
		BatchSize:        500,
		PeriodicInterval: 2 * time.Minute,
	}
	svc, _, _ := newTestRecoveryService(t, cfg)

	assert.Equal(t, 20*time.Minute, svc.stuckDuration)
	assert.Equal(t, 500, svc.batchSize)
	assert.Equal(t, 2*time.Minute, svc.periodicInterval)
}

// --- RecoverOnStartup ---

func TestRecoveryService_RecoverOnStartup_Success(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	now := time.Now().Add(-20 * time.Minute)
	stuckMatch := &domain.Match{ID: uuid.New(), StartedAt: &now}

	pendingMatch := &domain.Match{ID: uuid.New()}

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*domain.Match{stuckMatch}, nil)
	matchRepo.On("BatchUpdateStatus", mock.Anything, []uuid.UUID{stuckMatch.ID}, domain.MatchPending).
		Return(nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*domain.Match{pendingMatch}, nil)
	queueMgr.On("Enqueue", mock.Anything, pendingMatch).Return(nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
	queueMgr.AssertExpectations(t)
}

func TestRecoveryService_RecoverOnStartup_NoStuckNoPending(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(5), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*domain.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*domain.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
}

func TestRecoveryService_RecoverOnStartup_QueueSizeError_Continues(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), fmt.Errorf("redis error"))
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*domain.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*domain.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
}

func TestRecoveryService_RecoverOnStartup_StuckRecoveryError_Continues(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return(nil, fmt.Errorf("db error"))
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*domain.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
}

func TestRecoveryService_RecoverOnStartup_EnqueueFails(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*domain.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return(nil, fmt.Errorf("db error"))

	err := svc.RecoverOnStartup(context.Background())
	assert.Error(t, err)
}

func TestRecoveryService_RecoverOnStartup_PartialEnqueueFailure(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	m1 := &domain.Match{ID: uuid.New()}
	m2 := &domain.Match{ID: uuid.New()}

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*domain.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*domain.Match{m1, m2}, nil)
	queueMgr.On("Enqueue", mock.Anything, m1).Return(fmt.Errorf("redis error"))
	queueMgr.On("Enqueue", mock.Anything, m2).Return(nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err) // partial failures are logged, not returned
	queueMgr.AssertExpectations(t)
}

// --- recoverStuckRunning ---

func TestRecoveryService_RecoverStuckRunning_VerifiesMatchIDs(t *testing.T) {
	svc, matchRepo, _ := newTestRecoveryService(t, RecoveryConfig{})

	now := time.Now().Add(-15 * time.Minute)
	m1 := &domain.Match{ID: uuid.New(), StartedAt: &now}
	m2 := &domain.Match{ID: uuid.New(), StartedAt: &now}

	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*domain.Match{m1, m2}, nil)
	matchRepo.On("BatchUpdateStatus", mock.Anything, []uuid.UUID{m1.ID, m2.ID}, domain.MatchPending).
		Return(nil)

	count, err := svc.recoverStuckRunning(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	matchRepo.AssertExpectations(t)
}

// --- Start/Stop ---

func TestRecoveryService_StartStop(t *testing.T) {
	cfg := RecoveryConfig{
		PeriodicInterval: 100 * time.Millisecond,
	}
	svc, _, _ := newTestRecoveryService(t, cfg)

	svc.Start()

	// Let the goroutine start
	time.Sleep(50 * time.Millisecond)

	// Stop should not block
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return in time")
	}
}
