package redis

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
	redisi "github.com/redis/go-redis/v9"
	"github.com/whookdev/conductor/internal/config"
)

type RedisServer struct {
	cfg    *config.Config
	client *redisi.Client
	logger *slog.Logger
}

func New(cfg *config.Config) (*RedisServer, error) {
	logger := slog.With("component", "redis")

	rs := &RedisServer{
		cfg:    cfg,
		logger: logger,
	}

	return rs, nil
}

func (rs *RedisServer) Start(ctx context.Context) error {
	rs.client = redis.NewClient(&redis.Options{
		Addr:     rs.cfg.RedisURL,
		Password: "",
		DB:       0,
	})

	if err := rs.client.Ping(ctx).Err(); err != nil {
		rs.logger.Error("failed to connect to redis", "error", err)
		return err
	}

	rs.logger.Info("redis connection established successfully", "addr", rs.cfg.RedisURL)
	return nil
}

func (rs *RedisServer) Stop() error {
	if rs.client != nil {
		if err := rs.client.Close(); err != nil {
			rs.logger.Error("failed to close redis connection", "error", err)
			return err
		}
		rs.logger.Info("redis connection closed successfully")
	}
	return nil
}
