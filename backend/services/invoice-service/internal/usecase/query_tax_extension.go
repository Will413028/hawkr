package usecase

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 待裁決的展延申請（端點 20 → 41）
//
// 🔴 端點 20（TaxProfile.RequestExtension）**只寫一列 invoice_status_history，
// 不碰 seller_tax_profiles 的任何一欄**、不發事件、不寫 review_note。在本檔出現之前，
// 「賣家申請了展延」這件事沒有任何讀取面出口：
//
//	① A02 佇列不分——filter=UNDER_REVIEW 的述詞是
//	   `review_status IN ('UNDER_REVIEW','SELF_ISSUE_PENDING')`，設計書逐字說
//	   「都是待 ADM 決策，A02 的手勢不需要分開」⇒ 申請過與沒申請過的賣家混在同一頁
//	② A03 詳情沒有欄位——`SellerTaxProfileData` 只有 extension_count（准了幾次）
//	   與 can_request_extension（賣家還能不能按），沒有一欄說「有人正在等」
//	③ 沒有事件、沒有通知、沒有告警
//
// 於是 ADM 永遠不知道有一筆展延在等他，而 A03 的「核准展延」按鈕一直都在
// ——按不按純看他想不想。賣家那側同樣看不出差別：按下去 extensionCount 與
// deadlineAt 都不變 ⇒ 他無從判斷送出去了沒，只能一直按。
//
// dev 實測（2026-09-05，store 2605743425）：一筆 2026-09-03 送出的申請掛了兩天，
// extension_count 仍是 0，六個 A02 filter 沒有一個標示得出來。
//
// ⚠️ **不新增欄位也不新增狀態**：申請是一個事件不是一個狀態，把它反正規化到
// seller_tax_profiles 會多一個要與 history 對齊的第二真相。這裡讀既有的
// append-only 歷程，是它唯一的真實來源。
// ---------------------------------------------------------------------------

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
	// audit 未 wire 與「讀失敗」是同一種處置：讀取面的 mapper 不得因為部分 wiring
	// 就 panic（它服務五個端點，其中三個是賣家 S02 的主畫面）。⚠️ 但要與成功路徑
	// **分辨得出來**，所以照樣走下面那則 Warn ——靜默回 nil 的話，忘了注入
	// 稽核 repository 的那一天，症狀會是「展延申請全站消失」而沒有任何訊號。
	if u.audit == nil {
		u.logger().Warn("invoice.query.tax_extension_history_unavailable",
			zap.Int64("profile_id", p.ID),
			zap.String("degrade", "audit repository 未注入，待裁決的展延申請無法呈現"))
		return nil
	}
	rows, err := u.audit.ListStatusHistory(ctx, taxSubjectType, p.ID)
	if err != nil {
		// 降級成「沒有待裁決的申請」而不是中止整支查詢：稅務資料本身完全健康，
		// 稽核表一次抖動不該讓 S02／A03 整頁打不開——退回本修復之前的畫面，不會更差。
		//
		// ⚠️ 這條降級**會重新打開本檔要修的那個缺口**（ADM 看不見申請），
		// 所以一定要留痕：沒有這行 Warn，「申請又不見了」在下一次發生時無從診斷。
		u.logger().Warn("invoice.query.tax_extension_history_unavailable",
			zap.Int64("profile_id", p.ID),
			zap.String("degrade", "待裁決的展延申請無法呈現，A03 會退回看不見申請的狀態"),
			zap.Error(err))
		return nil
	}
	// ListStatusHistory 是 occurred_at ASC ⇒ 由後往前找第一筆即最新。
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
