//go:build integration

package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var errBoom = errors.New("boom")

type pingArgs struct {
	N int `json:"n"`
}

func (pingArgs) Kind() string { return "test_ping" }

type pingWorker struct {
	river.WorkerDefaults[pingArgs]
	got chan int
}

func (w *pingWorker) Work(_ context.Context, job *river.Job[pingArgs]) error {
	w.got <- job.Args.N
	return nil
}

func migrated(t *testing.T) *db.DB {
	t.Helper()
	d := pg.NewDB(t)
	if err := jobs.Migrate(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

func jobCount(t *testing.T, d *db.DB) int {
	t.Helper()
	var n int
	if err := d.Conn(context.Background()).QueryRow(context.Background(), "SELECT count(*) FROM river.river_job").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrateIsRepeatable(t *testing.T) {
	t.Parallel()
	d := migrated(t)
	if err := jobs.Migrate(context.Background(), d); err != nil {
		t.Fatalf("第二次 Migrate：%v", err)
	}
}

func TestEnqueueFollowsTheCallersTransaction(t *testing.T) {
	t.Parallel()
	d := migrated(t)
	c, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}

	err = d.InTx(context.Background(), func(ctx context.Context) error {
		if err := c.Enqueue(ctx, pingArgs{N: 1}, nil); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if n := jobCount(t, d); n != 0 {
		t.Fatalf("jobs = %d，transaction rollback 時 job 也不得排入", n)
	}

	if err := d.InTx(context.Background(), func(ctx context.Context) error {
		return c.Enqueue(ctx, pingArgs{N: 2}, nil)
	}); err != nil {
		t.Fatal(err)
	}
	if n := jobCount(t, d); n != 1 {
		t.Fatalf("jobs = %d，預期 commit 後有一個 job", n)
	}
}

func TestWorkerRunsEnqueuedJobsAndStopsWithTheContext(t *testing.T) {
	t.Parallel()
	d := migrated(t)
	w := &pingWorker{got: make(chan int, 1)}
	workers := river.NewWorkers()
	river.AddWorker(workers, w)
	c, err := jobs.NewWorker(d, workers, map[string]int{river.QueueDefault: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	if err := c.Enqueue(context.Background(), pingArgs{N: 7}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-w.got:
		if n != 7 {
			t.Fatalf("worker 收到 %d，預期 7", n)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("job 沒有被執行")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run 回傳 %v", err)
		}
	case <-time.After(40 * time.Second):
		t.Fatal("ctx 結束後 Run 沒有停止")
	}
}

func TestUniqueHelpersAreAcceptedByRiverAndMergeDuplicates(t *testing.T) {
	t.Parallel()
	d := migrated(t)
	c, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for name, opts := range map[string]river.InsertOpts{
		"by-args": jobs.UniqueByArgs(3),
		"sweep":   jobs.SweepOpts(time.Hour),
	} {
		o := opts
		first, err := c.EnqueueReport(ctx, pingArgs{N: 9}, &o)
		if err != nil || !first {
			t.Fatalf("%s: first insert = %v, %v", name, first, err)
		}
		second, err := c.EnqueueReport(ctx, pingArgs{N: 9}, &o)
		if err != nil || second {
			t.Fatalf("%s: duplicate insert = %v, %v; want merged", name, second, err)
		}
		if _, err := d.Conn(ctx).Exec(ctx, "DELETE FROM river.river_job"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCancelJobReportsTheStateAfterCancelling(t *testing.T) {
	t.Parallel()
	d := migrated(t)
	c, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := d.InTx(context.Background(), func(ctx context.Context) error {
		var err error
		id, err = c.EnqueueJob(ctx, pingArgs{N: 1}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var state rivertype.JobState
	if err := d.InTx(context.Background(), func(ctx context.Context) error {
		var err error
		state, _, err = c.CancelJob(ctx, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if state != rivertype.JobStateCancelled {
		t.Fatalf("取消一個還沒執行的 job 之後的狀態 = %s，預期 cancelled", state)
	}
}

func TestCancelJobReportsAMissingJobRowAsNotFound(t *testing.T) {
	t.Parallel()
	d := migrated(t)
	c, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	if err := d.InTx(context.Background(), func(ctx context.Context) error {
		var err error
		_, found, err = c.CancelJob(ctx, 987654321)
		return err
	}); err != nil {
		t.Fatalf("job 列不存在不該是錯誤：%v", err)
	}
	if found {
		t.Fatal("job 列不存在 found 應為 false")
	}
}
