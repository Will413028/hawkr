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
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// rewrapBatch 是 RewrapDEKs 每批取的資產數。
const rewrapBatch = 100

// plainReader 把解密的 Reader 與底層物件的 Closer 合成 io.ReadCloser。
type plainReader struct {
	io.Reader
	io.Closer
}

// openAsset 取資產的物件並串流解密；一段的 tag 驗過才交出那一段的明文。
func (s *Service) openAsset(ctx context.Context, a store.MediaAsset) (io.ReadCloser, error) {
	if a.KekID == nil || len(a.WrappedDek) == 0 {
		return nil, fmt.Errorf("media：機敏資產 %s 沒有 wrapped DEK", a.ID)
	}
	dek, err := s.d.Keyring.Unwrap(a.ID, *a.KekID, a.WrappedDek)
	if err != nil {
		return nil, fmt.Errorf("media：解開資產 %s 的 DEK：%w", a.ID, err)
	}
	rc, err := s.d.Store.Get(ctx, a.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("media：讀取資產 %s 的物件：%w", a.ID, err)
	}
	r, err := secure.NewReader(rc, dek)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("media：資產 %s 的密文 header：%w", a.ID, err)
	}
	return plainReader{Reader: r, Closer: rc}, nil
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
	if err != nil {
		return nil, fmt.Errorf("media：讀取資產：%w", err)
	}
	return s.openAsset(ctx, a)
}

// RewrapDEKs 把 kek_id 不是 primary 的機敏資產（DELETED 除外）的 DEK 解包再以 primary 包裹，回傳這次實際更新的筆數。
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
	return total, errors.Join(errs...)
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
