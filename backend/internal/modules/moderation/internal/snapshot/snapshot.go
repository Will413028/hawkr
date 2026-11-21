// Package snapshot 是檢舉當下的內容快照的正規化與雜湊（A7-D20）：快照內容由擁有者模組提供（social 的本文、媒體參照與作者；buyer 的公開檔案欄位；
// review 的評價；im 的被檢舉訊息與前後文），這裡依對象種類整理成各自的 struct（tagged union：Content 恰好有一個種類專屬的欄位），序列化成確定的 JSON 文字，
// 再取 SHA-256。同一份內容永遠得到同一個雜湊，內容改過雜湊就不同；雜湊不依賴各表的 version。
package snapshot

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// Content 是快照的內容：Kind 決定哪一個欄位有值（貼文、留言、討論區留言用 Social，買家用 Buyer，評價用 Review，聊天訊息用 Chat），其餘是 nil。
type Content struct {
	Kind   model.TargetKind `json:"kind"`
	Social *Social          `json:"social,omitempty"`
	Buyer  *Buyer           `json:"buyer,omitempty"`
	Review *Review          `json:"review,omitempty"`
	Chat   *Chat            `json:"chat,omitempty"`
	// ReviewReply 是店家對評價的回覆；Kind 為 REVIEW_REPLY。
	ReviewReply *ReviewReply `json:"review_reply,omitempty"`
}

// Social 是貼文、留言與討論區留言的快照。AuthoredAt 是作者發文的時間，PostID 是貼文與貼文留言連結的貼文，ThreadID 是討論區留言所屬的討論串（與 StoreID 組成連結）：
// 處分依據的內容版本在檢舉當下一併凍結（A7-D29）。沒有用到的欄位是零值，不出現在 JSON。
type Social struct {
	AuthorKind string    `json:"author_kind"`
	AuthorID   int64     `json:"author_id"`
	StoreID    int64     `json:"store_id"`
	Body       string    `json:"body"`
	MediaRefs  []string  `json:"media_refs"`
	AuthoredAt time.Time `json:"authored_at,omitzero"`
	PostID     string    `json:"post_id,omitempty"`
	ThreadID   string    `json:"thread_id,omitempty"`
}

// ReviewReply 是店家回覆的快照：作者與歸屬都是評價所屬的店家（StoreID）；ReviewID 是評價 ID（回覆沒有自己的 ID），ReviewerID 是評價的買家（上下文）。
type ReviewReply struct {
	StoreID    int64     `json:"store_id"`
	ReviewID   string    `json:"review_id"`
	ReviewerID int64     `json:"reviewer_id"`
	Body       string    `json:"body"`
	AuthoredAt time.Time `json:"authored_at,omitzero"`
}

// Buyer 是被檢舉買家的公開檔案。
type Buyer struct {
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
}

// Review 是評價的快照：作者永遠是買家；SubjectStoreID 是評價所屬的店家（上下文，不是作者，所以不放在 StoreOf 看的欄位）；ReplyBody 是店家回覆，當作上下文。
type Review struct {
	AuthorID       int64     `json:"author_id"`
	SubjectStoreID int64     `json:"subject_store_id"`
	ProductID      int64     `json:"product_id"`
	Rating         int       `json:"rating"`
	Body           string    `json:"body"`
	MediaRefs      []string  `json:"media_refs"`
	AuthoredAt     time.Time `json:"authored_at,omitzero"`
	ReplyBody      string    `json:"reply_body,omitempty"`
}

// Chat 是聊天訊息的快照：被檢舉的那則放在 Author*、Body、MediaRefs、AuthoredAt；StoreID 在寄件方是店家時等於 AuthorID、買家寄件時為 0；
// Context 是前後各 model.ChatContextWindow 則（不含被檢舉的那則），依 seq 由小到大。
type Chat struct {
	AuthorKind     string           `json:"author_kind"`
	AuthorID       int64            `json:"author_id"`
	StoreID        int64            `json:"store_id"`
	Body           string           `json:"body"`
	MediaRefs      []string         `json:"media_refs"`
	AuthoredAt     time.Time        `json:"authored_at,omitzero"`
	ConversationID string           `json:"conversation_id"`
	Seq            int64            `json:"seq"`
	Context        []ContextMessage `json:"context"`
}

