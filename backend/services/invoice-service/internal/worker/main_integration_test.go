//go:build integration

package worker

import (
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// pg 起一顆 postgres:16-alpine、重放 db/migrations，每支測試拿到一個自己的 database。
//
// 🔴 刻意不用「環境變數沒設就 t.Skip」：那會讓 CI 的 job 永久綠而測試從未執行過。
// 這條與 internal/usecase／internal/repository/postgres 的 integration_test.go 檔頭同源，
// 三處不得分歧。
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "invoice_worker_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

// testMetrics / testObsA / testObsB / testObsC 與 main_test.go 的那一組是同一個宣告的兩個
// build-tag 分身（兩檔互斥，永遠只有一份被編譯）。放兩份而不是抽第三個檔案：
// 讀 TestMain 的人需要在同一個畫面上看到「這四個是誰初始化的」。
var (
	testMetrics *Metrics
	testObsA    *outbox.Metrics
	testObsB    *outbox.Metrics
	testObsC    *outbox.Metrics
)

func TestMain(m *testing.M) {
	testMetrics = NewMetrics()
	testObsA = outbox.NewMetrics("test_invoice_issue")
	testObsB = outbox.NewMetrics("test_invoice_outbox")
	testObsC = outbox.NewMetrics("test_invoice_deadline")
	// RunTests 自己會 os.Exit —— 不要在它後面再寫任何東西。
	dbtest.RunTests(m, pg)
}
