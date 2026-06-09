package events

import (
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
)

// TournamentCreated is published when a new tournament is created.
type TournamentCreated struct {
	Version    int // Event schema version
	Tournament *domain.Tournament
}

// TournamentStarted is published when a tournament transitions to active.
type TournamentStarted struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
	Status       domain.TournamentStatus
	StartTime    *time.Time
}

// TournamentCompleted is published when a tournament finishes.
type TournamentCompleted struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
	Status       domain.TournamentStatus
	EndTime      *time.Time
}

// TournamentDeleted is published when a tournament is removed.
type TournamentDeleted struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
}

// ParticipantJoined is published when a program joins a tournament.
type ParticipantJoined struct {
	Version       int // Event schema version
	TournamentID  uuid.UUID
	ProgramID     uuid.UUID
	InitialRating int
}

// MatchesCreated is published when new matches are scheduled.
type MatchesCreated struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
	ProgramID    uuid.UUID // the program matches were created for (zero if bulk)
	MatchCount   int
}

// ProgramCompiled is published when async compilation of an uploaded program
// finishes (successfully or not). Carries everything the WebSocket layer needs
// to notify the team without extra DB reads.
type ProgramCompiled struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
	ProgramID    uuid.UUID
	TeamID       uuid.UUID
	Status       string  // domain.ProgramStatus: ready | failed
	ErrorMessage *string // компиляционная ошибка при status=failed
}

// GameRoundReset is published when a game round is reset (matches deleted, ratings reverted to 1500).
type GameRoundReset struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
	GameID       uuid.UUID
}

// MatchResultProcessed is published after ELO ratings are updated for a match.
type MatchResultProcessed struct {
	Version      int // Event schema version
	TournamentID uuid.UUID
	MatchID      uuid.UUID
	Program1ID   uuid.UUID
	Program2ID   uuid.UUID
	NewRating1   int
	NewRating2   int
	Winner       int
}
