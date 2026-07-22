package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xinyuan-js/myblog/apps/api/internal/auth"
	"github.com/xinyuan-js/myblog/apps/api/internal/blog"
	"github.com/xinyuan-js/myblog/apps/api/internal/config"
	"github.com/xinyuan-js/myblog/apps/api/internal/database"
	"github.com/xinyuan-js/myblog/apps/api/internal/httpapi"
	"github.com/xinyuan-js/myblog/apps/api/internal/server"
	"github.com/xinyuan-js/myblog/apps/api/internal/storage"
	"github.com/xinyuan-js/myblog/apps/api/internal/upload"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	startupContext, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startupCancel()
	db, err := database.Open(startupContext, cfg)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := database.Migrate(startupContext, db); err != nil {
		logger.Error("run database migrations", "error", err)
		os.Exit(1)
	}
	blogStore := blog.NewStore(db, cfg.MediaPublicURL)
	authService := auth.NewService(db, cfg)
	minioStore, err := storage.NewMinIO(cfg)
	if err != nil {
		logger.Error("create MinIO client", "error", err)
		os.Exit(1)
	}
	if err := minioStore.EnsureBucket(startupContext); err != nil {
		logger.Error("prepare MinIO bucket", "error", err)
		os.Exit(1)
	}
	uploadService := upload.NewService(db, minioStore, cfg)

	router, err := httpapi.NewRouter(cfg, logger, httpapi.Dependencies{
		DB: db, PublicStore: blogStore, AdminStore: blogStore, Auth: authService, Uploads: uploadService, MinIOReady: minioStore.Ready,
	})
	if err != nil {
		logger.Error("create HTTP router", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go runMaintenance(ctx, db, uploadService, logger)

	logger.Info("starting API server", "address", cfg.HTTPAddr, "environment", cfg.Environment)
	if err := server.Run(ctx, cfg, router, logger); err != nil {
		logger.Error("API server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
	logger.Info("API server stopped")
}

func runMaintenance(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, uploads *upload.Service, logger *slog.Logger) {
	run := func() {
		maintenanceContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if _, err := db.ExecContext(maintenanceContext, `DELETE FROM admin_sessions WHERE expires_at <= UTC_TIMESTAMP(6)`); err != nil {
			logger.Error("clean expired sessions", "error", err)
		}
		if _, err := db.ExecContext(maintenanceContext, `DELETE FROM oauth_states WHERE expires_at <= UTC_TIMESTAMP(6)`); err != nil {
			logger.Error("clean expired oauth states", "error", err)
		}
		if err := uploads.CleanupTrashed(maintenanceContext, time.Now().UTC().Add(-30*24*time.Hour), 100); err != nil {
			logger.Error("clean trashed uploads", "error", err)
		}
	}
	run()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
