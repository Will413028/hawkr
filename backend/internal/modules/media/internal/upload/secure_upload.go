package upload

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// SecureResult 是 UploadSecure 的結果；資產 ID 不回給上傳者，確認前只能憑 Token 讀（ReadStagedSecure）。
type SecureResult struct {
	Token       string
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
)

var errTooLarge = errors.New("media：機敏上傳超過大小上限")

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

// UploadSecure 在請求內辨認型別、邊讀邊加密到本機暫存檔，再把密文寫到 secure/，成功後建資產列（STAGED）與 READY 的暫存列並回 token。
// 明文只在記憶體的一個段內，落磁碟與落物件儲存的都只有密文；被拒的輸入（型別、空內容、超過上限）不寫任何物件。
//
// 順序（收斂，不留收不回來的物件）：驗證與加密到暫存檔 → 一個 transaction 建暫存列（UPLOADED、final_key、tmp_key 與 declared_md5 為 NULL）→
// transaction 外 Put → 一個 transaction 條件式建資產列並把暫存列轉 READY。Put 或最後的 transaction 失敗：先把列條件式轉 REJECTED，
// 轉成功才刪物件；轉換 0 列且重讀是 READY（commit 其實成功）就照成功回 token。行程中斷留下的 UPLOADED 列與物件由 GC 在過期加寬限後回收。
// 不得在 transaction 內呼叫。
func (s *Service) UploadSecure(ctx context.Context, uploader model.Uploader, body io.Reader) (SecureResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return SecureResult{}, err
	}
	if s.d.Keyring == nil {
		return SecureResult{}, model.ErrNoKeyring
	}
	if uploader.ID <= 0 {
		return SecureResult{}, apperr.InvalidInputOn([]string{"uploader"}, "上傳者不合法")
	}
	select {
	case s.secureSlots <- struct{}{}:
		defer func() { <-s.secureSlots }()
	default:
		return SecureResult{}, apperr.ResourceExhausted(secureBusyRetryAfter, "同時進行的機敏上傳太多，請稍後再試")
	}

	br := bufio.NewReaderSize(&capReader{r: body, max: model.MaxSecureUploadSize}, sniffBytes*8)
	head, err := br.Peek(sniffBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return SecureResult{}, secureReadErr(err)
	}
	contentType, err := sniffSecure(head)
	if err != nil {
		return SecureResult{}, err
	}

	dek, err := secure.NewDEK()
	if err != nil {
		return SecureResult{}, err
	}
	tmp, err := os.CreateTemp("", "media-secure-*")
	if err != nil {
		return SecureResult{}, fmt.Errorf("media：建立機敏暫存檔：%w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	plainSize, err := secure.Encrypt(tmp, br, dek)
	if err != nil {
		return SecureResult{}, secureReadErr(err)
	}
	if plainSize == 0 {
		return SecureResult{}, apperr.InvalidInputOn([]string{"body"}, "上傳的內容是空的")
	}
	info, err := tmp.Stat()
	if err != nil {
		return SecureResult{}, fmt.Errorf("media：讀取機敏暫存檔：%w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return SecureResult{}, fmt.Errorf("media：回到機敏暫存檔開頭：%w", err)
	}

	now := s.d.Now()
	uploadID, assetID, token := s.d.NewID(), s.d.NewID(), newToken()
	kekID, wrapped, err := s.d.Keyring.Wrap(assetID, dek)
	if err != nil {
		return SecureResult{}, err
	}
	finalKey := storage.NewKey(storage.PrefixSecure)
	expiresAt := now.Add(model.UploadTTL)

	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return s.q(ctx).InsertSecureUpload(ctx, store.InsertSecureUploadParams{
			ID: uploadID, TokenHash: hashToken(token), UploaderID: uploader.ID, UploaderAdmin: uploader.Admin,
			DeclaredSize: plainSize, DeclaredContentType: contentType, FinalKey: &finalKey, ExpiresAt: expiresAt, Now: now,
		})
	})
	if err != nil {
		return SecureResult{}, fmt.Errorf("media：建立機敏暫存上傳：%w", err)
	}
	result := SecureResult{Token: token, ContentType: contentType, SizeBytes: plainSize, ExpiresAt: expiresAt}

	putCtx, cancel := context.WithTimeout(ctx, model.SecureUploadTimeout)
	defer cancel()
	if err := s.d.Store.Put(putCtx, storage.PutInput{Key: finalKey, Body: tmp, Size: info.Size(), ContentType: secureObjectType}); err != nil {
		return s.abortSecure(ctx, uploadID, finalKey, result, fmt.Errorf("media：寫入機敏物件：%w", err))
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Now()
		q := s.q(ctx)
		if err := q.InsertAsset(ctx, store.InsertAssetParams{
			ID: assetID, Visibility: string(model.VisibilitySecure), ObjectKey: finalKey, ContentType: contentType, SizeBytes: plainSize,
			KekID: &kekID, WrappedDek: wrapped, Now: now,
		}); err != nil {
			return fmt.Errorf("media：建立機敏資產：%w", err)
		}
		n, err := q.MarkUploadReady(ctx, store.MarkUploadReadyParams{ID: uploadID, AssetID: &assetID, Now: now})
		if err != nil {
			return fmt.Errorf("media：標記就緒：%w", err)
		}
		if n == 0 {
			return errLost
		}
		return nil
	})
	if err != nil {
		return s.abortSecure(ctx, uploadID, finalKey, result, err)
	}
	return result, nil
}

