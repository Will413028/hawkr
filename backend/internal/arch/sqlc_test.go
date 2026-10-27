package arch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 每個模組的 sqlc 只讀自己的 schema.sql（與 objects.sql）加依賴表允許的模組的 pub.sql；每一份 pub.sql 只宣告自己的 pub_*。
func TestSQLCReadsOnlyItsOwnSchemaAndAllowedPubContracts(t *testing.T) {
	root := moduleRoot(t)
	configs, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", "db", "sqlc.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// 掃到零份也會「全部通過」：至少要看得到 seller 的。
	if len(configs) < 2 {
		t.Fatalf("只找到 %d 份 sqlc.yaml，檢查範圍不對", len(configs))
	}
	for _, path := range configs {
		module := filepath.Base(filepath.Dir(filepath.Dir(path)))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = os.Stat(filepath.Join(filepath.Dir(path), "objects.sql"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		hasObjects := err == nil
		_, err = os.Stat(filepath.Join(filepath.Dir(path), "schema.sql"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		for _, v := range CheckSQLCConfig(module, raw, Dependencies, err == nil, hasObjects) {
			t.Error(v)
		}
	}
	pubs, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", "db", "pub.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pubs) == 0 {
		t.Fatal("沒有任何 pub.sql（seller 發布 pub_exposable_stores），檢查範圍不對")
	}
	for _, path := range pubs {
		module := filepath.Base(filepath.Dir(filepath.Dir(path)))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range CheckPubSQL(module, string(raw)) {
			t.Error(v)
		}
		shapes := PubShapes(module, string(raw))
		if len(shapes) == 0 {
			t.Fatalf("%s 的 pub.sql 沒有宣告成表的形狀，形狀參照的檢查範圍不對", module)
		}
		queries, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", "db", "queries", "*.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if len(queries) == 0 {
			t.Fatal("沒有任何 query 檔，檢查範圍不對")
		}
		for _, q := range queries {
			consumer := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(q))))
			if consumer == module {
				continue
			}
			content, err := os.ReadFile(q)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range CheckPubShapeUse(consumer, filepath.Base(q), string(content), shapes) {
				t.Error(v)
			}
		}
	}
}

// 消費端的 query 只能呼叫 pub_* 函式，不能直接參照 pub.sql 為了 sqlc 宣告成表的形狀。
func TestCheckPubShapeUseFlagsDirectReferences(t *testing.T) {
	pub := "CREATE SCHEMA seller;\nCREATE TABLE seller.pub_exposable_store (store_id BIGINT NOT NULL);\n" +
		"CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store LANGUAGE sql STABLE AS $$ SELECT * FROM seller.pub_exposable_store $$;\n"
	shapes := PubShapes("seller", pub)
	if len(shapes) != 1 || shapes[0] != "seller.pub_exposable_store" {
		t.Fatalf("PubShapes = %v", shapes)
	}
	for name, q := range map[string]string{
		"FROM 形狀":   "SELECT store_id FROM seller.pub_exposable_store;",
		"JOIN 形狀":   "SELECT 1 FROM social.posts p JOIN seller.pub_exposable_store e ON e.store_id = p.store_id;",
		"INSERT 形狀": "INSERT INTO seller.pub_exposable_store VALUES (1);",
		"大寫":        "SELECT 1 FROM SELLER.PUB_EXPOSABLE_STORE;",
		"引號識別字":     `SELECT 1 FROM "seller"."pub_exposable_store";`,
		"引號與點的空白":   `SELECT 1 FROM seller . "pub_exposable_store";`,
		"區塊註解後":     "SELECT 1 FROM /* 業者 */ seller.pub_exposable_store;",
		"區塊註解夾在點旁":  "SELECT 1 FROM seller/* x */.pub_exposable_store;",
	} {
		if got := CheckPubShapeUse("social", "q.sql", q, shapes); !anyContains(got, "直接參照 seller.pub_exposable_store") {
			t.Errorf("%s：預期被擋下，實際：%v", name, got)
		}
	}
	ok := "-- 註解裡的 seller.pub_exposable_store 不算\n/* 區塊註解裡的 seller.pub_exposable_store /* 巢狀 */ 也不算 */\nSELECT p.id FROM social.posts p\n" +
		"WHERE EXISTS (SELECT 1 FROM seller.pub_exposable_stores(sqlc.arg(now)::timestamptz) e WHERE e.store_id = p.store_id);"
	if got := CheckPubShapeUse("social", "q.sql", ok, shapes); len(got) != 0 {
		t.Fatalf("呼叫函式被判為違規：%v", got)
	}
}

