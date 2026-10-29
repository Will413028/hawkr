//go:build integration

package seller_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1"
	adminidentityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	adminsellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	adminsellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	identitypb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
	"uuid"
)

// adminOrders 是買家側與店家側未結訂單 port 的替身（同一個實作，與正式組裝相同）。
type adminOrders struct {
	mu                    sync.Mutex
	buyerOpen, storeOpen  bool
	storeAsked, buyerAsks []int64
}

func (o *adminOrders) set(buyerOpen, storeOpen bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buyerOpen, o.storeOpen = buyerOpen, storeOpen
}

func (o *adminOrders) HasOpenOrders(_ context.Context, buyerID int64) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buyerAsks = append(o.buyerAsks, buyerID)
	return o.buyerOpen, nil
}

func (o *adminOrders) HasOpenStoreOrders(_ context.Context, storeID int64) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.storeAsked = append(o.storeAsked, storeID)
	return o.storeOpen, nil
}

type adminFixture struct {
	admin adminsellerconnect.SellerAdminServiceClient
	// governance 是措施的 client（管理員）：GetMeasure 帶關聯帳號與各自的終止合作生效時機。
	governance adminsellerconnect.SellerGovernanceAdminServiceClient
	// prospects 是商家名單的 client：管理員、一般帳號、沒有登入；admin 用 GET 讀取（NO_SIDE_EFFECTS）。
	prospects         adminsellerconnect.SellerProspectAdminServiceClient
	prospectsCustomer adminsellerconnect.SellerProspectAdminServiceClient
	prospectsAnon     adminsellerconnect.SellerProspectAdminServiceClient
	customer          adminsellerconnect.SellerAdminServiceClient
	anonymous         adminsellerconnect.SellerAdminServiceClient
	identityAdmin     adminidentityconnect.IdentityAdminServiceClient
	identityOther     adminidentityconnect.IdentityAdminServiceClient
	identityAnon      adminidentityconnect.IdentityAdminServiceClient
	module            *seller.Module
	social            *social.Module
	id                *identity.Module
	deletion          *accountdeletion.Module
	db                *db.DB
	bus               *events.Bus
	orders            *adminOrders
	dev               *identity.DevAuth
	adminID           int64
	customerID        int64
	users             int
	server            *httptest.Server
	adminToken        string
	customerToken     string
	// otherAdminToken 是另一位管理員的 token（授權是一位管理員的，不能互相借用）。
	otherAdminToken string
	// media 是證件圖片的替身（機敏讀取只有 (object_key, ref_type, ref_id) 全等讀得到）。
	media *media.Fake
	// registry 是模組的登記資料來源（公司型申請送件時的查詢快照）的替身：回應照順序消耗，沒有排定的是查無。
	registry *moduleRegistry
	// skew 讓模組的時鐘往前走（不 sleep）：授權過期的測試用。
	skew atomic.Int64
}

// now 是模組的時鐘：真實時間加上 skew。
func (f *adminFixture) now() time.Time { return time.Now().Add(time.Duration(f.skew.Load())) }

func setupAdmin(t *testing.T) *adminFixture {
	t.Helper()
	f := &adminFixture{registry: &moduleRegistry{}}
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "buyer", Dir: buyer.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()}, migrate.Module{Name: "delegation", Dir: delegation.Migrations()}, migrate.Module{Name: "social", Dir: social.Migrations()},
	))
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
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	// 與正式組裝相同：買家在建帳的 transaction 內由 AccountCreated 建立。
	events.Subscribe(bus, buyers.OnAccountCreated)
	orders := &adminOrders{}
	kycKeys, kycMAC := sealtest.Keys(t)
	// 轉正回以 token 命名的資產（asset-<token>），object key 是 <ref_type>/<token>。
	mediaFake := media.NewFake()
	mediaFake.Key = func(refType, token string) string { return refType + "/" + token }
	m := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: id, Media: mediaFake, Orders: orders, KYCKeys: kycKeys, KYCMAC: kycMAC,
		ProductStores: noProducts{}, Registry: f.registry, Now: f.now, Logger: log,
	})
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Identity: id, Buyer: buyers, Seller: m, Orders: orders, Now: time.Now, Logger: log})
	socialModule := social.New(social.Deps{
		DB: d, Events: bus, Media: mediaFake, Jobs: queue, Buyers: buyers, Stores: m, StoreBlocks: m, NewID: uuid.NewV7, Now: f.now, Logger: log,
	})
	events.Subscribe(bus, socialModule.OnMeasureCreated)
	events.Subscribe(bus, socialModule.OnMeasureRevoked)
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{
		Ready: d.Ping, Identity: id, Seller: m, Social: socialModule, Buyer: buyers, Images: images, Logger: log, Deletion: deletion,
		Delegation: delegation.New(delegation.Deps{DB: d, Identity: id, Seller: m, Now: time.Now, Logger: log}), Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	adminToken, err := dev.AdminToken("dev_admin_connect")
	if err != nil {
		t.Fatal(err)
	}
	customerToken, err := dev.CustomerToken("dev_customer_connect")
	if err != nil {
		t.Fatal(err)
	}
	authenticatedAdmin, err := id.Authenticate(ctx, adminToken)
	if err != nil {
		t.Fatal(err)
	}
	authenticatedCustomer, err := id.Authenticate(ctx, customerToken)
	if err != nil {
		t.Fatal(err)
	}
	otherAdminToken, err := dev.AdminToken("dev_admin_other")
	if err != nil {
		t.Fatal(err)
	}
	characterize.GrantAdminOwner(t, id, adminToken)
	characterize.GrantAdminOwner(t, id, otherAdminToken)
	f.server, f.adminToken, f.customerToken, f.otherAdminToken, f.media = server, adminToken, customerToken, otherAdminToken, mediaFake
	f.admin = adminsellerconnect.NewSellerAdminServiceClient(server.Client(), server.URL, clientOptions(adminToken)...)
	f.governance = adminsellerconnect.NewSellerGovernanceAdminServiceClient(server.Client(), server.URL, clientOptions(adminToken)...)
	f.customer = adminsellerconnect.NewSellerAdminServiceClient(server.Client(), server.URL, clientOptions(customerToken)...)
	f.prospects = adminsellerconnect.NewSellerProspectAdminServiceClient(server.Client(), server.URL, append(clientOptions(adminToken), connect.WithHTTPGet())...)
	f.prospectsCustomer = adminsellerconnect.NewSellerProspectAdminServiceClient(server.Client(), server.URL, clientOptions(customerToken)...)
	f.prospectsAnon = adminsellerconnect.NewSellerProspectAdminServiceClient(server.Client(), server.URL)
	f.anonymous = adminsellerconnect.NewSellerAdminServiceClient(server.Client(), server.URL)
	f.identityAdmin = adminidentityconnect.NewIdentityAdminServiceClient(server.Client(), server.URL, clientOptions(adminToken)...)
	f.identityOther = adminidentityconnect.NewIdentityAdminServiceClient(server.Client(), server.URL, clientOptions(customerToken)...)
	f.identityAnon = adminidentityconnect.NewIdentityAdminServiceClient(server.Client(), server.URL)
	f.module, f.social, f.id, f.deletion, f.db, f.bus, f.orders, f.dev = m, socialModule, id, deletion, d, bus, orders, dev
	f.adminID, f.customerID = authenticatedAdmin.Principal.ID, authenticatedCustomer.Principal.ID
	return f
}

