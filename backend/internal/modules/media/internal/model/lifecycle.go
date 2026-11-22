package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// UploadTTL 是暫存上傳的存活時間，從 BeginUpload 起算：PENDING、UPLOADED、READY 都在這個期限內，過了就不能完成、不能確認，
// 之後由 GC 回收。整個模組只有這一處定義。
const UploadTTL = time.Hour

// ProcessMaxAttempts 是處理 job 的重試上限，ProcessTimeout 是一次處理的時間上限，ProcessMaxBackoff 是兩次嘗試之間退避的保守上界
// （River 預設退避是嘗試次數的四次方秒，上限遠小於此）。
const (
	ProcessMaxAttempts = 5
	ProcessTimeout     = 5 * time.Minute
	ProcessMaxBackoff  = 15 * time.Minute
)

// StuckUploadGrace 是停在 UPLOADED 的列過期後、GC 才回收它之前的寬限：必須大於處理 job 所有嘗試加退避的總長，
// 否則 GC 可能回收還在處理的列（lifecycle_grace_test.go 斷言這個不等式）。
const StuckUploadGrace = 3 * time.Hour

// 單次上傳的大小上限（位元組），整個模組只有這一處定義：BeginUpload 拒絕宣告超過的 public 上傳，處理 job 也不讀超過的物件；UploadSecure 以 MaxSecureUploadSize 為讀取上限。
// 值取自 Java media-service 的 `hawkr.media.upload.max-image-size-bytes`（application.yml:117，一般圖片 10MB）
// 與 `max-secure-size-bytes`（application.yml:118，機敏檔案 100MB），Java 在 MediaUploadValidator.java:59-63 以宣告大小套用，Go 版的機敏上傳不收宣告，改以實際讀到的位元組數套用。
const (
	MaxPublicUploadSize int64 = 10 << 20
	MaxSecureUploadSize int64 = 100 << 20
)

// 公開圖片解碼前的像素上限，整個模組只有這一處定義；處理 job 讀標頭後先比對，超過就拒絕、不解碼。
// MaxImagePixels 是單一影格的像素數（4000 萬，約 7000x5700）；MaxAnimatedPixels 是動態圖「影格數 x 每格像素」的總量（5000 萬）。
// 理由見 CARRYOVER.md 的上限一列。
const (
	MaxImagePixels    int64 = 40_000_000
	MaxAnimatedPixels int64 = 50_000_000
	// MaxImageDimension 是單邊像素上限，對所有輸出格式一律套用（輸出格式要解碼並判斷透明後才知道，標頭階段無從分流）：
	// JPEG 編碼器在任一邊超過 65500 時一律失敗，在標頭就拒絕，不讓它變成重試到用盡的暫時失敗。
	// 代價是 PNG 與 GIF 的單邊也不得超過 65500；這種長寬比（像素總量仍在上限內）不是產品要的圖，所以不分格式放寬。
	MaxImageDimension = 65500
)

// ProcessQueue 是處理 job 專用的 River queue，ProcessQueueWorkers 是它同時執行的 job 數，整個模組只有這一處定義。
// 解碼的記憶體上限是「每個 job」的像素上限；queue 的併發乘上像素上限才是每個 process 的記憶體預算（見 CARRYOVER.md 的上限一列），
// 所以不與其他 job 共用 default queue 的併發。
const (
	ProcessQueue        = "media_process"
	ProcessQueueWorkers = 2
)

// PublicPutConcurrency 是同時在請求內解碼的 PutPublic 數量，整個模組只有這一處定義。
// PutPublic 與處理 job 在同一個 process 內用同一組 libvips 解碼，所以每個 process 同時解碼的上限是
// ProcessQueueWorkers + PublicPutConcurrency 張，記憶體預算是這個數乘上像素上限；PutPublic 的呼叫方是伺服器內部，
// 滿了就等名額（受 ctx 約束），不回錯誤。
const PublicPutConcurrency = 2

// 機敏串流上傳的資源界限，與大小上限同在 model：每個進行中的 UploadSecure 佔一個最大約 MaxSecureUploadSize 的密文暫存檔與一條連線，
// 所以同時進行的數量有上限（磁碟預算約 4 x 100 MiB），client 上傳（UploadSecure）滿了立即回 ResourceExhausted、不排隊，伺服器內部的 PutSecure 滿了等名額（受 ctx 約束）；寫入物件儲存的時限也明確。
const (
	SecureUploadConcurrency = 4
	SecureUploadTimeout     = DirectPutTimeout
)

// 直接寫入（UploadSecure、PutSecure、PutPublic）的時限：DirectPutTimeout 是寫入物件儲存的時間上限；
// DirectAbortTimeout 是失敗收尾（轉 REJECTED 與刪物件）的時間上限，收尾不受 ctx 取消影響，所以必須自己有時限，否則儲存卡住時永久佔一個名額。
const (
	DirectPutTimeout   = 5 * time.Minute
	DirectAbortTimeout = 30 * time.Second
)

