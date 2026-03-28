//go:build contract

package contract

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestContract_Program_Create_201_JSON(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	programID := uuid.New()

	// The JSON create path (handleJSONCreate) calls:
	//   1. program.Validate() — requires name, game_type, code_path, language
	//   2. programRepo.Create(ctx, program)
	h.ProgramRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(p *domain.Program) bool {
			return p.Name == "bot" &&
				p.GameType == "prisoners_dilemma" &&
				p.CodePath == "bot.py" &&
				p.Language == "python" &&
				p.UserID == h.TestUserID
		})).
		Return(nil).Once()

	resp := h.POST("/api/v1/programs").
		WithAuth(h.UserToken()).
		WithJSON(map[string]interface{}{
			"name":      "bot",
			"game_type": "prisoners_dilemma",
			"code_path": "bot.py",
			"language":  "python",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, "bot", data["name"])
	assert.Equal(t, "python", data["language"])
	_ = programID // programID is generated inside handler; we verify fields instead
}

func TestContract_Program_Create_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.POST("/api/v1/programs").
		WithJSON(map[string]interface{}{
			"name":      "bot",
			"game_type": "prisoners_dilemma",
			"language":  "python",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertErrorResponse(t, body)
}

func TestContract_Program_List_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	programID := uuid.New()

	h.ProgramRepo.EXPECT().
		GetByUserID(mock.Anything, h.TestUserID).
		Return([]*domain.Program{
			{
				ID:        programID,
				UserID:    h.TestUserID,
				Name:      "my-bot",
				GameType:  "prisoners_dilemma",
				Language:  "python",
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}, nil).Once()

	resp := h.GET("/api/v1/programs").
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	// Response is an array inside the data envelope.
	assert.Contains(t, string(body), "my-bot")
	assert.Contains(t, string(body), programID.String())
}

func TestContract_Program_Get_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	programID := uuid.New()

	h.ProgramRepo.EXPECT().
		GetByID(mock.Anything, programID).
		Return(&domain.Program{
			ID:        programID,
			UserID:    h.TestUserID,
			Name:      "my-bot",
			GameType:  "prisoners_dilemma",
			Language:  "python",
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil).Once()

	resp := h.GET(fmt.Sprintf("/api/v1/programs/%s", programID)).
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, programID.String(), data["id"])
	assert.Equal(t, "my-bot", data["name"])
}

func TestContract_Program_GetVersions_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	gameID := uuid.New()
	programID1 := uuid.New()
	programID2 := uuid.New()

	h.ProgramRepo.EXPECT().
		GetAllVersionsByTeamAndGame(mock.Anything, teamID, gameID).
		Return([]*domain.Program{
			{
				ID:        programID1,
				UserID:    h.TestUserID,
				Name:      "bot-v1",
				GameType:  "prisoners_dilemma",
				Language:  "python",
				TeamID:    &teamID,
				GameID:    &gameID,
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        programID2,
				UserID:    h.TestUserID,
				Name:      "bot-v2",
				GameType:  "prisoners_dilemma",
				Language:  "python",
				TeamID:    &teamID,
				GameID:    &gameID,
				Version:   2,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}, nil).Once()

	resp := h.GET(fmt.Sprintf("/api/v1/programs/versions?team_id=%s&game_id=%s", teamID, gameID)).
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	assert.Contains(t, string(body), "bot-v1")
	assert.Contains(t, string(body), "bot-v2")
}

func TestContract_Program_Update_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	programID := uuid.New()

	// Update handler calls CheckOwnership first.
	h.ProgramRepo.EXPECT().
		CheckOwnership(mock.Anything, programID, h.TestUserID).
		Return(true, nil).Once()

	// Then it calls GetByID to load the existing program.
	h.ProgramRepo.EXPECT().
		GetByID(mock.Anything, programID).
		Return(&domain.Program{
			ID:        programID,
			UserID:    h.TestUserID,
			Name:      "old-name",
			GameType:  "prisoners_dilemma",
			Language:  "python",
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil).Once()

	// Then Update with the modified program.
	h.ProgramRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(p *domain.Program) bool {
			return p.ID == programID && p.Name == "updated-bot"
		})).
		Return(nil).Once()

	resp := h.PUT(fmt.Sprintf("/api/v1/programs/%s", programID)).
		WithAuth(h.UserToken()).
		WithJSON(map[string]interface{}{
			"name":      "updated-bot",
			"code_path": "bot.py",
			"language":  "python",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, "updated-bot", data["name"])
}

