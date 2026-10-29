// Package model 是 social 的型別、列舉與各處共用的規則（長度上限、頁大小、預覽截斷）。
package model

import (
	"fmt"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

// 內容文字的長度上限（字元數，rune）。DB 的 CHECK 是第二道防線（schema.sql 的 posts_body_check、comments_body_check、
// forum_entries_body_check）。
const (
	MaxPostBodyRunes    = 2200
	MaxCommentBodyRunes = 2200
	// MaxHideReasonRunes 是隱藏與警告的理由文字、還原的原因的上限。
	MaxHideReasonRunes = 500
	// MaxSourceRefRunes 是處分與隱藏的來源參照（moderation 的案件，不透明）的上限。
	MaxSourceRefRunes = 200
	// PreviewMaxRunes 是事件裡內文預覽的字元數（以 rune 截斷，不切在字的中間）。
	PreviewMaxRunes = 100
)

// 清單每頁筆數的預設值與上限（舊 social 的 domain/constants.go：預設 20、上限 50；後台的清單上限 100 屬於 moderation）。
const (
	DefaultPageSize int32 = 20
	MaxPageSize     int32 = 50
)

// 熱門標籤的窗口：[from, to) 最長 30 天（超過是 InvalidInput），預設也是 30 天（呼叫端沒給窗口時用）。舊 social 允許到 1 年，
// 沒有 client 用超過 30 天；1 年的窗口在 10⁵ 貼文下 p95 392 ms，30 天 44 ms（step-1a-perf/README.md）。
const (
	MaxTrendingWindow     = 30 * 24 * time.Hour
	DefaultTrendingWindow = MaxTrendingWindow
)

// MaxTopPosts 是 TopPosts 能回的篇數上限，也是它從最新貼文取樣的篇數（website 現在的算法：最新 30 篇、依瀏覽數取前幾名）。
const MaxTopPosts = 30

// PostStatus 是貼文的狀態：DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED。
type PostStatus string

const (
	StatusDraft       PostStatus = "DRAFT"
	StatusPublished   PostStatus = "PUBLISHED"
	StatusUnpublished PostStatus = "UNPUBLISHED"
)

// PostStatusValues 回傳全部狀態；DB 的 CHECK 與它一致（模組的 TestChecksMatchGo）。
func PostStatusValues() []PostStatus {
	return []PostStatus{StatusDraft, StatusPublished, StatusUnpublished}
}

// ActorKind 是互動者的種類：BUYER 的 ID 是 buyer_id，STORE 的是 store_id；兩邊各自連號，所以身分一律成對比較。
type ActorKind string

const (
	ActorBuyer ActorKind = "BUYER"
	ActorStore ActorKind = "STORE"
)

// ActorKindValues 回傳全部種類；DB 的 CHECK 與它一致。
func ActorKindValues() []ActorKind { return []ActorKind{ActorBuyer, ActorStore} }

// Actor 是一次互動實際使用的身分 (kind, id)。
type Actor struct {
	Kind ActorKind
	ID   int64
}

// Caller 是呼叫者此刻的兩個身分：買家（BuyerID，0 表示沒有）與店家（Store，nil 表示沒有）。互動者由伺服器依資源歸屬推導：
// 呼叫者此刻的店擁有該貼文或討論區時是 STORE，否則是 BUYER（ADR 2026-10-17-social-actor-identity S2）。Store 是已解析、
// 有推導過條件的店家（seller.CurrentStore 或 seller.Store 的結果），能力在 social 內以 seller.Authorize 檢查。
type Caller struct {
	BuyerID int64
	Store   *seller.Store
}

// OwnsStore 回報呼叫者此刻的店是不是 storeID。
func (c Caller) OwnsStore(storeID int64) bool {
	return c.Store != nil && c.Store.StoreID == storeID
}

// IsAuthor 回報 (kind, id) 是不是呼叫者自己的某個身分。
func (c Caller) IsAuthor(a Actor) bool {
	switch a.Kind {
	case ActorBuyer:
		return c.BuyerID != 0 && c.BuyerID == a.ID
	case ActorStore:
		return c.Store != nil && c.Store.StoreID == a.ID
	}
	return false
}

// PostStats 是貼文的四個計數。
type PostStats struct {
	Likes, Comments, Views, Shares int64
}

// Post 是一篇貼文。
type Post struct {
	ID       uuid.UUID
	StoreID  int64
	Body     string
	ImageKey string
	// ProductID 是貼文帶的商品，nil 表示沒有。
	ProductID *int64
	Status    PostStatus
	// PublishedAt 是第一次發布的時間，草稿是 nil；之後不變，下架不清它、再發布沿用。
	PublishedAt *time.Time
	// Version 是樂觀鎖的版本，對外是 etag。
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Hashtags  []Hashtag
	Stats     PostStats
}

// Hashtag 是貼文的一個標籤：Name 是正規化後的名稱，Display 是使用者輸入的樣子。
type Hashtag struct {
	Name    string
	Display string
}

// HashtagCount 是一個標籤與它的公開貼文數。
type HashtagCount struct {
	Name  string
	Posts int64
}

// PostCursor 是動態牆與店家頁的排序鍵：(PublishedAt, ID) 由大到小。
type PostCursor struct {
	PublishedAt time.Time
	ID          uuid.UUID
}

// OwnPostCursor 是店主清單的排序鍵：(CreatedAt, ID) 由大到小。
type OwnPostCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// PostPage 是一頁貼文。Next 為 nil 表示這是最後一頁。
type PostPage struct {
	Posts []Post
	Next  *PostCursor
}

// OwnPostPage 是店主的一頁貼文。
type OwnPostPage struct {
	Posts []Post
	Next  *OwnPostCursor
}

// CreatePostInput 是新增貼文。ImageToken 是新上傳的圖的 staged token；Publish 為真時同一個動作發布，公開要件（恰好一張圖、內文
// 必填）在轉為公開時驗證，草稿可以不完整。
type CreatePostInput struct {
	Body       string
	ImageToken string
	ProductID  *int64
	Hashtags   []string
	Publish    bool
}

// UpdatePostInput 是修改貼文：Version 是讀取時拿到的版本。ImageToken 空字串沿用現在的圖，非空換成新圖；ClearImage 清掉圖
// （已發布的貼文不能清）。Hashtags 為 nil 保留原本的標籤，非 nil（含空）整批取代。
type UpdatePostInput struct {
	PostID     uuid.UUID
	Version    int64
	Body       string
	ImageToken string
	ClearImage bool
	ProductID  *int64
	Hashtags   *[]string
}

// CommentState 是留言列表項目的狀態：VISIBLE 有內文與作者，DELETED、HIDDEN 是佔位項目（頂層留言不可見而底下有可見的回覆）。
type CommentState string

const (
	CommentVisible CommentState = "VISIBLE"
	CommentDeleted CommentState = "DELETED"
	CommentHidden  CommentState = "HIDDEN"
)

// Comment 是留言列表的一個項目。佔位項目（State 不是 VISIBLE）只有 ID、PostID、建立時間與回覆數，沒有內文與作者。
type Comment struct {
	ID          uuid.UUID
	PostID      uuid.UUID
	ParentID    *uuid.UUID
	ReplyToID   *uuid.UUID
	State       CommentState
	Actor       Actor
	Body        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Likes       int64
	ReplyCount  int64
	LikedByMe   bool
	HiddenState *HiddenInfo
}

// HiddenInfo 是作者讀自己被隱藏的內容時看到的理由。
type HiddenInfo struct {
	Category string
	Text     string
	HiddenAt time.Time
}

// CommentCursor 是留言與回覆的排序鍵：(CreatedAt, ID) 由小到大。
type CommentCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// CommentPage 是一頁留言（或回覆）。
type CommentPage struct {
	Comments []Comment
	Next     *CommentCursor
}

// CommentInput 是新增留言或回覆。ParentID 為 nil 是頂層留言；回覆的父一律壓成頂層，ReplyToID 是被回覆的那一則（同篇、可見）。
type CommentInput struct {
	PostID    uuid.UUID
	Body      string
	ParentID  *uuid.UUID
	ReplyToID *uuid.UUID
}

// ForumInput 是新增討論區留言或回覆（容器是店家）。
type ForumInput struct {
	StoreID   int64
	Body      string
	ParentID  *uuid.UUID
	ReplyToID *uuid.UUID
}

// BodyText 是留言或討論區留言的內文。內文只有在買家刪帳的清除之後才是 NULL，而且那時列一定已刪除（comments_scrubbed_check），
// 所以讀到未刪除的列時一定有值；NULL 回空字串。
func BodyText(b *string) string {
	if b == nil {
		return ""
	}
	return *b
}

// TruncateRunes 以字元（rune）截斷 s 到最多 n 個，不切在字的中間。
func TruncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for count := 0; count < n; count++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[:i]
}

