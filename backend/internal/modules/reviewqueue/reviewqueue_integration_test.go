//go:build integration

package reviewqueue_test

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func setup(t *testing.T) (*db.DB, *reviewqueue.Module) {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "payment", Dir: payment.Migrations()},
		migrate.Module{Name: "moderation", Dir: moderation.Migrations()},
	))
	return d, reviewqueue.New(reviewqueue.Deps{DB: d})
}

var reporterSeq atomic.Int64

var base = time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func seedStore(t *testing.T, d *db.DB, n int, name string) int64 {
	t.Helper()
	ctx := context.Background()
	accountID, err := sellerfixture.NewAccount(ctx, d, base)
	must(t, err)
	id, err := sellerfixture.ExposableStoreNamed(ctx, d, accountID, base, name+fmt.Sprint(n))
	must(t, err)
	return id
}

func seedReport(t *testing.T, d *db.DB, storeID int64, category string, created time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	buyer := reporterSeq.Add(1)
	module, err := sellerfixture.NewModule(d, func() time.Time { return created })
	must(t, err)
	report, err := module.SubmitReport(ctx, seller.SubmitReportInput{StoreID: storeID, Category: policy.Category(category), Description: "測試", ReporterBuyerID: buyer})
	must(t, err)
	return report.ReportID
}

func seedOrder(t *testing.T, d *db.DB, no int, received, due time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	accountID, err := sellerfixture.NewAccount(ctx, d, received)
	must(t, err)
	module, err := sellerfixture.NewModule(d, func() time.Time { return received })
	must(t, err)
	result, err := module.CreateAuthorityOrder(ctx, seller.CreateAuthorityOrderInput{
		AdminID: 1, Agency: "測試機關", DocumentNo: fmt.Sprintf("測字第%d號", no), ReceivedAt: received,
		Kind: seller.AuthorityOther, Summary: "測試", Targets: []seller.AuthorityTargetInput{{AccountID: accountID}}, DueAt: &due,
	})
	must(t, err)
	return result.Order.ID
}

func world(t *testing.T, d *db.DB) {
	t.Helper()
	store := seedStore(t, d, 1, "範例小農")
	other := seedStore(t, d, 2, "晴空果園")
	T := base
	for range 4 {
		seedReport(t, d, store, "OTHER", T)
		seedOrder(t, d, int(T.Unix()), T, T.Add(72*time.Hour))
	}
	for range 3 {
		seedReport(t, d, other, "OTHER", T)
	}
	seedReport(t, d, store, "OTHER", T.Add(time.Hour))
	seedReport(t, d, other, "FOOD_SAFETY", T.Add(68*time.Hour))
	seedReport(t, d, other, "FOOD_SAFETY", T.Add(68*time.Hour))
	seedReport(t, d, store, "FOOD_SAFETY", T.Add(2*time.Hour))
	seedOrder(t, d, 100, T.Add(-24*time.Hour), T.Add(6*time.Hour))
	seedOrder(t, d, 101, T.Add(2*time.Hour), T.Add(6*time.Hour))
	seedOrder(t, d, 102, T.Add(2*time.Hour), T.Add(6*time.Hour))
	seedOrder(t, d, 103, T.Add(100*time.Hour), T.Add(240*time.Hour))
	seedReport(t, d, store, "OTHER", T.Add(500*time.Hour))
}

func reference(t *testing.T, d *db.DB, now time.Time) []reviewqueue.Item {
	t.Helper()
	ctx := context.Background()
	rows, err := d.Conn(ctx).Query(ctx, `SELECT kind, item_id, store_id, store_name, start_at, due_at FROM seller.pub_review_items($1::timestamptz)`, now)
	must(t, err)
	defer rows.Close()
	var out []reviewqueue.Item
	for rows.Next() {
		var (
			kind, id string
			store    *int64
			name     *string
			it       reviewqueue.Item
		)
		must(t, rows.Scan(&kind, &id, &store, &name, &it.Start, &it.Due))
		it.Kind, it.ItemID, it.StoreID = reviewqueue.Kind(kind), id, store
		if name != nil {
			it.StoreName = *name
		}
		out = append(out, it)
	}
	must(t, rows.Err())
	slices.SortFunc(out, func(a, b reviewqueue.Item) int {
		return cmpAll(a.Due.Compare(b.Due), a.Start.Compare(b.Start), strings.Compare(string(a.Kind), string(b.Kind)), strings.Compare(a.ItemID, b.ItemID))
	})
	return out
}

