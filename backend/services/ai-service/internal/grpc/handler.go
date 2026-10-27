package grpc

import (
	"context"
	"errors"
	"strconv"

	mediaClient "github.com/Will413028/hawkr/ai-service/internal/client/media"
	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AIGrpcServer struct {
	pb.UnimplementedAIImageServiceServer
	pb.UnimplementedAIAdminServiceServer
	pb.UnimplementedRecommendationServiceServer
	recommendUseCase *usecase.RecommendUseCase
	styleUseCase     *usecase.StyleUseCase
	quotaUseCase     *usecase.QuotaUseCase
	transformUseCase *usecase.TransformUseCase
	mediaClient      mediaClient.API
}

const aiStyleThumbnailKeyPrefix = "public/"

const genericFailureMessage = "服務暫時無法使用，請稍後再試"

func domainMessage(err error) string {
	var dc pkgerrors.DomainCoder
	if errors.As(err, &dc) {
		return dc.DomainMessage()
	}
	return genericFailureMessage
}

func NewAIGrpcServer(
	recommendUC *usecase.RecommendUseCase,
	styleUC *usecase.StyleUseCase,
	quotaUC *usecase.QuotaUseCase,
	transformUC *usecase.TransformUseCase,
	mc mediaClient.API,
) *AIGrpcServer {
	return &AIGrpcServer{
		recommendUseCase: recommendUC,
		styleUseCase:     styleUC,
		quotaUseCase:     quotaUC,
		transformUseCase: transformUC,
		mediaClient:      mc,
	}
}

func (s *AIGrpcServer) PersonalizedRecommend(ctx context.Context, req *pb.PersonalizedRequest) (*pb.PersonalizedRecommendResponse, error) {
	pkglogger.Info(ctx, "gRPC PersonalizedRecommend", zap.Int64("user_id", req.UserId), zap.String("scene", req.Scene))

	result, err := s.recommendUseCase.Personalized(ctx, req.UserId)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PersonalizedRecommendResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		return nil, err
	}

	sections := make([]*pb.PersonalizedSection, len(result.Sections))
	for i, sec := range result.Sections {
		products := make([]*pb.RecommendedProduct, len(sec.Products))
		for j, p := range sec.Products {
			products[j] = &pb.RecommendedProduct{
				Id:            strconv.FormatInt(p.ProductID, 10),
				Name:          p.Name,
				ImageKey:      p.ImageURL,
				PriceMin:      p.PriceMin,
				PriceMax:      p.PriceMax,
				Rating:        p.Rating,
				SellerName:    p.SellerName,
				SellerLogoKey: p.SellerLogo,
				Score:         p.Score,
				Reason:        p.Reason,
			}
		}
		sections[i] = &pb.PersonalizedSection{
			Title:    sec.Title,
			Type:     sec.Type,
			Products: products,
		}
	}

	return &pb.PersonalizedRecommendResponse{
		Code:    0,
		Message: "success",
		Data: &pb.PersonalizedRecommendResult{
			Sections:     sections,
			ModelVersion: result.ModelVersion,
		},
	}, nil
}

func (s *AIGrpcServer) ListAiStyles(ctx context.Context, req *pb.ListAiStylesRequest) (*pb.ListAiStylesResponse, error) {

	act, ok := ActorFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "actor not resolved")
	}
	storeID, ok := act.StoreID()
	if !ok {
		return nil, status.Error(codes.Internal, "resolved actor is not a store actor")
	}
	category := req.Category
	if category != domain.CategoryProduct && category != domain.CategoryPost {
		category = domain.CategoryProduct
	}
	pkglogger.Info(ctx, "gRPC ListAiStyles", zap.Int64("store_id", int64(storeID)), zap.String("category", category))

	result, err := s.styleUseCase.ListForApp(ctx, storeID, category)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ListAiStylesResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		return nil, err
	}

	items := make([]*pb.AiStyleItem, len(result.Styles))
	for i, st := range result.Styles {
		items[i] = &pb.AiStyleItem{
			Id:           st.ID,
			Name:         st.Name,
			Label:        st.Label,
			Prompt:       st.Prompt,
			ThumbnailKey: st.ThumbnailKey,
			SortOrder:    st.SortOrder,
		}
	}

	return &pb.ListAiStylesResponse{
		Code:    0,
		Message: "success",
		Data: &pb.AiStyleListData{
			DefaultPrompt: result.DefaultPrompt,
			WeeklyUsage:   result.WeeklyUsage,
			WeeklyLimit:   result.WeeklyLimit,
			Styles:        items,
		},
	}, nil
}

