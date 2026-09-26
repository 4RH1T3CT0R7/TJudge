package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProgramRepository — мок program-репозитория
type MockProgramRepository struct {
	mock.Mock
}

func (m *MockProgramRepository) Create(ctx context.Context, program *models.Program) error {
	args := m.Called(ctx, program)
	return args.Error(0)
}

func (m *MockProgramRepository) CreateWithAtomicVersion(ctx context.Context, program *models.Program) error {
	args := m.Called(ctx, program)
	return args.Error(0)
}

func (m *MockProgramRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Program), args.Error(1)
}

func (m *MockProgramRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Program, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Program), args.Error(1)
}

func (m *MockProgramRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProgramRepository) GetLatestVersion(ctx context.Context, teamID, gameID uuid.UUID) (int, error) {
	args := m.Called(ctx, teamID, gameID)
	return args.Int(0), args.Error(1)
}

func (m *MockProgramRepository) GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*models.Program, error) {
	args := m.Called(ctx, teamID, gameID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Program), args.Error(1)
}

func (m *MockProgramRepository) ClearErrorMessages(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	args := m.Called(ctx, tournamentID)
	return args.Get(0).(int64), args.Error(1)
}

// MockProgramTournamentRepo — мок TournamentRepo хендлера программ
type MockProgramTournamentRepo struct {
	mock.Mock
}

func (m *MockProgramTournamentRepo) AddParticipant(ctx context.Context, participant *models.TournamentParticipant) error {
	return m.Called(ctx, participant).Error(0)
}

func (m *MockProgramTournamentRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Tournament), args.Error(1)
}

// MockTeamMembershipChecker — мок чекера членства в команде
type MockTeamMembershipChecker struct {
	mock.Mock
}

func (m *MockTeamMembershipChecker) IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTeamMembershipChecker) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

// MockRoundCompletionChecker — мок чекера завершения раунда
type MockRoundCompletionChecker struct {
	mock.Mock
}

