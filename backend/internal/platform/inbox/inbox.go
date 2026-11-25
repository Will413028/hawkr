// Package inbox 是外部 provider 事件的 webhook inbox（收款開通 ADR D29；backend/AGENTS.md 單體規則 5）：platform 擁有 schema
// inbox（db/schema.sql），模組只經這裡的 API 讀寫。
//
// 流程：入口驗過簽章後在一個 transaction 內呼叫 Record（寫一列並排入派送 job），commit 之後才回 2xx；派送 job 在一個 transaction 內呼叫
// 模組登記的 Handler 並記下派送成功的時刻；模組在自己寫業務列的 transaction 內以 MarkProcessed 或 MarkRouteProcessed 標記已處理。
// 「已處理」由模組決定：handler 可以只排入自己的 job，由那個 job 涵蓋事件之後再標記（收款的同步 job 就是這樣）。
//
// 週期 job：補掃重派還沒派送成功、也還沒處理的列（派送 job 被丟棄或用完重試）；收到超過 deadAfter 仍沒有處理的列標成死信（終局：不再補掃、
// 不再派送、不能再標記已處理，記 Error log，Dead 列出）；清理刪除已處理或死信、過了保存期的列。
// payload 是原始內容，可能含個資：本套件與呼叫方都不得把它寫進 log。
package inbox

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Schema 是 inbox 的 schema 名稱。
const Schema = "inbox"

// Migrations 回傳 inbox schema 的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生
	}
	return sub
}

const (
	// maxAttempts 是派送 job 暫時性失敗時 River 重試的上限（退避是 River 預設的 attempt^4 秒）；用完之後由補掃重派。
	maxAttempts = 8
	// dispatchTimeout 是一次派送的上限。
	dispatchTimeout = 30 * time.Second
	// sweepGrace 是事件收到多久之後仍沒有派送成功、也沒有處理時由補掃重派。
	sweepGrace = 5 * time.Minute
	// sweepInterval 是補掃與清理的週期。
	sweepInterval = 15 * time.Minute
	// deadAfter 是事件收到多久之後仍沒有處理就標成死信。比補掃與各模組的重試都長：到這個時間還沒處理，代表重試沒有用，要人看。
	deadAfter = 7 * 24 * time.Hour
	// batch 是補掃與清理一次處理的上限。
	batch = 500
)

var providerPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Event 是 inbox 的一列。
type Event struct {
	Provider string
	EventID  string
	// RouteKey 是 provider 內的路由鍵（收款是外部帳戶 ID）。
	RouteKey   string
	Payload    []byte
	ReceivedAt time.Time
}

// Handler 是模組對一個 provider 的事件的處理：在派送 job 的 transaction 內執行（db.Conn(ctx) 加入它）。回傳錯誤時 transaction rollback，
// 由 River 退避重試；重試用完之後由補掃重派，所以 handler 必須可以重複執行。
type Handler func(ctx context.Context, e Event) error

// Source 是一個 provider 的登記：事件的保存期與處理者。
type Source struct {
	Provider string
	// Retention 是原始 payload 的保存期，從收到起算；已處理而且過了保存期的列由清理 job 刪除。
	Retention time.Duration
	Handle    Handler
}

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Jobs   jobs.Enqueuer
	Now    func() time.Time
	Logger *slog.Logger
}

// Inbox 是 webhook inbox。Register 在啟動時完成，之後只讀。
type Inbox struct {
	d       Deps
	mu      sync.RWMutex
	sources map[string]Source
}

// New 建立 inbox。
func New(d Deps) *Inbox {
	if d.DB == nil || d.Jobs == nil || d.Now == nil || d.Logger == nil {
		panic("inbox: missing dependency")
	}
	return &Inbox{d: d, sources: map[string]Source{}}
}

