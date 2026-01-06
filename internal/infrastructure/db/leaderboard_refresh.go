package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// LeaderboardRefresher периодически обновляет materialized views для leaderboard
type LeaderboardRefresher struct {
	db       *DB
	interval time.Duration
	log      *logger.Logger
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewLeaderboardRefresher создаёт новый refresher
func NewLeaderboardRefresher(db *DB, interval time.Duration, log *logger.Logger) *LeaderboardRefresher {
	return &LeaderboardRefresher{
		db:       db,
		interval: interval,
		log:      log,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start запускает периодическое обновление leaderboards
func (r *LeaderboardRefresher) Start() {
	r.log.Info("Starting leaderboard refresher",
		zap.Duration("interval", r.interval),
	)

	go r.run()
}

// Stop останавливает refresher
func (r *LeaderboardRefresher) Stop() {
	r.log.Info("Stopping leaderboard refresher")
	close(r.stopCh)
	<-r.doneCh
	r.log.Info("Leaderboard refresher stopped")
}

// run основной цикл обновления
func (r *LeaderboardRefresher) run() {
	defer close(r.doneCh)

	// Сразу обновляем при старте
	r.refresh()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.refresh()
		case <-r.stopCh:
			return
		}
	}
}

// refresh обновляет materialized views
func (r *LeaderboardRefresher) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()

	// Вызываем функцию refresh_leaderboards(), которая обновляет оба view конкурентно
	_, err := r.db.ExecContext(ctx, "SELECT refresh_leaderboards()")
	if err != nil {
		// Проверяем, существует ли функция
		if err == sql.ErrNoRows || isUndefinedFunctionError(err) {
			r.log.Info("Leaderboard materialized views not yet created, skipping refresh")
			return
		}

		r.log.LogError("Failed to refresh leaderboards", err)
		return
	}

	duration := time.Since(startTime)
	r.log.Info("Leaderboard materialized views refreshed",
		zap.Duration("duration", duration),
	)
}

// RefreshNow выполняет немедленное обновление (для использования после важных событий)
func (r *LeaderboardRefresher) RefreshNow() {
	r.log.Info("Manual leaderboard refresh requested")
	r.refresh()
}

// isUndefinedFunctionError проверяет, является ли ошибка "undefined function"
func isUndefinedFunctionError(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL error code 42883 = undefined_function
	return err.Error() == "pq: function refresh_leaderboards() does not exist" ||
		err.Error() == "ERROR: function refresh_leaderboards() does not exist (SQLSTATE 42883)"
}
