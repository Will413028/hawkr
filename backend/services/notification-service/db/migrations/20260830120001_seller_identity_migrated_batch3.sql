-- Seller notification identity-migration marker, batch 3 of N (issue #2707).
--
-- 語意同 batch1（20260830100002）／batch2（20260830110001）。這是最後 3 支，跟
-- 20260830120000_seller_target_field_store_id_rekey_batch3.sql 綁定同一組
-- event_type——issue #2707 39-event-type Phase B 全部收官，之後不再有 batch N+1。
UPDATE "notification_templates"
SET identity_migrated = true,
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND event_type IN (
    'comment.created',
    'comment.replied.seller',
    'comment.like_changed'
  );
