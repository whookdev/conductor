package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/whookdev/conductor/internal/config"
)

type Coordinator struct {
	cfg    *config.Config
	rdb    *redis.Client
	logger *slog.Logger
}

type ServerInfo struct {
	LastHeartbeat time.Time
	Load          int
}

func New(cfg *config.Config, redis *redis.Client) (*Coordinator, error) {
	logger := slog.With("component", "tunnel_coordinator")

	tc := &Coordinator{
		cfg:    cfg,
		logger: logger,
		rdb:    redis,
	}

	return tc, nil
}

func (tc *Coordinator) AssignTunnelServer(tunnelID string) (string, error) {
	serverInfos, err := tc.rdb.HGetAll(context.Background(), "tunnel_servers").Result()
	if err != nil {
		return "", fmt.Errorf("failed to get server info: %w", err)
	}

	var selectedServer string
	minLoad := math.MaxInt32

	for serverID, info := range serverInfos {
		var serverInfo ServerInfo
		json.Unmarshal([]byte(info), &serverInfo)

		if time.Since(serverInfo.LastHeartbeat) > 30*time.Second {
			continue
		}

		if serverInfo.Load < minLoad {
			minLoad = serverInfo.Load
			selectedServer = serverID
		}
	}

	if selectedServer == "" {
		return "", errors.New("no available tunnel servers")
	}

	err = tc.rdb.HSet(context.Background(),
		"tunnel_assignments",
		tunnelID,
		selectedServer,
	).Err()

	return selectedServer, err
}
