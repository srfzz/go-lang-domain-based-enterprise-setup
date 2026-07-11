package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// --- Metrics ---

type Metrics struct {
	mu             sync.RWMutex
	latencies      []float64
	total          int64
	success        int64
	failed         int64
	statusCount    map[int]int64
	startTime      time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{statusCount: make(map[int]int64), startTime: time.Now()}
}

func (m *Metrics) Record(duration time.Duration, status int) {
	ms := float64(duration.Microseconds()) / 1000.0
	m.mu.Lock()
	m.latencies = append(m.latencies, ms)
	m.total++
	if status >= 200 && status < 500 {
		m.success++
	} else {
		m.failed++
	}
	m.statusCount[status]++
	m.mu.Unlock()
}

func (m *Metrics) Snapshot() Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.latencies) == 0 {
		return Summary{}
	}
	elapsed := time.Since(m.startTime).Seconds()
	sorted := make([]float64, len(m.latencies))
	copy(sorted, m.latencies)
	sort.Float64s(sorted)

	return Summary{
		TotalRequests: m.total,
		Success:       m.success,
		Failed:        m.failed,
		RPS:           float64(m.total) / elapsed,
		P50:           percentile(sorted, 50),
		P90:           percentile(sorted, 90),
		P99:           percentile(sorted, 99),
		P999:          percentile(sorted, 99.9),
		Min:           sorted[0],
		Max:           sorted[len(sorted)-1],
		Avg:           avg(sorted),
		StatusCount:   m.statusCount,
		Elapsed:       elapsed,
	}
}

type Summary struct {
	TotalRequests int64              `json:"total_requests"`
	Success       int64              `json:"success"`
	Failed        int64              `json:"failed"`
	RPS           float64            `json:"rps"`
	P50           float64            `json:"p50_ms"`
	P90           float64            `json:"p90_ms"`
	P99           float64            `json:"p99_ms"`
	P999          float64            `json:"p999_ms"`
	Min           float64            `json:"min_ms"`
	Max           float64            `json:"max_ms"`
	Avg           float64            `json:"avg_ms"`
	StatusCount   map[int]int64      `json:"status_codes"`
	Elapsed       float64            `json:"elapsed_sec"`
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// --- HTTP Client ---

type Client struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 200,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
			},
		},
		headers: map[string]string{
			"Content-Type": "application/json",
			"X-Device-ID":  "loadtest",
		},
	}
}

func (c *Client) Do(method, path string, body interface{}, authToken string) (*http.Response, time.Duration, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, duration, err
	}
	// Drain body
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp, duration, nil
}

// --- Scenarios ---

type ScenarioConfig struct {
	Concurrency    int
	Duration       time.Duration
	BaseURL        string
	AdminEmail     string
	AdminPassword  string
	WarmupDuration time.Duration
}

type scenarioFunc func(*Client, *ScenarioConfig, *Metrics, *sync.WaitGroup)

func runScenario(desc string, fn scenarioFunc, cfg *ScenarioConfig) Summary {
	fmt.Printf("\n🚀 %s (%d concurrent, %s duration)\n", desc, cfg.Concurrency, cfg.Duration)
	metrics := NewMetrics()

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go fn(NewClient(cfg.BaseURL), cfg, metrics, &wg)
	}
	wg.Wait()

	summary := metrics.Snapshot()
	printSummary(summary)
	return summary
}

