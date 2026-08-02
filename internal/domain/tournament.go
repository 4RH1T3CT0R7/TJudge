package domain

import (
	"time"

	"github.com/google/uuid"
)

type TournamentStatus string

const (
	TournamentPending   TournamentStatus = "pending"
	TournamentActive    TournamentStatus = "active"
	TournamentCompleted TournamentStatus = "completed"
	TournamentCancelled TournamentStatus = "cancelled"
)

type Tournament struct {
	ID              uuid.UUID        `json:"id" db:"id"`
	Name            string           `json:"name" db:"name"`
	Code            string           `json:"code" db:"code"` // уникальный код, 6-8 символов
	Description     string           `json:"description" db:"description"`
	GameType        string           `json:"game_type" db:"game_type"`
	Status          TournamentStatus `json:"status" db:"status"`
	MaxParticipants *int             `json:"max_participants,omitempty" db:"max_participants"`
	MaxTeamSize     int              `json:"max_team_size" db:"max_team_size"`
	IsPermanent     bool             `json:"is_permanent" db:"is_permanent"`
	CreatorID       *uuid.UUID       `json:"creator_id,omitempty" db:"creator_id"`
	StartTime       *time.Time       `json:"start_time,omitempty" db:"start_time"`
	EndTime         *time.Time       `json:"end_time,omitempty" db:"end_time"`
	Metadata        map[string]any   `json:"metadata,omitempty" db:"metadata"`
	Version         int              `json:"version" db:"version"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at" db:"updated_at"`
}

type TournamentWithGames struct {
	Tournament
	Games []Game `json:"games"`
}

type TournamentParticipant struct {
	ID           uuid.UUID `json:"id" db:"id"`
	TournamentID uuid.UUID `json:"tournament_id" db:"tournament_id"`
	ProgramID    uuid.UUID `json:"program_id" db:"program_id"`
	Rating       int       `json:"rating" db:"rating"`
	Wins         int       `json:"wins" db:"wins"`
	Losses       int       `json:"losses" db:"losses"`
	Draws        int       `json:"draws" db:"draws"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type TournamentFilter struct {
	Status   TournamentStatus
	GameType string
	Limit    int
	Offset   int
}
