// Package reclaim 是物件的回收：擁有者的 MarkDeleted、暫存 GC 與實體刪除 job。
//
// 每一列都走三段（單體規則 5）：transaction 內標為 DELETING 並提交 → transaction 外刪物件（不存在視為成功）→
// 另一個 transaction 標為 DELETED。刪物件失敗時列留在 DELETING，下一輪從頭再試，所以不會有「列已標刪、物件還在卻沒人收」，
// 也不會有指著不存在物件的有效列（資產先離開 ACTIVE 才刪物件）。GC 只會標記過期、未確認的暫存（PENDING、UPLOADING、READY、REJECTED，以及過期超過 model.StuckUploadGrace 仍停在 UPLOADED 的列）；
// 已確認的資產只由擁有者的 MarkDeleted 刪除。
package reclaim

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// batchSize 是每一批標記或刪除的列數。
const batchSize = 200

// SweepInterval 是暫存 GC（同時補掃所有 DELETING 的列）的間隔。
const SweepInterval = 5 * time.Minute

// Deps 是回收的依賴。
type Deps struct {
	DB     *db.DB
	Jobs   jobs.Enqueuer
	Store  storage.Store
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 執行回收。
type Service struct{ d Deps }

// New 建立服務；必要的依賴缺少時 panic。
func New(d Deps) *Service {
	if d.DB == nil || d.Jobs == nil || d.Store == nil || d.Now == nil {
		panic("media/reclaim：DB、Jobs、Store 與 Now 是必要的")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// DeleteAssetsArgs 是實體刪除 job 的參數（job 種類 media.delete_assets）：處理所有 DELETING 的資產與暫存列。
type DeleteAssetsArgs struct{}

// Kind 是 River 的 job 種類。
func (DeleteAssetsArgs) Kind() string { return "media.delete_assets" }

// MaxAttempts 是刪除與 GC job 的重試上限；用盡後由下一輪 GC 補掃。
const MaxAttempts = 10

// InsertOpts 讓還沒結束的刪除 job 合併：正在執行的那一輪之後才標記的列，由下一輪 GC 補掃。
func (DeleteAssetsArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(MaxAttempts) }

// MarkDeleted 是擁有者的刪除指令：在擁有者的 transaction 內把 ACTIVE 的資產轉 DELETING 並排實體刪除 job，物件由 job 在 transaction 外刪。
// 冪等；重複的、不存在的、未確認的、已在刪除中的 ID 都忽略（未確認的暫存只由 GC 刪）。ctx 必須帶 transaction。
func (s *Service) MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error {
	if err := model.RequireTx(ctx); err != nil {
		return err
	}
	ids := slices.Clone(assetIDs)
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return nil
	}
	marked, err := s.q(ctx).MarkAssetsDeleting(ctx, store.MarkAssetsDeletingParams{Ids: ids, Now: s.d.Now()})
	if err != nil {
		return fmt.Errorf("media：標記資產刪除：%w", err)
	}
	if len(marked) == 0 {
		return nil
	}
	opts := DeleteAssetsArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, DeleteAssetsArgs{}, &opts)
}

// Result 是一輪回收的結果。
type Result struct {
	// Claimed 是這一輪 GC 標記的暫存列數。
	Claimed int
	// Deleted 是這一輪標為 DELETED 的暫存列與資產列數。
	Deleted int
}

// CollectGarbage 是暫存 GC：階段一標記過期且未確認的暫存列（含卡在 UPLOADED 過寬限的）（連同它們的資產列），之後同 DeleteAssets 刪物件並標為已刪。
// 不得在 transaction 內呼叫。
func (s *Service) CollectGarbage(ctx context.Context) (Result, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return Result{}, err
	}
	var res Result
	for {
		var n int
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			now := s.d.Now()
			claimed, err := s.q(ctx).ClaimExpiredUploads(ctx, store.ClaimExpiredUploadsParams{
				Now: now, UploadedBefore: now.Add(-model.StuckUploadGrace), MaxRows: batchSize,
			})
			if err != nil {
				return fmt.Errorf("media：標記過期暫存：%w", err)
			}
			n = len(claimed)
			var assetIDs []uuid.UUID
			for _, c := range claimed {
				if c.AssetID != nil {
					assetIDs = append(assetIDs, *c.AssetID)
				}
			}
			if len(assetIDs) == 0 {
				return nil
			}
			if _, err := s.q(ctx).ClaimAssetsOfUploads(ctx, store.ClaimAssetsOfUploadsParams{Ids: assetIDs, Now: now}); err != nil {
				return fmt.Errorf("media：標記暫存的資產：%w", err)
			}
			return nil
		})
		if err != nil {
			return res, err
		}
		res.Claimed += n
		if n < batchSize {
			break
		}
	}
	deleted, err := s.DeleteAssets(ctx)
	res.Deleted = deleted
	return res, err
}

