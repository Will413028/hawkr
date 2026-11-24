package mq

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/google/uuid"
)

type EventType string

const (
	EventTypeOrderCreated   EventType = "ORDER_CREATED"
	EventTypeOrderPaid      EventType = "ORDER_PAID"
	EventTypeOrderShipped   EventType = "ORDER_SHIPPED"
	EventTypeOrderCompleted EventType = "ORDER_COMPLETED"
	EventTypeOrderCancelled EventType = "ORDER_CANCELLED"

	EventTypeOrderRefundFinalized EventType = "ORDER_REFUND_FINALIZED"

	EventTypePaymentSuccess EventType = "PAYMENT_SUCCESS"
	EventTypePaymentFailed  EventType = "PAYMENT_FAILED"
	EventTypePaymentExpired EventType = "PAYMENT_EXPIRED"

	EventTypeShipmentCreated   EventType = "SHIPMENT_CREATED"
	EventTypeShipmentDelivered EventType = "SHIPMENT_DELIVERED"

	EventTypeReviewCreated EventType = "REVIEW_CREATED"
	EventTypeReviewHidden  EventType = "REVIEW_HIDDEN"

	EventTypeCouponIssued EventType = "COUPON_ISSUED"

	EventTypeAccountDeleted EventType = "ACCOUNT_DELETED"

	EventTypeSellerDeleted        EventType = "SELLER_DELETED"
	EventTypeSellerOnboarded      EventType = "SELLER_ONBOARDED"
	EventTypeSellerProfileChanged EventType = "SELLER_PROFILE_CHANGED"
	EventTypeSellerSuspended      EventType = "SELLER_SUSPENDED"
	EventTypeSellerUnsuspended    EventType = "SELLER_UNSUSPENDED"

	EventTypeBuyerDeleted EventType = "BUYER_DELETED"

	EventTypeInvoiceIssued                       EventType = "INVOICE_ISSUED"
	EventTypeInvoiceIssueFailed                  EventType = "INVOICE_ISSUE_FAILED"
	EventTypeInvoiceVoided                       EventType = "INVOICE_VOIDED"
	EventTypeInvoiceAllowanceIssued              EventType = "INVOICE_ALLOWANCE_ISSUED"
	EventTypeInvoiceReissued                     EventType = "INVOICE_REISSUED"
	EventTypeInvoiceReceiptIssued                EventType = "INVOICE_RECEIPT_ISSUED"
	EventTypeInvoiceOperationReviewRequired      EventType = "INVOICE_OPERATION_REVIEW_REQUIRED"
	EventTypeInvoiceComplaintCreated             EventType = "INVOICE_COMPLAINT_CREATED"
	EventTypeInvoiceComplaintResponded           EventType = "INVOICE_COMPLAINT_RESPONDED"
	EventTypeInvoiceComplaintOverdue             EventType = "INVOICE_COMPLAINT_OVERDUE"
	EventTypeInvoiceSellerTaxProfileStatusChange EventType = "INVOICE_SELLER_TAX_PROFILE_STATUS_CHANGED"
	EventTypeInvoiceResolvedExternally           EventType = "INVOICE_RESOLVED_EXTERNALLY"
)

