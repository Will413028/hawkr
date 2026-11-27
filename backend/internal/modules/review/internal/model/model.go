// Package model 是 review 模組的對外型別、常數、事件與錯誤原因。
package model

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

const (
	MinRating = 1
	MaxRating = 5
	// MaxContentRunes 是評價內容與店家回覆的長度上限，以字元（rune）計。
	MaxContentRunes = 1000
	// MaxImages 是一則評價的附圖張數上限；DB 以 review_images.position 的範圍保證。
	MaxImages = 4
	// ReviewWindow 是訂單完成後可以評價的期限。
	ReviewWindow = 30 * 24 * time.Hour
	// OrderStatusCompleted 是訂單狀態「已完成」，只有它能評價。
	OrderStatusCompleted = "50"
	// DefaultPageSize 與 MaxPageSize 是列表的預設與上限（AIP-158）。
	DefaultPageSize = 20
	MaxPageSize     = 100
	// MaxStoreRatingStatsBatch 是批次店家評分一次最多接受的店家數。
	MaxStoreRatingStatsBatch = 100
	MaxHideReasonRunes       = 500
	MaxSourceRefRunes        = 200
)

// Reason 是 review 的業務原因（模組端的常數，與 apperr 的分類搭配使用）。
type Reason string

const (
	ReasonAlreadyExists      Reason = "REVIEW_ALREADY_EXISTS"
	ReasonOrderNotFound      Reason = "ORDER_NOT_FOUND"
	ReasonOrderNotReviewable Reason = "ORDER_NOT_REVIEWABLE"
	ReasonWindowExpired      Reason = "REVIEW_WINDOW_EXPIRED"
	ReasonNotReplyable       Reason = "REVIEW_NOT_REPLYABLE"
	ReasonModerationRejected Reason = "MODERATION_REJECTED"
)

// reasoned 把業務原因附在 apperr 的錯誤上；Unwrap 讓 apperr.KindOf 與 errors.As(*apperr.Error) 照常運作。
type reasoned struct {
	err    *apperr.Error
	reason Reason
}

func (e *reasoned) Error() string { return e.err.Error() }
func (e *reasoned) Unwrap() error { return e.err }

// WithReason 回傳帶業務原因的 err。
func WithReason(err *apperr.Error, r Reason) error { return &reasoned{err: err, reason: r} }

// ReasonOf 回傳 err 帶的業務原因；沒有時是空字串。
func ReasonOf(err error) Reason {
	var r *reasoned
	if errors.As(err, &r) {
		return r.reason
	}
	return ""
}

// ErrOrderNotFound 是 Orders 的實作回報「訂單不存在」的方式；其他錯誤都是暫時性故障或回應不合法（兩者不得混為一談）。
var ErrOrderNotFound = errors.New("review：訂單不存在")

// OrderForReview 是評價資格與快照的唯一來源：訂單與它的項目。
type OrderForReview struct {
	OrderNo     string
	BuyerID     int64
	StoreID     int64
	Status      string
	CompletedAt time.Time
	Items       []OrderItem
}

// OrderItem 是訂單項目。ImageKey 為空表示沒有商品圖。
type OrderItem struct {
	OrderItemID int64
	ProductID   uuid.UUID // 全零 ＝ 沒有商品（RFQ 成交），不能評價
	SkuName     string
	ImageKey    string
	Price       int64
}

// Orders 是 review 對訂單的唯一依賴（遷移期由 legacy/orderservice 實作，ordering 搬進單體後換 in-process）。
// 訂單不存在回 ErrOrderNotFound，其他錯誤原樣回傳。
type Orders interface {
	OrderForReview(ctx context.Context, orderNo string) (OrderForReview, error)
}

// Image 是評價的一張附圖。
type Image struct {
	AssetID   uuid.UUID
	ObjectKey string
}

// Reply 是店家的回覆。
type Reply struct {
	Content   string
	RepliedAt time.Time
}

// Review 是一則評價。Content 空表示沒有文字；Reply 是 nil 表示還沒回覆；ItemImageKey 是 nil 表示訂單項目沒有圖。
type Review struct {
	ID           uuid.UUID
	OrderNo      string
	OrderItemID  int64
	BuyerID      int64
	StoreID      int64
	ProductID    uuid.UUID
	Rating       int
	Content      string
	Images       []Image
	SkuName      string
	ItemImageKey *string
	Price        int64
	CreatedAt    time.Time
	Reply        *Reply
}

