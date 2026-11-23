package model

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// Media 是 im 對 media 的依賴：圖片訊息在寫入的 transaction 內 Confirm，抹除的訊息的圖 MarkDeleted。兩個方法的 ctx 都必須帶 transaction。
type Media interface {
	Confirm(ctx context.Context, uploader media.Uploader, token string) (media.ConfirmedAsset, error)
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
}
