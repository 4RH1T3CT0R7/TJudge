package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/tournament"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTournamentService - мок сервиса турниров.
// все методы интерфейса присутствуют целиком, иначе не скомпилится
type MockTournamentService struct {
	mock.Mock
}

func (m *MockTournamentService) Create(ctx context.Context, req *tournament.CreateRequest) (*models.Tournament, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Tournament), args.Error(1)
}

func (m *MockTournamentService) GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Tournament), args.Error(1)
}

func (m *MockTournamentService) List(ctx context.Context, filter models.TournamentFilter) ([]*models.Tournament, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Tournament), args.Error(1)
}

func (m *MockTournamentService) Join(ctx context.Context, req *tournament.JoinRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockTournamentService) Start(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *MockTournamentService) Complete(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *MockTournamentService) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.LeaderboardEntry), args.Error(1)
}

func (m *MockTournamentService) CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority models.MatchPriority) (*models.Match, error) {
	args := m.Called(ctx, tournamentID, program1ID, program2ID, priority)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Match), args.Error(1)
}

func (m *MockTournamentService) GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*models.Match, error) {
	args := m.Called(ctx, tournamentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Match), args.Error(1)
}

func (m *MockTournamentService) Delete(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *MockTournamentService) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.CrossGameLeaderboardEntry), args.Error(1)
}

func (m *MockTournamentService) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*models.MatchRound, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MatchRound), args.Error(1)
}

// MockSchedulingService - мок планировщика матчей.
type MockSchedulingService struct {
	mock.Mock
}

func (m *MockSchedulingService) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockSchedulingService) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockSchedulingService) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	args := m.Called(ctx, tournamentID, gameType)
	return args.Int(0), args.Error(1)
}

// withTournamentID кладёт id турнира в chi-контекст запроса.
func withTournamentID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestTournamentHandler_Create(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully create tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		maxParticipants := 10
		reqBody := tournament.CreateRequest{
			Name:            "Test Tournament",
			GameType:        "chess",
			MaxParticipants: &maxParticipants,
		}

		expectedTournament := &models.Tournament{
			ID:              uuid.New(),
			Name:            reqBody.Name,
			GameType:        reqBody.GameType,
			Status:          models.TournamentPending,
			MaxParticipants: &maxParticipants,
		}

		mockService.On("Create", mock.Anything, &reqBody).Return(expectedTournament, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response models.Tournament
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedTournament.ID, response.ID)
		assert.Equal(t, expectedTournament.Name, response.Name)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Get(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedTournament := &models.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   models.TournamentActive,
		}

		mockService.On("GetByID", mock.Anything, tournamentID).Return(expectedTournament, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String(), nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Tournament
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedTournament.ID, response.ID)

		mockService.AssertExpectations(t)
	})

	// единственный представитель кейса «битый uuid» - путь один и тот же
	// (parseUUIDParam) во всех ручках, поэтому он не размазывается по каждой
	t.Run("invalid UUID format", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/invalid-uuid", nil)
		req = withTournamentID(req, "invalid-uuid")
		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_List(t *testing.T) {
	log, _ := logger.New("error", "json")

	// проверяется и happy-path, и что status/game_type из query доезжают до фильтра
	t.Run("list with filters", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		expectedTournaments := []*models.Tournament{
			{
				ID:       uuid.New(),
				Name:     "Tournament 1",
				GameType: "chess",
				Status:   models.TournamentActive,
			},
		}

		mockService.On("List", mock.Anything, mock.MatchedBy(func(filter models.TournamentFilter) bool {
			return filter.Status == models.TournamentActive && filter.GameType == "chess"
		})).Return(expectedTournaments, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments?status=active&game_type=chess", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.Tournament
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 1)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Join(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully join tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		reqBody := tournament.JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		mockService.On("Join", mock.Anything, &reqBody).Return(nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/join", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Join(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	// уже стартовавший турнир не пускает новых - ожидается 409
	t.Run("tournament already started", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		reqBody := tournament.JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		mockService.On("Join", mock.Anything, &reqBody).Return(errors.ErrTournamentStarted)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/join", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Join(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Start(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully start tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Start", mock.Anything, tournamentID).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/start", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Start(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Complete(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully complete tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Complete", mock.Anything, tournamentID).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/complete", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Complete(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "completed", response["status"])

		mockService.AssertExpectations(t)
	})

	// завершать можно только активный турнир
	t.Run("tournament not active", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Complete", mock.Anything, tournamentID).Return(errors.ErrConflict.WithMessage("tournament is not active"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/complete", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Complete(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})
}

// удаление турнира - админская операция
func TestTournamentHandler_Delete(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully delete tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Delete", mock.Anything, tournamentID).Return(nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tournaments/"+tournamentID.String(), nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())

		mockService.AssertExpectations(t)
	})

}

