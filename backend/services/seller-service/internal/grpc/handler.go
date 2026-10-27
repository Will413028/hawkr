package grpc

import (
	"context"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/delegation"
	"github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/go-pkg/twaddr"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/domain/certholder"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type SellerServer struct {
	pb.UnimplementedSellerServiceServer
	sellerUC              *usecase.SellerUseCase
	certificationUC       *usecase.CertificationUseCase
	payuniUC              *usecase.PayuniUseCase
	businessRegUC         *usecase.BusinessRegistrationUseCase
	geocodeUC             *usecase.AddressGeocodeUseCase
	deleteSellerUC        *usecase.DeleteSellerUseCase
	deleteSellerAccountUC *usecase.DeleteSellerAccountUseCase
	storeVisitorUC        *usecase.StoreVisitorUseCase
	recommendedSellersUC  *usecase.RecommendedSellersUseCase
	productViewUC         *usecase.ProductViewUseCase
	violationReportUC     *usecase.ViolationReportUseCase
	foodSafetyReportUC    *usecase.FoodSafetyReportUseCase
	violationUC           *usecase.SellerViolationUseCase
	foodSafetyEnabled     bool
	suspensionUC          *usecase.SellerSuspensionUseCase
	membershipUC          *usecase.SellerMembershipUseCase
	prospectSearcher      prospectSearcher
	prospectCreator       prospectCreator
	prospectAdmin         prospectAdminUseCase
	vendorPortal          *vendorPortalHandler

	buyerClient  usecase.BuyerClient
	reviewClient usecase.ReviewClient
	mediaBaseURL string
}

func NewSellerServer(
	cfg *config.Config,
	sellerUC *usecase.SellerUseCase,
	certificationUC *usecase.CertificationUseCase,
	payuniUC *usecase.PayuniUseCase,
	businessRegUC *usecase.BusinessRegistrationUseCase,
	geocodeUC *usecase.AddressGeocodeUseCase,
	deleteSellerUC *usecase.DeleteSellerUseCase,
	deleteSellerAccountUC *usecase.DeleteSellerAccountUseCase,
	storeVisitorUC *usecase.StoreVisitorUseCase,
	recommendedSellersUC *usecase.RecommendedSellersUseCase,
	productViewUC *usecase.ProductViewUseCase,
	violationReportUC *usecase.ViolationReportUseCase,
	foodSafetyReportUC *usecase.FoodSafetyReportUseCase,
	violationUC *usecase.SellerViolationUseCase,
	suspensionUC *usecase.SellerSuspensionUseCase,
	membershipUC *usecase.SellerMembershipUseCase,
	prospectUC *usecase.SellerProspectUseCase,
	vendorPortalRepo domain.VendorPortalRepository,
	buyerClient usecase.BuyerClient,
	reviewClient usecase.ReviewClient,
) *SellerServer {
	return &SellerServer{
		sellerUC:              sellerUC,
		certificationUC:       certificationUC,
		payuniUC:              payuniUC,
		businessRegUC:         businessRegUC,
		geocodeUC:             geocodeUC,
		deleteSellerUC:        deleteSellerUC,
		deleteSellerAccountUC: deleteSellerAccountUC,
		storeVisitorUC:        storeVisitorUC,
		recommendedSellersUC:  recommendedSellersUC,
		productViewUC:         productViewUC,
		violationReportUC:     violationReportUC,
		foodSafetyReportUC:    foodSafetyReportUC,
		violationUC:           violationUC,
		foodSafetyEnabled:     cfg.FeatureFlags.FoodSafetyEndpointEnabled,
		suspensionUC:          suspensionUC,
		membershipUC:          membershipUC,
		prospectSearcher:      prospectUC,
		prospectCreator:       prospectUC,
		prospectAdmin:         prospectUC,
		vendorPortal:          newVendorPortalHandler(vendorPortalRepo),
		buyerClient:           buyerClient,
		reviewClient:          reviewClient,
		mediaBaseURL:          cfg.MediaBaseURL,
	}
}

func (s *SellerServer) getSellerResponse(ctx context.Context, accountID int64) (*pb.SellerResponse, error) {
	return s.getSellerResponseByProfile(ctx, true, func(ctx context.Context) (*dto.SellerProfile, error) {
		return s.sellerUC.GetByAccountID(ctx, accountID)
	})
}

func (s *SellerServer) getSellerResponseByProfile(ctx context.Context, translateNotFound bool, fetch func(context.Context) (*dto.SellerProfile, error)) (*pb.SellerResponse, error) {
	profile, err := fetch(ctx)
	if err != nil {
		if translateNotFound && errors.IsNotFound(err) {
			return &pb.SellerResponse{Code: bizcode.SellerNotFound, Message: "找不到您的商家資料"}, nil
		}
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if profile.Status == string(domain.SellerStatusClosed) {
		return &pb.SellerResponse{Code: 4040, Message: "商家不存在或已關閉"}, nil
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-account-type"); len(vals) > 0 {
			if v, err := strconv.ParseInt(vals[0], 10, 32); err == nil {
				profile.AccountType = int32(v)
			}
		}
	}
	data := toSellerInfoProto(profile, s.mediaBaseURL)
	data.OverallCertificationStatus = profile.OverallCertificationStatus
	// 評分是 best-effort 的補充欄位：自己的期限，review-service 變慢時不拖住整個 GetSeller（單體的入口每個
	// 轉發請求都經它讀停權狀態）。
	rctx, cancel := context.WithTimeout(ctx, reviewStatsTimeout)
	if rs, err := s.reviewClient.GetSellerReviewStats(rctx, profile.StoreID); err == nil {
		data.Rating = domain.RoundRating(rs.AvgRating)
	}
	cancel()
	if profile.Status == string(domain.SellerStatusSuspended) && s.suspensionUC != nil {
		if active, err := s.suspensionUC.GetActive(ctx, profile.AccountID); err == nil && active != nil {
			data.SuspensionReasonCode = active.ReasonCode
		}
	}
	return &pb.SellerResponse{Code: 0, Data: data}, nil
}

// reviewStatsTimeout 是 GetSeller 補評分的上限。
const reviewStatsTimeout = 300 * time.Millisecond

