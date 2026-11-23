package media

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// DelegatedDeclaration 是 MediaDelegatedService 每個方法的代行宣告（API 設計規範（Connect）§二的 Delegated）：需要的能力 media.upload（B1-D16），
// 以及被拒絕時稽核列的動作與資源種類。上傳本身不寫成功的稽核（稽核留在擁有者確認時的寫入），所以動作 MEDIA_UPLOAD 只會出現在 FAILED 列。
var DelegatedDeclaration = httpdelegation.Declaration{
	Service: pb.File_hawkr_media_v1alpha1_media_proto.Services().ByName("MediaDelegatedService"),
	Methods: map[string]moddelegation.DelegatedMethod{
		mediav1alpha1connect.MediaDelegatedServiceBeginSecureUploadProcedure: {
			Capability: moddelegation.CapabilityMediaUpload,
			Action:     moddelegation.AuditMediaUpload,
			Resource:   moddelegation.ResourceMedia,
		},
	},
}

// DelegationReader 是 handler 需要的 delegation 公開 API（*delegation.Module）：能力與店家可代行的檢查（DelegatedRead）。
type DelegationReader interface {
	DelegatedRead(ctx context.Context, c moddelegation.DelegatedCaller, read func(ctx context.Context, st seller.Store) error) error
}

// DelegatedUploads 是代行開會話的來源（*media.Module）。
type DelegatedUploads interface {
	BeginSecureUpload(ctx context.Context, uploader media.Uploader, size int64) (media.SecureBeginResult, error)
}

// Delegated 實作 MediaDelegatedService：只做傳輸；授權在 delegation 的 DelegatedRead，會話在 media 模組。
type Delegated struct {
	runner  DelegationReader
	uploads DelegatedUploads
	log     *slog.Logger
}

var _ mediav1alpha1connect.MediaDelegatedServiceHandler = (*Delegated)(nil)

// NewDelegated 建立 handler；uploads 為 nil 介面時（media 還沒接上，由呼叫方明確傳 nil，不是裝著 nil 指標的介面）授權過後回 unavailable。
func NewDelegated(runner DelegationReader, uploads DelegatedUploads, log *slog.Logger) *Delegated {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Delegated{runner: runner, uploads: uploads, log: log}
}

// BeginSecureUpload 以代行 session 的管理員為上傳者開會話（B1-D16）：Uploader 是 {ID: 管理員, Admin: true, Store: 被代行的店家}，會話綁定那家店；段上傳與預覽走後台路由，只有這位管理員在同一家店的代行 session 之下能續傳，擁有者確認時也要帶同一家店。
// 成功不寫代行稽核；缺 media.upload 或店家不可代行由 DelegatedRead 拒絕並寫 FAILED。
func (h *Delegated) BeginSecureUpload(ctx context.Context, req *connect.Request[pb.MediaDelegatedServiceBeginSecureUploadRequest]) (*connect.Response[pb.MediaDelegatedServiceBeginSecureUploadResponse], error) {
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	var res media.SecureBeginResult
	err = h.runner.DelegatedRead(ctx, c, func(ctx context.Context, st seller.Store) error {
		if h.uploads == nil {
			return rpc.Error(connect.CodeUnavailable, "media 模組尚未接上")
		}
		var err error
		res, err = h.uploads.BeginSecureUpload(ctx, media.Uploader{ID: c.Session.Actor.AdminID, Admin: true, Store: st.StoreID}, req.Msg.GetSize())
		return err
	})
	if errors.Is(err, media.ErrNoKeyring) {
		h.log.ErrorContext(ctx, "media：機敏上傳需要 KEK 但模組沒有設定", "error", err)
		return nil, apperr.Unavailable("服務暫時無法使用")
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.MediaDelegatedServiceBeginSecureUploadResponse{
		StagedToken: res.Token,
		ChunkSize:   int32(res.ChunkSize),  //nolint:gosec // SecureChunkSize 是 4 MiB
		ChunkCount:  int32(res.ChunkCount), //nolint:gosec // 段總數最多 MaxSecureUploadSize / SecureChunkSize
		ExpireTime:  timestamppb.New(res.ExpiresAt),
	}), nil
}
