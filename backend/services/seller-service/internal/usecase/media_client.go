package usecase

import "context"

// MediaClient defines the interface for media-service operations.
type MediaClient interface {
	FinalizeImages(ctx context.Context, stagedTokens []string, refType string) ([]AssetInfo, error)
	LinkImages(ctx context.Context, refType string, refID string, assetIDs []string) error
	// ReleaseReferences drops asset_reference rows for the given (refType, refID)
	// pairs, except those whose asset_id is in keepAssetIDs, so the released
	// assets become GC-eligible (#1489).
	//
	// keepAssetIDs 為空 = 釋放全部（實體 / 帳號刪除）。非空 = 換圖收斂：
	// media-service 在單一交易內刪舊留新，故必須排在「Link 新資產 → 寫業務列」**之後**，
	// 才不會出現「業務列指著、卻沒有引用」而被 GC 實體刪除的資產。
	ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error
}

// AssetInfo represents a finalized asset returned by the media service.
type AssetInfo struct {
	AssetId     string
	StagedToken string
	ObjectKey   string
	IsSecure    bool
}
