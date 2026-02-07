package executor

import (
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	log, _ := logger.New("error", "json")
	return &Executor{
		config:           config.ExecutorConfig{},
		programsPath:     "/data/programs",
		hostProgramsPath: "/host/programs",
		containerPath:    "/programs",
		log:              log,
	}
}

// --- sanitizeForDB ---

func TestSanitizeForDB_NullBytes(t *testing.T) {
	result := sanitizeForDB("hello\x00world")
	assert.Equal(t, "helloworld", result)
}

func TestSanitizeForDB_Clean(t *testing.T) {
	result := sanitizeForDB("clean string")
	assert.Equal(t, "clean string", result)
}

func TestSanitizeForDB_Empty(t *testing.T) {
	result := sanitizeForDB("")
	assert.Equal(t, "", result)
}

func TestSanitizeForDB_MultipleNullBytes(t *testing.T) {
	result := sanitizeForDB("\x00start\x00middle\x00end\x00")
	assert.Equal(t, "startmiddleend", result)
}

// --- buildCommand ---

func TestBuildCommand_Basic(t *testing.T) {
	e := newTestExecutor(t)

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

func TestBuildCommand_WithIterations(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 100

	cmd := e.buildCommand("tug_of_war", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"tug_of_war", "-i", "100", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

func TestBuildCommand_WithVerbose(t *testing.T) {
	e := newTestExecutor(t)
	e.config.Verbose = true

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "-v", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

func TestBuildCommand_WithIterationsAndVerbose(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 50
	e.config.Verbose = true

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "-i", "50", "-v", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

// --- hostToContainerPath ---

func TestHostToContainerPath_MatchingPrefix(t *testing.T) {
	e := newTestExecutor(t)

	result := e.hostToContainerPath("/data/programs/team1/game1/v1_solution.py")

	assert.Equal(t, "/programs/team1/game1/v1_solution.py", result)
}

func TestHostToContainerPath_NonMatchingPrefix(t *testing.T) {
	e := newTestExecutor(t)

	result := e.hostToContainerPath("/other/path/solution.py")

	assert.Equal(t, "/other/path/solution.py", result)
}

// --- parseResult ---

func TestParseResult_ExitCode0_Player1Wins(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "15 10", "")

	require.NoError(t, err)
	assert.Equal(t, 15, result.Score1)
	assert.Equal(t, 10, result.Score2)
	assert.Equal(t, 1, result.Winner)
	assert.Equal(t, 0, result.ErrorCode)
}

func TestParseResult_ExitCode0_Player2Wins(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "10 15", "")

	require.NoError(t, err)
	assert.Equal(t, 10, result.Score1)
	assert.Equal(t, 15, result.Score2)
	assert.Equal(t, 2, result.Winner)
}

func TestParseResult_ExitCode0_Draw(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "10 10", "")

	require.NoError(t, err)
	assert.Equal(t, 10, result.Score1)
	assert.Equal(t, 10, result.Score2)
	assert.Equal(t, 0, result.Winner) // Draw
}

func TestParseResult_ExitCode1_Program1Error(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(1, "", "runtime error")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Winner) // Program 2 wins
	assert.Equal(t, 1, result.ErrorCode)
	assert.NotEmpty(t, result.ErrorMessage)
}

func TestParseResult_ExitCode2_Program2Error(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(2, "", "timeout")

	require.NoError(t, err)
	assert.Equal(t, 1, result.Winner) // Program 1 wins
	assert.Equal(t, 2, result.ErrorCode)
	assert.NotEmpty(t, result.ErrorMessage)
}

func TestParseResult_InvalidOutput_OneNumber(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "42", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 2 scores")
}

func TestParseResult_InvalidOutput_NonNumeric(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "abc def", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score1")
}

func TestParseResult_InvalidOutput_SecondNonNumeric(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "10 abc", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score2")
}

func TestParseResult_InvalidOutput_Empty(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 2 scores")
}

// --- boolPtr ---

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	falseVal := boolPtr(false)

	assert.True(t, *trueVal)
	assert.False(t, *falseVal)
}
