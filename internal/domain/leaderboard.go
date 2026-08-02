package domain

import "github.com/google/uuid"

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

type TeamLeaderboardEntry struct {
	Rank       int       `json:"rank"`
	TeamID     uuid.UUID `json:"team_id"`
	TeamName   string    `json:"team_name"`
	TotalScore int       `json:"total_score"` // сумма позиций по всем играм
	GameScores []struct {
		GameID   uuid.UUID `json:"game_id"`
		GameName string    `json:"game_name"`
		Rating   int       `json:"rating"`
		Position int       `json:"position"`
	} `json:"game_scores"`
}

// кросс-игровой рейтинг: команда и её рейтинг/позиция по каждой игре
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

type GameRatingInfo struct {
	GameID     uuid.UUID `json:"game_id"`
	GameName   string    `json:"game_name"`
	Rating     int       `json:"rating"`
	Wins       int       `json:"wins"`
	Losses     int       `json:"losses"`
	Draws      int       `json:"draws"`
	TotalGames int       `json:"total_games"`
}

// личные встречи пары команд в одной игре турнира.
// обе ориентации (AB и BA) уже слиты: wins это победы team_id
// над opponent_id, неважно кто ходил первым
type HeadToHeadCell struct {
	TeamID       uuid.UUID `json:"team_id" db:"team_id"`
	TeamName     string    `json:"team_name" db:"team_name"`
	OpponentID   uuid.UUID `json:"opponent_id" db:"opponent_id"`
	OpponentName string    `json:"opponent_name" db:"opponent_name"`
	Wins         int       `json:"wins" db:"wins"`
	Losses       int       `json:"losses" db:"losses"`
	Draws        int       `json:"draws" db:"draws"`
	ScoreFor     int       `json:"score_for" db:"score_for"`
	ScoreAgainst int       `json:"score_against" db:"score_against"`
}
