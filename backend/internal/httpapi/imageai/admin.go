package imageai

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/imageai/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/imageai/v1alpha1/imageaiv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
)

// AdminModule 是後台維護的介面（*imageai.Module）。
type AdminModule interface {
	ListStylesForAdmin(ctx context.Context, category *imageai.StyleCategory) ([]imageai.AdminStyle, error)
	ReadStyleForAdmin(ctx context.Context, id uuid.UUID) (imageai.AdminStyle, error)
	CreateStyle(ctx context.Context, in imageai.CreateStyleInput) (imageai.AdminStyle, error)
	UpdateStyle(ctx context.Context, in imageai.UpdateStyleInput) (imageai.AdminStyle, error)
	SetDefaultStyle(ctx context.Context, actor imageai.Actor, id uuid.UUID, version int64) (imageai.AdminStyle, error)
	DeleteStyle(ctx context.Context, actor imageai.Actor, id uuid.UUID, version int64) error
	GetQuota(ctx context.Context, storeID int64) (imageai.AdminQuota, error)
	UpdateQuota(ctx context.Context, in imageai.UpdateQuotaInput) (imageai.AdminQuota, error)
	DeleteQuota(ctx context.Context, actor imageai.Actor, storeID int64, version int64) error
}

// Admin 是 ImageAiAdminService 的 handler。授權（權限與稽核宣告）由 adminauth 的 interceptor 依 procedure 檢查；寫入的稽核紀錄由模組在同一個 transaction 內寫。
type Admin struct {
	m      AdminModule
	images rpc.Images
}

var _ imageaiv1alpha1connect.ImageAiAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler；images 組風格縮圖。
func NewAdmin(m AdminModule, images rpc.Images) *Admin { return &Admin{m: m, images: images} }

var (
	stylePaths = rpc.FieldPaths{
		"Category": "ai_style.category", "Name": "ai_style.name", "Label": "ai_style.label", "Prompt": "ai_style.prompt",
		"ThumbnailToken": "ai_style.thumbnail_staged_token", "Enabled": "ai_style.enabled",
		"Version": "ai_style.etag",
	}
	listStylePaths = rpc.FieldPaths{"Category": "category"}
	quotaPaths     = rpc.FieldPaths{"WeeklyLimit": "ai_quota.weekly_limit", "Memo": "ai_quota.memo"}
)

