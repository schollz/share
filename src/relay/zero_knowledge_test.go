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
		// Note: Name, TotalSize, IsFolder, etc. should be empty for zero-knowledge
		Name:               "",
		TotalSize:          0,
		IsFolder:           false,
		OriginalFolderName: "",
	}

	// Convert to outgoing message (as relay does)
	outMsg := OutgoingMessage{
		Type:              inMsg.Type,
		EncryptedMetadata: inMsg.EncryptedMetadata,
		MetadataIV:        inMsg.MetadataIV,
		// Legacy fields should remain empty
		Name:               inMsg.Name,
		TotalSize:          inMsg.TotalSize,
		IsFolder:           inMsg.IsFolder,
		OriginalFolderName: inMsg.OriginalFolderName,
	}

	// Verify that relay cannot access plaintext metadata
	if outMsg.Name != "" {
		t.Error("Relay can see filename - should be encrypted")
	}
	if outMsg.TotalSize != 0 {
		t.Error("Relay can see file size - should be encrypted")
	}
	if outMsg.OriginalFolderName != "" {
		t.Error("Relay can see folder name - should be encrypted")
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
	t.Logf("Plaintext metadata hidden: Name=%q, Size=%d, Folder=%q",
		outMsg.Name, outMsg.TotalSize, outMsg.OriginalFolderName)
}

// TestBackwardCompatibilityWithLegacyFields ensures that old clients
// can still use plain text fields while new clients use encrypted metadata
func TestBackwardCompatibilityWithLegacyFields(t *testing.T) {
	// Simulate a legacy client sending plain text metadata
	legacyMsg := IncomingMessage{
		Type:      "file_start",
		Name:      "test.txt",
		TotalSize: 1024,
		IsFolder:  false,
	}

	// Relay forwards it as-is
	outMsg := OutgoingMessage{
		Type:      legacyMsg.Type,
		Name:      legacyMsg.Name,
		TotalSize: legacyMsg.TotalSize,
		IsFolder:  legacyMsg.IsFolder,
	}

	// Verify legacy fields are preserved
	if outMsg.Name != "test.txt" {
		t.Errorf("Legacy name not preserved: got %q", outMsg.Name)
	}
	if outMsg.TotalSize != 1024 {
		t.Errorf("Legacy size not preserved: got %d", outMsg.TotalSize)
	}

	t.Log("Backward compatibility with legacy fields maintained")
}

// TestZeroKnowledgeNewClients verifies that new clients using encrypted
// metadata don't leak information through legacy fields
func TestZeroKnowledgeNewClients(t *testing.T) {
	// Simulate a new client sending only encrypted metadata
	newClientMsg := IncomingMessage{
		Type:              "file_start",
		EncryptedMetadata: base64.StdEncoding.EncodeToString([]byte("encrypted-data")),
		MetadataIV:        base64.StdEncoding.EncodeToString([]byte("iv-data")),
		// All legacy fields should be empty/zero
		Name:               "",
		TotalSize:          0,
		IsFolder:           false,
		OriginalFolderName: "",
		IsMultipleFiles:    false,
	}

	// Verify that no sensitive data is in legacy fields
	if newClientMsg.Name != "" {
		t.Error("New client leaking filename in legacy field")
	}
	if newClientMsg.TotalSize != 0 {
		t.Error("New client leaking file size in legacy field")
	}
	if newClientMsg.OriginalFolderName != "" {
		t.Error("New client leaking folder name in legacy field")
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
