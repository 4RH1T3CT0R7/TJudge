package domain

import (
	"time"

	"github.com/google/uuid"
)

type Team struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	TournamentID   uuid.UUID  `json:"tournament_id" db:"tournament_id"`
	Name           string     `json:"name" db:"name"`
	Code           string     `json:"code" db:"code"` // уникальный код, 6-8 символов
	LeaderID       uuid.UUID  `json:"leader_id" db:"leader_id"`
	IsDisqualified bool       `json:"is_disqualified" db:"is_disqualified"`
	DisqualifiedAt *time.Time `json:"disqualified_at,omitempty" db:"disqualified_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type TeamMember struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TeamID   uuid.UUID `json:"team_id" db:"team_id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

// команда вместе с участниками, отдаём в апи
type TeamWithMembers struct {
	Team
	Members []User `json:"members"`
}

type TeamFilter struct {
	TournamentID *uuid.UUID
	LeaderID     *uuid.UUID
	Limit        int
	Offset       int
}
