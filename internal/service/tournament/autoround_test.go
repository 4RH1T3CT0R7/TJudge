package tournament

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// processGame: проверки идут по порядку (активные матчи, cooldown, новые программы),
// раунд запускается только если все три пропустили
func TestAutoRoundScheduler_processGame(t *testing.T) {
	recent := time.Now().Add(-10 * time.Second)
	old := time.Now().Add(-time.Hour)

	tests := []struct {
		name       string
		lastRunAt  *time.Time
		hasActive  bool
		activeErr  error
		hasNew     bool
		wantNewChk bool // дошло ли до HasNewProgramsSince
		wantRun    bool // запущен ли раунд
	}{
		{name: "active_matches_wait", hasActive: true},
		{name: "active_check_error", activeErr: errors.New("db down")},
		{name: "cooldown_not_elapsed", lastRunAt: &recent},
		{name: "no_new_programs_after_last_run", lastRunAt: &old, wantNewChk: true},
		{name: "new_programs_after_last_run", lastRunAt: &old, hasNew: true, wantNewChk: true, wantRun: true},
		{name: "first_run_without_new_programs", wantNewChk: true, wantRun: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss, tournamentRepo, matchRepo, queueManager, distLock, gameRepo := newTestSchedulingService(t)
			log, _ := logger.New("error", "json")
			s := NewAutoRoundScheduler(ss, gameRepo, log, time.Second)
			ctx := context.Background()

			g := &models.AutoRoundGameInfo{
				TournamentID:    uuid.New(),
				GameID:          uuid.New(),
				GameType:        "dilemma",
				IntervalSeconds: 60,
				LastRunAt:       tt.lastRunAt,
			}

			gameRepo.On("HasActiveMatchesForGame", ctx, g.TournamentID, g.GameType).Return(tt.hasActive, tt.activeErr)
			if tt.wantNewChk {
				gameRepo.On("HasNewProgramsSince", ctx, g.TournamentID, g.GameType, mock.AnythingOfType("time.Time")).Return(tt.hasNew, nil)
			}
			if tt.wantRun {
				// pending уже есть - RunGameMatches просто ставит их в очередь
				pending := []*models.Match{{ID: uuid.New(), TournamentID: g.TournamentID, GameType: g.GameType}}
				distLock.On("WithLock", anyLock()...).Return(nil)
				tournamentRepo.On("GetByID", ctx, g.TournamentID).Return(activeTournament(g.TournamentID), nil)
				matchRepo.On("GetPendingByTournamentAndGame", ctx, g.TournamentID, g.GameType).Return(pending, nil)
				queueManager.On("EnqueueBatch", ctx, pending).Return(nil)
				gameRepo.On("UpdateAutoRoundLastRun", ctx, g.TournamentID, g.GameID).Return(nil)
			}

			s.processGame(ctx, g)

			gameRepo.AssertExpectations(t)
			matchRepo.AssertExpectations(t)
			queueManager.AssertExpectations(t)
			if !tt.wantNewChk {
				gameRepo.AssertNotCalled(t, "HasNewProgramsSince", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			if !tt.wantRun {
				distLock.AssertNotCalled(t, "WithLock", anyLock()...)
				gameRepo.AssertNotCalled(t, "UpdateAutoRoundLastRun", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

// ошибка RunGameMatches: время последнего запуска не сдвигается, иначе раунд
// потерялся бы до следующего cooldown
func TestAutoRoundScheduler_processGame_RunErrorKeepsLastRun(t *testing.T) {
	ss, tournamentRepo, matchRepo, _, distLock, gameRepo := newTestSchedulingService(t)
	log, _ := logger.New("error", "json")
	s := NewAutoRoundScheduler(ss, gameRepo, log, time.Second)
	ctx := context.Background()

	g := &models.AutoRoundGameInfo{TournamentID: uuid.New(), GameID: uuid.New(), GameType: "dilemma", IntervalSeconds: 60}

	gameRepo.On("HasActiveMatchesForGame", ctx, g.TournamentID, g.GameType).Return(false, nil)
	gameRepo.On("HasNewProgramsSince", ctx, g.TournamentID, g.GameType, time.Time{}).Return(true, nil)
	distLock.On("WithLock", anyLock()...).Return(nil)
	tournamentRepo.On("GetByID", ctx, g.TournamentID).Return(activeTournament(g.TournamentID), nil)
	matchRepo.On("GetPendingByTournamentAndGame", ctx, g.TournamentID, g.GameType).Return(nil, errors.New("db down"))

	s.processGame(ctx, g)

	gameRepo.AssertNotCalled(t, "UpdateAutoRoundLastRun", mock.Anything, mock.Anything, mock.Anything)
}

// tick обходит все игры с авто-раундом, Stop идемпотентен и гасит run
func TestAutoRoundScheduler_TickAndStop(t *testing.T) {
	ss, _, _, _, _, gameRepo := newTestSchedulingService(t)
	log, _ := logger.New("error", "json")
	s := NewAutoRoundScheduler(ss, gameRepo, log, time.Hour)
	ctx := context.Background()

	games := []*models.AutoRoundGameInfo{
		{TournamentID: uuid.New(), GameType: "dilemma"},
		{TournamentID: uuid.New(), GameType: "tug_of_war"},
	}
	gameRepo.On("GetAutoRoundEnabledGames", ctx).Return(games, nil)
	for _, g := range games {
		gameRepo.On("HasActiveMatchesForGame", ctx, g.TournamentID, g.GameType).Return(true, nil).Once()
	}

	s.tick(ctx)
	gameRepo.AssertExpectations(t)

	done := make(chan struct{})
	go func() {
		s.run(ctx)
		close(done)
	}()
	s.Stop()
	s.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run не завершился после Stop")
	}
}

func activeTournament(id uuid.UUID) *models.Tournament {
	return &models.Tournament{ID: id, Status: models.TournamentActive}
}
