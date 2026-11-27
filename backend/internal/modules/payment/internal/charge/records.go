package charge

import (
	"context"
	"fmt"
	"slices"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

var maxCursor = uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

func pageWindow(size int32, after *model.RecordCursor, orderNo string) (int32, uuid.UUID, error) {
	size, err := pagination.PageSize(size, model.DefaultRecordPageSize, model.MaxRecordPageSize)
	if err != nil {
		return 0, uuid.UUID{}, err
	}
	if len(orderNo) > 64 {
		return 0, uuid.UUID{}, apperr.InvalidInputOn([]string{"OrderNo"}, "訂單編號長度不合")
	}
	start := maxCursor
	if after != nil {
		start = after.ID
	}
	return size, start, nil
}

func next(more bool, lastID uuid.UUID) *model.RecordCursor {
	if !more {
		return nil
	}
	return &model.RecordCursor{ID: lastID}
}

// ListPayments 是後台的付款清單（新的在前、keyset）；status 不是付款狀態回 InvalidInput，空字串不篩。
func (s *Service) ListPayments(ctx context.Context, f model.PaymentFilter, after *model.RecordCursor, size int32) (model.PaymentRecordPage, error) {
	if f.Status != "" && !slices.Contains(model.PaymentStatusValues(), f.Status) {
		return model.PaymentRecordPage{}, apperr.InvalidInputOn([]string{"Status"}, "付款狀態不合")
	}
	size, start, err := pageWindow(size, after, f.OrderNo)
	if err != nil {
		return model.PaymentRecordPage{}, err
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).AdminListPayments(ctx, store.AdminListPaymentsParams{Status: string(f.Status), OrderNo: f.OrderNo, AfterID: start, MaxRows: size + 1})
	if err != nil {
		return model.PaymentRecordPage{}, fmt.Errorf("charge：列出付款：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	orderRows, err := store.New(s.d.DB.Conn(ctx)).AdminPaymentOrders(ctx, ids)
	if err != nil {
		return model.PaymentRecordPage{}, fmt.Errorf("charge：列出付款的訂單應收：%w", err)
	}
	ordersOf := make(map[uuid.UUID][]store.PaymentPaymentOrder, len(rows))
	for _, o := range orderRows {
		ordersOf[o.PaymentID] = append(ordersOf[o.PaymentID], store.PaymentPaymentOrder{ID: o.ID, PaymentID: o.PaymentID, OrderNo: o.OrderNo, StoreID: o.StoreID, Amount: o.Amount})
	}
	out := model.PaymentRecordPage{Records: make([]model.PaymentRecord, len(rows))}
	for i, r := range rows {
		rec := model.PaymentRecord{Payment: toModel(r, ordersOf[r.ID], ""), UpdatedAt: r.UpdatedAt}
		if r.ExternalPaymentID != nil {
			rec.ExternalPaymentID = *r.ExternalPaymentID
		}
		if r.FailureReason != nil {
			rec.FailureReason = *r.FailureReason
		}
		out.Records[i] = rec
	}
	if len(rows) > 0 {
		out.Next = next(more, rows[len(rows)-1].ID)
	}
	return out, nil
}

// ListTransfers 是後台的轉帳清單（新的在前、keyset）；status 不是轉帳狀態回 InvalidInput，空字串不篩。
func (s *Service) ListTransfers(ctx context.Context, f model.TransferFilter, after *model.RecordCursor, size int32) (model.TransferRecordPage, error) {
	if f.Status != "" && !slices.Contains(model.TransferStatusValues(), f.Status) {
		return model.TransferRecordPage{}, apperr.InvalidInputOn([]string{"Status"}, "轉帳狀態不合")
	}
	size, start, err := pageWindow(size, after, f.OrderNo)
	if err != nil {
		return model.TransferRecordPage{}, err
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).AdminListTransfers(ctx, store.AdminListTransfersParams{Status: string(f.Status), OrderNo: f.OrderNo, AfterID: start, MaxRows: size + 1})
	if err != nil {
		return model.TransferRecordPage{}, fmt.Errorf("charge：列出轉帳：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.TransferRecordPage{Records: make([]model.TransferRecord, len(rows))}
	for i, r := range rows {
		rec := model.TransferRecord{
			Transfer: model.Transfer{
				ID: r.ID, PaymentOrderID: r.PaymentOrderID, StoreID: r.StoreID, Amount: r.Amount, ReversedAmount: r.ReversedAmount,
				Status: model.TransferStatus(r.Status), CreatedAt: r.CreatedAt,
			},
			PaymentID: r.PaymentID,
			OrderNo:   r.OrderNo,
		}
		if r.FailureReason != nil {
			rec.FailureReason = *r.FailureReason
		}
		if r.ExternalTransferID != nil {
			rec.ExternalTransferID = *r.ExternalTransferID
		}
		out.Records[i] = rec
	}
	if len(rows) > 0 {
		out.Next = next(more, rows[len(rows)-1].ID)
	}
	return out, nil
}

// ListRefunds 是後台的退款清單（新的在前、keyset），每筆附撤回列；status 不是退款狀態回 InvalidInput，空字串不篩。
func (s *Service) ListRefunds(ctx context.Context, f model.RefundFilter, after *model.RecordCursor, size int32) (model.RefundRecordPage, error) {
	if f.Status != "" && !slices.Contains(model.RefundStatusValues(), f.Status) {
		return model.RefundRecordPage{}, apperr.InvalidInputOn([]string{"Status"}, "退款狀態不合")
	}
	size, start, err := pageWindow(size, after, f.OrderNo)
	if err != nil {
		return model.RefundRecordPage{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.AdminListRefunds(ctx, store.AdminListRefundsParams{Status: string(f.Status), OrderNo: f.OrderNo, AnomalyOnly: f.AnomalyOnly, AfterID: start, MaxRows: size + 1})
	if err != nil {
		return model.RefundRecordPage{}, fmt.Errorf("charge：列出退款：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	revs, err := q.AdminRefundReversals(ctx, ids)
	if err != nil {
		return model.RefundRecordPage{}, fmt.Errorf("charge：列出退款的撤回列：%w", err)
	}
	byRefund := make(map[uuid.UUID][]model.ReversalRecord, len(rows))
	for _, v := range revs {
		rec := model.ReversalRecord{ID: v.ID, Seq: v.Seq, Amount: v.Amount, Status: model.ReversalStatus(v.Status), CreatedAt: v.CreatedAt}
		if v.Reason != nil {
			rec.Reason = *v.Reason
		}
		if v.ExternalID != nil {
			rec.ExternalID = *v.ExternalID
		}
		byRefund[v.RefundID] = append(byRefund[v.RefundID], rec)
	}
	out := model.RefundRecordPage{Records: make([]model.RefundRecord, len(rows))}
	for i, r := range rows {
		rec := model.RefundRecord{
			Refund: model.Refund{
				ID: r.ID, PaymentOrderID: r.PaymentOrderID, OrderNo: r.OrderNo, RequestKey: r.RequestKey, Amount: r.Amount, Auto: r.Auto, Status: model.RefundStatus(r.Status),
				ReversalAmount: r.ReversalAmount, ReversalStatus: model.ReversalStatus(r.ReversalStatus), TopupStatus: model.TopupStatus(r.TopupStatus), CreatedAt: r.CreatedAt,
			},
			PaymentID: r.PaymentID,
			OrderNo:   r.OrderNo,
			Reversals: byRefund[r.ID],
		}
		if r.FailureReason != nil {
			rec.FailureReason = *r.FailureReason
		}
		if r.ExternalRefundID != nil {
			rec.ExternalRefundID = *r.ExternalRefundID
		}
		out.Records[i] = rec
	}
	if len(rows) > 0 {
		out.Next = next(more, rows[len(rows)-1].ID)
	}
	return out, nil
}
