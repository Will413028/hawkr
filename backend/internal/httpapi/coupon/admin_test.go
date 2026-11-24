package coupon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1/couponv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const callerAdmin = int64(77)

// fakeAdminModule 記下每支方法收到的輸入與 actor。
type fakeAdminModule struct {
	err   error
	calls []string

	actor      adminaudit.Actor
	createIn   coupon.CreateCampaignInput
	endIn      coupon.EndCampaignInput
	issueIn    coupon.IssueInput
	voidIn     coupon.VoidVoucherInput
	quotaStore int64
	quotaMax   int32
	listIn     coupon.AdminListInput
	vouchersIn coupon.VoucherListInput

	created coupon.Campaign
	detail  coupon.CampaignDetail
	list    coupon.AdminCampaignPage
	page    coupon.VoucherPage
}

func (f *fakeAdminModule) AdminCreateCampaign(_ context.Context, a adminaudit.Actor, in coupon.CreateCampaignInput) (coupon.Campaign, error) {
	f.calls, f.actor, f.createIn = append(f.calls, "Create"), a, in
	return f.created, f.err
}

func (f *fakeAdminModule) AdminPublishCampaign(_ context.Context, a adminaudit.Actor, _ uuid.UUID) error {
	f.calls, f.actor = append(f.calls, "Publish"), a
	return f.err
}

func (f *fakeAdminModule) AdminEndCampaign(_ context.Context, a adminaudit.Actor, in coupon.EndCampaignInput) (coupon.EndCampaignResult, error) {
	f.calls, f.actor, f.endIn = append(f.calls, "End"), a, in
	return coupon.EndCampaignResult{Voided: 4}, f.err
}

func (f *fakeAdminModule) AdminIssue(_ context.Context, a adminaudit.Actor, in coupon.IssueInput) (coupon.IssueResult, error) {
	f.calls, f.actor, f.issueIn = append(f.calls, "Issue"), a, in
	return coupon.IssueResult{Issued: 2, AlreadyHeld: 1}, f.err
}

func (f *fakeAdminModule) AdminVoidVoucher(_ context.Context, a adminaudit.Actor, in coupon.VoidVoucherInput) error {
	f.calls, f.actor, f.voidIn = append(f.calls, "Void"), a, in
	return f.err
}

func (f *fakeAdminModule) AdminSetSellerQuota(_ context.Context, a adminaudit.Actor, storeID int64, maxCount int32) error {
	f.calls, f.actor, f.quotaStore, f.quotaMax = append(f.calls, "Quota"), a, storeID, maxCount
	return f.err
}

func (f *fakeAdminModule) AdminListCampaigns(_ context.Context, in coupon.AdminListInput) (coupon.AdminCampaignPage, error) {
	f.calls, f.listIn = append(f.calls, "List"), in
	return f.list, f.err
}

func (f *fakeAdminModule) AdminGetCampaign(context.Context, uuid.UUID) (coupon.CampaignDetail, error) {
	f.calls = append(f.calls, "Get")
	return f.detail, f.err
}

func (f *fakeAdminModule) ListVouchers(_ context.Context, in coupon.VoucherListInput) (coupon.VoucherPage, error) {
	f.calls, f.vouchersIn = append(f.calls, "ListVouchers"), in
	return f.page, f.err
}

type adminRig struct {
	admin  *Admin
	module *fakeAdminModule
	client couponv1alpha1connect.CouponAdminServiceClient
}

func adminAs(p identity.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), p)))
		})
	}
}

// newAdminRig 把 Admin 掛在真的 Connect handler 後面（req.Spec().Procedure 才有值），呼叫者由 principal 決定。
func newAdminRig(t *testing.T, p identity.Principal) *adminRig {
	t.Helper()
	m := &fakeAdminModule{}
	a := NewAdmin(m)
	opts := connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)))
	path, h := couponv1alpha1connect.NewCouponAdminServiceHandler(a, opts)
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(adminAs(p)(mux))
	t.Cleanup(srv.Close)
	return &adminRig{admin: a, module: m, client: couponv1alpha1connect.NewCouponAdminServiceClient(srv.Client(), srv.URL)}
}

