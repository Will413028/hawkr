package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// commitDirect 是直接寫入（PutPublic）的後半段：暫存列已以 UPLOADED 與 final_key 建好；transaction 外 Put，
// 再以一個 transaction 建資產列並把暫存列轉 READY。任何一步失敗都走 abortDirect 收斂；最後的 transaction 其實已提交時視為成功。
func (s *Service) commitDirect(ctx context.Context, uploadID uuid.UUID, asset store.InsertAssetParams, body io.Reader, size int64, objectType string) error {
	putCtx, cancel := context.WithTimeout(ctx, model.DirectPutTimeout)
	defer cancel()
	if err := s.d.Store.Put(putCtx, storage.PutInput{Key: asset.ObjectKey, Body: body, Size: size, ContentType: objectType}); err != nil {
		return s.abortDirect(ctx, uploadID, asset.ObjectKey, fmt.Errorf("media：寫入物件：%w", err))
	}
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Now()
		q := s.q(ctx)
		asset.Now = now
		if err := q.InsertAsset(ctx, asset); err != nil {
			return fmt.Errorf("media：建立資產：%w", err)
		}
		n, err := q.MarkUploadReady(ctx, store.MarkUploadReadyParams{ID: uploadID, AssetID: &asset.ID, Now: now})
		if err != nil {
			return fmt.Errorf("media：標記就緒：%w", err)
		}
		if n == 0 {
			return errLost
		}
		return nil
	})
	if err != nil {
		return s.abortDirect(ctx, uploadID, asset.ObjectKey, err)
	}
	return nil
}

// abortDirect 收斂失敗：先把列條件式轉 REJECTED，轉成功才刪物件；轉換 0 列且重讀是 READY（最後的 transaction 其實已提交）就照成功回傳 nil，
// 重讀是 DELETING 或 DELETED（輸給 GC）回 FailedPrecondition「已過期」。內部的 errLost 不外漏。
// 不受 ctx 取消影響：client 斷線時列與物件仍要收尾，但整段收尾有 model.DirectAbortTimeout 的時限。
// 轉換本身失敗時什麼都不刪，列與物件留給 GC 在過期加寬限後回收。
func (s *Service) abortDirect(ctx context.Context, uploadID uuid.UUID, finalKey string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	if errors.Is(cause, errLost) {
		cause = errors.New("media：直接上傳的暫存列已被別處轉走，沒有完成")
	}
	reason := string(model.RejectProcessingFailed)
	n, err := s.q(ctx).MarkUploadRejected(ctx, store.MarkUploadRejectedParams{ID: uploadID, Reason: &reason, Now: s.d.Now()})
	if err != nil {
		s.d.Logger.ErrorContext(ctx, "media：直接上傳失敗後標記拒絕失敗，留給 GC", "upload_id", uploadID, "error", err)
		return cause
	}
	if n == 1 {
		if err := s.d.Store.Delete(ctx, finalKey); err != nil {
			s.d.Logger.WarnContext(ctx, "media：刪除失敗的物件失敗，留給 GC", "upload_id", uploadID, "error", err)
		}
		return cause
	}
	cur, err := s.q(ctx).GetUpload(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("media：直接上傳失敗後重讀暫存列：%w（原因：%w）", err, cause)
	}
	state := model.UploadState(cur.State)
	if state == model.StateReady {
		return nil
	}
	if state == model.StateDeleting || state == model.StateDeleted {
		return apperr.FailedPrecondition("%s", text.MediaUploadExpired(loc(ctx)))
	}
	return cause
}
