package content

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// Admin 實作 ContentAdminService。授權（只有管理員）由 rpc 的 interceptor 依 service 名稱檢查。
type Admin struct {
	m      *content.Module
	images rpc.Images
}

var _ contentv1alpha1connect.ContentAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m *content.Module, images rpc.Images) *Admin {
	return &Admin{m: m, images: images}
}

// adminID 是呼叫的管理員，告訴 media 圖片是誰上傳的。interceptor 已確認 Admin 的 service 一定是管理員。
func adminID(ctx context.Context) (int64, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return 0, errors.New("httpapi/content：後台的請求沒有管理員，interceptor 沒有掛上")
	}
	return p.ID, nil
}

// ==================== banner ====================

func (h *Admin) ListBanners(ctx context.Context, req *connect.Request[adminpb.ListBannersRequest]) (*connect.Response[adminpb.ListBannersResponse], error) {
	scope := contentv1alpha1connect.ContentAdminServiceListBannersProcedure
	after, err := decodeBannerCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListBanners(ctx, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListBannersResponse{
		Banners:       make([]*adminpb.Banner, len(page.Banners)),
		NextPageToken: encodeBannerCursor(scope, page.Next),
	}
	for i, b := range page.Banners {
		out.Banners[i] = h.toAdminBanner(b)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) GetBanner(ctx context.Context, req *connect.Request[adminpb.GetBannerRequest]) (*connect.Response[adminpb.GetBannerResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	b, err := h.m.Banner(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetBannerResponse{Banner: h.toAdminBanner(b)}), nil
}

func (h *Admin) CreateBanner(ctx context.Context, req *connect.Request[adminpb.CreateBannerRequest]) (*connect.Response[adminpb.CreateBannerResponse], error) {
	admin, err := adminID(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetBanner()
	b, err := h.m.CreateBanner(ctx, admin, bannerInput(msg))
	if err != nil {
		return nil, rpc.WithFieldPaths(err, bannerPaths(msg))
	}
	return connect.NewResponse(&adminpb.CreateBannerResponse{Banner: h.toAdminBanner(b)}), nil
}

// UpdateBanner 把 update_mask 內的欄位套在目前的 banner 上，再整筆交給模組；版本由模組比對。
func (h *Admin) UpdateBanner(ctx context.Context, req *connect.Request[adminpb.UpdateBannerRequest]) (*connect.Response[adminpb.UpdateBannerResponse], error) {
	admin, err := adminID(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetBanner()
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersion(msg.GetEtag())
	if err != nil {
		return nil, err
	}
	cur, err := h.m.Banner(ctx, id)
	if err != nil {
		return nil, err
	}
	merged := h.toAdminBanner(cur)
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	b, err := h.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: admin, ID: id, Version: version, Banner: bannerInput(merged)})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, bannerPaths(merged))
	}
	return connect.NewResponse(&adminpb.UpdateBannerResponse{Banner: h.toAdminBanner(b)}), nil
}

func (h *Admin) DeleteBanner(ctx context.Context, req *connect.Request[adminpb.DeleteBannerRequest]) (*connect.Response[adminpb.DeleteBannerResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteBanner(ctx, id, version); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.DeleteBannerResponse{}), nil
}

func (h *Admin) toAdminBanner(b content.Banner) *adminpb.Banner {
	return &adminpb.Banner{
		Id:           b.ID.String(),
		Title:        b.Title,
		ButtonText:   b.ButtonText,
		PreviewImage: h.images.Image(b.PreviewImageKey),
		PopupImage:   h.images.Image(b.PopupImageKey),
		CanvasJson:   b.CanvasJSON,
		Action:       toActionPB(b.Action),
		DisplayOrder: b.DisplayOrder,
		Active:       b.IsActive,
		StartTime:    toTimestampPB(b.StartsAt),
		EndTime:      toTimestampPB(b.EndsAt),
		CreateTime:   timestamppb.New(b.CreatedAt),
		UpdateTime:   timestamppb.New(b.UpdatedAt),
		Etag:         rpc.Etag(b.Version),
	}
}

// bannerInput 組成模組的輸入。圖片有 staged token 就換新，沒有就沿用現在的圖（由模組決定）。
func bannerInput(msg *adminpb.Banner) content.BannerInput {
	return content.BannerInput{
		Title:             msg.GetTitle(),
		ButtonText:        msg.GetButtonText(),
		PreviewImageToken: msg.GetPreviewImageStagedToken(),
		PopupImageToken:   msg.GetPopupImageStagedToken(),
		CanvasJSON:        msg.GetCanvasJson(),
		Action:            fromActionPB(msg.GetAction()),
		DisplayOrder:      msg.GetDisplayOrder(),
		IsActive:          msg.GetActive(),
		StartsAt:          fromTimestampPB(msg.GetStartTime()),
		EndsAt:            fromTimestampPB(msg.GetEndTime()),
	}
}

// ==================== 版本門檻 ====================

func (h *Admin) ListAppVersionGates(ctx context.Context, _ *connect.Request[adminpb.ListAppVersionGatesRequest]) (*connect.Response[adminpb.ListAppVersionGatesResponse], error) {
	gates, err := h.m.ListAppVersionGates(ctx)
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListAppVersionGatesResponse{AppVersionGates: make([]*adminpb.AppVersionGate, len(gates))}
	for i, g := range gates {
		out.AppVersionGates[i] = toAdminGate(g)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) CreateAppVersionGate(ctx context.Context, req *connect.Request[adminpb.CreateAppVersionGateRequest]) (*connect.Response[adminpb.CreateAppVersionGateResponse], error) {
	g, err := h.m.CreateAppVersionGate(ctx, gateInput(req.Msg.GetAppVersionGate()))
	if err != nil {
		return nil, rpc.WithFieldPaths(err, gatePaths)
	}
	return connect.NewResponse(&adminpb.CreateAppVersionGateResponse{AppVersionGate: toAdminGate(g)}), nil
}

func (h *Admin) UpdateAppVersionGate(ctx context.Context, req *connect.Request[adminpb.UpdateAppVersionGateRequest]) (*connect.Response[adminpb.UpdateAppVersionGateResponse], error) {
	msg := req.Msg.GetAppVersionGate()
	version, err := rpc.RequiredVersion(msg.GetEtag())
	if err != nil {
		return nil, err
	}
	cur, err := h.m.AppVersionGate(ctx, rpc.EnumFromProto[content.AppType](msg.GetAppType()))
	if err != nil {
		return nil, rpc.WithFieldPaths(err, gatePaths)
	}
	merged := toAdminGate(cur)
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	g, err := h.m.UpdateAppVersionGate(ctx, gateInput(merged), version)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, gatePaths)
	}
	return connect.NewResponse(&adminpb.UpdateAppVersionGateResponse{AppVersionGate: toAdminGate(g)}), nil
}

func toAdminGate(g content.AppVersionGate) *adminpb.AppVersionGate {
	return &adminpb.AppVersionGate{
		AppType:        rpc.EnumToProto[contentpb.AppType](g.AppType),
		MinimumVersion: g.MinimumVersion,
		StoreUrl:       g.StoreURL,
		NoticeTitle:    g.NoticeTitle,
		NoticeBody:     g.NoticeBody,
		UpdateTime:     timestamppb.New(g.UpdatedAt),
		Etag:           rpc.Etag(g.Version),
	}
}

func gateInput(msg *adminpb.AppVersionGate) content.AppVersionGate {
	return content.AppVersionGate{
		AppType:        rpc.EnumFromProto[content.AppType](msg.GetAppType()),
		MinimumVersion: msg.GetMinimumVersion(),
		StoreURL:       msg.GetStoreUrl(),
		NoticeTitle:    msg.GetNoticeTitle(),
		NoticeBody:     msg.GetNoticeBody(),
	}
}
