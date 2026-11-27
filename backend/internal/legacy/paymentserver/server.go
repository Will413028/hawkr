// Package paymentserver 提供舊服務（Java order、aftersales 與 Go invoice-service）還在呼叫的 PaymentService 方法，由 payment 模組實作（B2 步驟 4）。
// 其他方法回 Unimplemented：舊金流商專屬的 RPC（導轉表單、回跳頁、解密、ATM 退款頁）隨舊金流商刪除。
//
// 識別對應（過渡期）：舊契約的 trade_no 與 order_id 都是訂單編號（一張訂單一筆付款，D8）；退款的 refund_no 是呼叫端建立退款時給的冪等鍵
// （售後的案件編號、order 的 AUTO:{訂單}:{交易}），不是 payment 內部的退款 ID。
// 狀態碼（舊的 PaymentTransactionStatus 與退款狀態）見 paymentStatus、refundStatus；業務碼與舊 server 一致（404、400、428），其餘錯誤回 gRPC 狀態。
package paymentserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/payment"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/payment/paymentconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	codeSuccess            = 0
	codeInvalidInput       = 400
	codeNotFound           = 404
	codeFailedPrecondition = 428
	codeTransactionNotFound = 8015
)

const (
	grantOutcomeGranted   = "GRANTED"
	autoRefundKeyPrefix   = "AUTO:"
	successMessage        = "成功"
	closedMessage         = "關閉成功"
	nothingToCloseMessage = "無需關閉"
	isoSecondLayout       = "2006-01-02T15:04:05Z"
	maxAmountDigits       = 18
)

const (
	paymentStatusInit      = "0"
	paymentStatusPaid      = "1"
	paymentStatusFailed    = "2"
	paymentStatusClosed    = "5"
	refundStatusRequested  = "0"
	refundStatusProcessing = "1"
	refundStatusSucceeded  = "2"
	refundStatusFailed     = "3"
)

// Payments 是 server 用到的 payment 模組方法；*payment.Module 實作它。
type Payments interface {
	RequestPaymentTransfer(ctx context.Context, orderNo string) (payment.PaymentTransfer, error)
	CreatePaymentRefund(ctx context.Context, in payment.RefundRequest) (payment.PaymentRefund, error)
	RefundByRequestKey(ctx context.Context, requestKey string) (payment.PaymentRefund, error)
	RefundsByRequestKeys(ctx context.Context, requestKeys []string) ([]payment.PaymentRefund, error)
	LatestRefundsByOrderNos(ctx context.Context, orderNos []string) ([]payment.PaymentRefund, error)
	PaymentByOrderNo(ctx context.Context, orderNo string) (payment.OrderPayment, error)
	PaymentsByOrderNos(ctx context.Context, orderNos []string) ([]payment.OrderPayment, error)
	CloseOrderPayment(ctx context.Context, orderNo string) (bool, error)
}

var _ Payments = (*payment.Module)(nil)

// Server 實作 legacy 的 PaymentService：被呼叫的方法由 payment 模組服務，其餘 Unimplemented。
type Server struct {
	paymentconnect.UnimplementedPaymentServiceHandler
	payments Payments
}

var _ paymentconnect.PaymentServiceHandler = (*Server)(nil)

// New 建立 server；payments 必填。
func New(payments Payments) *Server {
	if payments == nil {
		panic("paymentserver: missing dependency")
	}
	return &Server{payments: payments}
}

func businessCode(err error) (code int32, ok bool) {
	switch apperr.KindOf(err) {
	case apperr.KindNotFound:
		return codeNotFound, true
	case apperr.KindInvalidInput:
		return codeInvalidInput, true
	case apperr.KindFailedPrecondition:
		return codeFailedPrecondition, true
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindUnauthenticated,
		apperr.KindPermissionDenied, apperr.KindUnavailable, apperr.KindResourceExhausted:
		return 0, false
	default:
		return 0, false
	}
}

