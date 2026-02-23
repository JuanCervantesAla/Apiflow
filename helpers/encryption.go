package helpers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// GetEncryptionKey returns the encryption key from environment or a default (should be 32 bytes for AES-256)
func GetEncryptionKey() []byte {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		// Default key - CHANGE THIS IN PRODUCTION!
		// Generate a secure key with: openssl rand -hex 32
		key = "capyflow-secret-key-change-me-32bytes!!"
	}
	// Ensure key is exactly 32 bytes
	keyBytes := []byte(key)
	if len(keyBytes) > 32 {
		return keyBytes[:32]
	}
	if len(keyBytes) < 32 {
		// Pad with zeros
		padding := make([]byte, 32-len(keyBytes))
		return append(keyBytes, padding...)
	}
	return keyBytes
}

// EncryptString encrypts a string using AES-256
func EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key := GetEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt and encode
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a string encrypted with EncryptString
func DecryptString(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key := GetEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
