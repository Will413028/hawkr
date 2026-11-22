package storage

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"uuid"
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

// SecurePrefix 回傳機敏資產所有段物件共用的前綴 `secure/<資產 ID>/`；資產列的 object_key 就是它，GC 與實體刪除依它列出物件。
func SecurePrefix(assetID uuid.UUID) string {
	return string(PrefixSecure) + "/" + assetID.String() + "/"
}

// NewSecureChunkKey 回傳 `secure/<資產 ID>/<段序號>/<嘗試 uuid v4>`：每次嘗試寫新的 key，不同嘗試不會互相覆蓋。
func NewSecureChunkKey(assetID uuid.UUID, index int) string {
	return SecurePrefix(assetID) + strconv.Itoa(index) + "/" + strings.TrimPrefix(NewKey(PrefixSecure), string(PrefixSecure)+"/")
}
