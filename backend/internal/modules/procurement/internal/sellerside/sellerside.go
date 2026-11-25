// Package sellerside 是賣家對詢價的使用：列出與讀取發給自己的詢價、回價草稿、送出正式回價、撤回與婉拒（SellerProcurementAppService）。
//
// 可見性只看邀請列的 (publication_id, store_id)，不依賴 buyer_id：買家刪除（buyer_id 為 NULL）後被邀請的店照常讀得到，沒被邀請的店與不存在的詢價都是同一個 NOT_FOUND（A9-D44(2)）。
// 賣家能力 CAP_MARKETING 在這裡檢查（讀取在查詢之前，寫入在 transaction 內先鎖店家列再檢查）；handler 只推導店家。
// 寫入在下列情況回 FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED（A9-D45(3)）：賣家已婉拒、這張需求有 ACTIVE 的接受單、買家已刪除、回價期限已過；全部由列推導，沒有儲存的「已結案」狀態（S6）。
// 寫入先鎖店家列（seller 在 procurement 之下），再鎖回價身分列（與買家的 AcceptSellerQuote 鎖同一列，兩邊串行），最後以 FOR SHARE 鎖發布列讀 buyer_id（BuyerDeleted 要等寫入提交）。
package sellerside

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// RequiredCapability 是賣家回價需要的賣家能力；宣告表（httpapi/sellerprocurement）引用它，檢查只在這個套件。
const RequiredCapability = seller.CapMarketing

// 分頁。
const (
	DefaultPageSize = 20
	MaxPageSize     = 50
)

