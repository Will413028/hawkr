//go:build integration

package adminaudit_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type fixture struct {
	db  *db.DB
	log *adminaudit.Log
	now time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{db: pg.NewDB(t, testdb.Migrations(migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()})), now: time.Date(2026, 11, 14, 4, 0, 0, 0, time.UTC)}
	f.log = adminaudit.New(f.db, func() time.Time { return f.now })
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) record(t *testing.T, e adminaudit.Entry) {
	t.Helper()
	must(t, f.db.InTx(context.Background(), func(ctx context.Context) error { return f.log.Record(ctx, e) }))
}

func entry(admin int64, action, targetID string) adminaudit.Entry {
	return adminaudit.Entry{Actor: adminaudit.Actor{AdminID: admin, Action: action}, TargetType: "payment_account", TargetID: targetID, Detail: map[string]any{"requested": true}}
}

// Record 不在 transaction 內呼叫會失敗且沒有寫入：紀錄與領域變更必須同生同滅。
// mutation：拿掉 TxFrom 檢查 → 在 transaction 外寫入成功，失敗。
func TestRecordOutsideATransactionFailsAndWritesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if err := f.log.Record(context.Background(), entry(1, "/x/Y", "5")); !errors.Is(err, adminaudit.ErrNoTransaction) {
		t.Fatalf("err=%v，預期 ErrNoTransaction", err)
	}
	if got, err := f.log.List(context.Background(), adminaudit.Filter{}); err != nil || len(got) != 0 {
		t.Fatalf("不該有紀錄：%v err=%v", got, err)
	}
}

// 領域變更 rollback 時紀錄跟著消失；commit 時一起留下。
// mutation：Record 用自己的連線（不加入呼叫方的 transaction）→ rollback 之後紀錄仍在，失敗。
func TestRecordRollsBackWithTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	boom := errors.New("domain change failed")
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		if err := f.log.Record(ctx, entry(1, "/x/Y", "5")); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if got, err := f.log.List(context.Background(), adminaudit.Filter{}); err != nil || len(got) != 0 {
		t.Fatalf("rollback 之後不該有紀錄：%v err=%v", got, err)
	}
	f.record(t, entry(1, "/x/Y", "5"))
	got, err := f.log.List(context.Background(), adminaudit.Filter{})
	if err != nil || len(got) != 1 || got[0].AdminID != 1 || got[0].Action != "/x/Y" || got[0].TargetType != "payment_account" || got[0].TargetID != "5" || !got[0].OccurredAt.Equal(f.now) || string(got[0].Detail) != `{"requested": true}` {
		t.Fatalf("commit 之後的紀錄=%+v err=%v", got, err)
	}
}

// 欄位不合回 invalid_input，呼叫方的 transaction 隨之失敗；schema 的約束是第二道（直接 INSERT 也擋）。
// mutation：validate 不檢查管理員 ID → 管理員 ID 0 的案例失敗（由 records_admin_id_check 擋下時錯誤不是 invalid_input）。
func TestRecordRejectsInvalidEntriesAndTheSchemaChecksBackThemUp(t *testing.T) {
	t.Parallel()
	f := setup(t)
	big := map[string]any{"x": strings.Repeat("a", adminaudit.MaxDetailBytes)}
	for name, e := range map[string]adminaudit.Entry{
		"管理員 ID 不是正數": entry(0, "/x/Y", "5"),
		"沒有動作":        entry(1, "", "5"),
		"動作太長":        entry(1, strings.Repeat("a", 201), "5"),
		"沒有對象 ID":     entry(1, "/x/Y", ""),
		"對象種類不合":      {Actor: adminaudit.Actor{AdminID: 1, Action: "/x/Y"}, TargetType: "Payment-Account", TargetID: "5"},
		"detail 太大":   {Actor: adminaudit.Actor{AdminID: 1, Action: "/x/Y"}, TargetType: "payment_account", TargetID: "5", Detail: big},
	} {
		err := f.db.InTx(context.Background(), func(ctx context.Context) error { return f.log.Record(ctx, e) })
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v，預期 invalid_input", name, err)
		}
	}
	ctx := context.Background()
	for name, sql := range map[string]string{
		"admin_id":    `INSERT INTO adminaudit.records (occurred_at, admin_id, action, target_type, target_id) VALUES (now(), 0, 'a', 'b', 'c')`,
		"detail 不是物件": `INSERT INTO adminaudit.records (occurred_at, admin_id, action, target_type, target_id, detail) VALUES (now(), 1, 'a', 'b', 'c', '[]')`,
		"target_type": `INSERT INTO adminaudit.records (occurred_at, admin_id, action, target_type, target_id) VALUES (now(), 1, 'a', 'B', 'c')`,
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql); err == nil {
			t.Errorf("%s：schema 沒有擋下", name)
		}
	}
}

