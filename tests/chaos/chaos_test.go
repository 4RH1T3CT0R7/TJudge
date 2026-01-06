//go:build chaos
// +build chaos

package chaos

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ChaosConfig holds configuration for chaos tests
type ChaosConfig struct {
	APIURL       string
	PostgresHost string
	RedisHost    string
	PostgresPort string
	RedisPort    string
}

func getConfig() ChaosConfig {
	return ChaosConfig{
		APIURL:       getEnv("CHAOS_API_URL", "http://localhost:8080"),
		PostgresHost: getEnv("DB_HOST", "localhost"),
		RedisHost:    getEnv("REDIS_HOST", "localhost"),
		PostgresPort: getEnv("DB_PORT", "5432"),
		RedisPort:    getEnv("REDIS_PORT", "6379"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// =============================================================================
// Chaos Test: API Resilience Under Load
// =============================================================================

func TestChaos_APIResilienceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("ConcurrentRequests", func(t *testing.T) {
		var successCount, failCount atomic.Int64
		var wg sync.WaitGroup

		concurrency := 100
		requestsPerWorker := 10

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for j := 0; j < requestsPerWorker; j++ {
					resp, err := client.Get(config.APIURL + "/health")
					if err != nil {
						failCount.Add(1)
						continue
					}

					if resp.StatusCode == http.StatusOK {
						successCount.Add(1)
					} else {
						failCount.Add(1)
					}
					resp.Body.Close()
				}
			}()
		}

		wg.Wait()

		total := successCount.Load() + failCount.Load()
		successRate := float64(successCount.Load()) / float64(total) * 100

		t.Logf("Total: %d, Success: %d, Fail: %d, Success Rate: %.2f%%",
			total, successCount.Load(), failCount.Load(), successRate)

		// Should have at least 90% success rate under load
		assert.Greater(t, successRate, 90.0,
			"Success rate should be > 90%% under load")
	})

	t.Run("BurstRequests", func(t *testing.T) {
		var successCount, failCount atomic.Int64

		// Send burst of 500 requests
		burstSize := 500
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				resp, err := client.Get(config.APIURL + "/health")
				if err != nil {
					failCount.Add(1)
					return
				}

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTooManyRequests {
					successCount.Add(1)
				} else {
					failCount.Add(1)
				}
				resp.Body.Close()
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		total := successCount.Load() + failCount.Load()
		t.Logf("Burst of %d requests completed in %v", total, duration)
		t.Logf("Success: %d, Fail: %d", successCount.Load(), failCount.Load())

		// API should handle burst without crashing
		assert.Greater(t, successCount.Load(), int64(burstSize/2),
			"At least half of burst requests should succeed or be rate limited")
	})
}

// =============================================================================
// Chaos Test: Connection Recovery
// =============================================================================

func TestChaos_ConnectionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("RecoveryAfterTemporaryFailure", func(t *testing.T) {
		// Make initial request to ensure API is up
		resp, err := client.Get(config.APIURL + "/health")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Simulate network issues by making requests with very short timeout
		shortClient := &http.Client{Timeout: 1 * time.Millisecond}
		for i := 0; i < 10; i++ {
			_, _ = shortClient.Get(config.APIURL + "/health")
		}

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// API should still be responsive
		resp, err = client.Get(config.APIURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("ContinuousOperationUnderStress", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var successCount, failCount atomic.Int64
		var wg sync.WaitGroup

		// Start workers that continuously make requests
		workers := 20
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for {
					select {
					case <-ctx.Done():
						return
					default:
						resp, err := client.Get(config.APIURL + "/health")
						if err != nil {
							failCount.Add(1)
							continue
						}

						if resp.StatusCode == http.StatusOK {
							successCount.Add(1)
						} else {
							failCount.Add(1)
						}
						resp.Body.Close()

						time.Sleep(10 * time.Millisecond)
					}
				}
			}()
		}

		wg.Wait()

		total := successCount.Load() + failCount.Load()
		successRate := float64(successCount.Load()) / float64(total) * 100

		t.Logf("30s stress test: Total=%d, Success=%d, Fail=%d, Rate=%.2f%%",
			total, successCount.Load(), failCount.Load(), successRate)

		assert.Greater(t, successRate, 95.0,
			"Success rate should be > 95%% during continuous operation")
	})
}

// =============================================================================
// Chaos Test: Timeout Handling
// =============================================================================

func TestChaos_TimeoutHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()

	t.Run("ShortTimeouts", func(t *testing.T) {
		// Client with very short timeout
		client := &http.Client{Timeout: 100 * time.Millisecond}

		var timeouts, successes int

		for i := 0; i < 100; i++ {
			resp, err := client.Get(config.APIURL + "/health")
			if err != nil {
				timeouts++
				continue
			}

			if resp.StatusCode == http.StatusOK {
				successes++
			}
			resp.Body.Close()
		}

		t.Logf("Short timeout test: Timeouts=%d, Successes=%d", timeouts, successes)

		// Some requests should succeed even with short timeout
		assert.Greater(t, successes, 50,
			"Most requests should succeed even with 100ms timeout")
	})

	t.Run("ClientCancellation", func(t *testing.T) {
		client := &http.Client{Timeout: 30 * time.Second}

		var cancelledRequests int

		for i := 0; i < 20; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)

			req, _ := http.NewRequestWithContext(ctx, "GET", config.APIURL+"/health", nil)
			resp, err := client.Do(req)

			if err != nil {
				cancelledRequests++
			} else {
				resp.Body.Close()
			}

			cancel()
		}

		t.Logf("Client cancellation test: %d cancelled requests", cancelledRequests)
	})
}