func (m *MockRoundCompletionChecker) IsRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error) {
	args := m.Called(ctx, tournamentID, gameID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRoundCompletionChecker) IsAutoRoundEnabled(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error) {
	args := m.Called(ctx, tournamentID, gameID)
	return args.Bool(0), args.Error(1)
}

// MockMatchExistenceChecker — мок чекера существования матчей
type MockMatchExistenceChecker struct {
	mock.Mock
}

func (m *MockMatchExistenceChecker) HasAnyRunningMatches(ctx context.Context, tournamentID uuid.UUID) (bool, error) {
	args := m.Called(ctx, tournamentID)
	return args.Bool(0), args.Error(1)
}

func (m *MockMatchExistenceChecker) GetActiveGameType(ctx context.Context, tournamentID uuid.UUID) (string, error) {
	args := m.Called(ctx, tournamentID)
	return args.String(0), args.Error(1)
}

// createMultipartRequest собирает multipart/form-data запрос из полей и
// опционального файла
func createMultipartRequest(t *testing.T, fields map[string]string, fileName string, fileContent []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if fileName != "" && fileContent != nil {
		part, err := writer.CreateFormFile("file", fileName)
		require.NoError(t, err)
		_, err = part.Write(fileContent)
		require.NoError(t, err)
	}

	for k, v := range fields {
		err := writer.WriteField(k, v)
		require.NoError(t, err)
	}

	err := writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestProgramHandler_Create(t *testing.T) {
	log, _ := logger.New("error", "json")

	// путь к коду задаёт только сервер: JSON с code_path больше не создаёт программу
	t.Run("json with code_path rejected", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		body, _ := json.Marshal(map[string]string{
			"name":      "stolen",
			"code_path": "/data/programs/aaaaaaaa_bbbbbbbb_cccccccc.py",
			"language":  "python",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockRepo.AssertNotCalled(t, "CreateWithAtomicVersion", mock.Anything, mock.Anything)
	})

	t.Run("missing user ID in context", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		reqBody := map[string]string{
			"name":      "My Chess AI",
			"game_type": "chess",
			"code_path": "/data/programs/chess/ai.py",
			"language":  "python",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestProgramHandler_List(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully list programs", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		expectedPrograms := []*models.Program{
			{
				ID:       uuid.New(),
				UserID:   userID,
				Name:     "Chess AI",
				GameType: "chess",
				Language: "python",
			},
			{
				ID:       uuid.New(),
				UserID:   userID,
				Name:     "Tic-Tac-Toe AI",
				GameType: "tictactoe",
				Language: "javascript",
			},
		}

		mockRepo.On("GetByUserID", mock.Anything, userID).Return(expectedPrograms, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs", nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*models.Program
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, expectedPrograms[0].Name, response[0].Name)

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_Get(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get program as owner", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		programID := uuid.New()
		expectedProgram := &models.Program{
			ID:       programID,
			UserID:   userID,
			Name:     "Chess AI",
			GameType: "chess",
			Language: "python",
		}

		mockRepo.On("GetByID", mock.Anything, programID).Return(expectedProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Program
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedProgram.ID, response.ID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("successfully get program as admin", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		programID := uuid.New()
		expectedProgram := &models.Program{
			ID:       programID,
			UserID:   uuid.New(), // другой пользователь
			Name:     "Chess AI",
			GameType: "chess",
			Language: "python",
		}

		mockRepo.On("GetByID", mock.Anything, programID).Return(expectedProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, models.RoleAdmin)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("forbidden when not owner", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		programID := uuid.New()
		otherProgram := &models.Program{
			ID:     programID,
			UserID: uuid.New(), // другой пользователь
		}

		mockRepo.On("GetByID", mock.Anything, programID).Return(otherProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_Delete(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully delete program", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		programID := uuid.New()

		program := &models.Program{
			ID:       programID,
			Name:     "test-program",
			UserID:   userID,
			FilePath: nil,
		}

		mockRepo.On("GetByID", mock.Anything, programID).Return(program, nil)
		mockRepo.On("Delete", mock.Anything, programID).Return(nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/programs/"+programID.String(), nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		mockRepo.AssertExpectations(t)
	})

	// каскад снёс бы матчи и очки соперников, итоги поменялись бы задним числом
	for _, status := range []models.TournamentStatus{models.TournamentActive, models.TournamentCompleted} {
		t.Run("conflict in "+string(status)+" tournament", func(t *testing.T) {
			mockRepo := new(MockProgramRepository)
			tournamentRepo := new(MockProgramTournamentRepo)
			handler := NewProgramHandler(mockRepo, tournamentRepo, nil, nil, nil, nil, nil, "", log)

			userID, programID, tournamentID := uuid.New(), uuid.New(), uuid.New()
			mockRepo.On("GetByID", mock.Anything, programID).Return(&models.Program{ID: programID, UserID: userID, TournamentID: &tournamentID}, nil)
			tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(&models.Tournament{ID: tournamentID, Status: status}, nil)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/programs/"+programID.String(), nil)
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", programID.String())
			req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.Delete(w, req)

			assert.Equal(t, http.StatusConflict, w.Code)
			mockRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
		})
	}
}

// версии команды видит любой её член, а не только тот, кто их загружал
func TestProgramHandler_GetVersions(t *testing.T) {
	log, _ := logger.New("error", "json")

	for name, isMember := range map[string]bool{"teammate": true, "outsider": false} {
		t.Run(name, func(t *testing.T) {
			mockRepo := new(MockProgramRepository)
			teamChecker := new(MockTeamMembershipChecker)
			handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, teamChecker, nil, "", log)

			userID, teamID, gameID := uuid.New(), uuid.New(), uuid.New()
			programs := []*models.Program{{ID: uuid.New(), UserID: uuid.New(), Name: "v1", Version: 1}}

			teamChecker.On("IsUserInTeam", mock.Anything, teamID, userID).Return(isMember, nil)
			mockRepo.On("GetAllVersionsByTeamAndGame", mock.Anything, teamID, gameID).Return(programs, nil).Maybe()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/versions?team_id="+teamID.String()+"&game_id="+gameID.String(), nil)
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))

			w := httptest.NewRecorder()
			handler.GetVersions(w, req)

			if !isMember {
				assert.Equal(t, http.StatusForbidden, w.Code)
				mockRepo.AssertNotCalled(t, "GetAllVersionsByTeamAndGame", mock.Anything, mock.Anything, mock.Anything)
				return
			}
			assert.Equal(t, http.StatusOK, w.Code)
			var response []*models.Program
			decodeJSONData(t, w.Body, &response)
			assert.Len(t, response, 1)
		})
	}
}

func TestProgramHandler_ClearProgramErrors(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		tournamentID := uuid.New()

		mockRepo.On("ClearErrorMessages", mock.Anything, tournamentID).Return(int64(5), nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/programs/clear-errors", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.ClearProgramErrors(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, float64(5), response["cleared"])
		assert.Contains(t, response["message"], "5")

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_Download(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("not owner", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		programID := uuid.New()

		mockRepo.On("GetByID", mock.Anything, programID).Return(&models.Program{ID: programID, UserID: uuid.New()}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String()+"/download", nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Download(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("file path nil", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, nil, nil, nil, nil, "", log)

		userID := uuid.New()
		programID := uuid.New()

		program := &models.Program{
			ID:       programID,
			UserID:   userID,
			Name:     "test-program",
			FilePath: nil,
		}

		mockRepo.On("GetByID", mock.Anything, programID).Return(program, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String()+"/download", nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Download(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_FileUpload(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("missing required fields", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := &ProgramHandler{
			programRepo: mockRepo,
			uploadDir:   t.TempDir(),
			maxFileSize: 10 * 1024 * 1024,
			log:         log,
		}

		userID := uuid.New()

		// форма с файлом, но без team_id/tournament_id/game_id
		req := createMultipartRequest(t, map[string]string{
			"name": "My Strategy",
		}, "strategy.py", []byte("print('hello')"))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("file too large", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		// крошечный лимит: multipart-парсер упрётся в MaxBytesReader и вернёт 400
		handler := &ProgramHandler{
			programRepo: mockRepo,
			uploadDir:   t.TempDir(),
			maxFileSize: 16,
			log:         log,
		}

		userID := uuid.New()
		teamID := uuid.New()
		tournamentID := uuid.New()
		gameID := uuid.New()

		req := createMultipartRequest(t, map[string]string{
			"team_id":       teamID.String(),
			"tournament_id": tournamentID.String(),
			"game_id":       gameID.String(),
			"name":          "My Strategy",
		}, "strategy.py", bytes.Repeat([]byte("A"), 5000))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non_member_blocked", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		mockTeamChecker := new(MockTeamMembershipChecker)
		handler := &ProgramHandler{
			programRepo: mockRepo,
			teamChecker: mockTeamChecker,
			uploadDir:   t.TempDir(),
			maxFileSize: 10 * 1024 * 1024,
			log:         log,
		}

		userID := uuid.New()
		teamID := uuid.New()
		tournamentID := uuid.New()
		gameID := uuid.New()

		mockTeamChecker.On("IsUserInTeam", mock.Anything, teamID, userID).Return(false, nil)

		req := createMultipartRequest(t, map[string]string{
			"team_id":       teamID.String(),
			"tournament_id": tournamentID.String(),
			"game_id":       gameID.String(),
			"name":          "My Strategy",
		}, "strategy.py", []byte("print('hello')"))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockTeamChecker.AssertExpectations(t)
	})

	t.Run("nil_team_checker_fails_closed", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := &ProgramHandler{
			programRepo: mockRepo,
			uploadDir:   t.TempDir(),
			maxFileSize: 10 * 1024 * 1024,
			log:         log,
		}

		userID := uuid.New()
		teamID := uuid.New()
		tournamentID := uuid.New()
		gameID := uuid.New()

		req := createMultipartRequest(t, map[string]string{
			"team_id":       teamID.String(),
			"tournament_id": tournamentID.String(),
			"game_id":       gameID.String(),
			"name":          "My Strategy",
		}, "strategy.py", []byte("print('hello')"))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("round completed blocks upload", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		mockRoundChecker := new(MockRoundCompletionChecker)
		mockTeamChecker := new(MockTeamMembershipChecker)
		handler := &ProgramHandler{
			programRepo:  mockRepo,
			uploadDir:    t.TempDir(),
			maxFileSize:  10 * 1024 * 1024,
			teamChecker:  mockTeamChecker,
			roundChecker: mockRoundChecker,
			log:          log,
		}

		userID := uuid.New()
		teamID := uuid.New()
		tournamentID := uuid.New()
		gameID := uuid.New()

		mockTeamChecker.On("IsUserInTeam", mock.Anything, teamID, userID).Return(true, nil)
		mockTeamChecker.On("GetByID", mock.Anything, teamID).Return(&models.Team{ID: teamID, TournamentID: tournamentID}, nil)
		mockRoundChecker.On("IsAutoRoundEnabled", mock.Anything, tournamentID, gameID).Return(false, nil)
		mockRoundChecker.On("IsRoundCompleted", mock.Anything, tournamentID, gameID).Return(true, nil)

		req := createMultipartRequest(t, map[string]string{
			"team_id":       teamID.String(),
			"tournament_id": tournamentID.String(),
			"game_id":       gameID.String(),
			"name":          "My Strategy",
		}, "strategy.py", []byte("print('hello')"))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockRoundChecker.AssertExpectations(t)
	})

	t.Run("running matches blocks upload", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		mockMatchChecker := new(MockMatchExistenceChecker)
		mockTeamChecker := new(MockTeamMembershipChecker)
		handler := &ProgramHandler{
			programRepo:  mockRepo,
			uploadDir:    t.TempDir(),
			maxFileSize:  10 * 1024 * 1024,
			teamChecker:  mockTeamChecker,
			matchChecker: mockMatchChecker,
			log:          log,
		}

		userID := uuid.New()
		teamID := uuid.New()
		tournamentID := uuid.New()
		gameID := uuid.New()

		mockTeamChecker.On("IsUserInTeam", mock.Anything, teamID, userID).Return(true, nil)
		mockTeamChecker.On("GetByID", mock.Anything, teamID).Return(&models.Team{ID: teamID, TournamentID: tournamentID}, nil)
		mockMatchChecker.On("HasAnyRunningMatches", mock.Anything, tournamentID).Return(true, nil)
		mockMatchChecker.On("GetActiveGameType", mock.Anything, tournamentID).Return("prisoners_dilemma", nil)

		req := createMultipartRequest(t, map[string]string{
			"team_id":       teamID.String(),
			"tournament_id": tournamentID.String(),
			"game_id":       gameID.String(),
			"name":          "My Strategy",
		}, "strategy.py", []byte("print('hello')"))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockMatchChecker.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		mockTeamChecker := new(MockTeamMembershipChecker)
		handler := &ProgramHandler{
			programRepo: mockRepo,
			teamChecker: mockTeamChecker,
			uploadDir:   t.TempDir(),
			maxFileSize: 10 * 1024 * 1024,
			log:         log,
		}

		userID := uuid.New()
		teamID := uuid.New()
		tournamentID := uuid.New()
		gameID := uuid.New()

		mockTeamChecker.On("IsUserInTeam", mock.Anything, teamID, userID).Return(true, nil)
		mockTeamChecker.On("GetByID", mock.Anything, teamID).Return(&models.Team{ID: teamID, TournamentID: tournamentID}, nil)
		mockRepo.On("GetLatestVersion", mock.Anything, teamID, gameID).Return(3, nil)
		mockRepo.On("CreateWithAtomicVersion", mock.Anything, mock.MatchedBy(func(p *models.Program) bool {
			return p.UserID == userID &&
				p.Name == "My Strategy" &&
				p.Language == "python" &&
				p.TeamID != nil && *p.TeamID == teamID &&
				p.TournamentID != nil && *p.TournamentID == tournamentID &&
				p.GameID != nil && *p.GameID == gameID &&
				p.FilePath != nil && *p.FilePath != ""
		})).Return(nil)

		req := createMultipartRequest(t, map[string]string{
			"team_id":       teamID.String(),
			"tournament_id": tournamentID.String(),
			"game_id":       gameID.String(),
			"name":          "My Strategy",
		}, "strategy.py", []byte("print('hello')"))

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.Create(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response models.Program
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "My Strategy", response.Name)
		assert.Equal(t, "python", response.Language)
		assert.Equal(t, userID, response.UserID)
		// свежезагруженная программа стартует в статусе compiling —
		// собирать её будет worker в песочнице (compiling -> ready/failed)
		assert.Equal(t, models.ProgramCompiling, response.Status)

		mockRepo.AssertExpectations(t)
	})

	// член команды турнира X не загрузит программу в турнир Y, и команда
	// не зальёт больше maxVersionsPerTeamGame версий
	t.Run("foreign tournament and quota", func(t *testing.T) {
		userID, teamID, tournamentID, gameID := uuid.New(), uuid.New(), uuid.New(), uuid.New()

		for name, tc := range map[string]struct {
			teamTournament uuid.UUID
			version        int
			code           int
		}{
			"foreign tournament": {uuid.New(), 0, http.StatusForbidden},
			"quota reached":      {tournamentID, maxVersionsPerTeamGame, http.StatusConflict},
		} {
			mockRepo := new(MockProgramRepository)
			mockTeamChecker := new(MockTeamMembershipChecker)
			handler := &ProgramHandler{programRepo: mockRepo, teamChecker: mockTeamChecker, uploadDir: t.TempDir(), maxFileSize: 1 << 20, log: log}

			mockTeamChecker.On("IsUserInTeam", mock.Anything, teamID, userID).Return(true, nil)
			mockTeamChecker.On("GetByID", mock.Anything, teamID).Return(&models.Team{ID: teamID, TournamentID: tc.teamTournament}, nil)
			mockRepo.On("GetLatestVersion", mock.Anything, teamID, gameID).Return(tc.version, nil)

			req := createMultipartRequest(t, map[string]string{
				"team_id":       teamID.String(),
				"tournament_id": tournamentID.String(),
				"game_id":       gameID.String(),
			}, "strategy.py", []byte("print('hello')"))
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))

			w := httptest.NewRecorder()
			handler.Create(w, req)

			assert.Equal(t, tc.code, w.Code, name)
			mockRepo.AssertNotCalled(t, "CreateWithAtomicVersion", mock.Anything, mock.Anything)
		}
	})
}

func TestCanonicalExtension(t *testing.T) {
	// каноническое расширение для известных языков
	cases := map[string]string{
		LangPython:     ".py",
		LangCpp:        ".cpp",
		LangC:          ".c",
		LangGo:         ".go",
		LangRust:       ".rs",
		LangJava:       ".java",
		LangJavaScript: ".js",
		LangRuby:       ".rb",
		LangPHP:        ".php",
		LangLua:        ".lua",
		"unknown":      "",
		"":             "",
	}
	for lang, want := range cases {
		assert.Equal(t, want, canonicalExtension(lang), "language=%q", lang)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"python", "script.py", "python"},
		{"go", "main.go", "go"},
		{"cpp", "solution.cpp", "cpp"},
		{"cc", "solution.cc", "cpp"},
		{"c", "main.c", "c"},
		{"rust", "lib.rs", "rust"},
		{"java", "Main.java", "java"},
		{"javascript", "index.js", "javascript"},
		{"ruby", "app.rb", "ruby"},
		{"php", "index.php", "php"},
		{"lua", "script.lua", "lua"},
		{"unknown_extension", "readme.txt", "unknown"},
		{"empty_filename", "", "unknown"},
		{"uppercase_py", "SCRIPT.PY", "python"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguage(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetShebang(t *testing.T) {
	tests := []struct {
		name     string
		language string
		expected string
	}{
		{"python", "python", "#!/usr/bin/env python3\n"},
		{"javascript", "javascript", "#!/usr/bin/env node\n"},
		{"ruby", "ruby", "#!/usr/bin/env ruby\n"},
		{"php", "php", "#!/usr/bin/env php\n"},
		{"lua", "lua", "#!/usr/bin/env lua\n"},
		{"go_no_shebang", "go", ""},
		{"cpp_no_shebang", "cpp", ""},
		{"unknown_no_shebang", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getShebang(tt.language)
			assert.Equal(t, tt.expected, result)
		})
	}
}