func cmpAll(cs ...int) int {
	for _, c := range cs {
		if c != 0 {
			return c
		}
	}
	return 0
}

func ident(items []reviewqueue.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = fmt.Sprintf("%s|%s|%s|%s", it.Due.UTC().Format(time.RFC3339Nano), it.Start.UTC().Format(time.RFC3339Nano), it.Kind, it.ItemID)
	}
	return out
}

func pageAll(t *testing.T, m *reviewqueue.Module, now time.Time, kinds []reviewqueue.Kind, size int32) ([]reviewqueue.Item, int) {
	t.Helper()
	var (
		all   []reviewqueue.Item
		after *reviewqueue.Cursor
		pages int
	)
	for {
		page, err := m.List(context.Background(), now, kinds, after, size)
		must(t, err)
		pages++
		if pages > 1000 {
			t.Fatal("翻頁沒有結束")
		}
		all = append(all, page...)
		if int32(len(page)) < size {
			return all, pages
		}
		c := reviewqueue.CursorOf(page[len(page)-1])
		after = &c
	}
}

// 翻頁不漏不重：每頁 1、2、3、7 筆，資料含期限相同、開始時間也相同、跨種類的列；逐頁讀完的序列等於 pub_review_items 依全序排好的序列。
// mutation：ORDER BY 或 keyset 的 row-value 少掉 item_id，同一個 (期限, 開始, 種類) 的多件就會在頁界重複或漏掉，這裡失敗（見 TestOrderingIsBytewiseNotLocale
// 對 item_id 的 COLLATE "C" 的 mutation；N＋1 的 mutation 在 handler 的測試）。
func TestListPagesWithoutGapsOrDuplicates(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	world(t, d)
	now := base.Add(24 * time.Hour)
	want := reference(t, d, now)
	if len(want) < 20 {
		t.Fatalf("資料集只有 %d 列，翻頁的測試沒有意義", len(want))
	}
	kindsSeen := map[reviewqueue.Kind]bool{}
	ties := 0
	for i, it := range want {
		kindsSeen[it.Kind] = true
		if i > 0 && want[i-1].Due.Equal(it.Due) && want[i-1].Start.Equal(it.Start) {
			ties++
		}
	}
	if len(kindsSeen) < 2 || ties < 5 {
		t.Fatalf("資料集沒有跨種類或沒有 (期限, 開始) 都相同的列：種類 %d、相同 %d", len(kindsSeen), ties)
	}
	for _, size := range []int32{1, 2, 3, 7} {
		got, pages := pageAll(t, m, now, reviewqueue.Kinds(), size)
		if !slices.Equal(ident(got), ident(want)) {
			t.Errorf("每頁 %d：逐頁讀完的序列與全序不同\n got: %v\nwant: %v", size, ident(got), ident(want))
		}
		if wantPages := len(want)/int(size) + 1; pages != wantPages {
			t.Errorf("每頁 %d：%d 頁，預期 %d 頁", size, pages, wantPages)
		}
	}
	all, err := m.List(context.Background(), now, reviewqueue.Kinds(), nil, int32(len(want)))
	must(t, err)
	if !slices.Equal(ident(all), ident(want)) {
		t.Errorf("一次讀完的序列與全序不同")
	}
	for i := range all {
		if all[i].StoreName != want[i].StoreName || (all[i].StoreID == nil) != (want[i].StoreID == nil) || (all[i].StoreID != nil && *all[i].StoreID != *want[i].StoreID) {
			t.Errorf("第 %d 列的店家 %v/%q，來源 %v/%q", i, all[i].StoreID, all[i].StoreName, want[i].StoreID, want[i].StoreName)
		}
	}
}

