package coupon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
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

// fakeCustomers 是店家的客戶：customers 之外的買家回 NotFound；err 非空時讀取失敗。
type fakeCustomers struct {
	customers map[int64]bool
	err       error
	asked     []int64
	calls     int
}

func (f *fakeCustomers) CustomersAmong(_ context.Context, storeID int64, ids []int64) (map[int64]bool, error) {
	f.calls++
	f.asked = append(f.asked, ids...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64]bool{}
	for _, id := range ids {
		if storeID == callerStoreID && f.customers[id] {
			out[id] = true
		}
	}
	return out, nil
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
	custs  *fakeCustomers
	stores *fakeStores
}

func newRig(t *testing.T) *rig {
	t.Helper()
	im, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{module: &fakeModule{}, buyers: &fakeBuyers{}, custs: &fakeCustomers{customers: map[int64]bool{9: true, 10: true}}, stores: &fakeStores{}}
	r.app, err = NewApp(r.module, r.buyers, r.custs, r.stores, im, slog.New(slog.DiscardHandler))
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
	if _, err := newApp(missing, r.module, r.buyers, r.custs, r.stores, r.app.images, r.app.log); err == nil {
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
	if got := toSellerCampaign(c, false); got.TotalIssueLimit != nil {
		t.Fatalf("無上限的活動 total_issue_limit 應不設，得到 %d", got.GetTotalIssueLimit())
	}
	limit := int32(1)
	c.TotalLimit = &limit
	if got := toSellerCampaign(c, false); got.TotalIssueLimit == nil || got.GetTotalIssueLimit() != 1 {
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
	r.module.detail = coupon.CampaignDetail{SellerCampaign: coupon.SellerCampaign{Campaign: campaign()}}
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
	r.module.detail = coupon.CampaignDetail{SellerCampaign: coupon.SellerCampaign{Campaign: campaign()}, TargetBuyers: []int64{9}}
	r.buyers.err = errors.New("buyer down")
	if _, err := r.app.GetCampaign(signedIn(), connect.NewRequest(&pb.GetCampaignRequest{CampaignId: campID.String()})); codeOf(err) != connect.CodeUnavailable {
		t.Fatalf("買家顯示資料讀取失敗應整個請求失敗（unavailable，與店家卡片同一種分類），得到 %v", err)
	}
}

// 名單裡的買家：buyer 標為已刪除的不設名稱；buyer 模組沒有回的買家是資料不一致，整個請求失敗，不補成空名稱。
func TestDesignatedBuyersDeletedIsEmptyNameAndMissingFailsTheRequest(t *testing.T) {
	r := newRig(t)
	name := "小明"
	r.module.detail = coupon.CampaignDetail{SellerCampaign: coupon.SellerCampaign{Campaign: campaign()}, TargetBuyers: []int64{9, 10}}
	r.buyers.displays = []buyer.Display{{BuyerID: 9, DisplayName: &name}, {BuyerID: 10, Deleted: true}}
	res, err := r.app.GetCampaign(signedIn(), connect.NewRequest(&pb.GetCampaignRequest{CampaignId: campID.String()}))
	if err != nil || len(res.Msg.GetDesignatedBuyers()) != 2 {
		t.Fatalf("err=%v %v", err, res.Msg.GetDesignatedBuyers())
	}
	if b := res.Msg.GetDesignatedBuyers(); b[0].GetName() != "小明" || b[1].GetName() != "" {
		t.Fatalf("名稱 = %q / %q，預期 小明 與 空（已刪除）", b[0].GetName(), b[1].GetName())
	}
	r.buyers.displays = []buyer.Display{{BuyerID: 9, DisplayName: &name}}
	_, err = r.app.GetCampaign(signedIn(), connect.NewRequest(&pb.GetCampaignRequest{CampaignId: campID.String()}))
	if err == nil || codeOf(err) != connect.CodeInternal {
		t.Fatalf("buyer 沒有回名單裡的買家要整個請求失敗（internal）：%v", err)
	}
}

// expired 取自模組（它的時鐘），handler 不另外判斷。
func TestSellerCampaignExpiredComesFromTheModule(t *testing.T) {
	r := newRig(t)
	r.module.list = coupon.SellerCampaignPage{Campaigns: []coupon.SellerCampaign{{Campaign: campaign(), Expired: true}}}
	r.module.detail = coupon.CampaignDetail{SellerCampaign: coupon.SellerCampaign{Campaign: campaign(), Expired: true}}
	list, err := r.app.ListCampaigns(signedIn(), connect.NewRequest(&pb.ListCampaignsRequest{}))
	if err != nil || !list.Msg.GetCampaigns()[0].GetExpired() {
		t.Fatalf("列表：err=%v %v", err, list.Msg.GetCampaigns())
	}
	got, err := r.app.GetCampaign(signedIn(), connect.NewRequest(&pb.GetCampaignRequest{CampaignId: campID.String()}))
	if err != nil || !got.Msg.GetCampaign().GetExpired() {
		t.Fatalf("明細：err=%v %v", err, got.Msg.GetCampaign())
	}
	r.module.list = coupon.SellerCampaignPage{Campaigns: []coupon.SellerCampaign{{Campaign: campaign()}}}
	list, err = r.app.ListCampaigns(signedIn(), connect.NewRequest(&pb.ListCampaignsRequest{}))
	if err != nil || list.Msg.GetCampaigns()[0].GetExpired() {
		t.Fatalf("模組說沒到期就不到期：err=%v %v", err, list.Msg.GetCampaigns())
	}
}

// MEMBER 活動可以建立：入口不再擋，資格由模組以消費紀錄投影判定；傳給模組的是 MEMBER。
func TestCreateCampaignAcceptsMemberTargetType(t *testing.T) {
	r := newRig(t)
	r.module.created = campaign()
	in := cashInput()
	in.TargetType = pb.TargetType_TARGET_TYPE_MEMBER
	if _, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in})); err != nil {
		t.Fatal(err)
	}
	if r.module.createIn.TargetType != coupon.TargetMember || !slices.Equal(r.module.calls, []string{"CreateCampaign"}) {
		t.Fatalf("target=%q calls=%v", r.module.createIn.TargetType, r.module.calls)
	}
}

