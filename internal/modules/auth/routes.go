package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/middleware"
	authHandler "github.com/yourorg/enterprise-api/internal/modules/auth/handler"
	authService "github.com/yourorg/enterprise-api/internal/modules/auth/service"
)

func RegisterRoutes(router *gin.Engine, db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) {
	svc := authService.NewAuthService(db, redisClient, cfg)
	handler := authHandler.NewAuthHandler(svc)

	auth := router.Group("/api/v1/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
		auth.POST("/refresh", handler.RefreshToken)
		auth.POST("/logout", middleware.AuthRequired(redisClient), handler.Logout)
	}
}
