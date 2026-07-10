package websocket

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yourorg/enterprise-api/internal/config"
)

func RegisterRoutes(router *gin.Engine, db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) {
	// WebSocket routes go here
}
