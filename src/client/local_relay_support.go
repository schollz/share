package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/schollz/share/src/discovery"
)

// SetupConnections sets up both local and internet relay connections
func SetupConnections(roomID, serverURL string, enableLocal bool) (*ConnectionManager, error) {
	cm := NewConnectionManager()

	var localRelay *discovery.LocalRelayManager
	var err error

	// Start local relay if enabled
	if enableLocal {
		localRelay, err = discovery.GetOrCreateLocalRelay()
		if err != nil {
			log.Printf("Warning: Failed to start local relay: %v", err)
		}
	}

	// Start peer discovery in background
	if localRelay != nil && enableLocal {
		go func() {
			_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// Try to discover peers
			peers, err := discovery.DiscoverPeers(roomID, 3*time.Second)
			if err != nil {
				log.Printf("Peer discovery: no local peers found")
				return
			}

			if len(peers) > 0 {
				// Found local peers, try to connect to their relay
				for _, peer := range peers {
					conn, err := ConnectWithTimeout(peer.RelayURL, 2*time.Second)
					if err != nil {
						log.Printf("Failed to connect to peer relay %s: %v", peer.RelayURL, err)
						continue
					}

					cm.AddConnection(conn, ConnectionTypeLocal, peer.RelayURL)
					log.Printf("✓ Using local relay connection")

					// Join the room through this connection
					joinMsg := map[string]interface{}{
						"type":   "join",
						"roomId": roomID,
					}
					sendProtobufMessage(conn, joinMsg)
					break // Only connect to one peer relay
				}
			}
		}()
	}

	// Connect to internet relay
	internetConn, err := ConnectWithTimeout(serverURL, 5*time.Second)
	if err != nil {
		// If we can't connect to internet relay, check if we have a local connection
		if enableLocal {
			time.Sleep(4 * time.Second) // Wait a bit more for local discovery
			cm.mutex.RLock()
			hasLocalConn := cm.PreferredConn != nil && cm.PreferredConn.Type == ConnectionTypeLocal
			cm.mutex.RUnlock()

			if !hasLocalConn {
				return nil, fmt.Errorf("failed to connect to internet relay and no local relay available: %w", err)
			}
			log.Printf("Using local relay only (internet relay unavailable)")
		} else {
			return nil, fmt.Errorf("failed to connect to internet relay: %w", err)
		}
	} else {
		cm.AddConnection(internetConn, ConnectionTypeInternet, serverURL)
	}

	// Wait a moment for potential local discovery
	if enableLocal {
		time.Sleep(2 * time.Second)
	}

	return cm, nil
}
