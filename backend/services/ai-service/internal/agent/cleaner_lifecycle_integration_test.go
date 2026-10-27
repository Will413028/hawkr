//go:build integration

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/infrastructure"
)

// TestSessionCleaner_StopDoesNotCloseTheSharedPool is the direct regression
// test for the #3519 slice B Task 7 review finding: after Task 7 item 1
// moved SessionCleaner onto the shared fx-named "ai_pool", Stop() still had
// its old c.pool.Close() call left over from when the cleaner owned a
// private pool. A test asserting only "Stop() doesn't panic/error" would not
// have caught that — this asserts the pool is still queryable afterward,
// which is the actual property Style/Quota/Transform/CandidateSearch need.
func TestSessionCleaner_StopDoesNotCloseTheSharedPool(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAgentTestDB(ctx, t)
	defer cleanup()

	c := &SessionCleaner{pool: pool}
	c.Start()
	c.Stop()

	var one int
	err := pool.QueryRow(ctx, `SELECT 1`).Scan(&one)
	require.NoError(t, err, "pool must still be usable after cleaner.Stop() — it is shared, not owned by the cleaner")
	require.Equal(t, 1, one)
}

// TestSessionCleanerLifecycle_PoolClosesOnlyAfterCleanerStops empirically
// proves the fx.Lifecycle OnStop ordering claim in registerSessionCleaner's
// doc comment (agent/di.go), using the actual production wiring functions —
// infrastructure.ProvidePool and registerSessionCleaner — against a real
// fx.Lifecycle (fxtest, the same primitive fx.New uses internally), not a
// hand-rolled stand-in. fx.ValidateApp (used elsewhere to check the graph
// wires) explicitly never runs constructors or lifecycle hooks per its own
// doc comment, so it cannot and does not prove this; this test does.
//
// Mechanism: ProvidePool registers its OnStop hook first (as it must, since
// registerSessionCleaner's own hook can only be appended after it receives
// the already-constructed pool as a parameter — exactly mirroring real fx
// dependency-driven construction order). A probe hook is registered between
// them. fx runs OnStop hooks in reverse registration order, so Stop() must
// visit: cleaner's OnStop, then the probe, then the pool's OnStop. The probe
// queries the pool — if ordering were wrong (e.g. pool closed before the
// cleaner's scheduler fully stopped), this query would fail.
func TestSessionCleanerLifecycle_PoolClosesOnlyAfterCleanerStops(t *testing.T) {
	ctx := context.Background()
	db, cleanup := pg.Clone(ctx, t)
	defer cleanup()

	cfg := &config.Config{
		DBHost:     db.Host,
		DBPort:     db.Port,
		DBUser:     db.User,
		DBPassword: db.Password,
		DBName:     db.Name,
		DBSSLMode:  "disable",
	}

	lc := fxtest.NewLifecycle(t)

	pool, err := infrastructure.ProvidePool(lc, cfg) // registers pool's OnStop (1st)
	require.NoError(t, err)

	var order []string
	lc.Append(fx.Hook{ // registers probe's OnStop (2nd)
		OnStop: func(context.Context) error {
			order = append(order, "probe")
			var one int
			probeErr := pool.QueryRow(context.Background(), `SELECT 1`).Scan(&one)
			require.NoError(t, probeErr, "pool must still be open when the probe runs — i.e. after the cleaner's OnStop but before the pool's own OnStop")
			return nil
		},
	})

	require.NoError(t, registerSessionCleaner(lc, pool)) // registers cleaner OnStart+OnStop (3rd)

	require.NoError(t, lc.Start(ctx))
	require.NoError(t, lc.Stop(ctx))

	require.Equal(t, []string{"probe"}, order, "the probe hook must have run exactly once, between the cleaner's stop and the pool's close")

	// After the full Stop() completes, the pool (registered 1st, so it stops
	// last) really is closed — confirming order didn't just accidentally not
	// matter here.
	var one int
	err = pool.QueryRow(context.Background(), `SELECT 1`).Scan(&one)
	require.Error(t, err, "pool should be closed once the full lifecycle Stop has completed")
}
