//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// stagedMedia 是 media 的替身：轉正回以 token 命名的資產。
type stagedMedia struct{}

func (stagedMedia) Finalize(_ context.Context, _ media.Uploader, token, refType string) (media.Asset, error) {
	// "expired-" 開頭的 token 模擬 media-service 拒絕轉正（無效、過期或不屬於這個帳號）：legacy 轉接層回 InvalidInput。
	if strings.HasPrefix(token, "expired-") {
		return media.Asset{}, fmt.Errorf("%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳"))
	}
	return media.Asset{ID: "asset-" + token, ObjectKey: refType + "/" + token, Secure: refType == media.RefTypeCertification || refType == media.RefTypeGovernanceEvidence}, nil
}
func (stagedMedia) Link(context.Context, media.Ref, []string) error   { return nil }
func (stagedMedia) Unlink(context.Context, media.Ref, []string) error { return nil }

type orders struct{ open bool }

func (o *orders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return o.open, nil }

// noDashboardOrders 是這個測試不讀儀表板的訂單時的替身：NewHandler 要求提供儀表板訂單的來源。
type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type fixture struct {
	app       sellerconnect.SellerAppServiceClient
	other     sellerconnect.SellerAppServiceClient
	public    sellerconnect.SellerPublicServiceClient
	signedIn  sellerconnect.SellerPublicServiceClient
	http      *http.Client
	base      string
	module    *seller.Module
	id        *identity.Module
	dbCount   func(t *testing.T, query string) int64
	inTx      func(context.Context, func(context.Context) error) error
	accountID int64
	orders    *orders
	adminTok  string
	registry  *fakeRegistry
	otherID   int64
}

func clientOptions(token string) []connect.ClientOption {
	auth := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	})
	return []connect.ClientOption{connect.WithInterceptors(auth)}
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "buyer", Dir: buyer.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()}))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Jobs: queue, Providers: identity.Providers{Customers: []identity.Clerk{dev.Customers()}, Admin: dev.Admins()}, Now: time.Now, Logger: log})
	sellerToken, err := id.DevSession(ctx, "dev_seller_connect")
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := id.DevSession(ctx, "dev_other_connect")
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := dev.AdminToken("dev_admin_connect")
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := id.Authenticate(ctx, sellerToken)
	if err != nil {
		t.Fatal(err)
	}
	otherAuthenticated, err := id.Authenticate(ctx, otherToken)
	if err != nil {
		t.Fatal(err)
	}
	registry := &fakeRegistry{}
	o := &orders{}
	kycKeys, kycMAC := sealtest.Keys(t)
	m := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: id, Media: stagedMedia{}, Orders: o, KYCKeys: kycKeys, KYCMAC: kycMAC,
		ProductStores: noProducts{}, Registry: registry, Now: time.Now, Logger: log,
	})
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	h, err := httpapi.NewHandler(httpapi.Deps{Social: &social.Module{},
		Ready: d.Ping, Identity: id, Seller: m, Buyer: buyers, Images: images, Logger: log,
		Deletion: &accountdeletion.Module{}, Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}}), Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, DashboardBudget: time.Minute, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	return &fixture{
		app:      sellerconnect.NewSellerAppServiceClient(server.Client(), server.URL, clientOptions(sellerToken)...),
		other:    sellerconnect.NewSellerAppServiceClient(server.Client(), server.URL, clientOptions(otherToken)...),
		public:   sellerconnect.NewSellerPublicServiceClient(server.Client(), server.URL, connect.WithHTTPGet()),
		signedIn: sellerconnect.NewSellerPublicServiceClient(server.Client(), server.URL, append(clientOptions(sellerToken), connect.WithHTTPGet())...),
		http:     server.Client(), base: server.URL, module: m, id: id, accountID: authenticated.Principal.ID, orders: o, adminTok: adminToken,
		registry: registry, otherID: otherAuthenticated.Principal.ID,
		inTx: d.InTx,
		dbCount: func(t *testing.T, query string) int64 {
			t.Helper()
			var n int64
			if err := d.Conn(ctx).QueryRow(ctx, query).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		},
	}
}

