// Package mediaref 維護貼文圖片在 media 的引用。引用只由這個 ref 的同步 job（SyncArgs）寫入：新增、換圖與店家退場的清除在
// transaction 內寫業務列、把被換掉的圖記成退場並排入同步；同步 job 依業務列目前的狀態 Link，再精確 Unlink 退場的資產。演算法、
// seq 協定與保底掃描在 media 模組（media.RefSyncer），這裡只提供本模組 schema 的 SQL、目前資產的讀取與 job 的外殼。
package mediaref

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// PostRef 是貼文圖的 ref（一篇貼文一張圖）。
func PostRef(id uuid.UUID) media.Ref { return media.Ref{Type: media.RefTypePost, ID: id.String()} }

// currentAssets 是業務列目前指著的資產；貼文沒有圖或已經清除（店家退場）的 ref 是終態，沒有目前的資產。
func currentAssets(ctx context.Context, q *store.Queries, ref media.Ref) ([]string, error) {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return nil, fmt.Errorf("social：media ref 的 ID %q 不是 UUID：%w", ref.ID, err)
	}
	if ref.Type != media.RefTypePost {
		return nil, fmt.Errorf("social：不認得的 media ref 類型 %s", ref.Type)
	}
	asset, err := q.PostAsset(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []string{*asset}, nil
}

type prefix struct{}

// KindPrefix 是本模組 River job 種類的前綴。
func (prefix) KindPrefix() string { return "social" }

// SyncArgs 同步一個 ref 的 media 引用（種類 social.media_sync）。
type SyncArgs = media.SyncArgs[prefix]

// SweepArgs 是保底掃描（種類 social.media_sweep）。
type SweepArgs = media.SweepArgs[prefix]

// NewSyncer 組出本模組的同步。
func NewSyncer(d *db.DB, m media.Media, jc jobs.Enqueuer, now func() time.Time, logger *slog.Logger) *media.RefSyncer {
	return media.NewRefSyncer[prefix](d, m, jc, now, logger,
		func(c db.DBTX) media.SyncQueries { return media.NewSyncQueries(store.New(c)) },
		func(ctx context.Context, ref media.Ref) ([]string, error) {
			return currentAssets(ctx, store.New(d.Conn(ctx)), ref)
		},
	)
}

// AddWorkers 登記 River worker。
func AddWorkers(workers *river.Workers, s *media.RefSyncer) { media.SyncAddWorkers[prefix](workers, s) }

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的保底掃描。
func PeriodicJobs() []*river.PeriodicJob { return media.SyncPeriodicJobs[prefix]() }

// Changed 在寫入 ref 的業務列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job。必須在 transaction 內。
func Changed(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, now time.Time, ref media.Ref, retired ...string) error {
	return media.SyncChanged(ctx, media.NewSyncQueries(q), media.SyncEnqueuer[prefix](jc), now, ref, retired...)
}

// Reject 拒絕把已退場的資產寫回 ref；必須在鎖住業務列的 transaction 內呼叫。
func Reject(ctx context.Context, q *store.Queries, ref media.Ref, candidates ...media.SyncCandidate) error {
	return media.SyncReject(ctx, media.NewSyncQueries(q), ref, candidates...)
}