func delegatedOwnerStoreID(storeID, accountID int64) int64 {
	if storeID > 0 {
		return storeID
	}
	return accountID
}

func (s *SellerServer) GetSeller(ctx context.Context, req *pb.GetSellerRequest) (*pb.SellerResponse, error) {
	if err := delegation.Validate(ctx, req.GetDelegation(),
		delegatedOwnerStoreID(req.StoreId, req.AccountId), delegation.CapStorefrontRead); err != nil {
		return nil, err
	}
	if req.StoreId > 0 {
		return s.getSellerResponseByProfile(ctx, false, func(ctx context.Context) (*dto.SellerProfile, error) {
			return s.sellerUC.GetPublicByStoreIDOrCode(ctx, req.StoreId)
		})
	}
	return s.getSellerResponse(ctx, req.AccountId)
}

func (s *SellerServer) GetSellerByMemberId(ctx context.Context, req *pb.GetSellerByMemberIdRequest) (*pb.SellerResponse, error) {
	return s.getSellerResponse(ctx, req.MemberId)
}

func (s *SellerServer) GetSellerByUserId(ctx context.Context, req *pb.GetSellerByUserIdRequest) (*pb.SellerResponse, error) {
	return s.getSellerResponse(ctx, req.UserId)
}

func (s *SellerServer) CreateSeller(ctx context.Context, req *pb.CreateSellerRequest) (*pb.SellerResponse, error) {
	profile, err := s.sellerUC.CreateSeller(ctx, usecase.CreateSellerInput{
		AccountID: req.UserId, StoreName: req.Name, ContactPhone: req.ContactPhone,
		ProspectInviteToken: req.ProspectInviteToken,
	})
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	if req.Description != "" || req.City != "" || req.Prize != "" {
		updateReq := &dto.UpdateSellerProfile{
			Target:        dto.TargetByStoreID(profile.StoreID),
			StoreName:     req.Name,
			Description:   req.Description,
			ContactPhone:  req.ContactPhone,
			City:          req.City,
			District:      req.District,
			AddressDetail: req.AddressDetail,
			Prize:         req.Prize,
		}
		if err := s.sellerUC.UpdateProfile(ctx, updateReq); err != nil {
			if code := errors.GetCode(err); code != errors.CodeUnknown {
				return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
		profile, err = s.sellerUC.GetByAccountID(ctx, profile.AccountID)
		if err != nil {
			if code := errors.GetCode(err); code != errors.CodeUnknown {
				return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
	}

	return &pb.SellerResponse{Code: 0, Data: toSellerInfoProto(profile, s.mediaBaseURL)}, nil
}

func (s *SellerServer) UpdateSeller(ctx context.Context, req *pb.UpdateSellerRequest) (*pb.SellerResponse, error) {
	target, err := sellerWriteTarget(req.GetAccountId(), req.GetStoreId())
	if err != nil {
		return nil, err
	}
	if err := delegation.Validate(ctx, req.GetDelegation(), req.GetStoreId(), delegation.CapStorefrontWrite); err != nil {
		return nil, err
	}
	expectedVersion, err := delegation.ParseExpectedVersion(req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	updateReq := &dto.UpdateSellerProfile{
		Target:          target,
		ExpectedVersion: expectedVersion,
		StoreName:       req.Name,
		Description:     req.Description,
		ContactPhone:    req.ContactPhone,
		City:            req.City,
		District:        req.District,
		AddressDetail:   req.AddressDetail,
		Birthday:        req.Birthday,
		Gender:          req.Gender,
		Prize:           req.Prize,
		WebsiteURL:      req.WebsiteUrl,
	}
	if err := s.sellerUC.UpdateProfile(ctx, updateReq); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	profile, err := s.readSellerByTarget(ctx, target)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.SellerResponse{Code: 0, Data: toSellerInfoProto(profile, s.mediaBaseURL)}, nil
}

func (s *SellerServer) UpdateMerchantProfile(ctx context.Context, req *pb.UpdateMerchantProfileRequest) (*pb.UpdateMerchantProfileResponse, error) {
	seller, err := s.sellerUC.GetByAccountID(ctx, req.GetAccountId())
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateMerchantProfileResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	expectedVersion, err := delegation.ParseExpectedVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	if expectedVersion == nil {
		return nil, delegation.InvalidCommand("expected_version is required")
	}
	ev := *expectedVersion

	result, err := s.payuniUC.UpdateMerchantProfile(ctx, &usecase.UpdateMerchantProfileRequest{
		StoreID: seller.StoreID, AccountID: req.GetAccountId(),
		IdempotencyKey: req.GetIdempotencyKey(), ExpectedVersion: ev,
		ContactName: req.GetContactName(), ContactMobile: req.GetContactMobile(),
		ContactTel: req.GetContactTel(), MerEmail: req.GetMerEmail(),
		MerAddr: req.GetMerAddr(), MerAddrPost: req.GetMerAddrPost(), MerTel: req.GetMerTel(),
	})
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateMerchantProfileResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.UpdateMerchantProfileResponse{
		Code: 0, MerchantProfileVersion: strconv.FormatInt(result.MerchantProfileVersion, 10),
	}, nil
}

func sellerWriteTarget(accountID, storeID int64) (dto.SellerTarget, error) {
	switch {
	case storeID > 0 && accountID > 0:
		return dto.SellerTarget{}, delegation.InvalidCommand("account_id and store_id are mutually exclusive")
	case storeID > 0:
		return dto.TargetByStoreID(storeID), nil
	case accountID > 0:
		return dto.TargetByAccountID(accountID), nil
	default:
		return dto.SellerTarget{}, delegation.InvalidCommand("account_id or store_id is required")
	}
}

func (s *SellerServer) readSellerByTarget(ctx context.Context, target dto.SellerTarget) (*dto.SellerProfile, error) {
	if target.StoreID() > 0 {
		return s.sellerUC.GetByStoreID(ctx, target.StoreID())
	}
	return s.sellerUC.GetByAccountID(ctx, target.AccountID())
}

func (s *SellerServer) ValidateSeller(ctx context.Context, req *pb.ValidateSellerRequest) (*pb.ValidateSellerResponse, error) {
	profile, err := s.sellerUC.GetByAccountID(ctx, req.AccountId)
	if err != nil {
		if errors.IsNotFound(err) {
			return &pb.ValidateSellerResponse{Code: 0, Valid: false}, nil
		}
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ValidateSellerResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	valid := domain.CanAcceptOrders(profile.PayuniRegistrationStatus, domain.SellerStatus(profile.Status))
	return &pb.ValidateSellerResponse{Code: 0, Valid: valid}, nil
}

func (s *SellerServer) GetSellerStats(ctx context.Context, req *pb.GetSellerStatsRequest) (*pb.SellerStatsResponse, error) {
	var profile *dto.SellerProfile
	var err error
	if req.StoreId > 0 {
		profile, err = s.sellerUC.GetPublicByStoreIDOrCode(ctx, req.StoreId)
	} else {
		profile, err = s.sellerUC.GetByAccountID(ctx, req.AccountId)
	}
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerStatsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if profile.Status == string(domain.SellerStatusClosed) {
		return &pb.SellerStatsResponse{Code: 4040, Message: "商家不存在或已關閉"}, nil
	}

	stats := &pb.SellerStats{
		StoreId: strconv.FormatInt(profile.StoreID, 10),
		Rating:  domain.RoundRating(profile.Rating),
	}

	if count, err := s.buyerClient.GetSubscriberCount(ctx, profile.StoreID); err == nil {
		stats.Followers = int32(count)
	}

	if rs, err := s.reviewClient.GetSellerReviewStats(ctx, profile.StoreID); err == nil {
		stats.ReviewCount = rs.TotalCount
		stats.Rating = domain.RoundRating(rs.AvgRating)
	}

	return &pb.SellerStatsResponse{Code: 0, Data: stats}, nil
}

func (s *SellerServer) GetSellerBatch(ctx context.Context, req *pb.GetSellerBatchRequest) (*pb.SellerBatchResponse, error) {
	var sellers []*pb.SellerInfo
	for _, storeID := range req.StoreIds {
		profile, err := s.sellerUC.GetByStoreID(ctx, storeID)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		info := toSellerInfoProto(profile, s.mediaBaseURL)
		sellers = append(sellers, info)
	}
	return &pb.SellerBatchResponse{Code: 0, Data: sellers}, nil
}

func (s *SellerServer) BatchGetSellerMeta(ctx context.Context, req *pb.BatchGetSellerMetaRequest) (*pb.BatchGetSellerMetaResponse, error) {
	sellers, err := s.sellerUC.BatchGetSellerMeta(ctx, req.StoreIds)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.BatchGetSellerMetaResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	protoSellers := make(map[int64]*pb.SellerMeta, len(sellers))
	for id, meta := range sellers {
		protoSellers[id] = toSellerMetaProto(meta, s.mediaBaseURL)
	}
	return &pb.BatchGetSellerMetaResponse{Code: 0, Sellers: protoSellers}, nil
}

func (s *SellerServer) ListPublicStoreIDsByCity(ctx context.Context, req *pb.ListPublicStoreIDsByCityRequest) (*pb.ListPublicStoreIDsByCityResponse, error) {
	city, ids, err := s.sellerUC.ListPublicStoreIDsByCity(ctx, req.City)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ListPublicStoreIDsByCityResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	storeIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		storeIDs = append(storeIDs, strconv.FormatInt(id, 10))
	}
	cityCode, _ := twaddr.CityCode(city)
	return &pb.ListPublicStoreIDsByCityResponse{Code: 0, City: city, CityCode: cityCode, StoreIds: storeIDs}, nil
}

func (s *SellerServer) ListPublicStoresByCity(ctx context.Context, _ *pb.ListPublicStoresByCityRequest) (*pb.ListPublicStoresByCityResponse, error) {
	cities, err := s.sellerUC.ListPublicStoresByCity(ctx)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ListPublicStoresByCityResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	result := make([]*pb.PublicCityStores, 0, len(cities))
	for _, city := range cities {
		storeIDs := make([]string, 0, len(city.StoreIDs))
		for _, id := range city.StoreIDs {
			storeIDs = append(storeIDs, strconv.FormatInt(id, 10))
		}
		previewIDs := make([]string, 0, len(city.PreviewStoreIDs))
		for _, id := range city.PreviewStoreIDs {
			previewIDs = append(previewIDs, strconv.FormatInt(id, 10))
		}
		result = append(result, &pb.PublicCityStores{
			CityCode: city.CityCode, CityName: city.CityName,
			StoreIds: storeIDs, PreviewStoreIds: previewIDs,
		})
	}
	return &pb.ListPublicStoresByCityResponse{Code: 0, Message: "success", Cities: result}, nil
}

func (s *SellerServer) ResolvePrincipals(ctx context.Context, req *pb.ResolvePrincipalsRequest) (*pb.ResolvePrincipalsResponse, error) {
	principals, unresolvedIDs, err := s.sellerUC.ResolvePrincipals(ctx, req.Ids)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ResolvePrincipalsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ResolvePrincipalsResponse{Code: 0, Principals: principals, UnresolvedIds: unresolvedIDs}, nil
}

func (s *SellerServer) ListStoreIDsByAccount(ctx context.Context, req *pb.ListStoreIDsByAccountRequest) (*pb.ListStoreIDsByAccountResponse, error) {
	storeIDs, err := s.sellerUC.ListStoreIDsByAccount(ctx, req.AccountId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ListStoreIDsByAccountResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ListStoreIDsByAccountResponse{Code: 0, StoreIds: storeIDs}, nil
}

func (s *SellerServer) ResolveSellerAccountID(ctx context.Context, req *pb.ResolveSellerAccountIDRequest) (*pb.ResolveSellerAccountIDResponse, error) {
	accountID, err := s.sellerUC.GetAccountIDByStoreID(ctx, req.StoreId)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "seller not found for store_id")
		}
		return nil, status.Errorf(codes.Internal, "resolve seller account id: %v", err)
	}
	return &pb.ResolveSellerAccountIDResponse{AccountId: accountID}, nil
}

func (s *SellerServer) GetCurrentStoreID(ctx context.Context, req *pb.GetCurrentStoreIDRequest) (*pb.GetCurrentStoreIDResponse, error) {
	profile, err := s.sellerUC.GetByAccountID(ctx, req.AccountId)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "no current store for account")
		}
		return nil, status.Errorf(codes.Internal, "get current store id: %v", err)
	}
	if profile.Status == string(domain.SellerStatusClosed) {
		return nil, status.Error(codes.NotFound, "no current store for account")
	}
	return &pb.GetCurrentStoreIDResponse{StoreId: profile.StoreID}, nil
}

