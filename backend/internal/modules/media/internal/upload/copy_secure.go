package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/hold"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const CopyInterval = 30 * time.Second

func (s *Service) CopyToSecure(ctx context.Context, uploader model.Uploader, sourceID uuid.UUID) (uuid.UUID, error) {
	if err := model.RequireTx(ctx); err != nil {
		return uuid.UUID{}, err
	}
	if s.d.Keyring == nil {
		return uuid.UUID{}, model.ErrNoKeyring
	}
	if err := checkUploader(ctx, uploader); err != nil {
		return uuid.UUID{}, err
	}
	q := s.q(ctx)
	src, err := q.GetAssetView(ctx, sourceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, fmt.Errorf("media：讀取來源資產：%w", err)
	}
	if err != nil || deriveState(src) != model.AssetActive || model.Visibility(src.Visibility) != model.VisibilityPublic {
		return uuid.UUID{}, apperr.NotFound("%s", text.MediaAssetNotFound(loc(ctx)))
	}
	if src.SizeBytes < 1 || src.SizeBytes > model.MaxPublicUploadSize {
		return uuid.UUID{}, fmt.Errorf("media：來源資產 %s 的大小 %d 不在可複製的範圍", sourceID, src.SizeBytes)
	}
	dek, err := secure.NewDEK()
	if err != nil {
		return uuid.UUID{}, err
	}
	now := s.d.Now()
	copyID, uploadID := s.d.NewID(), s.d.NewID()
	kekID, wrapped, err := s.d.Keyring.Wrap(copyID, dek)
	if err != nil {
		return uuid.UUID{}, err
	}
	count := secureChunkCount(src.SizeBytes)
	prefix := storage.SecurePrefix(copyID)
	if err := hold.Place(ctx, q, []uuid.UUID{sourceID}, model.CopySource, copyID, now.Add(model.CopyHoldTTL), now, false); err != nil {
		if errors.Is(err, model.ErrAssetGone) || errors.Is(err, model.ErrAssetNotFound) {
			return uuid.UUID{}, apperr.NotFound("%s", text.MediaAssetNotFound(loc(ctx)))
		}
		return uuid.UUID{}, err
	}
	if err := q.InsertCopyAsset(ctx, store.InsertCopyAssetParams{
		ID: copyID, ObjectKey: prefix, ContentType: src.ContentType, SizeBytes: src.SizeBytes, KekID: kekID, WrappedDek: wrapped, ChunkCount: i32(count), Now: now,
	}); err != nil {
		return uuid.UUID{}, fmt.Errorf("media：建立副本資產：%w", err)
	}
	if err := q.InsertCopyUpload(ctx, store.InsertCopyUploadParams{
		ID: uploadID, TokenHash: hashToken(newToken()), UploaderID: uploader.ID, UploaderAdmin: uploader.Admin, DelegatedStoreID: delegatedStore(uploader),
		DeclaredSize: src.SizeBytes, DeclaredContentType: src.ContentType, FinalKey: &prefix, AssetID: copyID, ChunkCount: i32(count), KekID: kekID, WrappedDek: wrapped,
		ExpiresAt: now.Add(model.UploadTTL), Now: now,
	}); err != nil {
		return uuid.UUID{}, fmt.Errorf("media：建立副本的暫存列：%w", err)
	}
	if err := q.InsertConfirmedClaim(ctx, store.InsertConfirmedClaimParams{UploadID: uploadID, AssetID: &copyID, Now: now}); err != nil {
		return uuid.UUID{}, fmt.Errorf("media：認領副本：%w", err)
	}
	if err := q.InsertCopyRequest(ctx, store.InsertCopyRequestParams{AssetID: copyID, SourceAssetID: sourceID, Now: now}); err != nil {
		return uuid.UUID{}, fmt.Errorf("media：寫入複製請求：%w", err)
	}
	return copyID, nil
}

