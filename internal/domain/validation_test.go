package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// --- User.Validate() ---

func TestUser_Validate_Success(t *testing.T) {
	u := &User{
		Username: "validuser",
		Email:    "user@example.com",
	}
	assert.NoError(t, u.Validate())
}

func TestUser_Validate_EmptyUsername(t *testing.T) {
	u := &User{
		Username: "",
		Email:    "user@example.com",
	}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestUser_Validate_EmptyEmail(t *testing.T) {
	u := &User{
		Username: "validuser",
		Email:    "",
	}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestUser_Validate_BothInvalid(t *testing.T) {
	u := &User{
		Username: "",
		Email:    "",
	}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username")
	assert.Contains(t, err.Error(), "email")
}

func TestUser_Validate_InvalidEmail(t *testing.T) {
	u := &User{
		Username: "validuser",
		Email:    "not-an-email",
	}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestUser_Validate_ShortUsername(t *testing.T) {
	u := &User{
		Username: "ab",
		Email:    "user@example.com",
	}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestUser_Validate_LongUsername(t *testing.T) {
	u := &User{
		Username: strings.Repeat("a", 51),
		Email:    "user@example.com",
	}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestUser_Validate_BoundaryUsername(t *testing.T) {
	// 3 chars - minimum valid
	u := &User{
		Username: "abc",
		Email:    "user@example.com",
	}
	assert.NoError(t, u.Validate())

	// 50 chars - maximum valid
	u.Username = strings.Repeat("a", 50)
	assert.NoError(t, u.Validate())
}

// --- Program.Validate() ---

func TestProgram_Validate_Success(t *testing.T) {
	p := &Program{
		Name:     "my-bot",
		GameType: "prisoners_dilemma",
		CodePath: "/path/to/code",
		Language: "python",
	}
	assert.NoError(t, p.Validate())
}

func TestProgram_Validate_EmptyName(t *testing.T) {
	p := &Program{
		Name:     "",
		GameType: "prisoners_dilemma",
		CodePath: "/path/to/code",
		Language: "python",
	}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestProgram_Validate_EmptyGameType(t *testing.T) {
	p := &Program{
		Name:     "bot",
		GameType: "",
		CodePath: "/path/to/code",
		Language: "python",
	}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "game_type")
}

func TestProgram_Validate_EmptyCodePath(t *testing.T) {
	p := &Program{
		Name:     "bot",
		GameType: "chess",
		CodePath: "",
		Language: "python",
	}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "code_path")
}

func TestProgram_Validate_EmptyLanguage(t *testing.T) {
	p := &Program{
		Name:     "bot",
		GameType: "chess",
		CodePath: "/path",
		Language: "",
	}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "language")
}

func TestProgram_Validate_NameTooLong(t *testing.T) {
	p := &Program{
		Name:     strings.Repeat("a", 101),
		GameType: "chess",
		CodePath: "/path",
		Language: "python",
	}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestProgram_Validate_MultipleEmpty(t *testing.T) {
	p := &Program{
		Name:     "",
		GameType: "",
		CodePath: "",
		Language: "",
	}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
	assert.Contains(t, err.Error(), "game_type")
	assert.Contains(t, err.Error(), "code_path")
	assert.Contains(t, err.Error(), "language")
}

// --- Tournament.Validate() ---

func TestTournament_Validate_Success(t *testing.T) {
	tr := &Tournament{
		Name:     "Test Tournament",
		GameType: "prisoners_dilemma",
		Status:   TournamentPending,
	}
	assert.NoError(t, tr.Validate())
}

func TestTournament_Validate_EmptyName(t *testing.T) {
	tr := &Tournament{
		Name:     "",
		GameType: "chess",
		Status:   TournamentPending,
	}
	err := tr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestTournament_Validate_NameTooLong(t *testing.T) {
	tr := &Tournament{
		Name:     strings.Repeat("a", 256),
		GameType: "chess",
		Status:   TournamentPending,
	}
	err := tr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestTournament_Validate_InvalidStatus(t *testing.T) {
	tr := &Tournament{
		Name:     "Test",
		GameType: "chess",
		Status:   TournamentStatus("invalid"),
	}
	err := tr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestTournament_Validate_AllValidStatuses(t *testing.T) {
	statuses := []TournamentStatus{
		TournamentPending,
		TournamentActive,
		TournamentCompleted,
		TournamentCancelled,
	}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			tr := &Tournament{
				Name:     "Test",
				GameType: "chess",
				Status:   status,
			}
			assert.NoError(t, tr.Validate())
		})
	}
}

func TestTournament_Validate_NegativeMaxParticipants(t *testing.T) {
	neg := -1
	tr := &Tournament{
		Name:            "Test",
		GameType:        "chess",
		Status:          TournamentPending,
		MaxParticipants: &neg,
	}
	err := tr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_participants")
}

func TestTournament_Validate_ZeroMaxParticipants(t *testing.T) {
	zero := 0
	tr := &Tournament{
		Name:            "Test",
		GameType:        "chess",
		Status:          TournamentPending,
		MaxParticipants: &zero,
	}
	err := tr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_participants")
}

func TestTournament_Validate_NilMaxParticipants(t *testing.T) {
	tr := &Tournament{
		Name:            "Test",
		GameType:        "chess",
		Status:          TournamentPending,
		MaxParticipants: nil,
	}
	assert.NoError(t, tr.Validate())
}

func TestTournament_Validate_PositiveMaxParticipants(t *testing.T) {
	pos := 10
	tr := &Tournament{
		Name:            "Test",
		GameType:        "chess",
		Status:          TournamentPending,
		MaxParticipants: &pos,
	}
	assert.NoError(t, tr.Validate())
}

// --- Match.Validate() ---

func TestMatch_Validate_Success(t *testing.T) {
	m := &Match{
		Program1ID: uuid.New(),
		Program2ID: uuid.New(),
		Status:     MatchPending,
		Priority:   PriorityMedium,
	}
	assert.NoError(t, m.Validate())
}

func TestMatch_Validate_InvalidStatus(t *testing.T) {
	m := &Match{
		Program1ID: uuid.New(),
		Program2ID: uuid.New(),
		Status:     MatchStatus("invalid"),
		Priority:   PriorityMedium,
	}
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestMatch_Validate_InvalidPriority(t *testing.T) {
	m := &Match{
		Program1ID: uuid.New(),
		Program2ID: uuid.New(),
		Status:     MatchPending,
		Priority:   MatchPriority("invalid"),
	}
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "priority")
}

func TestMatch_Validate_SamePrograms(t *testing.T) {
	id := uuid.New()
	m := &Match{
		Program1ID: id,
		Program2ID: id,
		Status:     MatchPending,
		Priority:   PriorityMedium,
	}
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "program")
}

func TestMatch_Validate_WinnerValues(t *testing.T) {
	tests := []struct {
		name    string
		winner  *int
		wantErr bool
	}{
		{"nil winner", nil, false},
		{"winner 0 (draw)", new(0), false},
		{"winner 1", new(1), false},
		{"winner 2", new(2), false},
		{"winner -1 invalid", new(-1), true},
		{"winner 3 invalid", new(3), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Match{
				Program1ID: uuid.New(),
				Program2ID: uuid.New(),
				Status:     MatchPending,
				Priority:   PriorityMedium,
				Winner:     tc.winner,
			}
			err := m.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "winner")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- TournamentParticipant.Validate() ---

func TestTournamentParticipant_Validate_Success(t *testing.T) {
	tp := &TournamentParticipant{
		Rating: 1000,
		Wins:   5,
		Losses: 3,
		Draws:  2,
	}
	assert.NoError(t, tp.Validate())
}

func TestTournamentParticipant_Validate_ZeroValues(t *testing.T) {
	tp := &TournamentParticipant{
		Rating: 0,
		Wins:   0,
		Losses: 0,
		Draws:  0,
	}
	assert.NoError(t, tp.Validate())
}

func TestTournamentParticipant_Validate_NegativeRating(t *testing.T) {
	tp := &TournamentParticipant{Rating: -1}
	err := tp.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rating")
}

func TestTournamentParticipant_Validate_NegativeWins(t *testing.T) {
	tp := &TournamentParticipant{Wins: -1}
	err := tp.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wins")
}

func TestTournamentParticipant_Validate_NegativeLosses(t *testing.T) {
	tp := &TournamentParticipant{Losses: -1}
	err := tp.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "losses")
}

func TestTournamentParticipant_Validate_NegativeDraws(t *testing.T) {
	tp := &TournamentParticipant{Draws: -1}
	err := tp.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draws")
}

// --- ValidatePassword() ---

func TestValidatePassword_Valid(t *testing.T) {
	assert.NoError(t, ValidatePassword("SecurePass1"))
}

func TestValidatePassword_Empty(t *testing.T) {
	err := ValidatePassword("")
	assert.Error(t, err)
}

func TestValidatePassword_TooShort(t *testing.T) {
	err := ValidatePassword("Ab1")
	assert.Error(t, err)
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	err := ValidatePassword("lowercase123")
	assert.Error(t, err)
}

func TestValidatePassword_NoDigit(t *testing.T) {
	err := ValidatePassword("NoDigitsHere")
	assert.Error(t, err)
}

func TestValidatePassword_NoLowercase(t *testing.T) {
	err := ValidatePassword("UPPERCASE123")
	assert.Error(t, err)
}

