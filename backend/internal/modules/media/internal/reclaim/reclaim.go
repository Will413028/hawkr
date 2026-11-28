// Package reclaim 是物件的回收：擁有者的 MarkDeleted（只新增刪除請求）、刪除請求的套用 job、暫存 GC 與實體刪除 job。
//
// 擁有者的 transaction 內只新增列（B1-D25）：MarkDeleted 寫 asset_deletion_requests，由 ApplyDeletions 在 media 自己的 transaction 內把資產標為 DELETING。
// 每一列都走三段（單體規則 5）：transaction 內標為 DELETING 並提交 → transaction 外刪物件（不存在視為成功）→
// 另一個 transaction 標為 DELETED（並刪掉它的認領與請求列）。刪物件失敗時列留在 DELETING，下一輪從頭再試，所以不會有「列已標刪、物件還在卻沒人收」，
// 也不會有指著不存在物件的有效列（資產先離開 ACTIVE 才刪物件）。GC 只會認領過期、未確認的暫存（PENDING、UPLOADING、READY、REJECTED，以及過期超過 model.StuckUploadGrace 仍停在 UPLOADED 的列）；
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

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/hold"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const batchSize = 200

// SweepInterval 是暫存 GC（同時補掃所有 DELETING 的列）的間隔。
const SweepInterval = 5 * time.Minute

// ApplyInterval 是刪除請求的套用間隔（B1-D25）：擁有者 commit 之後，物件最晚在這個間隔加一輪 DeleteAssets 內被刪；部分索引下空查詢的成本可忽略。
const ApplyInterval = 30 * time.Second

// GCLockTimeout 是 GC 每個 transaction 的鎖等待上限：GC 絕不等擁有者還沒提交的認領（多 token 的擁有者 transaction 可能同時握著別的列），
// 逾時就跳過那一列、留給下一輪，所以 GC 與擁有者之間不會死結、也不會被拖住。
const GCLockTimeout = hold.LockTimeout

var errUploadHeld = errors.New("media：暫存的資產有保全，或暫存列已被別處轉走，這一輪不回收")

// Deps 是回收的依賴。
type Deps struct {
	DB     *db.DB
	Store  storage.Store
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 執行回收。
type Service struct{ d Deps }

// New 建立服務；必要的依賴缺少時 panic。
func New(d Deps) *Service {
	if d.DB == nil || d.Store == nil || d.Now == nil {
		panic("media/reclaim：DB、Store 與 Now 是必要的")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// ApplyDeletionsArgs 是刪除請求套用與實體刪除 job 的參數（job 種類 media.apply_deletions）：先套用刪除請求，再處理所有 DELETING 的資產與暫存列。
type ApplyDeletionsArgs struct{}

// Kind 是 River 的 job 種類。
func (ApplyDeletionsArgs) Kind() string { return "media.apply_deletions" }

// MaxAttempts 是刪除與 GC job 的重試上限；用盡後由下一輪排程補掃。
const MaxAttempts = 10

// InsertOpts 以 ApplyInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併。
func (ApplyDeletionsArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(ApplyInterval)
	o.MaxAttempts = MaxAttempts
	return o
}

// MarkDeleted 是擁有者的刪除指令：在擁有者的 transaction 內只新增刪除請求列（B1-D25），不更新任何 media 既有的列、不排 job；
// 物件由 ApplyDeletions 套用後在 transaction 外刪。冪等；重複的、不存在的、未確認的、已請求過的 ID 都忽略（未確認的暫存只由 GC 刪）。
// 同 transaction 內剛 Confirm 的資產也算已確認（認領對自己可見）。ctx 必須帶 transaction。
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
	if _, err := s.q(ctx).RequestDeletions(ctx, store.RequestDeletionsParams{Ids: ids, Now: s.d.Now()}); err != nil {
		return fmt.Errorf("media：請求資產刪除：%w", err)
	}
	return nil
}

// ApplyDeletions 在 media 自己的 transaction 內把還沒套用的刪除請求所指的資產標為 DELETING（冪等，兩個 worker 並行安全），
// 並把已確認的 READY 暫存列物化成 CONFIRMED（settle，只影響 GC 的掃描量）。回傳這一輪套用的請求數。不得在 transaction 內呼叫。
func (s *Service) ApplyDeletions(ctx context.Context) (int, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return 0, err
	}
	total := 0
	after := store.ListPendingDeletionRequestsParams{MaxRows: batchSize}
	for {
		rows, err := s.q(ctx).ListPendingDeletionRequests(ctx, after)
		if err != nil {
			return total, fmt.Errorf("media：列出待套用的刪除請求：%w", err)
		}
		for _, r := range rows {
			n, err := s.applyRequest(ctx, r.AssetID)
			if err != nil {
				return total, err
			}
			total += n
		}
		if len(rows) < batchSize {
			break
		}
		last := rows[len(rows)-1]
		after.AfterRequestedAt, after.AfterID = last.RequestedAt, last.AssetID
	}
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			n, err = s.q(ctx).SettleConfirmedUploads(ctx, store.SettleConfirmedUploadsParams{Now: s.d.Now(), MaxRows: batchSize})
			return err
		})
		if err != nil {
			return total, fmt.Errorf("media：物化已確認的暫存：%w", err)
		}
		if n < batchSize {
			break
		}
	}
	return total, nil
}