// wantCode 確認錯誤的 code 與 ErrorInfo.reason，回傳第一個 BadRequest 違規的欄位路徑。
func wantCode(t *testing.T, err error, code connect.Code, reason string) string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code {
		t.Fatalf("err=%v want=%v", err, code)
	}
	gotReason, field := "", ""
	for _, detail := range ce.Details() {
		value, e := detail.Value()
		if e != nil {
			t.Fatal(e)
		}
		switch d := value.(type) {
		case *errdetails.ErrorInfo:
			gotReason = d.GetReason()
		case *errdetails.BadRequest:
			if len(d.GetFieldViolations()) > 0 {
				field = d.GetFieldViolations()[0].GetField()
			}
		}
	}
	if gotReason != reason {
		t.Fatalf("reason=%q want=%q", gotReason, reason)
	}
	return field
}

// errorReason 回傳 ErrorInfo.reason，沒有時是空字串。
func errorReason(err error) string {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return ""
	}
	for _, detail := range ce.Details() {
		if value, e := detail.Value(); e == nil {
			if info, ok := value.(*errdetails.ErrorInfo); ok {
				return info.GetReason()
			}
		}
	}
	return ""
}

// errorMetadata 回傳 ErrorInfo.metadata 的一個值。
func errorMetadata(t *testing.T, err error, key string) string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	for _, detail := range ce.Details() {
		value, e := detail.Value()
		if e != nil {
			t.Fatal(e)
		}
		if info, ok := value.(*errdetails.ErrorInfo); ok {
			return info.GetMetadata()[key]
		}
	}
	return ""
}

// capabilityStatus 取出 GetMyStore 回報的一個能力的狀態。
func capabilityStatus(t *testing.T, resp *pb.GetMyStoreResponse, c pb.SellerCapability) *pb.CapabilityStatus {
	t.Helper()
	return httpseller.CapabilityStatusOf(t, resp.GetCapabilities(), c)
}

func validApply() *pb.ApplyStoreRequest {
	return &pb.ApplyStoreRequest{StoreName: "範例小農", ContactPhone: "0912345678", Address: &commonpb.PostalAddress{TownCode: "63000050", AddressDetail: "測試路一號"}, AcceptedAgreements: pbAcceptances(seller.CurrentApplyAcceptances())}
}

func (f *fixture) apply(t *testing.T) *pb.Store {
	t.Helper()
	resp, err := f.app.ApplyStore(context.Background(), connect.NewRequest(validApply()))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetStore()
}

