//go:build benchmark
// +build benchmark

package benchmark

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
)

var (
	dbOnce         sync.Once
	database       *db.Database
	userRepo       *db.UserRepository
	tournamentRepo *db.TournamentRepository
	matchRepo      *db.MatchRepository
	programRepo    *db.ProgramRepository
)

func setupDatabase(b *testing.B) {
	dbOnce.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}

		log, err := logger.New("error", "json")
		if err != nil {
			b.Fatalf("Failed to create logger: %v", err)
		}

		m := metrics.New()

		database, err = db.New(&cfg.Database, log, m)
		if err != nil {
			b.Fatalf("Failed to connect to database: %v", err)
		}

		userRepo = db.NewUserRepository(database)
		tournamentRepo = db.NewTournamentRepository(database)
		matchRepo = db.NewMatchRepository(database)
		programRepo = db.NewProgramRepository(database)
	})
}

// BenchmarkDBHealth measures database health check performance
func BenchmarkDBHealth(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		database.Health(ctx)
	}
}

// BenchmarkDBHealthParallel measures parallel health check performance
func BenchmarkDBHealthParallel(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			database.Health(ctx)
		}
	})
}

// BenchmarkUserGetByID measures user lookup performance
func BenchmarkUserGetByID(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userRepo.GetByID(ctx, userID)
	}
}

// BenchmarkUserGetByUsername measures username lookup performance
func BenchmarkUserGetByUsername(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userRepo.GetByUsername(ctx, "nonexistent_user")
	}
}

// BenchmarkTournamentList measures tournament listing performance
func BenchmarkTournamentList(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	filter := domain.TournamentFilter{
		Limit:  20,
		Offset: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tournamentRepo.List(ctx, filter)
	}
}

// BenchmarkTournamentListParallel measures parallel tournament listing
func BenchmarkTournamentListParallel(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	filter := domain.TournamentFilter{
		Limit:  20,
		Offset: 0,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tournamentRepo.List(ctx, filter)
		}
	})
}

// BenchmarkTournamentGetByID measures single tournament fetch
func BenchmarkTournamentGetByID(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	tournamentID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tournamentRepo.GetByID(ctx, tournamentID)
	}
}

// BenchmarkMatchList measures match listing performance
func BenchmarkMatchList(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	tournamentID := uuid.New()
	filter := domain.MatchFilter{
		TournamentID: &tournamentID,
		Limit:        50,
		Offset:       0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchRepo.List(ctx, filter)
	}
}

// BenchmarkMatchCreate measures match creation performance
func BenchmarkMatchCreate(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()

	// Get existing tournament and programs
	tournaments, err := tournamentRepo.List(ctx, domain.TournamentFilter{Limit: 1})
	if err != nil || len(tournaments) == 0 {
		b.Skip("No tournaments found for benchmark")
	}

	programs, err := programRepo.List(ctx, tournaments[0].ID)
	if err != nil || len(programs) < 2 {
		b.Skip("Not enough programs found for benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		match := &domain.Match{
			ID:           uuid.New(),
			TournamentID: tournaments[0].ID,
			Program1ID:   programs[0].ID,
			Program2ID:   programs[1].ID,
			GameType:     tournaments[0].GameType,
			Status:       domain.MatchPending,
			Priority:     domain.PriorityMedium,
			CreatedAt:    time.Now(),
		}
		matchRepo.Create(ctx, match)
	}
}

// BenchmarkLeaderboardGet measures leaderboard retrieval performance
func BenchmarkLeaderboardGet(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	tournamentID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchRepo.GetLeaderboard(ctx, tournamentID, 100)
	}
}

// BenchmarkLeaderboardGetParallel measures parallel leaderboard retrieval
func BenchmarkLeaderboardGetParallel(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	tournamentID := uuid.New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			matchRepo.GetLeaderboard(ctx, tournamentID, 100)
		}
	})
}

// BenchmarkProgramList measures program listing performance
func BenchmarkProgramList(b *testing.B) {
	setupDatabase(b)

	ctx := context.Background()
	tournamentID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		programRepo.List(ctx, tournamentID)
	}
}

// BenchmarkConnectionPoolStats measures connection pool stats retrieval
func BenchmarkConnectionPoolStats(b *testing.B) {
	setupDatabase(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		database.Stats()
	}
}
