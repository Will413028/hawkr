//go:build integration

package notification

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// recorder 記下排入的 job，由測試決定何時展開（等同 River 取件）；渠道 job 另外記，不展開（這一步它什麼都不做）。
type recorder struct {
	mu         sync.Mutex
	args       []intake.ProcessArgs
	deliveries []intake.DeliverArgs
	purges     []purge.RecipientArgs
	// deliverUnique 記錄每個渠道 job 是否以參數（投遞紀錄 ID）唯一。
	deliverUnique []bool
}

func (r *recorder) Enqueue(_ context.Context, a river.JobArgs, opts *river.InsertOpts) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch a := a.(type) {
	case intake.ProcessArgs:
		r.args = append(r.args, a)
	case purge.RecipientArgs:
		r.purges = append(r.purges, a)
	case intake.DeliverArgs:
		r.deliveries = append(r.deliveries, a)
		r.deliverUnique = append(r.deliverUnique, opts != nil && opts.UniqueOpts.ByArgs)
	default:
		panic("unknown job args")
	}
	return nil
}

func (r *recorder) deliverJobs() ([]intake.DeliverArgs, []bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]intake.DeliverArgs(nil), r.deliveries...), append([]bool(nil), r.deliverUnique...)
}

func (r *recorder) takePurges() []purge.RecipientArgs {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.purges
	r.purges = nil
	return out
}

func (r *recorder) take() []intake.ProcessArgs {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.args
	r.args = nil
	return out
}

// 測試專用的型別：真正的兩個型別都不可聚合、也沒有 actor。
type likeParams struct {
	AccountID int64  `json:"account_id"`
	EventID   string `json:"event_id"`
	PostID    string `json:"post_id"`
	ActorID   int64  `json:"actor_id"`
	Note      string `json:"note"`
	// At 是事件發生時間；零值時用 epoch。
	At time.Time `json:"at"`
}

type pushParams struct {
	AccountID int64  `json:"account_id"`
	EventID   string `json:"event_id"`
}

var testPush = registry.Define(registry.Definition[pushParams]{
	Name: "test_push", Audience: model.AudienceBuyer, Category: model.CategoryLike, Presentation: model.PresentationSocial,
	Channels:          []model.Channel{model.ChannelInbox, model.ChannelPush, model.ChannelEmail},
	IdempotencySource: "test.Push.EventID",
	IdempotencyKey:    func(p pushParams) string { return p.EventID },
	OccurredAt:        func(pushParams) time.Time { return epoch },
	Recipient:         func(p pushParams) registry.Recipient { return registry.Recipient{AccountID: p.AccountID} },
	Text: func(l uitext.Locale, _ pushParams) (registry.Content, error) {
		return registry.Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
	},
	Example: pushParams{AccountID: 1, EventID: "e"},
})

var testLike = registry.Define(registry.Definition[likeParams]{
	Name: "test_like", Audience: model.AudienceBuyer, Category: model.CategoryLike, Presentation: model.PresentationSocial,
	Channels:          []model.Channel{model.ChannelInbox},
	IdempotencySource: "test.Like.EventID",
	IdempotencyKey:    func(p likeParams) string { return p.EventID },
	OccurredAt: func(p likeParams) time.Time {
		if p.At.IsZero() {
			return epoch
		}
		return p.At
	},
	Recipient:      func(p likeParams) registry.Recipient { return registry.Recipient{AccountID: p.AccountID} },
	Aggregatable:   true,
	AggregationKey: func(p likeParams) string { return "post:" + p.PostID },
	Actors:         func(p likeParams) []int64 { return []int64{p.ActorID} },
	Text: func(l uitext.Locale, _ likeParams) (registry.Content, error) {
		return registry.Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
	},
	Example: likeParams{AccountID: 1, EventID: "e", PostID: "p", ActorID: 2},
})

