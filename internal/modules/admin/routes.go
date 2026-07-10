package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/middleware"
	adminHandler "github.com/yourorg/enterprise-api/internal/modules/admin/handler"
	adminService "github.com/yourorg/enterprise-api/internal/modules/admin/service"
)

func RegisterRoutes(router *gin.Engine, db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) {
	svc := adminService.NewAdminService(db, redisClient, cfg)
	handler := adminHandler.NewAdminHandler(svc)

	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.AuthRequired(redisClient))
	admin.Use(middleware.RequirePermission(db, "manage", "admin"))
	{
		// Users
		admin.GET("/users", handler.ListUsers)
		admin.GET("/users/:id", handler.GetUser)
		admin.POST("/users", handler.CreateUser)
		admin.PUT("/users/:id/roles", handler.AssignRoles)

		// Roles
		admin.GET("/roles", handler.ListRoles)
		admin.POST("/roles", handler.CreateRole)
		admin.PUT("/roles/:id", handler.UpdateRole)
		admin.DELETE("/roles/:id", handler.DeleteRole)
		admin.PUT("/roles/:id/permissions", handler.AssignPermissions)

		// Permissions
		admin.GET("/permissions", handler.ListPermissions)
		admin.POST("/permissions", handler.CreatePermission)
		admin.PUT("/permissions/:id", handler.UpdatePermission)
		admin.DELETE("/permissions/:id", handler.DeletePermission)
	}
}
