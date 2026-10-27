//go:build integration

package devseed_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/devseed"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// 來函的對象位置是連續的（DB 的 prev_position 外鍵保證）；種子只能整份刪：一份來函同時指名種子帳號（位置 0）與非種子帳號（位置 1），
// 清除之後整份來函都消失（對象、被取代的告知、延長歷史、登記時已清理的快照），不會留下缺號；沒有碰到種子的來函原封不動。
// mutation：把 clearChildrenSQL 的對象刪除改回只刪種子帳號的對象 → 位置 1 的對象留下、位置 0 缺號，或自我參照外鍵擋下刪除，斷言失敗。
func TestClearingTheSeedDeletesWholeAuthorityOrdersAndNeverLeavesAGap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "seller", Dir: seller.Migrations()}))
	now := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	var seedStore, otherStore int64
	for i, dst := range []*int64{&seedStore, &otherStore} {
		if err := d.Pool().QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, '店', $3, $3) RETURNING store_id`,
			i+1, 100+i, now).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := d.Pool().Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	const order = `INSERT INTO seller.authority_orders (id, agency, document_no, received_at, kind, summary, due_at, hold_until, registered_at, registered_by_admin_id)
		VALUES ($1::uuid, 'x', 'y', $2::timestamptz, 'DATA_REQUEST', 's', $2::timestamptz + interval '1 hour', $2::timestamptz + interval '184 days', $2::timestamptz, 31)`
	const bothID, otherID = "00000000-0000-7000-8000-000000000001", "00000000-0000-7000-8000-000000000002"
	exec(order, bothID, now)
	exec(order, otherID, now)
	exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, 100), ($1, 1, 101)`, bothID)
	exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, 101)`, otherID)
	exec(`INSERT INTO seller.authority_order_prepurged_stores (order_id, store_id) VALUES ($1, $2)`, bothID, seedStore)

	// 結構保證：位置 2 沒有位置 1 的對象不能存在，刪掉中間的位置也不行。
	if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 3, 102)`, bothID); err == nil {
		t.Fatal("位置缺號必須被資料庫擋下")
	}
	if _, err := d.Pool().Exec(ctx, `DELETE FROM seller.authority_order_targets WHERE order_id = $1 AND position = 0`, bothID); err == nil {
		t.Fatal("刪掉中間的位置必須被資料庫擋下")
	}

	for _, stmt := range devseed.ClearChildrenSQL {
		if _, err := d.Pool().Exec(ctx, stmt, []int64{seedStore}); err != nil {
			t.Fatalf("%s：%v", stmt, err)
		}
	}
	var remaining []string
	rows, err := d.Pool().Query(ctx, `SELECT order_id::text || '/' || position FROM seller.authority_order_targets ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		remaining = append(remaining, s)
	}
	if len(remaining) != 1 || remaining[0] != otherID+"/0" {
		t.Fatalf("只剩沒碰到種子的來函的對象：%v", remaining)
	}
	var orders, prepurged int
	if err := d.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM seller.authority_orders), (SELECT count(*) FROM seller.authority_order_prepurged_stores)`).Scan(&orders, &prepurged); err != nil {
		t.Fatal(err)
	}
	if orders != 1 || prepurged != 0 {
		t.Fatalf("碰到種子的整份來函消失：orders=%d prepurged=%d", orders, prepurged)
	}
}

