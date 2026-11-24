package rfq

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// PreviewPublicationInput 是 PreviewRfqPublication 的輸入：買家預期的單價區間與補充說明、受眾與回價期限。SelectedCandidateIDs 的順序有意義；
// ResponseDeadline 是回價期限（當日），nil 表示不設期限。
type PreviewPublicationInput struct {
	BuyerID                                        int64
	RequestID, ClientRequestID, ExpectedRevisionID uuid.UUID
	LowerBoundAmount, UpperBoundAmount             int64
	BuyerNote                                      string
	CandidateSetID                                 *uuid.UUID
	SelectedCandidateIDs                           []uuid.UUID
	ResponseDeadline                               *time.Time
}

// PreviewPublication 依需求現行版本與買家給的單價區間、補充說明編出賣家看得到的內容，建立不可變的發布核准（ApprovalTTL 有效），不發布、不通知。
// 輸入檢查先於任何讀取：單價區間不合法（欄位 LowerBoundAmount、UpperBoundAmount）、補充說明去頭尾空白後超過 MaxBuyerNoteRunes（欄位 BuyerNote）、
// 欄位不合（沒帶候選集或沒選候選、候選重複）、回價期限不是合法日期，都回 INVALID_ARGUMENT 並指出欄位。
// 重送同一個 ClientRequestID 回第一次的核准（先於其餘需要讀取的檢查），內容（需求、讀到的版本、區間、補充說明、期限、候選集、受眾）不同回 INVALID_ARGUMENT（欄位 client_request_id）。
// 其後：回價期限早於今天（台北時區）回 INVALID_ARGUMENT（欄位 ResponseDeadline）；需求、候選集不屬於買家回 NOT_FOUND；讀到的版本不是現行版本回 ABORTED／VERSION_CONFLICT（request_revision）；
// 需求狀態不是 REQUIREMENT_READY 或 MATCHING、或沒有單位回 FAILED_PRECONDITION／REQUIREMENT_NOT_READY；候選集不是有效的候選集（A9-D29(4)）回 ABORTED／VERSION_CONFLICT（candidate_set）；候選不在候選集裡回 INVALID_ARGUMENT。
func (s Service) PreviewPublication(ctx context.Context, in PreviewPublicationInput) (model.PublicationPreview, error) {
	note, err := checkInput(in)
	if err != nil {
		return model.PublicationPreview{}, err
	}
	var out model.PublicationPreview
	err = s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		var err error
		if out, err = replayApproval(ctx, q, in, note); err != nil || out.ApprovalID != (uuid.UUID{}) {
			return err
		}
		now := s.Now()
		if in.ResponseDeadline != nil && deadlinePassed(*in.ResponseDeadline, now) {
			return apperr.InvalidInputOn([]string{"ResponseDeadline"}, "回價期限不得早於今天")
		}
		req, rev, err := lockRequest(ctx, q, in.BuyerID, in.RequestID, in.ExpectedRevisionID)
		if err != nil {
			return err
		}
		if _, err := quote(rev, in.LowerBoundAmount, in.UpperBoundAmount); err != nil {
			return err
		}
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
		stores, err := resolveStores(ctx, q, in)
		if err != nil {
			return err
		}
		a := store.InsertApprovalParams{
			ID: uuid.NewV7(), BuyerID: &in.BuyerID, RequestID: in.RequestID, RequestRevisionID: rev.ID,
			RangeLowerAmount: in.LowerBoundAmount, RangeUpperAmount: in.UpperBoundAmount, BuyerNote: note, CandidateSetID: in.CandidateSetID,
			SchemaVersion: rfqhash.SchemaV1, ClientRequestID: in.ClientRequestID, Now: now, ExpiresAt: now.Add(ApprovalTTL),
		}
		if in.ResponseDeadline != nil {
			d := dateOnly(*in.ResponseDeadline)
			a.ResponseDeadline = &d
		}
		n, err := q.InsertApproval(ctx, a)
		if err != nil {
			return fmt.Errorf("rfq：寫入發布核准：%w", err)
		}
		if n == 0 { // 同一個鍵的另一個請求搶先寫入並已提交：以它為準
			if out, err = replayApproval(ctx, q, in, note); err != nil || out.ApprovalID != (uuid.UUID{}) {
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

// checkInput 檢查不需要讀取的輸入，回去頭尾空白後的補充說明：單價區間、補充說明長度、受眾欄位與模式、回價期限是合法日期。
func checkInput(in PreviewPublicationInput) (string, error) {
	if err := checkRange(in.LowerBoundAmount, in.UpperBoundAmount); err != nil {
		return "", err
	}
	note := trimtext.Trim(in.BuyerNote)
	if utf8.RuneCountInString(note) > MaxBuyerNoteRunes {
		return "", apperr.InvalidInputOn([]string{"BuyerNote"}, "補充說明最多 %d 個字", MaxBuyerNoteRunes)
	}
	return note, checkAudienceShape(in)
}

// deadlinePassed 回傳回價期限（當日）是否早於 now 在營業時區（台北）的今天；期限就是今天不算過。
func deadlinePassed(deadline, now time.Time) bool { return dateOnly(deadline).Before(twtime.Day(now)) }

// checkAudienceShape 檢查受眾欄位，不讀資料庫：要有候選集與至少一個候選，且不重複；回價期限要是合法日期（零值不是）。
func checkAudienceShape(in PreviewPublicationInput) error {
	if in.ResponseDeadline != nil && in.ResponseDeadline.IsZero() {
		return apperr.InvalidInputOn([]string{"ResponseDeadline"}, "回價期限不是合法的日期")
	}
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

// replayApproval 以冪等鍵找第一次的核准：沒有回零值；有但內容（需求、讀到的版本、區間、補充說明、期限、候選集、受眾）不同回 INVALID_ARGUMENT。
func replayApproval(ctx context.Context, q *store.Queries, in PreviewPublicationInput, note string) (model.PublicationPreview, error) {
	a, err := q.GetApprovalByClientRequest(ctx, store.GetApprovalByClientRequestParams{BuyerID: &in.BuyerID, ClientRequestID: in.ClientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PublicationPreview{}, nil
	}
	if err != nil {
		return model.PublicationPreview{}, fmt.Errorf("rfq：讀取發布核准的冪等鍵：%w", err)
	}
	if a.RequestID != in.RequestID || a.RequestRevisionID != in.ExpectedRevisionID || a.RangeLowerAmount != in.LowerBoundAmount || a.RangeUpperAmount != in.UpperBoundAmount ||
		a.BuyerNote != note || !sameDate(a.ResponseDeadline, in.ResponseDeadline) || !sameUUID(a.CandidateSetID, in.CandidateSetID) {
		return model.PublicationPreview{}, idempotencyMismatch()
	}
	c, err := compile(ctx, q, in.BuyerID, a)
	if err != nil {
		return model.PublicationPreview{}, err
	}
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
	return approvalPreviewOf(a, c), nil
}

func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
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
		ExpiresAt: a.ExpiresAt, RecipientCount: int32(len(c.storeIDs)), //nolint:gosec // 受眾數不超過候選數
	}
}
