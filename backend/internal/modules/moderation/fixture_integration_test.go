//go:build integration

package moderation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdimage "image"
	"image/png"
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
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
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
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

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

func (*testQueue) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
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

type noProductVerifier struct{}

func (noProductVerifier) VerifyProduct(context.Context, uuid.UUID, int64) error {
	return social.ErrProductNotAvailable
}

type storeBlocksFromTable struct{ db *db.DB }

func (s storeBlocksFromTable) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	var blocked bool
	err := s.db.Conn(ctx).QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM seller.store_blocks WHERE store_id = $1 AND buyer_id = $2)", storeID, buyerID).Scan(&blocked)
	return blocked, err
}

type reviewOrders struct {
	mu   sync.Mutex
	byNo map[string]review.OrderForReview
}

func (o *reviewOrders) OrderForReview(_ context.Context, orderNo string) (review.OrderForReview, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	v, ok := o.byNo[orderNo]
	if !ok {
		return review.OrderForReview{}, review.ErrOrderNotFound
	}
	return v, nil
}

type chatOrders struct{}

func (chatOrders) OpenOrderBetween(context.Context, int64, int64) (bool, error) { return false, nil }

type chatStores struct{ *seller.Module }

func (s chatStores) Store(ctx context.Context, id int64) (seller.Store, error) {
	st, err := s.Module.Store(ctx, id)
	if err != nil {
		return seller.Store{}, err
	}
	st.Status, st.Sanctions = seller.Active, seller.NoSanctions()
	st.Conditions = seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true}
	return st, nil
}

type fixture struct {
	review    *review.Module
	im        *im.Module
	orders    *reviewOrders
	db        *db.DB
	identity  *identity.Module
	buyer     *buyer.Module
	seller    *seller.Module
	sellerRd  *seller.Reader
	social    *social.Module
	deletion  *accountdeletion.Module
	suspend   *accountsuspension.Module
	module    *Module
	media     *media.Module
	chatAsset uuid.UUID
	objects   *media.MemoryStore
	jobs      *testQueue
	bus       *events.Bus
	ids       atomic.Int64
	mu        sync.Mutex
	now       time.Time
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
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
		migrate.Module{Name: "review", Dir: review.Migrations()},
		migrate.Module{Name: "im", Dir: im.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: "moderation", Dir: Migrations()},
	))
	f := &fixture{db: d, now: time.Date(2026, 11, 2, 4, 0, 0, 0, time.UTC)}
	bus := events.NewBus()
	f.bus = bus
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := &testQueue{}
	f.jobs = q
	f.objects = media.NewMemoryStore()
	fm := media.New(media.Deps{DB: d, Jobs: q, Store: f.objects, NewID: uuid.NewV7, Now: f.clock, Logger: log})
	f.media = fm
	owner := ownerMedia{m: fm}
	f.identity = identity.New(identity.Deps{DB: d, Events: bus, Now: f.clock, Logger: log})
	f.buyer = buyer.New(buyer.Deps{DB: d, Events: bus, Media: owner, Now: f.clock, Logger: log})
	f.seller = seller.New(seller.Deps{KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: f.identity, Media: owner, Orders: noOrders{}, ProductStores: noProducts{}, Registry: noRegistry{}, Now: f.clock, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, f.clock), Events: bus, Jobs: q, Now: f.clock, Logger: log})})
	f.sellerRd = seller.NewReader(d, f.clock)
	f.social = social.New(social.Deps{
		DB: d, Events: bus, Media: owner, Jobs: q, Buyers: f.buyer, Stores: f.sellerRd, StoreBlocks: storeBlocksFromTable{d}, Products: noProductVerifier{},
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: social.ScheduleAuthorizerFunc(func(context.Context, social.ScheduledPublish) error { return social.ErrScheduleDenied })},
		NewID:               uuid.NewV7, Now: f.clock, Logger: log,
	})
	f.deletion = accountdeletion.New(accountdeletion.Deps{DB: d, Identity: f.identity, Buyer: f.buyer, Seller: f.seller, Orders: noOrders{}, Now: f.clock})
	f.suspend = accountsuspension.New(accountsuspension.Deps{DB: d, Events: bus, Identity: f.identity, Seller: f.seller, Logger: log})
	f.orders = &reviewOrders{byNo: map[string]review.OrderForReview{}}
	f.review = review.New(review.Deps{DB: d, Events: bus, Media: owner, Jobs: q, Buyers: f.buyer, Stores: f.sellerRd, Orders: f.orders, Catalog: ratingSink{}, NewID: uuid.NewV7, Now: f.clock, Logger: log})
	f.im = im.New(im.Deps{DB: d, Events: bus, Jobs: q, Media: owner, Buyers: f.buyer, Stores: chatStores{f.seller}, Orders: chatOrders{}, NewID: uuid.NewV7, Now: f.clock, Logger: log})
	f.module = New(Deps{DB: d, Events: bus, Social: f.social, Review: f.review, IM: f.im, Seller: f.seller, Buyer: f.buyer, Suspension: f.suspend, Media: fm, NewID: uuid.NewV7, Now: f.clock, Logger: log})
	events.Subscribe(bus, f.buyer.OnAccountCreated)
	events.Subscribe(bus, f.social.OnBuyerDeletionRequested)
	events.Subscribe(bus, f.social.OnBuyerDeleted)
	events.Subscribe(bus, f.social.OnStoreRetired)
	events.Subscribe(bus, f.social.OnMeasureCreated)
	events.Subscribe(bus, f.social.OnMeasureRevoked)
	events.Subscribe(bus, f.im.OnMeasureCreated)
	events.Subscribe(bus, f.im.OnMeasureRevoked)
	events.Subscribe(bus, f.review.OnMeasureCreated)
	events.Subscribe(bus, f.review.OnMeasureRevoked)
	events.Subscribe(bus, f.module.Retention().OnBuyerDeletionRequested)
	events.Subscribe(bus, f.module.Retention().OnBuyerDeleted)
	events.Subscribe(bus, f.module.Retention().OnStoreRetired)
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

