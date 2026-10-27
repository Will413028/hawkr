-- Enable the procurement quote-invite seller templates after the publisher and
-- MQ topology are deployed. This is forward-only: existing notifications are
-- not rewritten or backfilled.
--
-- The assertions make an accidental partial rollout fail during migration
-- rather than silently enabling a template in the wrong identity space.
DO $$
BEGIN
  IF (
    SELECT COUNT(*)
    FROM notification_templates
    WHERE event_type IN (
      'procurement.quote_invite.created.seller',
      'procurement.quote_invite.lost.seller'
    )
  ) <> 2 THEN
    RAISE EXCEPTION 'procurement quote-invite templates are missing';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM notification_templates
    WHERE event_type IN (
      'procurement.quote_invite.created.seller',
      'procurement.quote_invite.lost.seller'
    )
    AND (
      target_user_type <> 'seller'
      OR target_user_field <> 'store_id'
      OR identity_migrated IS NOT TRUE
    )
  ) THEN
    RAISE EXCEPTION 'procurement quote-invite templates must target migrated store_id seller identities';
  END IF;

  UPDATE notification_templates
  SET enabled = TRUE,
      updated_at = CURRENT_TIMESTAMP
  WHERE event_type IN (
    'procurement.quote_invite.created.seller',
    'procurement.quote_invite.lost.seller'
  );
END $$;
