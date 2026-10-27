UPDATE "notification_templates"
SET data_fields = data_fields - 'actor'
WHERE event_type IN (
    'aftersales.appeal.submitted.buyer',
    'aftersales.appeal.submitted.seller',
    'aftersales.platform_decision.made.buyer',
    'aftersales.platform_decision.made.seller'
);
