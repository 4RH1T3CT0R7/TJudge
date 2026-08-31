package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockMatchRepository - мок репозитория матчей
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

func (m *MockMatchRepository) List(ctx context.Context, filter models.MatchFilter) ([]*models.Match, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Match), args.Error(1)
}

func (m *MockMatchRepository) GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*storage.MatchStatistics, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.MatchStatistics), args.Error(1)
}

func (m *MockMatchRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.Match, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Match), args.Error(1)
}

// MockMatchCache - мок кэша матчей
type MockMatchCache struct {
	mock.Mock
}

func (m *MockMatchCache) Get(ctx context.Context, matchID uuid.UUID) (*models.MatchResult, error) {
	args := m.Called(ctx, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MatchResult), args.Error(1)
}

func (m *MockMatchCache) Set(ctx context.Context, matchID uuid.UUID, result *models.MatchResult) error {
	args := m.Called(ctx, matchID, result)
	return args.Error(0)
}

func (m *MockMatchCache) GetMatch(ctx context.Context, matchID uuid.UUID) (*models.Match, error) {
	args := m.Called(ctx, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Match), args.Error(1)
}

func (m *MockMatchCache) SetMatch(ctx context.Context, match *models.Match) error {
	args := m.Called(ctx, match)
	return args.Error(0)
}

// MockMatchQueueManager - мок менеджера очереди
type MockMatchQueueManager struct {
	mock.Mock
}

func (m *MockMatchQueueManager) GetStats(ctx context.Context) (*queue.QueueStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*queue.QueueStats), args.Error(1)
}

func (m *MockMatchQueueManager) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMatchQueueManager) PurgeInvalidMatches(ctx context.Context, validator func(matchID string) bool) (int64, error) {
	args := m.Called(ctx, validator)
	return args.Get(0).(int64), args.Error(1)
}

// MockMatchProgramLookup - мок поиска владельца программы
type MockMatchProgramLookup struct {
	mock.Mock
}

func (m *MockMatchProgramLookup) GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Program), args.Error(1)
}

