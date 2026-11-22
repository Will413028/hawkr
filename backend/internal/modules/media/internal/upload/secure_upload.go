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
	sniffBytes     = 512
	pdfContentType = "application/pdf"
	// secureObjectType 是 R2 物件的型別：密文一律是 octet-stream，真正的型別存在資產列。
	secureObjectType = "application/octet-stream"
	// secureBusyRetryAfter 是滿載時建議 client 重試的間隔。
	secureBusyRetryAfter = 5 * time.Second
	// metaNextIndex 是 FailedPrecondition 的 metadata 鍵：client 依它續傳（比照 tus 的 offset 不符）。
	metaNextIndex = "next_index"
)

var errTooLarge = errors.New("media：上傳超過大小上限")

// capReader 在讀超過 max 個位元組時回錯誤：大小不收宣告，上限以實際讀到的位元組數套用。
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

// sniffSecure 依前 512 位元組的 magic bytes 辨認：PDF 與 image.Detect 放行的圖片，其餘 InvalidInput。型別不收宣告。
func sniffSecure(head []byte) (string, error) {
	if len(head) == 0 {
		return "", apperr.InvalidInputOn([]string{"body"}, "上傳的內容是空的")
	}
	if len(head) >= 5 && string(head[:5]) == "%PDF-" {
		return pdfContentType, nil
	}
	f, err := image.Detect(head)
	if err != nil {
		return "", apperr.InvalidInputOn([]string{"body"}, "機敏上傳只接受 PDF 與 JPEG、PNG、WebP、GIF 圖片")
	}
	return f.ContentType(), nil
}

// secureChunkCount 是宣告的總大小（1 到 MaxSecureUploadSize）要切成的段數。
// u32 與 i32 轉換段序號與段總數：兩者最多 MaxSecureUploadSize / SecureChunkSize（25），不會溢位。
func u32(n int) uint32 { return uint32(n) } //nolint:gosec // 見上
func i32(n int) int32  { return int32(n) }  //nolint:gosec // 見上

func secureChunkCount(size int64) int {
	return int((size + model.SecureChunkSize - 1) / model.SecureChunkSize)
}

