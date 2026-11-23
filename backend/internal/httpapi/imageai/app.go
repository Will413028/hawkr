// Package imageai 實作賣家修圖 Connect API（ImageAiAppService）的傳輸：呼叫者的店家推導、請求欄位與錯誤路徑對應、風格轉成 proto。
// 大小、提示詞、格式、額度、名額與模型錯誤的規則都在 modules/imageai，這裡不重做。
package imageai

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1/imageaiv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Module 是修圖業務的介面（*imageai.Module）。
type Module interface {
	Transform(ctx context.Context, in imageai.TransformInput) (imageai.TransformOutput, error)
	ListStyles(ctx context.Context, category imageai.StyleCategory) (imageai.StyleList, error)
	CurrentUsage(ctx context.Context, storeID int64) (imageai.Usage, error)
}

// Stores 把登入的帳號對到目前的店家（*seller.Module）。
type Stores interface {
	CurrentStore(ctx context.Context, accountID int64) (seller.Store, error)
}

// App 是 ImageAiAppService 的 handler。
type App struct {
	m      Module
	stores Stores
	images rpc.Images
	log    *slog.Logger
}

var _ imageaiv1alpha1connect.ImageAiAppServiceHandler = (*App)(nil)

// NewApp 建立 handler；stores 推導呼叫者的店家，images 組風格縮圖。
func NewApp(m Module, stores Stores, images rpc.Images, log *slog.Logger) *App {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &App{m: m, stores: stores, images: images, log: log}
}

// 模組欄位違規的欄位名對到請求的 proto 路徑。
var (
	transformPaths = rpc.FieldPaths{"Image": "image_data", "MimeType": "mime_type", "Prompt": "prompt"}
	listPaths      = rpc.FieldPaths{"Category": "category"}
)

// store 回傳呼叫者目前的店家 ID；帳號沒有店家（沒有賣家角色）回 permission_denied。
func (h *App) store(ctx context.Context) (int64, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return 0, err
	}
	st, err := h.stores.CurrentStore(ctx, p.ID)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return 0, apperr.PermissionDenied("%s", rpc.CommonText(ctx, connect.CodePermissionDenied).Text)
		}
		return 0, err
	}
	return st.StoreID, nil
}

// TransformImage 以提示詞修一張圖。
func (h *App) TransformImage(ctx context.Context, req *connect.Request[pb.TransformImageRequest]) (*connect.Response[pb.TransformImageResponse], error) {
	storeID, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	out, err := h.m.Transform(ctx, imageai.TransformInput{
		StoreID: storeID, Image: msg.GetImageData(), MimeType: msg.GetMimeType(), Prompt: msg.GetPrompt(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, transformPaths)
	}
	return connect.NewResponse(&pb.TransformImageResponse{
		ImageData: out.Image, MimeType: out.MimeType, WeeklyUsage: out.Usage.Used, WeeklyLimit: out.Usage.Limit,
	}), nil
}

// ListAiStyles 列出某個用途的啟用中風格與本週用量；沒有設定用途是商品圖，UNSPECIFIED 不接受。
func (h *App) ListAiStyles(ctx context.Context, req *connect.Request[pb.ListAiStylesRequest]) (*connect.Response[pb.ListAiStylesResponse], error) {
	storeID, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	category := imageai.CategoryProduct
	if req.Msg.Category != nil {
		category = rpc.EnumFromProto[imageai.StyleCategory](req.Msg.GetCategory())
	}
	list, err := h.m.ListStyles(ctx, category)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	usage, err := h.m.CurrentUsage(ctx, storeID)
	if err != nil {
		return nil, err
	}
	styles := make([]*pb.AiStyle, 0, len(list.Styles))
	for _, s := range list.Styles {
		if err := rpc.ObjectKey(s.ThumbnailKey); err != nil {
			return nil, err
		}
		styles = append(styles, &pb.AiStyle{
			Id: s.ID.String(), Name: s.Name, Label: s.Label, Prompt: s.Prompt,
			Thumbnail: h.images.Image(s.ThumbnailKey), DisplayOrder: s.DisplayOrder,
		})
	}
	return connect.NewResponse(&pb.ListAiStylesResponse{
		DefaultPrompt: list.DefaultPrompt, WeeklyUsage: usage.Used, WeeklyLimit: usage.Limit, Styles: styles,
	}), nil
}
