package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-jet/jet/v2/generator/metadata"
	pggen "github.com/go-jet/jet/v2/generator/postgres"
	"github.com/go-jet/jet/v2/generator/template"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// migrations 是用 jet 建 query 的模組與它的 migration；新增一個模組就加一行。
var migrations = map[string]func() fs.FS{
	"social": social.Migrations,
}

func runTables(args []string) error {
	fl := flag.NewFlagSet("tables", flag.ContinueOnError)
	module := fl.String("module", "", "模組名稱")
	out := fl.String("out", "", "輸出目錄（先清空）")
	if err := fl.Parse(args); err != nil {
		return err
	}
	mig, ok := migrations[*module]
	if !ok || *out == "" {
		return fmt.Errorf("tables 需要 -module（目前只有 social）與 -out")
	}
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, testdb.Image,
		tcpostgres.WithDatabase("jet"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second),
			wait.ForListeningPort("5432/tcp"),
		)),
	)
	if err != nil {
		return fmt.Errorf("起 Postgres container（Docker 要在跑，見 AGENTS.local.md 的 DOCKER_HOST）：%w", err)
	}
	defer func() { _ = c.Terminate(context.Background()) }()
	url, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}
	d, err := db.Open(ctx, db.Config{URL: url, ConnectRetryBudget: 30 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()
	if err := migrate.Apply(ctx, d, []migrate.Module{{Name: *module, Dir: mig()}}); err != nil {
		return fmt.Errorf("套用 %s 的 migration：%w", *module, err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil { //nolint:gosec // 生成的原始碼目錄要能被一般使用者走訪
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(*out), ".jetgen-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	sqlDB := stdlib.OpenDBFromPool(d.Pool())
	defer func() { _ = sqlDB.Close() }()
	// 只要表型別：query 的結果用自己的 struct 以 pgx 掃描，jet 的 model 與 QRM 不用；view 不生成（公開可讀的規則在 visibility，不在 DB 的 view）；
	// 平放在輸出目錄（table/）。
	tpl := template.Default(postgres.Dialect).UseSchema(func(m metadata.Schema) template.Schema {
		sb := template.DefaultSQLBuilder().UseView(func(t metadata.Table) template.ViewSQLBuilder {
			v := template.DefaultViewSQLBuilder(t)
			v.Skip = true
			return v
		})
		return template.DefaultSchema(m).UsePath("").UseModel(template.DefaultModel().ShouldSkip(true)).UseSQLBuilder(sb)
	})
	if err := pggen.GenerateDB(sqlDB, *module, stage, tpl); err != nil {
		return err
	}
	if err := os.RemoveAll(*out); err != nil {
		return err
	}
	return os.Rename(stage, *out)
}
