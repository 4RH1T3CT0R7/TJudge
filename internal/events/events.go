package events

import (
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/google/uuid"
)

// доменные события. поле Version - версия схемы, ставится 1; пригодится если формат
// сообщения в редисе когда-нибудь поменяется и надо будет различать старое/новое

type TournamentCreated struct {
	Version    int
	Tournament *models.Tournament
}

type TournamentStarted struct {
	Version      int
	TournamentID uuid.UUID
	Status       models.TournamentStatus
	StartTime    *time.Time
}

type TournamentCompleted struct {
	Version      int
	TournamentID uuid.UUID
	Status       models.TournamentStatus
	EndTime      *time.Time
}

type TournamentDeleted struct {
	Version      int
	TournamentID uuid.UUID
}

// ProgramCompiled - асинхронная компиляция загруженной программы завершилась (успешно или нет).
// тащит с собой всё что нужно вебсокету, чтобы не лезть лишний раз в базу
type ProgramCompiled struct {
	Version      int
	TournamentID uuid.UUID
	ProgramID    uuid.UUID
	TeamID       uuid.UUID
	Status       string  // models.ProgramStatus: ready | failed
	ErrorMessage *string // компиляционная ошибка при status=failed
}

// GameRoundReset - раунд игры сброшен (матчи удалены, рейтинги откачены к 1500)
type GameRoundReset struct {
	Version      int
	TournamentID uuid.UUID
	GameID       uuid.UUID
}

// MatchResultProcessed - рейтинги ело после матча пересчитаны
type MatchResultProcessed struct {
	Version      int
	TournamentID uuid.UUID
	MatchID      uuid.UUID
	Program1ID   uuid.UUID
	Program2ID   uuid.UUID
	NewRating1   int
	NewRating2   int
	Winner       int
}
