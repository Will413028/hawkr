// Package migrate 在部署時套用各模組的 schema migration（ADR D3）。
//
// migration 檔 embed 進 binary，所以部署的 image 永遠只套用它自己帶的 migration，
// 不會出現「部署的程式」與「套用的 migration」不是同一個版本。檔案在開發時由 Atlas
// 產生（atlas migrate diff），這裡負責驗證 atlas.sum 與套用，執行期不需要 Atlas CLI。
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Module 是一個模組的 migration：Dir 的根放 *.sql 與 atlas.sum，Name 同時是它的 schema 名。
type Module struct {
	Name string
	Dir  fs.FS
}

type file struct {
	name    string
	version string
	body    []byte
}

func (f file) checksum() string {
	s := sha256.Sum256(f.body)
	return hex.EncodeToString(s[:])
}

// lockKey 是 migrate 的 advisory lock：同一時間只有一個 migrate 在跑（例如兩個 pod 同時啟動）。
const lockKey = 7_236_915_402

// Apply 依序套用 mods 裡尚未套用的 migration。任何模組驗證失敗時，什麼都不套用。
func Apply(ctx context.Context, d *db.DB, mods []Module) error {
	loaded := make([][]file, len(mods))
	for i, m := range mods {
		files, err := load(m)
		if err != nil {
			return err
		}
		loaded[i] = files
	}

	conn, err := d.Pool().Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate：取得連線：%w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("migrate：取得 advisory lock：%w", err)
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockKey) }()

	if _, err := conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS platform;
		CREATE TABLE IF NOT EXISTS platform.schema_migrations (
			module     text        NOT NULL,
			version    text        NOT NULL,
			name       text        NOT NULL,
			checksum   text        NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (module, version)
		)`); err != nil {
		return fmt.Errorf("migrate：建立紀錄表：%w", err)
	}

	for i, m := range mods {
		if err := applyModule(ctx, conn, m.Name, loaded[i]); err != nil {
			return err
		}
	}
	return nil
}

func applyModule(ctx context.Context, conn *pgxpool.Conn, module string, files []file) error {
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM platform.schema_migrations WHERE module = $1", module)
	if err != nil {
		return fmt.Errorf("migrate：讀取 %s 的紀錄：%w", module, err)
	}
	applied, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) ([2]string, error) {
		var v [2]string
		return v, r.Scan(&v[0], &v[1])
	})
	if err != nil {
		return fmt.Errorf("migrate：讀取 %s 的紀錄：%w", module, err)
	}

	byVersion := make(map[string]file, len(files))
	for _, f := range files {
		byVersion[f.version] = f
	}
	done := make(map[string]bool, len(applied))
	latest := ""
	for _, a := range applied {
		version, sum := a[0], a[1]
		f, ok := byVersion[version]
		if !ok {
			return fmt.Errorf("migrate：%s 已套用的版本 %s 不在 migration 目錄裡（檔案被刪除或改名）", module, version)
		}
		if f.checksum() != sum {
			return fmt.Errorf("migrate：%s 已套用的 %s 內容被修改；已套用的 migration 不得修改，請新增一個 migration", module, f.name)
		}
		done[version] = true
		latest = max(latest, version)
	}

	for _, f := range files {
		if done[f.version] {
			continue
		}
		if f.version < latest {
			return fmt.Errorf("migrate：%s 的 %s 版本早於已套用的 %s；請重新產生 migration，讓版本排在最後", module, f.name, latest)
		}
		if err := applyFile(ctx, conn, module, f); err != nil {
			return err
		}
	}
	return nil
}

func applyFile(ctx context.Context, conn *pgxpool.Conn, module string, f file) (err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate：%s/%s：開始 transaction：%w", module, f.name, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	// 沒有參數的 Exec 走 simple protocol，一個檔案裡可以有多個 statement。
	if _, err = tx.Exec(ctx, string(f.body)); err != nil {
		return fmt.Errorf("migrate：%s/%s：%w", module, f.name, err)
	}
	if _, err = tx.Exec(ctx,
		"INSERT INTO platform.schema_migrations (module, version, name, checksum) VALUES ($1, $2, $3, $4)",
		module, f.version, f.name, f.checksum()); err != nil {
		return fmt.Errorf("migrate：%s/%s：寫入紀錄：%w", module, f.name, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate：%s/%s：commit：%w", module, f.name, err)
	}
	return nil
}

// load 讀出模組的 migration 檔，並驗證檔名、directive 與 atlas.sum。
func load(m Module) ([]file, error) {
	entries, err := fs.ReadDir(m.Dir, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate：讀取 %s 的 migration 目錄：%w", m.Name, err)
	}
	var files []file
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".sql" {
			continue
		}
		version, _, _ := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		if version == "" || strings.Trim(version, "0123456789") != "" {
			return nil, fmt.Errorf("migrate：%s 的 %s 檔名不是 <版本>_<名稱>.sql", m.Name, e.Name())
		}
		body, err := fs.ReadFile(m.Dir, e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrate：讀取 %s/%s：%w", m.Name, e.Name(), err)
		}
		if d := directive(body); d != "" {
			return nil, fmt.Errorf("migrate：%s/%s 使用了尚未支援的 Atlas directive %q", m.Name, e.Name(), d)
		}
		files = append(files, file{name: e.Name(), version: version, body: body})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	for i := 1; i < len(files); i++ {
		if files[i].version == files[i-1].version {
			return nil, fmt.Errorf("migrate：%s 有兩個版本相同的 migration：%s、%s", m.Name, files[i-1].name, files[i].name)
		}
	}

	want, err := fs.ReadFile(m.Dir, "atlas.sum")
	if err != nil {
		return nil, fmt.Errorf("migrate：讀取 %s 的 atlas.sum：%w", m.Name, err)
	}
	if got := sumFile(files); got != string(want) {
		return nil, fmt.Errorf("migrate：%s 的 atlas.sum 和 migration 檔不符；migration 檔被手動改過，或改完沒有跑 atlas migrate hash", m.Name)
	}
	return files, nil
}

// directive 回傳檔案開頭的 Atlas directive（例如 `-- atlas:txmode none`）。
// Atlas 只認檔案開頭、第一個 statement 之前的 directive，所以只檢查那一段。
func directive(body []byte) string {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "-- atlas:"):
			return line
		case line == "" || strings.HasPrefix(line, "--"):
			continue
		default:
			return ""
		}
	}
	return ""
}

// sumFile 以 Atlas 的演算法算出 atlas.sum 的內容：逐檔累加的 SHA-256，最後再對
// 「檔名＋逐檔雜湊」取一次總雜湊。已用 12 個服務由 Atlas 產生的 atlas.sum 逐位元比對過。
func sumFile(files []file) string {
	running := sha256.New()
	hashes := make([]string, len(files))
	for i, f := range files {
		running.Write([]byte(f.name))
		running.Write(f.body)
		hashes[i] = base64.StdEncoding.EncodeToString(running.Sum(nil))
	}
	total := sha256.New()
	for i, f := range files {
		total.Write([]byte(f.name))
		total.Write([]byte(hashes[i]))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "h1:%s\n", base64.StdEncoding.EncodeToString(total.Sum(nil)))
	for i, f := range files {
		fmt.Fprintf(&b, "%s h1:%s\n", f.name, hashes[i])
	}
	return b.String()
}

// ForeignObjects 列出 database 裡不屬於 module 自己 schema 的物件。
//
// 在只套用過這個模組 migration 的空 database 上呼叫：migration 若參照別的模組
// （例如跨 schema FK），套用時就會失敗；若把物件建在別的 schema（包括 public），
// 這裡會列出來。兩者合起來保證模組只碰自己的 schema（ADR D3）。
func ForeignObjects(ctx context.Context, d *db.DB, module string) ([]string, error) {
	const q = `
		SELECT 'schema ' || nspname
		  FROM pg_namespace
		 WHERE nspname NOT IN ('pg_catalog', 'information_schema', 'public', 'platform', $1)
		   AND nspname NOT LIKE 'pg\_%'
		UNION ALL
		SELECT 'relation ' || n.nspname || '.' || c.relname
		  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'public'
		UNION ALL
		SELECT 'function ' || n.nspname || '.' || p.proname
		  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE n.nspname = 'public'
		UNION ALL
		SELECT 'type ' || n.nspname || '.' || t.typname
		  FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		 WHERE n.nspname = 'public'
		   -- 表的 row type 與 array type 由 Postgres 自動建立，表本身已列在 relation 裡。
		   AND t.typrelid = 0 AND t.typcategory <> 'A'
		ORDER BY 1`
	rows, err := d.Conn(ctx).Query(ctx, q, module)
	if err != nil {
		return nil, fmt.Errorf("migrate：檢查 %s 的物件：%w", module, err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("migrate：檢查 %s 的物件：%w", module, err)
	}
	return out, nil
}

// DescribeSchema 以文字列出 schema 裡的物件定義，順序固定：表與複合型別的欄位（型別、NOT NULL、預設值、
// identity、generated、collation、註解）、約束、索引、sequence、view、函式、enum 與 domain、trigger。
//
// 用來比對兩個 database 的同一個 schema 是否相同：模組的 schema.sql 加 objects.sql（Atlas 的 desired
// state 加手寫的 view 與函式，也是 sqlc 讀的檔）與 migration 套用後的結果若不同，sqlc 生成的程式就是對著一份
// 不存在的 schema 寫的。
func DescribeSchema(ctx context.Context, d *db.DB, schema string) (string, error) {
	const q = `
		SELECT 'column ' || c.relname || '.' || a.attname || ' ' || format_type(a.atttypid, a.atttypmod)
		       || CASE WHEN a.attnotnull THEN ' not null' ELSE '' END
		       || CASE a.attidentity WHEN 'a' THEN ' identity always' WHEN 'd' THEN ' identity by default' ELSE '' END
		       || CASE a.attgenerated WHEN 's' THEN ' generated stored' WHEN 'v' THEN ' generated virtual' ELSE '' END
		       || COALESCE(' default ' || pg_get_expr(ad.adbin, ad.adrelid), '')
		       || CASE WHEN a.attcollation <> 0 AND a.attcollation <> t.typcollation
		               THEN ' collate ' || (SELECT collname FROM pg_collation WHERE oid = a.attcollation) ELSE '' END
		       || COALESCE(' comment ' || col_description(c.oid, a.attnum), '')
		  FROM pg_class c
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		  JOIN pg_type t ON t.oid = a.atttypid
		  LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
		 WHERE n.nspname = $1 AND c.relkind IN ('r', 'p', 'c')
		UNION ALL
		SELECT 'constraint ' || c.relname || '.' || con.conname || ' ' || pg_get_constraintdef(con.oid)
		  FROM pg_constraint con
		  JOIN pg_class c ON c.oid = con.conrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1
		UNION ALL
		SELECT 'index ' || pg_get_indexdef(i.indexrelid)
		  FROM pg_index i
		  JOIN pg_class c ON c.oid = i.indexrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1
		UNION ALL
		SELECT 'table ' || c.relname || COALESCE(' comment ' || obj_description(c.oid, 'pg_class'), '')
		  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		UNION ALL
		SELECT 'sequence ' || c.relname || ' ' || format_type(s.seqtypid, NULL)
		       || ' start ' || s.seqstart || ' increment ' || s.seqincrement
		  FROM pg_sequence s
		  JOIN pg_class c ON c.oid = s.seqrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1
		UNION ALL
		SELECT 'view ' || c.relname || ' ' || pg_get_viewdef(c.oid)
		  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1 AND c.relkind IN ('v', 'm')
		UNION ALL
		SELECT 'function ' || pg_get_functiondef(p.oid)
		  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE n.nspname = $1 AND p.prokind IN ('f', 'p')
		UNION ALL
		SELECT 'type ' || t.typname || ' ' ||
		       CASE t.typtype
		         WHEN 'e' THEN 'enum (' || (SELECT string_agg(quote_literal(e.enumlabel), ', ' ORDER BY e.enumsortorder)
		                                      FROM pg_enum e WHERE e.enumtypid = t.oid) || ')'
		         WHEN 'd' THEN 'domain ' || format_type(t.typbasetype, t.typtypmod)
		         ELSE t.typtype::text
		       END
		  FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		 WHERE n.nspname = $1 AND t.typtype IN ('e', 'd', 'c') AND t.typrelid = 0
		UNION ALL
		SELECT 'trigger ' || pg_get_triggerdef(tg.oid)
		  FROM pg_trigger tg
		  JOIN pg_class c ON c.oid = tg.tgrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1 AND NOT tg.tgisinternal
		ORDER BY 1`
	rows, err := d.Conn(ctx).Query(ctx, q, schema)
	if err != nil {
		return "", fmt.Errorf("migrate：描述 schema %s：%w", schema, err)
	}
	lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", fmt.Errorf("migrate：描述 schema %s：%w", schema, err)
	}
	return strings.Join(lines, "\n"), nil
}

// DescribePub 以文字列出 schema 裡發布的讀取介面（名字是 pub_* 的關聯與函式）的形狀，順序固定：關聯（表、view 或複合型別，
// 不分種類）的每個欄位的名稱與型別，函式的參數與回傳。不比 NOT NULL：複合型別的欄位沒有 NOT NULL，契約檔（db/pub.sql）以表宣告、
// 寫 NOT NULL 是擁有者的承諾。
//
// 用來比對擁有者的 db/pub.sql（套到空 database）與 migration 套用後的真實物件：兩邊的輸出相同，消費端的 sqlc 讀到的形狀就是真的。
func DescribePub(ctx context.Context, d *db.DB, schema string) (string, error) {
	const q = `
		SELECT 'relation ' || c.relname || ' column ' || a.attnum || ' ' || a.attname || ' ' || format_type(a.atttypid, a.atttypmod)
		  FROM pg_class c
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		 WHERE n.nspname = $1 AND c.relkind IN ('r', 'p', 'v', 'm', 'c') AND c.relname LIKE 'pub\_%'
		UNION ALL
		SELECT 'function ' || p.proname || '(' || pg_get_function_arguments(p.oid) || ') returns ' || pg_get_function_result(p.oid)
		  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE n.nspname = $1 AND p.proname LIKE 'pub\_%'
		ORDER BY 1`
	rows, err := d.Conn(ctx).Query(ctx, q, schema)
	if err != nil {
		return "", fmt.Errorf("migrate：描述 %s 的 pub_*：%w", schema, err)
	}
	lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", fmt.Errorf("migrate：描述 %s 的 pub_*：%w", schema, err)
	}
	return strings.Join(lines, "\n"), nil
}

// FunctionsBreakingRule6 列出 schema 裡違反單體規則 6 的函式與程序，每筆是「名稱：原因」：
//   - 不是以 BEGIN ATOMIC 寫的（BEGIN ATOMIC 的函式才有 Postgres 的依賴追蹤，底層欄位改型別或刪除時被拒絕；`$$` 字串的函式到下一次呼叫才失敗）；
//   - 回傳表、view 的整列（`RETURNS [SETOF] <表>`：表加欄位之後內聯時就失敗）。回傳型別要是 CREATE TYPE 的複合型別或純量。
//
// 屬於 extension 的函式不算（不是模組寫的）。觸發函式（RETURNS trigger）不受 BEGIN ATOMIC 限制：Postgres 只允許以 plpgsql 等程序語言
// 寫它，SQL 函式不能回傳 trigger；它由 CREATE TRIGGER 綁在表上，不被別的模組呼叫。
func FunctionsBreakingRule6(ctx context.Context, d *db.DB, schema string) ([]string, error) {
	rows, err := d.Conn(ctx).Query(ctx, `
		SELECT p.proname || '：' || CASE WHEN p.prosqlbody IS NULL THEN '不是 BEGIN ATOMIC' ELSE '回傳 ' || c.oid::regclass::text || ' 的整列' END
		  FROM pg_proc p
		  JOIN pg_namespace n ON n.oid = p.pronamespace
		  JOIN pg_type t ON t.oid = p.prorettype
		  LEFT JOIN pg_class c ON c.oid = t.typrelid
		 WHERE n.nspname = $1
		   AND NOT EXISTS (SELECT 1 FROM pg_depend dep WHERE dep.classid = 'pg_proc'::regclass AND dep.objid = p.oid AND dep.deptype = 'e')
		   AND ((p.prosqlbody IS NULL AND p.prorettype <> 'trigger'::regtype) OR c.relkind IN ('r', 'p', 'v', 'm', 'f'))
		 ORDER BY p.proname COLLATE "C"`, schema)
	if err != nil {
		return nil, fmt.Errorf("migrate：檢查 %s 的函式：%w", schema, err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("migrate：檢查 %s 的函式：%w", schema, err)
	}
	return out, nil
}

// ErrNoSchema 表示模組的 migration 沒有建立與模組同名的 schema。
var ErrNoSchema = errors.New("migrate：模組的 migration 必須建立與模組同名的 schema")

// HasOwnSchema 回報 database 裡是否有與 module 同名的 schema。
func HasOwnSchema(ctx context.Context, d *db.DB, module string) error {
	var exists bool
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", module).Scan(&exists); err != nil {
		return fmt.Errorf("migrate：檢查 %s 的 schema：%w", module, err)
	}
	if !exists {
		return ErrNoSchema
	}
	return nil
}
