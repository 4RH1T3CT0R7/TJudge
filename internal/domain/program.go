package domain

import (
	"time"

	"github.com/google/uuid"
)

// Program представляет программу-бота пользователя
type Program struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	Name         string     `json:"name" db:"name"`
	GameType     string     `json:"game_type" db:"game_type"`
	CodePath     string     `json:"-" db:"code_path"`
	Language     string     `json:"language" db:"language"`
	TeamID       *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	TournamentID *uuid.UUID `json:"tournament_id,omitempty" db:"tournament_id"`
	GameID       *uuid.UUID `json:"game_id,omitempty" db:"game_id"`
	FilePath     *string    `json:"-" db:"file_path"`
	ErrorMessage *string    `json:"error_message,omitempty" db:"error_message"`
	Version      int        `json:"version" db:"version"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}
