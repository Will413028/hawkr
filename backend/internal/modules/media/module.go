package media

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/reclaim"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
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
	// BeginInput 是 BeginUpload 的宣告（只給 public；機敏檔案走 UploadSecure）。
	BeginInput = upload.BeginInput
	// SecureUploadResult 是 UploadSecure 的結果：token、辨認出的型別、明文大小與期限；資產 ID 不回給上傳者。
	SecureUploadResult = upload.SecureResult
	// PutPublicResult 是 PutPublic 的結果：token、重新編碼後的型別與大小、期限。
	PutPublicResult = upload.PublicPutResult
	// Keyring 是機敏資產的 KEK 金鑰環，由組裝點以 ParseKeyring 建立。
	Keyring = secure.Keyring
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
	// ObjectStore 是模組需要的物件儲存；組裝點由 R2 或 S3 相容替身（Garage）的實作提供，測試用記憶體 fake。
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

	// MaxPublicUploadSize 是單次公開圖片上傳宣告大小的上限（位元組），整個後端唯一的定義；BeginUpload 檢查它，超過回 invalid_argument。
	MaxPublicUploadSize = model.MaxPublicUploadSize
	// UploadTTL 是暫存上傳從 BeginUpload 起算的存活時間，整個模組唯一的定義。
	UploadTTL = model.UploadTTL
	// StuckUploadGrace 是卡在 UPLOADED 的暫存列過期後、GC 回收它之前的寬限。
	StuckUploadGrace = model.StuckUploadGrace
	// ProcessQueue 與 ProcessQueueWorkers 是處理 job 專用的 River queue 與它的併發上限，worker 角色必須設定這個 queue。
	ProcessQueue        = model.ProcessQueue
	ProcessQueueWorkers = model.ProcessQueueWorkers
	// ProcessMaxAttempts 是處理 job 的重試上限。
	ProcessMaxAttempts = upload.MaxAttempts

	// MaxSecureUploadSize 是單次機敏上傳的明文大小上限；SecureUploadConcurrency 與 SecureUploadTimeout 是直接寫入的資源界限。
	MaxSecureUploadSize     = model.MaxSecureUploadSize
	SecureUploadConcurrency = model.SecureUploadConcurrency
	SecureUploadTimeout     = model.DirectPutTimeout
)

// ParseKeyring 讀出 KEK 金鑰環：單一字串、逗號分隔的 `<kek_id>:<標準 base64 的 32 位元組>`，第一把用來寫入、全部都能讀。
// 空字串、格式錯、長度不對或 id 重複回錯誤；組裝點（環境變數 MEDIA_KEKS，B1-5 接線）遇到錯誤要讓啟動失敗。
func ParseKeyring(s string) (*Keyring, error) { return secure.ParseKeyring(s) }

// Queues 回傳 worker 角色必須設定的 River queue 與併發上限，組裝點以它建 queue 設定。
func Queues() map[string]int { return map[string]int{ProcessQueue: ProcessQueueWorkers} }

var (
	// ErrInTransaction：會碰物件儲存的呼叫，ctx 卻帶著 DB transaction。
	ErrInTransaction = model.ErrInTransaction
	// ErrNoKeyring：模組沒有 KEK 金鑰環，UploadSecure、ReadSecure、ReadStagedSecure、RewrapDEKs 拒絕執行且不寫任何物件。
	ErrNoKeyring = model.ErrNoKeyring
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
	// Process 是處理 job 的內容；沒給時 public 用內建的圖片處理（辨認、剝除 metadata），遇到 SECURE 列是永久失敗（機敏上傳不經處理 job）。
	Process ProcessFunc
	// NewID 產生資產與暫存列的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
	// Keyring 是 KEK 金鑰環；nil 時 UploadSecure、ReadSecure、ReadStagedSecure、RewrapDEKs 回 ErrNoKeyring、不寫任何物件。
	Keyring *Keyring
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
	if d.Process == nil && d.Store != nil {
		d.Process = upload.NewProcessor(d.Store)
	}
	return &Module{
		upload: upload.New(upload.Deps{
			DB: d.DB, Jobs: d.Jobs, Store: d.Store, Process: d.Process, NewID: d.NewID, Now: d.Now, Logger: d.Logger, Keyring: d.Keyring,
		}),
		reclaim: reclaim.New(reclaim.Deps{DB: d.DB, Jobs: d.Jobs, Store: d.Store, Now: d.Now, Logger: d.Logger}),
	}
}

