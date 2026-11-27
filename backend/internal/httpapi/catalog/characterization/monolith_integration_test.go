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
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/httpapi/catalog/catalogtest"
	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	catalogmod "github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
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

type noAccountOrders struct{}

func (noAccountOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noAccountOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return nil, nil
}

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
		VALUES ($1, $2, '測試店', 'logo/k', uuidv7(), '63000010', $3, $3)`, accountID, accountID, now)
	var storeID int64
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO seller.store_exposures (store_id) VALUES ($1)`, storeID)
	exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, now, uuid.NewV7())
	exec(`INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, now)
}

type monolith struct {
	db       *db.DB
	identity *identity.Module
	buyers   *buyer.Module
	sellers  *seller.Module
	srv      *httptest.Server
	adminToken string
}

func startMonolith(t *testing.T) *monolith {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalogmod.Migrations()},
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
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
	sellers := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: id, Media: media.NewFake(), Orders: noOrders{}, KYCMAC: sealtest.MAC(t),
		ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: queue, Now: time.Now, Logger: log}),
	})
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Identity: id, Buyer: buyers, Seller: sellers, Orders: noAccountOrders{}, Now: time.Now, Logger: log})
	catalogModule := catalogmod.New(catalogmod.Deps{DB: d, NewID: uuid.NewV7, Now: time.Now, Media: media.NewFake(), Restock: catalogtest.Notifier{}})
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{Procurement: &procurement.Module{}, Catalog: catalogModule, ImageAI: &imageai.Module{}, Assistant: &assistant.Module{},
		Ready: d.Ping, Identity: id, Seller: sellers, Social: &social.Module{}, Buyer: buyers, Images: images, Logger: log, Deletion: deletion,
		Delegation: delegation.New(delegation.Deps{DB: d, Identity: id, Seller: sellers, Social: &social.Module{}, Now: time.Now, Logger: log}),
		Stats:      ch.Stats(t), WebOrigin: ch.WebOrigin(t), DashboardOrders: noDashboardOrders{}, DashboardBudget: time.Minute,
		Discovery: &discovery.Module{}, ReviewQueue: &reviewqueue.Module{}, Review: &review.Module{}, CRM: &crm.Module{}, Coupon: &coupon.Module{},
		IM: &im.Module{}, BizCalendar: &bizcalendar.Store{}, Payment: &payment.Module{}, Moderation: &moderation.Module{}, Notification: &notification.Module{},
		BuyerPorts: httpbuyer.Ports{Stores: sellers},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	adminToken, err := dev.AdminToken("dev_admin_catalog")
	if err != nil {
		t.Fatal(err)
	}
	ch.GrantAdminOwner(t, id, adminToken)
	return &monolith{db: d, identity: id, buyers: buyers, sellers: sellers, srv: srv, adminToken: adminToken}
}

func (m *monolith) newCustomer(t *testing.T, withStore bool) (token, storeID string) {
	t.Helper()
	ctx := t.Context()
	account, token, err := m.identity.DevNewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if withStore {
		exposableStore(t, m.db, account)
		st, err := m.sellers.CurrentStore(ctx, account)
		if err != nil {
			t.Fatal(err)
		}
		storeID = rpc.FormatID(st.StoreID)
	}
	return token, storeID
}

// TestMonolith 對單體的 Connect 入口跑同一份情境（D8）。每個情境用自己的一組帳號與店家（買家是 dev session，賣家是 dev session 加一家可曝光的店）；
// 管理員是 dev 管理員（OWNER）。全部情境共用一個資料庫，情境只看名稱帶自己 nonce 的商品。
func TestMonolith(t *testing.T) {
	m := startMonolith(t)
	for _, sc := range scenarios {
		tokens := map[ch.Caller]string{}
		stores := map[ch.Caller]string{}
		for _, c := range []ch.Caller{ch.Buyer, ch.Buyer2, ch.Seller, ch.Seller2} {
			tokens[c], stores[c] = m.newCustomer(t, c == ch.Seller || c == ch.Seller2)
		}
		tokens[ch.Admin] = m.adminToken
		d := newConnectDriver(m.srv.Client(), m.srv.URL, tokens, stores)
		ch.Run[Driver](t, d, []ch.Scenario[Driver]{sc})
	}
}
