package domain

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 本檔守三條保證（W5-2／W5-3／W5-5）。每一條都配一個「反面」案例——
// 只驗正面的話，把規則整條刪掉測試照樣全綠。

const fxSellerTaxID = "12345678" // 8 碼，IsBusinessTaxID 認得

// ============================================================================
// W5-2 開票基礎是 Seller Logistics Charge，不是 Buyer Shipping Charge
// ============================================================================

// 🔴 本軸最刺眼的一組值：賣家做免運 ⇒ 買家運費 0，而平台照樣付了外送費。
//
// 變異：把 BuildLogisticsFeeInvoice 的基礎換成 b.BuyerShippingCharge ⇒ 本測試必須轉紅
// （charge 變 0 ⇒ 回 ErrLogisticsChargeNotPositive ⇒ 第一個 Fatalf 就攔下）。
func TestLogisticsInvoiceUsesSellerChargeNotBuyerShipping_FreeShippingOrder(t *testing.T) {
	const storeID = int64(2685914682)
	basis := LogisticsFeeBasis{
		EstimatedCost:       120, // 平台實付外送費
		BuyerShippingCharge: 0,   // 🔴 買家免運，一毛都沒收
	}

	draft, err := BuildLogisticsFeeInvoice(storeID, fxSellerTaxID, basis)
	if err != nil {
		t.Fatalf("買家免運不該讓平台開不出物流費發票: %v", err)
	}
	if draft.SellerLogisticsCharge != 120 {
		t.Errorf("開票基礎 = %d, want 120（＝實際物流費，非買家運費 0）", draft.SellerLogisticsCharge)
	}
	if draft.Header.TotalAmount != 120 {
		t.Errorf("表頭總額 = %d, want 120", draft.Header.TotalAmount)
	}
	if len(draft.Lines) != 1 || draft.Lines[0].NetAmount != 120 {
		t.Fatalf("明細 = %+v, want 單行 120", draft.Lines)
	}
	if draft.BuyerShippingChargeForReconciliation != 0 {
		t.Errorf("對帳欄 = %d, want 0（原樣留存）", draft.BuyerShippingChargeForReconciliation)
	}
}

// BuyerShippingCharge 改成任何值都不得改變任何一個金額欄位。
//
// 這條比上一條更強：上一條只證明「0 的時候沒被拿去用」，本條證明
// 「它根本不參與計算」——把基礎換成 BuyerShippingCharge 之後，
// 三個案例會算出 0／500／99999 三個不同的總額，全部轉紅。
func TestBuyerShippingChargeDoesNotInfluenceAnyAmount(t *testing.T) {
	const storeID = int64(2685914682)
	base := LogisticsFeeBasis{EstimatedCost: 180, PlatformSubsidy: 30}
	// 180 − 30 = 150
	const wantCharge = int64(150)

	for _, buyerShipping := range []int64{0, 500, 99999} {
		b := base
		b.BuyerShippingCharge = buyerShipping
		draft, err := BuildLogisticsFeeInvoice(storeID, fxSellerTaxID, b)
		if err != nil {
			t.Fatalf("buyerShipping=%d: %v", buyerShipping, err)
		}
		if draft.SellerLogisticsCharge != wantCharge {
			t.Errorf("buyerShipping=%d ⇒ 開票基礎 %d, want %d",
				buyerShipping, draft.SellerLogisticsCharge, wantCharge)
		}
		if draft.Header.TotalAmount != wantCharge {
			t.Errorf("buyerShipping=%d ⇒ 總額 %d, want %d",
				buyerShipping, draft.Header.TotalAmount, wantCharge)
		}
	}
}

// 方向：Platform → Seller，三個識別欄一個都不能錯。
func TestLogisticsInvoiceDirectionIsPlatformToSeller(t *testing.T) {
	const storeID = int64(2632049831)
	draft, err := BuildLogisticsFeeInvoice(storeID, fxSellerTaxID,
		LogisticsFeeBasis{EstimatedCost: 100})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if draft.IssuerType != codemeta.InvoiceIssuerTypePlatform.Value {
		t.Errorf("issuerType = %q, want PLATFORM", draft.IssuerType)
	}
	if draft.IssuerRefID != 0 {
		t.Errorf("issuerRefID = %d, want 0（chk_invoice_issuers_ref）", draft.IssuerRefID)
	}
	if draft.RecipientType != codemeta.InvoiceRecipientTypeSeller.Value {
		t.Errorf("recipientType = %q, want SELLER", draft.RecipientType)
	}
	if draft.RecipientRefID != storeID {
		t.Errorf("recipientRefID = %d, want store_id %d", draft.RecipientRefID, storeID)
	}
	if draft.DocumentKind != codemeta.InvoiceDocumentKindB2b.Value {
		t.Errorf("documentKind = %q, want B2B（賣家要拿得到進項憑證）", draft.DocumentKind)
	}
}

