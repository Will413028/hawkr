UPDATE "notification_templates"
SET target_user_field = 'notify_buyer_account_id'
WHERE event_type IN (
    'aftersales.appeal.submitted.buyer',
    'aftersales.case.applied.buyer',
    'aftersales.case.auto_completed_return_pending_timeout.buyer',
    'aftersales.case.auto_escalated.buyer',
    'aftersales.case.auto_rejected.buyer',
    'aftersales.case.cancelled_by_buyer.buyer',
    'aftersales.case.coupon_compensated.buyer',
    'aftersales.case.refund_approved.buyer',
    'aftersales.case.rejected.buyer',
    'aftersales.case.return_approved.buyer',
    'aftersales.case.return_failed.buyer',
    'aftersales.case.return_pending_24h.buyer',
    'aftersales.case.return_received.buyer',
    'aftersales.platform_decision.made.buyer'
);
