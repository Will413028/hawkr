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

	EventTypePostCreated          EventType = "POST_CREATED"
	EventTypePostLiked            EventType = "POST_LIKED"
	EventTypePostHiddenByPlatform EventType = "POST_HIDDEN_BY_PLATFORM"
	EventTypeCommentCreated       EventType = "COMMENT_CREATED"
	EventTypeCommentReplied       EventType = "COMMENT_REPLIED"
	EventTypeCommentUpdated       EventType = "COMMENT_UPDATED"
	EventTypeCommentDeleted       EventType = "COMMENT_DELETED"
	EventTypeCommentLikeChanged   EventType = "COMMENT_LIKE_CHANGED"
	EventTypeCounterDelta         EventType = "COUNTER_DELTA"

	EventTypeNotificationInboxNew      EventType = "NOTIFICATION_INBOX_NEW"
	EventTypeNotificationUnreadChanged EventType = "NOTIFICATION_UNREAD_CHANGED"
	EventTypeCouponIssued              EventType = "COUPON_ISSUED"

	EventTypeAccountDeleted EventType = "ACCOUNT_DELETED"

	EventTypeSellerDeleted           EventType = "SELLER_DELETED"
	EventTypeSellerOnboarded         EventType = "SELLER_ONBOARDED"
	EventTypeSellerProfileChanged    EventType = "SELLER_PROFILE_CHANGED"
	EventTypeSellerMembershipChanged EventType = "SELLER_MEMBERSHIP_CHANGED"
	EventTypeSellerSuspended         EventType = "SELLER_SUSPENDED"
	EventTypeSellerUnsuspended       EventType = "SELLER_UNSUSPENDED"

	EventTypeBuyerDeleted EventType = "BUYER_DELETED"

	EventTypeDelegationOperationCompleted EventType = "DELEGATION_OPERATION_COMPLETED"

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
	ReviewID              int64    `json:"review_id"`
	OrderNo               string   `json:"order_no"`
	OrderItemID           int64    `json:"order_item_id"`
	StoreID               int64    `json:"store_id"`
	NotifySellerAccountID int64    `json:"notify_seller_account_id"`
	BuyerID               int64    `json:"buyer_id"`
	Rating                int32    `json:"rating"`
	Content               string   `json:"content,omitempty"`
	ImageKeys             []string `json:"image_keys,omitempty"`
	CreatedAt             string   `json:"created_at,omitempty"`
	AvgRating             float64  `json:"avg_rating"`
	TotalCount            int64    `json:"total_count"`
	ProductID             int64    `json:"product_id"`
	ProductAvgRating      float64  `json:"product_avg_rating"`
	ProductReviewCount    int64    `json:"product_review_count"`
	BuyerNickname         string   `json:"buyer_nickname,omitempty"`
	BuyerAvatarKey        string   `json:"buyer_avatar_key,omitempty"`
}

