//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func TestMigrateIsRepeatableAgainstARealDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", pg.NewURL(t))

	if err := run(context.Background(), []string{"migrate"}); err != nil {
		t.Fatalf("migrate：%v", err)
	}
	if err := run(context.Background(), []string{"migrate"}); err != nil {
		t.Fatalf("第二次 migrate：%v", err)
	}
}

// 每個模組單獨套用在空的 database 上：參照別的模組會套用失敗，
// 把物件建到自己的 schema 以外會被 ForeignObjects 列出來（ADR D3）。
func TestEveryModuleStaysInsideItsOwnSchema(t *testing.T) {
	for _, m := range modules() {
		t.Run(m.Name, func(t *testing.T) {
			d := pg.NewDB(t)
			if err := migrate.Apply(context.Background(), d, []migrate.Module{m}); err != nil {
				t.Fatalf("單獨套用 %s 的 migration 失敗（是否參照了別的模組？）：%v", m.Name, err)
			}
			if err := migrate.HasOwnSchema(context.Background(), d, m.Name); err != nil {
				t.Fatal(err)
			}
			foreign, err := migrate.ForeignObjects(context.Background(), d, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range foreign {
				t.Errorf("%s 的 migration 在自己的 schema 以外建立了 %s", m.Name, o)
			}
		})
	}
}

// 模組的 schema.sql（表、約束與索引，Atlas 的 desired state）加 objects.sql（view、函式與複合型別，手寫 migration；沒有就略過）
// 是 sqlc 讀的檔；部署時套用的卻是 migrations/。兩者不同時，sqlc 生成的程式對著一份不存在的 schema 寫成，只會在執行期出錯。
// 這裡把兩者各自套用在空的 database 上（schema.sql 先、objects.sql 後），比對結果；也檢查 migration 套用後的每個函式符合單體規則 6（FunctionsBreakingRule6）。
func TestSchemaFileMatchesMigrations(t *testing.T) {
	ctx := context.Background()
	for _, m := range modules() {
		t.Run(m.Name, func(t *testing.T) {
			fromMigrations := pg.NewDB(t)
			if err := migrate.Apply(ctx, fromMigrations, []migrate.Module{m}); err != nil {
				t.Fatal(err)
			}
			fromSchema := pg.NewDB(t)
			for _, name := range []string{"schema.sql", "objects.sql"} {
				sql, err := os.ReadFile(filepath.Join("..", "..", "internal", "modules", m.Name, "db", name))
				if name == "objects.sql" && os.IsNotExist(err) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := fromSchema.Pool().Exec(ctx, string(sql)); err != nil {
					t.Fatalf("%s 無法套用：%v", name, err)
				}
			}
			// sqlc 讀的是 schema.sql：它若建立別的模組的表，參照別的模組的 query 就能通過生成，
			// 而 migration 沒變，下面的比對也看不出來。
			foreign, err := migrate.ForeignObjects(ctx, fromSchema, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range foreign {
				t.Errorf("%s 的 schema.sql 或 objects.sql 在自己的 schema 以外建立了 %s", m.Name, o)
			}
			want, err := migrate.DescribeSchema(ctx, fromSchema, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := migrate.DescribeSchema(ctx, fromMigrations, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			if want == "" {
				t.Fatal("schema.sql 沒有建立任何東西，比對沒有意義")
			}
			// 單體規則 6：模組的每個函式都是 BEGIN ATOMIC、回傳型別不是表或 view 的整列（pub_* 與內部函式都算）。
			broken, err := migrate.FunctionsBreakingRule6(ctx, fromMigrations, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range broken {
				t.Errorf("%s.%s（單體規則 6：函式一律 BEGIN ATOMIC、回傳型別不用表的整列）", m.Name, f)
			}
			if got != want {
				t.Errorf("migrations 與 schema.sql＋objects.sql 不一致；改完 schema.sql 要在模組的 db/ 跑 atlas migrate diff，改 objects.sql 要以 atlas migrate new 手寫 migration。\nschema.sql＋objects.sql：\n%s\n\nmigrations：\n%s", want, got)
			}
		})
	}
}

// 擁有者的 db/pub.sql 是消費端 sqlc 讀的契約檔（ADR 2026-09-24-go-modular-monolith 文末修訂）：它宣告的每個 pub_* 與 migration 套用後的
// 真實物件的欄位名稱與型別、函式的參數與回傳必須相同（雙向：真實的 pub_* 也都要在契約檔裡），兼作消費者契約測試。函式的寫法（單體規則 6）
// 由 TestSchemaFileMatchesMigrations 對每個模組檢查。
func TestPubContractsMatchMigrations(t *testing.T) {
	ctx := context.Background()
	checked := 0
	for _, m := range modules() {
		path := filepath.Join("..", "..", "internal", "modules", m.Name, "db", "pub.sql")
		contract, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		checked++
		t.Run(m.Name, func(t *testing.T) {
			fromMigrations := pg.NewDB(t)
			if err := migrate.Apply(ctx, fromMigrations, []migrate.Module{m}); err != nil {
				t.Fatal(err)
			}
			fromContract := pg.NewDB(t)
			if _, err := fromContract.Pool().Exec(ctx, string(contract)); err != nil {
				t.Fatalf("pub.sql 無法套用：%v", err)
			}
			want, err := migrate.DescribePub(ctx, fromContract, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := migrate.DescribePub(ctx, fromMigrations, m.Name)
			if err != nil {
				t.Fatal(err)
			}
			if want == "" {
				t.Fatal("pub.sql 沒有宣告任何 pub_*，比對沒有意義")
			}
			if got != want {
				t.Errorf("%s 的 pub.sql 與 migration 套用後的 pub_* 不一致；改 pub_* 時同一次變更內更新 pub.sql 與所有消費端。\npub.sql：\n%s\n\nmigrations：\n%s", m.Name, want, got)
			}
		})
	}
	if checked == 0 {
		t.Fatal("沒有任何模組有 pub.sql（seller 發布 pub_exposable_stores），檢查範圍不對")
	}
}

// worker 角色執行排入的 job：這裡以 revalidate 驗證 River 的 schema、queue 與 worker 的接線。
func TestWorkerRunsQueuedJobs(t *testing.T) {
	got := make(chan []string, 1)
	website := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var body revalidate.Args
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		got <- body.Tags
	}))
	defer website.Close()

	url := pg.NewURL(t)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("WEB_REVALIDATE_BASE_URL", website.URL)
	t.Setenv("WEB_REVALIDATE_SECRET", "test")
	t.Setenv("KYC_ENCRYPTION_KEYS", "test1:dGVzdC1reWMtZW5jcnlwdGlvbi1rZXktMDAwMDAwMDE=")
	t.Setenv("KYC_HASH_KEYS", "test1:dGVzdC1reWMtaGFzaC1obWFjLWtleS0wMDAwMDAwMDE=")
	t.Setenv("SMS_SENDER", "log")
	// 連線在第一次送出時才建立；這個測試不送事件。
	t.Setenv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:1/")
	t.Setenv("MEDIA_SERVICE_ADDR", "127.0.0.1:1")
	t.Setenv("ORDER_SERVICE_ADDR", "127.0.0.1:1")
	t.Setenv("PRODUCT_SERVICE_ADDR", "127.0.0.1:1")
	t.Setenv("REVIEW_SERVICE_ADDR", "127.0.0.1:1")
	ctx := context.Background()
	if err := run(ctx, []string{"migrate"}); err != nil {
		t.Fatal(err)
	}

	d, err := db.Open(ctx, db.Config{URL: url, ConnectRetryBudget: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	postID := uuid.MustParse("01998a3c-7d2e-7b10-8c4f-3a5e6d7f8091")
	if err := jc.Enqueue(ctx, revalidate.Args{Tags: []string{revalidate.PostTag(postID)}}, nil); err != nil {
		t.Fatal(err)
	}

	workerCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- run(workerCtx, []string{"worker"}) }()
	select {
	case tags := <-got:
		if len(tags) != 1 || tags[0] != revalidate.PostTag(postID) {
			t.Errorf("tags = %v", tags)
		}
	case err := <-done:
		t.Fatalf("worker 提早結束：%v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("30 秒內 worker 沒有執行 job")
	}
	stop()
	if err := <-done; err != nil {
		t.Fatalf("worker 停止時回傳 %v", err)
	}
}