// newSeller 建立一個帳號（買家在建帳時建立），並為它開店；name 是店名。
func (f *adminFixture) newSeller(t *testing.T, name string) (int64, seller.Store) {
	t.Helper()
	f.users++
	token, err := f.dev.CustomerToken("dev_seller_" + strconv.Itoa(f.users))
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := f.id.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.module.Apply(context.Background(), seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: authenticated.Principal.ID, StoreName: name, TownCode: "63000050", AddressDetail: "測試路一號"})
	if err != nil {
		t.Fatal(err)
	}
	s, err = f.module.UpdateProfile(context.Background(), seller.UpdateProfileInput{
		StoreID: s.StoreID, ExpectedVersion: s.Version,
		ContactName: ptr("王小明"), ContactMobile: ptr("0912345678"), SupportEmail: ptr("owner@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return authenticated.Principal.ID, s
}

func (f *adminFixture) scan(t *testing.T, dest any, query string, args ...any) {
	t.Helper()
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), query, args...).Scan(dest); err != nil {
		t.Fatal(err)
	}
}

func sid(id int64) string { return strconv.FormatInt(id, 10) }

// 後台的 service 只有管理員能呼叫：沒有登入是 unauthenticated，買家帳號是 permission_denied，而且什麼都不會被寫入。
func TestAdminServicesRejectCustomersAndVisitorsAndWriteNothing(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	victimAccount, victim := f.newSeller(t, "不該被動到的店")
	for name, c := range map[string]adminsellerconnect.SellerAdminServiceClient{"customer": f.customer, "visitor": f.anonymous} {
		want := connect.CodePermissionDenied
		if name == "visitor" {
			want = connect.CodeUnauthenticated
		}
		calls := map[string]func() error{
			"ListStores": func() error {
				_, err := c.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{}))
				return err
			},
			"GetStoreStats": func() error {
				_, err := c.GetStoreStats(ctx, connect.NewRequest(&adminsellerpb.GetStoreStatsRequest{}))
				return err
			},
			"GetStore": func() error {
				_, err := c.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(victim.StoreID)}))
				return err
			},
		}
		for method, call := range calls {
			if err := call(); connect.CodeOf(err) != want {
				t.Fatalf("%s %s: err=%v want=%v", name, method, err, want)
			}
		}
	}
	for name, c := range map[string]adminidentityconnect.IdentityAdminServiceClient{"customer": f.identityOther, "visitor": f.identityAnon} {
		want := connect.CodePermissionDenied
		if name == "visitor" {
			want = connect.CodeUnauthenticated
		}
		_, err := c.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(victimAccount), Reason: "越權"}))
		if connect.CodeOf(err) != want {
			t.Fatalf("%s DeleteAccount: err=%v want=%v", name, err, want)
		}
	}
	// 什麼都沒有被動到。
	got, err := f.module.Store(ctx, victim.StoreID)
	if err != nil {
		t.Fatal(err)
	}
	account, err := f.id.Account(ctx, victimAccount)
	if err != nil {
		t.Fatal(err)
	}
	var requests int64
	f.scan(t, &requests, "SELECT count(*) FROM accountdeletion.deletion_requests")
	if got.Status != seller.Active || got.Version != victim.Version || got.Suspended() || got.Closure != nil || account.Deleted || requests != 0 {
		t.Fatalf("store=%+v account=%+v requests=%d", got, account, requests)
	}
}

