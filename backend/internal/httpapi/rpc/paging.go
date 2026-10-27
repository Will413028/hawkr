package rpc

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"connectrpc.com/connect"
)

// pageTokenVersion 是 token 格式的版本；格式改了就加一，舊的 token 一律無法解讀。
const pageTokenVersion = 1

// EncodePageToken 把上一頁最後一筆的排序鍵（key，由各 handler 自己編碼）編成不透明的 page token
// （Google AIP-158）。scope 是這個 token 適用的請求：procedure 加上 page_size、page_token 以外的
// 參數，換了參數就不能沿用。key 為 nil 表示沒有下一頁，回傳空字串。
func EncodePageToken(scope string, key []byte) string {
	if key == nil {
		return ""
	}
	b := make([]byte, 0, 1+scopeLen+len(key))
	b = append(b, pageTokenVersion)
	b = append(b, scopeHash(scope)...)
	b = append(b, key...)
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodePageToken 取回 EncodePageToken 編進去的排序鍵。token 為空字串時回 nil（第一頁）；
// 無法解讀或 scope 不同時回 INVALID_ARGUMENT。
func DecodePageToken(scope, token string) ([]byte, error) {
	if token == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) <= 1+scopeLen || b[0] != pageTokenVersion || !bytes.Equal(b[1:1+scopeLen], scopeHash(scope)) {
		return nil, Error(connect.CodeInvalidArgument, "%s", "page_token 無法使用：它不是這個查詢的上一頁回傳的，請從第一頁重新讀取")
	}
	return b[1+scopeLen:], nil
}

const scopeLen = 8

func scopeHash(scope string) []byte {
	h := sha256.Sum256([]byte(scope))
	return h[:scopeLen]
}

// PageCursor 把 DecodePageToken 取回的排序鍵解成 T；token 為空字串（第一頁）時回 nil。
func PageCursor[T any](scope, token string) (*T, error) {
	data, err := DecodePageToken(scope, token)
	if err != nil || data == nil {
		return nil, err
	}
	var cursor T
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	return &cursor, nil
}

// TokenFor 把排序鍵 cursor（JSON）編成 page token（EncodePageToken）。
func TokenFor(scope string, cursor any) string {
	data, _ := json.Marshal(cursor)
	return EncodePageToken(scope, data)
}

// ValidCursor 是 PageCursor 加上 T 自己的檢查（Valid）：解得開但內容不合法的 token 回 INVALID_ARGUMENT。
func ValidCursor[T interface{ Valid() bool }](scope, token string) (*T, error) {
	c, err := PageCursor[T](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	if !(*c).Valid() {
		return nil, Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	return c, nil
}

// PageScope 是「某個 procedure、某個擁有者（買家或店家）」的 page token scope。
func PageScope(procedure string, ownerID int64) string {
	return FilterScope(procedure, strconv.FormatInt(ownerID, 10))
}

// FilterScope 是「某個 procedure、某一組篩選條件」的 page token scope：page token 只能用在同一組篩選，換了篩選就從第一頁
// 開始。filters 依固定的順序給（沒有設定的條件也要佔一個位置，例如空字串或 UNSPECIFIED 的名稱）。
func FilterScope(procedure string, filters ...string) string {
	return strings.Join(append([]string{procedure}, filters...), "\x00")
}
