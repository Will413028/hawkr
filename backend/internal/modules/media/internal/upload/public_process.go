package upload

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // 驗的是 client 宣告的 Content-MD5 完整性，不是安全用途
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
)

// NewProcessor 組出處理 job 的內容：public 走 PublicProcess；機敏檔案經單體串流上傳（UploadSecure），不經處理 job，
// 處理 job 遇到 SECURE 列是永久失敗（沒有人該為它排 job）。
func NewProcessor(store storage.Store) ProcessFunc {
	return func(ctx context.Context, in ProcessInput) (ProcessResult, error) {
		if in.Visibility != model.VisibilityPublic {
			return ProcessResult{}, model.Permanent(model.RejectProcessingFailed,
				errors.New("機敏上傳不經處理 job"))
		}
		return PublicProcess(ctx, store, in)
	}
}

// PublicProcess 讀 tmp 物件，驗實際大小與 MD5 等於宣告，辨認格式、重新編碼（去 metadata、轉 sRGB、套用方向），寫到 in.FinalKey。
// 大小、MD5 不符與辨認不出（含無法解碼、超過像素上限）是永久失敗；儲存錯誤原樣回傳，由 River 重試。
// 宣告大小超過 model.MaxPublicUploadSize 的直接拒絕；記憶體以宣告大小為上限：多讀一個位元組就能判斷「比宣告大」，不必把任意大的物件讀完。
func PublicProcess(ctx context.Context, store storage.Store, in ProcessInput) (ProcessResult, error) {
	if in.DeclaredSize > model.MaxPublicUploadSize {
		return ProcessResult{}, model.Permanent(model.RejectSizeMismatch,
			fmt.Errorf("宣告大小 %d 超過上限 %d", in.DeclaredSize, model.MaxPublicUploadSize))
	}
	rc, err := store.Get(ctx, in.TmpKey)
	if errors.Is(err, storage.ErrNotFound) {
		// tmp 物件不存在：與大小不符同一個原因碼，見 model.RejectSizeMismatch。
		return ProcessResult{}, model.Permanent(model.RejectSizeMismatch, err)
	}
	if err != nil {
		return ProcessResult{}, err
	}
	data, err := io.ReadAll(io.LimitReader(rc, in.DeclaredSize+1))
	_ = rc.Close()
	if err != nil {
		return ProcessResult{}, err
	}
	if int64(len(data)) != in.DeclaredSize {
		return ProcessResult{}, model.Permanent(model.RejectSizeMismatch,
			fmt.Errorf("實際大小與宣告 %d 不符", in.DeclaredSize))
	}
	want, err := base64.StdEncoding.DecodeString(in.DeclaredMD5)
	if err != nil || len(want) != md5.Size {
		return ProcessResult{}, model.Permanent(model.RejectChecksumMismatch, errors.New("宣告的 MD5 格式不合法"))
	}
	sum := md5.Sum(data) //nolint:gosec // 見上
	if subtle.ConstantTimeCompare(sum[:], want) != 1 {
		return ProcessResult{}, model.Permanent(model.RejectChecksumMismatch, errors.New("MD5 與宣告不符"))
	}
	res, err := sanitizePublic(ctx, data)
	if err != nil {
		if reason, ok := rejectionReason(err); ok {
			return ProcessResult{}, model.Permanent(reason, err)
		}
		return ProcessResult{}, err
	}
	ct := res.Format.ContentType()
	if err := store.Put(ctx, storage.PutInput{Key: in.FinalKey, Body: bytes.NewReader(res.Data), Size: int64(len(res.Data)), ContentType: ct}); err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{SizeBytes: int64(len(res.Data)), ContentType: ct}, nil
}

// sanitizePublic 是 public 圖片的辨認與重新編碼，處理 job 與 PutPublic 共用，上限只取自 model。
func sanitizePublic(ctx context.Context, data []byte) (image.Result, error) {
	return image.Sanitize(ctx, data, image.Limits{FramePixels: model.MaxImagePixels, AnimatedPixels: model.MaxAnimatedPixels, MaxDimension: model.MaxImageDimension})
}

// isProcessingFailure 回報錯誤是不是管線自己的永久失敗（不是輸入的問題，也不會因重試而成功）。
func isProcessingFailure(err error) bool { return errors.Is(err, image.ErrProcessing) }

// rejectionReason 把 Sanitize 的永久失敗對到暫存列的拒絕原因：輸入的問題是 UNRECOGNIZED_FORMAT，管線自己的失敗是 PROCESSING_FAILED；其餘不是永久失敗。
func rejectionReason(err error) (model.RejectReason, bool) {
	switch {
	case isImageRejection(err):
		return model.RejectUnrecognizedFormat, true
	case isProcessingFailure(err):
		return model.RejectProcessingFailed, true
	}
	return "", false
}

// isImageRejection 回報錯誤是不是輸入本身的問題（辨認不出、無法解碼、超過上限），這類不會因重試而成功。
func isImageRejection(err error) bool {
	return errors.Is(err, image.ErrUnrecognized) || errors.Is(err, image.ErrMalformed) || errors.Is(err, image.ErrTooLarge)
}
