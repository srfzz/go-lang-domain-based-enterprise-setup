//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/database"
	adminService "github.com/yourorg/enterprise-api/internal/modules/admin/service"
	"github.com/yourorg/enterprise-api/internal/router"
	"github.com/yourorg/enterprise-api/internal/shared/logger"
	"github.com/yourorg/enterprise-api/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

// --- Helpers ---

func setupTestEnv(t *testing.T) (*config.Config, *pgxpool.Pool, *redis.Client) {
	t.Helper()
	if os.Getenv("DB_HOST") == "" {
		t.Skip("SKIP: DB_HOST not set. Integration tests require PostgreSQL and Redis.")
	}
	if os.Getenv("JWT_PRIVATE_KEY_PATH") == "" {
		os.Setenv("JWT_PRIVATE_KEY_PATH", "../../keys/private.pem")
	}
	if os.Getenv("JWT_PUBLIC_KEY_PATH") == "" {
		os.Setenv("JWT_PUBLIC_KEY_PATH", "../../keys/public.pem")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	logger.Init(cfg.LogLevel, cfg.AppName, cfg.LogFilePath)
	if err := utils.LoadKeys(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath); err != nil {
		t.Fatalf("load keys: %v", err)
	}
	db, err := database.NewPostgresPool(cfg)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	redisClient := database.NewRedisClient(cfg)
	adminService.NewAdminService(db, redisClient, cfg).SeedDefaultAdmin(context.Background())
	return cfg, db, redisClient
}

func createTestServer(cfg *config.Config, db *pgxpool.Pool, redisClient *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return router.Setup(cfg, db, redisClient)
}

type testClient struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
}

func newTestClient(baseURL string) *testClient {
	return &testClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 200,
			},
		},
		headers: map[string]string{
			"Content-Type": "application/json",
			"X-Device-ID":  "integration-test",
		},
	}
}

func (c *testClient) do(method, path string, body interface{}, token string) (*http.Response, time.Duration, string) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, c.baseURL+path, reqBody)
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	dur := time.Since(start)
	if err != nil {
		return nil, dur, err.Error()
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	return resp, dur, string(bodyBytes)
}

func getToken(c *testClient, email, password string) string {
	resp, _, body := c.do("POST", "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	if resp == nil || resp.StatusCode != 200 {
		return ""
	}
	var r struct{ AccessToken string `json:"access_token"` }
	json.Unmarshal([]byte(body), &r)
	return r.AccessToken
}

type metrics struct {
	mu        sync.Mutex
	latencies []float64
	total     int64
	success   int64
}

func (m *metrics) add(dur time.Duration, status int) {
	m.mu.Lock()
	m.latencies = append(m.latencies, float64(dur.Microseconds())/1000.0)
	m.total++
	if status >= 200 && status < 500 {
		m.success++
	}
	m.mu.Unlock()
}

func (m *metrics) summary() (total, success int64, p50, p90, p99, avg, min, max float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.latencies) == 0 {
		return
	}
	total, success = m.total, m.success
	sorted := make([]float64, len(m.latencies))
	copy(sorted, m.latencies)
	sort.Float64s(sorted)
	min, max = sorted[0], sorted[len(sorted)-1]
	for _, v := range sorted {
		avg += v
	}
	avg /= float64(len(sorted))
	p50 = pctl(sorted, 50)
	p90 = pctl(sorted, 90)
	p99 = pctl(sorted, 99)
	return
}

func pctl(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func startServer(cfg *config.Config, db *pgxpool.Pool, redisClient *redis.Client) (*http.Server, net.Addr) {
	engine := createTestServer(cfg, db, redisClient)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	srv := &http.Server{Handler: engine}
	go srv.Serve(listener)
	time.Sleep(100 * time.Millisecond)
	return srv, listener.Addr()
}

// --- Tests ---

func TestFullFlow(t *testing.T) {
	cfg, db, redisClient := setupTestEnv(t)
	defer db.Close()
	defer redisClient.Close()

	srv, addr := startServer(cfg, db, redisClient)
	defer srv.Shutdown(context.Background())

	baseURL := "http://" + addr.String()
	c := newTestClient(baseURL)

	t.Run("Health", func(t *testing.T) {
		resp, dur, body := c.do("GET", "/health", nil, "")
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("health failed: %s", body)
		}
		t.Logf("✓ health: %dms", dur.Milliseconds())
	})

	var accessToken, refreshToken string

	t.Run("Register", func(t *testing.T) {
		email := fmt.Sprintf("flow_%d@test.com", time.Now().UnixNano())
		resp, dur, body := c.do("POST", "/api/v1/auth/register", map[string]string{
			"email": email, "password": "Test1234!", "full_name": "Flow User",
		}, "")
		if resp == nil || resp.StatusCode != 201 {
			t.Fatalf("register: %s", body)
		}
		var r struct{ AccessToken, RefreshToken string }
		json.Unmarshal([]byte(body), &r)
		accessToken, refreshToken = r.AccessToken, r.RefreshToken
		t.Logf("✓ register: %dms", dur.Milliseconds())
	})

	t.Run("Login", func(t *testing.T) {
		email := fmt.Sprintf("login_%d@test.com", time.Now().UnixNano())
		c.do("POST", "/api/v1/auth/register", map[string]string{
			"email": email, "password": "Test1234!", "full_name": "Login User",
		}, "")
		resp, dur, body := c.do("POST", "/api/v1/auth/login", map[string]string{
			"email": email, "password": "Test1234!",
		}, "")
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("login: %s", body)
		}
		t.Logf("✓ login: %dms", dur.Milliseconds())
	})

	t.Run("Refresh", func(t *testing.T) {
		if refreshToken == "" {
			t.Skip("no refresh token")
		}
		resp, dur, body := c.do("POST", "/api/v1/auth/refresh", map[string]string{
			"refresh_token": refreshToken,
		}, "")
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("refresh: %s", body)
		}
		t.Logf("✓ refresh: %dms", dur.Milliseconds())
	})

	t.Run("Logout", func(t *testing.T) {
		if accessToken == "" || refreshToken == "" {
			t.Skip("no tokens")
		}
		resp, dur, body := c.do("POST", "/api/v1/auth/logout", map[string]string{
			"refresh_token": refreshToken,
		}, accessToken)
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("logout: %s", body)
		}
		t.Logf("✓ logout: %dms", dur.Milliseconds())
	})
}