func NewReviewCreatedEvent(reviewID int64, orderNo string, orderItemID int64, storeID, notifySellerAccountID, buyerID int64, rating int32, content string, imageKeys []string, createdAt string, avgRating float64, totalCount int64, productID int64, productAvgRating float64, productReviewCount int64, buyerNickname, buyerAvatar string) *ReviewCreatedEvent {
	return &ReviewCreatedEvent{
		BaseEvent:             NewBaseEvent(EventTypeReviewCreated, "review-service"),
		ReviewID:              reviewID,
		OrderNo:               orderNo,
		OrderItemID:           orderItemID,
		StoreID:               storeID,
		NotifySellerAccountID: notifySellerAccountID,
		BuyerID:               buyerID,
		Rating:                rating,
		Content:               content,
		ImageKeys:             imageKeys,
		CreatedAt:             createdAt,
		AvgRating:             avgRating,
		TotalCount:            totalCount,
		ProductID:             productID,
		ProductAvgRating:      productAvgRating,
		ProductReviewCount:    productReviewCount,
		BuyerNickname:         buyerNickname,
		BuyerAvatarKey:        buyerAvatar,
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

type PostCreatedEvent struct {
	BaseEvent
	PostID          int64  `json:"post_id"`
	StoreID         int64  `json:"store_id"`
	FollowerID      int64  `json:"follower_id"`
	SellerName      string `json:"seller_name"`
	SellerAvatarKey string `json:"seller_avatar_key"`
	ContentPreview  string `json:"content_preview"`
}

func NewPostCreatedEvent(postID, storeID, followerID int64, sellerName, sellerAvatar, contentPreview string) *PostCreatedEvent {
	return &PostCreatedEvent{
		BaseEvent:       NewBaseEvent(EventTypePostCreated, "social-service"),
		PostID:          postID,
		StoreID:         storeID,
		FollowerID:      followerID,
		SellerName:      sellerName,
		SellerAvatarKey: sellerAvatar,
		ContentPreview:  contentPreview,
	}
}

type CommentSnapshot struct {
	CommentID   int64  `json:"commentId,string"`
	PostID      int64  `json:"postId,string"`
	UserID      int64  `json:"userId,string"`
	UserName    string `json:"userName"`
	AvatarKey   string `json:"avatarKey"`
	Role        int32  `json:"role"`
	Content     string `json:"content"`
	ParentID    int64  `json:"parentId,string"`
	ReplyToID   int64  `json:"replyToId,string"`
	ReplyToName string `json:"replyToName"`
	LikeCount   int32  `json:"likeCount"`
	ReplyCount  int32  `json:"replyCount"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type CommentCreatedEvent struct {
	BaseEvent
	CommentID            int64            `json:"commentId"`
	PostID               int64            `json:"postId"`
	CommenterID          int64            `json:"commenterId"`
	CommenterPrincipalID int64            `json:"commenterPrincipalId"`
	CommenterName        string           `json:"commenterName"`
	PostAuthorID         int64            `json:"postAuthorId"`
	PostAuthorStoreID    int64            `json:"postAuthorStoreId"`
	PostAuthorType       string           `json:"postAuthorType"`
	ContentPreview       string           `json:"contentPreview"`
	ParentCommentID      int64            `json:"parentCommentId,omitempty"`
	ClientRequestID      string           `json:"clientRequestId,omitempty"`
	Comment              *CommentSnapshot `json:"comment,omitempty"`
}

func NewCommentCreatedEvent(
	commentID, postID, commenterID, commenterPrincipalID int64,
	commenterName string,
	postAuthorID, postAuthorStoreID int64,
	postAuthorType, contentPreview string,
	parentCommentID int64,
	clientRequestID string,
	snapshot *CommentSnapshot,
) *CommentCreatedEvent {
	return &CommentCreatedEvent{
		BaseEvent:            NewBaseEvent(EventTypeCommentCreated, "social-service"),
		CommentID:            commentID,
		PostID:               postID,
		CommenterID:          commenterID,
		CommenterPrincipalID: commenterPrincipalID,
		CommenterName:        commenterName,
		PostAuthorID:         postAuthorID,
		PostAuthorStoreID:    postAuthorStoreID,
		PostAuthorType:       postAuthorType,
		ContentPreview:       contentPreview,
		ParentCommentID:      parentCommentID,
		ClientRequestID:      clientRequestID,
		Comment:              snapshot,
	}
}

type PostLikedEvent struct {
	BaseEvent
	SellerID  int64  `json:"seller_id"`
	StoreID   int64  `json:"store_id"`
	LikerName string `json:"likerName"`
	PostID    int64  `json:"post_id"`
}

func NewPostLikedEvent(sellerID, postID, storeID int64, likerName string) *PostLikedEvent {
	return &PostLikedEvent{
		BaseEvent: NewBaseEvent(EventTypePostLiked, "social-service"),
		SellerID:  sellerID,
		StoreID:   storeID,
		LikerName: likerName,
		PostID:    postID,
	}
}

type CommentRepliedEvent struct {
	BaseEvent
	ReplyToUserID      int64            `json:"replyToUserId"`
	ReplyToUserStoreID int64            `json:"replyToUserStoreId"`
	ReplyToUserType    string           `json:"replyToUserType"`
	ReplierID          int64            `json:"replierId"`
	ReplierPrincipalID int64            `json:"replierPrincipalId"`
	ReplierName        string           `json:"replierName"`
	PostID             int64            `json:"postId"`
	CommentID          int64            `json:"commentId"`
	ContentPreview     string           `json:"contentPreview"`
	ParentCommentID    int64            `json:"parentCommentId,omitempty"`
	ClientRequestID    string           `json:"clientRequestId,omitempty"`
	Comment            *CommentSnapshot `json:"comment,omitempty"`
}

func NewCommentRepliedEvent(
	replyToUserID, replierID, replierPrincipalID, replyToUserStoreID int64, replyToUserType, replierName string,
	postID, commentID int64, contentPreview string,
	parentCommentID int64,
	clientRequestID string,
	snapshot *CommentSnapshot,
) *CommentRepliedEvent {
	return &CommentRepliedEvent{
		BaseEvent:          NewBaseEvent(EventTypeCommentReplied, "social-service"),
		ReplyToUserID:      replyToUserID,
		ReplyToUserStoreID: replyToUserStoreID,
		ReplyToUserType:    replyToUserType,
		ReplierID:          replierID,
		ReplierPrincipalID: replierPrincipalID,
		ReplierName:        replierName,
		PostID:             postID,
		CommentID:          commentID,
		ContentPreview:     contentPreview,
		ParentCommentID:    parentCommentID,
		ClientRequestID:    clientRequestID,
		Comment:            snapshot,
	}
}

type CommentUpdatedEvent struct {
	BaseEvent
	CommentID int64  `json:"commentId"`
	PostID    int64  `json:"postId"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updatedAt"`
}

func NewCommentUpdatedEvent(commentID, postID int64, content, updatedAt string) *CommentUpdatedEvent {
	return &CommentUpdatedEvent{
		BaseEvent: NewBaseEvent(EventTypeCommentUpdated, "social-service"),
		CommentID: commentID,
		PostID:    postID,
		Content:   content,
		UpdatedAt: updatedAt,
	}
}

type CommentDeletedEvent struct {
	BaseEvent
	CommentID       int64 `json:"commentId"`
	PostID          int64 `json:"postId"`
	ParentCommentID int64 `json:"parentCommentId"`
}

func NewCommentDeletedEvent(commentID, postID, parentCommentID int64) *CommentDeletedEvent {
	return &CommentDeletedEvent{
		BaseEvent:       NewBaseEvent(EventTypeCommentDeleted, "social-service"),
		CommentID:       commentID,
		PostID:          postID,
		ParentCommentID: parentCommentID,
	}
}

type CommentLikeChangedEvent struct {
	BaseEvent
	CommentID            int64  `json:"commentId"`
	PostID               int64  `json:"postId"`
	LikeCount            int32  `json:"likeCount"`
	Action               string `json:"action"`
	UserID               int64  `json:"userId"`
	LikerKind            int32  `json:"likerKind"`
	CommentAuthorID      int64  `json:"commentAuthorId"`
	CommentAuthorStoreID int64  `json:"commentAuthorStoreId"`
	LikerPrincipalID     int64  `json:"likerPrincipalId"`
	LikerName            string `json:"likerName"`
	CommentAuthorType    string `json:"commentAuthorType"`
}

func NewCommentLikeChangedEvent(
	commentID, postID int64, likeCount int32, action string,
	userID int64, likerKind int32, commentAuthorID, commentAuthorStoreID, likerPrincipalID int64, likerName, commentAuthorType string,
) *CommentLikeChangedEvent {
	return &CommentLikeChangedEvent{
		BaseEvent:            NewBaseEvent(EventTypeCommentLikeChanged, "social-service"),
		CommentID:            commentID,
		PostID:               postID,
		LikeCount:            likeCount,
		Action:               action,
		UserID:               userID,
		LikerKind:            likerKind,
		CommentAuthorID:      commentAuthorID,
		CommentAuthorStoreID: commentAuthorStoreID,
		LikerPrincipalID:     likerPrincipalID,
		LikerName:            likerName,
		CommentAuthorType:    commentAuthorType,
	}
}

type CounterDeltaEvent struct {
	BaseEvent
	EntityType string `json:"entityType"`
	EntityID   int64  `json:"entityId"`
	Field      string `json:"field"`
	Delta      int32  `json:"delta"`
}

func NewCounterDeltaEvent(entityType string, entityID int64, field string, delta int32) *CounterDeltaEvent {
	return &CounterDeltaEvent{
		BaseEvent:  NewBaseEvent(EventTypeCounterDelta, "social-service"),
		EntityType: entityType,
		EntityID:   entityID,
		Field:      field,
		Delta:      delta,
	}
}

type NotificationInboxNewEvent struct {
	BaseEvent
	UserID      int64                  `json:"userId"`
	UserType    string                 `json:"userType"`
	Item        map[string]interface{} `json:"item"`
	UnreadCount int64                  `json:"unreadCount"`
}

func NewNotificationInboxNewEvent(userID int64, userType string, item map[string]interface{}, unreadCount int64) *NotificationInboxNewEvent {
	return &NotificationInboxNewEvent{
		BaseEvent:   NewBaseEvent(EventTypeNotificationInboxNew, "notification-service"),
		UserID:      userID,
		UserType:    userType,
		Item:        item,
		UnreadCount: unreadCount,
	}
}

type NotificationUnreadChangedEvent struct {
	BaseEvent
	UserID      int64  `json:"userId"`
	UserType    string `json:"userType"`
	UnreadCount int64  `json:"unreadCount"`
}

func NewNotificationUnreadChangedEvent(userID int64, userType string, unreadCount int64) *NotificationUnreadChangedEvent {
	return &NotificationUnreadChangedEvent{
		BaseEvent:   NewBaseEvent(EventTypeNotificationUnreadChanged, "notification-service"),
		UserID:      userID,
		UserType:    userType,
		UnreadCount: unreadCount,
	}
}

type PostHiddenByPlatformEvent struct {
	BaseEvent
	SellerID int64  `json:"seller_id"`
	StoreID  int64  `json:"store_id"`
	PostID   int64  `json:"post_id"`
	Reason   string `json:"reason"`
}

func NewPostHiddenByPlatformEvent(sellerID, postID, storeID int64, reason string) *PostHiddenByPlatformEvent {
	return &PostHiddenByPlatformEvent{
		BaseEvent: NewBaseEvent(EventTypePostHiddenByPlatform, "social-service"),
		SellerID:  sellerID,
		StoreID:   storeID,
		PostID:    postID,
		Reason:    reason,
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

type SellerMembershipChangedEvent struct {
	BaseEvent
	StoreID         string `json:"storeId"`
	PlanCode        string `json:"planCode"`
	TierRank        int    `json:"tierRank"`
	MonthlyOrderCap int    `json:"monthlyOrderCap"`
	EffectiveAt     string `json:"effectiveAt"`
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
