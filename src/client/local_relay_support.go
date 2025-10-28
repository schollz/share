package client

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/schollz/share/src/discovery"
)

const (
	// Connection timeouts
	peerDiscoveryTimeout  = 2 * time.Second
	internetConnTimeout   = 5 * time.Second
	localPeerConnTimeout  = 2 * time.Second
	localDiscoveryWait    = 1 * time.Second
	localFallbackWait     = 3 * time.Second
	overallConnectTimeout = 6 * time.Second
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

	// Channel to signal when local peer is found
	localPeerFound := make(chan bool, 1)

	// Start peer discovery in background
	if localRelay != nil && enableLocal {
		go func() {
			// Try to discover peers
			peers, err := discovery.DiscoverPeers(roomID, peerDiscoveryTimeout)
			if err != nil {
				log.Printf("Peer discovery: no local peers found")
				return
			}

			if len(peers) > 0 {
				// Found local peers, try to connect to their relay
				for _, peer := range peers {
					conn, err := ConnectWithTimeout(peer.RelayURL, localPeerConnTimeout)
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
					localPeerFound <- true
					break // Only connect to one peer relay
				}
			}
		}()
	}

	// Connect to internet relay (in parallel with discovery)
	internetConnChan := make(chan *websocket.Conn, 1)
	internetErrChan := make(chan error, 1)

	go func() {
		conn, err := ConnectWithTimeout(serverURL, internetConnTimeout)
		if err != nil {
			internetErrChan <- err
		} else {
			internetConnChan <- conn
		}
	}()

	// Wait for either internet connection or timeout
	select {
	case conn := <-internetConnChan:
		cm.AddConnection(conn, ConnectionTypeInternet, serverURL)
		// Give local discovery a bit more time if enabled
		// This allows preferring local connection if one is found quickly
		if enableLocal {
			select {
			case <-localPeerFound:
				// Local peer found, both connections active (local is preferred)
			case <-time.After(localDiscoveryWait):
				// Timeout waiting for local peer, continue with internet only
			}
		}
	case err := <-internetErrChan:
		// Internet connection failed, wait for local discovery
		if enableLocal {
			select {
			case <-localPeerFound:
				log.Printf("Using local relay only (internet relay unavailable)")
			case <-time.After(localFallbackWait):
				return nil, fmt.Errorf("failed to connect to internet relay and no local relay available: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to connect to internet relay: %w", err)
		}
	case <-time.After(overallConnectTimeout):
		// Overall timeout
		return nil, fmt.Errorf("timeout waiting for relay connections")
	}

	return cm, nil
}
