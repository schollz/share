package client

import (
	"github.com/gorilla/websocket"
	"github.com/schollz/share/src/relay"
	"google.golang.org/protobuf/proto"
)

// sendProtobufMessage sends a message using protobuf encoding
func sendProtobufMessage(conn *websocket.Conn, msg map[string]interface{}) error {
	// Convert map to protobuf message
	pbMsg := &relay.PBIncomingMessage{}
	
	if v, ok := msg["type"].(string); ok {
		pbMsg.Type = v
	}
	if v, ok := msg["roomId"].(string); ok {
		pbMsg.RoomId = v
	}
	if v, ok := msg["clientId"].(string); ok {
		pbMsg.ClientId = v
	}
	if v, ok := msg["pub"].(string); ok {
		pbMsg.Pub = v
	}
	if v, ok := msg["name"].(string); ok {
		pbMsg.Name = v
	}
	if v, ok := msg["size"].(int64); ok {
		pbMsg.Size = v
	}
	if v, ok := msg["iv_b64"].(string); ok {
		pbMsg.IvB64 = v
	}
	if v, ok := msg["data_b64"].(string); ok {
		pbMsg.DataB64 = v
	}
	if v, ok := msg["chunk_data"].(string); ok {
		pbMsg.ChunkData = v
	}
	if v, ok := msg["chunk_num"].(int); ok {
		pbMsg.ChunkNum = int32(v)
	}
	if v, ok := msg["total_size"].(int64); ok {
		pbMsg.TotalSize = v
	}
	
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

// receiveProtobufMessage receives a message in either protobuf or JSON format
func receiveProtobufMessage(conn *websocket.Conn) (*relay.OutgoingMessage, error) {
	msgType, raw, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	
	if msgType == websocket.BinaryMessage {
		// Protobuf message
		pbMsg := &relay.PBOutgoingMessage{}
		if err := proto.Unmarshal(raw, pbMsg); err != nil {
			return nil, err
		}
		
		// Convert to OutgoingMessage
		return &relay.OutgoingMessage{
			Type:      pbMsg.Type,
			From:      pbMsg.From,
			Mnemonic:  pbMsg.Mnemonic,
			RoomID:    pbMsg.RoomId,
			Pub:       pbMsg.Pub,
			Name:      pbMsg.Name,
			Size:      pbMsg.Size,
			IvB64:     pbMsg.IvB64,
			DataB64:   pbMsg.DataB64,
			ChunkData: pbMsg.ChunkData,
			ChunkNum:  int(pbMsg.ChunkNum),
			TotalSize: pbMsg.TotalSize,
			SelfID:    pbMsg.SelfId,
			Peers:     pbMsg.Peers,
			Count:     int(pbMsg.Count),
			Error:     pbMsg.Error,
		}, nil
	}
	
	// JSON message
	var msg relay.OutgoingMessage
	if err := conn.ReadJSON(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
