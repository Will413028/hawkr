package upload

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// SecureResult 是機敏上傳完成後的結果（PutSecure 與最後一段）；資產 ID 不回給上傳者，確認前只能憑 Token 讀（ReadStagedSecure）。
type SecureResult struct {
	Token       string
	ContentType string
	SizeBytes   int64
	ExpiresAt   time.Time
}

// SecureBeginResult 是 BeginSecureUpload 的結果：client 依 ChunkSize 把檔案切成 ChunkCount 段，依序（從 0 起）送。
type SecureBeginResult struct {
	Token      string
	ChunkSize  int
	ChunkCount int
	ExpiresAt  time.Time
}

// SecureChunkResult 是一段推進成功的結果。NextIndex 是下一個要送的段序號，最後一段送完時等於段總數且 Complete 為 true；
// ContentType 是第 0 段依 magic bytes 辨認出的型別（成功推進的請求之後一定有）。
type SecureChunkResult struct {
	NextIndex   int
	Complete    bool
	ContentType string
	SizeBytes   int64
	ExpiresAt   time.Time
}

const (
	sniffBytes           = 512
	pdfContentType       = "application/pdf"
	secureObjectType     = "application/octet-stream"
	secureBusyRetryAfter = 5 * time.Second
	metaNextIndex        = "next_index"
)

var errTooLarge = errors.New("media：上傳超過大小上限")

type capReader struct {
	r   io.Reader
	n   int64
	max int64
}

func (c *capReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.max {
		return 0, errTooLarge
	}
	return n, err
}

func sniffSecure(ctx context.Context, head []byte) (string, error) {
	if len(head) == 0 {
		return "", apperr.InvalidInputOn([]string{"body"}, "%s", text.MediaSecureContentEmpty(loc(ctx)))
	}
	if len(head) >= 5 && string(head[:5]) == "%PDF-" {
		return pdfContentType, nil
	}
	f, err := image.Detect(head)
	if err != nil {
		return "", apperr.InvalidInputOn([]string{"body"}, "%s", text.MediaSecureContentUnsupported(loc(ctx)))
	}
	return f.ContentType(), nil
}

func u32(n int) uint32 { return uint32(n) } //nolint:gosec // 見上
func i32(n int) int32  { return int32(n) }  //nolint:gosec // 見上

func secureChunkCount(size int64) int {
	return int((size + model.SecureChunkSize - 1) / model.SecureChunkSize)
}

func secureChunkLength(size int64, count, index int) int64 {
	if index < count-1 {
		return model.SecureChunkSize
	}
	return size - int64(count-1)*model.SecureChunkSize
}

// BeginSecureUpload 建立分段上傳會話：size（明文總位元組數）在 1 到 model.MaxSecureUploadSize 之間，否則 InvalidInput。
// 建暫存列（UPLOADING，預配資產 ID、包好的 DEK 與 kek_id、段總數、next_index 0）並回 token、段大小與段總數；不寫任何物件。
// 沒有 keyring 回 model.ErrNoKeyring。不得在 transaction 內呼叫。
func (s *Service) BeginSecureUpload(ctx context.Context, uploader model.Uploader, size int64) (SecureBeginResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return SecureBeginResult{}, err
	}
	if s.d.Keyring == nil {
		return SecureBeginResult{}, model.ErrNoKeyring
	}
	if err := checkUploader(ctx, uploader); err != nil {
		return SecureBeginResult{}, err
	}
	if size < 1 || size > model.MaxSecureUploadSize {
		return SecureBeginResult{}, apperr.InvalidInputOn([]string{"size"}, "%s", text.MediaSecureSizeOutOfRange(loc(ctx), text.MediaSecureSizeOutOfRangeArgs{Max: int(model.MaxSecureUploadSize)}))
	}
	dek, err := secure.NewDEK()
	if err != nil {
		return SecureBeginResult{}, err
	}
	now := s.d.Now()
	uploadID, assetID, token := s.d.NewID(), s.d.NewID(), newToken()
	kekID, wrapped, err := s.d.Keyring.Wrap(assetID, dek)
	if err != nil {
		return SecureBeginResult{}, err
	}
	count := secureChunkCount(size)
	expiresAt := now.Add(model.UploadTTL)
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return s.q(ctx).InsertSecureUpload(ctx, store.InsertSecureUploadParams{
			ID: uploadID, TokenHash: hashToken(token), UploaderID: uploader.ID, UploaderAdmin: uploader.Admin,
			DelegatedStoreID: delegatedStore(uploader),
			DeclaredSize:     size, PlannedAssetID: assetID, ChunkCount: i32(count), KekID: kekID, WrappedDek: wrapped,
			ExpiresAt: expiresAt, Now: now,
		})
	})
	if err != nil {
		return SecureBeginResult{}, fmt.Errorf("media：建立機敏暫存上傳：%w", err)
	}
	return SecureBeginResult{Token: token, ChunkSize: model.SecureChunkSize, ChunkCount: count, ExpiresAt: expiresAt}, nil
}