// ContextMessage 是聊天前後文的一則。已隱藏或已抹除的訊息是墓碑：Hidden 或 Erased 為真，不帶本文與圖片（Normalize 強制）。
type ContextMessage struct {
	Seq        int64     `json:"seq"`
	SenderKind string    `json:"sender_kind"`
	SenderID   int64     `json:"sender_id"`
	Type       string    `json:"type"`
	Body       string    `json:"body,omitempty"`
	MediaRefs  []string  `json:"media_refs,omitempty"`
	SentAt     time.Time `json:"sent_at"`
	Hidden     bool      `json:"hidden,omitempty"`
	Erased     bool      `json:"erased,omitempty"`
}

// Author 回傳快照凍結的作者；買家對象沒有作者（ok 為假）。評價的作者永遠是買家。
func (c Content) Author() (kind string, id int64, ok bool) {
	switch {
	case c.Social != nil:
		return c.Social.AuthorKind, c.Social.AuthorID, true
	case c.Review != nil:
		return string(social.ActorBuyer), c.Review.AuthorID, true
	case c.Chat != nil:
		return c.Chat.AuthorKind, c.Chat.AuthorID, true
	case c.ReviewReply != nil:
		return string(social.ActorStore), c.ReviewReply.StoreID, true
	}
	return "", 0, false
}

// StoreID 回傳快照凍結的歸屬店家（社群內容的歸屬店家；聊天訊息寄件方是店家時的店家）；其他是 0。
func (c Content) StoreID() int64 {
	switch {
	case c.Social != nil:
		return c.Social.StoreID
	case c.Chat != nil:
		return c.Chat.StoreID
	case c.ReviewReply != nil:
		return c.ReviewReply.StoreID
	}
	return 0
}

