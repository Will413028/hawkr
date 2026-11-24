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
		VALUES ($1, $2, '測試店', 'logo/k', 'logo-asset', '63000010', $3, $3)`, accountID, accountID, now)
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

// sellerStores 是 coupon 的 Stores port 的 adapter（與 cmd/hawkr 的 couponStores 同一個做法：問 seller 店家是不是停權）。
type sellerStores struct{ sellers *seller.Module }

func (s sellerStores) StoreSuspended(ctx context.Context, storeID int64) (bool, error) {
	st, err := s.sellers.Store(ctx, storeID)
	if err != nil {
		return false, err
	}
	return st.Suspended(), nil
}

// noMembers 是 MEMBER 資格的 port：投影還沒有接上，這裡的情境不涉及 MEMBER 資格。
type noMembers struct{}

func (noMembers) IsMember(context.Context, int64, int64) (bool, error) { return false, nil }

// monolith 是單體的 Connect 入口加它的資料庫、identity 與 coupon 模組：handler 與 production 一樣由 httpapi.NewHandler 組出，資料庫是 testdb 的真實 Postgres。
type monolith struct {
	db       *db.DB
	identity *identity.Module
	buyers   *buyer.Module
	sellers  *seller.Module
	coupon   *coupon.Module
	crm      *crm.Module
	srv      *httptest.Server
}

func startMonolith(t *testing.T) *monolith {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
		migrate.Module{Name: "coupon", Dir: coupon.Migrations()},
		migrate.Module{Name: "crm", Dir: crm.Migrations()},
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
		DB: d, Events: bus, Jobs: queue, Identity: id, Media: newMedia(), Orders: noOrders{}, KYCMAC: sealtest.MAC(t),
		ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: queue, Now: time.Now, Logger: log}),
	})
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Identity: id, Buyer: buyers, Seller: sellers, Orders: noAccountOrders{}, Now: time.Now, Logger: log})
	crmModule := crm.New(crm.Deps{DB: d, Jobs: queue, Buyers: buyers, Stores: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log})
	couponModule := coupon.New(coupon.Deps{DB: d, Stores: sellerStores{sellers}, Members: noMembers{}, NewID: uuid.NewV7, Now: time.Now})
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{Procurement: &procurement.Module{}, ImageAI: &imageai.Module{}, Assistant: &assistant.Module{},
		Ready: d.Ping, Identity: id, Seller: sellers, Social: &social.Module{}, Buyer: buyers, Images: images, Logger: log, Deletion: deletion,
		Delegation: delegation.New(delegation.Deps{DB: d, Identity: id, Seller: sellers, Social: &social.Module{}, Now: time.Now, Logger: log}),
		Stats:      ch.Stats(t), WebOrigin: ch.WebOrigin(t), DashboardOrders: noDashboardOrders{}, DashboardBudget: time.Minute,
		Discovery: &discovery.Module{}, ReviewQueue: &reviewqueue.Module{}, Review: &review.Module{}, CRM: crmModule, Coupon: couponModule,
		IM: &im.Module{}, BizCalendar: &bizcalendar.Store{}, Payment: &payment.Module{}, Moderation: &moderation.Module{}, Notification: &notification.Module{},
		BuyerPorts: httpbuyer.Ports{Stores: sellers},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &monolith{db: d, identity: id, buyers: buyers, sellers: sellers, coupon: couponModule, crm: crmModule, srv: srv}
}

// newCustomer 建立一個 dev session 帳號；withStore 為真時再為它開一家可曝光的店。回傳 token 與身分。
func (m *monolith) newCustomer(t *testing.T, withStore bool) (string, who) {
	t.Helper()
	ctx := t.Context()
	account, token, err := m.identity.DevNewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	w := who{Account: rpc.FormatID(account)}
	buyerID, err := m.buyers.CurrentBuyerID(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	w.Buyer = rpc.FormatID(buyerID)
	if withStore {
		exposableStore(t, m.db, account)
		st, err := m.sellers.CurrentStore(ctx, account)
		if err != nil {
			t.Fatal(err)
		}
		w.Store = rpc.FormatID(st.StoreID)
	}
	return token, w
}

// TestMonolith 對單體的 Connect 入口跑同一份情境（D8）。每個情境用自己的一組帳號與店家（買家是 dev session，賣家是 dev session 加一家可曝光的店）；
// 管理員的準備動作直接呼叫 coupon 模組（還沒有 admin 的 Connect API）。全部情境共用一個資料庫。
func TestMonolith(t *testing.T) {
	m := startMonolith(t)
	for _, sc := range scenarios {
		tokens := map[ch.Caller]string{}
		ids := map[ch.Caller]who{}
		for _, c := range []ch.Caller{ch.Buyer, ch.Buyer2, ch.Seller, ch.Seller2} {
			tokens[c], ids[c] = m.newCustomer(t, c == ch.Seller || c == ch.Seller2)
		}
		d := newConnectDriver(m.srv.Client(), m.srv.URL, tokens, ids, m.coupon, m.crm, m.db)
		ch.Run[Driver](t, d, []ch.Scenario[Driver]{sc})
	}
}

type noAccountOrders struct{}

func (noAccountOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noAccountOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }
