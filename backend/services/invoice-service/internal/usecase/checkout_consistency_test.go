package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// preview ↔ 實際開票 的等值斷言
//
// 🔴 這是本波最重要的一支測試。它守的不是某一個分支的正確性，而是一個**跨兩條路徑的
// 不變式**：checkout preview 對買家承諾的 invoice_mode，必須就是 order.paid 真的開出來的
// 那一個。
//
// 為什麼需要它：兩條路徑各有自己的入口（ResolveMode／Prepare）、各有自己的測試。
// 任何人在其中一條裡「順手」把 domain.ResolveFulfillmentMode 換成一段 if，
// **兩邊的既有測試都會繼續全綠** —— 症狀要到買家收到一張與結帳頁說法不同的發票
// （或根本沒收到）才會出現，而那時發票已經開出去、不可回收。
//
// 判準刻意只比對 preview 對外承諾的那兩個欄位（fulfillment_mode／not_applicable_reason）：
// document.status 與 manual_reason_code 不在 preview 的契約裡，把它們也拉進來比，
// 只會讓 fail-closed 三格（preview 拿不到那三個輸入）永遠不相等而必須加例外，
// 而例外一旦開始加，這支測試就會被稀釋成「比對我允許它比對的部分」。
// ---------------------------------------------------------------------------

// previewInputFrom 由 order.paid 的輸入推導出 preview 的輸入。
//
// 🔴 **推導而不是各寫一份**：兩邊各手寫一組金額時，改了其中一邊而忘了另一邊，
// 這支測試會因為「輸入不同」而失敗或（更糟）因為輸入剛好還是同一葉而繼續綠。
// 推導讓「同一組輸入」變成結構事實。
func previewInputFrom(in OrderPaidInput) ResolveModeInput {
	return ResolveModeInput{
		StoreID:        in.StoreID,
		BuyerID:        in.BuyerID,
		ProductAmount:  in.Snapshot.TotalOriginalAmount,
		ShippingAmount: in.Snapshot.ShippingFee,
		DiscountAmount: in.Snapshot.DiscountAmount,
		CheckoutID:     "CK-" + in.OrderNo,
	}
}

// assertPreviewMatchesPrepare 用同一組賣家事實與同一張訂單跑兩條路徑，斷言逐值相同。
func assertPreviewMatchesPrepare(
	t *testing.T,
	profile domain.SellerTaxProfile,
	account domain.ProviderAccount,
	mutate func(*OrderPaidInput),
) {
	t.Helper()

	paid := baseInput()
	if mutate != nil {
		mutate(&paid)
	}

	// ① order.paid 那條路徑。
	ph := newHarness(t, profile, account)
	got, err := ph.uc.Prepare(context.Background(), paid)
	if err != nil {
		t.Fatalf("Prepare 失敗：%v", err)
	}

	// ② checkout preview 那條路徑。
	ch := newCheckoutHarness(t, profile, account)
	if domain.IsBusinessTaxID(paid.BuyerTaxID) {
		// preview 沒有買家輸入可讀，B2B 的唯一來源是預設常用發票資料。
		ch.prefs.def, ch.prefs.defFound = taxIDPreference(), true
	}
	preview, err := ch.uc.ResolveMode(context.Background(), previewInputFrom(paid))
	if err != nil {
		t.Fatalf("ResolveMode 失敗：%v", err)
	}

	if preview.InvoiceMode != got.FulfillmentMode {
		t.Errorf("invoice_mode 不一致：preview=%q order.paid=%q\n"+
			"（買家在結帳頁看到的與實際開出來的是兩件事 —— 這正是共用 ResolveFulfillmentMode 要防的）",
			preview.InvoiceMode, got.FulfillmentMode)
	}
	if preview.NotApplicableReason != got.NotApplicableReason {
		t.Errorf("not_applicable_reason 不一致：preview=%q order.paid=%q",
			preview.NotApplicableReason, got.NotApplicableReason)
	}
	if preview.DocumentKind != got.DocumentKind {
		t.Errorf("document_kind 不一致：preview=%q order.paid=%q",
			preview.DocumentKind, got.DocumentKind)
	}
}

