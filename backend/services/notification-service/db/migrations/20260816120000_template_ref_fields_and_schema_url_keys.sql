-- 收件匣跳轉目標的 class 級修復（13 號 §5.2／§6.4；承 20260816093000 代行那一支）。
--
-- 同一個 root cause 有兩個外殼，兩個都是「模板產出的 data map 少了某個下游要讀的鍵」，
-- 而且兩者都不會有任何錯誤訊號 —— 通知照送、內文照對、outbox done，只有點下去沒反應。
--
--   ① data_fields 缺 refType/refId ⇒ GET /app/notifications 對該則回 refId=null、
--      refType=null。gateway 的 refType/refId **不是**讀 notifications.reference_id 欄位，
--      是讀 data map 裡的同名鍵，且**兩者皆非空**才回傳
--      （gateway/internal/handler/notification_inbox.go:41-64）。
--      實查全表 77 支 enabled template，19 支缺這組（代行那支已於前一個 migration 修掉）。
--
--   ② schema_url 的 {brace} 鍵名與 data_fields 的鍵名對不上 ⇒ RenderBraces 找不到 key 時
--      **原樣保留 `{key}`**（internal/worker/template_handler.go:376-380），於是送出去的
--      data.url 是一個沒渲染的模板字串。實查 57 支 schema_url 含 `{` 的 enabled template，
--      **23 支的 url 渲染後仍含 `{`**（不只 aftersales，訂單全系列與貼文三支都是）。
--      （實查基數：enabled 且 schema_url 非空 58 支，其中含 `{` 的 57 支，可行動的 23 支。）
--
-- 兩者都補上之後，「BuildDataMap 產出的 data 對每個 enabled template 都要交得出
-- 非空 refType+refId（白名單內）且渲染後的 url 不得含 `{`（全表）」由
-- internal/worker/template_contract_test.go 的第 6～8 條釘住。
--
-- ══════════════════════════════════════════════════════════════════════
-- Part A：19 支缺 refType/refId 的處置（15 支補、4 支刻意不給）
-- ══════════════════════════════════════════════════════════════════════
--
-- refType 一律取自**既有值域**，不新創值（現行值域＝ORDER / AFTERSALE / CHAT / POST /
-- PRODUCT / COUPON / SHIPMENT_ANOMALY / SELLER_SUSPENSION / MERCHANT_VERIFICATION /
-- DELEGATION）。refId 一律取自 publisher **實際會發**的欄位（逐支對過 publisher 原始碼，
-- 不是只對 golden —— golden 的 gate 只驗「template 讀的 ⊆ golden」，不驗
-- 「golden ⊆ publisher 實發」，所以 golden 有可能列了發不出來的鍵）。
--
-- ⚠️ 三個消費者都查過，沒有一支會因為新增 refType 而走到錯誤的跳轉分支：
--   • seller App inbox（notifications_screen.dart:233-273）：switch 只有 ORDER / CHAT /
--     AFTERSALE / AFTER_SALE / REFUND，**無 default** ⇒ POST / PRODUCT /
--     MERCHANT_VERIFICATION 一律 no-op（跟現在的 null 一樣不跳，不會跳錯）。
--   • buyer App inbox（notifications_screen.dart:295-318）：同上三個 case，AFTERSALE
--     走 /settings/after-sale/detail 帶 caseNo —— 正是本次 3 支買家售後模板要的目的地。
--   • 兩支 App 的 FCM router（seller push/router.dart:44-45、buyer push/destination.dart:53-58）
--     取 key 的順序都是 `data['type']` 優先、refType 只是 fallback，而模板一律會寫
--     data['type'] ⇒ FCM 路由完全不受影響。
--   • website（lib/notifications/resolve.ts:41/62/84）把 refType 轉小寫後比對三個集合；
--     "post" 已在 POST_REF_TYPES 內，但這批貼文/留言事件的 `type` 本來就命中
--     `type.includes("comment")` / `startsWith("post_")`，所以 isPostNotification 前後
--     都是 true；"product" / "aftersale" / "merchant_verification" 三個值不在任何集合，
--     href 前後皆為 undefined。零行為變化。
--   • 唯一可見差異是 App 收件匣的**圖示**：seller `_iconConfig` 第 2 段（refType 匹配）
--     原本因 refType=null 落到第 3 段的標題關鍵字猜測，現在 POST/PRODUCT→貼文圖示、
--     AFTERSALE→退款圖示。這是修正不是回歸（與既有 33 支售後模板一致）。

