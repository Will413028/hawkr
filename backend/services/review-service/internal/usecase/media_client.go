package usecase

import "context"

// MediaClient 處理評論圖片轉正（與 social service 相同介面）。
type MediaClient interface {
	// FinalizeAndLink 將單一 staged token 轉正並建立 asset_reference（防 GC）。
	FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error)
}

// AssetInfo 代表 media-service 轉正後的資產資訊。
type AssetInfo struct {
	AssetId   string
	ObjectKey string
}
