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
	"github.com/whookdev/conductor/internal/models"
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
	RelayWSUrl    string    `json:"relay_ws_url"`
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

func (c *Conductor) AssignRelayServer(projectName string) (*models.RelayAssignment, error) {
	serverInfos, err := c.rdb.HGetAll(context.Background(), c.cfg.RelayRegistryKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	var (
		selectedServerID   string
		selectedServerInfo ServerInfo
		minLoad            = math.MaxInt32
	)

	for serverID, info := range serverInfos {
		var serverInfo ServerInfo
		err := json.Unmarshal([]byte(info), &serverInfo)
		if err != nil {
			c.logger.Error("failed to unmarshal server info",
				"error", err,
				"server_id", serverID,
				"raw_info", info)
			continue
		}

		if time.Since(serverInfo.LastHeartbeat) > 30*time.Second {
			c.logger.Debug("skipping stale server",
				"server_id", serverID,
				"last_heartbeat", serverInfo.LastHeartbeat)
			continue
		}

		if serverInfo.Load < minLoad {
			minLoad = serverInfo.Load
			selectedServerID = serverID
			selectedServerInfo = serverInfo
		}
	}

	if selectedServerID == "" {
		return nil, errors.New("no available relay servers")
	}

	err = c.rdb.HSet(context.Background(),
		c.cfg.RelayAssignmentKey,
		projectName,
		selectedServerID,
	).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set relay assignment: %w", err)
	}

	assignment := &models.RelayAssignment{
		RelayID:    selectedServerID,
		RelayWSURL: selectedServerInfo.RelayWSUrl,
	}

	c.logger.Info("assigned relay server",
		"project", projectName,
		"server_id", selectedServerID,
		"relay_url", selectedServerInfo.RelayUrl,
		"relay_ws_url", selectedServerInfo.RelayWSUrl,
		"load", minLoad)

	return assignment, nil
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

// TODO: Consider moving this to a separate service, as there will be multiple
// conductors running in production, started at different times, we could run
// into the scenario where relays are getting reassigned when they're still
// alive, which would leave them in a unusable state but they should still send
// hearbeats which means they would reassign themselves after 15 seconds -
// perhaps this isn't an issue?
func (c *Conductor) StartCleanupRoutine(ctx context.Context) chan struct{} {
	done := make(chan struct{})

	c.logger.Info("starting cleanup routine")

	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Duration(30 * time.Second))
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := c.cleanupDeadRelays(); err != nil {
					c.logger.Error("failed cleanup", "error", err)
				}
			case <-ctx.Done():
				c.logger.Info("context cancelled, stopping cleanup routine")
				return
			}
		}
	}()

	return done
}

func (c *Conductor) cleanupDeadRelays() error {
	serverInfos, err := c.rdb.HGetAll(context.Background(), c.cfg.RelayRegistryKey).Result()
	if err != nil {
		return fmt.Errorf("unable to fetch relays: %w", err)
	}

	for serverID, info := range serverInfos {
		var serverInfo ServerInfo
		json.Unmarshal([]byte(info), &serverInfo)

		if time.Since(serverInfo.LastHeartbeat) > 30*time.Second {
			c.logger.Warn("relay unreachable", "relay_id", serverID)
			c.reassignRelay(serverID)
			c.rdb.HDel(context.Background(), c.cfg.RelayRegistryKey, serverID)
		}
	}

	return nil
}

// TODO: When the CLI loses connection with the relay server it's going to need
// to query the API to find the new websocket connection URL - so we need to
// add a method to find the current relay for a given project, we only support
// creating a new assignment and reassigning dead ones currently
func (c *Conductor) reassignRelay(serverID string) error {
	c.logger.Info("reassigning relay", "relay_id", serverID)

	assignments, err := c.rdb.HGetAll(context.Background(), c.cfg.RelayAssignmentKey).Result()
	if err != nil {
		return fmt.Errorf("unable to fetch assignments: %w", err)
	}

	for projectName, assignedRelayID := range assignments {
		if assignedRelayID == serverID {
			c.logger.Info("found project to reassign",
				"project", projectName,
				"old_relay", serverID)

			newRelay, err := c.AssignRelayServer(projectName)
			if err != nil {
				c.logger.Error("failed to reassign project to new relay",
					"project", projectName,
					"error", err)

				if err := c.rdb.HDel(context.Background(),
					c.cfg.RelayAssignmentKey,
					projectName).Err(); err != nil {
					c.logger.Error("failed to delete stale assignment",
						"project", projectName,
						"error", err)
				}
				continue
			}

			c.logger.Info("successfully reassigned project",
				"project", projectName,
				"old_relay", serverID,
				"new_relay", newRelay.RelayID,
				"new_relay_ws_url", newRelay.RelayWSURL)
		}
	}

	return nil
}
