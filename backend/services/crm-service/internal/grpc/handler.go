package grpc

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/crm-service/internal/usecase/dto"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
)

type CrmServer struct {
	pb.UnimplementedCrmServiceServer
	crmUC  *usecase.CrmUseCase
	logger *zap.Logger
}

func NewCrmServer(crmUC *usecase.CrmUseCase, logger *zap.Logger) *CrmServer {
	return &CrmServer{
		crmUC:  crmUC,
		logger: logger,
	}
}

// ==================== Customer ====================

func (s *CrmServer) GetCustomerList(ctx context.Context, req *pb.GetCustomerListRequest) (*pb.CustomerListResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CustomerListResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	var tagID int64
	if req.TagId != "" {
		tagID, err = strconv.ParseInt(req.TagId, 10, 64)
		if err != nil {
			return &pb.CustomerListResponse{Code: 400, Message: fmt.Sprintf("invalid tag_id: %s", req.TagId)}, nil
		}
	}
	result, err := s.crmUC.ListCustomers(ctx, dto.ListCustomersReq{
		StoreID:      storeID,
		Page:         req.Page,
		PageSize:     req.PageSize,
		Keyword:      req.Keyword,
		CustomerType: req.CustomerType,
		TagID:        tagID,
		SortBy:       req.SortBy,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		s.logger.Error("GetCustomerList failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CustomerListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	list := make([]*pb.CrmCustomer, 0, len(result.List))
	for _, c := range result.List {
		list = append(list, toCrmCustomerProto(c))
	}

	return &pb.CustomerListResponse{
		Code:    0,
		Message: "ok",
		Data: &pb.CustomerListData{
			Total: strconv.FormatInt(result.Total, 10),
			List:  list,
		},
	}, nil
}

func (s *CrmServer) GetCustomerStats(ctx context.Context, req *pb.GetCustomerStatsRequest) (*pb.CustomerStatsResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CustomerStatsResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	buyerID, err := strconv.ParseInt(req.BuyerId, 10, 64)
	if err != nil {
		return &pb.CustomerStatsResponse{Code: 400, Message: fmt.Sprintf("invalid buyer_id: %s", req.BuyerId)}, nil
	}
	customer, err := s.crmUC.GetCustomer(ctx, storeID, buyerID)
	if err != nil {
		s.logger.Error("GetCustomerStats failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CustomerStatsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	// Get customer tags
	tags, _ := s.crmUC.GetCustomerTags(ctx, storeID, buyerID)
	customer.Tags = tags

	return &pb.CustomerStatsResponse{
		Code:    0,
		Message: "ok",
		Data:    toCrmCustomerProto(customer),
	}, nil
}

func (s *CrmServer) UpdateCustomerStatus(ctx context.Context, req *pb.UpdateCustomerStatusRequest) (*pb.CrmResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	buyerID, err := strconv.ParseInt(req.BuyerId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid buyer_id: %s", req.BuyerId)}, nil
	}
	if err := s.crmUC.UpdateCustomerType(ctx, dto.UpdateCustomerTypeReq{
		StoreID:      storeID,
		BuyerID:      buyerID,
		CustomerType: req.CustomerType,
	}); err != nil {
		s.logger.Error("UpdateCustomerStatus failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CrmResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CrmResponse{Code: 0, Message: "ok"}, nil
}

// ==================== Tags ====================
func (s *CrmServer) CreateTag(ctx context.Context, req *pb.CreateTagRequest) (*pb.TagResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.TagResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	tag, err := s.crmUC.CreateTag(ctx, dto.CreateTagReq{
		StoreID: storeID,
		Name:    req.Name,
	})
	if err != nil {
		s.logger.Error("CreateTag failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.TagResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.TagResponse{Code: 0, Message: "ok", Data: toTagProto(tag)}, nil
}

func (s *CrmServer) ListTags(ctx context.Context, req *pb.ListTagsRequest) (*pb.TagListResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.TagListResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	tags, err := s.crmUC.ListTags(ctx, storeID)
	if err != nil {
		s.logger.Error("ListTags failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.TagListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	pbTags := make([]*pb.Tag, 0, len(tags))
	for _, t := range tags {
		pbTags = append(pbTags, toTagProto(t))
	}
	return &pb.TagListResponse{Code: 0, Message: "ok", Data: pbTags}, nil
}

func (s *CrmServer) UpdateTag(ctx context.Context, req *pb.UpdateTagRequest) (*pb.CrmResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	tagID, err := strconv.ParseInt(req.TagId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid tag_id: %s", req.TagId)}, nil
	}
	if err := s.crmUC.UpdateTag(ctx, dto.UpdateTagReq{
		StoreID: storeID,
		TagID:   tagID,
		Name:    req.Name,
	}); err != nil {
		s.logger.Error("UpdateTag failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CrmResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CrmResponse{Code: 0, Message: "ok"}, nil
}

func (s *CrmServer) DeleteTag(ctx context.Context, req *pb.DeleteTagRequest) (*pb.CrmResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	tagID, err := strconv.ParseInt(req.TagId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid tag_id: %s", req.TagId)}, nil
	}
	if err := s.crmUC.DeleteTag(ctx, storeID, tagID); err != nil {
		s.logger.Error("DeleteTag failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CrmResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CrmResponse{Code: 0, Message: "ok"}, nil
}

func (s *CrmServer) AddCustomerTags(ctx context.Context, req *pb.AddCustomerTagsRequest) (*pb.CrmResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	buyerID, err := strconv.ParseInt(req.BuyerId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid buyer_id: %s", req.BuyerId)}, nil
	}
	if err := s.crmUC.AddCustomerTags(ctx, dto.AddCustomerTagsByNameReq{
		StoreID:  storeID,
		BuyerID:  buyerID,
		TagNames: req.TagNames,
	}); err != nil {
		s.logger.Error("AddCustomerTags failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CrmResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CrmResponse{Code: 0, Message: "ok"}, nil
}

func (s *CrmServer) RemoveCustomerTag(ctx context.Context, req *pb.RemoveCustomerTagRequest) (*pb.CrmResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	buyerID, err := strconv.ParseInt(req.BuyerId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid buyer_id: %s", req.BuyerId)}, nil
	}
	tagID, err := strconv.ParseInt(req.TagId, 10, 64)
	if err != nil {
		return &pb.CrmResponse{Code: 400, Message: fmt.Sprintf("invalid tag_id: %s", req.TagId)}, nil
	}
	if err := s.crmUC.RemoveCustomerTag(ctx, dto.ManageCustomerTagReq{
		StoreID: storeID,
		BuyerID: buyerID,
		TagID:   tagID,
	}); err != nil {
		s.logger.Error("RemoveCustomerTag failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CrmResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CrmResponse{Code: 0, Message: "ok"}, nil
}

func (s *CrmServer) SearchCustomersByTags(ctx context.Context, req *pb.SearchCustomersByTagsRequest) (*pb.SearchCustomersByTagsResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.SearchCustomersByTagsResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	// string → int64 轉換
	tagIDs := make([]int64, 0, len(req.TagIds))
	for _, idStr := range req.TagIds {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return &pb.SearchCustomersByTagsResponse{
				Code:    400,
				Message: fmt.Sprintf("invalid tag_id: %s", idStr),
			}, nil
		}
		tagIDs = append(tagIDs, id)
	}

	result, err := s.crmUC.SearchCustomersByTags(ctx, dto.SearchCustomersByTagsReq{
		StoreID: storeID,
		TagIDs:  tagIDs,
	})
	if err != nil {
		s.logger.Error("SearchCustomersByTags failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.SearchCustomersByTagsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	// int64 → string 轉換
	customers := make([]*pb.MatchedCustomer, 0, len(result.Customers))
	for _, c := range result.Customers {
		customers = append(customers, &pb.MatchedCustomer{
			BuyerId:   strconv.FormatInt(c.BuyerID, 10),
			Nickname:  c.Nickname,
			Phone:     c.Phone,
			AvatarKey: c.Avatar, // 49 §6.6 S9：透傳 objectKey；Gateway 組 avatarUrl
		})
	}

	return &pb.SearchCustomersByTagsResponse{
		Code:    0,
		Message: "ok",
		Data: &pb.SearchCustomersByTagsData{
			Total:     strconv.FormatInt(int64(result.Total), 10),
			Customers: customers,
		},
	}, nil
}

func (s *CrmServer) GetBuyerVipStatuses(ctx context.Context, req *pb.GetBuyerVipStatusesRequest) (*pb.GetBuyerVipStatusesResponse, error) {
	buyerID, err := strconv.ParseInt(req.BuyerId, 10, 64)
	if err != nil {
		return &pb.GetBuyerVipStatusesResponse{Code: 400, Message: fmt.Sprintf("invalid buyer_id: %s", req.BuyerId)}, nil
	}
	storeIDs := make([]int64, 0, len(req.StoreIds))
	for _, idStr := range req.StoreIds {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return &pb.GetBuyerVipStatusesResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", idStr)}, nil
		}
		storeIDs = append(storeIDs, id)
	}
	statuses, err := s.crmUC.GetBuyerVipStatuses(ctx, buyerID, storeIDs)
	if err != nil {
		s.logger.Error("GetBuyerVipStatuses failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetBuyerVipStatusesResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	data := make([]*pb.BuyerVipStatus, 0, len(statuses))
	for storeID, customerType := range statuses {
		data = append(data, &pb.BuyerVipStatus{
			StoreId:      strconv.FormatInt(storeID, 10),
			CustomerType: customerType,
		})
	}
	return &pb.GetBuyerVipStatusesResponse{Code: 0, Message: "ok", Data: data}, nil
}

func (s *CrmServer) GetCustomerTags(ctx context.Context, req *pb.GetCustomerTagsRequest) (*pb.TagListResponse, error) {
	storeID, err := strconv.ParseInt(req.StoreId, 10, 64)
	if err != nil {
		return &pb.TagListResponse{Code: 400, Message: fmt.Sprintf("invalid store_id: %s", req.StoreId)}, nil
	}
	buyerID, err := strconv.ParseInt(req.BuyerId, 10, 64)
	if err != nil {
		return &pb.TagListResponse{Code: 400, Message: fmt.Sprintf("invalid buyer_id: %s", req.BuyerId)}, nil
	}
	tags, err := s.crmUC.GetCustomerTags(ctx, storeID, buyerID)
	if err != nil {
		s.logger.Error("GetCustomerTags failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.TagListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	pbTags := make([]*pb.Tag, 0, len(tags))
	for _, t := range tags {
		pbTags = append(pbTags, toTagProto(t))
	}
	return &pb.TagListResponse{Code: 0, Message: "ok", Data: pbTags}, nil
}