// 終止合作生效後系統關店：後台的店家讀取帶關店的紀錄（原因 TERMINATION 與造成關店的措施，沒有管理員與理由——它們在措施裡），清單預設不含已關閉的，
// 明列才有。後台沒有直接關店的方法。
func TestAdminReadsTheTerminationClosureOfAStore(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, s := f.newSeller(t, "要終止的店")
	created, err := f.module.CreateMeasure(ctx, seller.CreateMeasureInput{
		AdminID: f.adminID, StoreID: s.StoreID, Kind: seller.MeasureTermination, StatementToSeller: "停業", Note: "已確認",
		Termination: &seller.TerminationBasis{Ground: seller.GroundCeasedBusiness},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.orders.set(false, true)
	if err := f.module.EffectTermination(ctx, created.Measure.ID); err != nil {
		t.Fatal(err)
	}
	got, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(s.StoreID)}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetStore().GetStatus() != pb.StoreStatus_STORE_STATUS_ACTIVE || got.Msg.GetStore().GetClosure() != nil {
		t.Fatalf("有未結訂單不關店：%+v", got.Msg.GetStore())
	}
	if grounds := terminationGrounds(got.Msg); !got.Msg.GetCanCreateTermination() || !slices.Contains(grounds, pb.TerminationGround_TERMINATION_GROUND_NOTICE) ||
		slices.Contains(grounds, pb.TerminationGround_TERMINATION_GROUND_LINKED_ACCOUNT) {
		t.Fatalf("開著的店可以建立終止（含預告、不含關聯帳號）：%v", grounds)
	}
	// 每個可建立的終止合作一筆（事由不重複），只有預告終止是排程生效：生效時間是讀取當下加 30 日、說明句逐字（含台北時間），其他事由立即生效、不帶時間。
	seen := map[pb.TerminationGround]bool{}
	for _, o := range got.Msg.GetAvailableTerminations() {
		if seen[o.GetGround()] {
			t.Fatalf("事由 %v 重複：%v", o.GetGround(), got.Msg.GetAvailableTerminations())
		}
		seen[o.GetGround()] = true
		if o.GetExplanation().GetLocale() != "zh-TW" {
			t.Fatalf("事由 %v 的說明句語系 = %q，期望 zh-TW", o.GetGround(), o.GetExplanation().GetLocale())
		}
		// 依據的需求與說明句都由事由決定（model.TerminationGround.ViolationRequirement、TerminationEffect.Message）。
		wantReq := adminsellerpb.TerminationViolationRequirement_TERMINATION_VIOLATION_REQUIREMENT_OPTIONAL
		switch o.GetGround() {
		case pb.TerminationGround_TERMINATION_GROUND_NOTICE:
			wantReq = adminsellerpb.TerminationViolationRequirement_TERMINATION_VIOLATION_REQUIREMENT_FORBIDDEN
		case pb.TerminationGround_TERMINATION_GROUND_MATERIAL_BREACH:
			wantReq = adminsellerpb.TerminationViolationRequirement_TERMINATION_VIOLATION_REQUIREMENT_REQUIRED
		}
		if o.GetViolationRequirement() != wantReq {
			t.Fatalf("事由 %v 的 violation_requirement = %v，期望 %v", o.GetGround(), o.GetViolationRequirement(), wantReq)
		}
		if o.GetGround() != pb.TerminationGround_TERMINATION_GROUND_NOTICE {
			wantMsg := "立即生效：店家暫停營業。"
			if o.GetGround() == pb.TerminationGround_TERMINATION_GROUND_MATERIAL_BREACH {
				wantMsg = "立即生效：店家暫停營業。必須引用至少一筆未撤銷、已存在 7 日以上的認定。"
			}
			if o.GetEffectiveTime() != nil || o.GetExplanation().GetMessage() != wantMsg {
				t.Fatalf("事由 %v 要立即生效、說明句 %q：%+v", o.GetGround(), wantMsg, o)
			}
			continue
		}
		at := o.GetEffectiveTime().AsTime()
		if d := at.Sub(f.now().Add(720 * time.Hour)); d < -time.Minute || d > time.Minute {
			t.Fatalf("預告終止的生效時間 %v 要是現在加 30 日：偏差 %v", at, d)
		}
		want := "建立後 30 天生效（" + uitext.FormatDateTime(uitext.ZhTW, at) + "）。期間店家照常營業，賣家可提出申覆。"
		if o.GetExplanation().GetMessage() != want {
			t.Fatalf("預告終止的說明句 = %q，期望 %q", o.GetExplanation().GetMessage(), want)
		}
	}
	if !seen[pb.TerminationGround_TERMINATION_GROUND_NOTICE] {
		t.Fatalf("沒有預告終止：%v", got.Msg.GetAvailableTerminations())
	}
	f.orders.set(false, false)
	if err := f.module.EffectTermination(ctx, created.Measure.ID); err != nil {
		t.Fatal(err)
	}
	got, err = f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(s.StoreID)}))
	if err != nil {
		t.Fatal(err)
	}
	st := got.Msg.GetStore()
	if st.GetStatus() != pb.StoreStatus_STORE_STATUS_CLOSED || st.GetCloseTime() == nil ||
		st.GetClosure().GetCloser() != adminsellerpb.StoreCloser_STORE_CLOSER_TERMINATION || st.GetClosure().GetMeasureId() != created.Measure.ID.String() {
		t.Fatalf("store=%+v", st)
	}
	if len(got.Msg.GetActiveMeasures()) != 1 || got.Msg.GetActiveMeasures()[0].GetKind() != pb.MeasureKind_MEASURE_KIND_TERMINATION {
		t.Fatalf("active measures=%+v", got.Msg.GetActiveMeasures())
	}
	if grounds := terminationGrounds(got.Msg); !got.Msg.GetCanCreateTermination() || len(grounds) != 9 ||
		slices.Contains(grounds, pb.TerminationGround_TERMINATION_GROUND_NOTICE) {
		t.Fatalf("已關閉的店仍可建立 21.3 事由（T5b），預告終止不行：%v", grounds)
	}
	account, err := f.id.Account(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(account.Roles, identity.RoleSeller) || account.Deleted {
		t.Fatalf("closing the store must only remove the seller role: %+v", account)
	}
	list, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Msg.GetStores() {
		if item.GetStore().GetId() == sid(s.StoreID) {
			t.Fatal("closed store listed by default")
		}
	}
	list, err = f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{Status: pb.StoreStatus_STORE_STATUS_CLOSED}))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Msg.GetStores()) != 1 || list.Msg.GetStores()[0].GetStore().GetClosure().GetMeasureId() != created.Measure.ID.String() {
		t.Fatalf("closed list=%+v", list.Msg.GetStores())
	}
}

