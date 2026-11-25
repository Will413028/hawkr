package mq

const (
	ExchangeOrder        = "hawkr.order"
	ExchangePayment      = "hawkr.payment"
	ExchangeInvoice      = "hawkr.invoice"
	ExchangeMarketing    = "hawkr.marketing"
	ExchangeDelayed      = "hawkr.delayed"
	ExchangeShipping     = "hawkr.shipping"
	ExchangeReview       = "hawkr.review"
	ExchangeGovernance   = "hawkr.governance"
	ExchangeBuyer        = "hawkr.buyer"
	ExchangeSeller       = "hawkr.seller"
	ExchangeSellerEvents = "seller.events"
	ExchangeAftersales   = "aftersales.events"
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

	RoutingKeySellerOrderUpdated     = "seller.order.updated"
	RoutingKeySellerAfterSaleUpdated = "seller.after_sale.updated"

	RoutingKeyBuyerDeleted = "buyer.deleted"

	RoutingKeyRefundCompleted = "refund.completed"
	RoutingKeyPayoutCompleted = "payout.completed"

	RoutingKeyCouponIssued = "coupon.issued"

	RoutingKeyAccountDeleted = "account.deleted"

	RoutingKeySellerDeleted        = "seller.deleted"
	RoutingKeySellerOnboarded      = "seller.onboarded"
	RoutingKeySellerProfileChanged = "seller.profile.changed"
	RoutingKeySellerSuspended      = "seller.suspended"
	RoutingKeySellerUnsuspended    = "seller.unsuspended"

	RoutingKeyProcurementRfqPublicationInviteCreated = "procurement.rfq_publication_invite.created"

	RoutingKeyProcurementRfqPublicationInviteExternalCreated = "procurement.rfq_publication_invite.external_created"
)

const (
	RoutingKeyAftersalesRefundCompleted = "aftersales.refund.completed"
)

const (
	QueueInvoiceOrderPaid            = "invoice.order.paid"
	QueueInvoiceOrderRefundFinalized = "invoice.order.refund.finalized"

	QueueInvoiceOrderCancelled = "invoice.order.cancelled"
)