// kinds 篩選：每一種單獨篩選的集合等於全部裡那一種的集合（順序同全序）；空的 kinds 沒有結果（展開成全部是呼叫端的事）。
func TestListFiltersByKinds(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	world(t, d)
	now := base.Add(24 * time.Hour)
	want := reference(t, d, now)
	ctx := context.Background()
	for _, k := range reviewqueue.Kinds() {
		var wantKind []reviewqueue.Item
		for _, it := range want {
			if it.Kind == k {
				wantKind = append(wantKind, it)
			}
		}
		got, pages := pageAll(t, m, now, []reviewqueue.Kind{k}, 3)
		if !slices.Equal(ident(got), ident(wantKind)) {
			t.Errorf("%s：篩選的序列與全部裡那一種不同（%d 頁）\n got: %v\nwant: %v", k, pages, ident(got), ident(wantKind))
		}
	}
	both, err := m.List(ctx, now, []reviewqueue.Kind{reviewqueue.ReportTriage, reviewqueue.AuthorityOrder}, nil, 1000)
	must(t, err)
	if len(both) != len(want) {
		t.Errorf("兩種合起來 %d 列，這份資料集只有這兩種，預期 %d 列", len(both), len(want))
	}
	none, err := m.List(ctx, now, nil, nil, 10)
	must(t, err)
	if len(none) != 0 {
		t.Errorf("空的 kinds 回了 %d 列", len(none))
	}
}

// 輸入的檢查：limit 要正數、種類要有效（空字串與未知的值不是種類）、游標要是真的位置。
func TestListRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	_, m := setup(t)
	ctx := context.Background()
	good := reviewqueue.Cursor{Due: base, Start: base, Kind: reviewqueue.ReportTriage, ID: "x"}
	for name, call := range map[string]func() error{
		"limit 0":  func() error { _, err := m.List(ctx, base, reviewqueue.Kinds(), nil, 0); return err },
		"limit 負數": func() error { _, err := m.List(ctx, base, reviewqueue.Kinds(), nil, -1); return err },
		"空字串的種類":   func() error { _, err := m.List(ctx, base, []reviewqueue.Kind{""}, nil, 10); return err },
		"未知的種類":    func() error { _, err := m.List(ctx, base, []reviewqueue.Kind{"BOGUS"}, nil, 10); return err },
		"游標沒有時間": func() error {
			c := good
			c.Due = time.Time{}
			_, err := m.List(ctx, base, reviewqueue.Kinds(), &c, 10)
			return err
		},
		"游標的種類空": func() error {
			c := good
			c.Kind = ""
			_, err := m.List(ctx, base, reviewqueue.Kinds(), &c, 10)
			return err
		},
		"游標的 ID 空": func() error {
			c := good
			c.ID = ""
			_, err := m.List(ctx, base, reviewqueue.Kinds(), &c, 10)
			return err
		},
		"游標的時間不是 µs": func() error {
			c := good
			c.Start = base.Add(1)
			_, err := m.List(ctx, base, reviewqueue.Kinds(), &c, 10)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s：沒有回錯", name)
		}
	}
	if _, err := m.List(ctx, base, reviewqueue.Kinds(), &good, 10); err != nil {
		t.Errorf("有效的游標被拒絕：%v", err)
	}
}

// 模組認得的種類與來源一致：seller 的 10 支函式、payment 與 moderation 的函式各自的 kind 字面值，集合等於模組的 Kinds（加來源的種類時沒有同步
// model.Kinds，這裡失敗）。
// 查詢以 kinds 篩選，所以來源多出模組不認得的種類時，那些列在執行期不會出現在結果裡、也不報錯，只有這個測試（測試期）看得到。
func TestKindsMatchTheSource(t *testing.T) {
	t.Parallel()
	d, _ := setup(t)
	ctx := context.Background()
	fns := []string{
		"seller.review_report_triage", "seller.review_case_decision", "seller.review_case_inquiry_overdue", "seller.review_authority_order",
		"seller.review_authority_target_notice", "seller.review_measure_appeal", "seller.review_measure_lift_request", "seller.review_measure_review",
		"seller.review_kyc_application", "seller.review_shared_bank_account", "payment.pub_review_items", "moderation.review_moderation_case",
	}
	var sourceKinds []string
	for _, fn := range fns {
		var def string
		must(t, d.Conn(ctx).QueryRow(ctx, `SELECT pg_get_functiondef(($1||'(timestamptz)')::regprocedure)`, fn).Scan(&def))
		rest, found := def, 0
		for {
			i := strings.Index(rest, "SELECT '")
			if i < 0 {
				break
			}
			j := strings.Index(rest[i+8:], "'")
			if j < 0 {
				t.Fatalf("%s：讀不到 kind 字面值", fn)
			}
			sourceKinds = append(sourceKinds, rest[i+8:i+8+j])
			found++
			rest = rest[i+8+j:]
			if fn != "payment.pub_review_items" {
				break
			}
		}
		if found == 0 {
			t.Fatalf("%s：讀不到 kind 字面值", fn)
		}
	}
	slices.Sort(sourceKinds)
	var known []string
	for _, k := range reviewqueue.Kinds() {
		known = append(known, string(k))
	}
	slices.Sort(known)
	if !slices.Equal(sourceKinds, known) {
		t.Errorf("seller、payment 與 moderation 的函式發布的 kind %v，模組認得的 %v", sourceKinds, known)
	}
}