func printSummary(s Summary) {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  Total Requests:  %d\n", s.TotalRequests)
	fmt.Printf("  Success:         %d  Failed: %d\n", s.Success, s.Failed)
	fmt.Printf("  Throughput:      %.1f req/s\n", s.RPS)
	fmt.Printf("  Latency:\n")
	fmt.Printf("    min  = %8.2f ms\n", s.Min)
	fmt.Printf("    avg  = %8.2f ms\n", s.Avg)
	fmt.Printf("    p50  = %8.2f ms\n", s.P50)
	fmt.Printf("    p90  = %8.2f ms\n", s.P90)
	fmt.Printf("    p99  = %8.2f ms\n", s.P99)
	fmt.Printf("    p999 = %8.2f ms\n", s.P999)
	fmt.Printf("    max  = %8.2f ms\n", s.Max)
	fmt.Printf("  Status codes:    ")
	codes := make([]int, 0, len(s.StatusCount))
	for code := range s.StatusCount {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	for i, code := range codes {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%d: %d", code, s.StatusCount[code])
	}
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
}

func warmup(cfg *ScenarioConfig) Summary {
	fmt.Printf("\n🔥 Warmup... (%d concurrent, %s)\n", cfg.Concurrency, cfg.WarmupDuration)
	metrics := NewMetrics()
	stop := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := NewClient(cfg.BaseURL)
			start := time.Now()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if time.Since(start) >= cfg.WarmupDuration {
					return
				}
				resp, dur, err := client.Do("GET", "/health", nil, "")
				if err == nil {
					metrics.Record(dur, resp.StatusCode)
				}
				time.Sleep(time.Duration(50+id%50) * time.Millisecond)
			}
		}(i)
	}
	time.Sleep(cfg.WarmupDuration)
	close(stop)
	wg.Wait()
	s := metrics.Snapshot()
	fmt.Printf("  Warmup requests: %d (%.1f req/s)\n", s.TotalRequests, s.RPS)
	return s
}

// --- Worker Functions ---

func healthWorker(client *Client, cfg *ScenarioConfig, metrics *Metrics, wg *sync.WaitGroup) {
	defer wg.Done()
	start := time.Now()
	id := atomic.AddInt64(&workerID, 0)

	for {
		if time.Since(start) >= cfg.Duration {
			return
		}
		resp, dur, err := client.Do("GET", "/health", nil, "")
		if err != nil {
			metrics.Record(dur, 0)
		} else {
			metrics.Record(dur, resp.StatusCode)
		}
		time.Sleep(time.Duration(50+(id%50)) * time.Millisecond)
	}
}

var workerID int64

func mixedWorker(client *Client, cfg *ScenarioConfig, metrics *Metrics, wg *sync.WaitGroup) {
	defer wg.Done()

	// Each worker registers a unique user, logs in, and hits authenticated endpoints
	uid := atomic.AddInt64(&workerID, 1)
	start := time.Now()
	email := fmt.Sprintf("load_%d_%d@test.com", uid, time.Now().UnixNano())
	password := "LoadTestPass123!"
	deviceID := fmt.Sprintf("device-%d", uid)
	client.headers["X-Device-ID"] = deviceID

	var accessToken string

	for {
		if time.Since(start) >= cfg.Duration {
			return
		}

		step := time.Since(start).Seconds() / cfg.Duration.Seconds()

		if accessToken == "" {
			// Try login first
			loginResp, dur, err := client.Do("POST", "/api/v1/auth/login", map[string]string{
				"email":    email,
				"password": password,
			}, "")
			if err != nil || loginResp.StatusCode != 200 {
				metrics.Record(dur, 0)
				// Maybe not registered yet — try register
				regResp, dur2, err2 := client.Do("POST", "/api/v1/auth/register", map[string]string{
					"email":     email,
					"password":  password,
					"full_name": fmt.Sprintf("Load User %d", uid),
				}, "")
				if err2 != nil {
					metrics.Record(dur2, 0)
				} else {
					metrics.Record(dur2, regResp.StatusCode)
					if regResp.StatusCode == 201 {
						var result map[string]interface{}
						// Can't easily read body since we drain it — just log and retry
						_ = result
					}
				}
				time.Sleep(200 * time.Millisecond)
				continue
			}
			metrics.Record(dur, loginResp.StatusCode)
			// Login succeeded but we drained the body — need re-login to capture token
			// Let's use a different approach: use non-draining client for auth
			accessToken = getToken(client, email, password)
			if accessToken == "" {
				time.Sleep(200 * time.Millisecond)
				continue
			}
		}

		// Hit authenticated endpoints
		var resp *http.Response
		var dur time.Duration
		var err error

		// Rotate through endpoints
		switch int(step * 100) % 6 {
		case 0:
			resp, dur, err = client.Do("GET", "/api/v1/admin/users?limit=10", nil, accessToken)
		case 1:
			resp, dur, err = client.Do("GET", "/api/v1/admin/roles?limit=10", nil, accessToken)
		case 2:
			resp, dur, err = client.Do("GET", "/api/v1/admin/permissions?limit=10", nil, accessToken)
		case 3:
			resp, dur, err = client.Do("GET", "/api/v1/incidents/?limit=10", nil, accessToken)
		case 4:
			resp, dur, err = client.Do("GET", "/health", nil, "")
		default:
			resp, dur, err = client.Do("POST", "/api/v1/incidents/", map[string]string{
				"title":       fmt.Sprintf("Load test incident %d", uid),
				"description": "Generated during load test",
				"severity":    "low",
			}, accessToken)
		}

		if err != nil {
			metrics.Record(dur, 0)
		} else {
			metrics.Record(dur, resp.StatusCode)
		}

		// Refresh token periodically
		if step > 0.3 && step < 0.35 {
			// Refresh
			refreshToken := getRefreshToken(client, email, password)
			if refreshToken != "" {
				resp2, dur2, err2 := client.Do("POST", "/api/v1/auth/refresh", map[string]string{
					"refresh_token": refreshToken,
				}, "")
				if err2 == nil {
					metrics.Record(dur2, resp2.StatusCode)
				}
			}
		}

		time.Sleep(time.Duration(100+(uid%200)) * time.Millisecond)
	}
}

