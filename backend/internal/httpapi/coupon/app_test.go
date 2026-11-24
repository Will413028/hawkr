package coupon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	callerAccount = 1
	callerBuyerID = 501
	callerStoreID = 11
)

var (
	now      = time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	campID   = uuid.UUID{7}
	voucherA = uuid.UUID{8}
)

type fakeModule struct {
	err   error
	calls []string

	voucherIn    coupon.VoucherListInput
	claimableIn  coupon.ClaimableInput
	sellerListIn coupon.SellerListInput
	createIn     coupon.CreateCampaignInput
	endIn        coupon.EndCampaignInput
	claimedBy    int64
	detailStore  int64

	vouchers  coupon.VoucherPage
	view      coupon.VoucherView
	claimable coupon.CampaignPage
	created   coupon.Campaign
	list      coupon.SellerCampaignPage
	detail    coupon.CampaignDetail
}

func (f *fakeModule) ListVouchers(_ context.Context, in coupon.VoucherListInput) (coupon.VoucherPage, error) {
	f.calls = append(f.calls, "ListVouchers")
	f.voucherIn = in
	return f.vouchers, f.err
}

func (f *fakeModule) ListClaimable(_ context.Context, in coupon.ClaimableInput) (coupon.CampaignPage, error) {
	f.calls = append(f.calls, "ListClaimable")
	f.claimableIn = in
	return f.claimable, f.err
}

func (f *fakeModule) GetVoucher(context.Context, int64, uuid.UUID) (coupon.VoucherView, error) {
	f.calls = append(f.calls, "GetVoucher")
	return f.view, f.err
}

func (f *fakeModule) Claim(_ context.Context, _ uuid.UUID, buyerID int64) (coupon.ClaimResult, error) {
	f.calls = append(f.calls, "Claim")
	f.claimedBy = buyerID
	return coupon.ClaimResult{Voucher: f.view.Voucher}, f.err
}

func (f *fakeModule) CreateCampaign(_ context.Context, in coupon.CreateCampaignInput) (coupon.Campaign, error) {
	f.calls = append(f.calls, "CreateCampaign")
	f.createIn = in
	return f.created, f.err
}

func (f *fakeModule) ListSellerCampaigns(_ context.Context, in coupon.SellerListInput) (coupon.SellerCampaignPage, error) {
	f.calls = append(f.calls, "ListSellerCampaigns")
	f.sellerListIn = in
	return f.list, f.err
}

func (f *fakeModule) GetSellerCampaign(_ context.Context, storeID int64, _ uuid.UUID) (coupon.CampaignDetail, error) {
	f.calls = append(f.calls, "GetSellerCampaign")
	f.detailStore = storeID
	return f.detail, f.err
}

func (f *fakeModule) EndCampaign(_ context.Context, in coupon.EndCampaignInput) (coupon.EndCampaignResult, error) {
	f.calls = append(f.calls, "EndCampaign")
	f.endIn = in
	return coupon.EndCampaignResult{Voided: 3}, f.err
}

type fakeBuyers struct {
	displays []buyer.Display
	err      error
}

func (f *fakeBuyers) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	if accountID != callerAccount {
		return 0, apperr.NotFound("no buyer")
	}
	return callerBuyerID, nil
}

func (f *fakeBuyers) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) {
	return f.displays, f.err
}

// fakeStores 記下 ActingStore 被要求的能力；suspended 時回停權的拒絕。
type fakeStores struct {
	suspended bool
	asked     []seller.Capability
	cards     []seller.StoreCard
	cardsErr  error
}

func (f *fakeStores) ActingStore(_ context.Context, _ int64, c seller.Capability) (seller.Store, error) {
	f.asked = append(f.asked, c)
	if f.suspended {
		return seller.Store{}, apperr.PermissionDenied("suspended").WithReason(apperr.ReasonSellerSuspended)
	}
	return seller.Store{StoreID: callerStoreID, AccountID: callerAccount}, nil
}

func (f *fakeStores) StoreCards(context.Context, []int64) ([]seller.StoreCard, error) {
	return f.cards, f.cardsErr
}