func (s *Service) applyRequest(ctx context.Context, assetID uuid.UUID) (int, error) {
	applied := 0
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := hold.SetLockTimeout(ctx, s.d.DB.Conn(ctx)); err != nil {
			return err
		}
		q := s.q(ctx)
		now := s.d.Now()
		won, err := hold.Claim(ctx, q, assetID, now)
		if err != nil || !won {
			return err
		}
		n, err := q.ApplyDeletionRequest(ctx, store.ApplyDeletionRequestParams{AssetID: assetID, Now: now})
		applied = int(n)
		return err
	})
	if hold.IsContention(err) {
		s.d.Logger.Info("media：套用刪除請求撞上擁有者未提交的保全，跳過", "asset_id", assetID)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("media：套用資產 %s 的刪除請求：%w", assetID, err)
	}
	return applied, nil
}

// Result 是一輪回收的結果。
type Result struct {
	// Claimed 是這一輪 GC 認領的暫存列數。
	Claimed int
	// Deleted 是這一輪標為 DELETED 的暫存列與資產列數。
	Deleted int
}

// CollectGarbage 是暫存 GC：先補掃刪除請求，階段一認領過期且未確認的暫存列（含卡在 UPLOADED 過寬限的）（連同它們的資產列），之後同 DeleteAssets 刪物件並標為已刪。
// 認領與擁有者的 Confirm 搶同一個唯一鍵，每列各用一個帶 lock_timeout 的 transaction：撞上擁有者還沒提交的認領就跳過、留給下一輪。不得在 transaction 內呼叫。
func (s *Service) CollectGarbage(ctx context.Context) (Result, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return Result{}, err
	}
	var res Result
	if _, err := s.ApplyDeletions(ctx); err != nil {
		return res, err
	}
	afterExpires, afterID := time.Time{}, uuid.UUID{}
	for {
		now := s.d.Now()
		uploadedBefore := now.Add(-model.StuckUploadGrace)
		cands, err := s.q(ctx).ListReclaimableUploads(ctx, store.ListReclaimableUploadsParams{
			Now: now, UploadedBefore: uploadedBefore, AfterExpiresAt: afterExpires, AfterID: afterID, MaxRows: batchSize,
		})
		if err != nil {
			return res, fmt.Errorf("media：列出過期暫存：%w", err)
		}
		for _, c := range cands {
			won, err := s.reclaimUpload(ctx, c.ID, now, uploadedBefore)
			if err != nil {
				return res, err
			}
			if won {
				res.Claimed++
			}
		}
		if len(cands) < batchSize {
			break
		}
		last := cands[len(cands)-1]
		afterExpires, afterID = last.ExpiresAt, last.ID
	}
	deleted, err := s.DeleteAssets(ctx)
	res.Deleted = deleted
	return res, err
}

