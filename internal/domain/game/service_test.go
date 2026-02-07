package game

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGameRepository implements GameRepository
type MockGameRepository struct {
	mock.Mock
}

func (m *MockGameRepository) Create(ctx context.Context, game *domain.Game) error {
	return m.Called(ctx, game).Error(0)
}

func (m *MockGameRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameRepository) GetByName(ctx context.Context, name string) (*domain.Game, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameRepository) List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) Update(ctx context.Context, game *domain.Game) error {
	return m.Called(ctx, game).Error(0)
}

func (m *MockGameRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockGameRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	return m.Called(ctx, tournamentID, gameID).Error(0)
}

func (m *MockGameRepository) RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	return m.Called(ctx, tournamentID, gameID).Error(0)
}

func (m *MockGameRepository) Exists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func newTestGameService(t *testing.T) (*Service, *MockGameRepository) {
	repo := new(MockGameRepository)
	log, _ := logger.New("error", "json")
	return NewService(repo, log), repo
}

// --- Create ---

func TestService_Create_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	repo.On("Exists", ctx, "chess").Return(false, nil)
	repo.On("Create", ctx, mock.AnythingOfType("*domain.Game")).Return(nil)

	g, err := svc.Create(ctx, &CreateRequest{
		Name:        "chess",
		DisplayName: "Chess",
		Rules:       "# Rules",
	})

	require.NoError(t, err)
	assert.Equal(t, "chess", g.Name)
	assert.Equal(t, "Chess", g.DisplayName)
	repo.AssertExpectations(t)
}

func TestService_Create_InvalidNames(t *testing.T) {
	svc, _ := newTestGameService(t)
	ctx := context.Background()

	invalidNames := []struct {
		name string
		val  string
	}{
		{"uppercase", "Chess"},
		{"spaces", "my game"},
		{"hyphen", "my-game"},
		{"empty", ""},
		{"unicode", "игра"},
		{"special chars", "game@1"},
	}

	for _, tc := range invalidNames {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, &CreateRequest{Name: tc.val, DisplayName: "X"})
			assert.Error(t, err)
		})
	}
}

func TestService_Create_ValidNames(t *testing.T) {
	validNames := []string{"chess", "prisoners_dilemma", "game123", "a", "game_1_v2"}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			svc, repo := newTestGameService(t)
			ctx := context.Background()

			repo.On("Exists", ctx, name).Return(false, nil)
			repo.On("Create", ctx, mock.AnythingOfType("*domain.Game")).Return(nil)

			g, err := svc.Create(ctx, &CreateRequest{Name: name, DisplayName: "Display"})
			require.NoError(t, err)
			assert.Equal(t, name, g.Name)
			repo.AssertExpectations(t)
		})
	}
}

