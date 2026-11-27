// Package couponserver 在 Java order 與 aftersales 改寫成 Go 之前，以舊的 gRPC 契約把它們對 coupon 的呼叫轉給
// coupon 模組：order 的驗券、核銷、釋放，aftersales 的補償券。其餘 CouponService 方法一律 Unimplemented：買家、賣家、admin 的
// 入口已經是 Connect。本套件隨 Java 的呼叫端刪除。
//
// 契約與舊 coupon-service 相同（業務失敗用 code／message 的 envelope、gRPC 狀態是 OK），只有四處不同：
//   - ReserveCoupon 多兩個必填欄位 store_id 與 order_amount（D10），讓模組的 Lock 照常重驗店家範圍與門檻（BD1、BD2）；
//     缺值是 gRPC InvalidArgument，不略過檢查。
//   - 補償券的 GrantSellerCompensationCoupon 多一個必填的 source_id（售後單 id，D4），與 AFTERSALES_CASE 組成冪等鍵。
//   - Consume、Release 以 order_no 為鍵（D3），instance_id 不讀；Java 在鎖券回應遺失時沒有 instance_id 也能釋放。
//   - GrantSellerCompensationCoupon 的 message 欄位不讀：補償券模組沒有對應欄位，標題由「售後補償券 」加 ref_case_no 組成。
//
// Java 送的 couponTemplateId 是活動 id；每位買家在同一個活動至多持有一張券，所以對應是 (活動, 買家) → 券（D1）。沒有領過的
// 回「未領取」，不當場發券。
package couponserver

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/coupon"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/coupon/couponconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	codeOK            = 0
	codeInvalidParam  = 400
	codeNotClaimed    = 7011
	codeInvalidStatus = 7009
	codeReserveFailed = 7008
)

var compensationEndAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)

// Server 實作 legacy 的 CouponService 中 order 與 aftersales 呼叫的方法。
type Server struct {
	couponconnect.UnimplementedCouponServiceHandler
	coupon *coupon.Module
	log    *slog.Logger
}

var _ couponconnect.CouponServiceHandler = (*Server)(nil)

// New 建立 server。
func New(m *coupon.Module, log *slog.Logger) *Server {
	if m == nil {
		panic("couponserver: missing dependency")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Server{coupon: m, log: log}
}

func parseTemplate(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("template_id 不是有效的活動 id：%q", s)
	}
	return id, nil
}

func parsePositive(field, s string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s 必須是正整數：%q", field, s)
	}
	return n, nil
}

// ValidateCoupon 驗券不鎖：與 Lock 同一組檢查（模組的 Quote）。回應形狀：
//   - 通過：valid=true、discount_amount、template_id；discount_type 與 description 不填（Java 只讀 valid、discount_amount 與失敗時的 description）。
//   - 活動不存在、不可用、不在時間窗、不符資格、店家範圍或門檻、券已被占用：code=0、valid=false、description 是原因（與舊契約同形）。
//   - 沒領過：envelope code=7011，不當場發券（D1）。
//   - 參數錯誤：envelope code=400。
//
// 其餘錯誤（資料庫、port 讀取失敗）是 gRPC 錯誤，不降級成 valid=false。
func (s *Server) ValidateCoupon(ctx context.Context, req *connect.Request[pb.ValidateCouponRequest]) (*connect.Response[pb.ValidateCouponResponse], error) {
	m := req.Msg
	reply := func(code int32, msg string, data *pb.CouponValidateResult) (*connect.Response[pb.ValidateCouponResponse], error) {
		return connect.NewResponse(&pb.ValidateCouponResponse{Code: code, Message: msg, Data: data}), nil
	}
	campaignID, err := parseTemplate(m.GetTemplateId())
	if err != nil {
		return reply(codeInvalidParam, userMessage(err), nil)
	}
	storeID, err := parsePositive("store_id", m.GetStoreId())
	if err != nil {
		return reply(codeInvalidParam, userMessage(err), nil)
	}
	amount, err := parsePositive("order_amount", m.GetOrderAmount())
	if err != nil {
		return reply(codeInvalidParam, userMessage(err), nil)
	}
	v, err := s.coupon.VoucherForCampaign(ctx, m.GetBuyerId(), campaignID)
	if err != nil {
		return s.validateFailure(err, reply)
	}
	res, err := s.coupon.Quote(ctx, coupon.LockInput{VoucherID: v.ID, BuyerID: m.GetBuyerId(), StoreID: storeID, OrderAmount: amount})
	if err != nil {
		return s.validateFailure(err, reply)
	}
	return reply(codeOK, "success", &pb.CouponValidateResult{
		Valid:          true,
		DiscountAmount: strconv.FormatInt(res.Discount, 10),
		TemplateId:     campaignID.String(),
	})
}