// fillContact 補上送出 KYC 需要的店家聯絡資料（開店不帶這三欄，用 UpdateMyStore 寫）。
func (f *fixture) fillContact(t *testing.T, c sellerconnect.SellerAppServiceClient) *pb.Store {
	t.Helper()
	got, err := c.GetMyStore(context.Background(), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.UpdateMyStore(context.Background(), connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{Etag: got.Msg.GetStore().GetEtag(), ContactName: "王小明", ContactMobile: "0912345678", SupportEmail: "owner@example.com"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"contact_name", "contact_mobile", "support_email"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetStore()
}

// applyComplete 開店並補齊店家的聯絡資料。
func (f *fixture) applyComplete(t *testing.T, c sellerconnect.SellerAppServiceClient) *pb.Store {
	t.Helper()
	if _, err := c.ApplyStore(context.Background(), connect.NewRequest(validApply())); err != nil {
		t.Fatal(err)
	}
	return f.fillContact(t, c)
}

func (f *fixture) get(t *testing.T) *pb.GetMyStoreResponse {
	t.Helper()
	resp, err := f.app.GetMyStore(context.Background(), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg
}

func (f *fixture) isSeller(t *testing.T) bool {
	t.Helper()
	a, err := f.id.Account(context.Background(), f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Contains(a.Roles, identity.RoleSeller)
}

func (f *fixture) storeID(t *testing.T) int64 {
	t.Helper()
	s, err := f.module.CurrentStore(context.Background(), f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	return s.StoreID
}

func TestAuthorization(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	noLogin := sellerconnect.NewSellerAppServiceClient(f.http, f.base)
	_, err := noLogin.GetMyStore(ctx, connect.NewRequest(&pb.GetMyStoreRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated, "")
	_, err = noLogin.ApplyStore(ctx, connect.NewRequest(validApply()))
	wantCode(t, err, connect.CodeUnauthenticated, "")
	admin := sellerconnect.NewSellerAppServiceClient(f.http, f.base, clientOptions(f.adminTok)...)
	_, err = admin.GetMyStore(ctx, connect.NewRequest(&pb.GetMyStoreRequest{}))
	wantCode(t, err, connect.CodePermissionDenied, "")
	_, err = admin.ApplyStore(ctx, connect.NewRequest(validApply()))
	wantCode(t, err, connect.CodePermissionDenied, "")
}

// 申請開店（豁免宣告，帳號還不是賣家）建立店家並授予角色；新店的 GetMyStore 列出還缺的曝光條件。
func TestApplyStoreThenGetMyStoreListsMissingExposure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if f.isSeller(t) {
		t.Fatal("fixture account is already a seller")
	}
	created := f.apply(t)
	if created.GetId() == "" || created.GetStoreName() != "範例小農" || created.GetStatus() != pb.StoreStatus_STORE_STATUS_ACTIVE || created.GetEtag() == "" || created.GetLogo() != nil {
		t.Fatalf("created=%v", created)
	}
	if !f.isSeller(t) {
		t.Fatal("SELLER not granted")
	}
	got := f.get(t)
	if got.GetStore().GetId() != created.GetId() || got.GetSuspended() {
		t.Fatalf("got=%v", got)
	}
	want := []pb.SellerRequirement{pb.SellerRequirement_SELLER_REQUIREMENT_KYC_APPROVED, pb.SellerRequirement_SELLER_REQUIREMENT_LOGO}
	if listing := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_PUBLIC_LISTING); listing.GetActive() || !slices.Equal(listing.GetMissing(), want) {
		t.Fatalf("public listing=%v", listing)
	}
	if shipping := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_SHIPPING_SETUP); shipping.GetActive() || !slices.Equal(shipping.GetMissing(), want[:1]) {
		t.Fatalf("shipping setup=%v", shipping)
	}
	if write := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_STOREFRONT_WRITE); !write.GetActive() || len(write.GetMissing()) != 0 || len(got.GetCapabilities()) != len(seller.AllCapabilities()) {
		t.Fatalf("capabilities=%v", got.GetCapabilities())
	}
	_, err := f.app.ApplyStore(context.Background(), connect.NewRequest(validApply()))
	wantCode(t, err, connect.CodeAlreadyExists, "")
}

// 輸入不合法時不建店、不授予角色；沒有店家的帳號讀自己的店是 not_found。
func TestApplyStoreWithAnInvalidTownCodeCreatesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	bad := validApply()
	bad.Address.TownCode = "99999999"
	_, err := f.app.ApplyStore(ctx, connect.NewRequest(bad))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "address.town_code" {
		t.Fatalf("field=%q", field)
	}
	if n := f.dbCount(t, "SELECT count(*) FROM seller.stores"); n != 0 {
		t.Fatalf("stores=%d", n)
	}
	if f.isSeller(t) {
		t.Fatal("SELLER granted although the store was not created")
	}
	_, err = f.app.GetMyStore(ctx, connect.NewRequest(&pb.GetMyStoreRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.app.ApplyStore(ctx, connect.NewRequest(&pb.ApplyStoreRequest{}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

// 同意的文件不對時，錯誤到 client 手上的樣子：版本落後是 failed_precondition 加 reason AGREEMENT_OUTDATED；種類不對是
// invalid_argument，欄位指向 accepted_agreements。兩者都不建店。
func TestApplyStoreAgreementErrorsReachTheClient(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	outdated := validApply()
	outdated.AcceptedAgreements[0].Version = "2026-01-01"
	_, err := f.app.ApplyStore(ctx, connect.NewRequest(outdated))
	wantCode(t, err, connect.CodeFailedPrecondition, "AGREEMENT_OUTDATED")
	wrongKind := validApply()
	wrongKind.AcceptedAgreements = pbAcceptances(seller.CurrentKYCAcceptances())
	_, err = f.app.ApplyStore(ctx, connect.NewRequest(wrongKind))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "accepted_agreements" {
		t.Fatalf("field=%q", field)
	}
	unspecified := validApply()
	unspecified.AcceptedAgreements[0].Kind = pb.AgreementKind_AGREEMENT_KIND_UNSPECIFIED
	_, err = f.app.ApplyStore(ctx, connect.NewRequest(unspecified))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "accepted_agreements" {
		t.Fatalf("field=%q", field)
	}
	if n := f.dbCount(t, "SELECT count(*) FROM seller.stores"); n != 0 {
		t.Fatalf("stores=%d", n)
	}
}

func TestWritesWithoutAStoreAreNotFound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, err := f.other.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{StoreName: "x", Etag: "1"}}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.other.CloseMyStore(ctx, connect.NewRequest(&pb.CloseMyStoreRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.other.ListMyFollowers(ctx, connect.NewRequest(&pb.ListMyFollowersRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")
}

func TestUpdateMyStoreAppliesTheMaskAndOnboarding(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	resp, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{
			StoreName: "新店名", Description: "不在 mask 內",
			OnboardingCompleted: true,
			Etag:                created.GetEtag(),
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name", "onboarding_completed"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	s := resp.Msg.GetStore()
	if s.GetStoreName() != "新店名" || s.GetDescription() != "" || s.GetEtag() == created.GetEtag() {
		t.Fatalf("store=%v", s)
	}
	if s.GetContactPhone() != "0912345678" || s.GetAddress().GetTownCode() != "63000050" {
		t.Fatalf("欄位不在 mask 內卻被改掉：%v", s)
	}
	// 食品業者登錄不再是店家的欄位：food_business 是未知的路徑，不寫入。
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{Etag: s.GetEtag()},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"food_business"}},
	}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	// onboarding_completed 是 OUTPUT_ONLY：UpdateMyStore 不會改它。
	if s.GetOnboardingCompleted() {
		t.Fatalf("UpdateMyStore completed the onboarding: %v", s)
	}

	// 地址只有鄉鎮、沒有明細：可以成功寫入，明細為空字串。
	resp, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{
			Address: &commonpb.PostalAddress{TownCode: "63000050"},
			Etag:    s.GetEtag(),
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"address"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	s = resp.Msg.GetStore()
	if s.GetAddress() == nil || s.GetAddress().GetTownCode() != "63000050" || s.GetAddress().GetAddressDetail() != "" {
		t.Fatalf("update address town only failed: %v", s)
	}

	// 清除地址：update_mask 有 address 但沒有帶值。
	resp, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{Etag: s.GetEtag()},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"address"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	s = resp.Msg.GetStore()
	if s.GetAddress() != nil {
		t.Fatalf("clearing address failed: %v", s)
	}
}

// CompleteMyStoreOnboarding 標記完成、冪等（已完成時 etag 不變），過期的 etag 是 VERSION_CONFLICT。
func TestCompleteMyStoreOnboardingIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	first, err := f.app.CompleteMyStoreOnboarding(ctx, connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{Etag: created.GetEtag()}))
	if err != nil || !first.Msg.GetStore().GetOnboardingCompleted() || first.Msg.GetStore().GetEtag() == created.GetEtag() {
		t.Fatalf("first=%v err=%v", first, err)
	}
	again, err := f.app.CompleteMyStoreOnboarding(ctx, connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{Etag: first.Msg.GetStore().GetEtag()}))
	if err != nil || again.Msg.GetStore().GetEtag() != first.Msg.GetStore().GetEtag() {
		t.Fatalf("again=%v err=%v", again, err)
	}
	_, err = f.app.CompleteMyStoreOnboarding(ctx, connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{Etag: created.GetEtag()}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = f.app.CompleteMyStoreOnboarding(ctx, connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
		t.Fatalf("field=%q", field)
	}
}

func TestUpdateMyStoreVersionConflictAndMissingEtag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	mask := &fieldmaskpb.FieldMask{Paths: []string{"store_name"}}
	if _, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{StoreName: "第一次", Etag: created.GetEtag()}, UpdateMask: mask})); err != nil {
		t.Fatal(err)
	}
	_, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{StoreName: "過期的", Etag: created.GetEtag()}, UpdateMask: mask}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{StoreName: "過期的", LogoStagedToken: "t", Etag: created.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name", "logo_staged_token"}},
	}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	if got := f.get(t).GetStore(); got.GetLogo() != nil || got.GetStoreName() != "第一次" {
		t.Fatalf("conflicting update left a partial write: %v", got)
	}
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{StoreName: "沒有 etag"}, UpdateMask: mask}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "store.etag" {
		t.Fatalf("field=%q", field)
	}
	if got := f.get(t).GetStore().GetStoreName(); got != "第一次" {
		t.Fatalf("store_name=%q", got)
	}
}

