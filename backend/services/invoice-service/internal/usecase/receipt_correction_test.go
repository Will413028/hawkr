package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 🔴 收據不得進入更正鏈路（#3026 收尾稽查，dev 實測 O481452453 的收據
// canRequestCorrection 回 true）。
//
// 整條更正鏈路是**發票語意**：buildReissueDraft 寫死 B2C／B2B、執行面是 AMEGO
// 作廢重開，而收據沒有 provider_invoice_no、不打 AMEGO ⇒ 會在 provider 那一段
// 才炸，而買家早就按下送出了。
//
// 為什麼那一格原本會回 true：`tax_period_code` 被無條件填給了收據（實測收據帶著
// 11505，而收據根本沒有稅期別）⇒ WithinVoidWindow 照樣成立。
func TestCanRequestCorrection_FalseForReceipt(t *testing.T) {
	period := "11505" // 收據不該有稅期別，但實際資料就是這樣 —— 守衛不能依賴它為空
	receipt := domain.Document{
		DocumentKind:  codemeta.InvoiceDocumentKindReceipt.Value,
		Status:        codemeta.InvoiceDocumentStatusIssued.Value,
		TaxPeriodCode: &period,
	}
	q := (&Query{}).WithClock(func() time.Time {
		return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	})
	got, err := q.canRequestCorrection(context.Background(), receipt, false)
	if err != nil {
		t.Fatalf("canRequestCorrection: %v", err)
	}
	if got {
		t.Error("收據開了更正入口 —— 那顆按鈕按下去會在 AMEGO 那一段失敗")
	}
}

// 🔴 陰性對照：**發票線不得被一起關掉**。這是這次修改最容易造成的附帶損害
// ——把守衛寫成「非 B2C 就擋」會讓 B2B 也失去更正入口。
//
// 用「除了 document_kind 之外完全相同」的兩份文件對照：收據 false、發票 true。
// 只驗收據那一格的話，把 `return false` 寫成無條件也會綠。
func TestCanRequestCorrection_ReceiptGuardDoesNotAffectInvoices(t *testing.T) {
	// 2026-05 期（11505）的申報截止在 2026-07-15 之前 ⇒ 拿一個**還在窗口內**的
	// 固定時點注入 clock，不用真實時間（時間敏感邏輯一律注入 clock）。
	period := "11505"
	q := (&Query{}).WithClock(func() time.Time {
		return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	})
	base := domain.Document{
		Status:        codemeta.InvoiceDocumentStatusIssued.Value,
		TaxPeriodCode: &period,
	}
	for _, tc := range []struct {
		kind string
		want bool
	}{
		{codemeta.InvoiceDocumentKindReceipt.Value, false},
		{codemeta.InvoiceDocumentKindB2c.Value, true},
		{codemeta.InvoiceDocumentKindB2b.Value, true},
	} {
		doc := base
		doc.DocumentKind = tc.kind
		got, err := q.canRequestCorrection(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("kind=%s: %v", tc.kind, err)
		}
		if got != tc.want {
			t.Errorf("kind=%s canRequestCorrection = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

// 🔴 真正的入口守衛在寫入面：藏按鈕擋不住直接打端點的人（§17.6 收斂點）。
func TestCorrectionIntake_RejectsReceipt(t *testing.T) {
	u := &CorrectionIntake{deps: CorrectionIntakeDeps{
		Documents: &fakeReceiptDocs{doc: domain.Document{
			ID:           900001,
			DocumentKind: codemeta.InvoiceDocumentKindReceipt.Value,
			Status:       codemeta.InvoiceDocumentStatusIssued.Value,
		}},
	}}
	_, err := u.Submit(context.Background(), SubmitCorrectionRequest{
		OrderNo: "O481452453", BuyerID: 2623900520,
	})
	if err == nil {
		t.Fatal("收據送出更正申請竟然沒有被擋下")
	}
	if !strings.Contains(err.Error(), "收據") {
		t.Errorf("錯誤訊息沒有說明理由：%v", err)
	}
	if !isBizCode(err, int32(bizcode.InvoiceCorrectionNotAllowed)) {
		t.Errorf("業務碼不是 InvoiceCorrectionNotAllowed：%v", err)
	}
}

// fakeReceiptDocs 只實作這條路徑真正會用到的那一支。
type fakeReceiptDocs struct {
	domain.DocumentRepository
	doc domain.Document
}

func (f *fakeReceiptDocs) ListOrderDocumentsForBuyer(
	context.Context, string, int64,
) ([]domain.Document, error) {
	return []domain.Document{f.doc}, nil
}

// #3089 第 9 項：讀端的 canRequestCorrection 漏了 AlreadyReported。
//
// 兩個時間窗**不重合**，而讀端原本只看前者：
//
//	WithinVoidWindow  到「跨期後單月 5 日 23:59:59」為 true（AMEGO 關帳硬牆）
//	ReportedStatus    從**期末**起就不再是 NOT_REPORTED（先 UNKNOWN、
//	                  過了申報期限才 REPORTED），而寫端 AlreadyReported
//	                  ＝ `reported != NOT_REPORTED` ⇒ 直接拒絕
//
// 交集之外那一段：**每兩個月的單月 1–5 日整整五天，上一期的每一張發票
// 都是「按鈕亮著、按下去必得 ALREADY_REPORTED」**。它不是邊角案例——
// 那五天涵蓋上一期的全部發票。
//
// 期別 11505 ＝ 民國 115 年第 5 期（2026 年 9–10 月）：
//
//	期末       2026-10-31 23:59:59（台北）
//	關帳硬牆   2026-11-05 23:59:59（台北）
//	申報期限   2026-11-15 23:59:59（台北）
func TestCanRequestCorrection_ClosedBeforeFilingDueIsNotOffered(t *testing.T) {
	period := "11505"
	doc := domain.Document{
		DocumentKind:  codemeta.InvoiceDocumentKindB2c.Value,
		Status:        codemeta.InvoiceDocumentStatusIssued.Value,
		TaxPeriodCode: &period,
	}
	// 全部用台北中午對應的 UTC 時點，避開日界誤差。
	taipeiNoon := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 4, 0, 0, 0, time.UTC) // 台北 12:00
	}
	for _, tc := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{"期中（未申報）⇒ 開入口", taipeiNoon(2026, 10, 15), true},
		{"期末當天仍在期內 ⇒ 開入口", taipeiNoon(2026, 10, 31), true},
		// 🔴 這三天就是缺口：WithinVoidWindow 仍為 true，而寫端必拒。
		{"期末後第 1 天（UNKNOWN）⇒ 不開", taipeiNoon(2026, 11, 1), false},
		{"期末後第 3 天（UNKNOWN）⇒ 不開", taipeiNoon(2026, 11, 3), false},
		{"關帳牆當天（UNKNOWN）⇒ 不開", taipeiNoon(2026, 11, 5), false},
		// 關帳牆之後 WithinVoidWindow 自己就會擋，兩道各自成立。
		{"關帳牆之後 ⇒ 不開", taipeiNoon(2026, 11, 6), false},
		{"申報期限之後 ⇒ 不開", taipeiNoon(2026, 11, 20), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := (&Query{}).WithClock(func() time.Time { return tc.now })
			got, err := q.canRequestCorrection(context.Background(), doc, false)
			if err != nil {
				t.Fatalf("canRequestCorrection: %v", err)
			}
			if got != tc.want {
				t.Fatalf("canRequestCorrection = %v, want %v —— "+
					"讀端與寫端的窗口不一致時，買家看到的是一顆按下去必定失敗的按鈕", got, tc.want)
			}
		})
	}
}
