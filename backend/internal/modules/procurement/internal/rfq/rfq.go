// Package rfq 是買家詢價發布鏈的 use case（A9-3b-3c）：價格預覽、確認快照、發布預覽（核准）、執行發布。
// 每一支都在單一 transaction 內、鎖著需求列做完所有檢查與寫入；hash 由不可變的列在 Go 重算（rfqhash，S5），表裡沒有 hash 欄。
// 版本衝突的 conflictTarget：request_revision（讀到的需求版本不是現行版本）、price_preview、rfq_snapshot、candidate_set、publication_approval。
package rfq

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

const (
	// PricePreviewTTL 是價格預覽的有效時間；SnapshotTTL 與 ApprovalTTL 是確認快照與發布核准的有效時間（沿用 ai-service）。
	PricePreviewTTL = 15 * time.Minute
	SnapshotTTL     = 24 * time.Hour
	ApprovalTTL     = 24 * time.Hour
	// MaxBuyerNoteRunes 是買家補充說明的上限（字元數，proto 與 DB CHECK 同值）。
	MaxBuyerNoteRunes = 500
	// WishPoolPolicyID 與 WishPoolPolicyVersion 是公開池目前的受眾政策：全站未停權的賣家，沒有分類、地區或分級的限制。
	WishPoolPolicyID      = "WISH_POOL_ALL_ACTIVE_SELLERS_V1"
	WishPoolPolicyVersion = int32(1)
)

// 衝突目標（ErrorInfo.metadata 的 conflictTarget）。
const (
	TargetRequestRevision     = "request_revision"
	TargetPricePreview        = "price_preview"
	TargetRfqSnapshot         = "rfq_snapshot"
	TargetCandidateSet        = "candidate_set"
	TargetPublicationApproval = "publication_approval"
)

// Service 是詢價發布鏈的 use case；Events 只有 ExecutePublication 需要（發布事件在同 transaction 發出），沒給時它回組裝錯誤。
type Service struct {
	DB     *db.DB
	Now    func() time.Time
	Events *events.Bus
}

func versionConflict(msg, target string) error {
	return apperr.VersionConflict("%s", msg).WithMetadata("conflictTarget", target)
}

func idempotencyMismatch() error {
	return apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
}

// lockRequest 鎖住並讀現況：需求與對話屬於買家（NOT_FOUND）、鎖現行版本與需求列（順序同 turn.Direct，與搜尋提交、回答串行）、讀到的版本仍是現行版本。
func lockRequest(ctx context.Context, q *store.Queries, buyerID int64, requestID, expected uuid.UUID) (store.ProcurementRequest, store.ProcurementRequestRevision, error) {
	buyer := buyerID
	if _, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: requestID, BuyerID: &buyer}); errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, apperr.NotFound("找不到這張需求")
	} else if err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：讀取需求：%w", err)
	}
	if _, err := q.GetConversationByRequest(ctx, store.GetConversationByRequestParams{RequestID: requestID, BuyerID: buyerID}); errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, apperr.NotFound("找不到這張需求的對話")
	} else if err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：讀取對話：%w", err)
	}
	active, err := q.LockActiveRevision(ctx, store.LockActiveRevisionParams{RequestID: requestID, BuyerID: &buyer})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：鎖現行版本：%w", err)
	}
	hasActive := err == nil
	req, err := q.LockRequestForBuyer(ctx, store.LockRequestForBuyerParams{ID: requestID, BuyerID: &buyer})
	if err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：鎖需求：%w", err)
	}
	if !hasActive || active.ID != expected {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, versionConflict("需求已被改版，請重新載入", TargetRequestRevision)
	}
	return req, active, nil
}
