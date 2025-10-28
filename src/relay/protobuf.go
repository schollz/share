package relay

import (
	"encoding/json"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// Convert JSON IncomingMessage to Protobuf PBIncomingMessage
func incomingToPB(msg *IncomingMessage) *PBIncomingMessage {
	return &PBIncomingMessage{
		Type:      msg.Type,
		RoomId:    msg.RoomID,
		ClientId:  msg.ClientID,
		Pub:       msg.Pub,
		Name:      msg.Name,
		Size:      msg.Size,
		IvB64:     msg.IvB64,
		DataB64:   msg.DataB64,
		ChunkData: msg.ChunkData,
		ChunkNum:  int32(msg.ChunkNum),
		TotalSize: msg.TotalSize,
	}
}

// Convert Protobuf PBIncomingMessage to JSON IncomingMessage
func pbToIncoming(pb *PBIncomingMessage) *IncomingMessage {
	return &IncomingMessage{
		Type:      pb.Type,
		RoomID:    pb.RoomId,
		ClientID:  pb.ClientId,
		Pub:       pb.Pub,
		Name:      pb.Name,
		Size:      pb.Size,
		IvB64:     pb.IvB64,
		DataB64:   pb.DataB64,
		ChunkData: pb.ChunkData,
		ChunkNum:  int(pb.ChunkNum),
		TotalSize: pb.TotalSize,
	}
}

// Convert JSON OutgoingMessage to Protobuf PBOutgoingMessage
func outgoingToPB(msg *OutgoingMessage) *PBOutgoingMessage {
	return &PBOutgoingMessage{
		Type:      msg.Type,
		From:      msg.From,
		Mnemonic:  msg.Mnemonic,
		RoomId:    msg.RoomID,
		Pub:       msg.Pub,
		Name:      msg.Name,
		Size:      msg.Size,
		IvB64:     msg.IvB64,
		DataB64:   msg.DataB64,
		ChunkData: msg.ChunkData,
		ChunkNum:  int32(msg.ChunkNum),
		TotalSize: msg.TotalSize,
		SelfId:    msg.SelfID,
		Peers:     msg.Peers,
		Count:     int32(msg.Count),
		Error:     msg.Error,
	}
}

// Convert Protobuf PBOutgoingMessage to JSON OutgoingMessage
func pbToOutgoing(pb *PBOutgoingMessage) *OutgoingMessage {
	return &OutgoingMessage{
		Type:      pb.Type,
		From:      pb.From,
		Mnemonic:  pb.Mnemonic,
		RoomID:    pb.RoomId,
		Pub:       pb.Pub,
		Name:      pb.Name,
		Size:      pb.Size,
		IvB64:     pb.IvB64,
		DataB64:   pb.DataB64,
		ChunkData: pb.ChunkData,
		ChunkNum:  int(pb.ChunkNum),
		TotalSize: pb.TotalSize,
		SelfID:    pb.SelfId,
		Peers:     pb.Peers,
		Count:     int(pb.Count),
		Error:     pb.Error,
	}
}

// Detect if message is protobuf binary format (starts with field tag for "type" field)
// Protobuf field 1 (type) with wire type 2 (length-delimited) = 0x0A
func isProtobufMessage(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	// Check for protobuf field tag 0x0A (field 1, wire type 2)
	// This is the encoding for field 1 (type) which is a string
	return data[0] == 0x0A
}

// Encode OutgoingMessage to protobuf binary format
func encodeProtobuf(msg *OutgoingMessage) ([]byte, error) {
	pb := outgoingToPB(msg)
	return proto.Marshal(pb)
}

// Decode protobuf binary to IncomingMessage
func decodeProtobuf(data []byte) (*IncomingMessage, error) {
	pb := &PBIncomingMessage{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return nil, err
	}
	return pbToIncoming(pb), nil
}

// Helper to send a message to a websocket connection in the appropriate format
func sendMessage(conn *websocket.Conn, msg *OutgoingMessage, useProtobuf bool) error {
	if useProtobuf {
		data, err := encodeProtobuf(msg)
		if err != nil {
			return err
		}
		return conn.WriteMessage(websocket.BinaryMessage, data)
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
