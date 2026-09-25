package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/executor"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMatchRepository - мок MatchRepository
type MockMatchRepository struct {
	mock.Mock
}

func (m *MockMatchRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Match, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Match), args.Error(1)
}

func (m *MockMatchRepository) GetPending(ctx context.Context, limit int) ([]*models.Match, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Match), args.Error(1)
}

func (m *MockMatchRepository) GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*models.Match, error) {
	args := m.Called(ctx, stuckDuration, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Match), args.Error(1)
}

func (m *MockMatchRepository) BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status models.MatchStatus) error {
	args := m.Called(ctx, matchIDs, status)
	return args.Error(0)
}

func (m *MockMatchRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.MatchStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockMatchRepository) UpdateResult(ctx context.Context, id uuid.UUID, result *models.MatchResult) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

func (m *MockMatchRepository) UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *models.MatchResult) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

func (m *MockMatchRepository) MarkRatingApplied(ctx context.Context, matchID uuid.UUID) error {
	args := m.Called(ctx, matchID)
	return args.Error(0)
}

func (m *MockMatchRepository) ResetToPending(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockRatingRepository - мок RatingRepository
type MockRatingRepository struct {
	mock.Mock
}

func (m *MockRatingRepository) GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (int, int, error) {
	args := m.Called(ctx, tournamentID, program1ID, program2ID)
	return args.Int(0), args.Int(1), args.Error(2)
}

func (m *MockRatingRepository) GetByMatchID(ctx context.Context, matchID uuid.UUID) ([]*models.RatingHistory, error) {
	args := m.Called(ctx, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.RatingHistory), args.Error(1)
}

// MockRatingService - мок RatingService
type MockRatingService struct {
	mock.Mock
}

func (m *MockRatingService) ProcessMatchResult(ctx context.Context, match *models.Match, rating1, rating2 int) error {
	args := m.Called(ctx, match, rating1, rating2)
	return args.Error(0)
}

// MockExecutor - мок Executor
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, match *models.Match, program1Path, program2Path string) (*models.MatchResult, error) {
	args := m.Called(ctx, match, program1Path, program2Path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MatchResult), args.Error(1)
}

// MockProgramRepository - мок ProgramRepository
type MockProgramRepository struct {
	mock.Mock
}

func (m *MockProgramRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Program), args.Error(1)
}

func (m *MockProgramRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.Program, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Program), args.Error(1)
}

func (m *MockProgramRepository) UpdateCompileResult(ctx context.Context, id uuid.UUID, status models.ProgramStatus, codePath string, errorMessage *string) error {
	args := m.Called(ctx, id, status, codePath, errorMessage)
	return args.Error(0)
}

func (m *MockProgramRepository) GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*models.Program, error) {
	args := m.Called(ctx, olderThan, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Program), args.Error(1)
}

func newTestProcessor(t *testing.T) (*Processor, *MockMatchRepository, *MockRatingRepository, *MockProgramRepository, *MockRatingService, *MockExecutor) {
	t.Helper()
	matchRepo := new(MockMatchRepository)
	ratingRepo := new(MockRatingRepository)
	programRepo := new(MockProgramRepository)
	ratingService := new(MockRatingService)
	executor := new(MockExecutor)
	log, _ := logger.New("error", "json")

	p := &Processor{
		matchRepo:     matchRepo,
		ratingRepo:    ratingRepo,
		programRepo:   programRepo,
		ratingService: ratingService,
		executor:      executor,
		log:           log,
	}

	return p, matchRepo, ratingRepo, programRepo, ratingService, executor
}

func testProcessorMatch() *models.Match {
	return &models.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}
}

func twoPrograms(match *models.Match) []*models.Program {
	return []*models.Program{
		{ID: match.Program1ID, CodePath: "/path/p1"},
		{ID: match.Program2ID, CodePath: "/path/p2"},
	}
}

