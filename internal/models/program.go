package models

import (
	"time"

	"github.com/google/uuid"
)

// статус программы
type ProgramStatus string

const (
	// в очереди на компиляцию или уже компилится в песочнице воркера
	ProgramCompiling ProgramStatus = "compiling"
	// готова к матчам
	ProgramReady ProgramStatus = "ready"
	// компиляция или проверка синтаксиса упала, подробности елси есть в ErrorMessage
	ProgramFailed ProgramStatus = "failed"
)

// итог самопроверки: матча свежесобранной программы против эталонного бота игры
type CheckStatus string

const (
	CheckPending CheckStatus = "pending"
	CheckOK      CheckStatus = "ok"
	// программа упала или сходила не по правилам, подробности в CheckMessage
	CheckFailed CheckStatus = "failed"
)

type Program struct {
	ID           uuid.UUID     `json:"id" db:"id"`
	UserID       uuid.UUID     `json:"user_id" db:"user_id"`
	Name         string        `json:"name" db:"name"`
	GameType     string        `json:"game_type" db:"game_type"`
	CodePath     string        `json:"-" db:"code_path"`
	Language     string        `json:"language" db:"language"`
	Status       ProgramStatus `json:"status" db:"status"`
	TeamID       *uuid.UUID    `json:"team_id,omitempty" db:"team_id"`
	TournamentID *uuid.UUID    `json:"tournament_id,omitempty" db:"tournament_id"`
	GameID       *uuid.UUID    `json:"game_id,omitempty" db:"game_id"`
	FilePath     *string       `json:"-" db:"file_path"`
	ErrorMessage *string       `json:"error_message,omitempty" db:"error_message"`
	Version      int           `json:"version" db:"version"`
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at" db:"updated_at"`

	// nil - самопроверки не было (программа старше неё или сбой окружения)
	CheckStatus  *CheckStatus `json:"check_status,omitempty" db:"check_status"`
	CheckMessage *string      `json:"check_message,omitempty" db:"check_message"`
	CheckedAt    *time.Time   `json:"checked_at,omitempty" db:"checked_at"`
}