func adminPrincipal() identity.Principal {
	return identity.Principal{Kind: identity.PrincipalAdmin, ID: callerAdmin}
}

func adminCashInput() *adminpb.AdminCampaignInput {
	return &adminpb.AdminCampaignInput{
		Title: "平台券", Discount: &adminpb.AdminCampaignInput_Cash{Cash: &pb.CashDiscount{Amount: 50}}, ThresholdAmount: 200,
		TargetType: pb.TargetType_TARGET_TYPE_ALL, StartTime: timestamppb.New(now), EndTime: timestamppb.New(now.Add(24 * time.Hour)),
	}
}

func platformCampaign() coupon.Campaign {
	c := campaign()
	c.StoreID, c.IssuerStoreID = nil, nil
	admin := callerAdmin
	c.IssuerAdminID, c.Status = &admin, coupon.StatusDraft
	return c
}

func campaignIDString() string { return campID.String() }

// 九支方法每一支：缺能力的呼叫者（顧客帳號）被擋下，模組一次都沒有被呼叫。
func TestAdminMethodsRejectNonAdminCallersBeforeAnyModuleCall(t *testing.T) {
	r := newAdminRig(t, identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
	ctx := context.Background()
	id, voucher := campaignIDString(), voucherA.String()
	calls := map[string]func() error{
		"CreateCampaign": func() error {
			_, e := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: adminCashInput()}))
			return e
		},
		"PublishCampaign": func() error {
			_, e := r.client.PublishCampaign(ctx, connect.NewRequest(&adminpb.PublishCampaignRequest{CampaignId: id}))
			return e
		},
		"EndCampaign": func() error {
			_, e := r.client.EndCampaign(ctx, connect.NewRequest(&adminpb.EndCampaignRequest{CampaignId: id}))
			return e
		},
		"GrantVouchers": func() error {
			_, e := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: id, BuyerIds: []string{"9"}}))
			return e
		},
		"VoidVoucher": func() error {
			_, e := r.client.VoidVoucher(ctx, connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: voucher, Reason: "x"}))
			return e
		},
		"SetSellerQuota": func() error {
			_, e := r.client.SetSellerQuota(ctx, connect.NewRequest(&adminpb.SetSellerQuotaRequest{StoreId: "11", MaxActiveCampaigns: 3}))
			return e
		},
		"ListCampaigns": func() error {
			_, e := r.client.ListCampaigns(ctx, connect.NewRequest(&adminpb.ListCampaignsRequest{}))
			return e
		},
		"GetCampaign": func() error {
			_, e := r.client.GetCampaign(ctx, connect.NewRequest(&adminpb.GetCampaignRequest{CampaignId: id}))
			return e
		},
		"ListBuyerVouchers": func() error {
			_, e := r.client.ListBuyerVouchers(ctx, connect.NewRequest(&adminpb.ListBuyerVouchersRequest{BuyerId: "9"}))
			return e
		},
	}
	if len(calls) != couponAdminMethodCount() {
		t.Fatalf("測了 %d 支方法，service 有 %d 支", len(calls), couponAdminMethodCount())
	}
	for name, call := range calls {
		if err := call(); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s：顧客帳號呼叫回 %v，預期 permission_denied", name, err)
		}
	}
	if len(r.module.calls) != 0 {
		t.Fatalf("被擋下的呼叫不能碰模組：%v", r.module.calls)
	}
}

func couponAdminMethodCount() int {
	return adminpb.File_hawkr_admin_coupon_v1alpha1_coupon_proto.Services().ByName("CouponAdminService").Methods().Len()
}

