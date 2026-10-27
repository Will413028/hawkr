//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/arch"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// trimmedColumn 是一個頭尾不得有空白的文字欄位（platform/trimtext）。extra 是讓同一張表別的 CHECK 在只填這一欄時成立的欄位值。
type trimmedColumn struct {
	table, column, constraint string
	extra                     map[string]any
}

// trimmedColumns 是每個模組裡頭尾不得有空白的文字欄位。TestTrimmedTextChecksUseTheCanonicalPattern 要求它與 migration 套用後
// 用 trimtext.Check 的 CHECK 雙向相同：新的欄位用了 trimtext.Check 卻沒登記在這裡，也會失敗（登記才有下面的 SQL 寫入測試）。
// hideTarget 讓 social.hides 的 hides_target_check（恰好一個目標）在只填一欄時成立；暫時表沒有外鍵。
const hideTarget = "00000000-0000-7000-8000-000000000001"

var trimmedColumns = []trimmedColumn{
	{table: "identity.admin_role_events", column: "reason", constraint: "admin_role_events_reason_check"},
	{table: "accountdeletion.request_attempts", column: "reason", constraint: "request_attempts_reason_check",
		extra: map[string]any{"requester": "ADMIN", "operator_admin_id": 1}},
	{table: "agent.vendor_portal_invites", column: "invited_email", constraint: "vendor_portal_invites_email_check"},
	{table: "seller.stores", column: "store_name", constraint: "stores_store_name_check"},
	{table: "seller.stores", column: "contact_name", constraint: "stores_contact_name_check", extra: map[string]any{"store_name": "店"}},
	{table: "seller.food_registrations", column: "registration_number", constraint: "food_registrations_number_check"},
	{table: "seller.kyc_applications", column: "real_name", constraint: "kyc_applications_real_name_check"},
	{table: "seller.prospects", column: "display_name", constraint: "prospects_display_name_check"},
	{table: "seller.prospect_contacts", column: "value", constraint: "prospect_contacts_value_check"},
	{table: "social.hides", column: "reason_text", constraint: "hides_reason_text_check", extra: map[string]any{"comment_id": hideTarget}},
	{table: "social.hides", column: "source_ref", constraint: "hides_source_ref_check", extra: map[string]any{"comment_id": hideTarget}},
	{table: "social.hides", column: "restore_reason", constraint: "hides_restore_reason_check",
		extra: map[string]any{"comment_id": hideTarget, "restored_at": time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), "restored_by": int64(1)}},
	{table: "social.sanctions", column: "reason_text", constraint: "sanctions_reason_text_check"},
	{table: "social.sanctions", column: "source_ref", constraint: "sanctions_source_ref_check"},
	{table: "social.posts", column: "body", constraint: "posts_body_check"},
	{table: "social.comments", column: "body", constraint: "comments_body_check"},
	{table: "social.forum_entries", column: "body", constraint: "forum_entries_body_check"},
}

