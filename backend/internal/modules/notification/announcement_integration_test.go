//go:build integration

package notification

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// jobQueue 依排入順序記下公告派送排的 job；測試以 drain 等同 River 取件。
type jobQueue struct {
	mu      sync.Mutex
	pending []river.JobArgs
	// enqueued 是曾經排入過的 job 種類計數（含已展開的）。
	enqueued map[string]int
}

func (q *jobQueue) Enqueue(_ context.Context, a river.JobArgs, _ *river.InsertOpts) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueued == nil {
		q.enqueued = map[string]int{}
	}
	q.enqueued[a.Kind()]++
	q.pending = append(q.pending, a)
	return nil
}

func (q *jobQueue) pop() (river.JobArgs, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return nil, false
	}
	a := q.pending[0]
	q.pending = q.pending[1:]
	return a, true
}

func (q *jobQueue) count(kind string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueued[kind]
}

// testDirectory 以真的 buyer 模組列買家、以固定名單列店家（seller 的表不在這支測試的 schema 裡）。
type testDirectory struct {
	buyers *buyer.Module
	stores []announce.Recipient
}

func (d testDirectory) Buyers(ctx context.Context, after int64, limit int) ([]announce.Recipient, error) {
	rows, err := d.buyers.ListLiveAccounts(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]announce.Recipient, len(rows))
	for i, r := range rows {
		out[i] = announce.Recipient{AccountID: r.AccountID, Cursor: r.BuyerID}
	}
	return out, nil
}

func (d testDirectory) Stores(_ context.Context, after int64, limit int) ([]announce.Recipient, error) {
	var out []announce.Recipient
	for _, s := range d.stores {
		if s.Cursor > after && len(out) < limit {
			out = append(out, s)
		}
	}
	return out, nil
}

type announceFixture struct {
	db       *db.DB
	identity *identity.Module
	svc      *announce.Service
	jobs     *jobQueue
	dir      *testDirectory
	stores   *fakeStores
}

func setupAnnounce(t *testing.T, chunk int) *announceFixture {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "notification", Dir: Migrations()},
	))
	log := slog.New(slog.DiscardHandler)
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: nopJobs{}, Media: media.NewFake(), Now: time.Now, Logger: log})
	events.Subscribe[identity.AccountCreated](bus, buyers.OnAccountCreated)
	stores := &fakeStores{retired: map[int64]bool{}}
	q := &jobQueue{}
	dir := &testDirectory{buyers: buyers}
	svc := announce.New(announce.Deps{
		DB: d, Locks: intake.Locks{Accounts: id, Buyers: buyers, Stores: stores}, Directory: dir,
		Audit: adminaudit.New(d, time.Now), Jobs: q, Now: time.Now, NewID: uuid.NewV7, Logger: log, Chunk: chunk,
	})
	return &announceFixture{db: d, identity: id, svc: svc, jobs: q, dir: dir, stores: stores}
}

