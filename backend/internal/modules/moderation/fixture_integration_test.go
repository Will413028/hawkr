//go:build integration

package moderation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// testQueue 記錄排入的 job（必須在 transaction 內，與真正的 jobs.Client 相同）。
type testQueue struct {
	mu   sync.Mutex
	args []river.JobArgs
}

func (q *testQueue) Enqueue(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = append(q.args, args)
	return nil
}

// syncJobs 回傳排入的、屬於這個 ref 的 media 同步 job 數。
func (q *testQueue) syncJobs(ref media.Ref) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, a := range q.args {
		if s, ok := a.(mediaref.SyncArgs); ok && s.RefType == ref.Type && s.RefID == ref.ID {
			n++
		}
	}
	return n
}

func (*testQueue) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type noOrders struct{}

func (noOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type noProductVerifier struct{}

func (noProductVerifier) VerifyProduct(context.Context, int64, int64) error {
	return social.ErrProductNotAvailable
}

// storeBlocksFromTable 是 social 的店家封鎖檢查在測試裡的替身（seller 自己的封鎖規則由 seller 的測試驗證）。
type storeBlocksFromTable struct{ db *db.DB }

func (s storeBlocksFromTable) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	var blocked bool
	err := s.db.Conn(ctx).QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM seller.store_blocks WHERE store_id = $1 AND buyer_id = $2)", storeID, buyerID).Scan(&blocked)
	return blocked, err
}

type fixture struct {
	db       *db.DB
	identity *identity.Module
	buyer    *buyer.Module
	seller   *seller.Module
	sellerRd *seller.Reader
	social   *social.Module
	deletion *accountdeletion.Module
	suspend  *accountsuspension.Module
	module   *Module
	media    *media.Fake
	jobs     *testQueue
	ids      atomic.Int64
	mu       sync.Mutex
	now      time.Time
}

func (f *fixture) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fixture) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: "moderation", Dir: Migrations()},
	))
	f := &fixture{db: d, now: time.Date(2026, 11, 2, 4, 0, 0, 0, time.UTC)}
	bus := events.NewBus()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := &testQueue{}
	f.jobs = q
	fm := media.NewFake()
	f.media = fm
	fm.Key = func(_, token string) string { return "post/" + token + ".jpg" }
	f.identity = identity.New(identity.Deps{DB: d, Events: bus, Now: f.clock, Logger: log})
	f.buyer = buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: q, Media: fm, Now: f.clock, Logger: log})
	f.seller = seller.New(seller.Deps{KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: f.identity, Media: fm, Orders: noOrders{}, ProductStores: noProducts{}, Registry: noRegistry{}, Now: f.clock, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, f.clock), Events: bus, Jobs: q, Now: f.clock, Logger: log})})
	f.sellerRd = seller.NewReader(d, f.clock)
	f.social = social.New(social.Deps{
		DB: d, Events: bus, Media: fm, Jobs: q, Buyers: f.buyer, Stores: f.sellerRd, StoreBlocks: storeBlocksFromTable{d}, Products: noProductVerifier{},
		NewID: uuid.NewV7, Now: f.clock, Logger: log,
	})
	f.deletion = accountdeletion.New(accountdeletion.Deps{DB: d, Identity: f.identity, Buyer: f.buyer, Seller: f.seller, Orders: noOrders{}, Now: f.clock})
	f.suspend = accountsuspension.New(accountsuspension.Deps{DB: d, Events: bus, Identity: f.identity, Seller: f.seller, Logger: log})
	f.module = New(Deps{DB: d, Social: f.social, Seller: f.seller, Buyer: f.buyer, Suspension: f.suspend, Media: fm, Jobs: q, NewID: uuid.NewV7, Now: f.clock, Logger: log})
	events.Subscribe(bus, f.buyer.OnAccountCreated)
	events.Subscribe(bus, f.social.OnBuyerDeletionRequested)
	events.Subscribe(bus, f.social.OnBuyerDeleted)
	events.Subscribe(bus, f.social.OnStoreRetired)
	events.Subscribe(bus, f.social.OnMeasureCreated)
	events.Subscribe(bus, f.social.OnMeasureRevoked)
	events.Subscribe(bus, f.module.Retention().OnBuyerDeletionRequested)
	events.Subscribe(bus, f.module.Retention().OnBuyerDeleted)
	// 測試只關心社群內容與 social 讀 seller 的 pub_exposable_stores：把 KYC 的表放寬成可以直接寫最小的列（它自己的規則在 seller 的測試）。
	f.exec(t, `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	return f
}

func (f *fixture) next() int64 { return f.ids.Add(1) + 100 }

func (f *fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

// newBuyer 建一位營運中的買家（走正式的建帳事件），回傳買家與帳號。
func (f *fixture) newBuyer(t *testing.T) (buyerID, accountID int64) {
	t.Helper()
	ctx := context.Background()
	if err := f.db.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := f.buyer.OnAccountCreated(ctx, identity.AccountCreated{AccountID: accountID, Name: fmt.Sprintf("買家%d", accountID)}); err != nil {
		t.Fatal(err)
	}
	p, err := f.buyer.Profile(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	return p.BuyerID, accountID
}

// exposableStore 建一家可以公開曝光的店（直接寫 seller 的表，最小的列），回傳 store_id 與帳號。
func (f *fixture) exposableStore(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	accountID = f.next()
	f.exec(t, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $2, '測試店', 'logo/k', 'logo-asset', '63000010', $3, $3)`, accountID, accountID, f.clock())
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, f.clock(), uuid.NewV7())
	f.exec(t, `INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, f.clock())
	return storeID, accountID
}

func (f *fixture) post(t *testing.T, storeID int64, body string) social.Post {
	t.Helper()
	st, err := f.sellerRd.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.social.CreatePost(context.Background(), st, social.CreatePostInput{
		Body: body, ImageToken: fmt.Sprintf("tok-%d", f.next()), Publish: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (f *fixture) comment(t *testing.T, postID uuid.UUID, buyerID int64, body string) social.Comment {
	t.Helper()
	c, err := f.social.CreateComment(context.Background(), social.Caller{BuyerID: buyerID}, social.CommentInput{PostID: postID, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (f *fixture) readable(t *testing.T, kind social.TargetKind, id uuid.UUID) bool {
	t.Helper()
	o, err := f.social.ContentOwner(context.Background(), social.Target{Kind: kind, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return o.Exists && o.Readable
}

func commentTarget(c social.Comment) Target { return Target{Kind: TargetComment, ContentID: c.ID} }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustV[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func kindOf(err error) apperr.Kind { return apperr.KindOf(err) }

func identityCreated(accountID int64) identity.AccountCreated {
	return identity.AccountCreated{AccountID: accountID, Name: "店主"}
}
