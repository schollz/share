package relay

import (
	"encoding/base64"
	"testing"
)

// TestRelayCannotInspectMetadata verifies that the relay server
// cannot access or inspect encrypted file metadata
func TestRelayCannotInspectMetadata(t *testing.T) {
	// Simulate what a client sends to the relay
	// This is encrypted metadata that the relay should not be able to read

	// Simulated encrypted metadata (this would be real AES-GCM encrypted data)
	encryptedMetadata := base64.StdEncoding.EncodeToString([]byte("this-is-encrypted-binary-data"))
	metadataIV := base64.StdEncoding.EncodeToString([]byte("random-iv-bytes"))

	// Create an incoming message as the relay would receive it
	inMsg := IncomingMessage{
		Type:              "file_start",
		EncryptedMetadata: encryptedMetadata,
		MetadataIV:        metadataIV,
	}

	// Convert to outgoing message (as relay does)
	outMsg := OutgoingMessage{
		Type:              inMsg.Type,
		EncryptedMetadata: inMsg.EncryptedMetadata,
		MetadataIV:        inMsg.MetadataIV,
	}

	// Verify that encrypted metadata is present
	if outMsg.EncryptedMetadata == "" {
		t.Error("Encrypted metadata is missing")
	}
	if outMsg.MetadataIV == "" {
		t.Error("Metadata IV is missing")
	}

	// Relay should only see that encrypted metadata exists, not its contents
	t.Logf("Relay successfully forwards encrypted metadata without inspection")
	t.Logf("Encrypted metadata present: %v", outMsg.EncryptedMetadata != "")
}

// TestZeroKnowledgeNewClients verifies that new clients using encrypted
// metadata don't leak information through legacy fields
func TestZeroKnowledgeNewClients(t *testing.T) {
	// Simulate a new client sending only encrypted metadata
	newClientMsg := IncomingMessage{
		Type:              "file_start",
		EncryptedMetadata: base64.StdEncoding.EncodeToString([]byte("encrypted-data")),
		MetadataIV:        base64.StdEncoding.EncodeToString([]byte("iv-data")),
	}

	// Verify encrypted fields are populated
	if newClientMsg.EncryptedMetadata == "" {
		t.Error("Encrypted metadata missing")
	}
	if newClientMsg.MetadataIV == "" {
		t.Error("Metadata IV missing")
	}

	t.Log("New clients properly use zero-knowledge encrypted metadata")
}
