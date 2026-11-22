// Package crmconsumer 把舊 Java 服務的 order.completed 與 aftersales.refund.completed 事件接到 crm 模組的消費統計投影（過渡：ordering 搬進單體後退場）。
// RabbitMQ handler 只解碼與驗證，然後排一個 River job 並 ack；worker 呼叫 crm 模組（退款先在 tx 外向舊訂單服務解析店家與買家）。
// cmd/hawkr 的 worker 登記 job 並跑 Run（A6-6e 接上）；舊 crm-service 已刪除。
package crmconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Recorder 是 crm 模組的投影入口；正式組裝傳 *crm.Module。
type Recorder interface {
	RecordOrderCompleted(context.Context, crm.OrderCompleted) (bool, error)
	RecordRefund(context.Context, crm.Refund) (bool, error)
}

// Parties 解析訂單的買家與店家（退款事件沒有帶）；正式組裝傳 *orderservice.Client。訂單不存在回 orderservice.ErrOrderNotFound。
type Parties interface {
	OrderParties(ctx context.Context, orderNo string) (orderservice.OrderParties, error)
}

// Consumer 是 crm 的 legacy consumer。
type Consumer struct {
	CRM    Recorder
	Orders Parties
	Jobs   jobs.Enqueuer
}

type permanentError struct{ error }

func invalid(format string, args ...any) error { return permanentError{fmt.Errorf(format, args...)} }

// isPermanent 是兩個 handler 共用的分類：解碼失敗、必要欄位缺漏或格式不合都是 permanent（進 DLQ、不重試）。
func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// orderCompletedEvent 只列投影用到的欄位（Java 的 OrderCompletedPayload 其餘欄位忽略；ID 與金額是字串）。
type orderCompletedEvent struct {
	EventID           string `json:"event_id"`
	OrderNo           string `json:"order_no"`
	BuyerID           string `json:"buyer_id"`
	StoreID           string `json:"store_id"`
	TotalActualAmount string `json:"total_actual_amount"`
}

// OrderCompletedArgs 是 order.completed 的投影 job；冪等鍵 event_id（Java outbox 重試間不變）是 River 的唯一鍵，只減少重複排程，
// 正確性來自 crm.ledger 的 UNIQUE(dedup_key)。
type OrderCompletedArgs struct {
	EventID string `json:"event_id" river:"unique"`
	OrderNo string `json:"order_no"`
	BuyerID int64  `json:"buyer_id"`
	StoreID int64  `json:"store_id"`
	Amount  int64  `json:"amount"`
}

// Kind 是 River 的 job 種類。
func (OrderCompletedArgs) Kind() string { return "legacy.crm_order_completed" }

// InsertOpts 以 event_id 合併重複排程。
func (OrderCompletedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

func positiveID(field, v string) (int64, error) {
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, invalid("crmconsumer：%s 不是正整數", field)
	}
	return id, nil
}

// OrderCompleted 驗證 order.completed 並排入投影 job。
func (c Consumer) OrderCompleted(ctx context.Context, body []byte) error {
	var e orderCompletedEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return permanentError{fmt.Errorf("crmconsumer：order.completed payload：%w", err)}
	}
	if e.EventID == "" || len(e.EventID) > crm.MaxEventKeyLen {
		return invalid("crmconsumer：order.completed event_id 缺漏或過長")
	}
	if e.OrderNo == "" || utf8.RuneCountInString(e.OrderNo) > crm.MaxOrderNoLen {
		return invalid("crmconsumer：order.completed order_no 缺漏或超過 %d 個字元", crm.MaxOrderNoLen)
	}
	buyerID, err := positiveID("order.completed buyer_id", e.BuyerID)
	if err != nil {
		return err
	}
	storeID, err := positiveID("order.completed store_id", e.StoreID)
	if err != nil {
		return err
	}
	amount, err := strconv.ParseInt(e.TotalActualAmount, 10, 64)
	if err != nil || amount < 0 {
		return invalid("crmconsumer：order.completed total_actual_amount 不是非負整數")
	}
	args := OrderCompletedArgs{EventID: e.EventID, OrderNo: e.OrderNo, BuyerID: buyerID, StoreID: storeID, Amount: amount}
	opts := args.InsertOpts()
	return c.Jobs.Enqueue(ctx, args, &opts)
}

type orderCompletedWorker struct {
	river.WorkerDefaults[OrderCompletedArgs]
	crm Recorder
}

func (w *orderCompletedWorker) Work(ctx context.Context, job *river.Job[OrderCompletedArgs]) error {
	a := job.Args
	_, err := w.crm.RecordOrderCompleted(ctx, crm.OrderCompleted{EventID: a.EventID, OrderNo: a.OrderNo, StoreID: a.StoreID, BuyerID: a.BuyerID, Amount: a.Amount})
	return cancelIfHopeless(err)
}

