package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud/backend/internal/application/bootstrap"
	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/infrastructure/config"
	"cloud/backend/internal/infrastructure/logger"
	infraPg "cloud/backend/internal/infrastructure/postgres"
	infraRedis "cloud/backend/internal/infrastructure/redis"
	"cloud/backend/internal/infrastructure/security/password"
	httpiface "cloud/backend/internal/interfaces/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", slog.Any("error", err))
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.LogFormat)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgPool, err := infraPg.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer pgPool.Close()

	if err := infraPg.EnsurePhase4Schema(ctx, pgPool); err != nil {
		log.Error("failed to ensure runtime schema", slog.Any("error", err))
		os.Exit(1)
	}

	redisClient, err := infraRedis.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("failed to connect redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer redisClient.Close()

	userRepo := infraPg.NewUserRepository(pgPool)
	quotaRepo := infraPg.NewQuotaRepository(pgPool)
	if err := quotaRepo.UpsertDefaultUserQuota(ctx, cfg.DefaultUserQuotaBytes); err != nil {
		log.Error("failed to upsert default user quota", slog.Any("error", err))
		os.Exit(1)
	}

	adminSeeder := bootstrap.NewAdminSeeder(userRepo, password.NewArgon2IDHasher())
	if err := adminSeeder.Ensure(ctx, bootstrap.AdminSeedInput{
		Email:       cfg.BootstrapAdminEmail,
		Password:    cfg.BootstrapAdminPassword,
		DisplayName: cfg.BootstrapAdminName,
		Role:        identity.Role(cfg.BootstrapAdminRole),
		QuotaBytes:  nil,
	}); err != nil {
		log.Error("admin bootstrap failed", slog.Any("error", err))
		os.Exit(1)
	}
	if cfg.BootstrapAdminEmail != "" {
		log.Info("admin bootstrap evaluated", slog.String("email", cfg.BootstrapAdminEmail))
	}

	router, err := httpiface.NewRouter(log, cfg, pgPool, redisClient)
	if err != nil {
		log.Error("failed to build router", slog.Any("error", err))
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("api server listening", slog.String("addr", cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server stopped unexpectedly", slog.Any("error", err))
			cancel()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("received shutdown signal", slog.String("signal", sig.String()))
	case <-ctx.Done():
		log.Warn("context canceled, shutting down")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	log.Info("api shutdown complete")
}
