package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseBenchmarkOutput ---

func TestParseBenchmarkOutput_Valid(t *testing.T) {
	output := `BenchmarkHealthEndpoint-8    50000    25000 ns/op    1024 B/op    10 allocs/op
BenchmarkAuthLogin-8         1000   5000000 ns/op    2048 B/op    20 allocs/op`

	results := parseBenchmarkOutput(output)

	require.Len(t, results, 2)

	assert.Equal(t, "BenchmarkHealthEndpoint", results[0].Name)
	assert.Equal(t, int64(25000), results[0].NsOp)
	assert.Equal(t, int64(50000), results[0].Iterations)
	assert.Equal(t, int64(1024), results[0].BytesOp)
	assert.Equal(t, int64(10), results[0].AllocsOp)

	assert.Equal(t, "BenchmarkAuthLogin", results[1].Name)
	assert.Equal(t, int64(5000000), results[1].NsOp)
	assert.Equal(t, int64(1000), results[1].Iterations)
}

func TestParseBenchmarkOutput_Empty(t *testing.T) {
	results := parseBenchmarkOutput("")
	assert.Empty(t, results)
}

func TestParseBenchmarkOutput_MixedLines(t *testing.T) {
	output := `goos: darwin
goarch: arm64
pkg: github.com/example/test
BenchmarkHealthEndpoint-8    50000    25000 ns/op
PASS
ok  github.com/example/test  1.234s`

	results := parseBenchmarkOutput(output)
	require.Len(t, results, 1)
	assert.Equal(t, "BenchmarkHealthEndpoint", results[0].Name)
	assert.Equal(t, int64(25000), results[0].NsOp)
}

func TestParseBenchmarkOutput_NoMemory(t *testing.T) {
	output := `BenchmarkSimple-8    100000    500 ns/op`
	results := parseBenchmarkOutput(output)

	require.Len(t, results, 1)
	assert.Equal(t, "BenchmarkSimple", results[0].Name)
	assert.Equal(t, int64(500), results[0].NsOp)
	assert.Equal(t, int64(0), results[0].BytesOp)
	assert.Equal(t, int64(0), results[0].AllocsOp)
}

func TestParseBenchmarkOutput_NoCPUSuffix(t *testing.T) {
	output := `BenchmarkSimple    100000    500 ns/op`
	results := parseBenchmarkOutput(output)

	require.Len(t, results, 1)
	assert.Equal(t, "BenchmarkSimple", results[0].Name)
}

func TestParseBenchmarkOutput_InvalidLines(t *testing.T) {
	output := `not a benchmark line
just some text
BenchmarkValid-8    1000    5000 ns/op`

	results := parseBenchmarkOutput(output)
	require.Len(t, results, 1)
	assert.Equal(t, "BenchmarkValid", results[0].Name)
}

// --- getRating ---

func TestGetRating_Excellent(t *testing.T) {
	assert.Equal(t, RatingExcellent, getRating(0.1))
	assert.Equal(t, RatingExcellent, getRating(0.49))
	assert.Equal(t, RatingExcellent, getRating(0.5))
}

func TestGetRating_Good(t *testing.T) {
	assert.Equal(t, RatingGood, getRating(0.51))
	assert.Equal(t, RatingGood, getRating(0.75))
	assert.Equal(t, RatingGood, getRating(1.0))
}

func TestGetRating_Acceptable(t *testing.T) {
	assert.Equal(t, RatingAcceptable, getRating(1.01))
	assert.Equal(t, RatingAcceptable, getRating(1.5))
	assert.Equal(t, RatingAcceptable, getRating(2.0))
}

func TestGetRating_Poor(t *testing.T) {
	assert.Equal(t, RatingPoor, getRating(2.01))
	assert.Equal(t, RatingPoor, getRating(3.5))
	assert.Equal(t, RatingPoor, getRating(5.0))
}

func TestGetRating_Critical(t *testing.T) {
	assert.Equal(t, RatingCritical, getRating(5.01))
	assert.Equal(t, RatingCritical, getRating(10.0))
	assert.Equal(t, RatingCritical, getRating(100.0))
}

// --- getRatingColor ---

func TestGetRatingColor(t *testing.T) {
	assert.Equal(t, colorGreen, getRatingColor(RatingExcellent))
	assert.Equal(t, colorGreen, getRatingColor(RatingGood))
	assert.Equal(t, colorYellow, getRatingColor(RatingAcceptable))
	assert.Equal(t, colorYellow, getRatingColor(RatingPoor))
	assert.Equal(t, colorRed, getRatingColor(RatingCritical))
	assert.Equal(t, colorReset, getRatingColor(Rating("unknown")))
}

// --- getRatingSymbol ---

func TestGetRatingSymbol(t *testing.T) {
	assert.Equal(t, "+++", getRatingSymbol(RatingExcellent))
	assert.Equal(t, "++", getRatingSymbol(RatingGood))
	assert.Equal(t, "+", getRatingSymbol(RatingAcceptable))
	assert.Equal(t, "-", getRatingSymbol(RatingPoor))
	assert.Equal(t, "---", getRatingSymbol(RatingCritical))
	assert.Equal(t, "?", getRatingSymbol(Rating("unknown")))
}

// --- formatDuration ---

func TestFormatDuration_Nanoseconds(t *testing.T) {
	assert.Equal(t, "500ns", formatDuration(500))
	assert.Equal(t, "1ns", formatDuration(1))
	assert.Equal(t, "0ns", formatDuration(0))
}

func TestFormatDuration_Microseconds(t *testing.T) {
	result := formatDuration(1500) // 1.5µs
	assert.Contains(t, result, "µs")
}

func TestFormatDuration_Milliseconds(t *testing.T) {
	result := formatDuration(5_000_000) // 5ms
	assert.Contains(t, result, "ms")
	assert.Equal(t, "5.00ms", result)
}

func TestFormatDuration_Seconds(t *testing.T) {
	result := formatDuration(2_500_000_000) // 2.5s
	assert.Contains(t, result, "s")
	assert.Equal(t, "2.50s", result)
}

func TestFormatDuration_BoundaryMicrosecond(t *testing.T) {
	// Exactly 1µs
	result := formatDuration(1000)
	assert.Contains(t, result, "µs")
	assert.Equal(t, "1.00µs", result)
}

func TestFormatDuration_BoundaryMillisecond(t *testing.T) {
	// Exactly 1ms
	result := formatDuration(1_000_000)
	assert.Contains(t, result, "ms")
	assert.Equal(t, "1.00ms", result)
}

func TestFormatDuration_BoundarySecond(t *testing.T) {
	// Exactly 1s
	result := formatDuration(1_000_000_000)
	assert.Contains(t, result, "s")
	assert.Equal(t, "1.00s", result)
}
