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

// пустой конфиг подставляет дефолтные значения
func TestNewRecoveryService_Defaults(t *testing.T) {
	svc, _, _ := newTestRecoveryService(t, RecoveryConfig{})

	assert.Equal(t, 10*time.Minute, svc.stuckDuration)
	assert.Equal(t, 1000, svc.batchSize)
	assert.Equal(t, 5*time.Minute, svc.periodicInterval)
}

// зависшие running переводятся в pending, pending переотправляется в очередь
func TestRecoveryService_RecoverOnStartup_Success(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	pending := []*models.Match{{ID: uuid.New()}}

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("ResetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).Return(int64(1), nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).Return(pending, nil)
	queueMgr.On("EnqueueBatch", mock.Anything, pending).Return(nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
	queueMgr.AssertExpectations(t)
}

// ошибка чтения размера очереди не прерывает восстановление
func TestRecoveryService_RecoverOnStartup_QueueSizeError_Continues(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), fmt.Errorf("redis error"))
	matchRepo.On("ResetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).Return(int64(0), nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).Return([]*models.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
}

// ошибка восстановления зависших не прерывает восстановление pending
func TestRecoveryService_RecoverOnStartup_StuckRecoveryError_Continues(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("ResetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).
		Return(int64(0), fmt.Errorf("db error"))
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).Return([]*models.Match{}, nil)

	err := svc.RecoverOnStartup(context.Background())
	assert.NoError(t, err)
}

// ошибка чтения pending пробрасывается наружу
func TestRecoveryService_RecoverOnStartup_EnqueueFails(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	queueMgr.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)
	matchRepo.On("ResetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).Return(int64(0), nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).Return(nil, fmt.Errorf("db error"))

	err := svc.RecoverOnStartup(context.Background())
	assert.Error(t, err)
}

// периодика ставит pending в очередь и без застрявших: матч, исчерпавший
// ретраи пула на инфра-ошибке, лежит pending вне очереди
func TestRecoveryService_Periodic_EnqueuesPendingWithoutStuck(t *testing.T) {
	svc, matchRepo, queueMgr := newTestRecoveryService(t, RecoveryConfig{})

	orphan := []*models.Match{{ID: uuid.New(), Status: models.MatchPending}}
	matchRepo.On("ResetStuckRunning", mock.Anything, svc.stuckDuration, svc.batchSize).Return(int64(0), nil)
	matchRepo.On("GetPending", mock.Anything, svc.batchSize).Return(orphan, nil)
	queueMgr.On("EnqueueBatch", mock.Anything, orphan).Return(nil)

	svc.runPeriodicRecovery()
	queueMgr.AssertExpectations(t)
}

// Stop не блокируется при работающем периодическом цикле
func TestRecoveryService_StartStop(t *testing.T) {
	svc, _, _ := newTestRecoveryService(t, RecoveryConfig{PeriodicInterval: 100 * time.Millisecond})

	svc.Start()
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return in time")
	}
}
