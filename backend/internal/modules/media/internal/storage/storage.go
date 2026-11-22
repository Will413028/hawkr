// Package storage 是 media 模組的物件儲存 port、S3 相容 adapter（R2、Garage 等 S3 相容服務）與測試用的記憶體 fake。
//
// 建構不到有效實作時回傳錯誤，不退回 fake：缺設定要讓程式在啟動時就失敗。
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound 表示物件不存在（Get、Head）。Delete 不存在的物件不是錯誤。
var ErrNotFound = errors.New("storage: object not found")

// ErrSizeMismatch 表示 Put 的 Body 長度與 Size 不符；S3 與 Fake 行為一致，且在送出請求前就回。
var ErrSizeMismatch = errors.New("storage: body length differs from declared size")

// DefaultContentType 是 Put 沒給 ContentType 時物件的型別（與 S3 預設一致）。
const DefaultContentType = "application/octet-stream"

// MaxPresignExpiry 是 SigV4 預簽網址的有效期上限。
const MaxPresignExpiry = 7 * 24 * time.Hour

// Store 是 media 需要的物件儲存操作。所有呼叫都不得放在 DB transaction 內。
type Store interface {
	// PresignPut 產生預簽 PUT。簽章綁確切的 Content-Length、Content-Type、Content-MD5 與 If-None-Match: *，
	// client 必須原樣送出回傳的 Headers。Headers 不含 Content-Length：它由 body 的實際長度決定（瀏覽器與多數 HTTP 客戶端不允許自行設定），
	// body 長度必須等於 ContentLength。
	PresignPut(ctx context.Context, in PresignPutInput) (PresignedPut, error)
	// Get 串流讀取物件，呼叫方負責關閉。不存在回 ErrNotFound。
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Head 回傳物件的大小與型別。不存在回 ErrNotFound。
	Head(ctx context.Context, key string) (ObjectInfo, error)
	// Put 寫入物件。Body 應為 io.ReadSeeker（例如暫存檔）；非 seekable 的 Body 會整個讀進記憶體。
	// Body 剩餘長度與 Size 不符回 ErrSizeMismatch。同一個 key 重寫會覆蓋，讓處理 job 重試時冪等。
	Put(ctx context.Context, in PutInput) error
	// Delete 刪除物件；物件不存在視為成功。
	Delete(ctx context.Context, key string) error
	// List 回傳 key 以 prefix 開頭的所有物件 key（遞增排序，分頁在 adapter 內處理完）。沒有符合的回空 slice。
	List(ctx context.Context, prefix string) ([]string, error)
}

// PresignPutInput 是預簽 PUT 的參數。
type PresignPutInput struct {
	Key           string
	ContentLength int64
	ContentType   string
	// ContentMD5 是內容 MD5 的 base64（標準編碼）。
	ContentMD5 string
	Expires    time.Duration
}

// PresignedPut 是預簽 PUT 的結果。
type PresignedPut struct {
	URL string
	// Headers 是 client 上傳時必須送出的 header（值與簽章一致）。
	Headers map[string]string
}

// ObjectInfo 是 Head 的結果。
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// PutInput 是 Put 的參數；Body 讀滿 Size 個位元組。
type PutInput struct {
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
}