type BaseEvent struct {
	EventID       string            `json:"event_id"`
	EventType     EventType         `json:"event_type"`
	Version       string            `json:"version"`
	Timestamp     time.Time         `json:"timestamp"`
	Source        string            `json:"source"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func NewBaseEvent(eventType EventType, source string) BaseEvent {
	return BaseEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Version:   "1.0",
		Timestamp: time.Now().UTC(),
		Source:    source,
		Metadata:  make(map[string]string),
	}
}

type OrderItem struct {
	SkuID     int64   `json:"skuId"`
	ProductID int64   `json:"productId,omitempty"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type ReviewCreatedEvent struct {
	BaseEvent
	ReviewID           int64    `json:"review_id"`
	OrderNo            string   `json:"order_no"`
	OrderItemID        int64    `json:"order_item_id"`
	StoreID            int64    `json:"store_id"`
	BuyerID            int64    `json:"buyer_id"`
	Rating             int32    `json:"rating"`
	Content            string   `json:"content,omitempty"`
	ImageKeys          []string `json:"image_keys,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
	AvgRating          float64  `json:"avg_rating"`
	TotalCount         int64    `json:"total_count"`
	ProductID          int64    `json:"product_id"`
	ProductAvgRating   float64  `json:"product_avg_rating"`
	ProductReviewCount int64    `json:"product_review_count"`
}

func NewReviewCreatedEvent(reviewID int64, orderNo string, orderItemID int64, storeID, buyerID int64, rating int32, content string, imageKeys []string, createdAt string, avgRating float64, totalCount int64, productID int64, productAvgRating float64, productReviewCount int64) *ReviewCreatedEvent {
	return &ReviewCreatedEvent{
		BaseEvent:          NewBaseEvent(EventTypeReviewCreated, "review-service"),
		ReviewID:           reviewID,
		OrderNo:            orderNo,
		OrderItemID:        orderItemID,
		StoreID:            storeID,
		BuyerID:            buyerID,
		Rating:             rating,
		Content:            content,
		ImageKeys:          imageKeys,
		CreatedAt:          createdAt,
		AvgRating:          avgRating,
		TotalCount:         totalCount,
		ProductID:          productID,
		ProductAvgRating:   productAvgRating,
		ProductReviewCount: productReviewCount,
	}
}

type ReviewHiddenEvent struct {
	BaseEvent
	ReviewID           int64   `json:"review_id"`
	StoreID            int64   `json:"store_id"`
	ProductID          int64   `json:"product_id"`
	AvgRating          float64 `json:"avg_rating"`
	TotalCount         int64   `json:"total_count"`
	ProductAvgRating   float64 `json:"product_avg_rating"`
	ProductReviewCount int64   `json:"product_review_count"`
}

func NewReviewHiddenEvent(reviewID, storeID, productID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64) *ReviewHiddenEvent {
	return &ReviewHiddenEvent{
		BaseEvent:          NewBaseEvent(EventTypeReviewHidden, "review-service"),
		ReviewID:           reviewID,
		StoreID:            storeID,
		ProductID:          productID,
		AvgRating:          avgRating,
		TotalCount:         totalCount,
		ProductAvgRating:   productAvgRating,
		ProductReviewCount: productReviewCount,
	}
}

type ProductViewedEvent struct {
	BaseEvent
	SellerID    int64  `json:"seller_id"`
	ProductName string `json:"product_name"`
	ProductID   int64  `json:"product_id"`
	StoreID     int64  `json:"store_id"`
}

type OrderCompletedEvent struct {
	BaseEvent
	BuyerID              string `json:"buyer_id"`
	NotifyBuyerAccountID string `json:"notify_buyer_account_id"`
	OrderNo              string `json:"order_no"`
	StoreID              string `json:"store_id"`
	CompletedAt          string `json:"completed_at"`
}

func NewOrderCompletedEvent(buyerID, orderNo string) *OrderCompletedEvent {
	return &OrderCompletedEvent{
		BaseEvent: NewBaseEvent(EventTypeOrderCompleted, "order-service"),
		BuyerID:   buyerID,
		OrderNo:   orderNo,
	}
}

type AccountDeletedEvent struct {
	BaseEvent
	AccountID int64 `json:"accountId"`
}

type SellerDeletedEvent struct {
	BaseEvent
	AccountID int64 `json:"accountId"`
	StoreID   int64 `json:"store_id"`
}

type BuyerDeletedEvent struct {
	BaseEvent
	AccountID int64 `json:"accountId"`
	BuyerID   int64 `json:"buyerId"`
}

type SellerOnboardedEvent struct {
	BaseEvent
	AccountID int64 `json:"accountId"`
	StoreID   int64 `json:"store_id"`
}

type SellerProfileChangedEvent struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	StoreID    string `json:"store_id"`
	OccurredAt string `json:"occurred_at"`
}

func NewSellerProfileChangedEvent(sellerAccountID int64) *SellerProfileChangedEvent {
	return &SellerProfileChangedEvent{
		EventID:    uuid.NewString(),
		EventType:  string(EventTypeSellerProfileChanged),
		StoreID:    strconv.FormatInt(sellerAccountID, 10),
		OccurredAt: timeutil.FormatISO8601(time.Now()),
	}
}

type SellerSuspendedEvent struct {
	BaseEvent
	SuspensionID int64  `json:"suspensionId,string"`
	AccountID    int64  `json:"accountId,string"`
	ReasonCode   string `json:"reasonCode"`
	ReasonLabel  string `json:"reasonLabel,omitempty"`
	StoreID      int64  `json:"store_id,string"`
}

type SellerUnsuspendedEvent struct {
	BaseEvent
	SuspensionID int64 `json:"suspensionId,string"`
	AccountID    int64 `json:"accountId,string"`
	StoreID      int64 `json:"store_id,string"`
}

type SellerOrderUpdatedEvent struct {
	EventID        string `json:"eventId"`
	SellerID       int64  `json:"sellerId"`
	OrderNo        string `json:"orderNo"`
	Source         string `json:"source"`
	PreviousStatus string `json:"previousStatus,omitempty"`
	Status         string `json:"status,omitempty"`
	ShipmentStatus string `json:"shipmentStatus,omitempty"`
	ChangedAt      string `json:"changedAt"`
}

type SellerAfterSaleUpdatedEvent struct {
	EventID         string `json:"eventId"`
	SellerID        int64  `json:"sellerId"`
	OrderNo         string `json:"orderNo"`
	AfterSaleCaseNo string `json:"afterSaleCaseNo"`
	Source          string `json:"source"`
	Status          string `json:"status,omitempty"`
	ChangedAt       string `json:"changedAt"`
}

type DelegationOperationCompletedEvent struct {
	BaseEvent
	AuditLogID          string `json:"audit_log_id"`
	SessionID           string `json:"session_id"`
	SessionDisplayCode  string `json:"session_display_code"`
	ActorAdminID        string `json:"actor_admin_id"`
	ActorDisplayName    string `json:"actor_display_name"`
	TargetStoreID       string `json:"target_store_id"`
	TargetSellerAccount string `json:"target_seller_account_id"`
	Capability          string `json:"capability"`
	Action              string `json:"action"`
	ActionLabel         string `json:"action_label"`
	RiskLevel           string `json:"risk_level"`
	ResourceType        string `json:"resource_type"`
	ResourceID          string `json:"resource_id"`
	ResourceName        string `json:"resource_name,omitempty"`
	OperationReason     string `json:"operation_reason,omitempty"`
	CaseReference       string `json:"case_reference,omitempty"`
	OccurredAt          string `json:"occurred_at"`
}

func ToJSON(event interface{}) ([]byte, error) {
	return json.Marshal(event)
}

func FromJSON[T any](data []byte) (*T, error) {
	var event T
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
