package incident

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/middleware"
	incidentHandler "github.com/yourorg/enterprise-api/internal/modules/incident/handler"
	incidentRepo "github.com/yourorg/enterprise-api/internal/modules/incident/repository"
	incidentService "github.com/yourorg/enterprise-api/internal/modules/incident/service"
)

func RegisterRoutes(router *gin.Engine, db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) {
	repo := incidentRepo.NewIncidentRepository(db)
	svc := incidentService.NewIncidentService(repo)
	handler := incidentHandler.NewIncidentHandler(svc)

	inc := router.Group("/api/v1/incidents")
	inc.Use(middleware.AuthRequired(redisClient))
	{
		inc.POST("/", middleware.RequirePermission(db, "create", "incident"), handler.Create)
		inc.GET("/", middleware.RequirePermission(db, "read", "incident"), handler.List)
	}
}
