//go:build integration

// 本 package 的 integration TestMain。非 integration 模式**沒有** TestMain
// （本 package 不需要 process 級的初始化），所以這裡不需要成對的 `!integration` 分身
// ——那與 internal/worker 的兩份 TestMain 不同，理由是那邊有 promauto 名稱要在
// process 內只註冊一次。
package mq

import (
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "invoice_mq_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}
