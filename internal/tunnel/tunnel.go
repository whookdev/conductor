package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

type TunnelCoordinator struct {
	redisConn interface{}
}

type ServerInfo struct {
	LastHeartbeat time.Time
	Load          int
}

func (tc *TunnelCoordinator) AssignTunnelServer(tunnelID string) (string, error) {
	serverInfos, err := tc.redisConn.HGetAll(context.Background(), "tunnel_servers").Result()
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

	err = tc.redisConn.HSet(context.Background(),
		"tunnel_assignments",
		tunnelID,
		selectedServer,
	).Err()

	return selectedServer, err
}
