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

// AvatarRef 是買家頭像的 ref。
func AvatarRef(buyerID int64) media.Ref {
	return media.Ref{Type: media.RefTypeBuyerAvatar, ID: strconv.FormatInt(buyerID, 10)}
}

type prefix struct{}

// KindPrefix 是本模組 River job 種類的前綴。
func (prefix) KindPrefix() string { return "buyer" }

// SyncArgs 同步一個 ref 的 media 引用（種類 buyer.media_sync）。
type SyncArgs = media.SyncArgs[prefix]

// SweepArgs 是保底掃描（種類 buyer.media_sweep）。
type SweepArgs = media.SweepArgs[prefix]

// NewSyncer 組出本模組的同步。
func NewSyncer(d *db.DB, m media.Media, jc jobs.Enqueuer, now func() time.Time, logger *slog.Logger) *media.RefSyncer {
	return media.NewRefSyncer[prefix](d, m, jc, now, logger,
		func(c db.DBTX) media.SyncQueries { return media.NewSyncQueries(store.New(c)) },
		func(ctx context.Context, ref media.Ref) ([]string, error) {
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
