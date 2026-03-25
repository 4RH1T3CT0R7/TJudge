package domain

import (
	"time"

	"github.com/google/uuid"
)

// Game представляет игру в системе
type Game struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`                 // Уникальное название [a-z0-9_]+
	DisplayName string    `json:"display_name" db:"display_name"` // Название для отображения
	Rules       string    `json:"rules" db:"rules"`               // Правила в формате Markdown
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TournamentGame - связь турнира с игрой
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

// TournamentGameWithDetails содержит данные связи турнир-игра вместе с информацией об игре
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

// AutoRoundGameInfo содержит информацию для планировщика авто-раундов
type AutoRoundGameInfo struct {
	TournamentID    uuid.UUID  `db:"tournament_id"`
	GameID          uuid.UUID  `db:"game_id"`
	GameType        string     `db:"game_type"`
	IntervalSeconds int        `db:"auto_round_interval_seconds"`
	LastRunAt       *time.Time `db:"auto_round_last_run_at"`
}

// GameFilter - фильтр для списка игр
type GameFilter struct {
	Name   string
	Limit  int
	Offset int
}