// PresignTTL 是預簽 PUT 網址的有效期；不得超過 UploadTTL（網址比暫存列活得久沒有意義）。
const PresignTTL = 15 * time.Minute

// Visibility 是資產的可見性。值存在資料庫（CHECK），與 schema.sql 一致。
type Visibility string

const (
	// VisibilityPublic 的物件在 public/，由 image gateway 提供。
	VisibilityPublic Visibility = "PUBLIC"
	// VisibilitySecure 的物件在 secure/，只以密文存放。
	VisibilitySecure Visibility = "SECURE"
)

// Valid 回報 v 是不是已知的可見性。
func (v Visibility) Valid() bool { return v == VisibilityPublic || v == VisibilitySecure }

// UploadState 是暫存上傳的狀態。轉換圖見 db/schema.sql 的 staged_uploads。
type UploadState string

const (
	// StatePending：已發預簽網址，client 還沒回報完成。
	StatePending UploadState = "PENDING"
	// StateUploaded：client 回報完成，處理 job 進行中。
	StateUploaded UploadState = "UPLOADED"
	// StateReady：處理完成、資產列已建立，等擁有者確認。
	StateReady UploadState = "READY"
	// StateRejected：處理被拒絕，原因見 RejectReason。
	StateRejected UploadState = "REJECTED"
	// StateConfirmed：擁有者已在自己的 transaction 內確認。
	StateConfirmed UploadState = "CONFIRMED"
	// StateDeleting：GC 已標記，物件尚未刪掉。
	StateDeleting UploadState = "DELETING"
	// StateDeleted：GC 完成。
	StateDeleted UploadState = "DELETED"
)

// AssetState 是資產的狀態。
type AssetState string

const (
	// AssetStaged：處理完成、尚未確認。
	AssetStaged AssetState = "STAGED"
	// AssetActive：已確認，只由擁有者的 MarkDeleted 刪除。
	AssetActive AssetState = "ACTIVE"
	// AssetDeleting：已標記刪除，物件尚未刪掉。
	AssetDeleting AssetState = "DELETING"
	// AssetDeleted：物件已刪。
	AssetDeleted AssetState = "DELETED"
)

// RejectReason 是暫存上傳被拒絕的原因代碼。
type RejectReason string

const (
	RejectUnrecognizedFormat RejectReason = "UNRECOGNIZED_FORMAT"
	// RejectSizeMismatch：實際大小與宣告不符、宣告或實際超過上限，或 tmp 物件不存在（client 回報完成卻沒有上傳到宣告的內容，
	// 對 client 而言都是「上傳的位元組不是宣告的那份」；不另設原因碼，原因碼在 B1-3 成為對外契約）。
	RejectSizeMismatch     RejectReason = "SIZE_MISMATCH"
	RejectChecksumMismatch RejectReason = "CHECKSUM_MISMATCH"
	// RejectProcessingFailed：暫時失敗重試用盡。
	RejectProcessingFailed RejectReason = "PROCESSING_FAILED"
	// RejectExpired：處理完成時列已過期。
	RejectExpired RejectReason = "EXPIRED"
)

// Valid 回報 r 是不是已知的拒絕原因。
func (r RejectReason) Valid() bool {
	switch r {
	case RejectUnrecognizedFormat, RejectSizeMismatch, RejectChecksumMismatch, RejectProcessingFailed, RejectExpired:
		return true
	}
	return false
}

// PermanentError 是處理的永久失敗：重試不會變好，暫存列立即標為 REJECTED 並記 Reason。其他錯誤都視為暫時失敗。
type PermanentError struct {
	Reason RejectReason
	Err    error
}

func (e *PermanentError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("media：處理被拒絕（%s）", e.Reason)
	}
	return fmt.Sprintf("media：處理被拒絕（%s）：%v", e.Reason, e.Err)
}

func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent 回傳帶原因的永久失敗。
func Permanent(reason RejectReason, err error) error {
	return &PermanentError{Reason: reason, Err: err}
}

// ErrInTransaction 是呼叫會碰物件儲存的函式時，ctx 帶著 DB transaction：單體規則 5，R2 不在 transaction 內呼叫。
var ErrInTransaction = errors.New("media：不得在 DB transaction 內呼叫物件儲存")

// ErrNoTransaction 是呼叫必須與擁有者業務列同一個 transaction 的函式時，ctx 沒有 transaction。
var ErrNoTransaction = errors.New("media：必須在擁有者的 transaction 內呼叫")

// RequireNoTx 在 ctx 帶著 transaction 時回 ErrInTransaction。每個會呼叫物件儲存的函式第一步呼叫它；
// 這些函式本身也不收 transaction 參數，兩者合起來守單體規則 5。
func RequireNoTx(ctx context.Context) error {
	if _, ok := db.TxFrom(ctx); ok {
		return ErrInTransaction
	}
	return nil
}

// RequireTx 在 ctx 沒有 transaction 時回 ErrNoTransaction。
func RequireTx(ctx context.Context) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return ErrNoTransaction
	}
	return nil
}
