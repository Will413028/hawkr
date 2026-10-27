package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// fakes
//
// 嵌入介面而不是逐方法實作空殼：沒被覆寫的方法是 nil，被呼叫時 panic 並指名方法。
// 空殼會回零值，而「回零值」與「這條路徑本來就不該被走到」在測試裡長得一模一樣。
// ---------------------------------------------------------------------------

type stubIssuers struct{ domain.IssuerRepository }

func (stubIssuers) GetByRef(context.Context, string, int64) (domain.Issuer, error) {
	return domain.Issuer{ID: 700001, DisplayName: "測試賣場"}, nil
}

type stubProfiles struct {
	domain.SellerTaxProfileRepository
	profile domain.SellerTaxProfile
}

func (s stubProfiles) GetByIssuer(context.Context, int64) (domain.SellerTaxProfile, error) {
	return s.profile, nil
}

type stubAccounts struct {
	domain.ProviderAccountRepository
	account domain.ProviderAccount
}

func (s stubAccounts) GetByIssuer(context.Context, int64, string) (domain.ProviderAccount, error) {
	return s.account, nil
}

type stubPrefs struct {
	domain.BuyerPreferenceRepository
}

func (stubPrefs) GetDefault(context.Context, int64) (domain.BuyerPreference, bool, error) {
	return domain.BuyerPreference{}, false, nil
}

var testCutover = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// approvedSeller 是已核定、AMEGO 可用、切點已過的賣家。
func approvedSeller() (domain.SellerTaxProfile, domain.ProviderAccount) {
	return domain.SellerTaxProfile{
		ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		DefaultTaxCategory: 1,
	}, domain.ProviderAccount{
		ID:                  555,
		Status:              codemeta.InvoiceProviderAccountStatusActive.Value,
		AutomationCutoverAt: &testCutover,
	}
}

// blockedPendingSeller 是待審中且接單上限已滿的賣家。
func blockedPendingSeller() (domain.SellerTaxProfile, domain.ProviderAccount) {
	_, account := approvedSeller()
	return domain.SellerTaxProfile{
		ReviewStatus:            codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
		DefaultTaxCategory:      1,
		PendingOrderLimitCount:  50,
		PendingOrderLimitAmount: 100000,
		PendingOrderCount:       50,
		PendingOrderAmount:      100000,
	}, account
}

func newServer(profile domain.SellerTaxProfile, account domain.ProviderAccount) *InvoiceServer {
	return NewInvoiceServer(usecase.NewCheckout(
		stubIssuers{},
		stubProfiles{profile: profile},
		stubAccounts{account: account},
		stubPrefs{},
		nil,
	), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

// internalCtx 造一個「來自內部服務」的 context。
//
// ⚠️ 只帶 x-internal-service 不帶簽章 —— 那是刻意的：驗簽是 interceptor 的職責
// （InternalAuthServerInterceptor），本檔驗的是 handler 這一層的授權閘門。
func internalCtx() context.Context {
	return metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(interceptor.HeaderInternalService, "order-service"))
}

func resolveReq() *pb.ResolveOrderInvoiceModeInternalRequest {
	return &pb.ResolveOrderInvoiceModeInternalRequest{
		StoreId:       "2685914682",
		BuyerId:       "2623900520",
		ProductAmount: "100",
		CheckoutId:    "CK-0001",
	}
}

func validateReq() *pb.ValidateOrderInvoiceInputInternalRequest {
	return &pb.ValidateOrderInvoiceInputInternalRequest{
		StoreId:         "2685914682",
		BuyerId:         "2623900520",
		OrderNo:         "O000123456",
		Amount:          &pb.InvoiceAmountBreakdown{ProductAmount: "100"},
		SnapshotVersion: "1",
		IdempotencyKey:  "IK-0001",
	}
}

// ---------------------------------------------------------------------------
// internal-only 閘門
// ---------------------------------------------------------------------------

// TestResolveOrderInvoiceModeInternal_RejectsNonInternalCall 守 §5.2：
// 這兩支只給服務間呼叫，使用者面（gateway 代理過來、不帶 x-internal-service）一律拒絕。
//
// 判準是**兩件事**：回錯誤，而且 **data 是空的**。只斷言「有錯誤」的測試擋不住
// 「回了錯誤卻同時把 data 填好」這個形狀 —— 而 gRPC 允許 response 與 error 併回。
func TestResolveOrderInvoiceModeInternal_RejectsNonInternalCall(t *testing.T) {
	s := newServer(approvedSeller())

	resp, err := s.ResolveOrderInvoiceModeInternal(context.Background(), resolveReq())
	assertDeniedWithNoData(t, resp == nil, err)

	// 帶了 header 但值是空字串 ⇒ 一樣不算內部呼叫（interceptor 對空值也視為非內部）。
	empty := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(interceptor.HeaderInternalService, ""))
	resp, err = s.ResolveOrderInvoiceModeInternal(empty, resolveReq())
	assertDeniedWithNoData(t, resp == nil, err)
}

func TestValidateOrderInvoiceInputInternal_RejectsNonInternalCall(t *testing.T) {
	s := newServer(approvedSeller())

	resp, err := s.ValidateOrderInvoiceInputInternal(context.Background(), validateReq())
	assertDeniedWithNoData(t, resp == nil, err)
}

func assertDeniedWithNoData(t *testing.T, respIsNil bool, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("非內部呼叫必須被拒絕")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("status code = %v, want PermissionDenied", status.Code(err))
	}
	if !respIsNil {
		t.Error("被拒絕時不得吐出任何 data")
	}
}

