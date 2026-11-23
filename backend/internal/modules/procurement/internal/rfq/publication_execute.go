package rfq

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// ExecuteInput 是 ExecuteRfqPublication 的輸入：ApprovalVersion 是預覽回的核准版本，PayloadHash 是預覽回的賣家可見內容的 hash。
type ExecuteInput struct {
	BuyerID                                                    int64
	RequestID, ClientRequestID, ApprovalID, ExpectedRevisionID uuid.UUID
	ApprovalVersion                                            int32
	PayloadHash                                                string
}

// ExecutePublication 以核准的版本建立發布，同一個 transaction 寫入發布、指定對象的邀請與一則 RfqPublished 事件，任何一步失敗全部還原（核准的 PENDING→EXECUTING 也還原）。不建 run、不發通知。
// 需求或核准不屬於買家回 NOT_FOUND；讀到的需求版本不是現行版本回 ABORTED／VERSION_CONFLICT（request_revision）；需求狀態不是 REQUIREMENT_READY 或 MATCHING 回 FAILED_PRECONDITION／REQUIREMENT_NOT_READY；
// 核准是別的需求的、版本不符、不是 PENDING（已執行過）、已過期（expires_at 當下已過期）、凍結的需求版本已不是現行版本、回價期限已早於今天（台北時區）、PayloadHash 與核准的內容不符、指定對象的受眾名單是空的，回 ABORTED／VERSION_CONFLICT（publication_approval）；
// 指定對象的候選集已不是有效的候選集（search.IsEffective）回 ABORTED／VERSION_CONFLICT（candidate_set）。
// 重送同一個 ClientRequestID 回第一次的發布（Replayed 為 true，先於其餘檢查，不重複建邀請與事件），內容（需求、核准、核准版本、讀到的版本、PayloadHash）不同回 INVALID_ARGUMENT（欄位 client_request_id）。
func (s Service) ExecutePublication(ctx context.Context, in ExecuteInput) (model.Publication, error) {
	var out model.Publication
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		var err error
		if out, err = replayPublication(ctx, q, in); err != nil || out.ID != (uuid.UUID{}) {
			return err
		}
		req, rev, err := lockRequest(ctx, q, in.BuyerID, in.RequestID, in.ExpectedRevisionID)
		if err != nil {
			return err
		}
		stale := versionConflict("這份核准已經不是可執行的那一份，請重新確認後再試", TargetPublicationApproval)
		approval, err := q.GetApprovalForBuyer(ctx, store.GetApprovalForBuyerParams{ID: in.ApprovalID, BuyerID: &in.BuyerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這份發布核准")
		}
		if err != nil {
			return fmt.Errorf("rfq：讀取發布核准：%w", err)
		}
		if approval.RequestID != in.RequestID {
			return stale
		}
		now := s.Now()
		claimed, err := q.ClaimApprovalForExecute(ctx, store.ClaimApprovalForExecuteParams{ID: in.ApprovalID, Version: in.ApprovalVersion, BuyerID: &in.BuyerID, Now: now})
		if errors.Is(err, pgx.ErrNoRows) { // 同一個鍵的另一個請求剛執行完時，這裡才讀得到它的發布
			if out, err = replayPublication(ctx, q, in); err != nil || out.ID != (uuid.UUID{}) {
				return err
			}
			return stale
		}
		if err != nil {
			return fmt.Errorf("rfq：領取發布核准：%w", err)
		}
		if claimed.RequestRevisionID != rev.ID || (claimed.ResponseDeadline != nil && deadlinePassed(*claimed.ResponseDeadline, now)) {
			return stale
		}
		if claimed.AudienceMode == model.AudienceDirectRecipients {
			if claimed.CandidateSetID == nil {
				return stale
			}
			effective, err := search.IsEffective(ctx, q, req, in.BuyerID, *claimed.CandidateSetID)
			if err != nil {
				return err
			}
			if !effective {
				return versionConflict("這個候選集已不是有效的候選集，請重新預覽", TargetCandidateSet)
			}
		}
		c, err := compile(ctx, q, in.BuyerID, claimed)
		if err != nil {
			return err
		}
		if c.sellerHash != in.PayloadHash || (claimed.AudienceMode == model.AudienceDirectRecipients && len(c.storeIDs) == 0) {
			return stale
		}
		pub := store.InsertPublicationParams{
			ID: uuid.NewV7(), BuyerID: &in.BuyerID, RequestID: in.RequestID, RequestRevisionID: rev.ID, ApprovalID: claimed.ID, ApprovalVersion: in.ApprovalVersion,
			SchemaVersion: rfqhash.SchemaV1, ClientRequestID: in.ClientRequestID, Now: now,
		}
		n, err := q.InsertPublication(ctx, pub)
		if err != nil {
			return fmt.Errorf("rfq：寫入發布：%w", err)
		}
		if n == 0 { // 核准已領走，所以只可能是這個冪等鍵已用在別的發布：整筆還原
			return idempotencyMismatch()
		}
		invites := make([]model.RfqInvite, 0, len(c.storeIDs))
		for i, storeID := range c.storeIDs {
			inv := store.InsertInviteParams{ID: uuid.NewV7(), PublicationID: pub.ID, Position: int32(i), StoreID: storeID, Now: now} //nolint:gosec // 位置不超過候選數
			if err := q.InsertInvite(ctx, inv); err != nil {
				return fmt.Errorf("rfq：寫入邀請：%w", err)
			}
			invites = append(invites, model.RfqInvite{InviteID: inv.ID, StoreID: storeID})
		}
		if n, err := q.MarkApprovalSucceeded(ctx, store.MarkApprovalSucceededParams{ID: claimed.ID, Version: claimed.Version, Now: now}); err != nil || n != 1 {
			return fmt.Errorf("rfq：收尾發布核准 n=%d：%w", n, err)
		}
		if err := events.Publish(ctx, s.Events, model.RfqPublished{
			PublicationID: pub.ID, RequestID: in.RequestID, ApprovalID: claimed.ID, BuyerID: in.BuyerID, AudienceMode: claimed.AudienceMode, PublishedAt: now, Invites: invites,
		}); err != nil {
			return fmt.Errorf("rfq：發布事件：%w", err)
		}
		row, err := q.GetPublication(ctx, store.GetPublicationParams{ID: pub.ID, BuyerID: &in.BuyerID})
		if err != nil {
			return fmt.Errorf("rfq：讀回發布：%w", err)
		}
		out = publicationOf(row, claimed, c, false)
		return nil
	})
	return out, err
}