// detail 的大小只有一種量法：jsonb 的文字輸出（octet_length(detail::text)，records_detail_check）。{"x": "<n 個 a>"} 的文字輸出是 9+n 位元組，所以 MaxDetailBytes-9 個 a 剛好過、
// 多一個被約束擋下，Record 把它對映成 invalid_input（不是原始的約束錯誤）；剛好過的那筆確實寫進去。
// mutation：約束改回 pg_column_size(detail) → 邊界的行為不同（壓縮與標頭使位元組數不等於文字長度），邊界案例失敗；Record 不對映約束錯誤 → 超過的案例回的不是 invalid_input，失敗。
func TestDetailSizeIsMeasuredByTheJSONBTextOutputAtTheBoundary(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	at := func(n int) adminaudit.Entry {
		return adminaudit.Entry{Actor: adminaudit.Actor{AdminID: 1, Action: "/x/Y"}, TargetType: "payment_account", TargetID: "5", Detail: map[string]any{"x": strings.Repeat("a", n)}}
	}
	fits := adminaudit.MaxDetailBytes - 9
	if err := f.db.InTx(ctx, func(ctx context.Context) error { return f.log.Record(ctx, at(fits)) }); err != nil {
		t.Fatalf("剛好 %d 位元組的 detail 應該寫入：%v", adminaudit.MaxDetailBytes, err)
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error { return f.log.Record(ctx, at(fits+1)) })
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("多一個位元組 err=%v，預期 invalid_input", err)
	}
	var n int
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM adminaudit.records WHERE octet_length(detail::text) = $1", adminaudit.MaxDetailBytes).Scan(&n))
	if n != 1 {
		t.Fatalf("剛好上限的紀錄筆數=%d，預期 1", n)
	}
}

// List 依管理員、動作、對象與時間範圍（Since 含、Until 不含）篩選，新的在前。
// mutation：Until 改成含 → 時間範圍案例多一筆，失敗；篩選條件互相 OR → 筆數不對，失敗。
func TestListFiltersByAdminActionTargetAndTimeRangeNewestFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.record(t, entry(1, "/svc/A", "10"))
	f.now = f.now.Add(time.Hour)
	f.record(t, entry(2, "/svc/A", "11"))
	f.now = f.now.Add(time.Hour)
	f.record(t, entry(1, "/svc/B", "10"))
	ctx := context.Background()
	ids := func(fl adminaudit.Filter) []string {
		got, err := f.log.List(ctx, fl)
		must(t, err)
		var out []string
		for _, r := range got {
			out = append(out, r.TargetID+r.Action)
		}
		return out
	}
	want := func(name string, got []string, exp ...string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(exp, ",") {
			t.Errorf("%s：%v，預期 %v", name, got, exp)
		}
	}
	want("全部、新的在前", ids(adminaudit.Filter{}), "10/svc/B", "11/svc/A", "10/svc/A")
	want("管理員", ids(adminaudit.Filter{AdminID: 1}), "10/svc/B", "10/svc/A")
	want("動作", ids(adminaudit.Filter{Action: "/svc/A"}), "11/svc/A", "10/svc/A")
	want("對象", ids(adminaudit.Filter{TargetType: "payment_account", TargetID: "10"}), "10/svc/B", "10/svc/A")
	want("管理員加動作", ids(adminaudit.Filter{AdminID: 1, Action: "/svc/A"}), "10/svc/A")
	start := time.Date(2026, 11, 14, 4, 0, 0, 0, time.UTC)
	want("時間範圍（Since 含、Until 不含）", ids(adminaudit.Filter{Since: start, Until: start.Add(2 * time.Hour)}), "11/svc/A", "10/svc/A")
	want("limit", ids(adminaudit.Filter{Limit: 1}), "10/svc/B")
	if _, err := f.log.List(ctx, adminaudit.Filter{Limit: 5000}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("過大的 limit err=%v", err)
	}
}
