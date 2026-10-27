-- 36 §7.4 F4 consumer-side retrofit（issue #1051 M-1/M-2）：shipping-service 4 個
-- payload record 已於 PR #1001 加上 @JsonNaming(SnakeCaseStrategy)，publisher 端 JSON 全
-- snake_case，但 notification_templates 對應 row 仍引用 camelCase 變數
-- （target_user_field=buyerAccountId/sellerAccountId、data_fields field=orderNo/
-- buyerNickname/buyerAvatar、body_template/title_template/reference_id_template 內
-- #{orderNo} #{buyerNickname} #{buyerAvatar} 等），導致 template 渲染缺值 + 賣家「買家已下單」
-- 與買賣家「包裹送達」推播文案漏訂單編號/個人化字樣。
-- 本 migration 同步遷移為 snake_case，對齊 publisher payload contract。
-- 仿 20260618141717_order_payload_snake_case.sql 的 helper + UPDATE pattern。
--
-- 受影響 routing_key：shipment.created（1 row → seller）/ shipment.delivered（2 rows → buyer + seller）
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
WHERE routing_key IN ('shipment.created', 'shipment.delivered');

-- ============================================================
-- 2. title_template / body_template / body_fallback / reference_id_template / image_url_template
--    `#{camelCase}` → `#{snake_case}`
--
--    F4 ShipmentCreatedPayload / ShippingEventPayload 涉及的 camelCase 變數：
--    orderNo / shipmentNo / shippingMethod / shipTradeNo / labelUrl / labelExpireAt /
--    statusCode / statusDesc / sellerName / sellerAvatar / buyerNickname / buyerAvatar /
--    buyerAccountId / sellerAccountId / totalActualAmount / processingDeadline /
--    providerStatus / providerMessage / occurredAt
-- ============================================================
UPDATE notification_templates
SET
    title_template        = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                title_template,
                                '#{orderNo}',            '#{order_no}'),
                                '#{shipmentNo}',         '#{shipment_no}'),
                                '#{shippingMethod}',     '#{shipping_method}'),
                                '#{shipTradeNo}',        '#{ship_trade_no}'),
                                '#{labelUrl}',           '#{label_url}'),
                                '#{labelExpireAt}',      '#{label_expire_at}'),
                                '#{statusCode}',         '#{status_code}'),
                                '#{statusDesc}',         '#{status_desc}'),
                                '#{sellerName}',         '#{seller_name}'),
                                '#{sellerAvatar}',       '#{seller_avatar}'),
                                '#{buyerNickname}',      '#{buyer_nickname}'),
                                '#{buyerAvatar}',        '#{buyer_avatar}'),
                                '#{buyerAccountId}',     '#{buyer_account_id}'),
                                '#{sellerAccountId}',    '#{seller_account_id}'),
                                '#{totalActualAmount}',  '#{total_actual_amount}'),
                                '#{processingDeadline}', '#{processing_deadline}'),
                                '#{providerStatus}',     '#{provider_status}'),
                                '#{providerMessage}',    '#{provider_message}'),
                                '#{occurredAt}',         '#{occurred_at}'),
    body_template         = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                body_template,
                                '#{orderNo}',            '#{order_no}'),
                                '#{shipmentNo}',         '#{shipment_no}'),
                                '#{shippingMethod}',     '#{shipping_method}'),
                                '#{shipTradeNo}',        '#{ship_trade_no}'),
                                '#{labelUrl}',           '#{label_url}'),
                                '#{labelExpireAt}',      '#{label_expire_at}'),
                                '#{statusCode}',         '#{status_code}'),
                                '#{statusDesc}',         '#{status_desc}'),
                                '#{sellerName}',         '#{seller_name}'),
                                '#{sellerAvatar}',       '#{seller_avatar}'),
                                '#{buyerNickname}',      '#{buyer_nickname}'),
                                '#{buyerAvatar}',        '#{buyer_avatar}'),
                                '#{buyerAccountId}',     '#{buyer_account_id}'),
                                '#{sellerAccountId}',    '#{seller_account_id}'),
                                '#{totalActualAmount}',  '#{total_actual_amount}'),
                                '#{processingDeadline}', '#{processing_deadline}'),
                                '#{providerStatus}',     '#{provider_status}'),
                                '#{providerMessage}',    '#{provider_message}'),
                                '#{occurredAt}',         '#{occurred_at}'),
    body_fallback         = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                body_fallback,
                                '#{orderNo}',            '#{order_no}'),
                                '#{shipmentNo}',         '#{shipment_no}'),
                                '#{shippingMethod}',     '#{shipping_method}'),
                                '#{shipTradeNo}',        '#{ship_trade_no}'),
                                '#{labelUrl}',           '#{label_url}'),
                                '#{labelExpireAt}',      '#{label_expire_at}'),
                                '#{statusCode}',         '#{status_code}'),
                                '#{statusDesc}',         '#{status_desc}'),
                                '#{sellerName}',         '#{seller_name}'),
                                '#{sellerAvatar}',       '#{seller_avatar}'),
                                '#{buyerNickname}',      '#{buyer_nickname}'),
                                '#{buyerAvatar}',        '#{buyer_avatar}'),
                                '#{buyerAccountId}',     '#{buyer_account_id}'),
                                '#{sellerAccountId}',    '#{seller_account_id}'),
                                '#{totalActualAmount}',  '#{total_actual_amount}'),
                                '#{processingDeadline}', '#{processing_deadline}'),
                                '#{providerStatus}',     '#{provider_status}'),
                                '#{providerMessage}',    '#{provider_message}'),
                                '#{occurredAt}',         '#{occurred_at}'),
    reference_id_template = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                reference_id_template,
                                '#{orderNo}',            '#{order_no}'),
                                '#{shipmentNo}',         '#{shipment_no}'),
                                '#{shippingMethod}',     '#{shipping_method}'),
                                '#{shipTradeNo}',        '#{ship_trade_no}'),
                                '#{labelUrl}',           '#{label_url}'),
                                '#{labelExpireAt}',      '#{label_expire_at}'),
                                '#{statusCode}',         '#{status_code}'),
                                '#{statusDesc}',         '#{status_desc}'),
                                '#{sellerName}',         '#{seller_name}'),
                                '#{sellerAvatar}',       '#{seller_avatar}'),
                                '#{buyerNickname}',      '#{buyer_nickname}'),
                                '#{buyerAvatar}',        '#{buyer_avatar}'),
                                '#{buyerAccountId}',     '#{buyer_account_id}'),
                                '#{sellerAccountId}',    '#{seller_account_id}'),
                                '#{totalActualAmount}',  '#{total_actual_amount}'),
                                '#{processingDeadline}', '#{processing_deadline}'),
                                '#{providerStatus}',     '#{provider_status}'),
                                '#{providerMessage}',    '#{provider_message}'),
                                '#{occurredAt}',         '#{occurred_at}'),
    image_url_template    = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
                                image_url_template,
                                '#{orderNo}',            '#{order_no}'),
                                '#{shipmentNo}',         '#{shipment_no}'),
                                '#{shippingMethod}',     '#{shipping_method}'),
                                '#{shipTradeNo}',        '#{ship_trade_no}'),
                                '#{labelUrl}',           '#{label_url}'),
                                '#{labelExpireAt}',      '#{label_expire_at}'),
                                '#{statusCode}',         '#{status_code}'),
                                '#{statusDesc}',         '#{status_desc}'),
                                '#{sellerName}',         '#{seller_name}'),
                                '#{sellerAvatar}',       '#{seller_avatar}'),
                                '#{buyerNickname}',      '#{buyer_nickname}'),
                                '#{buyerAvatar}',        '#{buyer_avatar}'),
                                '#{buyerAccountId}',     '#{buyer_account_id}'),
                                '#{sellerAccountId}',    '#{seller_account_id}'),
                                '#{totalActualAmount}',  '#{total_actual_amount}'),
                                '#{processingDeadline}', '#{processing_deadline}'),
                                '#{providerStatus}',     '#{provider_status}'),
                                '#{providerMessage}',    '#{provider_message}'),
                                '#{occurredAt}',         '#{occurred_at}')