func TestTournamentHandler_GetLeaderboard(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get leaderboard", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedLeaderboard := []*models.LeaderboardEntry{
			{ProgramID: uuid.New(), Rating: 1800, Wins: 10, Losses: 2, Draws: 1},
			{ProgramID: uuid.New(), Rating: 1700, Wins: 8, Losses: 4, Draws: 1},
		}

		// дефолтный лимит лидерборда - 100
		mockService.On("GetLeaderboard", mock.Anything, tournamentID, 100).Return(expectedLeaderboard, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/leaderboard", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.GetLeaderboard(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.LeaderboardEntry
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, 1800, response[0].Rating)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_GetCrossGameLeaderboard(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get cross-game leaderboard", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedEntries := []*models.CrossGameLeaderboardEntry{
			{
				Rank:        1,
				TeamName:    "Team Alpha",
				ProgramID:   uuid.New(),
				ProgramName: "AlphaBot",
				TotalRating: 3600,
				TotalWins:   20,
				TotalLosses: 5,
				TotalGames:  25,
			},
			{
				Rank:        2,
				TeamName:    "Team Beta",
				ProgramID:   uuid.New(),
				ProgramName: "BetaBot",
				TotalRating: 3200,
				TotalWins:   15,
				TotalLosses: 10,
				TotalGames:  25,
			},
		}

		mockService.On("GetCrossGameLeaderboard", mock.Anything, tournamentID).Return(expectedEntries, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/cross-game-leaderboard", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.GetCrossGameLeaderboard(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.CrossGameLeaderboardEntry
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, 3600, response[0].TotalRating)
		assert.Equal(t, "Team Alpha", response[0].TeamName)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_RunAllMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	// прогон всего пула round-robin: планировщик возвращает число матчей в очереди.
	// на N участниках это N*(N-1) матчей на игру - число enqueued тут и проверяется
	t.Run("successfully run all matches", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RunAllMatches", mock.Anything, tournamentID).Return(15, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/run-matches", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.RunAllMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "started", response["status"])
		assert.Equal(t, float64(15), response["enqueued"])

		mockScheduling.AssertExpectations(t)
	})
}

func TestTournamentHandler_RunGameMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully run game matches", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RunGameMatches", mock.Anything, tournamentID, "prisoners_dilemma").Return(8, nil)

		body, _ := json.Marshal(map[string]string{"game_type": "prisoners_dilemma"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/games/run-matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.RunGameMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "started", response["status"])
		assert.Equal(t, "prisoners_dilemma", response["game_type"])
		assert.Equal(t, float64(8), response["enqueued"])

		mockScheduling.AssertExpectations(t)
	})

	// без game_type планировщику нечего раскладывать - 400
	t.Run("empty game_type", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		body, _ := json.Marshal(map[string]string{"game_type": ""})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/games/run-matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.RunGameMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_RetryFailedMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully retry failed matches", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RetryFailedMatches", mock.Anything, tournamentID).Return(3, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/retry-matches", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.RetryFailedMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "retried", response["status"])
		assert.Equal(t, float64(3), response["enqueued"])

		mockScheduling.AssertExpectations(t)
	})
}

