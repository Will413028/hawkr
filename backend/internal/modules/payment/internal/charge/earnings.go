package charge

import (
	"context"
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func earningsStatus(kind model.EarningsType, raw string) (model.EarningsStatus, error) {
	switch kind {
	case model.EarningsTransfer:
		switch model.TransferStatus(raw) {
		case model.TransferSucceeded:
			return model.EarningsCompleted, nil
		case model.TransferFailed:
			return model.EarningsFailed, nil
		case model.TransferPending, model.TransferAnomaly:
			return model.EarningsPending, nil
		}
	case model.EarningsReversal:
		switch model.ReversalStatus(raw) {
		case model.ReversalReversed:
			return model.EarningsCompleted, nil
		case model.ReversalReversing, model.ReversalAnomaly:
			return model.EarningsPending, nil
		case model.ReversalNone, model.ReversalWaiting, model.ReversalReleased:
		}
	case model.EarningsTopUp:
		switch model.TopupStatus(raw) {
		case model.TopupDone:
			return model.EarningsCompleted, nil
		case model.TopupWaiting, model.TopupSending, model.TopupAnomaly:
			return model.EarningsPending, nil
		case model.TopupNone, model.TopupReleased:
		}
	}
	return "", fmt.Errorf("charge：入帳帳本不認得的來源狀態 %s/%s", kind, raw)
}

// ListEarnings 是賣家入帳帳本（新的在前、keyset）：只列 storeID 這家店的訂單應收的轉帳、收回與補差額，沒有 provider 呼叫。
// OrderNo 長度不合與 From 不早於 To 回 InvalidInput；別家店的訂單編號沒有列，回空頁。
func (s *Service) ListEarnings(ctx context.Context, storeID int64, f model.EarningsFilter, after *model.RecordCursor, size int32) (model.EarningsPage, error) {
	if storeID <= 0 {
		return model.EarningsPage{}, apperr.InvalidInputOn([]string{"StoreID"}, "店家不合")
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return model.EarningsPage{}, apperr.InvalidInputOn([]string{"FromTime"}, "起始時間必須早於結束時間")
	}
	size, start, err := pageWindow(size, after, f.OrderNo)
	if err != nil {
		return model.EarningsPage{}, err
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).SellerListEarnings(ctx, store.SellerListEarningsParams{
		StoreID: storeID, OrderNo: f.OrderNo, FromTime: f.From, ToTime: f.To, AfterID: start, MaxRows: size + 1,
	})
	if err != nil {
		return model.EarningsPage{}, fmt.Errorf("charge：列出入帳帳本：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.EarningsPage{Entries: make([]model.EarningsEntry, len(rows))}
	for i, r := range rows {
		kind := model.EarningsType(r.Kind)
		status, err := earningsStatus(kind, r.Status)
		if err != nil {
			return model.EarningsPage{}, err
		}
		out.Entries[i] = model.EarningsEntry{ID: r.ID, Type: kind, OrderNo: r.OrderNo, Amount: r.Amount, Status: status, CreatedAt: r.CreatedAt}
	}
	if len(rows) > 0 {
		out.Next = next(more, rows[len(rows)-1].ID)
	}
	return out, nil
}

const orderEarningsPage = model.MaxRecordPageSize

// OrderEarnings 回傳 storeID 這家店一張訂單的全部入帳列（新的在前）與已完成列的淨額。訂單不是這家店的、或沒有訂單應收（沒有付款）回 NotFound，
// 兩者不分（不洩漏別家店的訂單存在與否）；有訂單應收但還沒有任何列時回空列與淨額 0。
func (s *Service) OrderEarnings(ctx context.Context, storeID int64, orderNo string) (model.OrderEarnings, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return model.OrderEarnings{}, apperr.InvalidInputOn([]string{"OrderNo"}, "訂單編號必填")
	}
	found, err := store.New(s.d.DB.Conn(ctx)).SellerHasOrderPayment(ctx, store.SellerHasOrderPaymentParams{StoreID: storeID, OrderNo: orderNo})
	if err != nil {
		return model.OrderEarnings{}, fmt.Errorf("charge：讀訂單應收：%w", err)
	}
	if !found {
		return model.OrderEarnings{}, apperr.NotFound("找不到這張訂單的入帳")
	}
	out := model.OrderEarnings{Entries: []model.EarningsEntry{}}
	var after *model.RecordCursor
	for {
		page, err := s.ListEarnings(ctx, storeID, model.EarningsFilter{OrderNo: orderNo}, after, orderEarningsPage)
		if err != nil {
			return model.OrderEarnings{}, err
		}
		for _, e := range page.Entries {
			if e.Status == model.EarningsCompleted {
				out.NetAmount += e.Amount
			}
		}
		out.Entries = append(out.Entries, page.Entries...)
		if page.Next == nil {
			return out, nil
		}
		after = page.Next
	}
}