// 寫入方法把 actor（管理員 ID 與 procedure 名稱）交給模組，由模組在同一個 transaction 寫後台紀錄；管理員身分不來自請求。
func TestAdminWritesCarryTheActorToTheModule(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()
	r.module.created = platformCampaign()
	id := campaignIDString()
	for procedure, call := range map[string]func() error{
		couponv1alpha1connect.CouponAdminServiceCreateCampaignProcedure: func() error {
			_, e := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: adminCashInput()}))
			return e
		},
		couponv1alpha1connect.CouponAdminServicePublishCampaignProcedure: func() error {
			_, e := r.client.PublishCampaign(ctx, connect.NewRequest(&adminpb.PublishCampaignRequest{CampaignId: id}))
			return e
		},
		couponv1alpha1connect.CouponAdminServiceEndCampaignProcedure: func() error {
			_, e := r.client.EndCampaign(ctx, connect.NewRequest(&adminpb.EndCampaignRequest{CampaignId: id, Reason: "  收尾  "}))
			return e
		},
		couponv1alpha1connect.CouponAdminServiceGrantVouchersProcedure: func() error {
			_, e := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: id, BuyerIds: []string{"9", "10"}}))
			return e
		},
		couponv1alpha1connect.CouponAdminServiceVoidVoucherProcedure: func() error {
			_, e := r.client.VoidVoucher(ctx, connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: voucherA.String(), Reason: " 卡住 "}))
			return e
		},
		couponv1alpha1connect.CouponAdminServiceSetSellerQuotaProcedure: func() error {
			_, e := r.client.SetSellerQuota(ctx, connect.NewRequest(&adminpb.SetSellerQuotaRequest{StoreId: "11", MaxActiveCampaigns: 8}))
			return e
		},
	} {
		r.module.actor = adminaudit.Actor{}
		if err := call(); err != nil {
			t.Fatalf("%s：%v", procedure, err)
		}
		if want := (adminaudit.Actor{AdminID: callerAdmin, Action: procedure}); r.module.actor != want {
			t.Errorf("%s 交給模組的 actor = %+v，預期 %+v", procedure, r.module.actor, want)
		}
	}
	if r.module.voidIn.Reason != "卡住" || r.module.voidIn.VoucherID != voucherA {
		t.Errorf("作廢的輸入 = %+v，預期去空白的理由與券 ID", r.module.voidIn)
	}
	if !slices.Equal(r.module.issueIn.BuyerIDs, []int64{9, 10}) || r.module.issueIn.CampaignID != campID {
		t.Errorf("發放的輸入 = %+v", r.module.issueIn)
	}
	if r.module.endIn.CampaignID != campID || r.module.endIn.AdminID != 0 || r.module.endIn.StoreID != 0 {
		t.Errorf("結束的輸入 = %+v：身分與店家範圍由模組依 actor 填，請求不帶", r.module.endIn)
	}
	if r.module.quotaStore != 11 || r.module.quotaMax != 8 {
		t.Errorf("配額 = %d/%d，預期 11/8", r.module.quotaStore, r.module.quotaMax)
	}
}

// 無發放上限與上限 0 是兩回事：不設送給模組 0（模組的無上限），設 0 在入口擋下、不碰模組，設正值照傳；回應的無上限不設欄位。
func TestAdminCreateCampaignDistinguishesNoLimitFromZero(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()
	r.module.created = platformCampaign()
	in := adminCashInput()
	res, err := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in}))
	if err != nil || r.module.createIn.TotalLimit != 0 {
		t.Fatalf("不設上限：err=%v 傳給模組 %d，預期 0（無上限）", err, r.module.createIn.TotalLimit)
	}
	if res.Msg.GetCampaign().GetCampaign().TotalIssueLimit != nil {
		t.Fatalf("無上限的活動回應 total_issue_limit 應不設，得到 %d", res.Msg.GetCampaign().GetCampaign().GetTotalIssueLimit())
	}
	zero := int32(0)
	in.TotalIssueLimit = &zero
	r.module.calls = nil
	_, err = r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || len(r.module.calls) != 0 {
		t.Fatalf("上限 0 要在入口擋下且不呼叫模組：err=%v calls=%v", err, r.module.calls)
	}
	five := int32(5)
	in.TotalIssueLimit = &five
	if _, err = r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in})); err != nil || r.module.createIn.TotalLimit != 5 {
		t.Fatalf("上限 5：err=%v 傳給模組 %d", err, r.module.createIn.TotalLimit)
	}
	limited := platformCampaign()
	limit := int32(5)
	limited.TotalLimit = &limit
	if got := toAdminCampaign(limited, false).GetCampaign().TotalIssueLimit; got == nil || *got != 5 {
		t.Fatalf("上限 5 的活動回應 = %v", got)
	}
}

