package content

import (
	"encoding/binary"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
)

// 模組回報的欄位對應到請求的路徑（ADR 2026-09-25-api-error-model D4）。模組的欄位名見各 Input 型別。

// bannerPaths 是 CreateBannerRequest、UpdateBannerRequest 的對應。點擊行為的值依行為的種類是不同的
// oneof 欄位，所以依模組收到的 b 決定。
func bannerPaths(b *adminpb.Banner) rpc.FieldPaths {
	p := rpc.FieldPaths{
		"Title":             "banner.title",
		"ButtonText":        "banner.button_text",
		"EndsAt":            "banner.end_time",
		"PreviewImageToken": "banner.preview_image_staged_token",
		"PopupImageToken":   "banner.popup_image_staged_token",
		"Action.Type":       "banner.action",
		"Action.Target":     "banner.action",
	}
	switch b.GetAction().GetKind().(type) {
	case *contentpb.BannerAction_Url:
		p["Action.Target"] = "banner.action.url"
	case *contentpb.BannerAction_ProductId:
		p["Action.Target"] = "banner.action.product_id"
	case *contentpb.BannerAction_PostId:
		p["Action.Target"] = "banner.action.post_id"
	}
	return p
}

// gatePaths 是 CreateAppVersionGateRequest、UpdateAppVersionGateRequest 的對應。
var gatePaths = rpc.FieldPaths{
	"AppType":        "app_version_gate.app_type",
	"MinimumVersion": "app_version_gate.minimum_version",
	"StoreURL":       "app_version_gate.store_url",
}

// checkVersionPaths 是 CheckAppVersionRequest 的對應。
var checkVersionPaths = rpc.FieldPaths{
	"AppType":       "app_type",
	"ClientVersion": "client_version",
}

func toActionPB(a content.BannerAction) *contentpb.BannerAction {
	switch a.Type {
	case content.BannerActionPopup:
		return &contentpb.BannerAction{Kind: &contentpb.BannerAction_Popup_{Popup: &contentpb.BannerAction_Popup{}}}
	case content.BannerActionURL:
		return &contentpb.BannerAction{Kind: &contentpb.BannerAction_Url{Url: a.Target}}
	case content.BannerActionProduct:
		return &contentpb.BannerAction{Kind: &contentpb.BannerAction_ProductId{ProductId: a.Target}}
	case content.BannerActionPost:
		return &contentpb.BannerAction{Kind: &contentpb.BannerAction_PostId{PostId: a.Target}}
	}
	return &contentpb.BannerAction{}
}

// fromActionPB 把請求的點擊行為轉成模組的型別；沒有指定行為時回零值，由模組回報必填。
func fromActionPB(a *contentpb.BannerAction) content.BannerAction {
	switch k := a.GetKind().(type) {
	case *contentpb.BannerAction_Popup_:
		return content.BannerAction{Type: content.BannerActionPopup}
	case *contentpb.BannerAction_Url:
		return content.BannerAction{Type: content.BannerActionURL, Target: k.Url}
	case *contentpb.BannerAction_ProductId:
		return content.BannerAction{Type: content.BannerActionProduct, Target: k.ProductId}
	case *contentpb.BannerAction_PostId:
		return content.BannerAction{Type: content.BannerActionPost, Target: k.PostId}
	}
	return content.BannerAction{}
}

func toTimestampPB(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func fromTimestampPB(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

// banner 清單的排序鍵：display_order（4 bytes）、created_at 的 Unix 微秒（8 bytes）、id（16 bytes）。
// created_at 在資料庫是微秒精度，所以編碼後比較不會失真。
func encodeBannerCursor(scope string, c *content.BannerCursor) string {
	if c == nil {
		return ""
	}
	b := binary.BigEndian.AppendUint32(nil, uint32(c.DisplayOrder)) //nolint:gosec // cursor encodes the signed sort key as unchanged bits
	b = binary.BigEndian.AppendUint64(b, uint64(c.CreatedAt.UnixMicro()))
	b = append(b, c.ID[:]...)
	return rpc.EncodePageToken(scope, b)
}

func decodeBannerCursor(scope, token string) (*content.BannerCursor, error) {
	b, err := rpc.DecodePageToken(scope, token)
	if err != nil || b == nil {
		return nil, err
	}
	if len(b) != 4+8+16 {
		return nil, badPageToken()
	}
	c := &content.BannerCursor{
		DisplayOrder: int32(binary.BigEndian.Uint32(b)),                           //nolint:gosec // cursor restores the original signed bits
		CreatedAt:    time.UnixMicro(int64(binary.BigEndian.Uint64(b[4:]))).UTC(), //nolint:gosec // cursor restores the original signed bits
		ID:           uuid.UUID(b[12:]),
	}
	return c, nil
}

func badPageToken() error {
	return rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 無法使用，請從第一頁重新讀取")
}
