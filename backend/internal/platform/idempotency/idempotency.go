// Package idempotency 是冪等重送比對「內容是否相同」的雜湊編碼：欄位依固定順序、以長度前綴編碼，不會因串接而混淆。
package idempotency

import (
	"crypto/sha256"
	"fmt"
	"hash"
)

// Hasher 累積正規化後的欄位，Sum 回傳 SHA-256。
type Hasher struct{ h hash.Hash }

// New 建立空的 Hasher。
func New() *Hasher { return &Hasher{h: sha256.New()} }

// Put 依序寫入欄位；沒有的選填欄位由呼叫方傳 "-"。
func (b *Hasher) Put(parts ...string) {
	for _, p := range parts {
		_, _ = fmt.Fprintf(b.h, "%d:%s|", len(p), p)
	}
}

// Sum 回傳目前累積內容的 SHA-256。
func (b *Hasher) Sum() []byte { return b.h.Sum(nil) }

// Hash 是 New、Put、Sum 的簡寫。
func Hash(parts ...string) []byte {
	b := New()
	b.Put(parts...)
	return b.Sum()
}