-- A1. 售後 5 支：refType=AFTERSALE、refId=case_no，並補 caseNo（schema_url 要用）。
--     publisher 對帳：CaseTransitionPayload / CaseAutoEscalatedPayload 皆為
--     @JsonNaming(SnakeCase) 的 record，case_no 為必填 component 恆非空
--     （aftersales-service mq/payload/CaseTransitionPayload.java:30、
--      CaseAutoEscalatedPayload.java:18）。
--     這 5 支的 schema_url 是 `hawkr://aftersales/{caseNo}`，其中 3 支
--     （auto_completed_return_pending_timeout ×2、return_pending_24h）的 data_fields
--     整個是 NULL ⇒ 現在送出去的 url 是字面 `hawkr://aftersales/{caseNo}`。
--     補 caseNo 之後才真的渲染得出來 —— 只補 refId 不補 caseNo 是修一半。
--     ⚠️ 這 3 支**不補 orderNo**：publisher 確實有發 order_no，但 golden 沒登記，
--     而 refType/refId 與 url 都用不到它；為了不讀沒登記的鍵而讓 gate 誤紅，不加。
UPDATE "notification_templates"
SET "data_fields" = coalesce("data_fields", '{}'::jsonb) || '{
      "refType": {"const": "AFTERSALE"},
      "refId":   {"field": "case_no"},
      "caseNo":  {"field": "case_no"}
    }'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" IN (
        'aftersales.case.auto_completed_return_pending_timeout.buyer',
        'aftersales.case.auto_completed_return_pending_timeout.seller',
        'aftersales.case.auto_escalated.buyer',
        'aftersales.case.auto_escalated.seller',
        'aftersales.case.return_pending_24h.buyer');

-- A2. 貼文/留言 3 支（camelCase wire）：refType=POST、refId=postId。
--     publisher 對帳：pkg/mq/events.go CommentCreatedEvent.PostID `json:"postId"`（:270）、
--     CommentLikeChangedEvent.PostID `json:"postId"`（:432）、
--     CommentRepliedEvent.PostID `json:"postId"`（:357）—— 三者皆為 int64 必填欄位。
--     取 postId 而不取 commentId：買家側的對應模板（comment.created.buyer /
--     comment.replied.buyer）早就是 refType=POST + refId=postId，賣家側對稱即可；
--     且 POST 這個 refType 的語意就是「哪一篇貼文」，留言 id 另外放在 data.commentId。
UPDATE "notification_templates"
SET "data_fields" = coalesce("data_fields", '{}'::jsonb) || '{
      "refType": {"const": "POST"},
      "refId":   {"field": "postId"}
    }'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" IN ('comment.created', 'comment.like_changed', 'comment.replied.seller')
  AND "target_user_type" = 'seller';

-- A3. post.liked（snake wire）：同 A2，但 publisher 的欄名是 post_id
--     （pkg/mq/events.go PostLikedEvent.PostID `json:"post_id"`，:337）。
UPDATE "notification_templates"
SET "data_fields" = coalesce("data_fields", '{}'::jsonb) || '{
      "refType": {"const": "POST"},
      "refId":   {"field": "post_id"}
    }'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'post.liked' AND "target_user_type" = 'seller';

