package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RateLimiter(redisClient *redis.Client, maxReqs int, windowSec int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		deviceID := c.GetHeader("X-Device-ID")
		key := fmt.Sprintf("rate_limit:%s:%s", ip, deviceID)

		ctx := c.Request.Context()
		val, err := redisClient.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rate limiter error"})
			return
		}
		if val >= maxReqs {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		pipe := redisClient.TxPipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, time.Duration(windowSec)*time.Second)
		_, err = pipe.Exec(ctx)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rate limiter error"})
			return
		}
		c.Next()
	}
}
