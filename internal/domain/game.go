package domain

import (
	"time"

	"github.com/google/uuid"
)

type Game struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`                 // уникальное имя [a-z0-9_]+
	DisplayName string    `json:"display_name" db:"display_name"` // для отображения
	Rules       string    `json:"rules" db:"rules"`               // markdown
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// связка турнир-игра
type TournamentGame struct {
	TournamentID          uuid.UUID  `json:"tournament_id" db:"tournament_id"`
	GameID                uuid.UUID  `json:"game_id" db:"game_id"`
	IsActive              bool       `json:"is_active" db:"is_active"`
	RoundCompleted        bool       `json:"round_completed" db:"round_completed"`
	RoundCompletedAt      *time.Time `json:"round_completed_at,omitempty" db:"round_completed_at"`
	CurrentRound          int        `json:"current_round" db:"current_round"`
	AutoRoundEnabled      bool       `json:"auto_round_enabled" db:"auto_round_enabled"`
	AutoRoundIntervalSecs int        `json:"auto_round_interval_seconds" db:"auto_round_interval_seconds"`
	AutoRoundLastRunAt    *time.Time `json:"auto_round_last_run_at,omitempty" db:"auto_round_last_run_at"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
}

// то же самое, но с именем игры (для апи)
type TournamentGameWithDetails struct {
	TournamentID          uuid.UUID  `json:"tournament_id" db:"tournament_id"`
	GameID                uuid.UUID  `json:"game_id" db:"game_id"`
	GameName              string     `json:"game_name" db:"game_name"`
	GameDisplayName       string     `json:"game_display_name" db:"game_display_name"`
	IsActive              bool       `json:"is_active" db:"is_active"`
	RoundCompleted        bool       `json:"round_completed" db:"round_completed"`
	RoundCompletedAt      *time.Time `json:"round_completed_at,omitempty" db:"round_completed_at"`
	CurrentRound          int        `json:"current_round" db:"current_round"`
	AutoRoundEnabled      bool       `json:"auto_round_enabled" db:"auto_round_enabled"`
	AutoRoundIntervalSecs int        `json:"auto_round_interval_seconds" db:"auto_round_interval_seconds"`
	AutoRoundLastRunAt    *time.Time `json:"auto_round_last_run_at,omitempty" db:"auto_round_last_run_at"`
}

// инфа для планировщика авто-раундов
type AutoRoundGameInfo struct {
	TournamentID    uuid.UUID  `db:"tournament_id"`
	GameID          uuid.UUID  `db:"game_id"`
	GameType        string     `db:"game_type"`
	IntervalSeconds int        `db:"auto_round_interval_seconds"`
	LastRunAt       *time.Time `db:"auto_round_last_run_at"`
}

type GameFilter struct {
	Name   string
	Limit  int
	Offset int
}
