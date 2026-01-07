//go:build benchmark
// +build benchmark

package benchmark

import (
	"fmt"
	"strings"
	"time"
)

// BenchmarkStandard defines expected performance standards for a benchmark
type BenchmarkStandard struct {
	Name            string
	Description     string
	ExpectedNsOp    int64         // Expected nanoseconds per operation
	ExpectedBytesOp int64         // Expected bytes allocated per operation
	MaxLatency      time.Duration // Maximum acceptable latency
	Category        string        // API, Worker, Queue, DB
}

// BenchmarkResult holds parsed benchmark result
type BenchmarkResult struct {
	Name       string
	NsOp       int64
	BytesOp    int64
	AllocsOp   int64
	Iterations int64
}

// Rating represents performance rating
type Rating string

const (
	RatingExcellent  Rating = "EXCELLENT"
	RatingGood       Rating = "GOOD"
	RatingAcceptable Rating = "ACCEPTABLE"
	RatingPoor       Rating = "POOR"
	RatingCritical   Rating = "CRITICAL"
)

// Standards defines expected performance for each benchmark
var Standards = map[string]BenchmarkStandard{
	// API Benchmarks
	"BenchmarkHealthEndpoint": {
		Name:         "Health Endpoint",
		Description:  "Basic health check endpoint",
		ExpectedNsOp: 50_000, // 50µs
		Category:     "API",
	},
	"BenchmarkAuthLogin": {
		Name:         "Auth Login",
		Description:  "User authentication with password hashing",
		ExpectedNsOp: 100_000_000, // 100ms (bcrypt is intentionally slow)
		Category:     "API",
	},
	"BenchmarkTournamentsList": {
		Name:         "Tournaments List",
		Description:  "List tournaments with pagination",
		ExpectedNsOp: 5_000_000, // 5ms
		Category:     "API",
	},
	"BenchmarkTournamentGet": {
		Name:         "Tournament Get",
		Description:  "Fetch single tournament by ID",
		ExpectedNsOp: 2_000_000, // 2ms
		Category:     "API",
	},
	"BenchmarkLeaderboard": {
		Name:         "Leaderboard",
		Description:  "Fetch tournament leaderboard",
		ExpectedNsOp: 10_000_000, // 10ms
		Category:     "API",
	},
	"BenchmarkAuthMiddleware": {
		Name:         "Auth Middleware",
		Description:  "JWT token validation middleware",
		ExpectedNsOp: 500_000, // 500µs
		Category:     "API",
	},
	"BenchmarkConcurrentRequests": {
		Name:         "Concurrent Requests",
		Description:  "Mixed endpoint requests under load",
		ExpectedNsOp: 1_000_000, // 1ms
		Category:     "API",
	},
	"BenchmarkJSONParsing": {
		Name:         "JSON Parsing",
		Description:  "Tournament JSON serialization/deserialization",
		ExpectedNsOp: 10_000, // 10µs
		Category:     "API",
	},

	// Worker Benchmarks
	"BenchmarkWorkerPool_ThroughputSmall": {
		Name:         "Worker Pool Small",
		Description:  "2-4 workers processing 100 matches",
		ExpectedNsOp: 100_000_000, // 100ms total
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ThroughputMedium": {
		Name:         "Worker Pool Medium",
		Description:  "4-8 workers processing 500 matches",
		ExpectedNsOp: 300_000_000, // 300ms total
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ThroughputLarge": {
		Name:         "Worker Pool Large",
		Description:  "8-16 workers processing 1000 matches",
		ExpectedNsOp: 500_000_000, // 500ms total
		Category:     "Worker",
	},
	"BenchmarkMatchCreation": {
		Name:         "Match Creation",
		Description:  "Create match object with UUID",
		ExpectedNsOp: 5_000, // 5µs
		Category:     "Worker",
	},
	"BenchmarkUUIDGeneration": {
		Name:         "UUID Generation",
		Description:  "Generate UUID v4",
		ExpectedNsOp: 1_000, // 1µs
		Category:     "Worker",
	},

	// Queue Benchmarks
	"BenchmarkQueueEnqueue": {
		Name:         "Queue Enqueue",
		Description:  "Add match to Redis queue",
		ExpectedNsOp: 500_000, // 500µs
		Category:     "Queue",
	},
	"BenchmarkQueueEnqueueParallel": {
		Name:         "Queue Enqueue Parallel",
		Description:  "Parallel enqueue operations",
		ExpectedNsOp: 200_000, // 200µs per op
		Category:     "Queue",
	},
	"BenchmarkQueueDequeue": {
		Name:         "Queue Dequeue",
		Description:  "Get match from Redis queue",
		ExpectedNsOp: 500_000, // 500µs
		Category:     "Queue",
	},
	"BenchmarkQueueEnqueueDequeue": {
		Name:         "Queue Enqueue+Dequeue",
		Description:  "Combined enqueue and dequeue",
		ExpectedNsOp: 1_000_000, // 1ms
		Category:     "Queue",
	},
	"BenchmarkQueueSize": {
		Name:         "Queue Size",
		Description:  "Get total queue size",
		ExpectedNsOp: 100_000, // 100µs
		Category:     "Queue",
	},
	"BenchmarkQueuePriorities": {
		Name:         "Queue Priorities",
		Description:  "Priority queue operations",
		ExpectedNsOp: 500_000, // 500µs
		Category:     "Queue",
	},

	// Worker Pool Benchmarks
	"BenchmarkWorkerPool_ProcessingLatency": {
		Name:         "Processing Latency",
		Description:  "10ms simulated processing latency",
		ExpectedNsOp: 200_000_000, // 200ms (includes scaling overhead)
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ScaleUp": {
		Name:         "Worker Autoscaling",
		Description:  "Autoscaling 1->16 workers with 200 matches",
		ExpectedNsOp: 1_500_000_000, // 1.5s (200 matches, 5ms each, scaling overhead)
		Category:     "Worker",
	},

	// Database Benchmarks
	"BenchmarkDBHealth": {
		Name:         "DB Health",
		Description:  "Database connection health check",
		ExpectedNsOp: 500_000, // 500µs
		Category:     "DB",
	},
	"BenchmarkDBHealthParallel": {
		Name:         "DB Health Parallel",
		Description:  "Parallel health checks",
		ExpectedNsOp: 200_000, // 200µs per op
		Category:     "DB",
	},
	"BenchmarkUserGetByID": {
		Name:         "User Get By ID",
		Description:  "Fetch user by UUID",
		ExpectedNsOp: 1_000_000, // 1ms
		Category:     "DB",
	},
	"BenchmarkTournamentList": {
		Name:         "Tournament List (DB)",
		Description:  "List tournaments from database",
		ExpectedNsOp: 2_000_000, // 2ms
		Category:     "DB",
	},
	"BenchmarkTournamentListParallel": {
		Name:         "Tournament List Parallel",
		Description:  "Parallel tournament listing",
		ExpectedNsOp: 1_000_000, // 1ms per op
		Category:     "DB",
	},
	"BenchmarkMatchList": {
		Name:         "Match List",
		Description:  "List matches by tournament",
		ExpectedNsOp: 3_000_000, // 3ms
		Category:     "DB",
	},
	"BenchmarkMatchCreate": {
		Name:         "Match Create",
		Description:  "Insert match into database",
		ExpectedNsOp: 2_000_000, // 2ms
		Category:     "DB",
	},
	"BenchmarkLeaderboardGet": {
		Name:         "Leaderboard Get (DB)",
		Description:  "Fetch leaderboard from materialized view",
		ExpectedNsOp: 5_000_000, // 5ms
		Category:     "DB",
	},
}

