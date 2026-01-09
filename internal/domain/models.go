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
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	Name         string     `json:"name" db:"name"`
	GameType     string     `json:"game_type" db:"game_type"`
	CodePath     string     `json:"code_path" db:"code_path"`
	Language     string     `json:"language" db:"language"`
	TeamID       *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	TournamentID *uuid.UUID `json:"tournament_id,omitempty" db:"tournament_id"`
	GameID       *uuid.UUID `json:"game_id,omitempty" db:"game_id"`
	FilePath     *string    `json:"file_path,omitempty" db:"file_path"`
	ErrorMessage *string    `json:"error_message,omitempty" db:"error_message"`
	Version      int        `json:"version" db:"version"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// Game представляет игру в системе
type Game struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`                 // Уникальное название [a-z0-9_]+
	DisplayName string    `json:"display_name" db:"display_name"` // Название для отображения
	Rules       string    `json:"rules" db:"rules"`               // Правила в формате Markdown
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Team представляет команду в турнире
type Team struct {
	ID           uuid.UUID `json:"id" db:"id"`
	TournamentID uuid.UUID `json:"tournament_id" db:"tournament_id"`
	Name         string    `json:"name" db:"name"`
	Code         string    `json:"code" db:"code"` // 6-8 символов уникальный код
	LeaderID     uuid.UUID `json:"leader_id" db:"leader_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// TeamMember представляет участника команды
type TeamMember struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TeamID   uuid.UUID `json:"team_id" db:"team_id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

// TeamWithMembers - команда с участниками для API ответов
type TeamWithMembers struct {
	Team
	Members []User `json:"members"`
}

// TournamentGame - связь турнира с игрой
type TournamentGame struct {
	TournamentID     uuid.UUID  `json:"tournament_id" db:"tournament_id"`
	GameID           uuid.UUID  `json:"game_id" db:"game_id"`
	IsActive         bool       `json:"is_active" db:"is_active"`
	RoundCompleted   bool       `json:"round_completed" db:"round_completed"`
	RoundCompletedAt *time.Time `json:"round_completed_at,omitempty" db:"round_completed_at"`
	CurrentRound     int        `json:"current_round" db:"current_round"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
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
	Code            string                 `json:"code" db:"code"` // 6-8 символов уникальный код
	Description     string                 `json:"description" db:"description"`
	GameType        string                 `json:"game_type" db:"game_type"`
	Status          TournamentStatus       `json:"status" db:"status"`
	MaxParticipants *int                   `json:"max_participants,omitempty" db:"max_participants"`
	MaxTeamSize     int                    `json:"max_team_size" db:"max_team_size"`
	IsPermanent     bool                   `json:"is_permanent" db:"is_permanent"`
	CreatorID       *uuid.UUID             `json:"creator_id,omitempty" db:"creator_id"`
	StartTime       *time.Time             `json:"start_time,omitempty" db:"start_time"`
	EndTime         *time.Time             `json:"end_time,omitempty" db:"end_time"`
	Metadata        map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	Version         int                    `json:"version" db:"version"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// TournamentWithGames - турнир с играми для API ответов
type TournamentWithGames struct {
	Tournament
	Games []Game `json:"games"`
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

// MatchRound представляет группу матчей одного раунда
type MatchRound struct {
	RoundNumber    int       `json:"round_number"`
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

// LeaderboardEntry - запись в таблице лидеров
type LeaderboardEntry struct {
	Rank        int        `json:"rank" db:"rank"`
	ProgramID   uuid.UUID  `json:"program_id" db:"program_id"`
	ProgramName string     `json:"program_name" db:"program_name"`
	TeamID      *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	TeamName    *string    `json:"team_name,omitempty" db:"team_name"`
	Rating      int        `json:"rating" db:"rating"`
	Wins        int        `json:"wins" db:"wins"`
	Losses      int        `json:"losses" db:"losses"`
	Draws       int        `json:"draws" db:"draws"`
	TotalGames  int        `json:"total_games" db:"total_games"`
}

// TeamLeaderboardEntry - запись в таблице лидеров для команд
type TeamLeaderboardEntry struct {
	Rank       int       `json:"rank"`
	TeamID     uuid.UUID `json:"team_id"`
	TeamName   string    `json:"team_name"`
	TotalScore int       `json:"total_score"` // Сумма позиций по всем играм
	GameScores []struct {
		GameID   uuid.UUID `json:"game_id"`
		GameName string    `json:"game_name"`
		Rating   int       `json:"rating"`
		Position int       `json:"position"`
	} `json:"game_scores"`
}

// CrossGameLeaderboardEntry - кросс-игровой рейтинг (команда - рейтинг по каждой игре - позиция)
type CrossGameLeaderboardEntry struct {
	Rank        int                       `json:"rank"`
	TeamID      *uuid.UUID                `json:"team_id,omitempty"`
	TeamName    string                    `json:"team_name"`
	ProgramID   uuid.UUID                 `json:"program_id"`
	ProgramName string                    `json:"program_name"`
	GameRatings map[string]GameRatingInfo `json:"game_ratings"` // game_id -> rating info
	TotalRating int                       `json:"total_rating"`
	TotalWins   int                       `json:"total_wins"`
	TotalLosses int                       `json:"total_losses"`
	TotalGames  int                       `json:"total_games"`
}

// GameRatingInfo - информация о рейтинге в конкретной игре
type GameRatingInfo struct {
	GameID     uuid.UUID `json:"game_id"`
	GameName   string    `json:"game_name"`
	Rating     int       `json:"rating"`
	Wins       int       `json:"wins"`
	Losses     int       `json:"losses"`
	Draws      int       `json:"draws"`
	TotalGames int       `json:"total_games"`
}

// GameFilter - фильтр для списка игр
type GameFilter struct {
	Name   string
	Limit  int
	Offset int
}

// TeamFilter - фильтр для списка команд
type TeamFilter struct {
	TournamentID *uuid.UUID
	LeaderID     *uuid.UUID
	Limit        int
	Offset       int
}