type rig struct {
	app    *App
	module *fakeModule
	buyers *fakeBuyers
	stores *fakeStores
}

func newRig(t *testing.T) *rig {
	t.Helper()
	im, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{module: &fakeModule{}, buyers: &fakeBuyers{}, stores: &fakeStores{}}
	r.app, err = NewApp(r.module, r.buyers, r.stores, im, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func signedIn() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
}

func codeOf(err error) connect.Code {
	return connect.CodeOf(rpc.ConnectError(context.Background(), err))
}

func reasonAndFields(err error) (reason string, fields []string) {
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		v, e := d.Value()
		if e != nil {
			continue
		}
		switch x := v.(type) {
		case *errdetails.ErrorInfo:
			reason = x.GetReason()
		case *errdetails.BadRequest:
			for _, f := range x.GetFieldViolations() {
				fields = append(fields, f.GetField())
			}
		}
	}
	return reason, fields
}

func cashInput() *pb.CampaignInput {
	return &pb.CampaignInput{
		Title: "春季券", Discount: &pb.CampaignInput_Cash{Cash: &pb.CashDiscount{Amount: 50}}, ThresholdAmount: 200,
		TargetType: pb.TargetType_TARGET_TYPE_ALL, StartTime: timestamppb.New(now), EndTime: timestamppb.New(now.Add(24 * time.Hour)),
	}
}

func campaign() coupon.Campaign {
	store := int64(callerStoreID)
	return coupon.Campaign{
		ID: campID, Title: "春季券", DiscountType: coupon.DiscountCash, DiscountAmount: 50, ThresholdAmount: 200, TargetType: coupon.TargetAll,
		StoreID: &store, IssuerStoreID: &store, StartAt: now, EndAt: now.Add(24 * time.Hour), Status: coupon.StatusActive, CreatedAt: now,
	}
}

// 宣告與 service 的方法一一對應：缺一列不能啟動；賣家方法宣告的能力就是 actingStore 向 seller 要的能力（宣告＝執行）。
func TestDeclarationsMatchTheServiceAndDriveTheCapabilityCheck(t *testing.T) {
	r := newRig(t)
	for _, p := range []string{couponv1alpha1connect.CouponAppServiceCreateCampaignProcedure, couponv1alpha1connect.CouponAppServiceEndCampaignProcedure} {
		if _, err := r.app.actingStore(signedIn(), p); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(r.stores.asked, []seller.Capability{seller.CapMarketing, seller.CapContentContract}) {
		t.Fatalf("向 seller 要的能力 = %v，預期 CAP_MARKETING 與 CAP_CONTENT_CONTRACT", r.stores.asked)
	}
	if _, err := r.app.actingStore(signedIn(), couponv1alpha1connect.CouponAppServiceClaimCouponProcedure); err == nil {
		t.Fatal("買家方法沒有宣告能力，不能取店家")
	}
	missing := map[string]declaration{}
	for k, v := range declarations {
		missing[k] = v
	}
	delete(missing, couponv1alpha1connect.CouponAppServiceGetCampaignProcedure)
	if _, err := newApp(missing, r.module, r.buyers, r.stores, r.app.images, r.app.now, r.app.log); err == nil {
		t.Fatal("缺宣告要不能啟動")
	}
}

func TestCreateCampaignDistinguishesNoLimitFromZero(t *testing.T) {
	r := newRig(t)
	r.module.created = campaign()
	in := cashInput()
	if _, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in})); err != nil {
		t.Fatal(err)
	}
	if r.module.createIn.TotalLimit != 0 {
		t.Fatalf("不設上限應傳給模組 0（無上限），得到 %d", r.module.createIn.TotalLimit)
	}
	zero := int32(0)
	in.TotalIssueLimit = &zero
	r.module.calls = nil
	_, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in}))
	_, fields := reasonAndFields(err)
	if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"campaign.total_issue_limit"}) || len(r.module.calls) != 0 {
		t.Fatalf("上限 0 要擋在入口且不呼叫模組：code=%v fields=%v calls=%v", codeOf(err), fields, r.module.calls)
	}
	five := int32(5)
	in.TotalIssueLimit = &five
	if _, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in})); err != nil || r.module.createIn.TotalLimit != 5 {
		t.Fatalf("上限 5：err=%v limit=%d", err, r.module.createIn.TotalLimit)
	}
}

