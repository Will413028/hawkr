//go:build integration

package testdb_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// atlasModule 建一個帶正確 atlas.sum 的模組。sum 的算法同 Atlas（migrate 自己的測試逐位元比對過）；
// 這裡算錯的話 migrate.Apply 會拒絕，測試直接失敗，不會默默通過。
func atlasModule(name string, files map[string]string) migrate.Module {
	names := make([]string, 0, len(files))
	fsys := fstest.MapFS{}
	for n, body := range files {
		names = append(names, n)
		fsys[n] = &fstest.MapFile{Data: []byte(body)}
	}
	sort.Strings(names)
	running := sha256.New()
	hashes := make([]string, len(names))
	for i, n := range names {
		running.Write([]byte(n))
		running.Write([]byte(files[n]))
		hashes[i] = base64.StdEncoding.EncodeToString(running.Sum(nil))
	}
	total := sha256.New()
	for i, n := range names {
		total.Write([]byte(n))
		total.Write([]byte(hashes[i]))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "h1:%s\n", base64.StdEncoding.EncodeToString(total.Sum(nil)))
	for i, n := range names {
		fmt.Fprintf(&b, "%s h1:%s\n", n, hashes[i])
	}
	fsys["atlas.sum"] = &fstest.MapFile{Data: []byte(b.String())}
	return migrate.Module{Name: name, Dir: fsys}
}

const (
	v1 = "CREATE SCHEMA shop;\nCREATE TABLE shop.items (id bigint PRIMARY KEY);"
	v2 = "ALTER TABLE shop.items ADD COLUMN name text NOT NULL DEFAULT '';"
)

func count(t *testing.T, d *db.DB, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := d.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 每支測試拿到 template 的一份複本：schema 與 migration 紀錄都在，資料互不相見；template 本身只能複製、不能連。
func TestEachDatabaseIsAnIsolatedCopyOfTheTemplate(t *testing.T) {
	t.Parallel()
	step := testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": v1}))
	a, b := pg.NewDB(t, step), pg.NewDB(t, step)
	if _, err := a.Pool().Exec(context.Background(), "INSERT INTO shop.items (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, b, "SELECT count(*) FROM shop.items"); n != 0 {
		t.Fatalf("另一份 database 看到了 %d 列", n)
	}
	if n := count(t, b, "SELECT count(*) FROM platform.schema_migrations WHERE module = 'shop'"); n != 1 {
		t.Fatalf("複本的 migration 紀錄 = %d 列，預期 1", n)
	}
	name, err := testdb.TemplateName(step)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, "SELECT count(*) FROM pg_database WHERE datname = $1 AND datistemplate AND NOT datallowconn", name); n != 1 {
		t.Fatalf("template %s 沒有標成只能複製、不能連線", name)
	}
}

// migration 改了就是另一個 template：拿到的是新的 schema，不是先前建好的舊 template。改動包括同一個檔案裡
// 長度不變的內容（items → itemz）與新增的檔案。
func TestChangedMigrationsBuildANewTemplate(t *testing.T) {
	t.Parallel()
	const tables = "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'shop' AND table_name = $1 AND column_name = $2"
	if n := count(t, pg.NewDB(t, testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": v1}))), tables, "items", "id"); n != 1 {
		t.Fatal("原本的 migration 應該有 shop.items")
	}
	edited := strings.Replace(v1, "shop.items", "shop.itemz", 1)
	if n := count(t, pg.NewDB(t, testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": edited}))), tables, "itemz", "id"); n != 1 {
		t.Fatal("改過內容的 migration 拿到了舊的 template")
	}
	added := testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": v1, "002_add_name.sql": v2}))
	if n := count(t, pg.NewDB(t, added), tables, "items", "name"); n != 1 {
		t.Fatal("多一個檔案的 migration 拿到了舊的 template")
	}
}

// 同一個 template 的第一次使用同時來自很多支平行測試：只建一次，每支都拿到完整的複本。
func TestConcurrentFirstUseBuildsTheTemplateOnce(t *testing.T) {
	t.Parallel()
	step := testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": v1, "002_add_name.sql": v2, "003_more.sql": "CREATE TABLE shop.more (id int);"}))
	for i := range 8 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			d := pg.NewDB(t, step)
			if n := count(t, d, "SELECT count(*) FROM platform.schema_migrations WHERE module = 'shop'"); n != 3 {
				t.Fatalf("migration 紀錄 = %d 列，預期 3", n)
			}
		})
	}
}

