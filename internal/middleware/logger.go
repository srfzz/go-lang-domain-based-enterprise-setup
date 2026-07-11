package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourorg/enterprise-api/internal/shared/logger"
	"github.com/yourorg/enterprise-api/internal/shared/utils"
	"go.uber.org/zap"
)

var auditWorkerPool = utils.NewWorkerPool(10, 1000)

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

		userID, exists := c.Get("userID")
		var uid *uuid.UUID
		if exists {
			id := userID.(uuid.UUID)
			uid = &id
		}
		deviceID := c.GetHeader("X-Device-ID")
		action := c.Request.Method + " " + c.Request.URL.Path
		ip := c.ClientIP()
		ua := c.Request.UserAgent()
		path := c.Request.URL.Path

		auditWorkerPool.Submit(func() {
			_, err := db.Exec(context.Background(),
				`INSERT INTO activity_logs (user_id, action, resource, ip_address, user_agent, device_id)
				 VALUES ($1, $2, $3, $4::inet, $5, $6)`,
				uid, action, path, ip, ua, deviceID,
			)
			if err != nil {
				logger.Error("failed to insert activity log", zap.Error(err))
			}
		})
	}
}