// B2B ⇒ 表頭必須拆稅。缺統編而靜默走成二聯式是本軸最安靜的錯法。
func TestLogisticsInvoiceHeaderSplitsTaxForB2B(t *testing.T) {
	draft, err := BuildLogisticsFeeInvoice(2685914682, fxSellerTaxID,
		LogisticsFeeBasis{EstimatedCost: 105})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got, want := draft.Header.TaxAmount, TaxOf(105); got != want {
		t.Errorf("taxAmount = %d, want %d（B2B 必須拆稅）", got, want)
	}
	if got, want := draft.Header.SalesAmount, int64(105)-TaxOf(105); got != want {
		t.Errorf("salesAmount = %d, want %d", got, want)
	}
	if draft.Header.TaxAmount == 0 {
		t.Error("B2B 表頭 taxAmount 為 0 —— 賣家拿不到可扣抵的進項憑證")
	}
}

// 反面：統編缺／格式錯一律擋，不得退化成二聯式。
func TestLogisticsInvoiceRejectsMissingRecipientTaxID(t *testing.T) {
	for _, taxID := range []string{"", "0000000000", "1234567", "1234567A"} {
		_, err := BuildLogisticsFeeInvoice(2685914682, taxID,
			LogisticsFeeBasis{EstimatedCost: 100})
		if !errors.Is(err, ErrLogisticsRecipientTaxIDRequired) {
			t.Errorf("taxID=%q ⇒ err=%v, want ErrLogisticsRecipientTaxIDRequired", taxID, err)
		}
	}
}

// 反面：認列為 0 或為負一律不開票。
func TestSellerLogisticsChargeGuards(t *testing.T) {
	cases := []struct {
		name  string
		basis LogisticsFeeBasis
		want  error
	}{
		{"成本為負", LogisticsFeeBasis{EstimatedCost: -1}, ErrLogisticsCostNegative},
		{"補貼為負", LogisticsFeeBasis{EstimatedCost: 10, PlatformSubsidy: -1}, ErrLogisticsSubsidyNegative},
		{"補貼超過成本", LogisticsFeeBasis{EstimatedCost: 10, PlatformSubsidy: 20}, ErrLogisticsChargeNegative},
		{"責任調整把認列壓成負", LogisticsFeeBasis{EstimatedCost: 10, ResponsibilityAdjustment: -20}, ErrLogisticsChargeNegative},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := SellerLogisticsCharge(c.basis); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}

	// 全額補貼 ⇒ 認列 0 ⇒ 不是一張 0 元發票，是不該存在的發票。
	if _, err := BuildLogisticsFeeInvoice(2685914682, fxSellerTaxID,
		LogisticsFeeBasis{EstimatedCost: 100, PlatformSubsidy: 100}); !errors.Is(err, ErrLogisticsChargeNotPositive) {
		t.Errorf("認列 0 ⇒ err = %v, want ErrLogisticsChargeNotPositive", err)
	}
}