// testTwin 與 testLike 的原始聚合鍵相同，驗證聚合鍵以型別名稱為前綴。
var testTwin = registry.Define(registry.Definition[likeParams]{
	Name: "test_agg_twin", Audience: model.AudienceBuyer, Category: model.CategoryLike, Presentation: model.PresentationSocial,
	Channels:          []model.Channel{model.ChannelInbox},
	IdempotencySource: "test.Twin.EventID",
	IdempotencyKey:    func(p likeParams) string { return p.EventID },
	OccurredAt:        func(likeParams) time.Time { return epoch },
	Recipient:         func(p likeParams) registry.Recipient { return registry.Recipient{AccountID: p.AccountID} },
	Aggregatable:      true,
	AggregationKey:    func(p likeParams) string { return "post:" + p.PostID },
	Actors:            func(p likeParams) []int64 { return []int64{p.ActorID} },
	Text: func(l uitext.Locale, _ likeParams) (registry.Content, error) {
		return registry.Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
	},
	Example: likeParams{AccountID: 1, EventID: "e", PostID: "p", ActorID: 2},
})

type storeNoteParams struct {
	AccountID int64  `json:"account_id"`
	StoreID   int64  `json:"store_id"`
	EventID   string `json:"event_id"`
}

// testStoreNote 是 STORE 受眾、有收件匣與推播的測試型別；testStorePushOnly 只有推播（意圖只經投遞紀錄找得到）。
var testStoreNote = registry.Define(registry.Definition[storeNoteParams]{
	Name: "test_store_note", Audience: model.AudienceStore, Category: model.CategoryOrder, Presentation: model.PresentationOrder,
	Channels:          []model.Channel{model.ChannelInbox, model.ChannelPush},
	IdempotencySource: "test.StoreNote.EventID",
	IdempotencyKey:    func(p storeNoteParams) string { return p.EventID },
	OccurredAt:        func(storeNoteParams) time.Time { return epoch },
	Recipient: func(p storeNoteParams) registry.Recipient {
		return registry.Recipient{AccountID: p.AccountID, StoreID: p.StoreID}
	},
	Text: func(l uitext.Locale, _ storeNoteParams) (registry.Content, error) {
		return registry.Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
	},
	Example: storeNoteParams{AccountID: 1, StoreID: 1, EventID: "e"},
})

var testStorePushOnly = registry.Define(registry.Definition[storeNoteParams]{
	Name: "test_store_push_only", Audience: model.AudienceStore, Category: model.CategoryOrder, Presentation: model.PresentationOrder,
	Channels:          []model.Channel{model.ChannelPush},
	IdempotencySource: "test.StorePushOnly.EventID",
	IdempotencyKey:    func(p storeNoteParams) string { return p.EventID },
	OccurredAt:        func(storeNoteParams) time.Time { return epoch },
	Recipient: func(p storeNoteParams) registry.Recipient {
		return registry.Recipient{AccountID: p.AccountID, StoreID: p.StoreID}
	},
	Text: func(l uitext.Locale, _ storeNoteParams) (registry.Content, error) {
		return registry.Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
	},
	Example: storeNoteParams{AccountID: 1, StoreID: 1, EventID: "e"},
})

type testRegistry struct{}

func (testRegistry) Lookup(name string) (registry.Entry, bool) {
	switch name {
	case "test_like":
		return testLike, true
	case "test_push":
		return testPush, true
	case "test_agg_twin":
		return testTwin, true
	case "test_store_note":
		return testStoreNote, true
	case "test_store_push_only":
		return testStorePushOnly, true
	}
	return registry.Lookup(name)
}

// fakeStores 取代 seller：測試只需要「店家是否已退場」。沒登記的店視為存在、營運中。
type fakeStores struct {
	mu      sync.Mutex
	retired map[int64]bool
}

