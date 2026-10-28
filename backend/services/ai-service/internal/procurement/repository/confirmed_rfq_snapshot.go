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

// ErrConfirmedRfqSnapshotNotFound 是 FindConfirmedRfqSnapshotByIdempotencyKey 找不到列時
// 的哨兵錯誤——跟 ErrPricePreviewNotFound 同一種用途：呼叫端只用它判斷「這個
// client_request_id 是不是第一次見到確認請求」，不拿它回應使用者已存在資源的存取控制
// 錯誤（見 Task 9 的消費方式）。
var ErrConfirmedRfqSnapshotNotFound = errors.New("procurement: confirmed RFQ snapshot not found")

// ErrConfirmedRfqSnapshotIdempotencyConflict（Task 18／Fix 7）是
// InsertConfirmedRfqSnapshot 撞到 uq_procurement_confirmed_rfq_snapshots_idempotency
// 之後，發現撞鍵後查到的既有列跟這次呼叫的輸入其實不是同一次確認請求的哨兵錯誤——
// 見 InsertConfirmedRfqSnapshot 的 doc comment。
// commerce.ConfirmRfqSnapshotUseCase.ConfirmRfqSnapshot 把它映射成
// domain.ErrIdempotencyKeyConflict，跟應用層那道
// FindConfirmedRfqSnapshotByIdempotencyKey 短路檢查（Task 17）共用同一種對外語意，
// 只是這個是在真正併發撞鍵時的最終仲裁路徑。
var ErrConfirmedRfqSnapshotIdempotencyConflict = errors.New("procurement: idempotency key already used for a different confirmed RFQ snapshot request")

// ConfirmedRfqSnapshot 是買家對某次 price preview 的確認快照（Slice 3a）。
// PricePreviewID／PricePreviewVersion 記錄確認的是哪一版試算；CanonicalPayloadJSON 是
// 確認當下的完整定案內容快照，見 schema procurement.sql 的表註解。
type ConfirmedRfqSnapshot struct {
	ID                   int64
	BuyerID              identity.BuyerID
	RequestID            int64
	RequestRevisionID    int64
	PricePreviewID       int64
	PricePreviewVersion  int64
	CanonicalPayloadJSON json.RawMessage
	PayloadHash          string
	ClientRequestID      string
	ExpiresAt            time.Time
	CreatedAt            time.Time
}

// InsertConfirmedRfqSnapshotInput 是一次確認寫入的完整輸入。
type InsertConfirmedRfqSnapshotInput struct {
	BuyerID              identity.BuyerID
	RequestID            int64
	RequestRevisionID    int64
	PricePreviewID       int64
	PricePreviewVersion  int64
	CanonicalPayloadJSON json.RawMessage
	PayloadHash          string
	ClientRequestID      string
	ExpiresAt            time.Time
}

func toConfirmedRfqSnapshotEntity(row gen.ProcurementConfirmedRfqSnapshot) ConfirmedRfqSnapshot {
	return ConfirmedRfqSnapshot{
		ID: row.ID, BuyerID: identity.BuyerID(row.BuyerID), RequestID: row.RequestID,
		RequestRevisionID:    row.RequestRevisionID,
		PricePreviewID:       row.PricePreviewID,
		PricePreviewVersion:  row.PricePreviewVersion,
		CanonicalPayloadJSON: row.CanonicalPayloadJson,
		PayloadHash:          row.PayloadHash,
		ClientRequestID:      row.ClientRequestID,
		ExpiresAt:            row.ExpiresAt.Time,
		CreatedAt:            row.CreatedAt.Time,
	}
}