// PutSecureChunk 收一段並推進會話；client 上傳用，同時處理中的段滿了（model.SecureUploadConcurrency）立即回 ResourceExhausted。
//
// 只收上傳者本人（非本人與不存在同一個 NotFound）、未過期、UPLOADING 的會話。index 不等於 next_index 回 FailedPrecondition 並在 metadata
// （next_index）帶下一個要送的序號。length 必須等於該段應有的長度，實際讀到的位元組數再核一次，不符回 InvalidInput。
// 第 0 段在寫任何物件之前依 magic bytes 辨認 PDF 或圖片，辨認不出就以條件式把會話轉 REJECTED 並回 InvalidInput，不寫物件。
//
// 順序（R2 呼叫不在 transaction 內）：驗證並加密到記憶體 → 寫新 key 的物件 → 一個 transaction 條件式推進（next_index 與 UPLOADING 與未過期）並記下
// 這段的 key；最後一段的同一個 transaction 建資產列（STAGED）並把會話轉 READY。推進失敗（輸給並行的同一段、會話已過期或已被拒）時，
// 在 transaction 外刪掉自己這次的物件。不得在 transaction 內呼叫。
func (s *Service) PutSecureChunk(ctx context.Context, uploader model.Uploader, token string, index int, body io.Reader, length int64) (SecureChunkResult, error) {
	return s.putChunk(ctx, uploader, token, index, body, length, false)
}