-- A4. 商品互動 2 支：refType=PRODUCT、refId=product_id。
--     publisher 對帳：product.viewed → pkg/mq/events.go ProductViewedEvent.ProductID
--     `json:"product_id"`（:543）；product.favorited **不是**綁在同名 routing key 上，
--     它綁 exchange=hawkr.buyer / routing_key=buyer.favorite.added
--     （20260624160650_rebind_product_favorited_to_buyer_favorite_added.sql），
--     payload 是 BuyerFavoriteEvent.ProductID `json:"product_id"`（events.go:563，string）。
--     PRODUCT 這個值已由 product.restock.target 在用（refId 同樣是 product_id），一致。
UPDATE "notification_templates"
SET "data_fields" = coalesce("data_fields", '{}'::jsonb) || '{
      "refType": {"const": "PRODUCT"},
      "refId":   {"field": "product_id"}
    }'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" IN ('product.favorited', 'product.viewed')
  AND "target_user_type" = 'seller';

-- A5. 實名認證 4 支：refType=MERCHANT_VERIFICATION、refId=merchant_id。
--     publisher 對帳：seller-service internal/mq/merchant_event_publisher.go:29 的
--     MerchantVerificationCompletedEvent.MerchantID `json:"merchant_id"`（int64，恆非空；
--     UseNumber 解碼後 toStringVal 轉字串）。整個 payload 只有 event_id / status /
--     reason / merchant_id 四個鍵，**沒有 certification_id 或 application_id** ——
--     gateway §6.4 的 legacy fallback（certification_id → MERCHANT_VERIFICATION）
--     因此從來沒有任何 publisher 餵得動它。
--     ⚠️ 誠實記錄一個限制：merchant_id ＝ 收件人自己的 account_id（它同時是本列的
--     target_user_field）。之所以仍然給，是因為「認證資料」與商家是 1:1（sellers 只有
--     一筆 current_certification_id），merchant_id 就是該資源的自然鍵；而賣家端的目的地
--     `/seller/certification` 是「登入者自己的認證頁」，本來就不吃路徑參數。
--     **refId 在這 4 支用來指名資源，不是拿來當頁面參數** —— 前端不應該拿它去打
--     任何 /certifications/{id} 形狀的 API（沒有那支 API）。
UPDATE "notification_templates"
SET "data_fields" = coalesce("data_fields", '{}'::jsonb) || '{
      "refType": {"const": "MERCHANT_VERIFICATION"},
      "refId":   {"field": "merchant_id"}
    }'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" IN (
        'merchant.verification.approved',
        'merchant.verification.forwarded',
        'merchant.verification.pending_info',
        'merchant.verification.rejected')
  AND "target_user_type" = 'seller';

-- A6. 刻意不給 refType/refId 的 4 支（**不是漏掉**，理由逐支寫在下方；
--     template_contract_test.go 的 deliberatelyNoRefContract 會斷言它們維持沒有，
--     免得下一個人「順手補齊」時硬塞一個值域外的 refType）：
--
--   • notification.system.maintenance / buyer
--   • notification.terms.update / buyer
--       這兩支是**廣播模板**（fcm_topic 非空、target_user_field 為空），走
--       broadcastTemplate 而不是 handleTemplate（template_handler.go:60-64），
--       **從頭到尾不會寫任何 inbox row**。refType/refId 是收件匣 API 的契約欄位，
--       對一則永遠不進收件匣的通知沒有意義。它們的目的地也不是資源：系統維護沒有
--       落地頁，條款更新的 schema_url 是固定路徑 /settings/terms。
--
--   • notification.announcement / seller
--       公告確實有資源 id（publisher 發 announcementId），但**值域裡沒有一個
--       refType 描述得了「公告」**。
--       ⚠️ 硬塞 POST 的實際受害者是 **website 賣家面**，不是 buyer App（這是一支
--       seller-only 模板，buyer App 根本收不到）：website 的 `isPostNotification`
--       比對 `POST_REF_TYPES`（`lib/notifications/resolve.ts:70-90`）會因 "post" 轉真，
--       接著 `resolvePostId`（:94-116）在 data 找不到 postId 時**直接把 refId 當 postId
--       回傳**，`resolvePostHref`（:145-156）於是產出
--       `/seller-center/posts?post=<announcementId>` —— 一篇不存在的貼文。硬塞其他值
--       同理。要給就得新增 ANNOUNCEMENT 值 —— 那是 13 號 §5.2 的值域變更，要連
--       gateway fallback 表、兩支 App 的 switch、website 的集合一起評估，不是本次
--       修復的範圍。announcementId 已在 data 內，前端要用拿得到。
--
--   • follow.added / seller
--       「被誰追蹤」的資源是那個**買家**，而值域裡沒有 BUYER / USER / FOLLOWER。
--       （seller App 的 _iconConfig 有 'USER' / 'FOLLOWER' 兩個 case，但那是前端
--        自己預留的圖示分支，後端從未產出過這兩個值，不能拿它當值域依據。）
--       publisher 只發 follower_id / follower_name / merchant_id，其中 merchant_id
--       又是收件人自己 ⇒ 沒有可用且語意正確的組合，維持 null。

