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
	// ReleaseChatReferences releases the media-service asset_reference rows for the
	// given chat message IDs (ref_type CHAT_MESSAGE) so the OSS blobs can be GC'd.
	// Used by account.deleted erasure. Fail-closed: a non-nil error must propagate.
	ReleaseChatReferences(ctx context.Context, refIDs []string) error
}
