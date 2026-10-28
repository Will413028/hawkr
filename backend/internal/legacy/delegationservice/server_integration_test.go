//go:build integration

package delegationservice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/account"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/account/accountconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

const (
	storeID        = int64(2685914682)
	closedStoreID  = int64(98)
	suspendedStore = int64(97)
	pendingStore   = int64(96)
)

func newStore(id int64, status seller.Status, suspended bool) seller.Store {
	var s seller.Store
	s.StoreID, s.AccountID, s.StoreName, s.Status, s.Sanctions = id, id+7, "梨山盛花園", status, sanctionsOf(suspended)
	// seller 讀店家時由 SQL 帶上的條件（Store.Conditions）：生命週期照 status，其餘都滿足。
	s.Conditions = seller.Conditions{
		Derived: true, Active: status != seller.Closed, NotPendingDeletion: status != seller.PendingDeletion, KYCApproved: true,
		StoreName: true, Logo: true, TownCode: true,
	}
	return s
}

// fakeStores 是 seller 模組的替身：stores 以外的店家不存在，down 時讀不到。
type fakeStores struct {
	mu     sync.Mutex
	stores map[int64]seller.Store
	down   bool
}

func (f *fakeStores) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

func (f *fakeStores) Store(_ context.Context, id int64) (seller.Store, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return seller.Store{}, errors.New("連不上 seller")
	}
	s, ok := f.stores[id]
	if !ok {
		return seller.Store{}, apperr.NotFound("店家不存在")
	}
	return s, nil
}

type fixture struct {
	db        *db.DB
	m         *delegation.Module
	identity  *identity.Module
	dev       *identity.DevAuth
	stores    *fakeStores
	client    accountconnect.DelegationServiceClient
	published []rabbitmq.PublishArgs
}

// setup 起 identity 與 legacy 的 DelegationService，client 以 gateway 的方式（gRPC、h2c）呼叫。
func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
	))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: d, dev: dev, stores: &fakeStores{stores: map[int64]seller.Store{
		storeID:        newStore(storeID, seller.Active, false),
		closedStoreID:  newStore(closedStoreID, seller.Closed, false),
		suspendedStore: newStore(suspendedStore, seller.Active, true),
		pendingStore:   newStore(pendingStore, seller.PendingDeletion, false),
	}}}
	bus := events.NewBus()
	events.Subscribe(bus, OnDelegationOperationCompleted(func(ctx context.Context, a rabbitmq.PublishArgs) error {
		f.published = append(f.published, a)
		return jc.Enqueue(ctx, a, rabbitmq.InsertOpts())
	}))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.identity = identity.New(identity.Deps{
		DB: d, Events: bus, Jobs: jc,
		Providers: identity.Providers{Admin: dev.Admins()},
		Now:       time.Now, Logger: log,
	})
	f.m = delegation.New(delegation.Deps{
		DB: d, Events: bus, Identity: f.identity, Seller: f.stores,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})
	mux := http.NewServeMux()
	mux.Handle(accountconnect.NewDelegationServiceHandler(New(f.m, log)))
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(srv.Close)
	f.client = accountconnect.NewDelegationServiceClient(grpcclient.HTTPClient(), srv.URL, connect.WithGRPC(),
		connect.WithInterceptors(grpcclient.Signer("secret", nil)))
	return f
}

