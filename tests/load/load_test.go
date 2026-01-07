//go:build load
// +build load

package load

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	baseURL = getEnv("LOAD_API_URL", "http://localhost:8080")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// LoadTestStats collects statistics from load tests
type LoadTestStats struct {
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedReqs        int64
	TotalLatencyMs    int64
	MinLatencyMs      int64
	MaxLatencyMs      int64
	RequestsPerSecond float64
	mu                sync.Mutex
}

func NewLoadTestStats() *LoadTestStats {
	return &LoadTestStats{
		MinLatencyMs: 1<<63 - 1, // max int64
	}
}

func (s *LoadTestStats) Record(latencyMs int64, success bool) {
	atomic.AddInt64(&s.TotalRequests, 1)
	atomic.AddInt64(&s.TotalLatencyMs, latencyMs)

	if success {
		atomic.AddInt64(&s.SuccessfulReqs, 1)
	} else {
		atomic.AddInt64(&s.FailedReqs, 1)
	}

	s.mu.Lock()
	if latencyMs < s.MinLatencyMs {
		s.MinLatencyMs = latencyMs
	}
	if latencyMs > s.MaxLatencyMs {
		s.MaxLatencyMs = latencyMs
	}
	s.mu.Unlock()
}

func (s *LoadTestStats) AvgLatencyMs() float64 {
	total := atomic.LoadInt64(&s.TotalRequests)
	if total == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&s.TotalLatencyMs)) / float64(total)
}

func (s *LoadTestStats) SuccessRate() float64 {
	total := atomic.LoadInt64(&s.TotalRequests)
	if total == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&s.SuccessfulReqs)) / float64(total) * 100
}

func (s *LoadTestStats) Print(name string) {
	fmt.Printf("\n=== %s Load Test Results ===\n", name)
	fmt.Printf("Total Requests:     %d\n", s.TotalRequests)
	fmt.Printf("Successful:         %d (%.2f%%)\n", s.SuccessfulReqs, s.SuccessRate())
	fmt.Printf("Failed:             %d\n", s.FailedReqs)
	fmt.Printf("Avg Latency:        %.2f ms\n", s.AvgLatencyMs())
	fmt.Printf("Min Latency:        %d ms\n", s.MinLatencyMs)
	fmt.Printf("Max Latency:        %d ms\n", s.MaxLatencyMs)
	fmt.Printf("Requests/sec:       %.2f\n", s.RequestsPerSecond)
	fmt.Println("================================")
}

// LoadTestClient wraps HTTP client for load tests
type LoadTestClient struct {
	client *http.Client
}

func NewLoadTestClient() *LoadTestClient {
	return &LoadTestClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *LoadTestClient) Do(method, path string, body interface{}, token string) (int, int64, error) {
	start := time.Now()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, time.Since(start).Milliseconds(), err
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, time.Since(start).Milliseconds(), nil
}

// TestLoad_HealthEndpoint tests health endpoint under load
func TestLoad_HealthEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 50
	duration := 10 * time.Second

	stats := NewLoadTestStats()
	client := NewLoadTestClient()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					status, latency, err := client.Do("GET", "/health", nil, "")
					success := err == nil && status == http.StatusOK
					stats.Record(latency, success)
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	stats.RequestsPerSecond = float64(stats.TotalRequests) / elapsed

	stats.Print("Health Endpoint")

	// Verify at least 95% success rate
	require.GreaterOrEqual(t, stats.SuccessRate(), 95.0, "Success rate should be at least 95%")
}

// TestLoad_TournamentsList tests tournament listing under load
func TestLoad_TournamentsList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 30
	duration := 10 * time.Second

	stats := NewLoadTestStats()
	client := NewLoadTestClient()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					status, latency, err := client.Do("GET", "/api/v1/tournaments?limit=20", nil, "")
					success := err == nil && (status == http.StatusOK || status == http.StatusUnauthorized)
					stats.Record(latency, success)
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	stats.RequestsPerSecond = float64(stats.TotalRequests) / elapsed

	stats.Print("Tournaments List")

	require.GreaterOrEqual(t, stats.SuccessRate(), 90.0, "Success rate should be at least 90%")
}

// TestLoad_AuthLogin tests authentication under load
func TestLoad_AuthLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 20
	duration := 10 * time.Second

	// First, create a test user
	client := NewLoadTestClient()
	timestamp := time.Now().UnixNano()
	username := fmt.Sprintf("loadtest_%d", timestamp)
	password := "LoadTestPass123!"

	registerReq := map[string]string{
		"username": username,
		"email":    username + "@loadtest.com",
		"password": password,
	}

	status, _, err := client.Do("POST", "/api/v1/auth/register", registerReq, "")
	if err != nil || (status != http.StatusOK && status != http.StatusCreated) {
		t.Logf("Failed to create test user: %v, status: %d", err, status)
	}

	loginReq := map[string]string{
		"username": username,
		"password": password,
	}

	stats := NewLoadTestStats()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					status, latency, err := client.Do("POST", "/api/v1/auth/login", loginReq, "")
					success := err == nil && status == http.StatusOK
					stats.Record(latency, success)
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	stats.RequestsPerSecond = float64(stats.TotalRequests) / elapsed

	stats.Print("Auth Login")

	require.GreaterOrEqual(t, stats.SuccessRate(), 80.0, "Success rate should be at least 80%")
}

