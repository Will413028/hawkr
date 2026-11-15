// Package snapshot 是檢舉當下的內容快照的正規化與雜湊（A7-D20）：快照內容由擁有者模組提供（social 的本文、媒體參照與作者；buyer 的公開檔案欄位），
// 這裡把它們整理成同一個形狀，序列化成確定的 JSON 文字，再取 SHA-256。同一份內容永遠得到同一個雜湊，內容改過雜湊就不同；雜湊不依賴各表的 version。
package snapshot

import (
	"crypto/sha256"
	"encoding/json"
	"slices"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// Content 是快照的內容。內容對象用 Author、StoreID、Body、MediaRefs；買家對象用 DisplayName、Avatar；沒用到的欄位是零值。
type Content struct {
	Kind        model.TargetKind `json:"kind"`
	AuthorKind  string           `json:"author_kind"`
	AuthorID    int64            `json:"author_id"`
	StoreID     int64            `json:"store_id"`
	Body        string           `json:"body"`
	MediaRefs   []string         `json:"media_refs"`
	DisplayName string           `json:"display_name"`
	Avatar      string           `json:"avatar"`
}

// Normalize 回傳正規化後的內容：換行統一成 LF、本文與名稱去頭尾空白（同 trimtext 的集合）、媒體參照排序去重、nil 切片變成空切片。
func Normalize(c Content) Content {
	c.Body = trimtext.Trim(strings.ReplaceAll(strings.ReplaceAll(c.Body, "\r\n", "\n"), "\r", "\n"))
	c.DisplayName = trimtext.Trim(c.DisplayName)
	c.Avatar = trimtext.Trim(c.Avatar)
	refs := slices.Clone(c.MediaRefs)
	slices.Sort(refs)
	c.MediaRefs = slices.Compact(refs)
	if c.MediaRefs == nil {
		c.MediaRefs = []string{}
	}
	return c
}

// Decode 讀回 Encode 產生的 JSON 文字。
func Decode(text string) (Content, error) {
	var c Content
	err := json.Unmarshal([]byte(text), &c)
	return c, err
}

// Encode 回傳正規化後內容的 JSON 文字與它的 SHA-256。
func Encode(c Content) (text string, sum [sha256.Size]byte) {
	b, err := json.Marshal(Normalize(c))
	if err != nil {
		panic(err) // 只有基本型別的 struct，不可能失敗
	}
	return string(b), sha256.Sum256(b)
}
