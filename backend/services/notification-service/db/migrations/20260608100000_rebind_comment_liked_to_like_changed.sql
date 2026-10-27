-- 41 號設計書 §6.1：social-service 直接拔除 comment.liked 改 publish comment.like_changed
-- （含 LIKE / UNLIKE 兩個 action）。notification-service consumer 必須同步改 routing key，
-- 並用 payload_filter 過濾 action=LIKE 才推播（UNLIKE 不發通知）。
--
-- 無過渡期：social-service 改完 commit 後，舊 comment.liked 不再有 producer，
-- 此 migration 必須與 social-service 改動同 deploy。
--
-- Schema 註解：notification_templates 為 DB-driven consumer 動態 binding 表，
-- worker/consumer.go 啟動時讀此表建立 RabbitMQ binding。改 routing_key 後重啟即生效。
--
-- 註：atlas 自動把每個 migration 包進 transaction，本檔案不寫 explicit BEGIN/COMMIT
-- （與本服務其他 28 個 migration 慣例一致）。explicit COMMIT 會讓 atlas 在最後 throw
-- "pq: unexpected transaction status idle" 並 exit 1，阻擋 deploy。

-- 1. 刪除舊 comment.liked template binding
DELETE FROM notification_templates
WHERE exchange = 'hawkr.content'
  AND routing_key = 'comment.liked';

-- 2. 新增 comment.like_changed template，payload_filter 過濾 action=LIKE
--    （UNLIKE 推送通知會干擾使用者，UX 上不發；SSE 仍會推 like count 變化）
-- id 必須帶值（20260404000001 已 DROP DEFAULT，改 TSID 由插入端產生），
-- 比照 20260603000000_shipment_anomaly_template.sql 用 EPOCH-based TSID。
INSERT INTO notification_templates (
    id,
    exchange, routing_key, queue_name,
    event_type, target_user_type, target_user_field,
    category, title_template, body_template, body_fallback,
    data_type, data_fields,
    schema_url, reference_id_template, fcm_topic,
    payload_filter,
    enabled
) VALUES (
    ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22),
    'hawkr.content', 'comment.like_changed', 'notification.comment.like_changed',
    'comment.like_changed', 'seller', 'commentAuthorId',
    'content',
    '貼文留言被點讚',
    '❤️ 買家喜歡你的回覆！你的專業解答讓 #{likerName} 覺得很有幫助。',
    NULL,
    'COMMENT_LIKED',
    '{"post_id": {"field": "postId"}, "comment_id": {"field": "commentId"}}',
    NULL,
    'comment_like_changed_#{commentId}',
    NULL,
    '{"action": "LIKE"}'::jsonb,  -- 只對 LIKE 推送、UNLIKE 不發通知
    true
);
