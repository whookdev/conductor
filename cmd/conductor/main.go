package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/whookdev/conductor/internal/config"
	"github.com/whookdev/conductor/internal/redis"
	"github.com/whookdev/conductor/internal/server"
	"github.com/whookdev/conductor/internal/tunnel"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.NewConfig()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	rdb, err := redis.New(cfg)
	if err != nil {
		logger.Error("failed to create redis client", "error", err)
	}

	tc, err := tunnel.New(cfg, rdb.Client)
	if err != nil {
		logger.Error("failed to create tunnel coordinator", "error", err)
	}

	srv, err := server.New(cfg, tc)
	if err != nil {
		logger.Error("failed to create server", "error", err)
		os.Exit(1)
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
		logger.Error("unable to connect to redis server", "error", err)
		os.Exit(1)
	}
	defer rdb.Stop()

	if err := srv.Start(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
