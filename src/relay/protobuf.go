package relay

import (
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// Convert JSON IncomingMessage to Protobuf PBIncomingMessage
func incomingToPB(msg *IncomingMessage) *PBIncomingMessage {
	return &PBIncomingMessage{
		Type:               msg.Type,
		RoomId:             msg.RoomID,
		ClientId:           msg.ClientID,
		Pub:                msg.Pub,
		Name:               msg.Name,
		Size:               msg.Size,
		IvB64:              msg.IvB64,
		DataB64:            msg.DataB64,
		ChunkData:          msg.ChunkData,
		ChunkNum:           int32(msg.ChunkNum),
		TotalSize:          msg.TotalSize,
		IsFolder:           msg.IsFolder,
		OriginalFolderName: msg.OriginalFolderName,
		EncryptedMetadata:  msg.EncryptedMetadata,
		MetadataIv:         msg.MetadataIV,
	}
}

// Convert Protobuf PBIncomingMessage to JSON IncomingMessage
func pbToIncoming(pb *PBIncomingMessage) *IncomingMessage {
	return &IncomingMessage{
		Type:               pb.Type,
		RoomID:             pb.RoomId,
		ClientID:           pb.ClientId,
		Pub:                pb.Pub,
		Name:               pb.Name,
		Size:               pb.Size,
		IvB64:              pb.IvB64,
		DataB64:            pb.DataB64,
		ChunkData:          pb.ChunkData,
		ChunkNum:           int(pb.ChunkNum),
		TotalSize:          pb.TotalSize,
		IsFolder:           pb.IsFolder,
		OriginalFolderName: pb.OriginalFolderName,
		IsMultipleFiles:    pb.IsMultipleFiles,
		EncryptedMetadata:  pb.EncryptedMetadata,
		MetadataIV:         pb.MetadataIv,
	}
}

// Convert JSON OutgoingMessage to Protobuf PBOutgoingMessage
func outgoingToPB(msg *OutgoingMessage) *PBOutgoingMessage {
	return &PBOutgoingMessage{
		Type:               msg.Type,
		From:               msg.From,
		Mnemonic:           msg.Mnemonic,
		RoomId:             msg.RoomID,
		Pub:                msg.Pub,
		Name:               msg.Name,
		Size:               msg.Size,
		IvB64:              msg.IvB64,
		DataB64:            msg.DataB64,
		ChunkData:          msg.ChunkData,
		ChunkNum:           int32(msg.ChunkNum),
		TotalSize:          msg.TotalSize,
		SelfId:             msg.SelfID,
		Peers:              msg.Peers,
		Count:              int32(msg.Count),
		Error:              msg.Error,
		IsFolder:           msg.IsFolder,
		OriginalFolderName: msg.OriginalFolderName,
		IsMultipleFiles:    msg.IsMultipleFiles,
		EncryptedMetadata:  msg.EncryptedMetadata,
		MetadataIv:         msg.MetadataIV,
	}
}

// Convert Protobuf PBOutgoingMessage to JSON OutgoingMessage
func pbToOutgoing(pb *PBOutgoingMessage) *OutgoingMessage {
	return &OutgoingMessage{
		Type:               pb.Type,
		From:               pb.From,
		Mnemonic:           pb.Mnemonic,
		RoomID:             pb.RoomId,
		Pub:                pb.Pub,
		Name:               pb.Name,
		Size:               pb.Size,
		IvB64:              pb.IvB64,
		DataB64:            pb.DataB64,
		ChunkData:          pb.ChunkData,
		ChunkNum:           int(pb.ChunkNum),
		TotalSize:          pb.TotalSize,
		SelfID:             pb.SelfId,
		Peers:              pb.Peers,
		Count:              int(pb.Count),
		Error:              pb.Error,
		IsFolder:           pb.IsFolder,
		OriginalFolderName: pb.OriginalFolderName,
		IsMultipleFiles:    pb.IsMultipleFiles,
		EncryptedMetadata:  pb.EncryptedMetadata,
		MetadataIV:         pb.MetadataIv,
	}
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

// Helper to send a protobuf message to a websocket connection
func sendMessage(conn *websocket.Conn, msg *OutgoingMessage, useProtobuf bool) error {
	data, err := encodeProtobuf(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}
