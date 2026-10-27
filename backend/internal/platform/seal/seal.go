// Package seal 是要保存的個資與憑證的加密與比對索引：
//
//   - Keyring 以 AES-256-GCM 加密。密文前面帶金鑰 ID，並以金鑰 ID 與呼叫方給的 AAD（additional authenticated data）
//     認證：密文不能搬到別的金鑰 ID 底下，也不能在呼叫方指定的脈絡（例如申請 ID 與欄位名）之間搬動。換金鑰時把新的
//     放在最前面、舊的留著，舊的密文仍解得開，新的一律用第一把加密。
//   - MAC 以 HMAC-SHA256 計算 blind index（只能比對相等、不能還原）。金鑰環的格式與 Keyring 相同，第一把用來計算，全部
//     都能用來查詢；每個 hash 帶算出它的金鑰 ID，輪替時才知道要用哪一把重算。
//
// 加密金鑰與 HMAC 金鑰必須是不同的金鑰（Distinct）。
package seal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// parseKeys 讀出 id:base64 的清單（逗號分隔，第一把是目前的金鑰），每把金鑰至少 minLen bytes；
// exactLen 為 true 時必須剛好是 minLen。
func parseKeys(s string, minLen int, exactLen bool) (ids []string, keys map[string][]byte, err error) {
	keys = map[string][]byte{}
	for _, part := range strings.Split(s, ",") {
		id, b64, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || id == "" || len(id) > 255 {
			return nil, nil, errors.New("seal：金鑰的格式是 id:base64")
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) < minLen || (exactLen && len(key) != minLen) {
			if exactLen {
				return nil, nil, fmt.Errorf("seal：金鑰 %s 要是 base64 的 %d bytes", id, minLen)
			}
			return nil, nil, fmt.Errorf("seal：金鑰 %s 要是 base64 且至少 %d bytes", id, minLen)
		}
		if _, dup := keys[id]; dup {
			return nil, nil, fmt.Errorf("seal：金鑰 %s 重複", id)
		}
		ids = append(ids, id)
		keys[id] = key
	}
	return ids, keys, nil
}

// Keyring 是有序的加密金鑰：第一把加密，全部都能解密。
type Keyring struct {
	ids  []string
	aead map[string]cipher.AEAD
	raw  map[string][]byte
}

// Parse 讀出加密金鑰。格式是逗號分隔的 id:base64(32 bytes)，例如 "k2:…,k1:…"；第一把是目前的金鑰。
func Parse(s string) (*Keyring, error) {
	ids, raw, err := parseKeys(s, 32, true)
	if err != nil {
		return nil, err
	}
	k := &Keyring{ids: ids, aead: map[string]cipher.AEAD{}, raw: raw}
	for id, key := range raw {
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		k.aead[id] = gcm
	}
	return k, nil
}

// additional 是 GCM 認證的 additional data：沒有脈絡（aad 為空）時就是金鑰 ID，與 identity 原本的 seal 相同，
// 已經存在的密文照樣解得開；有脈絡時是金鑰 ID、一個 0 byte，再接呼叫方的 AAD。
func additional(id string, aad []byte) []byte {
	if len(aad) == 0 {
		return []byte(id)
	}
	out := make([]byte, 0, len(id)+1+len(aad))
	out = append(out, id...)
	out = append(out, 0)
	return append(out, aad...)
}

// Has 回報金鑰環有沒有這個金鑰 ID（啟動時檢查資料庫參照的每一把金鑰都還在）。
func (k *Keyring) Has(id string) bool {
	_, ok := k.aead[id]
	return ok
}

// Seal 以第一把金鑰加密，aad 同時被認證（不加密、不存進密文，解密時必須給同一份）。格式：1 byte 的 ID 長度、ID、
// nonce、密文。
func (k *Keyring) Seal(plaintext, aad []byte) ([]byte, error) {
	id := k.ids[0]
	gcm := k.aead[id]
	out := append([]byte{byte(len(id))}, id...) //nolint:gosec // parseKeys rejects key IDs longer than 255 bytes
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("seal：產生 nonce：%w", err)
	}
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, additional(id, aad)), nil
}

// Open 解密；金鑰 ID 不認得、aad 與加密時不同或密文被改過時回錯。
func (k *Keyring) Open(ciphertext, aad []byte) ([]byte, error) {
	if len(ciphertext) < 1 {
		return nil, errors.New("seal：密文太短")
	}
	n := int(ciphertext[0])
	if len(ciphertext) < 1+n {
		return nil, errors.New("seal：密文太短")
	}
	id := string(ciphertext[1 : 1+n])
	gcm, ok := k.aead[id]
	if !ok {
		return nil, fmt.Errorf("seal：沒有金鑰 %s", id)
	}
	rest := ciphertext[1+n:]
	if len(rest) < gcm.NonceSize() {
		return nil, errors.New("seal：密文太短")
	}
	return gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], additional(id, aad))
}

// Hash 是一個 blind index：KeyID 是算出它的金鑰，Sum 是 HMAC-SHA256。
type Hash struct {
	KeyID string
	Sum   []byte
}

// minMACKeyLen 是 HMAC 金鑰的最短長度（與 SHA-256 的輸出同長）。
const minMACKeyLen = 32

// MAC 是有序的 HMAC 金鑰：第一把計算新的 hash，全部都能計算以便查詢輪替前寫入的 hash。
type MAC struct {
	ids  []string
	keys map[string][]byte
}

// ParseMAC 讀出 HMAC 金鑰，格式同 Parse（id:base64，逗號分隔，第一把是目前的金鑰），每把至少 32 bytes。
func ParseMAC(s string) (*MAC, error) {
	ids, keys, err := parseKeys(s, minMACKeyLen, false)
	if err != nil {
		return nil, err
	}
	return &MAC{ids: ids, keys: keys}, nil
}

// IDs 回傳金鑰環的金鑰 ID，第一把（目前的金鑰）在最前面。
func (m *MAC) IDs() []string { return slices.Clone(m.ids) }

// Has 回報金鑰環有沒有這個金鑰 ID。
func (m *MAC) Has(id string) bool {
	_, ok := m.keys[id]
	return ok
}

func (m *MAC) sum(id string, input []byte) Hash {
	h := hmac.New(sha256.New, m.keys[id])
	h.Write(input)
	return Hash{KeyID: id, Sum: h.Sum(nil)}
}

// Sum 以第一把金鑰計算；寫入的 hash 一律用它。input 由呼叫方加上欄位標籤做 domain separation。
func (m *MAC) Sum(input []byte) Hash { return m.sum(m.ids[0], input) }

// SumAll 以每一把金鑰各算一次（第一把在最前面）；查詢時比對全部，輪替前寫入的 hash 才找得到。
func (m *MAC) SumAll(input []byte) []Hash {
	out := make([]Hash, 0, len(m.ids))
	for _, id := range m.ids {
		out = append(out, m.sum(id, input))
	}
	return out
}

// Distinct 檢查 HMAC 金鑰沒有一把與加密金鑰相同：同一把金鑰兩種用途，洩漏其中一種就連帶破壞另一種。啟動時呼叫。
func Distinct(enc *Keyring, mac *MAC) error {
	for _, mid := range mac.ids {
		for eid, ekey := range enc.raw {
			if subtle.ConstantTimeCompare(mac.keys[mid], ekey) == 1 {
				return fmt.Errorf("seal：HMAC 金鑰 %s 與加密金鑰 %s 相同，兩種用途必須用不同的金鑰", mid, eid)
			}
		}
	}
	return nil
}
