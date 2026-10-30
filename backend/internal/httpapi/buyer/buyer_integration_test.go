//go:build integration

package buyer_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1"
	adminconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	buyerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	identitypb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	identityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
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
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type source struct {
	products  map[int64]httpbuyer.ProductInfo
	stores    map[int64]seller.StoreCard
	owners    map[int64]int64
	summaries []httpbuyer.MerchantSummary
	storesErr error
}

func (s *source) Cards(_ context.Context, _ []int64, _ bool) (map[int64]httpbuyer.ProductInfo, error) {
	return s.products, nil
}

func openCard(id int64, name string) seller.StoreCard {
	return seller.StoreCard{StoreID: id, StoreCode: 2600000000 + id, Name: &name, Status: seller.CardOpen}
}

// storeSource 是 Stores port 的替身，讀 source 裡設定的店家：stores 是有卡片的店，owners 是可以追蹤的店與它的擁有者帳號。
type storeSource struct{ *source }

func (s storeSource) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	var out []seller.StoreCard
	for _, id := range ids {
		if c, ok := s.stores[id]; ok {
			out = append(out, c)
		}
	}
	return out, s.storesErr
}

func (s storeSource) PublicStores(_ context.Context, ids []int64) ([]seller.Store, error) {
	var out []seller.Store
	for _, id := range ids {
		if owner, ok := s.owners[id]; ok {
			out = append(out, seller.Store{StoreID: id, AccountID: owner})
		}
	}
	return out, nil
}