// 建立只接受平台活動的合法輸入：MEMBER 暫時不能建、折抵必須二選一、時間必填、店家 ID 與名單的格式錯誤指出欄位，全部不呼叫模組。
func TestAdminCreateCampaignRejectsBadInputWithoutCallingTheModule(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()
	for name, mod := range map[string]func(*adminpb.AdminCampaignInput){
		"MEMBER":     func(in *adminpb.AdminCampaignInput) { in.TargetType = pb.TargetType_TARGET_TYPE_MEMBER },
		"沒有折抵":       func(in *adminpb.AdminCampaignInput) { in.Discount = nil },
		"沒有開始時間":     func(in *adminpb.AdminCampaignInput) { in.StartTime = nil },
		"沒有結束時間":     func(in *adminpb.AdminCampaignInput) { in.EndTime = nil },
		"店家 ID 格式錯誤": func(in *adminpb.AdminCampaignInput) { bad := "x"; in.StoreId = &bad },
		"名單格式錯誤":     func(in *adminpb.AdminCampaignInput) { in.DesignatedBuyerIds = []string{"9", "x"} },
		"名單過長": func(in *adminpb.AdminCampaignInput) {
			for range coupon.MaxTargetBuyers + 1 {
				in.DesignatedBuyerIds = append(in.DesignatedBuyerIds, "9")
			}
		},
	} {
		in := adminCashInput()
		mod(in)
		_, err := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：err=%v，預期 invalid_argument", name, err)
		}
	}
	if _, err := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("沒有 campaign：%v", err)
	}
	if len(r.module.calls) != 0 {
		t.Fatalf("不能呼叫模組：%v", r.module.calls)
	}
	in := adminCashInput()
	store := "11"
	in.StoreId, in.TargetType, in.DesignatedBuyerIds = &store, pb.TargetType_TARGET_TYPE_DESIGNATED, []string{"9"}
	r.module.created = platformCampaign()
	if _, err := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in})); err != nil {
		t.Fatal(err)
	}
	got := r.module.createIn
	if got.StoreID != 11 || got.TargetType != coupon.TargetDesignated || !slices.Equal(got.TargetBuyers, []int64{9}) || got.Title != "平台券" || got.DiscountAmount != 50 || got.ThresholdAmount != 200 || got.Issuer != (coupon.Issuer{}) {
		t.Fatalf("交給模組的輸入 = %+v", got)
	}
}

func TestAdminWriteInputValidation(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()
	id, voucher := campaignIDString(), voucherA.String()
	for name, call := range map[string]func() error{
		"作廢沒有理由": func() error {
			_, e := r.client.VoidVoucher(ctx, connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: voucher}))
			return e
		},
		"作廢理由只有空白": func() error {
			_, e := r.client.VoidVoucher(ctx, connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: voucher, Reason: " \t "}))
			return e
		},
		"券 ID 格式錯誤": func() error {
			_, e := r.client.VoidVoucher(ctx, connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: "x", Reason: "r"}))
			return e
		},
		"活動 ID 格式錯誤": func() error {
			_, e := r.client.PublishCampaign(ctx, connect.NewRequest(&adminpb.PublishCampaignRequest{CampaignId: "x"}))
			return e
		},
		"結束的活動 ID 格式錯誤": func() error {
			_, e := r.client.EndCampaign(ctx, connect.NewRequest(&adminpb.EndCampaignRequest{CampaignId: "x"}))
			return e
		},
		"發放沒有買家": func() error {
			_, e := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: id}))
			return e
		},
		"發放買家 ID 格式錯誤": func() error {
			_, e := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: id, BuyerIds: []string{"9", "0"}}))
			return e
		},
		"配額 0": func() error {
			_, e := r.client.SetSellerQuota(ctx, connect.NewRequest(&adminpb.SetSellerQuotaRequest{StoreId: "11"}))
			return e
		},
		"配額負數": func() error {
			_, e := r.client.SetSellerQuota(ctx, connect.NewRequest(&adminpb.SetSellerQuotaRequest{StoreId: "11", MaxActiveCampaigns: -1}))
			return e
		},
		"配額店家 ID 格式錯誤": func() error {
			_, e := r.client.SetSellerQuota(ctx, connect.NewRequest(&adminpb.SetSellerQuotaRequest{StoreId: "x", MaxActiveCampaigns: 1}))
			return e
		},
	} {
		if err := call(); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：err=%v，預期 invalid_argument", name, err)
		}
	}
	if len(r.module.calls) != 0 {
		t.Fatalf("不能呼叫模組：%v", r.module.calls)
	}
}