// InterpretResult interprets a benchmark result against standards
func InterpretResult(result BenchmarkResult) (rating Rating, analysis string) {
	standard, exists := Standards[result.Name]
	if !exists {
		return RatingAcceptable, fmt.Sprintf("No standard defined for %s", result.Name)
	}

	ratio := float64(result.NsOp) / float64(standard.ExpectedNsOp)

	switch {
	case ratio <= 0.5:
		rating = RatingExcellent
	case ratio <= 1.0:
		rating = RatingGood
	case ratio <= 2.0:
		rating = RatingAcceptable
	case ratio <= 5.0:
		rating = RatingPoor
	default:
		rating = RatingCritical
	}

	analysis = fmt.Sprintf(
		"[%s] %s\n"+
			"  Category:    %s\n"+
			"  Description: %s\n"+
			"  Actual:      %s/op\n"+
			"  Expected:    %s/op\n"+
			"  Ratio:       %.2fx\n"+
			"  Rating:      %s",
		result.Name,
		standard.Name,
		standard.Category,
		standard.Description,
		formatDuration(result.NsOp),
		formatDuration(standard.ExpectedNsOp),
		ratio,
		rating,
	)

	if result.BytesOp > 0 {
		analysis += fmt.Sprintf("\n  Memory:      %d B/op (%d allocs/op)", result.BytesOp, result.AllocsOp)
	}

	return rating, analysis
}

