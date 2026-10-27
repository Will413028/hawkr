package usecase

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// #3021：端點 17 的 canRequestExtension 必須與端點 20 的守衛逐字一致。
//
// 🔴 陰性那一半才是本票的內容：只驗「SELF_ISSUE_PENDING ＋ 0 次 → true」的話，
// 修法寫成「一律 true」也會通過，而缺陷正是「非 SELF_ISSUE_PENDING 時仍為 true」。
func TestCanRequestTaxExtension_MatchesEndpoint20Guard(t *testing.T) {
	pending := codemeta.SellerTaxReviewStatusSelfIssuePending.Value

	cases := []struct {
		name   string
		status string
		count  int32
		want   bool
	}{
		{"SELF_ISSUE_PENDING ＋ 尚未延長 → 可申請", pending, 0, true},
		{"SELF_ISSUE_PENDING ＋ 已延長一次 → 不可（T11 只給一次）", pending, 1, false},

		// 🔴 這一格就是 #3021：送審中平台正在審，補件期限不是賣家自己能動的。
		// 舊實作在這裡回 true ⇒ 畫面渲染 CTA ⇒ 按下去必回 9321。
		{"SUBMITTED → 不可（#3021）", codemeta.SellerTaxReviewStatusSubmitted.Value, 0, false},
		{"DRAFT → 不可", codemeta.SellerTaxReviewStatusDraft.Value, 0, false},
		{"APPROVED_INVOICE_REQUIRED → 不可", codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 0, false},
		{"REJECTED → 不可", codemeta.SellerTaxReviewStatusRejected.Value, 0, false},
		{"未送過審（空狀態）→ 不可", "", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanRequestTaxExtension(domain.SellerTaxProfile{
				ReviewStatus:   tc.status,
				ExtensionCount: int16(tc.count),
			})
			if got != tc.want {
				t.Fatalf("canRequestTaxExtension = %v, want %v（旗標與端點 20 的守衛必須一致，"+
					"否則賣家看得到按鈕、按下去回 9321）", got, tc.want)
			}
		})
	}
}

// #3089 第 13 項：NOT_APPLICABLE 的文件不得回 CARRIER_NOT_WON。
//
// 「平台不會為這張訂單開票」＝ 那張憑證從頭到尾不存在，談不上「載具中獎後才提供」。
// 而載具是買家結帳時就選好的，與平台後來決定不開票無關 ⇒ 判斷順序反了，
// 每一張帶載具的 NOT_APPLICABLE 都會對買家說「之後會有」。
// #3026 修 RECEIPT 那一格、#3084 是 B2B 載具那一格，這是同族第三格。
func TestDocumentFileUnavailableReason_NotApplicableBeatsCarrier(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	carrier := codemeta.InvoiceCarrierTypeMobileBarcode.Value
	issued := now.Add(-24 * time.Hour)

	cases := []struct {
		name string
		doc  domain.Document
		want string
	}{
		{
			name: "NOT_APPLICABLE ＋ 有載具 ⇒ NOT_APPLICABLE",
			doc: domain.Document{
				Status:      codemeta.InvoiceDocumentStatusNotApplicable.Value,
				CarrierType: &carrier,
			},
			want: FileUnavailableNotApplicable,
		},
		{
			// 🔴 陽性對照：真的開出來、真的有載具的票仍要回 CARRIER_NOT_WON。
			// 只驗前一格分不清「順序修好了」與「載具那一格被關掉」。
			name: "ISSUED ＋ 有載具 ⇒ CARRIER_NOT_WON",
			doc: domain.Document{
				Status:      codemeta.InvoiceDocumentStatusIssued.Value,
				CarrierType: &carrier,
			},
			want: FileUnavailableCarrierNotWon,
		},
		{
			name: "ISSUED ＋ 無載具 ＋ 超過保存期 ⇒ EXPIRED",
			doc: domain.Document{
				Status:           codemeta.InvoiceDocumentStatusIssued.Value,
				ProviderIssuedAt: &issued,
			},
			want: FileUnavailableExpired,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := c.doc
			if c.want == FileUnavailableExpired {
				old := now.Add(-providerFileRetention - time.Hour)
				d.ProviderIssuedAt = &old
			}
			if got := documentFileUnavailableReason(d, now); got != c.want {
				t.Fatalf("reason = %q，want %q", got, c.want)
			}
		})
	}
}

// #3089 第 10 項：intakeLimit 的述詞是「現在適不適用」，不是「欄位有沒有設定」。
//
// 舊述詞看兩個 limit 欄是否皆為 0，而它們有 DB 預設值（50／100000）
// ⇒ dev 上 6/6 列都非 0 ⇒ 永不回 nil ⇒ ADM 每一頁都渲染
// 「待審期間接單限制 0/50 筆」，而 58A 逐字要求非待審期間整塊不渲染。
func TestIntakeLimitApplies_PredicateIsReviewStatusNotFieldPresence(t *testing.T) {
	for _, st := range codemeta.AllSellerTaxReviewStatuss {
		p := domain.SellerTaxProfile{
			ReviewStatus: st.Value,
			// 刻意帶 DB 預設值：舊述詞在這組值下對每一個狀態都回 true。
			PendingOrderLimitCount: 50, PendingOrderLimitAmount: 100000,
		}
		want := st.Value == codemeta.SellerTaxReviewStatusSelfIssuePending.Value ||
			st.Value == codemeta.SellerTaxReviewStatusSupplementRequired.Value
		if got := IntakeLimitApplies(p); got != want {
			t.Errorf("IntakeLimitApplies(%s) = %v，want %v", st.Value, got, want)
		}
	}
	// 讀端與結帳端必須是同一份判準——intakeLimitOf 走的就是它。
	pending := domain.SellerTaxProfile{
		ReviewStatus:           codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
		PendingOrderLimitCount: 50, PendingOrderLimitAmount: 100000,
	}
	if !intakeLimitOf(pending).Applicable {
		t.Fatal("結帳端與讀端的判準分家了")
	}
}
