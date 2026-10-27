//go:build integration

package migrate_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

const (
	v1 = "CREATE SCHEMA shop;\nCREATE TABLE shop.items (id bigint PRIMARY KEY);"
	v2 = "ALTER TABLE shop.items ADD COLUMN name text NOT NULL DEFAULT '';"
)

func appliedVersions(t *testing.T, d *db.DB, module string) []string {
	t.Helper()
	rows, err := d.Conn(context.Background()).Query(context.Background(),
		"SELECT version FROM platform.schema_migrations WHERE module = $1 ORDER BY version", module)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func tableExists(t *testing.T, d *db.DB, name string) bool {
	t.Helper()
	var ok bool
	if err := d.Conn(context.Background()).QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestApplyRunsPendingMigrationsOnceAndInOrder(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t)
	mod := migrate.TestModule("shop", map[string]string{"002_add_name.sql": v2, "001_init.sql": v1})
	for range 2 {
		if err := migrate.Apply(context.Background(), d, []migrate.Module{mod}); err != nil {
			t.Fatal(err)
		}
	}
	if got := appliedVersions(t, d, "shop"); !slices.Equal(got, []string{"001", "002"}) {
		t.Fatalf("已套用 = %v", got)
	}
	var col bool
	if err := d.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'shop' AND column_name = 'name')").Scan(&col); err != nil || !col {
		t.Fatalf("002 沒有套用（err = %v）", err)
	}
}

func TestApplyOnlyRunsNewMigrations(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t)
	if err := migrate.Apply(context.Background(), d, []migrate.Module{migrate.TestModule("shop", map[string]string{"001_init.sql": v1})}); err != nil {
		t.Fatal(err)
	}
	// 若 001 被重跑，CREATE SCHEMA shop 會因 schema 已存在而失敗。
	if err := migrate.Apply(context.Background(), d, []migrate.Module{migrate.TestModule("shop", map[string]string{"001_init.sql": v1, "002_add_name.sql": v2})}); err != nil {
		t.Fatal(err)
	}
	if got := appliedVersions(t, d, "shop"); !slices.Equal(got, []string{"001", "002"}) {
		t.Fatalf("已套用 = %v", got)
	}
}

func TestApplyRefusesToRewriteHistory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		first map[string]string
		then  map[string]string
		want  string
	}{
		{"已套用的檔案被修改",
			map[string]string{"001_init.sql": v1},
			map[string]string{"001_init.sql": v1 + "\n-- 改過"},
			"內容被修改"},
		{"已套用的檔案被刪除",
			map[string]string{"001_init.sql": v1, "002_add_name.sql": v2},
			map[string]string{"001_init.sql": v1},
			"不在 migration 目錄裡"},
		{"新檔案的版本插在已套用的版本之前",
			map[string]string{"001_init.sql": v1, "003_add_name.sql": v2},
			map[string]string{"001_init.sql": v1, "002_late.sql": "SELECT 1;", "003_add_name.sql": v2},
			"版本早於"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := pg.NewDB(t)
			if err := migrate.Apply(context.Background(), d, []migrate.Module{migrate.TestModule("shop", c.first)}); err != nil {
				t.Fatal(err)
			}
			err := migrate.Apply(context.Background(), d, []migrate.Module{migrate.TestModule("shop", c.then)})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v，預期包含「%s」", err, c.want)
			}
		})
	}
}

func TestFailedMigrationLeavesNothingBehind(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t)
	bad := migrate.TestModule("shop", map[string]string{"001_init.sql": v1 + "\nCREATE TABLE shop.broken (id no_such_type);"})
	if err := migrate.Apply(context.Background(), d, []migrate.Module{bad}); err == nil {
		t.Fatal("預期失敗")
	}
	if tableExists(t, d, "shop.items") {
		t.Fatal("失敗的 migration 留下了 shop.items；整個檔案應該一起 rollback")
	}
	if got := appliedVersions(t, d, "shop"); len(got) != 0 {
		t.Fatalf("失敗的 migration 被記為已套用：%v", got)
	}
}

func TestConcurrentApplyRunsEachMigrationOnce(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t)
	mod := migrate.TestModule("shop", map[string]string{"001_init.sql": v1, "002_add_name.sql": v2})
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = migrate.Apply(context.Background(), d, []migrate.Module{mod})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("第 %d 個 Apply：%v", i, err)
		}
	}
	if got := appliedVersions(t, d, "shop"); !slices.Equal(got, []string{"001", "002"}) {
		t.Fatalf("已套用 = %v", got)
	}
}

func TestIsolationChecksCatchModulesThatLeaveTheirSchema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		sql     string
		wantErr bool     // 套用本身就會失敗
		foreign []string // 套用成功，但留下不屬於自己的物件
		noOwn   bool     // 沒有建立自己的 schema
	}{
		{name: "只碰自己的 schema", sql: v1},
		{name: "把表建在 public", sql: "CREATE SCHEMA shop;\nCREATE TABLE public.leak (id int);", foreign: []string{"relation public.leak"}},
		{name: "把 type 建在 public", sql: "CREATE SCHEMA shop;\nCREATE TYPE public.mood AS ENUM ('ok');", foreign: []string{"type public.mood"}},
		{name: "建立別的模組的 schema", sql: "CREATE SCHEMA shop;\nCREATE SCHEMA other;", foreign: []string{"schema other"}},
		{name: "跨 schema 的 FK", sql: "CREATE SCHEMA shop;\nCREATE TABLE shop.orders (seller_id bigint REFERENCES seller.stores (id));", wantErr: true},
		{name: "沒有建立自己的 schema", sql: "SELECT 1;", noOwn: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := pg.NewDB(t)
			err := migrate.Apply(context.Background(), d, []migrate.Module{migrate.TestModule("shop", map[string]string{"001_init.sql": c.sql})})
			if c.wantErr {
				if err == nil {
					t.Fatal("預期套用失敗")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := migrate.ForeignObjects(context.Background(), d, "shop")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, c.foreign) {
				t.Errorf("ForeignObjects = %v，預期 %v", got, c.foreign)
			}
			if err := migrate.HasOwnSchema(context.Background(), d, "shop"); (err != nil) != c.noOwn {
				t.Errorf("HasOwnSchema = %v，noOwn = %v", err, c.noOwn)
			}
		})
	}
}