func TestListStoresPagesOverConnectAndGetStoreCarriesTheKYCSummary(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	var want []string
	for _, name := range []string{"甲店", "乙店", "丙店", "丁店", "戊店"} {
		_, s := f.newSeller(t, name)
		want = append([]string{sid(s.StoreID)}, want...)
	}
	var got []string
	token := ""
	for pages := 0; pages < 10; pages++ {
		resp, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{PageSize: 2, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range resp.Msg.GetStores() {
			got = append(got, item.GetStore().GetId())
			if item.GetKycFlowStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_NOT_STARTED || item.GetSuspended() {
				t.Fatalf("item=%+v", item)
			}
		}
		if token = resp.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	// 換了篩選就不能沿用 token。
	first, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{PageSize: 2}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{PageSize: 2, Query: "店", PageToken: first.Msg.GetNextPageToken()}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	// 草稿不能當篩選。
	_, err = f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{KycStatus: pb.KYCStatus_KYC_STATUS_DRAFT}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "kyc_status" {
		t.Fatalf("field=%q", field)
	}
	// 「待審核」不是儲存的狀態：SUBMITTED 被拒絕，要用 kyc_awaiting_review（待辦的成員，與 kyc_awaiting_review_stores 同一個）。
	_, err = f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{KycStatus: pb.KYCStatus_KYC_STATUS_SUBMITTED}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "kyc_status" {
		t.Fatalf("field=%q", field)
	}
	awaiting, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{KycAwaitingReview: true}))
	if err != nil || len(awaiting.Msg.GetStores()) != 0 {
		t.Fatalf("awaiting=%v err=%v", awaiting.Msg, err)
	}
	// 營運中的店：代行的店家狀態原因是空的（空不表示能代行，授權在 preflight）。
	first0, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{PageSize: 1}))
	if err != nil || len(first0.Msg.GetStores()) != 1 || len(first0.Msg.GetStores()[0].GetDelegationUnavailabilities()) != 0 {
		t.Fatalf("delegation=%v err=%v", first0.Msg, err)
	}
	stats, err := f.admin.GetStoreStats(ctx, connect.NewRequest(&adminsellerpb.GetStoreStatsRequest{}))
	if err != nil || stats.Msg.GetTotalStores() != 5 || stats.Msg.GetNewStoresThisMonth() != 5 || stats.Msg.GetKycAwaitingReviewStores() != 0 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}

	// KYC 摘要：沒有送出過的申請（沒有、草稿）不設；送出之後有。
	accountID, s := f.newSeller(t, "有申請的店")
	noKYC, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(s.StoreID)}))
	if err != nil || noKYC.Msg.GetKyc() != nil {
		t.Fatalf("kyc=%+v err=%v", noKYC.Msg.GetKyc(), err)
	}
	draft, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{
		AccountID: accountID, SellerType: ptr(seller.KYCIndividual), RealName: ptr("王小明"), BirthDate: ptr("1990-05-06"), IDCardNo: ptr("A123456789"),
		EnglishName: ptr("Wang Farm"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr("012345678901"),
		ProductType: ptr("蔬果"),
		IDCardFront: &seller.KYCDocumentRef{StagedToken: "front"}, IDCardBack: &seller.KYCDocumentRef{StagedToken: "back"}, BankBook: &seller.KYCDocumentRef{StagedToken: "book"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(s.StoreID)})); err != nil || got.Msg.GetKyc() != nil {
		t.Fatalf("draft must stay hidden: %+v err=%v", got, err)
	}
	submitted, err := f.module.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version})
	if err != nil {
		t.Fatal(err)
	}
	withKYC, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(s.StoreID)}))
	if err != nil {
		t.Fatal(err)
	}
	k := withKYC.Msg.GetKyc()
	// 審核期限：審核中才有，開始是送出時間，期限 7 天（seller.review_kyc_application）。
	if d := k.GetDecideDeadline(); !d.GetStartTime().AsTime().Equal(k.GetSubmitTime().AsTime()) || !d.GetDueTime().AsTime().Equal(k.GetSubmitTime().AsTime().Add(7*24*time.Hour)) {
		t.Fatalf("decide_deadline=%+v submit_time=%v", d, k.GetSubmitTime())
	}
	if k.GetApplicationId() != submitted.ID.String() || k.GetStatus() != pb.KYCStatus_KYC_STATUS_SUBMITTED || k.GetFlowStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_SUBMITTED || k.GetSubmitTime() == nil || k.GetDecideTime() != nil {
		t.Fatalf("kyc=%+v", k)
	}
	listed, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{KycAwaitingReview: true}))
	if err != nil || len(listed.Msg.GetStores()) != 1 || listed.Msg.GetStores()[0].GetStore().GetId() != sid(s.StoreID) {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	// 不存在的店，與格式錯誤的 id。
	_, err = f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: "987654"}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: "not-a-number"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