// DeleteAssets 是實體刪除：處理所有 DELETING 的暫存列與資產，階段二在 transaction 外刪物件（暫存列刪 tmp 與正式 key），階段三另開
// transaction 標為 DELETED。單列失敗不影響其他列，失敗的列留在 DELETING，錯誤合併後回傳讓 job 重試。不得在 transaction 內呼叫。
func (s *Service) DeleteAssets(ctx context.Context) (int, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return 0, err
	}
	var errs []error
	total := 0
	after := uuid.UUID{}
	for {
		rows, err := s.q(ctx).ListDeletingUploads(ctx, store.ListDeletingUploadsParams{AfterID: after, MaxRows: batchSize})
		if err != nil {
			return total, errors.Join(append(errs, fmt.Errorf("media：列出刪除中的暫存：%w", err))...)
		}
		for _, r := range rows {
			if err := s.deleteUpload(ctx, r); err != nil {
				errs = append(errs, err)
				continue
			}
			total++
		}
		if len(rows) < batchSize {
			break
		}
		after = rows[len(rows)-1].ID
	}
	after = uuid.UUID{}
	for {
		rows, err := s.q(ctx).ListDeletingAssets(ctx, store.ListDeletingAssetsParams{AfterID: after, MaxRows: batchSize})
		if err != nil {
			return total, errors.Join(append(errs, fmt.Errorf("media：列出刪除中的資產：%w", err))...)
		}
		for _, r := range rows {
			if err := s.deleteAsset(ctx, r); err != nil {
				errs = append(errs, err)
				continue
			}
			total++
		}
		if len(rows) < batchSize {
			break
		}
		after = rows[len(rows)-1].ID
	}
	return total, errors.Join(errs...)
}

// deleteSecureObjects 刪機敏資產的所有段物件：secure_chunks 記錄的 key，加上前綴 secure/<資產 ID>/ 下列得出的所有物件
// （沒推進成功的孤兒嘗試、輸家沒刪掉的物件）。不存在視為成功；任何一個失敗就回錯誤，列留在 DELETING 下一輪重試。
func (s *Service) deleteSecureObjects(ctx context.Context, assetID uuid.UUID) error {
	chunks, err := s.q(ctx).ListSecureChunks(ctx, assetID)
	if err != nil {
		return fmt.Errorf("media：列出資產 %s 的段：%w", assetID, err)
	}
	listed, err := s.d.Store.List(ctx, storage.SecurePrefix(assetID))
	if err != nil {
		return fmt.Errorf("media：列出資產 %s 的物件：%w", assetID, err)
	}
	keys := slices.Clone(listed)
	for _, c := range chunks {
		keys = append(keys, c.ObjectKey)
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	for _, k := range keys {
		if err := s.d.Store.Delete(ctx, k); err != nil {
			return fmt.Errorf("media：刪除資產 %s 的物件 %s：%w", assetID, k, err)
		}
	}
	return nil
}

func (s *Service) deleteUpload(ctx context.Context, r store.ListDeletingUploadsRow) error {
	if r.PlannedAssetID != nil {
		if err := s.deleteSecureObjects(ctx, *r.PlannedAssetID); err != nil {
			return fmt.Errorf("media：刪除暫存 %s 的機敏物件：%w", r.ID, err)
		}
	}
	var keys []string
	if r.TmpKey != nil {
		keys = append(keys, *r.TmpKey)
	}
	if r.FinalKey != nil && r.PlannedAssetID == nil {
		keys = append(keys, *r.FinalKey)
	}
	for _, k := range keys {
		if err := s.d.Store.Delete(ctx, k); err != nil {
			return fmt.Errorf("media：刪除暫存 %s 的物件 %s：%w", r.ID, k, err)
		}
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.q(ctx).MarkUploadDeleted(ctx, store.MarkUploadDeletedParams{ID: r.ID, Now: s.d.Now()}); err != nil {
			return fmt.Errorf("media：標記暫存已刪：%w", err)
		}
		return nil
	})
}

func (s *Service) deleteAsset(ctx context.Context, r store.ListDeletingAssetsRow) error {
	if model.Visibility(r.Visibility) == model.VisibilitySecure {
		// 機敏資產的 object_key 是前綴，不是單一物件。
		if err := s.deleteSecureObjects(ctx, r.ID); err != nil {
			return err
		}
	} else if err := s.d.Store.Delete(ctx, r.ObjectKey); err != nil {
		return fmt.Errorf("media：刪除資產 %s 的物件：%w", r.ID, err)
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.q(ctx).MarkAssetDeleted(ctx, store.MarkAssetDeletedParams{ID: r.ID, Now: s.d.Now()}); err != nil {
			return fmt.Errorf("media：標記資產已刪：%w", err)
		}
		return nil
	})
}

type deleteWorker struct {
	river.WorkerDefaults[DeleteAssetsArgs]
	svc *Service
}

// Work 刪除所有標記中的物件；失敗由 River 重試（冪等）。
func (w *deleteWorker) Work(ctx context.Context, _ *river.Job[DeleteAssetsArgs]) error {
	_, err := w.svc.DeleteAssets(ctx)
	return err
}

// Timeout 是一次刪除的時間上限。
func (w *deleteWorker) Timeout(*river.Job[DeleteAssetsArgs]) time.Duration { return 5 * time.Minute }

// GCArgs 是暫存 GC 的排程 job（種類 media.gc_uploads）。
type GCArgs struct{}

// Kind 是 River 的 job 種類。
func (GCArgs) Kind() string { return "media.gc_uploads" }

// InsertOpts 以 SweepInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併。
func (GCArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(SweepInterval)
	o.MaxAttempts = MaxAttempts
	return o
}

type gcWorker struct {
	river.WorkerDefaults[GCArgs]
	svc *Service
}

// Work 執行一輪 GC；失敗由 River 重試，之後的排程也會補掃。
func (w *gcWorker) Work(ctx context.Context, _ *river.Job[GCArgs]) error {
	_, err := w.svc.CollectGarbage(ctx)
	return err
}

// Timeout 是一輪 GC 的時間上限。
func (w *gcWorker) Timeout(*river.Job[GCArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記實體刪除與 GC 的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &deleteWorker{svc: s})
	river.AddWorker(workers, &gcWorker{svc: s})
}

// PeriodicJobs 回傳啟動即執行、每 SweepInterval 的 GC 排程。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return GCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "media.gc_uploads", RunOnStart: true},
		),
	}
}
