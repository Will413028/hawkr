package rfq

import (
	"context"
	"math"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// PriceInput 是 PreviewRfqPrice 的輸入：買家預期的單價區間（新台幣元）。
type PriceInput struct {
	BuyerID                            int64
	RequestID, ExpectedRevisionID      uuid.UUID
	LowerBoundAmount, UpperBoundAmount int64
}

// PreviewPrice 依單價區間與需求現行版本的數量與單位算出總額區間；純計算，不寫入任何資料（A9-D35）。
// 區間不合法回 INVALID_ARGUMENT（欄位 LowerBoundAmount、UpperBoundAmount，先於任何讀取）；需求不屬於買家或沒有對話回 NOT_FOUND；
// 讀到的版本不是現行版本回 ABORTED／VERSION_CONFLICT（request_revision）；需求狀態不是 REQUIREMENT_READY 或 MATCHING、或沒有單位（算不出計價基準）回 FAILED_PRECONDITION／REQUIREMENT_NOT_READY；
// 有數量與單位卻算不出總額（溢位）回 FAILED_PRECONDITION。
func (s Service) PreviewPrice(ctx context.Context, in PriceInput) (model.PriceQuote, error) {
	if err := checkRange(in.LowerBoundAmount, in.UpperBoundAmount); err != nil {
		return model.PriceQuote{}, err
	}
	var out model.PriceQuote
	err := s.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		_, rev, err := readRequest(ctx, store.New(s.DB.Conn(ctx)), in.BuyerID, in.RequestID, in.ExpectedRevisionID)
		if err != nil {
			return err
		}
		out, err = quote(rev, in.LowerBoundAmount, in.UpperBoundAmount)
		return err
	})
	return out, err
}

func quote(rev store.ProcurementRequestRevision, lower, upper int64) (model.PriceQuote, error) {
	if rev.Unit == nil {
		return model.PriceQuote{}, notReady("需求還沒有單位，算不出計價基準")
	}
	basisQty, basisUnit := Basis(*rev.Unit)
	out := model.PriceQuote{LowerBoundAmount: lower, UpperBoundAmount: upper, BasisQuantity: basisQty, BasisUnit: basisUnit}
	if rev.Quantity != nil {
		lo, up, ok := Total(lower, upper, *rev.Quantity, basisQty)
		if !ok {
			return model.PriceQuote{}, apperr.FailedPrecondition("數量與單價相乘超出可算的範圍，請縮小單價範圍")
		}
		out.Total = &model.QuoteTotal{LowerBoundAmount: lo, UpperBoundAmount: up, Quantity: *rev.Quantity, Unit: basisUnit}
	}
	return out, nil
}

func checkRange(lower, upper int64) error {
	if !ValidRange(lower, upper) {
		return apperr.InvalidInputOn([]string{"LowerBoundAmount", "UpperBoundAmount"}, "預期單價的下界與上界必須是正整數，且下界不大於上界")
	}
	return nil
}

// Basis 回傳單價的計價基準：GRAM 以每 1000 個計價，其餘每 1 個。
func Basis(unit string) (quantity int32, basisUnit string) {
	if unit == "GRAM" {
		return 1000, unit
	}
	return 1, unit
}

// ValidRange 回傳單價區間是否合法：下界與上界是正整數、下界不大於上界。
func ValidRange(lower, upper int64) bool { return lower > 0 && upper > 0 && lower <= upper }

// Total 回傳單價區間乘以數量、除以計價基準的總額區間（四捨五入到元）；溢位時 ok 為 false。
func Total(lower, upper int64, quantity, basis int32) (totalLower, totalUpper int64, ok bool) {
	totalLower, ok = scale(lower, int64(quantity), int64(basis))
	if !ok {
		return 0, 0, false
	}
	totalUpper, ok = scale(upper, int64(quantity), int64(basis))
	return totalLower, totalUpper, ok
}

func scale(unitAmount, quantity, basis int64) (int64, bool) {
	if unitAmount > math.MaxInt64/quantity {
		return 0, false
	}
	product := unitAmount * quantity
	if product > math.MaxInt64-basis/2 {
		return 0, false
	}
	return (product + basis/2) / basis, true
}
