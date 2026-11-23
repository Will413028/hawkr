// Package mediaref 維護檢舉快照引用的圖片在 media 的引用（A7-D21 證據保全）：快照存在的期間，它引用的圖片在 media 有一筆以快照 ID 為 ref 的引用，
// 所以內容的作者把圖換掉、刪貼文或店家退場而釋放自己的引用時，圖片不會被 media 的 GC 回收；快照刪除時才解除這筆引用。
//
// 引用只由這個 ref 的同步 job（SyncArgs）寫入：快照寫入與刪除都在 transaction 內記下資產（Hold、Release），同步 job 依快照目前的狀態 Link，再精確 Unlink 釋放的資產。
// 演算法、seq 協定與保底掃描在 media 模組（media.RefSyncer），這裡只提供本模組 schema 的 SQL、目前資產的讀取與 job 的外殼。
// ref 的型別借用 RefTypeGovernanceEvidence（治理的佐證圖片引用），ID 是快照的 UUID。
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
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// SnapshotRef 是快照的 ref。
func SnapshotRef(id uuid.UUID) media.Ref {
	return media.Ref{Type: media.RefTypeGovernanceEvidence, ID: id.String()}
}

// Holds 在呼叫方的 transaction 內記下快照引用或釋放的資產並排入同步 job。
type Holds struct {
	DB   *db.DB
	Jobs jobs.Enqueuer
}

// Hold 登記新快照引用的資產：沒有資產時什麼都不做；必須在 transaction 內。
func (h Holds) Hold(ctx context.Context, snapshotID uuid.UUID, now time.Time, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	return media.SyncChanged(ctx, media.NewSyncQueries(store.New(h.DB.Conn(ctx))), media.SyncEnqueuer[prefix](h.Jobs), now, SnapshotRef(snapshotID))
}

// Release 在快照刪除的 transaction 內把它引用的資產記成退場並排入同步 job：同步 job 解除這些資產的引用；沒有資產時什麼都不做。
func (h Holds) Release(ctx context.Context, q *store.Queries, snapshotID uuid.UUID, now time.Time, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	return media.SyncChanged(ctx, media.NewSyncQueries(q), media.SyncEnqueuer[prefix](h.Jobs), now, SnapshotRef(snapshotID), assetIDs...)
}

// currentAssets 是快照目前引用的資產；快照已刪除（終態）的 ref 沒有目前的資產。
func currentAssets(ctx context.Context, q *store.Queries, ref media.Ref) ([]string, error) {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return nil, fmt.Errorf("moderation：media ref 的 ID %q 不是 UUID：%w", ref.ID, err)
	}
	if ref.Type != media.RefTypeGovernanceEvidence {
		return nil, fmt.Errorf("moderation：不認得的 media ref 類型 %s", ref.Type)
	}
	assets, err := q.SnapshotAssets(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return assets, err
}

type prefix struct{}

// KindPrefix 是本模組 River job 種類的前綴。
func (prefix) KindPrefix() string { return "moderation" }

// SyncArgs 同步一個 ref 的 media 引用（種類 moderation.media_sync）。
type SyncArgs = media.SyncArgs[prefix]

// SweepArgs 是保底掃描（種類 moderation.media_sweep）。
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
