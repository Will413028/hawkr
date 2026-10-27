// Package sellerrelay 將 seller 模組事件轉成尚未搬遷的 RabbitMQ 消費者所需訊息（遷移計畫 A3b 步驟 9c）。
//
// 每個轉送的模組事件對應一個 job（Kind 為 legacy.seller_<事件>）：OnX 在發布事件的 transaction 內排入，worker 在 transaction
// 外組出 payload（停權原因在這一刻才讀，不是事件當下的快照），再排入 rabbitmq.PublishArgs。一個模組事件只對應一個 routing
// key，所以 PublishArgs 以模組事件的 EventID（穩定的 UUIDv7）為唯一鍵不會衝突。wire 與舊 publisher（pkg/mq/events.go 與
// seller-service 的 merchant_verification_outbox）逐欄位相同——欄位名、JSON 型別、時間格式；差別只有 event_id：舊的生命週期
// 事件每次重試都換新的 UUID，這裡是穩定的。
//
// 不轉送的事件：StoreClosed 與 StoreReopened（關店可逆：下游的刪除動作等退場（第一段清理）的 StoreRetired；停權與可曝光的變化各有自己的事件）、StoreOpened（舊的 seller.onboarded 在 topology 沒有 queue、notification 也沒有模板）、StorePendingDeletion
// （等待刪除不另設 routing key，店家不可曝光由 seller.exposure.changed 涵蓋，ADR 2026-10-05-store-exposure-to-legacy 決策 4）、
// KYCSubmitted／KYCApproved／KYCCancelled／PayUniBindFailed（舊系統沒有對應的 routing key）、治理事件（A8 的 notification 訂閱）。
// review.created、review.hidden 的 consumer 單體不新增（seller 沒有 rating 欄位）。
//
// 隨 seller 的消費者（Java product、order、coupon，Go 的 im、social、pricebook、notification、cart）搬進單體而退場。
package sellerrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerreason"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	exchangeSeller   = "hawkr.seller"
	exchangeMerchant = "hawkr.merchant"
	source           = "seller-service"
)

type eventFields struct {
	EventID  string    `json:"event_id"`
	Occurred time.Time `json:"occurred"`
}

type storeFields struct {
	eventFields
	StoreID   int64 `json:"store_id"`
	AccountID int64 `json:"account_id"`
}

// StoreRetiredArgs 轉成 seller.deleted。關店可逆（終止合作的申覆期間內可以恢復），所以 StoreClosed 不轉送；退場（第一段清理）發的 StoreRetired
// 才是不可逆的點，下游（product、social、im、notification、pricebook）此時才刪除店家的資料。
type StoreRetiredArgs struct{ storeFields }

// StoreSuspendedArgs 轉成 seller.suspended；PeriodID 是 wire 的 suspensionId。
type StoreSuspendedArgs struct {
	storeFields
	PeriodID int64 `json:"period_id"`
}

// StoreUnsuspendedArgs 轉成 seller.unsuspended。
type StoreUnsuspendedArgs struct {
	storeFields
	PeriodID int64 `json:"period_id"`
}

// StoreProfileChangedArgs 轉成 seller.profile.changed。
type StoreProfileChangedArgs struct {
	eventFields
	StoreID int64 `json:"store_id"`
}

// StoreExposureChangedArgs 轉成 seller.exposure.changed。
type StoreExposureChangedArgs struct {
	eventFields
	StoreID   int64 `json:"store_id"`
	Exposable bool  `json:"exposable"`
	Version   int64 `json:"version"`
}

// PayUniForwardedArgs 轉成 merchant.verification.forwarded。
type PayUniForwardedArgs struct{ storeFields }

// PayUniApprovedArgs 轉成 merchant.verification.approved。
type PayUniApprovedArgs struct {
	storeFields
	Reason string `json:"reason"`
}

// PayUniPendingInfoArgs 轉成 merchant.verification.pending_info。
type PayUniPendingInfoArgs struct {
	storeFields
	Reason string `json:"reason"`
}

// PayUniUnderReviewArgs 轉成 merchant.verification.under_review。
type PayUniUnderReviewArgs struct{ storeFields }