// 「頭尾沒有空白的文字」在整個單體只有 platform/trimtext 一個定義（DB 的 CHECK 與寫入端的 Trim 同一個集合）。對 migration 套用後
// 的每個模組：
//   - 每個 CHECK 寫到空白的地方只能是 trimtext.Check 的展開（internal/arch 的 CheckTrimmedText 對 pg_get_constraintdef 的寫法：
//     不得用去空白的 trim 函式、比對樣式裡的空白、正則空白類別，不得另列空白碼位），用到它的欄位與 trimmedColumns 雙向相同；
//     索引定義同樣檢查；
//   - 每個登記的欄位，以 SQL 直接寫入只有空白、頭或尾有空白的值都被它的 CHECK 擋下，中間有空白的值通過。寫入的是
//     `LIKE … INCLUDING CONSTRAINTS` 複製的暫時表（同名、同定義的 CHECK，沒有外鍵與 NOT NULL），只填這一欄，其他 CHECK 不受影響。
func TestTrimmedTextChecksUseTheCanonicalPattern(t *testing.T) {
	ctx := context.Background()
	d := pg.NewDB(t)
	mods := modules()
	if err := migrate.Apply(ctx, d, mods); err != nil {
		t.Fatal(err)
	}
	schemas := make([]string, 0, len(mods))
	for _, m := range mods {
		schemas = append(schemas, m.Name)
	}
	canonical := regexp.MustCompile(`\b([a-z_][a-z0-9_]*) !~ '` + regexp.QuoteMeta(trimtext.Pattern) + `'::text`)

	rows, err := d.Pool().Query(ctx, `
		SELECT c.conrelid::regclass::text, c.conname, pg_get_constraintdef(c.oid)
		  FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace
		 WHERE c.contype = 'c' AND n.nspname = ANY($1) ORDER BY 1, 2`, schemas)
	if err != nil {
		t.Fatal(err)
	}
	type check struct{ Table, Name, Def string }
	checks, err := pgx.CollectRows(rows, pgx.RowToStructByPos[check])
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) < 100 {
		t.Fatalf("只讀到 %d 個 CHECK，檢查範圍不對", len(checks))
	}
	found := map[string]string{}
	for _, c := range checks {
		for _, m := range canonical.FindAllStringSubmatch(c.Def, -1) {
			found[c.Table+"."+m[1]] = c.Name
		}
		rest := canonical.ReplaceAllString(c.Def, "")
		for _, v := range arch.CheckTrimmedText(c.Table, c.Name, c.Def) {
			t.Errorf("%s：%s", v, c.Def)
		}
		if r, ok := spaceOutsidePattern(rest); ok {
			t.Errorf("%s 的 %s 另外列了空白碼位 %U，頭尾空白只能用 trimtext.Check：%s", c.Table, c.Name, r, c.Def)
		}
	}
	want := map[string]string{}
	for _, col := range trimmedColumns {
		want[col.table+"."+col.column] = col.constraint
	}
	if !maps.Equal(found, want) {
		t.Errorf("用 trimtext.Check 的欄位與 trimmedColumns 不同：\nDB：%v\n登記：%v", found, want)
	}

	rows, err = d.Pool().Query(ctx, `SELECT schemaname || '.' || indexname, indexdef FROM pg_indexes WHERE schemaname = ANY($1) ORDER BY 1`, schemas)
	if err != nil {
		t.Fatal(err)
	}
	type index struct{ Name, Def string }
	indexes, err := pgx.CollectRows(rows, pgx.RowToStructByPos[index])
	if err != nil {
		t.Fatal(err)
	}
	for _, ix := range indexes {
		for _, v := range arch.CheckTrimmedText("索引", ix.Name, ix.Def) {
			t.Errorf("%s：%s", v, ix.Def)
		}
	}

	conn, err := d.Pool().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var spaces []rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) {
			spaces = append(spaces, r)
		}
	}
	for i, col := range trimmedColumns {
		t.Run(col.table+"."+col.column, func(t *testing.T) {
			tmp := fmt.Sprintf("trimmed_%d", i)
			if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE TEMP TABLE %s (LIKE %s INCLUDING CONSTRAINTS)", tmp, col.table)); err != nil {
				t.Fatal(err)
			}
			var same bool
			if err := conn.QueryRow(ctx, `
				SELECT pg_get_constraintdef(a.oid) = pg_get_constraintdef(b.oid)
				  FROM pg_constraint a, pg_constraint b
				 WHERE a.conrelid = $1::regclass AND b.conrelid = $2::regclass AND a.conname = $3 AND b.conname = $3`,
				col.table, tmp, col.constraint).Scan(&same); err != nil || !same {
				t.Fatalf("暫時表沒有與 %s 相同的 %s（err = %v）", col.table, col.constraint, err)
			}
			if _, err := conn.Exec(ctx, fmt.Sprintf(`DO $$ DECLARE c text; BEGIN
				FOR c IN SELECT attname FROM pg_attribute WHERE attrelid = '%s'::regclass AND attnum > 0 AND attnotnull LOOP
					EXECUTE format('ALTER TABLE %s ALTER COLUMN %%I DROP NOT NULL', c);
				END LOOP; END $$`, tmp, tmp)); err != nil {
				t.Fatal(err)
			}
			cols := append([]string{col.column}, slices.Sorted(maps.Keys(col.extra))...)
			insert := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tmp, strings.Join(cols, ", "), placeholders(len(cols)))
			args := func(v string) []any {
				out := []any{v}
				for _, k := range cols[1:] {
					out = append(out, col.extra[k])
				}
				return out
			}
			for _, r := range spaces {
				s := string(r)
				for _, v := range []string{s, s + s, s + "字", "字" + s} {
					_, err := conn.Exec(ctx, insert, args(v)...)
					var pgErr *pgconn.PgError
					if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != col.constraint {
						t.Errorf("%+q：要被 %s 擋下，得到 %v", v, col.constraint, err)
					}
				}
				if _, err := conn.Exec(ctx, insert, args("字"+s+"字")...); err != nil {
					t.Errorf("%+q：中間的空白要通過：%v", "字"+s+"字", err)
				}
			}
		})
	}
}

// spaceOutsidePattern 回報 def（已去掉 trimtext.Check 的展開）裡第一個空白碼位的 \uXXXX 寫法或字元本身（U+0020 與換行除外：SQL 本身就用）。
func spaceOutsidePattern(def string) (rune, bool) {
	for _, m := range regexp.MustCompile(`\\u([0-9a-fA-F]{4})`).FindAllStringSubmatch(def, -1) {
		var r rune
		if _, err := fmt.Sscanf(m[1], "%x", &r); err == nil && unicode.IsSpace(r) {
			return r, true
		}
	}
	for _, r := range def {
		if unicode.IsSpace(r) && r != ' ' && r != '\n' {
			return r, true
		}
	}
	return 0, false
}

func placeholders(n int) string {
	ps := make([]string, n)
	for i := range ps {
		ps[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(ps, ", ")
}