// 列表依狀態與發行者篩選：條件交給模組；明確送出 UNSPECIFIED 的狀態、沒設內容的 issuer、格式錯誤的店家 ID 都被拒絕；
// page token 只能用在同一組篩選。
func TestAdminListCampaignsFiltersAndBindsThePageTokenToThem(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()
	r.module.list = coupon.AdminCampaignPage{
		Campaigns: []coupon.SellerCampaign{{Campaign: platformCampaign(), Expired: true}, {Campaign: campaign()}},
		Next:      &coupon.Cursor{At: now, ID: voucherA},
	}
	ended := pb.CampaignStatus_CAMPAIGN_STATUS_ENDED
	platform := &adminpb.IssuerFilter{Issuer: &adminpb.IssuerFilter_Platform{Platform: &adminpb.PlatformIssuer{}}}
	res, err := r.client.ListCampaigns(ctx, connect.NewRequest(&adminpb.ListCampaignsRequest{PageSize: 2, Status: &ended, Issuer: platform}))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.module.listIn; got.Status != coupon.StatusEnded || !got.PlatformOnly || got.StoreID != 0 || got.Size != 2 {
		t.Fatalf("交給模組的篩選 = %+v", got)
	}
	cs := res.Msg.GetCampaigns()
	if len(cs) != 2 || cs[0].GetIssuerAdminId() != "77" || !cs[0].GetCampaign().GetExpired() || cs[1].GetIssuerStoreId() != "11" || cs[1].GetIssuerAdminId() != "" {
		t.Fatalf("回應的活動 = %+v，預期第一個是管理員 77 發行且 expired、第二個是店家 11", cs)
	}
	token := res.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("有下一頁要回 token")
	}

	storeFilter := &adminpb.IssuerFilter{Issuer: &adminpb.IssuerFilter_StoreId{StoreId: "11"}}
	if _, err = r.client.ListCampaigns(ctx, connect.NewRequest(&adminpb.ListCampaignsRequest{Issuer: storeFilter})); err != nil {
		t.Fatal(err)
	}
	if got := r.module.listIn; got.PlatformOnly || got.StoreID != 11 || got.Status != "" {
		t.Fatalf("店家篩選交給模組的條件 = %+v", got)
	}
	if _, err = r.client.ListCampaigns(ctx, connect.NewRequest(&adminpb.ListCampaignsRequest{PageSize: 2, Status: &ended, Issuer: platform, PageToken: token})); err != nil {
		t.Fatalf("同一組篩選用 token：%v", err)
	}
	if r.module.listIn.After == nil || r.module.listIn.After.ID != voucherA {
		t.Fatalf("After = %+v", r.module.listIn.After)
	}
	active := pb.CampaignStatus_CAMPAIGN_STATUS_ACTIVE
	unspecified := pb.CampaignStatus_CAMPAIGN_STATUS_UNSPECIFIED
	badStore := &adminpb.IssuerFilter{Issuer: &adminpb.IssuerFilter_StoreId{StoreId: "x"}}
	r.module.calls = nil
	for name, req := range map[string]*adminpb.ListCampaignsRequest{
		"換了狀態用舊 token":     {Status: &active, Issuer: platform, PageToken: token},
		"換了發行者用舊 token":    {Status: &ended, Issuer: storeFilter, PageToken: token},
		"拿掉篩選用舊 token":     {PageToken: token},
		"明確送出 UNSPECIFIED": {Status: &unspecified},
		"issuer 沒有內容":      {Issuer: &adminpb.IssuerFilter{}},
		"店家 ID 格式錯誤":       {Issuer: badStore},
		"page_size 負數":     {PageSize: -1},
	} {
		if _, err := r.client.ListCampaigns(ctx, connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：err=%v，預期 invalid_argument", name, err)
		}
	}
	if len(r.module.calls) != 0 {
		t.Fatalf("被拒絕的請求不能碰模組：%v", r.module.calls)
	}
}

