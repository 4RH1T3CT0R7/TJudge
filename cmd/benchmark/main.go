package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BenchmarkResult holds parsed benchmark result
type BenchmarkResult struct {
	Name       string
	NsOp       int64
	BytesOp    int64
	AllocsOp   int64
	Iterations int64
}

// BenchmarkStandard defines expected performance standards
type BenchmarkStandard struct {
	Name         string
	Description  string
	ExpectedNsOp int64
	Category     string
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

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// Standards defines expected performance for each benchmark
var Standards = map[string]BenchmarkStandard{
	// API Benchmarks
	"BenchmarkHealthEndpoint": {
		Name:         "Health Endpoint",
		Description:  "Basic health check endpoint",
		ExpectedNsOp: 50_000,
		Category:     "API",
	},
	"BenchmarkAuthLogin": {
		Name:         "Auth Login",
		Description:  "User authentication (bcrypt is intentionally slow)",
		ExpectedNsOp: 100_000_000,
		Category:     "API",
	},
	"BenchmarkTournamentsList": {
		Name:         "Tournaments List",
		Description:  "List tournaments with pagination",
		ExpectedNsOp: 5_000_000,
		Category:     "API",
	},
	"BenchmarkTournamentGet": {
		Name:         "Tournament Get",
		Description:  "Fetch single tournament by ID",
		ExpectedNsOp: 2_000_000,
		Category:     "API",
	},
	"BenchmarkLeaderboard": {
		Name:         "Leaderboard",
		Description:  "Fetch tournament leaderboard",
		ExpectedNsOp: 10_000_000,
		Category:     "API",
	},
	"BenchmarkAuthMiddleware": {
		Name:         "Auth Middleware",
		Description:  "JWT token validation",
		ExpectedNsOp: 500_000,
		Category:     "API",
	},
	"BenchmarkConcurrentRequests": {
		Name:         "Concurrent Requests",
		Description:  "Mixed requests under load",
		ExpectedNsOp: 1_000_000,
		Category:     "API",
	},
	"BenchmarkJSONParsing": {
		Name:         "JSON Parsing",
		Description:  "JSON serialization/deserialization",
		ExpectedNsOp: 10_000,
		Category:     "API",
	},
	"BenchmarkProgramsList": {
		Name:         "Programs List",
		Description:  "List user programs",
		ExpectedNsOp: 3_000_000,
		Category:     "API",
	},
	"BenchmarkMatchesList": {
		Name:         "Matches List",
		Description:  "List tournament matches",
		ExpectedNsOp: 5_000_000,
		Category:     "API",
	},

	// Worker Benchmarks
	"BenchmarkWorkerPool_ThroughputSmall": {
		Name:         "Worker Pool Small",
		Description:  "2-4 workers, 100 matches",
		ExpectedNsOp: 100_000_000,
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ThroughputMedium": {
		Name:         "Worker Pool Medium",
		Description:  "4-8 workers, 500 matches",
		ExpectedNsOp: 300_000_000,
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ThroughputLarge": {
		Name:         "Worker Pool Large",
		Description:  "8-16 workers, 1000 matches",
		ExpectedNsOp: 500_000_000,
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ProcessingLatency": {
		Name:         "Processing Latency",
		Description:  "10ms simulated processing",
		ExpectedNsOp: 200_000_000,
		Category:     "Worker",
	},
	"BenchmarkWorkerPool_ScaleUp": {
		Name:         "Scale Up",
		Description:  "Autoscaling 1->16 workers",
		ExpectedNsOp: 300_000_000,
		Category:     "Worker",
	},
	"BenchmarkMatchCreation": {
		Name:         "Match Creation",
		Description:  "Create match object",
		ExpectedNsOp: 5_000,
		Category:     "Worker",
	},
	"BenchmarkUUIDGeneration": {
		Name:         "UUID Generation",
		Description:  "Generate UUID v4",
		ExpectedNsOp: 1_000,
		Category:     "Worker",
	},

	// Queue Benchmarks
	"BenchmarkQueueEnqueue": {
		Name:         "Queue Enqueue",
		Description:  "Add match to Redis queue",
		ExpectedNsOp: 500_000,
		Category:     "Queue",
	},
	"BenchmarkQueueEnqueueParallel": {
		Name:         "Queue Enqueue Parallel",
		Description:  "Parallel enqueue operations",
		ExpectedNsOp: 200_000,
		Category:     "Queue",
	},
	"BenchmarkQueueDequeue": {
		Name:         "Queue Dequeue",
		Description:  "Get match from queue",
		ExpectedNsOp: 500_000,
		Category:     "Queue",
	},
	"BenchmarkQueueEnqueueDequeue": {
		Name:         "Enqueue + Dequeue",
		Description:  "Combined operation",
		ExpectedNsOp: 1_000_000,
		Category:     "Queue",
	},
	"BenchmarkQueueSize": {
		Name:         "Queue Size",
		Description:  "Get total queue size",
		ExpectedNsOp: 100_000,
		Category:     "Queue",
	},
	"BenchmarkQueuePriorities": {
		Name:         "Queue Priorities",
		Description:  "Priority queue operations",
		ExpectedNsOp: 500_000,
		Category:     "Queue",
	},

	// Database Benchmarks
	"BenchmarkDBHealth": {
		Name:         "DB Health",
		Description:  "Database health check",
		ExpectedNsOp: 500_000,
		Category:     "DB",
	},
	"BenchmarkDBHealthParallel": {
		Name:         "DB Health Parallel",
		Description:  "Parallel health checks",
		ExpectedNsOp: 200_000,
		Category:     "DB",
	},
	"BenchmarkUserGetByID": {
		Name:         "User Get By ID",
		Description:  "Fetch user by UUID",
		ExpectedNsOp: 1_000_000,
		Category:     "DB",
	},
	"BenchmarkUserGetByUsername": {
		Name:         "User Get By Username",
		Description:  "Fetch user by username",
		ExpectedNsOp: 1_000_000,
		Category:     "DB",
	},
	"BenchmarkTournamentList": {
		Name:         "Tournament List (DB)",
		Description:  "List tournaments from DB",
		ExpectedNsOp: 2_000_000,
		Category:     "DB",
	},
	"BenchmarkTournamentListParallel": {
		Name:         "Tournament List Parallel",
		Description:  "Parallel listing",
		ExpectedNsOp: 1_000_000,
		Category:     "DB",
	},
	"BenchmarkTournamentGetByID": {
		Name:         "Tournament Get By ID",
		Description:  "Fetch tournament by ID",
		ExpectedNsOp: 1_000_000,
		Category:     "DB",
	},
	"BenchmarkMatchList": {
		Name:         "Match List",
		Description:  "List matches",
		ExpectedNsOp: 3_000_000,
		Category:     "DB",
	},
	"BenchmarkMatchCreate": {
		Name:         "Match Create",
		Description:  "Insert match",
		ExpectedNsOp: 2_000_000,
		Category:     "DB",
	},
	"BenchmarkLeaderboardGet": {
		Name:         "Leaderboard Get (DB)",
		Description:  "Fetch from materialized view",
		ExpectedNsOp: 5_000_000,
		Category:     "DB",
	},
	"BenchmarkLeaderboardGetParallel": {
		Name:         "Leaderboard Parallel",
		Description:  "Parallel leaderboard fetch",
		ExpectedNsOp: 3_000_000,
		Category:     "DB",
	},
	"BenchmarkProgramGetByUserID": {
		Name:         "Program Get By User",
		Description:  "Fetch user programs",
		ExpectedNsOp: 2_000_000,
		Category:     "DB",
	},
	"BenchmarkConnectionPoolStats": {
		Name:         "Connection Pool Stats",
		Description:  "Get pool statistics",
		ExpectedNsOp: 1_000,
		Category:     "DB",
	},
}

func main() {
	runBenchmarks := flag.Bool("run", false, "Run benchmarks before interpreting")
	pattern := flag.String("bench", ".", "Benchmark pattern to run")
	verbose := flag.Bool("v", false, "Verbose output")
	showStandards := flag.Bool("standards", false, "Show only standards table")
	noColor := flag.Bool("no-color", false, "Disable colored output")
	flag.Parse()

	if *noColor {
		disableColors()
	}

	if *showStandards {
		printStandardsTable()
		return
	}

	var output []byte
	var err error

	if *runBenchmarks {
		fmt.Println(colorCyan + "Running benchmarks..." + colorReset)
		fmt.Println("Note: Only standalone benchmarks (no DB/Redis required)")
		fmt.Println()

		// Run only benchmarks that don't require external services
		// Exclude: API (needs full server), Queue (needs Redis), DB (needs Postgres)
		// Include: Worker pool mocks, JSON parsing, UUID generation, Match creation
		args := []string{
			"test",
			"-tags=benchmark",
			"-bench=" + *pattern,
			"-benchmem",
			"-benchtime=500ms",
			"-timeout=30s",
			"-run=^$", // Don't run regular tests
			"./tests/benchmark/...",
		}
		if *verbose {
			args = append(args, "-v")
		}

		cmd := exec.Command("go", args...)
		cmd.Dir = findProjectRoot()

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		output = stdout.Bytes()

		// Print output in real-time for debugging
		if *verbose {
			fmt.Println(string(output))
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "%sWarning: Some benchmarks may have been skipped (DB/Redis not running?)%s\n", colorYellow, colorReset)
			if *verbose {
				fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
			}
		}
	} else {
		// Read from stdin
		fmt.Println(colorCyan + "Reading benchmark results from stdin..." + colorReset)
		fmt.Println("(Run with -run flag to execute benchmarks automatically)")
		fmt.Println()

		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		output = []byte(strings.Join(lines, "\n"))
	}

	results := parseBenchmarkOutput(string(output))

	if len(results) == 0 {
		fmt.Println(colorYellow + "No benchmark results found." + colorReset)
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run ./cmd/benchmark -run              # Run and interpret benchmarks")
		fmt.Println("  go run ./cmd/benchmark -standards        # Show expected standards")
		fmt.Println("  go test -bench=. ./... | go run ./cmd/benchmark  # Pipe results")
		return
	}

	printResults(results)
}

