package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// CopyPublic 把一份已確認（ACTIVE）的公開資產複製成新的資產：物件位元組原樣寫到新的 public/ key，建資產列（STAGED）與 READY 的暫存列並回 token，
// 之後由呼叫方在自己的 transaction 內 Confirm，與 PutPublic 的結果同形。來源資產不存在、不是 ACTIVE 或是機敏資產回 NotFound；來源只被讀取，
// 不改它的狀態。不得在 transaction 內呼叫。
func (s *Service) CopyPublic(ctx context.Context, uploader model.Uploader, sourceID uuid.UUID) (PublicPutResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return PublicPutResult{}, err
	}
	if uploader.ID <= 0 {
		return PublicPutResult{}, apperr.InvalidInputOn([]string{"uploader"}, "%s", text.MediaUploaderInvalid(loc(ctx)))
	}
	src, err := s.q(ctx).GetAsset(ctx, sourceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return PublicPutResult{}, fmt.Errorf("media：讀取來源資產：%w", err)
	}
	if err != nil || model.AssetState(src.State) != model.AssetActive || model.Visibility(src.Visibility) != model.VisibilityPublic {
		return PublicPutResult{}, apperr.NotFound("%s", text.MediaAssetNotFound(loc(ctx)))
	}
	rc, err := s.d.Store.Get(ctx, src.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return PublicPutResult{}, apperr.NotFound("%s", text.MediaAssetNotFound(loc(ctx)))
	}
	if err != nil {
		return PublicPutResult{}, fmt.Errorf("media：讀取來源物件：%w", err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(&capReader{r: rc, max: model.MaxPublicUploadSize})
	if err != nil {
		return PublicPutResult{}, fmt.Errorf("media：複製來源物件 %s：%w", sourceID, err)
	}
	if len(data) == 0 {
		return PublicPutResult{}, fmt.Errorf("media：來源資產 %s 的物件是空的", sourceID)
	}
	return s.stagePublic(ctx, uploader, data, src.ContentType)
}