func TestAdminGetCampaignShowsIssuedLimitStatusAndCounts(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	c := campaign()
	limit := int32(10)
	c.TotalLimit, c.IssuedCount, c.Status, c.TargetType = &limit, 4, coupon.StatusActive, coupon.TargetDesignated
	r.module.detail = coupon.CampaignDetail{
		SellerCampaign: coupon.SellerCampaign{Campaign: c}, TargetBuyers: []int64{9, 10}, Available: 1, Locked: 2, Redeemed: 3, Voided: 4,
	}
	res, err := r.client.GetCampaign(context.Background(), connect.NewRequest(&adminpb.GetCampaignRequest{CampaignId: campaignIDString()}))
	if err != nil {
		t.Fatal(err)
	}
	m := res.Msg
	sc := m.GetCampaign().GetCampaign()
	if sc.GetIssuedCount() != 4 || sc.GetTotalIssueLimit() != 10 || sc.GetStatus() != pb.CampaignStatus_CAMPAIGN_STATUS_ACTIVE || m.GetCampaign().GetIssuerStoreId() != "11" {
		t.Fatalf("活動 = %+v", m.GetCampaign())
	}
	if !slices.Equal(m.GetDesignatedBuyerIds(), []string{"9", "10"}) || m.GetAvailableCount() != 1 || m.GetLockedCount() != 2 || m.GetRedeemedCount() != 3 || m.GetVoidedCount() != 4 {
		t.Fatalf("名單與券數 = %+v", m)
	}
	if _, err := r.client.GetCampaign(context.Background(), connect.NewRequest(&adminpb.GetCampaignRequest{CampaignId: "x"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("格式錯誤的 ID：%v", err)
	}
}

func TestAdminListBuyerVouchersMapsEveryVoucherField(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	locked, redeemed, voided := now.Add(time.Minute), now.Add(2*time.Minute), now.Add(3*time.Minute)
	v := coupon.Voucher{
		ID: voucherA, CampaignID: campID, BuyerID: 9, Origin: coupon.OriginGrant, Status: coupon.VoucherLocked, OrderNo: "ORD-1",
		LockedAt: &locked, RedeemedAt: &redeemed, VoidedAt: &voided, CreatedAt: now,
	}
	r.module.page = coupon.VoucherPage{Vouchers: []coupon.VoucherView{{Voucher: v, Campaign: campaign(), Expired: true}}, Next: &coupon.Cursor{At: now, ID: voucherA}}
	locked2 := pb.VoucherStatus_VOUCHER_STATUS_LOCKED
	res, err := r.client.ListBuyerVouchers(context.Background(), connect.NewRequest(&adminpb.ListBuyerVouchersRequest{
		BuyerId: "9", PageSize: 5, Statuses: []pb.VoucherStatus{locked2, locked2, pb.VoucherStatus_VOUCHER_STATUS_AVAILABLE},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.module.vouchersIn; got.BuyerID != 9 || got.Size != 5 || got.Checkout != nil ||
		!slices.Equal(got.Statuses, []coupon.VoucherStatus{coupon.VoucherAvailable, coupon.VoucherLocked}) {
		t.Fatalf("交給模組的輸入 = %+v", got)
	}
	out := res.Msg.GetVouchers()
	if len(out) != 1 || res.Msg.GetNextPageToken() == "" {
		t.Fatalf("回應 = %+v", res.Msg)
	}
	g := out[0]
	if g.GetId() != voucherA.String() || g.GetCampaign().GetId() != campID.String() || g.GetBuyerId() != "9" ||
		g.GetStatus() != pb.VoucherStatus_VOUCHER_STATUS_LOCKED || g.GetOrigin() != pb.VoucherOrigin_VOUCHER_ORIGIN_GRANT || !g.GetExpired() ||
		g.GetOrderNo() != "ORD-1" || !g.GetLockedTime().AsTime().Equal(locked) || !g.GetRedeemedTime().AsTime().Equal(redeemed) ||
		!g.GetVoidedTime().AsTime().Equal(voided) || !g.GetCreateTime().AsTime().Equal(now) {
		t.Fatalf("券 = %+v", g)
	}
	plain := toAdminVoucher(coupon.VoucherView{Voucher: coupon.Voucher{ID: voucherA, BuyerID: 9, Status: coupon.VoucherAvailable, Origin: coupon.OriginClaim, CreatedAt: now}, Campaign: campaign()})
	if plain.OrderNo != nil || plain.LockedTime != nil || plain.RedeemedTime != nil || plain.VoidedTime != nil {
		t.Fatalf("沒有的欄位不該設：%+v", plain)
	}
	for name, req := range map[string]*adminpb.ListBuyerVouchersRequest{
		"買家 ID 格式錯誤":      {BuyerId: "x"},
		"狀態是 UNSPECIFIED": {BuyerId: "9", Statuses: []pb.VoucherStatus{pb.VoucherStatus_VOUCHER_STATUS_UNSPECIFIED}},
		"page_size 負數":    {BuyerId: "9", PageSize: -1},
		"token 不屬於這位買家":   {BuyerId: "10", PageToken: res.Msg.GetNextPageToken()},
	} {
		if _, err := r.client.ListBuyerVouchers(context.Background(), connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：err=%v，預期 invalid_argument", name, err)
		}
	}
}

// wireReason 取出 Connect 錯誤帶的 ErrorInfo.reason。
func wireReason(err error) string {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return ""
	}
	for _, d := range ce.Details() {
		if v, e := d.Value(); e == nil {
			if info, ok := v.(*errdetails.ErrorInfo); ok {
				return info.GetReason()
			}
		}
	}
	return ""
}

// 模組的業務原因對到契約的 ErrorReason；找不到不帶原因。
func TestAdminErrorsCarryTheContractReason(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	r.module.err = apperr.FailedPrecondition("%s", "領完了")
	r.admin.reasonOf = func(error) coupon.Reason { return coupon.ReasonIssueLimitReached }
	_, err := r.client.GrantVouchers(context.Background(), connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: campaignIDString(), BuyerIds: []string{"9"}}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || wireReason(err) != "COUPON_ISSUE_LIMIT_REACHED" {
		t.Fatalf("err=%v reason=%q，預期 failed_precondition 與 COUPON_ISSUE_LIMIT_REACHED", err, wireReason(err))
	}
	r.module.err = apperr.NotFound("%s", "找不到")
	r.admin.reasonOf = func(error) coupon.Reason { return coupon.ReasonVoucherNotFound }
	_, err = r.client.VoidVoucher(context.Background(), connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: voucherA.String(), Reason: "r"}))
	if connect.CodeOf(err) != connect.CodeNotFound || wireReason(err) != "" {
		t.Fatalf("找不到：err=%v reason=%q，預期 not_found 且不帶原因", err, wireReason(err))
	}
}

// 模組函式的欄位名對到請求路徑，路徑要真的存在於請求。
func TestAdminFieldNamesMapToRequestPaths(t *testing.T) {
	for name, c := range map[string]struct {
		msg   proto.Message
		paths rpc.FieldPaths
	}{
		"建立": {&adminpb.CreateCampaignRequest{}, adminCreatePaths},
		"結束": {&adminpb.EndCampaignRequest{}, adminEndPaths},
		"發放": {&adminpb.GrantVouchersRequest{}, adminGrantPaths},
		"作廢": {&adminpb.VoidVoucherRequest{}, adminVoidPaths},
		"配額": {&adminpb.SetSellerQuotaRequest{}, adminQuotaPaths},
		"列表": {&adminpb.ListCampaignsRequest{}, adminListPaths},
	} {
		if missing := rpc.CheckFieldPaths(c.msg.ProtoReflect().Descriptor(), c.paths); len(missing) != 0 {
			t.Errorf("%s：對應表的路徑在請求裡不存在：%v", name, missing)
		}
	}
}

func fieldViolations(t *testing.T, err error) []string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	var out []string
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			continue
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				out = append(out, fv.GetField())
			}
		}
	}
	return out
}

