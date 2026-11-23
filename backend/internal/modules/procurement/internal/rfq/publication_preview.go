package rfq

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// PreviewPublicationInput 是 PreviewRfqPublication 的輸入。CandidateSetID 與 SelectedCandidateIDs 只有 DIRECT_RECIPIENTS 用（順序有意義）；
// ResponseDeadline 是回價期限（當日），nil 表示不設期限。
type PreviewPublicationInput struct {
	BuyerID                                                    int64
	RequestID, ClientRequestID, ExpectedRevisionID, SnapshotID uuid.UUID
	AudienceMode                                               string
	CandidateSetID                                             *uuid.UUID
	SelectedCandidateIDs                                       []uuid.UUID
	ResponseDeadline                                           *time.Time
}

// PreviewPublication 依確認快照編出賣家看得到的內容，建立不可變的發布核准（ApprovalTTL 有效），不發布、不通知。
// 欄位與模式不合（AudienceMode 不是兩個模式之一、公開池帶了候選、指定對象沒帶候選集或沒選候選、候選重複或不在候選集裡）回 INVALID_ARGUMENT 並指出欄位；
// 需求、確認快照、候選集不屬於買家回 NOT_FOUND；讀到的版本不是現行版本回 ABORTED／VERSION_CONFLICT（request_revision）；
// 快照屬於別的需求或確認的不是現行版本回 ABORTED／VERSION_CONFLICT（rfq_snapshot）；候選集不是有效的候選集（A9-D29(4)）回 ABORTED／VERSION_CONFLICT（candidate_set）。
// 公開池的收件人在發布時決定，預覽的受眾是空的（recipient_count 0）。
// 重送同一個 ClientRequestID 回第一次的核准（先於版本與快照檢查），內容不同回 INVALID_ARGUMENT（欄位 client_request_id）。
func (s Service) PreviewPublication(ctx context.Context, in PreviewPublicationInput) (model.PublicationPreview, error) {
	if err := checkAudienceShape(in); err != nil {
		return model.PublicationPreview{}, err
	}
	var out model.PublicationPreview
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		var err error
		if out, err = replayApproval(ctx, q, in); err != nil || out.ApprovalID != (uuid.UUID{}) {
			return err
		}
		req, rev, err := lockRequest(ctx, q, in.BuyerID, in.RequestID, in.ExpectedRevisionID)
		if err != nil {
			return err
		}
		snap, err := q.GetConfirmedSnapshotForBuyer(ctx, store.GetConfirmedSnapshotForBuyerParams{ID: in.SnapshotID, BuyerID: &in.BuyerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這份確認快照")
		}
		if err != nil {
			return fmt.Errorf("rfq：讀取確認快照：%w", err)
		}
		if snap.RequestID != in.RequestID || snap.RequestRevisionID != rev.ID {
			return versionConflict("這份確認快照已不對應目前的需求，請重新確認", TargetRfqSnapshot)
		}
		var stores []int64
		if in.AudienceMode == model.AudienceDirectRecipients {
			set, err := q.GetCandidateSetForBuyer(ctx, store.GetCandidateSetForBuyerParams{ID: *in.CandidateSetID, RequestID: in.RequestID, BuyerID: in.BuyerID})
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("找不到這個候選集")
			}
			if err != nil {
				return fmt.Errorf("rfq：讀取候選集：%w", err)
			}
			effective, err := search.IsEffective(ctx, q, req, in.BuyerID, set.ID)
			if err != nil {
				return err
			}
			if !effective {
				return versionConflict("這個候選集已不是有效的候選集，請重新載入", TargetCandidateSet)
			}
			if stores, err = resolveStores(ctx, q, in); err != nil {
				return err
			}
		}
		now := s.Now()
		a := store.InsertApprovalParams{
			ID: uuid.NewV7(), BuyerID: &in.BuyerID, RequestID: in.RequestID, RequestRevisionID: rev.ID, ConfirmedRfqSnapshotID: snap.ID,
			AudienceMode: in.AudienceMode, SchemaVersion: rfqhash.SchemaV1, ClientRequestID: in.ClientRequestID, Now: now, ExpiresAt: now.Add(ApprovalTTL),
		}
		if in.ResponseDeadline != nil {
			d := dateOnly(*in.ResponseDeadline)
			a.ResponseDeadline = &d
		}
		if in.AudienceMode == model.AudienceWishPool {
			id, v := WishPoolPolicyID, WishPoolPolicyVersion
			a.AudiencePolicyID, a.AudiencePolicyVersion = &id, &v
		}
		n, err := q.InsertApproval(ctx, a)
		if err != nil {
			return fmt.Errorf("rfq：寫入發布核准：%w", err)
		}
		if n == 0 { // 同一個鍵的另一個請求搶先寫入並已提交：以它為準
			if out, err = replayApproval(ctx, q, in); err != nil || out.ApprovalID != (uuid.UUID{}) {
				return err
			}
			return fmt.Errorf("rfq：發布核准的冪等鍵衝突卻讀不到列")
		}
		for i, id := range stores {
			if err := q.InsertApprovalRecipient(ctx, store.InsertApprovalRecipientParams{ApprovalID: a.ID, Position: int32(i), StoreID: id}); err != nil { //nolint:gosec // 位置不超過候選數
				return fmt.Errorf("rfq：寫入核准的受眾：%w", err)
			}
		}
		row, err := q.GetApproval(ctx, store.GetApprovalParams{ID: a.ID, RequestID: in.RequestID, BuyerID: &in.BuyerID})
		if err != nil {
			return fmt.Errorf("rfq：讀回發布核准：%w", err)
		}
		out, err = previewOfApproval(ctx, q, in.BuyerID, row)
		return err
	})
	return out, err
}

