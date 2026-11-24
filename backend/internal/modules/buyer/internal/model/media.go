package model

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// Media 是買家對 media 的依賴：頭像在寫入的 transaction 內 Confirm，被換掉或終態的資產 MarkDeleted。兩個方法的 ctx 都必須帶 transaction。
type Media interface {
	Confirm(ctx context.Context, uploader media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error)
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
}
