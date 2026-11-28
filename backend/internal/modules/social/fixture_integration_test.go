//go:build integration

package social_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	goimage "image"
	"image/png"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/post"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type autoMedia struct {
	*media.Module
	mu     sync.Mutex
	tokens map[string]string
}

func (m *autoMedia) Confirm(ctx context.Context, u media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	key := fmt.Sprintf("%d/%s", u.ID, token)
	m.mu.Lock()
	real, ok := m.tokens[key]
	if !ok {
		var body bytes.Buffer
		if err := png.Encode(&body, goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))); err != nil {
			m.mu.Unlock()
			return media.ConfirmedAsset{}, err
		}
		put, err := m.PutPublic(context.Background(), u, &body)
		if err != nil {
			m.mu.Unlock()
			return media.ConfirmedAsset{}, err
		}
		real = put.Token
		m.tokens[key] = real
	}
	m.mu.Unlock()
	return m.Module.Confirm(ctx, u, real, want)
}

func (f *fixture) assetState(t *testing.T, id uuid.UUID) string {
	t.Helper()
	st, err := media.AssetStateOf(context.Background(), f.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(st)
}

func (f *fixture) postAsset(t *testing.T, postID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT image_asset_id FROM social.posts WHERE id=$1", postID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

type queue struct {
	mu   sync.Mutex
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}

func (q *queue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args, q.opts = nil, nil
}

func (q *queue) scheduledJobs() map[uuid.UUID]time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := map[uuid.UUID]time.Time{}
	for i, a := range q.args {
		if j, ok := a.(post.PublishScheduledArgs); ok {
			var at time.Time
			if q.opts[i] != nil {
				at = q.opts[i].ScheduledAt
			}
			out[j.ScheduleID] = at
		}
	}
	return out
}

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
	owner map[uuid.UUID]int64
	err   error
}

func (p *products) VerifyProduct(_ context.Context, productID uuid.UUID, storeID int64) error {
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

type scheduleAuth struct {
	mu    sync.Mutex
	err   error
	calls []social.ScheduledPublish
}

func (a *scheduleAuth) AuthorizeScheduledPublish(_ context.Context, p social.ScheduledPublish) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, p)
	return a.err
}

func (a *scheduleAuth) set(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.err = err
}

func (a *scheduleAuth) asked() []social.ScheduledPublish {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]social.ScheduledPublish(nil), a.calls...)
}

type eventLog struct {
	mu        sync.Mutex
	published []social.PostPublished
	canceled  []social.PostScheduleCanceled
}

func (e *eventLog) publishedEvents() []social.PostPublished {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]social.PostPublished(nil), e.published...)
}

func (e *eventLog) canceledEvents() []social.PostScheduleCanceled {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]social.PostScheduleCanceled(nil), e.canceled...)
}

type fixture struct {
	db        *db.DB
	module    *social.Module
	deletion  *accountdeletion.Module
	sellerMod *seller.Module
	buyers    *buyer.Module
	sellers   *seller.Reader
	bus       *events.Bus
	jobs      *queue
	media     *media.Module
	posts     *autoMedia
	products  *products
	deleg     *scheduleAuth
	events    *eventLog

	mu   sync.Mutex
	now  time.Time
	ids  atomic.Int64
	logs syncBuffer
}

type pendingStoreOrders struct{}

func (pendingStoreOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (pendingStoreOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return true, nil }

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
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
	))
	f := &fixture{
		db: database, bus: events.NewBus(), jobs: &queue{},
		products: &products{owner: map[uuid.UUID]int64{}}, deleg: &scheduleAuth{}, events: &eventLog{},
		now: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
	logger := slog.New(slog.NewTextHandler(&f.logs, nil))
	ring, err := media.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	f.media = media.New(media.Deps{DB: database, Jobs: f.jobs, Store: media.NewMemoryStore(), Keyring: ring, NewID: uuid.NewV7, Now: f.clock, Logger: logger})
	f.posts = &autoMedia{Module: f.media, tokens: map[string]string{}}
	f.buyers = buyer.New(buyer.Deps{DB: database, Events: f.bus, Media: f.media, Now: f.clock, Logger: logger})
	f.sellers = seller.NewReader(database, f.clock)
	f.sellerMod, err = sellerfixture.NewModuleWithEvents(database, f.bus, f.clock)
	if err != nil {
		t.Fatal(err)
	}
	f.module = social.New(social.Deps{
		DB: database, Events: f.bus, Media: f.posts, Jobs: f.jobs, Buyers: f.buyers, Stores: f.sellers, StoreBlocks: storeBlocksFromTable{database}, Products: f.products,
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: f.deleg},
		NewID:               uuid.NewV7, Now: f.clock, Logger: logger,
	})
	events.Subscribe(f.bus, func(_ context.Context, e social.PostPublished) error {
		f.events.mu.Lock()
		defer f.events.mu.Unlock()
		f.events.published = append(f.events.published, e)
		return nil
	})
	f.deletion = accountdeletion.New(accountdeletion.Deps{
		DB: database, Identity: identity.New(identity.Deps{DB: database, Events: f.bus, Now: f.clock}),
		Buyer: f.buyers, Seller: f.sellerMod, Orders: pendingStoreOrders{}, Now: f.clock,
	})
	events.Subscribe(f.bus, func(_ context.Context, e social.PostScheduleCanceled) error {
		f.events.mu.Lock()
		defer f.events.mu.Unlock()
		f.events.canceled = append(f.events.canceled, e)
		return nil
	})
	events.Subscribe(f.bus, f.module.OnBuyerDeletionRequested)
	events.Subscribe(f.bus, f.module.OnBuyerDeleted)
	events.Subscribe(f.bus, f.module.OnStoreRetired)
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

func (f *fixture) exposableStore(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	ctx := context.Background()
	var err error
	if accountID, err = sellerfixture.NewAccount(ctx, f.db, f.clock()); err != nil {
		t.Fatal(err)
	}
	if storeID, err = sellerfixture.ExposableStore(ctx, f.db, accountID, f.clock()); err != nil {
		t.Fatal(err)
	}
	return storeID, accountID
}

func (f *fixture) setExposable(t *testing.T, storeID int64, ok bool) {
	t.Helper()
	if err := sellerfixture.SetPayoutCapability(context.Background(), f.db, storeID, f.clock(), ok); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) beginAccountDeletion(t *testing.T, accountID int64) {
	t.Helper()
	ctx := context.Background()
	if err := f.buyers.OnAccountCreated(ctx, identity.AccountCreated{AccountID: accountID, Name: "店家"}); err != nil {
		t.Fatal(err)
	}
	state, err := f.deletion.Request(ctx, accountdeletion.RequestInput{AccountID: accountID, Requester: accountdeletion.AccountOwner})
	if err != nil || state != accountdeletion.PendingOrders {
		t.Fatalf("刪帳申請 state=%s err=%v，預期等待未完成訂單", state, err)
	}
}

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

func (f *fixture) publishedPost(t *testing.T, storeID int64, body string, tags ...string) social.Post {
	t.Helper()
	p, err := f.module.CreatePost(context.Background(), f.store(t, storeID), social.CreatePostInput{
		Body: body, ImageToken: fmt.Sprintf("tok-%d", f.next()), Hashtags: tags, Publish: true,
	}, nil)
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

type storeBlocksFromTable struct{ db *db.DB }

func (s storeBlocksFromTable) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	var blocked bool
	err := s.db.Conn(ctx).QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM seller.store_blocks WHERE store_id = $1 AND buyer_id = $2)", storeID, buyerID).Scan(&blocked)
	return blocked, err
}

func ptr[T any](v T) *T { return &v }
