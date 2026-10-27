-- 36 §7.4 F2 retrofit：order-service 7 個 OrderXxxPayload record 已於 PR #999（commit 84925c43）
-- 加上 @JsonNaming(SnakeCaseStrategy)，publisher 端 JSON 全 snake_case，但 notification_templates
-- 5 row 仍引用 camelCase 變數（#{orderNo} / #{sellerAvatar} / target_user_field=buyerAccountId/sellerAccountId
-- / data_fields field=orderNo），導致 template 渲染缺值 + 推播文案漏訂單編號。
-- 本 migration 同步遷移為 snake_case，對齊 publisher payload contract。
-- 仿 20260617000000_aftersales_payload_snake_case.sql 的 helper + UPDATE pattern。
--
-- 受影響 routing_key：order.paid（1 row）/ order.cancelled（2 rows）/ order.completed（2 rows）
-- 注意：order.shipped 屬 shipping-service publisher（非 F2 scope），本 migration 不動。
-- 同樣不動：order.delivered / order.receipt.confirmed —— 目前 notification_templates 無對應 row。
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
WHERE routing_key IN ('order.paid', 'order.cancelled', 'order.completed');

-- ============================================================
-- 2. title_template / body_template / body_fallback / reference_id_template / image_url_template
--    `#{camelCase}` → `#{snake_case}`
--
--    F2 OrderPaidPayload / OrderCancelledPayload / OrderCompletedPayload 涉及的 camelCase 變數：
--    orderNo / sellerAvatar / sellerName / buyerNickname / buyerAvatar / buyerAccountId /
--    sellerAccountId / totalActualAmount / confirmType / completedAt / occurredAt /
--    stockTxId / shippingMethod
-- ============================================================
UPDATE notification_templates
SET
    title_template        = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                title_template,
                                '#{orderNo}',           '#{order_no}'),
                                '#{sellerAvatar}',      '#{seller_avatar}'),
                                '#{sellerName}',        '#{seller_name}'),
                                '#{buyerNickname}',     '#{buyer_nickname}'),
                                '#{buyerAvatar}',       '#{buyer_avatar}'),
                                '#{buyerAccountId}',    '#{buyer_account_id}'),
                                '#{sellerAccountId}',   '#{seller_account_id}'),
                                '#{totalActualAmount}', '#{total_actual_amount}'),
                                '#{confirmType}',       '#{confirm_type}'),
                                '#{completedAt}',       '#{completed_at}'),
                                '#{occurredAt}',        '#{occurred_at}'),
                                '#{stockTxId}',         '#{stock_tx_id}'),
                                '#{shippingMethod}',    '#{shipping_method}'),
    body_template         = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                body_template,
                                '#{orderNo}',           '#{order_no}'),
                                '#{sellerAvatar}',      '#{seller_avatar}'),
                                '#{sellerName}',        '#{seller_name}'),
                                '#{buyerNickname}',     '#{buyer_nickname}'),
                                '#{buyerAvatar}',       '#{buyer_avatar}'),
                                '#{buyerAccountId}',    '#{buyer_account_id}'),
                                '#{sellerAccountId}',   '#{seller_account_id}'),
                                '#{totalActualAmount}', '#{total_actual_amount}'),
                                '#{confirmType}',       '#{confirm_type}'),
                                '#{completedAt}',       '#{completed_at}'),
                                '#{occurredAt}',        '#{occurred_at}'),
                                '#{stockTxId}',         '#{stock_tx_id}'),
                                '#{shippingMethod}',    '#{shipping_method}'),
    body_fallback         = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                body_fallback,
                                '#{orderNo}',           '#{order_no}'),
                                '#{sellerAvatar}',      '#{seller_avatar}'),
                                '#{sellerName}',        '#{seller_name}'),
                                '#{buyerNickname}',     '#{buyer_nickname}'),
                                '#{buyerAvatar}',       '#{buyer_avatar}'),
                                '#{buyerAccountId}',    '#{buyer_account_id}'),
                                '#{sellerAccountId}',   '#{seller_account_id}'),
                                '#{totalActualAmount}', '#{total_actual_amount}'),
                                '#{confirmType}',       '#{confirm_type}'),
                                '#{completedAt}',       '#{completed_at}'),
                                '#{occurredAt}',        '#{occurred_at}'),
                                '#{stockTxId}',         '#{stock_tx_id}'),
                                '#{shippingMethod}',    '#{shipping_method}'),
    reference_id_template = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                reference_id_template,
                                '#{orderNo}',           '#{order_no}'),
                                '#{sellerAvatar}',      '#{seller_avatar}'),
                                '#{sellerName}',        '#{seller_name}'),
                                '#{buyerNickname}',     '#{buyer_nickname}'),
                                '#{buyerAvatar}',       '#{buyer_avatar}'),
                                '#{buyerAccountId}',    '#{buyer_account_id}'),
                                '#{sellerAccountId}',   '#{seller_account_id}'),
                                '#{totalActualAmount}', '#{total_actual_amount}'),
                                '#{confirmType}',       '#{confirm_type}'),
                                '#{completedAt}',       '#{completed_at}'),
                                '#{occurredAt}',        '#{occurred_at}'),
                                '#{stockTxId}',         '#{stock_tx_id}'),
                                '#{shippingMethod}',    '#{shipping_method}'),
    image_url_template    = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                image_url_template,
                                '#{orderNo}',           '#{order_no}'),
                                '#{sellerAvatar}',      '#{seller_avatar}'),
                                '#{sellerName}',        '#{seller_name}'),
                                '#{buyerNickname}',     '#{buyer_nickname}'),
                                '#{buyerAvatar}',       '#{buyer_avatar}'),
                                '#{buyerAccountId}',    '#{buyer_account_id}'),
                                '#{sellerAccountId}',   '#{seller_account_id}'),
                                '#{totalActualAmount}', '#{total_actual_amount}'),
                                '#{confirmType}',       '#{confirm_type}'),
                                '#{completedAt}',       '#{completed_at}'),
                                '#{occurredAt}',        '#{occurred_at}'),
                                '#{stockTxId}',         '#{stock_tx_id}'),
                                '#{shippingMethod}',    '#{shipping_method}')
WHERE routing_key IN ('order.paid', 'order.cancelled', 'order.completed');

-- ============================================================
-- 3. data_fields JSONB 右側 "field" 值：camelCase → snake_case
--    （左側 FCM destination key 保持 camelCase，對齊 20260608160357 normalize migration 的 API 規範）
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
                                                                    data_fields,
                                                                    'orderNo',           'order_no'),
                                                                'sellerAvatar',      'seller_avatar'),
                                                            'sellerName',        'seller_name'),
                                                        'buyerNickname',     'buyer_nickname'),
                                                    'buyerAvatar',       'buyer_avatar'),
                                                'buyerAccountId',    'buyer_account_id'),
                                            'sellerAccountId',   'seller_account_id'),
                                        'totalActualAmount', 'total_actual_amount'),
                                    'confirmType',       'confirm_type'),
                                'completedAt',       'completed_at'),
                            'occurredAt',        'occurred_at'),
                        'stockTxId',         'stock_tx_id'),
                    'shippingMethod',    'shipping_method')
WHERE routing_key IN ('order.paid', 'order.cancelled', 'order.completed');