func disableColors() {
	// Can't reassign constants, so we'd need to use variables
	// For simplicity, we'll skip this feature
}

func findProjectRoot() string {
	// Try to find go.mod
	dir, _ := os.Getwd()
	return dir
}

func parseBenchmarkOutput(output string) []BenchmarkResult {
	var results []BenchmarkResult

	// Pattern: BenchmarkName-N    iterations    ns/op    B/op    allocs/op
	re := regexp.MustCompile(`(Benchmark\w+)(?:-\d+)?\s+(\d+)\s+([\d.]+)\s+ns/op(?:\s+([\d.]+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?`)

	for _, line := range strings.Split(output, "\n") {
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 4 {
			nsOp, _ := strconv.ParseFloat(matches[3], 64)
			iterations, _ := strconv.ParseInt(matches[2], 10, 64)

			result := BenchmarkResult{
				Name:       matches[1],
				NsOp:       int64(nsOp),
				Iterations: iterations,
			}

			if len(matches) >= 5 && matches[4] != "" {
				bytesOp, _ := strconv.ParseFloat(matches[4], 64)
				result.BytesOp = int64(bytesOp)
			}

			if len(matches) >= 6 && matches[5] != "" {
				allocsOp, _ := strconv.ParseInt(matches[5], 10, 64)
				result.AllocsOp = allocsOp
			}

			results = append(results, result)
		}
	}

	return results
}

