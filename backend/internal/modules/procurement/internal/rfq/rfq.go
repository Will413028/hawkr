// Package rfq 是買家詢價發布鏈的 use case（A9-3b-3c、A9-D35）：價格試算（純計算）、發布預覽（建立核准）、執行發布。
// 發布核准是唯一一個同意物件：不可變、會到期（ApprovalTTL）、只能執行一次；它存單價範圍、補充說明、回價期限、受眾與候選集，
// 賣家看得到的內容由核准與它對應的需求版本組出（compile.go）。寫入的 use case 在單一 transaction 內、鎖著需求列做完所有檢查與寫入；
// hash 由不可變的列在 Go 重算（rfqhash，S5），表裡沒有 hash 欄。
// 版本衝突的 conflictTarget：request_revision（讀到的需求版本不是現行版本）、candidate_set（候選集已不是有效的）、publication_approval（核准已不是可執行的那一份）。
package rfq

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

const (
	// ApprovalTTL 是發布核准的有效時間：到 CreatedAt+ApprovalTTL 的那一刻起就過期（expires_at 之前有效，等於 expires_at 已過期）。
	ApprovalTTL = 24 * time.Hour
	// MaxBuyerNoteRunes 是買家補充說明的上限（字元數，proto 與 DB CHECK 同值）。
	MaxBuyerNoteRunes = 500
)

// 衝突目標（ErrorInfo.metadata 的 conflictTarget）。
const (
	TargetRequestRevision     = "request_revision"
	TargetCandidateSet        = "candidate_set"
	TargetPublicationApproval = "publication_approval"
)

// Service 是詢價發布鏈的 use case；Events 是 ExecutePublication 在同 transaction 發布 RfqPublished 的事件匯流排（procurement.New 要求必給）。
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

func notReady(msg string) error {
	return apperr.FailedPrecondition("%s", msg).WithReason(apperr.ReasonRequirementNotReady)
}

// checkPublishable 要求需求在 REQUIREMENT_READY 或 MATCHING（與 search.Start 同一個判斷）。需求狀態改變不產生新版本，所以預覽與執行都要查。
func checkPublishable(req store.ProcurementRequest) error {
	if !search.Searchable(req.Status) {
		return notReady("需求還沒有備妥，不能發布詢價")
	}
	return nil
}

// checkOwner 要求需求與它的對話屬於買家（NOT_FOUND）。
func checkOwner(ctx context.Context, q *store.Queries, buyerID int64, requestID uuid.UUID) (store.ProcurementRequest, error) {
	buyer := buyerID
	req, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: requestID, BuyerID: &buyer})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, apperr.NotFound("找不到這張需求")
	}
	if err != nil {
		return store.ProcurementRequest{}, fmt.Errorf("rfq：讀取需求：%w", err)
	}
	if _, err := q.GetConversationByRequest(ctx, store.GetConversationByRequestParams{RequestID: requestID, BuyerID: buyerID}); errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, apperr.NotFound("找不到這張需求的對話")
	} else if err != nil {
		return store.ProcurementRequest{}, fmt.Errorf("rfq：讀取對話：%w", err)
	}
	return req, nil
}

// settle 在讀到現行版本之後做兩個檢查：讀到的版本仍是現行版本（request_revision），需求狀態可發布。
func settle(req store.ProcurementRequest, active store.ProcurementRequestRevision, hasActive bool, expected uuid.UUID) (store.ProcurementRequest, store.ProcurementRequestRevision, error) {
	if !hasActive || active.ID != expected {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, versionConflict("需求已被改版，請重新載入", TargetRequestRevision)
	}
	if err := checkPublishable(req); err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, err
	}
	return req, active, nil
}

// readRequest 讀現況（不鎖，給純計算用）：需求與對話屬於買家、讀到的版本是現行版本、需求狀態可發布。
func readRequest(ctx context.Context, q *store.Queries, buyerID int64, requestID, expected uuid.UUID) (store.ProcurementRequest, store.ProcurementRequestRevision, error) {
	req, err := checkOwner(ctx, q, buyerID, requestID)
	if err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, err
	}
	buyer := buyerID
	active, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: requestID, BuyerID: &buyer})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：讀取現行版本：%w", err)
	}
	return settle(req, active, err == nil, expected)
}

// lockRequest 鎖住並讀現況（寫入用）：需求與對話屬於買家（NOT_FOUND）、鎖現行版本與需求列（順序同 turn.Direct，與搜尋提交、回答串行）、
// 讀到的版本仍是現行版本、需求狀態可發布。
func lockRequest(ctx context.Context, q *store.Queries, buyerID int64, requestID, expected uuid.UUID) (store.ProcurementRequest, store.ProcurementRequestRevision, error) {
	if _, err := checkOwner(ctx, q, buyerID, requestID); err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, err
	}
	buyer := buyerID
	active, err := q.LockActiveRevision(ctx, store.LockActiveRevisionParams{RequestID: requestID, BuyerID: &buyer})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：鎖現行版本：%w", err)
	}
	hasActive := err == nil
	req, err := q.LockRequestForBuyer(ctx, store.LockRequestForBuyerParams{ID: requestID, BuyerID: &buyer})
	if err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, fmt.Errorf("rfq：鎖需求：%w", err)
	}
	return settle(req, active, hasActive, expected)
}
