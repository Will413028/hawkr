//go:build integration

package moderation_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1/moderationv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	appconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type noOrders struct{}

func (noOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

// adminStack 是單體的 Connect 入口加真的 moderation 模組：handler 與 production 一樣由 httpapi.NewHandler 組出，資料庫是 testdb 的真實 Postgres。
type adminStack struct {
	db         *db.DB
	identity   *identity.Module
	buyers     *buyer.Module
	social     *social.Module
	sellers    *seller.Reader
	sellerMod  *seller.Module
	moderation *moderation.Module
	// customerAccount 與 app 是一般顧客帳號與它的買家端入口（ModerationAppService）。
	customerAccount int64
	app             appconnect.ModerationAppServiceClient
	admin           moderationv1alpha1connect.ModerationAdminServiceClient
	// noRole 是認證過但沒有任何角色的管理員：每支方法都該回 permission_denied。
	noRole moderationv1alpha1connect.ModerationAdminServiceClient
	// customer 是一般顧客帳號：後台的 service 只給管理員。
	customer moderationv1alpha1connect.ModerationAdminServiceClient
	visitor  moderationv1alpha1connect.ModerationAdminServiceClient
}

func bearer(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}))
}

func startAdminStack(t *testing.T) *adminStack {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: "moderation", Dir: moderation.Migrations()},
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
	id := identity.New(identity.Deps{DB: d, Events: bus, Providers: identity.Providers{Admin: dev.Admins()}, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	fakeMedia := media.NewFake()
	sellers := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: id, Media: fakeMedia, Orders: noOrders{}, KYCMAC: sealtest.MAC(t),
		ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: queue, Now: time.Now, Logger: log}),
	})
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Identity: id, Buyer: buyers, Seller: sellers, Orders: noOrders{}, Now: time.Now, Logger: log})
	delegationDeps := delegation.Deps{DB: d, Identity: id, Seller: sellers, Now: time.Now, Logger: log}
	reader := seller.NewReader(d, time.Now)
	socialModule := social.New(social.Deps{
		DB: d, Events: bus, Media: ownerMedia{}, Jobs: queue, Buyers: buyers, Stores: reader, StoreBlocks: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log,
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: delegation.NewScheduleAuthorizer(delegationDeps)},
	})
	delegationDeps.Social = socialModule
	events.Subscribe(bus, socialModule.OnMeasureCreated)
	events.Subscribe(bus, socialModule.OnMeasureRevoked)
	suspension := accountsuspension.New(accountsuspension.Deps{DB: d, Events: bus, Identity: id, Seller: sellers, Logger: log})
	mod := moderation.New(moderation.Deps{
		DB: d, Social: socialModule, Review: &review.Module{}, IM: &im.Module{}, Seller: sellers, Buyer: buyers, Suspension: suspension,
		Media: fakeMedia, NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{
		Ready: d.Ping, Identity: id, Seller: sellers, Social: socialModule, Buyer: buyers, Images: images, Logger: log, Deletion: deletion,
		Delegation: delegation.New(delegationDeps), Stats: ch.Stats(t), WebOrigin: ch.WebOrigin(t), DashboardOrders: noDashboardOrders{}, DashboardBudget: time.Minute,
		Discovery: &discovery.Module{}, ReviewQueue: &reviewqueue.Module{}, Payment: &payment.Module{}, Moderation: mod,
		BuyerPorts: httpbuyer.Ports{Stores: sellers},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	ownerToken, err := dev.AdminToken("dev_admin_moderation_owner")
	if err != nil {
		t.Fatal(err)
	}
	ch.GrantAdminOwner(t, id, ownerToken)
	noRoleToken, err := dev.AdminToken("dev_admin_moderation_norole")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := id.Authenticate(t.Context(), noRoleToken); err != nil {
		t.Fatal(err)
	}
	customerAccount, customerToken, err := id.DevNewSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	hc := srv.Client()
	return &adminStack{
		db: d, identity: id, buyers: buyers, social: socialModule, sellers: reader, sellerMod: sellers, moderation: mod,
		customerAccount: customerAccount, app: appconnect.NewModerationAppServiceClient(hc, srv.URL, bearer(customerToken)),
		admin:    moderationv1alpha1connect.NewModerationAdminServiceClient(hc, srv.URL, bearer(ownerToken)),
		noRole:   moderationv1alpha1connect.NewModerationAdminServiceClient(hc, srv.URL, bearer(noRoleToken)),
		customer: moderationv1alpha1connect.NewModerationAdminServiceClient(hc, srv.URL, bearer(customerToken)),
		visitor:  moderationv1alpha1connect.NewModerationAdminServiceClient(hc, srv.URL),
	}
}

func (s *adminStack) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := s.db.Conn(t.Context()).Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}

func (s *adminStack) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.Conn(t.Context()).QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

func (s *adminStack) newBuyer(t *testing.T) int64 {
	t.Helper()
	var accountID int64
	if err := s.db.Conn(t.Context()).QueryRow(t.Context(), "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := s.buyers.OnAccountCreated(t.Context(), identity.AccountCreated{AccountID: accountID, Name: "買家"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.buyers.Profile(t.Context(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	return p.BuyerID
}

// world 開三個案件：貼文（進行中）、第二篇貼文（已結案）、留言（進行中）。回傳三個案件 ID。
type world struct{ postOpen, postClosed, commentOpen string }

func (s *adminStack) world(t *testing.T) world {
	t.Helper()
	ctx := t.Context()
	now := time.Now()
	var storeID, accountID int64
	if err := s.db.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	s.exec(t, `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	s.exec(t, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $1, '測試店', 'logo/k', uuidv7(), '63000010', $2, $2)`, accountID, now)
	if err := s.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	s.exec(t, `INSERT INTO seller.store_exposures (store_id) VALUES ($1)`, storeID)
	s.exec(t, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, now, uuid.NewV7())
	s.exec(t, `INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, now)
	st, err := s.sellers.Store(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	post := func(body string) social.Post {
		p, err := s.social.CreatePost(ctx, st, social.CreatePostInput{Body: body, ImageToken: uuid.NewV7().String(), Publish: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first, second := post("第一篇"), post("第二篇")
	author := s.newBuyer(t)
	comment, err := s.social.CreateComment(ctx, social.Caller{BuyerID: author}, social.CommentInput{PostID: first.ID, Body: "留言"})
	if err != nil {
		t.Fatal(err)
	}
	reporter := s.newBuyer(t)
	submit := func(target moderation.Target) string {
		r, err := s.moderation.SubmitReport(ctx, moderation.SubmitInput{ReporterBuyerID: reporter, Target: target, Category: policy.Spam})
		if err != nil {
			t.Fatal(err)
		}
		return r.Case.ID.String()
	}
	w := world{
		postClosed:  submit(moderation.Target{Kind: moderation.TargetPost, ContentID: second.ID}),
		postOpen:    submit(moderation.Target{Kind: moderation.TargetPost, ContentID: first.ID}),
		commentOpen: submit(moderation.Target{Kind: moderation.TargetComment, ContentID: comment.ID}),
	}
	got, err := s.admin.GetCase(ctx, connect.NewRequest(&adminpb.GetCaseRequest{Id: w.postClosed}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.admin.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{
		Id: w.postClosed, Etag: got.Msg.GetCase().GetEtag(), Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION,
	})); err != nil {
		t.Fatal(err)
	}
	return w
}

func caseIDs(cases []*adminpb.Case) []string {
	out := make([]string, len(cases))
	for i, c := range cases {
		out[i] = c.GetId()
	}
	return out
}

func wantSet(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	g, w := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	if !slices.Equal(g, w) {
		t.Fatalf("%s = %v，預期 %v", what, got, want)
	}
}

func wantCode(t *testing.T, what string, err error, want connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != want {
		t.Fatalf("%s：err=%v，預期 %v", what, err, want)
	}
}

// 每支方法：認證過但沒有權限的管理員回 permission_denied，顧客回 permission_denied，沒登入回 unauthenticated；被擋的 DecideCase 什麼都不寫。
func TestModerationAdminMethodsRequirePermissions(t *testing.T) {
	t.Parallel()
	s := startAdminStack(t)
	w := s.world(t)
	ctx := t.Context()
	for name, c := range map[string]moderationv1alpha1connect.ModerationAdminServiceClient{"noRole": s.noRole, "customer": s.customer, "visitor": s.visitor} {
		want := connect.CodePermissionDenied
		if name == "visitor" {
			want = connect.CodeUnauthenticated
		}
		_, err := c.ListCases(ctx, connect.NewRequest(&adminpb.ListCasesRequest{}))
		wantCode(t, name+" ListCases", err, want)
		_, err = c.GetCase(ctx, connect.NewRequest(&adminpb.GetCaseRequest{Id: w.postOpen}))
		wantCode(t, name+" GetCase", err, want)
		_, err = c.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: w.postOpen, Etag: "1", Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION}))
		wantCode(t, name+" DecideCase", err, want)
	}
	if n := s.count(t, "SELECT count(*) FROM moderation.cases WHERE closed_at IS NULL"); n != 2 {
		t.Fatalf("被擋的請求不能結案：進行中 %d，預期 2", n)
	}
}

func TestListCasesFiltersAndPagesThroughTheConnectEntry(t *testing.T) {
	t.Parallel()
	s := startAdminStack(t)
	w := s.world(t)
	ctx := t.Context()
	list := func(req *adminpb.ListCasesRequest) *adminpb.ListCasesResponse {
		res, err := s.admin.ListCases(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}
	wantSet(t, "全部", caseIDs(list(&adminpb.ListCasesRequest{}).GetCases()), w.postOpen, w.postClosed, w.commentOpen)
	wantSet(t, "進行中", caseIDs(list(&adminpb.ListCasesRequest{State: adminpb.CaseState_CASE_STATE_OPEN}).GetCases()), w.postOpen, w.commentOpen)
	closed := list(&adminpb.ListCasesRequest{State: adminpb.CaseState_CASE_STATE_CLOSED}).GetCases()
	wantSet(t, "已結案", caseIDs(closed), w.postClosed)
	if closed[0].GetState() != adminpb.CaseState_CASE_STATE_CLOSED || closed[0].GetDecision() != adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION || closed[0].GetResolveTime() == nil {
		t.Fatalf("已結案的案件欄位：%+v", closed[0])
	}
	wantSet(t, "貼文", caseIDs(list(&adminpb.ListCasesRequest{TargetKind: pb.ReportTargetKind_REPORT_TARGET_KIND_POST}).GetCases()), w.postOpen, w.postClosed)
	wantSet(t, "留言", caseIDs(list(&adminpb.ListCasesRequest{TargetKind: pb.ReportTargetKind_REPORT_TARGET_KIND_COMMENT}).GetCases()), w.commentOpen)
	wantSet(t, "進行中的貼文", caseIDs(list(&adminpb.ListCasesRequest{
		State: adminpb.CaseState_CASE_STATE_OPEN, TargetKind: pb.ReportTargetKind_REPORT_TARGET_KIND_POST,
	}).GetCases()), w.postOpen)

	// 一頁一筆走完：不重不漏，最後一頁沒有 token。
	var got []string
	token := ""
	for range 5 {
		page := list(&adminpb.ListCasesRequest{PageSize: 1, PageToken: token})
		got = append(got, caseIDs(page.GetCases())...)
		if token = page.GetNextPageToken(); token == "" {
			break
		}
	}
	if token != "" {
		t.Fatal("分頁沒有結束")
	}
	wantSet(t, "分頁走完", got, w.postOpen, w.postClosed, w.commentOpen)

	_, err := s.admin.ListCases(ctx, connect.NewRequest(&adminpb.ListCasesRequest{State: adminpb.CaseState(99)}))
	wantCode(t, "不認得的狀態", err, connect.CodeInvalidArgument)
	_, err = s.admin.ListCases(ctx, connect.NewRequest(&adminpb.ListCasesRequest{PageSize: -1}))
	wantCode(t, "負的 page_size", err, connect.CodeInvalidArgument)
	_, err = s.admin.ListCases(ctx, connect.NewRequest(&adminpb.ListCasesRequest{PageToken: "garbage"}))
	wantCode(t, "壞的 page_token", err, connect.CodeInvalidArgument)
}

func TestGetCaseAndNotFound(t *testing.T) {
	t.Parallel()
	s := startAdminStack(t)
	w := s.world(t)
	res, err := s.admin.GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: w.postOpen}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.GetCase().GetId() != w.postOpen || res.Msg.GetCase().GetState() != adminpb.CaseState_CASE_STATE_OPEN || res.Msg.GetCase().GetEtag() == "" ||
		len(res.Msg.GetReports()) != 1 || res.Msg.GetReports()[0].GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_PENDING {
		t.Fatalf("response = %+v", res.Msg)
	}
	_, err = s.admin.GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: uuid.New().String()}))
	wantCode(t, "不存在的案件", err, connect.CodeNotFound)
}

// DecideCase：成功結案（同 transaction 留後台動作紀錄）；已結案或過期的 etag 回 aborted 與 VERSION_CONFLICT；缺 etag 是 invalid_argument。
func TestDecideCaseClosesAndRejectsStaleEtags(t *testing.T) {
	t.Parallel()
	s := startAdminStack(t)
	w := s.world(t)
	ctx := t.Context()
	cur, err := s.admin.GetCase(ctx, connect.NewRequest(&adminpb.GetCaseRequest{Id: w.postOpen}))
	if err != nil {
		t.Fatal(err)
	}
	etag := cur.Msg.GetCase().GetEtag()

	_, err = s.admin.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: w.postOpen, Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION}))
	wantCode(t, "缺 etag", err, connect.CodeInvalidArgument)
	_, err = s.admin.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: w.postOpen, Etag: etag + "9", Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION}))
	wantCode(t, "過期的 etag", err, connect.CodeAborted)

	res, err := s.admin.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: w.postOpen, Etag: etag, Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION}))
	if err != nil {
		t.Fatal(err)
	}
	c := res.Msg.GetCase()
	if c.GetState() != adminpb.CaseState_CASE_STATE_CLOSED || c.GetDecision() != adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION || c.GetDecidedBy() == "" ||
		len(res.Msg.GetReports()) != 1 || res.Msg.GetReports()[0].GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION {
		t.Fatalf("結案的回應：%+v", res.Msg)
	}
	if n := s.count(t, "SELECT count(*) FROM adminaudit.records WHERE action = $1 AND target_type = 'moderation_case' AND target_id = $2",
		moderationv1alpha1connect.ModerationAdminServiceDecideCaseProcedure, w.postOpen); n != 1 {
		t.Fatalf("後台動作紀錄 %d 筆，預期 1", n)
	}

	// 結案是終態：再用結案前的 etag 或結案後的 etag 決定都是版本衝突。
	for _, e := range []string{etag, c.GetEtag()} {
		_, err = s.admin.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: w.postOpen, Etag: e, Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION}))
		wantCode(t, "已結案再決定", err, connect.CodeAborted)
	}
}

// ownerMedia 是 social 的 media 替身：每個 staged token 都確認成功，資產 ID 每次新產生。
type ownerMedia struct{}

func (ownerMedia) Confirm(_ context.Context, _ media.Uploader, token string, _ media.Visibility) (media.ConfirmedAsset, error) {
	return media.ConfirmedAsset{ID: uuid.NewV7(), ObjectKey: "post/" + token + ".jpg", Visibility: media.VisibilityPublic}, nil
}

func (ownerMedia) MarkDeleted(context.Context, []uuid.UUID) error { return nil }
