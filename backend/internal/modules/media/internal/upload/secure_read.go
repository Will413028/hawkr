package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// rewrapBatch 是 RewrapDEKs 每批取的資產數。
const rewrapBatch = 100

// openAsset 依 secure_chunks 按 idx 排序依序開各段的物件並串流解密；一個 AEAD 段的 tag 驗過才交出那一段的明文。
// 列數不等於資產列的段總數、或序號不連續，視為損毀（secure.ErrCorrupt）。第一段在這裡就打開，物件不存在在開啟時就回錯誤。
func (s *Service) openAsset(ctx context.Context, a store.MediaAsset) (io.ReadCloser, error) {
	if a.KekID == nil || len(a.WrappedDek) == 0 {
		return nil, fmt.Errorf("media：機敏資產 %s 沒有 wrapped DEK", a.ID)
	}
	if a.ChunkCount == nil || *a.ChunkCount <= 0 {
		return nil, fmt.Errorf("media：機敏資產 %s 沒有段總數：%w", a.ID, secure.ErrCorrupt)
	}
	dek, err := s.d.Keyring.Unwrap(a.ID, *a.KekID, a.WrappedDek)
	if err != nil {
		return nil, fmt.Errorf("media：解開資產 %s 的 DEK：%w", a.ID, err)
	}
	rows, err := s.q(ctx).ListSecureChunks(ctx, a.ID)
	if err != nil {
		return nil, fmt.Errorf("media：列出資產 %s 的段：%w", a.ID, err)
	}
	if len(rows) != int(*a.ChunkCount) {
		return nil, fmt.Errorf("media：資產 %s 有 %d 段、應有 %d 段：%w", a.ID, len(rows), *a.ChunkCount, secure.ErrCorrupt)
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		if int(r.Idx) != i {
			return nil, fmt.Errorf("media：資產 %s 的段序號不連續：%w", a.ID, secure.ErrCorrupt)
		}
		keys[i] = r.ObjectKey
	}
	r := &chunkedReader{ctx: ctx, store: s.d.Store, dek: dek, asset: a.ID, keys: keys}
	if err := r.openNext(); err != nil {
		return nil, err
	}
	return r, nil
}

// chunkedReader 把資產的各個上傳段依序接成一條明文串流；同一時間只開一個物件。
type chunkedReader struct {
	ctx   context.Context
	store storage.Store
	dek   []byte
	asset uuid.UUID
	keys  []string
	next  int
	cur   *secure.Reader
	rc    io.ReadCloser
	err   error
}

func (r *chunkedReader) openNext() error {
	index := r.next
	rc, err := r.store.Get(r.ctx, r.keys[index])
	if err != nil {
		return fmt.Errorf("media：讀取資產 %s 的第 %d 段：%w", r.asset, index, err)
	}
	cur, err := secure.NewChunkReader(rc, r.dek, secure.Chunk{Asset: r.asset, Index: u32(index), Count: u32(len(r.keys))})
	if err != nil {
		_ = rc.Close()
		return fmt.Errorf("media：資產 %s 第 %d 段的密文 header：%w", r.asset, index, err)
	}
	r.cur, r.rc = cur, rc
	r.next++
	return nil
}

// Read 實作 io.Reader：一段讀完（含驗過最後一個 AEAD 段）才接下一段；任何錯誤之後不再交出位元組。
func (r *chunkedReader) Read(p []byte) (int, error) {
	for {
		if r.err != nil {
			return 0, r.err
		}
		if r.cur == nil {
			if r.next >= len(r.keys) {
				return 0, io.EOF
			}
			if err := r.openNext(); err != nil {
				r.err = err
				return 0, err
			}
		}
		n, err := r.cur.Read(p)
		if errors.Is(err, io.EOF) {
			_ = r.rc.Close()
			r.cur, r.rc = nil, nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		if err != nil {
			r.err = err
			return n, err
		}
		return n, nil
	}
}

// Close 關掉目前開著的物件。
func (r *chunkedReader) Close() error {
	if r.rc == nil {
		return nil
	}
	err := r.rc.Close()
	r.cur, r.rc = nil, nil
	r.err = errors.New("media：機敏讀取已關閉")
	return err
}

// ReadSecure 讀機敏資產的明文：只讀 SECURE 且 ACTIVE 的資產，由擁有者先授權。公開資產回 model.ErrNotSecure，其餘（不存在、STAGED、
// 已刪除）NotFound。不得在 transaction 內呼叫。
func (s *Service) ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return nil, err
	}
	if s.d.Keyring == nil {
		return nil, model.ErrNoKeyring
	}
	a, err := s.q(ctx).GetAsset(ctx, assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("找不到這個資產")
	}
	if err != nil {
		return nil, fmt.Errorf("media：讀取資產：%w", err)
	}
	if model.Visibility(a.Visibility) != model.VisibilitySecure {
		return nil, model.ErrNotSecure
	}
	if model.AssetState(a.State) != model.AssetActive {
		return nil, apperr.NotFound("找不到這個資產")
	}
	return s.openAsset(ctx, a)
}

