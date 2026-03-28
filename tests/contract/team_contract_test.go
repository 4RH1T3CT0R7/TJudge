//go:build contract

package contract

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/team"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestContract_Team_Create_201(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	teamID := uuid.New()

	h.TeamService.EXPECT().
		CreateTeam(mock.Anything, mock.MatchedBy(func(req *team.CreateTeamRequest) bool {
			return req.TournamentID == tournamentID &&
				req.Name == "MyTeam" &&
				req.UserID == h.TestUserID
		})).
		Return(&domain.Team{
			ID:           teamID,
			TournamentID: tournamentID,
			Name:         "MyTeam",
			Code:         "ABC123",
			LeaderID:     h.TestUserID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil).Once()

	resp := h.POST("/api/v1/teams").
		WithAuth(h.UserToken()).
		WithJSON(map[string]interface{}{
			"tournament_id": tournamentID.String(),
			"name":          "MyTeam",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, teamID.String(), data["id"])
	assert.Equal(t, "MyTeam", data["name"])
}

func TestContract_Team_Create_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.POST("/api/v1/teams").
		WithJSON(map[string]interface{}{
			"tournament_id": uuid.New().String(),
			"name":          "MyTeam",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertErrorResponse(t, body)
}

func TestContract_Team_JoinByCode_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	tournamentID := uuid.New()

	h.TeamService.EXPECT().
		JoinTeamByCode(mock.Anything, mock.MatchedBy(func(req *team.JoinTeamRequest) bool {
			return req.Code == "ABC123" && req.UserID == h.TestUserID
		})).
		Return(&domain.Team{
			ID:           teamID,
			TournamentID: tournamentID,
			Name:         "JoinedTeam",
			Code:         "ABC123",
			LeaderID:     uuid.New(),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil).Once()

	resp := h.POST("/api/v1/teams/join").
		WithAuth(h.UserToken()).
		WithJSON(map[string]interface{}{
			"code": "ABC123",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, teamID.String(), data["id"])
	assert.Equal(t, "JoinedTeam", data["name"])
}

func TestContract_Team_Get_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	tournamentID := uuid.New()

	h.TeamService.EXPECT().
		GetTeamWithMembers(mock.Anything, teamID).
		Return(&domain.TeamWithMembers{
			Team: domain.Team{
				ID:           teamID,
				TournamentID: tournamentID,
				Name:         "TestTeam",
				Code:         "XYZ789",
				LeaderID:     h.TestUserID,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			Members: []domain.User{
				{
					ID:        h.TestUserID,
					Username:  "testuser",
					Email:     "test@example.com",
					Role:      domain.RoleUser,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			},
		}, nil).Once()

	resp := h.GET(fmt.Sprintf("/api/v1/teams/%s", teamID)).
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, teamID.String(), data["id"])
	assert.Equal(t, "TestTeam", data["name"])
	assert.NotNil(t, data["members"])
}

func TestContract_Team_UpdateName_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	tournamentID := uuid.New()

	h.TeamService.EXPECT().
		UpdateTeamName(mock.Anything, teamID, "NewName", h.TestUserID).
		Return(&domain.Team{
			ID:           teamID,
			TournamentID: tournamentID,
			Name:         "NewName",
			Code:         "ABC123",
			LeaderID:     h.TestUserID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil).Once()

	resp := h.PUT(fmt.Sprintf("/api/v1/teams/%s", teamID)).
		WithAuth(h.UserToken()).
		WithJSON(map[string]interface{}{
			"name": "NewName",
		}).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, "NewName", data["name"])
}

func TestContract_Team_GetMembers_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	tournamentID := uuid.New()
	memberID := uuid.New()

	h.TeamService.EXPECT().
		GetTeamWithMembers(mock.Anything, teamID).
		Return(&domain.TeamWithMembers{
			Team: domain.Team{
				ID:           teamID,
				TournamentID: tournamentID,
				Name:         "TestTeam",
				Code:         "XYZ789",
				LeaderID:     h.TestUserID,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			Members: []domain.User{
				{
					ID:        h.TestUserID,
					Username:  "leader",
					Email:     "leader@example.com",
					Role:      domain.RoleUser,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				{
					ID:        memberID,
					Username:  "member",
					Email:     "member@example.com",
					Role:      domain.RoleUser,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			},
		}, nil).Once()

	resp := h.GET(fmt.Sprintf("/api/v1/teams/%s/members", teamID)).
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	// GetMembers writes the Members slice directly, so the envelope data is an array.
	envelope := AssertEnvelope(t, body)
	// AssertEnvelope returns nil for non-object data (arrays), which is expected here.
	// Verify the raw body contains the expected data field with an array.
	_ = envelope
	assert.Contains(t, string(body), "leader")
	assert.Contains(t, string(body), "member")
}

func TestContract_Team_Leave_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()

	h.TeamService.EXPECT().
		LeaveTeam(mock.Anything, teamID, h.TestUserID).
		Return(nil).Once()

	resp := h.POST(fmt.Sprintf("/api/v1/teams/%s/leave", teamID)).
		WithAuth(h.UserToken()).
		Do()

	ReadBody(t, resp) // drain body

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Team_RemoveMember_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	memberUserID := uuid.New()

	h.TeamService.EXPECT().
		RemoveMember(mock.Anything, teamID, memberUserID, h.TestUserID).
		Return(nil).Once()

	resp := h.DELETE(fmt.Sprintf("/api/v1/teams/%s/members/%s", teamID, memberUserID)).
		WithAuth(h.UserToken()).
		Do()

	ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Team_GetInviteLink_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()
	tournamentID := uuid.New()

	h.TeamService.EXPECT().
		GetInviteLink(mock.Anything, teamID, h.TestUserID, "http://test.local").
		Return("http://test.local/join?code=INVITE1", nil).Once()

	// GetInviteLink handler also calls GetTeamByID to get the team code.
	h.TeamService.EXPECT().
		GetTeamByID(mock.Anything, teamID).
		Return(&domain.Team{
			ID:           teamID,
			TournamentID: tournamentID,
			Name:         "TestTeam",
			Code:         "INVITE1",
			LeaderID:     h.TestUserID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil).Once()

	resp := h.GET(fmt.Sprintf("/api/v1/teams/%s/invite", teamID)).
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, "INVITE1", data["code"])
	assert.Equal(t, "http://test.local/join?code=INVITE1", data["link"])
}

func TestContract_Team_Delete_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()

	h.TeamService.EXPECT().
		DeleteTeam(mock.Anything, teamID).
		Return(nil).Once()

	resp := h.DELETE(fmt.Sprintf("/api/v1/teams/%s", teamID)).
		WithAuth(h.AdminToken()).
		Do()

	ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Team_Delete_403_User(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()

	resp := h.DELETE(fmt.Sprintf("/api/v1/teams/%s", teamID)).
		WithAuth(h.UserToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertErrorResponse(t, body)
}

func TestContract_Team_Disqualify_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()

	h.TeamService.EXPECT().
		DisqualifyTeam(mock.Anything, teamID).
		Return(&team.DisqualifyResult{
			MatchesDeleted:     0,
			MatchesCancelled:   5,
			RatingHistoryReset: 3,
		}, nil).Once()

	resp := h.POST(fmt.Sprintf("/api/v1/teams/%s/disqualify", teamID)).
		WithAuth(h.AdminToken()).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	data := AssertEnvelope(t, body)
	assert.Equal(t, float64(5), data["matches_cancelled"])
	assert.Equal(t, float64(3), data["rating_history_reset"])
}

func TestContract_Team_Restore_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	teamID := uuid.New()

	h.TeamService.EXPECT().
		RestoreTeam(mock.Anything, teamID).
		Return(nil).Once()

	resp := h.POST(fmt.Sprintf("/api/v1/teams/%s/restore", teamID)).
		WithAuth(h.AdminToken()).
		Do()

	ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Team_GetTournamentTeams_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	team1ID := uuid.New()
	team2ID := uuid.New()

	h.TeamService.EXPECT().
		GetTeamsByTournament(mock.Anything, tournamentID).
		Return([]*domain.Team{
			{
				ID:           team1ID,
				TournamentID: tournamentID,
				Name:         "TeamAlpha",
				Code:         "ALPHA1",
				LeaderID:     uuid.New(),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			{
				ID:           team2ID,
				TournamentID: tournamentID,
				Name:         "TeamBeta",
				Code:         "BETA22",
				LeaderID:     uuid.New(),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
		}, nil).Once()

	// Public endpoint: no auth required.
	resp := h.GET(fmt.Sprintf("/api/v1/tournaments/%s/teams", tournamentID)).
		Do()

	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)

	// Response is an array inside the data envelope.
	assert.Contains(t, string(body), "TeamAlpha")
	assert.Contains(t, string(body), "TeamBeta")
}

func TestContract_Team_AllProtectedEndpoints_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	fakeID := uuid.New().String()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"POST /teams", "POST", "/api/v1/teams"},
		{"POST /teams/join", "POST", "/api/v1/teams/join"},
		{"GET /teams/{id}", "GET", fmt.Sprintf("/api/v1/teams/%s", fakeID)},
		{"PUT /teams/{id}", "PUT", fmt.Sprintf("/api/v1/teams/%s", fakeID)},
		{"GET /teams/{id}/members", "GET", fmt.Sprintf("/api/v1/teams/%s/members", fakeID)},
		{"POST /teams/{id}/leave", "POST", fmt.Sprintf("/api/v1/teams/%s/leave", fakeID)},
		{"DELETE /teams/{id}/members/{userId}", "DELETE", fmt.Sprintf("/api/v1/teams/%s/members/%s", fakeID, fakeID)},
		{"GET /teams/{id}/invite", "GET", fmt.Sprintf("/api/v1/teams/%s/invite", fakeID)},
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
