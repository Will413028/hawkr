//go:build !integration

// 本檔是**非** integration 模式下的 TestMain。integration 模式那一份在
// main_integration_test.go —— 同一個 package 只能有一個 TestMain，而 integration
// 模式還要多做一件事（起 postgres 容器並重放 migrations）。
package worker

import (
	"os"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/outbox"
)

// testMetrics / testObsA / testObsB / testObsC 是整個 package 共用的一組。
//
// 🔴 promauto 註冊是 process-global：`NewMetrics()` 或 `outbox.NewMetrics(name)` 在同一
// 個 process 內用同一個名字呼叫第二次會直接 panic。所以它們在 TestMain 建立一次，
// 每支測試共用。看到「duplicate metrics collector registration attempted」就是有人
// 在測試裡又建了一份。
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
	os.Exit(m.Run())
}