func TestProcessor_Process_AlreadyProcessed(t *testing.T) {
	p, matchRepo, _, _, _, _ := newTestProcessor(t)
	match := testProcessorMatch()

	// дубликат из очереди: UpdateStatus вернул ErrMatchAlreadyProcessed
	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).
		Return(models.ErrMatchAlreadyProcessed)

	err := p.Process(context.Background(), match)
	assert.NoError(t, err) // тихо пропускается, а не считается ошибкой
	matchRepo.AssertExpectations(t)
}

func TestProcessor_Process_UpdateStatusNotFound(t *testing.T) {
	p, matchRepo, _, _, _, _ := newTestProcessor(t)
	match := testProcessorMatch()

	// матч исчез: not-found мапится в ErrMatchNotFound (пул пропустит без retry)
	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).
		Return(errors.ErrNotFound)

	err := p.Process(context.Background(), match)
	assert.ErrorIs(t, err, ErrMatchNotFound)
	matchRepo.AssertExpectations(t)
}

func TestProcessor_Process_ExecutorFailure(t *testing.T) {
	p, matchRepo, _, programRepo, _, executor := newTestProcessor(t)
	match := testProcessorMatch()

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").
		Return(nil, fmt.Errorf("invalid output format"))
	matchRepo.On("UpdateResult", mock.Anything, match.ID, mock.AnythingOfType("*models.MatchResult")).Return(nil)

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	// ошибка программы терминальна: матч помечен failed, ретраи не нужны
	assert.ErrorIs(t, err, ErrProgramFailed)
	matchRepo.AssertExpectations(t)
	executor.AssertExpectations(t)
	matchRepo.AssertNotCalled(t, "ResetToPending", mock.Anything, mock.Anything)
}

func TestProcessor_Process_ExecutorInfraFailure_ResetsToPending(t *testing.T) {
	p, matchRepo, _, programRepo, _, executorMock := newTestProcessor(t)
	match := testProcessorMatch()

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	// инфраструктурная ошибка: docker daemon недоступен
	executorMock.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").
		Return(nil, &executor.InfraError{Err: fmt.Errorf("failed to create container: daemon unreachable")})
	matchRepo.On("ResetToPending", mock.Anything, match.ID).Return(nil)

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	// транзиентная ошибка не терминальна: пул будет ретраить
	assert.NotErrorIs(t, err, ErrProgramFailed)
	matchRepo.AssertExpectations(t)
	// матч не должен помечаться failed
	matchRepo.AssertNotCalled(t, "UpdateResult", mock.Anything, mock.Anything, mock.Anything)
	matchRepo.AssertNotCalled(t, "UpdateResultWithOutbox", mock.Anything, mock.Anything, mock.Anything)
}

// ctx матча истёк посреди исполнения: ретраев пула уже не будет, и pending вне
// очереди никто бы не подобрал - матч остаётся running до recovery
func TestProcessor_Process_InfraFailureAfterCancel_StaysRunning(t *testing.T) {
	p, matchRepo, _, programRepo, _, executorMock := newTestProcessor(t)
	match := testProcessorMatch()
	ctx, cancel := context.WithCancel(context.Background())

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	executorMock.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, &executor.InfraError{Err: context.Canceled})

	err := p.Process(ctx, match)
	assert.Error(t, err)
	matchRepo.AssertNotCalled(t, "ResetToPending", mock.Anything, mock.Anything)
}

