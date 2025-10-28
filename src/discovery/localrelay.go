package discovery

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/schollz/peerdiscovery"
	"github.com/schollz/share/src/relay"
)

// LocalRelayManager manages local relay server and peer discovery
type LocalRelayManager struct {
	Port         int
	IsRunning    bool
	ServerURL    string
	mutex        sync.Mutex
	cancelServer context.CancelFunc
	logger       *slog.Logger
}

// PeerInfo contains information about discovered peers
type PeerInfo struct {
	RelayURL string
	RoomID   string
	Address  string
}

var (
	globalManager *LocalRelayManager
	managerMutex  sync.Mutex
)

// GetOrCreateLocalRelay returns the global local relay manager, starting it if necessary
func GetOrCreateLocalRelay() (*LocalRelayManager, error) {
	managerMutex.Lock()
	defer managerMutex.Unlock()

	if globalManager != nil && globalManager.IsRunning {
		return globalManager, nil
	}

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	manager := &LocalRelayManager{
		Port:      port,
		ServerURL: fmt.Sprintf("ws://localhost:%d", port),
		logger:    nil,
	}

	// Start the relay server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	manager.cancelServer = cancel

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Use a silent logger for local relay to avoid cluttering output
		silentLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
		// Use empty embed.FS since we don't need web assets for local relay
		var emptyFS embed.FS
		relay.Start(port, 0, 0, emptyFS, silentLogger)
	}()

	// Give the server a moment to start
	time.Sleep(500 * time.Millisecond)

	manager.IsRunning = true
	globalManager = manager

	log.Printf("Local relay started on %s", manager.ServerURL)

	return manager, nil
}

// DiscoverPeers searches for local peers in the specified room
func DiscoverPeers(roomID string, timeout time.Duration) ([]PeerInfo, error) {
	var discoveredPeers []PeerInfo
	var mutex sync.Mutex

	// Create payload with room ID and local relay port
	localManager := globalManager
	payload := []byte(fmt.Sprintf("share:%s:%d", roomID, localManager.Port))

	discoveries, err := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1, // Find all peers
		TimeLimit: timeout,
		Delay:     500 * time.Millisecond,
		Payload:   payload,
		AllowSelf: false, // Don't discover ourselves
	})

	if err != nil {
		return nil, fmt.Errorf("peer discovery failed: %w", err)
	}

	for _, d := range discoveries {
		// Parse the payload to extract room ID and port
		var peerRoom string
		var peerPort int
		n, err := fmt.Sscanf(string(d.Payload), "share:%s:%d", &peerRoom, &peerPort)
		if err != nil || n != 2 {
			log.Printf("Failed to parse peer payload: %s", string(d.Payload))
			continue
		}

		// Only add peers in the same room
		if peerRoom == roomID {
			mutex.Lock()
			discoveredPeers = append(discoveredPeers, PeerInfo{
				RelayURL: fmt.Sprintf("ws://%s:%d", d.Address, peerPort),
				RoomID:   peerRoom,
				Address:  d.Address,
			})
			mutex.Unlock()
		}
	}

	return discoveredPeers, nil
}

// Shutdown stops the local relay server
func (m *LocalRelayManager) Shutdown() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cancelServer != nil {
		m.cancelServer()
	}
	m.IsRunning = false
}
