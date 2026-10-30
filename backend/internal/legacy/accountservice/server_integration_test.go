//go:build integration

package accountservice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/account"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/account/accountconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// fakeSuspender 替代 accountsuspension（它需要 seller 模組）：記下收到的紀錄，直接停權；err 不是 nil 時回它。
type fakeSuspender struct {
	m       *identity.Module
	records []accountsuspension.Record
	err     error
}

func (s *fakeSuspender) Suspend(ctx context.Context, accountID int64, rec accountsuspension.Record) (accountsuspension.Suspension, error) {
	s.records = append(s.records, rec)
	if s.err != nil {
		return accountsuspension.Suspension{}, s.err
	}
	rows, _, err := s.m.SuspendAccount(ctx, identity.SuspendInput{AccountID: accountID, Category: rec.Category, Reason: rec.Reason, SourceRef: rec.SourceRef})
	return rows, err
}

type fixture struct {
	suspender *fakeSuspender
	m         *identity.Module
	dev       *identity.DevAuth
	client    accountconnect.AccountServiceClient
	published []rabbitmq.PublishArgs
}

// setup 起 identity 與 legacy 的 AccountService，client 以舊服務的方式（gRPC、h2c）呼叫。
func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{dev: dev}
	bus := events.NewBus()
	events.Subscribe(bus, OnAccountDeleted(func(ctx context.Context, a rabbitmq.PublishArgs) error {
		f.published = append(f.published, a)
		return jc.Enqueue(ctx, a, rabbitmq.InsertOpts())
	}))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.m = identity.New(identity.Deps{
		DB: d, Events: bus, Jobs: jc, Providers: identity.Providers{},
		Now: time.Now, Logger: log,
	})
	mux := http.NewServeMux()
	f.suspender = &fakeSuspender{m: f.m}
	mux.Handle(accountconnect.NewAccountServiceHandler(New(f.m, f.suspender, log)))
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(srv.Close)
	f.client = accountconnect.NewAccountServiceClient(grpcclient.HTTPClient(), srv.URL, connect.WithGRPC(),
		connect.WithInterceptors(grpcclient.Signer("secret", nil)))
	return f
}