func (s *source) MerchantSummary(context.Context, int64) ([]httpbuyer.MerchantSummary, error) {
	return s.summaries, nil
}
func (*source) SpendingRecords(context.Context, int64, int64, int, int) (httpbuyer.SpendingPage, error) {
	return httpbuyer.SpendingPage{}, nil
}
func (*source) OpenOrders(context.Context, int64) (bool, error) { return false, nil }
func (s *source) HasOpenOrders(ctx context.Context, buyerID int64) (bool, error) {
	return s.OpenOrders(ctx, buyerID)
}
func (*source) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }
func (*source) VIPStatuses(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

type fixture struct {
	app        buyerconnect.BuyerAppServiceClient
	admin      adminconnect.BuyerAdminServiceClient
	identity   identityconnect.IdentityAppServiceClient
	base       string
	http       *http.Client
	buyerToken string
	adminToken string
	db         *db.DB
	module     *buyer.Module
	id         *identity.Module
	dev        *identity.DevAuth
	accountID  int64
	buyerID    int64
	source     *source
	stores     *switchStores
	seller     *seller.Module
}

// switchStores 讓測試把 Stores port 從替身換成 seller 模組的實作。
type switchStores struct{ current httpbuyer.Stores }

func (s *switchStores) StoreCards(ctx context.Context, ids []int64) ([]seller.StoreCard, error) {
	return s.current.StoreCards(ctx, ids)
}

func (s *switchStores) PublicStores(ctx context.Context, ids []int64) ([]seller.Store, error) {
	return s.current.PublicStores(ctx, ids)
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
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "buyer", Dir: buyer.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()}, migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()}, migrate.Module{Name: "delegation", Dir: delegation.Migrations()}))
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
	adminToken, err := dev.AdminToken("dev_admin_connect")
	if err != nil {
		t.Fatal(err)
	}
	characterize.GrantAdminOwner(t, id, adminToken)
	// 與正式組裝相同：買家在建帳的 transaction 內由 AccountCreated 建立（cmd/hawkr 的 subscribeBuyerLifecycle），
	// 測試不自己建買家、也不在 ctx 手塞 principal。buyer_id 的序列先推開，讓 buyer_id 與 account_id 不同值，
	// 誤把帳號 ID 當買家 ID 的程式會失敗。
	if _, err := d.Pool().Exec(ctx, `ALTER TABLE buyer.buyers ALTER COLUMN buyer_id RESTART WITH 5000`); err != nil {
		t.Fatal(err)
	}
	m := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	events.Subscribe(bus, m.OnAccountCreated)
	buyerToken, err := id.DevSession(ctx, "dev_buyer_connect")
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := id.Authenticate(ctx, buyerToken)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := m.Profile(ctx, authenticated.Principal.ID)
	if err != nil {
		t.Fatal(err)
	}
	stores := &switchStores{}
	s := &source{products: map[int64]httpbuyer.ProductInfo{}, stores: map[int64]seller.StoreCard{}, owners: map[int64]int64{}}
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	kycKeys, kycMAC := sealtest.Keys(t)
	sellerModule := seller.New(seller.Deps{KYCKeys: kycKeys, KYCMAC: kycMAC, DB: d, Events: bus, Jobs: queue, Identity: id, Media: noMedia{}, Orders: s, ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log})
	h, err := httpapi.NewHandler(httpapi.Deps{Social: &social.Module{}, Ready: d.Ping, Buyer: m, Seller: sellerModule, Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{}, BuyerPorts: httpbuyer.Ports{Products: s, Stores: stores, Orders: s, CRM: s}, Deletion: accountdeletion.New(accountdeletion.Deps{DB: d, Buyer: m, Seller: sellerModule, Identity: id, Orders: s, Now: time.Now}), Identity: id, Delegation: delegation.New(delegation.Deps{DB: d, Identity: id, Seller: sellerModule, Now: time.Now, Logger: log}), Images: images, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	stores.current = storeSource{s}
	f := &fixture{stores: stores, seller: sellerModule, base: server.URL, http: server.Client(), buyerToken: buyerToken, adminToken: adminToken, db: d, module: m, id: id, dev: dev, accountID: authenticated.Principal.ID, buyerID: profile.BuyerID, source: s}
	f.app = buyerconnect.NewBuyerAppServiceClient(f.http, f.base, clientOptions(buyerToken)...)
	f.admin = adminconnect.NewBuyerAdminServiceClient(f.http, f.base, clientOptions(adminToken)...)
	f.identity = identityconnect.NewIdentityAppServiceClient(f.http, f.base, clientOptions(buyerToken)...)
	return f
}

// noProducts 是 seller 的商品所屬店家 port 的替身：這裡的測試不彙總瀏覽事件。
// noRegistry 是登記資料來源的替身：這個測試不送公司型申請，來源不會被呼叫。
type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type noMedia struct{}

func (noMedia) Finalize(context.Context, media.Uploader, string, string) (media.Asset, error) {
	return media.Asset{}, errors.New("not used")
}
func (noMedia) Link(context.Context, media.Ref, []string) error   { return nil }
func (noMedia) Unlink(context.Context, media.Ref, []string) error { return nil }

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

func address(name string) *pb.Address {
	return &pb.Address{
		ReceiverName: name, ReceiverPhone: "0912345678",
		PostalAddress: &commonpb.PostalAddress{TownCode: "63000050", AddressDetail: "一號"},
	}
}

func TestConnectAuthorization(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	noLogin := buyerconnect.NewBuyerAppServiceClient(f.http, f.base)
	_, err := noLogin.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated, "")
	adminAsBuyer := buyerconnect.NewBuyerAppServiceClient(f.http, f.base, clientOptions(f.adminToken)...)
	_, err = adminAsBuyer.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	wantCode(t, err, connect.CodePermissionDenied, "")
	accountAsAdmin := adminconnect.NewBuyerAdminServiceClient(f.http, f.base, clientOptions(f.buyerToken)...)
	_, err = accountAsAdmin.GetBuyerStats(ctx, connect.NewRequest(&adminpb.GetBuyerStatsRequest{}))
	wantCode(t, err, connect.CodePermissionDenied, "")
	bareToken, err := f.id.DevSession(ctx, "dev_no_buyer")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := f.id.Authenticate(ctx, bareToken)
	if err != nil {
		t.Fatal(err)
	}
	// 帳號活著 ⇔ 買家活著，正式流程不會留下沒有買家的存活帳號；這裡只關閉買家、不刪帳號，模擬不變量被破壞：
	// 回 internal，不當成「不是買家」（permission_denied）或「沒有買家」。
	noBuyerID, err := f.module.CurrentBuyerID(ctx, auth.Principal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.module.CloseForDeletion(ctx, noBuyerID); err != nil {
		t.Fatal(err)
	}
	noBuyer := buyerconnect.NewBuyerAppServiceClient(f.http, f.base, clientOptions(bareToken)...)
	_, err = noBuyer.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	wantCode(t, err, connect.CodeInternal, "")
}

// 地址上限（W4）在 Connect 上是 failed_precondition（要先刪掉一筆），帶給使用者看的 LocalizedMessage；預設頁大小一頁就是完整清單。
func TestConnectAddressCap(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for range buyer.MaxAddresses {
		if _, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("收件人")})); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("收件人")}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	var ce *connect.Error
	localized := ""
	if errors.As(err, &ce) {
		for _, detail := range ce.Details() {
			if v, e := detail.Value(); e == nil {
				if m, ok := v.(*errdetails.LocalizedMessage); ok {
					localized = m.GetMessage()
				}
			}
		}
	}
	if !strings.Contains(localized, "最多 20 筆") {
		t.Fatalf("localized=%q", localized)
	}
	list, err := f.app.ListAddresses(ctx, connect.NewRequest(&pb.ListAddressesRequest{}))
	if err != nil || len(list.Msg.GetAddresses()) != buyer.MaxAddresses || list.Msg.GetNextPageToken() != "" {
		t.Fatalf("list=%d next=%q err=%v", len(list.Msg.GetAddresses()), list.Msg.GetNextPageToken(), err)
	}
}

