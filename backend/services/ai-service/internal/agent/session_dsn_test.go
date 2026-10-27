package agent

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Will413028/hawkr/ai-service/internal/config"
)

func TestSessionDSN_IsPoolerSafe(t *testing.T) {
	cfg := &config.Config{
		DBHost: "pooler.example.com",
		DBPort: "6432",
		DBUser: "u", DBPassword: "p", DBName: "hawkr_ai", DBSSLMode: "require",
	}
	dsn := sessionDSN(cfg)

	cc, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig 失敗：%v", err)
	}
	if cc.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("ADK session DSN 的 DefaultQueryExecMode = %v，want QueryExecModeExec\n"+
			"  具名 prepared statement 在 transaction pooling 後面會間歇性失敗，"+
			"而這條路徑繞過 pkg/database.NewPoolWithConfig。", cc.DefaultQueryExecMode)
	}
}

func TestSessionDSN_GormAcceptsIt(t *testing.T) {
	cfg := &config.Config{
		DBHost: "db.invalid", DBPort: "6432", DBUser: "u", DBPassword: "p",
		DBName: "hawkr_ai", DBSSLMode: "disable",
	}
	dsn := sessionDSN(cfg)
	if !strings.Contains(dsn, "default_query_exec_mode=exec") {
		t.Fatalf("DSN 應帶 pooler-safe exec mode：%s", dsn)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm 不接受帶 default_query_exec_mode 的 DSN：%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取得 sql.DB 失敗：%v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}