func (s *Service) putChunk(ctx context.Context, uploader model.Uploader, token string, index int, body io.Reader, length int64, wait bool) (SecureChunkResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return SecureChunkResult{}, err
	}
	if s.d.Keyring == nil {
		return SecureChunkResult{}, model.ErrNoKeyring
	}
	if err := checkUploader(ctx, uploader); err != nil {
		return SecureChunkResult{}, err
	}
	row, err := s.own(ctx, uploader, token)
	if err != nil {
		return SecureChunkResult{}, err
	}
	if model.Visibility(row.Visibility) != model.VisibilitySecure {
		return SecureChunkResult{}, model.ErrNotSecure
	}
	if row.PlannedAssetID == nil || row.ChunkCount == nil || row.NextIndex == nil || row.KekID == nil {
		return SecureChunkResult{}, fmt.Errorf("media：機敏暫存上傳 %s 缺少分段欄位", row.ID)
	}
	count, next := int(*row.ChunkCount), int(*row.NextIndex)
	if err := chunkSessionOpen(ctx, row, s.d.Now()); err != nil {
		return SecureChunkResult{}, err
	}
	if index != next {
		return SecureChunkResult{}, nextIndexMismatch(ctx, next)
	}
	expected := secureChunkLength(row.DeclaredSize, count, index)
	if length != expected {
		return SecureChunkResult{}, chunkLengthMismatch(ctx, expected)
	}
	release, err := acquire(ctx, s.secureSlots, wait, apperr.ResourceExhausted(secureBusyRetryAfter, "%s", text.MediaSecureBusy(loc(ctx))))
	if err != nil {
		return SecureChunkResult{}, err
	}
	defer release()

	br := bufio.NewReaderSize(&capReader{r: body, max: expected}, sniffBytes*8)
	var sniffed *string
	if index == 0 {
		head, err := br.Peek(sniffBytes)
		if err != nil && !errors.Is(err, io.EOF) {
			return SecureChunkResult{}, chunkReadErr(ctx, err, expected)
		}
		if len(head) == 0 {
			return SecureChunkResult{}, chunkLengthMismatch(ctx, expected)
		}
		contentType, err := sniffSecure(ctx, head)
		if err != nil {
			return SecureChunkResult{}, s.rejectFirstChunk(ctx, row.ID, err)
		}
		sniffed = &contentType
	}
	dek, err := s.d.Keyring.Unwrap(*row.PlannedAssetID, *row.KekID, row.WrappedDek)
	if err != nil {
		return SecureChunkResult{}, fmt.Errorf("media：解開機敏暫存上傳 %s 的 DEK：%w", row.ID, err)
	}
	var cipherText bytes.Buffer
	cipherText.Grow(int(expected) + secure.HeaderSize + (int(expected)/secure.SegmentSize+1)*secure.TagSize)
	plainSize, err := secure.EncryptChunk(&cipherText, br, dek, secure.Chunk{Asset: *row.PlannedAssetID, Index: u32(index), Count: u32(count)})
	if err != nil {
		return SecureChunkResult{}, chunkReadErr(ctx, err, expected)
	}
	if plainSize != expected {
		return SecureChunkResult{}, chunkLengthMismatch(ctx, expected)
	}

	key := storage.NewSecureChunkKey(*row.PlannedAssetID, index)
	putCtx, cancel := context.WithTimeout(ctx, model.DirectPutTimeout)
	defer cancel()
	if err := s.d.Store.Put(putCtx, storage.PutInput{Key: key, Body: bytes.NewReader(cipherText.Bytes()), Size: int64(cipherText.Len()), ContentType: secureObjectType}); err != nil {
		s.deleteChunkObject(ctx, row.ID, key)
		return SecureChunkResult{}, fmt.Errorf("media：寫入機敏段 %d：%w", index, err)
	}
	if err := s.advance(ctx, row, index, count, key, sniffed); err != nil {
		if err := s.settleLostChunk(ctx, row, index, key, err); !errors.Is(err, errChunkCommitted) {
			return SecureChunkResult{}, err
		}
	}
	return s.chunkResult(row, index, count, sniffed)
}

func (s *Service) advance(ctx context.Context, row store.MediaStagedUpload, index, count int, key string, sniffed *string) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Now()
		q := s.q(ctx)
		n, err := q.AdvanceSecureUpload(ctx, store.AdvanceSecureUploadParams{ID: row.ID, ChunkIndex: i32(index), ContentType: sniffed, Now: now})
		if err != nil {
			return fmt.Errorf("media：推進機敏上傳：%w", err)
		}
		if n == 0 {
			return errLost
		}
		if err := q.InsertSecureChunk(ctx, store.InsertSecureChunkParams{AssetID: *row.PlannedAssetID, Idx: i32(index), ObjectKey: key}); err != nil {
			return fmt.Errorf("media：記下機敏段：%w", err)
		}
		if index != count-1 {
			return nil
		}
		prefix := storage.SecurePrefix(*row.PlannedAssetID)
		if err := q.InsertSecureAssetFromUpload(ctx, store.InsertSecureAssetFromUploadParams{ObjectKey: prefix, Now: now, UploadID: row.ID}); err != nil {
			return fmt.Errorf("media：建立機敏資產：%w", err)
		}
		n, err = q.MarkSecureUploadReady(ctx, store.MarkSecureUploadReadyParams{ID: row.ID, FinalKey: &prefix, Now: now})
		if err != nil {
			return fmt.Errorf("media：標記就緒：%w", err)
		}
		if n == 0 {
			return errLost
		}
		return nil
	})
}