// 🔴 跨語言契約：本軸的認列算式必須與 payment-service 的 LogisticsAmounts.accrualAmount
// 逐字一致。兩份實作是因為跨語言無法共用，不是因為它們可以各走各的。
//
// fixture 不是手寫的：期望值直接從 Java 原始碼讀出來（§0.3）。Java 檔搬家或改算式
// 都會 fail-loud，而不是讓兩邊靜默漂移。
func TestAccrualFormulaMatchesPaymentServiceSource(t *testing.T) {
	root := logisticsRepoRoot(t)
	path := filepath.Join(root,
		"services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/logistics/domain/LogisticsAmounts.java")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀不到 payment-service 的 LogisticsAmounts.java（%s）——"+
			"檔案搬家了就要同步本測試與 SellerLogisticsCharge 的註解: %v", path, err)
	}
	src := string(raw)

	// 對照組：先確認這份檔案真的是我們以為的那一份（否則下面的「找不到」會是假陰性）。
	if !strings.Contains(src, "public static long accrualAmount(") {
		t.Fatalf("%s 裡找不到 accrualAmount 宣告 —— 讀到的不是預期的檔案", path)
	}

	body := regexp.MustCompile(`(?s)public static long accrualAmount\(.*?\n    \}`).FindString(src)
	if body == "" {
		t.Fatalf("抽不出 accrualAmount 的函式體 —— 抽取器失效（不是「算式不見了」）")
	}
	const wantExpr = "Math.addExact(Math.subtractExact(estimatedCost, platformSubsidy), responsibilityAdjustment)"
	if !strings.Contains(body, wantExpr) {
		t.Fatalf("payment-service 的認列算式已改變（找不到 %q）——"+
			"Go 側 SellerLogisticsCharge 必須同步，否則兩軸金額會靜默分歧\n抽到的函式體：\n%s",
			wantExpr, body)
	}

	// 同一組輸入，Go 側必須算出同一個數。三個案例覆蓋 adjustment 的正／零／負。
	for _, c := range []struct {
		cost, subsidy, adjustment, want int64
	}{
		{120, 0, 0, 120},
		{180, 30, 0, 150},
		{180, 30, 20, 170},
		{180, 30, -50, 100},
	} {
		got, err := SellerLogisticsCharge(LogisticsFeeBasis{
			EstimatedCost:            c.cost,
			PlatformSubsidy:          c.subsidy,
			ResponsibilityAdjustment: c.adjustment,
		})
		if err != nil {
			t.Fatalf("cost=%d subsidy=%d adj=%d: %v", c.cost, c.subsidy, c.adjustment, err)
		}
		if got != c.want {
			t.Errorf("cost=%d subsidy=%d adj=%d ⇒ %d, want %d",
				c.cost, c.subsidy, c.adjustment, got, c.want)
		}
	}
}

// ============================================================================
// W5-5 Buyer Refund ≠ 取消 Seller Logistics Charge
// ============================================================================

// 🔴 全額退款之後，賣家的物流費一毛都不能少。
//
// 變異：把 ApplyBuyerRefund 改成 `charge - buyerRefundAmount` ⇒ 本測試轉紅。
func TestBuyerRefundDoesNotCancelSellerLogisticsCharge(t *testing.T) {
	basis := LogisticsFeeBasis{EstimatedCost: 150, BuyerShippingCharge: 60}
	before, err := SellerLogisticsCharge(basis)
	if err != nil {
		t.Fatalf("before: %v", err)
	}

	// 買家整筆訂單全退（含那 60 元運費）。
	for _, refund := range []int64{0, 60, 1200, 99999} {
		after, err := ApplyBuyerRefund(basis, refund)
		if err != nil {
			t.Fatalf("refund=%d: %v", refund, err)
		}
		if after != before {
			t.Errorf("退款 %d 後物流費 = %d, want %d（退款走價金軸，物流費走成本軸）",
				refund, after, before)
		}
	}
}

// 正對照：上一條不是「這個函式忽略一切輸入」——責任調整**確實**會改變金額。
// 少了這一條，把 ApplyBuyerRefund 改成 `return 0, nil` 也能讓上一條變綠？
// 不會（before 也是 150），但把兩者都改成常數就會 —— 本條釘住真正的可變軸。
func TestResponsibilityAdjustmentIsTheOnlyWayToReduceTheCharge(t *testing.T) {
	basis := LogisticsFeeBasis{EstimatedCost: 150, BuyerShippingCharge: 60}
	full, err := ApplyBuyerRefund(basis, 1200)
	if err != nil {
		t.Fatalf("full: %v", err)
	}
	if full != 150 {
		t.Fatalf("baseline = %d, want 150", full)
	}

	relieved := basis
	relieved.ResponsibilityAdjustment = -50 // ADR-004 判定平台負擔 50
	got, err := ApplyBuyerRefund(relieved, 1200)
	if err != nil {
		t.Fatalf("relieved: %v", err)
	}
	if got != 100 {
		t.Errorf("責任調整 −50 後 = %d, want 100 —— 責任必須是可動的那一軸", got)
	}
}

