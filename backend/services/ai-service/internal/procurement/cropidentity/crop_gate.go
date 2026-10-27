package cropidentity

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

type GateResult struct {
	Item *string

	Resolution *domain.ItemResolution
	Pending    bool
	Candidates []domain.CropDisambiguationOption
	Selection  *domain.CropSelection

	PendingReason domain.CropPendingReason

	FactsAsOf time.Time

	ResolveFailed bool

	evaluated bool
}

func (g GateResult) Evaluated() bool {
	return g.evaluated || g.Resolution != nil
}

type Resolver interface {
	Resolve(ctx context.Context, itemText string) (DisambiguationResult, error)
}

// GateOption 調整這一輪的判定條件。用 variadic 是為了不動既有呼叫端與測試的簽章。
type GateOption func(*gateOptions)

type gateOptions struct{ awaitingBuyerAnswer bool }

// AwaitingBuyerAnswer 告訴 gate「買家正在回答上一輪的消歧提問」。
//
// ⛔ 此時不得重查：重查會產生一份新的候選清單並蓋掉買家眼前那一份，等於把他正在
// 回答的問題換掉。待選內容由既有的 interrupt 持有，這一輪不碰。
func AwaitingBuyerAnswer() GateOption {
	return func(o *gateOptions) { o.awaitingBuyerAnswer = true }
}

func ComputeGate(ctx context.Context, resolver Resolver, before, after domain.Slots, opts ...GateOption) GateResult {
	var o gateOptions
	for _, opt := range opts {
		opt(&o)
	}
	if !needsResolution(before, after, o) {

		return GateResult{Item: after.Item}
	}
	if resolver == nil {
		return GateResult{Item: after.Item}
	}

	now := time.Now().UTC()
	res, err := resolver.Resolve(ctx, *after.Item)
	if err != nil {

		pkglogger.Error(ctx, "procurement: crop resolution failed, clearing any previous resolution for this request",
			zap.Error(err))
		return GateResult{Item: after.Item, ResolveFailed: true, evaluated: true, FactsAsOf: now}
	}

	switch {
	case res.Unavailable:

		return GateResult{
			Item: after.Item, Pending: true, Candidates: []domain.CropDisambiguationOption{}, Selection: res.Selection,
			PendingReason: domain.CropPendingReasonUnavailable, evaluated: true, FactsAsOf: now,
		}
	case res.NeedsDisambiguation:
		return GateResult{
			Item: after.Item, Pending: true, Candidates: res.Candidates, Selection: res.Selection,
			PendingReason: domain.CropPendingReasonAmbiguous, evaluated: true, FactsAsOf: now,
		}
	default:
		return GateResult{
			Item: after.Item,
			Resolution: domain.NewSystemItemResolution(
				res.CropCode, res.GroupCode, res.CommodityCode, res.FallbackGroupCode, res.Label,
				res.CategoryOrigin),
			evaluated: true, FactsAsOf: now,
		}
	}
}

func (g GateResult) ApplyTo(slots *domain.Slots, patch *domain.SlotPatch) {
	if g.Resolution == nil {
		return
	}
	slots.ItemResolution = g.Resolution
	if patch != nil {
		patch.ItemResolution = g.Resolution
	}
}

// needsResolution 判斷這一輪要不要真的去查 Pricebook。
//
// 「品項文字有沒有變」不足以當判準：resolver 失敗時會把既有解析一併清掉（見上面
// 那行 log），於是同一個品項在下一輪仍然沒有解析結果，而文字沒變就永遠不會再查
// ——這張需求會卡在「沒有品項分類」直到買家自己去改品項。
//
// 三條分支：
//   - 品項變了 → 查（原本就有的行為）。
//   - 品項沒變、但還沒有解析結果 → 上一輪不是失敗就是還沒查成功，重試。
//   - 品項沒變、已經有解析結果 → 不查（避免每一輪重打 Pricebook）。
//
// 例外是買家正在回答消歧提問：那一輪「沒有解析結果」是正常狀態，重查會換掉他
// 眼前的候選（見 AwaitingBuyerAnswer）。
func needsResolution(before, after domain.Slots, o gateOptions) bool {
	if after.Item == nil {
		return false
	}
	if itemChanged(before, after) {
		return true
	}
	if o.awaitingBuyerAnswer {
		return false
	}
	return after.ItemResolution == nil
}

func itemChanged(before, after domain.Slots) bool {
	if after.Item == nil {
		return false
	}
	return before.Item == nil || *before.Item != *after.Item
}
