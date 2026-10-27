package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/mq"
)

// orderCompletedEvent matches order-service's OrderCompletedPayload.
// IDs are strings because Java publishes them as String fields.
type orderCompletedEvent struct {
	EventID           string `json:"event_id"`
	OrderNo           string `json:"order_no"`
	BuyerID           string `json:"buyer_id"`
	StoreID           string `json:"store_id"`
	TotalActualAmount string `json:"total_actual_amount"`
}

func orderCompletedHandler(uc *usecase.CrmUseCase) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event orderCompletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			pkglogger.Error(ctx, "[MQ] Failed to unmarshal order.completed event", zap.Error(err))
			return nil
		}

		// fail-loud：parse 失敗回傳 error 讓 ResilientConsumer 走重試 / DLQ，
		// 禁止吞成 0（amount 吞 0 = 買家消費金額統計靜默歸零）
		storeID, err := strconv.ParseInt(event.StoreID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse store_id %q: %w", event.StoreID, err)
		}
		buyerID, err := strconv.ParseInt(event.BuyerID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse buyer_id %q: %w", event.BuyerID, err)
		}
		amount, err := strconv.ParseInt(event.TotalActualAmount, 10, 64)
		if err != nil {
			return fmt.Errorf("parse total_actual_amount %q: %w", event.TotalActualAmount, err)
		}

		// fail-loud：order_no 是冪等去重鍵，缺了無法防止 MQ 重投重複累加，
		// 回 error 走重試 / DLQ，禁止靜默記帳（會破壞「不重複」不變式）。
		if event.OrderNo == "" {
			return fmt.Errorf("order.completed missing order_no (cannot dedup): seller=%d buyer=%d", storeID, buyerID)
		}

		// fail-loud：event_id 是 ledger 冪等鍵，缺了無法防止 MQ 重投重複累加，
		// 回 error 走重試 / DLQ，禁止靜默記帳。
		if event.EventID == "" {
			return fmt.Errorf("order.completed missing event_id (cannot dedup): order_no=%s", event.OrderNo)
		}

		if storeID == 0 || buyerID == 0 {
			pkglogger.Warn(ctx, "[MQ] order.completed missing IDs", zap.String("seller", event.StoreID), zap.String("buyer", event.BuyerID))
			return nil
		}

		recorded, err := uc.RecordOrder(ctx, event.EventID, event.OrderNo, storeID, buyerID, amount)
		if err != nil {
			return fmt.Errorf("record order seller=%d buyer=%d: %w", storeID, buyerID, err)
		}

		if recorded {
			pkglogger.Info(ctx, "[MQ] Recorded order.completed", zap.Int64("seller", storeID), zap.Int64("buyer", buyerID), zap.Int64("amount", amount), zap.String("orderNo", event.OrderNo))
		}
		return nil
	}
}

// refundCompletedEvent matches aftersales-service's RefundCompletedPayload.
type refundCompletedEvent struct {
	EventID       string `json:"event_id"`
	CaseNo        string `json:"case_no"`
	OrderNo       string `json:"order_no"`
	RefundOutcome string `json:"refund_outcome"`
	RefundAmount  string `json:"refund_amount"`
	ExternalRefID string `json:"external_ref_id"`
}

// refundDedupKey derives a stable idempotency key for a refund event.
// event_id is NOT stable for aftersales.refund.completed — aftersales-
// service's RefundSyncWorker (@Scheduled hourly) republishes any DONE
// refund task older than 1h indefinitely, with a fresh UUID event_id each
// time. Mirrors order-service's own consumer of this same event
// (AfterSalesRefundEventHandler.java), which dedups on financial_log's
// external_txn_id, falling back to "AS_REFUND_"+case_no — see spec §2.
//
// Unlike event_id (always a UUID, safely bounded), external_ref_id/case_no
// are externally-sourced strings of unaudited length — bounded here against
// crm_customer_ledger.event_id's VARCHAR(64) so a too-long value fails loud
// with a clear message instead of surfacing as an opaque DB constraint
// error several layers away.
const maxDedupKeyLen = 64

func refundDedupKey(externalRefID, caseNo string) (string, error) {
	key := externalRefID
	if key == "" {
		if caseNo == "" {
			return "", fmt.Errorf("missing both external_ref_id and case_no — cannot derive a stable dedup key")
		}
		key = "AS_REFUND_" + caseNo
	}
	if len(key) > maxDedupKeyLen {
		return "", fmt.Errorf("derived dedup key exceeds %d chars (len=%d): %q", maxDedupKeyLen, len(key), key)
	}
	return key, nil
}

