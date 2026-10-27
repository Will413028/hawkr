package database

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func execModeTestConfig() DBConfig {
	return DBConfig{
		Host: "db.example.com", Port: "6432", User: "u", Password: "p",
		Name: "hawkr_ai", SSLMode: "require",
	}
}

func TestDSN_YieldsPoolerSafeExecMode(t *testing.T) {
	for name, dsn := range map[string]string{
		"URI 形式":     BuildDSN(execModeTestConfig()),
		"keyword 形式": BuildDSNKeyword(execModeTestConfig()),
	} {
		t.Run(name, func(t *testing.T) {
			cc, err := pgx.ParseConfig(dsn)
			if err != nil {
				t.Fatalf("pgx.ParseConfig(%q) 失敗: %v", dsn, err)
			}
			if cc.DefaultQueryExecMode != pgx.QueryExecModeExec {
				t.Errorf("DefaultQueryExecMode = %v, want QueryExecModeExec（transaction pooling 安全）\n"+
					"  pgx 的預設是 QueryExecModeCacheStatement（具名 prepared statement），"+
					"在 PgBouncer transaction pooling 後面會間歇性失敗。",
					cc.DefaultQueryExecMode)
			}

			if v, ok := cc.RuntimeParams["default_query_exec_mode"]; ok {
				t.Errorf("default_query_exec_mode 仍留在 RuntimeParams（值 %q）——會被當成 server 參數送出", v)
			}
		})
	}
}