// PayUniRejectedArgs 轉成 merchant.verification.rejected；StatusCode 是 PayUni 狀態在錯誤目錄的碼，沒有審核意見時取文案。
type PayUniRejectedArgs struct {
	storeFields
	Reason     string `json:"reason"`
	StatusCode string `json:"status_code"`
}

// KYCRejectedArgs 轉成 merchant.verification.rejected（管理員退回；Reason 是退回理由）。
type KYCRejectedArgs struct {
	storeFields
	Reason string `json:"reason"`
}

func (StoreRetiredArgs) Kind() string         { return "legacy.seller_store_retired" }
func (StoreSuspendedArgs) Kind() string       { return "legacy.seller_store_suspended" }
func (StoreUnsuspendedArgs) Kind() string     { return "legacy.seller_store_unsuspended" }
func (StoreProfileChangedArgs) Kind() string  { return "legacy.seller_store_profile_changed" }
func (StoreExposureChangedArgs) Kind() string { return "legacy.seller_store_exposure_changed" }
func (PayUniForwardedArgs) Kind() string      { return "legacy.seller_payuni_forwarded" }
func (PayUniApprovedArgs) Kind() string       { return "legacy.seller_payuni_approved" }
func (PayUniPendingInfoArgs) Kind() string    { return "legacy.seller_payuni_pending_info" }
func (PayUniUnderReviewArgs) Kind() string    { return "legacy.seller_payuni_under_review" }
func (PayUniRejectedArgs) Kind() string       { return "legacy.seller_payuni_rejected" }
func (KYCRejectedArgs) Kind() string          { return "legacy.seller_kyc_rejected" }

func fields(id fmt.Stringer, at time.Time) eventFields {
	return eventFields{EventID: id.String(), Occurred: at}
}

func store(f eventFields, storeID, accountID int64) storeFields {
	return storeFields{eventFields: f, StoreID: storeID, AccountID: accountID}
}

// OnStoreRetired 等 OnX 在發布事件的 transaction 內把事件轉成 job（transactional outbox）：排入失敗時發布的寫入一起回滾。
func OnStoreRetired(q jobs.Enqueuer) func(context.Context, seller.StoreRetired) error {
	return func(ctx context.Context, e seller.StoreRetired) error {
		return q.Enqueue(ctx, StoreRetiredArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID)}, nil)
	}
}

func OnStoreSuspended(q jobs.Enqueuer) func(context.Context, seller.StoreSuspended) error {
	return func(ctx context.Context, e seller.StoreSuspended) error {
		return q.Enqueue(ctx, StoreSuspendedArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID), e.PeriodID}, nil)
	}
}

func OnStoreUnsuspended(q jobs.Enqueuer) func(context.Context, seller.StoreUnsuspended) error {
	return func(ctx context.Context, e seller.StoreUnsuspended) error {
		return q.Enqueue(ctx, StoreUnsuspendedArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID), e.PeriodID}, nil)
	}
}

func OnStoreProfileChanged(q jobs.Enqueuer) func(context.Context, seller.StoreProfileChanged) error {
	return func(ctx context.Context, e seller.StoreProfileChanged) error {
		return q.Enqueue(ctx, StoreProfileChangedArgs{fields(e.EventID, e.OccurredAt), e.StoreID}, nil)
	}
}

func OnStoreExposureChanged(q jobs.Enqueuer) func(context.Context, seller.StoreExposureChanged) error {
	return func(ctx context.Context, e seller.StoreExposureChanged) error {
		return q.Enqueue(ctx, StoreExposureChangedArgs{fields(e.EventID, e.OccurredAt), e.StoreID, e.Exposable, e.Version}, nil)
	}
}

func OnPayUniForwarded(q jobs.Enqueuer) func(context.Context, seller.PayUniForwarded) error {
	return func(ctx context.Context, e seller.PayUniForwarded) error {
		return q.Enqueue(ctx, PayUniForwardedArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID)}, nil)
	}
}

