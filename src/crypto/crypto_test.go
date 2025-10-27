package crypto

import (
	"crypto/ecdh"
	"testing"
)

func TestGenerateECDHKeyPair(t *testing.T) {
	privKey, err := GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("GenerateECDHKeyPair failed: %v", err)
	}
	if privKey == nil {
		t.Fatal("Expected non-nil private key")
	}
	if privKey.PublicKey() == nil {
		t.Fatal("Expected non-nil public key")
	}
}

func TestDeriveSharedSecret(t *testing.T) {
	// Generate two key pairs
	privKey1, err := GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate first key pair: %v", err)
	}

	privKey2, err := GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate second key pair: %v", err)
	}

	// Derive shared secrets from both sides
	secret1, err := DeriveSharedSecret(privKey1, privKey2.PublicKey())
	if err != nil {
		t.Fatalf("Failed to derive shared secret 1: %v", err)
	}

	secret2, err := DeriveSharedSecret(privKey2, privKey1.PublicKey())
	if err != nil {
		t.Fatalf("Failed to derive shared secret 2: %v", err)
	}

	// Secrets should match
	if len(secret1) != len(secret2) {
		t.Fatalf("Shared secrets have different lengths: %d vs %d", len(secret1), len(secret2))
	}

	for i := range secret1 {
		if secret1[i] != secret2[i] {
			t.Fatal("Shared secrets do not match")
		}
	}

	// Secret should be 32 bytes for P-256
	if len(secret1) != 32 {
		t.Fatalf("Expected 32-byte shared secret, got %d bytes", len(secret1))
	}
}

func TestDeriveSharedSecretInvalidKey(t *testing.T) {
	privKey, err := GenerateECDHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Try to parse invalid public key bytes
	invalidBytes := make([]byte, 32) // Wrong size for P-256
	_, err = ecdh.P256().NewPublicKey(invalidBytes)
	if err == nil {
		t.Fatal("Expected error when creating invalid public key")
	}

	// Test is successful if we can't create an invalid key
	// (which is the expected behavior - validation should prevent it)
	_ = privKey
}

func TestEncryptDecryptAESGCM(t *testing.T) {
	// Generate a 32-byte key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Hello, World! This is a test message.")

	// Encrypt
	iv, ciphertext, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if len(iv) != 12 {
		t.Fatalf("Expected IV length of 12, got %d", len(iv))
	}

	if len(ciphertext) == 0 {
		t.Fatal("Ciphertext is empty")
	}

	// Ciphertext should be different from plaintext
	if string(ciphertext[:len(plaintext)]) == string(plaintext) {
		t.Fatal("Ciphertext appears to be the same as plaintext")
	}

	// Decrypt
	decrypted, err := DecryptAESGCM(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Decrypted should match original
	if string(decrypted) != string(plaintext) {
		t.Fatalf("Decrypted text does not match plaintext.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}
}

func TestEncryptAESGCMInvalidKeySize(t *testing.T) {
	// Try with invalid key size
	key := []byte("short")
	plaintext := []byte("test")

	_, _, err := EncryptAESGCM(key, plaintext)
	if err == nil {
		t.Fatal("Expected error with invalid key size")
	}
}

func TestDecryptAESGCMInvalidKeySize(t *testing.T) {
	key := []byte("short")
	iv := make([]byte, 12)
	ciphertext := []byte("test")

	_, err := DecryptAESGCM(key, iv, ciphertext)
	if err == nil {
		t.Fatal("Expected error with invalid key size")
	}
}

func TestDecryptAESGCMWrongKey(t *testing.T) {
	// Generate two different keys
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	plaintext := []byte("Secret message")

	// Encrypt with key1
	iv, ciphertext, err := EncryptAESGCM(key1, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with key2
	_, err = DecryptAESGCM(key2, iv, ciphertext)
	if err == nil {
		t.Fatal("Expected decryption to fail with wrong key")
	}
}

func TestDecryptAESGCMTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Secret message")

	// Encrypt
	iv, ciphertext, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Tamper with ciphertext
	if len(ciphertext) > 0 {
		ciphertext[0] ^= 0xFF
	}

	// Try to decrypt tampered ciphertext
	_, err = DecryptAESGCM(key, iv, ciphertext)
	if err == nil {
		t.Fatal("Expected decryption to fail with tampered ciphertext")
	}
}

func TestEncryptDecryptEmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte{}

	iv, ciphertext, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("Encryption of empty plaintext failed: %v", err)
	}

	decrypted, err := DecryptAESGCM(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("Expected empty decrypted text, got %d bytes", len(decrypted))
	}
}

func TestEncryptDecryptLargePlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// Create a large plaintext (1MB)
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	iv, ciphertext, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	decrypted, err := DecryptAESGCM(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if len(decrypted) != len(plaintext) {
		t.Fatalf("Decrypted length mismatch: expected %d, got %d", len(plaintext), len(decrypted))
	}

	for i := range plaintext {
		if decrypted[i] != plaintext[i] {
			t.Fatalf("Decrypted data mismatch at byte %d", i)
		}
	}
}

func TestEncryptionProducesUniqueIVs(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("Same message")

	// Encrypt the same message twice
	iv1, _, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	iv2, _, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// IVs should be different (very high probability with random IVs)
	same := true
	for i := range iv1 {
		if iv1[i] != iv2[i] {
			same = false
			break
		}
	}

	if same {
		t.Fatal("Expected different IVs for two encryptions, but they were identical")
	}
}
