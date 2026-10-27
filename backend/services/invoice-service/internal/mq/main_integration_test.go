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

// pg 起一顆 postgres:16-alpine、重放 db/migrations，每支測試拿到一個自己的 database。
//
// 🔴 刻意不用「環境變數沒設就 t.Skip」：那會讓 CI 的 job 永久綠而測試從未執行過。
// 這條與 internal/usecase／internal/worker／internal/repository/postgres 的
// integration 檔頭同源，四處不得分歧。
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "invoice_mq_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	// RunTests 自己會 os.Exit —— 不要在它後面再寫任何東西。
	dbtest.RunTests(m, pg)
}
