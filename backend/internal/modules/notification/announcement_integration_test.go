//go:build integration

package notification

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/app"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/stream"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type steppingClock struct {
	mu  sync.Mutex
	now time.Time
}

func newSteppingClock() *steppingClock {
	return &steppingClock{now: time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)}
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(time.Millisecond)
	return c.now
}

type jobQueue struct {
	mu      sync.Mutex
	pending []river.JobArgs
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

type testDirectory struct {
	buyers *buyer.Module
	stores []announce.Recipient
	poison *announce.Recipient
}

func (d testDirectory) Buyers(ctx context.Context, after int64, limit int) ([]announce.Recipient, error) {
	first := d.poison != nil && after == 0
	n := limit
	if first {
		n = limit - 1
	}
	rows, err := d.buyers.ListLiveAccounts(ctx, after, n)
	if err != nil {
		return nil, err
	}
	var out []announce.Recipient
	if first {
		out = append(out, *d.poison)
	}
	for _, r := range rows {
		out = append(out, announce.Recipient{AccountID: r.AccountID, Cursor: r.BuyerID})
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
	buyers   *buyer.Module
	svc      *announce.Service
	inbox    *app.Service
	jobs     *jobQueue
	dir      *testDirectory
	stores   *fakeStores
	clock    *steppingClock
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
	clock := newSteppingClock()
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: time.Now, Logger: log})
	events.Subscribe[identity.AccountCreated](bus, buyers.OnAccountCreated)
	stores := &fakeStores{retired: map[int64]bool{}}
	q := &jobQueue{}
	dir := &testDirectory{buyers: buyers}
	svc := announce.New(announce.Deps{
		DB: d, Locks: intake.Locks{Accounts: id, Buyers: buyers, Stores: stores}, Directory: dir,
		Audit: adminaudit.New(d, clock.Now), Signals: dbSignals{d}, Jobs: q, Now: clock.Now, NewID: uuid.NewV7, Logger: log, Chunk: chunk,
	})
	inbox := app.New(app.Deps{DB: d, Accounts: id, Registry: registry.Registry{}, Now: time.Now, Logger: log})
	return &announceFixture{db: d, identity: id, buyers: buyers, svc: svc, inbox: inbox, jobs: q, dir: dir, stores: stores, clock: clock}
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

// 沒有 device token 的帳號也收到收件匣那份（舊系統只送有 token 的人）；兩個帳號都計入 inbox_count，派送完成後 dispatched 為真。
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

// 推播與收件匣分開：每個收件人（不看 token）寫一筆 PENDING 的推播投遞紀錄並排渠道 job；關掉公告推播的人沒有推播紀錄，收件匣照樣有。
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
	in := announce.CreateInput{
		Actor:     adminaudit.Actor{AdminID: 7, Action: "/hawkr.admin.notification.v1alpha1.NotificationAdminService/CreateAnnouncement"},
		Audiences: []string{"BUYER"}, Title: "Hello", Body: "World", RequestID: &rid,
	}

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

// 建立的公告出現在收件人的收件匣：列表（含 category=ANNOUNCEMENT 篩選，total 計入）、未讀數、全部已讀都看得到它；
// 文字是公告的英文原文，zh-TW 請求也一樣。公告的收件匣列只存公告 ID，標題與內文在讀取時由公告型別解析。
// mutation：型別註冊表排除 announcement → 收件匣的型別篩選把公告列藏起來，這個測試紅。
func TestAnnouncementShowsInTheInboxAndIsRenderedInEnglish(t *testing.T) {
	f := setupAnnounce(t, 0)
	ctx := context.Background()
	acct := f.newAccount(t)
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}, Title: "Scheduled maintenance", Body: "Down on Sunday."})
	f.drain(t)
	r := app.Recipient{AccountID: acct, Audience: model.AudienceBuyer}
	announcementCategory := model.CategoryAnnouncement

	for name, category := range map[string]*model.PrefCategory{"不篩選": nil, "ANNOUNCEMENT": &announcementCategory} {
		page, err := f.inbox.ListInbox(ctx, r, category, uitext.ZhTW, nil, 10)
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("%s：total = %d、items = %d，預期 1、1", name, page.Total, len(page.Items))
		}
		it := page.Items[0]
		if it.Title != "Scheduled maintenance" || it.Body != "Down on Sunday." || it.Category != model.CategoryAnnouncement || it.Presentation != model.PresentationSystem || it.ReadAt != nil {
			t.Errorf("%s：項目 %+v", name, it)
		}
	}
	if n, err := f.inbox.UnreadCount(ctx, r); err != nil || n != 1 {
		t.Fatalf("未讀數 = %d（%v），預期 1", n, err)
	}
	if n, err := f.inbox.MarkAllRead(ctx, r); err != nil || n != 1 {
		t.Fatalf("全部已讀改了 %d 列（%v），預期 1", n, err)
	}
	if n, err := f.inbox.UnreadCount(ctx, r); err != nil || n != 0 {
		t.Errorf("全部已讀之後未讀數 = %d（%v），預期 0", n, err)
	}
	if f.inboxRowFor(t, a.ID, acct) != 1 {
		t.Error("收件匣列不該被已讀刪掉")
	}
}

