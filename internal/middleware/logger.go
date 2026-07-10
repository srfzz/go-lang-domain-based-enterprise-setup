package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourorg/enterprise-api/internal/shared/logger"
	"go.uber.org/zap"
)

func ActivityLogger(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		end := time.Now()
		latency := end.Sub(start)

		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)

		go func() {
			userID, exists := c.Get("userID")
			var uid *uuid.UUID
			if exists {
				id := userID.(uuid.UUID)
				uid = &id
			}
			deviceID := c.GetHeader("X-Device-ID")
			action := c.Request.Method + " " + c.Request.URL.Path
			_, err := db.Exec(context.Background(),
				`INSERT INTO activity_logs (user_id, action, resource, ip_address, user_agent, device_id)
				 VALUES ($1, $2, $3, $4::inet, $5, $6)`,
				uid, action, c.Request.URL.Path, c.ClientIP(), c.Request.UserAgent(), deviceID,
			)
			if err != nil {
				logger.Error("failed to insert activity log", zap.Error(err))
			}
		}()
	}
}
