package upload

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // 暫存列的 declared_md5 欄位要求內容 MD5，不是安全用途
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// PublicPutResult 是 PutPublic 的結果；資產 ID 不回給呼叫方，確認後由 Confirm 回傳。
type PublicPutResult struct {
	Token       string
	ContentType string
	SizeBytes   int64
	ExpiresAt   time.Time
}

// preparePublic 是 PutPublic 的驗證與重新編碼：讀 body（上限以實際讀到的位元組套用）、辨認並重新編碼，與處理 job 用同一組規則。
// 解碼前先取 publicSlots 的名額，等待受 ctx 約束。辨認不出、無法解碼、超過像素或邊長上限、空內容與超過大小上限都是 InvalidInput；
// 管線自己的永久失敗（image.ErrProcessing）與其餘錯誤都不是 InvalidInput。
func (s *Service) preparePublic(ctx context.Context, body io.Reader) (image.Result, error) {
	data, err := io.ReadAll(&capReader{r: body, max: model.MaxPublicUploadSize})
	if errors.Is(err, errTooLarge) {
		return image.Result{}, apperr.InvalidInputOn([]string{"body"}, "%s", text.MediaPublicUploadTooLarge(loc(ctx), text.MediaPublicUploadTooLargeArgs{Max: int(model.MaxPublicUploadSize)}))
	}
	if err != nil {
		return image.Result{}, fmt.Errorf("media：讀取公開上傳：%w", err)
	}
	if len(data) == 0 {
		return image.Result{}, apperr.InvalidInputOn([]string{"body"}, "%s", text.MediaPublicContentEmpty(loc(ctx)))
	}
	release, err := acquire(ctx, s.publicSlots, true, nil)
	if err != nil {
		return image.Result{}, err
	}
	defer release()
	res, err := sanitizePublic(ctx, data)
	return res, publicErr(ctx, err)
}

// publicErr 把 Sanitize 的錯誤分類給 PutPublic 的呼叫方：輸入的問題是 InvalidInput，管線自己的失敗是內部錯誤（不說成輸入的格式不對）。
func publicErr(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case isImageRejection(err):
		return apperr.InvalidInputOn([]string{"body"}, "%s", text.MediaPublicContentUnsupported(loc(ctx)))
	case isProcessingFailure(err):
		return fmt.Errorf("media：處理公開圖片失敗：%w", err)
	}
	return err
}

// PutPublic 在請求內驗證並重新編碼圖片，寫入 public/，成功後建資產列（STAGED）與 READY 的暫存列並回 token，之後由擁有者在自己的 transaction 內 Confirm。
// 順序與失敗收斂見 commitDirect：被拒的輸入不寫任何物件也不建列；失敗留下的列與物件由 GC 回收。
// 暫存列的 tmp_key 是從未寫入的保留 key，declared_md5 是寫入物件（重新編碼後內容）的 MD5，為了滿足 public 列的欄位約束。不得在 transaction 內呼叫。
func (s *Service) PutPublic(ctx context.Context, uploader model.Uploader, body io.Reader) (PublicPutResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return PublicPutResult{}, err
	}
	if uploader.ID <= 0 {
		return PublicPutResult{}, apperr.InvalidInputOn([]string{"uploader"}, "%s", text.MediaUploaderInvalid(loc(ctx)))
	}
	res, err := s.preparePublic(ctx, body)
	if err != nil {
		return PublicPutResult{}, err
	}
	sum := md5.Sum(res.Data) //nolint:gosec // 見上
	declaredMD5 := base64.StdEncoding.EncodeToString(sum[:])

	contentType := res.Format.ContentType()
	size := int64(len(res.Data))
	now := s.d.Now()
	uploadID, assetID, token := s.d.NewID(), s.d.NewID(), newToken()
	tmpKey, finalKey := storage.NewKey(storage.PrefixTmp), storage.NewKey(storage.PrefixPublic)
	expiresAt := now.Add(model.UploadTTL)

	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := q.InsertUpload(ctx, store.InsertUploadParams{
			ID: uploadID, TokenHash: hashToken(token), UploaderID: uploader.ID, UploaderAdmin: uploader.Admin,
			Visibility: string(model.VisibilityPublic), DeclaredSize: size, DeclaredContentType: contentType, DeclaredMd5: &declaredMD5,
			TmpKey: &tmpKey, ExpiresAt: expiresAt, Now: now,
		}); err != nil {
			return err
		}
		n, err := q.MarkUploadUploaded(ctx, store.MarkUploadUploadedParams{ID: uploadID, FinalKey: &finalKey, Now: now})
		if err != nil {
			return err
		}
		if n != 1 {
			return errLost
		}
		return nil
	})
	if err != nil {
		return PublicPutResult{}, fmt.Errorf("media：建立公開暫存上傳：%w", err)
	}
	asset := store.InsertAssetParams{ID: assetID, Visibility: string(model.VisibilityPublic), ObjectKey: finalKey, ContentType: contentType, SizeBytes: size}
	if err := s.commitDirect(ctx, uploadID, asset, bytes.NewReader(res.Data), size, contentType); err != nil {
		return PublicPutResult{}, err
	}
	return PublicPutResult{Token: token, ContentType: contentType, SizeBytes: size, ExpiresAt: expiresAt}, nil
}