func setupICU(t *testing.T) (*db.DB, *reviewqueue.Module) {
	t.Helper()
	ctx := context.Background()
	adminURL := pg.NewURL(t)
	admin, err := pgx.Connect(ctx, adminURL)
	must(t, err)
	t.Cleanup(func() { _ = admin.Close(context.WithoutCancel(ctx)) })
	name := fmt.Sprintf("icu_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name+` TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER icu ICU_LOCALE 'en-US'`); err != nil {
		t.Skipf("這個 Postgres 不能建立 ICU 預設 collation 的 database：%v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.WithoutCancel(ctx), `DROP DATABASE `+name+` WITH (FORCE)`)
	})
	u, err := url.Parse(adminURL)
	must(t, err)
	u.Path = "/" + name
	d, err := db.Open(ctx, db.Config{URL: u.String(), MaxConns: 5, ConnectRetryBudget: 10 * time.Second})
	must(t, err)
	t.Cleanup(d.Close)
	must(t, migrate.Apply(ctx, d, []migrate.Module{{Name: "seller", Dir: seller.Migrations()}, {Name: "payment", Dir: payment.Migrations()}, {Name: "moderation", Dir: moderation.Migrations()}}))
	return d, reviewqueue.New(reviewqueue.Deps{DB: d})
}

// 全序是位元組順序，不是資料庫的 collation：在預設 collation 是 ICU 語言排序的 database 裡（a < A < b < B），query 的 ORDER BY 與 keyset 的
// row-value 比較沒有 COLLATE "C" 時依語言排序，逐頁讀完的序列不等於位元組順序（A < B < a < b），翻頁在頁界漏掉或重複。
// mutation：拿掉 ORDER BY 或 keyset 任一處的 item_id 的 COLLATE "C"，這裡失敗。kind 的 COLLATE "C" 不在內：現在的 kind 值在預設與 C collation
// 下順序相同，拿掉它這裡抓不到（實測）；保留它是為了與 item_id 一致、不依賴資料庫的 collation。觸發：新增的 kind 值若與既有值在
// collation 下順序不同（例如 '_' 對字母的位置），這支 ICU 測試要加入跨種類的列。
func TestOrderingIsBytewiseNotLocale(t *testing.T) {
	t.Parallel()
	d, m := setupICU(t)
	ctx := context.Background()
	_, err := d.Conn(ctx).Exec(ctx, `
		CREATE OR REPLACE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
		LANGUAGE sql STABLE
		BEGIN ATOMIC
		    SELECT 'REPORT_TRIAGE'::text, v.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
		           NULL::uuid, NULL::uuid, NULL::uuid, NULL::bigint, NULL::text, at, at
		    FROM (VALUES ('B'), ('a'), ('A'), ('b'), ('a-1'), ('a/1'), ('ab'), ('Ab'), ('_x'), ('1')) AS v(id);
		END`)
	must(t, err)
	var locale, bytewise []string
	for _, q := range []struct {
		dst   *[]string
		order string
	}{{&locale, `ORDER BY item_id`}, {&bytewise, `ORDER BY item_id COLLATE "C"`}} {
		rows, err := d.Conn(ctx).Query(ctx, `SELECT item_id FROM seller.pub_review_items($1::timestamptz) `+q.order, base)
		must(t, err)
		for rows.Next() {
			var s string
			must(t, rows.Scan(&s))
			*q.dst = append(*q.dst, s)
		}
		must(t, rows.Err())
	}
	if slices.Equal(locale, bytewise) {
		t.Fatalf("語言排序與位元組順序相同（%v），測試沒有鑑別力", locale)
	}
	want := slices.Clone(bytewise)
	slices.Sort(want)
	if !slices.Equal(bytewise, want) {
		t.Fatalf("COLLATE \"C\" 的順序 %v 不是位元組順序 %v", bytewise, want)
	}
	for _, size := range []int32{1, 2, 3, 7} {
		got, _ := pageAll(t, m, base, []reviewqueue.Kind{reviewqueue.ReportTriage}, size)
		ids := make([]string, len(got))
		for i, it := range got {
			ids[i] = it.ItemID
		}
		if !slices.Equal(ids, want) {
			t.Errorf("每頁 %d：逐頁讀完 %v，位元組順序 %v", size, ids, want)
		}
	}
}