// ForumEntry 是店家討論區的一個項目；佔位項目（State 不是 VISIBLE）沒有內文與作者。
type ForumEntry struct {
	ID         uuid.UUID
	StoreID    int64
	ParentID   *uuid.UUID
	ReplyToID  *uuid.UUID
	State      CommentState
	Actor      Actor
	Body       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ReplyCount int64
}

// ForumPage 是一頁討論區項目。
type ForumPage struct {
	Entries []ForumEntry
	Next    *CommentCursor
}

// TargetKind 是隱藏與還原的對象種類（只收買家寫的留言與討論區留言）；ContentOwner 另外認得 POST。
type TargetKind string

const (
	TargetComment    TargetKind = "COMMENT"
	TargetForumEntry TargetKind = "FORUM_ENTRY"
	TargetPost       TargetKind = "POST"
)

// Target 是一則內容。
type Target struct {
	Kind TargetKind
	ID   uuid.UUID
}

// HideInput 是隱藏一則買家的內容：Category 是社群內容可用的政策分類（platform/policy.CommunityAssessable），Text 是給作者看的理由，
// SourceRef 是 moderation 的案件的不透明參照（social 不讀 moderation）。
type HideInput struct {
	Target    Target
	Category  string
	Text      string
	SourceRef string
	AdminID   int64
}