func ptr[T any](v T) *T { return &v }

type deletionState struct {
	accountDeleted bool
	storeStatus    seller.Status
	sellerRole     bool
	buyerStatus    string
	requests       int64
	attempts       int64
}

func (f *adminFixture) deletionState(t *testing.T, accountID, storeID int64) deletionState {
	t.Helper()
	ctx := context.Background()
	account, err := f.id.Account(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.module.Store(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	out := deletionState{accountDeleted: account.Deleted, storeStatus: st.Status, sellerRole: slices.Contains(account.Roles, identity.RoleSeller)}
	f.scan(t, &out.buyerStatus, "SELECT status FROM buyer.buyers WHERE account_id = $1", accountID)
	f.scan(t, &out.requests, "SELECT count(*) FROM accountdeletion.deletion_requests WHERE account_id = $1", accountID)
	f.scan(t, &out.attempts, "SELECT count(*) FROM accountdeletion.request_attempts a JOIN accountdeletion.deletion_requests r ON r.id = a.request_id WHERE r.account_id = $1", accountID)
	return out
}

// DeleteAccount：以管理員為申請者，買家、店家與帳號在同一個 transaction 內關閉與刪除，申請紀錄記下管理員與理由。
func TestDeleteAccountClosesBuyerStoreAndAccountTogetherAndRecordsTheAdmin(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, s := f.newSeller(t, "要刪的店")

	resp, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: "  使用者要求  "}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_DELETED {
		t.Fatalf("state=%v", resp.Msg.GetState())
	}
	got := f.deletionState(t, accountID, s.StoreID)
	if !got.accountDeleted || got.storeStatus != seller.Closed || got.sellerRole || got.buyerStatus != "CLOSED" || got.requests != 1 || got.attempts != 1 {
		t.Fatalf("state=%+v", got)
	}
	var requester, reason string
	var operator int64
	f.scan(t, &requester, "SELECT requester FROM accountdeletion.request_attempts ORDER BY attempted_at DESC LIMIT 1")
	f.scan(t, &operator, "SELECT operator_admin_id FROM accountdeletion.request_attempts ORDER BY attempted_at DESC LIMIT 1")
	f.scan(t, &reason, "SELECT reason FROM accountdeletion.request_attempts ORDER BY attempted_at DESC LIMIT 1")
	if requester != "ADMIN" || operator != f.adminID || reason != "使用者要求" {
		t.Fatalf("attempt: requester=%s operator=%d reason=%q", requester, operator, reason)
	}
	closed, err := f.module.Store(ctx, s.StoreID)
	if err != nil || closed.Closure == nil || closed.Closure.By != seller.ClosedByDeletion {
		t.Fatalf("closure=%+v err=%v", closed.Closure, err)
	}
	// 重送：帳號已經刪除，回已刪除，不再多一筆申請。
	again, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: "重送"}))
	if err != nil || again.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_DELETED {
		t.Fatalf("again=%+v err=%v", again, err)
	}
}

// 有未結訂單時帳號、買家與店家一起進入等待（管理員也不能略過），店家等待期間連管理員也不能單獨關店。
func TestDeleteAccountWaitsForOpenStoreOrdersEvenForAnAdmin(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, s := f.newSeller(t, "有訂單的店")
	f.orders.set(false, true)

	resp, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: "使用者要求"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_PENDING_ORDERS {
		t.Fatalf("state=%v", resp.Msg.GetState())
	}
	got := f.deletionState(t, accountID, s.StoreID)
	if got.accountDeleted || got.storeStatus != seller.PendingDeletion || !got.sellerRole || got.buyerStatus == "CLOSED" || got.requests != 1 || got.attempts != 1 {
		t.Fatalf("state=%+v", got)
	}
	// 再申請一次：仍是等待，申請紀錄追加一筆，流程只有一筆。
	again, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: "再試"}))
	if err != nil || again.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_PENDING_ORDERS {
		t.Fatalf("again=%+v err=%v", again, err)
	}
	if got := f.deletionState(t, accountID, s.StoreID); got.requests != 1 || got.attempts != 2 {
		t.Fatalf("state=%+v", got)
	}
	// 最後一筆訂單完成：流程完成，同一個 transaction 關店與刪帳號。
	f.orders.set(false, false)
	state, err := f.deletion.CompletePending(ctx, accountID)
	if err != nil || state != accountdeletion.Completed {
		t.Fatalf("state=%v err=%v", state, err)
	}
	if got := f.deletionState(t, accountID, s.StoreID); !got.accountDeleted || got.storeStatus != seller.Closed || got.buyerStatus != "CLOSED" {
		t.Fatalf("state=%+v", got)
	}
}

