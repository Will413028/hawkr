-- 代行完成通知補上 refType / refId（56 號 §6.3 / 13 號 §5.2）。
--
-- 病徵（dev 實測）：賣家收得到「平台管理員 … 已代您執行『上架商品』」，但
-- GET /app/notifications 對這一則回 refId=null、refType=null、無任何跳轉目標。
--
-- 成因：`20260815130000_delegation_operation_completed_template.sql` 把
-- data_fields 與 schema_url 都定為 NULL，於是模板引擎產出的 data map 只有
-- type 與 category 兩個鍵（internal/worker/template_handler.go:141-146）；而
-- gateway 的 refType/refId **不是**讀 notifications.reference_id 欄位，是讀
-- data map 裡的同名鍵（gateway/internal/handler/notification_inbox.go:41-64，
-- 只有 data["refType"] 與 data["refId"] 皆非空才回傳，否則落到 order_id /
-- conversation_id / certification_id / post_id 四個 legacy fallback，本事件
-- 一個都沒有）⇒ 通知送達、內文正確、稽核有寫、outbox done，每一層都對，
-- 只有召回價值是零。
--
-- 前一版那兩個 NULL 的理由（「賣家側零改動、沒有代行紀錄的落地頁」）對
-- schema_url 仍然成立，對 data_fields 不成立：refType/refId 是**收件匣 API 的
-- 契約欄位**，不是 deep link。
--
-- ── refType='DELEGATION'（const），refId=audit_log_id ──────────────────
-- ① **不複用 PRODUCT / ORDER / POST**：兩個賣家端消費者對這些值都有既有跳轉
--    分支（seller App `notifications_screen.dart:233-273` 的 inbox switch、
--    `notification/data/push/router.dart:56-127` 的 FCM router），把 audit_log_id
--    餵給 case 'ORDER' 會 push 一個不存在的訂單詳情 —— 錯誤跳轉比不跳更糟。
-- ② **不把 refType 綁 resource_type**（`{"field":"resource_type"}`）：值域是小寫
--    storefront/product/post/order，而且**訂單面的 resource_id 恆為空字串** ——
--    訂單號是 `O` + 9 位數，塞不進 admin_operation_audit_logs.resource_id
--    (bigint)，parseOptionalInt64 存 NULL（account-service
--    internal/delegation/usecase/audit.go:461-465）。gateway 要求 refId 非空才
--    回傳，所以訂單面照樣是 null，等於沒修。
-- ③ audit_log_id **恆非空**，且與本列 reference_id_template（`#{audit_log_id}`）
--    同值 ⇒ refId 與 notifications.reference_id 一致，與全表其他模板的慣例相同
--    （ORDER→order_no、AFTERSALE→caseNo、SELLER_SUSPENSION→suspensionId、
--    CHAT→conversationId，四者皆 refId ≡ reference_id）。
-- ④ 語意也對：一則通知對應**一次代行操作**，不是對應一個商品。同一商品被代行
--    兩次是兩則獨立通知，用 productId 當 refId 會讓兩則指向同一個 ref。
--
-- ── schema_url 維持 NULL ───────────────────────────────────────────────
-- ① 賣家端沒有代行紀錄落地頁（56 §2.2 第 10 條，本期 Seller Center／Seller App
--    零改動）。
-- ② seller App 的 FCM router 對未知 type 走 default 分支，只要 data['url'] 非空
--    就 `launchUrl(..., LaunchMode.externalApplication)`（`push/router.dart:185-194`）
--    ⇒ 填一個沒有頁面的 deep link 會變成不可預期的外開行為，而本事件的
--    data.type=DELEGATION_OPERATION_COMPLETED 正好不在該 switch 內。
-- ③ gateway 的 inboxItemDTO 根本沒有 actionUrl 欄位，website 的
--    `resolveNotification` 讀的是 item.actionUrl（lib/notifications/resolve.ts:293）
--    ⇒ 賣家端目前沒有任何一處會消費 data.url。
-- ④ schema_url 是單一字串模板，無法依 resource_type 分歧 —— 四種資源的落地頁
--    互不相同，一條 URL 必然對其中三種是錯的。
--
-- ── resourceType / resourceId / resourceName ──────────────────────────
-- 這三個是「跳到被操作的資源」所需的最小材料，全部已在 publisher 實發的
-- payload 內（tests/contract/golden/event_payload_keys.json 已列，publisher 零改動）。
-- 前端日後要讓賣家從通知直接進到那個商品／貼文／店面時，不需要後端再改一次。
--   • resourceType：storefront | product | post | order（小寫，account-service
--     internal/delegation/domain/audit_log.go:24-29 的封閉四值）
--   • resourceId  ：TSID 字串；**訂單面恆為空字串**（見上 ②），前端須判空
--   • resourceName：訂單面唯一拿得到單號的地方 —— gateway 對訂單同時傳
--     ResourceID 與 ResourceName 兩個 orderNo（admin_delegated_order.go:292,296），
--     只有 resource_name 存得下。其他資源型別此欄是商品名／貼文摘要／店名。
--     為 NULL 時 publisher 不發這個鍵，FieldSpec.Resolve 回 (nil,false)，
--     BuildDataMap 直接略過，不會寫出空鍵。
UPDATE "notification_templates"
SET "data_fields" = '{
  "refType":      {"const": "DELEGATION"},
  "refId":        {"field": "audit_log_id"},
  "resourceType": {"field": "resource_type"},
  "resourceId":   {"field": "resource_id"},
  "resourceName": {"field": "resource_name"}
}'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'delegation.operation.completed.seller'
  AND "target_user_type" = 'seller';