// 指定發放的名單只收這家店的客戶：不是客戶的回 invalid_argument 並指出第幾個，不呼叫模組；crm 讀取失敗整個失敗。
func TestCreateCampaignDesignatedListMustBeStoreCustomers(t *testing.T) {
	r := newRig(t)
	r.module.created = campaign()
	in := cashInput()
	in.TargetType = pb.TargetType_TARGET_TYPE_DESIGNATED
	in.DesignatedBuyerIds = []string{"9", "77"}
	_, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in}))
	_, fields := reasonAndFields(err)
	if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"campaign.designated_buyer_ids[1]"}) || len(r.module.calls) != 0 {
		t.Fatalf("非客戶：code=%v fields=%v calls=%v", codeOf(err), fields, r.module.calls)
	}
	in.DesignatedBuyerIds = []string{"9", "10"}
	if _, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in})); err != nil || !slices.Equal(r.module.createIn.TargetBuyers, []int64{9, 10}) {
		t.Fatalf("全是客戶：err=%v in=%+v", err, r.module.createIn)
	}
	r.module.calls = nil
	r.custs.err = errors.New("crm down")
	if _, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in})); err == nil || codeOf(err) == connect.CodeInvalidArgument || len(r.module.calls) != 0 {
		t.Fatalf("crm 讀取失敗要整個請求失敗（不是 invalid_argument）：%v calls=%v", err, r.module.calls)
	}
}

// 指定名單超過上限在入口擋下：invalid_argument 在 designated_buyer_ids，crm 與模組都沒被呼叫；剛好上限的名單一次 crm 查詢。
func TestCreateCampaignDesignatedListAboveTheCapMakesNoCalls(t *testing.T) {
	r := newRig(t)
	in := cashInput()
	in.TargetType = pb.TargetType_TARGET_TYPE_DESIGNATED
	for i := 0; i <= coupon.MaxTargetBuyers; i++ {
		in.DesignatedBuyerIds = append(in.DesignatedBuyerIds, strconv.Itoa(1000+i))
	}
	_, err := r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in}))
	_, fields := reasonAndFields(err)
	if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"campaign.designated_buyer_ids"}) || r.custs.calls != 0 || len(r.custs.asked) != 0 || len(r.module.calls) != 0 {
		t.Fatalf("超過上限：code=%v fields=%v crm=%d module=%v", codeOf(err), fields, r.custs.calls, r.module.calls)
	}
	in.DesignatedBuyerIds = in.DesignatedBuyerIds[:coupon.MaxTargetBuyers]
	_, _ = r.app.CreateCampaign(signedIn(), connect.NewRequest(&pb.CreateCampaignRequest{Campaign: in}))
	if r.custs.calls != 1 {
		t.Fatalf("上限內的名單要一次 crm 查詢，實際 %d 次", r.custs.calls)
	}
}

// 結帳的 order_amount 超過上限：錢包空與不空都回同一個 invalid_argument，欄位是 checkout.order_amount，模組沒被呼叫。
func TestListMyVouchersRejectsOrderAmountAboveTheMaximum(t *testing.T) {
	for name, wallet := range map[string]coupon.VoucherPage{
		"empty wallet": {},
		"has vouchers": {Vouchers: []coupon.VoucherView{{Voucher: coupon.Voucher{ID: voucherA, CreatedAt: now}, Campaign: campaign()}}},
	} {
		r := newRig(t)
		r.module.vouchers = wallet
		co := &pb.Checkout{StoreId: "11", OrderAmount: coupon.MaxOrderAmount + 1}
		_, err := r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{Checkout: co}))
		_, fields := reasonAndFields(err)
		if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"checkout.order_amount"}) || len(r.module.calls) != 0 {
			t.Errorf("%s：code=%v fields=%v calls=%v", name, codeOf(err), fields, r.module.calls)
		}
		co.OrderAmount = coupon.MaxOrderAmount
		if _, err := r.app.ListMyVouchers(signedIn(), connect.NewRequest(&pb.ListMyVouchersRequest{Checkout: co})); err != nil {
			t.Errorf("%s：上限本身要接受：%v", name, err)
		}
	}
}

// 賣家的列表與明細宣告收縮類能力：停權的店家仍能找出活動 ID 並結束它。
func TestSellerReadsDeclareTheContractionCapability(t *testing.T) {
	r := newRig(t)
	for _, p := range []string{couponv1alpha1connect.CouponAppServiceListCampaignsProcedure, couponv1alpha1connect.CouponAppServiceGetCampaignProcedure, couponv1alpha1connect.CouponAppServiceEndCampaignProcedure} {
		if _, err := r.app.actingStore(signedIn(), p); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(r.stores.asked, []seller.Capability{seller.CapContentContract, seller.CapContentContract, seller.CapContentContract}) {
		t.Fatalf("向 seller 要的能力 = %v，預期三個都是 CAP_CONTENT_CONTRACT", r.stores.asked)
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
	app, err := NewApp(failing, r.buyers, r.custs, r.stores, r.app.images, r.app.log)
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
