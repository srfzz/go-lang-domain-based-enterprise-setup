package middleware

import (
	"net/http"
	"sync"

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

func getLimiter(ip string, burst int, r rate.Limit) *rate.Limiter {
	val, _ := clients.LoadOrStore(ip, &ipLimiter{
		limiter: rate.NewLimiter(r, burst),
	})
	return val.(*ipLimiter).limiter
}

func Throttle(burst int, r rate.Limit) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getLimiter(ip, burst, r)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "server busy, please retry"})
			return
		}
		c.Next()
	}
}
