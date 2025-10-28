package relay

import (
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// TestProtobufMessaging tests that Protobuf clients can communicate through the relay
func TestProtobufMessaging(t *testing.T) {
	// Setup test server
	roomMux.Lock()
	rooms = make(map[string]*Room)
	roomMux.Unlock()

	server := setupTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect first client using Protobuf
	pbConn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect first Protobuf client: %v", err)
	}
	defer pbConn1.Close()

	// Send Protobuf join message
	pbJoinMsg1 := &PBIncomingMessage{
		Type:     "join",
		RoomId:   "protobuf-test-room",
		ClientId: "pb-client-1",
	}
	pbData1, _ := proto.Marshal(pbJoinMsg1)
	pbConn1.WriteMessage(websocket.BinaryMessage, pbData1)

	// Read joined response
	pbConn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, raw, err := pbConn1.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read Protobuf response: %v", err)
	}

	if msgType != websocket.BinaryMessage {
		t.Fatalf("Expected binary message, got type %d", msgType)
	}

	pbResp1 := &PBOutgoingMessage{}
	if err := proto.Unmarshal(raw, pbResp1); err != nil {
		t.Fatalf("Failed to unmarshal Protobuf response: %v", err)
	}

	if pbResp1.Type != "joined" {
		t.Fatalf("Expected 'joined' message, got '%s'", pbResp1.Type)
	}

	// Read initial peers message for first client (count will be 1)
	pbConn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = pbConn1.ReadMessage()

	// Connect second client using Protobuf
	pbConn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect second Protobuf client: %v", err)
	}
	defer pbConn2.Close()

	// Send Protobuf join message
	pbJoinMsg2 := &PBIncomingMessage{
		Type:     "join",
		RoomId:   "protobuf-test-room",
		ClientId: "pb-client-2",
	}
	pbData2, _ := proto.Marshal(pbJoinMsg2)
	pbConn2.WriteMessage(websocket.BinaryMessage, pbData2)

	// Read joined response for second client
	pbConn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType2, raw2, err := pbConn2.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read Protobuf response for second client: %v", err)
	}

	if msgType2 != websocket.BinaryMessage {
		t.Fatalf("Expected binary message, got type %d", msgType2)
	}

	pbResp2 := &PBOutgoingMessage{}
	if err := proto.Unmarshal(raw2, pbResp2); err != nil {
		t.Fatalf("Failed to unmarshal Protobuf response: %v", err)
	}

	if pbResp2.Type != "joined" {
		t.Fatalf("Expected 'joined' message, got '%s'", pbResp2.Type)
	}

	// Read peers message from first client
	pbConn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType3, raw3, err := pbConn1.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read peers message for first client: %v", err)
	}

	if msgType3 != websocket.BinaryMessage {
		t.Fatalf("Expected binary message, got type %d", msgType3)
	}

	peersMsg1 := &PBOutgoingMessage{}
	if err := proto.Unmarshal(raw3, peersMsg1); err != nil {
		t.Fatalf("Failed to unmarshal peers message: %v", err)
	}

	if peersMsg1.Type != "peers" {
		t.Fatalf("Expected 'peers' message, got type='%s'", peersMsg1.Type)
	}

	// Read peers message from second client
	pbConn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType4, raw4, err := pbConn2.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read peers message for second client: %v", err)
	}

	if msgType4 != websocket.BinaryMessage {
		t.Fatalf("Expected binary message, got type %d", msgType4)
	}

	peersMsg2 := &PBOutgoingMessage{}
	if err := proto.Unmarshal(raw4, peersMsg2); err != nil {
		t.Fatalf("Failed to unmarshal peers message: %v", err)
	}

	if peersMsg2.Type != "peers" {
		t.Fatalf("Expected 'peers' message, got type='%s'", peersMsg2.Type)
	}

	// Verify both clients see 2 peers
	if peersMsg1.Count != 2 || peersMsg2.Count != 2 {
		t.Fatalf("Expected both clients to see 2 peers, got %d and %d", peersMsg1.Count, peersMsg2.Count)
	}

	t.Log("Successfully tested Protobuf messaging between clients")
}
