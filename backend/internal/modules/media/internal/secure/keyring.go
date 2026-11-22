package secure

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"uuid"
)

const maxKEKIDLen = 255

// Keyring 是 KEK 的金鑰環：第一把（primary）用來寫入，全部都能讀。
type Keyring struct {
	ids  []string
	keys map[string][]byte
}

// ParseKeyring 讀出金鑰環。格式比照 internal/platform/seal：單一字串、逗號分隔的 `<kek_id>:<標準 base64 的 32 位元組>`，
// 第一把是 primary。空字串、格式錯、長度不是 32、id 重複都回錯誤（組裝點啟動失敗）。錯誤不含金鑰內容。
func ParseKeyring(s string) (*Keyring, error) {
	if strings.TrimSpace(s) == "" {
		return nil, errors.New("secure：KEK 設定是空的")
	}
	k := &Keyring{keys: map[string][]byte{}}
	for _, part := range strings.Split(s, ",") {
		id, b64, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || id == "" || len(id) > maxKEKIDLen {
			return nil, errors.New("secure：KEK 的格式是 id:base64")
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) != KeySize {
			return nil, fmt.Errorf("secure：KEK %s 要是標準 base64 的 %d 位元組", id, KeySize)
		}
		if _, dup := k.keys[id]; dup {
			return nil, fmt.Errorf("secure：KEK %s 重複", id)
		}
		k.ids = append(k.ids, id)
		k.keys[id] = key
	}
	return k, nil
}

// PrimaryID 回傳新寫入使用的 kek_id。
func (k *Keyring) PrimaryID() string { return k.ids[0] }

// IDs 回傳全部 kek_id，primary 在最前面。
func (k *Keyring) IDs() []string { return append([]string(nil), k.ids...) }

func wrapAAD(assetID uuid.UUID, kekID string) []byte {
	return append(assetID[:], kekID...)
}

// Wrap 以 primary 包裹 DEK，回傳 kek_id 與 wrapped_dek（nonce＋密文＋tag）。AAD 是資產 ID 加 kek_id，
// 防止把別列的 wrapped DEK 換過來。
func (k *Keyring) Wrap(assetID uuid.UUID, dek []byte) (string, []byte, error) {
	return k.WrapWith(k.PrimaryID(), assetID, dek)
}

// WrapWith 以指定的 kek_id 包裹。
func (k *Keyring) WrapWith(kekID string, assetID uuid.UUID, dek []byte) (string, []byte, error) {
	key, ok := k.keys[kekID]
	if !ok {
		return "", nil, fmt.Errorf("secure：找不到 KEK %s", kekID)
	}
	aead, err := newGCM(key)
	if err != nil {
		return "", nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", nil, fmt.Errorf("secure：產生 nonce：%w", err)
	}
	return kekID, aead.Seal(nonce, nonce, dek, wrapAAD(assetID, kekID)), nil
}

// Unwrap 依列上的 kek_id 找金鑰解包；找不到、AAD 不符或被竄改都回錯誤。
func (k *Keyring) Unwrap(assetID uuid.UUID, kekID string, wrapped []byte) ([]byte, error) {
	key, ok := k.keys[kekID]
	if !ok {
		return nil, fmt.Errorf("secure：找不到 KEK %s", kekID)
	}
	aead, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrCorrupt
	}
	nonce, ct := wrapped[:aead.NonceSize()], wrapped[aead.NonceSize():]
	dek, err := aead.Open(nil, nonce, ct, wrapAAD(assetID, kekID))
	if err != nil {
		return nil, ErrCorrupt
	}
	return dek, nil
}
