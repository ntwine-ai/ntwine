package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/user"
)

var magicHeader = []byte("SSLP")

func deriveKey() ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	salt := []byte("socratic-slopinar-local-config-v1")
	h := sha256.New()
	h.Write([]byte(hostname + u.Username))
	h.Write(salt)
	return h.Sum(nil), nil
}

func encrypt(plaintext []byte) ([]byte, error) {
	key, err := deriveKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	result := make([]byte, len(magicHeader)+len(ciphertext))
	copy(result, magicHeader)
	copy(result[len(magicHeader):], ciphertext)
	return result, nil
}

func decrypt(data []byte) ([]byte, error) {
	key, err := deriveKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	ciphertext := data[len(magicHeader):]
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

func hasEncryptionHeader(data []byte) bool {
	if len(data) < len(magicHeader) {
		return false
	}
	for i, b := range magicHeader {
		if data[i] != b {
			return false
		}
	}
	return true
}

func encryptAndWrite(path string, plaintext []byte) error {
	encrypted, err := encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	return os.WriteFile(path, encrypted, 0o600)
}

func readAndDecrypt(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	if hasEncryptionHeader(data) {
		plaintext, err := decrypt(data)
		if err != nil {
			return nil, false, fmt.Errorf("decrypt %s: %w", path, err)
		}
		return plaintext, true, nil
	}
	return data, false, nil
}