// 買家與店家的存在檢查在模組：handler 只把模組回的欄位錯誤對應到請求路徑（發放的索引是請求陣列的位置）。
func TestAdminMapsTheModulesExistenceErrorsToRequestPaths(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()

	r.module.err = apperr.InvalidInputOn([]string{apperr.IndexedField("TargetBuyers", 2)}, "買家不存在")
	in := adminCashInput()
	in.TargetType, in.DesignatedBuyerIds = pb.TargetType_TARGET_TYPE_DESIGNATED, []string{"9", "10", "13"}
	_, err := r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldViolations(t, err), []string{"campaign.designated_buyer_ids[2]"}) {
		t.Fatalf("建立：err=%v 欄位=%v，預期 invalid_argument 指出 designated_buyer_ids[2]", err, fieldViolations(t, err))
	}

	r.module.err = apperr.InvalidInputOn([]string{apperr.IndexedField("BuyerIDs", 2)}, "買家不存在")
	_, err = r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: campaignIDString(), BuyerIds: []string{"9", "9", "12"}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldViolations(t, err), []string{"buyer_ids[2]"}) {
		t.Fatalf("發放：err=%v 欄位=%v，預期 invalid_argument 指出 buyer_ids[2]", err, fieldViolations(t, err))
	}

	r.module.err = apperr.InvalidInputOn([]string{"StoreID"}, "店家不存在")
	in = adminCashInput()
	store := "99"
	in.StoreId = &store
	_, err = r.client.CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: in}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldViolations(t, err), []string{"campaign.store_id"}) {
		t.Fatalf("店家：err=%v 欄位=%v，預期 invalid_argument 指出 campaign.store_id", err, fieldViolations(t, err))
	}
}

