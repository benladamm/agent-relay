package relay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// DeriveKey generates a 32-byte AES key from a passphrase.
func DeriveKey(passphrase string, salt []byte) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(passphrase))
	hasher.Write(salt)
	return hasher.Sum(nil)
}

// EncryptPayload encrypts data using AES-256-GCM with a passphrase.
func EncryptPayload(plainText []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := DeriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	sealed := gcm.Seal(nil, nonce, plainText, nil)

	// Result: salt (16) + nonce (12) + ciphertext
	result := make([]byte, 0, len(salt)+len(nonce)+len(sealed))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, sealed...)
	return result, nil
}

// DecryptPayload decrypts AES-256-GCM data using the passphrase.
func DecryptPayload(cipherText []byte, passphrase string) ([]byte, error) {
	if len(cipherText) < 28 {
		return nil, errors.New("ciphertext payload is too short")
	}

	salt := cipherText[:16]
	nonce := cipherText[16:28]
	data := cipherText[28:]

	key := DeriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, data, nil)
}