func OnPayUniApproved(q jobs.Enqueuer) func(context.Context, seller.PayUniApproved) error {
	return func(ctx context.Context, e seller.PayUniApproved) error {
		return q.Enqueue(ctx, PayUniApprovedArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID), e.Reason}, nil)
	}
}

func OnPayUniPendingInfo(q jobs.Enqueuer) func(context.Context, seller.PayUniPendingInfo) error {
	return func(ctx context.Context, e seller.PayUniPendingInfo) error {
		return q.Enqueue(ctx, PayUniPendingInfoArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID), e.Reason}, nil)
	}
}

func OnPayUniUnderReview(q jobs.Enqueuer) func(context.Context, seller.PayUniUnderReview) error {
	return func(ctx context.Context, e seller.PayUniUnderReview) error {
		return q.Enqueue(ctx, PayUniUnderReviewArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID)}, nil)
	}
}

func OnPayUniRejected(q jobs.Enqueuer) func(context.Context, seller.PayUniRejected) error {
	return func(ctx context.Context, e seller.PayUniRejected) error {
		return q.Enqueue(ctx, PayUniRejectedArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID), e.Reason, e.StatusCode}, nil)
	}
}

func OnKYCRejected(q jobs.Enqueuer) func(context.Context, seller.KYCRejected) error {
	return func(ctx context.Context, e seller.KYCRejected) error {
		return q.Enqueue(ctx, KYCRejectedArgs{store(fields(e.EventID, e.OccurredAt), e.StoreID, e.AccountID), e.Reason}, nil)
	}
}

// Stores 是 relay 讀店家目前停權原因的 port；seller.Module 實作它。
type Stores interface {
	Store(context.Context, int64) (seller.Store, error)
}

// Worker 組出舊 publisher 的 payload 並排入送出的 job。
type Worker struct {
	Stores Stores
	Jobs   jobs.Enqueuer
	Log    *slog.Logger
}

// NewWorker 建立 Worker。
func NewWorker(stores Stores, queue jobs.Enqueuer, log *slog.Logger) *Worker {
	return &Worker{Stores: stores, Jobs: queue, Log: log}
}

type typedWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	work func(context.Context, T) error
}

func (w *typedWorker[T]) Work(ctx context.Context, job *river.Job[T]) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return w.work(ctx, job.Args)
}

// AddWorkers 登記全部 relay 的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker[StoreRetiredArgs]{work: w.storeRetired})
	river.AddWorker(workers, &typedWorker[StoreSuspendedArgs]{work: w.storeSuspended})
	river.AddWorker(workers, &typedWorker[StoreUnsuspendedArgs]{work: w.storeUnsuspended})
	river.AddWorker(workers, &typedWorker[StoreProfileChangedArgs]{work: w.storeProfileChanged})
	river.AddWorker(workers, &typedWorker[StoreExposureChangedArgs]{work: w.storeExposureChanged})
	river.AddWorker(workers, &typedWorker[PayUniForwardedArgs]{work: w.payUniForwarded})
	river.AddWorker(workers, &typedWorker[PayUniApprovedArgs]{work: w.payUniApproved})
	river.AddWorker(workers, &typedWorker[PayUniPendingInfoArgs]{work: w.payUniPendingInfo})
	river.AddWorker(workers, &typedWorker[PayUniUnderReviewArgs]{work: w.payUniUnderReview})
	river.AddWorker(workers, &typedWorker[PayUniRejectedArgs]{work: w.payUniRejected})
	river.AddWorker(workers, &typedWorker[KYCRejectedArgs]{work: w.kycRejected})
}

// baseEvent 是舊 pkg/mq 的 BaseEvent：timestamp 是 time.Time 的 JSON（RFC3339Nano、UTC），version 固定 "1.0"；
// correlation_id 與 metadata 在舊 publisher 是空值，omitempty 之下不出現，這裡不帶。
type baseEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

func base(f eventFields, eventType string) baseEvent {
	return baseEvent{EventID: f.EventID, EventType: eventType, Version: "1.0", Timestamp: f.Occurred.UTC(), Source: source}
}

