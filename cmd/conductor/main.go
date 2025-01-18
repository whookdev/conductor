package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/whookdev/conductor/internal/conductor"
	"github.com/whookdev/conductor/internal/config"
	"github.com/whookdev/conductor/internal/redis"
	"github.com/whookdev/conductor/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := initiateApp(logger); err != nil {
		logger.Error("error in app lifecycle", "error", err)
	}
}

func initiateApp(logger *slog.Logger) error {
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	rdb, err := redis.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("creating redis client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	if err := rdb.Start(ctx); err != nil {
		return fmt.Errorf("reaching redis server: %w", err)
	}
	defer func() error {
		if err := rdb.Stop(); err != nil {
			return fmt.Errorf("stopping redis client: %w", err)
		}
		return nil
	}()

	c, err := conductor.New(cfg, rdb.Client, logger)
	if err != nil {
		return fmt.Errorf("creating coordinator: %w", err)
	}

	c.StartCleanupRoutine(ctx)

	srv, err := server.New(cfg, c, logger)
	if err != nil {
		return fmt.Errorf("creating HTTP server: %w", err)
	}

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	return nil
}
