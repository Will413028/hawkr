//go:build integration

package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// The package shares one postgres container and hands each test its own
// database cloned from it, instead of booting a container per test. The
// mechanism and its rationale live in go-pkg/testutil/dbtest.
//
// What is specific to this package: four tests seed store_id 8001 for the same
// week, while ai_usage is PRIMARY KEY (store_id, week_start) and ai_quotas is
// PRIMARY KEY (store_id) — a single shared database would not weaken those
// tests, it would break them outright on a duplicate key. Worse, the failure is
// not always loud: TestQuota_UpdateQuota_WritesUnderStore asserts a row
// is ABSENT after DeleteQuota, and a row left behind by a sibling test inverts
// that assertion silently. The per-test database also keeps the migration seed
// rows the style tests count against per-test.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "ai_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupAiTestPool returns a pgx pool onto a database of this test's own.
// Every usecase in this package now goes through this — StyleUseCase (Wave 2),
// QuotaUseCase (Wave 3), CandidateSearchUseCase (Wave 4), and TransformUseCase
// (Wave 5, the last one). Pool goes through database.NewPool — the same path
// production uses (infrastructure.ProvidePool) — so QueryExecMode matches what
// the real usecase connects with.
//
// setupAiTestDB (the GORM counterpart, returning *gorm.DB) is gone as of Wave 5:
// TransformUseCase was its last caller in this package, and nothing else here
// ever used it.
func setupAiTestPool(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