// ReadStagedSecure 是確認前的預覽：憑 token 讀上傳者自己、READY 且未過期的機敏暫存；資產 ID 不外露。
// 非上傳者與不存在同一個 NotFound；已確認、被拒、處理中或已過期回 FailedPrecondition；public 的暫存回 model.ErrNotSecure。
func (s *Service) ReadStagedSecure(ctx context.Context, uploader model.Uploader, token string) (io.ReadCloser, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return nil, err
	}
	if s.d.Keyring == nil {
		return nil, model.ErrNoKeyring
	}
	row, err := s.own(ctx, uploader, token)
	if err != nil {
		return nil, err
	}
	if model.Visibility(row.Visibility) != model.VisibilitySecure {
		return nil, model.ErrNotSecure
	}
	if model.UploadState(row.State) != model.StateReady || !row.ExpiresAt.After(s.d.Now()) || row.AssetID == nil {
		return nil, apperr.FailedPrecondition("這筆上傳現在不能預覽")
	}
	a, err := s.q(ctx).GetAsset(ctx, *row.AssetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("找不到這個資產")
	}
	if err != nil {
		return nil, fmt.Errorf("media：讀取資產：%w", err)
	}
	return s.openAsset(ctx, a)
}

// RewrapDEKs 把 kek_id 不是 primary 的機敏資產（DELETED 除外）與還在 UPLOADING 的機敏暫存列的 DEK 解包再以 primary 包裹，回傳這次實際更新的筆數。
// 以 keyset 分批；UPDATE 帶 `kek_id = <舊>`，所以可以重跑、中斷後接著跑，並行的兩次執行只有一方生效。不碰物件儲存。
// 單列失敗（例如未知的 kek_id）不中止其他列，錯誤合併後回傳。不得在 transaction 內呼叫。
func (s *Service) RewrapDEKs(ctx context.Context) (int, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return 0, err
	}
	if s.d.Keyring == nil {
		return 0, model.ErrNoKeyring
	}
	primary := s.d.Keyring.PrimaryID()
	var errs []error
	total := 0
	after := uuid.UUID{}
	for {
		rows, err := s.q(ctx).ListAssetsToRewrap(ctx, store.ListAssetsToRewrapParams{PrimaryKekID: &primary, AfterID: after, MaxRows: rewrapBatch})
		if err != nil {
			return total, errors.Join(append(errs, fmt.Errorf("media：列出待重新包裹的資產：%w", err))...)
		}
		for _, r := range rows {
			n, err := s.rewrapOne(ctx, r, primary)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			total += n
		}
		if len(rows) < rewrapBatch {
			break
		}
		after = rows[len(rows)-1].ID
	}
	after = uuid.UUID{}
	for {
		rows, err := s.q(ctx).ListUploadsToRewrap(ctx, store.ListUploadsToRewrapParams{PrimaryKekID: &primary, AfterID: after, MaxRows: rewrapBatch})
		if err != nil {
			return total, errors.Join(append(errs, fmt.Errorf("media：列出待重新包裹的上傳中暫存：%w", err))...)
		}
		for _, r := range rows {
			n, err := s.rewrapUpload(ctx, r, primary)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			total += n
		}
		if len(rows) < rewrapBatch {
			break
		}
		after = rows[len(rows)-1].ID
	}
	return total, errors.Join(errs...)
}

// rewrapUpload 重新包裹還在 UPLOADING 的暫存列的 DEK（資產列建立前 DEK 只在暫存列）；AAD 的資產 ID 是預配的資產 ID。
func (s *Service) rewrapUpload(ctx context.Context, r store.ListUploadsToRewrapRow, primary string) (int, error) {
	if r.KekID == nil || r.PlannedAssetID == nil {
		return 0, fmt.Errorf("media：機敏暫存上傳 %s 沒有 kek_id 或預配的資產 ID", r.ID)
	}
	dek, err := s.d.Keyring.Unwrap(*r.PlannedAssetID, *r.KekID, r.WrappedDek)
	if err != nil {
		return 0, fmt.Errorf("media：解開暫存上傳 %s 的 DEK：%w", r.ID, err)
	}
	_, wrapped, err := s.d.Keyring.WrapWith(primary, *r.PlannedAssetID, dek)
	if err != nil {
		return 0, err
	}
	n, err := s.q(ctx).RewrapUpload(ctx, store.RewrapUploadParams{ID: r.ID, OldKekID: r.KekID, NewKekID: &primary, NewWrappedDek: wrapped, Now: s.d.Now()})
	if err != nil {
		return 0, fmt.Errorf("media：更新暫存上傳 %s 的 wrapped DEK：%w", r.ID, err)
	}
	return int(n), nil
}

func (s *Service) rewrapOne(ctx context.Context, r store.ListAssetsToRewrapRow, primary string) (int, error) {
	if r.KekID == nil {
		return 0, fmt.Errorf("media：機敏資產 %s 沒有 kek_id", r.ID)
	}
	dek, err := s.d.Keyring.Unwrap(r.ID, *r.KekID, r.WrappedDek)
	if err != nil {
		return 0, fmt.Errorf("media：解開資產 %s 的 DEK：%w", r.ID, err)
	}
	_, wrapped, err := s.d.Keyring.WrapWith(primary, r.ID, dek)
	if err != nil {
		return 0, err
	}
	n, err := s.q(ctx).RewrapAsset(ctx, store.RewrapAssetParams{
		ID: r.ID, OldKekID: r.KekID, NewKekID: &primary, NewWrappedDek: wrapped, Now: s.d.Now(),
	})
	if err != nil {
		return 0, fmt.Errorf("media：更新資產 %s 的 wrapped DEK：%w", r.ID, err)
	}
	return int(n), nil
}