-- ══════════════════════════════════════════════════════════════════════
-- Part B：23 支 url 渲染不出來的 schema_url（20 支改鍵名 + 3 支已由 A1 補鍵解決）
-- ══════════════════════════════════════════════════════════════════════
--
-- 修法一律是**把 {brace} 的鍵名改成 data_fields 已有的那個鍵**，而不是新增一個
-- snake_case 別名鍵：brace 名是模板內部的東西、改它不影響任何人；多塞一個 data 鍵
-- 則會進到每一則 FCM payload（有 4KB 上限），而且會改變 App 端
-- `_firstNonEmpty(['order_id','orderSn','orderNo','refId'])` 這類查找的優先序。
--
-- 今日行為風險：零。data.url 現在**沒有任何做得了事的消費者** —— gateway 的
-- inboxItemDTO 沒有 actionUrl 欄位、website 的 resolveNotification 讀的是
-- item.actionUrl（resolve.ts:293）、buyer App 的 destination.dart 從不讀 'url'；
-- 唯一會用的是 seller App FCM router 的 default 分支（router.dart:185-194）把
-- data['url'] 丟給 launchUrl，而 seller App 的 Info.plist 只註冊了 GoogleURLSchemes、
-- **沒有註冊 hawkr scheme** ⇒ canLaunchUrl 回 false，修前修後都是 no-op。
-- 修它是為了讓 notifications.data.url 這個已經落庫的欄位是對的，等前端接落地頁時
-- 不必再回來補一次 migration。

-- B1. 售後 1 支用了 snake 的 {case_no}，其餘 33 支都是 {caseNo}（data 鍵名亦為 caseNo）。
UPDATE "notification_templates"
SET "schema_url" = 'hawkr://aftersales/{caseNo}',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "schema_url" = 'hawkr://aftersales/{case_no}';

-- B2. 訂單全系列 12 支：data 鍵是 orderId（值為 order_no），brace 卻寫 order_id。
UPDATE "notification_templates"
SET "schema_url" = 'hawkr://orders/{orderId}',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "schema_url" = 'hawkr://orders/{order_id}';

-- B3. 物流異常：data 鍵是 orderNo，brace 寫 order_no。
UPDATE "notification_templates"
SET "schema_url" = 'hawkr://orders/{orderNo}',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "schema_url" = 'hawkr://orders/{order_no}';

-- B4. 優惠券到期：data 鍵是 couponId（值為 coupon_instance_id），brace 寫 coupon_id。
UPDATE "notification_templates"
SET "schema_url" = 'hawkr://coupons/{couponId}',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "schema_url" = 'hawkr://coupons/{coupon_id}';

-- B5. 貼文 3 支：data 鍵是 postId，brace 寫 post_id。
--     ⚠️ 這裡**逐支列 event_type**而不是照 schema_url 值整批改：
--     post.hidden_by_platform.seller 的 schema_url 也是 hawkr://posts/{post_id}，
--     但它的 data 鍵剛好就叫 post_id、現在渲染得出來，整批改會把它從對的改成錯的。
UPDATE "notification_templates"
SET "schema_url" = 'hawkr://posts/{postId}',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "schema_url" = 'hawkr://posts/{post_id}'
  AND "event_type" IN ('comment.created.buyer', 'comment.replied.buyer', 'post.created');

