-- Fix notification_preferences user_type: 'merchant' → 'seller'
-- Root cause: gateway stored seller preferences with user_type='merchant' but
-- send_push.go queries with 'seller' (from template target_user_type), causing
-- preference check to always return nil (fail-open) and push through regardless.

-- 先刪掉會衝突的 seller row（同 user+category 已有 merchant row 的）
DELETE FROM notification_preferences
WHERE user_type = 'seller'
  AND EXISTS (
    SELECT 1 FROM notification_preferences np2
    WHERE np2.user_id = notification_preferences.user_id
      AND np2.user_type = 'merchant'
      AND np2.category = notification_preferences.category
  );

-- 再把 merchant → seller
UPDATE notification_preferences SET user_type = 'seller' WHERE user_type = 'merchant';