// BeginUpload 建一筆暫存列，回傳預簽 PUT 與不透明 token（資料表只存雜湊）。不得在 transaction 內呼叫。
func (m *Module) BeginUpload(ctx context.Context, uploader Uploader, in BeginInput) (BeginResult, error) {
	return m.upload.Begin(ctx, uploader, in)
}

// UploadSecure 在請求內辨認 PDF 或圖片、邊讀邊加密（STREAM 分塊 AEAD、每檔隨機 DEK 以 KEK 包裹）寫入 secure/，成功後建資產列與 READY 的暫存列並回 token。
// 大小與型別都不收宣告：超過 MaxSecureUploadSize、空內容與辨認不出的型別回 InvalidInput 且不寫任何物件；同時進行超過
// SecureUploadConcurrency 立即回 ResourceExhausted。沒有 keyring 回 ErrNoKeyring。不得在 transaction 內呼叫。
func (m *Module) UploadSecure(ctx context.Context, uploader Uploader, body io.Reader) (SecureUploadResult, error) {
	return m.upload.UploadSecure(ctx, uploader, body)
}

// PutSecure 是模組內部的機敏直接寫入：出貨單等由伺服器產生的 PDF 或圖片走這裡，管線與 UploadSecure 相同（magic bytes 辨認、STREAM 加密、不寫明文）。
// 回傳暫存 token；擁有者在自己的 transaction 內以 Uploader 為 uploader 呼叫 Confirm，未確認的由 GC 回收。不得在 transaction 內呼叫。
// 滿了等名額（受 ctx 約束，ctx 結束回 ctx 的錯誤），不回 ResourceExhausted。
func (m *Module) PutSecure(ctx context.Context, uploader Uploader, body io.Reader) (SecureUploadResult, error) {
	return m.upload.PutSecure(ctx, uploader, body)
}

// PutPublic 是模組內部的公開圖片直接寫入：只收 JPEG、PNG、WebP、GIF，規則與公開上傳相同（重新編碼、去 metadata、大小與像素上限），同步處理後寫入 public/ 並回傳立即可 Confirm 的 token。
// PDF 與辨認不出的內容回 InvalidInput，不寫物件也不建列。不得在 transaction 內呼叫；解碼的同時進行數量有上限，滿了等名額（受 ctx 約束）。
func (m *Module) PutPublic(ctx context.Context, uploader Uploader, body io.Reader) (PutPublicResult, error) {
	return m.upload.PutPublic(ctx, uploader, body)
}

// ReadSecure 讀機敏資產的明文（只讀 SECURE 且 ACTIVE；公開資產回 ErrNotSecure，其餘 NotFound）；授權由擁有者先做。
// 一段的 tag 驗過才交出那一段的明文，呼叫方負責關閉。不得在 transaction 內呼叫。
func (m *Module) ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	return m.upload.ReadSecure(ctx, assetID)
}

// ReadStagedSecure 是確認前的預覽：只有上傳者本人讀得到自己 READY、未過期的機敏暫存。不得在 transaction 內呼叫。
func (m *Module) ReadStagedSecure(ctx context.Context, uploader Uploader, token string) (io.ReadCloser, error) {
	return m.upload.ReadStagedSecure(ctx, uploader, token)
}

// RewrapDEKs 把不是 primary KEK 包裹的機敏資產的 DEK 重新包裹，回傳更新筆數；可重跑，不碰物件儲存。
func (m *Module) RewrapDEKs(ctx context.Context) (int, error) { return m.upload.RewrapDEKs(ctx) }

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