// secureReadErr 把讀 body 的錯誤分類：超過上限是 InvalidInput，其餘（client 中斷等）原樣回傳。
func secureReadErr(err error) error {
	if errors.Is(err, errTooLarge) {
		return apperr.InvalidInputOn([]string{"body"}, "上傳的大小超過上限 %d 位元組", model.MaxSecureUploadSize)
	}
	return fmt.Errorf("media：讀取機敏上傳：%w", err)
}

// abortSecure 收斂失敗：先把列條件式轉 REJECTED，轉成功才刪物件；轉換 0 列且重讀是 READY（最後的 transaction 其實已提交）就照成功回傳，
// 重讀是 DELETING 或 DELETED（輸給 GC）回 FailedPrecondition「已過期」。內部的 errLost 不外漏。
// 不受 ctx 取消影響：client 斷線時列與物件仍要收尾，但整段收尾有 model.SecureAbortTimeout 的時限。
// 轉換本身失敗時什麼都不刪，列與物件留給 GC 在過期加寬限後回收。
func (s *Service) abortSecure(ctx context.Context, uploadID uuid.UUID, finalKey string, ok SecureResult, cause error) (SecureResult, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.SecureAbortTimeout)
	defer cancel()
	if errors.Is(cause, errLost) {
		cause = errors.New("media：機敏上傳的暫存列已被別處轉走，沒有完成")
	}
	reason := string(model.RejectProcessingFailed)
	n, err := s.q(ctx).MarkUploadRejected(ctx, store.MarkUploadRejectedParams{ID: uploadID, Reason: &reason, Now: s.d.Now()})
	if err != nil {
		s.d.Logger.ErrorContext(ctx, "media：機敏上傳失敗後標記拒絕失敗，留給 GC", "upload_id", uploadID, "error", err)
		return SecureResult{}, cause
	}
	if n == 1 {
		if err := s.d.Store.Delete(ctx, finalKey); err != nil {
			s.d.Logger.WarnContext(ctx, "media：刪除失敗的機敏物件失敗，留給 GC", "upload_id", uploadID, "error", err)
		}
		return SecureResult{}, cause
	}
	cur, err := s.q(ctx).GetUpload(ctx, uploadID)
	if err != nil {
		return SecureResult{}, fmt.Errorf("media：機敏上傳失敗後重讀暫存列：%w（原因：%w）", err, cause)
	}
	state := model.UploadState(cur.State)
	if state == model.StateReady {
		return ok, nil
	}
	if state == model.StateDeleting || state == model.StateDeleted {
		return SecureResult{}, apperr.FailedPrecondition("這筆上傳已過期")
	}
	return SecureResult{}, cause
}
