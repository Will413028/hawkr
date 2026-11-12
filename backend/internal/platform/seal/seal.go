// Package seal 是防詐比對用的 blind index：MAC 以 HMAC-SHA256 計算（只能比對相等、不能還原）。金鑰環是 id:base64 的清單，
// 第一把用來計算，全部都能用來查詢；每個 hash 帶算出它的金鑰 ID，輪替時才知道要用哪一把重算。
// 平台不再加密保存個資（Stripe 計畫 D22：不收身分證號與銀行帳號，統編是明文的市集登記資料），所以沒有加密的那一半。
package seal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
)

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

// ParseMAC 讀出 HMAC 金鑰。格式是逗號分隔的 id:base64，例如 "k2:…,k1:…"，第一把是目前的金鑰，每把至少 32 bytes。
func ParseMAC(s string) (*MAC, error) {
	keys := map[string][]byte{}
	var ids []string
	for _, part := range strings.Split(s, ",") {
		id, b64, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || id == "" || len(id) > 255 {
			return nil, errors.New("seal：金鑰的格式是 id:base64")
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) < minMACKeyLen {
			return nil, fmt.Errorf("seal：金鑰 %s 要是 base64 且至少 %d bytes", id, minMACKeyLen)
		}
		if _, dup := keys[id]; dup {
			return nil, fmt.Errorf("seal：金鑰 %s 重複", id)
		}
		ids = append(ids, id)
		keys[id] = key
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
