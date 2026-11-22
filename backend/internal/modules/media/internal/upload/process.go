package upload

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// ProcessInput 是處理一筆已上傳暫存的輸入；FinalKey 在 CompleteUpload 時就產生並記在暫存列，重試寫同一個 key。
type ProcessInput struct {
	UploadID            uuid.UUID
	Visibility          model.Visibility
	TmpKey              string
	FinalKey            string
	DeclaredSize        int64
	DeclaredContentType string
	DeclaredMD5         string
}

// ProcessResult 是處理寫出的正式物件：資產列的 size 與型別取自這裡，不取宣告值。
type ProcessResult struct {
	SizeBytes   int64
	ContentType string
}

// ProcessFunc 讀暫存物件、驗證、剝除或加密後寫入 in.FinalKey。永久失敗回 model.Permanent，其餘錯誤視為暫時失敗由 River 重試。
// 它在 transaction 外呼叫（ctx 不帶 transaction）。
type ProcessFunc func(ctx context.Context, in ProcessInput) (ProcessResult, error)

// MaxAttempts 是處理 job 的重試上限；用盡時暫存列標為 REJECTED（PROCESSING_FAILED）。
const MaxAttempts = model.ProcessMaxAttempts

// ProcessTimeout 是一次處理的時間上限。
const ProcessTimeout = model.ProcessTimeout