// cancelIfHopeless 把重試不會變好的錯誤轉成 River 的取消（不再重試）；其他錯誤（DB、網路）照常重試。
func cancelIfHopeless(err error) error {
	if errors.Is(err, crm.ErrInvalidEvent) || errors.Is(err, crm.ErrPartiesMismatch) {
		return river.JobCancel(err)
	}
	return err
}

// refundCompletedEvent 只列投影用到的欄位（Java 的 RefundCompletedPayload）。
type refundCompletedEvent struct {
	EventID       string `json:"event_id"`
	CaseNo        string `json:"case_no"`
	OrderNo       string `json:"order_no"`
	RefundOutcome string `json:"refund_outcome"`
	RefundAmount  string `json:"refund_amount"`
	ExternalRefID string `json:"external_ref_id"`
}

// refundDedupKey 是退款事件的冪等鍵。event_id 不穩定：Java 的 RefundCompletedPayload 每次建構都產生新 UUID，RefundSyncWorker 每小時重發
// 已完成的退款，所以用 external_ref_id，缺時 "AS_REFUND_"+case_no（同 order-service 自己消費這個事件的鍵）；兩者都缺或超過長度上限都是 permanent。
func refundDedupKey(externalRefID, caseNo string) (string, error) {
	key := externalRefID
	if key == "" {
		if caseNo == "" {
			return "", invalid("crmconsumer：external_ref_id 與 case_no 都缺，無法導出穩定的冪等鍵")
		}
		key = "AS_REFUND_" + caseNo
	}
	if len(key) > crm.MaxEventKeyLen {
		return "", invalid("crmconsumer：冪等鍵超過 %d 個位元組", crm.MaxEventKeyLen)
	}
	return key, nil
}

// RefundCompletedArgs 是 aftersales.refund.completed 的投影 job；唯一鍵是退款的冪等鍵（不是 event_id）。
type RefundCompletedArgs struct {
	Key     string `json:"key" river:"unique"`
	OrderNo string `json:"order_no"`
	Outcome string `json:"outcome"`
	Amount  int64  `json:"amount"`
}

// Kind 是 River 的 job 種類。
func (RefundCompletedArgs) Kind() string { return "legacy.crm_refund_completed" }

// InsertOpts 以冪等鍵合併重複排程。
func (RefundCompletedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// RefundCompleted 驗證 aftersales.refund.completed 並排入投影 job。
func (c Consumer) RefundCompleted(ctx context.Context, body []byte) error {
	var e refundCompletedEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return permanentError{fmt.Errorf("crmconsumer：aftersales.refund.completed payload：%w", err)}
	}
	if e.EventID == "" {
		return invalid("crmconsumer：aftersales.refund.completed 缺 event_id")
	}
	if e.OrderNo == "" || utf8.RuneCountInString(e.OrderNo) > crm.MaxOrderNoLen {
		return invalid("crmconsumer：aftersales.refund.completed order_no 缺漏或超過 %d 個字元", crm.MaxOrderNoLen)
	}
	key, err := refundDedupKey(e.ExternalRefID, e.CaseNo)
	if err != nil {
		return err
	}
	var amount int64
	switch crm.RefundOutcome(e.RefundOutcome) {
	case crm.RefundPartial:
		amount, err = strconv.ParseInt(e.RefundAmount, 10, 64)
		if err != nil || amount <= 0 {
			return invalid("crmconsumer：PARTIAL_REFUND 的 refund_amount 不是正整數")
		}
	case crm.RefundFull:
		// 全額退款不信上游的 refund_amount：投影以自己記錄的淨額歸零，所以刻意不解析。
	default:
		return invalid("crmconsumer：未知的 refund_outcome %q", e.RefundOutcome)
	}
	args := RefundCompletedArgs{Key: key, OrderNo: e.OrderNo, Outcome: e.RefundOutcome, Amount: amount}
	opts := args.InsertOpts()
	return c.Jobs.Enqueue(ctx, args, &opts)
}

type refundCompletedWorker struct {
	river.WorkerDefaults[RefundCompletedArgs]
	crm    Recorder
	orders Parties
}

// Work 在 transaction 外向舊訂單服務解析訂單的店家與買家（CD10），再投影。訂單不存在是 permanent（取消）；其他錯誤重試。
func (w *refundCompletedWorker) Work(ctx context.Context, job *river.Job[RefundCompletedArgs]) error {
	a := job.Args
	p, err := w.orders.OrderParties(ctx, a.OrderNo)
	if errors.Is(err, orderservice.ErrOrderNotFound) {
		return river.JobCancel(err)
	}
	if err != nil {
		return err
	}
	_, err = w.crm.RecordRefund(ctx, crm.Refund{Key: a.Key, OrderNo: a.OrderNo, StoreID: p.StoreID, BuyerID: p.BuyerID, Outcome: crm.RefundOutcome(a.Outcome), Amount: a.Amount})
	return cancelIfHopeless(err)
}

// AddWorkers 登記兩個投影 worker。
func (c Consumer) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &orderCompletedWorker{crm: c.CRM})
	river.AddWorker(workers, &refundCompletedWorker{crm: c.CRM, orders: c.Orders})
}
