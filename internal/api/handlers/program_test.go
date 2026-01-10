package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProgramRepository mocks the program repository
type MockProgramRepository struct {
	mock.Mock
}

func (m *MockProgramRepository) Create(ctx context.Context, program *domain.Program) error {
	args := m.Called(ctx, program)
	return args.Error(0)
}

func (m *MockProgramRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Program), args.Error(1)
}

func (m *MockProgramRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Program, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Program), args.Error(1)
}

func (m *MockProgramRepository) Update(ctx context.Context, program *domain.Program) error {
	args := m.Called(ctx, program)
	return args.Error(0)
}

func (m *MockProgramRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProgramRepository) CheckOwnership(ctx context.Context, programID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, programID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockProgramRepository) GetLatestVersion(ctx context.Context, teamID, gameID uuid.UUID) (int, error) {
	args := m.Called(ctx, teamID, gameID)
	return args.Int(0), args.Error(1)
}

func (m *MockProgramRepository) GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*domain.Program, error) {
	args := m.Called(ctx, teamID, gameID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Program), args.Error(1)
}

func (m *MockProgramRepository) ClearErrorMessages(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	args := m.Called(ctx, tournamentID)
	return args.Get(0).(int64), args.Error(1)
}

func TestProgramHandler_Create(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully create program", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		reqBody := map[string]string{
			"name":      "My Chess AI",
			"game_type": "chess",
			"code_path": "/programs/chess/ai.py",
			"language":  "python",
		}

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(p *domain.Program) bool {
			return p.UserID == userID && p.Name == reqBody["name"]
		})).Return(nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Add user ID to context
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response domain.Program
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, reqBody["name"], response.Name)
		assert.Equal(t, userID, response.UserID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("missing user ID in context", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		reqBody := map[string]string{
			"name":      "My Chess AI",
			"game_type": "chess",
			"code_path": "/programs/chess/ai.py",
			"language":  "python",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid request body", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("validation error - empty name", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		reqBody := map[string]string{
			"name":      "", // Invalid
			"game_type": "chess",
			"code_path": "/programs/chess/ai.py",
			"language":  "python",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestProgramHandler_List(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully list programs", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		expectedPrograms := []*domain.Program{
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

		var response []*domain.Program
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
		assert.Equal(t, expectedPrograms[0].Name, response[0].Name)

		mockRepo.AssertExpectations(t)
	})

	t.Run("missing user ID in context", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()

		mockRepo.On("GetByUserID", mock.Anything, userID).Return(nil, errors.ErrInternal.WithMessage("database error"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs", nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_Get(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get program", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		programID := uuid.New()
		expectedProgram := &domain.Program{
			ID:       programID,
			UserID:   uuid.New(),
			Name:     "Chess AI",
			GameType: "chess",
			Language: "python",
		}

		mockRepo.On("GetByID", mock.Anything, programID).Return(expectedProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Program
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedProgram.ID, response.ID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/invalid-uuid", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("program not found", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		programID := uuid.New()

		mockRepo.On("GetByID", mock.Anything, programID).Return(nil, errors.ErrNotFound.WithMessage("program not found"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/programs/"+programID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_Update(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully update program", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		programID := uuid.New()

		existingProgram := &domain.Program{
			ID:       programID,
			UserID:   userID,
			Name:     "Old Name",
			GameType: "chess",
			CodePath: "/old/path",
			Language: "python",
		}

		reqBody := map[string]string{
			"name":      "New Name",
			"code_path": "/new/path",
			"language":  "javascript",
		}

		mockRepo.On("CheckOwnership", mock.Anything, programID, userID).Return(true, nil)
		mockRepo.On("GetByID", mock.Anything, programID).Return(existingProgram, nil)
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(p *domain.Program) bool {
			return p.Name == reqBody["name"] && p.CodePath == reqBody["code_path"]
		})).Return(nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/programs/"+programID.String(), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Update(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Program
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, reqBody["name"], response.Name)

		mockRepo.AssertExpectations(t)
	})

	t.Run("not the owner", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		programID := uuid.New()

		reqBody := map[string]string{
			"name":      "New Name",
			"code_path": "/new/path",
			"language":  "javascript",
		}

		mockRepo.On("CheckOwnership", mock.Anything, programID, userID).Return(false, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/programs/"+programID.String(), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Update(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestProgramHandler_Delete(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully delete program", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		programID := uuid.New()

		program := &domain.Program{
			ID:       programID,
			Name:     "test-program",
			UserID:   userID,
			FilePath: nil,
		}

		mockRepo.On("CheckOwnership", mock.Anything, programID, userID).Return(true, nil)
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

	t.Run("not the owner", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()
		programID := uuid.New()

		mockRepo.On("CheckOwnership", mock.Anything, programID, userID).Return(false, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/programs/"+programID.String(), nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", programID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockRepo := new(MockProgramRepository)
		handler := NewProgramHandler(mockRepo, nil, nil, log)

		userID := uuid.New()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/programs/invalid-uuid", nil)

		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