func getToken(client *Client, email, password string) string {
	data, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest("POST", client.baseURL+"/api/v1/auth/login", bytes.NewReader(data))
	for k, v := range client.headers {
		req.Header.Set(k, v)
	}
	resp, err := client.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
	}
	json.Unmarshal(body, &result)
	return result.AccessToken
}

func getRefreshToken(client *Client, email, password string) string {
	data, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest("POST", client.baseURL+"/api/v1/auth/login", bytes.NewReader(data))
	for k, v := range client.headers {
		req.Header.Set(k, v)
	}
	resp, err := client.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.Unmarshal(body, &result)
	return result.RefreshToken
}

func adminWorker(client *Client, cfg *ScenarioConfig, metrics *Metrics, wg *sync.WaitGroup) {
	defer wg.Done()

	// Login as admin
	accessToken := getToken(client, cfg.AdminEmail, cfg.AdminPassword)
	if accessToken == "" {
		fmt.Println("  ⚠ Failed to get admin token, trying register...")
		regResp, dur, err := client.Do("POST", "/api/v1/auth/register", map[string]string{
			"email":     cfg.AdminEmail,
			"password":  cfg.AdminPassword,
			"full_name": "Load Test Admin",
		}, "")
		if err == nil {
			metrics.Record(dur, regResp.StatusCode)
		} else {
			metrics.Record(dur, 0)
		}
		// Wait and retry
		time.Sleep(2 * time.Second)
		accessToken = getToken(client, cfg.AdminEmail, cfg.AdminPassword)
	}
	if accessToken == "" {
		fmt.Println("  ✗ Cannot get admin token, skipping worker")
		return
	}

	start := time.Now()
	uid := atomic.AddInt64(&workerID, 1)

	for {
		if time.Since(start) >= cfg.Duration {
			return
		}

		// Create a user
		userEmail := fmt.Sprintf("created_%d_%d@test.com", uid, time.Now().UnixNano())
		resp, dur, err := client.Do("POST", "/api/v1/admin/users", map[string]interface{}{
			"email":     userEmail,
			"password":  "CreatedPass123!",
			"full_name": fmt.Sprintf("Created User %d", uid),
		}, accessToken)
		if err != nil {
			metrics.Record(dur, 0)
		} else {
			metrics.Record(dur, resp.StatusCode)
		}

		// List users
		resp2, dur2, err2 := client.Do("GET", "/api/v1/admin/users?limit=20", nil, accessToken)
		if err2 != nil {
			metrics.Record(dur2, 0)
		} else {
			metrics.Record(dur2, resp2.StatusCode)
		}

		// List roles
		resp3, dur3, err3 := client.Do("GET", "/api/v1/admin/roles?limit=20", nil, accessToken)
		if err3 != nil {
			metrics.Record(dur3, 0)
		} else {
			metrics.Record(dur3, resp3.StatusCode)
		}

		time.Sleep(time.Duration(200+(uid%100)) * time.Millisecond)
	}
}

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "Base URL of the API")
	concurrency := flag.Int("c", 20, "Number of concurrent workers")
	duration := flag.Duration("d", 30*time.Second, "Test duration")
	warmupDur := flag.Duration("warmup", 5*time.Second, "Warmup duration")
	adminEmail := flag.String("admin-email", "admin@example.com", "Admin email")
	adminPassword := flag.String("admin-password", "admin123!", "Admin password")
	output := flag.String("o", "", "Output file for JSON results")
	flag.Parse()

	cfg := &ScenarioConfig{
		Concurrency:    *concurrency,
		Duration:       *duration,
		BaseURL:        *baseURL,
		AdminEmail:     *adminEmail,
		AdminPassword:  *adminPassword,
		WarmupDuration: *warmupDur,
	}

	fmt.Println(strings.Repeat("═", 60))
	fmt.Println("  LOAD TEST")
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("  Target:        %s\n", cfg.BaseURL)
	fmt.Printf("  Concurrency:   %d\n", cfg.Concurrency)
	fmt.Printf("  Duration:      %s\n", cfg.Duration)
	fmt.Printf("  Warmup:        %s\n", cfg.WarmupDuration)
	fmt.Println(strings.Repeat("═", 60))

	// Trap Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n⚠ Interrupted, printing results...")
	}()

	// Warmup
	warmup(cfg)

	// 1. Health endpoint benchmark
	healthResult := runScenario("Health endpoint", healthWorker, cfg)

	// 2. Mixed authenticated workload
	mixedResult := runScenario("Mixed workload (register + login + API calls)", mixedWorker, cfg)

	// 3. Admin operations
	adminResult := runScenario("Admin operations", adminWorker, cfg)

	// Final summary
	fmt.Println(strings.Repeat("═", 60))
	fmt.Println("  FINAL RESULTS")
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("  %-25s %12s %12s %12s %12s %12s\n", "Scenario", "Total", "RPS", "p50(ms)", "p99(ms)", "Errors")
	fmt.Printf("  %-25s %12d %12.1f %12.2f %12.2f %12d\n", "Health", healthResult.TotalRequests, healthResult.RPS, healthResult.P50, healthResult.P99, healthResult.Failed)
	fmt.Printf("  %-25s %12d %12.1f %12.2f %12.2f %12d\n", "Mixed", mixedResult.TotalRequests, mixedResult.RPS, mixedResult.P50, mixedResult.P99, mixedResult.Failed)
	fmt.Printf("  %-25s %12d %12.1f %12.2f %12.2f %12d\n", "Admin", adminResult.TotalRequests, adminResult.RPS, adminResult.P50, adminResult.P99, adminResult.Failed)
	fmt.Println(strings.Repeat("═", 60))

	// Estimate max capacity based on p99 < 2000ms threshold
	if mixedResult.P99 < 2000 && mixedResult.Failed < int64(float64(mixedResult.TotalRequests)*0.05) {
		estRPS := mixedResult.RPS * 2.0
		fmt.Printf("\n✅ System appears stable at %d concurrent users\n", cfg.Concurrency)
		fmt.Printf("   Estimated max throughput: ~%.0f req/s\n", estRPS)
		fmt.Printf("   To find the breaking point, increase -c and -d flags\n")
	} else if mixedResult.P99 < 5000 {
		fmt.Printf("\n⚠ System is under moderate load at %d concurrent users\n", cfg.Concurrency)
		fmt.Printf("   p99=%.1fms. Consider increasing resources or tuning.\n", mixedResult.P99)
	} else {
		fmt.Printf("\n❌ System is saturated at %d concurrent users\n", cfg.Concurrency)
		fmt.Printf("   p99=%.1fms, errors=%d. This is near the breaking point.\n", mixedResult.P99, mixedResult.Failed)
	}

	// Write JSON output
	if *output != "" {
		results := map[string]Summary{
			"health": healthResult,
			"mixed":  mixedResult,
			"admin":  adminResult,
		}
		data, _ := json.MarshalIndent(results, "", "  ")
		os.WriteFile(*output, data, 0644)
		fmt.Printf("\n📄 Results written to %s\n", *output)
	}
}


