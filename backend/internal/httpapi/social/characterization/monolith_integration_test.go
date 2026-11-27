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

// 圖片參照是全數字的 token，確認一律成功。
func newMedia() *media.Fake { return media.NewFake() }

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
		VALUES ($1, $2, '測試店', 'logo/k', uuidv7(), '63000010', $3, $3)`, accountID, accountID, now)
	var storeID int64
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	// 店家的公告列開店時與店家列一起建立；撤銷措施會重算停權並重新公告，沒有這一列會回 internal。
	exec(`INSERT INTO seller.store_exposures (store_id) VALUES ($1)`, storeID)
	exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, now, uuid.NewV7())
	exec(`INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, now)
}

// monolith 是單體的 Connect 入口加它的資料庫與 identity：handler 與 production 一樣由 httpapi.NewHandler 組出，資料庫是 testdb 的真實 Postgres。
type monolith struct {
	db         *db.DB
	identity   *identity.Module
	srv        *httptest.Server
	adminToken string
}

func startMonolith(t *testing.T) *monolith {
	t.Helper()
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
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
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
		DB: d, Events: bus, Media: ownerMedia{}, Jobs: queue, Buyers: buyers, Stores: sellers, StoreBlocks: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log,
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: delegation.NewScheduleAuthorizer(delegationDeps)},
	})
	delegationDeps.Social = socialModule
	events.Subscribe(bus, socialModule.OnMeasureCreated)
	events.Subscribe(bus, socialModule.OnMeasureRevoked)
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{Procurement: &procurement.Module{}, Catalog: &catalogmod.Module{}, ImageAI: &imageai.Module{}, Assistant: &assistant.Module{},
		Ready: d.Ping, Identity: id, Seller: sellers, Social: socialModule, Buyer: buyers, Images: images, Logger: log, Deletion: deletion,
		Delegation: delegation.New(delegationDeps), Stats: ch.Stats(t), WebOrigin: ch.WebOrigin(t), DashboardOrders: noDashboardOrders{}, DashboardBudget: time.Minute,
		Discovery: &discovery.Module{}, ReviewQueue: &reviewqueue.Module{}, Review: &review.Module{}, CRM: &crm.Module{}, Coupon: &coupon.Module{}, IM: &im.Module{}, BizCalendar: &bizcalendar.Store{}, Payment: &payment.Module{}, Moderation: &moderation.Module{}, Notification: &notification.Module{},
		BuyerPorts: httpbuyer.Ports{Stores: sellers},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	adminToken, err := dev.AdminToken("dev_admin_characterization")
	if err != nil {
		t.Fatal(err)
	}
	ch.GrantAdminOwner(t, id, adminToken)
	return &monolith{db: d, identity: id, srv: srv, adminToken: adminToken}
}

// newCustomer 建立一個 dev session 帳號；withStore 為真時再為它開一家可曝光的店。
func (m *monolith) newCustomer(t *testing.T, withStore bool) string {
	t.Helper()
	account, token, err := m.identity.DevNewSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if withStore {
		exposableStore(t, m.db, account)
	}
	return token
}

// TestMonolith 對單體的 Connect 入口跑同一份情境（D8）。買家是 dev session，賣家是 dev session 加一家可曝光的店，管理員是 dev admin token（OWNER）；
// 全部情境共用一個資料庫，每個情境用自己的帳號與店家。
func TestMonolith(t *testing.T) {
	m := startMonolith(t)
	// 情境會留下封鎖等狀態，每個情境用自己的一組帳號與店家，彼此不影響。
	for _, sc := range scenarios {
		tokens := map[ch.Caller]string{ch.Admin: m.adminToken}
		for _, c := range []ch.Caller{ch.Buyer, ch.Buyer2, ch.Seller, ch.Seller2} {
			tokens[c] = m.newCustomer(t, c == ch.Seller || c == ch.Seller2)
		}
		ch.Run[Driver](t, newConnectDriver(m.srv.Client(), m.srv.URL, tokens), []ch.Scenario[Driver]{sc})
	}
}

type noAccountOrders struct{}

func (noAccountOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noAccountOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

// ownerMedia 是 social 的 media 替身：每個 staged token 都確認成功，資產 ID 每次新產生。
type ownerMedia struct{}

func (ownerMedia) Confirm(_ context.Context, _ media.Uploader, token string, _ media.Visibility) (media.ConfirmedAsset, error) {
	return media.ConfirmedAsset{ID: uuid.NewV7(), ObjectKey: "post/" + token + ".jpg", Visibility: media.VisibilityPublic}, nil
}

func (ownerMedia) MarkDeleted(context.Context, []uuid.UUID) error { return nil }