-- B6. 到貨通知：schema_url 寫成 `#{deepLink}`，但 schema_url **只經過 RenderBraces**
--     （template_handler.go:145），而 RenderBraces 遇到 `#{` 會刻意跳過交給
--     RenderTemplate 的語法（:362-366）—— 而 schema_url 從來不經過 RenderTemplate。
--     ⇒ 這個佔位符無論如何都不會被替換，送出去的 url 恆為字面 `#{deepLink}`。
--     data 鍵是 deep_link（publisher buyer-service 發 deep_link，golden 已登記）。
UPDATE "notification_templates"
SET "schema_url" = '{deep_link}',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "schema_url" = '#{deepLink}';

-- ══════════════════════════════════════════════════════════════════════
-- Part C：A1 帶出的一個副作用，在同一批修掉（§2.1「同批修改的兩部分互相抵銷」）
-- ══════════════════════════════════════════════════════════════════════
--
-- A1 給 5 支售後模板補了 refId=case_no。其中**恰好一支**因此讓 seller App 多打一次
-- 注定 404 的請求：
--
--   `aftersales.case.auto_completed_return_pending_timeout.seller` 的 category 是
--   `order_update` ⇒ seller App 的 `_isOrderNotification`（notifications_screen.dart:718-724）
--   為真（它 OR 了 category.contains('ORDER')）。為真之後畫面會用
--   `_orderNoFromNotification`（:703-716）取到的值去 prefetch 買家名稱／頭像
--   （`_notificationBuyerMetaProvider(orderNo)`，:369-376）。
--   而該函式只在 refType=='ORDER' 時才用 refId，否則走
--   `['orderNo','orderSn','order_sn','orderId','refId','ref_id']` 的順序查找 ——
--   這一列的 data 沒有前四個，**第五順位 refId 就被當成訂單號**（值其實是 caseNo）。
--   補 refId 之前該查找回空字串、prefetch 不會發生；補了之後每列多一次 404。
--   畫面結果不變（名稱／頭像本來就抓不到），但那是「錯得剛好看不出來」。
--
-- 為什麼只有這一支：其餘 30 餘支售後模板的 data 都帶 orderNo，查找在第一順位就命中
-- 真正的訂單號；兩支買家向的 `..._timeout.buyer` / `return_pending_24h.buyer` 的
-- category 是 `refund_update`、event_type 也不含 ORDER ⇒ `_isOrderNotification` 為假。
-- （判定 query 見本檔 commit 說明；實查全表 enabled row 僅此一筆命中。）
--
-- 修法取「補 orderNo」而不是「改 category」：category 決定使用者的通知偏好分組，
-- 動它會改變賣家能不能關掉這類通知；而補 orderNo 讓這一列與它的 30 餘支同型模板
-- 一致，順帶讓 prefetch 真的抓得到買家資訊。
--
-- publisher 對帳：`ReturnPendingTimeoutCloseService.autoClose`
-- （aftersales-service service/ReturnPendingTimeoutCloseService.java:110-119）呼叫
-- `tryPublishCaseTransitionWithContext(..., caseNo, entity.getOrderNo(), ...)`，
-- 而 `CaseTransitionPayload` 是 @JsonNaming(SnakeCase) 的 record、`orderNo` 為必填
-- component（CaseTransitionPayload.java:31）⇒ wire 上恆有 `order_no`。
-- golden 先前只登記了 case_no / seller_account_id（golden 的 gate 只驗
-- 「template 讀的 ⊆ golden」，不驗「golden ⊆ publisher 實發」，所以少列不會轉紅），
-- 本次同 PR 補上 order_no —— 這正是 golden README「改 template 若讀了新 key 必須
-- 同步補、且該 key 必須是 publisher 真的會發的」要求的流程。
UPDATE "notification_templates"
SET "data_fields" = coalesce("data_fields", '{}'::jsonb) || '{"orderNo": {"field": "order_no"}}'::jsonb,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'aftersales.case.auto_completed_return_pending_timeout.seller'
  AND "target_user_type" = 'seller';