func TestConnectVersionConflicts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	profile, err := f.app.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	old := profile.Msg.GetProfile().GetEtag()
	_, err = f.app.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{Profile: &pb.Profile{DisplayName: "新暱稱", Etag: old}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}}}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{Profile: &pb.Profile{DisplayName: "覆寫", Etag: old}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}}}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	profile, err = f.app.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	if err != nil || profile.Msg.GetProfile().GetDisplayName() != "新暱稱" {
		t.Fatalf("profile=%v err=%v", profile, err)
	}
	_, err = f.app.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{Profile: &pb.Profile{DisplayName: "缺版本"}}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	created, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("原名")}))
	if err != nil {
		t.Fatal(err)
	}
	a := created.Msg.GetAddress()
	newAddress := proto.Clone(a).(*pb.Address)
	newAddress.ReceiverName = "新名"
	_, err = f.app.UpdateAddress(ctx, connect.NewRequest(&pb.UpdateAddressRequest{Address: newAddress, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"receiver_name"}}}))
	if err != nil {
		t.Fatal(err)
	}
	newAddress.ReceiverName = "覆寫"
	_, err = f.app.UpdateAddress(ctx, connect.NewRequest(&pb.UpdateAddressRequest{Address: newAddress, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"receiver_name"}}}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	stored, err := f.app.GetAddress(ctx, connect.NewRequest(&pb.GetAddressRequest{Id: a.GetId()}))
	if err != nil || stored.Msg.GetAddress().GetReceiverName() != "新名" {
		t.Fatalf("address=%v err=%v", stored, err)
	}
	newAddress.Etag = ""
	_, err = f.app.UpdateAddress(ctx, connect.NewRequest(&pb.UpdateAddressRequest{Address: newAddress}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	plans, err := f.admin.ListMembershipPlans(ctx, connect.NewRequest(&adminpb.ListMembershipPlansRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	plan := plans.Msg.GetMembershipPlans()[0]
	stale := proto.Clone(plan).(*adminpb.MembershipPlan)
	stale.MonthlyPrice++
	_, err = f.admin.UpdateMembershipPlan(ctx, connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{MembershipPlan: stale, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"monthly_price"}}}))
	if err != nil {
		t.Fatal(err)
	}
	stale.MonthlyPrice++
	_, err = f.admin.UpdateMembershipPlan(ctx, connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{MembershipPlan: stale, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"monthly_price"}}}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	plans, err = f.admin.ListMembershipPlans(ctx, connect.NewRequest(&adminpb.ListMembershipPlansRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range plans.Msg.GetMembershipPlans() {
		if current.GetPlanCode() == plan.GetPlanCode() && current.GetMonthlyPrice() != plan.GetMonthlyPrice()+1 {
			t.Fatalf("plan overwritten: %v", current)
		}
	}
	stale.Etag = ""
	_, err = f.admin.UpdateMembershipPlan(ctx, connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{MembershipPlan: stale}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

func TestConnectAddressOwnershipAndPaging(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var rows []*pb.Address
	for _, name := range []string{"王小明甲", "王小明乙", "王小明丙"} {
		resp, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address(name)}))
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, resp.Msg.GetAddress())
	}
	var got []string
	var token string
	for range rows {
		resp, err := f.app.ListAddresses(ctx, connect.NewRequest(&pb.ListAddressesRequest{PageSize: 1, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Msg.GetAddresses()) != 1 {
			t.Fatalf("page=%v", resp.Msg)
		}
		got = append(got, resp.Msg.GetAddresses()[0].GetId())
		token = resp.Msg.GetNextPageToken()
		if len(got) == 1 {
			if got[0] != rows[0].GetId() || !resp.Msg.GetAddresses()[0].GetDefaultAddress() {
				t.Fatalf("default first: %v", resp.Msg)
			}
			_, err := f.app.ListFavorites(ctx, connect.NewRequest(&pb.ListFavoritesRequest{PageToken: token}))
			wantCode(t, err, connect.CodeInvalidArgument, "")
		}
	}
	if token != "" || len(map[string]bool{got[0]: true, got[1]: true, got[2]: true}) != 3 {
		t.Fatalf("paging=%v token=%q", got, token)
	}
	otherToken, err := f.id.DevSession(ctx, "dev_other_buyer")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := f.id.Authenticate(ctx, otherToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.CurrentBuyerID(ctx, auth.Principal.ID); err != nil {
		t.Fatalf("another account has no buyer: %v", err)
	}
	other := buyerconnect.NewBuyerAppServiceClient(f.http, f.base, clientOptions(otherToken)...)
	_, err = other.ListAddresses(ctx, connect.NewRequest(&pb.ListAddressesRequest{PageToken: tokenForDifferentCaller(t, f, ctx)}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	for _, action := range []func(string) error{
		func(id string) error {
			_, err := other.GetAddress(ctx, connect.NewRequest(&pb.GetAddressRequest{Id: id}))
			return err
		},
		func(id string) error {
			a := address("竄改")
			a.Id, a.Etag = id, rows[0].GetEtag()
			_, err := other.UpdateAddress(ctx, connect.NewRequest(&pb.UpdateAddressRequest{Address: a}))
			return err
		},
		func(id string) error {
			_, err := other.DeleteAddress(ctx, connect.NewRequest(&pb.DeleteAddressRequest{Id: id}))
			return err
		},
	} {
		owned := action(rows[0].GetId())
		missing := action("999999999")
		wantCode(t, owned, connect.CodeNotFound, "")
		wantCode(t, missing, connect.CodeNotFound, "")
		if owned.Error() != missing.Error() {
			t.Fatalf("ownership leaked: owned=%v missing=%v", owned, missing)
		}
	}
}

func TestConnectPartialUpdatesAndRequiredClears(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("王小明甲")}))
	if err != nil {
		t.Fatal(err)
	}
	a := created.Msg.GetAddress()
	updated, err := f.app.UpdateAddress(ctx, connect.NewRequest(&pb.UpdateAddressRequest{
		Address: &pb.Address{
			Id: a.GetId(), Etag: a.GetEtag(),
			PostalAddress: &commonpb.PostalAddress{TownCode: "65000010", AddressDetail: "二號"},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"postal_address"}},
	}))
	if err != nil || updated.Msg.GetAddress().GetPostalAddress().GetTownCode() != "65000010" || updated.Msg.GetAddress().GetReceiverName() != a.GetReceiverName() {
		t.Fatalf("partial address=%v err=%v", updated, err)
	}
	a = updated.Msg.GetAddress()
	_, err = f.app.UpdateAddress(ctx, connect.NewRequest(&pb.UpdateAddressRequest{
		Address:    &pb.Address{Id: a.GetId(), Etag: a.GetEtag()},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"postal_address"}},
	}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "address.postal_address" {
		t.Fatalf("clear postal_address field=%q", field)
	}
	profile, err := f.app.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := f.app.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{
		Profile:    &pb.Profile{Etag: profile.Msg.GetProfile().GetEtag(), Phone: "0912345678"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"phone"}},
	}))
	if err != nil || changed.Msg.GetProfile().GetPhone() != "0912345678" || changed.Msg.GetProfile().GetDisplayName() != profile.Msg.GetProfile().GetDisplayName() {
		t.Fatalf("partial profile=%v err=%v", changed, err)
	}
	_, err = f.app.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{
		Profile:    &pb.Profile{Etag: changed.Msg.GetProfile().GetEtag()},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
	}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "profile.display_name" {
		t.Fatalf("clear display_name field=%q", field)
	}
}

