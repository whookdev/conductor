package conductor

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

type Conductor struct {
	cfg    *config.Config
	rdb    *redis.Client
	logger *slog.Logger
}

type ServerInfo struct {
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Load          int       `json:"load"`
	RelayUrl      string    `json:"relay_url"`
}

func New(cfg *config.Config, redis *redis.Client, logger *slog.Logger) (*Conductor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if redis == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	logger = logger.With("component", "conductor")

	tc := &Conductor{
		cfg:    cfg,
		logger: logger,
		rdb:    redis,
	}

	return tc, nil
}

func (c *Conductor) AssignTunnelServer(projectName string) (string, error) {
	serverInfos, err := c.rdb.HGetAll(context.Background(), c.cfg.RelayRegistryKey).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get server info: %w", err)
	}

	var selectedServer string
	minLoad := math.MaxInt32

	for serverID, info := range serverInfos {
		var serverInfo ServerInfo
		err := json.Unmarshal([]byte(info), &serverInfo)
		if err != nil {
			c.logger.Error("failed to umarshal server info", "error", err, "server_id", serverID, "raw_info", info)
			continue
		}

		if time.Since(serverInfo.LastHeartbeat) > 30*time.Second {
			c.logger.Info("invalid heartbeat", "serverID", serverID, "server info", serverInfo)
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

	err = c.rdb.HSet(context.Background(),
		c.cfg.RelayAssignmentKey,
		projectName,
		selectedServer,
	).Err()

	return selectedServer, err
}

func (c *Conductor) GetProjectRelayServer(projectName string) (string, error) {
	relayServer, err := c.rdb.HGet(context.Background(),
		c.cfg.RelayAssignmentKey,
		projectName).Result()
	if err != nil {
		return "", fmt.Errorf("unable to find relay server assigned to project: %w", err)
	}

	var serverInfo ServerInfo
	info, err := c.rdb.HGet(context.Background(),
		c.cfg.RelayRegistryKey,
		relayServer).Result()
	if err != nil {
		return "", fmt.Errorf("unable to fetch relay server info: %w", err)
	}

	if err := json.Unmarshal([]byte(info), &serverInfo); err != nil {
		return "", fmt.Errorf("failed to unmarshal server info")
	}

	if serverInfo.RelayUrl == "" {
		return "", errors.New("server info does not contain a relay url")
	}

	return serverInfo.RelayUrl, nil
}
