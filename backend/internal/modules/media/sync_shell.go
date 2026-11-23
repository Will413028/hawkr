package media

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// SyncKind 提供擁有模組的 River job 種類前綴（例如 "review"）：同步 job 的種類是 "<前綴>.media_sync"，保底掃描是
// "<前綴>.media_sweep"。實作型別是各模組自己的零大小型別。
type SyncKind interface{ KindPrefix() string }

func kindPrefix[K SyncKind]() string {
	var k K
	return k.KindPrefix()
}

// SyncArgs 同步一個 ref 的 media 引用，以 (ref_type, ref_id) 為唯一鍵：同一個 ref 同時最多一個排隊中或執行中的 job。
type SyncArgs[K SyncKind] struct {
	RefType string `json:"ref_type" river:"unique"`
	RefID   string `json:"ref_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (SyncArgs[K]) Kind() string { return kindPrefix[K]() + ".media_sync" }

// InsertOpts 是同步 job 的排入選項。
func (SyncArgs[K]) InsertOpts() river.InsertOpts { return SyncInsertOpts() }

// SweepArgs 是保底掃描：清掉過期的退場紀錄，補排版本比已同步的新的 ref。
type SweepArgs[K SyncKind] struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs[K]) Kind() string { return kindPrefix[K]() + ".media_sweep" }

// InsertOpts 把同一個週期內的排程與重啟合併。
func (SweepArgs[K]) InsertOpts() river.InsertOpts { return SyncSweepInsertOpts() }

// SyncEnqueuer 回傳排入 ref 的同步 job 的函式（ctx 內有 transaction 時在該 transaction 內）。
func SyncEnqueuer[K SyncKind](jc jobs.Enqueuer) func(context.Context, Ref) error {
	return func(ctx context.Context, ref Ref) error {
		opts := SyncArgs[K]{}.InsertOpts()
		return jc.Enqueue(ctx, SyncArgs[K]{RefType: ref.Type, RefID: ref.ID}, &opts)
	}
}

// NewRefSyncer 組出擁有模組的同步：queries 把模組的 sqlc 查詢（NewSyncQueries）綁到連線上，current 讀 ref 的業務列目前指著的資產。
func NewRefSyncer[K SyncKind](
	d *db.DB, m Media, jc jobs.Enqueuer, now func() time.Time, logger *slog.Logger,
	queries func(db.DBTX) SyncQueries, current func(ctx context.Context, ref Ref) ([]string, error),
) *RefSyncer {
	return &RefSyncer{DB: d, Media: m, Now: now, Logger: logger, Queries: queries, Current: current, Enqueue: SyncEnqueuer[K](jc)}
}

type syncWorker[K SyncKind] struct {
	river.WorkerDefaults[SyncArgs[K]]
	syncer *RefSyncer
}

// Work 在 transaction 外呼叫 media；失敗由 River 重試，需要再跑一輪時 snooze，一切就緒時在結算的 transaction 內完成。
func (w *syncWorker[K]) Work(ctx context.Context, job *river.Job[SyncArgs[K]]) error {
	return SyncWork(ctx, w.syncer, job, Ref{Type: job.Args.RefType, ID: job.Args.RefID})
}

// NextRetry 封頂重試間隔，讓 media 恢復後很快登記（GC 寬限之內）。
func (w *syncWorker[K]) NextRetry(job *river.Job[SyncArgs[K]]) time.Time {
	return SyncNextRetry(*job.AttemptedAt, job.Attempt)
}

// Timeout 是一次同步的時間上限。
func (w *syncWorker[K]) Timeout(*river.Job[SyncArgs[K]]) time.Duration { return SyncJobTimeout }

type sweepWorker[K SyncKind] struct {
	river.WorkerDefaults[SweepArgs[K]]
	syncer *RefSyncer
}

// Work 執行保底掃描。
func (w *sweepWorker[K]) Work(ctx context.Context, _ *river.Job[SweepArgs[K]]) error {
	_, err := w.syncer.Sweep(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *sweepWorker[K]) Timeout(*river.Job[SweepArgs[K]]) time.Duration { return time.Minute }

// SyncAddWorkers 登記擁有模組的同步與保底掃描 River worker。
func SyncAddWorkers[K SyncKind](workers *river.Workers, s *RefSyncer) {
	river.AddWorker(workers, &syncWorker[K]{syncer: s})
	river.AddWorker(workers, &sweepWorker[K]{syncer: s})
}

// SyncPeriodicJobs 回傳啟動即執行、每 15 分鐘的保底掃描。
func SyncPeriodicJobs[K SyncKind]() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(SyncSweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs[K]{}, nil },
		&river.PeriodicJobOpts{ID: kindPrefix[K]() + ".media_sweep", RunOnStart: true},
	)}
}

type (
	refKey = struct {
		RefType string
		RefID   string
	}
	retireParams = struct {
		RefType string
		RefID   string
		AssetID string
		Now     time.Time
	}
	bumpParams = struct {
		RefType string
		RefID   string
		Now     time.Time
	}
	assetsParams = struct {
		RefType  string
		RefID    string
		AssetIds []string
	}
	unlinkParams = struct {
		Now      *time.Time
		RefType  string
		RefID    string
		AssetIds []string
	}
	settleParams = struct {
		Seq     int64
		RefType string
		RefID   string
	}
	healingParams = struct {
		RefType    string
		RefID      string
		HealBefore *time.Time
	}
	purgeParams = struct {
		HealBefore   *time.Time
		RetireBefore time.Time
	}
	unsyncedParams = struct {
		IdleBefore time.Time
		MaxRows    int32
	}
	unsyncedRow = struct {
		RefType   string
		RefID     string
		UpdatedAt time.Time
	}
)

// SyncSQL 是擁有模組的 sqlc 查詢（*store.Queries）。各模組的參數型別同名且欄位相同，所以用結構轉換接成 SyncQueries，
// 型別參數由方法簽章推得，呼叫端只寫 NewSyncQueries(store.New(c))。
type SyncSQL[RA ~retireParams, BU ~bumpParams, RS ~refKey, RI ~refKey, RM ~assetsParams, MU ~unlinkParams, SR ~settleParams, HR ~healingParams, PR ~purgeParams, US ~unsyncedParams, UR ~unsyncedRow] interface {
	RetireAsset(ctx context.Context, arg RA) error
	BumpRefSeq(ctx context.Context, arg BU) error
	RefSeq(ctx context.Context, arg RS) (int64, error)
	RetiredAssetIDs(ctx context.Context, arg RI) ([]string, error)
	RetiredAmong(ctx context.Context, arg RM) ([]string, error)
	MarkUnlinked(ctx context.Context, arg MU) error
	SettleRefSeq(ctx context.Context, arg SR) (int64, error)
	HealingRetired(ctx context.Context, arg HR) (bool, error)
	PurgeRetired(ctx context.Context, arg PR) error
	UnsyncedRefs(ctx context.Context, arg US) ([]UR, error)
}

type syncAdapter[RA ~retireParams, BU ~bumpParams, RS ~refKey, RI ~refKey, RM ~assetsParams, MU ~unlinkParams, SR ~settleParams, HR ~healingParams, PR ~purgeParams, US ~unsyncedParams, UR ~unsyncedRow] struct {
	q SyncSQL[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]
}

// NewSyncQueries 把模組的 sqlc 查詢接成 SyncQueries。
func NewSyncQueries[RA ~retireParams, BU ~bumpParams, RS ~refKey, RI ~refKey, RM ~assetsParams, MU ~unlinkParams, SR ~settleParams, HR ~healingParams, PR ~purgeParams, US ~unsyncedParams, UR ~unsyncedRow](q SyncSQL[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) SyncQueries {
	return syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]{q}
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) RetireAsset(ctx context.Context, ref Ref, assetID string, now time.Time) error {
	return a.q.RetireAsset(ctx, RA(retireParams{ref.Type, ref.ID, assetID, now}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) BumpRefSeq(ctx context.Context, ref Ref, now time.Time) error {
	return a.q.BumpRefSeq(ctx, BU(bumpParams{ref.Type, ref.ID, now}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) RefSeq(ctx context.Context, ref Ref) (int64, bool, error) {
	seq, err := a.q.RefSeq(ctx, RS(refKey{ref.Type, ref.ID}))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return seq, err == nil, err
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) RetiredAssetIDs(ctx context.Context, ref Ref) ([]string, error) {
	return a.q.RetiredAssetIDs(ctx, RI(refKey{ref.Type, ref.ID}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) RetiredAmong(ctx context.Context, ref Ref, assetIDs []string) ([]string, error) {
	return a.q.RetiredAmong(ctx, RM(assetsParams{ref.Type, ref.ID, assetIDs}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) MarkUnlinked(ctx context.Context, ref Ref, assetIDs []string, now time.Time) error {
	return a.q.MarkUnlinked(ctx, MU(unlinkParams{&now, ref.Type, ref.ID, assetIDs}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) SettleRefSeq(ctx context.Context, ref Ref, seq int64) (int64, error) {
	return a.q.SettleRefSeq(ctx, SR(settleParams{seq, ref.Type, ref.ID}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) HealingRetired(ctx context.Context, ref Ref, healBefore time.Time) (bool, error) {
	return a.q.HealingRetired(ctx, HR(healingParams{ref.Type, ref.ID, &healBefore}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) PurgeRetired(ctx context.Context, healBefore, retireBefore time.Time) error {
	return a.q.PurgeRetired(ctx, PR(purgeParams{&healBefore, retireBefore}))
}

func (a syncAdapter[RA, BU, RS, RI, RM, MU, SR, HR, PR, US, UR]) UnsyncedRefs(ctx context.Context, idleBefore time.Time, limit int32) ([]SyncUnsynced, error) {
	rows, err := a.q.UnsyncedRefs(ctx, US(unsyncedParams{idleBefore, limit}))
	if err != nil {
		return nil, err
	}
	out := make([]SyncUnsynced, len(rows))
	for i, r := range rows {
		row := unsyncedRow(r)
		out[i] = SyncUnsynced{Ref: Ref{Type: row.RefType, ID: row.RefID}, UpdatedAt: row.UpdatedAt}
	}
	return out, nil
}
