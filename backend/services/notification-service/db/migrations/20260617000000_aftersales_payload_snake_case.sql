-- 36 §7.4 Idempotency Conformance Contract F5 typed-record refactor follow-up：
-- aftersales-service 8 個 publisher payload 從 camelCase 翻轉為 snake_case wire（@JsonNaming SnakeCaseStrategy），
-- 同步 update notification_templates 35 row aftersales 路由：
--
-- 1. target_user_field：buyerAccountId / sellerAccountId → buyer_account_id / seller_account_id
-- 2. title_template / body_template / body_fallback / reference_id_template / image_url_template：
--    `#{camelCase}` 變數參考 → `#{snake_case}`（payload 端的 lookup 名稱）
-- 3. data_fields JSONB 右側 "field" 值：camelCase → snake_case（左側 FCM destination key 保持 camelCase，
--    對齊 20260608160357 normalize migration 的 API 規範）
--
-- 不動既有 notifications.data inbox row：歷史內容已 frozen，不影響未來 FCM/SSE 渲染。
--
-- 注意：不要在此檔寫顯式 BEGIN/COMMIT（Atlas 預設 txmode=file 已將整個 migration 包入 tx）。

-- ============================================================
-- Helper: 對單一 jsonb 內所有 leaf 的 "field" 值執行 oldVal → newVal 重命名。
-- 只 rewrite 值為 oldVal 的 "field" entry，其他 key 不動。
-- ============================================================
CREATE OR REPLACE FUNCTION pg_temp.rename_field_value(j jsonb, old_val text, new_val text)
RETURNS jsonb AS $$
DECLARE
    k text;
    v jsonb;
    result jsonb := '{}'::jsonb;
BEGIN
    IF j IS NULL THEN
        RETURN NULL;
    END IF;
    FOR k, v IN SELECT * FROM jsonb_each(j)
    LOOP
        IF v ? 'field' AND v ->> 'field' = old_val THEN
            result := result || jsonb_build_object(k, jsonb_set(v, '{field}', to_jsonb(new_val)));
        ELSE
            result := result || jsonb_build_object(k, v);
        END IF;
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- ============================================================
-- 1. target_user_field：buyerAccountId / sellerAccountId → snake_case
-- ============================================================
UPDATE notification_templates
SET target_user_field = CASE target_user_field
    WHEN 'buyerAccountId'  THEN 'buyer_account_id'
    WHEN 'sellerAccountId' THEN 'seller_account_id'
    ELSE target_user_field
END
WHERE routing_key LIKE 'aftersales%';

-- ============================================================
-- 2. title_template / body_template / body_fallback / reference_id_template / image_url_template
--    #{camelCase} 變數參考 → #{snake_case}
-- ============================================================
UPDATE notification_templates
SET
    title_template = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
        COALESCE(title_template, ''),
        '#{caseNo}', '#{case_no}'),
        '#{orderNo}', '#{order_no}'),
        '#{originalDeadlineHours}', '#{original_deadline_hours}'),
        '#{buyerAccountId}', '#{buyer_account_id}'),
        '#{sellerAccountId}', '#{seller_account_id}'),
        '#{refundTaskId}', '#{refund_task_id}'),
        '#{cancelId}', '#{cancel_id}'),
        '#{resolutionType}', '#{resolution_type}'),
        '#{refundAmount}', '#{refund_amount}'),
        '#{paymentMethod}', '#{payment_method}'),
        '#{externalRefId}', '#{external_ref_id}'),
        '#{buyerId}', '#{buyer_id}'),
        '#{sellerId}', '#{seller_id}'),
        '#{occurredAt}', '#{occurred_at}'),
        '#{shippingMethod}', '#{shipping_method}'),
        '#{deadlineAt}', '#{deadline_at}'),
        '#{autoCompleteReason}', '#{auto_complete_reason}'),
        '#{autoCompleteReasonLabel}', '#{auto_complete_reason_label}'),
        '#{storePickupDeadlineAt}', '#{store_pickup_deadline_at}'),
        '#{sellerPickedUpAt}', '#{seller_picked_up_at}'),
    body_template = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
        COALESCE(body_template, ''),
        '#{caseNo}', '#{case_no}'),
        '#{orderNo}', '#{order_no}'),
        '#{originalDeadlineHours}', '#{original_deadline_hours}'),
        '#{buyerAccountId}', '#{buyer_account_id}'),
        '#{sellerAccountId}', '#{seller_account_id}'),
        '#{refundTaskId}', '#{refund_task_id}'),
        '#{cancelId}', '#{cancel_id}'),
        '#{resolutionType}', '#{resolution_type}'),
        '#{refundAmount}', '#{refund_amount}'),
        '#{paymentMethod}', '#{payment_method}'),
        '#{externalRefId}', '#{external_ref_id}'),
        '#{buyerId}', '#{buyer_id}'),
        '#{sellerId}', '#{seller_id}'),
        '#{occurredAt}', '#{occurred_at}'),
        '#{shippingMethod}', '#{shipping_method}'),
        '#{deadlineAt}', '#{deadline_at}'),
        '#{autoCompleteReason}', '#{auto_complete_reason}'),
        '#{autoCompleteReasonLabel}', '#{auto_complete_reason_label}'),
        '#{storePickupDeadlineAt}', '#{store_pickup_deadline_at}'),
        '#{sellerPickedUpAt}', '#{seller_picked_up_at}'),
    reference_id_template = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
        COALESCE(reference_id_template, ''),
        '#{caseNo}', '#{case_no}'),
        '#{orderNo}', '#{order_no}'),
        '#{originalDeadlineHours}', '#{original_deadline_hours}'),
        '#{buyerAccountId}', '#{buyer_account_id}'),
        '#{sellerAccountId}', '#{seller_account_id}'),
        '#{refundTaskId}', '#{refund_task_id}'),
        '#{cancelId}', '#{cancel_id}'),
        '#{resolutionType}', '#{resolution_type}'),
        '#{refundAmount}', '#{refund_amount}'),
        '#{paymentMethod}', '#{payment_method}'),
        '#{externalRefId}', '#{external_ref_id}'),
        '#{buyerId}', '#{buyer_id}'),
        '#{sellerId}', '#{seller_id}'),
        '#{occurredAt}', '#{occurred_at}'),
        '#{shippingMethod}', '#{shipping_method}'),
        '#{deadlineAt}', '#{deadline_at}'),
        '#{autoCompleteReason}', '#{auto_complete_reason}'),
        '#{autoCompleteReasonLabel}', '#{auto_complete_reason_label}'),
        '#{storePickupDeadlineAt}', '#{store_pickup_deadline_at}'),
        '#{sellerPickedUpAt}', '#{seller_picked_up_at}'),
    image_url_template = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
        COALESCE(image_url_template, ''),
        '#{caseNo}', '#{case_no}'),
        '#{orderNo}', '#{order_no}'),
        '#{originalDeadlineHours}', '#{original_deadline_hours}'),
        '#{buyerAccountId}', '#{buyer_account_id}'),
        '#{sellerAccountId}', '#{seller_account_id}'),
        '#{refundTaskId}', '#{refund_task_id}'),
        '#{cancelId}', '#{cancel_id}'),
        '#{resolutionType}', '#{resolution_type}'),
        '#{refundAmount}', '#{refund_amount}'),
        '#{paymentMethod}', '#{payment_method}'),
        '#{externalRefId}', '#{external_ref_id}'),
        '#{buyerId}', '#{buyer_id}'),
        '#{sellerId}', '#{seller_id}'),
        '#{occurredAt}', '#{occurred_at}'),
        '#{shippingMethod}', '#{shipping_method}'),
        '#{deadlineAt}', '#{deadline_at}'),
        '#{autoCompleteReason}', '#{auto_complete_reason}'),
        '#{autoCompleteReasonLabel}', '#{auto_complete_reason_label}'),
        '#{storePickupDeadlineAt}', '#{store_pickup_deadline_at}'),
        '#{sellerPickedUpAt}', '#{seller_picked_up_at}')
