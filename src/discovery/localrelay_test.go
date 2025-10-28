package discovery

import (
	"testing"
	"time"
)

func TestGetOrCreateLocalRelay(t *testing.T) {
	// Reset global manager for test
	managerMutex.Lock()
	globalManager = nil
	managerMutex.Unlock()

	manager, err := GetOrCreateLocalRelay()
	if err != nil {
		t.Fatalf("Failed to create local relay: %v", err)
	}

	if !manager.IsRunning {
		t.Error("Manager should be running")
	}

	if manager.Port == 0 {
		t.Error("Manager should have a valid port")
	}

	if manager.ServerURL == "" {
		t.Error("Manager should have a valid server URL")
	}

	// Test that getting the manager again returns the same instance
	manager2, err := GetOrCreateLocalRelay()
	if err != nil {
		t.Fatalf("Failed to get existing local relay: %v", err)
	}

	if manager.Port != manager2.Port {
		t.Error("Second call should return same manager instance")
	}

	// Cleanup
	manager.Shutdown()
}

func TestDiscoverPeers(t *testing.T) {
	// This test is expected to find no peers in most environments
	// We're just testing that the discovery doesn't crash

	// Setup local relay first
	managerMutex.Lock()
	globalManager = nil
	managerMutex.Unlock()

	_, err := GetOrCreateLocalRelay()
	if err != nil {
		t.Skipf("Skipping peer discovery test due to local relay setup failure: %v", err)
	}

	roomID := "test-room-discovery"
	timeout := 1 * time.Second

	peers, err := DiscoverPeers(roomID, timeout)
	// Note: err might be nil or might indicate no peers found - both are ok
	if err != nil {
		t.Logf("Peer discovery completed with: %v (expected if no peers available)", err)
	}

	if len(peers) > 0 {
		t.Logf("Found %d peers (unexpected in test environment)", len(peers))
	} else {
		t.Log("No peers found (expected in test environment)")
	}
}

func TestLocalRelayManagerShutdown(t *testing.T) {
	// Reset global manager for test
	managerMutex.Lock()
	globalManager = nil
	managerMutex.Unlock()

	manager, err := GetOrCreateLocalRelay()
	if err != nil {
		t.Fatalf("Failed to create local relay: %v", err)
	}

	if !manager.IsRunning {
		t.Fatal("Manager should be running after creation")
	}

	manager.Shutdown()

	if manager.IsRunning {
		t.Error("Manager should not be running after shutdown")
	}
}
