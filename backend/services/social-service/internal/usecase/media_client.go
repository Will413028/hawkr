package usecase

import (
	"context"

	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
)

// AssetInfo 為共用 media.AssetInfo 的 type alias，方便 usecase 層直接使用。
type AssetInfo = pkgmedia.AssetInfo

// MediaClient defines the interface for media-service operations.
type MediaClient interface {
	FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error)
	// ReleaseReferences drops asset_reference rows for the given (refType, refID)
	// pairs, except those whose asset_id is in keepAssetIDs (#1489).
	//
	// keepAssetIDs 為空 = 釋放全部（實體刪除）。非空 = 換圖收斂：media-service 在單一交易內
	// 刪舊留新，故必須排在「Link 新資產 → 寫業務列」**之後**。
	ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error
}
