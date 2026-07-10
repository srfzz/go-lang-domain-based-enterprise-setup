package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func Throttle(burst int, r rate.Limit) gin.HandlerFunc {
	limiter := rate.NewLimiter(r, burst)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "server busy, please retry"})
			return
		}
		res := limiter.Reserve()
		if res.Delay() > 0 {
			time.Sleep(res.Delay())
		}
		c.Next()
	}
}