// getWithRouteContext собирает GET-запрос с id в chi-контексте
func getWithRouteContext(matchID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", matchID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestMatchHandler_Get(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("попадание в кэш", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		matchID := uuid.New()
		cachedMatch := &models.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "chess",
			Status:       models.MatchCompleted,
		}

		mockCache.On("GetMatch", mock.Anything, matchID).Return(cachedMatch, nil)

		w := httptest.NewRecorder()
		handler.Get(w, getWithRouteContext(matchID.String()))

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, cachedMatch.ID, response.ID)

		mockCache.AssertExpectations(t)
		// при попадании в кэш репозиторий не трогается
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("промах кэша - чтение из базы", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		matchID := uuid.New()
		dbMatch := &models.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "chess",
			Status:       models.MatchRunning,
		}

		mockCache.On("GetMatch", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(dbMatch, nil)

		w := httptest.NewRecorder()
		handler.Get(w, getWithRouteContext(matchID.String()))

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, dbMatch.ID, response.ID)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("матч не найден", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		matchID := uuid.New()

		mockCache.On("GetMatch", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(nil, errors.ErrNotFound.WithMessage("match not found"))

		w := httptest.NewRecorder()
		handler.Get(w, getWithRouteContext(matchID.String()))

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}

func TestMatchHandler_List(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("список матчей с дефолтной пагинацией", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		expectedMatches := []*models.Match{
			{ID: uuid.New(), TournamentID: uuid.New(), Program1ID: uuid.New(), Program2ID: uuid.New(), GameType: "chess", Status: models.MatchCompleted},
			{ID: uuid.New(), TournamentID: uuid.New(), Program1ID: uuid.New(), Program2ID: uuid.New(), GameType: "chess", Status: models.MatchPending},
		}

		// дефолт: limit=50, offset=0
		mockRepo.On("List", mock.Anything, mock.MatchedBy(func(filter models.MatchFilter) bool {
			return filter.Limit == 50 && filter.Offset == 0
		})).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.Match
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)

		mockRepo.AssertExpectations(t)
	})

	t.Run("фильтр по турниру прокидывается в репозиторий", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		tournamentID := uuid.New()
		expectedMatches := []*models.Match{
			{ID: uuid.New(), TournamentID: tournamentID, Program1ID: uuid.New(), Program2ID: uuid.New(), GameType: "chess", Status: models.MatchCompleted},
		}

		mockRepo.On("List", mock.Anything, mock.MatchedBy(func(filter models.MatchFilter) bool {
			return filter.TournamentID != nil && *filter.TournamentID == tournamentID
		})).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?tournament_id="+tournamentID.String(), nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("битый tournament_id даёт 400", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?tournament_id=invalid", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestMatchHandler_GetStatistics(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("статистика по всем матчам", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		expectedStats := &storage.MatchStatistics{
			Total:     100,
			Completed: 80,
			Running:   15,
			Failed:    5,
			Pending:   0,
		}

		mockRepo.On("GetStatistics", mock.Anything, (*uuid.UUID)(nil)).Return(expectedStats, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/statistics", nil)
		w := httptest.NewRecorder()

		handler.GetStatistics(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response storage.MatchStatistics
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, 100, response.Total)
		assert.Equal(t, 80, response.Completed)

		mockRepo.AssertExpectations(t)
	})

	t.Run("битый tournament_id даёт 400", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/statistics?tournament_id=invalid", nil)
		w := httptest.NewRecorder()

		handler.GetStatistics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestMatchHandler_GetQueueStats(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("успех", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandler(mockRepo, mockCache, nil, mockQueue, log)

		expectedStats := &queue.QueueStats{
			High:   5,
			Medium: 10,
			Low:    3,
			Total:  18,
		}

		mockQueue.On("GetStats", mock.Anything).Return(expectedStats, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/queue/stats", nil)
		w := httptest.NewRecorder()

		handler.GetQueueStats(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response queue.QueueStats
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, int64(5), response.High)
		assert.Equal(t, int64(10), response.Medium)
		assert.Equal(t, int64(3), response.Low)
		assert.Equal(t, int64(18), response.Total)

		mockQueue.AssertExpectations(t)
	})

	t.Run("без менеджера очереди - 500", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		// queueManager == nil, хендлер должен деградировать в 500
		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/queue/stats", nil)
		w := httptest.NewRecorder()

		handler.GetQueueStats(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestMatchHandler_ClearQueue(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("успех", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandler(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("Clear", mock.Anything).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/clear", nil)
		w := httptest.NewRecorder()

		handler.ClearQueue(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "All queues cleared successfully", response["message"])

		mockQueue.AssertExpectations(t)
	})

	t.Run("без менеджера очереди - 500", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/clear", nil)
		w := httptest.NewRecorder()

		handler.ClearQueue(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestMatchHandler_PurgeInvalidMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("успех", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandler(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("PurgeInvalidMatches", mock.Anything, mock.AnythingOfType("func(string) bool")).Return(int64(7), nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/purge", nil)
		w := httptest.NewRecorder()

		handler.PurgeInvalidMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "Invalid matches purged successfully", response["message"])
		assert.Equal(t, float64(7), response["purged_count"])

		mockQueue.AssertExpectations(t)
	})

	t.Run("без менеджера очереди - 500", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		handler := NewMatchHandler(mockRepo, mockCache, nil, nil, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/purge", nil)
		w := httptest.NewRecorder()

		handler.PurgeInvalidMatches(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestMatchHandler_ErrorFiltering(t *testing.T) {
	log, _ := logger.New("error", "json")

	// getFailedMatch собирает упавший матч с заданным победителем и текстом ошибки
	getFailedMatch := func(matchID, program1ID, program2ID uuid.UUID, winner int, errorMsg string) *models.Match {
		return &models.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			GameType:     "prisoners_dilemma",
			Status:       models.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}
	}

	t.Run("админ видит полный текст ошибки", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandler(mockRepo, mockCache, mockProgramLookup, nil, log)

		matchID := uuid.New()
		errorMsg := "runtime error: index out of bounds at line 42"
		match := getFailedMatch(matchID, uuid.New(), uuid.New(), 1, errorMsg)

		mockCache.On("GetMatch", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		req := getWithRouteContext(matchID.String())
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
		ctx = context.WithValue(ctx, middleware.RoleKey, models.RoleAdmin)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, errorMsg, *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("владелец упавшей программы видит свою ошибку", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandler(mockRepo, mockCache, mockProgramLookup, nil, log)

		matchID := uuid.New()
		ownerID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		errorMsg := "segfault in user code at line 15"

		// winner=1 значит победил program1, упал program2
		match := getFailedMatch(matchID, program1ID, program2ID, 1, errorMsg)
		failedProgram := &models.Program{ID: program2ID, UserID: ownerID, Name: "my-bot"}

		mockCache.On("GetMatch", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)
		mockProgramLookup.On("GetByID", mock.Anything, program2ID).Return(failedProgram, nil)

		req := getWithRouteContext(matchID.String())
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, ownerID)
		ctx = context.WithValue(ctx, middleware.RoleKey, models.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, errorMsg, *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockProgramLookup.AssertExpectations(t)
	})

	t.Run("чужой пользователь видит обезличенное сообщение", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandler(mockRepo, mockCache, mockProgramLookup, nil, log)

		matchID := uuid.New()
		programOwnerID := uuid.New()
		otherUserID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		errorMsg := "segfault in user code at line 15"

		// упала program2, но запрашивает не её владелец
		match := getFailedMatch(matchID, program1ID, program2ID, 1, errorMsg)
		failedProgram := &models.Program{ID: program2ID, UserID: programOwnerID, Name: "opponent-bot"}

		mockCache.On("GetMatch", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)
		mockProgramLookup.On("GetByID", mock.Anything, program2ID).Return(failedProgram, nil)

		req := getWithRouteContext(matchID.String())
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, otherUserID)
		ctx = context.WithValue(ctx, middleware.RoleKey, models.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, "Программа оппонента завершилась с ошибкой", *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockProgramLookup.AssertExpectations(t)
	})

	t.Run("без победителя ошибка скрыта", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandler(mockRepo, mockCache, mockProgramLookup, nil, log)

		matchID := uuid.New()
		userID := uuid.New()
		errorMsg := "both programs crashed"

		match := &models.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       models.MatchFailed,
			Winner:       nil, // без winner упавшую программу определить нельзя
			ErrorMessage: &errorMsg,
		}

		mockCache.On("GetMatch", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		req := getWithRouteContext(matchID.String())
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, models.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Match
		decodeJSONData(t, w.Body, &response)
		require.NotNil(t, response.ErrorMessage)
		// упавшую программу не определить, поэтому текст скрыт
		assert.Equal(t, "Ошибка выполнения матча", *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}