func TestSellerCampaignWireKeepsNoLimitUnset(t *testing.T) {
	c := campaign()
	if got := toSellerCampaign(c, now); got.TotalIssueLimit != nil {
		t.Fatalf("無上限的活動 total_issue_limit 應不設，得到 %d", got.GetTotalIssueLimit())
	}
	limit := int32(1)
	c.TotalLimit = &limit
	if got := toSellerCampaign(c, now); got.TotalIssueLimit == nil || got.GetTotalIssueLimit() != 1 {
		t.Fatalf("上限 1 的活動 total_issue_limit = %v", got.TotalIssueLimit)
	}
}

func TestSuspendedSellerIsRejectedBeforeAnyModuleCall(t *testing.T) {
	r := newRig(t)
	r.stores.suspended = true
	_, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: cashInput()}))
	reason, _ := reasonAndFields(err)
	if codeOf(err) != connect.CodePermissionDenied || reason != "SELLER_SUSPENDED" || len(r.module.calls) != 0 {
		t.Fatalf("停權：code=%v reason=%q calls=%v", codeOf(err), reason, r.module.calls)
	}
}

func TestCreateCampaignRejectsSelfDesignation(t *testing.T) {
	r := newRig(t)
	in := cashInput()
	in.TargetType = pb.TargetType_TARGET_TYPE_DESIGNATED
	in.DesignatedBuyerIds = []string{"9", "501"}
	_, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in}))
	_, fields := reasonAndFields(err)
	if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"campaign.designated_buyer_ids"}) || len(r.module.calls) != 0 {
		t.Fatalf("自己指定自己：code=%v fields=%v calls=%v", codeOf(err), fields, r.module.calls)
	}
	in.DesignatedBuyerIds = []string{"9", "x"}
	if _, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("買家 ID 格式錯誤：%v", err)
	}
}

func TestEndCampaignIsScopedToTheActingStore(t *testing.T) {
	r := newRig(t)
	r.module.detail = coupon.CampaignDetail{Campaign: campaign()}
	res, err := r.app.EndCampaign(signedIn(), connect.NewRequest(&pb.EndCampaignRequest{CampaignId: campID.String()}))
	if err != nil || r.module.endIn.StoreID != callerStoreID || r.module.endIn.CampaignID != campID || res.Msg.GetVoidedVouchers() != 3 {
		t.Fatalf("err=%v in=%+v voided=%d", err, r.module.endIn, res.Msg.GetVoidedVouchers())
	}
	if !slices.Equal(r.stores.asked, []seller.Capability{seller.CapContentContract}) {
		t.Fatalf("結束活動要的能力 = %v", r.stores.asked)
	}
}

// 顯示資料讀取失敗整個請求失敗，不降級成空資料（D3、BD28）。
func TestDisplayDataFailureFailsTheRequest(t *testing.T) {
	r := newRig(t)
	r.module.vouchers = coupon.VoucherPage{Vouchers: []coupon.VoucherView{{Voucher: coupon.Voucher{ID: voucherA, CreatedAt: now}, Campaign: campaign()}}}
	r.stores.cardsErr = errors.New("seller down")
	_, err := r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{}))
	if codeOf(err) != connect.CodeUnavailable {
		t.Fatalf("店家卡片讀取失敗應整個請求失敗（unavailable），得到 %v", err)
	}
	r.module.detail = coupon.CampaignDetail{Campaign: campaign(), TargetBuyers: []int64{9}}
	r.buyers.err = errors.New("buyer down")
	if _, err := r.app.GetCampaign(signedIn(), connect.NewRequest(&pb.GetCampaignRequest{CampaignId: campID.String()})); err == nil {
		t.Fatal("買家顯示資料讀取失敗應整個請求失敗")
	}
}