func (s *Service) ProcessCopies(ctx context.Context) (int, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return 0, err
	}
	if s.d.Keyring == nil {
		return 0, model.ErrNoKeyring
	}
	var errs []error
	total := 0
	for {
		rows, err := s.q(ctx).ListPendingCopyRequests(ctx, rewrapBatch)
		if err != nil {
			return total, errors.Join(append(errs, fmt.Errorf("media：列出待複製的請求：%w", err))...)
		}
		progress := 0
		for _, r := range rows {
			if err := s.copyOne(ctx, r); err != nil {
				errs = append(errs, err)
				continue
			}
			progress++
		}
		total += progress
		if progress == 0 || len(rows) < rewrapBatch {
			break
		}
	}
	return total, errors.Join(errs...)
}

func (s *Service) copyOne(ctx context.Context, r store.MediaAssetCopyRequest) error {
	q := s.q(ctx)
	src, err := q.GetAssetView(ctx, r.SourceAssetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.failCopy(ctx, r, "來源資產不存在")
	}
	if err != nil {
		return fmt.Errorf("media：讀取複製請求 %s 的來源：%w", r.AssetID, err)
	}
	if model.AssetState(src.State) != model.AssetStaged || model.Visibility(src.Visibility) != model.VisibilityPublic {
		return s.failCopy(ctx, r, "來源資產已不在或不是公開資產")
	}
	dst, err := q.GetAssetView(ctx, r.AssetID)
	if err != nil {
		return fmt.Errorf("media：讀取複製請求 %s 的副本：%w", r.AssetID, err)
	}
	if model.AssetState(dst.State) != model.AssetStaged || dst.KekID == nil || dst.ChunkCount == nil {
		return s.failCopy(ctx, r, "副本資產已不在或缺少加密欄位")
	}
	rc, err := s.d.Store.Get(ctx, src.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return s.failCopy(ctx, r, "來源物件不存在")
	}
	if err != nil {
		return fmt.Errorf("media：讀取複製請求 %s 的來源物件：%w", r.AssetID, err)
	}
	data, err := io.ReadAll(&capReader{r: rc, max: model.MaxPublicUploadSize})
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("media：讀取複製請求 %s 的來源位元組：%w", r.AssetID, err)
	}
	if int64(len(data)) != dst.SizeBytes {
		return s.failCopy(ctx, r, "來源物件大小與登記的不符")
	}
	dek, err := s.d.Keyring.Unwrap(dst.ID, *dst.KekID, dst.WrappedDek)
	if err != nil {
		return fmt.Errorf("media：解開副本 %s 的 DEK：%w", dst.ID, err)
	}
	count := int(*dst.ChunkCount)
	keys := make([]string, 0, count)
	for index := range count {
		key, err := s.putCopyChunk(ctx, dst.ID, dek, data, index, count)
		if err != nil {
			s.deleteCopyObjects(ctx, keys)
			return err
		}
		keys = append(keys, key)
	}
	won, abandoned := false, false
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := hold.SetLockTimeout(ctx, s.d.DB.Conn(ctx)); err != nil {
			return err
		}
		q := s.q(ctx)
		state, err := q.LockAssetStateShared(ctx, r.AssetID)
		if err != nil {
			return fmt.Errorf("media：鎖定副本 %s 的狀態：%w", r.AssetID, err)
		}
		if model.AssetState(state) != model.AssetStaged {
			abandoned = true
			failedAt := s.d.Now()
			if _, err := q.FailCopyRequest(ctx, store.FailCopyRequestParams{AssetID: r.AssetID, Now: &failedAt}); err != nil {
				return fmt.Errorf("media：標記複製失敗：%w", err)
			}
			if err := hold.ReleaseSource(ctx, q, model.CopySource, r.AssetID); err != nil {
				return fmt.Errorf("media：釋放待複製保全：%w", err)
			}
			return nil
		}
		now := s.d.Now()
		n, err := q.CompleteCopyRequest(ctx, store.CompleteCopyRequestParams{AssetID: r.AssetID, Now: &now})
		if err != nil {
			return fmt.Errorf("media：完成複製請求：%w", err)
		}
		if n == 0 {
			return nil
		}
		for index, key := range keys {
			if err := q.InsertSecureChunk(ctx, store.InsertSecureChunkParams{AssetID: r.AssetID, Idx: i32(index), ObjectKey: key}); err != nil {
				return fmt.Errorf("media：記下副本的段：%w", err)
			}
		}
		if err := hold.ReleaseSource(ctx, q, model.CopySource, r.AssetID); err != nil {
			return fmt.Errorf("media：釋放待複製保全：%w", err)
		}
		won = true
		return nil
	})
	if err != nil || !won {
		s.deleteCopyObjects(ctx, keys)
	}
	if err == nil && abandoned {
		s.d.Logger.WarnContext(ctx, "media：放棄複製請求", "asset_id", r.AssetID, "source_asset_id", r.SourceAssetID, "reason", "副本資產已在刪除")
	}
	return err
}

