package worker

import (
	"context"
	"fmt"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMatchRepository is a mock for MatchRepository
type MockMatchRepository struct {
	mock.Mock
}

func (m *MockMatchRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockMatchRepository) UpdateResult(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

// MockProcessorRatingRepository is a mock for RatingRepository in processor
type MockProcessorRatingRepository struct {
	mock.Mock
}

func (m *MockProcessorRatingRepository) GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (int, int, error) {
	args := m.Called(ctx, tournamentID, program1ID, program2ID)
	return args.Int(0), args.Int(1), args.Error(2)
}

// MockProcessorRatingService is a mock for RatingService
type MockProcessorRatingService struct {
	mock.Mock
}

func (m *MockProcessorRatingService) ProcessMatchResult(ctx context.Context, match *domain.Match, rating1, rating2 int) error {
	args := m.Called(ctx, match, rating1, rating2)
	return args.Error(0)
}

// MockExecutor is a mock for Executor
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, match *domain.Match, program1Path, program2Path string) (*domain.MatchResult, error) {
	args := m.Called(ctx, match, program1Path, program2Path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MatchResult), args.Error(1)
}

// MockProcessorProgramRepository is a mock for ProgramRepository
type MockProcessorProgramRepository struct {
	mock.Mock
}

func (m *MockProcessorProgramRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Program), args.Error(1)
}

func (m *MockProcessorProgramRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Program, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Program), args.Error(1)
}

func newTestProcessor(t *testing.T) (*Processor, *MockMatchRepository, *MockProcessorRatingRepository, *MockProcessorProgramRepository, *MockProcessorRatingService, *MockExecutor) {
	t.Helper()
	matchRepo := new(MockMatchRepository)
	ratingRepo := new(MockProcessorRatingRepository)
	programRepo := new(MockProcessorProgramRepository)
	ratingService := new(MockProcessorRatingService)
	executor := new(MockExecutor)
	log, _ := logger.New("error", "json")

	p := &Processor{
		matchRepo:     matchRepo,
		ratingRepo:    ratingRepo,
		programRepo:   programRepo,
		ratingService: ratingService,
		executor:      executor,
		matchCache:    nil, // nil cache - tests should not reach cache calls
		log:           log,
	}

	return p, matchRepo, ratingRepo, programRepo, ratingService, executor
}

func TestProcessor_Process_UpdateStatusNotFound(t *testing.T) {
	p, matchRepo, _, _, _, _ := newTestProcessor(t)
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, domain.MatchRunning).
		Return(errors.ErrNotFound)

	err := p.Process(context.Background(), match)
	assert.ErrorIs(t, err, ErrMatchNotFound)
	matchRepo.AssertExpectations(t)
}

func TestProcessor_Process_UpdateStatusFatalError(t *testing.T) {
	p, matchRepo, _, _, _, _ := newTestProcessor(t)
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, domain.MatchRunning).
		Return(fmt.Errorf("connection refused"))

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update match status")
	matchRepo.AssertExpectations(t)
}

func TestProcessor_Process_GetProgramsError(t *testing.T) {
	p, matchRepo, _, programRepo, _, _ := newTestProcessor(t)
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, domain.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return(nil, errors.ErrNotFound)

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get programs")
	matchRepo.AssertExpectations(t)
	programRepo.AssertExpectations(t)
}

func TestProcessor_Process_Program2NotInResults(t *testing.T) {
	p, matchRepo, _, programRepo, _, _ := newTestProcessor(t)
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}

	// GetByIDs returns only program1 — program2 is missing
	matchRepo.On("UpdateStatus", mock.Anything, match.ID, domain.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return([]*domain.Program{{ID: match.Program1ID, CodePath: "/path/p1"}}, nil)

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "program2")
	assert.Contains(t, err.Error(), "not found")
	matchRepo.AssertExpectations(t)
	programRepo.AssertExpectations(t)
}

func TestProcessor_Process_ExecutorFailure(t *testing.T) {
	p, matchRepo, _, programRepo, _, executor := newTestProcessor(t)
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, domain.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return([]*domain.Program{
			{ID: match.Program1ID, CodePath: "/path/p1"},
			{ID: match.Program2ID, CodePath: "/path/p2"},
		}, nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").
		Return(nil, fmt.Errorf("docker timeout"))
	matchRepo.On("UpdateResult", mock.Anything, match.ID, mock.AnythingOfType("*domain.MatchResult")).Return(nil)

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute match")
	matchRepo.AssertExpectations(t)
	executor.AssertExpectations(t)
}

func TestProcessor_Process_UpdateResultFailure(t *testing.T) {
	p, matchRepo, _, programRepo, _, executor := newTestProcessor(t)
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}

	result := &domain.MatchResult{
		MatchID: match.ID,
		Winner:  1,
	}

	matchRepo.On("UpdateStatus", mock.Anything, match.ID, domain.MatchRunning).Return(nil)
	programRepo.On("GetByIDs", mock.Anything, []uuid.UUID{match.Program1ID, match.Program2ID}).
		Return([]*domain.Program{
			{ID: match.Program1ID, CodePath: "/path/p1"},
			{ID: match.Program2ID, CodePath: "/path/p2"},
		}, nil)
	executor.On("Execute", mock.Anything, match, "/path/p1", "/path/p2").
		Return(result, nil)
	matchRepo.On("UpdateResult", mock.Anything, match.ID, result).
		Return(fmt.Errorf("db error"))

	err := p.Process(context.Background(), match)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update match result")
	matchRepo.AssertExpectations(t)
}

// --- updateRatings ---

func TestProcessor_UpdateRatings_Success(t *testing.T) {
	p, _, ratingRepo, _, ratingService, _ := newTestProcessor(t)

	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}
	result := &domain.MatchResult{
		MatchID: match.ID,
		Winner:  1,
	}

	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(1200, 1000, nil)
	ratingService.On("ProcessMatchResult", mock.Anything, match, 1200, 1000).Return(nil)

	err := p.updateRatings(context.Background(), match, result)
	assert.NoError(t, err)
	ratingRepo.AssertExpectations(t)
	ratingService.AssertExpectations(t)
}

func TestProcessor_UpdateRatings_GetRatingsError(t *testing.T) {
	p, _, ratingRepo, _, _, _ := newTestProcessor(t)

	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}
	result := &domain.MatchResult{MatchID: match.ID, Winner: 1}

	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(0, 0, fmt.Errorf("db error"))

	err := p.updateRatings(context.Background(), match, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get participant ratings")
}

func TestProcessor_UpdateRatings_ProcessError(t *testing.T) {
	p, _, ratingRepo, _, ratingService, _ := newTestProcessor(t)

	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
	}
	result := &domain.MatchResult{MatchID: match.ID, Winner: 1}

	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(1200, 1000, nil)
	ratingService.On("ProcessMatchResult", mock.Anything, match, 1200, 1000).
		Return(fmt.Errorf("rating error"))

	err := p.updateRatings(context.Background(), match, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process match result")
}

// --- isNotFoundError ---

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
			result := isNotFoundError(tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