// payment 的待辦和 seller 的一起排序分頁：建立失敗的收款帳戶出現在結果裡，帶店家 ID、店名是空字串（由後台入口補），期限是失敗後 24 小時。
// mutation：items.sql 拿掉 payment.pub_review_items 的 UNION ALL 分支 → 沒有這一列，失敗。
func TestPaymentItemsAreMergedIntoTheQueue(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	ctx := context.Background()
	_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO payment.accounts (store_id, provider, requirement_collection, create_failed_at, created_at, updated_at)
		VALUES (42, 'fake', 'PROVIDER', $1, $1, $1)`, base)
	must(t, err)
	items, err := m.List(ctx, base.Add(time.Hour), reviewqueue.Kinds(), nil, 10)
	must(t, err)
	if len(items) != 1 || items[0].Kind != reviewqueue.PaymentAccountCreateFailed || items[0].ItemID != "42" || items[0].StoreID == nil || *items[0].StoreID != 42 ||
		items[0].StoreName != "" || !items[0].Due.Equal(base.Add(24*time.Hour)) {
		t.Fatalf("items=%+v", items)
	}
}

// 清理進了終局的收款帳戶（purge_blocked_at 有值）也出現在待辦，種類是 PAYMENT_ACCOUNT_PURGE_STUCK，開始時間是進終局的時刻、期限 24 小時；建立失敗的種類不受影響。
// mutation：payment.pub_review_items 拿掉 PURGE_STUCK 分支 → 沒有這一列，失敗。
func TestPaymentPurgeStuckItemsAreMergedIntoTheQueue(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	ctx := context.Background()
	_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO payment.accounts (store_id, provider, requirement_collection, create_called_at, purge_pending_at, purge_blocked_at, purge_blocked_reason, created_at, updated_at)
		VALUES (43, 'fake', 'PROVIDER', $1, $1, $1, 'provider_refused：test', $1, $1)`, base)
	must(t, err)
	items, err := m.List(ctx, base.Add(time.Hour), reviewqueue.Kinds(), nil, 10)
	must(t, err)
	if len(items) != 1 || items[0].Kind != reviewqueue.PaymentAccountPurgeStuck || items[0].ItemID != "43" || items[0].StoreID == nil || *items[0].StoreID != 43 ||
		!items[0].Start.Equal(base) || !items[0].Due.Equal(base.Add(24*time.Hour)) {
		t.Fatalf("items=%+v", items)
	}
}

type modCase struct {
	id       uuid.UUID
	opened   time.Time
	reports  int
	target   string
	decision string
}

func seedCase(t *testing.T, d *db.DB, c modCase, contentID *uuid.UUID, buyerID *int64) {
	t.Helper()
	ctx := context.Background()
	switch c.decision {
	case "":
		_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO moderation.cases (id, target_kind, content_id, buyer_id, opened_at) VALUES ($1, $2, $3, $4, $5)`,
			c.id, c.target, contentID, buyerID, c.opened)
		must(t, err)
	case "VIOLATION":
		_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO moderation.cases (id, target_kind, content_id, buyer_id, opened_at, decision, decision_category, decision_action, decided_by_admin_id, closed_at)
			VALUES ($1, $2, $3, $4, $5::timestamptz, 'VIOLATION', 'SPAM', 'WARN', 1, $5::timestamptz + interval '1 hour')`, c.id, c.target, contentID, buyerID, c.opened)
		must(t, err)
	case "NO_VIOLATION":
		_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO moderation.cases (id, target_kind, content_id, buyer_id, opened_at, decision, decided_by_admin_id, closed_at)
			VALUES ($1, $2, $3, $4, $5::timestamptz, 'NO_VIOLATION', 1, $5::timestamptz + interval '1 hour')`, c.id, c.target, contentID, buyerID, c.opened)
		must(t, err)
	default:
		t.Fatalf("不認得的決定 %q", c.decision)
	}
	for i := 0; i < c.reports; i++ {
		_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO moderation.reports (id, case_id, reporter_buyer_id, category, created_at) VALUES ($1, $2, $3, 'SPAM', $4)`,
			uuid.NewV7(), c.id, reporterSeq.Add(1), c.opened.Add(time.Duration(i)*time.Minute))
		must(t, err)
	}
}

