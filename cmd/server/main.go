package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/surajkadam7/iot-backend/internal/api"
	"github.com/surajkadam7/iot-backend/internal/auth"
	"github.com/surajkadam7/iot-backend/internal/config"
	"github.com/surajkadam7/iot-backend/internal/db"
	"github.com/surajkadam7/iot-backend/internal/iot"
	"github.com/surajkadam7/iot-backend/internal/observability"
	"github.com/surajkadam7/iot-backend/internal/repository"
	"github.com/surajkadam7/iot-backend/internal/state"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
	"github.com/surajkadam7/iot-backend/internal/ws"
	"github.com/surajkadam7/iot-backend/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := observability.NewLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Ping(ctx, pool); err != nil {
		return err
	}
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		return err
	}

	repo := repository.NewPostgres(pool)
	store := state.New()
	validator, err := auth.NewValidator(cfg.AuthMode, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTJWKSURL)
	if err != nil {
		return err
	}
	hub := ws.NewHub(validator, repo, store, cfg.FrontendOrigin, log)
	ingest := telemetry.New(repo, store, hub, log)

	if cfg.MQTTEnabled {
		sub, err := iot.NewSubscriber(cfg, ingest, log)
		if err != nil {
			return err
		}
		if err := sub.Start(ctx); err != nil {
			return err
		}
		log.Info("mqtt subscriber started")
	} else {
		log.Info("mqtt disabled")
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.New(cfg, log, pool, repo, store, validator, hub).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
