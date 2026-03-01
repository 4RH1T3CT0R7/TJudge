package events

import (
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
)

// TournamentCreated is published when a new tournament is created.
type TournamentCreated struct {
	Tournament *domain.Tournament
}

// TournamentStarted is published when a tournament transitions to active.
type TournamentStarted struct {
	TournamentID uuid.UUID
	Status       domain.TournamentStatus
	StartTime    *time.Time
}

// TournamentCompleted is published when a tournament finishes.
type TournamentCompleted struct {
	TournamentID uuid.UUID
	Status       domain.TournamentStatus
	EndTime      *time.Time
}

// TournamentDeleted is published when a tournament is removed.
type TournamentDeleted struct {
	TournamentID uuid.UUID
}

// ParticipantJoined is published when a program joins a tournament.
type ParticipantJoined struct {
	TournamentID  uuid.UUID
	ProgramID     uuid.UUID
	InitialRating int
}

// MatchesCreated is published when new matches are scheduled.
type MatchesCreated struct {
	TournamentID uuid.UUID
	ProgramID    uuid.UUID // the program matches were created for (zero if bulk)
	MatchCount   int
}

// GameRoundReset is published when a game round is reset (matches deleted, ratings reverted to 1500).
type GameRoundReset struct {
	TournamentID uuid.UUID
	GameID       uuid.UUID
}

// MatchResultProcessed is published after ELO ratings are updated for a match.
type MatchResultProcessed struct {
	TournamentID uuid.UUID
	MatchID      uuid.UUID
	Program1ID   uuid.UUID
	Program2ID   uuid.UUID
	NewRating1   int
	NewRating2   int
	Winner       int
}
