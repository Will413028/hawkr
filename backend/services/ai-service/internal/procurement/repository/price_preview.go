package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ErrPricePreviewNotFound 是 FindPricePreviewByIdempotencyKey 找不到列時的哨兵錯誤——
// 沒有 procurement_price_previews 表自己的「不存在 vs 不屬於這個買家」歧義問題（跟
// ErrNotFound 那組不同：呼叫端只會用它判斷「這個 client_request_id 是不是第一次見到」，
// 不會拿它回應使用者已存在資源的存取控制錯誤，見 Task 6 的消費方式）。
var ErrPricePreviewNotFound = errors.New("procurement: price preview not found")

// ErrPricePreviewIdempotencyConflict（Task 18／Fix 7）是 InsertPricePreview
// 撞到 uq_procurement_price_previews_idempotency 之後，發現「撞鍵之後查到的既有
// 列」跟「這次呼叫的輸入」其實不是同一次試算的哨兵錯誤——見 InsertPricePreview
// 的 doc comment。pricing.PreviewRfqPriceUseCase.PreviewRfqPrice 把它映射成
// domain.ErrIdempotencyKeyConflict，跟應用層那道 FindPricePreviewByIdempotencyKey
// 短路檢查（Task 15／I3）共用同一種對外語意，只是這個是在真正併發撞鍵時的最終仲裁
// 路徑，不是預先查詢的路徑。
var ErrPricePreviewIdempotencyConflict = errors.New("procurement: idempotency key already used for a different price preview payload")

// PricePreview 是一次「報價前試算」的完整持久化結果。ReferenceFactsHash／
// ReferencePayloadJSON／UnavailableReason 依 ReferenceAvailability 互斥
// （見 schema procurement.sql 的表註解），寫入端負責保證組合一致，這裡只搬運。
type PricePreview struct {
	ID                      int64
	BuyerID                 identity.BuyerID
	RequestID               int64
	RequestRevisionID       int64
	PricingMode             string
	QualityKnowledgeState   string
	ReferenceAvailability   string
	ReferenceFactsHash      *string
	ReferencePayloadJSON    json.RawMessage
	UnavailableReason       *string
	BuyerRequestedRangeJSON json.RawMessage
	// MarketPriceBasisJSON／QuoteTotalRangeJSON 是 v6 設計稿 03 市場價格參考
	// 區塊的兩個附加欄位（見 schema procurement.sql 的表註解）。各自可為 nil：
	// 前者與 ReferencePayloadJSON 同生同滅，後者在需求數量未填時沒有值。
	MarketPriceBasisJSON json.RawMessage
	QuoteTotalRangeJSON  json.RawMessage
	DealTemperatureJSON  json.RawMessage
	ClientRequestID      string
	PreviewInputHash     string
	PayloadHash          string
	Version              int64
	ExpiresAt            time.Time
	CreatedAt            time.Time
}

// InsertPricePreviewInput 是一次試算寫入的完整輸入。Version 不在這裡——固定從 1 開始，
// 由 InsertPricePreview 自己填，呼叫端不能覆寫（這張表目前只有 INSERT，沒有更新版本
// 的路徑；Version 欄位保留給未來若試算結果需要覆寫時使用，非本 task 範圍）。
type InsertPricePreviewInput struct {
	BuyerID                 identity.BuyerID
	RequestID               int64
	RequestRevisionID       int64
	PricingMode             string
	QualityKnowledgeState   string
	ReferenceAvailability   string
	ReferenceFactsHash      *string
	ReferencePayloadJSON    json.RawMessage
	UnavailableReason       *string
	BuyerRequestedRangeJSON json.RawMessage
	MarketPriceBasisJSON    json.RawMessage
	QuoteTotalRangeJSON     json.RawMessage
	DealTemperatureJSON     json.RawMessage
	ClientRequestID         string
	PreviewInputHash        string
	PayloadHash             string
	ExpiresAt               time.Time
}

func toPricePreviewEntity(row gen.ProcurementPricePreview) PricePreview {
	return PricePreview{
		ID: row.ID, BuyerID: identity.BuyerID(row.BuyerID), RequestID: row.RequestID,
		RequestRevisionID:       row.RequestRevisionID,
		PricingMode:             row.PricingMode,
		QualityKnowledgeState:   row.QualityKnowledgeState,
		ReferenceAvailability:   row.ReferenceAvailability,
		ReferenceFactsHash:      row.ReferenceFactsHash,
		ReferencePayloadJSON:    row.ReferencePayloadJson,
		UnavailableReason:       row.UnavailableReason,
		BuyerRequestedRangeJSON: row.BuyerRequestedRangeJson,
		MarketPriceBasisJSON:    row.MarketPriceBasisJson,
		QuoteTotalRangeJSON:     row.QuoteTotalRangeJson,
		DealTemperatureJSON:     row.DealTemperatureJson,
		ClientRequestID:         row.ClientRequestID,
		PreviewInputHash:        row.PreviewInputHash,
		PayloadHash:             row.PayloadHash,
		Version:                 row.Version,
		ExpiresAt:               row.ExpiresAt.Time,
		CreatedAt:               row.CreatedAt.Time,
	}
}