func (f *fakeStores) LockNotRetired(ctx context.Context, storeID int64) error {
	if _, ok := db.TxFrom(ctx); !ok {
		panic("LockNotRetired 必須在 transaction 內呼叫")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if storeID <= 0 || f.retired[storeID] {
		return apperr.NotFound("店家已退場")
	}
	return nil
}

func (f *fakeStores) retire(storeID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retired[storeID] = true
}

// nopJobs 接受 buyer 排的 job（測試不展開它們）。
type nopJobs struct{}

func (nopJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

type fixture struct {
	db       *db.DB
	identity *identity.Module
	buyer    *buyer.Module
	stores   *fakeStores
	bus      *events.Bus
	jobs     *recorder
	module   *Module
	logs     *syncBuffer
}

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

func setupIntake(t *testing.T) *fixture {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "notification", Dir: Migrations()},
	))
	logs := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: nopJobs{}, Media: media.NewFake(), Now: time.Now, Logger: log})
	events.Subscribe[identity.AccountCreated](bus, buyers.OnAccountCreated)
	stores := &fakeStores{retired: map[int64]bool{}}
	rec := &recorder{}
	m := newModule(Deps{DB: d, Identity: id, Buyer: buyers, Seller: stores, Jobs: rec, Now: time.Now, NewID: uuid.NewV7, Logger: log}, testRegistry{})
	events.Subscribe[accountsuspension.Suspended](bus, m.OnSuspended)
	events.Subscribe[accountsuspension.Unsuspended](bus, m.OnUnsuspended)
	events.Subscribe[identity.AccountDeleted](bus, m.OnAccountDeleted)
	events.Subscribe[buyer.BuyerDeleted](bus, m.OnBuyerDeleted)
	events.Subscribe[seller.StoreRetired](bus, m.OnStoreRetired)
	return &fixture{db: d, identity: id, buyer: buyers, stores: stores, bus: bus, jobs: rec, module: m, logs: logs}
}

func (f *fixture) newAccount(t *testing.T) int64 {
	t.Helper()
	id, err := f.identity.DevNewAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) publish(t *testing.T, e any) {
	t.Helper()
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		switch e := e.(type) {
		case accountsuspension.Suspended:
			return events.Publish(ctx, f.bus, e)
		case accountsuspension.Unsuspended:
			return events.Publish(ctx, f.bus, e)
		}
		panic("unknown event")
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) accept(t *testing.T, typ string, p any) {
	t.Helper()
	err := f.db.InTx(context.Background(), func(ctx context.Context) error { return f.module.intake.Accept(ctx, typ, p) })
	if err != nil {
		t.Fatal(err)
	}
}

// run 展開目前排著的全部 job 並回傳 job 數。
func (f *fixture) run(t *testing.T) int {
	t.Helper()
	jobs := f.jobs.take()
	for _, j := range jobs {
		if err := f.module.intake.Process(context.Background(), j.IntentID); err != nil {
			t.Fatal(err)
		}
	}
	return len(jobs)
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) inboxCount(t *testing.T, account int64) int {
	return f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1`, account)
}

func suspended(account int64, id uuid.UUID) accountsuspension.Suspended {
	return accountsuspension.Suspended{AccountID: account, SuspensionID: id, Category: policy.Spam, SuspendedAt: epoch}
}

func TestSuspendedEventCreatesTheInboxRow(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	sid := uuid.NewV7()
	f.publish(t, suspended(acct, sid))
	if got := f.inboxCount(t, acct); got != 0 {
		t.Fatalf("訂閱者只排意圖，不寫收件匣；現在有 %d 列", got)
	}
	if n := f.run(t); n != 1 {
		t.Fatalf("排入 %d 個 job，預期 1", n)
	}
	var typ, aud string
	var storeID *int64
	var params string
	var readAt *time.Time
	err := f.db.Pool().QueryRow(context.Background(),
		`SELECT type, audience, store_id, params::text, read_at FROM notification.inbox_items WHERE account_id = $1`, acct).
		Scan(&typ, &aud, &storeID, &params, &readAt)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "account_suspended" || aud != "BUYER" || storeID != nil || readAt != nil {
		t.Fatalf("收件匣列不對：%s %s %v %v", typ, aud, storeID, readAt)
	}
	e, _ := registry.Lookup(typ)
	r, err := e.Render(uitext.ZhTW, []byte(params))
	if err != nil || r.Title != "帳號已被停權" {
		t.Fatalf("參數快照組不出停權通知：%v %+v", err, r)
	}
	// email 渠道是 PENDING 的投遞紀錄加一個以投遞紀錄 ID 唯一的渠道 job；job 這一步不送出，紀錄留在 PENDING。
	jobs, unique := f.jobs.deliverJobs()
	var deliveryID uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.deliveries WHERE account_id = $1 AND channel = 'EMAIL'`, acct).Scan(&deliveryID); err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].DeliveryID != deliveryID || !unique[0] {
		t.Fatalf("渠道 job = %+v（唯一 %v），預期只有投遞紀錄 %s 一個、以參數唯一", jobs, unique, deliveryID)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND channel = 'EMAIL' AND status = 'PENDING'`, acct); n != 1 {
		t.Fatalf("email 投遞紀錄 %d 列，預期 1", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND channel = 'PUSH'`, acct); n != 0 {
		t.Fatalf("停權型別沒有推播渠道，卻有 %d 列", n)
	}
}

