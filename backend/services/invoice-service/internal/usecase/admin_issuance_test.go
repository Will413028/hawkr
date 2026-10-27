package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// 端點 57 是**不可逆的對外動作**：開出去的發票收不回來。本檔的陰性測試因此全部
// 額外斷言「一列都沒建」——只回錯誤但已經建了 document，比不回錯誤更糟。

type issuanceFixture struct {
	uc       *AdminIssuance
	docs     *fakeDocRepo
	ops      *fakeOpRepo
	audit    *fakeAudit
	issuers  *fakeIssuerRepo
	profiles *fakeProfileRepo
}

func newIssuanceFixture() *issuanceFixture {
	docs := &fakeDocRepo{byRequestNo: map[string]domain.Document{}}
	ops := &fakeOpRepo{}
	audit := &fakeAudit{}
	taxID := "12345678"
	legal := "測試股份有限公司"
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{
		ID: 900, IssuerType: "PLATFORM", IssuerRefID: 0, DisplayName: "範例生鮮股份有限公司",
	}}
	profiles := &fakeProfileRepo{profile: domain.SellerTaxProfile{
		ID: 5, IssuerID: 900, TaxID: &taxID, LegalName: &legal,
	}}
	seq := int64(0)
	uc := NewAdminIssuance(directTx{}, issuers, profiles, docs, ops, audit,
		ManualIssuanceConfig{PlatformTaxID: "45444444"},
		IssuanceConfig{MaxAttempts: 5},
		func() int64 { seq++; return 90000 + seq },
		zap.NewNop(),
	).WithClock(func() time.Time { return adminNow })
	return &issuanceFixture{uc: uc, docs: docs, ops: ops, audit: audit, issuers: issuers, profiles: profiles}
}

func manualInput(kind string) ManualIssuanceInput {
	return ManualIssuanceInput{
		ActorAccountID: 42,
		Issuer:         ManualIssuanceIssuer{Kind: codemeta.InvoiceIssuerTypePlatform.Value},
		Recipient:      ManualIssuanceRecipient{Kind: codemeta.InvoiceRecipientTypeExternal.Value, Name: "某公司"},
		Source: ManualIssuanceSource{
			Kind: issuanceSourceManualLines,
			Lines: []ManualIssuanceLine{
				{LineType: "PRODUCT", Name: "平台服務費", Quantity: 1, UnitPrice: 1000, NetAmount: 1000, TaxCategory: "TAXABLE"},
			},
		},
		DocumentKind: kind, Reason: "補開服務費發票", Confirmed: true, IdempotencyKey: "idem-i",
	}
}

func TestAdminIssuance_PlatformToExternal_B2C(t *testing.T) {
	fx := newIssuanceFixture()
	got, err := fx.uc.Create(context.Background(), manualInput("B2C"))
	if err != nil {
		t.Fatalf("Create 失敗：%v", err)
	}
	if len(fx.docs.created) != 1 {
		t.Fatalf("應建一列 document，實得 %d", len(fx.docs.created))
	}
	d := fx.docs.created[0]
	// chk_invdoc_source_ref 是雙向等式：MANUAL ⇒ source_ref 必須留空。
	if d.SourceType != sourceTypeManual || d.SourceRef != "" {
		t.Fatalf("MANUAL 來源不得帶 source_ref，實得 type=%q ref=%q", d.SourceType, d.SourceRef)
	}
	if d.TriggerType != triggerTypeManual {
		t.Fatalf("trigger 應為 ADM_MANUAL，實得 %q", d.TriggerType)
	}
	// chk_invdoc_kind_mode：非 RECEIPT ⇒ 不得是 PLATFORM_RECEIPT。
	if d.FulfillmentMode != codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		t.Fatalf("B2C 的履約模式應為 AMEGO_AUTO，實得 %q", d.FulfillmentMode)
	}
	// chk_invdoc_receipt 是雙向等式：非 RECEIPT ⇒ receipt_no 必須留空。
	if d.ReceiptNo != "" {
		t.Fatalf("非 RECEIPT 不得帶 receipt_no，實得 %q", d.ReceiptNo)
	}
	// chk_invdoc_deadline_presence：B2C/B2B ⟺ statutory_deadline_at 非空。
	if d.StatutoryDeadlineAt == nil {
		t.Fatal("B2C 必須有法定期限")
	}
	// chk_invdoc_b2c：B2C 的稅額恆 0、統編恆 10 個 0。
	if d.TaxAmount != 0 || d.RecipientTaxID != domain.B2CBuyerIdentifier {
		t.Fatalf("B2C 的稅額應為 0、統編應為 10 個 0，實得 tax=%d id=%q", d.TaxAmount, d.RecipientTaxID)
	}
	// chk_invdoc_total：TotalAmount = Sales + FreeTax + ZeroTax + Tax。
	if d.TotalAmount != d.SalesAmount+d.FreeTaxSalesAmount+d.ZeroTaxSalesAmount+d.TaxAmount {
		t.Fatalf("表頭四欄不自洽：%+v", d)
	}
	if d.TotalAmount != 1000 {
		t.Fatalf("總額應為 1000，實得 %d", d.TotalAmount)
	}
	// issuer_name_snapshot 是印在發票上的銷售人名稱，不得是佔位字串。
	if d.IssuerNameSnapshot != "範例生鮮股份有限公司" || d.IssuerTaxIDSnapshot != "45444444" {
		t.Fatalf("PLATFORM 的抬頭／統編快照不正確：%q / %q", d.IssuerNameSnapshot, d.IssuerTaxIDSnapshot)
	}
	// 冪等鍵落 provider_request_no（唯一擋得住重複開票的索引），且長度不得超過 varchar(40)。
	if !strings.HasPrefix(d.ProviderRequestNo, "MAN-") || len(d.ProviderRequestNo) > 40 {
		t.Fatalf("provider_request_no 不合格：%q（len=%d）", d.ProviderRequestNo, len(d.ProviderRequestNo))
	}
	// operation 必須建立**並**推進到 READY：停在 CREATED 的話沒有任何 worker 撈得到它。
	if len(fx.ops.created) != 1 || got.Status != "READY" {
		t.Fatalf("應建 operation 並推進 READY，實得 created=%d status=%q", len(fx.ops.created), got.Status)
	}
	if got.DocumentID == 0 || got.JobID == 0 {
		t.Fatalf("回應必須帶 documentId 與 jobId，實得 %+v", got)
	}
	assertNoPIIInAudit(t, fx.audit.admin[0])
}