func moderationWorld(t *testing.T, d *db.DB) (open, other []modCase) {
	t.Helper()
	post, comment, entry, reused := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	buyerA, buyerB := int64(501), int64(502)
	mk := func(opened time.Time, reports int, target, decision string) modCase {
		return modCase{id: uuid.NewV7(), opened: opened, reports: reports, target: target, decision: decision}
	}
	openOne := mk(base.Add(1*time.Hour), 1, "POST", "")
	openMany := mk(base.Add(2*time.Hour), 4, "COMMENT", "")
	closedViolation := mk(base.Add(3*time.Hour), 2, "FORUM_ENTRY", "VIOLATION")
	closedNo := mk(base.Add(4*time.Hour), 1, "COMMENT", "NO_VIOLATION")
	reopened := mk(base.Add(30*time.Hour), 2, "FORUM_ENTRY", "")
	buyerOpen := mk(base.Add(5*time.Hour), 3, "BUYER", "")
	buyerClosed := mk(base.Add(6*time.Hour), 1, "BUYER", "NO_VIOLATION")
	buyerReopened := mk(base.Add(40*time.Hour), 1, "BUYER", "")
	seedCase(t, d, openOne, &post, nil)
	seedCase(t, d, openMany, &comment, nil)
	seedCase(t, d, closedViolation, &entry, nil)
	seedCase(t, d, closedNo, &reused, nil)
	seedCase(t, d, reopened, &entry, nil)
	seedCase(t, d, buyerOpen, nil, &buyerA)
	seedCase(t, d, buyerClosed, nil, &buyerB)
	seedCase(t, d, buyerReopened, nil, &buyerB)
	return []modCase{openOne, openMany, reopened, buyerOpen, buyerReopened}, []modCase{closedViolation, closedNo, buyerClosed}
}

// 每種領域狀態進不進待辦，等於 moderation 的判定式（案件的 closed_at IS NULL）：進行中的（一筆或多筆檢舉、結案後同對象重開的新案件、買家對象）都在，
// 結案的（VIOLATION、NO_VIOLATION、買家的結案）都不在；期限是開案後 72 小時、開始是期限前 24 小時，kind 是 MODERATION_CASE，item_id 與 CaseID 是案件 ID，沒有店家。
// mutation：moderation.review_moderation_case 漏掉一種進行中的狀態（例如 WHERE 加 AND target_kind <> 'BUYER'）→ 進行中的買家案件不在待辦，這裡失敗。
func TestModerationCasesAreInTheQueueExactlyWhenOpen(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	ctx := context.Background()
	open, other := moderationWorld(t, d)
	now := base.Add(100 * time.Hour)
	items, err := m.List(ctx, now, []reviewqueue.Kind{reviewqueue.ModerationCase}, nil, 100)
	must(t, err)
	got := map[string]reviewqueue.Item{}
	for _, it := range items {
		got[it.ItemID] = it
	}
	if len(got) != len(items) || len(items) != len(open) {
		t.Fatalf("待辦 %d 件，預期進行中的 %d 件：%+v", len(items), len(open), items)
	}
	for _, c := range open {
		it, ok := got[c.id.String()]
		if !ok {
			t.Errorf("進行中的案件 %s（%s、%d 筆檢舉）不在待辦", c.id, c.target, c.reports)
			continue
		}
		due := c.opened.Add(72 * time.Hour)
		if it.Kind != reviewqueue.ModerationCase || it.CaseID == nil || *it.CaseID != c.id || it.StoreID != nil || it.StoreName != "" ||
			!it.Due.Equal(due) || !it.Start.Equal(due.Add(-24*time.Hour)) {
			t.Errorf("案件 %s 的列 %+v，預期期限 %v、開始 %v", c.id, it, due, due.Add(-24*time.Hour))
		}
	}
	for _, c := range other {
		if _, ok := got[c.id.String()]; ok {
			t.Errorf("結案的案件 %s（%s）在待辦裡", c.id, c.decision)
		}
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].Due.After(items[i].Due) {
			t.Errorf("沒有依期限排序：%v", ident(items))
		}
	}
}