// secureChunkLength 是第 index 段應有的明文長度：除最後一段外剛好 SecureChunkSize，最後一段是餘數（1 到 SecureChunkSize）。
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
	if uploader.ID <= 0 {
		return SecureBeginResult{}, apperr.InvalidInputOn([]string{"uploader"}, "上傳者不合法")
	}
	if size < 1 || size > model.MaxSecureUploadSize {
		return SecureBeginResult{}, apperr.InvalidInputOn([]string{"size"}, "機敏上傳的大小必須在 1 到 %d 位元組之間", model.MaxSecureUploadSize)
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
			DeclaredSize: size, PlannedAssetID: assetID, ChunkCount: i32(count), KekID: kekID, WrappedDek: wrapped,
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
	if uploader.ID <= 0 {
		return SecureChunkResult{}, apperr.InvalidInputOn([]string{"uploader"}, "上傳者不合法")
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
		return SecureChunkResult{}, nextIndexMismatch(next)
	}
	expected := secureChunkLength(row.DeclaredSize, count, index)
	if length != expected {
		return SecureChunkResult{}, chunkLengthMismatch(expected)
	}
	release, err := acquire(ctx, s.secureSlots, wait, apperr.ResourceExhausted(secureBusyRetryAfter, "同時處理的機敏上傳太多，請稍後再試"))
	if err != nil {
		return SecureChunkResult{}, err
	}
	defer release()

	br := bufio.NewReaderSize(&capReader{r: body, max: expected}, sniffBytes*8)
	var sniffed *string
	if index == 0 {
		head, err := br.Peek(sniffBytes)
		if err != nil && !errors.Is(err, io.EOF) {
			return SecureChunkResult{}, chunkReadErr(err, expected)
		}
		if len(head) == 0 {
			return SecureChunkResult{}, chunkLengthMismatch(expected)
		}
		contentType, err := sniffSecure(head)
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
		return SecureChunkResult{}, chunkReadErr(err, expected)
	}
	if plainSize != expected {
		return SecureChunkResult{}, chunkLengthMismatch(expected)
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

// advance 是推進的 transaction：條件式推進 next_index 並記下這段的 key；最後一段同一個 transaction 建資產列並轉 READY。
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

// settleLostChunk 處理推進失敗：先看這段的 key 是不是已經記進 secure_chunks（transaction 其實已提交，例如 commit 時連線中斷），
// 是就回 errChunkCommitted（caller 照成功處理）；確定沒有才刪自己的物件，查不出來就什麼都不刪（刪錯會破壞已提交的段，
// 沒刪的孤兒由 GC 依前綴回收）。回傳給 client 的錯誤依會話現況：輸給並行的同一段、過期、已被拒。
func (s *Service) settleLostChunk(ctx context.Context, row store.MediaStagedUpload, index int, key string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	c, err := s.q(ctx).GetSecureChunk(ctx, store.GetSecureChunkParams{AssetID: *row.PlannedAssetID, Idx: i32(index)})
	switch {
	case err == nil && c.ObjectKey == key:
		return errChunkCommitted
	case err == nil || errors.Is(err, pgx.ErrNoRows):
		s.deleteChunkObject(ctx, row.ID, key)
	default:
		s.d.Logger.WarnContext(ctx, "media：推進失敗後查不出自己的段是否已記錄，物件留給 GC", "upload_id", row.ID, "index", index, "error", err)
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
		return nextIndexMismatch(int(*cur.NextIndex))
	}
	return cause
}

// errChunkCommitted 是 settleLostChunk 的內部訊號：推進的 transaction 其實已提交，這段算成功。
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

// deleteChunkObject 刪自己這次嘗試寫的物件（key 只屬於這次嘗試）；不受 ctx 取消影響但有時限，失敗只記錄，孤兒由 GC 依前綴回收。
func (s *Service) deleteChunkObject(ctx context.Context, uploadID uuid.UUID, key string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	if err := s.d.Store.Delete(ctx, key); err != nil {
		s.d.Logger.WarnContext(ctx, "media：刪除沒推進成功的機敏段失敗，留給 GC", "upload_id", uploadID, "key", key, "error", err)
	}
}

// rejectFirstChunk 把第 0 段辨認不出的會話條件式轉 REJECTED（只在 UPLOADING 且 next_index 0），然後回 InvalidInput；不寫任何物件。
// 轉換失敗或輸給並行的請求都不改變回給這次請求的結論（這份內容不是 PDF 或圖片）。
func (s *Service) rejectFirstChunk(ctx context.Context, uploadID uuid.UUID, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	reason := string(model.RejectUnrecognizedFormat)
	if _, err := s.q(ctx).RejectSecureUploadAtFirstChunk(ctx, store.RejectSecureUploadAtFirstChunkParams{ID: uploadID, Reason: &reason, Now: s.d.Now()}); err != nil {
		s.d.Logger.ErrorContext(ctx, "media：標記機敏上傳被拒失敗，留給 GC", "upload_id", uploadID, "error", err)
	}
	return cause
}

// PutSecure 是伺服器內部的機敏直接寫入（B1-D14）：出貨單等由伺服器產生的 PDF 或圖片走這裡。同一個分段格式：讀完 body（以實際位元組數套用
// MaxSecureUploadSize 上限，因為段總數要在寫第一段前定下來）、BeginSecureUpload、依 SecureChunkSize 切段依序推進，不另留單一物件的寫法。
// 呼叫方是沒有人能重試的伺服器內部，所以每段滿了等名額（受 ctx 約束）而不是回 ResourceExhausted。
// 失敗留下的會話與已寫的段由 GC 在過期後回收。不得在 transaction 內呼叫。
func (s *Service) PutSecure(ctx context.Context, uploader model.Uploader, body io.Reader) (SecureResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return SecureResult{}, err
	}
	if s.d.Keyring == nil {
		return SecureResult{}, model.ErrNoKeyring
	}
	data, err := io.ReadAll(&capReader{r: body, max: model.MaxSecureUploadSize})
	if err != nil {
		return SecureResult{}, secureReadErr(err)
	}
	if len(data) == 0 {
		return SecureResult{}, apperr.InvalidInputOn([]string{"body"}, "上傳的內容是空的")
	}
	begin, err := s.BeginSecureUpload(ctx, uploader, int64(len(data)))
	if err != nil {
		return SecureResult{}, err
	}
	size := int64(len(data))
	var last SecureChunkResult
	for i := range begin.ChunkCount {
		start := int64(i) * model.SecureChunkSize
		n := secureChunkLength(size, begin.ChunkCount, i)
		last, err = s.putChunk(ctx, uploader, begin.Token, i, bytes.NewReader(data[start:start+n]), n, true)
		if err != nil {
			return SecureResult{}, err
		}
	}
	return SecureResult{Token: begin.Token, ContentType: last.ContentType, SizeBytes: last.SizeBytes, ExpiresAt: last.ExpiresAt}, nil
}

// chunkSessionOpen 檢查會話還能收段：只有 UPLOADING 且未過期；已完成（READY、CONFIRMED）回 FailedPrecondition 並帶段總數當 next_index
// （client 重送最後一段時知道已經完成），被拒與過期、已進 GC 各回對應的 FailedPrecondition。
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
		return nextIndexMismatch(int(*row.ChunkCount))
	case model.StatePending, model.StateUploaded, model.StateDeleting, model.StateDeleted:
		return apperr.FailedPrecondition("%s", text.MediaUploadExpired(loc(ctx)))
	}
	return apperr.FailedPrecondition("%s", text.MediaUploadExpired(loc(ctx)))
}

func nextIndexMismatch(next int) error {
	return apperr.FailedPrecondition("段序號不符，下一段是 %d", next).WithMetadata(metaNextIndex, strconv.Itoa(next))
}

func chunkLengthMismatch(expected int64) error {
	return apperr.InvalidInputOn([]string{"length"}, "這一段必須剛好 %d 位元組", expected)
}

// chunkReadErr 把讀 body 的錯誤分類：超過這段的長度是 InvalidInput，其餘（client 中斷等）原樣回傳。
func chunkReadErr(err error, expected int64) error {
	if errors.Is(err, errTooLarge) {
		return chunkLengthMismatch(expected)
	}
	return fmt.Errorf("media：讀取機敏上傳：%w", err)
}

// secureReadErr 把讀 body 的錯誤分類：超過上限是 InvalidInput，其餘（client 中斷等）原樣回傳。
func secureReadErr(err error) error {
	if errors.Is(err, errTooLarge) {
		return apperr.InvalidInputOn([]string{"body"}, "上傳的大小超過上限 %d 位元組", model.MaxSecureUploadSize)
	}
	return fmt.Errorf("media：讀取機敏上傳：%w", err)
}