// logo 與封面隨 UpdateMyStore 的 staged token 一起換，logo 滿足曝光條件；沒列進 update_mask 的不換；只有空白的 token
// 指向請求的欄位。
func TestLogoAndCoverChangeThroughUpdateMyStoreTokens(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	logo, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{LogoStagedToken: "logo-token", CoverStagedToken: "cover-token", Prize: "金牌", Etag: created.GetEtag()},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"logo_staged_token", "prize"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	s := logo.Msg.GetStore()
	if uri := s.GetLogo().GetUriTemplate(); !strings.HasPrefix(uri, "https://media.example.test/i/{preset}/") || !strings.HasSuffix(uri, "logo-token") || s.GetPrize() != "金牌" {
		t.Fatalf("store=%v", s)
	}
	if s.GetCoverImage() != nil {
		t.Fatalf("cover changed although it is not in the mask: %v", s)
	}
	cover, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{CoverStagedToken: "cover-token", Etag: s.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"cover_staged_token"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cover.Msg.GetStore().GetCoverImage() == nil || cover.Msg.GetStore().GetLogo() == nil || cover.Msg.GetStore().GetLogo().GetUriTemplate() != s.GetLogo().GetUriTemplate() {
		t.Fatalf("store=%v", cover.Msg.GetStore())
	}
	got := f.get(t)
	if listing := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_PUBLIC_LISTING); !slices.Equal(listing.GetMissing(), []pb.SellerRequirement{pb.SellerRequirement_SELLER_REQUIREMENT_KYC_APPROVED}) {
		t.Fatalf("missing=%v", listing.GetMissing())
	}
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{LogoStagedToken: "  ", Etag: cover.Msg.GetStore().GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"logo_staged_token"}},
	}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "store.logo_staged_token" {
		t.Fatalf("field=%q", field)
	}
}