// sellerDeleted 是舊 SellerDeletedEvent：id 是 JSON 數字。
type sellerDeleted struct {
	baseEvent
	AccountID int64 `json:"accountId"`
	StoreID   int64 `json:"store_id"`
}

// sellerSuspended 是舊 SellerSuspendedEvent：id 是 JSON 字串（,string），reasonCode 不省略。
type sellerSuspended struct {
	baseEvent
	SuspensionID int64  `json:"suspensionId,string"`
	AccountID    int64  `json:"accountId,string"`
	ReasonCode   string `json:"reasonCode"`
	ReasonLabel  string `json:"reasonLabel,omitempty"`
	StoreID      int64  `json:"store_id,string"`
}

// sellerUnsuspended 是舊 SellerUnsuspendedEvent。
type sellerUnsuspended struct {
	baseEvent
	SuspensionID int64 `json:"suspensionId,string"`
	AccountID    int64 `json:"accountId,string"`
	StoreID      int64 `json:"store_id,string"`
}

// sellerProfileChanged 是舊 SellerProfileChangedEvent：沒有 BaseEvent，occurred_at 是秒精度的 UTC（pkg/timeutil 的
// FormatISO8601，單體 import 不到 pkg，格式相同）。
type sellerProfileChanged struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	StoreID    string `json:"store_id"`
	OccurredAt string `json:"occurred_at"`
}

// sellerExposureChanged 是新的事件（沒有舊 publisher），形狀由 Java 派工依這裡實作。
type sellerExposureChanged struct {
	baseEvent
	StoreID         int64 `json:"store_id"`
	Exposable       bool  `json:"exposable"`
	ExposureVersion int64 `json:"exposure_version"`
}