func (s *AIGrpcServer) ListAiStylesForAdmin(ctx context.Context, req *pb.ListAiStylesForAdminRequest) (*pb.ListAiStylesForAdminResponse, error) {
	pkglogger.Info(ctx, "gRPC ListAiStylesForAdmin", zap.String("category", req.Category))

	result, err := s.styleUseCase.ListForAdmin(ctx, req.Category)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ListAiStylesForAdminResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		return nil, err
	}

	items := make([]*pb.AiStyleAdminItem, len(result))
	for i, st := range result {
		items[i] = &pb.AiStyleAdminItem{
			Id:           st.ID,
			Name:         st.Name,
			Label:        st.Label,
			Prompt:       st.Prompt,
			ThumbnailKey: st.ThumbnailKey,
			SortOrder:    st.SortOrder,
			CreatedAt:    st.CreatedAt,
			UpdatedAt:    st.UpdatedAt,
			Category:     st.Category,
			IsDefault:    st.IsDefault,
			Enabled:      st.Enabled,
		}
	}

	return &pb.ListAiStylesForAdminResponse{
		Code:    0,
		Message: "success",
		Data:    items,
	}, nil
}

func (s *AIGrpcServer) CreateAiStyle(ctx context.Context, req *pb.CreateAiStyleRequest) (*pb.AiStyleResponse, error) {
	pkglogger.Info(ctx, "gRPC CreateAiStyle", zap.String("name", req.Name))

	styleID := s.styleUseCase.GenID()

	thumbnailKey := req.ThumbnailKey
	if req.ThumbnailStagedToken != nil && *req.ThumbnailStagedToken != "" {
		info, err := s.mediaClient.FinalizeAndLink(ctx, *req.ThumbnailStagedToken, pkgmedia.RefTypeAiStyle, strconv.FormatInt(styleID, 10))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "縮圖上傳失敗，請重新上傳")
		}
		thumbnailKey = info.ObjectKey
	} else if thumbnailKey != "" {

		if err := mediaurl.ValidateObjectKey(thumbnailKey, aiStyleThumbnailKeyPrefix); err != nil {
			return &pb.AiStyleResponse{Code: 400, Message: err.Error()}, nil
		}
	}

	ucReq := dto.CreateStyleRequest{
		Name:         req.Name,
		Label:        req.Label,
		Prompt:       req.Prompt,
		ThumbnailKey: thumbnailKey,
		Category:     req.Category,
		SortOrder:    req.SortOrder,
		IsDefault:    req.IsDefault,
		Enabled:      req.Enabled,
	}

	if err := s.styleUseCase.Create(ctx, styleID, ucReq); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.AiStyleResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		return nil, err
	}

	return &pb.AiStyleResponse{Code: 0, Message: "success"}, nil
}

func (s *AIGrpcServer) UpdateAiStyle(ctx context.Context, req *pb.UpdateAiStyleRequest) (*pb.AiStyleResponse, error) {
	pkglogger.Info(ctx, "gRPC UpdateAiStyle", zap.String("id", req.Id))

	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.AiStyleResponse{Code: 400, Message: "invalid id"}, nil
	}

	ucReq := dto.UpdateStyleRequest{}
	if req.HasLabel {
		ucReq.Label = &req.Label
	}
	if req.HasPrompt {
		ucReq.Prompt = &req.Prompt
	}
	var newThumbnailAssetID string
	if req.ThumbnailStagedToken != nil && *req.ThumbnailStagedToken != "" {

		info, fErr := s.mediaClient.FinalizeAndLink(ctx, *req.ThumbnailStagedToken, pkgmedia.RefTypeAiStyle, req.Id)
		if fErr != nil {
			return nil, status.Errorf(codes.Internal, "縮圖上傳失敗，請重新上傳")
		}
		key := info.ObjectKey
		ucReq.ThumbnailKey = &key
		newThumbnailAssetID = info.AssetId
	} else if req.HasThumbnailKey {

		key := req.ThumbnailKey
		if vErr := mediaurl.ValidateObjectKey(key, aiStyleThumbnailKeyPrefix); vErr != nil {
			return &pb.AiStyleResponse{Code: 400, Message: vErr.Error()}, nil
		}
		ucReq.ThumbnailKey = &key
	}
	if req.HasSortOrder {
		ucReq.SortOrder = &req.SortOrder
	}
	if req.HasEnabled {
		ucReq.Enabled = &req.Enabled
	}
	if req.HasIsDefault {
		ucReq.IsDefault = &req.IsDefault
	}

	if err := s.styleUseCase.Update(ctx, id, ucReq); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.AiStyleResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		pkglogger.Error(ctx, "ai style update failed", zap.String("id", req.Id), zap.Error(err))
		return &pb.AiStyleResponse{Code: 400, Message: genericFailureMessage}, nil
	}

	if newThumbnailAssetID != "" {
		if relErr := s.mediaClient.ReleaseReferences(ctx, pkgmedia.RefTypeAiStyle,
			[]string{req.Id}, []string{newThumbnailAssetID}); relErr != nil {
			pkglogger.Warn(ctx, "failed to release superseded ai-style thumbnail reference", zap.String("id", req.Id), zap.Error(relErr))
		}
	}

	return &pb.AiStyleResponse{Code: 0, Message: "success"}, nil
}