// updateRatings: ошибка ProcessMatchResult пробрасывается наверх
func TestProcessor_UpdateRatings_ProcessError(t *testing.T) {
	p, _, ratingRepo, _, ratingService, _ := newTestProcessor(t)
	match := testProcessorMatch()
	result := &models.MatchResult{MatchID: match.ID, Winner: 1}

	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(1200, 1000, nil)
	ratingService.On("ProcessMatchResult", mock.Anything, match, 1200, 1000).
		Return(fmt.Errorf("rating error"))

	err := p.updateRatings(context.Background(), match, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process match result")
}

func TestProcessor_Process_Success(t *testing.T) {
	p, matchRepo, ratingRepo, programRepo, ratingService, executor := newTestProcessor(t)
	match := testProcessorMatch()
	result := &models.MatchResult{MatchID: match.ID, Winner: 1, ErrorCode: 0}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").Return(result, nil)
	matchRepo.On("UpdateResultWithOutbox", mock.Anything, match.ID, result).Return(nil)
	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(1200, 1000, nil)
	ratingService.On("ProcessMatchResult", mock.Anything, match, 1200, 1000).Return(nil)
	// fast-path обновил рейтинг: outbox-задача закрывается
	matchRepo.On("MarkRatingApplied", mock.Anything, match.ID).Return(nil)

	err := p.Process(context.Background(), match)
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
	programRepo.AssertExpectations(t)
	executor.AssertExpectations(t)
	ratingRepo.AssertExpectations(t)
	ratingService.AssertExpectations(t)
}

// матч отменили или удалили пока он играл: результат не пишется, рейтинг не трогается
func TestProcessor_Process_NoLongerRunning_DiscardsResult(t *testing.T) {
	p, matchRepo, _, programRepo, ratingService, executor := newTestProcessor(t)
	match := testProcessorMatch()
	result := &models.MatchResult{MatchID: match.ID, Winner: 1}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").Return(result, nil)
	matchRepo.On("UpdateResultWithOutbox", mock.Anything, match.ID, result).Return(models.ErrMatchAlreadyProcessed)

	err := p.Process(context.Background(), match)
	assert.NoError(t, err)
	ratingService.AssertNotCalled(t, "ProcessMatchResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessor_Process_RatingFailureNonFatal(t *testing.T) {
	p, matchRepo, ratingRepo, programRepo, _, executor := newTestProcessor(t)
	match := testProcessorMatch()
	result := &models.MatchResult{MatchID: match.ID, Winner: 1, ErrorCode: 0}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").Return(result, nil)
	matchRepo.On("UpdateResultWithOutbox", mock.Anything, match.ID, result).Return(nil)
	// сбой рейтинга не валит матч: outbox доберёт позже
	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(0, 0, fmt.Errorf("redis connection lost"))

	err := p.Process(context.Background(), match)
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
	programRepo.AssertExpectations(t)
	executor.AssertExpectations(t)
	ratingRepo.AssertExpectations(t)
}

func TestProcessor_Process_ErrorCode_SkipsRatings(t *testing.T) {
	p, matchRepo, ratingRepo, programRepo, ratingService, executor := newTestProcessor(t)
	match := testProcessorMatch()
	result := &models.MatchResult{MatchID: match.ID, Winner: 0, ErrorCode: 1, ErrorMessage: "timeout"}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, models.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(twoPrograms(match), nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").Return(result, nil)
	matchRepo.On("UpdateResultWithOutbox", mock.Anything, match.ID, result).Return(nil)

	err := p.Process(context.Background(), match)
	assert.NoError(t, err)
	matchRepo.AssertExpectations(t)
	programRepo.AssertExpectations(t)
	executor.AssertExpectations(t)
	// при ErrorCode != 0 рейтинг не трогается
	ratingRepo.AssertNotCalled(t, "GetParticipantRatings", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	ratingService.AssertNotCalled(t, "ProcessMatchResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"ErrNotFound", errors.ErrNotFound, true},
		{"AppError_NotFound", errors.ErrNotFound.WithMessage("x"), true},
		{"ErrMatchNotFound", ErrMatchNotFound, true},
		{"not_found_string", fmt.Errorf("resource not found in db"), false},
		{"no_rows_string", fmt.Errorf("sql: no rows in result set"), false},
		{"generic_error", fmt.Errorf("connection refused"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isNotFoundError(tc.err))
		})
	}
}
