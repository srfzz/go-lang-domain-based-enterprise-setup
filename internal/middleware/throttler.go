package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen int64
}

var (
	clients = sync.Map{}
)

func keyForRequest(c *gin.Context) string {
	ip := c.ClientIP()
	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		deviceID = "unknown"
	}
	return fmt.Sprintf("%s:%s", ip, deviceID)
}

func getLimiter(key string, burst int, r rate.Limit) *rate.Limiter {
	val, _ := clients.LoadOrStore(key, &ipLimiter{
		limiter:  rate.NewLimiter(r, burst),
		lastSeen: time.Now().Unix(),
	})
	entry := val.(*ipLimiter)
	entry.lastSeen = time.Now().Unix()
	return entry.limiter
}

// cleanupIdle removes limiters unused for >10 minutes (runs once per minute)
func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			now := time.Now().Unix()
			clients.Range(func(key, val interface{}) bool {
				entry := val.(*ipLimiter)
				if now-entry.lastSeen > 600 {
					clients.Delete(key)
				}
				return true
			})
		}
	}()
}

func Throttle(burst int, r rate.Limit) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyForRequest(c)
		limiter := getLimiter(key, burst, r)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "server busy, please retry"})
			return
		}
		c.Next()
	}
}
