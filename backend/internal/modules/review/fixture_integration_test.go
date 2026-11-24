//go:build integration

package review_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	goimage "image"
	"image/png"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
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

func (q *queue) kinds() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []string
	for _, a := range q.args {
		out = append(out, a.Kind())
	}
	return out
}

func (q *queue) count(kind string) int {
	n := 0
	for _, k := range q.kinds() {
		if k == kind {
			n++
		}
	}
	return n
}

func (q *queue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = nil
}

// orders 是 review 的訂單 port 的替身：以訂單號查，錯誤由測試設定（ErrOrderNotFound 或暫時性故障）。
type orders struct {
	mu    sync.Mutex
	byNo  map[string]review.OrderForReview
	err   error
	calls int
}

func (o *orders) OrderForReview(_ context.Context, orderNo string) (review.OrderForReview, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls++
	if o.err != nil {
		return review.OrderForReview{}, o.err
	}
	v, ok := o.byNo[orderNo]
	if !ok {
		return review.OrderForReview{}, review.ErrOrderNotFound
	}
	return v, nil
}

type fixture struct {
	db      *db.DB
	module  *review.Module
	buyers  *buyer.Module
	sellers *seller.Reader
	bus     *events.Bus
	jobs    *queue
	media   *media.Module
	orders  *orders

	mu      sync.Mutex
	now     time.Time
	ids     atomic.Int64
	created []review.ReviewCreated
	changed []review.ReviewAggregateChanged
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
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "review", Dir: review.Migrations()},
	))
	f := &fixture{
		db: database, bus: events.NewBus(), jobs: &queue{}, orders: &orders{byNo: map[string]review.OrderForReview{}},
		now: time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC),
	}
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	ring, err := media.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	f.media = media.New(media.Deps{DB: database, Jobs: f.jobs, Store: media.NewMemoryStore(), Keyring: ring, NewID: uuid.NewV7, Now: f.clock, Logger: logger})
	f.buyers = buyer.New(buyer.Deps{DB: database, Events: f.bus, Media: f.media, Now: f.clock, Logger: logger})
	f.sellers = seller.NewReader(database, f.clock)
	f.module = review.New(review.Deps{
		DB: database, Events: f.bus, Media: f.media, Jobs: f.jobs, Buyers: f.buyers, Stores: f.sellers, Orders: f.orders,
		NewID: uuid.NewV7, Now: f.clock, Logger: logger,
	})
	events.Subscribe(f.bus, func(_ context.Context, e review.ReviewCreated) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.created = append(f.created, e)
		return nil
	})
	events.Subscribe(f.bus, func(_ context.Context, e review.ReviewAggregateChanged) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.changed = append(f.changed, e)
		return nil
	})
	f.exec(t, `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	return f
}

// stage 以 b 的帳號為上傳者放 n 張公開 PNG，回傳可 Confirm 的 token 與資產 ID（同順序）。
func (f *fixture) stage(t *testing.T, b buyer.Profile, n int) ([]string, []uuid.UUID) {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	tokens, assets := make([]string, n), make([]uuid.UUID, n)
	for i := range n {
		put, err := f.media.PutPublic(context.Background(), media.Uploader{ID: b.AccountID}, bytes.NewReader(body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		tokens[i] = put.Token
		if err := f.db.Conn(context.Background()).QueryRow(context.Background(),
			"SELECT asset_id FROM media.staged_uploads WHERE token_hash = sha256($1::bytea)", put.Token).Scan(&assets[i]); err != nil {
			t.Fatal(err)
		}
	}
	return tokens, assets
}

func (f *fixture) assetState(t *testing.T, id uuid.UUID) string {
	t.Helper()
	st, err := media.AssetStateOf(context.Background(), f.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(st)
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

// exposableStore 建一家店（直接寫 seller 的表，最小的列），回傳 store_id。
func (f *fixture) exposableStore(t *testing.T) int64 {
	t.Helper()
	accountID := f.next()
	f.exec(t, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $2, '測試店', 'logo/k', $4, '63000010', $3, $3)`, accountID, accountID, f.clock(), uuid.NewV7())
	storeID := f.scalar(t, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID)
	f.exec(t, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, f.clock(), uuid.NewV7())
	f.exec(t, `INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, f.clock())
	return storeID
}

func (f *fixture) store(t *testing.T, storeID int64) seller.Store {
	t.Helper()
	s, err := f.sellers.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

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

// item 是登記進訂單替身的一個項目；訂單已完成一天。
type item struct {
	orderNo string
	itemID  int64
}

func (f *fixture) order(b buyer.Profile, storeID, productID int64) item {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.ids.Add(1) + 1000
	no := fmt.Sprintf("N%d", n)
	f.orders.byNo[no] = review.OrderForReview{
		OrderNo: no, BuyerID: b.BuyerID, StoreID: storeID, Status: review.OrderStatusCompleted, CompletedAt: f.now.Add(-24 * time.Hour),
		Items: []review.OrderItem{{OrderItemID: n, ProductID: productID, SkuName: "紅茶 100g", Price: 120, ImageKey: "order/img.jpg"}},
	}
	return item{no, n}
}

func (f *fixture) submit(t *testing.T, b buyer.Profile, storeID, productID int64, rating int, content string) review.Review {
	t.Helper()
	it := f.order(b, storeID, productID)
	r, err := f.module.Submit(context.Background(), review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: rating, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	f.advance(time.Minute)
	return r
}

func ids(p review.Page) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Reviews))
	for i, r := range p.Reviews {
		out[i] = r.ID
	}
	return out
}

func sameIDs(got []uuid.UUID, want ...uuid.UUID) bool { return slices.Equal(got, want) }

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func kindOf(err error) apperr.Kind { return apperr.KindOf(err) }

func revalidations(q *queue) [][]string {
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
