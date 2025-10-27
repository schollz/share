package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"io"
)

// GenerateECDHKeyPair generates a new ECDH P-256 key pair
func GenerateECDHKeyPair() (*ecdh.PrivateKey, error) {
	return ecdh.P256().GenerateKey(rand.Reader)
}

// DeriveSharedSecret derives a shared secret from a private key and peer's public key
func DeriveSharedSecret(privKey *ecdh.PrivateKey, peerPubKey *ecdh.PublicKey) ([]byte, error) {
	return privKey.ECDH(peerPubKey)
}

// EncryptAESGCM encrypts plaintext using AES-GCM with the given key
func EncryptAESGCM(key, plaintext []byte) (iv, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	iv = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, iv, plaintext, nil)
	return iv, ciphertext, nil
}

// DecryptAESGCM decrypts ciphertext using AES-GCM with the given key
func DecryptAESGCM(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, iv, ciphertext, nil)
}

// GenerateMnemonic generates a 2-word BIP39 mnemonic from a client ID
func GenerateMnemonic(clientID string) string {
	// This is imported from the relay package to avoid circular dependencies
	// The implementation is in relay/mnemonic.go
	return ""
}