// InsertPricePreview 寫入一筆試算結果。冪等：同一買家同一 client_request_id 重送時，
// 回傳前次寫入的那一列，不建立第二筆。
//
// 撞冪等鍵不拋錯（query 用 ON CONFLICT DO UPDATE，見 PostgreSQL 規範第十二章），因此
// 這支方法放進呼叫端的交易裡也不會把它打成 aborted。
//
// ⛔ 回傳既有列前必須比對 PreviewInputHash：兩個「同一個 clientRequestID、不同 payload」
// 的請求真的併發時，撞鍵不代表是同一次試算的重送。usecase 層那次
// FindPricePreviewByIdempotencyKey 短路檢查是優化，不是唯一防線。
func (r *Repo) InsertPricePreview(ctx context.Context, in InsertPricePreviewInput) (PricePreview, error) {
	id := r.newID()
	row, err := r.q(ctx).InsertPricePreview(ctx, gen.InsertPricePreviewParams{
		ID: id, BuyerID: int64(in.BuyerID), RequestID: in.RequestID,
		RequestRevisionID:       in.RequestRevisionID,
		PricingMode:             in.PricingMode,
		QualityKnowledgeState:   in.QualityKnowledgeState,
		ReferenceAvailability:   in.ReferenceAvailability,
		ReferenceFactsHash:      in.ReferenceFactsHash,
		ReferencePayloadJson:    in.ReferencePayloadJSON,
		UnavailableReason:       in.UnavailableReason,
		BuyerRequestedRangeJson: in.BuyerRequestedRangeJSON,
		MarketPriceBasisJson:    in.MarketPriceBasisJSON,
		QuoteTotalRangeJson:     in.QuoteTotalRangeJSON,
		DealTemperatureJson:     in.DealTemperatureJSON,
		ClientRequestID:         in.ClientRequestID,
		PreviewInputHash:        in.PreviewInputHash,
		PayloadHash:             in.PayloadHash,
		Version:                 1,
		ExpiresAt:               timestamptzParam(in.ExpiresAt),
	})
	if err != nil {
		return PricePreview{}, err
	}
	preview := toPricePreviewEntity(row)
	// row.ID != id ⇒ 回傳的是既有列，不是這次插入的。
	if row.ID != id && preview.PreviewInputHash != in.PreviewInputHash {
		return PricePreview{}, ErrPricePreviewIdempotencyConflict
	}
	return preview, nil
}

// FindPricePreviewByIdempotencyKey 依冪等鍵 (buyer_id, client_request_id) 找出既有的
// price preview。找不到回 ErrPricePreviewNotFound。
func (r *Repo) FindPricePreviewByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (PricePreview, error) {
	row, err := r.q(ctx).FindPricePreviewByIdempotencyKey(ctx, gen.FindPricePreviewByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PricePreview{}, ErrPricePreviewNotFound
		}
		return PricePreview{}, err
	}
	return toPricePreviewEntity(row), nil
}

// FindPricePreviewByIDAndBuyer 依 (id, buyer_id) 找出既有的 price preview——查詢條件
// 同時帶 buyer_id，而不是先用 id 查再事後比對 owner：這樣別的買家猜中一個屬於別人的
// pricePreviewId 時，這裡直接回 ErrPricePreviewNotFound，跟「這個 id 根本不存在」是同一種
// 錯誤形狀，不會洩漏「這個 preview 其實存在，只是不是你的」（Task 9 消費，issue #2403
// 驗收條件明確要求 wrong-owner／random ID 回同一個 404 shape）。
func (r *Repo) FindPricePreviewByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (PricePreview, error) {
	row, err := r.q(ctx).FindPricePreviewByIDAndBuyer(ctx, gen.FindPricePreviewByIDAndBuyerParams{
		ID: id, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PricePreview{}, ErrPricePreviewNotFound
		}
		return PricePreview{}, err
	}
	return toPricePreviewEntity(row), nil
}

// HasPricePreview 回報這個 request 是否已經有至少一筆試算結果——Task 11
// 用它決定 turn 回覆要不要附上「去看看報價試算」的提示（usecase.PricePreviewChecker）。
// 不刻意區分「零筆」與查詢錯誤：呼叫端（turn.go）把 err != nil 當成「這次不確定，
// 這一輪先不顯示提示」處理，不阻塞既有的對話流程——這個提示本身只是錦上添花的
// 導引，不是任何寫入路徑的前置條件，沒有必要為了它讓整輪 turn 失敗。
func (r *Repo) HasPricePreview(ctx context.Context, requestID int64) (bool, error) {
	count, err := r.q(ctx).CountPricePreviewsByRequestID(ctx, requestID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
