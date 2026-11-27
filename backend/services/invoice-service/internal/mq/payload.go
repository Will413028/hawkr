package mq

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type orderPaidPayload struct {
	EventID               string `json:"event_id"`
	OrderNo               string `json:"order_no"`
	OccurredAt            string `json:"occurred_at"`
	BuyerID               int64  `json:"buyer_id"`
	StoreID               int64  `json:"store_id"`
	NotifyBuyerAccountID  int64  `json:"notify_buyer_account_id"`
	TotalActualAmount     int64  `json:"total_actual_amount"`
	ShippingMethod        string `json:"shipping_method"`
	CorrelationIDMetadata string `json:"correlation_id"`
}

type orderRefundFinalizedPayload struct {
	EventID                string `json:"event_id"`
	OrderNo                string `json:"order_no"`
	OccurredAt             string `json:"occurred_at"`
	StoreID                string `json:"store_id"`
	BuyerID                string `json:"buyer_id"`
	RefundRef              string `json:"refund_ref"`
	RefundSource           string `json:"refund_source"`
	RefundOutcome          string `json:"refund_outcome"`
	RefundAmount           string `json:"refund_amount"`
	OrderTotalActualAmount string `json:"order_total_actual_amount"`
	CumulativeRefundAmount string `json:"cumulative_refund_amount"`
	CaseNo                 string `json:"case_no"`
	CancelID               string `json:"cancel_id"`
	PaymentMethod          string `json:"payment_method"`
	RefundedAt             string `json:"refunded_at"`
	CorrelationIDMetadata  string `json:"correlation_id"`
}

type orderCancelledPayload struct {
	EventID      string `json:"event_id"`
	OrderNo      string `json:"order_no"`
	OccurredAt   string `json:"occurred_at"`
	BuyerID      int64  `json:"buyer_id"`
	CancelReason string `json:"cancel_reason"`
}

func parseWireAmount(field, raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("invoice: %s is empty (wire 上必為十進位整數字串)", field)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invoice: %s %q is not an integer: %w", field, raw, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("invoice: %s %q is negative", field, raw)
	}
	return v, nil
}

func (p orderRefundFinalizedPayload) toRefundFinalized(occurredAt time.Time) (RefundFinalized, error) {
	refundAmount, err := parseWireAmount("refund_amount", p.RefundAmount)
	if err != nil {
		return RefundFinalized{}, err
	}
	orderTotal, err := parseWireAmount("order_total_actual_amount", p.OrderTotalActualAmount)
	if err != nil {
		return RefundFinalized{}, err
	}
	cumulative, err := parseWireAmount("cumulative_refund_amount", p.CumulativeRefundAmount)
	if err != nil {
		return RefundFinalized{}, err
	}
	refundedAt, err := parseEventTime(p.RefundedAt)
	if err != nil {
		return RefundFinalized{}, fmt.Errorf("refunded_at: %w", err)
	}
	return RefundFinalized{
		OrderNo:                p.OrderNo,
		RefundRef:              p.RefundRef,
		RefundSource:           p.RefundSource,
		RefundOutcome:          p.RefundOutcome,
		RefundAmount:           refundAmount,
		OrderTotalActualAmount: orderTotal,
		CumulativeRefundAmount: cumulative,
		CaseNo:                 p.CaseNo,
		CancelID:               p.CancelID,
		PaymentMethod:          p.PaymentMethod,
		RefundedAt:             refundedAt,
		OccurredAt:             occurredAt,
	}, nil
}

const (
	dedupPrefixOrderPaid = "ORDER_PAID:"
	dedupPrefixRefund    = "REFUND:"

	dedupPrefixOrderCancelled = "ORDER_CANCELLED:"

	dedupKeyMaxLen = 160
)

func orderPaidDedupKey(orderNo string) string { return dedupPrefixOrderPaid + orderNo }

func refundDedupKey(refundRef string) string { return dedupPrefixRefund + refundRef }

func orderCancelledDedupKey(orderNo string) string { return dedupPrefixOrderCancelled + orderNo }

func checkDedupKey(key string) error {
	if len(key) > dedupKeyMaxLen {
		return fmt.Errorf("invoice: business_dedup_key %q exceeds %d bytes", key, dedupKeyMaxLen)
	}
	return nil
}

var eventTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

func parseEventTime(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("invoice: occurred_at is empty")
	}
	for _, layout := range eventTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invoice: occurred_at %q matches none of %d known layouts", s, len(eventTimeLayouts))
}
