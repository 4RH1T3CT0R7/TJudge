package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/executor"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Моки ---

type MockCompileQueue struct {
	mock.Mock
}

func (m *MockCompileQueue) Enqueue(ctx context.Context, programID uuid.UUID) error {
	args := m.Called(ctx, programID)
	return args.Error(0)
}

func (m *MockCompileQueue) Dequeue(ctx context.Context, timeout time.Duration) (*queue.CompileTask, error) {
	args := m.Called(ctx, timeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*queue.CompileTask), args.Error(1)
}

type MockProgramCompiler struct {
	mock.Mock
}

func (m *MockProgramCompiler) Compile(ctx context.Context, program *models.Program) (*executor.CompileResult, error) {
	args := m.Called(ctx, program)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*executor.CompileResult), args.Error(1)
}

func newTestCompileWorker(t *testing.T) (*CompileWorker, *MockCompileQueue, *MockProgramRepository, *MockProgramCompiler, *capturingNotifier) {
	t.Helper()
	q := new(MockCompileQueue)
	repo := new(MockProgramRepository)
	compiler := new(MockProgramCompiler)
	bus := &capturingNotifier{}
	log, _ := logger.New("error", "json")
	mr := miniredis.RunT(t)
	lock := cache.NewDistributedLock(cache.NewFromClient(redis.NewClient(&redis.Options{Addr: mr.Addr()})))
	w := NewCompileWorker(q, repo, compiler, lock, bus, log, 2)
	return w, q, repo, compiler, bus
}

func compilingProgram() *models.Program {
	tournamentID := uuid.New()
	teamID := uuid.New()
	src := "/data/programs/abc.c"
	return &models.Program{
		ID:           uuid.New(),
		Language:     "c",
		Status:       models.ProgramCompiling,
		CodePath:     src,
		FilePath:     &src,
		TournamentID: &tournamentID,
		TeamID:       &teamID,
	}
}

// --- Тесты ---

func TestCompileWorker_ProcessTask_Success(t *testing.T) {
	w, _, repo, compiler, bus := newTestCompileWorker(t)
	program := compilingProgram()
	task := &queue.CompileTask{ProgramID: program.ID}

	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)
	compiler.On("Compile", mock.Anything, program).
		Return(&executor.CompileResult{OK: true, ExecPath: "/data/programs/abc"}, nil)
	repo.On("UpdateCompileResult", mock.Anything, program.ID, models.ProgramReady, "/data/programs/abc", (*string)(nil)).
		Return(true, nil)

	w.processTask(context.Background(), 1, task)

	repo.AssertExpectations(t)
	compiler.AssertExpectations(t)
	// событие ProgramCompiled опубликовано со статусом ready
	assert.Len(t, bus.published, 1)
	evt, ok := bus.published[0].(events.ProgramCompiled)
	assert.True(t, ok)
	assert.Equal(t, "ready", evt.Status)
	assert.Equal(t, program.ID, evt.ProgramID)
	assert.Equal(t, *program.TournamentID, evt.TournamentID)
}

func TestCompileWorker_ProcessTask_CompileError(t *testing.T) {
	w, _, repo, compiler, bus := newTestCompileWorker(t)
	program := compilingProgram()
	task := &queue.CompileTask{ProgramID: program.ID}

	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)
	compiler.On("Compile", mock.Anything, program).
		Return(&executor.CompileResult{OK: false, Log: "main.c:1: error: expected ';'"}, nil)
	repo.On("UpdateCompileResult", mock.Anything, program.ID, models.ProgramFailed, program.CodePath, mock.MatchedBy(func(msg *string) bool {
		return msg != nil && *msg == "main.c:1: error: expected ';'"
	})).Return(true, nil)

	w.processTask(context.Background(), 1, task)

	repo.AssertExpectations(t)
	assert.Len(t, bus.published, 1)
	evt := bus.published[0].(events.ProgramCompiled)
	assert.Equal(t, "failed", evt.Status)
	assert.NotNil(t, evt.ErrorMessage)
}

// поздний дубль сборки: итог не записан, событие не публикуется
func TestCompileWorker_ProcessTask_ResultNotApplied(t *testing.T) {
	w, _, repo, compiler, bus := newTestCompileWorker(t)
	program := compilingProgram()

	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)
	compiler.On("Compile", mock.Anything, program).
		Return(&executor.CompileResult{OK: true, ExecPath: "/data/programs/abc"}, nil)
	repo.On("UpdateCompileResult", mock.Anything, program.ID, models.ProgramReady, "/data/programs/abc", (*string)(nil)).
		Return(false, nil)

	w.processTask(context.Background(), 1, &queue.CompileTask{ProgramID: program.ID})

	repo.AssertExpectations(t)
	assert.Empty(t, bus.published)
}

