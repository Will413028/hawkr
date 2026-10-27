// Package jobs 是單體的非同步工作，底層是 River（ADR D6）。
//
// Enqueue 在 ctx 裡有 transaction 時用同一個 transaction 排入：業務寫入與 job 一起
// commit 或一起 rollback，沒有「資料寫了、工作沒排」的 dual-write。
// 外部 provider 呼叫也走這裡（ADR D5）：先在 transaction 內寫業務列並排入帶冪等鍵的
// job，由 worker 在 transaction 外呼叫 provider。
//
// job 的參數型別與 worker 由各模組定義，直接使用 River 的 river.JobArgs 與
// river.WorkerDefaults；這裡只負責連線、schema 與 transaction 的接法。
package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Schema 是 River 資料表所在的 schema，和各模組的 schema 分開。
const Schema = "river"

type Enqueuer interface {
	Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error
}

// ReportingEnqueuer 另外回報是否真的排入：job 被唯一鍵略過（同一個 job 還在排隊、執行或等重試）時 inserted 為 false。
type ReportingEnqueuer interface {
	Enqueuer
	EnqueueReport(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (inserted bool, err error)
}

// softStopTimeout 是停止時等進行中 job 收尾的時間；超過就取消它們，之後依重試設定再跑。
const softStopTimeout = 30 * time.Second

// Client 排入 job；由 NewWorker 建立的 Client 也負責執行 job。
type Client struct {
	river *river.Client[pgx.Tx]
}

// NewInsertOnly 建立只排入、不執行 job 的 Client，給 serve 角色用。
func NewInsertOnly(d *db.DB) (*Client, error) {
	c, err := river.NewClient(riverpgxv5.New(d.Pool()), &river.Config{Schema: Schema})
	if err != nil {
		return nil, fmt.Errorf("jobs：建立 client：%w", err)
	}
	return &Client{river: c}, nil
}

// NewWorker 建立會執行 job 的 Client，給 worker 角色用。queues 的值是該 queue 同時執行的上限；periodic 是排程的
// job（多個 worker 同時跑時由 River 選出一個 leader 排入，不會重複）。
func NewWorker(d *db.DB, workers *river.Workers, queues map[string]int, periodic []*river.PeriodicJob) (*Client, error) {
	qc := make(map[string]river.QueueConfig, len(queues))
	for name, n := range queues {
		qc[name] = river.QueueConfig{MaxWorkers: n}
	}
	c, err := river.NewClient(riverpgxv5.New(d.Pool()), &river.Config{
		Schema:       Schema,
		Queues:       qc,
		Workers:      workers,
		PeriodicJobs: periodic,
	})
	if err != nil {
		return nil, fmt.Errorf("jobs：建立 worker client：%w", err)
	}
	return &Client{river: c}, nil
}

// Enqueue 排入一個 job。ctx 裡有 transaction 時，job 在該 transaction commit 後才可見。
func (c *Client) Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	_, err := c.EnqueueReport(ctx, args, opts)
	return err
}

// EnqueueReport 同 Enqueue，並回報是否真的排入（被唯一鍵略過回 false）。
func (c *Client) EnqueueReport(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (bool, error) {
	var res *rivertype.JobInsertResult
	var err error
	if tx, ok := db.TxFrom(ctx); ok {
		res, err = c.river.InsertTx(ctx, tx, args, opts)
	} else {
		res, err = c.river.Insert(ctx, args, opts)
	}
	if err != nil {
		return false, fmt.Errorf("jobs：排入 %s：%w", args.Kind(), err)
	}
	return !res.UniqueSkippedAsDuplicate, nil
}

// Run 執行 job 直到 ctx 結束，之後停止取新工作並等進行中的 job 收尾。
func (c *Client) Run(ctx context.Context) error {
	if err := c.river.Start(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("jobs：啟動：%w", err)
	}
	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(context.Background(), softStopTimeout)
	defer cancel()
	if err := c.river.Stop(stopCtx); err == nil {
		return nil
	}
	hardCtx, cancelHard := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelHard()
	if err := c.river.StopAndCancel(hardCtx); err != nil {
		return fmt.Errorf("jobs：停止：%w", err)
	}
	return nil
}

// Migrate 建立 River 的 schema 並套用它的 migration。可以重複執行。
func Migrate(ctx context.Context, d *db.DB) error {
	if _, err := d.Pool().Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+pgx.Identifier{Schema}.Sanitize()); err != nil {
		return fmt.Errorf("jobs：建立 schema：%w", err)
	}
	m, err := rivermigrate.New(riverpgxv5.New(d.Pool()), &rivermigrate.Config{Schema: Schema})
	if err != nil {
		return fmt.Errorf("jobs：建立 migrator：%w", err)
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("jobs：套用 migration：%w", err)
	}
	return nil
}
