-- 41 號設計書 §3.3.2 / CLAUDE.md §2.2:JSON Body key 一律 camelCase。
--
-- 既有 notification_templates.data_fields destination key 用 snake_case
-- (post_id / comment_id / commenter_id / order_id / 等共 20 個),
-- 寫入 notifications.data jsonb 後 client 解析時違反 API 規範。
--
-- 本 migration:
-- 1. UPDATE notification_templates.data_fields:rename 所有 snake_case 為 camelCase
-- 2. UPDATE notifications.data(既有 inbox row):同樣 rename(避免歷史資料 mixed key)
--
-- 影響:
-- - inbox API GET /api/v1/app/notifications 回傳 records[].data 變 camelCase
-- - SSE notification.new event item.data 變 camelCase
-- - FCM data payload 變 camelCase(mobile 端需配合解析)
--
-- 前端三端(web / buyer / seller)需同時 deploy 解析改動。
--
-- 注意:不要在此檔寫顯式 BEGIN/COMMIT。Atlas 預設 txmode=file,
-- 已把整個 migration 包在它管理的 transaction 內;檔內再 COMMIT 會
-- 提早結束該 tx,導致 Atlas 寫版本紀錄時 connection 已 idle
-- (pq: unexpected transaction status idle)。

-- ============================================================
-- Helper: 對 jsonb 內單一 top-level key rename(舊 key value 移到新 key,刪舊 key)
-- 用法:rename_jsonb_key(data, 'post_id', 'postId')
-- ============================================================
CREATE OR REPLACE FUNCTION pg_temp.rename_jsonb_key(j jsonb, old_key text, new_key text)
RETURNS jsonb AS $$
BEGIN
    IF j IS NULL OR NOT (j ? old_key) THEN
        RETURN j;
    END IF;
    RETURN (j - old_key) || jsonb_build_object(new_key, j -> old_key);
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- ============================================================
-- 1. notification_templates.data_fields:rename 20 個 snake_case destination key
-- ============================================================
UPDATE notification_templates
SET data_fields = pg_temp.rename_jsonb_key(
    pg_temp.rename_jsonb_key(
        pg_temp.rename_jsonb_key(
            pg_temp.rename_jsonb_key(
                pg_temp.rename_jsonb_key(
                    pg_temp.rename_jsonb_key(
                        pg_temp.rename_jsonb_key(
                            pg_temp.rename_jsonb_key(
                                pg_temp.rename_jsonb_key(
                                    pg_temp.rename_jsonb_key(
                                        pg_temp.rename_jsonb_key(
                                            pg_temp.rename_jsonb_key(
                                                pg_temp.rename_jsonb_key(
                                                    pg_temp.rename_jsonb_key(
                                                        pg_temp.rename_jsonb_key(
                                                            pg_temp.rename_jsonb_key(
                                                                pg_temp.rename_jsonb_key(
                                                                    pg_temp.rename_jsonb_key(
                                                                        pg_temp.rename_jsonb_key(
                                                                            pg_temp.rename_jsonb_key(data_fields, 'announcement_id', 'announcementId'),
                                                                            'anomaly_type', 'anomalyType'),
                                                                        'appeal_id', 'appealId'),
                                                                    'appeal_status', 'appealStatus'),
                                                                'case_no', 'caseNo'),
                                                            'comment_id', 'commentId'),
                                                        'commenter_id', 'commenterId'),
                                                    'coupon_id', 'couponId'),
                                                'decision_reason', 'decisionReason'),
                                            'follower_id', 'followerId'),
                                        'order_id', 'orderId'),
                                    'order_no', 'orderNo'),
                                'penalty_type', 'penaltyType'),
                            'post_id', 'postId'),
                        'product_id', 'productId'),
                    'report_id', 'reportId'),
                'review_id', 'reviewId'),
            'revoked_reason', 'revokedReason'),
        'seller_id', 'sellerId'),
    'violation_id', 'violationId');

-- ============================================================
-- 2. notifications.data:既有 inbox row 同樣 normalize(避免讀路徑 mixed key)
--
-- 注意 data 內可能還有其他非 data_fields 來源的 key(template 內 hardcode 寫入的、
-- url 模板、type 等),這些 key 已是 camelCase 或單字(無 _),不會被 rename 影響。
-- ============================================================
UPDATE notifications
SET data = pg_temp.rename_jsonb_key(
    pg_temp.rename_jsonb_key(
        pg_temp.rename_jsonb_key(
            pg_temp.rename_jsonb_key(
                pg_temp.rename_jsonb_key(
                    pg_temp.rename_jsonb_key(
                        pg_temp.rename_jsonb_key(
                            pg_temp.rename_jsonb_key(
                                pg_temp.rename_jsonb_key(
                                    pg_temp.rename_jsonb_key(
                                        pg_temp.rename_jsonb_key(
                                            pg_temp.rename_jsonb_key(
                                                pg_temp.rename_jsonb_key(
                                                    pg_temp.rename_jsonb_key(
                                                        pg_temp.rename_jsonb_key(
                                                            pg_temp.rename_jsonb_key(
                                                                pg_temp.rename_jsonb_key(
                                                                    pg_temp.rename_jsonb_key(
                                                                        pg_temp.rename_jsonb_key(
                                                                            pg_temp.rename_jsonb_key(data, 'announcement_id', 'announcementId'),
                                                                            'anomaly_type', 'anomalyType'),
                                                                        'appeal_id', 'appealId'),
                                                                    'appeal_status', 'appealStatus'),
                                                                'case_no', 'caseNo'),
                                                            'comment_id', 'commentId'),
                                                        'commenter_id', 'commenterId'),
                                                    'coupon_id', 'couponId'),
                                                'decision_reason', 'decisionReason'),
                                            'follower_id', 'followerId'),
                                        'order_id', 'orderId'),
                                    'order_no', 'orderNo'),
                                'penalty_type', 'penaltyType'),
                            'post_id', 'postId'),
                        'product_id', 'productId'),
                    'report_id', 'reportId'),
                'review_id', 'reviewId'),
            'revoked_reason', 'revokedReason'),
        'seller_id', 'sellerId'),
    'violation_id', 'violationId')
WHERE data IS NOT NULL;
