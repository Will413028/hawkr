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

// 本檔是 B08「我的發票資料」寫入面的 Layer 5（§6.1.1 端點 2／3／4／5）。
//
// 錯誤面的兩條規則與 query.go 相同：業務錯誤走 envelope（GetCode() 是 Gateway 的
// EnvelopeGuardInterceptor 唯一的判定依據）、故障走 gRPC error。
//
// 🔴 **Tier A 的回應必須與「不存在」逐位元相同**：三支帶 profile_id 的 RPC 在
// 越權與查無時都回同一個 9302 envelope，因為那個判定發生在 SQL 的 WHERE，
// 本層沒有第二個分支可以回不同的東西。

// clearDefaultSentinel 是端點 5 的「清除預設」哨兵（§6.1.1）。
//
// pkg/tsid 的 time-based 高位恆非零 ⇒ 不會有任何一筆真資料的 id 是 "0"，
// 用數字字串是為了讓「profileId 是數字字串」不出現例外分支。
// 端點 2／3／4 收到它一律 400（它不是一筆資料，不可增刪改）——由 parseID 擋下。
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
		// #3015 裁決 A：這一行在 2026-09-02 之前不存在 —— gateway 強制要求該 header、
		// proto 也帶著它，而本層從不讀 ⇒ 同鍵不同 body 會建立第二筆資料。
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrIdempotencyKeyReused) {
			// 同一把鍵配上不同的請求內容 ⇒ 9350，且**沒有執行**。
			// 不靜默回放第一次的結果：那會讓呼叫端以為它送出的新內容被接受了。
			return &pb.BuyerInvoiceProfileResponse{
				Code:    bizcode.InvoiceIdempotencyPayloadMismatch,
				Message: "相同的 Idempotency-Key 已用於不同的請求內容",
			}, nil
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.BuyerInvoiceProfileResponse{Code: code, Message: msg}, nil
		}
		// 🔴 err 不進 log 的 message 以外的地方，且 usecase 從不把載具明碼放進錯誤訊息
		// ——這裡是明碼最後一個可能外洩的位置（§8.1.3）。
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
		// 回 {"profileId":"0","isDefault":false}（§6.1.1 端點 5 逐字）。其餘欄位留空：
		// 這不是一筆資料，填任何時間戳都是憑空造事實。
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