WHERE routing_key LIKE 'aftersales%';

-- body_fallback 通常為純文字無 placeholder；保留 NULL 與既有純文字不動。
-- 若未來需要 placeholder 化可在後續 migration 補上 fallback REPLACE。

-- ============================================================
-- 3. data_fields JSONB：右側 "field" 值 camelCase → snake_case
--    左側 destination key 保留（對齊 20260608160357 normalize）。
-- ============================================================
UPDATE notification_templates
SET data_fields = pg_temp.rename_field_value(
    pg_temp.rename_field_value(
        pg_temp.rename_field_value(
            pg_temp.rename_field_value(
                pg_temp.rename_field_value(
                    pg_temp.rename_field_value(
                        pg_temp.rename_field_value(
                            pg_temp.rename_field_value(
                                pg_temp.rename_field_value(
                                    pg_temp.rename_field_value(
                                        pg_temp.rename_field_value(
                                            pg_temp.rename_field_value(
                                                pg_temp.rename_field_value(
                                                    pg_temp.rename_field_value(
                                                        pg_temp.rename_field_value(
                                                            pg_temp.rename_field_value(
                                                                pg_temp.rename_field_value(
                                                                    pg_temp.rename_field_value(
                                                                        pg_temp.rename_field_value(
                                                                            pg_temp.rename_field_value(
                                                                                data_fields,
                                                                                'caseNo', 'case_no'),
                                                                            'orderNo', 'order_no'),
                                                                        'buyerAccountId', 'buyer_account_id'),
                                                                    'sellerAccountId', 'seller_account_id'),
                                                                'refundTaskId', 'refund_task_id'),
                                                            'cancelId', 'cancel_id'),
                                                        'resolutionType', 'resolution_type'),
                                                    'refundAmount', 'refund_amount'),
                                                'paymentMethod', 'payment_method'),
                                            'externalRefId', 'external_ref_id'),
                                        'buyerId', 'buyer_id'),
                                    'sellerId', 'seller_id'),
                                'originalDeadlineHours', 'original_deadline_hours'),
                            'occurredAt', 'occurred_at'),
                        'shippingMethod', 'shipping_method'),
                    'deadlineAt', 'deadline_at'),
                'autoCompleteReason', 'auto_complete_reason'),
            'autoCompleteReasonLabel', 'auto_complete_reason_label'),
        'storePickupDeadlineAt', 'store_pickup_deadline_at'),
    'sellerPickedUpAt', 'seller_picked_up_at')
WHERE routing_key LIKE 'aftersales%' AND data_fields IS NOT NULL;