func TestCompileWorker_ProcessTask_InfraErrorLeavesCompiling(t *testing.T) {
	w, _, repo, compiler, bus := newTestCompileWorker(t)
	program := compilingProgram()
	task := &queue.CompileTask{ProgramID: program.ID}

	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)
	// docker недоступен: инфраструктурная ошибка
	compiler.On("Compile", mock.Anything, program).
		Return(nil, fmt.Errorf("failed to create builder container: daemon unreachable"))

	w.processTask(context.Background(), 1, task)

	// статус не меняется: stuck-recovery повторит задачу позже
	repo.AssertNotCalled(t, "UpdateCompileResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, bus.published)
}

func TestCompileWorker_ProcessTask_DuplicateSkipped(t *testing.T) {
	w, _, repo, compiler, _ := newTestCompileWorker(t)
	program := compilingProgram()
	program.Status = models.ProgramReady // уже обработана
	task := &queue.CompileTask{ProgramID: program.ID}

	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)

	w.processTask(context.Background(), 1, task)

	compiler.AssertNotCalled(t, "Compile", mock.Anything, mock.Anything)
}

func TestCompileWorker_ProcessTask_ProgramDeleted(t *testing.T) {
	w, _, repo, compiler, _ := newTestCompileWorker(t)
	programID := uuid.New()
	task := &queue.CompileTask{ProgramID: programID}

	repo.On("GetByID", mock.Anything, programID).Return(nil, errors.ErrProgramNotFound)

	w.processTask(context.Background(), 1, task)

	compiler.AssertNotCalled(t, "Compile", mock.Anything, mock.Anything)
}

func TestCompileWorker_RecoverStuck(t *testing.T) {
	w, q, repo, _, _ := newTestCompileWorker(t)
	p1 := compilingProgram()
	p2 := compilingProgram()

	repo.On("GetStuckCompiling", mock.Anything, w.stuckOlderThan, w.stuckBatchSize).
		Return([]*models.Program{p1, p2}, nil)
	q.On("Enqueue", mock.Anything, p1.ID).Return(nil)
	q.On("Enqueue", mock.Anything, p2.ID).Return(nil)

	w.recoverStuck(context.Background())

	q.AssertExpectations(t)
}

// пока программа собирается (лок держит другая горутина или реплика), дубль
// задачи не запускает вторую сборку и не трогает статус
func TestCompileWorker_ProcessTask_SkipsWhileCompiling(t *testing.T) {
	w, _, repo, compiler, bus := newTestCompileWorker(t)
	program := compilingProgram()
	_, err := w.lock.Lock(context.Background(), compileLockKey(program.ID), time.Minute)
	require.NoError(t, err)

	w.processTask(context.Background(), 1, &queue.CompileTask{ProgramID: program.ID})

	repo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	compiler.AssertNotCalled(t, "Compile", mock.Anything, mock.Anything)
	assert.Empty(t, bus.published)
}

// лок снимается после сборки, следующая задача той же программы не блокируется
func TestCompileWorker_ProcessTask_ReleasesLock(t *testing.T) {
	w, _, repo, compiler, _ := newTestCompileWorker(t)
	program := compilingProgram()
	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)
	compiler.On("Compile", mock.Anything, program).Return(nil, fmt.Errorf("daemon unreachable"))

	w.processTask(context.Background(), 1, &queue.CompileTask{ProgramID: program.ID})

	locked, err := w.lock.IsLocked(context.Background(), compileLockKey(program.ID))
	require.NoError(t, err)
	assert.False(t, locked)
}

// stuck-recovery не переотправляет программу, которая прямо сейчас собирается
func TestCompileWorker_RecoverStuck_SkipsCompiling(t *testing.T) {
	w, q, repo, _, _ := newTestCompileWorker(t)
	building := compilingProgram()
	lost := compilingProgram()
	_, err := w.lock.Lock(context.Background(), compileLockKey(building.ID), time.Minute)
	require.NoError(t, err)

	repo.On("GetStuckCompiling", mock.Anything, w.stuckOlderThan, w.stuckBatchSize).
		Return([]*models.Program{building, lost}, nil)
	q.On("Enqueue", mock.Anything, lost.ID).Return(nil)

	w.recoverStuck(context.Background())

	q.AssertExpectations(t)
	q.AssertNotCalled(t, "Enqueue", mock.Anything, building.ID)
}

func TestCompileWorker_StartStop(t *testing.T) {
	w, q, repo, _, _ := newTestCompileWorker(t)
	w.dequeueWaitLimit = 10 * time.Millisecond
	w.stuckInterval = time.Hour // recovery в этом тесте не запускается

	q.On("Dequeue", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	repo.On("GetStuckCompiling", mock.Anything, mock.Anything, mock.Anything).
		Return([]*models.Program{}, nil).Maybe()

	w.Start()
	time.Sleep(30 * time.Millisecond)
	w.Stop() // не должен зависнуть
}