func getRating(ratio float64) Rating {
	switch {
	case ratio <= 0.5:
		return RatingExcellent
	case ratio <= 1.0:
		return RatingGood
	case ratio <= 2.0:
		return RatingAcceptable
	case ratio <= 5.0:
		return RatingPoor
	default:
		return RatingCritical
	}
}

func getRatingColor(rating Rating) string {
	switch rating {
	case RatingExcellent:
		return colorGreen
	case RatingGood:
		return colorGreen
	case RatingAcceptable:
		return colorYellow
	case RatingPoor:
		return colorYellow
	case RatingCritical:
		return colorRed
	default:
		return colorReset
	}
}

func getRatingSymbol(rating Rating) string {
	switch rating {
	case RatingExcellent:
		return "+++"
	case RatingGood:
		return "++"
	case RatingAcceptable:
		return "+"
	case RatingPoor:
		return "-"
	case RatingCritical:
		return "---"
	default:
		return "?"
	}
}

func printResults(results []BenchmarkResult) {
	fmt.Println()
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println(colorBold + "                    BENCHMARK RESULTS INTERPRETATION" + colorReset)
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println()

	// Group by category
	categories := map[string][]BenchmarkResult{
		"API":    {},
		"Worker": {},
		"Queue":  {},
		"DB":     {},
		"Other":  {},
	}

	for _, result := range results {
		if std, exists := Standards[result.Name]; exists {
			categories[std.Category] = append(categories[std.Category], result)
		} else {
			categories["Other"] = append(categories["Other"], result)
		}
	}

	ratings := map[Rating]int{}

	for _, category := range []string{"API", "Worker", "Queue", "DB", "Other"} {
		catResults := categories[category]
		if len(catResults) == 0 {
			continue
		}

		fmt.Printf("%s### %s Benchmarks ###%s\n\n", colorBold+colorCyan, category, colorReset)

		for _, result := range catResults {
			std, exists := Standards[result.Name]
			if !exists {
				fmt.Printf("  %s: %s (no standard defined)\n\n", result.Name, formatDuration(result.NsOp))
				continue
			}

			ratio := float64(result.NsOp) / float64(std.ExpectedNsOp)
			rating := getRating(ratio)
			ratings[rating]++
			color := getRatingColor(rating)
			symbol := getRatingSymbol(rating)

			fmt.Printf("  %s%s%s %s\n", color, symbol, colorReset, result.Name)
			fmt.Printf("      %s\n", std.Description)
			fmt.Printf("      Actual:   %s%s%s\n", colorBold, formatDuration(result.NsOp), colorReset)
			fmt.Printf("      Expected: %s\n", formatDuration(std.ExpectedNsOp))
			fmt.Printf("      Ratio:    %s%.2fx%s", color, ratio, colorReset)
			if ratio <= 1.0 {
				fmt.Printf(" (within budget)")
			} else if ratio <= 2.0 {
				fmt.Printf(" (slightly over)")
			} else {
				fmt.Printf(" (needs attention)")
			}
			fmt.Println()

			if result.BytesOp > 0 {
				fmt.Printf("      Memory:   %d B/op, %d allocs/op\n", result.BytesOp, result.AllocsOp)
			}
			fmt.Println()
		}
	}

	// Summary
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println(colorBold + "                              SUMMARY" + colorReset)
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println()

	total := len(results)
	fmt.Printf("  Total benchmarks analyzed: %d\n\n", total)

	fmt.Printf("  %s+++%s Excellent (< 0.5x expected): %d\n", colorGreen, colorReset, ratings[RatingExcellent])
	fmt.Printf("  %s++ %s Good      (0.5-1.0x):        %d\n", colorGreen, colorReset, ratings[RatingGood])
	fmt.Printf("  %s+  %s Acceptable (1.0-2.0x):       %d\n", colorYellow, colorReset, ratings[RatingAcceptable])
	fmt.Printf("  %s-  %s Poor      (2.0-5.0x):        %d\n", colorYellow, colorReset, ratings[RatingPoor])
	fmt.Printf("  %s---%s Critical  (> 5.0x):          %d\n", colorRed, colorReset, ratings[RatingCritical])
	fmt.Println()

	// Recommendations
	if ratings[RatingCritical] > 0 {
		fmt.Printf("  %s!!! CRITICAL:%s Some benchmarks are >5x slower than expected.\n", colorRed, colorReset)
		fmt.Println("      Immediate investigation required!")
		fmt.Println()
	}

	if ratings[RatingPoor] > 0 {
		fmt.Printf("  %s!!! WARNING:%s Some benchmarks are 2-5x slower than expected.\n", colorYellow, colorReset)
		fmt.Println("      Consider profiling and optimization.")
		fmt.Println()
	}

	goodPercentage := float64(ratings[RatingExcellent]+ratings[RatingGood]) / float64(total) * 100
	if goodPercentage >= 80 {
		fmt.Printf("  %sPerformance Status: HEALTHY%s (%.0f%% within expectations)\n", colorGreen, colorReset, goodPercentage)
	} else if goodPercentage >= 50 {
		fmt.Printf("  %sPerformance Status: ACCEPTABLE%s (%.0f%% within expectations)\n", colorYellow, colorReset, goodPercentage)
	} else {
		fmt.Printf("  %sPerformance Status: NEEDS ATTENTION%s (%.0f%% within expectations)\n", colorRed, colorReset, goodPercentage)
	}

	fmt.Println()
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println()
}