func millis(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

func paymentStatus(s payment.PaymentStatus) (string, error) {
	switch s {
	case payment.PaymentCreating, payment.PaymentAwaitingPayment, payment.PaymentProcessing:
		return paymentStatusInit, nil
	case payment.PaymentSucceeded:
		return paymentStatusPaid, nil
	case payment.PaymentCreateFailed:
		return paymentStatusFailed, nil
	case payment.PaymentCanceled:
		return paymentStatusClosed, nil
	case payment.PaymentAnomaly:
		return "", fmt.Errorf("paymentserver：付款 ANOMALY，結果待人工核對")
	}
	return "", fmt.Errorf("paymentserver：不認得的付款狀態 %q", s)
}

func refundStatus(s payment.PaymentRefundStatus) (string, error) {
	switch s {
	case payment.RefundPending:
		return refundStatusRequested, nil
	case payment.RefundRefunding, payment.RefundAnomaly:
		return refundStatusProcessing, nil
	case payment.RefundSucceeded:
		return refundStatusSucceeded, nil
	case payment.RefundFailed:
		return refundStatusFailed, nil
	}
	return "", fmt.Errorf("paymentserver：不認得的退款狀態 %q", s)
}

func parseAmount(raw string) (amount int64, problem string) {
	if len(raw) == 0 || len(raw) > maxAmountDigits {
		return 0, "金額不合法"
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, "金額必須是正整數"
	}
	return v, ""
}

// GrantPayment 對應轉帳：寫轉帳列並排 job（D6），冪等（已有有效轉帳回它）。付款還沒入帳、賣家收款帳戶未開通或沒有可轉金額（FailedPrecondition）回 gRPC 錯誤：
// order 的撥款 worker 對 gRPC 錯誤重試、對業務碼 ≠ 0 直接轉人工，前者才符合「等條件成立」。
func (s *Server) GrantPayment(ctx context.Context, req *connect.Request[pb.GrantPaymentRequest]) (*connect.Response[pb.GrantPaymentResponse], error) {
	orderNo := strings.TrimSpace(req.Msg.GetOrderNo())
	if orderNo == "" {
		return connect.NewResponse(&pb.GrantPaymentResponse{Code: codeInvalidInput, Message: "order_no 必填"}), nil
	}
	tr, err := s.payments.RequestPaymentTransfer(ctx, orderNo)
	if err != nil {
		if code, ok := businessCode(err); ok && code != codeFailedPrecondition {
			return connect.NewResponse(&pb.GrantPaymentResponse{Code: code, Message: err.Error()}), nil
		}
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.GrantPaymentResponse{
		Code: codeSuccess, Message: successMessage, GrantOutcome: grantOutcomeGranted,
		Data: &pb.GrantPaymentResult{
			TradeNo: orderNo, OrderId: orderNo, Amount: strconv.FormatInt(tr.Amount, 10), GrantedAt: millis(tr.CreatedAt),
		},
	}), nil
}

// CreateRefund 對應建立退款：trade_no 是訂單編號，refund_no 是呼叫端的冪等鍵（必填），以 AUTO: 開頭的是自動退款（訂單一方判斷並發起，D5 修訂）。
// 退款在 provider 接受之前就回成功（code 0，狀態 REQUESTED）：結果由 refund.succeeded 與 refund.failed 事件帶。
func (s *Server) CreateRefund(ctx context.Context, req *connect.Request[pb.CreateRefundRequest]) (*connect.Response[pb.CreateRefundResponse], error) {
	m := req.Msg
	key := strings.TrimSpace(m.GetRefundNo())
	if key == "" {
		return connect.NewResponse(&pb.CreateRefundResponse{Code: codeInvalidInput, Message: "refund_no（冪等鍵）必填"}), nil
	}
	amount, problem := parseAmount(m.GetAmount())
	if problem != "" {
		return connect.NewResponse(&pb.CreateRefundResponse{Code: codeInvalidInput, Message: problem}), nil
	}
	r, err := s.payments.CreatePaymentRefund(ctx, payment.RefundRequest{
		OrderNo: strings.TrimSpace(m.GetTradeNo()), Amount: amount, RequestKey: key, Auto: strings.HasPrefix(key, autoRefundKeyPrefix),
	})
	if err != nil {
		if code, ok := businessCode(err); ok {
			return connect.NewResponse(&pb.CreateRefundResponse{Code: code, Message: err.Error()}), nil
		}
		return nil, connectError(err)
	}
	detail, err := refundDetail(r)
	if err != nil {
		return nil, connectError(err)
	}
	detail.Reason = m.GetReason()
	return connect.NewResponse(&pb.CreateRefundResponse{Code: codeSuccess, Message: successMessage, Data: detail}), nil
}

func refundDetail(r payment.PaymentRefund) (*pb.RefundDetail, error) {
	status, err := refundStatus(r.Status)
	if err != nil {
		return nil, err
	}
	return &pb.RefundDetail{
		RefundNo: r.RequestKey, TradeNo: r.OrderNo, OrderId: r.OrderNo, Amount: strconv.FormatInt(r.Amount, 10), Status: status,
		ProviderStatus: string(r.Status), ProviderMessage: r.FailureReason, CreatedAt: millis(r.CreatedAt),
	}, nil
}

// GetRefund 以呼叫端的冪等鍵（舊的 refund_no）讀退款。
func (s *Server) GetRefund(ctx context.Context, req *connect.Request[pb.GetRefundRequest]) (*connect.Response[pb.GetRefundResponse], error) {
	key := strings.TrimSpace(req.Msg.GetRefundNo())
	if key == "" {
		return connect.NewResponse(&pb.GetRefundResponse{Code: codeInvalidInput, Message: "refund_no 必填"}), nil
	}
	r, err := s.payments.RefundByRequestKey(ctx, key)
	if err != nil {
		if code, ok := businessCode(err); ok {
			return connect.NewResponse(&pb.GetRefundResponse{Code: code, Message: err.Error()}), nil
		}
		return nil, connectError(err)
	}
	detail, err := refundDetail(r)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.GetRefundResponse{Code: codeSuccess, Message: successMessage, Data: detail}), nil
}