// ReportedReview 是檢舉用的評價證據讀取（給 moderation 凍結快照）：作者、本文、評分、圖片 asset ID、發表時間、所屬店家與商品、店家回覆。
// 它不是顯示用的投影（沒有 ItemImageKey、SkuName、Price），兩者各自演進。
type ReportedReview struct {
	ID        uuid.UUID
	BuyerID   int64
	StoreID   int64
	ProductID uuid.UUID
	Rating    int
	Content   string
	// AssetIDs 是評價附圖的 asset ID，依位置排序。
	AssetIDs  []string
	CreatedAt time.Time
	Reply     *Reply
}

// RelayReview 是 relay 讀到的單則評價：Public 是 job 執行當下它是否公開可讀（沒有未還原的隱藏、作者不在刪帳中）。
type RelayReview struct {
	Review
	Public bool
}

// ReviewStore 是評價所屬的店與它現在是否公開可讀（回覆的店家推導用，不套公開可讀述詞）。
type ReviewStore struct {
	StoreID int64
	Public  bool
}

// Cursor 是 keyset 的位置：(created_at, id) 由大到小，之後的是比它更舊的。
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Page 是一頁評價；Next 是 nil 表示沒有下一頁（不回總數，AIP-158）。
type Page struct {
	Reviews []Review
	Next    *Cursor
}

// Filter 是公開列表的篩選。Rating 是 0（不篩）或 1 到 5。
type Filter struct {
	Rating     int
	WithImages bool
}

// SubmitInput 是提交評價的輸入。price、sku_name、product_id 都取自訂單，不在輸入裡。
type SubmitInput struct {
	BuyerID      int64
	OrderNo      string
	OrderItemID  int64
	Rating       int
	Content      string
	StagedTokens []string
}

// ReplyInput 是店家回覆的輸入；Store 是已解析的店家身分。
type ReplyInput struct {
	Store    seller.Store
	ReviewID uuid.UUID
	Content  string
}

// HideInput 是隱藏評價的輸入（moderation 呼叫）。
type HideInput struct {
	ReviewID  uuid.UUID
	Category  string
	Text      string
	SourceRef string
	AdminID   int64
}

// RestoreInput 是還原隱藏的輸入。
type RestoreInput struct {
	ReviewID uuid.UUID
	AdminID  int64
	Reason   string
	// SourceRef 非空時只還原來源參照相同的未還原隱藏；有未還原隱藏但來源不同回 FailedPrecondition。空字串還原目前的那一筆。
	SourceRef string
}

// Hide 是一筆隱藏紀錄；Created 為假表示同一則評價已有未還原的隱藏（冪等回傳既有的）。
type Hide struct {
	ID         uuid.UUID
	ReviewID   uuid.UUID
	Category   string
	Text       string
	SourceRef  string
	AdminID    int64
	HiddenAt   time.Time
	RestoredAt *time.Time
	Created    bool
}

// ProductStats 是商品的評價統計（只算公開可讀的評價）；Distribution[i] 是 (i+1) 星的評價數。
type ProductStats struct {
	Count int64
	// WithImagesCount 是公開可讀且帶圖的評價數，與列表的 with_images 篩選同一個條件。
	WithImagesCount int64
	Average         float64
	Distribution    [5]int64
}

// StoreStats 是店家的評價統計；Distribution[i] 是 (i+1) 星的評價數。
type StoreStats struct {
	Count        int64
	Average      float64
	Distribution [5]int64
}

// StoreRating 是一家有公開評價的店的評分。
type StoreRating struct {
	Count   int64
	Average float64
}

// ReviewCreated 在提交成功的 transaction 內發布。
type ReviewCreated struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	ReviewID   uuid.UUID
	BuyerID    int64
	StoreID    int64
	ProductID  uuid.UUID
}

// Catalog 是 review 同步商品評分的 catalog 公開 API；在改變公開評價集合的同一個 transaction 內呼叫，失敗讓整個 transaction 回滾。
type Catalog interface {
	SetRating(ctx context.Context, productID uuid.UUID, average float64, count int32) error
}

// ReviewHidden 一則評價被隱藏（只在真的新增一筆隱藏時發，冪等的重複隱藏不發），在隱藏的同一個 transaction 內發布。只帶參照與列舉，不帶管理員說明文字；
// SanctionID 是隱藏紀錄的 ID，BuyerID 是評價作者，SourceRef 是案件的不透明參照（沒有案件時空字串）。
type ReviewHidden struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	SanctionID uuid.UUID
	ReviewID   uuid.UUID
	BuyerID    int64
	Category   policy.Category
	SourceRef  string
	AppealPath policy.AppealPath
}

// ReviewRestored 一筆隱藏被管理員還原（同一個 transaction 內發布）；欄位同 ReviewHidden，SanctionID 是被還原的隱藏紀錄。
type ReviewRestored struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	SanctionID uuid.UUID
	ReviewID   uuid.UUID
	BuyerID    int64
	Category   policy.Category
	SourceRef  string
	AppealPath policy.AppealPath
}
