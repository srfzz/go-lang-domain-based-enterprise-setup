package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/database"
	"github.com/yourorg/enterprise-api/internal/modules/admin/service"
	"github.com/yourorg/enterprise-api/internal/router"
	"github.com/yourorg/enterprise-api/internal/shared/logger"
	"github.com/yourorg/enterprise-api/internal/shared/storage"
	"github.com/yourorg/enterprise-api/internal/shared/utils"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	logger.Init(cfg.LogLevel, cfg.LogFilePath)
	defer logger.Sync()

	logger.Info("starting application", zap.String("app", cfg.AppName), zap.String("env", cfg.AppEnv))

	if err := utils.LoadKeys(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath); err != nil {
		logger.Fatal("failed to load JWT keys", zap.Error(err))
	}

	db, err := database.NewPostgresPool(cfg)
	if err != nil {
		logger.Fatal("database connection failed", zap.Error(err))
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		logger.Fatal("migration failed", zap.Error(err))
	}
	logger.Info("migrations completed")

	redisClient := database.NewRedisClient(cfg)
	defer redisClient.Close()

	// Initialize file storage backend
	store, err := storage.NewFromConfig(cfg)
	if err != nil {
		logger.Fatal("storage backend init failed", zap.Error(err))
	}
	logger.Info("storage backend initialized", zap.String("driver", cfg.StorageDriver))

	_ = store

	// Seed default admin user, roles, and permissions
	adminSvc := service.NewAdminService(db, redisClient, cfg)
	adminSvc.SeedDefaultAdmin(context.Background())

	r := router.Setup(cfg, db, redisClient)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server listening", zap.String("port", cfg.AppPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}
	logger.Info("server exited gracefully")
}
