// Package mediaref 維護風格縮圖在 media 的引用（做法同 content 的 banner，ADR 2026-10-03-media-references-owner-side）。引用只由這個 ref 的同步 job（SyncArgs）寫入：
// 新增、換圖與刪除在 transaction 內寫業務列、把被換掉的圖記成退場並排入同步；同步 job 依業務列目前的狀態 Link，再精確 Unlink 退場的資產。演算法、
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

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// StyleRef 是一個風格縮圖的 ref。
func StyleRef(id uuid.UUID) media.Ref { return media.Ref{Type: media.RefTypeAiStyle, ID: id.String()} }

// currentAssets 是業務列目前指著的資產；業務列不存在（已刪除）的 ref 是終態，沒有目前的資產；沒有縮圖的風格（seed 的預設風格）也沒有資產。
func currentAssets(ctx context.Context, q *store.Queries, ref media.Ref) ([]string, error) {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return nil, fmt.Errorf("imageai：media ref 的 ID %q 不是 UUID：%w", ref.ID, err)
	}
	if ref.Type != media.RefTypeAiStyle {
		return nil, fmt.Errorf("imageai：不認得的 media ref 類型 %s", ref.Type)
	}
	asset, err := q.StyleThumbnailAsset(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && asset == "") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []string{asset}, nil
}

type prefix struct{}

// KindPrefix 是本模組 River job 種類的前綴。
func (prefix) KindPrefix() string { return "imageai" }

// SyncArgs 同步一個 ref 的 media 引用（種類 imageai.media_sync）。
type SyncArgs = media.SyncArgs[prefix]

// SweepArgs 是保底掃描（種類 imageai.media_sweep）。
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

// Thumbnails 是風格縮圖在 media 的轉正與引用登記（internal/admin 的 Thumbnails 介面的實作）。
type Thumbnails struct {
	DB    *db.DB
	Media media.Media
	Jobs  jobs.Enqueuer
}

// ThumbnailField 是縮圖的輸入欄位名（模組輸入的 ThumbnailToken）。
const ThumbnailField = "ThumbnailToken"

// Finalize 把新上傳的縮圖轉正（transaction 外，不建立引用）；token 不可用回 InvalidInput（欄位 ThumbnailToken，message 是給使用者的文字）。
func (t *Thumbnails) Finalize(ctx context.Context, adminID int64, token, message string) (key, assetID string, err error) {
	a, err := media.FinalizeField(ctx, t.Media, media.Uploader{ID: adminID, Admin: true}, media.Field{Name: ThumbnailField, Message: message}, token, media.RefTypeAiStyle)
	if err != nil {
		return "", "", fmt.Errorf("imageai：轉正縮圖：%w", err)
	}
	return a.ObjectKey, a.ID, nil
}

// Reject 拒絕把已退場的資產寫回風格的 ref；必須在鎖住風格列的 transaction 內呼叫。
func (t *Thumbnails) Reject(ctx context.Context, styleID uuid.UUID, assetID string) error {
	return media.SyncReject(ctx, media.NewSyncQueries(store.New(t.DB.Conn(ctx))), StyleRef(styleID), media.SyncCandidate{Field: ThumbnailField, AssetID: assetID})
}

// Changed 在寫入風格列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job。必須在 transaction 內。
func (t *Thumbnails) Changed(ctx context.Context, now time.Time, styleID uuid.UUID, retired ...string) error {
	return media.SyncChanged(ctx, media.NewSyncQueries(store.New(t.DB.Conn(ctx))), media.SyncEnqueuer[prefix](t.Jobs), now, StyleRef(styleID), retired...)
}
