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

// storeBlocksFromTable 是 social 的店家封鎖檢查在測試裡的替身（seller 自己的封鎖規則由 seller 的測試驗證）。
type storeBlocksFromTable struct{ db *db.DB }

func (s storeBlocksFromTable) StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error) {
	var blocked bool
	err := s.db.Conn(ctx).QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM seller.store_blocks WHERE store_id = $1 AND buyer_id = $2)", storeID, buyerID).Scan(&blocked)
	return blocked, err
}

// reviewOrders 是 review 的訂單 port 的替身：以訂單號查。
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

// chatOrders 是 im 的訂單 port 的替身：雙方之間沒有進行中的訂單。
type chatOrders struct{}

func (chatOrders) OpenOrderBetween(context.Context, int64, int64) (bool, error) { return false, nil }

// chatStores 是 im 的 Stores port：守門的鎖與封鎖檢查走真正的 seller 模組，店家資料的條件在這裡固定成營運中（seller 自己的條件推導由 seller 的測試驗證）。
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
	aggs      atomic.Int64
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
	f.review = review.New(review.Deps{DB: d, Events: bus, Media: owner, Jobs: q, Buyers: f.buyer, Stores: f.sellerRd, Orders: f.orders, Catalog: ratingSink{aggs: &f.aggs}, NewID: uuid.NewV7, Now: f.clock, Logger: log})
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

// exposableStore 建一家可以公開曝光的店（直接寫 seller 的表，最小的列），回傳 store_id 與帳號。帳號是不存在於 identity 的編號。
func (f *fixture) exposableStore(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	return f.exposableStoreOn(t, f.next())
}

// exposableStoreOnIdentityAccount 同 exposableStore，但帳號是真的 identity 帳號（沒有買家列），讓 CloseCurrent 這類要鎖帳號列的路徑走得通。
func (f *fixture) exposableStoreOnIdentityAccount(t *testing.T) (storeID, accountID int64) {
	t.Helper()
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	return f.exposableStoreOn(t, accountID)
}

func (f *fixture) exposableStoreOn(t *testing.T, accountID int64) (storeID, _ int64) {
	t.Helper()
	f.exec(t, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $2, '測試店', 'logo/k', uuidv7(), '63000010', $3, $3)`, accountID, accountID, f.clock())
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO seller.store_exposures (store_id) VALUES ($1)`, storeID)
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

// stagedUploader 是測試裡所有暫存圖片的上傳者：擁有媒體的模組（social、review、im、seller）換了各自的擁有者 ID 呼叫 Confirm，
// ownerMedia 一律以這個上傳者確認。
var stagedUploader = media.Uploader{ID: 1}

// ownerMedia 是擁有媒體的模組的 media 替身：確認真的 media 模組裡由 stage 建立的暫存，上傳者固定是 stagedUploader。
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

// stage 在真的 media 模組建立一筆就緒的暫存圖片並回 token。
func (f *fixture) stage(t *testing.T) string {
	t.Helper()
	res, err := f.media.PutPublic(context.Background(), stagedUploader, bytes.NewReader(tinyPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	return res.Token
}

// assetState 是資產在 media 的狀態；資產不存在時失敗。
func (f *fixture) assetState(t *testing.T, id uuid.UUID) string {
	t.Helper()
	st, err := media.AssetStateOf(context.Background(), f.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(st)
}

type ratingSink struct{ aggs *atomic.Int64 }

func (s ratingSink) SetRating(context.Context, uuid.UUID, float64, int32) error {
	s.aggs.Add(1)
	return nil
}

// stagedAssets 是推導後處於 STAGED（還沒確認）的資產數。
func (f *fixture) stagedAssets(t *testing.T) int {
	t.Helper()
	n, err := media.CountAssetsInState(context.Background(), f.db, media.AssetStaged)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