// BatchGetRefundProviderStatusByCaseIds 以案件編號（建立退款時的冪等鍵）批次讀退款的 provider 狀態；沒有退款的案件不出現在 entries。
func (s *Server) BatchGetRefundProviderStatusByCaseIds(ctx context.Context, req *connect.Request[pb.BatchGetRefundProviderStatusByCaseIdsRequest]) (*connect.Response[pb.BatchGetRefundProviderStatusResponse], error) {
	rows, err := s.payments.RefundsByRequestKeys(ctx, req.Msg.GetCaseIds())
	if err != nil {
		return refundStatusFailure(err)
	}
	entries := make(map[string]*pb.ProviderStatusEntry, len(rows))
	for _, r := range rows {
		if _, dup := entries[r.RequestKey]; dup {
			return nil, connectError(apperr.FailedPrecondition("同一個案件編號對到多筆付款的退款"))
		}
		entries[r.RequestKey] = &pb.ProviderStatusEntry{ProviderStatus: string(r.Status), ProviderMessage: r.FailureReason}
	}
	return connect.NewResponse(&pb.BatchGetRefundProviderStatusResponse{Code: codeSuccess, Message: successMessage, Entries: entries}), nil
}

// BatchGetRefundProviderStatusByOrderIds 以訂單編號批次讀每張訂單最新一筆退款的 provider 狀態；沒有退款的訂單不出現在 entries。
func (s *Server) BatchGetRefundProviderStatusByOrderIds(ctx context.Context, req *connect.Request[pb.BatchGetRefundProviderStatusByOrderIdsRequest]) (*connect.Response[pb.BatchGetRefundProviderStatusResponse], error) {
	rows, err := s.payments.LatestRefundsByOrderNos(ctx, req.Msg.GetOrderIds())
	if err != nil {
		return refundStatusFailure(err)
	}
	entries := make(map[string]*pb.ProviderStatusEntry, len(rows))
	for _, r := range rows {
		entries[r.OrderNo] = &pb.ProviderStatusEntry{ProviderStatus: string(r.Status), ProviderMessage: r.FailureReason}
	}
	return connect.NewResponse(&pb.BatchGetRefundProviderStatusResponse{Code: codeSuccess, Message: successMessage, Entries: entries}), nil
}

func refundStatusFailure(err error) (*connect.Response[pb.BatchGetRefundProviderStatusResponse], error) {
	if code, ok := businessCode(err); ok {
		return connect.NewResponse(&pb.BatchGetRefundProviderStatusResponse{Code: code, Message: err.Error()}), nil
	}
	return nil, connectError(err)
}

// BatchGetProviderStatusByOrderIds 以訂單編號批次讀付款的 provider 狀態（付款狀態名與最近一次嘗試失敗的原因碼）；沒有付款的訂單不出現在 entries。
func (s *Server) BatchGetProviderStatusByOrderIds(ctx context.Context, req *connect.Request[pb.BatchGetProviderStatusByOrderIdsRequest]) (*connect.Response[pb.BatchGetProviderStatusByOrderIdsResponse], error) {
	rows, err := s.payments.PaymentsByOrderNos(ctx, req.Msg.GetOrderIds())
	if err != nil {
		if code, ok := businessCode(err); ok {
			return connect.NewResponse(&pb.BatchGetProviderStatusByOrderIdsResponse{Code: code, Message: err.Error()}), nil
		}
		return nil, connectError(err)
	}
	entries := make(map[string]*pb.ProviderStatusEntry, len(rows))
	for _, p := range rows {
		entries[p.Order.OrderNo] = &pb.ProviderStatusEntry{ProviderStatus: string(p.Status), ProviderMessage: p.LastError}
	}
	return connect.NewResponse(&pb.BatchGetProviderStatusByOrderIdsResponse{Code: codeSuccess, Message: successMessage, Entries: entries}), nil
}