// 一個永遠寫不進去的收件人（買家帶了 store_id，違反 inbox_items_store_check 與 deliveries_store_check）不擋住整則公告：兩條鏈都略過它、游標照常前進，
// 同一頁與之後的收件人照常收到，dispatched 為真；略過的收件人沒有任何列。
// mutation：fanout.go 的 page 對永久性錯誤也直接 return → 派送卡在第一頁，後面的人收不到，dispatched 不會變真，失敗。
func TestOneRecipientThatCanNeverBeWrittenDoesNotStopTheAnnouncement(t *testing.T) {
	f := setupAnnounce(t, 2)
	var accounts []int64
	for range 4 {
		accounts = append(accounts, f.newAccount(t))
	}
	f.dir.poison = &announce.Recipient{AccountID: accounts[0], StoreID: 99, Cursor: 0}
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	f.drain(t)

	if f.inboxRows(t, a.ID) != 4 || f.pushRows(t, a.ID) != 4 {
		t.Errorf("收件匣 %d 列、推播 %d 筆，預期各 4（四個真的買家都要收到，毒收件人不多不少）", f.inboxRows(t, a.ID), f.pushRows(t, a.ID))
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.inbox_items WHERE announcement_id = $1 AND audience = 'BUYER' AND store_id IS NOT NULL`, a.ID); n != 0 {
		t.Errorf("毒收件人寫進了 %d 列", n)
	}
	for _, acct := range accounts {
		if f.inboxRowFor(t, a.ID, acct) != 1 {
			t.Errorf("帳號 %d 沒有收到收件匣列", acct)
		}
	}
	if got := f.reload(t); !got.Dispatched || got.InboxCount != 4 {
		t.Errorf("dispatched = %v、inbox_count = %d，預期 true、4", got.Dispatched, got.InboxCount)
	}
}

// 最後一頁不滿 Chunk 個收件人就直接收尾，不多排一個空頁：3 個買家、一頁 2 人，收件匣與推播各兩個 job（開頭一個加第二頁一個）。
// mutation：fanout.go 的 `len(recipients) >= s.d.Chunk` 改成 `len(recipients) > 0` → 各多排一個空頁，失敗。
func TestLastPartialPageFinishesWithoutAnExtraEmptyPage(t *testing.T) {
	f := setupAnnounce(t, 2)
	for range 3 {
		f.newAccount(t)
	}
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	f.drain(t)
	if n := f.jobs.count(announce.InboxArgs{}.Kind()); n != 2 {
		t.Errorf("收件匣派送 job 共 %d 個，預期 2", n)
	}
	if n := f.jobs.count(announce.PushArgs{}.Kind()); n != 2 {
		t.Errorf("推播派送 job 共 %d 個，預期 2", n)
	}
	if f.inboxRows(t, a.ID) != 3 || !f.reload(t).Dispatched {
		t.Errorf("收件匣 %d 列、dispatched = %v，預期 3、true", f.inboxRows(t, a.ID), f.reload(t).Dispatched)
	}
}

// 清除一個收件人的個資時，所有推播收件人共用的公告意圖（id 與公告相同，參數只有公告 ID）不被清掉：其他收件人的投遞紀錄仍連得到有內容的意圖，
// 收件匣照樣組得出公告；被清除的收件人自己的列沒了。
// mutation：retention.sql 的 ScrubRecipientIntentParams 拿掉 NOT EXISTS announcements 條件 → 共用意圖被換成 '{}'，失敗。
func TestPurgingOneRecipientKeepsTheSharedAnnouncementIntent(t *testing.T) {
	f := setupAnnounce(t, 0)
	ctx := context.Background()
	gone, kept := f.newAccount(t), f.newAccount(t)
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}, Title: "Maintenance", Body: "Tonight."})
	f.drain(t)
	if f.pushRows(t, a.ID) != 2 || f.inboxRows(t, a.ID) != 2 {
		t.Fatalf("前提：兩人各有收件匣列與推播紀錄，現在 %d、%d", f.inboxRows(t, a.ID), f.pushRows(t, a.ID))
	}

	svc := purge.New(purge.Deps{DB: f.db, Jobs: f.jobs, Now: f.clock.Now, Logger: slog.New(slog.DiscardHandler)})
	if err := svc.Recipient(ctx, purge.RecipientArgs{AccountID: gone, Audience: "BUYER"}); err != nil {
		t.Fatal(err)
	}
	if f.inboxRowFor(t, a.ID, gone) != 0 || f.inboxRowFor(t, a.ID, kept) != 1 {
		t.Errorf("被清除的人收件匣 %d 列、另一人 %d 列，預期 0、1", f.inboxRowFor(t, a.ID, gone), f.inboxRowFor(t, a.ID, kept))
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.intents WHERE id = $1 AND params <> '{}'::jsonb`, a.ID); n != 1 {
		t.Errorf("公告的共用意圖參數被清掉了")
	}
	if n := f.scan(t, `SELECT count(*) FROM notification.deliveries d JOIN notification.intents i ON i.id = d.intent_id
		WHERE d.account_id = $1 AND i.params <> '{}'::jsonb`, kept); n != 1 {
		t.Errorf("另一人的推播紀錄連不到有內容的意圖：%d", n)
	}
	page, err := f.inbox.ListInbox(ctx, app.Recipient{AccountID: kept, Audience: model.AudienceBuyer}, nil, uitext.EN, nil, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].Title != "Maintenance" {
		t.Errorf("另一人的收件匣 = %+v（%v）", page.Items, err)
	}
}

