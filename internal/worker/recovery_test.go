package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestRecoveryService(t *testing.T, cfg RecoveryConfig) (*RecoveryService, *MockMatchRepository, *MockQueueManager) {
	t.Helper()
	matchRepo := new(MockMatchRepository)
	queueMgr := new(MockQueueManager)
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
	stuckMatch := &models.Match{ID: uuid.New(), StartedAt: &now}

	pendingMatch := &models.Match{ID: uuid.New()}

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*models.Match{stuckMatch}, nil)
	matchRepo.On("BatchUpdateStatus", mock.Anything, []uuid.UUID{stuckMatch.ID}, models.MatchPending).
		Return(nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*models.Match{pendingMatch}, nil)
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
		Return([]*models.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*models.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
}

func TestRecoveryService_RecoverOnStartup_QueueSizeError_Continues(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), fmt.Errorf("redis error"))
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*models.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*models.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
}

func TestRecoveryService_RecoverOnStartup_StuckRecoveryError_Continues(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return(nil, fmt.Errorf("db error"))
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*models.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
}

func TestRecoveryService_RecoverOnStartup_EnqueueFails(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*models.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return(nil, fmt.Errorf("db error"))

	err := svc.RecoverOnStartup(context.Background())
	assert.Error(t, err)
}

func TestRecoveryService_RecoverOnStartup_PartialEnqueueFailure(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	m1 := &models.Match{ID: uuid.New()}
	m2 := &models.Match{ID: uuid.New()}

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*models.Match{}, nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).
		Return([]*models.Match{m1, m2}, nil)
	queueMgr.On("Enqueue", mock.Anything, m1).Return(fmt.Errorf("redis error"))
	queueMgr.On("Enqueue", mock.Anything, m2).Return(nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err) // частичные ошибки логируются, а не возвращаются
	queueMgr.AssertExpectations(t)
}

// --- recoverStuckRunning ---

func TestRecoveryService_RecoverStuckRunning_VerifiesMatchIDs(t *testing.T) {
	svc, matchRepo, _ := newTestRecoveryService(t, RecoveryConfig{})

	now := time.Now().Add(-15 * time.Minute)
	m1 := &models.Match{ID: uuid.New(), StartedAt: &now}
	m2 := &models.Match{ID: uuid.New(), StartedAt: &now}

	matchRepo.On("GetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return([]*models.Match{m1, m2}, nil)
	matchRepo.On("BatchUpdateStatus", mock.Anything, []uuid.UUID{m1.ID, m2.ID}, models.MatchPending).
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

	// Даём горутине запуститься
	time.Sleep(50 * time.Millisecond)

	// Stop не должен блокироваться
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// успех
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return in time")
	}
}
