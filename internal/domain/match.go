package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrMatchAlreadyProcessed означает, что матч уже не в статусе pending
// и не может быть взят в обработку (защита от дублирования)
var ErrMatchAlreadyProcessed = errors.New("match already processed or in progress")

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
	MatchCancelled MatchStatus = "cancelled"
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
	RoundNumber  int           `json:"round_number" db:"round_number"` // Номер раунда для группировки
	Score1       *int          `json:"score1,omitempty" db:"score1"`
	Score2       *int          `json:"score2,omitempty" db:"score2"`
	Winner       *int          `json:"winner,omitempty" db:"winner"`
	ErrorCode    *int          `json:"error_code,omitempty" db:"error_code"`
	ErrorMessage *string       `json:"error_message,omitempty" db:"error_message"`
	StartedAt    *time.Time    `json:"started_at,omitempty" db:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
}

// MatchRound представляет группу матчей одного раунда для конкретной игры
type MatchRound struct {
	RoundNumber    int       `json:"round_number"`
	GameType       string    `json:"game_type"`
	TotalMatches   int       `json:"total_matches"`
	CompletedCount int       `json:"completed_count"`
	PendingCount   int       `json:"pending_count"`
	RunningCount   int       `json:"running_count"`
	FailedCount    int       `json:"failed_count"`
	Matches        []*Match  `json:"matches"`
	CreatedAt      time.Time `json:"created_at"`
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