func TestListMyVouchersValidatesStatusesAndCheckout(t *testing.T) {
	r := newRig(t)
	_, err := r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{Statuses: []pb.VoucherStatus{pb.VoucherStatus_VOUCHER_STATUS_UNSPECIFIED}}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UNSPECIFIED 的 status：%v", err)
	}
	_, err = r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{PageSize: -1}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("負的 page_size：%v", err)
	}
	co := &pb.Checkout{StoreId: "11", OrderAmount: 300}
	_, err = r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{Checkout: co, Statuses: []pb.VoucherStatus{pb.VoucherStatus_VOUCHER_STATUS_REDEEMED}}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("checkout 加上非 AVAILABLE 的 status：%v", err)
	}
	if len(r.module.calls) != 0 {
		t.Fatalf("不合法的請求不能呼叫模組：%v", r.module.calls)
	}
	r.module.vouchers = coupon.VoucherPage{Vouchers: []coupon.VoucherView{{Voucher: coupon.Voucher{ID: voucherA, Status: coupon.VoucherAvailable, CreatedAt: now}, Campaign: campaign(), EstimatedDiscount: 50}}}
	res, err := r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{Checkout: co}))
	if err != nil || r.module.voucherIn.Checkout == nil || r.module.voucherIn.Checkout.OrderAmount != 300 || r.module.voucherIn.BuyerID != callerBuyerID {
		t.Fatalf("err=%v in=%+v", err, r.module.voucherIn)
	}
	if v := res.Msg.GetVouchers()[0]; v.EstimatedDiscount == nil || v.GetEstimatedDiscount() != 50 || v.GetStatus() != pb.VoucherStatus_VOUCHER_STATUS_AVAILABLE {
		t.Fatalf("voucher = %v", v)
	}
}