// TestLoad_MixedEndpoints tests various endpoints under mixed load
func TestLoad_MixedEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 40
	duration := 15 * time.Second

	// Create test user and get token
	client := NewLoadTestClient()
	timestamp := time.Now().UnixNano()
	username := fmt.Sprintf("mixedload_%d", timestamp)

	registerReq := map[string]string{
		"username": username,
		"email":    username + "@loadtest.com",
		"password": "MixedLoadPass123!",
	}

	var token string
	status, _, err := client.Do("POST", "/api/v1/auth/register", registerReq, "")
	if err == nil && (status == http.StatusOK || status == http.StatusCreated) {
		// Parse token from response (simplified for load test)
		loginReq := map[string]string{
			"username": username,
			"password": "MixedLoadPass123!",
		}

		req, _ := http.NewRequest("POST", baseURL+"/api/v1/auth/login", nil)
		body, _ := json.Marshal(loginReq)
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var authResp struct {
				AccessToken string `json:"access_token"`
			}
			json.NewDecoder(resp.Body).Decode(&authResp)
			token = authResp.AccessToken
			resp.Body.Close()
		}
	}

	endpoints := []struct {
		method string
		path   string
		weight int // Higher weight = more frequent
	}{
		{"GET", "/health", 10},
		{"GET", "/api/v1/tournaments", 5},
		{"GET", "/api/v1/programs", 3},
		{"GET", "/api/v1/auth/me", 2},
	}

	// Build weighted endpoint list
	var weightedEndpoints []struct {
		method string
		path   string
	}
	for _, ep := range endpoints {
		for i := 0; i < ep.weight; i++ {
			weightedEndpoints = append(weightedEndpoints, struct {
				method string
				path   string
			}{ep.method, ep.path})
		}
	}

	stats := NewLoadTestStats()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			idx := workerID
			for {
				select {
				case <-stopCh:
					return
				default:
					ep := weightedEndpoints[idx%len(weightedEndpoints)]
					status, latency, err := client.Do(ep.method, ep.path, nil, token)
					success := err == nil && status >= 200 && status < 500
					stats.Record(latency, success)
					idx++
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	stats.RequestsPerSecond = float64(stats.TotalRequests) / elapsed

	stats.Print("Mixed Endpoints")

	require.GreaterOrEqual(t, stats.SuccessRate(), 85.0, "Success rate should be at least 85%")
}

// TestLoad_RateLimiting tests rate limiting behavior
func TestLoad_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 100
	duration := 5 * time.Second

	stats := NewLoadTestStats()
	client := NewLoadTestClient()

	var rateLimitedCount int64
	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					status, latency, err := client.Do("GET", "/health", nil, "")
					success := err == nil && (status == http.StatusOK || status == http.StatusTooManyRequests)

					if status == http.StatusTooManyRequests {
						atomic.AddInt64(&rateLimitedCount, 1)
					}

					stats.Record(latency, success)
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	stats.RequestsPerSecond = float64(stats.TotalRequests) / elapsed

	stats.Print("Rate Limiting")

	t.Logf("Rate limited requests: %d (%.2f%%)", rateLimitedCount, float64(rateLimitedCount)/float64(stats.TotalRequests)*100)

	// We expect rate limiting to kick in with 100 concurrent clients
	require.GreaterOrEqual(t, stats.SuccessRate(), 50.0, "At least 50% of requests should succeed even under heavy load")
}

// TestLoad_SustainedLoad tests system behavior under sustained load
func TestLoad_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 25
	duration := 30 * time.Second

	stats := NewLoadTestStats()
	client := NewLoadTestClient()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					status, latency, err := client.Do("GET", "/health", nil, "")
					success := err == nil && status == http.StatusOK
					stats.Record(latency, success)

					// Small delay to simulate realistic traffic
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	stats.RequestsPerSecond = float64(stats.TotalRequests) / elapsed

	stats.Print("Sustained Load (30s)")

	require.GreaterOrEqual(t, stats.SuccessRate(), 98.0, "Success rate should be at least 98% under sustained load")
	require.Less(t, stats.AvgLatencyMs(), 100.0, "Average latency should be less than 100ms")
}

// TestLoad_BurstTraffic tests system behavior under burst traffic
func TestLoad_BurstTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	burstSize := 200
	numBursts := 5
	burstInterval := 2 * time.Second

	stats := NewLoadTestStats()
	client := NewLoadTestClient()

	for burst := 0; burst < numBursts; burst++ {
		var wg sync.WaitGroup

		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status, latency, err := client.Do("GET", "/health", nil, "")
				success := err == nil && (status == http.StatusOK || status == http.StatusTooManyRequests)
				stats.Record(latency, success)
			}()
		}

		wg.Wait()

		if burst < numBursts-1 {
			time.Sleep(burstInterval)
		}
	}

	stats.Print(fmt.Sprintf("Burst Traffic (%dx%d requests)", numBursts, burstSize))

	require.GreaterOrEqual(t, stats.SuccessRate(), 70.0, "Success rate should be at least 70% under burst traffic")
}