const (
	reasonMalformedPayload = "malformed_payload"
	reasonMissingField     = "missing_required_field"
	reasonOrderNotFound    = "order_not_found"
)

func classifyRefundError(err error) error {
	if errors.Is(err, usecase.ErrOrderNotFound) {
		return mq.Permanent(reasonOrderNotFound, err)
	}
	return err
}

func refundCompletedHandler(uc *usecase.CrmUseCase) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event refundCompletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return mq.Permanent(reasonMalformedPayload, fmt.Errorf("unmarshal aftersales.refund.completed: %w", err))
		}

		// fail-loud：event_id 只做基本 payload 完整性檢查，本身「不」是 ledger
		// 冪等鍵（refundDedupKey 才是 —— aftersales-service 的 RefundSyncWorker
		// 每小時重發同一筆退款會換新的 event_id，見 refundDedupKey 註解）。
		if event.EventID == "" {
			return mq.Permanent(reasonMissingField, fmt.Errorf("aftersales.refund.completed missing event_id: order_no=%s", event.OrderNo))
		}
		if event.OrderNo == "" {
			return mq.Permanent(reasonMissingField, fmt.Errorf("aftersales.refund.completed missing order_no: event_id=%s", event.EventID))
		}

		dedupKey, err := refundDedupKey(event.ExternalRefID, event.CaseNo)
		if err != nil {
			return mq.Permanent(reasonMalformedPayload, fmt.Errorf("aftersales.refund.completed order_no=%s: %w", event.OrderNo, err))
		}

		var refundAmount int64
		switch event.RefundOutcome {
		case "PARTIAL_REFUND":
			amount, err := strconv.ParseInt(event.RefundAmount, 10, 64)
			if err != nil {
				return mq.Permanent(reasonMalformedPayload, fmt.Errorf("parse refund_amount %q for PARTIAL_REFUND order_no=%s: %w", event.RefundAmount, event.OrderNo, err))
			}
			refundAmount = amount
		case "FULL_REFUND":
			// refund_amount not trusted for FULL_REFUND — RecordRefund self-nets
			// from ledger history instead (see spec §2). Deliberately not parsed.
		default:
			return mq.Permanent(reasonMalformedPayload, fmt.Errorf("unknown refund_outcome %q for order_no=%s", event.RefundOutcome, event.OrderNo))
		}

		recorded, err := uc.RecordRefund(ctx, dedupKey, event.OrderNo, event.RefundOutcome, refundAmount)
		if err != nil {
			return classifyRefundError(fmt.Errorf("record refund order_no=%s: %w", event.OrderNo, err))
		}
		if recorded {
			pkglogger.Info(ctx, "[MQ] Recorded aftersales.refund.completed", zap.String("orderNo", event.OrderNo), zap.String("outcome", event.RefundOutcome))
		}
		return nil
	}
}

// buyerDeletedHandler 收 buyer.deleted，抹除該買家在所有賣家 CRM 的個資快取（#2847）。
//
// 用 mq.BindTypedHandler 而非自訂 struct：這是 Go 服務發的事件（pkg/mq.BuyerDeletedEvent），
// 型別就在共用套件裡，另外宣告一份 struct 等於製造第二份契約。
// （上面兩支 handler 用自訂 struct 是因為它們接的是 Java 服務發的事件，欄位是字串。）
func buyerDeletedHandler(uc *usecase.CrmUseCase) func(ctx context.Context, routingKey string, body []byte) error {
	return mq.BindTypedHandler(nil, func(ctx context.Context, routingKey string, payload *mq.BuyerDeletedEvent) error {
		if payload.BuyerID == 0 {
			// fail-loud：買家 id 缺失時不能靜默跳過——那會讓一筆該抹除的個資留著，
			// 而且沒有任何訊號。回 error 走重試／DLQ。
			return fmt.Errorf("buyer.deleted event missing buyerId (accountId=%d)", payload.AccountID)
		}

		rows, err := uc.RedactDeletedBuyerPII(ctx, payload.BuyerID)
		if err != nil {
			pkglogger.Error(ctx, "[MQ] crm PII redaction failed",
				zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}
		pkglogger.Info(ctx, "[MQ] crm PII redacted for deleted buyer",
			zap.Int64("buyer_id", payload.BuyerID), zap.Int64("rows", rows))
		return nil
	})
}