// replayPublication 以冪等鍵找第一次的發布：沒有回零值；有但內容（需求、核准、核准版本、讀到的版本、PayloadHash）不同回 INVALID_ARGUMENT。
func replayPublication(ctx context.Context, q *store.Queries, in ExecuteInput) (model.Publication, error) {
	pub, err := q.GetPublicationByClientRequest(ctx, store.GetPublicationByClientRequestParams{BuyerID: &in.BuyerID, ClientRequestID: in.ClientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Publication{}, nil
	}
	if err != nil {
		return model.Publication{}, fmt.Errorf("rfq：讀取發布的冪等鍵：%w", err)
	}
	if pub.RequestID != in.RequestID || pub.ApprovalID != in.ApprovalID || pub.ApprovalVersion != in.ApprovalVersion || pub.RequestRevisionID != in.ExpectedRevisionID {
		return model.Publication{}, idempotencyMismatch()
	}
	approval, err := q.GetApprovalForBuyer(ctx, store.GetApprovalForBuyerParams{ID: pub.ApprovalID, BuyerID: &in.BuyerID})
	if err != nil {
		return model.Publication{}, fmt.Errorf("rfq：讀取發布 %s 的核准：%w", pub.ID, err)
	}
	c, err := compile(ctx, q, in.BuyerID, approval)
	if err != nil {
		return model.Publication{}, err
	}
	if c.sellerHash != in.PayloadHash {
		return model.Publication{}, idempotencyMismatch()
	}
	return publicationOf(pub, approval, c, true), nil
}

func publicationOf(p store.ProcurementRfqPublication, a store.ProcurementRfqPublicationApproval, c compiled, replayed bool) model.Publication {
	return model.Publication{
		ID: p.ID, Version: p.Version, State: p.State, AudienceMode: a.AudienceMode, AudienceHash: c.audienceHash, SellerFacingHash: c.sellerHash,
		PublishedAt: p.PublishedAt, Replayed: replayed,
	}
}