func TestApplyBuyerRefundRejectsNegativeRefund(t *testing.T) {
	_, err := ApplyBuyerRefund(LogisticsFeeBasis{EstimatedCost: 10}, -1)
	if !errors.Is(err, ErrLogisticsBuyerRefundNegative) {
		t.Errorf("err = %v, want ErrLogisticsBuyerRefundNegative", err)
	}
}

// ============================================================================
// W5-3 補收／退還 ⇒ Document Required；未定案 ⇒ MANUAL_DOCUMENT_REQUIRED
// ============================================================================

func TestBothAdjustmentDirectionsRequireADocument(t *testing.T) {
	for _, direction := range []LogisticsAdjustmentDirection{
		LogisticsAdjustmentSupplementaryCollection,
		LogisticsAdjustmentRefundToSeller,
	} {
		got, err := DispositionOfLogisticsAdjustment(direction)
		if err != nil {
			t.Fatalf("direction=%s: %v", direction, err)
		}
		if got != LogisticsDispositionDocumentRequired {
			t.Errorf("direction=%s ⇒ %s, want DOCUMENT_REQUIRED", direction, got)
		}
	}
}

// 反面：未知方向 不得靜默當成「不用開憑證」。
func TestUnknownAdjustmentDirectionFailsClosed(t *testing.T) {
	for _, direction := range []LogisticsAdjustmentDirection{"", "RESPONSIBILITY", "WHATEVER"} {
		if _, err := DispositionOfLogisticsAdjustment(direction); !errors.Is(err, ErrLogisticsAdjustmentDirectionInvalid) {
			t.Errorf("direction=%q ⇒ err=%v, want ErrLogisticsAdjustmentDirectionInvalid", direction, err)
		}
	}
}

// 🔴 會計形式尚未確認 ⇒ 新調整一律落 MANUAL_DOCUMENT_REQUIRED。
func TestInitialDocStatusIsManualWhileAccountingFormUndecided(t *testing.T) {
	if LogisticsAccountingFormDecided() {
		t.Fatal("LogisticsAccountingFormDecided 回 true —— W5-3 明載會計形式尚未確認；" +
			"真的定案了要連同 PrepareLogisticsAdjustmentDocument 的路徑一起補，不是只翻這個布林")
	}
	if got := InitialLogisticsAdjustmentDocStatus(); got != LogisticsDocStatusManualDocumentRequired {
		t.Errorf("初始狀態 = %s, want MANUAL_DOCUMENT_REQUIRED", got)
	}
}

// 🔴 陰性核心：MANUAL_DOCUMENT_REQUIRED 是一個**真的會擋住**的狀態。
//
// 兩條斷言缺一不可：
//   - 回錯誤（不是 log 完往下走）
//   - **草稿是零值**（不是「算好了但沒送」——後者很容易被下一個人順手接上送出路徑）
//
// 變異：把 PrepareLogisticsAdjustmentDocument 的狀態守衛拿掉 / 改成放行 ⇒ 本測試轉紅。
func TestManualDocumentRequiredActuallyBlocksIssuance(t *testing.T) {
	adj := LogisticsAdjustment{
		StoreID:        2685914682,
		RecipientTaxID: fxSellerTaxID,
		Direction:      LogisticsAdjustmentSupplementaryCollection,
		Amount:         300,
		DocStatus:      LogisticsDocStatusManualDocumentRequired,
	}

	draft, err := PrepareLogisticsAdjustmentDocument(adj)
	if !errors.Is(err, ErrLogisticsManualDocumentRequired) {
		t.Fatalf("err = %v, want ErrLogisticsManualDocumentRequired —— 不得 fake 自動開票", err)
	}
	if draft.SellerLogisticsCharge != 0 || len(draft.Lines) != 0 ||
		draft.Header.TotalAmount != 0 || draft.IssuerType != "" {
		t.Errorf("被擋下時仍回了非零草稿 %+v —— "+
			"「擋住了」與「算好了但沒送」必須長得不一樣", draft)
	}
}

// 反面（fail-closed）：值域外的狀態值同樣擋住，不得只擋那一個字串。
func TestUnknownDocStatusAlsoBlocks(t *testing.T) {
	for _, st := range []LogisticsAdjustmentDocStatus{"", "ISSUED", "READY", "manual_document_required"} {
		adj := LogisticsAdjustment{
			StoreID: 2685914682, RecipientTaxID: fxSellerTaxID,
			Direction: LogisticsAdjustmentSupplementaryCollection, Amount: 300, DocStatus: st,
		}
		if _, err := PrepareLogisticsAdjustmentDocument(adj); !errors.Is(err, ErrLogisticsManualDocumentRequired) {
			t.Errorf("status=%q ⇒ err=%v, want 擋住（fail-closed）", st, err)
		}
	}
}