// checkAudienceShape 檢查受眾欄位與模式相合，不讀資料庫：公開池不帶候選；指定對象要有候選集與至少一個候選，且不重複；回價期限要是合法日期（零值不是）。
func checkAudienceShape(in PreviewPublicationInput) error {
	if in.ResponseDeadline != nil && in.ResponseDeadline.IsZero() {
		return apperr.InvalidInputOn([]string{"ResponseDeadline"}, "回價期限不是合法的日期")
	}
	switch in.AudienceMode {
	case model.AudienceWishPool:
		if in.CandidateSetID != nil {
			return apperr.InvalidInputOn([]string{"CandidateSetID"}, "公開池不指定候選集")
		}
		if len(in.SelectedCandidateIDs) > 0 {
			return apperr.InvalidInputOn([]string{"SelectedCandidateIDs"}, "公開池不指定候選")
		}
	case model.AudienceDirectRecipients:
		if in.CandidateSetID == nil {
			return apperr.InvalidInputOn([]string{"CandidateSetID"}, "指定對象要帶候選集")
		}
		if len(in.SelectedCandidateIDs) == 0 {
			return apperr.InvalidInputOn([]string{"SelectedCandidateIDs"}, "指定對象至少要選一個候選")
		}
		seen := make(map[uuid.UUID]bool, len(in.SelectedCandidateIDs))
		for i, id := range in.SelectedCandidateIDs {
			if seen[id] {
				return apperr.InvalidInputOn([]string{apperr.IndexedField("SelectedCandidateIDs", i)}, "候選重複")
			}
			seen[id] = true
		}
	default:
		return apperr.InvalidInputOn([]string{"AudienceMode"}, "不支援的發布對象種類")
	}
	return nil
}

// resolveStores 依 SelectedCandidateIDs 的順序（契約的一部分）把候選對到店家；候選不在候選集裡回 INVALID_ARGUMENT（指出是第幾個）。
func resolveStores(ctx context.Context, q *store.Queries, in PreviewPublicationInput) ([]int64, error) {
	candidates, err := q.ListCandidates(ctx, store.ListCandidatesParams{CandidateSetID: *in.CandidateSetID, BuyerID: in.BuyerID})
	if err != nil {
		return nil, fmt.Errorf("rfq：讀取候選：%w", err)
	}
	byID := make(map[uuid.UUID]int64, len(candidates))
	for _, c := range candidates {
		byID[c.ID] = c.StoreID
	}
	out := make([]int64, 0, len(in.SelectedCandidateIDs))
	for i, id := range in.SelectedCandidateIDs {
		storeID, ok := byID[id]
		if !ok {
			return nil, apperr.InvalidInputOn([]string{apperr.IndexedField("SelectedCandidateIDs", i)}, "這個候選不在候選集裡")
		}
		out = append(out, storeID)
	}
	return out, nil
}

// replayApproval 以冪等鍵找第一次的核准：沒有回零值；有但內容（需求、快照、版本、模式、期限、受眾）不同回 INVALID_ARGUMENT。
func replayApproval(ctx context.Context, q *store.Queries, in PreviewPublicationInput) (model.PublicationPreview, error) {
	a, err := q.GetApprovalByClientRequest(ctx, store.GetApprovalByClientRequestParams{BuyerID: &in.BuyerID, ClientRequestID: in.ClientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PublicationPreview{}, nil
	}
	if err != nil {
		return model.PublicationPreview{}, fmt.Errorf("rfq：讀取發布核准的冪等鍵：%w", err)
	}
	if a.RequestID != in.RequestID || a.ConfirmedRfqSnapshotID != in.SnapshotID || a.RequestRevisionID != in.ExpectedRevisionID ||
		a.AudienceMode != in.AudienceMode || !sameDate(a.ResponseDeadline, in.ResponseDeadline) {
		return model.PublicationPreview{}, idempotencyMismatch()
	}
	c, err := compile(ctx, q, in.BuyerID, a)
	if err != nil {
		return model.PublicationPreview{}, err
	}
	if in.AudienceMode == model.AudienceDirectRecipients {
		stores, err := resolveStores(ctx, q, in)
		var bad *apperr.Error
		if errors.As(err, &bad) {
			return model.PublicationPreview{}, idempotencyMismatch()
		}
		if err != nil {
			return model.PublicationPreview{}, err
		}
		if !slices.Equal(stores, c.storeIDs) {
			return model.PublicationPreview{}, idempotencyMismatch()
		}
	}
	return approvalPreviewOf(a, c), nil
}

func sameDate(stored, in *time.Time) bool {
	if stored == nil || in == nil {
		return stored == nil && in == nil
	}
	return dateOnly(*stored).Equal(dateOnly(*in))
}

func previewOfApproval(ctx context.Context, q *store.Queries, buyerID int64, a store.ProcurementRfqPublicationApproval) (model.PublicationPreview, error) {
	c, err := compile(ctx, q, buyerID, a)
	if err != nil {
		return model.PublicationPreview{}, err
	}
	return approvalPreviewOf(a, c), nil
}

func approvalPreviewOf(a store.ProcurementRfqPublicationApproval, c compiled) model.PublicationPreview {
	return model.PublicationPreview{
		ApprovalID: a.ID, ApprovalVersion: a.Version, AudienceHash: c.audienceHash, SellerFacingHash: c.sellerHash, Payload: c.payload,
		ExpiresAt: a.ExpiresAt, PolicyID: c.policyID, PolicyVersion: c.policyVersion, RecipientCount: int32(len(c.storeIDs)), //nolint:gosec // 受眾數不超過候選數
	}
}