func (f *announceFixture) newAccount(t *testing.T) int64 {
	t.Helper()
	id, err := f.identity.DevNewAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *announceFixture) create(t *testing.T, in announce.CreateInput) announce.Announcement {
	t.Helper()
	if in.Actor.AdminID == 0 {
		in.Actor = adminaudit.Actor{AdminID: 7, Action: "/hawkr.admin.notification.v1alpha1.NotificationAdminService/CreateAnnouncement"}
	}
	if in.Title == "" {
		in.Title, in.Body = "Scheduled maintenance", "The platform is down for maintenance tonight."
	}
	a, err := f.svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// drain 展開排著的 job 直到沒有，回傳展開的派送 job 數與排出的渠道 job 數。
func (f *announceFixture) drain(t *testing.T) (pages, deliveries int) {
	t.Helper()
	ctx := context.Background()
	for {
		j, ok := f.jobs.pop()
		if !ok {
			return pages, deliveries
		}
		switch a := j.(type) {
		case announce.InboxArgs:
			pages++
			if err := f.svc.ProcessInbox(ctx, a); err != nil {
				t.Fatal(err)
			}
		case announce.PushArgs:
			pages++
			if err := f.svc.ProcessPush(ctx, a); err != nil {
				t.Fatal(err)
			}
		case intake.DeliverArgs:
			deliveries++
		default:
			t.Fatalf("意外的 job %T", j)
		}
	}
}

func (f *announceFixture) scan(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *announceFixture) inboxRows(t *testing.T, announcement uuid.UUID) int {
	return f.scan(t, `SELECT count(*) FROM notification.inbox_items WHERE announcement_id = $1`, announcement)
}

func (f *announceFixture) inboxRowFor(t *testing.T, announcement uuid.UUID, account int64) int {
	return f.scan(t, `SELECT count(*) FROM notification.inbox_items WHERE announcement_id = $1 AND account_id = $2`, announcement, account)
}

func (f *announceFixture) pushRows(t *testing.T, announcement uuid.UUID) int {
	return f.scan(t, `SELECT count(*) FROM notification.deliveries WHERE intent_id = $1 AND channel = 'PUSH' AND status = 'PENDING'`, announcement)
}

func (f *announceFixture) reload(t *testing.T) announce.Announcement {
	t.Helper()
	page, err := f.svc.List(context.Background(), nil, 50)
	if err != nil || len(page.Announcements) == 0 {
		t.Fatalf("List = %+v, %v", page, err)
	}
	return page.Announcements[0]
}

// 沒有 device token 的帳號也收到收件匣那份（缺陷 5–7：舊系統只送有 token 的人）；兩個帳號都計入 inbox_count，派送完成後 dispatched 為真。
// mutation：收件匣加上「只寫給有 token 的帳號」→ 沒有 token 的帳號沒有收件匣列，失敗。
func TestAccountWithoutDeviceTokenStillGetsTheInboxItem(t *testing.T) {
	f := setupAnnounce(t, 0)
	withToken, without := f.newAccount(t), f.newAccount(t)
	mustExec(t, f.db, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ('tok', $1, 'BUYER', $2)`, withToken, epoch)

	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	if a.Dispatched || a.InboxCount != 0 {
		t.Fatalf("剛建立：%+v，派送還沒開始", a)
	}
	f.drain(t)

	if f.inboxRowFor(t, a.ID, withToken) != 1 || f.inboxRowFor(t, a.ID, without) != 1 {
		t.Fatalf("兩個帳號都要有收件匣列（有 token：%d，沒有 token：%d）", f.inboxRowFor(t, a.ID, withToken), f.inboxRowFor(t, a.ID, without))
	}
	got := f.reload(t)
	if got.InboxCount != 2 || !got.Dispatched {
		t.Errorf("派送完成後 inbox_count = %d、dispatched = %v，預期 2、true", got.InboxCount, got.Dispatched)
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.inbox_items WHERE announcement_id = $1 AND type = 'announcement' AND audience = 'BUYER' AND store_id IS NULL AND read_at IS NULL`, a.ID); n != 2 {
		t.Errorf("收件匣列的形狀不對：%d 列符合", n)
	}
}

// 推播與收件匣分開：每個收件人（不看 token）寫一筆 PENDING 的推播投遞紀錄並排渠道 job；關掉公告推播的人沒有推播紀錄，收件匣照樣有（D18）。
func TestPushRespectsTheAnnouncementPreferenceAndInboxDoesNot(t *testing.T) {
	f := setupAnnounce(t, 0)
	on, off := f.newAccount(t), f.newAccount(t)
	mustExec(t, f.db, `INSERT INTO notification.preference_overrides (account_id, audience, category, channel, enabled, updated_at) VALUES ($1, 'BUYER', 'ANNOUNCEMENT', 'PUSH', false, $2)`, off, epoch)

	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	_, deliveries := f.drain(t)

	if f.inboxRowFor(t, a.ID, on) != 1 || f.inboxRowFor(t, a.ID, off) != 1 {
		t.Error("兩個帳號都要有收件匣列（收件匣不可關）")
	}
	if f.pushRows(t, a.ID) != 1 || deliveries != 1 {
		t.Errorf("推播紀錄 %d 筆、渠道 job %d 個，預期各 1（關掉推播的帳號沒有）", f.pushRows(t, a.ID), deliveries)
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.deliveries WHERE intent_id = $1 AND account_id = $2`, a.ID, off); n != 0 {
		t.Errorf("關掉公告推播的帳號有 %d 筆投遞紀錄", n)
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.deliveries WHERE intent_id = $1 AND channel = 'EMAIL'`, a.ID); n != 0 {
		t.Errorf("公告不寄 email，卻有 %d 筆 email 紀錄", n)
	}
}

// 派送分頁、可重跑：一頁 2 人、5 個帳號要多個 job；把第一頁重跑一次（等同 job 失敗重試）不會多寫列、不會重複計數、不會重複排渠道 job。
func TestFanOutIsChunkedAndRerunningAPageIsIdempotent(t *testing.T) {
	f := setupAnnounce(t, 2)
	for range 5 {
		f.newAccount(t)
	}
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	first, ok := f.jobs.pop()
	if !ok {
		t.Fatal("建立時要排出第一個收件匣 job")
	}
	inbox, ok := first.(announce.InboxArgs)
	if !ok {
		t.Fatalf("第一個 job 是 %T", first)
	}
	f.jobs.mu.Lock()
	f.jobs.pending = append([]river.JobArgs{first}, f.jobs.pending...)
	f.jobs.mu.Unlock()

	if err := f.svc.ProcessInbox(context.Background(), inbox); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ProcessInbox(context.Background(), inbox); err != nil {
		t.Fatalf("同一頁重跑：%v", err)
	}
	if f.inboxRows(t, a.ID) != 2 || f.reload(t).InboxCount != 2 {
		t.Fatalf("重跑第一頁後：%d 列、inbox_count %d，預期各 2", f.inboxRows(t, a.ID), f.reload(t).InboxCount)
	}
	pages, deliveries := f.drain(t)
	if f.inboxRows(t, a.ID) != 5 || f.pushRows(t, a.ID) != 5 || deliveries != 5 || f.reload(t).InboxCount != 5 {
		t.Errorf("全部派送後：收件匣 %d 列、推播 %d 筆、渠道 job %d 個、inbox_count %d，預期各 5", f.inboxRows(t, a.ID), f.pushRows(t, a.ID), deliveries, f.reload(t).InboxCount)
	}
	if pages < 6 {
		t.Errorf("5 人、一頁 2 人，收件匣與推播各至少 3 頁加結尾，只展開 %d 個 job", pages)
	}
	if !f.reload(t).Dispatched {
		t.Error("全部處理完 dispatched 要是真")
	}
}

// 受眾逐個處理：BUYER 與 STORE 都選時，店家收件人帶 store_id，兩邊都完成才算派送完成。
func TestBothAudiencesAreDispatchedInTurn(t *testing.T) {
	f := setupAnnounce(t, 1)
	acct := f.newAccount(t)
	seller := f.newAccount(t)
	f.dir.stores = []announce.Recipient{{AccountID: seller, StoreID: 41, Cursor: 41}, {AccountID: acct, StoreID: 42, Cursor: 42}}

	a := f.create(t, announce.CreateInput{Audiences: []string{"STORE", "BUYER"}})
	if len(a.Audiences) != 2 || a.Audiences[0] != "BUYER" || a.Audiences[1] != "STORE" {
		t.Fatalf("audiences 要排序：%v", a.Audiences)
	}
	f.drain(t)
	if n := f.scan(t, `SELECT count(*) FROM notification.inbox_items WHERE announcement_id = $1 AND audience = 'BUYER' AND store_id IS NULL`, a.ID); n != 2 {
		t.Errorf("買家收件匣 %d 列，預期 2（兩個帳號都有買家）", n)
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.inbox_items WHERE announcement_id = $1 AND audience = 'STORE' AND store_id IN (41, 42)`, a.ID); n != 2 {
		t.Errorf("店家收件匣 %d 列，預期 2（store 41、42）", n)
	}
	if got := f.reload(t); got.InboxCount != 4 || !got.Dispatched {
		t.Errorf("inbox_count = %d、dispatched = %v，預期 4、true", got.InboxCount, got.Dispatched)
	}
	f.stores.retire(42)
	b := f.create(t, announce.CreateInput{Audiences: []string{"STORE"}})
	f.drain(t)
	if f.inboxRows(t, b.ID) != 1 {
		t.Errorf("已退場的店不寫收件匣：%d 列，預期 1", f.inboxRows(t, b.ID))
	}
}

// 同一個管理員以同一個 request_id 重送：回第一次的那一則、只有一則公告、只排一組 job、只寫一筆稽核；內容不同回 InvalidInput 指向 request_id。
// mutation：建立時不帶 request_id（拿掉冪等）→ 重送建出第二則公告，失敗。
func TestCreateWithTheSameRequestIDMakesOneAnnouncement(t *testing.T) {
	f := setupAnnounce(t, 0)
	rid := uuid.NewV4()
	in := announce.CreateInput{Audiences: []string{"BUYER"}, Title: "Hello", Body: "World", RequestID: &rid}

	first := f.create(t, in)
	second := f.create(t, in)
	if second.ID != first.ID {
		t.Fatalf("重送回 %s，預期第一次的 %s", second.ID, first.ID)
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.announcements`); n != 1 {
		t.Errorf("公告 %d 則，預期 1", n)
	}
	if f.jobs.count("notification.announcement_inbox") != 1 || f.jobs.count("notification.announcement_push") != 1 {
		t.Errorf("重送不該再排派送 job：inbox %d、push %d", f.jobs.count("notification.announcement_inbox"), f.jobs.count("notification.announcement_push"))
	}
	if n := f.scan(t, `SELECT count(*) FROM adminaudit.records WHERE target_type = 'announcement'`); n != 1 {
		t.Errorf("稽核紀錄 %d 筆，預期 1", n)
	}

	in.Body = "Different"
	_, err := f.svc.Create(context.Background(), in)
	var ae *apperr.Error
	if apperr.KindOf(err) != apperr.KindInvalidInput || !asAppErr(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "RequestID" {
		t.Errorf("內容不同的重送：%v，預期 InvalidInput（RequestID）", err)
	}

	other := uuid.NewV4()
	in.Body, in.RequestID = "World", &other
	if third := f.create(t, in); third.ID == first.ID {
		t.Error("不同的 request_id 是另一則公告")
	}
	// 沒有 request_id 不做冪等；另一個管理員用同一個 request_id 也是另一則。
	in.RequestID = nil
	f.create(t, in)
	in.RequestID = &rid
	in.Actor = adminaudit.Actor{AdminID: 8, Action: "x"}
	if other := f.create(t, in); other.ID == first.ID {
		t.Error("request_id 的範圍是管理員，另一個管理員的同一個 request_id 是另一則")
	}
}

func asAppErr(err error, out **apperr.Error) bool { return errors.As(err, out) }

// 欄位驗證：受眾不可空、不可重複、只能是 BUYER 或 STORE；標題與內文必填、去頭尾空白、有長度上限（以字計）。
func TestCreateValidatesAudiencesTitleAndBody(t *testing.T) {
	f := setupAnnounce(t, 0)
	long := func(n int) string {
		s := ""
		for range n {
			s += "字"
		}
		return s
	}
	cases := map[string]struct {
		in    announce.CreateInput
		field string
	}{
		"沒有受眾":   {announce.CreateInput{Title: "t", Body: "b"}, "Audiences"},
		"重複受眾":   {announce.CreateInput{Audiences: []string{"BUYER", "BUYER"}, Title: "t", Body: "b"}, "Audiences"},
		"不認得的受眾": {announce.CreateInput{Audiences: []string{"ADMIN"}, Title: "t", Body: "b"}, "Audiences"},
		"空白標題":   {announce.CreateInput{Audiences: []string{"BUYER"}, Title: " 　", Body: "b"}, "Title"},
		"標題過長":   {announce.CreateInput{Audiences: []string{"BUYER"}, Title: long(announce.MaxTitleRunes + 1), Body: "b"}, "Title"},
		"空內文":    {announce.CreateInput{Audiences: []string{"BUYER"}, Title: "t"}, "Body"},
		"內文過長":   {announce.CreateInput{Audiences: []string{"BUYER"}, Title: "t", Body: long(announce.MaxBodyRunes + 1)}, "Body"},
	}
	for name, c := range cases {
		c.in.Actor = adminaudit.Actor{AdminID: 7, Action: "x"}
		_, err := f.svc.Create(context.Background(), c.in)
		var ae *apperr.Error
		if !asAppErr(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != c.field {
			t.Errorf("%s：%v，預期 InvalidInput（%s）", name, err, c.field)
		}
	}
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}, Title: "  Title\t", Body: long(announce.MaxBodyRunes) + " "})
	if a.Title != "Title" {
		t.Errorf("標題要去頭尾空白：%q", a.Title)
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.announcements`); n != 1 {
		t.Errorf("驗證失敗不該留下公告：%d 則", n)
	}
}

// 列表：新到舊、keyset 分頁、page_size 預設與上限。
func TestListAnnouncementsPagesNewestFirst(t *testing.T) {
	f := setupAnnounce(t, 0)
	var ids []uuid.UUID
	for i := range 3 {
		a := f.create(t, announce.CreateInput{Audiences: []string{"STORE"}, Title: string(rune('A' + i)), Body: "b"})
		ids = append(ids, a.ID)
		time.Sleep(2 * time.Millisecond)
	}
	p1, err := f.svc.List(context.Background(), nil, 2)
	if err != nil || len(p1.Announcements) != 2 || p1.Next == nil {
		t.Fatalf("第一頁 = %+v, %v", p1, err)
	}
	if p1.Announcements[0].ID != ids[2] || p1.Announcements[1].ID != ids[1] {
		t.Errorf("要新到舊：%v", []uuid.UUID{p1.Announcements[0].ID, p1.Announcements[1].ID})
	}
	p2, err := f.svc.List(context.Background(), p1.Next, 2)
	if err != nil || len(p2.Announcements) != 1 || p2.Next != nil || p2.Announcements[0].ID != ids[0] {
		t.Fatalf("第二頁 = %+v, %v", p2, err)
	}
	if _, err := f.svc.List(context.Background(), nil, -1); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("負的 page_size：%v", err)
	}
}
