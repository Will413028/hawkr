//go:build integration

package im_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	goimage "image"
	"image/png"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

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

type orders struct {
	mu    sync.Mutex
	open  bool
	err   error
	calls int
	inTx  bool
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

type buyers struct {
	mu       sync.Mutex
	accounts map[int64]int64
	blocking bool
	blockErr error
	owner    *buyer.Module
}

func seedBuyers(t *testing.T, d *db.DB, buyerIDs []int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Pool().Exec(ctx, `INSERT INTO buyer.buyers (buyer_id, account_id, display_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE SELECT g, 1000 + g, 'b', now(), now() FROM unnest($1::bigint[]) AS g`, buyerIDs); err != nil {
		t.Fatal(err)
	}
}

func (b *buyers) LockNotClosed(ctx context.Context, id int64) error {
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

type stores struct {
	mu        sync.Mutex
	owner     *seller.Module
	byID      map[int64]seller.Store
	err       error
	block     bool
	blockErr  error
	afterLock func()
}

func (s *stores) Store(ctx context.Context, id int64) (seller.Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return seller.Store{}, s.err
	}
	if st, ok := s.byID[id]; ok {
		return st, nil
	}
	return s.owner.Store(ctx, id)
}

// LockNotRetired 委派給真正的 seller 模組（守門的查詢與鎖由它擁有），取到鎖之後呼叫 afterLock。
func (s *stores) LockNotRetired(ctx context.Context, id int64) error {
	err := s.owner.LockNotRetired(ctx, id)
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

func (s *stores) StoresBlockingBuyer(_ context.Context, _ int64, ids []int64) (map[int64]bool, error) {
	return s.blockedAll(ids)
}

func (s *stores) BuyersBlockedByStore(_ context.Context, _ int64, ids []int64) (map[int64]bool, error) {
	return s.blockedAll(ids)
}

func (s *stores) blockedAll(ids []int64) (map[int64]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[int64]bool{}
	if s.block {
		for _, id := range ids {
			out[id] = true
		}
	}
	return out, s.blockErr
}

func activeStore(id, account int64) seller.Store {
	return seller.Store{
		StoreID: id, AccountID: account, Status: seller.Active, Sanctions: seller.NoSanctions(),
		Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true},
	}
}

func (*queue) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return nil, nil
}

type noOrders struct{}

func (noOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type fixture struct {
	db     *db.DB
	m      *im.Module
	bus    *events.Bus
	jobs   *queue
	media  *media.Module
	buyers *buyers
	stores *stores
	orders *orders

	clockMu sync.Mutex
	now     time.Time
	s1      int64
	s2      int64
	sent    []im.MessageSent
}

func (f *fixture) clock() time.Time {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	f.now = f.now.Add(time.Millisecond)
	return f.now
}

func (f *fixture) advance(d time.Duration) {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	f.now = f.now.Add(d)
}

func setup(t *testing.T) *fixture {
	t.Helper()
	database := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "im", Dir: im.Migrations()},
	))
	f := &fixture{
		db: database, bus: events.NewBus(), jobs: &queue{},
		buyers: &buyers{accounts: map[int64]int64{}}, stores: &stores{byID: map[int64]seller.Store{}}, orders: &orders{},
		now: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC),
	}
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	ring, err := media.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	f.media = media.New(media.Deps{DB: database, Jobs: f.jobs, Store: media.NewMemoryStore(), Keyring: ring, NewID: uuid.NewV7, Now: f.clock, Logger: logger})
	bus := f.bus
	identityModule := identity.New(identity.Deps{DB: database, Events: bus, Now: f.clock, Logger: logger})
	f.stores.owner = seller.New(seller.Deps{KYCMAC: sealtest.MAC(t), DB: database, Events: bus, Jobs: f.jobs, Identity: identityModule, Media: f.media, Orders: noOrders{}, ProductStores: noProducts{}, Registry: noRegistry{}, Now: f.clock, Logger: logger,
		Payment: payment.New(payment.Deps{DB: database, Audit: adminaudit.New(database, f.clock), Events: bus, Jobs: f.jobs, Now: f.clock, Logger: logger})})
	seedBuyers(t, database, []int64{b1, b2, 5, 7})
	for i := range 2 {
		accountID, err := sellerfixture.NewAccount(context.Background(), database, f.clock())
		if err != nil {
			t.Fatal(err)
		}
		store, err := f.stores.owner.Apply(context.Background(), seller.ApplyInput{
			AccountID: accountID, StoreName: fmt.Sprintf("測試店 %d", i+1), ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號",
			AcceptedAgreements: seller.CurrentApplyAcceptances(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			f.s1 = store.StoreID
		} else {
			f.s2 = store.StoreID
		}
	}
	f.buyers.owner = buyer.New(buyer.Deps{DB: database, Events: f.bus, Media: f.media, Now: f.clock, Logger: logger})
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

func (f *fixture) stage(t *testing.T, accountID int64) (string, uuid.UUID) {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	put, err := f.media.PutPublic(context.Background(), media.Uploader{ID: accountID}, &body)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT asset_id FROM media.staged_uploads WHERE token_hash = sha256($1::bytea)", put.Token).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return put.Token, id
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

func (f *fixture) buyerParty(id int64) im.Party { return im.Party{Kind: im.PartyBuyer, BuyerID: id} }

func (f *fixture) storeParty(t *testing.T, id int64) im.Party {
	t.Helper()
	st, err := f.stores.Store(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return im.Party{Kind: im.PartyStore, Store: &st}
}

func (f *fixture) say(t *testing.T, from im.Party, counterpart int64, text string) im.Message {
	t.Helper()
	m, err := f.m.Send(context.Background(), im.SendInput{RequestID: uuid.NewV4(), Sender: from, CounterpartID: counterpart, Type: im.TypeText, Text: text})
	if err != nil {
		t.Fatalf("send %q: %v", text, err)
	}
	return m
}

func kindOf(err error) apperr.Kind { return apperr.KindOf(err) }

func (f *fixture) conversation(t *testing.T, buyerID, storeID int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT id FROM im.conversations WHERE buyer_id = $1 AND store_id = $2", buyerID, storeID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