func TestConnectDeletedStoreCardsAndUnfollow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.source.stores[3], f.source.owners[3] = openCard(3, "店家"), 999
	f.source.products[9] = httpbuyer.ProductInfo{ID: 9, StoreID: 3, Name: "蘋果", Available: true}
	if _, err := f.app.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{StoreId: "3"})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.AddFavorite(ctx, connect.NewRequest(&pb.AddFavoriteRequest{ProductId: "9"})); err != nil {
		t.Fatal(err)
	}
	f.source.summaries = []httpbuyer.MerchantSummary{{StoreID: 3, Spent: 100}}
	delete(f.source.stores, 3)
	follows, err := f.app.ListFollows(ctx, connect.NewRequest(&pb.ListFollowsRequest{}))
	if err != nil || len(follows.Msg.GetFollows()) != 1 || follows.Msg.GetFollows()[0].GetStore() != nil || follows.Msg.GetFollows()[0].GetStoreId() != "3" {
		t.Fatalf("follows=%v err=%v", follows, err)
	}
	members, err := f.app.ListMemberCards(ctx, connect.NewRequest(&pb.ListMemberCardsRequest{}))
	if err != nil || len(members.Msg.GetMemberCards()) != 1 || members.Msg.GetMemberCards()[0].GetStore() != nil || members.Msg.GetMemberCards()[0].GetStoreId() != "3" {
		t.Fatalf("members=%v err=%v", members, err)
	}
	favorites, err := f.app.ListFavorites(ctx, connect.NewRequest(&pb.ListFavoritesRequest{}))
	if err != nil || len(favorites.Msg.GetFavorites()) != 1 || favorites.Msg.GetFavorites()[0].GetProduct().GetStore() != nil || favorites.Msg.GetFavorites()[0].GetProduct().GetStoreId() != "3" || favorites.Msg.GetFavorites()[0].GetProduct().GetAvailable() {
		t.Fatalf("favorites=%v err=%v", favorites, err)
	}
	if _, err := f.app.UnfollowStore(ctx, connect.NewRequest(&pb.UnfollowStoreRequest{StoreId: "3"})); err != nil {
		t.Fatal(err)
	}
	f.source.storesErr = errors.New("seller unavailable")
	_, err = f.app.ListFavorites(ctx, connect.NewRequest(&pb.ListFavoritesRequest{}))
	wantCode(t, err, connect.CodeUnavailable, "")
}

