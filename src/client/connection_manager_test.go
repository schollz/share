package client

import (
	"testing"
	"time"
)

func TestNewConnectionManager(t *testing.T) {
	cm := NewConnectionManager()
	if cm == nil {
		t.Fatal("NewConnectionManager should return a non-nil manager")
	}

	if cm.Connections == nil {
		t.Error("Connections slice should be initialized")
	}

	if cm.PreferredConn != nil {
		t.Error("PreferredConn should be nil initially")
	}

	cm.Close()
}

func TestConnectionManagerPreference(t *testing.T) {
	cm := NewConnectionManager()
	defer cm.Close()

	// Test that GetPreferredConnectionType returns empty string when no connection
	connType := cm.GetPreferredConnectionType()
	if connType != "" {
		t.Errorf("Expected empty connection type, got %s", connType)
	}
}

func TestConnectWithTimeout(t *testing.T) {
	// Test connection timeout with invalid URL
	_, err := ConnectWithTimeout("ws://invalid-host-that-does-not-exist:9999", 1*time.Second)
	if err == nil {
		t.Error("Expected error when connecting to invalid host")
	}
}