func TestPageTokenIsBoundToTheFilter(t *testing.T) {
	r := newRig(t)
	next := &coupon.Cursor{At: now, ID: voucherA}
	r.module.claimable = coupon.CampaignPage{Campaigns: []coupon.Campaign{campaign()}, Next: next}
	first, err := r.app.ListClaimableCampaigns(signedIn(), connect.NewRequest(&pb.ListClaimableCampaignsRequest{PageSize: 1}))
	if err != nil || first.Msg.GetNextPageToken() == "" {
		t.Fatalf("err=%v token=%q", err, first.Msg.GetNextPageToken())
	}
	store := "11"
	_, err = r.app.ListClaimableCampaigns(signedIn(), connect.NewRequest(&pb.ListClaimableCampaignsRequest{PageToken: first.Msg.GetNextPageToken(), StoreId: &store}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換了 store_id 還用舊 token 要回 invalid_argument：%v", err)
	}
	if _, err = r.app.ListClaimableCampaigns(signedIn(), connect.NewRequest(&pb.ListClaimableCampaignsRequest{PageToken: first.Msg.GetNextPageToken()})); err != nil {
		t.Fatal(err)
	}
	if r.module.claimableIn.After == nil || r.module.claimableIn.After.ID != voucherA {
		t.Fatalf("After = %+v", r.module.claimableIn.After)
	}
}

// 模組的業務原因對到契約的 ErrorReason；沒有對應的（找不到）不帶原因。
func TestClaimFailureCarriesTheContractReason(t *testing.T) {
	cases := map[coupon.Reason]string{
		coupon.ReasonIssueLimitReached: "COUPON_ISSUE_LIMIT_REACHED",
		coupon.ReasonStoreSuspended:    "COUPON_STORE_SUSPENDED",
		coupon.ReasonNotEligible:       "COUPON_NOT_ELIGIBLE",
		coupon.ReasonVoucherNotFound:   "",
	}
	for reason, want := range cases {
		r := newRig(t)
		r.app.reasonOf = func(error) coupon.Reason { return reason }
		r.module.err = apperr.FailedPrecondition("x")
		_, err := r.app.ClaimCoupon(signedIn(), connect.NewRequest(&pb.ClaimCouponRequest{CampaignId: campID.String()}))
		got, _ := reasonAndFields(err)
		if codeOf(err) != connect.CodeFailedPrecondition || got != want {
			t.Errorf("%s：code=%v reason=%q，預期 %q", reason, codeOf(err), got, want)
		}
	}
	r := newRig(t)
	if _, err := r.app.ClaimCoupon(signedIn(), connect.NewRequest(&pb.ClaimCouponRequest{CampaignId: "nope"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("campaign_id 不是 UUID：%v", err)
	}
}

func TestModuleFieldNamesMapToRequestPaths(t *testing.T) {
	desc := (&pb.CreateCampaignRequest{}).ProtoReflect().Descriptor()
	if missing := rpc.CheckFieldPaths(desc, createPaths); len(missing) != 0 {
		t.Fatalf("對應表的路徑在請求裡不存在：%v", missing)
	}
}

// 錯誤句子與 LocalizedMessage.locale 都依請求語系：en 的請求拿到 en 的句子，標記也是 en（不是寫死 zh-TW）；沒有 header 是 zh-TW。
func TestErrorLocaleFollowsTheRequestLocale(t *testing.T) {
	r := newRig(t)
	r.module.err = nil
	failing := &localizedModule{fakeModule: r.module}
	app, err := NewApp(failing, r.buyers, r.stores, r.app.images, r.app.now, r.app.log)
	if err != nil {
		t.Fatal(err)
	}
	opts := connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)))
	path, h := couponv1alpha1connect.NewCouponAppServiceHandler(app, opts)
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(withAccount(mux))
	defer srv.Close()
	client := couponv1alpha1connect.NewCouponAppServiceClient(srv.Client(), srv.URL)
	for lang, want := range map[string]string{"en": "en", "": "zh-TW", "zh-TW": "zh-TW"} {
		req := connect.NewRequest(&pb.ClaimCouponRequest{CampaignId: campID.String()})
		if lang != "" {
			req.Header().Set("Accept-Language", lang)
		}
		_, err := client.ClaimCoupon(context.Background(), req)
		var ce *connect.Error
		if !errors.As(err, &ce) {
			t.Fatalf("%q：預期 connect 錯誤：%v", lang, err)
		}
		var locale, message string
		for _, d := range ce.Details() {
			if v, e := d.Value(); e == nil {
				if m, ok := v.(*errdetails.LocalizedMessage); ok {
					locale, message = m.GetLocale(), m.GetMessage()
				}
			}
		}
		if locale != want || (want == "en") != strings.HasPrefix(message, "EN:") {
			t.Errorf("Accept-Language %q：locale=%q message=%q，預期 locale %q", lang, locale, message, want)
		}
	}
}

// localizedModule 的 Claim 依 ctx 的語系產生句子，語系是實際使用的語系（同模組的 model.Precondition）。
type localizedModule struct{ *fakeModule }

func (m *localizedModule) Claim(ctx context.Context, _ uuid.UUID, _ int64) (coupon.ClaimResult, error) {
	loc := uitext.LocaleFrom(ctx)
	text := "活動已結束"
	if loc == uitext.EN {
		text = "EN: campaign ended"
	}
	return coupon.ClaimResult{}, apperr.FailedPrecondition("%s", uitext.Message{Text: text, Locale: loc})
}

// withAccount 把測試帳號放進請求（模擬入口的 session 中介層）。
func withAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})))
	})
}

// 格式錯誤的 ID 回 invalid_argument 並指出欄位，不安靜變成 0 或空值（BD14、BD24），也不呼叫模組。
func TestMalformedIDsAreRejectedNotSwallowed(t *testing.T) {
	r := newRig(t)
	bad := "x"
	_, err := r.app.ListClaimableCampaigns(signedIn(), connect.NewRequest(&pb.ListClaimableCampaignsRequest{StoreId: &bad}))
	_, fields := reasonAndFields(err)
	if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"store_id"}) {
		t.Fatalf("store_id：code=%v fields=%v", codeOf(err), fields)
	}
	_, err = r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{Checkout: &pb.Checkout{StoreId: bad, OrderAmount: 100}}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("checkout.store_id：%v", err)
	}
	if len(r.module.calls) != 0 {
		t.Fatalf("不能呼叫模組：%v", r.module.calls)
	}
}