func (s *Service) reclaimUpload(ctx context.Context, id uuid.UUID, now, uploadedBefore time.Time) (bool, error) {
	won := false
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := hold.SetLockTimeout(ctx, s.d.DB.Conn(ctx)); err != nil {
			return err
		}
		q := s.q(ctx)
		if _, err := q.ReclaimUpload(ctx, store.ReclaimUploadParams{ID: id, Now: now, UploadedBefore: uploadedBefore}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		row, err := q.MarkReclaimedUploadDeleting(ctx, store.MarkReclaimedUploadDeletingParams{ID: id, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return errUploadHeld
		}
		if err != nil {
			return fmt.Errorf("media：暫存 %s 轉 DELETING：%w", id, err)
		}
		var gated []uuid.UUID
		for _, a := range []*uuid.UUID{row.AssetID, row.PlannedAssetID} {
			if a != nil {
				gated = append(gated, *a)
			}
		}
		clear, err := hold.ClaimAll(ctx, q, gated, now)
		if err != nil {
			return err
		}
		if !clear {
			return errUploadHeld
		}
		if row.AssetID != nil {
			if _, err := q.ClaimAssetsOfUploads(ctx, store.ClaimAssetsOfUploadsParams{Ids: []uuid.UUID{*row.AssetID}, Now: now}); err != nil {
				return fmt.Errorf("media：標記暫存的資產：%w", err)
			}
		}
		won = true
		return nil
	})
	if errors.Is(err, errUploadHeld) {
		return false, nil
	}
	if hold.IsContention(err) {
		s.d.Logger.Info("media：GC 撞上擁有者未提交的認領或保全，跳過", "upload_id", id)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("media：認領過期暫存 %s：%w", id, err)
	}
	return won, nil
}

// DeleteAssets 是實體刪除：處理所有 DELETING 的暫存列與資產，階段二在 transaction 外刪物件（暫存列刪 tmp 與正式 key），階段三另開
// transaction 標為 DELETED 並刪掉它的認領與請求列。單列失敗不影響其他列，失敗的列留在 DELETING，錯誤合併後回傳讓 job 重試。不得在 transaction 內呼叫。
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
			deleted, err := s.deleteUpload(ctx, r)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if deleted {
				total++
			}
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
			deleted, err := s.deleteAsset(ctx, r)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if deleted {
				total++
			}
		}
		if len(rows) < batchSize {
			break
		}
		after = rows[len(rows)-1].ID
	}
	return total, errors.Join(errs...)
}

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

func (s *Service) deleteUpload(ctx context.Context, r store.ListDeletingUploadsRow) (bool, error) {
	proceed, err := s.recheckUpload(ctx, r.ID)
	if err != nil || !proceed {
		return false, err
	}
	if r.PlannedAssetID != nil {
		if err := s.deleteSecureObjects(ctx, *r.PlannedAssetID); err != nil {
			return false, fmt.Errorf("media：刪除暫存 %s 的機敏物件：%w", r.ID, err)
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
			return false, fmt.Errorf("media：刪除暫存 %s 的物件 %s：%w", r.ID, k, err)
		}
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.q(ctx).MarkUploadDeleted(ctx, store.MarkUploadDeletedParams{ID: r.ID, Now: s.d.Now()}); err != nil {
			return fmt.Errorf("media：標記暫存已刪：%w", err)
		}
		if _, err := s.q(ctx).DeleteClaimOfUpload(ctx, r.ID); err != nil {
			return fmt.Errorf("media：刪除暫存 %s 的認領：%w", r.ID, err)
		}
		if r.PlannedAssetID != nil {
			if err := s.q(ctx).DeleteSecureChunks(ctx, *r.PlannedAssetID); err != nil {
				return fmt.Errorf("media：刪除暫存 %s 的段記錄：%w", r.ID, err)
			}
		}
		return nil
	})
	return err == nil, err
}

func (s *Service) recheckUpload(ctx context.Context, id uuid.UUID) (bool, error) {
	proceed := false
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.LockDeletingUpload(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("media：鎖定刪除中的暫存 %s：%w", id, err)
		}
		now := s.d.Now()
		for _, assetID := range []*uuid.UUID{row.AssetID, row.PlannedAssetID} {
			if assetID == nil {
				continue
			}
			held, err := q.IsAssetHeld(ctx, store.IsAssetHeldParams{AssetID: *assetID, Now: now})
			if err != nil {
				return fmt.Errorf("media：查暫存 %s 的資產保全：%w", id, err)
			}
			if !held {
				continue
			}
			n, err := q.RestoreHeldUpload(ctx, store.RestoreHeldUploadParams{ID: id, Now: now})
			if err != nil {
				return fmt.Errorf("media：退回有保全的暫存 %s：%w", id, err)
			}
			if n == 0 {
				return fmt.Errorf("media：暫存 %s 的資產有保全，但無法判定退回哪個原狀態", id)
			}
			if _, err := q.DeleteClaimOfUpload(ctx, id); err != nil {
				return fmt.Errorf("media：刪除暫存 %s 的認領：%w", id, err)
			}
			return nil
		}
		proceed = true
		return nil
	})
	return proceed, err
}

