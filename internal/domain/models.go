package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role - роль пользователя в системе
type Role string

const (
	RoleUser  Role = "user"  // Обычный пользователь
	RoleAdmin Role = "admin" // Администратор
)

// User представляет пользователя системы
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         Role      `json:"role" db:"role"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Program представляет программу-бота пользователя
type Program struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Name      string    `json:"name" db:"name"`
	GameType  string    `json:"game_type" db:"game_type"`
	CodePath  string    `json:"code_path" db:"code_path"`
	Language  string    `json:"language" db:"language"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TournamentStatus - статус турнира
type TournamentStatus string

const (
	TournamentPending   TournamentStatus = "pending"
	TournamentActive    TournamentStatus = "active"
	TournamentCompleted TournamentStatus = "completed"
	TournamentCancelled TournamentStatus = "cancelled"
)

// Tournament представляет турнир
type Tournament struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	Name            string                 `json:"name" db:"name"`
	GameType        string                 `json:"game_type" db:"game_type"`
	Status          TournamentStatus       `json:"status" db:"status"`
	MaxParticipants *int                   `json:"max_participants,omitempty" db:"max_participants"`
	StartTime       *time.Time             `json:"start_time,omitempty" db:"start_time"`
	EndTime         *time.Time             `json:"end_time,omitempty" db:"end_time"`
	Metadata        map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	Version         int                    `json:"version" db:"version"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// TournamentParticipant представляет участника турнира
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

// TournamentFilter фильтр для списка турниров
type TournamentFilter struct {
	Status   TournamentStatus
	GameType string
	Limit    int
	Offset   int
}

// MatchFilter фильтр для списка матчей
type MatchFilter struct {
	TournamentID *uuid.UUID
	ProgramID    *uuid.UUID
	Status       MatchStatus
	GameType     string
	Limit        int
	Offset       int
}

// MatchStatus - статус матча
type MatchStatus string

const (
	MatchPending   MatchStatus = "pending"
	MatchRunning   MatchStatus = "running"
	MatchCompleted MatchStatus = "completed"
	MatchFailed    MatchStatus = "failed"
)

// MatchPriority - приоритет матча
type MatchPriority string

const (
	PriorityHigh   MatchPriority = "high"
	PriorityMedium MatchPriority = "medium"
	PriorityLow    MatchPriority = "low"
)

// Match представляет матч между двумя программами
type Match struct {
	ID           uuid.UUID     `json:"id" db:"id"`
	TournamentID uuid.UUID     `json:"tournament_id" db:"tournament_id"`
	Program1ID   uuid.UUID     `json:"program1_id" db:"program1_id"`
	Program2ID   uuid.UUID     `json:"program2_id" db:"program2_id"`
	GameType     string        `json:"game_type" db:"game_type"`
	Status       MatchStatus   `json:"status" db:"status"`
	Priority     MatchPriority `json:"priority" db:"priority"`
	Score1       *int          `json:"score1,omitempty" db:"score1"`
	Score2       *int          `json:"score2,omitempty" db:"score2"`
	Winner       *int          `json:"winner,omitempty" db:"winner"`
	ErrorMessage *string       `json:"error_message,omitempty" db:"error_message"`
	StartedAt    *time.Time    `json:"started_at,omitempty" db:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
}

// RatingHistory представляет историю изменения рейтинга
type RatingHistory struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	ProgramID    uuid.UUID  `json:"program_id" db:"program_id"`
	TournamentID uuid.UUID  `json:"tournament_id" db:"tournament_id"`
	OldRating    int        `json:"old_rating" db:"old_rating"`
	NewRating    int        `json:"new_rating" db:"new_rating"`
	Change       int        `json:"change" db:"change"`
	MatchID      *uuid.UUID `json:"match_id,omitempty" db:"match_id"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// MatchResult - результат матча для обработки воркером
type MatchResult struct {
	MatchID      uuid.UUID
	Score1       int
	Score2       int
	Winner       int // 0 - draw, 1 - program1, 2 - program2
	ErrorCode    int // exit code от tjudge-cli
	ErrorMessage string
	Duration     time.Duration
}

// LeaderboardEntry - запись в таблице лидеров
type LeaderboardEntry struct {
	Rank        int       `json:"rank"`
	ProgramID   uuid.UUID `json:"program_id"`
	ProgramName string    `json:"program_name"`
	Rating      int       `json:"rating"`
	Wins        int       `json:"wins"`
	Losses      int       `json:"losses"`
	Draws       int       `json:"draws"`
	TotalGames  int       `json:"total_games"`
}
