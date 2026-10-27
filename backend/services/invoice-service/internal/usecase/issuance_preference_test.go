package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestPrepare_PreferenceOwnershipMismatchGoesToAwaitingManual 是 I-47 在 usecase 這一側
// 的落點：ORD adapter 判出「這筆常用發票資料不屬於本訂單的買家」之後，決策樹必須把它
// 翻成 AWAITING_MANUAL ＋ PREFERENCE_OWNERSHIP_MISMATCH。
//
// 🔴 反面（＝這支測試真正在守的東西）：**不得** fallback 成「無載具但照常開票」。
// 那會把一次 IDOR 嘗試靜默吞掉——發票照開、當事人不知道有人拿他的載具去對別人的訂單試。
func TestPrepare_PreferenceOwnershipMismatchGoesToAwaitingManual(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	in := baseInput()
	in.PreferenceOwnershipMismatch = true

	got, err := h.uc.Prepare(context.Background(), in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got.DocumentStatus != codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		t.Fatalf("status = %q, want AWAITING_MANUAL（fallback 成無載具照常開票 ＝ 把 IDOR 吞掉）",
			got.DocumentStatus)
	}
	if got.ManualReasonCode != codemeta.InvoiceManualReasonPreferenceOwnershipMismatch.Value {
		t.Fatalf("manual_reason_code = %q, want %q（用 BUYER_INFO_CONFLICT 之類會讓 ADM 看到指錯方向的理由）",
			got.ManualReasonCode, codemeta.InvoiceManualReasonPreferenceOwnershipMismatch.Value)
	}
	// 陰性：沒有送出去、沒有建 operation。
	if n := len(h.provider.requests); n != 0 {
		t.Fatalf("provider 收到 %d 次請求，歸屬驗證失敗的訂單不得送出", n)
	}
	if got.OperationID != 0 {
		t.Fatalf("AWAITING_MANUAL 不應建立 operation，卻建了 %d", got.OperationID)
	}
	// 理由碼必須與狀態同一交易寫入（不是先建 PENDING 再靠掃描器改過去）。
	if len(h.docs.awaitingManual) != 1 ||
		h.docs.awaitingManual[0] != codemeta.InvoiceManualReasonPreferenceOwnershipMismatch.Value {
		t.Fatalf("manual_reason_code 未與狀態同一交易寫入：%v", h.docs.awaitingManual)
	}
}

// TestPrepare_PreferenceOwnershipMismatchDoesNotHijackOtherLeaves：這一格只作用在
// 真的會送出去的那條路徑。賣家還沒串接（LEGACY_MANUAL）時本來就不開票，拿這個理由碼去
// 蓋它會讓 ADM 工作台上出現一個與事實無關的原因。
func TestPrepare_PreferenceOwnershipMismatchDoesNotHijackOtherLeaves(t *testing.T) {
	draft := domain.SellerTaxProfile{ReviewStatus: codemeta.SellerTaxReviewStatusDraft.Value}
	h := newHarness(t, draft, activeAccount())
	in := baseInput()
	in.PreferenceOwnershipMismatch = true

	got, err := h.uc.Prepare(context.Background(), in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got.DocumentStatus == codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		t.Fatalf("LEGACY_MANUAL 的葉節點不該被歸屬錯誤改寫成 AWAITING_MANUAL")
	}
	if got.ManualReasonCode != "" {
		t.Fatalf("manual_reason_code = %q，不開票的葉節點不該帶它", got.ManualReasonCode)
	}
}

// TestPrepare_NoMismatchKeepsHappyPath 是鑑別力那一半：沒有歸屬錯誤時，同樣的輸入
// 必須照常走到 PENDING ＋ READY。少了它，上面那支「轉紅」可能只是因為所有輸入都轉紅。
func TestPrepare_NoMismatchKeepsHappyPath(t *testing.T) {
	h := newHarness(t, approvedProfile(1), activeAccount())
	in := baseInput()
	in.PreferenceOwnershipMismatch = false

	got, err := h.uc.Prepare(context.Background(), in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got.DocumentStatus != codemeta.InvoiceDocumentStatusPending.Value {
		t.Fatalf("status = %q, want PENDING", got.DocumentStatus)
	}
	if got.ManualReasonCode != "" {
		t.Fatalf("manual_reason_code = %q, want 空", got.ManualReasonCode)
	}
}
