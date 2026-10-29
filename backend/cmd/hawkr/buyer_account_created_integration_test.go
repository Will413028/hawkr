//go:build integration

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type accountCreatedClerk struct{}

func (accountCreatedClerk) Issuer() string          { return "https://clerk.account-created.test" }
func (accountCreatedClerk) OwnsUser(id string) bool { return strings.HasPrefix(id, "user_") }
func (accountCreatedClerk) Verify(_ context.Context, _ string) (identity.ClerkIdentity, error) {
	return identity.ClerkIdentity{Subject: "user_created", Name: "新買家"}, nil
}
func (accountCreatedClerk) DeleteUser(context.Context, string) error { return nil }

func TestAccountCreationProvisionsBuyerInSameTransaction(t *testing.T) {
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "buyer", Dir: buyer.Migrations()}))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	clock := func() time.Time { return time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC) }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	accounts := identity.New(identity.Deps{DB: d, Events: bus, Jobs: queue, Providers: identity.Providers{Customers: []identity.Clerk{accountCreatedClerk{}}}, Now: clock, Logger: logger})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: queue, Now: clock, Logger: logger})
	subscribeBuyerLifecycle(bus, buyers)
	claims, _ := json.Marshal(map[string]string{"iss": accountCreatedClerk{}.Issuer(), "sub": "user_created"})
	token := "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".sig"
	authenticated, err := accounts.Authenticate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	// 買家不是角色（W5）：建帳之後沒有任何角色，買家由訂閱 AccountCreated 在同一個 transaction 建立。
	if len(authenticated.Principal.Roles) != 0 {
		t.Fatalf("created roles=%v", authenticated.Principal.Roles)
	}
	profile, err := buyers.Profile(ctx, authenticated.Principal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.BuyerID <= 0 || profile.DisplayName != "新買家" {
		t.Fatalf("profile=%+v", profile)
	}
}