func (s *AIGrpcServer) DeleteAiStyle(ctx context.Context, req *pb.DeleteAiStyleRequest) (*pb.AiStyleResponse, error) {
	pkglogger.Info(ctx, "gRPC DeleteAiStyle", zap.String("id", req.Id))

	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.AiStyleResponse{Code: 400, Message: "invalid id"}, nil
	}

	if err := s.styleUseCase.Delete(ctx, id); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.AiStyleResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		pkglogger.Error(ctx, "ai style delete failed", zap.String("id", req.Id), zap.Error(err))
		return &pb.AiStyleResponse{Code: 400, Message: genericFailureMessage}, nil
	}

	if relErr := s.mediaClient.ReleaseReferences(ctx, pkgmedia.RefTypeAiStyle, []string{req.Id}, nil); relErr != nil {
		pkglogger.Warn(ctx, "failed to release ai-style thumbnail reference on delete", zap.String("id", req.Id), zap.Error(relErr))
	}

	return &pb.AiStyleResponse{Code: 0, Message: "success"}, nil
}

func (s *AIGrpcServer) GetAiQuota(ctx context.Context, req *pb.GetAiQuotaRequest) (*pb.AiQuotaResponse, error) {
	pkglogger.Info(ctx, "gRPC GetAiQuota", zap.String("store_id", req.StoreId))

	raw, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.AiQuotaResponse{Code: 400, Message: "invalid store_id"}, nil
	}
	storeID := identity.StoreID(raw)

	result, err := s.quotaUseCase.GetQuota(ctx, storeID)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.AiQuotaResponse{Code: int32(code), Message: domainMessage(err)}, nil
		}
		return nil, err
	}

	return &pb.AiQuotaResponse{
		Code:    0,
		Message: "success",
		Data: &pb.AiQuotaData{
			StoreId:     result.StoreID,
			WeeklyLimit: result.WeeklyLimit,
			WeeklyUsage: result.WeeklyUsage,
			Memo:        result.Memo,
			UpdatedAt:   result.UpdatedAt,
			IsCustom:    result.IsCustom,
		},
	}, nil
}

func (s *AIGrpcServer) UpdateAiQuota(ctx context.Context, req *pb.UpdateAiQuotaRequest) (*pb.AiQuotaResponse, error) {
	pkglogger.Info(ctx, "gRPC UpdateAiQuota", zap.String("store_id", req.StoreId))

	raw, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.AiQuotaResponse{Code: 400, Message: "invalid store_id"}, nil
	}
	storeID := identity.StoreID(raw)

	ucReq := dto.UpdateQuotaRequest{
		WeeklyLimit: req.WeeklyLimit,
		Memo:        req.Memo,
	}

	if err := s.quotaUseCase.UpdateQuota(ctx, storeID, ucReq); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {

			return &pb.AiQuotaResponse{Code: 400, Message: domainMessage(err)}, nil
		}
		pkglogger.Error(ctx, "ai quota update failed", zap.String("store_id", req.StoreId), zap.Error(err))
		return &pb.AiQuotaResponse{Code: 400, Message: genericFailureMessage}, nil
	}

	return &pb.AiQuotaResponse{Code: 0, Message: "success"}, nil
}

func (s *AIGrpcServer) DeleteAiQuota(ctx context.Context, req *pb.DeleteAiQuotaRequest) (*pb.AiQuotaResponse, error) {
	pkglogger.Info(ctx, "gRPC DeleteAiQuota", zap.String("store_id", req.StoreId))

	raw, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.AiQuotaResponse{Code: 400, Message: "invalid store_id"}, nil
	}
	storeID := identity.StoreID(raw)

	if err := s.quotaUseCase.DeleteQuota(ctx, storeID); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {

			return &pb.AiQuotaResponse{Code: 400, Message: domainMessage(err)}, nil
		}
		pkglogger.Error(ctx, "ai quota delete failed", zap.String("store_id", req.StoreId), zap.Error(err))
		return &pb.AiQuotaResponse{Code: 400, Message: genericFailureMessage}, nil
	}

	return &pb.AiQuotaResponse{Code: 0, Message: "success"}, nil
}

