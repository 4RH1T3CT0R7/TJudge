package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/executor"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

type MockCompileProgramRepo struct {
	mock.Mock
}

func (m *MockCompileProgramRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Program), args.Error(1)
}

func (m *MockCompileProgramRepo) UpdateCompileResult(ctx context.Context, id uuid.UUID, status domain.ProgramStatus, codePath string, errorMessage *string) error {
	args := m.Called(ctx, id, status, codePath, errorMessage)
	return args.Error(0)
}

func (m *MockCompileProgramRepo) GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*domain.Program, error) {
	args := m.Called(ctx, olderThan, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Program), args.Error(1)
}

type MockProgramCompiler struct {
	mock.Mock
}

func (m *MockProgramCompiler) Compile(ctx context.Context, program *domain.Program) (*executor.CompileResult, error) {
	args := m.Called(ctx, program)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*executor.CompileResult), args.Error(1)
}

func newTestCompileWorker(t *testing.T) (*CompileWorker, *MockCompileQueue, *MockCompileProgramRepo, *MockProgramCompiler, *capturingBus) {
	t.Helper()
	q := new(MockCompileQueue)
	repo := new(MockCompileProgramRepo)
	compiler := new(MockProgramCompiler)
	bus := &capturingBus{}
	log, _ := logger.New("error", "json")
	w := NewCompileWorker(q, repo, compiler, bus, log)
	return w, q, repo, compiler, bus
}

func compilingProgram() *domain.Program {
	tournamentID := uuid.New()
	teamID := uuid.New()
	src := "/data/programs/abc.c"
	return &domain.Program{
		ID:           uuid.New(),
		Language:     "c",
		Status:       domain.ProgramCompiling,
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
	repo.On("UpdateCompileResult", mock.Anything, program.ID, domain.ProgramReady, "/data/programs/abc", (*string)(nil)).
		Return(nil)

	w.processTask(context.Background(), 1, task)

	repo.AssertExpectations(t)
	compiler.AssertExpectations(t)
	// Событие ProgramCompiled опубликовано со статусом ready.
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
	repo.On("UpdateCompileResult", mock.Anything, program.ID, domain.ProgramFailed, program.CodePath, mock.MatchedBy(func(msg *string) bool {
		return msg != nil && *msg == "main.c:1: error: expected ';'"
	})).Return(nil)

	w.processTask(context.Background(), 1, task)

	repo.AssertExpectations(t)
	assert.Len(t, bus.published, 1)
	evt := bus.published[0].(events.ProgramCompiled)
	assert.Equal(t, "failed", evt.Status)
	assert.NotNil(t, evt.ErrorMessage)
}

func TestCompileWorker_ProcessTask_InfraErrorLeavesCompiling(t *testing.T) {
	w, _, repo, compiler, bus := newTestCompileWorker(t)
	program := compilingProgram()
	task := &queue.CompileTask{ProgramID: program.ID}

	repo.On("GetByID", mock.Anything, program.ID).Return(program, nil)
	// Docker недоступен: инфраструктурная ошибка.
	compiler.On("Compile", mock.Anything, program).
		Return(nil, fmt.Errorf("failed to create builder container: daemon unreachable"))

	w.processTask(context.Background(), 1, task)

	// Статус НЕ меняется - stuck-recovery повторит задачу позже.
	repo.AssertNotCalled(t, "UpdateCompileResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, bus.published)
}

func TestCompileWorker_ProcessTask_DuplicateSkipped(t *testing.T) {
	w, _, repo, compiler, _ := newTestCompileWorker(t)
	program := compilingProgram()
	program.Status = domain.ProgramReady // уже обработана
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
		Return([]*domain.Program{p1, p2}, nil)
	q.On("Enqueue", mock.Anything, p1.ID).Return(nil)
	q.On("Enqueue", mock.Anything, p2.ID).Return(nil)

	w.recoverStuck(context.Background())

	q.AssertExpectations(t)
}

func TestCompileWorker_StartStop(t *testing.T) {
	w, q, repo, _, _ := newTestCompileWorker(t)
	w.dequeueWaitLimit = 10 * time.Millisecond
	w.stuckInterval = time.Hour // recovery в этом тесте не дёргаем

	q.On("Dequeue", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	repo.On("GetStuckCompiling", mock.Anything, mock.Anything, mock.Anything).
		Return([]*domain.Program{}, nil).Maybe()

	w.Start()
	time.Sleep(30 * time.Millisecond)
	w.Stop() // не должен зависнуть
}