func (s *Server) validateFailure(err error, reply func(int32, string, *pb.CouponValidateResult) (*connect.Response[pb.ValidateCouponResponse], error)) (*connect.Response[pb.ValidateCouponResponse], error) {
	if coupon.ReasonOf(err) == coupon.ReasonVoucherNotFound {
		return reply(codeNotClaimed, "此優惠券尚未領取", nil)
	}
	switch apperr.KindOf(err) {
	case apperr.KindInvalidInput:
		return reply(codeInvalidParam, userMessage(err), nil)
	case apperr.KindNotFound, apperr.KindFailedPrecondition:
		return reply(codeOK, "success", &pb.CouponValidateResult{Valid: false, Description: userMessage(err)})
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindUnauthenticated,
		apperr.KindPermissionDenied, apperr.KindUnavailable, apperr.KindResourceExhausted:
		return nil, connectError(err)
	default:
		return nil, connectError(err)
	}
}

// ReserveCoupon 把買家在這個活動持有的券鎖給 order_no：與 Validate 同一個 (活動, 買家) → 券的對應（D1），再以 store_id 與
// order_amount 呼叫模組的 Lock，店家範圍與門檻每次重驗（D10）。同一筆訂單重送冪等，instance_id 是券 id。
//   - template_id、store_id、order_amount、order_no 缺值或格式錯誤：gRPC InvalidArgument（不略過檢查、不降級成 envelope）。
//   - 沒領過：success=false、code=7011（與 Validate 同）。
//   - 不可用、不在時間窗、不符資格、店家範圍、門檻、被別單鎖住、這筆訂單已持有別張券：success=false、code=7008、message 是原因。
//
// 其餘錯誤（資料庫、port 讀取失敗）是 gRPC 錯誤，Java 當鎖券失敗補償。
func (s *Server) ReserveCoupon(ctx context.Context, req *connect.Request[pb.ReserveCouponRequest]) (*connect.Response[pb.ReserveCouponResponse], error) {
	m := req.Msg
	invalid := func(err error) (*connect.Response[pb.ReserveCouponResponse], error) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	campaignID, err := parseTemplate(m.GetTemplateId())
	if err != nil {
		return invalid(err)
	}
	storeID, err := parsePositive("store_id", m.GetStoreId())
	if err != nil {
		return invalid(err)
	}
	amount, err := parsePositive("order_amount", m.GetOrderAmount())
	if err != nil {
		return invalid(err)
	}
	fail := func(code int32, msg string) (*connect.Response[pb.ReserveCouponResponse], error) {
		return connect.NewResponse(&pb.ReserveCouponResponse{Code: code, Message: msg}), nil
	}
	v, err := s.coupon.VoucherForCampaign(ctx, m.GetBuyerId(), campaignID)
	if err == nil {
		var res coupon.LockResult
		res, err = s.coupon.Lock(ctx, coupon.LockInput{VoucherID: v.ID, BuyerID: m.GetBuyerId(), OrderNo: m.GetOrderNo(), StoreID: storeID, OrderAmount: amount})
		if err == nil {
			return connect.NewResponse(&pb.ReserveCouponResponse{Code: codeOK, Message: "success", Success: true, InstanceId: res.Voucher.ID.String()}), nil
		}
	}
	if coupon.ReasonOf(err) == coupon.ReasonVoucherNotFound {
		return fail(codeNotClaimed, "此優惠券尚未領取")
	}
	switch apperr.KindOf(err) {
	case apperr.KindInvalidInput:
		return invalid(err)
	case apperr.KindNotFound, apperr.KindFailedPrecondition:
		return fail(codeReserveFailed, userMessage(err))
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindUnauthenticated,
		apperr.KindPermissionDenied, apperr.KindUnavailable, apperr.KindResourceExhausted:
		return nil, connectError(err)
	default:
		return nil, connectError(err)
	}
}

