package mq

const (
	ExchangeOrder        = "hawkr.order"
	ExchangePayment      = "hawkr.payment"
	ExchangeInvoice      = "hawkr.invoice"
	ExchangeMarketing    = "hawkr.marketing"
	ExchangeNotification = "hawkr.notification"
	ExchangeDelayed      = "hawkr.delayed"
	ExchangeShipping     = "hawkr.shipping"
	ExchangeMedia        = "hawkr.media"
	ExchangeReview       = "hawkr.review"
	ExchangeContent      = "hawkr.content"
	ExchangeAccount      = "hawkr.account"
	ExchangeGovernance   = "hawkr.governance"
	ExchangeBuyer        = "hawkr.buyer"
	ExchangeSeller       = "hawkr.seller"
	ExchangeSellerEvents = "seller.events"
	ExchangeAftersales   = "aftersales.events"
	ExchangeMerchant     = "hawkr.merchant"
	ExchangeProcurement  = "procurement.events"

	ExchangeDelegation = "hawkr.delegation"

	ExchangeSellerWorkspace = "hawkr.seller_workspace"
)

const (
	RoutingKeyOrderPaid      = "order.paid"
	RoutingKeyOrderCompleted = "order.completed"
	RoutingKeyOrderCancelled = "order.cancelled"

	RoutingKeyOrderRefundFinalized = "order.refund.finalized"

	RoutingKeyInvoiceIssued                       = "invoice.issued"
	RoutingKeyInvoiceIssueFailed                  = "invoice.issue_failed"
	RoutingKeyInvoiceVoided                       = "invoice.voided"
	RoutingKeyInvoiceAllowanceIssued              = "invoice.allowance.issued"
	RoutingKeyInvoiceReissued                     = "invoice.reissued"
	RoutingKeyInvoiceReceiptIssued                = "invoice.receipt.issued"
	RoutingKeyInvoiceOperationReviewRequired      = "invoice.operation.review_required"
	RoutingKeyInvoiceComplaintCreated             = "invoice.complaint.created"
	RoutingKeyInvoiceComplaintResponded           = "invoice.complaint.responded"
	RoutingKeyInvoiceComplaintOverdue             = "invoice.complaint.overdue"
	RoutingKeyInvoiceSellerTaxProfileStatusChange = "invoice.seller_tax_profile.status_changed"
	RoutingKeyInvoiceResolvedExternally           = "invoice.resolved_externally"

	RoutingKeyPaymentSuccess = "payment.success"
	RoutingKeyPaymentFailed  = "payment.failed"
	RoutingKeyPaymentExpired = "payment.expired"

	RoutingKeyShipmentCreated   = "shipment.created"
	RoutingKeyShipmentDelivered = "shipment.delivered"

	RoutingKeyReviewCreated = "review.created"
	RoutingKeyReviewHidden  = "review.hidden"

	RoutingKeyPostCreated          = "post.created"
	RoutingKeyPostLiked            = "post.liked"
	RoutingKeyPostHiddenByPlatform = "post.hidden_by_platform"
	RoutingKeyCommentCreated       = "comment.created"
	RoutingKeyCommentReplied       = "comment.replied"
	RoutingKeyCommentUpdated       = "comment.updated"
	RoutingKeyCommentDeleted       = "comment.deleted"
	RoutingKeyCommentLikeChanged   = "comment.like_changed"
	RoutingKeyCounterDelta         = "counter.delta"
	RoutingKeyProductViewed        = "product.viewed"

	RoutingKeyNotificationInboxNew      = "notification.inbox.new"
	RoutingKeyNotificationUnreadChanged = "notification.unread_changed"

	RoutingKeySellerOrderUpdated     = "seller.order.updated"
	RoutingKeySellerAfterSaleUpdated = "seller.after_sale.updated"

	RoutingKeyBuyerDeleted = "buyer.deleted"

	RoutingKeyRefundCompleted = "refund.completed"
	RoutingKeyPayoutCompleted = "payout.completed"

	RoutingKeyCouponIssued = "coupon.issued"

	RoutingKeyAccountDeleted = "account.deleted"

	RoutingKeyDelegationOperationCompleted = "delegation.operation.completed"

	RoutingKeySellerDeleted           = "seller.deleted"
	RoutingKeySellerOnboarded         = "seller.onboarded"
	RoutingKeySellerProfileChanged    = "seller.profile.changed"
	RoutingKeySellerSuspended         = "seller.suspended"
	RoutingKeySellerUnsuspended       = "seller.unsuspended"
	RoutingKeySellerMembershipChanged = "seller.membership_changed"

	RoutingKeyProcurementRfqPublicationInviteCreated = "procurement.rfq_publication_invite.created"

	RoutingKeyProcurementRfqPublicationInviteExternalCreated = "procurement.rfq_publication_invite.external_created"
)

const (
	RoutingKeyAftersalesRefundCompleted = "aftersales.refund.completed"
)

const (
	QueueSellerReviewCreated = "seller.review.created"
	QueueSellerReviewHidden  = "seller.review.hidden"

	QueueSocialBuyerDeleted = "social.buyer.deleted"

	QueueSocialSellerDeleted = "social.seller.deleted"

	QueueSocialCounterDelta = "social.counter.delta"

	QueueInvoiceOrderPaid            = "invoice.order.paid"
	QueueInvoiceOrderRefundFinalized = "invoice.order.refund.finalized"

	QueueInvoiceOrderCancelled = "invoice.order.cancelled"

	QueueIMAccountDeleted = "im.account.deleted"

	QueueIMBuyerDeleted = "im.buyer.deleted"

	QueueIMSellerDeleted = "im.seller.deleted"

	QueuePricebookSellerDeleted = "pricebook.seller.deleted"

	QueueNotificationAccountDeleted = "notification.account.deleted"

	QueueNotificationBuyerDeleted = "notification.buyer.deleted"

	QueueNotificationSellerDeleted = "notification.seller.deleted"

	QueueIMSellerSuspended = "im.seller.suspended"

	QueueSellerAccountDeleted = "seller.account.deleted"
)