func TestAdminFlow(t *testing.T) {
	cfg, db, redisClient := setupTestEnv(t)
	defer db.Close()
	defer redisClient.Close()

	srv, addr := startServer(cfg, db, redisClient)
	defer srv.Shutdown(context.Background())

	baseURL := "http://" + addr.String()
	c := newTestClient(baseURL)
	token := getToken(c, cfg.AdminEmail, cfg.AdminPassword)
	if token == "" {
		t.Fatal("cannot get admin token")
	}

	t.Run("ListUsers", func(t *testing.T) {
		resp, dur, body := c.do("GET", "/api/v1/admin/users?limit=10&offset=0", nil, token)
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("list users: %s", body)
		}
		t.Logf("✓ list users: %dms", dur.Milliseconds())
	})

	t.Run("ListRoles", func(t *testing.T) {
		resp, dur, body := c.do("GET", "/api/v1/admin/roles?limit=10&offset=0", nil, token)
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("list roles: %s", body)
		}
		t.Logf("✓ list roles: %dms", dur.Milliseconds())
	})

	t.Run("ListPermissions", func(t *testing.T) {
		resp, dur, body := c.do("GET", "/api/v1/admin/permissions?limit=10&offset=0", nil, token)
		if resp == nil || resp.StatusCode != 200 {
			t.Fatalf("list perms: %s", body)
		}
		t.Logf("✓ list perms: %dms", dur.Milliseconds())
	})

	t.Run("CreateUser", func(t *testing.T) {
		email := fmt.Sprintf("new_%d@test.com", time.Now().UnixNano())
		resp, dur, body := c.do("POST", "/api/v1/admin/users", map[string]interface{}{
			"email": email, "password": "NewPass123!", "full_name": "New User",
		}, token)
		if resp == nil || resp.StatusCode != 201 {
			t.Fatalf("create user: %s", body)
		}
		t.Logf("✓ create user: %dms", dur.Milliseconds())
	})
}

func TestConcurrentLoad(t *testing.T) {
	cfg, db, redisClient := setupTestEnv(t)
	defer db.Close()
	defer redisClient.Close()

	srv, addr := startServer(cfg, db, redisClient)
	defer srv.Shutdown(context.Background())

	baseURL := "http://" + addr.String()
	concurrency := 10
	duration := 10 * time.Second

	t.Logf("Concurrent test: %d users × %s", concurrency, duration)

	var m metrics
	var wg sync.WaitGroup
	stop := make(chan struct{})
	var counter int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cl := newTestClient(baseURL)
			cl.headers["X-Device-ID"] = fmt.Sprintf("conc-%d", id)
			email := fmt.Sprintf("conc_%d_%d@test.com", id, time.Now().UnixNano())

			// Register + get token
			cl.do("POST", "/api/v1/auth/register", map[string]string{
				"email": email, "password": "ConcPass123!", "full_name": fmt.Sprintf("Conc %d", id),
			}, "")
			token := getToken(cl, email, "ConcPass123!")

			for {
				select {
				case <-stop:
					return
				default:
				}

				atomic.AddInt64(&counter, 1)

				resp, dur, _ := cl.do("GET", "/health", nil, "")
				if resp != nil {
					m.add(dur, resp.StatusCode)
				}

				if token != "" {
					paths := []string{
						"/api/v1/admin/users?limit=10&offset=0",
						"/api/v1/admin/roles?limit=10&offset=0",
						"/api/v1/admin/permissions?limit=10&offset=0",
					}
					path := paths[atomic.LoadInt64(&counter)%3]
					resp2, dur2, _ := cl.do("GET", path, nil, token)
					if resp2 != nil {
						m.add(dur2, resp2.StatusCode)
					}
				}
				time.Sleep(time.Duration(100+id*20) * time.Millisecond)
			}
		}(i)
	}

	time.AfterFunc(duration, func() { close(stop) })
	wg.Wait()

	total, success, p50, p90, p99, avg, min, max := m.summary()
	rps := float64(total) / duration.Seconds()

	t.Logf("")
	t.Logf("══════════ CONCURRENT RESULTS ══════════")
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Total:       %d", total)
	t.Logf("  Success:     %d", success)
	t.Logf("  Failed:      %d", total-success)
	t.Logf("  Throughput:  %.1f req/s", rps)
	t.Logf("  Latency:")
	t.Logf("    min  = %8.2f ms", min)
	t.Logf("    avg  = %8.2f ms", avg)
	t.Logf("    p50  = %8.2f ms", p50)
	t.Logf("    p90  = %8.2f ms", p90)
	t.Logf("    p99  = %8.2f ms", p99)
	t.Logf("    max  = %8.2f ms", max)
	t.Logf("════════════════════════════════════════")

	if p99 > 5000 {
		t.Errorf("p99 %.2fms exceeds 5000ms threshold", p99)
	}
	if total-success > int64(float64(total)*0.10) {
		t.Errorf("error rate %.1f%% exceeds 10%%", float64(total-success)/float64(total)*100)
	}
}