// 同一個 transaction：關店的最後一步失敗（StoreClosed 的訂閱者回錯誤）時，買家、店家、帳號與申請紀錄整個回滾。
func TestDeleteAccountRollsEverythingBackWhenAParticipantFails(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, s := f.newSeller(t, "中途失敗的店")
	var mu sync.Mutex
	failing := true
	events.Subscribe(f.bus, func(_ context.Context, _ seller.StoreClosed) error {
		mu.Lock()
		defer mu.Unlock()
		if failing {
			return errors.New("下游失敗")
		}
		return nil
	})

	_, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: "使用者要求"}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("err=%v", err)
	}
	got := f.deletionState(t, accountID, s.StoreID)
	if got.accountDeleted || got.storeStatus != seller.Active || !got.sellerRole || got.buyerStatus == "CLOSED" || got.requests != 0 || got.attempts != 0 {
		t.Fatalf("a failed deletion left state behind: %+v", got)
	}
	read, err := f.module.Store(ctx, s.StoreID)
	if err != nil || read.Closure != nil || read.Version != s.Version {
		t.Fatalf("store=%+v err=%v", read, err)
	}

	mu.Lock()
	failing = false
	mu.Unlock()
	resp, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: "使用者要求"}))
	if err != nil || resp.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_DELETED {
		t.Fatalf("retry: %+v err=%v", resp, err)
	}
}

func TestDeleteAccountValidatesItsInput(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, s := f.newSeller(t, "不該被刪的店")
	for name, tt := range map[string]struct {
		req   *adminpb.DeleteAccountRequest
		field string
	}{
		"no reason":    {&adminpb.DeleteAccountRequest{AccountId: sid(accountID)}, "reason"},
		"blank reason": {&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: " \t"}, "reason"},
		"long reason":  {&adminpb.DeleteAccountRequest{AccountId: sid(accountID), Reason: strings.Repeat("理", 1001)}, "reason"},
		"bad account":  {&adminpb.DeleteAccountRequest{AccountId: "abc", Reason: "x"}, "account_id"},
		"zero account": {&adminpb.DeleteAccountRequest{AccountId: "0", Reason: "x"}, "account_id"},
	} {
		_, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(tt.req))
		if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != tt.field {
			t.Fatalf("%s: field=%q", name, field)
		}
	}
	_, err := f.identityAdmin.DeleteAccount(ctx, connect.NewRequest(&adminpb.DeleteAccountRequest{AccountId: "987654", Reason: "x"}))
	wantCode(t, err, connect.CodeNotFound, "")
	if got := f.deletionState(t, accountID, s.StoreID); got.accountDeleted || got.requests != 0 {
		t.Fatalf("state=%+v", got)
	}
}

type moduleRegistry struct {
	mu      sync.Mutex
	replies []moduleRegistryReply
}

type moduleRegistryReply struct {
	reg   gcis.Registration
	found bool
	err   error
}

func (r *moduleRegistry) reply(reg gcis.Registration, found bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies = append(r.replies, moduleRegistryReply{reg, found, err})
}

func (r *moduleRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.replies) == 0 {
		return gcis.Registration{}, false, nil
	}
	reply := r.replies[0]
	r.replies = r.replies[1:]
	return reply.reg, reply.found, reply.err
}