// TestRequireInternalCall_AcceptsInternalCall 是閘門的陰性面：
// 帶了 header 的呼叫必須通過。少了這一支，把閘門寫成「一律拒絕」也會全綠。
func TestRequireInternalCall_AcceptsInternalCall(t *testing.T) {
	if err := requireInternalCall(internalCtx()); err != nil {
		t.Fatalf("內部呼叫不該被擋：%v", err)
	}
}

// ---------------------------------------------------------------------------
// envelope 形狀
// ---------------------------------------------------------------------------

func TestResolveOrderInvoiceModeInternal_Success(t *testing.T) {
	s := newServer(approvedSeller())
	resp, err := s.ResolveOrderInvoiceModeInternal(internalCtx(), resolveReq())
	if err != nil {
		t.Fatalf("內部呼叫失敗：%v", err)
	}
	if resp.GetCode() != 0 || resp.GetMessage() != "success" {
		t.Fatalf("envelope = (%d, %q), want (0, success)", resp.GetCode(), resp.GetMessage())
	}
	d := resp.GetData()
	if d == nil {
		t.Fatal("code 0 必須帶 data")
	}
	if d.GetInvoiceMode() != codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		t.Errorf("invoice_mode = %q, want AMEGO_AUTO", d.GetInvoiceMode())
	}
	if d.GetPreviewExpiresAt() == "" {
		t.Error("preview_expires_at 必須由後端給（§6.1.2(a)：前端不得自算 TTL）")
	}
	if got := d.GetAmount().GetInvoiceAmount(); got != "100" {
		t.Errorf("invoice_amount = %q, want \"100\"（金額一律字串）", got)
	}
	if got := d.GetAmount().GetTaxCategory(); got != "1" {
		t.Errorf("tax_category = %q, want \"1\"（CodeMeta 的 Value 不是 Code）", got)
	}
	// 沒有預設常用發票資料、也不在待審期間 ⇒ 兩者都是 null 而不是空 message。
	if d.GetDefaultProfile() != nil {
		t.Error("沒有預設常用發票資料時必須回 null，不得合成一列（§6.1.1 定版）")
	}
	if d.GetIntakeLimit() != nil {
		t.Error("非待審期間 intake_limit 必須為 null；回一組全 0 會被讀成「上限 0 筆」")
	}
}

// TestResolveOrderInvoiceModeInternal_MissingStoreID：契約違規回 InvalidArgument。
//
// 靜默當成 0 的話，決策樹會回一個看起來很合理的 LEGACY_MANUAL，
// 而真正的問題（呼叫端沒帶 store_id）永遠不會有人發現。
func TestResolveOrderInvoiceModeInternal_MissingStoreID(t *testing.T) {
	s := newServer(approvedSeller())
	req := resolveReq()
	req.StoreId = ""
	if _, err := s.ResolveOrderInvoiceModeInternal(internalCtx(), req); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestValidateOrderInvoiceInputInternal_Success(t *testing.T) {
	s := newServer(approvedSeller())
	resp, err := s.ValidateOrderInvoiceInputInternal(internalCtx(), validateReq())
	if err != nil {
		t.Fatalf("內部呼叫失敗：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d（message=%q），want 0", resp.GetCode(), resp.GetMessage())
	}
	d := resp.GetData()
	if !d.GetValid() {
		t.Error("valid 應為 true")
	}
	if d.GetSnapshotJson() == "" {
		t.Error("放行時必須回可持久化的快照")
	}
	if d.GetInvoiceMode() != codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		t.Errorf("invoice_mode = %q, want AMEGO_AUTO", d.GetInvoiceMode())
	}
}

// TestValidateOrderInvoiceInputInternal_RejectionStillCarriesData 守
// 「驗證失敗回非 0 業務碼，且**不得** code=0 ＋ 空 data」。
//
// 9326 是最能說明理由的那一格：§6.1.2 對它明文要求 `data.intakeLimit` 帶現況
// （HTTP 200 ＋ preserveData）。回 code≠0 ＋ 空 data 的話，「上限還差多少」永遠到不了前端。
func TestValidateOrderInvoiceInputInternal_RejectionStillCarriesData(t *testing.T) {
	s := newServer(blockedPendingSeller())

	req := validateReq()
	req.Invoice = &pb.InvoiceInputSpec{}

	resp, err := s.ValidateOrderInvoiceInputInternal(internalCtx(), req)
	if err != nil {
		t.Fatalf("業務拒絕不該回 gRPC error（那會讓六個分支塌縮成同一個 424）：%v", err)
	}
	if resp.GetCode() != bizcode.SellerInvoiceIntakeLimitReached {
		t.Fatalf("code = %d, want %d", resp.GetCode(), bizcode.SellerInvoiceIntakeLimitReached)
	}
	if resp.GetMessage() == "" {
		t.Error("非 0 業務碼必須帶訊息")
	}
	d := resp.GetData()
	if d == nil {
		t.Fatal("拒絕時仍必須帶 data")
	}
	if d.GetValid() {
		t.Error("valid 應為 false")
	}
	if d.GetIntakeLimit() == nil {
		t.Fatal("9326 必須帶 intake_limit 現況（§6.1.2）")
	}
	if !d.GetIntakeLimit().GetBlocked() {
		t.Error("intake_limit.blocked 應為 true")
	}
	if d.GetSnapshotJson() != "" {
		t.Error("拒絕時不得回快照")
	}
}
