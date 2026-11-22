// Package media 是 MediaAppService 的 handler：登入的買家與賣家以預簽網址直傳公開圖片到暫存區，並讀處理狀態。
//
// 上傳者一律是呼叫者的帳號（由伺服器推導，請求不帶）；管理員等非顧客帳號回 permission_denied。
// 模組的錯誤由 rpc 的 interceptor 轉換：暫存不存在與不是呼叫者的是同一個 not_found，過期是 failed_precondition；
// 處理被拒絕不是錯誤，以 state 與 reject_reason 回報。
package media

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// Uploads 是暫存上傳的來源（*media.Module）。
type Uploads interface {
	BeginUpload(ctx context.Context, uploader media.Uploader, in media.BeginInput) (media.BeginResult, error)
	CompleteUpload(ctx context.Context, uploader media.Uploader, token string) error
	GetUpload(ctx context.Context, uploader media.Uploader, token string) (media.UploadStatus, error)
}

// App 實作 MediaAppService。
type App struct {
	uploads Uploads
}

var _ mediav1alpha1connect.MediaAppServiceHandler = (*App)(nil)

// NewApp 建立 handler；m 為 nil 時每個方法回 unavailable。
func NewApp(m *media.Module) *App {
	if m == nil {
		return &App{}
	}
	return newApp(m)
}

func newApp(uploads Uploads) *App { return &App{uploads: uploads} }

func (h *App) uploader(ctx context.Context) (media.Uploader, Uploads, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return media.Uploader{}, nil, err
	}
	if h.uploads == nil {
		return media.Uploader{}, nil, rpc.Error(connect.CodeUnavailable, "media 模組尚未接上")
	}
	return media.Uploader{ID: p.ID}, h.uploads, nil
}

func (h *App) BeginUpload(ctx context.Context, req *connect.Request[pb.BeginUploadRequest]) (*connect.Response[pb.BeginUploadResponse], error) {
	u, uploads, err := h.uploader(ctx)
	if err != nil {
		return nil, err
	}
	res, err := uploads.BeginUpload(ctx, u, media.BeginInput{Size: req.Msg.GetSize(), ContentType: req.Msg.GetContentType(), MD5: req.Msg.GetMd5()})
	if err != nil {
		return nil, err
	}
	headers := make(map[string]string, len(res.Headers))
	for k, v := range res.Headers {
		if !strings.EqualFold(k, "Content-Length") {
			headers[k] = v
		}
	}
	return connect.NewResponse(&pb.BeginUploadResponse{
		StagedToken:   res.Token,
		UploadUrl:     res.URL,
		UploadHeaders: headers,
		ExpireTime:    timestamppb.New(res.ExpiresAt),
	}), nil
}

func (h *App) CompleteUpload(ctx context.Context, req *connect.Request[pb.CompleteUploadRequest]) (*connect.Response[pb.CompleteUploadResponse], error) {
	u, uploads, err := h.uploader(ctx)
	if err != nil {
		return nil, err
	}
	if err := uploads.CompleteUpload(ctx, u, req.Msg.GetStagedToken()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CompleteUploadResponse{}), nil
}

func (h *App) GetUpload(ctx context.Context, req *connect.Request[pb.GetUploadRequest]) (*connect.Response[pb.GetUploadResponse], error) {
	u, uploads, err := h.uploader(ctx)
	if err != nil {
		return nil, err
	}
	st, err := uploads.GetUpload(ctx, u, req.Msg.GetStagedToken())
	if err != nil {
		return nil, err
	}
	out := &pb.GetUploadResponse{State: rpc.EnumToProto[pb.UploadState](st.State), ExpireTime: timestamppb.New(st.ExpiresAt)}
	if st.RejectReason != "" {
		r := rpc.EnumToProto[pb.RejectReason](st.RejectReason)
		out.RejectReason = &r
	}
	return connect.NewResponse(out), nil
}
