//go:build integration

package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// Same shared-container-per-package, own-database-per-test mechanism as
// internal/usecase/dbtest_test.go and internal/procurement/repository/dbtest_test.go
// — see either for the rationale. TemplateDB is scoped to this package's own
// test binary/process (dbtest boots one container per package regardless of
// the name), so "ai_agent_test" doesn't need to match or avoid any sibling
// package's name — it's just this package's own label.
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "ai_agent_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// setupAgentTestDB returns a pgx pool onto a database of this test's own,
// holding the fully migrated schema. Pool goes through database.NewPool —
// the same path SessionCleaner uses in production — so QueryExecMode and
// connect_timeout match what the real cleaner connects with.
func setupAgentTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	db, drop := pg.Clone(ctx, t)

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		drop()
	}
}
