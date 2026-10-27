-- Seller notification write-side identity-space fix, batch 3 of N (issue #2707).
--
-- 延續 batch1（20260830100000）／batch2（20260830110000）。這是最後 3 支：
-- comment.created／comment.replied.seller／comment.like_changed——batch2 特意排除
-- 這 3 支，因為 notification-service worker/template_handler.go 的
-- actorFieldByEventType 為它們登記了 self-notification skip，actor 欄位（原本是
-- commenterPrincipalId／replierPrincipalId／likerPrincipalId，已解析成 account_id）
-- 必須跟 target_user_field 落在同一個身分空間，否則翻旗標後 skip 比對永遠不成立
-- （賣家開始收到自己動作的推播，零錯誤訊號，是 #1626 那個 bug class 的鏡像重演）。
--
-- 本批同一個 commit 已經同步把 worker/template_handler.go 的 actorFieldByEventType
-- 改成對應的原始未解析欄位（commenterId／replierId／userId——這三個欄位本來就一直
-- 存在，是 gateway SSE 判斷 fromMe 用的 active-role surrogate，Phase A（PR #2921）
-- 之前就有，本次沒有新增任何 payload 欄位，純粹是 notification-service 這一端改讀
-- 哪個既有欄位），所以這 3 支現在可以安全跟 batch1/2 一樣執行 target_user_field
-- retarget，不會讓 self-skip 失效。
--
-- target 欄位改指向的原始 store_id 欄位（social-service PR #2921 就已存在，逐一對過
-- pkg/mq/events.go 原始碼確認 JSON key 名稱）：
--   comment.created        → postAuthorStoreId   (CommentCreatedEvent.PostAuthorStoreID)
--   comment.replied.seller → replyToUserStoreId  (CommentRepliedEvent.ReplyToUserStoreID)
--   comment.like_changed   → commentAuthorStoreId (CommentLikeChangedEvent.CommentAuthorStoreID)
--
-- buyer 側的兩張 sibling template（comment.created.buyer／comment.replied.buyer）不受
-- 本次影響，target_user_field 維持原樣（buyer_id 空間，本來就不需要 rekey）。
UPDATE "notification_templates"
SET target_user_field = 'postAuthorStoreId',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND event_type = 'comment.created'
  AND target_user_field = 'postAuthorId';

UPDATE "notification_templates"
SET target_user_field = 'replyToUserStoreId',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND event_type = 'comment.replied.seller'
  AND target_user_field = 'replyToUserId';

UPDATE "notification_templates"
SET target_user_field = 'commentAuthorStoreId',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND event_type = 'comment.like_changed'
  AND target_user_field = 'commentAuthorId';