func TestAdminIssuance_Receipt(t *testing.T) {
	fx := newIssuanceFixture()
	in := manualInput("RECEIPT")
	in.Issuer = ManualIssuanceIssuer{Kind: codemeta.InvoiceIssuerTypePlatform.Value}
	if _, err := fx.uc.Create(context.Background(), in); err != nil {
		t.Fatalf("Create 失敗：%v", err)
	}
	d := fx.docs.created[0]
	// 三條雙向等式一起看：RECEIPT ⟺ PLATFORM_RECEIPT ⟺ receipt_no 非空；
	// 且 RECEIPT 不得有 provider（chk_invdoc_receipt_no_provider）、不得有法定期限。
	if d.FulfillmentMode != codemeta.InvoiceFulfillmentModePlatformReceipt.Value {
		t.Fatalf("RECEIPT 的履約模式應為 PLATFORM_RECEIPT，實得 %q", d.FulfillmentMode)
	}
	if d.ReceiptNo == "" {
		t.Fatal("RECEIPT 必須帶 receipt_no")
	}
	if d.Provider != "" {
		t.Fatalf("RECEIPT 不得帶 provider，實得 %q", d.Provider)
	}
	if d.StatutoryDeadlineAt != nil {
		t.Fatal("RECEIPT 不得有法定期限")
	}
}

