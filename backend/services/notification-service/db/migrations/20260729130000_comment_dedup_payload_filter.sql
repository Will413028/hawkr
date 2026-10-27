-- comment.created / comment.created.buyer and comment.replied.seller / comment.replied.buyer
-- each pair shares one routing key with no payload_filter, so every event matches ALL
-- templates on that key unconditionally -- the post/comment owner gets two near-duplicate
-- notifications per comment/reply. social-service already publishes the disambiguating
-- signal on both payloads (postAuthorType / replyToUserType, "buyer" or "seller" --
-- see comment.go's resolvePostAuthorType and the inline replyToUserType computation),
-- it was just never wired into a payload_filter. Mirrors the existing
-- payload_filter-disambiguates-shared-routing-key pattern already used elsewhere
-- (e.g. order.cancelled.buyer vs order.cancelled.seller_suspended on cancel_reason).
UPDATE "notification_templates"
SET payload_filter = '{"postAuthorType": "seller"}'
WHERE event_type = 'comment.created';

UPDATE "notification_templates"
SET payload_filter = '{"postAuthorType": "buyer"}'
WHERE event_type = 'comment.created.buyer';

UPDATE "notification_templates"
SET payload_filter = '{"replyToUserType": "seller"}'
WHERE event_type = 'comment.replied.seller';

UPDATE "notification_templates"
SET payload_filter = '{"replyToUserType": "buyer"}'
WHERE event_type = 'comment.replied.buyer';