// ProcessArgs 是處理 job 的參數（job 種類 media.process_upload）。
type ProcessArgs struct {
	UploadID uuid.UUID `json:"upload_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (ProcessArgs) Kind() string { return "media.process_upload" }

// InsertOpts 讓同一筆暫存還沒結束的處理 job 合併。
func (ProcessArgs) InsertOpts() river.InsertOpts {
	opts := jobs.UniqueByArgs(MaxAttempts)
	opts.Queue = model.ProcessQueue
	return opts
}

func (s *Service) enqueueProcess(ctx context.Context, id uuid.UUID) error {
	opts := ProcessArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, ProcessArgs{UploadID: id}, &opts)
}

// Process 處理一筆 UPLOADED 的暫存：成功時以條件式 transaction 建資產列並轉 READY；永久失敗立即轉 REJECTED；暫時失敗回錯誤讓 River
// 重試，用盡（attempt >= maxAttempts）轉 REJECTED（PROCESSING_FAILED）；列在處理中途過期，條件式轉換失敗，刪掉剛寫的正式物件並轉 REJECTED（EXPIRED）。
// 不是 UPLOADED 的列直接回 nil（重複的 job、已被處理過）。不得在 transaction 內呼叫。
func (s *Service) Process(ctx context.Context, id uuid.UUID, attempt, maxAttempts int) error {
	if err := model.RequireNoTx(ctx); err != nil {
		return err
	}
	row, err := s.q(ctx).GetUpload(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("media：讀取暫存上傳：%w", err)
	}
	if model.UploadState(row.State) != model.StateUploaded || row.FinalKey == nil {
		return nil
	}
	pin := ProcessInput{
		UploadID: row.ID, Visibility: model.Visibility(row.Visibility), FinalKey: *row.FinalKey,
		DeclaredSize: row.DeclaredSize, DeclaredContentType: row.DeclaredContentType,
	}
	if row.TmpKey != nil {
		pin.TmpKey = *row.TmpKey
	}
	if row.DeclaredMd5 != nil {
		pin.DeclaredMD5 = *row.DeclaredMd5
	}
	res, err := s.d.Process(ctx, pin)
	if err == nil {
		if res.ContentType == "" || res.SizeBytes < 0 {
			s.d.Logger.ErrorContext(ctx, "media：處理函式回傳不合法的結果，標記拒絕", "upload_id", row.ID)
			return s.reject(ctx, row.ID, model.RejectProcessingFailed)
		}
		return s.finish(ctx, row, res)
	}
	var perm *model.PermanentError
	switch {
	case errors.As(err, &perm) && perm.Reason.Valid():
		return s.reject(ctx, row.ID, perm.Reason)
	case errors.Is(ctx.Err(), context.Canceled):
		// worker 關機不是這個 job 的失敗，不消耗最後一次嘗試；River 會重試。job 自己的逾時是 DeadlineExceeded，照一般暫時失敗算。
		return fmt.Errorf("media：處理暫存上傳 %s 被中斷：%w", row.ID, err)
	case attempt >= maxAttempts:
		s.d.Logger.ErrorContext(ctx, "media：處理重試用盡，標記拒絕", "upload_id", row.ID, "error", err)
		return s.reject(ctx, row.ID, model.RejectProcessingFailed)
	default:
		return fmt.Errorf("media：處理暫存上傳 %s：%w", row.ID, err)
	}
}

var errLost = errors.New("media：暫存列已不是 UPLOADED 或已過期")

// finish 以一個 transaction 建資產列並把暫存列轉 READY；輸給對方時依列的現況收尾。
func (s *Service) finish(ctx context.Context, row store.MediaStagedUpload, res ProcessResult) error {
	assetID := s.d.NewID()
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Now()
		q := s.q(ctx)
		if err := q.InsertAsset(ctx, store.InsertAssetParams{
			ID: assetID, Visibility: row.Visibility, ObjectKey: *row.FinalKey, ContentType: res.ContentType, SizeBytes: res.SizeBytes, Now: now,
		}); err != nil {
			return fmt.Errorf("media：建立資產：%w", err)
		}
		n, err := q.MarkUploadReady(ctx, store.MarkUploadReadyParams{ID: row.ID, AssetID: &assetID, Now: now})
		if err != nil {
			return fmt.Errorf("media：標記就緒：%w", err)
		}
		if n == 0 {
			return errLost
		}
		return nil
	})
	switch {
	case err == nil:
		// 提交之後才刪暫存物件；刪失敗由 bucket 對 tmp/ 的 lifecycle 兜底（CONFIRMED 的列不進 GC）。
		if row.TmpKey == nil {
			return nil
		}
		if err := s.d.Store.Delete(ctx, *row.TmpKey); err != nil {
			s.d.Logger.WarnContext(ctx, "media：刪除暫存物件失敗，留給 bucket lifecycle", "upload_id", row.ID, "error", err)
		}
		return nil
	case errors.Is(err, errLost):
		return s.lost(ctx, row)
	default:
		return err
	}
}

// lost 處理條件式轉換輸掉：列是 READY 或 CONFIRMED（正式 key 已屬於資產）就什麼都不做；其他情況都要刪掉自己寫的正式物件，
// 因為沒有資產指向它。列還是 UPLOADED（只可能是過期）時先轉 REJECTED（EXPIRED）帶著 final_key 進 GC，刪失敗交給 GC 再刪；
// 其他狀態（PENDING、DELETING、DELETED、REJECTED、列不存在）GC 不會再看這個 key，刪失敗就回傳錯誤讓 River 重試，
// 重試時條件式轉換會再輸一次、再進來刪。物件不存在視為成功。
func (s *Service) lost(ctx context.Context, row store.MediaStagedUpload) error {
	cur, err := s.q(ctx).GetUpload(ctx, row.ID)
	missing := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !missing {
		return fmt.Errorf("media：重讀暫存上傳：%w", err)
	}
	uploaded := false
	if !missing {
		switch model.UploadState(cur.State) {
		case model.StateReady, model.StateConfirmed:
			return nil
		case model.StateUploaded:
			uploaded = true
		case model.StatePending, model.StateRejected, model.StateDeleting, model.StateDeleted:
		}
	}
	if err := s.d.Store.Delete(ctx, *row.FinalKey); err != nil {
		if !uploaded {
			return fmt.Errorf("media：刪除未被資產採用的正式物件：%w", err)
		}
		s.d.Logger.WarnContext(ctx, "media：刪除未被資產採用的正式物件失敗，留給 GC", "upload_id", row.ID, "error", err)
	}
	if uploaded {
		return s.reject(ctx, row.ID, model.RejectExpired)
	}
	return nil
}

// reject 不受 ctx 取消影響：最後一次重試逾時時，列仍要離開 UPLOADED（只有 REJECTED 的列會進 GC）。
func (s *Service) reject(ctx context.Context, id uuid.UUID, reason model.RejectReason) error {
	ctx = context.WithoutCancel(ctx)
	r := string(reason)
	if _, err := s.q(ctx).MarkUploadRejected(ctx, store.MarkUploadRejectedParams{ID: id, Reason: &r, Now: s.d.Now()}); err != nil {
		return fmt.Errorf("media：標記拒絕：%w", err)
	}
	return nil
}

type processWorker struct {
	river.WorkerDefaults[ProcessArgs]
	svc *Service
}

// Work 處理一筆暫存；重複執行安全（不是 UPLOADED 的列直接略過）。
func (w *processWorker) Work(ctx context.Context, job *river.Job[ProcessArgs]) error {
	return w.svc.Process(ctx, job.Args.UploadID, job.Attempt, job.MaxAttempts)
}

// Timeout 是一次處理的時間上限。
func (w *processWorker) Timeout(*river.Job[ProcessArgs]) time.Duration { return ProcessTimeout }

// AddWorkers 登記處理 job 的 River worker。
func AddWorkers(workers *river.Workers, s *Service) { river.AddWorker(workers, &processWorker{svc: s}) }
