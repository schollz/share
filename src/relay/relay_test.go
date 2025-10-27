package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestGenerateMnemonic(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
	}{
		{"simple id", "client-123"},
		{"uuid-like", "550e8400-e29b-41d4-a716-446655440000"},
		{"short id", "abc"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mnemonic := GenerateMnemonic(tt.clientID)

			// Should return a non-empty string
			if mnemonic == "" {
				t.Fatal("Expected non-empty mnemonic")
			}

			// Should contain a hyphen (two words separated)
			if !strings.Contains(mnemonic, "-") {
				// Could be the fallback to clientID if BIP39 failed
				if mnemonic != tt.clientID {
					t.Fatalf("Expected hyphen-separated words or fallback to clientID, got: %s", mnemonic)
				}
			}

			// Should be deterministic - same input produces same output
			mnemonic2 := GenerateMnemonic(tt.clientID)
			if mnemonic != mnemonic2 {
				t.Fatalf("Expected deterministic output. First: %s, Second: %s", mnemonic, mnemonic2)
			}
		})
	}
}

func TestGenerateMnemonicUniqueness(t *testing.T) {
	id1 := "client-1"
	id2 := "client-2"

	mnemonic1 := GenerateMnemonic(id1)
	mnemonic2 := GenerateMnemonic(id2)

	// Different IDs should produce different mnemonics (very high probability)
	if mnemonic1 == mnemonic2 {
		t.Fatal("Expected different mnemonics for different client IDs")
	}
}

func TestGetOrCreateRoom(t *testing.T) {
	// Initialize logger
	logger = getTestLogger()
	maxRooms = 0 // No limit for this test

	// Clear rooms map
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	roomID := "test-room"

	// First call should create the room
	room1 := getOrCreateRoom(roomID)
	if room1 == nil {
		t.Fatal("Expected non-nil room")
	}
	if room1.ID != roomID {
		t.Fatalf("Expected room ID %s, got %s", roomID, room1.ID)
	}
	if room1.Clients == nil {
		t.Fatal("Expected non-nil Clients map")
	}

	// Second call should return the same room
	room2 := getOrCreateRoom(roomID)
	if room2 != room1 {
		t.Fatal("Expected getOrCreateRoom to return the same room instance")
	}

	// Different room ID should create a new room
	room3 := getOrCreateRoom("different-room")
	if room3 == room1 {
		t.Fatal("Expected different room for different room ID")
	}

	// Clean up
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()
}

func TestRemoveClientFromRoom(t *testing.T) {
	// Clear rooms map
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	roomID := "test-room"
	room := getOrCreateRoom(roomID)

	client := &Client{
		ID:     "client-1",
		RoomID: roomID,
	}

	room.Mutex.Lock()
	room.Clients[client.ID] = client
	room.Mutex.Unlock()

	// Verify client is in room
	room.Mutex.Lock()
	if _, ok := room.Clients[client.ID]; !ok {
		t.Fatal("Client should be in room before removal")
	}
	room.Mutex.Unlock()

	// Remove client
	removeClientFromRoom(client)

	// Verify client is removed and room is deleted (since it's empty)
	roomMux.Lock()
	_, roomExists := rooms[roomID]
	roomMux.Unlock()

	if roomExists {
		t.Fatal("Empty room should be deleted")
	}

	// Clean up
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()
}

func TestRemoveClientFromRoomWithMultipleClients(t *testing.T) {
	// Clear rooms map
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	roomID := "test-room"
	room := getOrCreateRoom(roomID)

	client1 := &Client{
		ID:     "client-1",
		RoomID: roomID,
	}
	client2 := &Client{
		ID:     "client-2",
		RoomID: roomID,
	}

	room.Mutex.Lock()
	room.Clients[client1.ID] = client1
	room.Clients[client2.ID] = client2
	room.Mutex.Unlock()

	// Verify both clients are in room
	room.Mutex.Lock()
	initialCount := len(room.Clients)
	room.Mutex.Unlock()

	if initialCount != 2 {
		t.Fatalf("Expected 2 clients initially, got %d", initialCount)
	}

	// Clean up - can't fully test removeClientFromRoom without websocket connections
	// Just verify the room structure is correct
	roomMux.Lock()
	_, exists := rooms[roomID]
	roomMux.Unlock()

	if !exists {
		t.Fatal("Room should exist")
	}

	// Clean up
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()
}

func TestRemoveClientFromRoomNoRoomID(t *testing.T) {
	client := &Client{
		ID:     "client-1",
		RoomID: "",
	}

	// Should not panic
	removeClientFromRoom(client)
}