func (s *Service) settleLostChunk(ctx context.Context, row store.MediaStagedUpload, index int, key string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	c, err := s.q(ctx).GetSecureChunk(ctx, store.GetSecureChunkParams{AssetID: *row.PlannedAssetID, Idx: i32(index)})
	switch {
	case err == nil && c.ObjectKey == key:
		return errChunkCommitted
	case (err == nil || errors.Is(err, pgx.ErrNoRows)) && errors.Is(cause, errLost):
		s.deleteChunkObject(ctx, row.ID, key)
	case err == nil || errors.Is(err, pgx.ErrNoRows):
		s.d.Logger.WarnContext(ctx, "media：推進結果不明，密文不刪，等資產刪除或暫存過期回收時移除", "upload_id", row.ID, "index", index, "error", cause)
		return cause
	default:
		s.d.Logger.WarnContext(ctx, "media：推進失敗後查不出自己的段是否已記錄，密文不刪，等資產刪除或暫存過期回收時移除", "upload_id", row.ID, "index", index, "error", err)
		return cause
	}
	if !errors.Is(cause, errLost) {
		return cause
	}
	cur, err := s.q(ctx).GetUpload(ctx, row.ID)
	if err != nil {
		return fmt.Errorf("media：推進失敗後重讀暫存列：%w（原因：%w）", err, cause)
	}
	if err := chunkSessionOpen(ctx, cur, s.d.Now()); err != nil {
		return err
	}
	if cur.NextIndex != nil && int(*cur.NextIndex) != index {
		return nextIndexMismatch(ctx, int(*cur.NextIndex))
	}
	return cause
}

var errChunkCommitted = errors.New("media：機敏段其實已記錄")