// admin 建立一位管理員；enabled 時開通代行。
func (f *fixture) admin(t *testing.T, email string, enabled bool) string {
	t.Helper()
	ctx := context.Background()
	// dev 的管理員的 email 是 <使用者>@dev.localhost：ops@hawkr.example 變成 dev_ops@dev.localhost。
	local, _, _ := strings.Cut(email, "@")
	tok, err := f.dev.AdminToken("dev_" + local)
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.identity.Authenticate(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	id := a.Principal.ID
	if enabled {
		if _, err := f.identity.GrantAdminRole(ctx, identity.AdminRoleChangeInput{AdminID: id, Role: adminperm.RoleOwner, Reason: "測試的管理員", Source: identity.AdminRoleSourceCLI}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.m.SetAdminEligibility(ctx, delegation.AdminEligibilityChange{AdminID: id, OperatorAdminID: id, Enabled: enabled}); err != nil {
		t.Fatal(err)
	}
	return formatID(id)
}

func caseRef() *string { s := "HK-2026-0927"; return &s }

func TestPreflightCreateAndTheWorkspace(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	admin := f.admin(t, "ops@hawkr.example", true)
	store := formatID(storeID)

	// 還沒開通：allowed=false 加業務碼，回應本身成功（code 0）；選項與能力仍然非空。
	pre, err := f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: admin, TargetStoreId: store}))
	if err != nil || pre.Msg.GetCode() != 0 || pre.Msg.GetData().GetAllowed() || pre.Msg.GetData().GetDeniedCode() != 9286 ||
		len(pre.Msg.GetData().GetReasonOptions()) != 3 || len(pre.Msg.GetData().GetCapabilities()) != 9 {
		t.Fatalf("preflight = %v, %v", pre, err)
	}
	g, err := f.client.GrantDelegationAuthorization(ctx, connect.NewRequest(&pb.GrantDelegationAuthorizationRequest{
		ActorAdminId: admin, TargetStoreId: store, Source: "contract", SourceReference: "CT-2026-0927",
		GrantedCapabilities: []string{"products.read", "products.write"}, OperationReason: "合約簽署", IdempotencyKey: "g1",
	}))
	if err != nil || g.Msg.GetCode() != 0 || g.Msg.GetData().GetOutcome() != "created" || !g.Msg.GetData().GetAuthorization().GetEffective() ||
		g.Msg.GetData().GetAuthorization().EffectiveUntil != nil || g.Msg.GetData().GetAuthorization().GetApprovedByAdminDisplayName() != "dev_ops@dev.localhost" {
		t.Fatalf("開通 = %v, %v", g, err)
	}
	// 以假的店家狀態 preflight。
	pre, err = f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: admin, TargetStoreId: store}))
	d := pre.Msg.GetData()
	if err != nil || !d.GetAllowed() || d.GetDeniedCode() != 0 || d.GetTtlSeconds() != 1800 || d.GetIdleSeconds() != 900 ||
		d.GetTarget().GetStoreName() != "梨山盛花園" || d.GetTarget().GetSellerAccountId() != formatID(storeID+7) ||
		d.GetActor().GetDisplayName() != "dev_ops@dev.localhost" || !d.GetCaseReferenceRequired() {
		t.Fatalf("preflight = %v, %v", pre, err)
	}

	created, err := f.client.CreateSession(ctx, connect.NewRequest(&pb.CreateSessionRequest{
		ActorAdminId: admin, ActorDisplayName: "營運 A", TargetStoreId: store, ReasonCode: "seller_request", CaseReference: caseRef(),
		RequestedCapabilities: []string{"products.read", "products.write"}, RequestedTtlSeconds: 1800, IdempotencyKey: "ik-1",
		RequestId: "req-1", CorrelationId: "corr-1",
	}))
	if err != nil || created.Msg.GetCode() != 0 {
		t.Fatalf("建立 = %v, %v", created, err)
	}
	cd := created.Msg.GetData()
	summary := cd.GetSession()
	// session ID 是十進位字串：gateway 的 delegationSessionIDRE 與 admin 的 isDelegatedSessionId 都是 ^\d{1,19}$。
	if !regexp.MustCompile(`^\d{1,19}$`).MatchString(summary.GetSessionId()) || summary.GetStatus() != "active" || summary.GetRevision() != 0 ||
		summary.GetActor().GetDisplayName() != "營運 A" || summary.GetReason().GetLabel() != "賣家請求（管理員記錄）" ||
		summary.GetCaseReference() != "HK-2026-0927" || len(cd.GetHandoffCode()) != 48 {
		t.Fatalf("摘要 = %v", cd)
	}

	// 只有兌換 handoff 時才簽發 credential，反查回同一份摘要。
	r, err := f.client.RedeemHandoff(ctx, connect.NewRequest(&pb.RedeemHandoffRequest{HandoffCode: cd.GetHandoffCode()}))
	if err != nil || len(r.Msg.GetData().GetOperationCredential()) != 48 || r.Msg.GetData().GetSession().GetSessionId() != summary.GetSessionId() {
		t.Fatalf("兌換 = %v, %v", r, err)
	}
	credential := r.Msg.GetData().GetOperationCredential()
	r, err = f.client.RedeemHandoff(ctx, connect.NewRequest(&pb.RedeemHandoffRequest{HandoffCode: cd.GetHandoffCode()}))
	if err != nil || r.Msg.GetCode() != 9282 {
		t.Fatalf("第二次兌換 = %v, %v", r, err)
	}
	cur, err := f.client.GetCurrentSession(ctx, connect.NewRequest(&pb.GetCurrentSessionRequest{OperationCredential: credential, CountsAsActivity: true}))
	if err != nil || cur.Msg.GetData().GetDisplayCode() != summary.GetDisplayCode() || cur.Msg.GetData().GetIssuedAt() != summary.GetIssuedAt() {
		t.Fatalf("current = %v, %v", cur, err)
	}
	cur, _ = f.client.GetCurrentSession(ctx, connect.NewRequest(&pb.GetCurrentSessionRequest{OperationCredential: "dops_x"}))
	if cur.Msg.GetCode() != 9274 {
		t.Fatalf("對不到的 credential = %v", cur)
	}

	// 兩階段稽核：confirm 在同一個 transaction 內排入 delegation.operation.completed。
	intent, err := f.client.RecordAuditIntent(ctx, connect.NewRequest(&pb.RecordAuditIntentRequest{
		SessionId: summary.GetSessionId(), Capability: "products.write", Action: "PRODUCT_UPDATE", ResourceType: "product",
		ResourceId: "2701445588392017", ResourceName: "梨山水蜜桃", OperationReason: "賣家請求修正商品說明",
		ResourceVersionBefore: "12", BeforeSnapshotJson: `{"title":"舊的"}`, IdempotencyKey: "ik-a", RequestId: "req-a",
		CorrelationId: "corr-a", NotifySeller: true, Origin: "interactive",
	}))
	if err != nil || intent.Msg.GetCode() != 0 || intent.Msg.GetData().GetResult() != "PENDING" {
		t.Fatalf("intent = %v, %v", intent, err)
	}
	conf, err := f.client.ConfirmAuditResult(ctx, connect.NewRequest(&pb.ConfirmAuditResultRequest{
		AuditLogId: intent.Msg.GetData().GetAuditLogId(), Result: "SUCCEEDED", ResourceVersionAfter: "13",
		ChangedFields: []*pb.ChangedField{{Field: "title", Label: "標題", Before: "", After: "新的"}},
	}))
	if err != nil || conf.Msg.GetData().GetResult() != "SUCCEEDED" || len(f.published) != 1 {
		t.Fatalf("confirm = %v, %v，事件 %d 則", conf, err, len(f.published))
	}
	var body map[string]any
	if err := json.Unmarshal(f.published[0].Body, &body); err != nil || body["audit_log_id"] != intent.Msg.GetData().GetAuditLogId() ||
		body["session_display_code"] != summary.GetDisplayCode() || body["action_label"] != "編輯商品" || body["correlation_id"] != "corr-a" {
		t.Fatalf("事件 = %s, %v", f.published[0].Body, err)
	}
	if n := f.count(t, `SELECT count(*) FROM river.river_job WHERE kind = 'legacy.rabbitmq_publish'`); n != 1 {
		t.Fatalf("排入的 job %d 個", n)
	}

	logs, err := f.client.ListAuditLogs(ctx, connect.NewRequest(&pb.ListAuditLogsRequest{SessionId: summary.GetSessionId()}))
	if err != nil || logs.Msg.GetTotal() != "1" || logs.Msg.GetPage() != "1" || logs.Msg.GetPageSize() != "20" {
		t.Fatalf("稽核 = %v, %v", logs, err)
	}
	e := logs.Msg.GetRecords()[0]
	if e.GetActionLabel() != "編輯商品" || e.GetResourceId() != "2701445588392017" || e.GetResourceVersionAfter() != "13" ||
		e.NotifiedAt == nil || e.ConfirmedAt == nil || len(e.GetChangedFields()) != 1 || e.GetChangedFields()[0].GetBefore() != "" ||
		e.GetTarget().GetStoreName() != "梨山盛花園" || e.GetCaseReference() != "HK-2026-0927" {
		t.Fatalf("稽核列 = %v", e)
	}
	sum, err := f.client.GetSessionSummary(ctx, connect.NewRequest(&pb.GetSessionSummaryRequest{SessionId: summary.GetSessionId()}))
	if err != nil || sum.Msg.GetData().GetOperationCount() != "1" || !sum.Msg.GetData().GetNotifiedSeller() ||
		sum.Msg.GetData().GetAuditRetentionDays() != "730" || sum.Msg.GetData().GetFulfillmentActionCount() != "0" || sum.Msg.GetData().RevokedAt != nil ||
		sum.Msg.GetData().GetSession().GetSessionId() != summary.GetSessionId() || sum.Msg.GetData().GetSession().GetReason().GetCode() != summary.GetReason().GetCode() {
		t.Fatalf("摘要 = %v, %v", sum, err)
	}

	// 撤銷（治理）；撤回授權的面板與清單。
	rv, err := f.client.RevokeSession(ctx, connect.NewRequest(&pb.RevokeSessionRequest{
		SessionId: summary.GetSessionId(), ActorAdminId: admin, RevokeReason: "賣家要求中止", IdempotencyKey: "rv-1",
	}))
	if err != nil || rv.Msg.GetData().GetStatus() != "revoked" || rv.Msg.GetData().GetRevision() != 1 || rv.Msg.GetData().GetRevokedAt() == "" {
		t.Fatalf("撤銷 = %v, %v", rv, err)
	}
	sessions, err := f.client.ListSessions(ctx, connect.NewRequest(&pb.ListSessionsRequest{ActorAdminId: admin}))
	if err != nil || sessions.Msg.GetTotal() != "1" || sessions.Msg.GetRecords()[0].GetStatus() != "revoked" ||
		sessions.Msg.GetRecords()[0].GetRevokedByAdminDisplayName() != "dev_ops@dev.localhost" {
		t.Fatalf("清單 = %v, %v", sessions, err)
	}
	all, err := f.client.ListAllAuditLogs(ctx, connect.NewRequest(&pb.ListAllAuditLogsRequest{TargetStoreId: store}))
	if err != nil || all.Msg.GetTotal() != "1" {
		t.Fatalf("跨 session 的稽核 = %v, %v", all, err)
	}
	panel, err := f.client.GetDelegationAuthorization(ctx, connect.NewRequest(&pb.GetDelegationAuthorizationRequest{ActorAdminId: admin, TargetStoreId: store}))
	if err != nil || !panel.Msg.GetData().GetExists() || panel.Msg.GetData().GetActiveSessionCount() != "0" {
		t.Fatalf("面板 = %v, %v", panel, err)
	}
	revoked, err := f.client.RevokeDelegationAuthorization(ctx, connect.NewRequest(&pb.RevokeDelegationAuthorizationRequest{
		ActorAdminId: admin, TargetStoreId: store, RevokeReason: "合約到期", IdempotencyKey: "r1",
	}))
	if err != nil || revoked.Msg.GetData().GetAlreadyRevoked() || revoked.Msg.GetData().GetAuthorization().GetRevokeReason() != "合約到期" {
		t.Fatalf("撤回 = %v, %v", revoked, err)
	}
	history, err := f.client.ListDelegationAuthorizationHistory(ctx, connect.NewRequest(&pb.ListDelegationAuthorizationHistoryRequest{ActorAdminId: admin, TargetStoreId: store}))
	if err != nil || history.Msg.GetTotal() != "2" || history.Msg.GetRecords()[0].GetActionLabel() != "撤回授權" ||
		history.Msg.GetRecords()[0].Source != nil || history.Msg.GetRecords()[1].GetSource() != "contract" {
		t.Fatalf("歷史 = %v, %v", history, err)
	}
	list, err := f.client.ListDelegationAuthorizations(ctx, connect.NewRequest(&pb.ListDelegationAuthorizationsRequest{ActorAdminId: admin, Status: "ineffective"}))
	if err != nil || list.Msg.GetTotal() != "1" {
		t.Fatalf("總覽 = %v, %v", list, err)
	}
}