const (
	msgAiTransformNoImage       = "這次沒有成功產生圖片，請再試一次"
	msgAiTransformSafetyBlocked = "這張照片或描述沒有通過內容審查，請換一張照片或修改描述"
	msgAiTransformRecitation    = "產生的圖片與版權內容過於相似，請換一張照片或修改描述"
	msgAiProviderUnavailable    = "AI 美化暫時無法使用，請稍後再試"
)

func aiTransformBizError(err error) (int32, string, bool) {
	var refusal *domain.ProviderRefusalError
	if errors.As(err, &refusal) {

		if refusal.BlockReason != "" {
			return bizcode.AiTransformSafetyBlocked, msgAiTransformSafetyBlocked, true
		}
		switch refusal.FinishReason {
		case domain.FinishReasonImageSafety, domain.FinishReasonImageProhibited:
			return bizcode.AiTransformSafetyBlocked, msgAiTransformSafetyBlocked, true
		case domain.FinishReasonImageRecitation:
			return bizcode.AiTransformRecitation, msgAiTransformRecitation, true
		default:

			return bizcode.AiTransformNoImage, msgAiTransformNoImage, true
		}
	}

	var unavailable *domain.ProviderUnavailableError
	if errors.As(err, &unavailable) {
		return bizcode.AiProviderUnavailable, msgAiProviderUnavailable, true
	}

	return 0, "", false
}

func (s *AIGrpcServer) TransformImage(ctx context.Context, req *pb.TransformImageRequest) (*pb.TransformImageResponse, error) {

	act, ok := ActorFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "actor not resolved")
	}
	storeID, ok := act.StoreID()
	if !ok {
		return nil, status.Error(codes.Internal, "resolved actor is not a store actor")
	}
	pkglogger.Info(ctx, "gRPC TransformImage", zap.Int64("store_id", int64(storeID)), zap.Int("image_size", len(req.ImageData)))

	resp, err := s.transformUseCase.Transform(ctx, storeID, req.ImageData, req.MimeType, req.Prompt)
	if err != nil {
		var inflightErr *usecase.InFlightLimitError
		if errors.As(err, &inflightErr) {
			return nil, status.Error(codes.ResourceExhausted, inflightErr.Error())
		}
		var quotaErr *usecase.QuotaExceededError
		if errors.As(err, &quotaErr) {
			return &pb.TransformImageResponse{Code: bizcode.AiQuotaExceeded, Message: quotaErr.Error()}, nil
		}
		if code, msg, ok := aiTransformBizError(err); ok {

			pkglogger.Warn(ctx, "ai transform rejected",
				zap.Int32("biz_code", code), zap.Error(err))
			return &pb.TransformImageResponse{Code: code, Message: msg}, nil
		}
		pkglogger.Error(ctx, "ai transform failed", zap.Int64("store_id", int64(storeID)), zap.Error(err))
		return &pb.TransformImageResponse{Code: 500, Message: genericFailureMessage}, nil
	}

	return &pb.TransformImageResponse{
		Code:    0,
		Message: "success",
		Data: &pb.TransformImageData{
			ImageBase64: resp.ImageBase64,
			MimeType:    resp.MimeType,
			WeeklyUsage: resp.WeeklyUsage,
			WeeklyLimit: resp.WeeklyLimit,
		},
	}, nil
}

func init() {
	var (
		imagePrefix     = "/" + pb.AIImageService_ServiceDesc.ServiceName + "/"
		adminPrefix     = "/" + pb.AIAdminService_ServiceDesc.ServiceName + "/"
		recommendPrefix = "/" + pb.RecommendationService_ServiceDesc.ServiceName + "/"
	)

	for _, m := range []string{"TransformImage", "ListAiStyles"} {
		registerMethodPolicy(imagePrefix+m, MethodPolicy{Actor: ActorSeller})
	}

	for _, m := range []string{
		"ListAiStylesForAdmin", "CreateAiStyle", "UpdateAiStyle", "DeleteAiStyle",
		"GetAiQuota", "UpdateAiQuota", "DeleteAiQuota",
	} {
		registerMethodPolicy(adminPrefix+m, MethodPolicy{Actor: ActorAdmin})
	}

	registerMethodPolicy(recommendPrefix+"PersonalizedRecommend", MethodPolicy{Actor: ActorNone})
}