// 合法組合矩陣是正面列舉：不在表上的一律 9346，且一列都不能建。
func TestAdminIssuance_ComboMatrix(t *testing.T) {
	cases := []struct {
		name      string
		issuer    string
		recipient string
		source    string
		ok        bool
	}{
		{"賣家→非會員 手填", "SELLER", "EXTERNAL", issuanceSourceManualLines, true},
		{"平台→賣家 手填", "PLATFORM", "SELLER", issuanceSourceManualLines, true},
		{"平台→非會員 手填", "PLATFORM", "EXTERNAL", issuanceSourceManualLines, true},
		{"賣家→賣家（自己開給自己）", "SELLER", "SELLER", issuanceSourceManualLines, false},
		{"平台→買家（平台不是銷售方）", "PLATFORM", "BUYER", issuanceSourceManualLines, false},
		{"賣家→非會員 但走訂單快照", "SELLER", "EXTERNAL", issuanceSourceOrderSnapshot, false},
		{"平台→非會員 但走訂單快照", "PLATFORM", "EXTERNAL", issuanceSourceOrderSnapshot, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newIssuanceFixture()
			in := manualInput("B2C")
			in.Issuer = ManualIssuanceIssuer{Kind: c.issuer, StoreID: 2685914682}
			in.Recipient = ManualIssuanceRecipient{Kind: c.recipient, StoreID: 2685914682, Name: "某公司"}
			in.Source.Kind = c.source
			// 方向二恆為 B2B（平台對營業人開票）。
			if c.recipient == "SELLER" {
				in.DocumentKind = "B2B"
			}

			_, err := fx.uc.Create(context.Background(), in)
			if c.ok {
				if err != nil {
					t.Fatalf("合法組合被拒：%v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("非法組合被放行了")
			}
			// 矩陣檢查排在能力缺口之前，所以本表的每一列非法組合都是 9346——
			// 包含那兩列 EXTERNAL ＋ ORDER_SNAPSHOT（它們本來就不在 ✅ 四列裡，
			// 不是「合法但還沒接線」）。唯一走到能力缺口的是 SELLER→BUYER＋ORDER_SNAPSHOT，
			// 由 TestAdminIssuance_OrderSnapshotUnavailable 守。
			if got := bizCodeOf(t, err); got != bizcode.InvoiceIssuanceTargetInvalid {
				t.Fatalf("期望 9346，實得 %d", got)
			}
			if len(fx.docs.created) != 0 {
				t.Fatal("非法組合不得建立任何 document")
			}
		})
	}
}

// ORDER_SNAPSHOT 是矩陣允許但本服務湊不齊事實的分支：回錯誤，不是用零值往下走。
func TestAdminIssuance_OrderSnapshotUnavailable(t *testing.T) {
	fx := newIssuanceFixture()
	in := manualInput("B2C")
	in.Issuer = ManualIssuanceIssuer{Kind: "SELLER", StoreID: 2685914682}
	in.Recipient = ManualIssuanceRecipient{Kind: "BUYER", BuyerID: 2623900520}
	in.Source = ManualIssuanceSource{Kind: issuanceSourceOrderSnapshot, OrderNo: "O2026082000001"}

	_, err := fx.uc.Create(context.Background(), in)
	if !errors.Is(err, ErrIssuanceOrderSnapshotUnavailable) {
		t.Fatalf("期望 ErrIssuanceOrderSnapshotUnavailable，實得 %v", err)
	}
	if len(fx.docs.created) != 0 {
		t.Fatal("不得建立任何 document")
	}
}

func TestAdminIssuance_Guards(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ManualIssuanceInput)
		want error
	}{
		{"缺二次確認", func(i *ManualIssuanceInput) { i.Confirmed = false }, ErrIssuanceConfirmRequired},
		{"缺理由", func(i *ManualIssuanceInput) { i.Reason = "  " }, ErrIssuanceReasonRequired},
		{"未知 documentKind", func(i *ManualIssuanceInput) { i.DocumentKind = "B2X" }, ErrIssuanceDocumentKindInvalid},
		{"未知 sourceKind", func(i *ManualIssuanceInput) { i.Source.Kind = "SOMETHING" }, ErrIssuanceSourceKindInvalid},
		{"零行", func(i *ManualIssuanceInput) { i.Source.Lines = nil }, ErrIssuanceLinesRequired},
		{"金額對不起來", func(i *ManualIssuanceInput) { i.Source.Lines[0].NetAmount = 999 }, ErrIssuanceLineInvalid},
		{"PRODUCT 行為負", func(i *ManualIssuanceInput) {
			i.Source.Lines[0].UnitPrice = -1000
			i.Source.Lines[0].NetAmount = -1000
		}, ErrIssuanceLineSignInvalid},
		{"未知 lineType", func(i *ManualIssuanceInput) { i.Source.Lines[0].LineType = "BUNDLE" }, ErrIssuanceLineTypeInvalid},
		// #3089 B 組第 6 項：chk_invline_qty 在此之前**全鏈路零業務守衛**
		// （usecase／grpc／gateway／domain 四層皆無）⇒ 實打 HTTP 500。
		// quantity=0 是真實誤操作的形狀（ADM 在數量格打 0）。
		{"數量 0", func(i *ManualIssuanceInput) {
			i.Source.Lines[0].Quantity = 0
			i.Source.Lines[0].NetAmount = 0
		}, ErrIssuanceLineQuantityInvalid},
		// 🔴 這一格是本項的關鍵：quantity=-1 ＋ unitPrice=-100 ⇒ netAmount=100，
		// **chk_invline_amount 與 chk_invline_sign 兩道都會通過**（乘法對得上、
		// 非折扣行金額為正）。只有數量那一道擋得住 ⇒ 它必須排在那兩道之前。
		{"數量負且金額與 sign 都對得上", func(i *ManualIssuanceInput) {
			i.Source.Lines[0].Quantity = -1
			i.Source.Lines[0].UnitPrice = -100
			i.Source.Lines[0].NetAmount = 100
		}, ErrIssuanceLineQuantityInvalid},
		{"未知稅別", func(i *ManualIssuanceInput) { i.Source.Lines[0].TaxCategory = "SOMETHING" }, ErrIssuanceLineTaxInvalid},
		// #3089 稽核 A-2：折扣行與商品行稅別不同 ⇒ 單一稅別的加總為負，
		// **而總額仍為正** ⇒ ErrIssuanceTotalNotPositive 那道放行，落庫 23514。
		// 每一行單獨看都合法（qty／amount／sign 三道全過），只有表頭看得出來。
		{"折扣行稅別與商品行不同（表頭單欄為負）", func(i *ManualIssuanceInput) {
			i.Source.Lines[0].TaxCategory = "TAX_EXEMPT"
			i.Source.Lines[0].Quantity = 1
			i.Source.Lines[0].UnitPrice = 1000
			i.Source.Lines[0].NetAmount = 1000
			i.Source.Lines = append(i.Source.Lines, ManualIssuanceLine{
				LineType: "DISCOUNT", Name: "折抵", TaxCategory: "TAXABLE",
				Quantity: 1, UnitPrice: -500, NetAmount: -500,
			})
		}, ErrIssuanceHeaderNegative},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newIssuanceFixture()
			in := manualInput("B2C")
			c.mut(&in)
			if _, err := fx.uc.Create(context.Background(), in); !errors.Is(err, c.want) {
				t.Fatalf("期望 %v，實得 %v", c.want, err)
			}
			if len(fx.docs.created) != 0 {
				t.Fatal("守衛不成立不得建立任何 document")
			}
		})
	}
}

