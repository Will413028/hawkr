//go:build integration

package social_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// queue 記錄排入的 job（必須在 transaction 內，與真正的 jobs.Client 相同）。
type queue struct {
	mu   sync.Mutex
	args []river.JobArgs
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = append(q.args, args)
	return nil
}

func (q *queue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = nil
}

// revalidations 回傳目前排入的失效 job 的 tag 清單。
func (q *queue) revalidations() [][]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out [][]string
	for _, a := range q.args {
		if r, ok := a.(revalidate.Args); ok {
			out = append(out, r.Tags)
		}
	}
	return out
}

func (q *queue) kinds() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []string
	for _, a := range q.args {
		out = append(out, a.Kind())
	}
	return out
}

type products struct {
	mu    sync.Mutex
	owner map[int64]int64 // 商品 → 店家
	err   error
}

func (p *products) VerifyProduct(_ context.Context, productID, storeID int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	owner, ok := p.owner[productID]
	switch {
	case !ok:
		return social.ErrProductNotAvailable
	case owner != storeID:
		return social.ErrProductNotOwned
	}
	return nil
}

type fixture struct {
	db       *db.DB
	module   *social.Module
	buyers   *buyer.Module
	sellers  *seller.Reader
	bus      *events.Bus
	jobs     *queue
	media    *media.Fake
	products *products

	mu   sync.Mutex
	now  time.Time
	ids  atomic.Int64
	logs syncBuffer
}

// syncBuffer 收集模組的 log（並行安全），測試用它檢查 warn。
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
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
	database := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
	))
	f := &fixture{
		db: database, bus: events.NewBus(), jobs: &queue{}, media: media.NewFake(),
		products: &products{owner: map[int64]int64{}},
		now:      time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
	f.media.Key = func(_, token string) string { return "post/" + token + ".jpg" }
	logger := slog.New(slog.NewTextHandler(&f.logs, nil))
	f.buyers = buyer.New(buyer.Deps{DB: database, Events: f.bus, Jobs: f.jobs, Media: f.media, Now: f.clock, Logger: logger})
	f.sellers = seller.NewReader(database, f.clock)
	f.module = social.New(social.Deps{
		DB: database, Events: f.bus, Media: f.media, Jobs: f.jobs, Buyers: f.buyers, Stores: f.sellers, StoreBlocks: storeBlocksFromTable{database}, Products: f.products,
		NewID: uuid.NewV7, Now: f.clock, Logger: logger,
	})
	events.Subscribe(f.bus, f.module.OnBuyerDeletionRequested)
	events.Subscribe(f.bus, f.module.OnBuyerDeleted)
	events.Subscribe(f.bus, f.module.OnStoreRetired)
	// 測試只關心 social 讀 seller 的 pub_exposable_stores：把 KYC 與 PayUni 的表放寬成可以直接寫最小的列（它們自己的規則在 seller 的測試）。
	f.exec(t, `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid IN ('seller.kyc_applications'::regclass, 'seller.payuni_registrations'::regclass) AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	return f
}

func (f *fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}

func (f *fixture) scalar(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

func (f *fixture) next() int64 { return f.ids.Add(1) + 100 }

// exposableStore 建一家可以公開曝光的店（直接寫 seller 的表，最小的列），回傳 store_id 與帳號。
func (f *fixture) exposableStore(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	accountID = f.next()
	f.exec(t, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $2, '測試店', 'logo/k', 'logo-asset', '63000010', $3, $3)`, accountID, accountID, f.clock())
	storeID = f.scalar(t, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID)
	f.exec(t, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, f.clock(), uuid.NewV7())
	f.exec(t, `INSERT INTO seller.payuni_registrations (store_id, mer_id, application_id, bind_status, bound_at, outcome, status_observed_at, created_at, updated_at)
		SELECT $1::bigint, 'HAWK' || $1::bigint::text, application_id, 'BOUND', $2, 'APPROVED', $2, $2, $2 FROM seller.kyc_applications WHERE store_id = $1::bigint`, storeID, f.clock())
	return storeID, accountID
}

// setExposable 讓店家回到可曝光（true）或不可曝光（false，KYC 退回）。
func (f *fixture) setExposable(t *testing.T, storeID int64, ok bool) {
	t.Helper()
	status := "APPROVED"
	if !ok {
		status = "REJECTED"
	}
	f.exec(t, "UPDATE seller.kyc_applications SET status = $2 WHERE store_id = $1", storeID, status)
}

// store 讀出店家（已推導條件），作為店家身分的呼叫者。
func (f *fixture) store(t *testing.T, storeID int64) seller.Store {
	t.Helper()
	s, err := f.sellers.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fixture) asStore(t *testing.T, storeID int64) social.Caller {
	t.Helper()
	s := f.store(t, storeID)
	return social.Caller{Store: &s}
}

// newBuyer 建一位營運中的買家。
func (f *fixture) newBuyer(t *testing.T) buyer.Profile {
	t.Helper()
	id := f.next()
	if err := f.buyers.OnAccountCreated(context.Background(), identity.AccountCreated{AccountID: id, Name: fmt.Sprintf("買家%d", id)}); err != nil {
		t.Fatal(err)
	}
	p, err := f.buyers.Profile(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (f *fixture) asBuyer(p buyer.Profile) social.Caller { return social.Caller{BuyerID: p.BuyerID} }

// publishedPost 建一篇已發布的貼文（圖是新上傳的 token）。
func (f *fixture) publishedPost(t *testing.T, storeID int64, body string, tags ...string) social.Post {
	t.Helper()
	p, err := f.module.CreatePost(context.Background(), f.store(t, storeID), social.CreatePostInput{
		Body: body, ImageToken: fmt.Sprintf("tok-%d", f.next()), Hashtags: tags, Publish: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

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

type (
	uuidT  = uuid.UUID
	buyerP = buyer.Profile
)

func uuidNew() uuid.UUID { return uuid.NewV7() }

// storeBlocksFromTable 是 social 的店家封鎖檢查在測試裡的替身：直接讀 seller.store_blocks（seller 自己的封鎖規則由 seller 的測試驗證）。
type storeBlocksFromTable struct{ db *db.DB }

func (s storeBlocksFromTable) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	var blocked bool
	err := s.db.Conn(ctx).QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM seller.store_blocks WHERE store_id = $1 AND buyer_id = $2)", storeID, buyerID).Scan(&blocked)
	return blocked, err
}
