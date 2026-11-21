// Package storage 是 media 模組的物件儲存 port、S3 相容 adapter（R2、MinIO）與測試用的記憶體 fake。
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

// Store 是 media 需要的物件儲存操作。所有呼叫都不得放在 DB transaction 內。
type Store interface {
	// PresignPut 產生預簽 PUT。簽章綁確切的 Content-Length、Content-Type、Content-MD5 與 If-None-Match: *，
	// client 必須原樣送出回傳的 Headers。
	PresignPut(ctx context.Context, in PresignPutInput) (PresignedPut, error)
	// Get 串流讀取物件，呼叫方負責關閉。不存在回 ErrNotFound。
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Head 回傳物件的大小與型別。不存在回 ErrNotFound。
	Head(ctx context.Context, key string) (ObjectInfo, error)
	// Put 寫入物件；同一個 key 重寫會覆蓋，讓處理 job 重試時冪等。
	Put(ctx context.Context, in PutInput) error
	// Delete 刪除物件；物件不存在視為成功。
	Delete(ctx context.Context, key string) error
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
