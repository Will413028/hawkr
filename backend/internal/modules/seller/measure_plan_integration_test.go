//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// recorder 記下最後一次送出的 SQL 與參數，讓測試對「程式實際送出的查詢」做 EXPLAIN，不在測試裡另寫一份查詢。
type recorder struct {
	inner store.DBTX
	sql   string
	args  []any
}

func (r *recorder) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return r.inner.Exec(ctx, sql, args...)
}

func (r *recorder) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.sql, r.args = sql, args
	return r.inner.Query(ctx, sql, args...)
}

func (r *recorder) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return r.inner.QueryRow(ctx, sql, args...)
}

func (f *fixture) explain(t *testing.T, r *recorder) string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "EXPLAIN (COSTS OFF) "+r.sql, r.args...)
	must(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		must(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	must(t, rows.Err())
	return strings.Join(lines, "\n")
}

// seedHistory 寫入大量已結束的停權（歷史）與少量此刻生效中的停權：historical 筆已結束的認定停權、timed 筆結束時間在未來的
// 認定停權、open 筆沒有固定期間的 RISK，另有 expiredRisk 筆已結束但沒撤銷的 RISK（仍是 ENFORCED）、revokedRisk 筆沒有固定期間但
// 已撤銷的 RISK；每個帳號各一家店。
func (f *fixture) seedHistory(t *testing.T, accounts, historical, timed, open, expiredRisk, revokedRisk int) {
	t.Helper()
	ctx := context.Background()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		c := f.db.Conn(ctx)
		for _, stmt := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at)
				SELECT 2900000000 + g, 100000 + g, 'x', $1, $1 FROM generate_series(1, $2::int) g`, []any{f.now, accounts}},
			{`CREATE TEMP TABLE seed ON COMMIT DROP AS
				SELECT g, md5('m' || g)::uuid AS id, st.store_id, st.account_id, $6::timestamptz - interval '100 days' + g * interval '1 minute' AS created_at,
				       CASE WHEN g <= $2::int THEN 'RISK_OPEN' WHEN g <= $2::int + $3::int THEN 'RISK_EXPIRED' WHEN g <= $2::int + $3::int + $4::int THEN 'TIMED'
				            WHEN g <= $2::int + $3::int + $4::int + $7::int THEN 'RISK_REVOKED' ELSE 'HISTORY' END AS shape
				FROM generate_series(1, $2::int + $3::int + $4::int + $5::int + $7::int) g
				JOIN (SELECT store_id, account_id, row_number() OVER (ORDER BY store_id) AS rn FROM seller.stores WHERE store_code > 2900000000) st
				  ON st.rn = (g % $1::int) + 1`, []any{accounts, open, expiredRisk, timed, historical, f.now, revokedRisk}},
			{`INSERT INTO seller.violations (id, store_id, account_id, category, severity, status, origin_measure_id, created_by_admin_id, created_at, updated_at)
				SELECT id, store_id, account_id, 'FRAUD', 'LOW', 'ENFORCED', id, 1, created_at, $1 FROM seed WHERE shape IN ('TIMED', 'HISTORY')`, []any{f.now}},
			{`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, lead_finding_id, review_by, statement_to_seller, ends_at, status, version, current_revision,
					created_by_admin_id, created_at, updated_at, starts_at, revoked_reason, revoked_by_admin_id, revoked_at)
				SELECT id, account_id, store_id, 'SUSPENSION', CASE WHEN shape LIKE 'RISK%' THEN 'RISK' ELSE 'VIOLATION' END,
				       CASE WHEN shape LIKE 'RISK%' THEN 'FRAUD' END, CASE WHEN shape LIKE 'RISK%' THEN NULL ELSE id END,
				       CASE WHEN shape LIKE 'RISK%' THEN $1::timestamptz - interval '60 days' END, 'x',
				       CASE shape WHEN 'RISK_OPEN' THEN NULL WHEN 'RISK_REVOKED' THEN NULL WHEN 'TIMED' THEN $1::timestamptz + interval '10 days' ELSE $1::timestamptz - interval '50 days' END,
				       CASE WHEN shape = 'RISK_REVOKED' THEN 'REVOKED' ELSE 'ENFORCED' END, 1, 1, 1, created_at, $1, created_at,
				       CASE WHEN shape = 'RISK_REVOKED' THEN 'OTHER' END, CASE WHEN shape = 'RISK_REVOKED' THEN 1::bigint END,
				       CASE WHEN shape = 'RISK_REVOKED' THEN $1::timestamptz - interval '1 day' END
				FROM seed`, []any{f.now}},
			{`INSERT INTO seller.measure_findings (measure_id, violation_id) SELECT id, id FROM seed WHERE shape IN ('TIMED', 'HISTORY')`, nil},
			{`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at)
				SELECT id, 1, 'x',
				       CASE shape WHEN 'RISK_OPEN' THEN NULL WHEN 'RISK_REVOKED' THEN NULL WHEN 'TIMED' THEN $1::timestamptz + interval '10 days' ELSE $1::timestamptz - interval '50 days' END,
				       1, created_at FROM seed`, []any{f.now}},
		} {
			if _, err := c.Exec(ctx, stmt.sql, stmt.args...); err != nil {
				return err
			}
		}
		return nil
	}))
	for _, table := range []string{"stores", "violations", "measures", "measure_findings", "measure_revisions"} {
		if _, err := f.db.Conn(ctx).Exec(ctx, "ANALYZE seller."+table); err != nil {
			t.Fatal(err)
		}
	}
}

// 「此刻生效中」的讀取量不隨歷史的認定停權成長：在 6 萬筆已結束的停權之上，對帳、後台清單的停權篩選與措施列表的
// ACTIVE、ENDED 篩選用到預期的索引，而且沒有掃描措施或版本的整張表。結果也一致：EffectiveMeasuresAll 與逐帳號的
// EffectiveMeasures 對同一個時間點回傳同一批措施。有固定期間與沒有固定期間的各用 measures 上的一個部分索引、都有 Index Cond：有固定期間的走
// measures_active_timed（ends_at > now），沒有固定期間的走 measures_active_open（索引只含未撤銷也未結束的列；種子含已結束但沒撤銷的
// RISK 與沒有固定期間但已撤銷的 RISK，它們不在索引裡）。
func TestEffectiveMeasuresReadsScaleWithTheEffectiveSetNotTheHistory(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const accounts, historical, timed, open, expiredRisk, revokedRisk = 3000, 60000, 20, 10, 30, 30
	f.seedHistory(t, accounts, historical, timed, open, expiredRisk, revokedRisk)

	rec := &recorder{inner: f.db.Conn(ctx)}
	q := store.New(rec)
	suspended, err := stint.SuspendedAccounts(ctx, q, f.now)
	must(t, err)
	if len(suspended) == 0 {
		t.Fatal("seed 沒有生效中的停權")
	}
	plan := f.explain(t, rec)
	t.Logf("EffectiveMeasuresAll（對帳、停權篩選、措施列表的 state 篩選共用）：\n%s", plan)
	for leg, pattern := range map[string]string{
		"結束時間在未來的那一段要以 ends_at > now 作為 Index Cond（不是 Filter）":                                 `measures_active_timed[^\n]*\n\s*Index Cond: [^\n]*ends_at > [^\n]*starts_at <= `,
		"沒有固定期間的那一段要走只含生效中列的 measures_active_open，以 starts_at <= now 作為 Index Cond（不是 Filter）": `measures_active_open[^\n]*\n\s*Index Cond: [^\n]*starts_at <= `,
	} {
		if !regexp.MustCompile(pattern).MatchString(plan) {
			t.Errorf("%s：\n%s", leg, plan)
		}
	}
	for _, gone := range []string{"measure_revisions", "measures_review"} {
		if strings.Contains(plan, gone) {
			t.Errorf("EffectiveMeasuresAll 的計畫不得出現 %s（目前值只讀 measures）：\n%s", gone, plan)
		}
	}
	// 已結束與已撤銷由索引排除（部分索引的條件與 Index Cond），不是讀出來再丟掉：Filter 不得出現時間或狀態。BitmapOr 上 planner 會留下
	// `kind = ANY(...)` 的 Filter，它被兩個部分索引的條件蘊含，不會丟掉任何列。
	if m := regexp.MustCompile(`Filter: [^\n]*(ends_at|starts_at|status)`).FindString(plan); m != "" {
		t.Errorf("EffectiveMeasuresAll 不得以 Filter 排除已結束或已撤銷的列（%s）：\n%s", m, plan)
	}
	for _, seq := range []string{"Seq Scan on measures"} {
		if strings.Contains(plan, seq) {
			t.Errorf("EffectiveMeasuresAll 不得掃描整張表（%s）：\n%s", seq, plan)
		}
	}

	ids, err := stint.ActiveMeasureIDs(ctx, q, 0, f.now)
	must(t, err)
	if len(ids) != timed+open {
		t.Fatalf("生效中的措施 %d 筆，預期 %d", len(ids), timed+open)
	}
	byAccount, err := stint.EffectiveMany(ctx, q, suspended, f.now)
	must(t, err)
	perAccount := 0
	for _, s := range byAccount {
		perAccount += len(s.Measures)
	}
	if perAccount != len(ids) {
		t.Fatalf("逐帳號的 EffectiveMeasures 回傳 %d 筆，EffectiveMeasuresAll 回傳 %d 筆：兩條路徑的定義要一致", perAccount, len(ids))
	}
	if _, err := stint.Effective(ctx, q, suspended[0], f.now); err != nil {
		t.Fatal(err)
	}
	plan = f.explain(t, rec)
	t.Logf("EffectiveMeasures（單一帳號）：\n%s", plan)
	if !strings.Contains(plan, "measures_enforced_suspension") {
		t.Errorf("單一帳號的 EffectiveMeasures 要走 measures_enforced_suspension：\n%s", plan)
	}

	scheduled, err := stint.ScheduledMeasureIDs(ctx, q, 0, f.now)
	must(t, err)
	for _, state := range []string{"ACTIVE", "ENDED"} {
		_, err = q.AdminMeasures(ctx, store.AdminMeasuresParams{State: &state, ActiveIds: ids, ScheduledIds: scheduled, PageSize: 21})
		must(t, err)
		plan = f.explain(t, rec)
		t.Logf("AdminMeasures state=%s：\n%s", state, plan)
		want := map[string]string{"ACTIVE": "measures_pkey", "ENDED": "measures_created"}[state]
		if !strings.Contains(plan, want) || strings.Contains(plan, "Seq Scan on measures") {
			t.Errorf("AdminMeasures state=%s 要走 %s、不得掃描整張措施表：\n%s", state, want, plan)
		}
	}

	if _, err := q.AccountsWithOpenPeriod(ctx, store.AccountsWithOpenPeriodParams{AfterAccountID: 0, BatchSize: 100}); err != nil {
		t.Fatal(err)
	}
	plan = f.explain(t, rec)
	t.Logf("AccountsWithOpenPeriod：\n%s", plan)
}

// shapedMeasure 是資料集裡的一筆措施：revisions 依序是各版本的結束時間（nil 為沒有固定期間），最後一版是目前版本。
type shapedMeasure struct {
	name      string
	account   int
	kind      model.MeasureKind
	basis     model.Basis
	revoked   bool
	createdAt time.Time
	revisions []*time.Time
	want      bool
}

// 逐帳號的 EffectiveMeasures、全部帳號的 EffectiveMeasuresAll 與 model.StateOf 對同一份資料集回傳同一批生效中的措施（集合相等，
// 不只筆數）：資料集含已撤銷、警告、結束時間剛好是 now、剛好晚 1µs、尚未開始、延長與縮短的多版本、沒有固定期間與已結束的 RISK，
// 並且與人工標註的預期集合相等，三條路徑不會一起錯。
func TestEffectiveMeasuresPerAccountAllAccountsAndStateOfAgreeOnADataset(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	now := f.now
	us := time.Microsecond
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	vio, risk := model.BasisViolation, model.BasisRisk
	susp, warn := model.MeasureSuspension, model.MeasureWarning
	past := now.Add(-48 * time.Hour)
	cases := []shapedMeasure{
		{"撤銷的認定停權", 1, susp, vio, true, past, []*time.Time{at(time.Hour)}, false},
		{"撤銷的開放式 RISK", 2, susp, risk, true, past, []*time.Time{nil}, false},
		{"警告", 3, warn, vio, false, past, []*time.Time{nil}, false},
		{"結束時間剛好是 now", 4, susp, vio, false, past, []*time.Time{at(0)}, false},
		{"結束時間晚 now 1µs", 5, susp, vio, false, past, []*time.Time{at(us)}, true},
		{"延長：舊版已過、目前版在未來", 6, susp, vio, false, past, []*time.Time{at(-time.Hour), at(time.Hour)}, true},
		{"縮短：舊版在未來、目前版已過", 7, susp, vio, false, past, []*time.Time{at(time.Hour), at(-us)}, false},
		{"縮短到剛好 now", 8, susp, vio, false, past, []*time.Time{at(time.Hour), at(0)}, false},
		{"開放式 RISK", 9, susp, risk, false, past, []*time.Time{nil}, true},
		{"已結束的 RISK（舊版開放、目前版已過）", 10, susp, risk, false, past, []*time.Time{nil, at(-time.Hour)}, false},
		{"RISK 結束後又改回開放式", 11, susp, risk, false, past, []*time.Time{nil, at(-time.Hour), nil}, true},
		{"剛好開始", 12, susp, vio, false, now, []*time.Time{at(time.Hour)}, true},
		{"晚 1µs 才開始", 13, susp, vio, false, now.Add(us), []*time.Time{at(time.Hour)}, false},
		{"同帳號：生效中", 14, susp, vio, false, past, []*time.Time{at(time.Hour)}, true},
		{"同帳號：已結束", 14, susp, vio, false, past, []*time.Time{at(-time.Hour)}, false},
		{"同帳號：已撤銷的開放式 RISK", 14, susp, risk, true, past, []*time.Time{nil}, false},
	}
	ids := make([]string, len(cases))
	for i := range cases {
		ids[i] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		c := f.db.Conn(ctx)
		exec := func(sql string, args ...any) error { _, err := c.Exec(ctx, sql, args...); return err }
		for a := 1; a <= 14; a++ {
			if err := exec(`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, 'x', $3, $3)`, 2900000000+a, 100000+a, now); err != nil {
				return err
			}
		}
		for i, m := range cases {
			var storeID int64
			if err := c.QueryRow(ctx, `SELECT store_id FROM seller.stores WHERE account_id = $1`, 100000+m.account).Scan(&storeID); err != nil {
				return err
			}
			var finding, reviewBy, ground, revokedReason, revokedBy, revokedAt any
			if m.basis == risk {
				ground, reviewBy = "FRAUD", m.createdAt.Add(24*time.Hour)
			} else {
				finding = ids[i]
			}
			if m.revoked {
				revokedReason, revokedBy, revokedAt = "OTHER", 1, now.Add(-time.Hour)
			}
			status := "ENFORCED"
			if m.revoked {
				status = "REVOKED"
			}
			if m.basis == vio {
				if err := exec(`INSERT INTO seller.violations (id, store_id, account_id, category, severity, status, origin_measure_id, created_by_admin_id, created_at, updated_at)
					VALUES ($1, $2, $3, 'FRAUD', 'LOW', 'ENFORCED', $1, 1, $4, $4)`, ids[i], storeID, 100000+m.account, m.createdAt); err != nil {
					return err
				}
			}
			if err := exec(`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, lead_finding_id, review_by, statement_to_seller, ends_at, status, version,
					current_revision, created_by_admin_id, created_at, updated_at, starts_at, revoked_reason, revoked_by_admin_id, revoked_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'x', $15, $9, 1, $10, 1, $11, $11, $16, $12, $13, $14)`,
				ids[i], 100000+m.account, storeID, string(m.kind), string(m.basis), ground, finding, reviewBy, status, len(m.revisions), m.createdAt,
				revokedReason, revokedBy, revokedAt, m.revisions[len(m.revisions)-1], m.createdAt); err != nil {
				return err
			}
			if m.basis == vio {
				if err := exec(`INSERT INTO seller.measure_findings (measure_id, violation_id) VALUES ($1, $1)`, ids[i]); err != nil {
					return err
				}
			}
			for r, ends := range m.revisions {
				if err := exec(`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at) VALUES ($1, $2, 'x', $3, 1, $4)`,
					ids[i], r+1, ends, m.createdAt); err != nil {
					return err
				}
			}
		}
		return nil
	}))

	var accounts []int64
	for a := 1; a <= 14; a++ {
		accounts = append(accounts, int64(100000+a))
	}
	q := store.New(f.db.Conn(ctx))
	perAccount := map[string]bool{}
	byAccount, err := stint.EffectiveMany(ctx, q, accounts, now)
	must(t, err)
	for _, s := range byAccount {
		for _, id := range s.IDs() {
			perAccount[id.String()] = true
		}
	}
	allAccounts := map[string]bool{}
	all, err := q.EffectiveMeasuresAll(ctx, now)
	must(t, err)
	for _, r := range all {
		allAccounts[r.MeasureID.String()] = true
	}
	if len(all) != len(allAccounts) {
		t.Fatalf("EffectiveMeasuresAll 回傳重複的措施：%d 列、%d 筆不同", len(all), len(allAccounts))
	}
	stored, err := q.MeasuresByIDs(ctx, uuidsOf(t, ids))
	must(t, err)
	stateOf, want := map[string]bool{}, map[string]bool{}
	for _, row := range stored {
		if model.StateOf(model.MeasureStatus(row.Status), model.MeasureKind(row.Kind), row.StartsAt, row.EndsAt, now) == model.StateActive {
			stateOf[row.ID.String()] = true
		}
	}
	if len(stored) != len(cases) {
		t.Fatalf("資料集 %d 筆，讀回 %d 筆", len(cases), len(stored))
	}
	for i, m := range cases {
		if m.want {
			want[ids[i]] = true
		}
	}
	keys := func(set map[string]bool) []string {
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	f.assertCurrentValuesMatchHistory(t)
	for name, got := range map[string]map[string]bool{"逐帳號 EffectiveMeasures": perAccount, "EffectiveMeasuresAll": allAccounts, "StateOf": stateOf} {
		if !slices.Equal(keys(got), keys(want)) {
			t.Errorf("%s 的生效中集合與預期不同\n實際 %v\n預期 %v", name, labelled(cases, ids, got), labelled(cases, ids, want))
		}
	}
}

func uuidsOf(t *testing.T, ids []string) []uuid.UUID {
	t.Helper()
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.MustParse(id)
	}
	return out
}

func labelled(cases []shapedMeasure, ids []string, set map[string]bool) []string {
	var out []string
	for i, c := range cases {
		if set[ids[i]] {
			out = append(out, c.name)
		}
	}
	return out
}

// measures 的目前值（給賣家的理由、結束時間）與 measure_revisions 裡 current_revision 那一版的快照相同，而且版本連號：
// 目前值與歷史由同一個寫入路徑維持一致（InsertMeasure、AdvanceMeasureRevision 的單一語句）。
func (f *fixture) assertCurrentValuesMatchHistory(t *testing.T) {
	t.Helper()
	if n := f.count(t, `SELECT count(*) FROM seller.measures m LEFT JOIN seller.measure_revisions r ON r.measure_id = m.id AND r.revision = m.current_revision
		WHERE r.measure_id IS NULL OR r.statement_to_seller <> m.statement_to_seller OR r.ends_at IS DISTINCT FROM m.ends_at`); n != 0 {
		t.Errorf("%d 筆措施的目前值與 current_revision 那一版的快照不同", n)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measures m WHERE m.current_revision <> (SELECT count(*) FROM seller.measure_revisions r WHERE r.measure_id = m.id)
		OR m.current_revision <> (SELECT max(revision) FROM seller.measure_revisions r WHERE r.measure_id = m.id)`); n != 0 {
		t.Errorf("%d 筆措施的 current_revision 不是歷史的版本數與最大版本", n)
	}
}
