//go:build integration

package accountservice

import (
	"context"
	"fmt"
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
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type fixture struct {
	m      *identity.Module
	db     *db.DB
	dev    *identity.DevAuth
	client accountconnect.AccountServiceClient
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}))
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{dev: dev, db: d}
	bus := events.NewBus()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.m = identity.New(identity.Deps{
		DB: d, Events: bus, Providers: identity.Providers{},
		Now: time.Now, Logger: log,
	})
	mux := http.NewServeMux()
	mux.Handle(accountconnect.NewAccountServiceHandler(New(f.m, log)))
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(srv.Close)
	f.client = accountconnect.NewAccountServiceClient(grpcclient.HTTPClient(), srv.URL, connect.WithGRPC(),
		connect.WithInterceptors(grpcclient.Signer("secret", nil)))
	return f
}

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
	if _, err := f.db.Pool().Exec(ctx, `UPDATE identity.accounts SET email = $1 WHERE id = $2`, fmt.Sprintf("dev_%d@dev.localhost", id), id); err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		if err := f.m.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: r}); err != nil {
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
}

func TestRolesAndDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	if err := f.m.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: identity.RoleSeller}); err != nil {
		t.Fatal(err)
	}
	if a, _ := f.m.Account(ctx, id); !slices.Equal(a.Roles, []identity.Role{identity.RoleSeller}) {
		t.Fatalf("角色 = %v", a.Roles)
	}
	if a, err := f.m.Account(ctx, id); err != nil || a.Deleted || !slices.Equal(a.Roles, []identity.Role{identity.RoleSeller}) {
		t.Fatalf("帳號 = %+v, %v", a, err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if a, err := f.m.Account(ctx, id); err != nil || !a.Deleted {
		t.Fatalf("刪除後的帳號 = %+v, %v", a, err)
	}
}
