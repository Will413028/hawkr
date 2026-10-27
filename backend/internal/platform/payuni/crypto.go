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
)

const (
	keyLen = 32
	ivLen  = 16
)

// encrypt 以 AES-256-GCM（16 bytes 的 IV，PayUni 的規格）加密，輸出 hex(base64(密文) + ":::" + base64(tag))。
func encrypt(plain string, key, iv []byte) (string, error) {
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(iv) != ivLen {
		return "", fmt.Errorf("payuni：HashIV 必須是 %d bytes", ivLen)
	}
	sealed := aead.Seal(nil, iv, []byte(plain), nil)
	tagAt := len(sealed) - aead.Overhead()
	combined := base64.StdEncoding.EncodeToString(sealed[:tagAt]) + ":::" + base64.StdEncoding.EncodeToString(sealed[tagAt:])
	return hex.EncodeToString([]byte(combined)), nil
}

// decrypt 解 encrypt 的格式；GCM 的 tag 同時驗證內容沒有被改動。
func decrypt(encrypted string, key, iv []byte) (string, error) {
	raw, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("payuni：hex 解碼：%w", err)
	}
	parts := strings.Split(string(raw), ":::")
	if len(parts) != 2 {
		return "", errors.New("payuni：密文格式不合")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("payuni：密文 base64：%w", err)
	}
	tag, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("payuni：tag base64：%w", err)
	}
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(iv) != ivLen {
		return "", fmt.Errorf("payuni：HashIV 必須是 %d bytes", ivLen)
	}
	plain, err := aead.Open(nil, iv, append(ciphertext, tag...), nil)
	if err != nil {
		return "", fmt.Errorf("payuni：解密：%w", err)
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != keyLen {
		return nil, fmt.Errorf("payuni：HashKey 必須是 %d bytes", keyLen)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithNonceSize(block, ivLen)
}

// hashInfo 是 SHA256(HashKey + EncryptInfo + HashIV) 的大寫 hex。
func hashInfo(encryptInfo string, key, iv []byte) string {
	sum := sha256.Sum256([]byte(string(key) + encryptInfo + string(iv)))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
