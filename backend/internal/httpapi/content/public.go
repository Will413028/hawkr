// Package content 是 content 模組的 Connect handler：前台的 hawkr.content.v1alpha1 與後台的
// hawkr.admin.content.v1alpha1。這裡只做欄位的轉換與分頁 token、field mask 這類傳輸層的事，
// 業務規則在 internal/modules/content。
package content

import (
	"context"

	"connectrpc.com/connect"

	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1/contentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
)

// Public 實作 ContentPublicService。
type Public struct {
	m      *content.Module
	images rpc.Images
}

var _ contentv1alpha1connect.ContentPublicServiceHandler = (*Public)(nil)

// NewPublic 建立前台的 handler。
func NewPublic(m *content.Module, images rpc.Images) *Public {
	return &Public{m: m, images: images}
}

func (h *Public) ListLiveBanners(ctx context.Context, req *connect.Request[contentpb.ListLiveBannersRequest]) (*connect.Response[contentpb.ListLiveBannersResponse], error) {
	scope := contentv1alpha1connect.ContentPublicServiceListLiveBannersProcedure
	after, err := decodeBannerCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListLiveBanners(ctx, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	out := &contentpb.ListLiveBannersResponse{
		Banners:       make([]*contentpb.Banner, len(page.Banners)),
		NextPageToken: encodeBannerCursor(scope, page.Next),
	}
	for i, b := range page.Banners {
		out.Banners[i] = &contentpb.Banner{
			Id:           b.ID.String(),
			Title:        b.Title,
			ButtonText:   b.ButtonText,
			PreviewImage: h.images.Image(b.PreviewImageKey),
			PopupImage:   h.images.Image(b.PopupImageKey),
			Action:       toActionPB(b.Action),
		}
	}
	return connect.NewResponse(out), nil
}

func (h *Public) CheckAppVersion(ctx context.Context, req *connect.Request[contentpb.CheckAppVersionRequest]) (*connect.Response[contentpb.CheckAppVersionResponse], error) {
	v, err := h.m.CheckAppVersion(ctx, rpc.EnumFromProto[content.AppType](req.Msg.GetAppType()), req.Msg.GetClientVersion())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, checkVersionPaths)
	}
	return connect.NewResponse(&contentpb.CheckAppVersionResponse{
		UpdateRequired: v.UpdateRequired,
		MinimumVersion: v.MinimumVersion,
		StoreUrl:       v.StoreURL,
		NoticeTitle:    v.NoticeTitle,
		NoticeBody:     v.NoticeBody,
	}), nil
}