// 正對照：上面那兩條不是「這個函式永遠回錯」——READY_TO_ISSUE ＋ 補收確實出得了草稿。
// 少了它，把 PrepareLogisticsAdjustmentDocument 改成 `return zero, ErrX` 也會全綠。
func TestReadyToIssueSupplementaryCollectionProducesADraft(t *testing.T) {
	adj := LogisticsAdjustment{
		StoreID: 2685914682, RecipientTaxID: fxSellerTaxID,
		Direction: LogisticsAdjustmentSupplementaryCollection, Amount: 300,
		DocStatus: LogisticsDocStatusReadyToIssue,
	}
	draft, err := PrepareLogisticsAdjustmentDocument(adj)
	if err != nil {
		t.Fatalf("READY_TO_ISSUE 的補收應該出得了草稿: %v", err)
	}
	if draft.SellerLogisticsCharge != 300 || draft.Header.TotalAmount != 300 {
		t.Errorf("draft charge=%d total=%d, want 300/300",
			draft.SellerLogisticsCharge, draft.Header.TotalAmount)
	}
	if draft.RecipientType != codemeta.InvoiceRecipientTypeSeller.Value {
		t.Errorf("recipientType = %q, want SELLER", draft.RecipientType)
	}
}

// 退還即使定案也不從這條路出去：它是折讓／退回憑證，不是新的銷項發票。
func TestRefundToSellerNeverIssuesANewSalesInvoice(t *testing.T) {
	adj := LogisticsAdjustment{
		StoreID: 2685914682, RecipientTaxID: fxSellerTaxID,
		Direction: LogisticsAdjustmentRefundToSeller, Amount: 300,
		DocStatus: LogisticsDocStatusReadyToIssue,
	}
	draft, err := PrepareLogisticsAdjustmentDocument(adj)
	if !errors.Is(err, ErrLogisticsManualDocumentRequired) {
		t.Fatalf("err = %v, want 擋住（退還須走折讓／退回憑證）", err)
	}
	if len(draft.Lines) != 0 {
		t.Errorf("被擋下時仍回了明細 %+v", draft.Lines)
	}
}

// ============================================================================
// helper
// ============================================================================

func logisticsRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("找不到 repo root（往上 12 層都沒有 go.work）")
	return ""
}

// ============================================================================
// 調整方向由 payment 側的兩個金額欄推導（不是另存一個碼）
// ============================================================================

func TestAdjustmentDirectionIsDerivedFromTheTwoAmountColumns(t *testing.T) {
	cases := []struct {
		name       string
		chargeable int64
		credit     int64
		want       LogisticsAdjustmentDirection
	}{
		{"只有應收為正 ⇒ 補收", 300, 0, LogisticsAdjustmentSupplementaryCollection},
		{"只有貸記為正 ⇒ 退還", 0, 120, LogisticsAdjustmentRefundToSeller},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := LogisticsAdjustmentDirectionOf(c.chargeable, c.credit)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

// 🔴 ADR-008 §1：應收與貸記是**兩個科目**，不是一個科目的正負。
// 同一列兩邊都掛著（或兩邊都空）代表產生它的 reconciliation 算錯了 ——
// 挑一邊當方向會把帳務錯誤變成一張開出去的憑證。
func TestAdjustmentDirectionFailsClosedWhenAmbiguous(t *testing.T) {
	cases := []struct {
		name       string
		chargeable int64
		credit     int64
	}{
		{"兩邊同時為正", 300, 120},
		{"兩邊都是 0", 0, 0},
		{"應收為負", -1, 0},
		{"貸記為負", 0, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := LogisticsAdjustmentDirectionOf(c.chargeable, c.credit)
			if !errors.Is(err, ErrLogisticsAdjustmentDirectionAmbiguous) {
				t.Fatalf("err=%v, want ErrLogisticsAdjustmentDirectionAmbiguous", err)
			}
			if got != "" {
				t.Errorf("拒絕時不得回一個看起來可用的方向，got %q", got)
			}
		})
	}
}