// ConsumeCoupon 核銷持有這筆訂單的券，以 order_no 為鍵、冪等（重送回成功）；instance_id 與 buyer_id 不讀。
// 這筆訂單沒有持有任何券時回 success=false、code=7009（與舊契約的「狀態不允許」相同）。
func (s *Server) ConsumeCoupon(ctx context.Context, req *connect.Request[pb.ConsumeCouponRequest]) (*connect.Response[pb.ConsumeCouponResponse], error) {
	_, err := s.coupon.Redeem(ctx, req.Msg.GetOrderNo())
	switch {
	case err == nil:
		return connect.NewResponse(&pb.ConsumeCouponResponse{Code: codeOK, Message: "success", Success: true}), nil
	case apperr.KindOf(err) == apperr.KindInvalidInput:
		return connect.NewResponse(&pb.ConsumeCouponResponse{Code: codeInvalidParam, Message: userMessage(err)}), nil
	case coupon.ReasonOf(err) == coupon.ReasonNoVoucherForOrder:
		return connect.NewResponse(&pb.ConsumeCouponResponse{Code: codeInvalidStatus, Message: "消耗失敗"}), nil
	default:
		return nil, connectError(err)
	}
}

// ReleaseCoupon 釋放持有這筆訂單的券，以 order_no 為鍵、冪等（D3）：重送、回應遺失後的重送、這筆訂單沒有持有任何券
// （結帳鎖券失敗而沒記下 instance id、或本來就沒用券）都回成功。instance_id 可省略，有值也不讀。
func (s *Server) ReleaseCoupon(ctx context.Context, req *connect.Request[pb.ReleaseCouponRequest]) (*connect.Response[pb.ReleaseCouponResponse], error) {
	_, err := s.coupon.Release(ctx, req.Msg.GetOrderNo())
	switch {
	case err == nil:
		return connect.NewResponse(&pb.ReleaseCouponResponse{Code: codeOK, Message: "success", Success: true}), nil
	case apperr.KindOf(err) == apperr.KindInvalidInput:
		return connect.NewResponse(&pb.ReleaseCouponResponse{Code: codeInvalidParam, Message: userMessage(err)}), nil
	default:
		return nil, connectError(err)
	}
}

// GrantSellerCompensationCoupon 發賣家補償券：source_id（售後單 id，必填）與 AFTERSALES_CASE 是冪等鍵，重送回同一張券的資料，
// 不重發。到期日是 compensationEndAt（舊契約沒有到期日欄位）；標題是「售後補償券 」加 ref_case_no。
// 規則與參數錯誤都是 envelope code=400，其他錯誤是 gRPC 錯誤（aftersales 以重送處理）。
func (s *Server) GrantSellerCompensationCoupon(ctx context.Context, req *connect.Request[pb.GrantSellerCompensationCouponRequest]) (*connect.Response[pb.GrantSellerCompensationCouponResponse], error) {
	m := req.Msg
	bad := func(msg string) (*connect.Response[pb.GrantSellerCompensationCouponResponse], error) {
		return connect.NewResponse(&pb.GrantSellerCompensationCouponResponse{Code: codeInvalidParam, Message: msg}), nil
	}
	storeID, err := parsePositive("store_id", m.GetStoreId())
	if err != nil {
		return bad(userMessage(err))
	}
	buyerID, err := parsePositive("buyer_id", m.GetBuyerId())
	if err != nil {
		return bad(userMessage(err))
	}
	amount, err := parsePositive("amount", m.GetAmount())
	if err != nil {
		return bad(userMessage(err))
	}
	minOrder, err := parsePositive("min_order_amount", m.GetMinOrderAmount())
	if err != nil {
		return bad(userMessage(err))
	}
	sourceID := strings.TrimSpace(m.GetSourceId())
	if sourceID == "" {
		return bad("source_id 必填")
	}
	res, err := s.coupon.IssueCompensation(ctx, coupon.CompensationInput{
		SourceType: coupon.SourceAftersalesCase, SourceID: sourceID, BuyerID: buyerID, StoreID: storeID,
		Amount: amount, MinOrderAmount: minOrder, EndAt: compensationEndAt,
		Title:  strings.TrimSpace("售後補償券 " + m.GetRefCaseNo()),
		Issuer: coupon.Issuer{StoreID: storeID},
	})
	if err != nil {
		if apperr.KindOf(err) == apperr.KindInvalidInput {
			return bad(userMessage(err))
		}
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.GrantSellerCompensationCouponResponse{
		Code: codeOK, Message: "success",
		Data: &pb.SellerCompensationCouponData{
			TemplateId:     res.Campaign.ID.String(),
			Amount:         strconv.FormatInt(res.Campaign.DiscountAmount, 10),
			MinOrderAmount: strconv.FormatInt(res.Campaign.ThresholdAmount, 10),
		},
	}), nil
}