// TestResolveMode_MatchesPrepare_AllLeaves 走遍 §7.1.3 的每一葉。
//
// 表格尾端的窮舉斷言不是裝飾：刪掉一個 case 只會讓迴圈少跑一次、不會轉紅，
// 而「刪掉一個 case」正是這張表最容易被改壞的方式。
func TestResolveMode_MatchesPrepare_AllLeaves(t *testing.T) {
	exempt := approvedProfile(int16(domain.TaxCategoryExempt))

	cases := []struct {
		leaf    string
		profile domain.SellerTaxProfile
		account domain.ProviderAccount
		mutate  func(*OrderPaidInput)
	}{
		{
			leaf:    "NA1",
			profile: approvedProfile(1), account: activeAccount(),
			mutate: func(in *OrderPaidInput) {
				in.Snapshot.TotalActualAmount = 0
				in.Snapshot.TotalOriginalAmount = 0
				in.Snapshot.Items = nil
			},
		},
		{
			leaf: "R1",
			profile: domain.SellerTaxProfile{
				ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
				DefaultTaxCategory: 1,
			},
			account: activeAccount(),
		},
		{
			leaf: "S1",
			profile: domain.SellerTaxProfile{
				ReviewStatus:       codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
				DefaultTaxCategory: 1,
			},
			account: activeAccount(),
		},
		{
			leaf: "S2",
			profile: domain.SellerTaxProfile{
				ReviewStatus:       codemeta.SellerTaxReviewStatusUnderReview.Value,
				DefaultTaxCategory: 1,
			},
			account: activeAccount(),
		},
		{
			leaf:    "S3",
			profile: approvedProfile(1),
			account: domain.ProviderAccount{
				ID:                  555,
				Status:              codemeta.InvoiceProviderAccountStatusSuspended.Value,
				AutomationCutoverAt: &testCutover,
			},
		},
		{
			leaf:    "S4",
			profile: approvedProfile(1),
			account: domain.ProviderAccount{ID: 555, Status: codemeta.InvoiceProviderAccountStatusActive.Value},
		},
		{
			leaf:    "AM-a",
			profile: approvedProfile(1), account: activeAccount(),
			mutate: func(in *OrderPaidInput) { in.HasPlatformFundedDiscount = true },
		},
		{
			leaf:    "AM-b",
			profile: approvedProfile(1), account: activeAccount(),
			mutate: func(in *OrderPaidInput) { in.IsConsolidatedOrder = true },
		},
		{
			leaf:    "AM-c",
			profile: exempt, account: activeAccount(),
			mutate: func(in *OrderPaidInput) {
				in.Snapshot.ShippingFee = 60
				in.Snapshot.TotalActualAmount = 160
			},
		},
		{
			leaf:    "B2C",
			profile: approvedProfile(1), account: activeAccount(),
		},
		{
			leaf:    "B2B",
			profile: approvedProfile(1), account: activeAccount(),
			mutate: func(in *OrderPaidInput) {
				in.BuyerTaxID = "12345678"
				in.BuyerTitle = "測試買方股份有限公司"
			},
		},
	}

	seen := map[string]bool{}
	for _, c := range cases {
		seen[c.leaf] = true
		t.Run(c.leaf, func(t *testing.T) {
			assertPreviewMatchesPrepare(t, c.profile, c.account, c.mutate)
		})
	}

	for _, leaf := range []string{"NA1", "R1", "S1", "S2", "S3", "S4", "AM-a", "AM-b", "AM-c", "B2C", "B2B"} {
		if !seen[leaf] {
			t.Errorf("§7.1.3 的葉節點 %s 沒有被這張表覆蓋", leaf)
		}
	}
}

// ---------------------------------------------------------------------------
// 快照讀取小工具（給 checkout_test.go 用）
// ---------------------------------------------------------------------------

func snapshotMap(t *testing.T, out ValidateInvoiceOutput) map[string]any {
	t.Helper()
	if out.SnapshotJSON == nil {
		t.Fatal("放行時必須回快照：order-service 要拿它寫 order_invoice_snapshots")
	}
	var m map[string]any
	if err := json.Unmarshal(out.SnapshotJSON, &m); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}
	return m
}

func snapshotKeys(t *testing.T, out ValidateInvoiceOutput) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	for k := range snapshotMap(t, out) {
		keys[k] = true
	}
	return keys
}

// snapshotField 回傳欄位的字串化值。null 回空字串 —— 呼叫端要區分「沒有這個欄位」與
// 「這個欄位是 null」時請用 snapshotKeys。
func snapshotField(t *testing.T, out ValidateInvoiceOutput, key string) string {
	t.Helper()
	v, ok := snapshotMap(t, out)[key]
	if !ok {
		t.Fatalf("快照沒有欄位 %q", key)
	}
	if v == nil {
		return ""
	}
	if f, isFloat := v.(float64); isFloat {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%v", v)
}
