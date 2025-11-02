package client

import (
	"encoding/json"
	"testing"

	"github.com/schollz/e2ecp/src/crypto"
)

func TestMetadataEncryptionDecryption(t *testing.T) {
	// Generate a key pair for testing
	privKey, err := crypto.GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Generate shared secret (in real scenario this would be derived from peer's public key)
	sharedSecret, err := crypto.DeriveSharedSecret(privKey, privKey.PublicKey())
	if err != nil {
		t.Fatalf("Failed to derive shared secret: %v", err)
	}

	// Create test metadata
	metadata := FileMetadata{
		Name:               "test-file.txt",
		TotalSize:          12345,
		IsFolder:           false,
		OriginalFolderName: "",
		IsMultipleFiles:    false,
	}

	// Marshal metadata
	metadataJSON, err := MarshalMetadata(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	// Encrypt metadata
	iv, encryptedMetadata, err := crypto.EncryptAESGCM(sharedSecret, metadataJSON)
	if err != nil {
		t.Fatalf("Failed to encrypt metadata: %v", err)
	}

	// Verify encrypted data is not the same as original
	if string(encryptedMetadata) == string(metadataJSON) {
		t.Fatal("Encrypted metadata is the same as plain text")
	}

	// Decrypt metadata
	decryptedMetadataJSON, err := crypto.DecryptAESGCM(sharedSecret, iv, encryptedMetadata)
	if err != nil {
		t.Fatalf("Failed to decrypt metadata: %v", err)
	}

	// Unmarshal decrypted metadata
	decryptedMetadata, err := UnmarshalMetadata(decryptedMetadataJSON)
	if err != nil {
		t.Fatalf("Failed to unmarshal metadata: %v", err)
	}

	// Verify decrypted metadata matches original
	if decryptedMetadata.Name != metadata.Name {
		t.Errorf("Name mismatch: got %s, want %s", decryptedMetadata.Name, metadata.Name)
	}
	if decryptedMetadata.TotalSize != metadata.TotalSize {
		t.Errorf("TotalSize mismatch: got %d, want %d", decryptedMetadata.TotalSize, metadata.TotalSize)
	}
	if decryptedMetadata.IsFolder != metadata.IsFolder {
		t.Errorf("IsFolder mismatch: got %v, want %v", decryptedMetadata.IsFolder, metadata.IsFolder)
	}

	t.Log("Metadata encryption/decryption successful")
}

func TestFolderMetadataEncryptionDecryption(t *testing.T) {
	// Generate a key pair for testing
	privKey, err := crypto.GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Generate shared secret
	sharedSecret, err := crypto.DeriveSharedSecret(privKey, privKey.PublicKey())
	if err != nil {
		t.Fatalf("Failed to derive shared secret: %v", err)
	}

	// Create test metadata for folder
	metadata := FileMetadata{
		Name:               "myfolder.zip",
		TotalSize:          54321,
		IsFolder:           true,
		OriginalFolderName: "myfolder",
		IsMultipleFiles:    false,
	}

	// Marshal metadata
	metadataJSON, err := MarshalMetadata(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	// Encrypt metadata
	iv, encryptedMetadata, err := crypto.EncryptAESGCM(sharedSecret, metadataJSON)
	if err != nil {
		t.Fatalf("Failed to encrypt metadata: %v", err)
	}

	// Decrypt metadata
	decryptedMetadataJSON, err := crypto.DecryptAESGCM(sharedSecret, iv, encryptedMetadata)
	if err != nil {
		t.Fatalf("Failed to decrypt metadata: %v", err)
	}

	// Unmarshal decrypted metadata
	decryptedMetadata, err := UnmarshalMetadata(decryptedMetadataJSON)
	if err != nil {
		t.Fatalf("Failed to unmarshal metadata: %v", err)
	}

	// Verify all fields
	if decryptedMetadata.Name != metadata.Name {
		t.Errorf("Name mismatch: got %s, want %s", decryptedMetadata.Name, metadata.Name)
	}
	if decryptedMetadata.TotalSize != metadata.TotalSize {
		t.Errorf("TotalSize mismatch: got %d, want %d", decryptedMetadata.TotalSize, metadata.TotalSize)
	}
	if decryptedMetadata.IsFolder != metadata.IsFolder {
		t.Errorf("IsFolder mismatch: got %v, want %v", decryptedMetadata.IsFolder, metadata.IsFolder)
	}
	if decryptedMetadata.OriginalFolderName != metadata.OriginalFolderName {
		t.Errorf("OriginalFolderName mismatch: got %s, want %s", decryptedMetadata.OriginalFolderName, metadata.OriginalFolderName)
	}

	t.Log("Folder metadata encryption/decryption successful")
}

func TestMetadataZeroKnowledge(t *testing.T) {
	// This test verifies that encrypted metadata cannot be inspected by the relay

	// Generate a key pair
	privKey, err := crypto.GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Generate shared secret
	sharedSecret, err := crypto.DeriveSharedSecret(privKey, privKey.PublicKey())
	if err != nil {
		t.Fatalf("Failed to derive shared secret: %v", err)
	}

	// Create metadata with sensitive information
	metadata := FileMetadata{
		Name:               "secret-document.pdf",
		TotalSize:          999999,
		IsFolder:           false,
		OriginalFolderName: "",
		IsMultipleFiles:    false,
	}

	// Marshal and encrypt
	metadataJSON, err := MarshalMetadata(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	_, encryptedMetadata, err := crypto.EncryptAESGCM(sharedSecret, metadataJSON)
	if err != nil {
		t.Fatalf("Failed to encrypt metadata: %v", err)
	}

	// Verify that sensitive data is not visible in encrypted form
	encryptedStr := string(encryptedMetadata)

	// The relay should not be able to see the filename
	if containsSubstring(encryptedStr, "secret-document") {
		t.Error("Filename visible in encrypted metadata")
	}

	// The relay should not be able to see "is_folder"
	if containsSubstring(encryptedStr, "is_folder") {
		t.Error("Field name 'is_folder' visible in encrypted metadata")
	}

	// Verify that attempting to parse as JSON fails (encrypted data is not JSON)
	var jsonTest map[string]interface{}
	if json.Unmarshal(encryptedMetadata, &jsonTest) == nil {
		t.Error("Encrypted metadata can be parsed as JSON - not properly encrypted!")
	}

	t.Log("Metadata is properly encrypted and zero-knowledge")
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