// account 以 dev token 建立帳號（第一次認證時建立），再授予 roles。買家不是角色（W5）：帳號活著就是買家。
func (f *fixture) account(t *testing.T, roles ...identity.Role) int64 {
	t.Helper()
	ctx := context.Background()
	_, tok, err := f.m.DevNewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.m.Authenticate(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	id := a.Principal.ID
	for _, r := range roles {
		if err := f.m.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: r, SourceID: nil}); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// account_type 的買家位元由 adapter 合成（W5）：identity 只存 SELLER，舊契約仍是買家 1、兩者 3。
func TestGetAccountInfo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, identity.RoleSeller)
	resp, err := f.client.GetAccountInfo(ctx, connect.NewRequest(&pb.GetAccountInfoRequest{AccountId: id}))
	if err != nil || resp.Msg.GetCode() != 0 {
		t.Fatalf("= %v, %v", resp, err)
	}
	a := resp.Msg.GetData()
	if a.GetAccountId() != id || a.GetAccountType() != 3 || a.GetStatus() != "active" || a.GetPhone() != "" || !strings.HasSuffix(a.GetEmail(), "@dev.localhost") {
		t.Fatalf("AccountInfo = %v", a)
	}
	buyerOnly := f.account(t)
	resp, err = f.client.GetAccountInfo(ctx, connect.NewRequest(&pb.GetAccountInfoRequest{AccountId: buyerOnly}))
	if err != nil || resp.Msg.GetData().GetAccountType() != 1 {
		t.Fatalf("沒有另外申請角色的帳號 = %v, %v", resp, err)
	}
	if _, _, err := f.m.SuspendAccount(ctx, identity.SuspendInput{AccountID: id, Category: policy.Other, Reason: "檢舉成立"}); err != nil {
		t.Fatal(err)
	}
	resp, _ = f.client.GetAccountInfo(ctx, connect.NewRequest(&pb.GetAccountInfoRequest{AccountId: id}))
	if resp.Msg.GetData().GetStatus() != "suspended" {
		t.Fatalf("停權的狀態 = %q", resp.Msg.GetData().GetStatus())
	}
	// 刪除的帳號與不存在的帳號一樣是 404（舊的 account-service 查不到軟刪的帳號）。
	other := f.account(t)
	if err := f.m.DeleteAccount(ctx, other); err != nil {
		t.Fatal(err)
	}
	for _, missing := range []int64{other, 999} {
		resp, err := f.client.GetAccountInfo(ctx, connect.NewRequest(&pb.GetAccountInfoRequest{AccountId: missing}))
		if err != nil || resp.Msg.GetCode() != codeNotFound {
			t.Fatalf("帳號 %d = %v, %v", missing, resp, err)
		}
	}
	batch, err := f.client.GetAccountInfoBatch(ctx, connect.NewRequest(&pb.GetAccountInfoBatchRequest{AccountIds: []int64{id, other, 999, buyerOnly}}))
	if err != nil || len(batch.Msg.GetData()) != 2 || batch.Msg.GetData()[0].GetAccountId() != id || batch.Msg.GetData()[0].GetAccountType() != 3 ||
		batch.Msg.GetData()[1].GetAccountType() != 1 {
		t.Fatalf("batch = %v, %v", batch, err)
	}
	// 帳號沒有手機：以手機找一律是「找不到」，與原本查無此手機相同。
	byPhone, err := f.client.GetAccountByPhone(ctx, connect.NewRequest(&pb.GetAccountByPhoneRequest{Phone: "0900000000"}))
	if err != nil || byPhone.Msg.GetCode() != codeNotFound || byPhone.Msg.GetData() != nil {
		t.Fatalf("以手機找 = %v, %v", byPhone, err)
	}
}

func TestRolesAndDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	// BindBuyer 沒有呼叫方，不實作。
	if _, err := f.client.BindBuyer(ctx, connect.NewRequest(&pb.BindBuyerRequest{AccountId: id})); connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("BindBuyer = %v", err)
	}
	// 賣家角色由 seller 模組建店時授予；這裡直接以 identity 授予，驗證舊 RemoveRole 的來源比對。
	for _, source := range []int64{11, 12} {
		if err := f.m.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: identity.RoleSeller, SourceID: &source}); err != nil {
			t.Fatal(err)
		}
	}
	// 重新入駐之後，遲到的舊撤銷（來源 11）不生效（#3189）。
	r, err := f.client.RemoveRole(ctx, connect.NewRequest(&pb.RemoveRoleRequest{
		AccountId: id, Role: pb.Role_ROLE_SELLER, SourceId: 11, OtherRoleState: pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT,
	}))
	if err != nil || r.Msg.GetCode() != 0 || r.Msg.GetAccountFullyDeleted() {
		t.Fatalf("過期的撤銷 = %v, %v", r, err)
	}
	if a, _ := f.m.Account(ctx, id); !slices.Equal(a.Roles, []identity.Role{identity.RoleSeller}) {
		t.Fatalf("角色 = %v", a.Roles)
	}
	// 移除最後一個角色也不刪帳號（帳號活著就是買家），即使舊呼叫方回報另一個角色 ABSENT。
	r, err = f.client.RemoveRole(ctx, connect.NewRequest(&pb.RemoveRoleRequest{
		AccountId: id, Role: pb.Role_ROLE_SELLER, SourceId: 12, OtherRoleState: pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT,
	}))
	if err != nil || r.Msg.GetCode() != 0 || r.Msg.GetAccountFullyDeleted() {
		t.Fatalf("移除賣家 = %v, %v", r, err)
	}
	// 買家不是角色：舊的 ROLE_BUYER 一律拒絕，帳號不變。
	r, err = f.client.RemoveRole(ctx, connect.NewRequest(&pb.RemoveRoleRequest{
		AccountId: id, Role: pb.Role_ROLE_BUYER, OtherRoleState: pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT,
	}))
	if err != nil || r.Msg.GetCode() != codeInvalidInput || r.Msg.GetAccountFullyDeleted() {
		t.Fatalf("移除買家 = %v, %v", r, err)
	}
	if a, err := f.m.Account(ctx, id); err != nil || a.Deleted || len(a.Roles) != 0 {
		t.Fatalf("帳號 = %+v, %v", a, err)
	}
	if len(f.published) != 0 {
		t.Fatalf("事件 = %+v", f.published)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(f.published) != 1 || f.published[0].Exchange != "hawkr.account" || f.published[0].RoutingKey != "account.deleted" {
		t.Fatalf("事件 = %+v", f.published)
	}
	var e struct {
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		AccountID int64  `json:"accountId"`
	}
	if err := json.Unmarshal(f.published[0].Body, &e); err != nil || e.EventType != "ACCOUNT_DELETED" || e.AccountID != id || e.EventID == "" || e.EventID != f.published[0].EventID {
		t.Fatalf("account.deleted = %s, %v", f.published[0].Body, err)
	}
	// 已刪除的帳號：移除賣家冪等成功，不再發事件。
	if r, err := f.client.RemoveRole(ctx, connect.NewRequest(&pb.RemoveRoleRequest{AccountId: id, Role: pb.Role_ROLE_SELLER})); err != nil || r.Msg.GetCode() != 0 {
		t.Fatalf("移除已刪帳號的賣家 = %v, %v", r, err)
	}

	r, err = f.client.RemoveRole(ctx, connect.NewRequest(&pb.RemoveRoleRequest{AccountId: id, Role: pb.Role_ROLE_UNSPECIFIED}))
	if err != nil || r.Msg.GetCode() != codeInvalidInput {
		t.Fatalf("不認得的角色 = %v, %v", r, err)
	}
	if len(f.published) != 1 {
		t.Fatalf("事件 %d 則", len(f.published))
	}
}

func TestSuspendAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	if s, err := f.client.SuspendAccount(ctx, connect.NewRequest(&pb.SuspendAccountRequest{AccountId: id, Reason: "BAN_USER"})); err != nil || !s.Msg.GetSuccess() {
		t.Fatalf("SuspendAccount = %v, %v", s, err)
	}
	if got := f.suspender.records; len(got) != 1 || got[0] != (accountsuspension.Record{Category: policy.Other, Reason: "BAN_USER"}) {
		t.Fatalf("交給 accountsuspension 的紀錄 = %+v，預期類別 OTHER、說明 BAN_USER", got)
	}
	if cur, ok, err := f.m.CurrentSuspension(ctx, id); err != nil || !ok || cur.Category != policy.Other {
		t.Fatalf("CurrentSuspension = %+v, %v, %v", cur, ok, err)
	}
	_, err := f.client.SuspendAccount(ctx, connect.NewRequest(&pb.SuspendAccountRequest{AccountId: 999}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("停權不存在的帳號 = %v", err)
	}
	f.suspender.err = apperr.FailedPrecondition("帳號擁有商店，不能停權").WithReason(apperr.ReasonAccountHasStore)
	_, err = f.client.SuspendAccount(ctx, connect.NewRequest(&pb.SuspendAccountRequest{AccountId: id}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("有店的帳號停權 = %v，預期 failed precondition", err)
	}
	f.suspender.err = apperr.InvalidInput("理由超過 1000 字元")
	_, err = f.client.SuspendAccount(ctx, connect.NewRequest(&pb.SuspendAccountRequest{AccountId: id}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("理由過長 = %v，預期 invalid argument", err)
	}
	f.suspender.err = nil
	// 還沒實作的方法（已經搬到 Connect 或刪除）回 unimplemented。
	_, err = f.client.GoogleOAuth(ctx, connect.NewRequest(&pb.GoogleOAuthRequest{}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnimplemented {
		t.Fatalf("GoogleOAuth = %v", err)
	}
}