func TestService_Create_AlreadyExists(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	repo.On("Exists", ctx, "chess").Return(true, nil)

	_, err := svc.Create(ctx, &CreateRequest{Name: "chess", DisplayName: "Chess"})
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestService_Create_ExistsCheckError(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	repo.On("Exists", ctx, "chess").Return(false, errors.ErrInternal)

	_, err := svc.Create(ctx, &CreateRequest{Name: "chess", DisplayName: "Chess"})
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestService_Create_RepoError(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	repo.On("Exists", ctx, "chess").Return(false, nil)
	repo.On("Create", ctx, mock.AnythingOfType("*domain.Game")).Return(errors.ErrInternal)

	_, err := svc.Create(ctx, &CreateRequest{Name: "chess", DisplayName: "Chess"})
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// --- List ---

func TestService_List_LimitClamping(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero becomes 50", 0, 50},
		{"negative becomes 50", -1, 50},
		{"101 becomes 50", 101, 50},
		{"50 stays 50", 50, 50},
		{"1 stays 1", 1, 1},
		{"100 stays 100", 100, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newTestGameService(t)
			ctx := context.Background()

			repo.On("List", ctx, mock.MatchedBy(func(f domain.GameFilter) bool {
				return f.Limit == tc.expected
			})).Return([]*domain.Game{}, nil)

			_, err := svc.List(ctx, domain.GameFilter{Limit: tc.input})
			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

func TestService_List_RepoError(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	repo.On("List", ctx, mock.Anything).Return(nil, errors.ErrInternal)

	_, err := svc.List(ctx, domain.GameFilter{})
	assert.Error(t, err)
}

// --- Update ---

func TestService_Update_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	existing := &domain.Game{ID: id, Name: "chess", DisplayName: "Old"}
	repo.On("GetByID", ctx, id).Return(existing, nil)
	repo.On("Update", ctx, mock.AnythingOfType("*domain.Game")).Return(nil)

	g, err := svc.Update(ctx, id, &UpdateRequest{DisplayName: "New", Rules: "new rules"})
	require.NoError(t, err)
	assert.Equal(t, "New", g.DisplayName)
	assert.Equal(t, "new rules", g.Rules)
	repo.AssertExpectations(t)
}

func TestService_Update_NotFound(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

	_, err := svc.Update(ctx, id, &UpdateRequest{DisplayName: "X"})
	assert.Error(t, err)
}

func TestService_Update_RepoError(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	existing := &domain.Game{ID: id, Name: "chess"}
	repo.On("GetByID", ctx, id).Return(existing, nil)
	repo.On("Update", ctx, mock.Anything).Return(errors.ErrInternal)

	_, err := svc.Update(ctx, id, &UpdateRequest{DisplayName: "X"})
	assert.Error(t, err)
}

// --- Delete ---

func TestService_Delete_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	repo.On("Delete", ctx, id).Return(nil)

	err := svc.Delete(ctx, id)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_Delete_NotFound(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	repo.On("Delete", ctx, id).Return(errors.ErrNotFound)

	err := svc.Delete(ctx, id)
	assert.Error(t, err)
}

// --- GetByID / GetByName ---

func TestService_GetByID_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	expected := &domain.Game{ID: id, Name: "chess"}
	repo.On("GetByID", ctx, id).Return(expected, nil)

	g, err := svc.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, expected, g)
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

	_, err := svc.GetByID(ctx, id)
	assert.Error(t, err)
}

func TestService_GetByName_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	expected := &domain.Game{Name: "chess"}
	repo.On("GetByName", ctx, "chess").Return(expected, nil)

	g, err := svc.GetByName(ctx, "chess")
	require.NoError(t, err)
	assert.Equal(t, expected, g)
}

func TestService_GetByName_NotFound(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()

	repo.On("GetByName", ctx, "nonexistent").Return(nil, errors.ErrNotFound)

	_, err := svc.GetByName(ctx, "nonexistent")
	assert.Error(t, err)
}

// --- AddToTournament ---

func TestService_AddToTournament_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID, gID := uuid.New(), uuid.New()

	repo.On("GetByID", ctx, gID).Return(&domain.Game{ID: gID}, nil)
	repo.On("AddToTournament", ctx, tID, gID).Return(nil)

	err := svc.AddToTournament(ctx, tID, gID)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_AddToTournament_GameNotFound(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID, gID := uuid.New(), uuid.New()

	repo.On("GetByID", ctx, gID).Return(nil, errors.ErrNotFound)

	err := svc.AddToTournament(ctx, tID, gID)
	assert.Error(t, err)
}

func TestService_AddToTournament_RepoError(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID, gID := uuid.New(), uuid.New()

	repo.On("GetByID", ctx, gID).Return(&domain.Game{ID: gID}, nil)
	repo.On("AddToTournament", ctx, tID, gID).Return(errors.ErrInternal)

	err := svc.AddToTournament(ctx, tID, gID)
	assert.Error(t, err)
}

// --- RemoveFromTournament ---

func TestService_RemoveFromTournament_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID, gID := uuid.New(), uuid.New()

	repo.On("RemoveFromTournament", ctx, tID, gID).Return(nil)

	err := svc.RemoveFromTournament(ctx, tID, gID)
	assert.NoError(t, err)
}

func TestService_RemoveFromTournament_Error(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID, gID := uuid.New(), uuid.New()

	repo.On("RemoveFromTournament", ctx, tID, gID).Return(errors.ErrNotFound)

	err := svc.RemoveFromTournament(ctx, tID, gID)
	assert.Error(t, err)
}

// --- GetByTournamentID ---

func TestService_GetByTournamentID_Success(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID := uuid.New()

	expected := []*domain.Game{{Name: "chess"}}
	repo.On("GetByTournamentID", ctx, tID).Return(expected, nil)

	games, err := svc.GetByTournamentID(ctx, tID)
	require.NoError(t, err)
	assert.Len(t, games, 1)
}

func TestService_GetByTournamentID_Error(t *testing.T) {
	svc, repo := newTestGameService(t)
	ctx := context.Background()
	tID := uuid.New()

	repo.On("GetByTournamentID", ctx, tID).Return(nil, errors.ErrInternal)

	_, err := svc.GetByTournamentID(ctx, tID)
	assert.Error(t, err)
}
