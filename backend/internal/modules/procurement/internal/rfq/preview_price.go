package rfq

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// PreviewPriceInput 是 PreviewRfqPrice 的輸入：買家預期的單價區間（新台幣元），ClientRequestID 是冪等鍵。
type PreviewPriceInput struct {
	BuyerID                            int64
	RequestID, ExpectedRevisionID      uuid.UUID
	ClientRequestID                    uuid.UUID
	LowerBoundAmount, UpperBoundAmount int64
}

// PreviewPrice 依單價區間與需求現行版本的數量與單位算出總額區間，存成 PricePreviewTTL 有效的預覽。
// 需求不屬於買家或沒有對話回 NOT_FOUND；讀到的版本不是現行版本回 ABORTED／VERSION_CONFLICT（request_revision）；區間不合法回 INVALID_ARGUMENT（欄位 LowerBoundAmount、UpperBoundAmount）；
// 需求沒有單位（算不出計價基準）回 FAILED_PRECONDITION／REQUIREMENT_NOT_READY；有數量與單位卻算不出總額（溢位）回 FAILED_PRECONDITION。
// 重送同一個 ClientRequestID 回第一次的預覽（先於其餘檢查），內容不同回 INVALID_ARGUMENT（欄位 client_request_id）。
func (s Service) PreviewPrice(ctx context.Context, in PreviewPriceInput) (model.PricePreview, error) {
	var out model.PricePreview
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		var err error
		if out, err = replayPreview(ctx, q, in); err != nil || out.ID != (uuid.UUID{}) {
			return err
		}
		_, rev, err := lockRequest(ctx, q, in.BuyerID, in.RequestID, in.ExpectedRevisionID)
		if err != nil {
			return err
		}
		if !ValidRange(in.LowerBoundAmount, in.UpperBoundAmount) {
			fields := []string{"LowerBoundAmount", "UpperBoundAmount"}
			return apperr.InvalidInputOn(fields, "預期單價的下界與上界必須是正整數，且下界不大於上界")
		}
		if rev.Unit == nil {
			return apperr.FailedPrecondition("需求還沒有單位，算不出計價基準").WithReason(apperr.ReasonRequirementNotReady)
		}
		basisQty, basisUnit := Basis(*rev.Unit)
		p := store.InsertPricePreviewParams{
			ID: uuid.NewV7(), BuyerID: &in.BuyerID, RequestID: in.RequestID, RequestRevisionID: rev.ID,
			RangeLowerAmount: in.LowerBoundAmount, RangeUpperAmount: in.UpperBoundAmount, BasisQuantity: basisQty, BasisUnit: basisUnit,
			ClientRequestID: in.ClientRequestID, Now: s.Now(),
		}
		p.ExpiresAt = p.Now.Add(PricePreviewTTL)
		if rev.Quantity != nil {
			lo, up, ok := Total(in.LowerBoundAmount, in.UpperBoundAmount, *rev.Quantity, basisQty)
			if !ok {
				return apperr.FailedPrecondition("數量與單價相乘超出可算的範圍，請縮小單價範圍")
			}
			p.TotalLowerAmount, p.TotalUpperAmount, p.TotalQuantity, p.TotalUnit = &lo, &up, rev.Quantity, &basisUnit
		}
		n, err := q.InsertPricePreview(ctx, p)
		if err != nil {
			return fmt.Errorf("rfq：寫入價格預覽：%w", err)
		}
		if n == 0 { // 同一個鍵的另一個請求搶先寫入並已提交：以它為準
			if out, err = replayPreview(ctx, q, in); err != nil || out.ID != (uuid.UUID{}) {
				return err
			}
			return fmt.Errorf("rfq：價格預覽的冪等鍵衝突卻讀不到列")
		}
		row, err := q.GetPricePreview(ctx, store.GetPricePreviewParams{ID: p.ID, RequestID: in.RequestID, BuyerID: &in.BuyerID})
		if err != nil {
			return fmt.Errorf("rfq：讀回價格預覽：%w", err)
		}
		out, err = pricePreviewOf(row)
		return err
	})
	return out, err
}

// replayPreview 以冪等鍵找第一次的預覽：沒有回零值；有但內容（需求、讀到的版本、區間）不同回 INVALID_ARGUMENT。
func replayPreview(ctx context.Context, q *store.Queries, in PreviewPriceInput) (model.PricePreview, error) {
	row, err := q.GetPricePreviewByClientRequest(ctx, store.GetPricePreviewByClientRequestParams{BuyerID: &in.BuyerID, ClientRequestID: in.ClientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PricePreview{}, nil
	}
	if err != nil {
		return model.PricePreview{}, fmt.Errorf("rfq：讀取價格預覽的冪等鍵：%w", err)
	}
	if row.RequestID != in.RequestID || row.RequestRevisionID != in.ExpectedRevisionID ||
		row.RangeLowerAmount != in.LowerBoundAmount || row.RangeUpperAmount != in.UpperBoundAmount {
		return model.PricePreview{}, idempotencyMismatch()
	}
	return pricePreviewOf(row)
}

func pricePreviewOf(p store.ProcurementPricePreview) (model.PricePreview, error) {
	hash, err := PreviewHash(p)
	if err != nil {
		return model.PricePreview{}, err
	}
	out := model.PricePreview{
		ID: p.ID, RequestID: p.RequestID, RequestRevisionID: p.RequestRevisionID, LowerBoundAmount: p.RangeLowerAmount, UpperBoundAmount: p.RangeUpperAmount,
		BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit, PayloadHash: hash, ExpiresAt: p.ExpiresAt,
	}
	if p.TotalLowerAmount != nil {
		out.Total = &model.QuoteTotal{LowerBoundAmount: *p.TotalLowerAmount, UpperBoundAmount: *p.TotalUpperAmount, Quantity: *p.TotalQuantity, Unit: *p.TotalUnit}
	}
	return out, nil
}