// GetActivePaymentStatusByOrderNo 讀訂單的付款狀態（order 逾期取消前的守門）：沒有付款是 code 0 且沒有 data（訂單未建立付款屬正常）。
// 付款是 ANOMALY 回 gRPC 錯誤，order 端因此保守不取消。
func (s *Server) GetActivePaymentStatusByOrderNo(ctx context.Context, req *connect.Request[pb.GetActivePaymentStatusByOrderNoRequest]) (*connect.Response[pb.GetActivePaymentStatusByOrderNoResponse], error) {
	orderNo := strings.TrimSpace(req.Msg.GetOrderNo())
	if orderNo == "" {
		return connect.NewResponse(&pb.GetActivePaymentStatusByOrderNoResponse{Code: codeInvalidInput, Message: "order_no 必填"}), nil
	}
	p, err := s.payments.PaymentByOrderNo(ctx, orderNo)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return connect.NewResponse(&pb.GetActivePaymentStatusByOrderNoResponse{Code: codeSuccess, Message: successMessage}), nil
	}
	if err != nil {
		return nil, connectError(err)
	}
	status, err := paymentStatus(p.Status)
	if err != nil {
		return nil, connectError(apperr.FailedPrecondition(err.Error()))
	}
	return connect.NewResponse(&pb.GetActivePaymentStatusByOrderNoResponse{Code: codeSuccess, Message: successMessage, Data: paymentDetail(p, status)}), nil
}

func paymentDetail(p payment.OrderPayment, status string) *pb.PaymentDetail {
	d := &pb.PaymentDetail{
		TradeNo: p.Order.OrderNo, OrderId: p.Order.OrderNo, BuyerId: strconv.FormatInt(p.BuyerID, 10), Amount: strconv.FormatInt(p.Order.Amount, 10),
		Status: status, ProviderStatus: string(p.Status), ProviderMessage: p.LastError, CreatedAt: millis(p.CreatedAt),
	}
	if p.PaidAt != nil {
		d.PayTime = millis(*p.PaidAt)
	}
	return d
}

// ClosePaymentByOrderNo 在訂單取消或逾期時取消這張訂單未完成的 PaymentIntent（排 job，provider 在 transaction 外呼叫）；沒有需要關閉的付款回「無需關閉」。
func (s *Server) ClosePaymentByOrderNo(ctx context.Context, req *connect.Request[pb.ClosePaymentByOrderNoRequest]) (*connect.Response[pb.ClosePaymentByOrderNoResponse], error) {
	orderNo := strings.TrimSpace(req.Msg.GetOrderNo())
	if orderNo == "" {
		return connect.NewResponse(&pb.ClosePaymentByOrderNoResponse{Code: codeInvalidInput, Message: "order_no 必填"}), nil
	}
	queued, err := s.payments.CloseOrderPayment(ctx, orderNo)
	if err != nil {
		return nil, connectError(err)
	}
	msg := nothingToCloseMessage
	if queued {
		msg = closedMessage
	}
	return connect.NewResponse(&pb.ClosePaymentByOrderNoResponse{Code: codeSuccess, Message: msg}), nil
}

// GetPaymentFactsInternal 回發票需要的付款事實：只有已入帳而且有入帳時間的付款才有，查無或未入帳一律回非 0 業務碼（發票側 fail-loud，不能回 code 0 加空 data）。
// pay_time 與 provider_paid_at 都是 provider 的入帳時間（秒精度的 UTC）：入帳的付款一定有它，provider_paid_at 因此一律有值，不省略。
func (s *Server) GetPaymentFactsInternal(ctx context.Context, req *connect.Request[pb.GetPaymentFactsInternalRequest]) (*connect.Response[pb.GetPaymentFactsInternalResponse], error) {
	orderNo := strings.TrimSpace(req.Msg.GetOrderNo())
	if orderNo == "" {
		return connect.NewResponse(&pb.GetPaymentFactsInternalResponse{Code: codeInvalidInput, Message: "order_no 必填"}), nil
	}
	p, err := s.payments.PaymentByOrderNo(ctx, orderNo)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return connect.NewResponse(&pb.GetPaymentFactsInternalResponse{Code: codeTransactionNotFound, Message: "查無這張訂單的已付款交易"}), nil
	}
	if err != nil {
		return nil, connectError(err)
	}
	if p.Status != payment.PaymentSucceeded || p.PaidAt == nil {
		return connect.NewResponse(&pb.GetPaymentFactsInternalResponse{Code: codeTransactionNotFound, Message: "查無這張訂單的已付款交易"}), nil
	}
	paidAt := p.PaidAt.UTC().Format(isoSecondLayout)
	return connect.NewResponse(&pb.GetPaymentFactsInternalResponse{Code: codeSuccess, Message: successMessage, Data: &pb.PaymentFactsData{
		OrderNo: orderNo, TradeNo: orderNo, ProviderPaidAt: &paidAt, PayTime: paidAt, TradeAmt: strconv.FormatInt(p.Order.Amount, 10),
	}}), nil
}
