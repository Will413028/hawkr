package grpc

import (
	"context"
	"errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


const clearDefaultSentinel = "0"

// CreateBuyerInvoiceProfile 新增常用發票資料（端點 2）。
func (s *InvoiceServer) CreateBuyerInvoiceProfile(
	ctx context.Context, req *pb.CreateBuyerInvoiceProfileRequest,
) (*pb.BuyerInvoiceProfileResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}

	profile, err := s.prefs.Create(ctx, usecase.CreateProfileInput{
		BuyerID:     buyerID,
		CarrierType: req.GetCarrierType(),
		Value:       req.GetValue(),
		BuyerTaxID:  req.GetBuyerTaxId(),
		BuyerName:   req.GetBuyerName(),
		Label:       req.GetLabel(),
		SetDefault:  req.GetSetDefault(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrIdempotencyKeyReused) {
			return &pb.BuyerInvoiceProfileResponse{
				Code:    bizcode.InvoiceIdempotencyPayloadMismatch,
				Message: "相同的 Idempotency-Key 已用於不同的請求內容",
			}, nil
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.BuyerInvoiceProfileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.preference.create.failed", err)
	}
	return &pb.BuyerInvoiceProfileResponse{Code: 0, Message: msgSuccess, Data: toProfileInfo(profile)}, nil
}

// UpdateBuyerInvoiceProfile 編輯常用發票資料（端點 3）。
func (s *InvoiceServer) UpdateBuyerInvoiceProfile(
	ctx context.Context, req *pb.UpdateBuyerInvoiceProfileRequest,
) (*pb.BuyerInvoiceProfileResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	profileID, err := parseID(req.GetProfileId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "profile_id: %v", err)
	}

	profile, err := s.prefs.Update(ctx, usecase.UpdateProfileInput{
		BuyerID:    buyerID,
		ProfileID:  profileID,
		Value:      req.GetValue(),
		BuyerTaxID: req.GetBuyerTaxId(),
		BuyerName:  req.GetBuyerName(),
		Label:      req.GetLabel(),
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.BuyerInvoiceProfileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.preference.update.failed", err)
	}
	return &pb.BuyerInvoiceProfileResponse{Code: 0, Message: msgSuccess, Data: toProfileInfo(profile)}, nil
}

// DeleteBuyerInvoiceProfile 刪除常用發票資料（端點 4）。
//
// 失敗時 deleted 留 false：回 code≠0 ＋ deleted=true 會讓前端把列從畫面上拿掉，
// 而資料還在。
func (s *InvoiceServer) DeleteBuyerInvoiceProfile(
	ctx context.Context, req *pb.DeleteBuyerInvoiceProfileRequest,
) (*pb.DeleteBuyerInvoiceProfileResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	profileID, err := parseID(req.GetProfileId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "profile_id: %v", err)
	}

	if err := s.prefs.Delete(ctx, buyerID, profileID); err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.DeleteBuyerInvoiceProfileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.preference.delete.failed", err)
	}
	return &pb.DeleteBuyerInvoiceProfileResponse{Code: 0, Message: msgSuccess, Deleted: true}, nil
}

// SetDefaultBuyerInvoiceProfile 設為預設（端點 5），profile_id = "0" 為清除預設。
func (s *InvoiceServer) SetDefaultBuyerInvoiceProfile(
	ctx context.Context, req *pb.SetDefaultBuyerInvoiceProfileRequest,
) (*pb.BuyerInvoiceProfileResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}

	if req.GetProfileId() == clearDefaultSentinel {
		if err := s.prefs.ClearDefault(ctx, buyerID); err != nil {
			if code, msg, ok := bizOf(err); ok {
				return &pb.BuyerInvoiceProfileResponse{Code: code, Message: msg}, nil
			}
			return nil, s.internal(ctx, "invoice.preference.clear_default.failed", err)
		}
		return &pb.BuyerInvoiceProfileResponse{
			Code:    0,
			Message: msgSuccess,
			Data:    &pb.InvoiceProfileInfo{ProfileId: clearDefaultSentinel, IsDefault: false},
		}, nil
	}

	profileID, err := parseID(req.GetProfileId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "profile_id: %v", err)
	}

	profile, err := s.prefs.SetDefault(ctx, buyerID, profileID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.BuyerInvoiceProfileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.preference.set_default.failed", err)
	}
	return &pb.BuyerInvoiceProfileResponse{Code: 0, Message: msgSuccess, Data: toProfileInfo(profile)}, nil
}