func (s *Service) deleteAsset(ctx context.Context, r store.ListDeletingAssetsRow) (bool, error) {
	proceed, err := s.recheckAsset(ctx, r.ID)
	if err != nil || !proceed {
		return false, err
	}
	if model.Visibility(r.Visibility) == model.VisibilitySecure {
		if err := s.deleteSecureObjects(ctx, r.ID); err != nil {
			return false, err
		}
	} else if err := s.d.Store.Delete(ctx, r.ObjectKey); err != nil {
		return false, fmt.Errorf("media：刪除資產 %s 的物件：%w", r.ID, err)
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := q.SettleDeletedAssetUploads(ctx, store.SettleDeletedAssetUploadsParams{AssetID: r.ID, Now: s.d.Now()}); err != nil {
			return fmt.Errorf("media：轉換資產 %s 的暫存列：%w", r.ID, err)
		}
		if _, err := q.DeleteClaimsOfAsset(ctx, r.ID); err != nil {
			return fmt.Errorf("media：刪除資產 %s 的認領：%w", r.ID, err)
		}
		if _, err := q.DeleteRequestOfAsset(ctx, r.ID); err != nil {
			return fmt.Errorf("media：刪除資產 %s 的刪除請求：%w", r.ID, err)
		}
		if _, err := s.q(ctx).MarkAssetDeleted(ctx, store.MarkAssetDeletedParams{ID: r.ID, Now: s.d.Now()}); err != nil {
			return fmt.Errorf("media：標記資產已刪：%w", err)
		}
		if model.Visibility(r.Visibility) == model.VisibilitySecure {
			if err := s.q(ctx).DeleteSecureChunks(ctx, r.ID); err != nil {
				return fmt.Errorf("media：刪除資產 %s 的段記錄：%w", r.ID, err)
			}
		}
		return nil
	})
	return err == nil, err
}

func (s *Service) recheckAsset(ctx context.Context, id uuid.UUID) (bool, error) {
	proceed := false
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := q.LockDeletingAsset(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("media：鎖定刪除中的資產 %s：%w", id, err)
		}
		now := s.d.Now()
		held, err := q.IsAssetHeld(ctx, store.IsAssetHeldParams{AssetID: id, Now: now})
		if err != nil {
			return fmt.Errorf("media：查資產 %s 的保全：%w", id, err)
		}
		if !held {
			proceed = true
			return nil
		}
		if _, err := q.RestoreHeldAsset(ctx, store.RestoreHeldAssetParams{ID: id, Now: now}); err != nil {
			return fmt.Errorf("media：退回有保全的資產 %s：%w", id, err)
		}
		if _, err := q.ReopenDeletionRequest(ctx, id); err != nil {
			return fmt.Errorf("media：重開資產 %s 的刪除請求：%w", id, err)
		}
		return nil
	})
	return proceed, err
}

type deleteWorker struct {
	river.WorkerDefaults[ApplyDeletionsArgs]
	svc *Service
}

// Work 套用刪除請求，再刪除所有標記中的物件；失敗由 River 重試（冪等）。
func (w *deleteWorker) Work(ctx context.Context, _ *river.Job[ApplyDeletionsArgs]) error {
	if _, err := w.svc.ApplyDeletions(ctx); err != nil {
		return err
	}
	_, err := w.svc.DeleteAssets(ctx)
	return err
}

// Timeout 是一次套用與刪除的時間上限。
func (w *deleteWorker) Timeout(*river.Job[ApplyDeletionsArgs]) time.Duration { return 5 * time.Minute }

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
	river.AddWorker(workers, &expireHoldsWorker{svc: s})
}

// PeriodicJobs 回傳啟動即執行的排程：每 ApplyInterval 套用刪除請求並刪物件，每 SweepInterval 的 GC。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(ApplyInterval),
			func() (river.JobArgs, *river.InsertOpts) { return ApplyDeletionsArgs{}, nil },
			&river.PeriodicJobOpts{ID: "media.apply_deletions", RunOnStart: true},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return GCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "media.gc_uploads", RunOnStart: true},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(ExpireHoldsInterval),
			func() (river.JobArgs, *river.InsertOpts) { return ExpireHoldsArgs{}, nil },
			&river.PeriodicJobOpts{ID: "media.expire_holds", RunOnStart: true},
		),
	}
}
