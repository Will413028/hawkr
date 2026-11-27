package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


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

	ph := newHarness(t, profile, account)
	got, err := ph.uc.Prepare(context.Background(), paid)
	if err != nil {
		t.Fatalf("Prepare 失敗：%v", err)
	}

	ch := newCheckoutHarness(t, profile, account)
	if domain.IsBusinessTaxID(paid.BuyerTaxID) {
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
