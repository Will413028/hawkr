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

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
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
// Blocks 與 Open 是預覽與執行發布時的店家檢查（A9-D45(4)）：封鎖這位買家的店、不是營業中的店都不能發布；兩者都必給，缺一個發布一律回組裝錯誤（不當成沒有封鎖）。
type Service struct {
	DB     *db.DB
	Now    func() time.Time
	Events *events.Bus
	Blocks candidate.BlockedStores
	Open   candidate.OpenStores
}

const (
	msgCandidateSetStalePreview = "這個候選集已不是有效的候選集，請重新載入"
	msgCandidateSetStaleExecute = "這個候選集已不是有效的候選集，請重新預覽"
)

func audienceUnavailable(ctx context.Context) error {
	return versionConflict(text.RfqAudienceUnavailable(uitext.LocaleFrom(ctx)).Text, TargetCandidateSet)
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

func checkPublishable(req store.ProcurementRequest) error {
	if !search.Searchable(req.Status) {
		return notReady("需求還沒有備妥，不能發布詢價")
	}
	return nil
}

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

func settle(req store.ProcurementRequest, active store.ProcurementRequestRevision, hasActive bool, expected uuid.UUID) (store.ProcurementRequest, store.ProcurementRequestRevision, error) {
	if !hasActive || active.ID != expected {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, versionConflict("需求已被改版，請重新載入", TargetRequestRevision)
	}
	if err := checkPublishable(req); err != nil {
		return store.ProcurementRequest{}, store.ProcurementRequestRevision{}, err
	}
	return req, active, nil
}

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

func (s Service) checkStores(ctx context.Context, buyerID int64, storeIDs []int64) error {
	if s.Blocks == nil || s.Open == nil {
		return errors.New("rfq：Service.Blocks 與 Service.Open 必填：發布要檢查封鎖與營業中")
	}
	blocked, err := s.Blocks.StoreIDsBlockingBuyer(ctx, buyerID, storeIDs)
	if err != nil {
		return fmt.Errorf("rfq：讀封鎖這位買家的店家：%w", err)
	}
	open, err := s.Open.OpenStoreIDs(ctx, storeIDs)
	if err != nil {
		return fmt.Errorf("rfq：讀營業中的店家：%w", err)
	}
	for _, id := range storeIDs {
		if _, isBlocked := blocked[id]; isBlocked {
			return audienceUnavailable(ctx)
		}
		if _, isOpen := open[id]; !isOpen {
			return audienceUnavailable(ctx)
		}
	}
	return nil
}