// =============================================================================
// Chaos Test: Resource Exhaustion
// =============================================================================

func TestChaos_ResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()

	t.Run("ConnectionExhaustion", func(t *testing.T) {
		// Create many clients to exhaust connections
		var responses []*http.Response

		for i := 0; i < 200; i++ {
			client := &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        1,
					MaxIdleConnsPerHost: 1,
				},
			}

			resp, err := client.Get(config.APIURL + "/health")
			if err == nil {
				responses = append(responses, resp)
			}
		}

		successCount := len(responses)
		t.Logf("Connection exhaustion: %d successful connections", successCount)

		// Cleanup
		for _, resp := range responses {
			resp.Body.Close()
		}

		// API should still be responsive after stress
		time.Sleep(500 * time.Millisecond)

		mainClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := mainClient.Get(config.APIURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})
}

// =============================================================================
// Chaos Test: Slow Client
// =============================================================================

func TestChaos_SlowClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("SlowReaderDoesNotBlockServer", func(t *testing.T) {
		// Start slow reader in background
		done := make(chan struct{})
		go func() {
			defer close(done)

			resp, err := client.Get(config.APIURL + "/health")
			if err != nil {
				return
			}
			defer resp.Body.Close()

			// Read very slowly
			buf := make([]byte, 1)
			for {
				_, err := resp.Body.Read(buf)
				if err != nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()

		// Make other requests while slow reader is active
		fastClient := &http.Client{Timeout: 5 * time.Second}
		var successCount int

		for i := 0; i < 50; i++ {
			resp, err := fastClient.Get(config.APIURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				successCount++
				resp.Body.Close()
			}
		}

		t.Logf("Slow client test: %d fast requests succeeded while slow reader active", successCount)
		assert.Greater(t, successCount, 40,
			"Fast requests should succeed while slow client is reading")

		// Wait for slow reader to finish (or timeout)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
}

// =============================================================================
// Chaos Test: Error Injection Simulation
// =============================================================================

func TestChaos_ErrorInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("InvalidRequests", func(t *testing.T) {
		// Send various invalid requests
		invalidPaths := []string{
			"/api/v1/tournaments/not-a-uuid",
			"/api/v1/programs/invalid-id",
			"/api/v1/matches/bad-id",
			"/api/v1/nonexistent",
			"/../../../etc/passwd",
			"/api/v1/auth/login",           // Without body
			"/api/v1/tournaments?limit=-1", // Invalid limit
		}

		for _, path := range invalidPaths {
			resp, err := client.Get(config.APIURL + path)
			if err != nil {
				continue
			}

			// Should return proper error codes, not 500
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Path %s should not cause 500 error", path)
			resp.Body.Close()
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		malformedBodies := []string{
			`{invalid json}`,
			`{"unclosed": `,
			`[1, 2, 3`,
			`null`,
			`""`,
			string([]byte{0x00, 0x01, 0x02}), // Binary garbage
		}

		for _, malformed := range malformedBodies {
			req, _ := http.NewRequest("POST", config.APIURL+"/api/v1/auth/login",
				strings.NewReader(malformed))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			// Should handle gracefully, not crash
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Malformed JSON should be handled gracefully")
			resp.Body.Close()
		}
	})

	t.Run("LargePayload", func(t *testing.T) {
		// Try to send very large payload
		largePayload := make([]byte, 10*1024*1024) // 10MB
		for i := range largePayload {
			largePayload[i] = 'a'
		}

		req, _ := http.NewRequest("POST", config.APIURL+"/api/v1/auth/register",
			bytes.NewReader(largePayload))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			// Should reject large payloads gracefully
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
			resp.Body.Close()
		}
	})
}

// =============================================================================
// Chaos Test: Concurrent State Mutations
// =============================================================================

func TestChaos_ConcurrentStateMutations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("ConcurrentRegistrations", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, conflictCount, errorCount atomic.Int64

		timestamp := time.Now().UnixNano()
		users := 50

		for i := 0; i < users; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				username := fmt.Sprintf("chaos_user_%d_%d", timestamp, idx)
				reqBody := fmt.Sprintf(`{"username":"%s","email":"%s@test.com","password":"SecurePass123!"}`,
					username, username)

				req, _ := http.NewRequest("POST", config.APIURL+"/api/v1/auth/register",
					strings.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					errorCount.Add(1)
					return
				}
				defer resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK, http.StatusCreated:
					successCount.Add(1)
				case http.StatusConflict:
					conflictCount.Add(1)
				default:
					errorCount.Add(1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent registrations: Success=%d, Conflict=%d, Error=%d",
			successCount.Load(), conflictCount.Load(), errorCount.Load())
	})
}

// =============================================================================
// Chaos Test: Graceful Degradation
// =============================================================================

func TestChaos_GracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := getConfig()
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("HealthEndpointAlwaysResponsive", func(t *testing.T) {
		// Health endpoint should always respond quickly
		for i := 0; i < 100; i++ {
			start := time.Now()
			resp, err := client.Get(config.APIURL + "/health")
			duration := time.Since(start)

			if err == nil {
				resp.Body.Close()
				assert.Less(t, duration, 1*time.Second,
					"Health check should respond within 1 second")
			}
		}
	})

	t.Run("MetricsEndpointResponsive", func(t *testing.T) {
		resp, err := client.Get(config.APIURL + "/metrics")
		if err == nil {
			defer resp.Body.Close()
			// Metrics endpoint should be available
			assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode)
		}
	})
}