// 發放請求的原始 ID 個數上限是 2 * MaxIssueBatch：超過立即 invalid_argument，不呼叫模組；上限內（含重複）原樣交給模組去重。
func TestAdminGrantBoundsTheRawBuyerList(t *testing.T) {
	r := newAdminRig(t, adminPrincipal())
	ctx := context.Background()
	ids := make([]string, 0, coupon.MaxIssueRequestBuyers+1)
	for i := range coupon.MaxIssueRequestBuyers + 1 {
		ids = append(ids, strconv.Itoa(i%7+1))
	}
	_, err := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: campaignIDString(), BuyerIds: ids}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldViolations(t, err), []string{"buyer_ids"}) || len(r.module.calls) != 0 {
		t.Fatalf("原始長度 %d 超過上限：err=%v 欄位=%v calls=%v", len(ids), err, fieldViolations(t, err), r.module.calls)
	}
	ids = ids[:coupon.MaxIssueRequestBuyers]
	if _, err := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: campaignIDString(), BuyerIds: ids})); err != nil {
		t.Fatalf("上限內的重複名單：%v", err)
	}
	if len(r.module.issueIn.BuyerIDs) != coupon.MaxIssueRequestBuyers {
		t.Fatalf("交給模組 %d 個，預期原樣 %d 個（去重在模組）", len(r.module.issueIn.BuyerIDs), coupon.MaxIssueRequestBuyers)
	}
	if _, err := r.client.GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: campaignIDString()})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("空名單：%v", err)
	}
}