// 停權的賣家：寫入得到 permission_denied 與 SELLER_SUSPENDED，讀自己的店與關店仍可以。
func TestSuspendedSellerCanReadAndCloseButNotWrite(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	f.suspendStore(t, f.storeID(t))
	_, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{StoreName: "x", Etag: created.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name"}}}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{LogoStagedToken: "t", Etag: created.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"logo_staged_token"}}}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.CompleteMyStoreOnboarding(ctx, connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{Etag: created.GetEtag()}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.ListMyFollowers(ctx, connect.NewRequest(&pb.ListMyFollowersRequest{}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.GetMyDashboard(ctx, connect.NewRequest(&pb.GetMyDashboardRequest{}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")

	got := f.get(t)
	if !got.GetSuspended() {
		t.Fatalf("suspended=%v", got.GetSuspended())
	}
	suspendedWant := []pb.SellerRequirement{pb.SellerRequirement_SELLER_REQUIREMENT_NOT_SUSPENDED}
	if write := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_STOREFRONT_WRITE); write.GetActive() || !slices.Equal(write.GetMissing(), suspendedWant) {
		t.Fatalf("storefront write=%v", write)
	}
	if closing := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_ACCOUNT_CLOSE); !closing.GetActive() || len(closing.GetMissing()) != 0 {
		t.Fatalf("account close=%v", closing)
	}
	if listing := capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_PUBLIC_LISTING); !slices.Contains(listing.GetMissing(), pb.SellerRequirement_SELLER_REQUIREMENT_NOT_SUSPENDED) {
		t.Fatalf("public listing=%v", listing)
	}

	closed, err := f.app.CloseMyStore(ctx, connect.NewRequest(&pb.CloseMyStoreRequest{}))
	if err != nil || closed.Msg.GetStore().GetStatus() != pb.StoreStatus_STORE_STATUS_CLOSED || f.isSeller(t) {
		t.Fatalf("closed=%v err=%v", closed, err)
	}
	_, err = f.app.GetMyStore(ctx, connect.NewRequest(&pb.GetMyStoreRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")
}

// 儀表板經完整的入口：訂單來源（舊服務）壞掉時只有銷售與訂單狀態不設，粉絲與瀏覽數來自單體自己的模組，照樣有值；期間一直有。
// fixture 注入寬鬆的 DashboardBudget：粉絲與瀏覽數是真的 DB 讀取，測的是「來源壞掉只影響自己的區塊」，不是機器夠不夠快。
func TestDashboardKeepsTheSectionsWhoseSourcesWorkWhenOrdersAreDown(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.apply(t)
	resp, err := f.app.GetMyDashboard(ctx, connect.NewRequest(&pb.GetMyDashboardRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	d := resp.Msg.GetDashboard()
	if d.GetSales() != nil || d.GetOrderStatusCounts() != nil {
		t.Fatalf("訂單來源壞了，銷售與訂單狀態必須不設：%v", d)
	}
	if d.GetFollowers() == nil || d.GetFollowers().GetTotal() != 0 || d.GetFollowers().GetNewFollowers() == nil ||
		d.GetProductViews() == nil || d.GetProductViews().GetCurrent() != 0 || d.GetProductViews().GetPrevious() != 0 {
		t.Fatalf("單體自己的模組讀得到，必須有值（零是真的零）：%v", d)
	}
	first, last := d.GetCurrentPeriod().GetFirstDate(), d.GetCurrentPeriod().GetLastDate()
	span := time.Date(int(last.GetYear()), time.Month(last.GetMonth()), int(last.GetDay()), 0, 0, 0, 0, time.UTC).
		Sub(time.Date(int(first.GetYear()), time.Month(first.GetMonth()), int(first.GetDay()), 0, 0, 0, 0, time.UTC))
	if span != 6*24*time.Hour {
		t.Fatalf("current_period 必須含頭含尾共 7 天：%v 到 %v", first, last)
	}
}

// 隨帳號刪除等待中的店家：寫入與關店是 failed_precondition，讀自己的店仍可以。
func TestStoreWaitingForDeletionRejectsWritesButCanBeRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	storeID := f.storeID(t)
	// 與刪帳號流程相同：進入等待要在 transaction 內呼叫。
	if err := f.inTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, storeID) }); err != nil {
		t.Fatal(err)
	}
	_, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{StoreName: "x", Etag: created.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name"}}}))
	wantCode(t, err, connect.CodeFailedPrecondition, "SELLER_REQUIREMENT_MISSING")
	if got := errorMetadata(t, err, "missingRequirements"); got != "NOT_PENDING_DELETION" {
		t.Fatalf("missingRequirements=%q，應為 NOT_PENDING_DELETION", got)
	}
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: &pb.Store{LogoStagedToken: "t", Etag: created.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"logo_staged_token"}}}))
	wantCode(t, err, connect.CodeFailedPrecondition, "SELLER_REQUIREMENT_MISSING")
	_, err = f.app.CloseMyStore(ctx, connect.NewRequest(&pb.CloseMyStoreRequest{}))
	wantCode(t, err, connect.CodeFailedPrecondition, "SELLER_REQUIREMENT_MISSING")
	got := f.get(t)
	pendingWant := []pb.SellerRequirement{pb.SellerRequirement_SELLER_REQUIREMENT_NOT_PENDING_DELETION}
	if got.GetStore().GetStatus() != pb.StoreStatus_STORE_STATUS_PENDING_DELETION || got.GetSuspended() ||
		capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_ACCOUNT_CLOSE).GetActive() ||
		!slices.Equal(capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_STOREFRONT_WRITE).GetMissing(), pendingWant) ||
		!slices.Contains(capabilityStatus(t, got, pb.SellerCapability_SELLER_CAPABILITY_PUBLIC_LISTING).GetMissing(), pb.SellerRequirement_SELLER_REQUIREMENT_NOT_PENDING_DELETION) {
		t.Fatalf("got=%v", got)
	}
}