func (s *Service) chunkResult(row store.MediaStagedUpload, index, count int, sniffed *string) (SecureChunkResult, error) {
	contentType := sniffed
	if contentType == nil {
		contentType = row.DeclaredContentType
	}
	if contentType == nil || *contentType == "" {
		return SecureChunkResult{}, fmt.Errorf("media：機敏暫存上傳 %s 推進後沒有型別", row.ID)
	}
	return SecureChunkResult{
		NextIndex: index + 1, Complete: index == count-1, ContentType: *contentType, SizeBytes: row.DeclaredSize, ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *Service) deleteChunkObject(ctx context.Context, uploadID uuid.UUID, key string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	if err := s.d.Store.Delete(ctx, key); err != nil {
		s.d.Logger.WarnContext(ctx, "media：刪除沒推進成功的機敏段失敗，密文等資產刪除或暫存過期回收時移除", "upload_id", uploadID, "key", key, "error", err)
	}
}

func (s *Service) rejectFirstChunk(ctx context.Context, uploadID uuid.UUID, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	reason := string(model.RejectUnrecognizedFormat)
	if _, err := s.q(ctx).RejectSecureUploadAtFirstChunk(ctx, store.RejectSecureUploadAtFirstChunkParams{ID: uploadID, Reason: &reason, Now: s.d.Now()}); err != nil {
		s.d.Logger.ErrorContext(ctx, "media：標記機敏上傳被拒失敗，留給 GC", "upload_id", uploadID, "error", err)
	}
	return cause
}

type exactReader struct {
	r   io.Reader
	n   int64
	eof bool
}

func (e *exactReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	e.n += int64(n)
	if errors.Is(err, io.EOF) {
		e.eof = true
	}
	return n, err
}

// PutSecure 是伺服器內部的機敏直接寫入（B1-D14）：出貨單等由伺服器產生的 PDF 或圖片走這裡。同一個分段格式：呼叫方宣告總大小 size
// （1 到 MaxSecureUploadSize，用來算段總數），BeginSecureUpload 後邊讀邊依 SecureChunkSize 切段依序推進，同一時間只緩衝一段，不整檔緩衝。
// 實際讀到的位元組數與 size 不符（不足或多出）回 InvalidInput，已寫的段與會話由 GC 在過期後回收。
// 呼叫方是沒有人能重試的伺服器內部，所以每段滿了等名額（受 ctx 約束）而不是回 ResourceExhausted。不得在 transaction 內呼叫。
func (s *Service) PutSecure(ctx context.Context, uploader model.Uploader, body io.Reader, size int64) (SecureResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return SecureResult{}, err
	}
	if s.d.Keyring == nil {
		return SecureResult{}, model.ErrNoKeyring
	}
	begin, err := s.BeginSecureUpload(ctx, uploader, size)
	if err != nil {
		return SecureResult{}, err
	}
	src := &exactReader{r: body}
	mismatch := apperr.InvalidInputOn([]string{"body"}, "%s", text.MediaSecureSizeMismatch(loc(ctx), text.MediaSecureSizeMismatchArgs{Size: int(size)}))
	var last SecureChunkResult
	for i := range begin.ChunkCount {
		n := secureChunkLength(size, begin.ChunkCount, i)
		last, err = s.putChunk(ctx, uploader, begin.Token, i, io.LimitReader(src, n), n, true)
		if err != nil {
			if src.eof && src.n < int64(i)*model.SecureChunkSize+n {
				return SecureResult{}, mismatch
			}
			return SecureResult{}, err
		}
	}
	var probe [1]byte
	if n, _ := src.Read(probe[:]); n > 0 {
		return SecureResult{}, mismatch
	}
	return SecureResult{Token: begin.Token, ContentType: last.ContentType, SizeBytes: last.SizeBytes, ExpiresAt: last.ExpiresAt}, nil
}

func chunkSessionOpen(ctx context.Context, row store.MediaStagedUpload, now time.Time) error {
	switch model.UploadState(row.State) {
	case model.StateUploading:
		if !row.ExpiresAt.After(now) {
			return apperr.FailedPrecondition("%s", text.MediaUploadExpired(loc(ctx)))
		}
		return nil
	case model.StateRejected:
		return apperr.FailedPrecondition("%s", text.MediaUploadRejected(loc(ctx)))
	case model.StateReady, model.StateConfirmed:
		if row.ChunkCount == nil {
			return fmt.Errorf("media：機敏暫存上傳 %s 沒有段總數", row.ID)
		}
		return nextIndexMismatch(ctx, int(*row.ChunkCount))
	case model.StatePending, model.StateUploaded, model.StateDeleting, model.StateDeleted:
		return apperr.FailedPrecondition("%s", text.MediaUploadExpired(loc(ctx)))
	}
	return apperr.FailedPrecondition("%s", text.MediaUploadExpired(loc(ctx)))
}

func nextIndexMismatch(ctx context.Context, next int) error {
	return apperr.FailedPrecondition("%s", text.MediaSecureChunkIndexMismatch(loc(ctx), text.MediaSecureChunkIndexMismatchArgs{Next: next})).WithMetadata(metaNextIndex, strconv.Itoa(next))
}

func chunkLengthMismatch(ctx context.Context, expected int64) error {
	return apperr.InvalidInputOn([]string{"length"}, "%s", text.MediaSecureChunkLengthMismatch(loc(ctx), text.MediaSecureChunkLengthMismatchArgs{Expected: int(expected)}))
}

func chunkReadErr(ctx context.Context, err error, expected int64) error {
	if errors.Is(err, errTooLarge) {
		return chunkLengthMismatch(ctx, expected)
	}
	return fmt.Errorf("media：讀取機敏上傳：%w", err)
}