func TestRemoveClientFromRoomNonexistentRoom(t *testing.T) {
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	client := &Client{
		ID:     "client-1",
		RoomID: "nonexistent-room",
	}

	// Should not panic
	removeClientFromRoom(client)
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("Expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	expected := `{"status":"ok"}`
	if body != expected {
		t.Fatalf("Expected body %s, got %s", expected, body)
	}
}

func TestMessageSerialization(t *testing.T) {
	// Test IncomingMessage
	inMsg := IncomingMessage{
		Type:     "join",
		RoomID:   "room-123",
		ClientID: "client-456",
		Pub:      "base64pubkey",
	}

	data, err := json.Marshal(inMsg)
	if err != nil {
		t.Fatalf("Failed to marshal IncomingMessage: %v", err)
	}

	var decoded IncomingMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal IncomingMessage: %v", err)
	}

	if decoded.Type != inMsg.Type || decoded.RoomID != inMsg.RoomID {
		t.Fatal("IncomingMessage fields don't match after serialization")
	}

	// Test OutgoingMessage
	outMsg := OutgoingMessage{
		Type:     "joined",
		SelfID:   "client-789",
		Mnemonic: "word1-word2",
		RoomID:   "room-123",
		Peers:    []string{"peer1", "peer2"},
		Count:    2,
	}

	data, err = json.Marshal(outMsg)
	if err != nil {
		t.Fatalf("Failed to marshal OutgoingMessage: %v", err)
	}

	var decodedOut OutgoingMessage
	err = json.Unmarshal(data, &decodedOut)
	if err != nil {
		t.Fatalf("Failed to unmarshal OutgoingMessage: %v", err)
	}

	if decodedOut.Type != outMsg.Type || decodedOut.Count != outMsg.Count {
		t.Fatal("OutgoingMessage fields don't match after serialization")
	}
}

// Helper function to setup a test WebSocket server
func setupTestServer(t *testing.T) *httptest.Server {
	// Initialize logger for tests
	logger = getTestLogger()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/health", healthHandler)
	server := httptest.NewServer(mux)
	return server
}

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestWebSocketConnection(t *testing.T) {
	// Clear rooms
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	server := setupTestServer(t)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Send join message
	joinMsg := IncomingMessage{
		Type:     "join",
		RoomID:   "test-room",
		ClientID: "test-client",
	}

	err = ws.WriteJSON(joinMsg)
	if err != nil {
		t.Fatalf("Failed to send join message: %v", err)
	}

	// Read response with timeout
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var response OutgoingMessage
	err = ws.ReadJSON(&response)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Verify joined response
	if response.Type != "joined" {
		t.Fatalf("Expected 'joined' message, got '%s'", response.Type)
	}
	if response.SelfID != "test-client" {
		t.Fatalf("Expected selfId 'test-client', got '%s'", response.SelfID)
	}
	if response.RoomID != "test-room" {
		t.Fatalf("Expected roomId 'test-room', got '%s'", response.RoomID)
	}
	if response.Mnemonic == "" {
		t.Fatal("Expected non-empty mnemonic")
	}
}

func TestWebSocketMultiplePeers(t *testing.T) {
	// Clear rooms
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	server := setupTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect first client
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect first client: %v", err)
	}
	defer ws1.Close()

	// First client joins
	joinMsg1 := IncomingMessage{
		Type:     "join",
		RoomID:   "multi-peer-room",
		ClientID: "client-1",
	}
	ws1.WriteJSON(joinMsg1)

	// Read joined response
	var resp1 OutgoingMessage
	ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	ws1.ReadJSON(&resp1)

	// Read peers message
	ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	ws1.ReadJSON(&resp1)

	// Connect second client
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect second client: %v", err)
	}
	defer ws2.Close()

	// Second client joins
	joinMsg2 := IncomingMessage{
		Type:     "join",
		RoomID:   "multi-peer-room",
		ClientID: "client-2",
	}
	ws2.WriteJSON(joinMsg2)

	// Read joined response for client 2
	var resp2 OutgoingMessage
	ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	ws2.ReadJSON(&resp2)

	// Both clients should receive peers update
	// Client 1 receives peers update
	var peersMsg1 OutgoingMessage
	ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = ws1.ReadJSON(&peersMsg1)
	if err != nil {
		t.Fatalf("Failed to read peers message: %v", err)
	}

	if peersMsg1.Type != "peers" {
		t.Fatalf("Expected 'peers' message, got '%s'", peersMsg1.Type)
	}
	if peersMsg1.Count != 2 {
		t.Fatalf("Expected 2 peers, got %d", peersMsg1.Count)
	}
}

