package domain

import "github.com/google/uuid"

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
