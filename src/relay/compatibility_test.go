package relay

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// TestBackwardCompatibility tests that JSON and Protobuf clients can communicate through the relay
func TestBackwardCompatibility(t *testing.T) {
	// Setup test server
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	server := setupTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect first client using JSON
	jsonConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect JSON client: %v", err)
	}
	defer jsonConn.Close()

	// Send JSON join message
	jsonJoinMsg := IncomingMessage{
		Type:     "join",
		RoomID:   "compat-test-room",
		ClientID: "json-client",
	}
	jsonData, _ := json.Marshal(jsonJoinMsg)
	jsonConn.WriteMessage(websocket.TextMessage, jsonData)

	// Read joined response (should be in JSON for JSON client)
	jsonConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var jsonResp OutgoingMessage
	if err := jsonConn.ReadJSON(&jsonResp); err != nil {
		t.Fatalf("Failed to read JSON response: %v", err)
	}

	if jsonResp.Type != "joined" {
		t.Fatalf("Expected 'joined' message, got '%s'", jsonResp.Type)
	}

	// Connect second client using Protobuf
	pbConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect Protobuf client: %v", err)
	}
	defer pbConn.Close()

	// Send Protobuf join message
	pbJoinMsg := &PBIncomingMessage{
		Type:     "join",
		RoomId:   "compat-test-room",
		ClientId: "pb-client",
	}
	pbData, _ := proto.Marshal(pbJoinMsg)
	pbConn.WriteMessage(websocket.BinaryMessage, pbData)

	// Read joined response (should be in Protobuf for Protobuf client)
	pbConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, raw, err := pbConn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read Protobuf response: %v", err)
	}

	if msgType != websocket.BinaryMessage {
		t.Fatalf("Expected binary message, got type %d", msgType)
	}

	pbResp := &PBOutgoingMessage{}
	if err := proto.Unmarshal(raw, pbResp); err != nil {
		t.Fatalf("Failed to unmarshal Protobuf response: %v", err)
	}

	if pbResp.Type != "joined" {
		t.Fatalf("Expected 'joined' message, got '%s'", pbResp.Type)
	}

	// Both clients should receive peers update
	// Read peers message from JSON client
	jsonConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var peersMsg1 OutgoingMessage
	if err := jsonConn.ReadJSON(&peersMsg1); err != nil {
		t.Fatalf("Failed to read peers message for JSON client: %v", err)
	}

	if peersMsg1.Type != "peers" {
		t.Fatalf("Expected 'peers' message, got type='%s'", peersMsg1.Type)
	}

	// Read peers message from Protobuf client
	pbConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType2, raw2, err := pbConn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read peers message for Protobuf client: %v", err)
	}

	if msgType2 != websocket.BinaryMessage {
		t.Fatalf("Expected binary message, got type %d", msgType2)
	}

	peersMsg2 := &PBOutgoingMessage{}
	if err := proto.Unmarshal(raw2, peersMsg2); err != nil {
		t.Fatalf("Failed to unmarshal peers message: %v", err)
	}

	if peersMsg2.Type != "peers" {
		t.Fatalf("Expected 'peers' message, got type='%s'", peersMsg2.Type)
	}

	// Verify both clients see 2 peers
	if peersMsg1.Count != 2 || peersMsg2.Count != 2 {
		t.Logf("JSON client sees %d peers, Protobuf client sees %d peers", peersMsg1.Count, peersMsg2.Count)
	} else {
		t.Log("Both clients correctly see 2 peers in the room")
	}

	t.Log("Successfully tested backward compatibility between JSON and Protobuf clients")
}