func (h *Admin) ListAiStylesForAdmin(ctx context.Context, req *connect.Request[adminpb.ListAiStylesForAdminRequest]) (*connect.Response[adminpb.ListAiStylesForAdminResponse], error) {
	var category *imageai.StyleCategory
	if req.Msg.Category != nil {
		c := rpc.EnumFromProto[imageai.StyleCategory](req.Msg.GetCategory())
		category = &c
	}
	styles, err := h.m.ListStylesForAdmin(ctx, category)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listStylePaths)
	}
	out := &adminpb.ListAiStylesForAdminResponse{AiStyles: make([]*adminpb.AiStyle, len(styles))}
	for i, s := range styles {
		if out.AiStyles[i], err = h.toStyle(s); err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) CreateAiStyle(ctx context.Context, req *connect.Request[adminpb.CreateAiStyleRequest]) (*connect.Response[adminpb.CreateAiStyleResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetAiStyle()
	s, err := h.m.CreateStyle(ctx, imageai.CreateStyleInput{
		Actor: imageai.Actor{AdminID: admin, Action: req.Spec().Procedure},
		Name:  msg.GetName(), Label: msg.GetLabel(), Prompt: msg.GetPrompt(),
		Category:       rpc.EnumFromProto[imageai.StyleCategory](msg.GetCategory()),
		ThumbnailToken: msg.GetThumbnailStagedToken(), DisplayOrder: msg.GetDisplayOrder(),
		Enabled: msg.GetEnabled(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, stylePaths)
	}
	out, err := h.toStyle(s)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.CreateAiStyleResponse{AiStyle: out}), nil
}

// UpdateAiStyle 把 update_mask 內的欄位套在目前的風格上，再整筆交給模組；name、category 不能改（IMMUTABLE，與目前不同回 invalid_argument）；
// default_style 是 OUTPUT_ONLY，在 mask 裡也略過；版本由模組比對。
func (h *Admin) UpdateAiStyle(ctx context.Context, req *connect.Request[adminpb.UpdateAiStyleRequest]) (*connect.Response[adminpb.UpdateAiStyleResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetAiStyle()
	id, err := rpc.ParseUUID("ai_style.id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "ai_style.etag")
	if err != nil {
		return nil, err
	}
	cur, err := h.m.ReadStyleForAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	merged, err := h.toStyle(cur)
	if err != nil {
		return nil, err
	}
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	s, err := h.m.UpdateStyle(ctx, imageai.UpdateStyleInput{
		Actor: imageai.Actor{AdminID: admin, Action: req.Spec().Procedure}, ID: id, Version: version,
		Label: merged.GetLabel(), Prompt: merged.GetPrompt(), ThumbnailToken: merged.GetThumbnailStagedToken(),
		DisplayOrder: merged.GetDisplayOrder(), Enabled: merged.GetEnabled(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, stylePaths)
	}
	out, err := h.toStyle(s)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.UpdateAiStyleResponse{AiStyle: out}), nil
}

// SetDefaultAiStyle 把風格設成它用途的預設（自訂方法）；etag 選填，帶了由模組比對。
func (h *Admin) SetDefaultAiStyle(ctx context.Context, req *connect.Request[adminpb.SetDefaultAiStyleRequest]) (*connect.Response[adminpb.SetDefaultAiStyleResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	s, err := h.m.SetDefaultStyle(ctx, imageai.Actor{AdminID: admin, Action: req.Spec().Procedure}, id, version)
	if err != nil {
		return nil, err
	}
	out, err := h.toStyle(s)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.SetDefaultAiStyleResponse{AiStyle: out}), nil
}

func (h *Admin) DeleteAiStyle(ctx context.Context, req *connect.Request[adminpb.DeleteAiStyleRequest]) (*connect.Response[adminpb.DeleteAiStyleResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteStyle(ctx, imageai.Actor{AdminID: admin, Action: req.Spec().Procedure}, id, version); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.DeleteAiStyleResponse{}), nil
}

func (h *Admin) GetAiQuota(ctx context.Context, req *connect.Request[adminpb.GetAiQuotaRequest]) (*connect.Response[adminpb.GetAiQuotaResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	q, err := h.m.GetQuota(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetAiQuotaResponse{AiQuota: toQuota(q)}), nil
}

// UpdateAiQuota 把 update_mask 內的欄位套在目前的額度（沒有個別設定時是預設額度）上，再交給模組；沒帶 etag 只能建立，建立時 weekly_limit 必填
// （在 update_mask 裡，沒有 mask 時要非零），否則 invalid_argument（ai_quota.weekly_limit）。
func (h *Admin) UpdateAiQuota(ctx context.Context, req *connect.Request[adminpb.UpdateAiQuotaRequest]) (*connect.Response[adminpb.UpdateAiQuotaResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetAiQuota()
	storeID, err := rpc.ParseID("ai_quota.store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(msg.GetEtag())
	if err != nil {
		return nil, err
	}
	cur, err := h.m.GetQuota(ctx, storeID)
	if err != nil {
		return nil, err
	}
	merged := toQuota(cur)
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	if version == 0 && !cur.Custom {
		fields, err := rpc.WritableFields(msg, req.Msg.GetUpdateMask())
		if err != nil {
			return nil, err
		}
		if !slices.Contains(fields, "weekly_limit") {
			return nil, rpc.RequiredField("ai_quota.weekly_limit", "建立個別額度必須提供 weekly_limit")
		}
	}
	q, err := h.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{
		Actor: imageai.Actor{AdminID: admin, Action: req.Spec().Procedure}, StoreID: storeID,
		WeeklyLimit: merged.GetWeeklyLimit(), Memo: merged.GetMemo(), Version: version,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, quotaPaths)
	}
	return connect.NewResponse(&adminpb.UpdateAiQuotaResponse{AiQuota: toQuota(q)}), nil
}

func (h *Admin) DeleteAiQuota(ctx context.Context, req *connect.Request[adminpb.DeleteAiQuotaRequest]) (*connect.Response[adminpb.DeleteAiQuotaResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteQuota(ctx, imageai.Actor{AdminID: admin, Action: req.Spec().Procedure}, storeID, version); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.DeleteAiQuotaResponse{}), nil
}

func (h *Admin) toStyle(s imageai.AdminStyle) (*adminpb.AiStyle, error) {
	if err := rpc.ObjectKey(s.ThumbnailKey); err != nil {
		return nil, err
	}
	return &adminpb.AiStyle{
		Id: s.ID.String(), Name: s.Name, Label: s.Label, Prompt: s.Prompt, Thumbnail: h.images.Image(s.ThumbnailKey),
		DisplayOrder: s.DisplayOrder, DefaultStyle: s.IsDefault, Enabled: s.Enabled,
		Category:   rpc.EnumToProto[pb.AiStyleCategory](s.Category),
		CreateTime: timestamppb.New(s.CreatedAt), UpdateTime: timestamppb.New(s.UpdatedAt), Etag: rpc.Etag(s.Version),
	}, nil
}

func toQuota(q imageai.AdminQuota) *adminpb.AiQuota {
	out := &adminpb.AiQuota{
		StoreId: rpc.FormatID(q.StoreID), WeeklyLimit: q.WeeklyLimit, WeeklyUsage: q.WeeklyUsage, Memo: q.Memo, Custom: q.Custom,
	}
	if q.Custom {
		out.UpdateTime = timestamppb.New(q.UpdatedAt)
		out.Etag = rpc.Etag(q.Version)
	}
	return out
}