// RestoreInput 是管理員更正：還原目前的隱藏，Reason 是手動填的原因。
type RestoreInput struct {
	Target  Target
	AdminID int64
	Reason  string
}

// Hide 是一筆隱藏紀錄。
type Hide struct {
	ID         uuid.UUID
	Target     Target
	Category   string
	Text       string
	SourceRef  string
	AdminID    int64
	HiddenAt   time.Time
	RestoredAt *time.Time
	// Created 為假表示同一則內容已經有未還原的隱藏，回的是既有的那一筆（冪等）。
	Created bool
}

// SanctionKind 是買家的社群處分種類；目前只有警告。
type SanctionKind string

const SanctionWarning SanctionKind = "WARNING"

// SanctionKindValues 回傳全部種類；DB 的 CHECK 與它一致。
func SanctionKindValues() []SanctionKind { return []SanctionKind{SanctionWarning} }

// WarnInput 是對買家的警告。
type WarnInput struct {
	BuyerID   int64
	Category  string
	Text      string
	SourceRef string
	AdminID   int64
}

// Sanction 是買家的一筆社群處分紀錄。
type Sanction struct {
	ID        uuid.UUID
	BuyerID   int64
	Kind      SanctionKind
	Category  string
	Text      string
	SourceRef string
	AdminID   int64
	CreatedAt time.Time
	EndsAt    *time.Time
}

// SanctionCursor 是處分清單的排序鍵：(CreatedAt, ID) 由大到小。
type SanctionCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// SanctionPage 是一頁處分紀錄。
type SanctionPage struct {
	Sanctions []Sanction
	Next      *SanctionCursor
}

// HiddenContent 是作者讀得到的、自己被隱藏的內容與理由。
type HiddenContent struct {
	HideID    uuid.UUID
	Target    Target
	PostID    *uuid.UUID
	StoreID   *int64
	Body      string
	CreatedAt time.Time
	Category  string
	Text      string
	HiddenAt  time.Time
}

// HiddenCursor 是被隱藏內容清單的排序鍵：(HiddenAt, HideID) 由大到小。
type HiddenCursor struct {
	HiddenAt time.Time
	HideID   uuid.UUID
}

// HiddenPage 是一頁被隱藏的內容。
type HiddenPage struct {
	Items []HiddenContent
	Next  *HiddenCursor
}

// ContentOwner 是 moderation 驗證內容歸屬的結果：內容存在（沒有刪除）、作者 (kind, id)、所屬店家、此刻是不是公開可讀。貼文的作者是它
// 所屬的店家（STORE）。
type ContentOwner struct {
	Exists   bool
	Author   Actor
	StoreID  int64
	Readable bool
}

// TakedownContentError 是內容下架的對象不成立：內容不存在、已刪除，或不是措施的店家自己寫的（買家寫的內容走隱藏）。訂閱者回這個錯誤讓建立措施的
// transaction 回滾；呼叫方用 errors.As 取出是哪一則。
type TakedownContentError struct {
	Kind contentkind.Kind
	ID   uuid.UUID
}

func (e *TakedownContentError) Error() string {
	return fmt.Sprintf("social：內容下架的對象不存在、已刪除或不是這家店寫的：%s %s", e.Kind, e.ID)
}