func TestContract_Program_Delete_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	programID := uuid.New()

	// Delete handler checks ownership.
	h.ProgramRepo.EXPECT().
		CheckOwnership(mock.Anything, programID, h.TestUserID).
		Return(true, nil).Once()

	// Then loads the program to delete the file.
	h.ProgramRepo.EXPECT().
		GetByID(mock.Anything, programID).
		Return(&domain.Program{
			ID:        programID,
			UserID:    h.TestUserID,
			Name:      "to-delete",
			GameType:  "prisoners_dilemma",
			Language:  "python",
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil).Once()

	// Then deletes from DB.
	h.ProgramRepo.EXPECT().
		Delete(mock.Anything, programID).
		Return(nil).Once()

	resp := h.DELETE(fmt.Sprintf("/api/v1/programs/%s", programID)).
		WithAuth(h.UserToken()).
		Do()

	ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Program_Download_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	programID := uuid.New()

	// Create file inside the handler's uploadDir so path validation passes.
	filePath := filepath.Join(h.UploadDir, "test.py")
	err := os.WriteFile(filePath, []byte("print('hello')"), 0644)
	require.NoError(t, err)

	h.ProgramRepo.EXPECT().
		GetByID(mock.Anything, programID).
		Return(&domain.Program{
			ID:        programID,
			UserID:    h.TestAdminID,
			Name:      "test",
			GameType:  "prisoners_dilemma",
			Language:  "python",
			FilePath:  &filePath,
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil).Once()

	resp := h.GET(fmt.Sprintf("/api/v1/programs/%s/download", programID)).
		WithAuth(h.AdminToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
	assert.Equal(t, "print('hello')", string(body))
}

func TestContract_Program_ClearErrors_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.ProgramRepo.EXPECT().
		ClearErrorMessages(mock.Anything, tournamentID).
		Return(int64(7), nil).Once()

	resp := h.POST(fmt.Sprintf("/api/v1/tournaments/%s/programs/clear-errors", tournamentID)).
		WithAuth(h.AdminToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, float64(7), data["cleared"])
	assert.Contains(t, data["message"], "7")
}

func TestContract_Program_AllProtectedEndpoints_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	fakeID := uuid.New().String()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET /programs", "GET", "/api/v1/programs"},
		{"GET /programs/{id}", "GET", fmt.Sprintf("/api/v1/programs/%s", fakeID)},
		{"POST /programs", "POST", "/api/v1/programs"},
		{"PUT /programs/{id}", "PUT", fmt.Sprintf("/api/v1/programs/%s", fakeID)},
		{"DELETE /programs/{id}", "DELETE", fmt.Sprintf("/api/v1/programs/%s", fakeID)},
		{"GET /programs/{id}/download", "GET", fmt.Sprintf("/api/v1/programs/%s/download", fakeID)},
		{"GET /programs/versions", "GET", fmt.Sprintf("/api/v1/programs/versions?team_id=%s&game_id=%s", fakeID, fakeID)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rb *RequestBuilder
			switch tc.method {
			case "GET":
				rb = h.GET(tc.path)
			case "POST":
				rb = h.POST(tc.path)
			case "PUT":
				rb = h.PUT(tc.path)
			case "DELETE":
				rb = h.DELETE(tc.path)
			}

			resp := rb.Do()
			body := ReadBody(t, resp)

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"expected 401 for unauthenticated %s", tc.name)
			AssertErrorResponse(t, body)
		})
	}
}
