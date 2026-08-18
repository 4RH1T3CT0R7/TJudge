package events

import (
	"context"
	"errors"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Notifier рассылает доменные события: обновляет кэши, шлёт вебсокет-сообщения,
// пробрасывает событие в редис для другого процесса. что именно происходит на каждое
// событие - зависит от того какие коллабораторы подсунули в SyncNotifier (в апи один
// набор, в воркере другой).
type Notifier interface {
	TournamentCreated(ctx context.Context, e TournamentCreated)
	TournamentStarted(ctx context.Context, e TournamentStarted)
	TournamentCompleted(ctx context.Context, e TournamentCompleted)
	TournamentDeleted(ctx context.Context, e TournamentDeleted)
	ParticipantJoined(ctx context.Context, e ParticipantJoined)
	MatchesCreated(ctx context.Context, e MatchesCreated)
	GameRoundReset(ctx context.Context, e GameRoundReset)
	MatchResultProcessed(ctx context.Context, e MatchResultProcessed)
	ProgramCompiled(ctx context.Context, e ProgramCompiled)
}

// TournamentCacheWriter - кусок кэша турниров, который нужен нотифаеру.
type TournamentCacheWriter interface {
	Set(ctx context.Context, tournament *models.Tournament) error
	Invalidate(ctx context.Context, tournamentID uuid.UUID) error
}

// LeaderboardCacheWriter - кусок кэша лидерборда для нотифаера.
type LeaderboardCacheWriter interface {
	UpdateRating(ctx context.Context, tournamentID, programID uuid.UUID, rating int) error
	UpdateRatingsBatch(ctx context.Context, updates []cache.RatingUpdate) error
	Clear(ctx context.Context, tournamentID uuid.UUID) error
	InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error
}

// Broadcaster шлёт сообщение вебсокет-клиентам турнира.
type Broadcaster interface {
	Broadcast(tournamentID uuid.UUID, messageType string, payload any)
}

// SyncNotifier синхронно применяет побочные эффекты события. коллабораторы опциональны:
// nil просто пропускается. так одна реализация покрывает три топологии (воркер, апи,
// мост из редиса) - разница только в том что передать в поля.
type SyncNotifier struct {
	TournamentCache TournamentCacheWriter  // в воркере nil - кэш турниров там не трогаем
	Leaderboard     LeaderboardCacheWriter // может быть nil
	Broadcaster     Broadcaster            // в воркере nil - вебсокета нет
	Redis           *RedisEventPublisher   // в апи nil - наружу не публикуем, уже пришло из редиса
	Log             *logger.Logger
}

func (n *SyncNotifier) TournamentCreated(ctx context.Context, e TournamentCreated) {
	if n.TournamentCache != nil {
		n.logErr("TournamentCreated", n.TournamentCache.Set(ctx, e.Tournament))
	}
}

func (n *SyncNotifier) TournamentStarted(ctx context.Context, e TournamentStarted) {
	if n.TournamentCache != nil {
		n.logErr("TournamentStarted", n.TournamentCache.Invalidate(ctx, e.TournamentID))
	}
	if n.Broadcaster != nil {
		n.Broadcaster.Broadcast(e.TournamentID, "tournament_update", map[string]any{
			"status":     e.Status,
			"start_time": e.StartTime,
		})
	}
}

func (n *SyncNotifier) TournamentCompleted(ctx context.Context, e TournamentCompleted) {
	if n.TournamentCache != nil {
		n.logErr("TournamentCompleted", n.TournamentCache.Invalidate(ctx, e.TournamentID))
	}
	if n.Broadcaster != nil {
		n.Broadcaster.Broadcast(e.TournamentID, "tournament_update", map[string]any{
			"status":   e.Status,
			"end_time": e.EndTime,
		})
	}
}

func (n *SyncNotifier) TournamentDeleted(ctx context.Context, e TournamentDeleted) {
	// турнир удалили - выкидываем и его кэш, и лидерборд
	if n.TournamentCache != nil {
		n.logErr("TournamentDeleted", n.TournamentCache.Invalidate(ctx, e.TournamentID))
	}
	if n.Leaderboard != nil {
		n.logErr("TournamentDeleted", n.Leaderboard.Clear(ctx, e.TournamentID))
	}
}

func (n *SyncNotifier) ParticipantJoined(ctx context.Context, e ParticipantJoined) {
	if n.TournamentCache != nil {
		n.logErr("ParticipantJoined", n.TournamentCache.Invalidate(ctx, e.TournamentID))
	}
	if n.Leaderboard != nil {
		n.logErr("ParticipantJoined", errors.Join(
			n.Leaderboard.UpdateRating(ctx, e.TournamentID, e.ProgramID, e.InitialRating),
			n.Leaderboard.InvalidateFullLeaderboard(ctx, e.TournamentID),
		))
	}
}

func (n *SyncNotifier) MatchesCreated(ctx context.Context, e MatchesCreated) {
	if n.Broadcaster != nil {
		n.Broadcaster.Broadcast(e.TournamentID, "matches_created", map[string]any{
			"program_id":    e.ProgramID.String(),
			"matches_count": e.MatchCount,
		})
	}
}

func (n *SyncNotifier) GameRoundReset(ctx context.Context, e GameRoundReset) {
	// раунд сбросили - инвалидируем турнир и целиком чистим лидерборд
	// (раньше Clear звался из двух хендлеров, но DEL идемпотентен так что хватает одного)
	if n.TournamentCache != nil {
		n.logErr("GameRoundReset", n.TournamentCache.Invalidate(ctx, e.TournamentID))
	}
	if n.Leaderboard != nil {
		n.logErr("GameRoundReset", n.Leaderboard.Clear(ctx, e.TournamentID))
	}
}

func (n *SyncNotifier) MatchResultProcessed(ctx context.Context, e MatchResultProcessed) {
	if n.Leaderboard != nil {
		// два ZADD одним пайплайном - один RTT вместо двух
		updates := []cache.RatingUpdate{
			{TournamentID: e.TournamentID, ProgramID: e.Program1ID, Rating: e.NewRating1},
			{TournamentID: e.TournamentID, ProgramID: e.Program2ID, Rating: e.NewRating2},
		}
		n.logErr("MatchResultProcessed", errors.Join(
			n.Leaderboard.UpdateRatingsBatch(ctx, updates),
			n.Leaderboard.InvalidateFullLeaderboard(ctx, e.TournamentID),
		))
	}
	if n.Redis != nil {
		n.logErr("MatchResultProcessed", n.Redis.Publish(ctx, "MatchResultProcessed", e))
	}
	if n.Broadcaster != nil {
		n.Broadcaster.Broadcast(e.TournamentID, "match_result", map[string]any{
			"match_id":    e.MatchID.String(),
			"program1_id": e.Program1ID.String(),
			"program2_id": e.Program2ID.String(),
			"new_rating1": e.NewRating1,
			"new_rating2": e.NewRating2,
			"winner":      e.Winner,
		})
	}
}

func (n *SyncNotifier) ProgramCompiled(ctx context.Context, e ProgramCompiled) {
	if n.Redis != nil {
		n.logErr("ProgramCompiled", n.Redis.Publish(ctx, "ProgramCompiled", e))
	}
	if n.Broadcaster != nil {
		n.Broadcaster.Broadcast(e.TournamentID, "program_update", map[string]any{
			"program_id":    e.ProgramID.String(),
			"team_id":       e.TeamID.String(),
			"status":        e.Status,
			"error_message": e.ErrorMessage,
		})
	}
}

// logErr логирует ошибку побочного эффекта и едет дальше - событие домена
// не должно падать из-за того что кэш или вебсокет чихнули
func (n *SyncNotifier) logErr(event string, err error) {
	if err == nil {
		return
	}
	n.Log.Error("Event handler error",
		zap.String("event_type", event),
		zap.Error(err),
	)
}

// NoopNotifier ничего не делает, удобно в тестах сервисов
type NoopNotifier struct{}

func (NoopNotifier) TournamentCreated(context.Context, TournamentCreated)       {}
func (NoopNotifier) TournamentStarted(context.Context, TournamentStarted)       {}
func (NoopNotifier) TournamentCompleted(context.Context, TournamentCompleted)   {}
func (NoopNotifier) TournamentDeleted(context.Context, TournamentDeleted)       {}
func (NoopNotifier) ParticipantJoined(context.Context, ParticipantJoined)       {}
func (NoopNotifier) MatchesCreated(context.Context, MatchesCreated)             {}
func (NoopNotifier) GameRoundReset(context.Context, GameRoundReset)             {}
func (NoopNotifier) MatchResultProcessed(context.Context, MatchResultProcessed) {}
func (NoopNotifier) ProgramCompiled(context.Context, ProgramCompiled)           {}