func (s *SellerServer) CompleteOnboarding(ctx context.Context, req *pb.CompleteOnboardingRequest) (*pb.SellerResponse, error) {
	if err := s.sellerUC.CompleteOnboarding(ctx, req.AccountId); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.SellerResponse{Code: 0, Message: "success"}, nil
}

func (s *SellerServer) UpdateLogo(ctx context.Context, req *pb.UpdateLogoRequest) (*pb.UpdateLogoResponse, error) {
	target, err := sellerWriteTarget(req.GetAccountId(), req.GetStoreId())
	if err != nil {
		return nil, err
	}
	if err := delegation.Validate(ctx, req.GetDelegation(), req.GetStoreId(), delegation.CapStorefrontWrite); err != nil {
		return nil, err
	}
	if req.StagedToken == "" {
		return &pb.UpdateLogoResponse{Code: int32(errors.CodeInvalidInput), Message: "staged_token is required"}, nil
	}
	expectedVersion, err := delegation.ParseExpectedVersion(req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	logoKey, err := s.sellerUC.UpdateLogo(ctx, target, req.StagedToken, expectedVersion)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateLogoResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	profile, err := s.readSellerByTarget(ctx, target)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateLogoResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.UpdateLogoResponse{
		Code: 0,
		Data: &pb.LogoData{
			LogoKey: logoKey,
			Version: strconv.FormatInt(profile.Version, 10),
		},
	}, nil
}

func (s *SellerServer) UpdateCover(ctx context.Context, req *pb.UpdateCoverRequest) (*pb.UpdateCoverResponse, error) {
	target, err := sellerWriteTarget(req.GetAccountId(), req.GetStoreId())
	if err != nil {
		return nil, err
	}
	if err := delegation.Validate(ctx, req.GetDelegation(), req.GetStoreId(), delegation.CapStorefrontWrite); err != nil {
		return nil, err
	}
	if req.StagedToken == "" {
		return &pb.UpdateCoverResponse{Code: int32(errors.CodeInvalidInput), Message: "staged_token is required"}, nil
	}
	expectedVersion, err := delegation.ParseExpectedVersion(req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	coverKey, err := s.sellerUC.UpdateCover(ctx, target, req.StagedToken, expectedVersion)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateCoverResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	profile, err := s.readSellerByTarget(ctx, target)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateCoverResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.UpdateCoverResponse{
		Code: 0,
		Data: &pb.CoverData{
			CoverKey: coverKey,
			Version:  strconv.FormatInt(profile.Version, 10),
		},
	}, nil
}

func (s *SellerServer) GetPayuniStatus(ctx context.Context, req *pb.GetPayuniStatusRequest) (*pb.PayuniStatusResponse, error) {
	profile, err := s.sellerUC.GetByAccountID(ctx, req.UserId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.PayuniStatusResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	payuniStatus, err := s.sellerUC.GetPayuniStatus(ctx, profile.AccountID)
	if err != nil {
		if errors.IsNotFound(err) {
			return &pb.PayuniStatusResponse{
				Code: 0,
				Data: &pb.PayuniStatus{
					AccountId:     strconv.FormatInt(profile.AccountID, 10),
					PayuniStatus:  domain.PayuniStatusUnregistered.String(),
					OverallStatus: profile.OverallCertificationStatus,
					Freshness:     "not_applicable",
				},
			}, nil
		}
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.PayuniStatusResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.PayuniStatusResponse{Code: 0, Data: toPayuniStatusProto(profile, payuniStatus)}, nil
}

func (s *SellerServer) RegisterPayuni(ctx context.Context, req *pb.RegisterPayuniRequest) (*pb.PayuniStatusResponse, error) {
	return nil, status.Error(codes.FailedPrecondition, "PayUni 送件已改由平台審核核准後觸發")
}

func (s *SellerServer) RetryPayuniRegistration(ctx context.Context, req *pb.RetryPayuniRequest) (*pb.PayuniStatusResponse, error) {
	return nil, status.Error(codes.FailedPrecondition, "PayUni 送件已改由平台審核核准後觸發")
}

func (s *SellerServer) RefreshPayuniStatus(ctx context.Context, req *pb.RefreshPayuniStatusRequest) (*pb.PayuniStatusResponse, error) {
	profile, err := s.sellerUC.GetByAccountID(ctx, req.UserId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.PayuniStatusResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	if err := s.payuniUC.RefreshStatus(ctx, profile.StoreID); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.PayuniStatusResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	profile, err = s.sellerUC.GetByAccountID(ctx, req.UserId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.PayuniStatusResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	payuniStatus, err := s.sellerUC.GetPayuniStatus(ctx, profile.AccountID)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.PayuniStatusResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.PayuniStatusResponse{Code: 0, Data: toPayuniStatusProto(profile, payuniStatus)}, nil
}

func (s *SellerServer) GetSellerPayuniConfig(ctx context.Context, req *pb.GetSellerPayuniConfigRequest) (*pb.SellerPayuniConfigResponse, error) {
	profile, err := s.sellerUC.GetByAccountID(ctx, req.AccountId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerPayuniConfigResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	payuniStatus, err := s.sellerUC.GetPayuniStatus(ctx, profile.AccountID)
	if err != nil {
		if errors.IsNotFound(err) {
			return &pb.SellerPayuniConfigResponse{
				Code: 0,
				Data: &pb.SellerPayuniConfig{
					AccountId:  strconv.FormatInt(profile.AccountID, 10),
					SellerName: profile.StoreName,
					IsEnabled:  false,
				},
			}, nil
		}
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.SellerPayuniConfigResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.SellerPayuniConfigResponse{Code: 0, Data: toSellerPayuniConfigProto(profile, payuniStatus)}, nil
}

func (s *SellerServer) CreateCertificationDraft(ctx context.Context, req *pb.CreateCertificationDraftRequest) (*pb.CertificationApplicationResponse, error) {
	draftReq := &dto.CreateCertificationDraft{
		AccountID:  req.AccountId,
		SellerType: req.SellerType,
	}
	app, err := s.certificationUC.CreateDraft(ctx, draftReq)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CertificationApplicationResponse{Code: 0, Data: toCertificationApplicationProto(app)}, nil
}

func (s *SellerServer) SaveCertificationProgress(ctx context.Context, req *pb.SaveCertificationProgressRequest) (*pb.CertificationApplicationResponse, error) {
	progressReq := &dto.SaveProgress{
		ApplicationID: parseInt64(req.ApplicationId),
		CurrentStep:   int(req.CurrentStep),
		StepData:      req.StepData,
	}
	if err := s.certificationUC.SaveProgress(ctx, progressReq); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	app, err := s.certificationUC.GetApplication(ctx, parseInt64(req.ApplicationId))
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CertificationApplicationResponse{Code: 0, Data: toCertificationApplicationProto(app)}, nil
}

func (s *SellerServer) CancelCertification(ctx context.Context, req *pb.CancelCertificationRequest) (*pb.CertificationApplicationResponse, error) {
	if err := s.certificationUC.CancelDraft(ctx, parseInt64(req.ApplicationId), req.GetReason()); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	app, err := s.certificationUC.GetApplication(ctx, parseInt64(req.ApplicationId))
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CertificationApplicationResponse{Code: 0, Data: toCertificationApplicationProto(app)}, nil
}

func (s *SellerServer) SubmitCertificationApplication(ctx context.Context, req *pb.SubmitCertificationApplicationRequest) (*pb.CertificationApplicationResponse, error) {
	submitReq := &dto.SubmitApplication{
		ID:                   parseInt64(req.ApplicationId),
		SellerType:           "", // Will be retrieved from existing application
		RealName:             req.RealName,
		IDCardNo:             req.IdCardNo,
		BirthDate:            req.BirthDate,
		IDCardIssueDate:      req.IdCardIssueDate,
		IDCardIssueType:      req.IdCardIssueType,
		IDCardIssueLocation:  req.IdCardIssueLocation,
		EnglishName:          req.EnglishName,
		IDCardFrontAssetID:   req.IdCardFrontAssetId,
		IDCardBackAssetID:    req.IdCardBackAssetId,
		BankBookAssetID:      req.BankBookAssetId,
		BankName:             req.BankName,
		BankCode:             req.BankCode,
		BankAccount:          req.BankAccount,
		CompanyType:          req.CompanyType,
		CompanyName:          req.CompanyName,
		TaxID:                req.TaxId,
		CompanyRegisterDate:  req.CompanyRegisterDate,
		CompanyCity:          req.CompanyCity,
		CompanyDistrict:      req.CompanyDistrict,
		CompanyAddressDetail: req.CompanyAddressDetail,
		CompanyEmail:         req.CompanyEmail,
		CompanyPhone:         req.CompanyPhone,
		ProductType:          req.ProductType,
		IsProcessedFood:      req.IsProcessedFood,
		FoodBusinessRegNo:    req.FoodBusinessRegNo,
		DocumentAssetIDs:     req.DocumentAssetIds,
	}
	if err := s.certificationUC.Submit(ctx, submitReq); err != nil {
		return nil, err
	}

	app, err := s.certificationUC.GetApplication(ctx, parseInt64(req.ApplicationId))
	if err != nil {
		return nil, err
	}
	return &pb.CertificationApplicationResponse{Code: 0, Data: toCertificationApplicationProto(app)}, nil
}

func (s *SellerServer) GetCertificationApplication(ctx context.Context, req *pb.GetCertificationApplicationRequest) (*pb.CertificationApplicationResponse, error) {
	app, err := s.certificationUC.GetApplication(ctx, parseInt64(req.ApplicationId))
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CertificationApplicationResponse{Code: 0, Data: toCertificationApplicationProto(app)}, nil
}

func (s *SellerServer) GetCertificationApplicationForAdmin(ctx context.Context, req *pb.GetCertificationApplicationForAdminRequest) (*pb.CertificationApplicationResponse, error) {
	app, err := s.certificationUC.GetApplicationForAdmin(ctx, parseInt64(req.ApplicationId))
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationApplicationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CertificationApplicationResponse{Code: 0, Data: toCertificationApplicationProto(app)}, nil
}

func (s *SellerServer) MatchCertificateHolder(ctx context.Context, req *pb.MatchCertificateHolderRequest) (*pb.MatchCertificateHolderResponse, error) {
	res, err := s.certificationUC.MatchCertificateHolder(ctx, req.StoreId, req.HolderName)
	if err != nil {
		return nil, err
	}
	return certHolderResponse(res.Matched, res.MatchedField, res.HasReference), nil
}

func certHolderResponse(matched bool, field certholder.MatchField, hasReference bool) *pb.MatchCertificateHolderResponse {
	matched = matched && hasReference
	if !matched {
		field = certholder.MatchNone
	}
	return &pb.MatchCertificateHolderResponse{
		Matched:      matched,
		MatchedField: string(field),
		HasReference: hasReference,
	}
}

func (s *SellerServer) LookupBusinessRegistration(ctx context.Context, req *pb.LookupBusinessRegistrationRequest) (*pb.LookupBusinessRegistrationResponse, error) {
	br, err := s.businessRegUC.Lookup(ctx, req.Ban)
	if err != nil {
		return nil, err
	}
	if br == nil {
		return &pb.LookupBusinessRegistrationResponse{Code: 0, Data: &pb.BusinessRegistrationData{Found: false}}, nil
	}
	return &pb.LookupBusinessRegistrationResponse{Code: 0, Data: &pb.BusinessRegistrationData{
		Found:             true,
		Ban:               br.Ban,
		BusinessNm:        br.BusinessNm,
		BusinessAddress:   br.BusinessAddress,
		BusinessSetupDate: br.BusinessSetupDate,
		BusinessType:      br.BusinessType,
		CapitalAmount:     strconv.FormatInt(br.CapitalAmount, 10),
	}}, nil
}

func (s *SellerServer) ApproveCertification(ctx context.Context, req *pb.ApproveCertificationRequest) (*pb.ReviewResponse, error) {
	reviewReq := &dto.ReviewApplication{
		ApplicationID:    parseInt64(req.ApplicationId),
		ReviewerID:       req.ReviewerId,
		ReviewerName:     req.ReviewerName,
		Comment:          req.Comment,
		RequiredFields:   req.RequiredFields,
		PayuniWebsiteURL: req.PayuniWebsiteUrl,
	}
	if err := s.certificationUC.Approve(ctx, reviewReq); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ReviewResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ReviewResponse{Code: 0, Message: "approved"}, nil
}

func (s *SellerServer) RejectCertification(ctx context.Context, req *pb.RejectCertificationRequest) (*pb.ReviewResponse, error) {
	reviewReq := &dto.ReviewApplication{
		ApplicationID:   parseInt64(req.ApplicationId),
		ReviewerID:      req.ReviewerId,
		ReviewerName:    req.ReviewerName,
		Comment:         req.Comment,
		RejectionReason: req.RejectionReason,
		RequiredFields:  req.RequiredFields,
	}
	if err := s.certificationUC.Reject(ctx, reviewReq); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ReviewResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ReviewResponse{Code: 0, Message: "rejected"}, nil
}

func (s *SellerServer) GetCertificationReviews(ctx context.Context, req *pb.GetCertificationReviewsRequest) (*pb.CertificationReviewsResponse, error) {
	reviews, err := s.certificationUC.GetReviews(ctx, parseInt64(req.ApplicationId))
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationReviewsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	var result []*pb.CertificationReviewData
	for _, r := range reviews {
		var reviewerID int64
		if r.ReviewerID != nil {
			reviewerID = *r.ReviewerID
		}
		result = append(result, &pb.CertificationReviewData{
			Id:              strconv.FormatInt(r.ID, 10),
			ApplicationId:   strconv.FormatInt(r.ApplicationID, 10),
			ReviewerId:      strconv.FormatInt(reviewerID, 10),
			ReviewerName:    r.ReviewerName,
			Action:          r.Action,
			Result:          r.Result,
			Comment:         r.Comment,
			RejectionReason: r.RejectionReason,
			RequiredFields:  r.RequiredFields,
			CreatedAt:       timeutil.FormatISO8601(r.CreatedAt),
		})
	}
	return &pb.CertificationReviewsResponse{Code: 0, Reviews: result}, nil
}

func (s *SellerServer) GetCertificationDraft(ctx context.Context, req *pb.GetDraftRequest) (*pb.CertificationDraftResponse, error) {
	app, err := s.certificationUC.GetDraft(ctx, req.AccountId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationDraftResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if app == nil {
		return &pb.CertificationDraftResponse{Code: 0, Data: nil}, nil
	}
	return &pb.CertificationDraftResponse{Code: 0, Data: toCertificationDraftProto(app)}, nil
}

func (s *SellerServer) SaveCertificationDraft(ctx context.Context, req *pb.SaveDraftRequest) (*pb.CertificationDraftResponse, error) {
	app, err := s.certificationUC.GetDraft(ctx, req.AccountId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationDraftResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if app == nil {
		draftReq := &dto.CreateCertificationDraft{
			AccountID:  req.AccountId,
			SellerType: domain.SellerTypeIndividual.String(),
		}
		app, err = s.certificationUC.CreateDraft(ctx, draftReq)
		if err != nil {
			if code := errors.GetCode(err); code != errors.CodeUnknown {
				return &pb.CertificationDraftResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
	}

	progressReq := &dto.SaveProgress{
		ApplicationID: app.ID,
		CurrentStep:   int(req.CurrentStep),
		StepData:      req.StepDataJson,
	}
	if err := s.certificationUC.SaveProgress(ctx, progressReq); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationDraftResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.CertificationDraftResponse{
		Code: 0,
		Data: &pb.CertificationDraft{
			Id:          strconv.FormatInt(app.ID, 10),
			AccountId:   strconv.FormatInt(app.AccountID, 10),
			CurrentStep: req.CurrentStep,
			TotalSteps:  req.TotalSteps,
			Status:      domain.CertStatusDraft.String(),
		},
	}, nil
}

func (s *SellerServer) SubmitCertification(ctx context.Context, req *pb.SubmitCertificationRequest) (*pb.CertificationDraftResponse, error) {
	app, err := s.certificationUC.GetDraft(ctx, req.AccountId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationDraftResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if app == nil {
		return &pb.CertificationDraftResponse{Code: int32(errors.CodeNotFound), Message: "draft not found"}, nil
	}

	submitReq := &dto.SubmitApplication{
		ID: app.ID,
	}
	if err := s.certificationUC.Submit(ctx, submitReq); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.CertificationDraftResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.CertificationDraftResponse{
		Code: 0,
		Data: &pb.CertificationDraft{
			Id:          strconv.FormatInt(app.ID, 10),
			AccountId:   strconv.FormatInt(app.AccountID, 10),
			CurrentStep: int32(app.CurrentStep),
			TotalSteps:  int32(app.TotalSteps),
			Status:      domain.CertStatusSubmitted.String(),
		},
	}, nil
}

func (s *SellerServer) ListSellersForAdmin(ctx context.Context, req *pb.ListSellersForAdminRequest) (*pb.ListSellersForAdminResponse, error) {
	result, err := s.sellerUC.ListForAdmin(ctx, &dto.AdminListSellersRequest{
		Page:            req.Page,
		Limit:           req.Limit,
		ApprovalStatus:  req.ApprovalStatus,
		Keyword:         req.Keyword,
		LifecycleStatus: req.LifecycleStatus,
	})
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ListSellersForAdminResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	protoList := make([]*pb.AdminSellerItem, len(result.List))
	for i, item := range result.List {
		certifiedAt := ""
		if item.CertifiedAt != nil {
			certifiedAt = timeutil.FormatISO8601(*item.CertifiedAt)
		}

		var currentCertID int64
		if item.CurrentCertificationID != nil {
			currentCertID = *item.CurrentCertificationID
		}
		protoList[i] = &pb.AdminSellerItem{
			AccountId:                  strconv.FormatInt(item.AccountID, 10),
			StoreId:                    strconv.FormatInt(item.StoreID, 10),
			StoreName:                  item.StoreName,
			StoreCode:                  strconv.FormatInt(item.StoreCode, 10),
			Logo:                       item.Logo,
			ContactPhone:               item.ContactPhone,
			Email:                      item.Email,
			City:                       item.City,
			District:                   item.District,
			AddressDetail:              item.AddressDetail,
			PostalCode:                 item.PostalCode,
			Rating:                     item.Rating,
			SellerType:                 item.SellerType,
			PayuniMerStatus:            domain.PayuniMerStatus(item.PayuniStatus).String(),
			MerStatus:                  item.MerStatus,
			MemberStatus:               item.MemberStatus,
			ApprovalStatus:             item.ApprovalStatus,
			OverallCertificationStatus: item.OverallCertificationStatus,
			CertifiedAt:                certifiedAt,
			CreatedAt:                  timeutil.FormatISO8601(item.CreatedAt),
			UpdatedAt:                  timeutil.FormatISO8601(item.UpdatedAt),
			CurrentCertificationId:     strconv.FormatInt(currentCertID, 10),
			SeoTitle:                   item.SeoTitle,
			SeoDescription:             item.SeoDescription,
			Description:                item.Description,
			CoverKey:                   item.CoverKey,
			LifecycleStatus:            item.LifecycleStatus,
		}
	}

	return &pb.ListSellersForAdminResponse{
		Code:    0,
		Message: "success",
		Data: &pb.AdminSellerListData{
			List:  protoList,
			Total: result.Total,
			Page:  result.Page,
			Limit: result.Limit,
		},
	}, nil
}

func (s *SellerServer) GetSellerStatsForAdmin(ctx context.Context, req *pb.GetSellerStatsForAdminRequest) (*pb.GetSellerStatsForAdminResponse, error) {
	stats, err := s.sellerUC.GetStatsForAdmin(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetSellerStatsForAdminResponse{
		Code:    0,
		Message: "success",
		Data: &pb.AdminSellerStats{
			Total:      strconv.FormatInt(stats.Total, 10),
			Pending:    strconv.FormatInt(stats.Pending, 10),
			Approved:   strconv.FormatInt(stats.Approved, 10),
			Rejected:   strconv.FormatInt(stats.Rejected, 10),
			MonthlyNew: strconv.FormatInt(stats.MonthlyNew, 10),
		},
	}, nil
}

func (s *SellerServer) ListCertificationsForAdmin(ctx context.Context, req *pb.ListCertificationsForAdminRequest) (*pb.ListCertificationsForAdminResponse, error) {
	result, err := s.certificationUC.ListForAdmin(ctx, &dto.AdminListCertificationsRequest{
		Page:    req.Page,
		Limit:   req.Limit,
		Status:  req.Status,
		Keyword: req.Keyword,
	})
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ListCertificationsForAdminResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	protoList := make([]*pb.AdminCertificationItem, len(result.List))
	for i, item := range result.List {
		var documentUrls []string
		for _, assetId := range item.DocumentAssetIds {
			if assetId != "" {
				documentUrls = append(documentUrls, buildMediaUrl(assetId))
			}
		}

		protoList[i] = &pb.AdminCertificationItem{
			Id:                         strconv.FormatInt(item.ID, 10),
			AccountId:                  strconv.FormatInt(item.AccountID, 10),
			ApplicationNo:              item.ApplicationNo,
			Status:                     item.Status,
			SellerType:                 item.SellerType,
			RealName:                   item.RealName,
			StoreName:                  item.StoreName,
			CurrentStep:                int32(item.CurrentStep),
			TotalSteps:                 int32(item.TotalSteps),
			SubmittedAt:                timeutil.FormatISO8601Ptr(item.SubmittedAt),
			RejectedAt:                 timeutil.FormatISO8601Ptr(item.RejectedAt),
			CreatedAt:                  timeutil.FormatISO8601(item.CreatedAt),
			UpdatedAt:                  timeutil.FormatISO8601(item.UpdatedAt),
			OverallCertificationStatus: item.Status,

			IdCardFrontUrl: buildMediaUrl(item.IdCardFrontAssetId),
			IdCardBackUrl:  buildMediaUrl(item.IdCardBackAssetId),
			BankBookUrl:    buildMediaUrl(item.BankBookAssetId),
			DocumentUrls:   documentUrls,
		}
	}

	return &pb.ListCertificationsForAdminResponse{
		Code:    0,
		Message: "success",
		Data: &pb.AdminCertificationListData{
			List:  protoList,
			Total: result.Total,
			Page:  result.Page,
			Limit: result.Limit,
			Stats: &pb.AdminCertificationStats{
				Total:     result.Stats.Total,
				Submitted: result.Stats.Submitted,
				Approved:  result.Stats.Approved,
				Rejected:  result.Stats.Rejected,
			},
		},
	}, nil
}

func buildMediaUrl(assetId string) string {
	if assetId == "" {
		return ""
	}
	return "/api/v1/media/secure-download/" + assetId
}

func (s *SellerServer) GetSellerShippingProfile(ctx context.Context, req *pb.GetSellerShippingProfileRequest) (*pb.GetSellerShippingProfileResponse, error) {
	profile, err := s.sellerUC.GetShippingProfile(ctx, req.AccountId, req.StoreId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.GetSellerShippingProfileResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.GetSellerShippingProfileResponse{Code: 0, Data: toSellerShippingProfileProto(profile)}, nil
}

func (s *SellerServer) RecordStoreVisit(ctx context.Context, req *pb.RecordStoreVisitRequest) (*pb.RecordStoreVisitResponse, error) {
	if err := s.storeVisitorUC.RecordVisit(ctx, req.StoreId, req.VisitorBuyerId); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.RecordStoreVisitResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.RecordStoreVisitResponse{Code: 0}, nil
}

func (s *SellerServer) GetStoreVisitors(ctx context.Context, req *pb.GetStoreVisitorsRequest) (*pb.StoreVisitorListResponse, error) {
	list, err := s.storeVisitorUC.GetVisitors(ctx, req.StoreId, req.Page, req.PageSize)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.StoreVisitorListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	items := make([]*pb.StoreVisitorItem, 0, len(list.Records))
	for _, r := range list.Records {
		items = append(items, &pb.StoreVisitorItem{
			VisitorBuyerId: strconv.FormatInt(r.VisitorBuyerID, 10),
			VisitCount:     r.VisitCount,
			LastVisitedAt:  r.LastVisitedAt,
		})
	}
	return &pb.StoreVisitorListResponse{
		Code: 0,
		Data: &pb.StoreVisitorListData{
			Records:  items,
			Total:    list.Total,
			Page:     list.Page,
			PageSize: list.PageSize,
		},
	}, nil
}

func (s *SellerServer) GetRecommendedSellers(ctx context.Context, req *pb.GetRecommendedSellersRequest) (*pb.GetRecommendedSellersResponse, error) {
	sellers, err := s.recommendedSellersUC.GetRecommendedSellers(ctx, req.Limit)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.GetRecommendedSellersResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	result := make([]*pb.RecommendedSeller, 0, len(sellers))
	for _, seller := range sellers {
		result = append(result, &pb.RecommendedSeller{
			StoreId:     strconv.FormatInt(seller.StoreID, 10),
			StoreCode:   strconv.FormatInt(seller.StoreCode, 10),
			Name:        seller.Name,
			LogoKey:     seller.Logo,
			CoverKey:    seller.CoverKey,
			Description: seller.Description,
			Rating:      seller.Rating,
			OrderCount:  seller.OrderCount,
			City:        seller.City,
			District:    seller.District,
			Followers:   seller.Followers,
		})
	}

	return &pb.GetRecommendedSellersResponse{
		Code:    0,
		Message: "success",
		Sellers: result,
	}, nil
}

func (s *SellerServer) RecordProductView(ctx context.Context, req *pb.RecordProductViewRequest) (*pb.RecordProductViewResponse, error) {
	if err := s.productViewUC.RecordViewByProductID(ctx, parseInt64(req.ProductId)); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.RecordProductViewResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.RecordProductViewResponse{Code: 0}, nil
}

func (s *SellerServer) GetProductViewStats(ctx context.Context, req *pb.GetProductViewStatsRequest) (*pb.ProductViewStatsResponse, error) {
	viewCount, growthPct, err := s.productViewUC.GetStats(ctx, req.StoreId)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.ProductViewStatsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ProductViewStatsResponse{
		Code:      0,
		ViewCount: viewCount,
		GrowthPct: growthPct,
	}, nil
}

func (s *SellerServer) DeleteSellerAccount(ctx context.Context, req *pb.DeleteSellerAccountRequest) (*pb.DeleteSellerAccountResponse, error) {
	if err := s.deleteSellerAccountUC.Execute(ctx, usecase.DeleteSellerAccountInput{
		AccountID:            req.AccountId,
		IsAdmin:              req.Actor == pb.DeleteActor_DELETE_ACTOR_ADMIN,
		OperatorID:           req.OperatorId,
		OverrideActiveOrders: req.OverrideActiveOrders,
		Reason:               req.Reason,
	}); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.DeleteSellerAccountResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.DeleteSellerAccountResponse{Code: 0}, nil
}

func (s *SellerServer) UpdateSellerSEO(ctx context.Context, req *pb.UpdateSellerSEORequest) (*pb.UpdateSellerSEOResponse, error) {
	if err := s.sellerUC.UpdateSEOFields(ctx, req.AccountId, req.SeoTitle, req.SeoDescription); err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateSellerSEOResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.UpdateSellerSEOResponse{Code: 0, Message: "success"}, nil
}

func (s *SellerServer) UpdateSellerFoodRegistration(ctx context.Context, req *pb.UpdateSellerFoodRegistrationRequest) (*pb.UpdateSellerFoodRegistrationResponse, error) {
	isFood, regNo, updatedAt, err := s.sellerUC.UpdateFoodBusinessRegistration(ctx, req.AccountId, req.IsProcessedFood, req.FoodBusinessRegNo)
	if err != nil {
		if code := errors.GetCode(err); code != errors.CodeUnknown {
			return &pb.UpdateSellerFoodRegistrationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.UpdateSellerFoodRegistrationResponse{
		Code:              0,
		Message:           "success",
		IsProcessedFood:   isFood,
		FoodBusinessRegNo: regNo,
		UpdatedAt:         strconv.FormatInt(updatedAt.Unix(), 10),
	}, nil
}

func (s *SellerServer) SuggestAddress(ctx context.Context, req *pb.SuggestAddressRequest) (*pb.SuggestAddressResponse, error) {
	items, err := s.geocodeUC.Suggest(ctx, req.County, req.District, req.Query, 20)
	if err != nil {
		return nil, err
	}
	data := make([]*pb.AddressSuggestionData, 0, len(items))
	for _, it := range items {
		data = append(data, &pb.AddressSuggestionData{
			FullAddress: it.FullAddress,
			County:      it.County,
			District:    it.District,
			Detail:      it.Detail,
		})
	}
	return &pb.SuggestAddressResponse{Code: 0, Data: data}, nil
}

func (s *SellerServer) ValidateAddress(ctx context.Context, req *pb.ValidateAddressRequest) (*pb.ValidateAddressResponse, error) {
	result, err := s.geocodeUC.Validate(ctx, req.Address)
	if err != nil {
		return nil, err
	}
	return &pb.ValidateAddressResponse{Code: 0, Data: &pb.AddressValidationData{
		FullAddress: result.FullAddress,
		County:      result.County,
		District:    result.District,
		Detail:      result.Detail,
		Lat:         strconv.FormatFloat(result.Lat, 'f', -1, 64),
		Lng:         strconv.FormatFloat(result.Lng, 'f', -1, 64),
		Valid:       result.Valid,
	}}, nil
}

func (s *SellerServer) EnsureVendorPortalInvite(ctx context.Context, req *pb.EnsureVendorPortalInviteRequest) (*pb.EnsureVendorPortalInviteResponse, error) {
	return s.vendorPortal.EnsureVendorPortalInvite(ctx, req)
}

func (s *SellerServer) MarkVendorPortalInviteSecretDelivered(ctx context.Context, req *pb.MarkVendorPortalInviteSecretDeliveredRequest) (*pb.MarkVendorPortalInviteSecretDeliveredResponse, error) {
	return s.vendorPortal.MarkVendorPortalInviteSecretDelivered(ctx, req)
}
