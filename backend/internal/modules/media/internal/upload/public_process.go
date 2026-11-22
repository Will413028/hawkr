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

// NewProcessor 組出處理 job 的內容：public 走 PublicProcess；secure 交給 secure（之後提供），沒給時回暫時失敗。
func NewProcessor(store storage.Store, secure ProcessFunc) ProcessFunc {
	return func(ctx context.Context, in ProcessInput) (ProcessResult, error) {
		if in.Visibility == model.VisibilityPublic {
			return PublicProcess(ctx, store, in)
		}
		if secure == nil {
			return ProcessResult{}, errors.New("media/upload：機敏資產的處理尚未接線")
		}
		return secure(ctx, in)
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
	res, err := image.Sanitize(ctx, data, image.Limits{FramePixels: model.MaxImagePixels, AnimatedPixels: model.MaxAnimatedPixels, MaxDimension: model.MaxImageDimension})
	if err != nil {
		if errors.Is(err, image.ErrUnrecognized) || errors.Is(err, image.ErrMalformed) || errors.Is(err, image.ErrTooLarge) {
			return ProcessResult{}, model.Permanent(model.RejectUnrecognizedFormat, err)
		}
		return ProcessResult{}, err
	}
	ct := res.Format.ContentType()
	if err := store.Put(ctx, storage.PutInput{Key: in.FinalKey, Body: bytes.NewReader(res.Data), Size: int64(len(res.Data)), ContentType: ct}); err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{SizeBytes: int64(len(res.Data)), ContentType: ct}, nil
}