// AssetIDs 回傳快照引用的全部圖片資產（證據保全）：被檢舉內容的圖片，聊天加上前後文的圖片；排序去重。
func (c Content) AssetIDs() []string {
	var refs []string
	switch {
	case c.Social != nil:
		refs = c.Social.MediaRefs
	case c.Review != nil:
		refs = c.Review.MediaRefs
	case c.Chat != nil:
		refs = slices.Clone(c.Chat.MediaRefs)
		for _, m := range c.Chat.Context {
			if !m.Hidden && !m.Erased {
				refs = append(refs, m.MediaRefs...)
			}
		}
	}
	out := slices.Clone(refs)
	slices.Sort(out)
	out = slices.Compact(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// Takedownable 說明這份快照是不是店家寫的內容的種類（社群內容、聊天訊息、評價回覆）：只有這些能走 seller 的內容下架措施。
func (c Content) Takedownable() bool { return c.Social != nil || c.Chat != nil || c.ReviewReply != nil }

// Validate 檢查 Kind 與種類專屬欄位一致：恰好有一個欄位有值，而且就是 Kind 對應的那個。
func (c Content) Validate() error {
	set := 0
	for _, p := range []bool{c.Social != nil, c.Buyer != nil, c.Review != nil, c.Chat != nil, c.ReviewReply != nil} {
		if p {
			set++
		}
	}
	var ok bool
	switch c.Kind {
	case model.TargetPost, model.TargetComment, model.TargetForumEntry:
		ok = c.Social != nil
	case model.TargetBuyer:
		ok = c.Buyer != nil
	case model.TargetReview:
		ok = c.Review != nil
	case model.TargetChatMessage:
		ok = c.Chat != nil
	case model.TargetReviewReply:
		ok = c.ReviewReply != nil
	}
	if !ok || set != 1 {
		return fmt.Errorf("snapshot：種類 %q 與欄位不一致", c.Kind)
	}
	return nil
}

func body(s string) string {
	return trimtext.Trim(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"))
}

func at(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) } // 資料庫的精度；讀回與重算的雜湊一致

func refs(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	out = slices.Compact(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// Normalize 回傳正規化後的內容：換行統一成 LF、本文與名稱去頭尾空白（同 trimtext 的集合）、媒體參照排序去重、nil 切片變成空切片；
// 聊天前後文依 seq 排序，墓碑不帶本文與圖片。不修改傳入的內容。
func Normalize(c Content) Content {
	if c.Social != nil {
		s := *c.Social
		s.Body, s.AuthoredAt, s.MediaRefs = body(s.Body), at(s.AuthoredAt), refs(s.MediaRefs)
		c.Social = &s
	}
	if c.Buyer != nil {
		b := *c.Buyer
		b.DisplayName, b.Avatar = trimtext.Trim(b.DisplayName), trimtext.Trim(b.Avatar)
		c.Buyer = &b
	}
	if c.Review != nil {
		r := *c.Review
		r.Body, r.ReplyBody, r.AuthoredAt, r.MediaRefs = body(r.Body), body(r.ReplyBody), at(r.AuthoredAt), refs(r.MediaRefs)
		c.Review = &r
	}
	if c.ReviewReply != nil {
		r := *c.ReviewReply
		r.Body, r.AuthoredAt = body(r.Body), at(r.AuthoredAt)
		c.ReviewReply = &r
	}
	if c.Chat != nil {
		ch := *c.Chat
		ch.Body, ch.AuthoredAt, ch.MediaRefs = body(ch.Body), at(ch.AuthoredAt), refs(ch.MediaRefs)
		ctx := make([]ContextMessage, len(ch.Context))
		for i, m := range ch.Context {
			m.SentAt = at(m.SentAt)
			if m.Hidden || m.Erased {
				m.Body, m.MediaRefs = "", nil
			} else {
				m.Body = body(m.Body)
				if len(m.MediaRefs) > 0 {
					m.MediaRefs = refs(m.MediaRefs)
				} else {
					m.MediaRefs = nil
				}
			}
			ctx[i] = m
		}
		slices.SortStableFunc(ctx, func(a, b ContextMessage) int {
			switch {
			case a.Seq < b.Seq:
				return -1
			case a.Seq > b.Seq:
				return 1
			}
			return 0
		})
		ch.Context = ctx
		c.Chat = &ch
	}
	return c
}

// Decode 讀回 Encode 產生的 JSON 文字；種類與欄位不一致回錯誤。
func Decode(text string) (Content, error) {
	var c Content
	if err := json.Unmarshal([]byte(text), &c); err != nil {
		return Content{}, err
	}
	if err := c.Validate(); err != nil {
		return Content{}, err
	}
	return c, nil
}

// ErrInvalid 是 Encode 收到種類與欄位不一致的內容。
var ErrInvalid = errors.New("snapshot：內容不合法")

// Encode 回傳正規化後內容的 JSON 文字與它的 SHA-256；種類與欄位不一致是呼叫端的接線錯誤，panic。
func Encode(c Content) (text string, sum [sha256.Size]byte) {
	if err := c.Validate(); err != nil {
		panic(fmt.Errorf("%w：%w", ErrInvalid, err))
	}
	b, err := json.Marshal(Normalize(c))
	if err != nil {
		panic(err) // 只有基本型別的 struct，不可能失敗
	}
	return string(b), sha256.Sum256(b)
}

// StoreOf 回傳快照凍結的作者店家（同內容下架的歸屬驗證：作者是店家且作者 ID 就是歸屬店家）：作者不是店家、買家與評價對象或快照無法讀回時是 nil。
func StoreOf(text string) *int64 {
	c, err := Decode(text)
	if err != nil {
		return nil
	}
	kind, id, ok := c.Author()
	store := c.StoreID()
	if !ok || kind != string(social.ActorStore) || store <= 0 || id != store {
		return nil
	}
	return &store
}