func TestDescribeSchemaSeesEveryKindOfDifference(t *testing.T) {
	t.Parallel()
	const base = `
		CREATE SCHEMA shop;
		CREATE TYPE shop.mood AS ENUM ('ok', 'bad');
		CREATE TABLE shop.items (
		    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		    name text NOT NULL DEFAULT '' CHECK (name <> 'x'),
		    mood shop.mood
		);
		CREATE INDEX items_name ON shop.items (name);
		CREATE VIEW shop.named AS SELECT id FROM shop.items WHERE name <> '';
		CREATE FUNCTION shop.touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
		CREATE TRIGGER items_touch BEFORE UPDATE ON shop.items FOR EACH ROW EXECUTE FUNCTION shop.touch();`
	describe := func(sql string) string {
		t.Helper()
		d := pg.NewDB(t)
		if _, err := d.Pool().Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
		out, err := migrate.DescribeSchema(context.Background(), d, "shop")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	want := describe(base)
	if got := describe(base); got != want {
		t.Fatalf("同一份 DDL 描述不同：\n%s\n---\n%s", want, got)
	}
	variants := map[string][2]string{
		"enum 的值":   {"('ok', 'bad')", "('ok', 'meh')"},
		"identity":  {"GENERATED ALWAYS AS IDENTITY", "GENERATED BY DEFAULT AS IDENTITY"},
		"預設值":       {"DEFAULT ''", "DEFAULT '-'"},
		"CHECK":     {"name <> 'x'", "name <> 'y'"},
		"索引":        {"(name)", "(name DESC)"},
		"view":      {"WHERE name <> ''", "WHERE name IS NOT NULL"},
		"函式":        {"RETURN NEW;", "RETURN OLD;"},
		"trigger":   {"BEFORE UPDATE", "BEFORE INSERT"},
		"collation": {"name text NOT NULL", `name text COLLATE "C" NOT NULL`},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			changed := strings.Replace(base, v[0], v[1], 1)
			if changed == base {
				t.Fatalf("測試寫錯：%q 不在 DDL 裡", v[0])
			}
			if describe(changed) == want {
				t.Errorf("DescribeSchema 沒有發現 %s 的差異", name)
			}
		})
	}
}

// 單體規則 6 的函式檢查：`$$` 字串的函式（SQL 與 plpgsql）、回傳表或 view 整列的 BEGIN ATOMIC 函式（SETOF 與單列）都被列出；
// 回傳 CREATE TYPE 的複合型別與純量的 BEGIN ATOMIC 函式不列出；觸發函式（RETURNS trigger，只能是 plpgsql 等程序語言）不列出；
// 別的 schema 的函式不看。
func TestFunctionsBreakingRule6ListsLooseBodiesAndWholeRowReturns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	_, err := d.Pool().Exec(ctx, `
		CREATE SCHEMA shop;
		CREATE SCHEMA other;
		CREATE TABLE shop.items (id BIGINT PRIMARY KEY, name TEXT);
		CREATE VIEW shop.item_names AS SELECT id, name FROM shop.items;
		CREATE TYPE shop.item_ref AS (id BIGINT);
		CREATE FUNCTION shop.loose_sql() RETURNS BIGINT LANGUAGE sql STABLE AS $$ SELECT count(*) FROM shop.items $$;
		CREATE FUNCTION shop.loose_plpgsql() RETURNS BIGINT LANGUAGE plpgsql STABLE AS $$ BEGIN RETURN 1; END $$;
		CREATE FUNCTION shop.whole_rows() RETURNS SETOF shop.items LANGUAGE sql STABLE BEGIN ATOMIC SELECT * FROM shop.items; END;
		CREATE FUNCTION shop.whole_row() RETURNS shop.item_names LANGUAGE sql STABLE BEGIN ATOMIC SELECT * FROM shop.item_names LIMIT 1; END;
		CREATE FUNCTION shop.refs() RETURNS SETOF shop.item_ref LANGUAGE sql STABLE BEGIN ATOMIC SELECT id FROM shop.items; END;
		CREATE FUNCTION shop.total() RETURNS BIGINT LANGUAGE sql STABLE BEGIN ATOMIC SELECT count(*) FROM shop.items; END;
		CREATE FUNCTION shop.touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.name := lower(NEW.name); RETURN NEW; END $$;
		CREATE TRIGGER items_touch BEFORE INSERT ON shop.items FOR EACH ROW EXECUTE FUNCTION shop.touch();
		CREATE FUNCTION other.loose() RETURNS BIGINT LANGUAGE sql STABLE AS $$ SELECT 1 $$;`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := migrate.FunctionsBreakingRule6(ctx, d, "shop")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"loose_plpgsql：不是 BEGIN ATOMIC", "loose_sql：不是 BEGIN ATOMIC", "whole_row：回傳 shop.item_names 的整列", "whole_rows：回傳 shop.items 的整列"}
	if !slices.Equal(got, want) {
		t.Errorf("FunctionsBreakingRule6 = %q，要 %q", got, want)
	}
}
