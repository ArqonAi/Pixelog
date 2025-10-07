// Package crypto provides AES-256-GCM encryption for .pixe files
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptionService provides encryption and decryption functionality
type EncryptionService struct{}

// NewEncryptionService creates a new encryption service
func NewEncryptionService() *EncryptionService {
	return &EncryptionService{}
}

// Encrypt encrypts data using AES-256-GCM with a password-derived key
func (e *EncryptionService) Encrypt(data []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password required for encryption")
	}

	// Generate a random salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Combine salt + nonce + ciphertext
	encrypted := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	encrypted = append(encrypted, salt...)
	encrypted = append(encrypted, nonce...)
	encrypted = append(encrypted, ciphertext...)

	return encrypted, nil
}

// Decrypt decrypts AES-256-GCM encrypted data
func (e *EncryptionService) Decrypt(encrypted []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password required for decryption")
	}

	if len(encrypted) < 44 { // 32 (salt) + 12 (nonce) minimum
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Extract salt, nonce, and ciphertext
	salt := encrypted[:32]
	nonce := encrypted[32:44]
	ciphertext := encrypted[44:]

	// Derive key using same parameters
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// GenerateRandomPassword creates a cryptographically secure random password
func (e *EncryptionService) GenerateRandomPassword(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	
	password := make([]byte, length)
	for i := range password {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}
		password[i] = charset[randomIndex.Int64()]
	}
	
	return string(password), nil
}

// HashPassword creates a SHA-256 hash of the password for verification
func (e *EncryptionService) HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}