// Register 登記一個 provider 的處理者；同一個 provider 登記兩次、名稱不合或沒有處理者時 panic（組裝錯誤）。
func (i *Inbox) Register(s Source) {
	if !providerPattern.MatchString(s.Provider) || s.Retention <= 0 || s.Handle == nil {
		panic(fmt.Sprintf("inbox: invalid source %q", s.Provider))
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, ok := i.sources[s.Provider]; ok {
		panic(fmt.Sprintf("inbox: provider %q registered twice", s.Provider))
	}
	i.sources[s.Provider] = s
}

func (i *Inbox) source(provider string) (Source, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	s, ok := i.sources[provider]
	return s, ok
}

// ErrUnknownProvider 表示 provider 沒有登記。
var ErrUnknownProvider = errors.New("inbox: provider is not registered")

// ErrNoTransaction 表示 Record 在 transaction 外呼叫：寫入與排入派送 job 必須同一個 transaction。
var ErrNoTransaction = errors.New("inbox: Record must run inside a transaction")

const insertEventSQL = `INSERT INTO inbox.events (provider, event_id, route_key, payload, received_at, purge_after)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (provider, event_id) DO NOTHING`

// Record 在呼叫方的 transaction 內寫一個事件並排入它的派送 job（以 (provider, event_id) 為唯一鍵）。(provider, event_id) 已存在的重送
// 事件不寫入、不排 job，回傳 false。ReceivedAt 由這裡以 clock 填上。
func (i *Inbox) Record(ctx context.Context, e Event) (inserted bool, err error) {
	if _, ok := db.TxFrom(ctx); !ok {
		return false, ErrNoTransaction
	}
	src, ok := i.source(e.Provider)
	if !ok {
		return false, fmt.Errorf("%w: %s", ErrUnknownProvider, e.Provider)
	}
	now := i.d.Now().UTC()
	tag, err := i.d.DB.Conn(ctx).Exec(ctx, insertEventSQL, e.Provider, e.EventID, e.RouteKey, e.Payload, now, now.Add(src.Retention))
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	return true, enqueueDispatch(ctx, i.d.Jobs, e.Provider, e.EventID)
}

const markProcessedSQL = `UPDATE inbox.events SET processed_at = $3, processed_by = $4
WHERE provider = $1 AND event_id = $2 AND processed_at IS NULL AND dead_at IS NULL`

// MarkProcessed 在呼叫方的 transaction 內把一個事件標記為已處理（已處理的不變）。by 是處理者的名稱。
func (i *Inbox) MarkProcessed(ctx context.Context, provider, eventID, by string) error {
	_, err := i.d.DB.Conn(ctx).Exec(ctx, markProcessedSQL, provider, eventID, i.d.Now().UTC(), by)
	return err
}

const markRouteProcessedSQL = `UPDATE inbox.events SET processed_at = $4, processed_by = $5
WHERE provider = $1 AND route_key = $2 AND processed_at IS NULL AND dead_at IS NULL AND received_at <= $3`

// MarkRouteProcessed 在呼叫方的 transaction 內把一個路由鍵在 receivedBefore（含）之前收到的未處理事件整批標記為已處理，回傳標記的列數。
// 用在「重讀即最新」的處理：讀取開始前收到的事件都被這次讀取涵蓋，讀取期間到的留給下一輪。
func (i *Inbox) MarkRouteProcessed(ctx context.Context, provider, routeKey string, receivedBefore time.Time, by string) (int64, error) {
	tag, err := i.d.DB.Conn(ctx).Exec(ctx, markRouteProcessedSQL, provider, routeKey, receivedBefore, i.d.Now().UTC(), by)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

const routePendingSQL = `SELECT EXISTS (SELECT 1 FROM inbox.events WHERE provider = $1 AND route_key = $2 AND processed_at IS NULL AND dead_at IS NULL)`

// RoutePending 回傳路由鍵還有沒有未處理的事件。
func (i *Inbox) RoutePending(ctx context.Context, provider, routeKey string) (bool, error) {
	var pending bool
	err := i.d.DB.Conn(ctx).QueryRow(ctx, routePendingSQL, provider, routeKey).Scan(&pending)
	return pending, err
}

const staleRoutesSQL = `SELECT DISTINCT route_key FROM inbox.events
WHERE provider = $1 AND processed_at IS NULL AND dead_at IS NULL AND received_at < $2 AND route_key > $3
ORDER BY route_key
LIMIT $4`

// StaleRoutes 回傳有 receivedBefore 之前收到卻還沒處理的事件的路由鍵，依路由鍵升序、從 after 之後分批（limit 筆）。
func (i *Inbox) StaleRoutes(ctx context.Context, provider string, receivedBefore time.Time, after string, limit int) ([]string, error) {
	rows, err := i.d.DB.Conn(ctx).Query(ctx, staleRoutesSQL, provider, receivedBefore, after, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

const loadEventSQL = `SELECT route_key, payload, received_at, processed_at IS NOT NULL OR dead_at IS NOT NULL FROM inbox.events WHERE provider = $1 AND event_id = $2`

const markDispatchedSQL = `UPDATE inbox.events SET dispatched_at = $3 WHERE provider = $1 AND event_id = $2`

// Dispatch 是派送 job 的內容：在一個 transaction 內呼叫 provider 的 Handler 並記下派送成功的時刻。列不存在（已清理）、已處理或已是死信時什麼都不做；
// handler 回傳錯誤時 rollback 並回傳錯誤（由 River 重試）。給 worker 與測試呼叫。
func (i *Inbox) Dispatch(ctx context.Context, provider, eventID string) error {
	src, ok := i.source(provider)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownProvider, provider)
	}
	return i.d.DB.InTx(ctx, func(ctx context.Context) error {
		conn := i.d.DB.Conn(ctx)
		e := Event{Provider: provider, EventID: eventID}
		var processed bool
		err := conn.QueryRow(ctx, loadEventSQL, provider, eventID).Scan(&e.RouteKey, &e.Payload, &e.ReceivedAt, &processed)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if processed {
			return nil
		}
		if err := src.Handle(ctx, e); err != nil {
			return err
		}
		_, err = conn.Exec(ctx, markDispatchedSQL, provider, eventID, i.d.Now().UTC())
		return err
	})
}

const undispatchedSQL = `SELECT provider, event_id FROM inbox.events
WHERE processed_at IS NULL AND dead_at IS NULL AND dispatched_at IS NULL AND received_at < $1 AND (provider, event_id) > ($2, $3)
ORDER BY provider, event_id
LIMIT $4`

// Sweep 是補掃：重派收到超過 sweepGrace、仍沒有派送成功也沒有處理的事件（派送 job 被丟棄或用完重試；還在排隊或重試中的被唯一鍵合併），
// 分批、各批一個 transaction。回傳排入的事件數（唯一鍵合併掉的也算）。
func (i *Inbox) Sweep(ctx context.Context) (int, error) {
	before := i.d.Now().UTC().Add(-sweepGrace)
	afterProvider, afterID := "", ""
	n := 0
	for {
		rows, err := i.d.DB.Conn(ctx).Query(ctx, undispatchedSQL, before, afterProvider, afterID, batch)
		if err != nil {
			return n, err
		}
		type key struct{ provider, eventID string }
		keys, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (key, error) {
			var k key
			err := r.Scan(&k.provider, &k.eventID)
			return k, err
		})
		if err != nil {
			return n, err
		}
		err = i.d.DB.InTx(ctx, func(ctx context.Context) error {
			for _, k := range keys {
				if err := enqueueDispatch(ctx, i.d.Jobs, k.provider, k.eventID); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return n, err
		}
		n += len(keys)
		if len(keys) < batch {
			return n, nil
		}
		afterProvider, afterID = keys[len(keys)-1].provider, keys[len(keys)-1].eventID
	}
}

const purgeSQL = `DELETE FROM inbox.events WHERE ctid IN (
    SELECT ctid FROM inbox.events WHERE (processed_at IS NOT NULL OR dead_at IS NOT NULL) AND purge_after <= $1 LIMIT $2
)`

// Purge 是清理：刪除已處理或死信、而且過了保存期的事件（含原始 payload），分批。還在補掃的未處理列不刪：它們由補掃或模組的掃描處理。回傳刪除的列數。
func (i *Inbox) Purge(ctx context.Context) (int64, error) {
	now := i.d.Now().UTC()
	var total int64
	for {
		tag, err := i.d.DB.Conn(ctx).Exec(ctx, purgeSQL, now, batch)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < batch {
			return total, nil
		}
	}
}

const buryDeadSQL = `UPDATE inbox.events SET dead_at = $2,
    dead_reason = CASE WHEN dispatched_at IS NULL THEN 'undispatched' ELSE 'unprocessed' END
WHERE ctid IN (
    SELECT ctid FROM inbox.events WHERE processed_at IS NULL AND dead_at IS NULL AND received_at < $1 LIMIT $3
)
RETURNING provider`

// Bury 把收到超過 deadAfter 仍沒有處理的事件標成死信，回傳各 provider 標記的列數（分批做完）。標記之後事件不再被補掃或派送，也不能再標記已處理：
// 要處理它的人讀 Dead，修好原因後以 provider 的方式重建（例如重新向 provider 取得物件），不在這裡重派。
func (i *Inbox) Bury(ctx context.Context) (map[string]int, error) {
	now := i.d.Now().UTC()
	out := map[string]int{}
	for {
		rows, err := i.d.DB.Conn(ctx).Query(ctx, buryDeadSQL, now.Add(-deadAfter), now, batch)
		if err != nil {
			return out, err
		}
		providers, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return out, err
		}
		for _, p := range providers {
			out[p]++
		}
		if len(providers) < batch {
			return out, nil
		}
	}
}

// DeadEvent 是一個死信的 metadata（不含 payload）。
type DeadEvent struct {
	Provider   string
	EventID    string
	RouteKey   string
	Reason     string
	ReceivedAt time.Time
	DeadAt     time.Time
}

const deadSQL = `SELECT provider, event_id, route_key, dead_reason, received_at, dead_at FROM inbox.events
WHERE provider = $1 AND dead_at IS NOT NULL ORDER BY dead_at DESC, event_id LIMIT $2`

// Dead 回傳一個 provider 最近的死信（新的在前，最多 limit 筆）。
func (i *Inbox) Dead(ctx context.Context, provider string, limit int) ([]DeadEvent, error) {
	rows, err := i.d.DB.Conn(ctx).Query(ctx, deadSQL, provider, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DeadEvent, error) {
		var d DeadEvent
		err := r.Scan(&d.Provider, &d.EventID, &d.RouteKey, &d.Reason, &d.ReceivedAt, &d.DeadAt)
		return d, err
	})
}

// DispatchArgs 是派送一個事件的 job，以 (provider, event_id) 為唯一鍵。
type DispatchArgs struct {
	Provider string `json:"provider" river:"unique"`
	EventID  string `json:"event_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (DispatchArgs) Kind() string { return "inbox.dispatch" }

// InsertOpts 是派送 job 的排入選項。
func (DispatchArgs) InsertOpts() river.InsertOpts {
	return jobs.UniqueByArgs(maxAttempts)
}

func enqueueDispatch(ctx context.Context, jc jobs.Enqueuer, provider, eventID string) error {
	args := DispatchArgs{Provider: provider, EventID: eventID}
	opts := args.InsertOpts()
	return jc.Enqueue(ctx, args, &opts)
}

// SweepArgs 是週期的補掃與清理。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "inbox.sweep" }

// InsertOpts 把同一段時間內的排程與重啟合併。
func (SweepArgs) InsertOpts() river.InsertOpts {
	return jobs.SweepOpts(10 * time.Minute)
}

type dispatchWorker struct {
	river.WorkerDefaults[DispatchArgs]
	inbox *Inbox
}

func (w *dispatchWorker) Work(ctx context.Context, job *river.Job[DispatchArgs]) error {
	err := w.inbox.Dispatch(ctx, job.Args.Provider, job.Args.EventID)
	if errors.Is(err, ErrUnknownProvider) {
		return river.JobCancel(err)
	}
	return err
}

func (w *dispatchWorker) Timeout(*river.Job[DispatchArgs]) time.Duration { return dispatchTimeout }

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	inbox *Inbox
}

func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	dead, err := w.inbox.Bury(ctx)
	for p, n := range dead {
		w.inbox.d.Logger.ErrorContext(ctx, "inbox：事件超過期限仍沒有處理，標成死信", "provider", p, "events", n)
	}
	if err != nil {
		return err
	}
	n, err := w.inbox.Sweep(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		w.inbox.d.Logger.WarnContext(ctx, "inbox：補掃重派了沒有派送成功的事件", "events", n)
	}
	_, err = w.inbox.Purge(ctx)
	return err
}

func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 River worker。
func (i *Inbox) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &dispatchWorker{inbox: i})
	river.AddWorker(workers, &sweepWorker{inbox: i})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的補掃與清理。
func (i *Inbox) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(sweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "inbox.sweep", RunOnStart: true},
	)}
}
