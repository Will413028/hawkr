//go:build integration

package worker

import (
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "invoice_worker_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

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
	dbtest.RunTests(m, pg)
}