func TestErrorsKeepTheLegacyWireBehavior(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	admin := f.admin(t, "ops@hawkr.example", true)
	outsider := f.admin(t, "outsider@hawkr.example", false)
	store := formatID(storeID)

	// 不合法的 ID：放在回應的 code。
	pre, err := f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: "abc", TargetStoreId: store}))
	if err != nil || pre.Msg.GetCode() != 9260 {
		t.Fatalf("= %v, %v", pre, err)
	}
	for _, id := range []string{"7494590778405027840", "not-a-session", uuid.NewV7().String()} {
		s, err := f.client.GetSessionSummary(ctx, connect.NewRequest(&pb.GetSessionSummaryRequest{SessionId: id}))
		if err != nil || s.Msg.GetCode() != 9277 {
			t.Fatalf("%s = %v, %v", id, s, err)
		}
	}
	intent, err := f.client.RecordAuditIntent(ctx, connect.NewRequest(&pb.RecordAuditIntentRequest{SessionId: "abc"}))
	if err != nil || intent.Msg.GetCode() != 9260 {
		t.Fatalf("= %v, %v", intent, err)
	}
	list, err := f.client.ListDelegationAuthorizations(ctx, connect.NewRequest(&pb.ListDelegationAuthorizationsRequest{ActorAdminId: ""}))
	if err != nil || list.Msg.GetCode() != 9261 {
		t.Fatalf("沒有管理員 = %v, %v", list, err)
	}
	list, _ = f.client.ListDelegationAuthorizations(ctx, connect.NewRequest(&pb.ListDelegationAuthorizationsRequest{ActorAdminId: outsider}))
	if list.Msg.GetCode() != 9288 {
		t.Fatalf("沒有開通 = %v", list)
	}
	// 沒有開通的管理員：seller 模組讀不到也照樣回 9288（不需要店家的判斷先做）。
	f.stores.setDown(true)
	pre, err = f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: outsider, TargetStoreId: store}))
	if err != nil || pre.Msg.GetData().GetDeniedCode() != 9288 || pre.Msg.GetData().GetTarget().GetStoreName() != "（查無此店家）" ||
		pre.Msg.GetData().GetTarget().GetSellerAccountId() != "0" {
		t.Fatalf("= %v, %v", pre, err)
	}
	// 讀不到店家不是拒絕：gRPC 的 internal（不是 unavailable，#3330），gateway 回 9280。
	_, err = f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: admin, TargetStoreId: store}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("seller 模組掛了 = %v", err)
	}
	f.stores.setDown(false)
	// 不存在的店家。
	pre, _ = f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: admin, TargetStoreId: "99"}))
	if pre.Msg.GetData().GetDeniedCode() != 9265 {
		t.Fatalf("= %v", pre)
	}
	// 舊碼只在這一層對應：已關閉的店家模型裡是 DenialStoreClosed（店家存在），對外仍是 9265（舊版關店即軟刪除）；停權與等待刪除沒有店面寫入能力，是 9272。
	// denied_label 是 seller 給的句子（StoreActionDelegate 的第一個原因），不是舊的固定句。
	for _, tc := range []struct {
		store string
		want  int32
		label string
	}{
		{formatID(closedStoreID), 9265, "店家已關閉，無法代行。"},
		{formatID(suspendedStore), 9272, "帳號停權中，無法代行。"},
		{formatID(pendingStore), 9272, "店家等待刪除，無法代行。"},
	} {
		pre, _ = f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: admin, TargetStoreId: tc.store}))
		if pre.Msg.GetData().GetDeniedCode() != tc.want || pre.Msg.GetData().GetDeniedLabel() != tc.label {
			t.Fatalf("店家 %s 的 preflight = %v，want %d %q", tc.store, pre, tc.want, tc.label)
		}
	}
	closedGrant, _ := f.client.GrantDelegationAuthorization(ctx, connect.NewRequest(&pb.GrantDelegationAuthorizationRequest{
		ActorAdminId: admin, TargetStoreId: formatID(closedStoreID), Source: "contract", SourceReference: "CT-2026-0927",
		GrantedCapabilities: []string{"products.read"}, OperationReason: "合約簽署",
	}))
	if closedGrant.Msg.GetCode() != 9265 {
		t.Fatalf("開通已關閉的店家 = %v", closedGrant)
	}
	g, _ := f.client.GrantDelegationAuthorization(ctx, connect.NewRequest(&pb.GrantDelegationAuthorizationRequest{
		ActorAdminId: admin, TargetStoreId: "99", Source: "contract", SourceReference: "CT-2026-0927",
		GrantedCapabilities: []string{"products.read"}, OperationReason: "合約簽署",
	}))
	if g.Msg.GetCode() != 9265 {
		t.Fatalf("開通不存在的店家 = %v", g)
	}
	until := "2026-01-01"
	g, _ = f.client.GrantDelegationAuthorization(ctx, connect.NewRequest(&pb.GrantDelegationAuthorizationRequest{
		ActorAdminId: admin, TargetStoreId: store, Source: "contract", SourceReference: "CT-2026-0927",
		GrantedCapabilities: []string{"products.read"}, OperationReason: "合約簽署", EffectiveUntil: &until,
	}))
	if g.Msg.GetCode() != 9260 {
		t.Fatalf("期限的格式 = %v", g)
	}
	// 管理員從 session 解析出來卻不存在：不變量被打破，是 internal。
	_, err = f.client.Preflight(ctx, connect.NewRequest(&pb.PreflightRequest{ActorAdminId: "999999", TargetStoreId: store}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("= %v", err)
	}
	// 登出已經移到單體：這支沒有呼叫方了。
	_, err = f.client.RevokeSessionsByActor(ctx, connect.NewRequest(&pb.RevokeSessionsByActorRequest{ActorAdminId: admin}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("= %v", err)
	}
	// 批次：筆數超出範圍是整支 9260；逐項的錯誤只落在那一項。
	b, err := f.client.RecordAuditIntentBatch(ctx, connect.NewRequest(&pb.RecordAuditIntentBatchRequest{}))
	if err != nil || b.Msg.GetCode() != 9260 {
		t.Fatalf("= %v, %v", b, err)
	}
	b, err = f.client.RecordAuditIntentBatch(ctx, connect.NewRequest(&pb.RecordAuditIntentBatchRequest{Items: []*pb.RecordAuditIntentRequest{
		{SessionId: "bad", ResourceId: "1"}, {SessionId: "999999999", ResourceId: "2"},
	}}))
	if err != nil || b.Msg.GetCode() != 0 || len(b.Msg.GetResults()) != 2 || b.Msg.GetResults()[0].GetBizCode() != 9260 ||
		b.Msg.GetResults()[1].GetBizCode() != 9277 || b.Msg.GetResults()[1].GetResourceId() != "2" {
		t.Fatalf("逐項 = %v, %v", b, err)
	}
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func sanctionsOf(suspended bool) *seller.Sanctions {
	if !suspended {
		return seller.NoSanctions()
	}
	ground := seller.RiskFraud
	return seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisRisk, RiskGround: &ground})
}