// DISCOUNT 行必須為負（chk_invline_sign），且總額不得歸零。
func TestAdminIssuance_DiscountLineAndZeroTotal(t *testing.T) {
	fx := newIssuanceFixture()
	in := manualInput("B2C")
	in.Source.Lines = append(in.Source.Lines, ManualIssuanceLine{
		LineType: "DISCOUNT", Name: "折抵", Quantity: 1, UnitPrice: -1000, NetAmount: -1000, TaxCategory: "TAXABLE",
	})
	if _, err := fx.uc.Create(context.Background(), in); !errors.Is(err, ErrIssuanceTotalNotPositive) {
		t.Fatalf("總額 0 的發票不該開出來，實得 %v", err)
	}
	if len(fx.docs.created) != 0 {
		t.Fatal("不得建立任何 document")
	}
}

func TestAdminIssuance_B2BRequiresEightDigitTaxID(t *testing.T) {
	fx := newIssuanceFixture()
	in := manualInput("B2B")
	in.Recipient.TaxID = "0012345678" // 左補零成 10 碼會被 AMEGO 3040122 拒
	if _, err := fx.uc.Create(context.Background(), in); !errors.Is(err, ErrIssuanceRecipientTaxID) {
		t.Fatalf("期望 ErrIssuanceRecipientTaxID，實得 %v", err)
	}
}

// 同樣內容的第二次請求回 9347 ＋ 既有 documentId，不是再開一張。
func TestAdminIssuance_Idempotent(t *testing.T) {
	fx := newIssuanceFixture()
	first, err := fx.uc.Create(context.Background(), manualInput("B2C"))
	if err != nil {
		t.Fatalf("第一次開票失敗：%v", err)
	}
	// 把第一次的結果放進「已存在」表，模擬第二次請求。
	fx.docs.byRequestNo[fx.docs.created[0].ProviderRequestNo] = domain.Document{
		ID: first.DocumentID, Status: "PENDING",
	}

	second, err := fx.uc.Create(context.Background(), manualInput("B2C"))
	if err != nil {
		t.Fatalf("9347 是 200 不是錯誤，實得 %v", err)
	}
	if !second.Duplicate {
		t.Fatal("重複請求必須標記為 Duplicate")
	}
	if second.DocumentID != first.DocumentID {
		t.Fatalf("必須回既有 documentId %d，實得 %d", first.DocumentID, second.DocumentID)
	}
	if len(fx.docs.created) != 1 {
		t.Fatalf("重複請求不得再建一列，實得 %d 列", len(fx.docs.created))
	}
}

// 冪等鍵只由請求內容決定：內容不同就是另一張發票。
func TestAdminIssuance_RequestNoVariesWithContent(t *testing.T) {
	issuer := domain.Issuer{ID: 900}
	a := manualProviderRequestNo(issuer, manualInput("B2C"))
	in := manualInput("B2C")
	in.Source.Lines[0].NetAmount = 2000
	in.Source.Lines[0].UnitPrice = 2000
	b := manualProviderRequestNo(issuer, in)
	if a == b {
		t.Fatal("金額不同的兩份請求不得共用同一把冪等鍵")
	}
	// Idempotency-Key 不進鍵：同內容配不同鍵仍然是同一張發票。
	in2 := manualInput("B2C")
	in2.IdempotencyKey = "another-key"
	if manualProviderRequestNo(issuer, in2) != a {
		t.Fatal("Idempotency-Key 不得影響業務冪等鍵——含了它就會開出第二張發票")
	}
}