// 期限與時間無關：at 只是呼叫方傳入的時間，不是 now()；案件已逾期（at 在期限之後）仍在待辦，at 早於開案也一樣在。
func TestModerationCaseDeadlineDoesNotDependOnAt(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	ctx := context.Background()
	open, _ := moderationWorld(t, d)
	for _, at := range []time.Time{base.Add(-time.Hour), base.Add(3 * time.Hour), base.Add(1000 * time.Hour)} {
		items, err := m.List(ctx, at, []reviewqueue.Kind{reviewqueue.ModerationCase}, nil, 100)
		must(t, err)
		if len(items) != len(open) {
			t.Errorf("at=%v：待辦 %d 件，預期 %d", at, len(items), len(open))
		}
	}
}

// 跨種類的 keyset：seller 的通報、payment 的收款帳戶與 moderation 的案件交錯，其中有期限與開始時間都相同的列；每頁 1、2、3、5 筆讀完，
// 序列等於三個來源的全部列依 (期限, 開始, 種類, ID) 的位元組順序排好的序列，沒有漏、沒有重複，也沒有多出別的列。
// mutation：items.sql 拿掉 moderation 的 UNION ALL 分支，序列少了案件，這裡失敗。
func TestCrossSourceKeysetPagingWithModeration(t *testing.T) {
	t.Parallel()
	d, m := setup(t)
	ctx := context.Background()
	store := seedStore(t, d, 1, "範例小農")
	open, _ := moderationWorld(t, d)
	seedReport(t, d, store, "OTHER", base.Add(1*time.Hour))
	seedReport(t, d, store, "OTHER", base.Add(2*time.Hour))
	seedReport(t, d, store, "FOOD_SAFETY", base.Add(69*time.Hour))
	for i, at := range []time.Time{base.Add(49 * time.Hour), base.Add(50 * time.Hour)} {
		_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO payment.accounts (store_id, provider, requirement_collection, create_failed_at, created_at, updated_at)
			VALUES ($1, 'fake', 'PROVIDER', $2, $2, $2)`, int64(900+i), at)
		must(t, err)
	}
	now := base.Add(200 * time.Hour)
	var want []reviewqueue.Item
	want = append(want, reference(t, d, now)...)
	rows, err := d.Conn(ctx).Query(ctx, `SELECT kind, item_id, start_at, due_at FROM payment.pub_review_items($1::timestamptz) UNION ALL SELECT kind, item_id, start_at, due_at FROM moderation.pub_review_items($1::timestamptz)`, now)
	must(t, err)
	for rows.Next() {
		var kind, id string
		var it reviewqueue.Item
		must(t, rows.Scan(&kind, &id, &it.Start, &it.Due))
		it.Kind, it.ItemID = reviewqueue.Kind(kind), id
		want = append(want, it)
	}
	must(t, rows.Err())
	rows.Close()
	slices.SortFunc(want, func(a, b reviewqueue.Item) int {
		return cmpAll(a.Due.Compare(b.Due), a.Start.Compare(b.Start), strings.Compare(string(a.Kind), string(b.Kind)), strings.Compare(a.ItemID, b.ItemID))
	})
	if len(want) != 3+2+len(open) {
		t.Fatalf("參考序列 %d 列，預期 %d：%v", len(want), 3+2+len(open), ident(want))
	}
	for _, size := range []int32{1, 2, 3, 5} {
		got, _ := pageAll(t, m, now, reviewqueue.Kinds(), size)
		if !slices.Equal(ident(got), ident(want)) {
			t.Errorf("每頁 %d 筆：\n得到 %v\n預期 %v", size, ident(got), ident(want))
		}
		seen := map[string]bool{}
		for _, it := range got {
			k := string(it.Kind) + "/" + it.ItemID
			if seen[k] {
				t.Errorf("每頁 %d 筆：%s 重複", size, k)
			}
			seen[k] = true
		}
	}
}