WHERE routing_key IN ('shipment.created', 'shipment.delivered');

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
                                                                    pg_temp.rename_field_value(
                                                                        pg_temp.rename_field_value(
                                                                            pg_temp.rename_field_value(
                                                                                pg_temp.rename_field_value(
                                                                                    pg_temp.rename_field_value(
                                                                                        pg_temp.rename_field_value(
                                                                                            data_fields,
                                                                                            'orderNo',            'order_no'),
                                                                                        'shipmentNo',         'shipment_no'),
                                                                                    'shippingMethod',     'shipping_method'),
                                                                                'shipTradeNo',        'ship_trade_no'),
                                                                            'labelUrl',           'label_url'),
                                                                        'labelExpireAt',      'label_expire_at'),
                                                                    'statusCode',         'status_code'),
                                                                'statusDesc',         'status_desc'),
                                                            'sellerName',         'seller_name'),
                                                        'sellerAvatar',       'seller_avatar'),
                                                    'buyerNickname',      'buyer_nickname'),
                                                'buyerAvatar',        'buyer_avatar'),
                                            'buyerAccountId',     'buyer_account_id'),
                                        'sellerAccountId',    'seller_account_id'),
                                    'totalActualAmount',  'total_actual_amount'),
                                'processingDeadline', 'processing_deadline'),
                            'providerStatus',     'provider_status'),
                        'providerMessage',    'provider_message'),
                    'occurredAt',         'occurred_at')
WHERE routing_key IN ('shipment.created', 'shipment.delivered');
