//go:build integration

package im_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
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

func (q *queue) count(kind string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, a := range q.args {
		if a.Kind() == kind {
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

// orders 是 im 的訂單 port 的替身。
type orders struct {
	mu    sync.Mutex
	open  bool
	err   error
	calls int
	// inTx 記錄任何一次呼叫時 ctx 是否帶著 transaction（Send 不得在寫入 transaction 內呼叫 legacy）。
	inTx bool
}

func (o *orders) OpenOrderBetween(ctx context.Context, _, _ int64) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls++
	if _, ok := db.TxFrom(ctx); ok {
		o.inTx = true
	}
	return o.open, o.err
}

// buyers 是 im 的 Buyers port 的替身：買家的帳號預設是 1000 + buyer_id，測試可以改（同一個帳號同時有買家與店家身分）。
type buyers struct {
	mu       sync.Mutex
	accounts map[int64]int64
	blocking bool
	blockErr error
	// owner 是真正的 buyer 模組：守門 LockNotClosed 與關閉買家（CloseForDeletion）都走它，買家列在第一次被守門時自動建立（營運中）。
	owner *buyer.Module
	db    *db.DB
}

// ensureBuyer 建立指定 buyer_id 的買家列（營運中）；已存在不動。測試用固定的 ID，不經 OnAccountCreated。
func ensureBuyer(ctx context.Context, d *db.DB, id int64) error {
	_, err := d.Pool().Exec(ctx, `INSERT INTO buyer.buyers (buyer_id, account_id, display_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE VALUES ($1::bigint, 1000 + $1::bigint, 'b', now(), now()) ON CONFLICT (buyer_id) DO NOTHING`, id)
	return err
}

// ensureStore 建立指定 store_id 的店家列（營運中）；已存在不動。
func ensureStore(ctx context.Context, d *db.DB, id int64) error {
	_, err := d.Pool().Exec(ctx, `INSERT INTO seller.stores (store_id, store_code, account_id, store_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE VALUES ($1::bigint, $1::bigint, 2000 + $1::bigint, 's', now(), now()) ON CONFLICT (store_id) DO NOTHING`, id)
	return err
}

func (b *buyers) LockNotClosed(ctx context.Context, id int64) error {
	if err := ensureBuyer(ctx, b.db, id); err != nil {
		return err
	}
	return b.owner.LockNotClosed(ctx, id)
}

func (b *buyers) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if a, ok := b.accounts[id]; ok {
		return a, nil
	}
	return 1000 + id, nil
}

func (b *buyers) ProfileByBuyerID(context.Context, int64) (buyer.Profile, error) {
	return buyer.Profile{}, nil
}

func (b *buyers) IsBlockingStore(context.Context, int64, int64) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.blocking, b.blockErr
}

// stores 是 im 的 Stores port 的替身：店家預設是營運中、沒有停權，帳號是 2000 + store_id。
type stores struct {
	mu    sync.Mutex
	db    *db.DB
	byID  map[int64]seller.Store
	err   error
	block bool
	// blockErr 讓 StoreBlocksBuyer 回錯誤。
	blockErr error
	// afterLock 在 LockNotRetired 取到鎖之後呼叫（Send 的 transaction 此時持有買家與店家的守門鎖），測試用它把 transaction 停在持鎖中。
	afterLock func()
}

func (s *stores) Store(_ context.Context, id int64) (seller.Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return seller.Store{}, s.err
	}
	if st, ok := s.byID[id]; ok {
		return st, nil
	}
	return activeStore(id, 2000+id), nil
}

// LockNotRetired 是 seller.LockNotRetired 的同形替身（seller 的完整模組太重）：transaction 內對店家列 FOR KEY SHARE，已退場與不存在回 NotFound。
// 真正的 seller 實作由 seller 與 cmd/hawkr 的整合測試驗證。
func (s *stores) LockNotRetired(ctx context.Context, id int64) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("LockNotRetired 必須在 transaction 內呼叫")
	}
	if err := ensureStore(ctx, s.db, id); err != nil {
		return err
	}
	var retired bool
	err := s.db.Conn(ctx).QueryRow(ctx, "SELECT retired_at IS NOT NULL FROM seller.stores WHERE store_id = $1 FOR KEY SHARE", id).Scan(&retired)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && retired) {
		return apperr.NotFound("店家已退場")
	}
	if err == nil && s.afterLock != nil {
		s.afterLock()
	}
	return err
}

func (s *stores) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.block, s.blockErr
}

func activeStore(id, account int64) seller.Store {
	return seller.Store{
		StoreID: id, AccountID: account, Status: seller.Active, Sanctions: seller.NoSanctions(),
		Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true},
	}
}

type fixture struct {
	db     *db.DB
	m      *im.Module
	bus    *events.Bus
	jobs   *queue
	media  *media.Fake
	buyers *buyers
	stores *stores
	orders *orders

	clockMu sync.Mutex
	now     time.Time
	sent    []im.MessageSent
}

// clock 每次呼叫前進一毫秒：對話列表的排序鍵與訊息時間在測試中是確定的。
func (f *fixture) clock() time.Time {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	f.now = f.now.Add(time.Millisecond)
	return f.now
}

func setup(t *testing.T) *fixture {
	t.Helper()
	database := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "im", Dir: im.Migrations()},
	))
	f := &fixture{
		db: database, bus: events.NewBus(), jobs: &queue{}, media: media.NewFake(),
		buyers: &buyers{accounts: map[int64]int64{}}, stores: &stores{byID: map[int64]seller.Store{}}, orders: &orders{},
		now: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC),
	}
	f.media.Key = func(_, token string) string { return "chat/" + token + ".jpg" }
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	f.stores.db, f.buyers.db = database, database
	f.buyers.owner = buyer.New(buyer.Deps{DB: database, Events: f.bus, Jobs: f.jobs, Media: f.media, Now: f.clock, Logger: logger})
	f.m = im.New(im.Deps{
		DB: database, Events: f.bus, Jobs: f.jobs, Media: f.media, Buyers: f.buyers, Stores: f.stores, Orders: f.orders,
		NewID: uuid.NewV7, Now: f.clock, Logger: logger,
	})
	events.Subscribe(f.bus, f.m.OnBuyerDeleted)
	events.Subscribe(f.bus, f.m.OnStoreRetired)
	events.Subscribe(f.bus, func(_ context.Context, e im.MessageSent) error {
		f.clockMu.Lock()
		defer f.clockMu.Unlock()
		f.sent = append(f.sent, e)
		return nil
	})
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

func (f *fixture) buyerParty(id int64) im.Party { return im.Party{Kind: im.PartyBuyer, BuyerID: id} }

func (f *fixture) storeParty(t *testing.T, id int64) im.Party {
	t.Helper()
	st, err := f.stores.Store(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return im.Party{Kind: im.PartyStore, Store: &st}
}

// say 以對方的 ID 送出文字（首則訊息或既有對話都是這條路徑：對話由 (buyer_id, store_id) 決定）。
func (f *fixture) say(t *testing.T, from im.Party, counterpart int64, text string) im.Message {
	t.Helper()
	m, err := f.m.Send(context.Background(), im.SendInput{Sender: from, CounterpartID: counterpart, Type: im.TypeText, Text: text})
	if err != nil {
		t.Fatalf("send %q: %v", text, err)
	}
	return m
}

func kindOf(err error) apperr.Kind { return apperr.KindOf(err) }

// conversation 回傳 (buyer, store) 的對話 ID。
func (f *fixture) conversation(t *testing.T, buyerID, storeID int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT id FROM im.conversations WHERE buyer_id = $1 AND store_id = $2", buyerID, storeID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