// 收件匣組文字時，單列的問題（參數快照壞掉）記錄後略過、其他列照常；讀公告內容本身失敗（資料庫錯誤）則整個請求失敗，不能悄悄藏掉通知。
// mutation：inbox.go 的 renderRows 把 Load 的讀取錯誤也記錄後略過 → 第二段不再回錯誤，失敗。
func TestInboxSkipsABrokenRowButFailsOnALoadError(t *testing.T) {
	f := setupAnnounce(t, 0)
	ctx := context.Background()
	acct := f.newAccount(t)
	f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}, Title: "Good", Body: "b"})
	broken := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}, Title: "Broken", Body: "b"})
	f.drain(t)
	r := app.Recipient{AccountID: acct, Audience: model.AudienceBuyer}

	if _, err := f.db.Pool().Exec(ctx, `UPDATE notification.inbox_items SET params = '{"announcement_id": 7}' WHERE announcement_id = $1`, broken.ID); err != nil {
		t.Fatal(err)
	}
	page, err := f.inbox.ListInbox(ctx, r, nil, uitext.EN, nil, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].Title != "Good" || page.Total != 2 {
		t.Fatalf("壞掉的快照應只略過那一列：items = %+v、total = %d（%v）", page.Items, page.Total, err)
	}

	if _, err := f.db.Pool().Exec(ctx, `ALTER TABLE notification.announcements RENAME TO announcements_unreachable`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.inbox.ListInbox(ctx, r, nil, uitext.EN, nil, 10); err == nil {
		t.Fatal("讀公告內容失敗應讓請求失敗，不該回一份少了通知的收件匣")
	}
}

// 公告的收件匣寫入與串流訊號同一個 transaction：訂閱者收到的未讀數與最新一則，和 UnreadCount 同一條規則（只算註冊的型別，含 announcement）。
// mutation：fanout.go 的 writeInbox 不呼叫 Signals.InboxChanged → 訂閱者收不到公告的訊號，逾時失敗。
func TestAnnouncementFanOutSignalsTheRecipientsStream(t *testing.T) {
	f := setupAnnounce(t, 0)
	hub := stream.NewHub(inboxReader(f.inbox), slog.New(slog.DiscardHandler), time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- hub.Run(ctx, stream.DialPG(f.db.Pool().Config().ConnConfig)) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run：%v", err)
		}
	})
	select {
	case <-hub.Listening():
	case <-time.After(5 * time.Second):
		t.Fatal("LISTEN 連線沒有建立")
	}
	acct := f.newAccount(t)
	sub, err := hub.Subscribe(stream.Key{AccountID: acct, Audience: model.AudienceBuyer}, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.Close)
	next := func() stream.Event {
		c, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		ev, err := sub.Next(c)
		if err != nil {
			t.Fatalf("Next：%v", err)
		}
		return ev
	}
	if ev := next(); ev.Inbox == nil || ev.Inbox.UnreadCount != 0 {
		t.Fatalf("連線建立的第一則應是空收件匣：%+v", ev)
	}

	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	f.drain(t)
	ev := next()
	var rowID uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.inbox_items WHERE announcement_id = $1 AND account_id = $2`, a.ID, acct).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	if ev.Inbox == nil || ev.Inbox.UnreadCount != 1 || ev.Inbox.LatestItemID == nil || *ev.Inbox.LatestItemID != rowID {
		t.Fatalf("訂閱者應收到未讀 1 與公告列 %s：%+v", rowID, ev)
	}
	if !slices.Equal(ev.Inbox.NewItemIDs, []uuid.UUID{rowID}) {
		t.Fatalf("公告扇出的訊號應帶這個收件人的新列 ID %s：%+v", rowID, ev.Inbox)
	}
}
