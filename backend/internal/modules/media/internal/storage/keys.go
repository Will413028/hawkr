package storage

import (
	"crypto/rand"
	"encoding/hex"
)

// Prefix 是 object key 的第一段。
type Prefix string

const (
	// PrefixTmp 放尚未確認的上傳；bucket lifecycle 與 GC 只處理這個前綴。
	PrefixTmp Prefix = "tmp"
	// PrefixPublic 放公開讀取的資產。
	PrefixPublic Prefix = "public"
	// PrefixSecure 放只以密文存放的機敏資產。
	PrefixSecure Prefix = "secure"
)

// NewKey 回傳 `<prefix>/<uuid v4>`。key 由伺服器隨機產生，不含呼叫端檔名，也不含任何可猜的部分。
func NewKey(p Prefix) string {
	var b [16]byte
	// crypto/rand.Read 不會回傳錯誤（失敗時直接終止程式）。
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	var h [32]byte
	hex.Encode(h[:], b[:])
	return string(p) + "/" + string(h[0:8]) + "-" + string(h[8:12]) + "-" + string(h[12:16]) + "-" + string(h[16:20]) + "-" + string(h[20:32])
}
