//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// makeSeededStoresExposable 讓 seedHistory 的店家（store_code > 2900000000）滿足與時間無關的曝光條件：logo、鄉鎮市區、核准的 KYC 申請、
// 綁定且核准的 PayUni 註冊。是否可曝光只剩帳號有沒有生效中的措施。
func (f *fixture) makeSeededStoresExposable(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		c := f.db.Conn(ctx)
		for _, stmt := range []string{
			`UPDATE seller.stores SET logo_key = 'logo/' || store_id, logo_asset_id = 'asset-' || store_id, town_code = '63000010'
				WHERE store_code > 2900000000`,
			`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, real_name, english_name, bank_name, bank_code,
					bank_account_enc, bank_account_hash, bank_account_hash_key, id_card_no_enc, id_card_hash, id_card_hash_key,
					submitted_at, submission_seq, decided_at, created_at, updated_at)
				SELECT md5('kyc' || store_id)::uuid, store_id, 'APPROVED', 'INDIVIDUAL', 'x', 'x', 'x', '004',
				       '\x01', sha256(('b' || store_id)::bytea), 'k', '\x01', sha256(('i' || store_id)::bytea), 'k', $1, 1, $1, $1, $1
				FROM seller.stores WHERE store_code > 2900000000`,
			`INSERT INTO seller.payuni_registrations (store_id, mer_id, application_id, bind_status, bound_at, outcome, status_observed_at,
					created_at, updated_at)
				SELECT store_id, 'HAWK' || store_id, md5('kyc' || store_id)::uuid, 'BOUND', $1, 'APPROVED', $1, $1, $1
				FROM seller.stores WHERE store_code > 2900000000`,
		} {
			var args []any
			if strings.Contains(stmt, "$1") {
				args = []any{f.now}
			}
			if _, err := c.Exec(ctx, stmt, args...); err != nil {
				return err
			}
		}
		return nil
	}))
	for _, table := range []string{"stores", "kyc_applications", "payuni_registrations"} {
		_, err := f.db.Conn(ctx).Exec(ctx, "ANALYZE seller."+table)
		must(t, err)
	}
}