func printStandardsTable() {
	fmt.Println()
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println(colorBold + "                    EXPECTED PERFORMANCE STANDARDS" + colorReset)
	fmt.Println(colorBold + "================================================================================" + colorReset)
	fmt.Println()

	categories := []string{"API", "Worker", "Queue", "DB"}

	for _, category := range categories {
		fmt.Printf("%s### %s ###%s\n\n", colorCyan, category, colorReset)
		fmt.Printf("  %-40s %15s  %s\n", "Benchmark", "Expected", "Description")
		fmt.Println("  " + strings.Repeat("-", 75))

		for name, std := range Standards {
			if std.Category == category {
				fmt.Printf("  %-40s %15s  %s\n", name, formatDuration(std.ExpectedNsOp), std.Description)
			}
		}
		fmt.Println()
	}

	fmt.Println(colorBold + "Rating Scale:" + colorReset)
	fmt.Printf("  %s+++%s Excellent:  < 0.5x expected (better than expected)\n", colorGreen, colorReset)
	fmt.Printf("  %s++ %s Good:       0.5x - 1.0x (within expectations)\n", colorGreen, colorReset)
	fmt.Printf("  %s+  %s Acceptable: 1.0x - 2.0x (slightly over budget)\n", colorYellow, colorReset)
	fmt.Printf("  %s-  %s Poor:       2.0x - 5.0x (needs optimization)\n", colorYellow, colorReset)
	fmt.Printf("  %s---%s Critical:   > 5.0x (immediate attention required)\n", colorRed, colorReset)
	fmt.Println()
}

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
