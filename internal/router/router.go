package router

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/middleware"
	"github.com/yourorg/enterprise-api/internal/modules/auth"
	"github.com/yourorg/enterprise-api/internal/modules/incident"
	"github.com/yourorg/enterprise-api/internal/modules/websocket"
)

func Setup(cfg *config.Config, db *pgxpool.Pool, redisClient *redis.Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(middleware.CORS())
	r.Use(middleware.ActivityLogger(db))
	r.Use(middleware.RateLimiter(redisClient, cfg.RateLimitRequests, cfg.RateLimitDurationSec))
	r.Use(middleware.Throttle(cfg.ThrottleBurst, rate.Limit(cfg.ThrottleRate)))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	auth.RegisterRoutes(r, db, redisClient, cfg)
	incident.RegisterRoutes(r, db, redisClient, cfg)
	websocket.RegisterRoutes(r, db, redisClient, cfg)

	return r
}
