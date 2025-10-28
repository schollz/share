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
		log.Printf("[DEBUG] Local relay already running on %s", globalManager.ServerURL)
		return globalManager, nil
	}

	// Find an available port - bind to 0.0.0.0 to allow connections from other machines
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Printf("[DEBUG] Failed to find available port: %v", err)
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	log.Printf("[DEBUG] Reserved port %d for local relay", port)

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
		log.Printf("[DEBUG] Starting local relay server on port %d", port)
		relay.Start(port, 0, 0, emptyFS, silentLogger)
	}()

	// Give the server a moment to start
	time.Sleep(500 * time.Millisecond)

	manager.IsRunning = true
	globalManager = manager

	log.Printf("[LOCAL RELAY] Started on port %d (accessible at ws://0.0.0.0:%d)", port, port)

	return manager, nil
}

// DiscoverPeers searches for local peers in the specified room
func DiscoverPeers(roomID string, timeout time.Duration) ([]PeerInfo, error) {
	var discoveredPeers []PeerInfo
	var mutex sync.Mutex

	// Create payload with room ID and local relay port
	localManager := globalManager
	if localManager == nil {
		log.Printf("[DEBUG] DiscoverPeers called but no local relay manager exists")
		return nil, fmt.Errorf("no local relay manager")
	}

	payload := []byte(fmt.Sprintf("share:%s:%d", roomID, localManager.Port))
	log.Printf("[PEER DISCOVERY] Starting discovery for room '%s' (broadcasting port %d, timeout %v)", roomID, localManager.Port, timeout)

	discoveries, err := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1, // Find all peers
		TimeLimit: timeout,
		Delay:     500 * time.Millisecond,
		Payload:   payload,
		AllowSelf: false, // Don't discover ourselves
	})

	if err != nil {
		log.Printf("[PEER DISCOVERY] Discovery failed: %v", err)
		return nil, fmt.Errorf("peer discovery failed: %w", err)
	}

	log.Printf("[PEER DISCOVERY] Received %d discovery responses", len(discoveries))

	for i, d := range discoveries {
		log.Printf("[PEER DISCOVERY] Response %d: Address=%s, Payload=%s", i+1, d.Address, string(d.Payload))
		
		// Parse the payload to extract room ID and port
		var peerRoom string
		var peerPort int
		n, err := fmt.Sscanf(string(d.Payload), "share:%s:%d", &peerRoom, &peerPort)
		if err != nil || n != 2 {
			log.Printf("[PEER DISCOVERY] Failed to parse peer payload: %s (error: %v)", string(d.Payload), err)
			continue
		}

		log.Printf("[PEER DISCOVERY] Parsed: room=%s, port=%d", peerRoom, peerPort)

		// Only add peers in the same room
		if peerRoom == roomID {
			peerInfo := PeerInfo{
				RelayURL: fmt.Sprintf("ws://%s:%d", d.Address, peerPort),
				RoomID:   peerRoom,
				Address:  d.Address,
			}
			mutex.Lock()
			discoveredPeers = append(discoveredPeers, peerInfo)
			mutex.Unlock()
			log.Printf("[PEER DISCOVERY] ✓ Found matching peer: %s (room: %s)", peerInfo.RelayURL, peerRoom)
		} else {
			log.Printf("[PEER DISCOVERY] Ignoring peer in different room: %s (expected: %s)", peerRoom, roomID)
		}
	}

	if len(discoveredPeers) == 0 {
		log.Printf("[PEER DISCOVERY] No peers found in room '%s'", roomID)
	} else {
		log.Printf("[PEER DISCOVERY] Found %d peer(s) in room '%s'", len(discoveredPeers), roomID)
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
