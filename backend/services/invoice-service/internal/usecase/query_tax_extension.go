package usecase

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TaxExtensionRequest 是一筆尚未被裁決的展延申請。
type TaxExtensionRequest struct {
	RequestedAt time.Time
	Reason      string
}

// PendingTaxExtensionRequest 回這位賣家尚未被裁決的展延申請；沒有就回 nil。
//
// 判準是 `extension_count == 0`：EXTEND 一生只准一次（`chk_stp_extension`），
// count 一旦為 1 就代表最新的那次申請已經被准了 ⇒ 不需要拿 history 的時間戳
// 去跟「上一次核准」比，那個比較在只准一次的前提下永遠得到同一個答案。
//
// ⚠️ 若哪天 EXTEND 改成可以准多次，本函式會開始把**已核准**的申請報成待裁決。
// 守它的是 TestPendingTaxExtensionRequest_OnlyWhenNotYetGranted 對 count=1 的斷言，
// 以及 chk_stp_extension 這條 CHECK 本身——放寬那條約束時必須回來改這裡。
func (u *Query) PendingTaxExtensionRequest(ctx context.Context, p domain.SellerTaxProfile) *TaxExtensionRequest {
	if p.ID == 0 || p.ExtensionCount != 0 {
		return nil
	}
	if u.audit == nil {
		u.logger().Warn("invoice.query.tax_extension_history_unavailable",
			zap.Int64("profile_id", p.ID),
			zap.String("degrade", "audit repository 未注入，待裁決的展延申請無法呈現"))
		return nil
	}
	rows, err := u.audit.ListStatusHistory(ctx, taxSubjectType, p.ID)
	if err != nil {
		u.logger().Warn("invoice.query.tax_extension_history_unavailable",
			zap.Int64("profile_id", p.ID),
			zap.String("degrade", "待裁決的展延申請無法呈現，A03 會退回看不見申請的狀態"),
			zap.Error(err))
		return nil
	}
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.ReasonCode == nil || *r.ReasonCode != taxReasonExtensionRequested {
			continue
		}
		out := &TaxExtensionRequest{RequestedAt: r.OccurredAt}
		if r.ReasonDetail != nil {
			out.Reason = *r.ReasonDetail
		}
		return out
	}
	return nil
}
