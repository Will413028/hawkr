//go:build integration

package deletion

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var postgres *testdb.Server

func TestMain(tests *testing.M) { testdb.Main(tests, &postgres) }

type cleanupClock struct {
	now time.Time
}

func (clock *cleanupClock) Now() time.Time { return clock.now }

func (clock *cleanupClock) NowOrNil() *time.Time { return &clock.now }

func TestCleanupUniquePeriodSurvivesCompletedRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := postgres.NewDB(t, testdb.Jobs())
	clock := &cleanupClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	client, err := river.NewClient(riverpgxv5.New(database.Pool()), &river.Config{
		Schema: jobs.Schema,
		Test:   river.TestConfig{Time: clock},
	})
	if err != nil {
		t.Fatal(err)
	}
	enqueue := func() {
		t.Helper()
		if _, err := client.Insert(ctx, CleanupArgs{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	enqueue()
	if _, err := database.Conn(ctx).Exec(ctx, "UPDATE river.river_job SET state='completed', finalized_at=now() WHERE kind='buyer.cleanup_pii'"); err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(30 * time.Minute)
	enqueue()
	clock.now = clock.now.Add(30 * time.Minute)
	enqueue()
	var completed, scheduled int
	err = database.Conn(ctx).QueryRow(ctx, "SELECT count(*) FILTER (WHERE state='completed'), count(*) FILTER (WHERE state<>'completed') FROM river.river_job WHERE kind='buyer.cleanup_pii'").Scan(&completed, &scheduled)
	if err != nil {
		t.Fatal(err)
	}
	if completed != 1 || scheduled != 1 {
		t.Fatalf("same period must dedupe and next period must enqueue: completed=%d scheduled=%d", completed, scheduled)
	}
}