func TestCloseMyStoreIsRejectedWhileTheStoreHasOpenOrders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.apply(t)
	f.orders.open = true
	_, err := f.app.CloseMyStore(ctx, connect.NewRequest(&pb.CloseMyStoreRequest{}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	if f.get(t).GetStore().GetStatus() != pb.StoreStatus_STORE_STATUS_ACTIVE {
		t.Fatal("store closed although it has open orders")
	}
	f.orders.open = false
	closed, err := f.app.CloseMyStore(ctx, connect.NewRequest(&pb.CloseMyStoreRequest{}))
	if err != nil || closed.Msg.GetStore().GetStatus() != pb.StoreStatus_STORE_STATUS_CLOSED {
		t.Fatalf("closed=%v err=%v", closed, err)
	}
}

// ReadSecure 實作 media.Media；這些測試不讀機敏資產。
func (stagedMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}

// 措施記在舊 stint、帳號目前的 stint 繼承停權時，GetMyStore 的 suspended 仍為真，停權的內容經 ListMyMeasures(state=ACTIVE)
// 讀到（措施的效果是帳號範圍）。
func TestGetMyStoreShowsASuspensionRecordedOnAnEarlierStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.apply(t)
	old, err := f.module.CurrentStore(ctx, f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	record := f.suspendStore(t, old.StoreID)
	if _, err := f.module.CloseCurrent(ctx, f.accountID); err != nil {
		t.Fatal(err)
	}
	// Apply 在停權中會拒絕，所以直接寫一個新的 stint 重現「新 stint 繼承停權」。
	if err := f.inTx(ctx, func(ctx context.Context) error {
		tx, _ := db.TxFrom(ctx)
		_, err := tx.Exec(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, '新店', now(), now())`, old.StoreCode+1, f.accountID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if f.storeID(t) == old.StoreID {
		t.Fatal("沒有新的 stint")
	}
	got := f.get(t)
	if !got.GetSuspended() {
		t.Fatalf("suspended=%v", got.GetSuspended())
	}
	active, err := f.app.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{State: pb.MeasureState_MEASURE_STATE_ACTIVE}))
	if err != nil || len(active.Msg.GetMeasures()) != 1 || active.Msg.GetMeasures()[0].GetId() != record.ID.String() {
		t.Fatalf("active=%v err=%v", active, err)
	}
}
