// Package mediaref 維護買家頭像在 media 的引用。引用只由這個 ref 的同步 job（SyncArgs）寫入：換頭像與關閉在 transaction
// 內寫買家列、把被換掉的頭像記成退場並排入同步；同步 job 依買家列目前的狀態 Link，再精確 Unlink 退場的資產。演算法、seq 協定
// 與保底掃描在 media 模組（media.RefSyncer），這裡只提供本模組 schema 的 SQL、目前頭像的讀取與 job 的外殼。
package mediaref

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// SyncArgs 同步一個 ref 的 media 引用，以 (ref_type, ref_id) 為唯一鍵：同一個 ref 同時最多一個排隊中或執行中的 job。
type SyncArgs struct {
	RefType string `json:"ref_type" river:"unique"`
	RefID   string `json:"ref_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (SyncArgs) Kind() string { return "buyer.media_sync" }

// InsertOpts 是同步 job 的排入選項。
func (SyncArgs) InsertOpts() river.InsertOpts { return media.SyncInsertOpts() }

// SweepArgs 是保底掃描：清掉過期的退場紀錄，補排版本比已同步的新的 ref。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "buyer.media_sweep" }

// InsertOpts 把同一個週期內的排程與重啟合併。
func (SweepArgs) InsertOpts() river.InsertOpts { return media.SyncSweepInsertOpts() }

// AvatarRef 是買家頭像的 ref。
func AvatarRef(buyerID int64) media.Ref {
	return media.Ref{Type: media.RefTypeBuyerAvatar, ID: strconv.FormatInt(buyerID, 10)}
}

// Enqueuer 回傳排入 ref 的同步 job 的函式（ctx 內有 transaction 時在該 transaction 內）。
func Enqueuer(jc jobs.Enqueuer) func(context.Context, media.Ref) error {
	return func(ctx context.Context, ref media.Ref) error {
		opts := SyncArgs{}.InsertOpts()
		return jc.Enqueue(ctx, SyncArgs{RefType: ref.Type, RefID: ref.ID}, &opts)
	}
}

// Changed 在寫入 ref 的業務列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job。必須在 transaction 內。
func Changed(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, now time.Time, ref media.Ref, retired ...string) error {
	return media.SyncChanged(ctx, queries{q}, Enqueuer(jc), now, ref, retired...)
}

// Reject 拒絕把已退場的資產寫回 ref；必須在鎖住業務列的 transaction 內呼叫。
func Reject(ctx context.Context, q *store.Queries, ref media.Ref, candidates ...media.SyncCandidate) error {
	return media.SyncReject(ctx, queries{q}, ref, candidates...)
}

// NewSyncer 組出本模組的同步。
func NewSyncer(d *db.DB, m media.Media, jc jobs.Enqueuer, now func() time.Time, logger *slog.Logger) *media.RefSyncer {
	return &media.RefSyncer{
		DB: d, Media: m, Now: now, Logger: logger,
		Queries: func(c db.DBTX) media.SyncQueries { return queries{store.New(c)} },
		Current: func(ctx context.Context, ref media.Ref) ([]string, error) {
			if ref.Type != media.RefTypeBuyerAvatar {
				return nil, fmt.Errorf("buyer：不認得的 media ref 類型 %s", ref.Type)
			}
			buyerID, err := strconv.ParseInt(ref.ID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("buyer：media ref 的 ID %q 不是買家 ID：%w", ref.ID, err)
			}
			row, err := store.New(d.Conn(ctx)).AvatarAsset(ctx, buyerID)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				return nil, nil
			case err != nil:
				return nil, err
			case row.Status != string(model.Closed) && row.AvatarAssetID != nil:
				return []string{*row.AvatarAssetID}, nil
			}
			return nil, nil
		},
		Enqueue: Enqueuer(jc),
	}
}

type worker struct {
	river.WorkerDefaults[SyncArgs]
	syncer *media.RefSyncer
}

// Work 在 transaction 外呼叫 media；失敗由 River 重試，需要再跑一輪時 snooze，一切就緒時在結算的 transaction 內完成。
func (w *worker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	return media.SyncWork(ctx, w.syncer, job, media.Ref{Type: job.Args.RefType, ID: job.Args.RefID})
}

// NextRetry 封頂重試間隔，讓 media 恢復後很快登記（GC 寬限之內）。
func (w *worker) NextRetry(job *river.Job[SyncArgs]) time.Time {
	return media.SyncNextRetry(*job.AttemptedAt, job.Attempt)
}

// Timeout 是一次同步的時間上限。
func (w *worker) Timeout(*river.Job[SyncArgs]) time.Duration { return media.SyncJobTimeout }

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	syncer *media.RefSyncer
}

// Work 執行保底掃描。
func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := w.syncer.Sweep(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return time.Minute }

// AddWorkers 登記 River worker。
func AddWorkers(workers *river.Workers, s *media.RefSyncer) {
	river.AddWorker(workers, &worker{syncer: s})
	river.AddWorker(workers, &sweepWorker{syncer: s})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的保底掃描。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(media.SyncSweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "buyer.media_sweep", RunOnStart: true},
	)}
}

// queries 把 sqlc 的查詢接成 media.SyncQueries。
type queries struct{ q *store.Queries }

func (a queries) RetireAsset(ctx context.Context, ref media.Ref, assetID string, now time.Time) error {
	return a.q.RetireAsset(ctx, store.RetireAssetParams{RefType: ref.Type, RefID: ref.ID, AssetID: assetID, Now: now})
}

func (a queries) BumpRefSeq(ctx context.Context, ref media.Ref, now time.Time) error {
	return a.q.BumpRefSeq(ctx, store.BumpRefSeqParams{RefType: ref.Type, RefID: ref.ID, Now: now})
}

func (a queries) RefSeq(ctx context.Context, ref media.Ref) (int64, bool, error) {
	seq, err := a.q.RefSeq(ctx, store.RefSeqParams{RefType: ref.Type, RefID: ref.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return seq, err == nil, err
}

func (a queries) RetiredAssetIDs(ctx context.Context, ref media.Ref) ([]string, error) {
	return a.q.RetiredAssetIDs(ctx, store.RetiredAssetIDsParams{RefType: ref.Type, RefID: ref.ID})
}

func (a queries) RetiredAmong(ctx context.Context, ref media.Ref, assetIDs []string) ([]string, error) {
	return a.q.RetiredAmong(ctx, store.RetiredAmongParams{RefType: ref.Type, RefID: ref.ID, AssetIds: assetIDs})
}

func (a queries) MarkUnlinked(ctx context.Context, ref media.Ref, assetIDs []string, now time.Time) error {
	return a.q.MarkUnlinked(ctx, store.MarkUnlinkedParams{RefType: ref.Type, RefID: ref.ID, AssetIds: assetIDs, Now: &now})
}

func (a queries) SettleRefSeq(ctx context.Context, ref media.Ref, seq int64) (int64, error) {
	return a.q.SettleRefSeq(ctx, store.SettleRefSeqParams{RefType: ref.Type, RefID: ref.ID, Seq: seq})
}

func (a queries) HealingRetired(ctx context.Context, ref media.Ref, healBefore time.Time) (bool, error) {
	return a.q.HealingRetired(ctx, store.HealingRetiredParams{RefType: ref.Type, RefID: ref.ID, HealBefore: &healBefore})
}

func (a queries) PurgeRetired(ctx context.Context, healBefore, retireBefore time.Time) error {
	return a.q.PurgeRetired(ctx, store.PurgeRetiredParams{HealBefore: &healBefore, RetireBefore: retireBefore})
}

func (a queries) UnsyncedRefs(ctx context.Context, idleBefore time.Time, limit int32) ([]media.SyncUnsynced, error) {
	rows, err := a.q.UnsyncedRefs(ctx, store.UnsyncedRefsParams{IdleBefore: idleBefore, MaxRows: limit})
	if err != nil {
		return nil, err
	}
	out := make([]media.SyncUnsynced, len(rows))
	for i, r := range rows {
		out[i] = media.SyncUnsynced{Ref: media.Ref{Type: r.RefType, ID: r.RefID}, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}