// Stores 是 seller 的方法（*seller.Module 滿足它）：LockNotRetired 在 transaction 內以 FOR KEY SHARE 鎖店家列，Store 讀店家此刻的狀態。
type Stores interface {
	LockNotRetired(ctx context.Context, storeID int64) error
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Service 是賣家回價的 use case。
type Service struct {
	DB     *db.DB
	Now    func() time.Time
	Stores Stores
}

func (s Service) check() error {
	if s.Stores == nil {
		return errors.New("sellerside：Service.Stores 必填")
	}
	return nil
}

// authorize 檢查店家有 RequiredCapability；已關閉回 NOT_FOUND，停權回 PERMISSION_DENIED，其他前置條件沒滿足回 FAILED_PRECONDITION（seller.Authorize）。
func (s Service) authorize(ctx context.Context, storeID int64) error {
	st, err := s.Stores.Store(ctx, storeID)
	if err != nil {
		return err
	}
	return seller.Authorize(st, RequiredCapability)
}

// guardWrite 是寫入的守門，要在 transaction 內、先於 procurement 的任何列：鎖店家列（已退場回 NOT_FOUND）後再讀店家此刻的狀態檢查能力，
// 所以檢查與寫入之間提交的停權會被看到（fail-closed）。
func (s Service) guardWrite(ctx context.Context, storeID int64) error {
	if err := s.Stores.LockNotRetired(ctx, storeID); err != nil {
		return err
	}
	return s.authorize(ctx, storeID)
}

func notFound() error { return apperr.NotFound("找不到這筆詢價") }

func recipientClosed(msg string) error {
	return apperr.FailedPrecondition("%s", msg).WithReason(apperr.ReasonRfqRecipientClosed)
}

// view 是一筆發給這家店的詢價：賣家看得到的內容與它的 hash 都由核准與需求版本重算（S5、A9-D35）。
type view struct {
	pub     store.ProcurementRfqPublication
	payload model.SellerFacing
	hash    string
}

func newView(pub store.ProcurementRfqPublication, a store.ProcurementRfqPublicationApproval, rev store.ProcurementRequestRevision) (view, error) {
	payload, hash, err := rfq.SellerFacingOf(a, rev)
	if err != nil {
		return view{}, err
	}
	return view{pub: pub, payload: payload, hash: hash}, nil
}

func (v view) summary() model.SellerPublicationSummary {
	return model.SellerPublicationSummary{
		PublicationID: v.pub.ID, Version: v.pub.Version, SellerFacingHash: v.hash, PublishedAt: v.pub.PublishedAt,
	}
}

// loadView 讀一筆發給這家店的詢價；沒有邀請列（不存在、別家的、沒被邀請）回 NOT_FOUND。
func loadView(ctx context.Context, q *store.Queries, storeID int64, publicationID uuid.UUID) (view, error) {
	row, err := q.GetSellerPublicationView(ctx, store.GetSellerPublicationViewParams{PublicationID: publicationID, StoreID: storeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return view{}, notFound()
	}
	if err != nil {
		return view{}, fmt.Errorf("sellerside：讀取詢價 %s：%w", publicationID, err)
	}
	return newView(row.ProcurementRfqPublication, row.ProcurementRfqPublicationApproval, row.ProcurementRequestRevision)
}

// versionOf 把一個回價版本的列轉成回應：狀態、hash 與是否超出區間都由列推導（區間在發布核准，已隨賣家可見內容帶來）。
func versionOf(resp store.ProcurementSellerResponse, v store.ProcurementSellerResponseVersion, payload model.SellerFacing, now time.Time) (model.SellerResponseVersion, error) {
	qt, err := quote.QuoteOf(v)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	hash, err := quote.PayloadHash(qt)
	if err != nil {
		return model.SellerResponseVersion{}, err
	}
	out := model.SellerResponseVersion{
		ResponseID: resp.ID, Version: v.Version, PublicationID: resp.PublicationID,
		Status:      quote.VersionStatus(v.Version, resp.LatestVersion, resp.WithdrawnVersion, v.ValidUntil, now),
		PayloadHash: hash, SubmittedAt: v.SubmittedAt, Quote: qt,
		OutsideRange: quote.OutsideRange(payload.LowerBoundAmount, payload.UpperBoundAmount, payload.BasisQuantity, payload.BasisUnit, v),
	}
	if v.Version < resp.LatestVersion {
		out.SupersededByVersion = v.Version + 1
	}
	return out, nil
}

func draftOf(d store.ProcurementSellerResponseDraft) (*model.SellerDraft, error) {
	var q model.QuoteInput
	if err := json.Unmarshal(d.Payload, &q); err != nil {
		return nil, fmt.Errorf("sellerside：草稿 %s 的內容讀不開：%w", d.SellerResponseID, err)
	}
	return &model.SellerDraft{Version: d.Version, UpdatedAt: d.UpdatedAt, Quote: q}, nil
}

// closureOf 把身分列轉成結案狀態。
func closureOf(r store.ProcurementSellerResponse) model.ResponseClosure {
	c := model.ResponseClosure{ResponseID: r.ID, PublicationID: r.PublicationID, LatestVersion: r.LatestVersion, WriteClosedAt: r.WriteClosedAt}
	if r.WithdrawnVersion != nil {
		c.WithdrawnVersion = *r.WithdrawnVersion
	}
	if r.WriteClosedReason != nil {
		c.WriteClosedReason = *r.WriteClosedReason
	}
	return c
}

// ensureAndLock 建立身分列（只有被邀請的店會建立）並鎖住它；沒有邀請列回 NOT_FOUND。要在 loadView 之後呼叫（沒被邀請的店已先回 NOT_FOUND）。
func (s Service) ensureAndLock(ctx context.Context, q *store.Queries, storeID int64, publicationID uuid.UUID) (store.ProcurementSellerResponse, error) {
	if err := q.EnsureSellerResponse(ctx, store.EnsureSellerResponseParams{ID: uuid.NewV7(), PublicationID: publicationID, StoreID: storeID, Now: s.Now()}); err != nil {
		return store.ProcurementSellerResponse{}, fmt.Errorf("sellerside：建立回價身分列：%w", err)
	}
	return s.lock(ctx, q, storeID, publicationID)
}

// lock 鎖住這家店對這筆詢價的身分列；沒有列回 NOT_FOUND。
func (s Service) lock(ctx context.Context, q *store.Queries, storeID int64, publicationID uuid.UUID) (store.ProcurementSellerResponse, error) {
	r, err := q.LockSellerResponseForStore(ctx, store.LockSellerResponseForStoreParams{PublicationID: publicationID, StoreID: storeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementSellerResponse{}, notFound()
	}
	if err != nil {
		return store.ProcurementSellerResponse{}, fmt.Errorf("sellerside：鎖回價身分列：%w", err)
	}
	return r, nil
}

// checkWritable 檢查寫入的四個停止條件（A9-D45(3)）：已婉拒、買家已刪除、這張需求有 ACTIVE 的接受單、回價期限已過。
// 先看身分列與發布列已有的事實，最後才查接受單。
func checkWritable(ctx context.Context, q *store.Queries, resp store.ProcurementSellerResponse, v view, now time.Time) error {
	buyerID, err := q.LockPublicationBuyer(ctx, v.pub.ID)
	if err != nil {
		return fmt.Errorf("sellerside：鎖發布列 %s：%w", v.pub.ID, err)
	}
	switch {
	case resp.WriteClosedAt != nil:
		return recipientClosed("你已婉拒這筆詢價")
	case buyerID == nil:
		return recipientClosed("買家已刪除，這筆詢價不再收報價")
	case v.payload.ResponseDate != nil && rfq.DeadlinePassed(*v.payload.ResponseDate, now):
		return recipientClosed("這筆詢價的回價期限已過")
	}
	active, err := q.HasActiveAcceptance(ctx, resp.RequestID)
	if err != nil {
		return fmt.Errorf("sellerside：讀取需求 %s 的成交：%w", resp.RequestID, err)
	}
	if active {
		return recipientClosed("買家已接受報價，這筆詢價不再收報價")
	}
	return nil
}