func TestCheckSQLCConfigFlagsForbiddenSchemas(t *testing.T) {
	deps := map[string][]string{"seller": {}, "social": {"seller"}, "buyer": {}}
	cases := []struct {
		name, module, config, want string
		objects                    bool
	}{
		{"讀別的模組的 schema.sql（內部表）", "social", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../seller/db/schema.sql\"]\n", "不得讀別的模組的內部表", false},
		{"依賴表不允許", "social", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../buyer/db/pub.sql\"]\n", "依賴表不允許 social 依賴 buyer", false},
		{"下層讀上層", "seller", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../social/db/pub.sql\"]\n", "依賴表不允許 seller 依賴 social", false},
		{"讀自己的 pub.sql", "seller", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../seller/db/pub.sql\"]\n", "列了自己的 pub.sql", false},
		{"沒有自己的 schema.sql", "social", "version: \"2\"\nsql:\n  - schema: \"../../seller/db/pub.sql\"\n", "正好列一次自己的 schema.sql", false},
		{"重複", "social", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"schema.sql\"]\n", "重複列出", false},
		{"目錄", "social", "version: \"2\"\nsql:\n  - schema: \"../../seller/db/\"\n", "只能是自己的 schema.sql", false},
		{"沒有 schema", "social", "version: \"2\"\nsql:\n  - queries: \"queries/\"\n", "沒有 schema", false},
		{"讀別的模組的 objects.sql", "social", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../seller/db/objects.sql\"]\n", "不得讀別的模組的內部表", false},
		{"有 objects.sql 沒列", "seller", "version: \"2\"\nsql:\n  - schema: \"schema.sql\"\n", "沒有列 objects.sql", true},
		{"沒有 objects.sql 卻列", "seller", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"objects.sql\"]\n", "模組沒有 db/objects.sql", false},
		{"objects.sql 在 schema.sql 之前", "seller", "version: \"2\"\nsql:\n  - schema: [\"objects.sql\", \"schema.sql\"]\n", "緊接在 schema.sql 之後", true},
		{"objects.sql 重複", "seller", "version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"objects.sql\", \"objects.sql\"]\n", "重複列出", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CheckSQLCConfig(c.module, []byte(c.config), deps, true, c.objects); !anyContains(got, c.want) {
				t.Fatalf("預期出現「%s」，實際：%v", c.want, got)
			}
		})
	}
	// 沒有自己資料的模組（沒有 db/schema.sql）：列了 schema.sql 或一份 pub.sql 都沒讀是違規，只讀依賴表允許的 pub.sql 才通過。
	for name, tc := range map[string]struct{ config, want string }{
		"沒有 schema.sql 卻列了":         {"version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../seller/db/pub.sql\"]\n", "模組沒有 db/schema.sql"},
		"沒有 schema.sql 也沒有 pub.sql": {"version: \"2\"\nsql:\n  - schema: []\n", "至少要讀一份依賴表允許的 pub.sql"},
	} {
		if got := CheckSQLCConfig("social", []byte(tc.config), deps, false, false); !anyContains(got, tc.want) {
			t.Errorf("%s：預期出現「%s」，實際：%v", name, tc.want, got)
		}
	}
	if got := CheckSQLCConfig("social", []byte("version: \"2\"\nsql:\n  - schema: [\"../../seller/db/pub.sql\"]\n"), deps, false, false); len(got) != 0 {
		t.Fatalf("沒有 schema.sql、只讀允許的 pub.sql 被判為違規：%v", got)
	}
	for _, ok := range []string{
		"version: \"2\"\nsql:\n  - schema: \"schema.sql\"\n",
		"version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"../../seller/db/pub.sql\"]\n",
		"version: \"2\"\nsql:\n  - schema: [\"schema.sql\", \"objects.sql\", \"../../seller/db/pub.sql\"]\n",
	} {
		if got := CheckSQLCConfig("social", []byte(ok), deps, true, strings.Contains(ok, "objects.sql")); len(got) != 0 {
			t.Fatalf("合法的設定被判為違規：%v\n%s", got, ok)
		}
	}
}

func TestCheckPubSQLFlagsAnythingButOwnPubShapes(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"內部表", "CREATE SCHEMA seller;\nCREATE TABLE seller.stores (store_id BIGINT);", "名字必須是 seller.pub_*"},
		{"別的 schema 的 pub", "CREATE SCHEMA seller;\nCREATE TABLE buyer.pub_x (id BIGINT);", "名字必須是 seller.pub_*"},
		{"別的 schema", "CREATE SCHEMA buyer;\nCREATE TABLE seller.pub_x (id BIGINT);", "建立了別的 schema buyer"},
		{"函式回傳 RETURNS TABLE", "CREATE SCHEMA seller;\nCREATE FUNCTION seller.pub_f(at TIMESTAMPTZ) RETURNS TABLE(store_id BIGINT) LANGUAGE sql AS $$ SELECT 1 $$;", "RETURNS SETOF seller.pub_*"},
		{"函式回傳內部表", "CREATE SCHEMA seller;\nCREATE FUNCTION seller.pub_f(at TIMESTAMPTZ) RETURNS SETOF seller.stores LANGUAGE sql AS $$ SELECT * FROM seller.stores $$;", "RETURNS SETOF seller.pub_*"},
		{"其他語句", "CREATE SCHEMA seller;\nCREATE TABLE seller.pub_x (id BIGINT);\nINSERT INTO seller.pub_x VALUES (1);", "不能有：INSERT"},
		{"CREATE INDEX", "CREATE SCHEMA seller;\nCREATE TABLE seller.pub_x (id BIGINT);\nCREATE INDEX pub_x_id ON seller.pub_x (id);", "不能有：CREATE INDEX"},
		{"沒有物件", "-- 空\nCREATE SCHEMA seller;", "沒有任何 pub_* 物件"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CheckPubSQL("seller", c.content); !anyContains(got, c.want) {
				t.Fatalf("預期出現「%s」，實際：%v", c.want, got)
			}
		})
	}
	ok := "-- 註解裡的 CREATE TABLE seller.stores 不算；\nCREATE SCHEMA seller;\nCREATE TABLE seller.pub_x (id BIGINT NOT NULL);\n" +
		"CREATE FUNCTION seller.pub_xs(at TIMESTAMPTZ) RETURNS SETOF seller.pub_x LANGUAGE sql STABLE AS $$ SELECT * FROM seller.pub_x; $$;\n"
	if got := CheckPubSQL("seller", ok); len(got) != 0 {
		t.Fatalf("合法的 pub.sql 被判為違規：%v", got)
	}
}