// 混合的來函（也有非種子的對象、措施或認定）種子不清：Check 拒絕並列出來函 id，一筆都不刪；純種子的來函不被列出（由 clearChildrenSQL 連同依附資料刪除）。
// 非種子帳號合法地導出措施與認定（來函指名它）：沒有這個檢查，清除會刪掉非種子的對象並撞上 measures／finding_authority_orders 的外鍵。
// 「非種子」以店家為單位（與 clearChildrenSQL 一致）：指名種子帳號較早關閉的 stint 的對象也是非種子。
// mutation：把 mixedOrdersSQL 的 derived 與 findings 兩個 EXISTS 改成 false → 對應的來函不在拒絕訊息裡，斷言失敗；
// 把 targets 的 EXISTS 改成 false → 只有不碰種子的對象的來函不在訊息裡，斷言失敗；targets 改回「帳號與店家都不是種子」→ 指名舊 stint 的來函不在訊息裡。
func TestCheckRefusesMixedAuthorityOrdersAndListsTheirIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "seller", Dir: seller.Migrations()}))
	now := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	var seedStore, otherStore int64
	for i, dst := range []*int64{&seedStore, &otherStore} {
		if err := d.Pool().QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, '店', $3, $3) RETURNING store_id`,
			i+1, 100+i, now).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := d.Pool().Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// 種子帳號較早的 stint（已關閉的店家，不是種子店家）：它的措施、認定與指名它的對象都是非種子資料；只指名種子帳號（不指定店家）的對象不構成混合。
	var oldStore int64
	closeTx, err := d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeTx.QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, status, current_closed_at, created_at, updated_at) VALUES (3, 100, '舊店', 'CLOSED', $1, $1, $1) RETURNING store_id`,
		now).Scan(&oldStore); err != nil {
		t.Fatal(err)
	}
	if _, err := closeTx.Exec(ctx, `INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES ($1, $2, 'ACCOUNT_OWNER')`, oldStore, now); err != nil {
		t.Fatal(err)
	}
	if err := closeTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	const order = `INSERT INTO seller.authority_orders (id, agency, document_no, received_at, kind, summary, due_at, hold_until, registered_at, registered_by_admin_id)
		VALUES ($1::uuid, 'x', 'y', $2::timestamptz, 'DATA_REQUEST', 's', $2::timestamptz + interval '1 hour', $2::timestamptz + interval '184 days', $2::timestamptz, 31)`
	const (
		pureID     = "00000000-0000-7000-8000-000000000001" // 只指名種子帳號
		targetsID  = "00000000-0000-7000-8000-000000000002" // 種子帳號加非種子帳號，沒有依附資料
		measureID  = "00000000-0000-7000-8000-000000000003" // 種子帳號較早 stint（非種子店家）的措施引用
		findingID  = "00000000-0000-7000-8000-000000000004" // 非種子店家的認定引用
		untouchID  = "00000000-0000-7000-8000-000000000005" // 沒碰到種子
		oldStintID = "00000000-0000-7000-8000-000000000006" // 種子帳號加指名它較早 stint 的對象，沒有依附資料
		measureUID = "00000000-0000-7000-8000-0000000000a1"
		violation  = "00000000-0000-7000-8000-0000000000b1"
	)
	for _, id := range []string{pureID, targetsID, measureID, findingID, untouchID, oldStintID} {
		exec(order, id, now)
	}
	exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, 100)`, pureID)
	exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, 100), ($1, 1, 101)`, targetsID)
	for _, id := range []string{measureID, findingID} {
		exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, 100)`, id)
	}
	exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, 101)`, untouchID)
	exec(`INSERT INTO seller.authority_order_targets (order_id, position, account_id, store_id) VALUES ($1, 0, 100, NULL), ($1, 1, 100, $2)`, oldStintID, oldStore)

	tx, err := d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, starts_at, current_revision, created_by_admin_id, created_at, updated_at, source_authority_order_id)
		 VALUES ('` + measureUID + `', 100, ` + itoa(oldStore) + `, 'SUSPENSION', 'RISK', 'FRAUD', '2026-10-20T00:00:00Z', 's', '2026-10-01T04:00:00Z', 1, 31, '2026-10-01T04:00:00Z', '2026-10-01T04:00:00Z', '` + measureID + `')`,
		`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ('` + measureUID + `', 1, 's', 31, '2026-10-01T04:00:00Z')`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// 認定：violations 的來源措施外鍵可延遲，認定與措施成對建立。
	tx, err = d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, lead_finding_id, statement_to_seller, starts_at, ends_at, current_revision, created_by_admin_id, created_at, updated_at)
		 VALUES ('00000000-0000-7000-8000-0000000000a2', 100, ` + itoa(oldStore) + `, 'SUSPENSION', 'VIOLATION', '` + violation + `', 's', '2026-10-01T04:00:00Z', '2026-10-08T04:00:00Z', 1, 31, '2026-10-01T04:00:00Z', '2026-10-01T04:00:00Z')`,
		`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at) VALUES ('00000000-0000-7000-8000-0000000000a2', 1, 's', '2026-10-08T04:00:00Z', 31, '2026-10-01T04:00:00Z')`,
		`INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at)
		 VALUES ('` + violation + `', ` + itoa(oldStore) + `, 100, 'FRAUD', 'LOW', '00000000-0000-7000-8000-0000000000a2', 31, '2026-10-01T04:00:00Z', '2026-10-01T04:00:00Z')`,
		`INSERT INTO seller.measure_findings (measure_id, violation_id) VALUES ('00000000-0000-7000-8000-0000000000a2', '` + violation + `')`,
		`INSERT INTO seller.finding_authority_orders (violation_id, order_id) VALUES ('` + violation + `', '` + findingID + `')`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s：%v", stmt, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	stores := []devseed.Store{{StoreID: seedStore, StoreCode: 1, AccountID: 100, Name: "店", TownCode: "x", LogoKey: "k", LogoAssetID: "a", PayUniMerStatus: "1", PayUniMemberStatus: "1"}}
	err = devseed.CheckForTest(ctx, d.Pool(), stores, nil, now)
	if err == nil {
		t.Fatal("混合的來函必須讓前提檢查拒絕")
	}
	msg := err.Error()
	for _, id := range []string{targetsID, measureID, findingID, oldStintID} {
		if !strings.Contains(msg, id) {
			t.Errorf("拒絕訊息要列出混合的來函 %s：%s", id, msg)
		}
	}
	for _, id := range []string{pureID, untouchID} {
		if strings.Contains(msg, id) {
			t.Errorf("純種子或沒碰到種子的來函不該被列出 %s：%s", id, msg)
		}
	}
	var orders, targets int
	if err := d.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM seller.authority_orders), (SELECT count(*) FROM seller.authority_order_targets)`).Scan(&orders, &targets); err != nil {
		t.Fatal(err)
	}
	if orders != 6 || targets != 8 {
		t.Fatalf("Check 不寫任何資料：orders=%d targets=%d", orders, targets)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
