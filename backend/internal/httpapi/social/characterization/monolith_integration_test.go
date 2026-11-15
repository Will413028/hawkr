//go:build integration

package characterization

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
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

type noOrders struct{}

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

// 圖片參照是全數字的 token，轉正一律成功。
func newMedia() *media.Fake {
	f := media.NewFake()
	f.Key = func(refType, token string) string { return refType + "/" + token }
	return f
}

// exposableStore 為帳號開一家可以公開曝光的店（直接寫 seller 的表，最小的列；與 social 的 integration fixture 相同，KYC 的約束先放寬）。
func exposableStore(t *testing.T, d *db.DB, accountID int64) {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := d.Conn(ctx).Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	now := time.Now()
	exec(`DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	exec(`INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $2, '測試店', 'logo/k', 'logo-asset', '63000010', $3, $3)`, accountID, accountID, now)
	var storeID int64
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, now, uuid.NewV7())
	exec(`INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, now)
}

// TestMonolith 對單體的 Connect 入口跑同一份情境（D8）：handler 與 production 一樣由 httpapi.NewHandler 組出，資料庫是 testdb 的真實 Postgres。
// 買家是 dev session，賣家是 dev session 加一家可曝光的店；全部情境共用一個資料庫，情境只看自己建立的資料。
func TestMonolith(t *testing.T) {
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
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
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	fakeMedia := newMedia()
	sellers := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: id, Media: fakeMedia, Orders: noOrders{}, KYCMAC: sealtest.MAC(t),
		ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: queue, Now: time.Now, Logger: log}),
	})
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Identity: id, Buyer: buyers, Seller: sellers, Orders: noAccountOrders{}, Now: time.Now, Logger: log})
	delegationDeps := delegation.Deps{DB: d, Identity: id, Seller: sellers, Now: time.Now, Logger: log}
	socialModule := social.New(social.Deps{
		DB: d, Events: bus, Media: fakeMedia, Jobs: queue, Buyers: buyers, Stores: sellers, StoreBlocks: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log,
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: delegation.NewScheduleAuthorizer(delegationDeps)},
	})
	delegationDeps.Social = socialModule
	events.Subscribe(bus, socialModule.OnMeasureCreated)
	events.Subscribe(bus, socialModule.OnMeasureRevoked)
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{
		Ready: d.Ping, Identity: id, Seller: sellers, Social: socialModule, Buyer: buyers, Images: images, Logger: log, Deletion: deletion,
		Delegation: delegation.New(delegationDeps), Stats: ch.Stats(t), WebOrigin: ch.WebOrigin(t), DashboardOrders: noDashboardOrders{}, DashboardBudget: time.Minute,
		Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{}, Payment: &payment.Module{},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	tokens := map[ch.Caller]string{}
	for _, c := range []ch.Caller{ch.Buyer, ch.Buyer2, ch.Seller, ch.Seller2} {
		account, token, err := id.DevNewSession(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		tokens[c] = token
		if c == ch.Seller || c == ch.Seller2 {
			exposableStore(t, d, account)
		}
	}
	ch.Run[Driver](t, newConnectDriver(srv.Client(), srv.URL, tokens), scenarios)
}

type noAccountOrders struct{}

func (noAccountOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noAccountOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }
