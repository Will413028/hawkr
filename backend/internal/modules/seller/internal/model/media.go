package model

import (
	"context"
	"errors"
	"io"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Media 是 seller 對 media 的依賴：寫入的 transaction 內 Confirm 新資產，被換掉或終態的資產 MarkDeleted（治理佐證從不 MarkDeleted）；
// 機敏檔案（KYC 證件、治理佐證）用 ReadSecure 讀。Confirm 與 MarkDeleted 的 ctx 必須帶 transaction。
type Media interface {
	Confirm(ctx context.Context, uploader media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error)
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
	ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, string, error)
}

// ReadSecure 讀一份機敏檔（KYC 證件、治理佐證）：呼叫方在 transaction 內寫好讀取紀錄並 commit 之後，在 transaction 外呼叫這裡。
// 回傳的 Body 是 media 的明文串流（呼叫方負責關閉），ContentType 是上傳時 media 依內容記下的型別（media 只收 PDF 與圖片）。media 說不是機敏資產
// （media.ErrNotSecure，資料或設定錯）原樣回錯誤；NotFound 是我們有這份檔而 media 說沒有，資料不一致，要人處理（ErrFileMissing）。
func ReadSecure(ctx context.Context, m Media, assetID uuid.UUID) (FileContent, error) {
	body, contentType, err := m.ReadSecure(ctx, assetID)
	if err != nil {
		if errors.Is(err, media.ErrNotSecure) || apperr.KindOf(err) != apperr.KindNotFound {
			return FileContent{}, err
		}
		return FileContent{}, ErrFileMissing
	}
	return FileContent{Body: body, ContentType: contentType}, nil
}

// ConfirmItem 是請求裡一個要確認的 staged token，Field 是它在輸入的欄位名（清單元素帶索引）。
type ConfirmItem = media.ConfirmItem

// ConfirmAll 在寫入的 transaction 內依序確認 items 的 staged token，一次回報所有不合格的（語意見 media.ConfirmFields）。
func ConfirmAll(ctx context.Context, m Media, uploader media.Uploader, items []ConfirmItem, want media.Visibility) ([]media.ConfirmedAsset, error) {
	return media.ConfirmFields(ctx, m, uploader, items, want)
}