// 沒有 step 的是空的 database（測 migration 本身用）。
func TestNoStepsGivesAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t)
	if n := count(t, d, "SELECT count(*) FROM pg_namespace WHERE nspname IN ('platform', 'river', 'shop')"); n != 0 {
		t.Fatalf("空的 database 有 %d 個 schema", n)
	}
}

// 跨行程：另一個套件行程（沒有共用的行程內狀態，只共用 container）同時第一次用到同一個 template，
// 靠 advisory lock 排隊，只建一次；沒有 lock 的話 CREATE DATABASE 會撞名。
// 內容帶時間戳：每次執行都是新的 template，-count=N 也會真的建。
func TestTemplateBuildAcrossProcessesIsSerializedByAdvisoryLock(t *testing.T) {
	t.Parallel()
	step := testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": v1, "002_add_name.sql": v2, "003_cross.sql": fmt.Sprintf("CREATE TABLE shop.cross_process_%d (id int);", time.Now().UnixNano())}))
	for i := range 6 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			d := pg.Peer().NewDB(t, step) // 每個 Peer 有自己的 sync.Once 表，等於另一個行程
			if n := count(t, d, "SELECT count(*) FROM platform.schema_migrations WHERE module = 'shop'"); n != 3 {
				t.Fatalf("migration 紀錄 = %d 列，預期 3", n)
			}
		})
	}
	t.Cleanup(func() {
		name, err := testdb.TemplateName(step)
		if err != nil {
			t.Error(err)
			return
		}
		conn, err := pgx.Connect(context.Background(), pg.AdminURL())
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.Close(context.Background()) }()
		var n int
		if err := conn.QueryRow(context.Background(), "SELECT count(*) FROM pg_database WHERE datname = $1 AND datistemplate", name).Scan(&n); err != nil || n != 1 {
			t.Errorf("template 應該恰好一個且已標記：n=%d err=%v", n, err)
		}
	})
}

// 前一個行程建到一半就結束（database 在、沒標成 template、裡面有殘缺的內容）：下一個使用者 DROP 後重建。
func TestHalfBuiltTemplateIsRebuilt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	step := testdb.Migrations(atlasModule("shop", map[string]string{"001_init.sql": v1, "002_half.sql": fmt.Sprintf("CREATE TABLE shop.half_%d (id int);", time.Now().UnixNano())}))
	name, err := testdb.TemplateName(step)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, pg.AdminURL())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	d := pg.Peer().NewDB(t, step)
	if n := count(t, d, "SELECT count(*) FROM platform.schema_migrations WHERE module = 'shop'"); n != 2 {
		t.Fatalf("重建後的 migration 紀錄 = %d 列，預期 2", n)
	}
	var isTemplate bool
	if err := conn.QueryRow(ctx, "SELECT datistemplate FROM pg_database WHERE datname = $1", name).Scan(&isTemplate); err != nil || !isTemplate {
		t.Fatalf("重建後應該是 template：%v %v", isTemplate, err)
	}
}

// 找到的共用 container 在用到之前被回收：第一次失敗，重試一次起一個新的；兩次都失敗才回錯誤。
func TestStartRetriesOnceWhenTheSharedContainerVanishes(t *testing.T) {
	ctx := context.Background()
	calls := 0
	srv, err := testdb.StartWith(ctx, func(ctx context.Context, img string, opts ...testcontainers.ContainerCustomizer) (*tcpostgres.PostgresContainer, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("container 已被 Ryuk 回收")
		}
		return testdb.RealRun(ctx, img, opts...)
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	srv.Close()

	calls = 0
	_, err = testdb.StartWith(ctx, func(context.Context, string, ...testcontainers.ContainerCustomizer) (*tcpostgres.PostgresContainer, error) {
		calls++
		return nil, errors.New("docker 壞了")
	})
	if err == nil || calls != 2 {
		t.Fatalf("兩次都失敗應該回錯誤且只試兩次：calls=%d err=%v", calls, err)
	}
}