func TestTournamentHandler_GetMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get matches with defaults", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedMatches := []*models.Match{
			{
				ID:           uuid.New(),
				TournamentID: tournamentID,
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "prisoners_dilemma",
				Status:       models.MatchCompleted,
			},
			{
				ID:           uuid.New(),
				TournamentID: tournamentID,
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "prisoners_dilemma",
				Status:       models.MatchPending,
			},
		}

		// дефолт: limit=50, offset=0
		mockService.On("GetMatches", mock.Anything, tournamentID, 50, 0).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/matches", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.GetMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.Match
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_GetMatchesByRounds(t *testing.T) {
	log, _ := logger.New("error", "json")

	// матчи, разложенные по раундам round-robin, со счётчиками статусов
	t.Run("successfully get matches by rounds", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedRounds := []*models.MatchRound{
			{
				RoundNumber:    1,
				GameType:       "prisoners_dilemma",
				TotalMatches:   6,
				CompletedCount: 6,
				Matches: []*models.Match{
					{ID: uuid.New(), TournamentID: tournamentID, RoundNumber: 1, Status: models.MatchCompleted},
				},
			},
			{
				RoundNumber:    2,
				GameType:       "prisoners_dilemma",
				TotalMatches:   6,
				CompletedCount: 3,
				PendingCount:   2,
				RunningCount:   1,
				Matches: []*models.Match{
					{ID: uuid.New(), TournamentID: tournamentID, RoundNumber: 2, Status: models.MatchRunning},
				},
			},
		}

		mockService.On("GetMatchesByRounds", mock.Anything, tournamentID).Return(expectedRounds, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/matches/rounds", nil)
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.GetMatchesByRounds(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.MatchRound
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, 1, response[0].RoundNumber)
		assert.Equal(t, 6, response[0].TotalMatches)
		assert.Equal(t, 2, response[1].RoundNumber)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_CreateMatch(t *testing.T) {
	log, _ := logger.New("error", "json")

	// приоритет не прислали - хендлер должен подставить medium
	t.Run("success with default priority", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()

		expectedMatch := &models.Match{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			Priority:     models.PriorityMedium,
		}

		mockService.On("CreateMatch", mock.Anything, tournamentID, program1ID, program2ID, models.PriorityMedium).Return(expectedMatch, nil)

		body, _ := json.Marshal(map[string]any{
			"program1_id": program1ID,
			"program2_id": program2ID,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req = withTournamentID(req, tournamentID.String())
		w := httptest.NewRecorder()

		handler.CreateMatch(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, models.PriorityMedium, response.Priority)

		mockService.AssertExpectations(t)
	})
}

// --- история рейтинга (переехало из rating_history_test.go) ---

type MockRatingHistoryRepository struct {
	mock.Mock
}

func (m *MockRatingHistoryRepository) GetByProgramAndTournament(ctx context.Context, programID, tournamentID uuid.UUID, limit int) ([]*models.RatingHistory, error) {
	args := m.Called(ctx, programID, tournamentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.RatingHistory), args.Error(1)
}

func newTestRatingHistoryHandler(t *testing.T) (*RatingHistoryHandler, *MockRatingHistoryRepository) {
	t.Helper()
	repo := new(MockRatingHistoryRepository)
	log, _ := logger.New("error", "json")
	return NewRatingHistoryHandler(repo, log), repo
}

// ratingHistoryRequest собирает запрос с двумя path-параметрами: id и programId.
func ratingHistoryRequest(tournamentID, programID string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID)
	rctx.URLParams.Add("programId", programID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestRatingHistoryHandler_Success(t *testing.T) {
	handler, repo := newTestRatingHistoryHandler(t)
	tournamentID := uuid.New()
	programID := uuid.New()

	repo.On("GetByProgramAndTournament", mock.Anything, programID, tournamentID, 200).
		Return([]*models.RatingHistory{
			{ProgramID: programID, TournamentID: tournamentID, OldRating: 1500, NewRating: 1516, Change: 16},
			{ProgramID: programID, TournamentID: tournamentID, OldRating: 1516, NewRating: 1508, Change: -8},
		}, nil)

	rr := httptest.NewRecorder()
	handler.GetProgramRatingHistory(rr, ratingHistoryRequest(tournamentID.String(), programID.String()))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []*models.RatingHistory
	decodeJSONData(t, rr.Body, &result)
	assert.Len(t, result, 2)
	assert.Equal(t, 16, result[0].Change)
	repo.AssertExpectations(t)
}

// --- GetHeadToHead (живёт в GameRoundHandler, мок расширен в game_test.go) ---

func TestGameHandler_GetHeadToHead_Success(t *testing.T) {
	handler, svc, leaderboardRepo, _, _, _ := newGameHandlerWithAllRepos(t)
	tournamentID := uuid.New()
	gameID := uuid.New()
	teamA, teamB := uuid.New(), uuid.New()

	svc.On("GetByID", mock.Anything, gameID).
		Return(&models.Game{ID: gameID, Name: "dilemma"}, nil)
	leaderboardRepo.On("GetHeadToHead", mock.Anything, tournamentID, "dilemma").
		Return([]*models.HeadToHeadCell{
			{TeamID: teamA, TeamName: "alpha", OpponentID: teamB, OpponentName: "beta", Wins: 2, Losses: 1, Draws: 1},
		}, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetHeadToHead(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []*models.HeadToHeadCell
	decodeJSONData(t, rr.Body, &result)
	assert.Len(t, result, 1)
	assert.Equal(t, "alpha", result[0].TeamName)
	assert.Equal(t, 2, result[0].Wins)
	svc.AssertExpectations(t)
	leaderboardRepo.AssertExpectations(t)
}