// measureScans 回傳 EXPLAIN (FORMAT JSON) 計畫裡每個讀 seller.measures 的節點（掃描方式、索引、Index Cond）。
func measureScans(t *testing.T, plan []byte) []map[string]any {
	t.Helper()
	var doc []struct {
		Plan map[string]any `json:"Plan"`
	}
	must(t, json.Unmarshal(plan, &doc))
	if len(doc) != 1 {
		t.Fatalf("EXPLAIN 回傳 %d 個計畫", len(doc))
	}
	var out []map[string]any
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		if n["Relation Name"] == "measures" {
			out = append(out, n)
		}
		children, _ := n["Plans"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	walk(doc[0].Plan)
	return out
}

// 上層模組 join pub_exposable_stores 的標準寫法（db/pub.sql 的用法：`EXISTS (... pub_exposable_stores($now) e WHERE e.store_id = p.store_id)`）
// 與 seller 自己的 exposure.Classify（ExposableStoreIDs）讀 measures 時以帳號為鍵：外層小時，每個讀 measures 的節點都是 measures_enforced_suspension
// 上以外層 account_id 為 Index Cond 的探測；外層大（2000 筆商品、3000 家店）時 planner 可以改成 hash anti join，生效集合只讀一次。
// 任何情況下都不能把沒有帳號條件的 measures 掃描重複執行（EXPLAIN ANALYZE 的 loops），不重掃全平台的生效中措施（審查實測：UNION ALL 兩段時帳號條件推不進去，成本是外層列數 × 全平台
// 生效中的措施）。種子：3000 家可曝光條件都滿足的店、6 萬筆已結束的停權、1500 筆生效中；商品 49 與 2000 筆兩種外層。
// mutation：effective_measures 改回 UNION ALL 兩段 → 外層小的兩個查詢出現沒有 account_id 的 measures_active_timed、measures_active_open 掃描，
// 而且每個探測執行一次，失敗。
func TestExposableStoresJoinProbesMeasuresByAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.seedHistory(t, 3000, 60000, 1000, 500, 30, 30)
	f.makeSeededStoresExposable(t)
	conn := f.db.Conn(ctx)
	for _, n := range []int{49, 2000} {
		table := fmt.Sprintf("public.plan_products_%d", n)
		_, err := conn.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s AS
			SELECT g AS product_id, s.store_id FROM generate_series(1, %d) g
			JOIN (SELECT store_id, row_number() OVER (ORDER BY store_id) AS rn FROM seller.stores WHERE store_code > 2900000000) s
			  ON s.rn = (g * 7) %% 3000 + 1`, table, n))
		must(t, err)
		_, err = conn.Exec(ctx, "CREATE INDEX ON "+table+" (store_id)")
		must(t, err)
		_, err = conn.Exec(ctx, "ANALYZE "+table)
		must(t, err)
	}

	var sample []int64
	rows, err := conn.Query(ctx, "SELECT store_id FROM seller.stores WHERE store_code > 2900000000 ORDER BY store_id LIMIT 100")
	must(t, err)
	for rows.Next() {
		var id int64
		must(t, rows.Scan(&id))
		sample = append(sample, id)
	}
	must(t, rows.Err())
	rec := &recorder{inner: conn}
	exposable, err := store.New(rec).ExposableStoreIDs(ctx, store.ExposableStoreIDsParams{StoreIds: sample, At: f.now})
	must(t, err)
	if len(exposable) == 0 || len(exposable) == len(sample) {
		t.Fatalf("種子要同時有可曝光與停權中的店：%d／%d 可曝光", len(exposable), len(sample))
	}

	queries := map[string]struct {
		sql  string
		args []any
	}{
		"exposure.Classify（ExposableStoreIDs）": {rec.sql, rec.args},
	}
	for _, n := range []int{49, 2000} {
		queries[fmt.Sprintf("上層模組的 EXISTS join（%d 筆商品）", n)] = struct {
			sql  string
			args []any
		}{fmt.Sprintf(`SELECT p.product_id FROM public.plan_products_%d p
			WHERE EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamptz) e WHERE e.store_id = p.store_id)`, n), []any{f.now}}
	}
	for name, q := range queries {
		var plan []byte
		must(t, conn.QueryRow(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+q.sql, q.args...).Scan(&plan))
		scans := measureScans(t, plan)
		if len(scans) == 0 {
			t.Errorf("%s：計畫沒有讀 measures：\n%s", name, plan)
			continue
		}
		for _, s := range scans {
			cond := strings.Join(indexConds(s), " ")
			probed := strings.Contains(cond, "account_id = ")
			loops, _ := s["Actual Loops"].(float64)
			// 以帳號探測（measures_enforced_suspension 上 account_id 為 Index Cond），或整個生效集合只讀一次（外層很大時 hash anti join
			// 合理）；不能是沒有帳號條件的掃描被重複執行（UNION ALL 時每個探測重掃全平台的生效中措施）。
			if !probed && loops > 1 {
				t.Errorf("%s：讀 measures 沒有以外層的 account_id 為 Index Cond，卻執行了 %v 次（%v，Index Cond %q）：\n%s",
					name, loops, s["Node Type"], cond, plan)
			}
			// 外層小（49 筆商品、Classify 的 100 家）時一定是以帳號探測。
			if !strings.Contains(name, "2000") && (!probed || !strings.Contains(cond, "measures_enforced_suspension")) {
				t.Errorf("%s：讀 measures 要是 measures_enforced_suspension 上以外層的 account_id 為 Index Cond 的探測，實際是 %v（%q）：\n%s",
					name, s["Node Type"], cond, plan)
			}
		}
	}
}

// indexConds 回傳節點與它底下的 Bitmap Index Scan 的「索引名稱: Index Cond」。
func indexConds(n map[string]any) []string {
	var out []string
	if c, ok := n["Index Cond"].(string); ok {
		idx, _ := n["Index Name"].(string)
		out = append(out, idx+": "+c)
	}
	children, _ := n["Plans"].([]any)
	for _, c := range children {
		if m, ok := c.(map[string]any); ok && m["Relation Name"] == nil {
			out = append(out, indexConds(m)...)
		}
	}
	return out
}
