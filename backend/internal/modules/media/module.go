package media

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/reclaim"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/upload"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// 暫存上傳的型別與常數（定義在 internal/model，這裡是對外的別名）。
type (
	// Visibility 是資產的可見性。
	Visibility = model.Visibility
	// UploadState 是暫存上傳的狀態。
	UploadState = model.UploadState
	// RejectReason 是暫存上傳被拒絕的原因代碼。
	RejectReason = model.RejectReason
	// BeginInput 是 BeginUpload 的宣告。
	BeginInput = upload.BeginInput
	// BeginResult 是 BeginUpload 的結果：預簽 PUT 與只出現一次的 token。
	BeginResult = upload.BeginResult
	// UploadStatus 是 GetUpload 的結果。
	UploadStatus = upload.Status
	// ConfirmedAsset 是 Confirm 回傳的資產。
	ConfirmedAsset = upload.ConfirmedAsset
	// ProcessFunc 是處理 job 的內容。
	ProcessFunc = upload.ProcessFunc
	// ProcessInput 是處理一筆暫存的輸入。
	ProcessInput = upload.ProcessInput
	// ProcessResult 是處理寫出的正式物件。
	ProcessResult = upload.ProcessResult
	// GCResult 是一輪 GC 的結果。
	GCResult = reclaim.Result
	// ObjectStore 是模組需要的物件儲存；組裝點由 R2 或 MinIO 的實作提供，測試用記憶體 fake。
	ObjectStore = storage.Store
)

const (
	VisibilityPublic = model.VisibilityPublic
	VisibilitySecure = model.VisibilitySecure

	UploadPending   = model.StatePending
	UploadUploaded  = model.StateUploaded
	UploadReady     = model.StateReady
	UploadRejected  = model.StateRejected
	UploadConfirmed = model.StateConfirmed
	UploadDeleting  = model.StateDeleting
	UploadDeleted   = model.StateDeleted

	RejectUnrecognizedFormat = model.RejectUnrecognizedFormat
	RejectSizeMismatch       = model.RejectSizeMismatch
	RejectChecksumMismatch   = model.RejectChecksumMismatch
	RejectProcessingFailed   = model.RejectProcessingFailed
	RejectExpired            = model.RejectExpired

	// UploadTTL 是暫存上傳從 BeginUpload 起算的存活時間，整個模組唯一的定義。
	UploadTTL = model.UploadTTL
	// StuckUploadGrace 是卡在 UPLOADED 的暫存列過期後、GC 回收它之前的寬限。
	StuckUploadGrace = model.StuckUploadGrace
	// ProcessMaxAttempts 是處理 job 的重試上限。
	ProcessMaxAttempts = upload.MaxAttempts
)

var (
	// ErrInTransaction：會碰物件儲存的呼叫，ctx 卻帶著 DB transaction。
	ErrInTransaction = model.ErrInTransaction
	// ErrNoTransaction：Confirm、MarkDeleted 必須在擁有者的 transaction 內呼叫。
	ErrNoTransaction = model.ErrNoTransaction
)

// Permanent 把處理的永久失敗標上拒絕原因：暫存列立即轉 REJECTED，不重試。
func Permanent(reason RejectReason, err error) error { return model.Permanent(reason, err) }

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB   *db.DB
	Jobs jobs.Enqueuer
	// Store 是物件儲存。
	Store ObjectStore
	// Process 是處理 job 的內容。
	Process ProcessFunc
	// NewID 產生資產與暫存列的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}

// Module 是 media 的暫存上傳與資產生命週期。
type Module struct {
	upload  *upload.Service
	reclaim *reclaim.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Module{
		upload: upload.New(upload.Deps{
			DB: d.DB, Jobs: d.Jobs, Store: d.Store, Process: d.Process, NewID: d.NewID, Now: d.Now, Logger: d.Logger,
		}),
		reclaim: reclaim.New(reclaim.Deps{DB: d.DB, Jobs: d.Jobs, Store: d.Store, Now: d.Now, Logger: d.Logger}),
	}
}

// BeginUpload 建一筆暫存列，回傳預簽 PUT 與不透明 token（資料表只存雜湊）。不得在 transaction 內呼叫。
func (m *Module) BeginUpload(ctx context.Context, uploader Uploader, in BeginInput) (BeginResult, error) {
	return m.upload.Begin(ctx, uploader, in)
}

// CompleteUpload 驗上傳者、標記已上傳並在同一個 transaction 排處理 job；重入成功。
func (m *Module) CompleteUpload(ctx context.Context, uploader Uploader, token string) error {
	return m.upload.Complete(ctx, uploader, token)
}

// GetUpload 回傳暫存的狀態與拒絕原因，只有上傳者本人看得到。
func (m *Module) GetUpload(ctx context.Context, uploader Uploader, token string) (UploadStatus, error) {
	return m.upload.Get(ctx, uploader, token)
}

// Confirm 在擁有者的 transaction 內確認暫存（B1-D1）：只接受就緒、未過期、上傳者相符的暫存；重入回同一個資產，而且只回這筆暫存的資產。
// ctx 必須帶 transaction，確認與業務列一起提交或回滾。
func (m *Module) Confirm(ctx context.Context, uploader Uploader, token string) (ConfirmedAsset, error) {
	return m.upload.Confirm(ctx, uploader, token)
}

// MarkDeleted 是擁有者的刪除指令（B1-D9）：在擁有者的 transaction 內把已確認的資產標為刪除並排實體刪除 job。冪等。
func (m *Module) MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error {
	return m.reclaim.MarkDeleted(ctx, assetIDs)
}

// ProcessUpload 是 media.process_upload job 的內容，給 worker 與測試呼叫。
func (m *Module) ProcessUpload(ctx context.Context, uploadID uuid.UUID, attempt, maxAttempts int) error {
	return m.upload.Process(ctx, uploadID, attempt, maxAttempts)
}

// CollectGarbage 是暫存 GC 的內容（標記過期、未確認的暫存，之後刪物件），給 worker 與測試呼叫。
func (m *Module) CollectGarbage(ctx context.Context) (GCResult, error) {
	return m.reclaim.CollectGarbage(ctx)
}

// DeleteAssets 是實體刪除 job 的內容（刪所有標記中的物件），給 worker 與測試呼叫。
func (m *Module) DeleteAssets(ctx context.Context) (int, error) { return m.reclaim.DeleteAssets(ctx) }

// AddWorkers 登記 River worker（處理、實體刪除、暫存 GC）。
func (m *Module) AddWorkers(workers *river.Workers) {
	upload.AddWorkers(workers, m.upload)
	reclaim.AddWorkers(workers, m.reclaim)
}

// PeriodicJobs 回傳暫存 GC 的排程。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return reclaim.PeriodicJobs() }