func (s *Service) putCopyChunk(ctx context.Context, assetID uuid.UUID, dek, data []byte, index, count int) (string, error) {
	start := int64(index) * model.SecureChunkSize
	end := min(start+model.SecureChunkSize, int64(len(data)))
	var cipherText bytes.Buffer
	if _, err := secure.EncryptChunk(&cipherText, bytes.NewReader(data[start:end]), dek, secure.Chunk{Asset: assetID, Index: u32(index), Count: u32(count)}); err != nil {
		return "", fmt.Errorf("media：加密副本 %s 的第 %d 段：%w", assetID, index, err)
	}
	key := storage.NewSecureChunkKey(assetID, index)
	putCtx, cancel := context.WithTimeout(ctx, model.DirectPutTimeout)
	defer cancel()
	if err := s.d.Store.Put(putCtx, storage.PutInput{Key: key, Body: bytes.NewReader(cipherText.Bytes()), Size: int64(cipherText.Len()), ContentType: secureObjectType}); err != nil {
		return "", fmt.Errorf("media：寫入副本 %s 的第 %d 段：%w", assetID, index, err)
	}
	return key, nil
}

func (s *Service) deleteCopyObjects(ctx context.Context, keys []string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), model.DirectAbortTimeout)
	defer cancel()
	for _, key := range keys {
		err := s.d.Store.Delete(ctx, key)
		if err == nil {
			continue
		}
		if recordErr := s.q(ctx).InsertOrphanObject(ctx, store.InsertOrphanObjectParams{ObjectKey: key, Now: s.d.Now()}); recordErr != nil {
			s.d.Logger.ErrorContext(ctx, "media：刪除沒記錄的副本段失敗，也沒能記入待刪清單，物件會遺留", "key", key, "error", err, "record_error", recordErr)
			continue
		}
		s.d.Logger.WarnContext(ctx, "media：刪除沒記錄的副本段失敗，已記入待刪清單由回收 job 重試", "key", key, "error", err)
	}
}

func (s *Service) failCopy(ctx context.Context, r store.MediaAssetCopyRequest, reason string) error {
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := hold.SetLockTimeout(ctx, s.d.DB.Conn(ctx)); err != nil {
			return err
		}
		q := s.q(ctx)
		failedAt := s.d.Now()
		if _, err := q.FailCopyRequest(ctx, store.FailCopyRequestParams{AssetID: r.AssetID, Now: &failedAt}); err != nil {
			return fmt.Errorf("media：標記複製失敗：%w", err)
		}
		if err := hold.ReleaseSource(ctx, q, model.CopySource, r.AssetID); err != nil {
			return fmt.Errorf("media：釋放待複製保全：%w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.d.Logger.WarnContext(ctx, "media：放棄複製請求", "asset_id", r.AssetID, "source_asset_id", r.SourceAssetID, "reason", reason)
	return nil
}

type ProcessCopiesArgs struct{}

func (ProcessCopiesArgs) Kind() string { return "media.copy_to_secure" }

func (ProcessCopiesArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(CopyInterval)
	o.MaxAttempts = MaxAttempts
	return o
}

type copyWorker struct {
	river.WorkerDefaults[ProcessCopiesArgs]
	svc *Service
}

func (w *copyWorker) Work(ctx context.Context, _ *river.Job[ProcessCopiesArgs]) error {
	_, err := w.svc.ProcessCopies(ctx)
	return err
}

func (w *copyWorker) Timeout(*river.Job[ProcessCopiesArgs]) time.Duration { return 5 * time.Minute }

func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(CopyInterval),
			func() (river.JobArgs, *river.InsertOpts) { return ProcessCopiesArgs{}, nil },
			&river.PeriodicJobOpts{ID: "media.copy_to_secure", RunOnStart: true},
		),
	}
}