func (f *fixture) exposableStore(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	return f.exposableStoreWithNewAccount(t)
}

func (f *fixture) exposableStoreOnIdentityAccount(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	return f.exposableStoreWithNewAccount(t)
}

func (f *fixture) exposableStoreWithNewAccount(t *testing.T) (storeID, accountID int64) {
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

func (f *fixture) post(t *testing.T, storeID int64, body string) social.Post {
	t.Helper()
	st, err := f.sellerRd.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.social.CreatePost(context.Background(), st, social.CreatePostInput{
		Body: body, ImageToken: f.stage(t), Publish: true,
	}, nil)
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

var stagedUploader = media.Uploader{ID: 1}

type ownerMedia struct{ m *media.Module }

func (o ownerMedia) Confirm(ctx context.Context, _ media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	return o.m.Confirm(ctx, stagedUploader, token, want)
}

func (o ownerMedia) MarkDeleted(ctx context.Context, ids []uuid.UUID) error {
	return o.m.MarkDeleted(ctx, ids)
}

func (o ownerMedia) ReadSecure(ctx context.Context, id uuid.UUID) (io.ReadCloser, string, error) {
	return o.m.ReadSecure(ctx, id)
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func (f *fixture) stage(t *testing.T) string {
	t.Helper()
	res, err := f.media.PutPublic(context.Background(), stagedUploader, bytes.NewReader(tinyPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	return res.Token
}

func (f *fixture) assetState(t *testing.T, id uuid.UUID) string {
	t.Helper()
	st, err := media.AssetStateOf(context.Background(), f.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(st)
}

type ratingSink struct{}

func (ratingSink) LockProduct(context.Context, uuid.UUID) (catalog.Rating, error) {
	return catalog.Rating{}, nil
}

func (ratingSink) SetRating(context.Context, uuid.UUID, float64, int32) error { return nil }

func (f *fixture) stagedAssets(t *testing.T) int {
	t.Helper()
	n, err := media.CountAssetsInState(context.Background(), f.db, media.AssetStaged)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
