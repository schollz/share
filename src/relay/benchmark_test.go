package relay

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"
)

// BenchmarkJSONEncode tests the performance of JSON encoding
func BenchmarkJSONEncode(b *testing.B) {
	msg := IncomingMessage{
		Type:      "file_chunk",
		RoomID:    "test-room-12345",
		ClientID:  "client-67890",
		Pub:       "base64encodedpublickey1234567890abcdef",
		Name:      "example-file.txt",
		Size:      1024000,
		IvB64:     "base64encodediv123456",
		DataB64:   "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkData: "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkNum:  42,
		TotalSize: 10240000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONDecode tests the performance of JSON decoding
func BenchmarkJSONDecode(b *testing.B) {
	msg := IncomingMessage{
		Type:      "file_chunk",
		RoomID:    "test-room-12345",
		ClientID:  "client-67890",
		Pub:       "base64encodedpublickey1234567890abcdef",
		Name:      "example-file.txt",
		Size:      1024000,
		IvB64:     "base64encodediv123456",
		DataB64:   "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkData: "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkNum:  42,
		TotalSize: 10240000,
	}

	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded IncomingMessage
		err := json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONRoundTrip tests the performance of a full encode/decode cycle
func BenchmarkJSONRoundTrip(b *testing.B) {
	msg := OutgoingMessage{
		Type:      "file_chunk",
		From:      "sender-client-123",
		Mnemonic:  "apple-banana",
		RoomID:    "test-room-12345",
		Pub:       "base64encodedpublickey1234567890abcdef",
		Name:      "example-file.txt",
		Size:      1024000,
		IvB64:     "base64encodediv123456",
		DataB64:   "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkData: "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkNum:  42,
		TotalSize: 10240000,
		SelfID:    "self-client-456",
		Peers:     []string{"peer1", "peer2", "peer3"},
		Count:     3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}

		var decoded OutgoingMessage
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONEncodeSmallMessage tests encoding of small messages (e.g., join, peers)
func BenchmarkJSONEncodeSmallMessage(b *testing.B) {
	msg := IncomingMessage{
		Type:     "join",
		RoomID:   "test-room",
		ClientID: "client-123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONDecodeSmallMessage tests decoding of small messages
func BenchmarkJSONDecodeSmallMessage(b *testing.B) {
	msg := IncomingMessage{
		Type:     "join",
		RoomID:   "test-room",
		ClientID: "client-123",
	}

	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded IncomingMessage
		err := json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONEncodeLargeChunk tests encoding with a large data chunk (simulating file transfer)
func BenchmarkJSONEncodeLargeChunk(b *testing.B) {
	// Create a large base64 string (~64KB)
	largeData := make([]byte, 65536)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	msg := IncomingMessage{
		Type:      "file_chunk",
		RoomID:    "test-room-12345",
		ClientID:  "client-67890",
		ChunkData: string(largeData),
		ChunkNum:  1,
		TotalSize: 1048576,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONDecodeLargeChunk tests decoding with a large data chunk
func BenchmarkJSONDecodeLargeChunk(b *testing.B) {
	// Create a large base64 string (~64KB)
	largeData := make([]byte, 65536)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	msg := IncomingMessage{
		Type:      "file_chunk",
		RoomID:    "test-room-12345",
		ClientID:  "client-67890",
		ChunkData: string(largeData),
		ChunkNum:  1,
		TotalSize: 1048576,
	}

	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded IncomingMessage
		err := json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================================
// Protobuf Benchmarks
// ============================================================================

// BenchmarkProtobufEncode tests the performance of Protobuf encoding
func BenchmarkProtobufEncode(b *testing.B) {
	msg := &PBIncomingMessage{
		Type:      "file_chunk",
		RoomId:    "test-room-12345",
		ClientId:  "client-67890",
		Pub:       "base64encodedpublickey1234567890abcdef",
		Name:      "example-file.txt",
		Size:      1024000,
		IvB64:     "base64encodediv123456",
		DataB64:   "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkData: "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkNum:  42,
		TotalSize: 10240000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtobufDecode tests the performance of Protobuf decoding
func BenchmarkProtobufDecode(b *testing.B) {
	msg := &PBIncomingMessage{
		Type:      "file_chunk",
		RoomId:    "test-room-12345",
		ClientId:  "client-67890",
		Pub:       "base64encodedpublickey1234567890abcdef",
		Name:      "example-file.txt",
		Size:      1024000,
		IvB64:     "base64encodediv123456",
		DataB64:   "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkData: "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkNum:  42,
		TotalSize: 10240000,
	}

	data, _ := proto.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoded := &PBIncomingMessage{}
		err := proto.Unmarshal(data, decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtobufRoundTrip tests the performance of a full encode/decode cycle
func BenchmarkProtobufRoundTrip(b *testing.B) {
	msg := &PBOutgoingMessage{
		Type:      "file_chunk",
		From:      "sender-client-123",
		Mnemonic:  "apple-banana",
		RoomId:    "test-room-12345",
		Pub:       "base64encodedpublickey1234567890abcdef",
		Name:      "example-file.txt",
		Size:      1024000,
		IvB64:     "base64encodediv123456",
		DataB64:   "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkData: "VGhpcyBpcyBhIHRlc3QgZGF0YSBzdHJpbmcgdGhhdCByZXByZXNlbnRzIGEgY2h1bmsgb2YgYmlnZ2VyIGZpbGUgZGF0YS4gSXQgc2hvdWxkIGJlIGxvbmcgZW5vdWdoIHRvIG1ha2UgdGhlIGJlbmNobWFyayByZWFsaXN0aWMu",
		ChunkNum:  42,
		TotalSize: 10240000,
		SelfId:    "self-client-456",
		Peers:     []string{"peer1", "peer2", "peer3"},
		Count:     3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := proto.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}

		decoded := &PBOutgoingMessage{}
		err = proto.Unmarshal(data, decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtobufEncodeSmallMessage tests encoding of small messages (e.g., join, peers)
func BenchmarkProtobufEncodeSmallMessage(b *testing.B) {
	msg := &PBIncomingMessage{
		Type:     "join",
		RoomId:   "test-room",
		ClientId: "client-123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtobufDecodeSmallMessage tests decoding of small messages
func BenchmarkProtobufDecodeSmallMessage(b *testing.B) {
	msg := &PBIncomingMessage{
		Type:     "join",
		RoomId:   "test-room",
		ClientId: "client-123",
	}

	data, _ := proto.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoded := &PBIncomingMessage{}
		err := proto.Unmarshal(data, decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtobufEncodeLargeChunk tests encoding with a large data chunk (simulating file transfer)
func BenchmarkProtobufEncodeLargeChunk(b *testing.B) {
	// Create a large base64 string (~64KB) - use base64 encoding to ensure valid UTF-8
	largeData := make([]byte, 65536)
	for i := range largeData {
		largeData[i] = byte(i%64) + 32 // Generate printable ASCII characters
	}

	msg := &PBIncomingMessage{
		Type:      "file_chunk",
		RoomId:    "test-room-12345",
		ClientId:  "client-67890",
		ChunkData: string(largeData),
		ChunkNum:  1,
		TotalSize: 1048576,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtobufDecodeLargeChunk tests decoding with a large data chunk
func BenchmarkProtobufDecodeLargeChunk(b *testing.B) {
	// Create a large base64 string (~64KB) - use base64 encoding to ensure valid UTF-8
	largeData := make([]byte, 65536)
	for i := range largeData {
		largeData[i] = byte(i%64) + 32 // Generate printable ASCII characters
	}

	msg := &PBIncomingMessage{
		Type:      "file_chunk",
		RoomId:    "test-room-12345",
		ClientId:  "client-67890",
		ChunkData: string(largeData),
		ChunkNum:  1,
		TotalSize: 1048576,
	}

	data, _ := proto.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoded := &PBIncomingMessage{}
		err := proto.Unmarshal(data, decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}
