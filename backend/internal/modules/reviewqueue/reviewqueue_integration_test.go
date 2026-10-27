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

	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// setup 給這支測試一個套用了 seller migration 的 database（本模組沒有自己的 schema，讀 seller 發布的 pub_review_items，函式建在 seller 的表上）。
func setup(t *testing.T) (*db.DB, *reviewqueue.Module) {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "seller", Dir: seller.Migrations()}))
	return d, reviewqueue.New(reviewqueue.Deps{DB: d})
}

// base 是測試固定的時間起點；測試不讀時鐘（now 一律是注入的值）。
var reporterSeq atomic.Int64

var base = time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seedStore 建一個店家（直接寫 seller 的表：本模組的測試只需要 pub_review_items 讀得到的事實，不經 seller 的指令）。
func seedStore(t *testing.T, d *db.DB, n int, name string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	must(t, d.Conn(ctx).QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4) RETURNING store_id`, 2600000000+int64(n), 1000+int64(n), name, base).Scan(&id))
	return id
}

// seedReport 建一份待分案的通報（REPORT_TRIAGE：開始是 created_at，期限是 +72 小時，食安 +4 小時）。
func seedReport(t *testing.T, d *db.DB, storeID int64, category string, created time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	// 每份通報一個檢舉人：同一個買家對同一家店同一天只能有一份（reports_buyer_daily）。
	buyer := reporterSeq.Add(1)
	_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at)
		VALUES ($1, $2, $3, '測試', $4, $5, $5)`, id, storeID, category, buyer, created)
	must(t, err)
	return id
}

// seedOrder 建一件處理中的主管機關來函（AUTHORITY_ORDER：開始是 received_at，期限是 due_at；沒有單一店家）。
func seedOrder(t *testing.T, d *db.DB, no int, received, due time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	_, err := d.Conn(ctx).Exec(ctx, `INSERT INTO seller.authority_orders (id, agency, document_no, received_at, kind, summary, due_at, hold_until, registered_at, registered_by_admin_id)
		VALUES ($1, '測試機關', $2, $3, 'OTHER', '測試', $4, $5, $3, 1)`, id, fmt.Sprintf("測字第%d號", no), received, due, received.Add(184*24*time.Hour))
	must(t, err)
	return id
}

// world 建一份含期限相同、開始時間也相同、跨種類的待辦：
//   - 通報（一般 72 小時）created 在 T，與來函 received T、due T+72h：兩個種類的期限與開始時間都相同，只靠種類分先後；
//   - 同一個 (期限, 開始) 的多件同種類：靠 item_id 分先後；
//   - 期限相同、開始時間不同；期限不同；食安通報（4 小時）的期限落在別件的期限之間。
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
	seedReport(t, d, store, "OTHER", T.Add(time.Hour))             // 期限 T+73h
	seedReport(t, d, other, "FOOD_SAFETY", T.Add(68*time.Hour))    // 期限 T+72h，開始 T+68h：期限相同、開始時間不同
	seedReport(t, d, other, "FOOD_SAFETY", T.Add(68*time.Hour))    // 同上，與上一件完全相同
	seedReport(t, d, store, "FOOD_SAFETY", T.Add(2*time.Hour))     // 期限 T+6h
	seedOrder(t, d, 100, T.Add(-24*time.Hour), T.Add(6*time.Hour)) // 期限 T+6h，開始 T-24h：與上一件期限相同，開始較早
	seedOrder(t, d, 101, T.Add(2*time.Hour), T.Add(6*time.Hour))   // 期限 T+6h，開始 T+2h：與食安通報 (T+6h, T+2h) 完全相同、種類不同
	seedOrder(t, d, 102, T.Add(2*time.Hour), T.Add(6*time.Hour))   // 同上，同種類的第二件
	seedOrder(t, d, 103, T.Add(100*time.Hour), T.Add(240*time.Hour))
	seedReport(t, d, store, "OTHER", T.Add(500*time.Hour))
}

// reference 是 pub_review_items 在 now 的全部列，由 Go 以位元組順序排成全序（不經過本模組的 SQL）。
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

// pageAll 以 size 逐頁讀完（游標取自上一頁最後一列），回傳依序拼起來的列與頁數。
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
	// 一次讀完與逐頁一致，列的內容（店家與店名）也與來源相同。
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

// 模組認得的種類與來源一致：seller 的 10 支函式各自的 kind 字面值，集合等於模組的 Kinds（加來源的種類時沒有同步 model.Kinds，這裡失敗）。
// 查詢以 kinds 篩選，所以來源多出模組不認得的種類時，那些列在執行期不會出現在結果裡、也不報錯，只有這個測試（測試期）看得到。
func TestKindsMatchTheSource(t *testing.T) {
	t.Parallel()
	d, _ := setup(t)
	ctx := context.Background()
	fns := []string{
		"review_report_triage", "review_case_decision", "review_case_inquiry_overdue", "review_authority_order",
		"review_authority_target_notice", "review_measure_appeal", "review_measure_lift_request", "review_measure_review",
		"review_kyc_application", "review_payuni_bind_retry",
	}
	var sourceKinds []string
	// 每支函式的 kind 字面值：從函式定義裡讀（pg_get_functiondef 的第一個 SELECT 字面值）。
	for _, fn := range fns {
		var def string
		must(t, d.Conn(ctx).QueryRow(ctx, `SELECT pg_get_functiondef(('seller.'||$1||'(timestamptz)')::regprocedure)`, fn).Scan(&def))
		i := strings.Index(def, "SELECT '")
		j := strings.Index(def[i+8:], "'")
		if i < 0 || j < 0 {
			t.Fatalf("%s：讀不到 kind 字面值", fn)
		}
		sourceKinds = append(sourceKinds, def[i+8:i+8+j])
	}
	slices.Sort(sourceKinds)
	var known []string
	for _, k := range reviewqueue.Kinds() {
		known = append(known, string(k))
	}
	slices.Sort(known)
	if !slices.Equal(sourceKinds, known) {
		t.Errorf("seller 的 10 支函式發布的 kind %v，模組認得的 %v", sourceKinds, known)
	}
}

// setupICU 給這支測試一個預設 collation 是 ICU 語言排序（en-US）的 database，套用了與 setup 相同的 migration（只有 seller 的）。
// 測試用的 Postgres 預設是 C 系的 collation，位元組順序與資料庫的預設順序相同，COLLATE "C" 有沒有寫都看不出來。
func setupICU(t *testing.T) (*db.DB, *reviewqueue.Module) {
	t.Helper()
	ctx := context.Background()
	adminURL := pg.NewURL(t) // 一個空的 database，只借它的連線建立另一個 database
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
	must(t, migrate.Apply(ctx, d, []migrate.Module{{Name: "seller", Dir: seller.Migrations()}}))
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
	// 前提：這個 database 的語言排序真的與位元組順序不同（否則測試是空轉）。
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
	slices.Sort(want) // Go 的字串比較就是位元組順序
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
