package payuni

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-querystring/query"
)

// Encrypt encrypts data using AES-256-GCM with 16-byte IV (PAYUNi spec)
// Output: hex(base64(ciphertext) + ":::" + base64(tag))
func Encrypt(data interface{}, key, iv string) (string, error) {
	v, err := query.Values(data)
	if err != nil {
		return "", fmt.Errorf("failed to encode data: %w", err)
	}
	plainText := v.Encode()

	if plainText == "" {
		return "", errors.New("empty plain text")
	}

	return EncryptString(plainText, key, iv)
}

// EncryptString encrypts a raw string
func EncryptString(plainText, key, iv string) (string, error) {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)

	// Ensure Key is 32 bytes
	var keyArr [32]byte
	copy(keyArr[:], keyBytes)
	if len(keyBytes) > 32 {
		copy(keyArr[:], keyBytes[:32])
	}

	// Ensure IV is 16 bytes (PAYUNi uses 16 bytes, not standard 12)
	var nonceArr [16]byte
	copy(nonceArr[:], ivBytes)
	if len(ivBytes) > 16 {
		copy(nonceArr[:], ivBytes[:16])
	}

	block, err := aes.NewCipher(keyArr[:])
	if err != nil {
		return "", err
	}

	// AES-GCM with 16-byte nonce
	aesgcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", err
	}

	ciphertextWithTag := aesgcm.Seal(nil, nonceArr[:], []byte(plainText), nil)

	// Split ciphertext and tag
	// Tag size is 16 bytes for GCM
	if len(ciphertextWithTag) < 16 {
		return "", errors.New("ciphertext too short")
	}

	tagStart := len(ciphertextWithTag) - 16
	ciphertext := ciphertextWithTag[:tagStart]
	tag := ciphertextWithTag[tagStart:]

	// "hex(base64(ciphertext) + ':::' + base64(tag))"
	cipherBase64 := base64.StdEncoding.EncodeToString(ciphertext)
	tagBase64 := base64.StdEncoding.EncodeToString(tag)

	combined := fmt.Sprintf("%s:::%s", cipherBase64, tagBase64)

	return hex.EncodeToString([]byte(combined)), nil
}

// Decrypt decrypts the specialized PAYUNi format
func Decrypt(encryptStr, key, iv string) (string, error) {
	// 1. Hex Decode
	decodedHex, err := hex.DecodeString(encryptStr)
	if err != nil {
		return "", fmt.Errorf("hex decode error: %w", err)
	}
	combined := string(decodedHex)

	// 2. Split ":::"
	parts := strings.Split(combined, ":::")
	if len(parts) != 2 {
		return "", errors.New("invalid format: missing ':::'")
	}
	cipherBase64 := parts[0]
	tagBase64 := parts[1]

	// 3. Base64 Decode
	ciphertext, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode ciphertext error: %w", err)
	}
	tag, err := base64.StdEncoding.DecodeString(tagBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode tag error: %w", err)
	}

	// 4. Reconstruct ciphertext + tag
	ciphertext = append(ciphertext, tag...)

	// 5. Decrypt
	keyBytes := []byte(key)
	ivBytes := []byte(iv)

	var keyArr [32]byte
	copy(keyArr[:], keyBytes)

	var nonceArr [16]byte
	copy(nonceArr[:], ivBytes)

	block, err := aes.NewCipher(keyArr[:])
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", err
	}

	plaintext, err := aesgcm.Open(nil, nonceArr[:], ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// GenerateHash SHA256(key + encryptInfo + iv).ToUpper()
func GenerateHash(encryptInfo, key, iv string) string {
	raw := fmt.Sprintf("%s%s%s", key, encryptInfo, iv)
	hash := sha256.Sum256([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}