// PrintSummary prints a summary of benchmark results
func PrintSummary(results []BenchmarkResult) {
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("  BENCHMARK RESULTS INTERPRETATION")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println()

	categories := map[string][]BenchmarkResult{
		"API":    {},
		"Worker": {},
		"Queue":  {},
		"DB":     {},
	}

	// Group results by category
	for _, result := range results {
		if standard, exists := Standards[result.Name]; exists {
			categories[standard.Category] = append(categories[standard.Category], result)
		}
	}

	ratings := map[Rating]int{}

	// Print by category
	for _, category := range []string{"API", "Worker", "Queue", "DB"} {
		catResults := categories[category]
		if len(catResults) == 0 {
			continue
		}

		fmt.Printf("\n### %s Benchmarks ###\n\n", category)

		for _, result := range catResults {
			rating, analysis := InterpretResult(result)
			ratings[rating]++
			fmt.Println(analysis)
			fmt.Println()
		}
	}

	// Print summary
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("  SUMMARY")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Printf("\n  Total benchmarks: %d\n", len(results))
	fmt.Printf("  Excellent: %d\n", ratings[RatingExcellent])
	fmt.Printf("  Good:      %d\n", ratings[RatingGood])
	fmt.Printf("  Acceptable: %d\n", ratings[RatingAcceptable])
	fmt.Printf("  Poor:      %d\n", ratings[RatingPoor])
	fmt.Printf("  Critical:  %d\n", ratings[RatingCritical])
	fmt.Println()

	// Recommendations
	if ratings[RatingCritical] > 0 || ratings[RatingPoor] > 0 {
		fmt.Println("  RECOMMENDATIONS:")
		if ratings[RatingCritical] > 0 {
			fmt.Println("  - CRITICAL: Some benchmarks are >5x slower than expected. Immediate investigation needed.")
		}
		if ratings[RatingPoor] > 0 {
			fmt.Println("  - WARNING: Some benchmarks are 2-5x slower than expected. Consider optimization.")
		}
	} else if ratings[RatingAcceptable] > len(results)/2 {
		fmt.Println("  RECOMMENDATIONS:")
		fmt.Println("  - Performance is acceptable but could be improved in some areas.")
	} else {
		fmt.Println("  STATUS: Performance is within expected parameters.")
	}

	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println()

	// Print reference table
	fmt.Println("  REFERENCE: Expected Performance Standards")
	fmt.Println("-" + strings.Repeat("-", 79))
	fmt.Printf("  %-40s %15s %s\n", "Benchmark", "Expected", "Category")
	fmt.Println("-" + strings.Repeat("-", 79))

	for name, std := range Standards {
		fmt.Printf("  %-40s %15s %s\n", name, formatDuration(std.ExpectedNsOp), std.Category)
	}
	fmt.Println()
}

// formatDuration formats nanoseconds as human-readable duration
func formatDuration(ns int64) string {
	d := time.Duration(ns) * time.Nanosecond
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	case d >= time.Microsecond:
		return fmt.Sprintf("%.2fµs", float64(d)/float64(time.Microsecond))
	default:
		return fmt.Sprintf("%dns", ns)
	}
}
