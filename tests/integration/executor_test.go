//go:build integration && docker
// +build integration,docker

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/executor"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ExecutorTestSuite tests Docker executor functionality
type ExecutorTestSuite struct {
	suite.Suite
	executor *executor.DockerExecutor
	ctx      context.Context
}

func (s *ExecutorTestSuite) SetupSuite() {
	if os.Getenv("RUN_DOCKER_TESTS") != "true" {
		s.T().Skip("Skipping Docker tests (set RUN_DOCKER_TESTS=true)")
	}

	s.ctx = context.Background()
	log, _ := logger.New("debug", "json")

	cfg := config.ExecutorConfig{
		DockerImage:       getEnv("EXECUTOR_DOCKER_IMAGE", "ghcr.io/bmstu-itstech/tjudge-executor:latest"),
		Timeout:           30 * time.Second,
		MemoryLimit:       "256m",
		CPULimit:          "1",
		DefaultIterations: 100,
	}

	var err error
	s.executor, err = executor.NewDockerExecutor(cfg, log)
	require.NoError(s.T(), err)
}

func (s *ExecutorTestSuite) TearDownSuite() {
	if s.executor != nil {
		s.executor.Close()
	}
}

// =============================================================================
// Basic Execution Tests
// =============================================================================

func (s *ExecutorTestSuite) TestExecutor_SimpleMatch() {
	program1 := &executor.Program{
		ID:       "test-program-1",
		Language: "python",
		Code:     `print("0")`, // Always play position 0
	}
	program2 := &executor.Program{
		ID:       "test-program-2",
		Language: "python",
		Code:     `print("1")`, // Always play position 1
	}

	result, err := s.executor.Execute(s.ctx, "tictactoe", program1, program2, 10)
	require.NoError(s.T(), err)

	assert.NotNil(s.T(), result)
	assert.True(s.T(), result.Completed)
	assert.GreaterOrEqual(s.T(), result.Iterations, 1)
}

func (s *ExecutorTestSuite) TestExecutor_Timeout() {
	// Program that runs forever
	program1 := &executor.Program{
		ID:       "timeout-program",
		Language: "python",
		Code:     `while True: pass`,
	}
	program2 := &executor.Program{
		ID:       "normal-program",
		Language: "python",
		Code:     `print("0")`,
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	result, err := s.executor.Execute(ctx, "tictactoe", program1, program2, 1)

	// Should either error or return timeout result
	if err == nil {
		assert.True(s.T(), result.TimedOut || !result.Completed)
	}
}

func (s *ExecutorTestSuite) TestExecutor_InvalidProgram() {
	program1 := &executor.Program{
		ID:       "invalid-program",
		Language: "python",
		Code:     `syntax error here!!!`,
	}
	program2 := &executor.Program{
		ID:       "valid-program",
		Language: "python",
		Code:     `print("0")`,
	}

	result, err := s.executor.Execute(s.ctx, "tictactoe", program1, program2, 1)

	// Should handle gracefully
	if err == nil {
		assert.NotNil(s.T(), result)
		assert.Contains(s.T(), result.Error, "syntax")
	}
}

func (s *ExecutorTestSuite) TestExecutor_MemoryLimit() {
	// Program that tries to allocate too much memory
	program1 := &executor.Program{
		ID:       "memory-hog",
		Language: "python",
		Code:     `x = "A" * (1024 * 1024 * 500)`, // Try 500MB
	}
	program2 := &executor.Program{
		ID:       "normal",
		Language: "python",
		Code:     `print("0")`,
	}

	result, err := s.executor.Execute(s.ctx, "tictactoe", program1, program2, 1)

	// Should be killed due to memory limit
	if err == nil {
		assert.True(s.T(), result.Error != "" || !result.Completed)
	}
}

// =============================================================================
// Isolation Tests
// =============================================================================

func (s *ExecutorTestSuite) TestExecutor_NetworkIsolation() {
	// Program that tries to access network
	program1 := &executor.Program{
		ID:       "network-test",
		Language: "python",
		Code: `
import socket
try:
    s = socket.socket()
    s.connect(("8.8.8.8", 53))
    print("network-accessible")
except:
    print("0")
`,
	}
	program2 := &executor.Program{
		ID:       "normal",
		Language: "python",
		Code:     `print("1")`,
	}

	result, err := s.executor.Execute(s.ctx, "tictactoe", program1, program2, 1)
	require.NoError(s.T(), err)

	// Network should be blocked
	assert.NotContains(s.T(), result.Output, "network-accessible")
}

func (s *ExecutorTestSuite) TestExecutor_FileSystemIsolation() {
	// Program that tries to read sensitive files
	program1 := &executor.Program{
		ID:       "filesystem-test",
		Language: "python",
		Code: `
try:
    with open("/etc/passwd") as f:
        print("file-readable")
except:
    print("0")
`,
	}
	program2 := &executor.Program{
		ID:       "normal",
		Language: "python",
		Code:     `print("1")`,
	}

	result, err := s.executor.Execute(s.ctx, "tictactoe", program1, program2, 1)
	require.NoError(s.T(), err)

	// Should not be able to read system files
	assert.NotContains(s.T(), result.Output, "file-readable")
}

// =============================================================================
// Concurrent Execution Tests
// =============================================================================

func (s *ExecutorTestSuite) TestExecutor_ConcurrentMatches() {
	const numMatches = 5

	results := make(chan *executor.Result, numMatches)
	errors := make(chan error, numMatches)

	for i := 0; i < numMatches; i++ {
		go func(idx int) {
			program1 := &executor.Program{
				ID:       "concurrent-p1",
				Language: "python",
				Code:     `print("0")`,
			}
			program2 := &executor.Program{
				ID:       "concurrent-p2",
				Language: "python",
				Code:     `print("1")`,
			}

			result, err := s.executor.Execute(s.ctx, "tictactoe", program1, program2, 5)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	successCount := 0
	for i := 0; i < numMatches; i++ {
		select {
		case <-results:
			successCount++
		case err := <-errors:
			s.T().Logf("Match error: %v", err)
		case <-time.After(60 * time.Second):
			s.T().Fatal("Timeout waiting for concurrent matches")
		}
	}

	assert.GreaterOrEqual(s.T(), successCount, numMatches-1, "Most matches should succeed")
}

func TestExecutorSuite(t *testing.T) {
	suite.Run(t, new(ExecutorTestSuite))
}