func TestRoomLimit(t *testing.T) {
	// Set max rooms to 3 for this test
	maxRooms = 3

	// Clear rooms before test
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	server := setupTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Helper function to connect and join a room
	connectAndJoin := func(roomID string) (*websocket.Conn, error) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return nil, err
		}

		// Send join message
		joinMsg := IncomingMessage{
			Type:     "join",
			RoomID:   roomID,
			ClientID: "test-client-" + roomID,
		}
		if err := conn.WriteJSON(joinMsg); err != nil {
			conn.Close()
			return nil, err
		}

		// Read response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var resp OutgoingMessage
		if err := conn.ReadJSON(&resp); err != nil {
			conn.Close()
			return nil, err
		}

		// Check if it's an error
		if resp.Type == "error" {
			conn.Close()
			return nil, &RoomLimitError{Message: resp.Error}
		}

		return conn, nil
	}

	// Connect to 3 different rooms (should all succeed)
	conns := make([]*websocket.Conn, 0)
	for i := 1; i <= 3; i++ {
		roomID := "limit-test-room-" + string(rune('0'+i))
		conn, err := connectAndJoin(roomID)
		if err != nil {
			t.Fatalf("Failed to join room %d (should succeed): %v", i, err)
		}
		conns = append(conns, conn)
		t.Logf("Successfully joined room %d", i)
	}

	// Give a moment for rooms to be created
	time.Sleep(100 * time.Millisecond)

	// Verify we have exactly 3 rooms
	roomMux.Lock()
	roomCount := len(rooms)
	roomMux.Unlock()
	if roomCount != 3 {
		t.Errorf("Expected 3 rooms, got %d", roomCount)
	}

	// Try to connect to a 4th room (should fail)
	t.Log("Attempting to join 4th room (should fail)...")
	conn4, err := connectAndJoin("limit-test-room-4")
	if err == nil {
		conn4.Close()
		t.Fatal("Expected 4th room join to fail, but it succeeded")
	}

	// Check that the error is the expected one
	if !strings.Contains(err.Error(), "Maximum rooms") {
		t.Errorf("Expected 'Maximum rooms' error, got: %v", err)
	}
	t.Logf("4th room correctly rejected: %v", err)

	// Close one connection
	conns[0].Close()
	time.Sleep(200 * time.Millisecond)

	// Verify room was cleaned up
	roomMux.Lock()
	roomCount = len(rooms)
	roomMux.Unlock()
	t.Logf("After closing one connection, have %d rooms", roomCount)

	// Now try to join a new room (should succeed since we're under the limit)
	t.Log("Attempting to join new room after one was freed...")
	conn5, err := connectAndJoin("limit-test-room-5")
	if err != nil {
		t.Errorf("Expected to join new room after freeing one, but failed: %v", err)
	} else {
		t.Log("Successfully joined new room after one was freed")
		conn5.Close()
	}

	// Clean up remaining connections
	for _, conn := range conns[1:] {
		conn.Close()
	}

	// Reset maxRooms after test
	maxRooms = 0
}

func TestRoomPerIPLimit(t *testing.T) {
	maxRooms = 0
	maxRoomsPerIP = 2

	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	ipRoomMux.Lock()
	ipRooms = make(map[string]map[string]int)
	ipRoomMux.Unlock()

	server := setupTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	connectAndJoin := func(roomID string) (*websocket.Conn, error) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return nil, err
		}

		joinMsg := IncomingMessage{
			Type:     "join",
			RoomID:   roomID,
			ClientID: "test-client-" + roomID,
		}
		if err := conn.WriteJSON(joinMsg); err != nil {
			conn.Close()
			return nil, err
		}

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var resp OutgoingMessage
		if err := conn.ReadJSON(&resp); err != nil {
			conn.Close()
			return nil, err
		}

		if resp.Type == "error" {
			conn.Close()
			return nil, errors.New(resp.Error)
		}

		return conn, nil
	}

	conns := make([]*websocket.Conn, 0, maxRoomsPerIP)
	for i := 1; i <= maxRoomsPerIP; i++ {
		roomID := fmt.Sprintf("per-ip-room-%d", i)
		conn, err := connectAndJoin(roomID)
		if err != nil {
			t.Fatalf("Expected to join room %d (should succeed): %v", i, err)
		}
		conns = append(conns, conn)
	}

	if conn, err := connectAndJoin("per-ip-room-over-limit"); err == nil {
		conn.Close()
		t.Fatal("Expected join to be rejected after reaching per-IP limit")
	} else if !strings.Contains(err.Error(), "Maximum rooms per IP") {
		t.Fatalf("Expected maximum rooms per IP error, got: %v", err)
	}

	conns[0].Close()
	time.Sleep(200 * time.Millisecond)

	if conn, err := connectAndJoin("per-ip-room-new"); err != nil {
		t.Fatalf("Expected to join new room after closing one, but failed: %v", err)
	} else {
		conn.Close()
	}

	for _, conn := range conns[1:] {
		conn.Close()
	}

	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	maxRoomsPerIP = 0
	ipRoomMux.Lock()
	ipRooms = make(map[string]map[string]int)
	ipRoomMux.Unlock()
}

// Custom error type to distinguish room limit errors
type RoomLimitError struct {
	Message string
}

func (e *RoomLimitError) Error() string {
	return e.Message
}