// merchantVerification 是舊 MerchantVerificationCompletedEvent：沒有 BaseEvent，沒有 timestamp。
type merchantVerification struct {
	EventID    string `json:"event_id"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	MerchantID int64  `json:"merchant_id"`
	StoreID    int64  `json:"store_id"`
}

// enqueuePublish 不在 transaction 內排入（relay 要讀 seller）；relay job 在排入後失敗而重試時，PublishArgs 以 event_id 做的
// unique 讓同一則事件只有一個送出的 job。
func (w *Worker) enqueuePublish(ctx context.Context, eventID, exchange, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.Jobs.Enqueue(ctx, rabbitmq.PublishArgs{EventID: eventID, Exchange: exchange, RoutingKey: key, Body: body}, rabbitmq.InsertOpts())
}

func (w *Worker) storeRetired(ctx context.Context, a StoreRetiredArgs) error {
	return w.enqueuePublish(ctx, a.EventID, exchangeSeller, "seller.deleted", sellerDeleted{
		baseEvent: base(a.eventFields, "SELLER_DELETED"), AccountID: a.AccountID, StoreID: a.StoreID,
	})
}

// storeSuspended 在 relay 這一刻讀店家目前生效中的措施（事件不帶原因：持續停權中措施的新增或結束是同一段期間）：有依據風險類型的
// 停權措施時，用舊契約管理員停權的碼 9603 與風險類型的名稱；只有依據認定的停權，或 relay 時已沒有生效中的措施，reasonCode 是空字串、
// 不帶 reasonLabel（舊系統的 seller.suspended 只有管理員停權才發，違規停權在 sellerserver 的 suspended_governance 也沒有原因碼），
// 事件照樣送出。
func (w *Worker) storeSuspended(ctx context.Context, a StoreSuspendedArgs) error {
	st, err := w.Stores.Store(ctx, a.StoreID)
	if err != nil {
		return err
	}
	code, label, err := sellerreason.Of(st.Sanctions)
	if err != nil {
		return err
	}
	return w.enqueuePublish(ctx, a.EventID, exchangeSeller, "seller.suspended", sellerSuspended{
		baseEvent: base(a.eventFields, "SELLER_SUSPENDED"), SuspensionID: a.PeriodID, AccountID: a.AccountID,
		ReasonCode: code, ReasonLabel: label, StoreID: a.StoreID,
	})
}

func (w *Worker) storeUnsuspended(ctx context.Context, a StoreUnsuspendedArgs) error {
	return w.enqueuePublish(ctx, a.EventID, exchangeSeller, "seller.unsuspended", sellerUnsuspended{
		baseEvent: base(a.eventFields, "SELLER_UNSUSPENDED"), SuspensionID: a.PeriodID, AccountID: a.AccountID, StoreID: a.StoreID,
	})
}

func (w *Worker) storeProfileChanged(ctx context.Context, a StoreProfileChangedArgs) error {
	return w.enqueuePublish(ctx, a.EventID, exchangeSeller, "seller.profile.changed", sellerProfileChanged{
		EventID: a.EventID, EventType: "SELLER_PROFILE_CHANGED", StoreID: strconv.FormatInt(a.StoreID, 10),
		OccurredAt: a.Occurred.UTC().Format(time.RFC3339),
	})
}

func (w *Worker) storeExposureChanged(ctx context.Context, a StoreExposureChangedArgs) error {
	return w.enqueuePublish(ctx, a.EventID, exchangeSeller, "seller.exposure.changed", sellerExposureChanged{
		baseEvent: base(a.eventFields, "SELLER_EXPOSURE_CHANGED"), StoreID: a.StoreID, Exposable: a.Exposable, ExposureVersion: a.Version,
	})
}

func (w *Worker) merchant(ctx context.Context, a storeFields, key, status, reason string) error {
	return w.enqueuePublish(ctx, a.EventID, exchangeMerchant, "merchant.verification."+key, merchantVerification{
		EventID: a.EventID, Status: status, Reason: reason, MerchantID: a.AccountID, StoreID: a.StoreID,
	})
}

// PayUni 綁定成功對應舊的 forwarded：status 是 forwarded_to_payuni（CARRYOVER 的 PayUni 事件對應）。
func (w *Worker) payUniForwarded(ctx context.Context, a PayUniForwardedArgs) error {
	return w.merchant(ctx, a.storeFields, "forwarded", "forwarded_to_payuni", "")
}

func (w *Worker) payUniApproved(ctx context.Context, a PayUniApprovedArgs) error {
	return w.merchant(ctx, a.storeFields, "approved", "approved", a.Reason)
}

func (w *Worker) payUniPendingInfo(ctx context.Context, a PayUniPendingInfoArgs) error {
	return w.merchant(ctx, a.storeFields, "pending_info", "pending_info", pendingInfoReason(a.Reason))
}

func (w *Worker) payUniUnderReview(ctx context.Context, a PayUniUnderReviewArgs) error {
	return w.merchant(ctx, a.storeFields, "under_review", "under_review", "")
}

func (w *Worker) payUniRejected(ctx context.Context, a PayUniRejectedArgs) error {
	return w.merchant(ctx, a.storeFields, "rejected", "rejected", rejectedReason(a.StatusCode, a.Reason))
}

func (w *Worker) kycRejected(ctx context.Context, a KYCRejectedArgs) error {
	return w.merchant(ctx, a.storeFields, "rejected", "rejected", a.Reason)
}

// pendingInfoReason 是舊 payuniPendingInfoReason：有審核意見用它（原樣），沒有時是 MEMBER_5 的文案。
func pendingInfoReason(reason string) string {
	if strings.TrimSpace(reason) != "" {
		return reason
	}
	return payuniNotices["MEMBER_5"]
}

// rejectedReason 是舊 payuniRejectNotificationReason＝ResolvePayuniError(code, reason).Message：沒有狀態碼時沒有文案；
// 審核意見（去掉前後空白）優先；沒有審核意見時依狀態碼取文案，錄在目錄裡的碼取它的文案，目錄沒有的碼用 fail-closed 的
// 通用文案，不顯示原始碼。
func rejectedReason(code, reason string) string {
	if code == "" {
		return ""
	}
	if message := strings.TrimSpace(reason); message != "" {
		return message
	}
	if notice, known := payuniNotices[code]; known {
		return notice
	}
	return payuniNotices[payuniUnknown]
}