// 每個模組的 db/schema.sql 只有表、約束與索引（Atlas 只讀它），view、函式與型別在 db/objects.sql（單體規則 6）。
func TestDesiredStateIsSplitBetweenSchemaAndObjects(t *testing.T) {
	root := moduleRoot(t)
	var files []string
	for _, name := range []string{"schema.sql", "objects.sql"} {
		m, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", "db", name))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	objects := 0
	for _, path := range files {
		module := filepath.Base(filepath.Dir(filepath.Dir(path)))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(path) == "objects.sql" {
			objects++
		}
		for _, v := range CheckDesiredStateSplit(module, filepath.Base(path), string(raw)) {
			t.Error(v)
		}
	}
	if objects == 0 {
		t.Fatal("沒有任何 objects.sql（seller 的曝光規則在那裡），檢查範圍不對")
	}
}

func TestCheckDesiredStateSplitFlagsObjectsInTheWrongFile(t *testing.T) {
	fn := "CREATE FUNCTION s.f(at TIMESTAMPTZ) RETURNS SETOF s.r LANGUAGE sql STABLE BEGIN ATOMIC SELECT CASE WHEN true THEN 1 END; SELECT 2; END;"
	for name, c := range map[string]struct{ file, sql, want string }{
		"schema.sql 的 view":         {"schema.sql", "CREATE TABLE s.t (id BIGINT);\nCREATE VIEW s.v AS SELECT id FROM s.t;", "VIEW"},
		"schema.sql 的函式":            {"schema.sql", fn, "FUNCTION"},
		"schema.sql 的 OR REPLACE":   {"schema.sql", "CREATE OR REPLACE VIEW s.v AS SELECT 1;", "VIEW"},
		"schema.sql 的複合型別":          {"schema.sql", "CREATE TYPE s.r AS (id BIGINT);", "TYPE"},
		"schema.sql 的 materialized": {"schema.sql", "CREATE MATERIALIZED VIEW s.v AS SELECT 1;", "VIEW"},
		"objects.sql 的表":            {"objects.sql", "CREATE TYPE s.r AS (id BIGINT);\nCREATE TABLE s.t (id BIGINT);", "CREATE TABLE s.t"},
		"objects.sql 的索引":           {"objects.sql", fn + "\nCREATE INDEX t_id ON s.t (id);", "CREATE INDEX t_id"},
		"objects.sql 的 ALTER":       {"objects.sql", "ALTER TABLE s.t ADD COLUMN v TEXT;", "ALTER TABLE"},
		"objects.sql 的 enum":        {"objects.sql", "CREATE TYPE s.state AS ENUM ('a', 'b');", "CREATE TYPE s.state AS ENUM"},
	} {
		if got := CheckDesiredStateSplit("s", c.file, c.sql); !anyContains(got, c.want) {
			t.Errorf("%s：預期出現「%s」，實際：%v", name, c.want, got)
		}
	}
	for name, c := range map[string]struct{ file, sql string }{
		"schema.sql 的表與索引":  {"schema.sql", "-- CREATE VIEW 在註解裡不算\nCREATE TABLE s.t (id BIGINT, v TEXT DEFAULT 'CREATE VIEW');\nCREATE INDEX t_id ON s.t (id);"},
		"objects.sql 的物件":   {"objects.sql", "CREATE VIEW s.v AS SELECT 1;\nCREATE TYPE s.r AS (id BIGINT);\n" + fn + "\nCOMMENT ON VIEW s.v IS '說明';"},
		"BEGIN ATOMIC 裡的分號": {"objects.sql", fn},
		"schema.sql 的 enum": {"schema.sql", "CREATE TYPE s.state AS ENUM ('a', 'b');\nCREATE TABLE s.t (id BIGINT, st s.state);"},
	} {
		if got := CheckDesiredStateSplit("s", c.file, c.sql); len(got) != 0 {
			t.Errorf("%s：不該被擋下：%v", name, got)
		}
	}
}