func tokenForDifferentCaller(t *testing.T, f *fixture, ctx context.Context) string {
	t.Helper()
	resp, err := f.app.ListAddresses(ctx, connect.NewRequest(&pb.ListAddressesRequest{PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetNextPageToken()
}

func TestConnectPendingAndProductCards(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.source.stores[3], f.source.owners[3] = openCard(3, "店家"), 999
	f.source.stores[4], f.source.owners[4] = openCard(4, "自己"), f.accountID
	f.source.products[9] = httpbuyer.ProductInfo{ID: 9, StoreID: 3, Name: "蘋果", MinPrice: 10, MaxPrice: 10, Available: true}
	_, err := f.app.AddFavorite(ctx, connect.NewRequest(&pb.AddFavoriteRequest{ProductId: "9"}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.AddFavorite(ctx, connect.NewRequest(&pb.AddFavoriteRequest{ProductId: "10"}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.SubscribeRestock(ctx, connect.NewRequest(&pb.SubscribeRestockRequest{ProductId: "9"}))
	if err != nil {
		t.Fatal(err)
	}
	favorites, err := f.app.ListFavorites(ctx, connect.NewRequest(&pb.ListFavoritesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if favorites.Msg.GetTotalSize() != 2 {
		t.Fatalf("total=%d", favorites.Msg.GetTotalSize())
	}
	for _, item := range favorites.Msg.GetFavorites() {
		switch item.GetProduct().GetId() {
		case "9":
			if !item.GetProduct().GetRestockSubscribed() {
				t.Fatal("restock missing")
			}
		case "10":
			if !item.GetProduct().GetDeleted() || !proto.Equal(item.GetProduct(), &pb.ProductCard{Id: "10", Deleted: true}) {
				t.Fatalf("tombstone=%v", item.GetProduct())
			}
		default:
			t.Fatalf("unexpected product %v", item.GetProduct())
		}
	}
	_, err = f.app.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{StoreId: "4"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	_, err = f.app.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{StoreId: "999"}))
	wantCode(t, err, connect.CodeNotFound, "")
	if err := f.module.BeginDeletion(ctx, f.buyerID); err != nil {
		t.Fatal(err)
	}
	_, err = f.app.AddFavorite(ctx, connect.NewRequest(&pb.AddFavoriteRequest{ProductId: "11"}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	_, err = f.app.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{StoreId: "3"}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
}

func TestConnectDeleteAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	resp, err := f.identity.DeleteAccount(context.Background(), connect.NewRequest(&identitypb.DeleteAccountRequest{}))
	if err != nil || resp.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_DELETED {
		t.Fatalf("delete=%v err=%v", resp, err)
	}
	if account, err := f.id.Account(context.Background(), f.accountID); err != nil || !account.Deleted {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	// 關閉買家唯一的路徑是 accountdeletion 在刪帳的 transaction 內呼叫 CloseForDeletion（buyer 不訂閱 AccountDeleted）。
	if ds, err := f.module.BatchDisplay(context.Background(), []int64{f.buyerID}); err != nil || len(ds) != 1 || !ds[0].Deleted {
		t.Fatalf("帳號刪了、買家沒關：display=%+v err=%v", ds, err)
	}
}

func requiredPaths(desc protoreflect.MessageDescriptor, prefix []protoreflect.Name) [][]protoreflect.Name {
	var out [][]protoreflect.Name
	for i := range desc.Fields().Len() {
		fd := desc.Fields().Get(i)
		if !rpc.HasBehavior(fd, annotations.FieldBehavior_REQUIRED) {
			continue
		}
		path := append(append([]protoreflect.Name(nil), prefix...), fd.Name())
		out = append(out, path)
		if fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() {
			out = append(out, requiredPaths(fd.Message(), path)...)
		}
	}
	return out
}

func clearPath(msg protoreflect.Message, path []protoreflect.Name) {
	for _, name := range path[:len(path)-1] {
		msg = msg.Mutable(msg.Descriptor().Fields().ByName(name)).Message()
	}
	msg.Clear(msg.Descriptor().Fields().ByName(path[len(path)-1]))
}

func pathName(path []protoreflect.Name) string {
	result := ""
	for i, name := range path {
		if i > 0 {
			result += "."
		}
		result += string(name)
	}
	return result
}

func TestRequiredFieldsAreEnforced(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("原名")}))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := f.app.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	plans, err := f.admin.ListMembershipPlans(ctx, connect.NewRequest(&adminpb.ListMembershipPlansRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		valid func() proto.Message
		call  func(proto.Message) error
	}{
		{func() proto.Message { return &pb.CreateAddressRequest{Address: address("新名")} }, func(m proto.Message) error {
			_, e := f.app.CreateAddress(ctx, connect.NewRequest(m.(*pb.CreateAddressRequest)))
			return e
		}},
		{func() proto.Message {
			a := proto.Clone(created.Msg.GetAddress()).(*pb.Address)
			return &pb.UpdateAddressRequest{Address: a}
		}, func(m proto.Message) error {
			_, e := f.app.UpdateAddress(ctx, connect.NewRequest(m.(*pb.UpdateAddressRequest)))
			return e
		}},
		{func() proto.Message {
			return &pb.UpdateProfileRequest{Profile: &pb.Profile{DisplayName: "暱稱", Etag: profile.Msg.GetProfile().GetEtag()}}
		}, func(m proto.Message) error {
			_, e := f.app.UpdateProfile(ctx, connect.NewRequest(m.(*pb.UpdateProfileRequest)))
			return e
		}},
		{func() proto.Message {
			return &adminpb.UpdateMembershipPlanRequest{MembershipPlan: proto.Clone(plans.Msg.GetMembershipPlans()[0]).(*adminpb.MembershipPlan)}
		}, func(m proto.Message) error {
			_, e := f.admin.UpdateMembershipPlan(ctx, connect.NewRequest(m.(*adminpb.UpdateMembershipPlanRequest)))
			return e
		}},
	}
	checked := 0
	for _, c := range cases {
		for _, path := range requiredPaths(c.valid().ProtoReflect().Descriptor(), nil) {
			name := pathName(path)
			t.Run(string(c.valid().ProtoReflect().Descriptor().Name())+"."+name, func(t *testing.T) {
				msg := c.valid()
				clearPath(msg.ProtoReflect(), path)
				if len(path) == 2 {
					switch request := msg.(type) {
					case *pb.UpdateAddressRequest:
						request.UpdateMask = &fieldmaskpb.FieldMask{Paths: []string{string(path[1])}}
					case *pb.UpdateProfileRequest:
						request.UpdateMask = &fieldmaskpb.FieldMask{Paths: []string{string(path[1])}}
					}
				}
				if got := wantCode(t, c.call(msg), connect.CodeInvalidArgument, ""); got != name {
					t.Fatalf("field=%q want=%q", got, name)
				}
			})
			checked++
		}
	}
	if checked != 13 {
		t.Fatalf("want exactly 13 required fields, got %d", checked)
	}
}

// noDashboardOrders 是這個測試不讀儀表板的訂單時的替身：NewHandler 要求提供儀表板訂單的來源。
type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

// 追蹤的對象以 seller 模組判斷：開了店但還沒通過身分認證的店家不能追蹤，與不存在的店家同一個 not_found（不洩漏哪些店存在）。
func TestFollowStoreRejectsAStoreThatIsNotPubliclyExposable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.stores.current = f.seller
	ownerToken, err := f.id.DevSession(ctx, "dev_unfinished_store_owner")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := f.id.Authenticate(ctx, ownerToken)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := f.seller.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: owner.Principal.ID, StoreName: "還沒通過認證", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{StoreId: strconv.FormatInt(opened.StoreID, 10)}))
	known := wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.app.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{StoreId: "999999"}))
	if unknown := wantCode(t, err, connect.CodeNotFound, ""); known != unknown {
		t.Fatalf("不公開（%q）與不存在（%q）不同", known, unknown)
	}
	follows, err := f.app.ListFollows(ctx, connect.NewRequest(&pb.ListFollowsRequest{}))
	if err != nil || len(follows.Msg.GetFollows()) != 0 {
		t.Fatalf("follows=%v err=%v", follows, err)
	}
}

// ReadSecure 實作 media.Media；這些測試不讀機敏資產。
func (noMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}
