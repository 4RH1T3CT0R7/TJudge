package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
)

// TournamentCacheWriter is the subset of tournament cache used by event handlers.
type TournamentCacheWriter interface {
	Set(ctx context.Context, tournament *domain.Tournament) error
	Invalidate(ctx context.Context, tournamentID uuid.UUID) error
}

// LeaderboardCacheWriter is the subset of leaderboard cache used by event handlers.
type LeaderboardCacheWriter interface {
	UpdateRating(ctx context.Context, tournamentID, programID uuid.UUID, rating int) error
	Clear(ctx context.Context, tournamentID uuid.UUID) error
	InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error
}

// TournamentCacheHandler handles tournament cache invalidation in response to events.
type TournamentCacheHandler struct {
	tournamentCache  TournamentCacheWriter
	leaderboardCache LeaderboardCacheWriter
	log              *logger.Logger
}

// NewTournamentCacheHandler creates a handler that manages tournament cache side-effects.
func NewTournamentCacheHandler(tc TournamentCacheWriter, lc LeaderboardCacheWriter, log *logger.Logger) *TournamentCacheHandler {
	return &TournamentCacheHandler{
		tournamentCache:  tc,
		leaderboardCache: lc,
		log:              log,
	}
}

func (h *TournamentCacheHandler) Handle(ctx context.Context, event any) error {
	switch e := event.(type) {
	case events.TournamentCreated:
		return h.tournamentCache.Set(ctx, e.Tournament)

	case events.TournamentStarted:
		return h.tournamentCache.Invalidate(ctx, e.TournamentID)

	case events.TournamentCompleted:
		return h.tournamentCache.Invalidate(ctx, e.TournamentID)

	case events.TournamentDeleted:
		return errors.Join(
			h.tournamentCache.Invalidate(ctx, e.TournamentID),
			h.leaderboardCache.Clear(ctx, e.TournamentID),
		)

	case events.ParticipantJoined:
		return h.tournamentCache.Invalidate(ctx, e.TournamentID)

	case events.GameRoundReset:
		return errors.Join(
			h.tournamentCache.Invalidate(ctx, e.TournamentID),
			h.leaderboardCache.Clear(ctx, e.TournamentID),
		)

	default:
		return fmt.Errorf("TournamentCacheHandler: unexpected event type %T", event)
	}
}

// LeaderboardCacheHandler handles leaderboard cache updates in response to events.
type LeaderboardCacheHandler struct {
	cache LeaderboardCacheWriter
	log   *logger.Logger
}

// NewLeaderboardCacheHandler creates a handler that manages leaderboard cache side-effects.
func NewLeaderboardCacheHandler(cache LeaderboardCacheWriter, log *logger.Logger) *LeaderboardCacheHandler {
	return &LeaderboardCacheHandler{cache: cache, log: log}
}

func (h *LeaderboardCacheHandler) Handle(ctx context.Context, event any) error {
	switch e := event.(type) {
	case events.ParticipantJoined:
		return errors.Join(
			h.cache.UpdateRating(ctx, e.TournamentID, e.ProgramID, e.InitialRating),
			h.cache.InvalidateFullLeaderboard(ctx, e.TournamentID),
		)

	case events.MatchResultProcessed:
		return errors.Join(
			h.cache.UpdateRating(ctx, e.TournamentID, e.Program1ID, e.NewRating1),
			h.cache.UpdateRating(ctx, e.TournamentID, e.Program2ID, e.NewRating2),
			h.cache.InvalidateFullLeaderboard(ctx, e.TournamentID),
		)

	case events.GameRoundReset:
		return h.cache.Clear(ctx, e.TournamentID)

	default:
		return fmt.Errorf("LeaderboardCacheHandler: unexpected event type %T", event)
	}
}