func (f *adminFixture) suspendWithRisk(t *testing.T, storeID int64) uuid.UUID {
	t.Helper()
	res, err := f.module.CreateMeasure(context.Background(), seller.CreateMeasureInput{
		AdminID: f.adminID, StoreID: storeID, Kind: seller.MeasureSuspension, StatementToSeller: "風險處置", ReviewBy: ptr(f.now().Add(7 * 24 * time.Hour)), Note: "內部",
		Risk: &seller.RiskBasis{Ground: seller.RiskFraud},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Measure.ID
}

// 措施記在舊 stint、帳號目前的 stint 繼承停權時，後台的 GetStore 仍帶出生效中的措施，ListStores 的 suspended 篩選與列上的
// suspended 與它一致；措施被撤銷之後三者一起變。
func TestGetStoreAndListStoresShowTheEffectiveMeasureRecordedOnAnEarlierStint(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, old := f.newSeller(t, "舊店")
	_, bystander := f.newSeller(t, "旁人的店")
	id := f.suspendWithRisk(t, old.StoreID)
	if _, err := f.module.CloseCurrent(ctx, accountID); err != nil {
		t.Fatal(err)
	}
	var newID int64
	if err := f.db.Conn(ctx).QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, '新店', now(), now()) RETURNING store_id`,
		old.StoreCode+1, accountID).Scan(&newID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.store_exposures (store_id) VALUES ($1)", newID); err != nil {
		t.Fatal(err)
	}
	got, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(newID)}))
	if err != nil {
		t.Fatal(err)
	}
	measures := got.Msg.GetActiveMeasures()
	if len(measures) != 1 || measures[0].GetId() != id.String() || measures[0].GetState() != pb.MeasureState_MEASURE_STATE_ACTIVE {
		t.Fatalf("active_measures=%v", measures)
	}
	suspendedList := func(filter bool) []string {
		resp, err := f.admin.ListStores(ctx, connect.NewRequest(&adminsellerpb.ListStoresRequest{Suspended: &filter, PageSize: 50}))
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, item := range resp.Msg.GetStores() {
			if item.GetSuspended() != filter {
				t.Fatalf("filter=%v item=%v", filter, item)
			}
			ids = append(ids, item.GetStore().GetId())
		}
		slices.Sort(ids)
		return ids
	}
	if suspended := suspendedList(true); !slices.Contains(suspended, sid(newID)) || slices.Contains(suspended, sid(bystander.StoreID)) {
		t.Fatalf("suspended=%v", suspended)
	}
	if free := suspendedList(false); slices.Contains(free, sid(newID)) || !slices.Contains(free, sid(bystander.StoreID)) {
		t.Fatalf("not suspended=%v", free)
	}
	if _, err := f.module.RevokeMeasure(ctx, seller.RevokeMeasureInput{AdminID: f.adminID, MeasureID: id, ExpectedVersion: 1, Reason: seller.RevokeAdminCorrection}); err != nil {
		t.Fatal(err)
	}
	after, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(newID)}))
	if err != nil || len(after.Msg.GetActiveMeasures()) != 0 {
		t.Fatalf("after=%v err=%v", after, err)
	}
	if suspended := suspendedList(true); slices.Contains(suspended, sid(newID)) {
		t.Fatalf("revoked but still listed: %v", suspended)
	}
}

// GetStore 的 active_measures 只帶生效中的停權措施：已結束、已撤銷、尚未開始以外的警告都不在；兩個並存的停權措施都帶出。
func TestGetStoreListsEveryEffectiveSuspensionOfTheAccountAndNothingElse(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, s := f.newSeller(t, "多筆措施的店")
	a := f.suspendWithRisk(t, s.StoreID)
	b := f.suspendWithRisk(t, s.StoreID)
	warning, err := f.module.CreateMeasure(ctx, seller.CreateMeasureInput{
		AdminID: f.adminID, StoreID: s.StoreID, Kind: seller.MeasureWarning, StatementToSeller: "警告", Note: "內部",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.MisleadingInfo, Severity: seller.SeverityLow}},
	})
	if err != nil {
		t.Fatal(err)
	}
	revoked := f.suspendWithRisk(t, s.StoreID)
	if _, err := f.module.RevokeMeasure(ctx, seller.RevokeMeasureInput{AdminID: f.adminID, MeasureID: revoked, ExpectedVersion: 1, Reason: seller.RevokeAdminCorrection}); err != nil {
		t.Fatal(err)
	}
	got, err := f.admin.GetStore(ctx, connect.NewRequest(&adminsellerpb.GetStoreRequest{Id: sid(s.StoreID)}))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range got.Msg.GetActiveMeasures() {
		ids = append(ids, m.GetId())
	}
	want := []string{a.String(), b.String()}
	slices.Sort(ids)
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("active_measures=%v want=%v (warning=%s revoked=%s)", ids, want, warning.Measure.ID, revoked)
	}
}

// terminationGrounds 是 GetStore 回應可建立的終止合作的事由（依回應順序）。
func terminationGrounds(r *adminsellerpb.GetStoreResponse) []pb.TerminationGround {
	out := make([]pb.TerminationGround, 0, len(r.GetAvailableTerminations()))
	for _, o := range r.GetAvailableTerminations() {
		out = append(out, o.GetGround())
	}
	return out
}

// 措施詳情的關聯帳號帶「建立關聯終止」的生效時機：可以建立的那列有 termination_option（事由 LINKED_ACCOUNT、立即生效不帶時間、zh-TW 的說明句逐字），
// 已建立關聯終止的那列（不能再建立）不設。
// mutation：toLinkedAccount 不設 TerminationOption → 可建立的那列斷言失敗；toTerminationOption 不帶 Explanation → 說明句斷言失敗；
// linkedAccounts 對不能建立的列也填效果 → 已建立的那列斷言失敗。
func TestGetMeasureLinkedAccountCarriesTheLinkedTerminationOption(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, sA := f.newSeller(t, "來源店")
	_, sB := f.newSeller(t, "同身分的店")
	_, sC := f.newSeller(t, "同身分的另一店")
	for i, s := range []seller.Store{sA, sB, sC} {
		f.submitKYC(t, s.StoreID, "A123456789", []string{"012345678901", "999888777666", "555444333222"}[i])
	}
	source, err := f.module.CreateMeasure(ctx, seller.CreateMeasureInput{
		AdminID: f.adminID, StoreID: sA.StoreID, Kind: seller.MeasureTermination, StatementToSeller: "終止合作", Note: "內部",
		Termination: &seller.TerminationBasis{Ground: seller.GroundFraud},
	})
	if err != nil {
		t.Fatal(err)
	}
	read := func() map[string]*adminsellerpb.LinkedAccount {
		got, err := f.governance.GetMeasure(ctx, connect.NewRequest(&adminsellerpb.GetMeasureRequest{Id: source.Measure.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]*adminsellerpb.LinkedAccount{}
		for _, l := range got.Msg.GetMeasure().GetLinkedAccounts() {
			out[l.GetStoreId()] = l
		}
		return out
	}
	linked := read()
	if len(linked) != 2 {
		t.Fatalf("同身分的兩個帳號都列出：%v", linked)
	}
	for _, s := range []seller.Store{sB, sC} {
		l := linked[sid(s.StoreID)]
		o := l.GetTerminationOption()
		if !l.GetCanCreateLinkedTermination() || o.GetGround() != pb.TerminationGround_TERMINATION_GROUND_LINKED_ACCOUNT || o.GetEffectiveTime() != nil ||
			o.GetExplanation().GetLocale() != "zh-TW" ||
			o.GetExplanation().GetMessage() != "立即生效：店家暫停營業。來源措施撤銷時，這筆會一併撤銷。賣家可對這筆單獨申覆。" ||
			o.GetViolationRequirement() != adminsellerpb.TerminationViolationRequirement_TERMINATION_VIOLATION_REQUIREMENT_FORBIDDEN {
			t.Fatalf("可建立的關聯帳號 %d 要帶立即生效的選項：%+v", s.StoreID, l)
		}
	}
	if _, err := f.module.CreateMeasure(ctx, seller.CreateMeasureInput{
		AdminID: f.adminID, StoreID: sB.StoreID, Kind: seller.MeasureTermination, StatementToSeller: "關聯終止", Note: "內部",
		Termination: &seller.TerminationBasis{Ground: seller.GroundLinkedAccount, SourceMeasureID: &source.Measure.ID},
	}); err != nil {
		t.Fatal(err)
	}
	linked = read()
	if l := linked[sid(sB.StoreID)]; l.GetCanCreateLinkedTermination() || l.GetTerminationOption() != nil || l.GetLinkedMeasureId() == "" {
		t.Fatalf("已建立關聯終止的列不能再建立、不帶選項：%+v", l)
	}
	if l := linked[sid(sC.StoreID)]; !l.GetCanCreateLinkedTermination() || l.GetTerminationOption() == nil {
		t.Fatalf("另一個帳號仍可建立：%+v", l)
	}
}

// submitKYC 為店家所屬的帳號存草稿並送出申請（身分證號與銀行帳號由參數給，同一個身分證號的帳號互為關聯帳號）。
func (f *adminFixture) submitKYC(t *testing.T, storeID int64, idCard, bank string) {
	t.Helper()
	ctx := context.Background()
	var accountID int64
	f.scan(t, &accountID, "SELECT account_id FROM seller.stores WHERE store_id=$1", storeID)
	draft, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{
		AccountID: accountID, SellerType: ptr(seller.KYCIndividual), RealName: ptr("王小明"), BirthDate: ptr("1990-05-06"), IDCardNo: ptr(idCard),
		EnglishName: ptr("Wang Farm"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr(bank),
		ProductType: ptr("蔬果"),
		IDCardFront: &seller.KYCDocumentRef{StagedToken: "front"}, IDCardBack: &seller.KYCDocumentRef{StagedToken: "back"}, BankBook: &seller.KYCDocumentRef{StagedToken: "book"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version}); err != nil {
		t.Fatal(err)
	}
}

// 內容下架經 Connect 建立：對象與社群類別的認定寫進模組、回應帶對象與不可延長／結束的原因；重送同一個 request_id 回原本那筆；不合格的對象在模組被拒絕，欄位路徑指到 content_takedown。
func TestAdminCreatesAContentTakedownOverConnect(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, s := f.newSeller(t, "有貼文的店")
	content := f.seedContent(t, s.StoreID)
	post, forum := content.post, content.entry
	req := &adminsellerpb.CreateMeasureRequest{
		RequestId: uuid.New().String(), StoreId: sid(s.StoreID), Statement: "貼文含垃圾訊息", Note: "內部紀錄",
		Measure: &adminsellerpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminsellerpb.ContentTakedownInput{
			Contents: []*pb.ContentRef{
				{Kind: pb.ContentKind_CONTENT_KIND_FORUM_ENTRY, Id: forum.String()},
				{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: post.String()},
			},
			Violation: &adminsellerpb.ViolationBasis{Finding: &adminsellerpb.NewFinding{
				Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW}},
		}},
	}
	created, err := f.governance.CreateMeasure(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	m := created.Msg.GetMeasure()
	if m.GetKind() != pb.MeasureKind_MEASURE_KIND_CONTENT_TAKEDOWN || m.GetState() != pb.MeasureState_MEASURE_STATE_ISSUED || m.GetEndTime() != nil ||
		len(m.GetFindings()) != 1 || m.GetFindings()[0].GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_SPAM ||
		len(m.GetContents()) != 2 || m.GetContents()[0].GetKind() != pb.ContentKind_CONTENT_KIND_FORUM_ENTRY || m.GetContents()[0].GetId() != forum.String() ||
		m.GetContents()[1].GetId() != post.String() || m.GetCanEnd() || m.GetCanExtend() || !m.GetCanRevoke() ||
		len(m.GetEndUnavailabilities()) != 1 || m.GetEndUnavailabilities()[0].GetReason() != adminsellerpb.MeasureActionUnavailableReason_MEASURE_ACTION_UNAVAILABLE_REASON_CONTENT_TAKEDOWN_REVOKE_ONLY {
		t.Fatalf("measure=%v", m)
	}
	again, err := f.governance.CreateMeasure(ctx, connect.NewRequest(req))
	if err != nil || again.Msg.GetMeasure().GetId() != m.GetId() {
		t.Fatalf("重送 err=%v id=%v want %v", err, again.Msg.GetMeasure().GetId(), m.GetId())
	}
	got, err := f.governance.GetMeasure(ctx, connect.NewRequest(&adminsellerpb.GetMeasureRequest{Id: m.GetId()}))
	if err != nil || len(got.Msg.GetMeasure().GetContents()) != 2 {
		t.Fatalf("GetMeasure err=%v measure=%v", err, got.Msg.GetMeasure())
	}
	req.RequestId = uuid.New().String()
	req.GetContentTakedown().Contents = nil
	_, err = f.governance.CreateMeasure(ctx, connect.NewRequest(req))
	var fields []string
	if ce := (*connect.Error)(nil); errors.As(err, &ce) {
		for _, d := range ce.Details() {
			if v, e := d.Value(); e == nil {
				if br, ok := v.(*errdetails.BadRequest); ok {
					for _, fv := range br.GetFieldViolations() {
						fields = append(fields, fv.GetField())
					}
				}
			}
		}
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"content_takedown.contents"}) {
		t.Fatalf("沒有對象 err=%v fields=%v", err, fields)
	}
}