// InsertConfirmedRfqSnapshot 寫入一筆確認快照。冪等：同一買家同一 client_request_id
// 重送時回傳前次寫入的那一列，不建立第二筆。
//
// 撞冪等鍵不拋錯（query 用 ON CONFLICT DO UPDATE，見 PostgreSQL 規範第十二章），因此
// 這支方法放進呼叫端的交易裡也不會把它打成 aborted。
//
// ⛔ 回傳既有列前必須比對 RequestID／PricePreviewID／PricePreviewVersion 三欄：兩個
// 「同一個 clientRequestID、不同確認請求」的呼叫真的併發時，撞鍵不代表是同一次確認的
// 重送。usecase 層那次短路檢查是優化，不是唯一防線。
func (r *Repo) InsertConfirmedRfqSnapshot(ctx context.Context, in InsertConfirmedRfqSnapshotInput) (ConfirmedRfqSnapshot, error) {
	id := r.newID()
	row, err := r.q(ctx).InsertConfirmedRfqSnapshot(ctx, gen.InsertConfirmedRfqSnapshotParams{
		ID: id, BuyerID: int64(in.BuyerID), RequestID: in.RequestID,
		RequestRevisionID:    in.RequestRevisionID,
		PricePreviewID:       in.PricePreviewID,
		PricePreviewVersion:  in.PricePreviewVersion,
		CanonicalPayloadJson: in.CanonicalPayloadJSON,
		PayloadHash:          in.PayloadHash,
		ClientRequestID:      in.ClientRequestID,
		ExpiresAt:            timestamptzParam(in.ExpiresAt),
	})
	if err != nil {
		return ConfirmedRfqSnapshot{}, err
	}
	snapshot := toConfirmedRfqSnapshotEntity(row)
	// row.ID != id ⇒ 回傳的是既有列，不是這次插入的。
	if row.ID != id && (snapshot.RequestID != in.RequestID ||
		snapshot.PricePreviewID != in.PricePreviewID ||
		snapshot.PricePreviewVersion != in.PricePreviewVersion) {
		return ConfirmedRfqSnapshot{}, ErrConfirmedRfqSnapshotIdempotencyConflict
	}
	return snapshot, nil
}

// FindConfirmedRfqSnapshotByIdempotencyKey 依冪等鍵 (buyer_id, client_request_id) 找出既有
// 的確認快照。找不到回 ErrConfirmedRfqSnapshotNotFound。
func (r *Repo) FindConfirmedRfqSnapshotByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (ConfirmedRfqSnapshot, error) {
	row, err := r.q(ctx).FindConfirmedRfqSnapshotByIdempotencyKey(ctx, gen.FindConfirmedRfqSnapshotByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConfirmedRfqSnapshot{}, ErrConfirmedRfqSnapshotNotFound
		}
		return ConfirmedRfqSnapshot{}, err
	}
	return toConfirmedRfqSnapshotEntity(row), nil
}

// FindConfirmedRfqSnapshotByIDAndBuyer 依 (id, buyerID) 找出一筆確認快照，形狀與理由
// 同 FindPricePreviewByIDAndBuyer：查詢**必須**同時收斂到買家，否則呼叫端只要猜到 id
// 就能讀到別人的確認快照。查不到與不屬於這個買家回同一個 ErrConfirmedRfqSnapshotNotFound
// ——wrong-owner 與 not-found 對外是同一種 404，不洩漏「這個 id 存在，只是不是你的」。
//
// ⛔ 刻意不收斂到 request_id：那是呼叫端的業務判斷（publication preview 會另外比對
// snapshot.RequestID 是否等於這次 preview 的 request，並回一個可復原的 stale 錯誤），
// 混進查詢條件會讓「別人的」與「同一個買家但別的 request 的」變成同一種 404，
// 而後者的正確復原方式是重新確認、不是死路。
func (r *Repo) FindConfirmedRfqSnapshotByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (ConfirmedRfqSnapshot, error) {
	row, err := r.q(ctx).FindConfirmedRfqSnapshotByIDAndBuyer(ctx, gen.FindConfirmedRfqSnapshotByIDAndBuyerParams{
		ID: id, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConfirmedRfqSnapshot{}, ErrConfirmedRfqSnapshotNotFound
		}
		return ConfirmedRfqSnapshot{}, err
	}
	return toConfirmedRfqSnapshotEntity(row), nil
}

// FindLatestConfirmedRfqSnapshotByRequestID 找出這個 request 目前最新一筆確認快照
// （Task 3 的 RfqConfirmationReader 用它判斷 RequestRevisionID 是否還對得上目前
// active revision——同一個 request 理論上可能因為 rewind／改動後重新確認而有多筆，
// 只取最新那筆才是「現在算不算過期」的正確依據）。找不到回
// ErrConfirmedRfqSnapshotNotFound，跟 FindConfirmedRfqSnapshotByIdempotencyKey
// 同一個哨兵錯誤——呼叫端（domain.PlanDialogue 的間接消費者）只需要分辨
// 「有沒有確認過」，不需要另一種 not-found 語意。
func (r *Repo) FindLatestConfirmedRfqSnapshotByRequestID(ctx context.Context, requestID int64, buyerID identity.BuyerID) (ConfirmedRfqSnapshot, error) {
	row, err := r.q(ctx).FindLatestConfirmedRfqSnapshotByRequestID(ctx, gen.FindLatestConfirmedRfqSnapshotByRequestIDParams{
		RequestID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConfirmedRfqSnapshot{}, ErrConfirmedRfqSnapshotNotFound
		}
		return ConfirmedRfqSnapshot{}, err
	}
	return toConfirmedRfqSnapshotEntity(row), nil
}