func TestUnsuspendedEventCreatesTheInboxRow(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.publish(t, accountsuspension.Unsuspended{AccountID: acct, SuspensionID: uuid.NewV7(), Category: policy.Spam, ReleasedAt: epoch})
	f.run(t)
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = 'account_unsuspended'`, acct); n != 1 {
		t.Fatalf("解除停權的收件匣列 %d，預期 1", n)
	}
}

func TestSameEventTwiceMakesOneIntentAndOneInboxRow(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	e := suspended(acct, uuid.NewV7())
	f.publish(t, e)
	f.publish(t, e)
	if n := f.count(t, `SELECT count(*) FROM notification.intents`); n != 1 {
		t.Fatalf("意圖 %d 個，預期 1", n)
	}
	if n := f.run(t); n != 1 {
		t.Fatalf("job %d 個，預期 1（重複事件不再排）", n)
	}
	// job 重試（at-least-once）：同一個意圖再展開一次，收件匣與投遞紀錄都不增加。
	var intentID uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.intents`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if err := f.module.intake.Process(context.Background(), intentID); err != nil {
		t.Fatal(err)
	}
	if got := f.inboxCount(t, acct); got != 1 {
		t.Fatalf("收件匣 %d 列，預期 1", got)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1`, acct); n != 1 {
		t.Fatalf("投遞紀錄 %d 列，預期 1", n)
	}
}

func TestPreferencesDisableOnlyDisableableCategories(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	// 互動分類 LIKE 的收件匣可關。
	f.mustExec(t, `INSERT INTO notification.preference_overrides (account_id, audience, category, channel, enabled, updated_at) VALUES ($1, 'BUYER', 'LIKE', 'INBOX', false, $2)`, acct, epoch)
	// 交易必要的 ACCOUNT 即使有覆寫列也不可關。
	f.mustExec(t, `INSERT INTO notification.preference_overrides (account_id, audience, category, channel, enabled, updated_at) VALUES ($1, 'BUYER', 'ACCOUNT', 'INBOX', false, $2)`, acct, epoch)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "like-1", PostID: "p1", ActorID: acct + 100})
	f.publish(t, suspended(acct, uuid.NewV7()))
	f.run(t)
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = 'test_like'`, acct); n != 0 {
		t.Fatalf("關掉的互動分類仍寫了 %d 列收件匣", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = 'account_suspended'`, acct); n != 1 {
		t.Fatalf("不可關的分類收件匣 %d 列，預期 1", n)
	}
}

func (f *fixture) mustExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.db.Pool().Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestSelfNotificationIsSkipped(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "self", PostID: "p1", ActorID: acct})
	f.run(t)
	if got := f.inboxCount(t, acct); got != 0 {
		t.Fatalf("自己讚自己仍寫了 %d 列", got)
	}
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "other", PostID: "p1", ActorID: other})
	f.run(t)
	if got := f.inboxCount(t, acct); got != 1 {
		t.Fatalf("別人的讚收件匣 %d 列，預期 1", got)
	}
}

func TestUnreadNotificationsWithTheSameAggregationKeyMerge(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	a, b, c := f.newAccount(t), f.newAccount(t), f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "l1", PostID: "p1", ActorID: a})
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "l2", PostID: "p1", ActorID: b})
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "l3", PostID: "p2", ActorID: a})
	f.run(t)
	if got := f.inboxCount(t, acct); got != 2 {
		t.Fatalf("p1 兩個讚合併、p2 另一列，共 2 列；現在 %d", got)
	}
	var actors []int64
	if err := f.db.Pool().QueryRow(context.Background(),
		`SELECT actor_account_ids FROM notification.inbox_items WHERE account_id = $1 AND aggregation_key = 'test_like:post:p1'`, acct).Scan(&actors); err != nil {
		t.Fatal(err)
	}
	if len(actors) != 2 || actors[0] != min(a, b) || actors[1] != max(a, b) {
		t.Fatalf("合併後的 actor 是 %v，預期 [%d %d]", actors, a, b)
	}
	// 已讀之後的新意圖另開一列。
	f.mustExec(t, `UPDATE notification.inbox_items SET read_at = $2 WHERE account_id = $1 AND aggregation_key = 'test_like:post:p1'`, acct, epoch)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "l4", PostID: "p1", ActorID: c})
	f.run(t)
	if got := f.inboxCount(t, acct); got != 3 {
		t.Fatalf("已讀之後的新讚應另開一列，共 3 列；現在 %d", got)
	}
}

func TestIntentArrivingAfterAccountDeletionWritesNothing(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.publish(t, suspended(acct, uuid.NewV7()))
	if err := f.identity.DeleteAccount(context.Background(), acct); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	if got := f.inboxCount(t, acct); got != 0 {
		t.Fatalf("已刪除的帳號仍寫了 %d 列收件匣", got)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1`, acct); n != 0 {
		t.Fatalf("已刪除的帳號仍寫了 %d 列投遞紀錄", n)
	}
}

func TestDeliveryChannelsEachGetARecordAndAJobInTheSameTransaction(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.accept(t, "test_push", pushParams{AccountID: acct, EventID: "p1"})
	f.run(t)
	if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND status = 'PENDING'`, acct); n != 2 {
		t.Fatalf("推播與 email 各一筆 PENDING 投遞紀錄，現在 %d", n)
	}
	if n := f.inboxCount(t, acct); n != 1 {
		t.Fatalf("收件匣 %d 列，預期 1", n)
	}
	jobs, unique := f.jobs.deliverJobs()
	if len(jobs) != 2 || !unique[0] || !unique[1] || jobs[0].DeliveryID == jobs[1].DeliveryID {
		t.Fatalf("渠道 job = %+v（唯一 %v），預期兩筆紀錄各一個", jobs, unique)
	}
	for _, j := range jobs {
		if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE id = $1 AND status = 'PENDING' AND channel IN ('PUSH', 'EMAIL')`, j.DeliveryID); n != 1 {
			t.Fatalf("渠道 job %s 沒有對應的 PENDING 投遞紀錄", j.DeliveryID)
		}
	}
	// 重試：紀錄已存在，不再排渠道 job。
	var intentID uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.intents`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if err := f.module.intake.Process(context.Background(), intentID); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.jobs.deliverJobs(); len(again) != 2 {
		t.Fatalf("重試後渠道 job %d 個，預期仍是 2", len(again))
	}
}

// worker 重試時，意圖若合併進過既有列、那一列之後被讀過，重試不能再建一列：判斷靠意圖與收件匣列的連結。
func TestRetryOfAMergedIntentDoesNotCreateARowAfterTheRowWasRead(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	a, b := f.newAccount(t), f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "A", PostID: "p1", ActorID: a})
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "B", PostID: "p1", ActorID: b})
	f.run(t)
	if got := f.inboxCount(t, acct); got != 1 {
		t.Fatalf("A 寫入、B 合併，應該 1 列；現在 %d", got)
	}
	f.mustExec(t, `UPDATE notification.inbox_items SET read_at = $2 WHERE account_id = $1`, acct, epoch)
	var intentB uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.intents WHERE idempotency_key = 'test_like:B'`).Scan(&intentB); err != nil {
		t.Fatal(err)
	}
	if err := f.module.intake.Process(context.Background(), intentB); err != nil {
		t.Fatal(err)
	}
	if got := f.inboxCount(t, acct); got != 1 {
		t.Fatalf("重試 B 後收件匣 %d 列，預期仍是 1", got)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_item_intents`); n != 2 {
		t.Fatalf("兩個意圖都該有連結，現在 %d", n)
	}
}

func (f *fixture) inboxNote(t *testing.T, acct int64) (note string, occurredAt time.Time) {
	t.Helper()
	if err := f.db.Pool().QueryRow(context.Background(),
		`SELECT params->>'note', occurred_at FROM notification.inbox_items WHERE account_id = $1`, acct).Scan(&note, &occurredAt); err != nil {
		t.Fatal(err)
	}
	return note, occurredAt
}

// 合併時以事件時間決定參數誰比較新，不看 job 的處理順序；actor 兩邊都收。
func TestMergeKeepsTheParamsOfTheNewerEventRegardlessOfProcessingOrder(t *testing.T) {
	older, newer := epoch.Add(time.Hour), epoch.Add(2*time.Hour)
	for name, order := range map[string][]string{"newer first": {"new", "old"}, "older first": {"old", "new"}} {
		t.Run(name, func(t *testing.T) {
			f := setupIntake(t)
			acct := f.newAccount(t)
			a, b := f.newAccount(t), f.newAccount(t)
			events := map[string]likeParams{
				"new": {AccountID: acct, EventID: "new", PostID: "p1", ActorID: a, Note: "newer", At: newer},
				"old": {AccountID: acct, EventID: "old", PostID: "p1", ActorID: b, Note: "older", At: older},
			}
			for _, k := range order {
				f.accept(t, "test_like", events[k])
			}
			f.run(t)
			note, at := f.inboxNote(t, acct)
			if note != "newer" || !at.Equal(newer) {
				t.Fatalf("參數 = %q、事件時間 %v，預期 newer、%v", note, at, newer)
			}
			var actors []int64
			if err := f.db.Pool().QueryRow(context.Background(), `SELECT actor_account_ids FROM notification.inbox_items WHERE account_id = $1`, acct).Scan(&actors); err != nil {
				t.Fatal(err)
			}
			if len(actors) != 2 {
				t.Fatalf("actor 要取聯集，現在 %v", actors)
			}
		})
	}
}

// 兩個型別的同名聚合鍵不會合併成同一列。
func TestSameRawAggregationKeyOfTwoTypesStaysTwoRows(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "l1", PostID: "p1", ActorID: other})
	f.accept(t, "test_agg_twin", likeParams{AccountID: acct, EventID: "t1", PostID: "p1", ActorID: other})
	f.run(t)
	if got := f.inboxCount(t, acct); got != 2 {
		t.Fatalf("不同型別、同名聚合鍵應各一列，現在 %d", got)
	}
}

func TestSkippedIntentsAreLogged(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "self", PostID: "p1", ActorID: acct})
	f.run(t)
	if !strings.Contains(f.logs.String(), "收件人就是 actor") || !strings.Contains(f.logs.String(), "type=test_like") || !strings.Contains(f.logs.String(), "intent_id=") {
		t.Fatalf("自我略過沒有記錄：%q", f.logs.String())
	}
	gone := f.newAccount(t)
	f.publish(t, suspended(gone, uuid.NewV7()))
	if err := f.identity.DeleteAccount(context.Background(), gone); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	if !strings.Contains(f.logs.String(), "收件帳號已刪除") {
		t.Fatalf("帳號已刪除的略過沒有記錄：%q", f.logs.String())
	}
}
