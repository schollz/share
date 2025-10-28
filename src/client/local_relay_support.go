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
	log.Printf("[CONNECTION SETUP] Starting connection setup for room '%s'", roomID)
	log.Printf("[CONNECTION SETUP] Internet relay: %s", serverURL)
	log.Printf("[CONNECTION SETUP] Local relay enabled: %v", enableLocal)
	
	cm := NewConnectionManager()

	var localRelay *discovery.LocalRelayManager
	var err error

	// Start local relay if enabled
	if enableLocal {
		log.Printf("[CONNECTION SETUP] Attempting to start/get local relay...")
		localRelay, err = discovery.GetOrCreateLocalRelay()
		if err != nil {
			log.Printf("[CONNECTION SETUP] Warning: Failed to start local relay: %v", err)
		} else {
			log.Printf("[CONNECTION SETUP] Local relay ready: %s", localRelay.ServerURL)
		}
	}

	// Channel to signal when local peer is found
	localPeerFound := make(chan bool, 1)

	// Start peer discovery in background
	if localRelay != nil && enableLocal {
		log.Printf("[CONNECTION SETUP] Starting peer discovery in background...")
		go func() {
			log.Printf("[DISCOVERY GOROUTINE] Starting discovery for room '%s'", roomID)
			// Try to discover peers
			peers, err := discovery.DiscoverPeers(roomID, peerDiscoveryTimeout)
			if err != nil {
				log.Printf("[DISCOVERY GOROUTINE] Discovery error: %v", err)
				return
			}

			log.Printf("[DISCOVERY GOROUTINE] Discovery returned %d peers", len(peers))
			
			if len(peers) > 0 {
				// Found local peers, try to connect to their relay
				for i, peer := range peers {
					log.Printf("[DISCOVERY GOROUTINE] Attempting to connect to peer %d: %s", i+1, peer.RelayURL)
					conn, err := ConnectWithTimeout(peer.RelayURL, localPeerConnTimeout)
					if err != nil {
						log.Printf("[DISCOVERY GOROUTINE] Failed to connect to peer relay %s: %v", peer.RelayURL, err)
						continue
					}

					log.Printf("[DISCOVERY GOROUTINE] Successfully connected to peer relay %s", peer.RelayURL)
					cm.AddConnection(conn, ConnectionTypeLocal, peer.RelayURL)
					log.Printf("[DISCOVERY GOROUTINE] ✓ Using local relay connection")

					// Join the room through this connection
					joinMsg := map[string]interface{}{
						"type":   "join",
						"roomId": roomID,
					}
					sendProtobufMessage(conn, joinMsg)
					log.Printf("[DISCOVERY GOROUTINE] Sent join message to local relay")
					localPeerFound <- true
					break // Only connect to one peer relay
				}
			} else {
				log.Printf("[DISCOVERY GOROUTINE] No peers found")
			}
		}()
	} else {
		if !enableLocal {
			log.Printf("[CONNECTION SETUP] Local relay disabled by user")
		} else {
			log.Printf("[CONNECTION SETUP] Local relay not available, skipping discovery")
		}
	}

	// Connect to internet relay (in parallel with discovery)
	log.Printf("[CONNECTION SETUP] Connecting to internet relay in parallel...")
	internetConnChan := make(chan *websocket.Conn, 1)
	internetErrChan := make(chan error, 1)

	go func() {
		log.Printf("[INTERNET GOROUTINE] Attempting connection to %s", serverURL)
		conn, err := ConnectWithTimeout(serverURL, internetConnTimeout)
		if err != nil {
			log.Printf("[INTERNET GOROUTINE] Connection failed: %v", err)
			internetErrChan <- err
		} else {
			log.Printf("[INTERNET GOROUTINE] Connection successful")
			internetConnChan <- conn
		}
	}()

	// Wait for either internet connection or timeout
	log.Printf("[CONNECTION SETUP] Waiting for connections...")
	select {
	case conn := <-internetConnChan:
		log.Printf("[CONNECTION SETUP] Internet connection established")
		cm.AddConnection(conn, ConnectionTypeInternet, serverURL)
		// Give local discovery a bit more time if enabled
		// This allows preferring local connection if one is found quickly
		if enableLocal {
			log.Printf("[CONNECTION SETUP] Waiting %v for local peer discovery...", localDiscoveryWait)
			select {
			case <-localPeerFound:
				log.Printf("[CONNECTION SETUP] ✓ Local peer found! Using local relay")
			case <-time.After(localDiscoveryWait):
				log.Printf("[CONNECTION SETUP] Local discovery timeout, using internet relay")
			}
		}
	case err := <-internetErrChan:
		log.Printf("[CONNECTION SETUP] Internet connection failed: %v", err)
		// Internet connection failed, wait for local discovery
		if enableLocal {
			log.Printf("[CONNECTION SETUP] Waiting for local discovery (fallback mode)...")
			select {
			case <-localPeerFound:
				log.Printf("[CONNECTION SETUP] Using local relay only (internet relay unavailable)")
			case <-time.After(localFallbackWait):
				log.Printf("[CONNECTION SETUP] No local relay available either")
				return nil, fmt.Errorf("failed to connect to internet relay and no local relay available: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to connect to internet relay: %w", err)
		}
	case <-time.After(overallConnectTimeout):
		log.Printf("[CONNECTION SETUP] Overall timeout reached")
		// Overall timeout
		return nil, fmt.Errorf("timeout waiting for relay connections")
	}

	connType := cm.GetPreferredConnectionType()
	log.Printf("[CONNECTION SETUP] ✓ Setup complete. Active connections: %d, Preferred: %s", len(cm.Connections), connType)
	return cm, nil
}
