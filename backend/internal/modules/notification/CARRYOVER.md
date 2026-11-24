# notification 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

來源：`backend/docs/plans/2026-11-23-a8-notification/inventory.md` §2–§6 每一列（機制欄開頭的 `§n#k` 是盤點的節與列號；§5.4 的 94 列模板，尚未搬進單體的發送端 72 列合成一列、其餘逐列），決定欄取自盤點的「去向」欄；計畫 `backend/docs/plans/2026-11-23-a8-notification.md` 的 D0–D16。本檔對應 A8-1a（骨架、schema、註冊表）；決定欄寫明步驟的項目在該步驟才實作。

## 沿用盤點

notification-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| §2#1 收件匣表 `notifications`：存渲染後的 title/body 與 `data` jsonb，收件人 `(user_id, user_type)`，軟刪（`deleted_at`） | client 要看到發送當下的文字；收件人身分空間與帳號無關即可 | 不成立：多語系（D1）要求讀取時依請求語系組字；渲染後文字含第三方名稱，個資清除做不到；`user_id` 混 buyer_id 與 store_id 兩個身分空間（缺陷 #12 的根源）；系統未上線，舊資料不搬 | 收件匣存事件型別與參數快照、讀取時渲染，保留期後硬刪（`baseline-brief.md` Q4、Q6） | 改寫成 新模組收件匣表（型別＋參數快照＋聚合鍵＋actor 清單＋已讀，讀取時依請求語系組字；A8-1）；舊資料不搬（系統未上線） |
| §2#2 投遞紀錄 `push_notifications`：`status` 含 `skipped`/`partial`，冪等用 `ExistsPushProcessed`（不分 status）以 `reference_id` 為鍵，空值即無冪等 | 要知道每筆推播有沒有送達、重送不重複 | 不成立：`reference_id` 可空造成無冪等（缺陷 #3、#4）；沒有 email 渠道、沒有永久失敗狀態；紀錄只在 failed/skipped 超過 30 天才刪，sent 永久保留（缺陷 #16） | 每渠道投遞紀錄是唯一帳本，UNIQUE (意圖, 收件人, 渠道) 當冪等鍵（`baseline-brief.md` Q1） | 改寫成 每渠道投遞紀錄（待送／已送／永久失敗、實際送出的文字與 locale，UNIQUE（意圖、收件人、渠道）；D10；A8-1、A8-3） |
| §2#3 裝置 token 表 `device_tokens`：UNIQUE (user_id, user_type, token)、`platform`、`is_active`、`updated_at` 兼作最後使用時間 | 一個 token 在一個時間點只該送給一個人；要能清掉不再使用的 token | 不成立：同一個 token 可同時掛在多個帳號下（缺陷 #13，登出後換人登入會收到前一人的通知）；`platform` 在 Android 移除後只剩 ios（D7）；`updated_at` 語意不清、清理靠 90 天殭屍判定 | token 為主鍵、綁一個帳號與一支 App，記最後註冊時間，過期清除（`baseline-brief.md` Q10） | 改寫成 以 token 為主鍵、綁一個帳號與一支 App、記 `last_registered_at`、無 `platform` 的裝置 token 表（D7；A8-1、A8-3） |
| §2#4 偏好表 `notification_preferences`：`(user_id, user_type, category, enabled)`，沒有列＝預設開，只擋推播 | 使用者要能關掉非必要通知；只存覆寫即可 | 部分成立：只存覆寫列的做法成立；偏好只擋推播、不擋收件匣與 email 不成立（D4 把收件匣與 email 納入偏好渠道）；分類不在清單內也能寫入（缺陷 #10、#11） | 偏好是 (分類, 渠道) 的覆寫列，不可關的分類由註冊表宣告（`baseline-brief.md` Q7） | 改寫成 偏好覆寫列 `(account_id, 受眾, 分類, 渠道, enabled)`，只存覆寫（D4；A8-1） |
| 每受眾偏好分類集合：`internal/model/model.go` 的 `categoryDefs`（店家限定 REVIEW、FOLLOW、PAYOUT、SHIPMENT_ANOMALY；買家限定 FOLLOWED_STORE_POST、COUPON、RESTOCK），沿用舊服務 `SellerCategories`／`BuyerCategories`（`services/notification-service/internal/domain/notification_preference.go:31-48`） | 偏好 API 只該提供該受眾真的會收到的開關 | 成立：每個分類的型別都只送給單一受眾（`inventory.md` §5） | 同：分類依受眾劃定範圍，偏好 API 只列相關開關 | 沿用；重新評估的時機：有型別需要對另一受眾使用某分類時，同一次變更擴充該集合（否則 Validate 會拒絕） |
| §2#5 通知模板表 `notification_templates`：94 列資料驅動（title/body 模板、payload_filter、image_key_template、push_mode…），5 分鐘快取 | 非工程人員要能不改程式碼調整通知文字 | 不成立：後台模板編輯、預覽、試發都不做（範圍）；模板是資料所以沒有型別檢查與固定輸出測試（缺陷 #4、#10、#23）；多語系文案放模組 `internal/text`（D1） | 通知型別是程式碼，文字走編譯期檢查的訊息目錄，每型別有固定輸出測試（`baseline-brief.md` Q3） | 刪除（通知型別定義在程式碼的型別註冊表，94 列模板退場；A8-1、A8-7） |
| §2#6 `system_notification_logs`：公告記錄表 | 後台公告要有發送紀錄 | 不成立：repository 與 domain 型別存在但沒有接進 DI、沒有任何 usecase 或 RPC 使用（死碼，缺陷 #22）；公告的資料模型隨 A8-2 重新設計 | 公告是資料，收件匣與推播分開 job（`baseline-brief.md` Q1） | 刪除（死碼，缺陷 #22；A8-7） |
| §3.1#1 消費 `aftersales.events` 的 `#` queue，查模板分流 | 售後事件要通知買賣雙方 | 不成立：A8 只接單體模組的事件；aftersales 還在 Java，從 A8-7 起到它搬進單體前沒有通知（Will 2026-11-23 接受） | 不寫 legacy 轉接，通知型別隨發送端搬遷（D13，`baseline-brief.md` 1(a)） | 隨發送端搬遷（D13） |
| §3.1#2 消費 `hawkr.buyer` 的 `#`：唯一模板 `product.favorited` | 收藏商品要通知賣家 | 不成立：D3 裁決收藏不做 | 有使用者需要知道且有發送端才做型別（D3） | 刪除（D13 不接 RabbitMQ 消費者；唯一模板 `product.favorited` 依 D3 不做；A8-7） |
| §3.1#3 消費 `hawkr.content` 的 `#`：10 個社群模板（留言、讚、追蹤、貼文、下架、瀏覽） | 社群互動要通知作者與追蹤者 | 部分成立：需求仍在，social 已在單體；MQ 消費者與 JSON payload 不成立；`product.viewed` 依 D3 不做 | 單體內以同 transaction 的同步訂閱者只排意圖 job（單體規則 4） | 改寫成 social 單體事件的同步訂閱者（`comment_created`、`comment_liked`、`comment_replied`、`store_followed`、`post_published`、`store_content_taken_down`、`post_liked`；`product.viewed` 不做，D3；A8-3） |
| §3.1#4 消費 `hawkr.delegation` 的 `#`：代行完成通知 | 賣家要知道代行者完成的操作 | 成立：delegation 已在單體；消費方式不成立（MQ） | 同步訂閱者排意圖 job | 改寫成 delegation 事件的同步訂閱者（`delegation_operation_completed`；A8-3） |
| §3.1#5 消費 `hawkr.im` 的 `#`：聊天離線推播 | 對方離線時推播新訊息 | 成立；但聊天訊息不寫收件匣（D3）、只推播；消費方式不成立（MQ） | 聊天走推播不入收件匣，抑制以深連結指向的聊天室判斷（D6） | 改寫成 `im.MessageSent` 的同步訂閱者（`chat_message`，只推播不寫收件匣；A8-3） |
| §3.1#6 消費 `hawkr.invoice` 的 `#`：13 個發票模板 | 發票開立與異常要通知賣家 | 不成立：invoice 還沒搬進單體 | 通知型別隨發送端搬遷（D13） | 隨發送端搬遷（D13） |
| §3.1#7 消費 `hawkr.marketing` 的 `#`：2 個行銷模板 | 行銷事件要通知買家 | 不成立：發送端還沒搬進單體 | 通知型別隨發送端搬遷（D13） | 隨發送端搬遷（D13） |
| §3.1#8 消費 `hawkr.merchant` 的 `#`：5 個商家認證模板（沒有任何發送端） | 商家認證結果要通知賣家 | 部分成立：核准與退回由單體的 seller KYC 事件承接；另三個 key 沒有發送端 | 有發送端才做型別（D3、D13） | 改寫成 seller KYC 事件訂閱（`kyc_approved`、`kyc_rejected`；A8-3）；`forwarded`、`pending_info`、`under_review` 三個 key 隨發送端搬遷（D13） |
| §3.1#9 消費 `hawkr.order` 的 `#`：7 個訂單模板 | 訂單狀態變更要通知買賣家 | 不成立：ordering 還沒搬進單體 | 通知型別隨發送端搬遷（D13） | 隨發送端搬遷（D13） |
| §3.1#10 消費 `hawkr.product` 的 `#`：補貨與商品認證模板 | 補貨要通知有意願的買家 | 部分成立：補貨由 `buyer.RestockTargeted`（單體）承接；商品認證三個 key 的發送端還在 Java | 補貨屬行銷分類，預設開、可關（D4） | 改寫成 `buyer.RestockTargeted` 訂閱（`product_restocked`；A8-3）；`product.certification.*` 三個 key 隨發送端搬遷（D13） |
| §3.1#11 消費 `hawkr.review` 的 `#`：評價建立通知賣家 | 賣家要知道有新評價 | 成立：review 已在單體；消費方式不成立（MQ） | 同步訂閱者排意圖 job | 改寫成 `review.ReviewCreated` 的同步訂閱者（`review_created`；A8-3） |
| §3.1#12 消費 `hawkr.seller` 的 `#`：店家停權與解除 | 店家停權要通知賣家 | 成立：seller 已在單體；消費方式不成立（MQ） | 同步訂閱者排意圖 job | 改寫成 `seller.StoreSuspended`、`StoreUnsuspended` 的同步訂閱者（`store_suspended`、`store_unsuspended`；A8-3） |
| §3.1#13 消費 `hawkr.shipping` 的 `#`：5 個出貨模板 | 出貨狀態要通知買賣家 | 不成立：fulfillment 與 Java shipping 還沒搬進單體 | 通知型別隨發送端搬遷（D13） | 隨發送端搬遷（D13） |
| §3.1#14 消費 `payment.events` 的 `#`：1 個付款模板 | 付款結果要通知 | 不成立：該 key 的發送端還在 Java | 通知型別隨發送端搬遷（D13） | 隨發送端搬遷（D13） |
| §3.1#15 消費 `procurement.events` 的 `#`：詢價邀請 | RFQ 發布要通知被邀請的賣家 | 成立：procurement 已在單體；消費方式不成立（MQ） | 同步訂閱者排意圖 job；`rfq_invited` 歸互動（D3） | 改寫成 `procurement.RfqPublished` 的同步訂閱者（`rfq_invited`；A8-3）；外部業者 key 隨發送端搬遷（D13） |
| §3.2#1 `buyer.deleted` 靜態 queue：以 accountId 呼叫 buyer、seller 兩個服務列舉歷史 id，紅線化 chat 收件匣並軟刪 | 買家刪帳後其收件匣的個資要清掉 | 成立（需求）；跨服務 gRPC 列舉歷史 id 不成立（單體同一資料庫）；只處理 chat 與軟刪（軟刪留下資料）不符 180 天硬刪（D2） | 訂閱擁有者的刪除事件，排 job 硬刪收件人的資料（A8-1c，被擋於 D16） | 已建（A8-1c）：訂閱 `buyer.BuyerDeleted` 的同步訂閱者只排 `notification.purge_recipient` job（帳號的 BUYER 受眾）；job 分批把該收件人的意圖參數換成 `'{}'`（經收件匣連結與投遞紀錄找到）、清掉投遞紀錄的送出文字與語系（列留著，D16）、刪收件匣列與帳號的 BUYER 偏好；worker 在帳號共享鎖之後以 `buyer.ShareLockByAccount`（`FOR KEY SHARE`）鎖買家列，買家已關閉不寫 |
| §3.2#2 `account.deleted` 靜態 queue：只做 chat 紅線化，不停用 token、不刪偏好 | 帳號刪除後不該再收到通知、偏好不留 | 成立（需求）；舊實作不完整（缺陷 #14） | 訂閱 `identity.AccountDeleted` 清收件匣與偏好，同 transaction 刪 token（A8-3） | 已建（A8-1c）：訂閱 `identity.AccountDeleted` 的同步訂閱者只排清除 job（帳號的全部受眾），清除內容同 §3.2#1，偏好兩個受眾都刪；token 的同步刪除在 A8-3 |
| §3.2#3 `seller.deleted` 靜態 queue：紅線化 chat、軟刪退休 store 的收件匣、停用 token | 店家退休後其收件匣個資要清掉 | 成立（需求）；軟刪不成立 | 訂閱 `seller.StoreRetired` 清收件匣參數與偏好（A8-1c） | 已建（A8-1c）：訂閱 `seller.StoreRetired` 的同步訂閱者只排清除 job（帳號的 STORE 受眾，收件匣、投遞紀錄、意圖以 `store_id` 限定那間店，同帳號的另一間店不動；偏好是帳號與受眾的、沒有店別，同帳號可能已重開另一間店，所以不刪偏好），清除內容同 §3.2#1；worker 以 `seller.LockNotRetired`（`FOR KEY SHARE`）鎖店家列，店家已退場不寫 |
| §3.3#1 發布 `notification.inbox.new` 給 gateway SSE（fire-and-forget、錯誤丟棄、含整個 item） | web 即時看到新通知 | 不成立：單體是單一行程，hub 用記憶體即可；串流只是訊號，資料由列表保證（缺陷 #19、#20） | 只傳失效訊號、不做續傳游標（D11） | 改寫成 單體行程內 hub 的串流失效訊號（未讀數與最新收件匣 ID，不經 MQ、不帶 item；D11；A8-2） |
| §3.3#2 發布 `notification.unread_changed` | 已讀、刪除後其他分頁更新未讀數 | 不成立：不經 MQ | 單體行程內 hub 的串流失效訊號，鍵 `(account_id, 受眾)`（D11） | 改寫成 單體行程內 hub 的串流失效訊號（未讀數與最新收件匣 ID，不經 MQ；D11；A8-2） |
| §4#1 FCM 可見推播（`SendEachForMulticast`，Notification payload 帶標題、內文、圖片，APNs sound） | iOS App 要收到可見推播 | 成立：兩支 App 只有 iOS；Firebase 專案要換新的（Will 的外部前置） | 可見通知（APNs alert）才不會被 iOS 節流（A8-6）；adapter 介面不綁 provider | 改寫成 FCM adapter 可見通知（APNs alert，FCM `notification` payload；A8-3、A8-6） |
| §4#2 FCM 無聲推播（data-only、`content-available`）給 14 個 silent 模板 | App 在背景重讀訂單、售後或賣家狀態 | 不成立：iOS 會節流 data-only 推播；需求改由可見推播 payload 的狀態重讀提示承載 | 狀態重讀提示 enum（ORDER、AFTER_SALE、SELLER_STATUS，可為空）放在推播與串流訊號（D6） | 刪除（App 改用可見通知，刪 data-only 推播路徑；狀態重讀改由推播 payload 的重讀提示 enum 承載，D6；A8-2、A8-6） |
| §4#3 FCM topic 廣播 `SendToTopic` | 對所有人廣播 | 不成立：`fcm_topic` 欄位已刪，只剩介面與停用的 stub（死碼，缺陷 #22） | 公告是資料，收件匣與推播分開 job | 刪除（死碼，缺陷 #22；A8-7） |
| §4#4 FCM 錯誤分類：只在 Unregistered、SenderIDMismatch 停用 token，依 token 值停用所有使用者列 | 死 token 不再重送 | 成立（分類）；依 token 值停用所有使用者列不成立（token 現在只綁一個帳號）；ThirdPartyAuthError 沒有告警 | FCM 官方做法：只在 Unregistered 類錯誤清 token，授權錯誤告警，每批最多 500 個 token（缺陷 #26） | 保留→FCM adapter（只在 Unregistered、SenderIDMismatch 停用 token，ThirdPartyAuthError 告警，每批最多 500 個 token；A8-3） |
| §4#5 Email（Resend，`pkg/mail`）：設定載入但沒有程式讀取，prod 啟動仍強制 `MAIL_PROVIDER=resend` | 交易必要通知與刪帳確認要能寄信 | 不成立：現況是死設定；寄件地址來源（Clerk webhook）已失效，改用 `identity.ContactEmail`；真實寄信等 Will 買網域 | 交易信專用子網域＋ provider adapter 加本機替身，只寄已驗證信箱、不寄 suppression 名單（D8） | 改寫成 Resend adapter 加本機替身，寄件對象只限 `identity.ContactEmail` 已驗證者（D8；A8-3）；舊 `MAIL_*`、`RESEND_API_KEY` 死設定刪除（A8-7） |
| §4#6 SMS：不存在 | （無） | 不成立：不做 SMS；取貨簡訊的前提隨 B3 刪超商取貨消失（D5） | 通知 ADR 決策 9：不做 | 不做（範圍：不做 SMS，通知 ADR 決策 9；取貨簡訊依 D5 刪除） |
| §4#7 LINE 通知：不存在（LINE 只用於登入） | （無） | 不成立：不做 LINE 通知 | 通知 ADR 決策 9：不做 | 不做（範圍：不做 LINE，通知 ADR 決策 9） |
| §4#8 Web Push：不存在（web 用 SSE） | （無） | 不成立：不做 Web Push；觸發重評的條件是未觸達率連續 4 週超過 10% | 通知 ADR 決策 9：不做 | 不做（範圍：不做 Web Push，通知 ADR 決策 9） |
| §4#9 站內收件匣 `notifications` 表當作渠道 | 使用者要有可回看的通知列表 | 部分成立：需求成立；把收件匣當投遞渠道不成立 | 收件匣不是渠道，收件匣列本身就是「已進收件匣」的紀錄，聚合時多個意圖合併成一列 | 保留→新模組收件匣（A8-1） |
| §4#10 即時（SSE）到 web：MQ→gateway SSE | web 不重整也看到未讀數變化 | 成立（需求）；MQ 轉接不成立 | Connect server streaming 失效訊號，客戶端收到後重拉列表（D11） | 改寫成 Connect server streaming 失效訊號（D11；A8-2） |
| §4#11 媒體 URL 組裝：`MEDIA_BASE_URL` ＋ 44 個模板的 `image_key_template` | 商品類推播要帶圖片 | 部分成立：需求成立；圖片 key 放在模板資料裡、有殘留佔位符就省略不成立 | 註冊表型別的可選圖片欄位，商品類型別帶圖，FCM payload 帶圖片 URL，iOS Notification Service Extension 下載（D14） | 改寫成 註冊表可選圖片欄位、商品類型別帶圖（D14；A8-1、A8-3） |
| §4#12 呼叫 buyer-service、seller-service gRPC（只為個資清除列舉歷史 id） | 刪帳時知道這個帳號曾經有哪些 buyer_id、store_id | 不成立：收件人現在以 `(account_id, 受眾, store_id)` 存，刪除事件直接帶帳號 | 同庫、事件驅動，不跨服務列舉 | 刪除（個資清除改為單體事件訂閱，不再跨服務 gRPC 列舉歷史 id；A8-1、A8-7） |
| §4#13 服務自己的 env（DB、gRPC port、otel 等） | 獨立部署的服務要自己的連線設定 | 不成立：單體是單一 binary、設定集中在 `cmd/hawkr` | 隨 notification-service 刪除 | 刪除（隨 notification-service 刪除；A8-7） |
| §5.1#1 以 `(exchange, routing_key)` 查模板，一個 key 可對買、賣各一張 | 事件要能對應到通知 | 不成立：不經 DB 模板；事件是單體的 typed 結構 | 型別註冊表以事件型別對應型別（程式碼） | 改寫成 型別註冊表以事件型別查型別（程式碼，不經 DB 模板；D13、A8-1） |
| §5.1#2 payload 以 `UseNumber` 解 JSON，保留 64 位 TSID（issue #1316） | MQ payload 的大整數不能失真 | 不成立：單體事件是 typed 結構、不解 JSON payload；意圖參數快照由型別自己的 typed 參數編解碼，ID 一律 typed | typed 事件，不做動態 JSON 欄位取值 | 刪除（單體事件是 typed 結構，不再解碼 JSON payload；A8-1） |
| §5.1#3 `payload_filter`：每個 key 等值（缺 key 不通過）或 `$ne`，值以字串比較 | 同一個 routing key 依欄位值分流成不同通知（例如 `MeasureCreated` 依 `Kind`） | 不成立：條件是資料而不是型別；字串比較沒有型別檢查（10 個啟用模板在用） | 型別內的 typed Go 條件，或依事件欄位分流成不同型別（A8-3） | 改寫成 型別註冊表依事件欄位分流型別（如 `MeasureCreated` 依 `Kind`、解除請求只送 `MeasureRequestDecided`，D3；A8-3） |
| §5.1#4 收件人：讀 `target_user_field`（number/json.Number/string 必須 >0），錯誤就略過這則 | 知道通知要給誰 | 不成立：欄位是資料、錯誤只 warn 略過；身分空間混用 | worker 解析收件人 `(account_id, 受眾, store_id)`，店家受眾的解析集中一處 | 改寫成 worker 解析收件人 `(account_id, 受眾 BUYER 或 STORE, store_id)`（A8-1） |
| §5.1#5 自我略過（見 §5.3） | 自己的操作不通知自己 | 成立（需求）；比對的身分空間不一致（缺陷 #12） | 以帳號比對的自我略過 | 改寫成 帳號比對的自我略過（缺陷 #12；A8-1） |
| §5.1#6 渲染：`#{field}`／`#{field\|default}`，缺欄位的 `#{x}` 原樣留在輸出 | 把事件欄位組成可讀的文字 | 不成立：缺欄位把佔位符送給使用者；只有 zh-TW（缺陷 #27） | 模組 `internal/text` 的訊息目錄（zh-TW、en），生成的函式參數有型別，每個型別有固定輸出比對測試（D1） | 改寫成 模組 `internal/text` 伺服器組字 zh-TW／en，每個型別固定輸出比對測試（D1；A8-1） |
| §5.1#7 data map：`data_fields` FieldSpec、固定 `type`、`category`、`url`（`{key}` 替換）、`imageUrl` | client 要拿到分類、連結與圖片 | 不成立：client 對 type 做互動分支的前提已核對（A8-0 U8，只有重讀狀態與聊天抑制）；資料欄位是模板資料 | 伺服器組好標題、內文、深連結與呈現分類 enum，推播與串流另帶狀態重讀提示（D6） | 改寫成 伺服器給的深連結＋呈現分類 enum＋狀態重讀提示 enum（D6；A8-1、A8-2） |
| §5.1#8 `reference_id_template`：渲染時忽略 missing 清單，空字串＝無冪等 | 同一事件重送不重複 | 不成立：缺欄位就沒有冪等（缺陷 #3、#4） | 型別必須宣告冪等鍵來源；意圖冪等鍵＝事件來源身分，註冊表自檢強制 | 改寫成 型別必須宣告冪等鍵來源（缺陷 #3、#4 消失；A8-1） |
| §5.1#9 `push_mode`：silent 不寫收件匣只送 data-only；其他任何值視為 visible（無 CHECK） | 有些事件只需要 App 重讀狀態、沒有可見文字 | 不成立：data-only 推播被 iOS 節流（見 §4 #2）；未知值被當 visible（缺陷 #24） | 型別註冊表宣告渠道（收件匣、推播、email）；沒有可見文字的型別不建，狀態重讀改用提示 enum（D6） | 改寫成 型別註冊表宣告渠道（收件匣、推播、email；`chat_message` 只推播，D3；silent 模式不再存在；A8-1） |
| §5.1#10 推播：冪等檢查→偏好→取 active tokens→FCM→依錯誤停用 token→寫投遞紀錄，同步在 consumer 內 | 送出推播並記錄結果 | 不成立：外部 provider 不在 DB transaction 內呼叫、要有帶冪等鍵的 job（單體規則 5） | 意圖 job 展開為每渠道投遞 job，偏好在 worker 套用（D10） | 改寫成 意圖 job 展開為每渠道投遞 job，偏好在 worker 套用（D10；A8-1、A8-3） |
| §5.1#11 收件匣寫入：`ON CONFLICT (user_id, user_type, event_type, reference_id) DO NOTHING`，新增才發 SSE | 同一事件不重複進收件匣；有新列才通知 client | 部分成立：冪等需求成立；以 reference_id 為鍵、讀不到時無冪等不成立；沒有聚合 | worker 寫收件匣，有聚合鍵時未讀列 upsert（D12）；冪等由意圖冪等鍵 UNIQUE 保證 | 改寫成 worker 寫收件匣，有聚合鍵時未讀列 upsert，冪等 UNIQUE（D12；A8-1） |
| §5.3#1 `comment.created` 自我略過：`commenterId` 比 `postAuthorStoreId` | 店家自己留言不通知自己 | 不成立：比對的是兩個欄位的值而不是帳號；actor 缺失時靜默不略過 | 型別宣告 actor 帳號 ID，worker 以帳號比對（缺陷 #12） | 改寫成 `comment_created` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| §5.3#2 `comment.created.buyer` 自我略過：`commenterPrincipalId`（賣家時是 account_id）比 `postAuthorId`（buyer_id） | 買家版留言通知 | 不成立：身分空間不一致，且貼文作者恆為店家、單體沒有買家版發送端（D3） | 不做此型別 | 刪除（`comment.created.buyer` 買家版不做：貼文作者恆為店家，單體無發送端，D3；A8-3） |
| §5.3#3 `comment.replied.seller` 自我略過：`replierId` 比 `replyToUserStoreId` | 自己回覆自己不通知 | 不成立：同 #1，用欄位值比對 | `comment_replied` 型別以帳號比對 | 改寫成 `comment_replied` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| §5.3#4 `comment.replied.buyer` 自我略過：`replierPrincipalId` 比 `replyToUserId` | 自己回覆自己不通知 | 不成立：身分空間不一致（賣家回覆時 account_id 對 buyer_id） | `comment_replied` 型別以帳號比對 | 改寫成 `comment_replied` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| §5.3#5 `comment.like_changed` 自我略過：`userId`（買家是 buyer_id）比 `commentAuthorStoreId` | 自己按讚自己的留言不通知 | 不成立：身分空間不一致 | `comment_liked` 型別以帳號比對；讚用聚合鍵（D12） | 改寫成 `comment_liked` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| §6#1 MQ 消費：`ResilientConsumer`，MaxRetries 3，republish 到 `{queue}.retry`，超過進 DLQ；模板 handler 只在 JSON 解碼失敗才回 error | 訊息處理失敗要重試、不丟 | 不成立：推播與收件匣失敗不走重試（缺陷 #1，吞錯誤後 ACK）；單體不經 MQ | River job 重試（D0b）；來源 transaction 只排一個意圖 job | 改寫成 單體事件同步訂閱者只排意圖 job，River 重試（D0b、D10；缺陷 #1；A8-1） |
| §6#2 模板快取刷新：5 分鐘一次，只換 in-memory map | 模板資料改動後生效 | 不成立：沒有模板資料（缺陷 #25） | 隨模板退場刪除 | 刪除（模板退場，沒有快取，缺陷 #25；A8-7） |
| §6#3 清理 job：每 6 小時、批次 1000 筆循環 | 舊資料要清除 | 成立（需求）；內容與期限不成立（見 3a–3c） | River 週期 job（`jobs.SweepOpts`），A8-1c | 改寫成 保留期清除 job（A8-1）；token 清除見 3c（A8-3） |
| §6#3a 刪 `push_notifications` status 為 failed/skipped 且超過 30 天 | 投遞紀錄不無限成長 | 不成立：sent 永久保留（缺陷 #16） | 投遞紀錄 180 天硬刪，與收件匣同一個清除 job（D15） | 改寫成 投遞紀錄 180 天硬刪，與收件匣同一 job（D15；A8-1） |
| §6#3b 刪 `notifications` 已軟刪且 `deleted_at` 超過 90 天 | 收件匣不無限成長 | 不成立：未刪的永遠保留（缺陷 #16） | 收件匣 180 天硬刪、不再軟刪（D2） | 改寫成 收件匣 180 天硬刪，不再軟刪（D2；A8-1） |
| §6#3c 停用 `device_tokens` is_active 且 `updated_at` 超過 90 天 | 清掉殭屍 token | 不成立：`updated_at` 語意不清，只停用不刪 | 每日 job 刪 60 天未註冊的 token（`last_registered_at`，D7，A8-3） | 改寫成 每日 job 刪 60 天未更新的 token（`last_registered_at`；D7；A8-3） |
| §6#4 重送／補送：無 | 投遞永久失敗後要有手段重送 | 成立（缺口）：舊系統沒有任何補送手段 | 依投遞紀錄重排永久失敗的手段與 runbook，加未觸達查詢（A8-3；通知 ADR 待辦「至少送達一個渠道」） | 改寫成 依投遞紀錄重排永久失敗的手段與 runbook，加未觸達查詢（A8-3） |
| §6#5 排程發送、聚合、節流：無 | 互動洗版（讚、追蹤）要收斂 | 成立（需求）；時間窗摘要沒有需求 | 型別宣告聚合鍵，未讀時合併成一列（D12）；不做每日摘要信 | 改寫成 聚合鍵未讀合併（D12；A8-1）；時間窗摘要不做（範圍）；提醒類事件的排程隨發送端搬遷（D13） |
| §6#6 冪等：推播靠 `ExistsPushProcessed`、收件匣靠 partial unique index，都以 `reference_id` 為鍵 | 同一事件不重複通知 | 不成立：非原子的 check-then-act，鍵可空（缺陷 #2、#3） | 意圖冪等鍵＝事件來源身分 UNIQUE；投遞冪等鍵＝（意圖、收件人、渠道）UNIQUE | 改寫成 意圖冪等鍵＝事件來源身分，投遞冪等鍵＝（意圖、收件人、渠道）UNIQUE（A8-1、A8-3） |
| §6#7 廣播：同步迴圈，每批 100 個 token、逐一經 SendPushUseCase | 後台公告要發給所有人 | 不成立：只送有 token 的人、同步逾時、計數不實（缺陷 #5–#7） | 公告是資料，收件匣與推播分開 job（A8-2、A8-3） | 改寫成 公告是資料，收件匣與推播分開 job（缺陷 #5–#7；A8-2、A8-3） |
| §5.4 72 個尚未搬進單體的發送端的模板（#1, #2, #3, #4, #5, #6, #7, #8, #9, #10, #11, #12, #13, #14, #15, #16, #17, #18, #19, #20, #21, #22, #23, #24, #25, #26, #27, #28, #29, #30, #31, #32, #33, #34, #35, #36, #37, #52, #53, #54, #55, #56, #57, #58, #59, #60, #61, #62, #63, #64, #65, #66, #68, #69, #71, #73, #74, #75, #76, #77, #78, #79, #80, #81, #82, #87, #88, #89, #90, #91, #92, #94；event_type 與 exchange 見盤點 §5.4） | 舊服務的事件（aftersales、invoice、order、shipping、marketing、payment、product 認證、merchant 三個 key 等）要通知買賣雙方 | 不成立：發送端還在 Java 或還沒搬的 Go 服務；A8 只接單體模組的事件，不經 `internal/legacy` 轉接（100% 重工）；從 A8-7 起到各發送端搬進單體前沒有這些通知（Will 2026-11-23 接受） | 通知型別隨發送端模組搬遷時定義（主計畫「模組完成的定義」；D13，`baseline-brief.md` 1(a)） | 隨發送端搬遷（D13） |
| §5.4 #38 `product.favorited`（seller，follower，visible） | 這個事件要通知 | 不成立：D3：收藏不做；A8-3 | 有使用者需要知道且有發送端才做型別（D3） | 不做（D3：收藏不做；A8-3） |
| §5.4 #39 `comment.created.buyer`（buyer，message，visible） | 這個事件要通知 | 不成立：D3：貼文作者恆為店家，單體無買家版發送端；A8-3 | 有使用者需要知道且有發送端才做型別（D3） | 不做（D3：貼文作者恆為店家，單體無買家版發送端；A8-3） |
| §5.4 #40 `comment.created`（seller，content，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `comment_created`（STORE、互動；A8-3） |
| §5.4 #41 `comment.like_changed`（seller，content，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `comment_liked`（BUYER 或 STORE、聚合鍵 `comment_like:{comment_id}`；D12；A8-3） |
| §5.4 #42 `comment.replied.buyer`（buyer，message，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `comment_replied`（BUYER 受眾；A8-3） |
| §5.4 #43 `comment.replied.seller`（seller，content，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `comment_replied`（STORE 受眾；A8-3） |
| §5.4 #44 `follow.added`（seller，follower，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `store_followed`（聚合鍵 `follow:{store_id}`；D12；A8-3） |
| §5.4 #45 `post.created`（buyer，content，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `post_published`（BUYER 追蹤者；A8-3） |
| §5.4 #46 `post.hidden_by_platform.seller`（seller，system，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `store_content_taken_down`（STORE、交易必要、`ACCOUNT`；A8-3） |
| §5.4 #47 `post.liked`（seller，content，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `post_liked`（聚合鍵 `post_like:{post_id}`；D12；A8-3） |
| §5.4 #48 `product.viewed`（seller，follower，visible） | 這個事件要通知 | 不成立：D3：`product.viewed` 不做；A8-3 | 有使用者需要知道且有發送端才做型別（D3） | 不做（D3：`product.viewed` 不做；A8-3） |
| §5.4 #49 `delegation.operation.completed.seller`（seller，system，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `delegation_operation_completed`（STORE、交易必要、`ACCOUNT`；A8-3） |
| §5.4 #50 `chat.message.sent`（buyer，chat，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `chat_message`（BUYER 受眾，只推播不寫收件匣；D3；A8-3） |
| §5.4 #51 `chat.message.sent`（seller，chat，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `chat_message`（STORE 受眾，只推播不寫收件匣；D3；A8-3） |
| §5.4 #67 `merchant.verification.approved`（seller，system，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `kyc_approved`（STORE、交易必要、`ACCOUNT`；A8-3） |
| §5.4 #70 `merchant.verification.rejected`（seller，system，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `kyc_rejected`（STORE、交易必要、`ACCOUNT`，帶退回理由；A8-3） |
| §5.4 #72 `notification.announcement`（seller，system，visible） | 後台對全體發公告 | 不成立：模板是標記列（disabled）、`hawkr.notification` 不被消費；公告是資料、派送是 Fanout 型別 | 公告是資料，收件匣與推播分開 job（A8-2、A8-3） | 改寫成 後台公告資料（註冊表的 Fanout 型別 `announcement`，不走意圖管線；收件匣與推播分開 job；A8-2c 建，推播渠道 A8-3） |
| §5.4 #83 `product.restock.target`（buyer，restock，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `product_restocked`（BUYER、行銷、`PROMOTION`；D4；A8-3） |
| §5.4 #84 `review.created`（seller，order_update，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `review_created`（STORE、互動、`REVIEW`；A8-3） |
| §5.4 #85 `seller.suspended.notice`（seller，seller_account，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `store_suspended`（STORE、交易必要、`ACCOUNT`；A8-3） |
| §5.4 #86 `seller.unsuspended.notice`（seller，seller_account，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `store_unsuspended`（STORE、交易必要、`ACCOUNT`；A8-3） |
| §5.4 #93 `procurement.rfq_publication_invite.created.seller`（seller，order_update，visible） | 這個事件要通知 | 成立（需求）；模板資料、字串欄位取值、`reference_id_template` 不成立 | 型別註冊表的 typed 型別：typed 參數、受眾、偏好分類、呈現分類、冪等鍵來源、zh-TW／en 文案（D1、D4、D6） | 改寫成 型別 `rfq_invited`（STORE、互動、`PROCUREMENT`；D3；A8-3） |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。A8-1a 沒有對外 API；16 支 RPC 加 SSE 的逐列盤點在 `inventory.md` §1，其去向欄由 A8-2 實作時逐列對照。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| gateway 對 notification-service 的 16 支 gRPC 與一條 SSE（`inventory.md` §1，含錯誤吞掉的呼叫與跨身分的串流） | 四個 client 要讀寫收件匣、偏好、token 與公告 | 不成立：舊 REST、手寫 client 與 gRPC 呼叫隨 A8-7 刪除；缺陷 #8、#11、#17、#18、#19 在舊傳輸層 | Connect service 加生成的 client，錯誤用 `ErrorReason`，分頁 cursor，串流只傳失效訊號 | 全部在 A8-2 重新設計，本步驟不新增對外方法；`inventory.md` §1 的去向欄為準 |

- 收件匣列表依 `updated_at` 排序分頁：聚合列合併新事件會更新 `updated_at`（依最後活動排，是有意的），所以翻頁途中被合併的列可能跳過或重複；client 收到串流失效訊號時從第一頁重新整理，不接著用舊的 page token。

## 意圖管線（A8-1b）

- 意圖冪等鍵＝型別名稱＋事件來源身分，由 `intents_idempotency_key_key` 與 `InsertIntent` 的 `ON CONFLICT` 保證；同一事件投遞兩次只有一個意圖、一個 job。守門：`TestSameEventTwiceMakesOneIntentAndOneInboxRow`。
- 同步訂閱者只准呼叫 `Accept`，`Accept` 只准插意圖列與排 job；A8-3 的裝置 token 同步刪除訂閱者要登記在 `subscribers_test.go` 的 `tokenPurgeSubscribers`（現在是空的）。守門：`TestSynchronousSubscribersOnlyAccept`、`TestAcceptOnlyInsertsTheIntentAndEnqueues`。
- 寫收件匣與投遞紀錄的 transaction 先以共享鎖鎖 identity 帳號列（`identity.ShareLockAccount`，`FOR SHARE`：只要帳號 commit 前不被刪，多個 worker 不互相等）、再碰 notification 的列（`platform/lockorder`）；帳號已刪除的意圖不寫任何列。守門：`TestIntentArrivingAfterAccountDeletionWritesNothing` 與 integration 測試的 lockorder checker。
- 偏好：不可關（`model.Disableable`）的渠道忽略覆寫列；可關的看覆寫列，沒有列就是預設開。自我略過比的是帳號（actor 帳號 ID 與收件帳號 ID），不是店或買家 ID。守門：`TestPreferencesDisableOnlyDisableableCategories`、`TestSelfNotificationIsSkipped`。
- 收件匣的聚合：有聚合鍵的型別，同一個收件人（含 store_id）未讀的同鍵列合併（actor 取聯集；參數只在來源事件時間 `occurred_at` 不早於該列目前的事件時才換，與 job 處理順序無關），已讀之後另開一列；聚合鍵與冪等鍵一樣以型別名稱為前綴，超過 200 字元改存 `<型別>:sha256`。worker 重試靠連結表 `inbox_item_intents`（意圖主鍵）判斷已寫過，合併進去的列之後被讀過也不會重複建列。守門：`TestUnreadNotificationsWithTheSameAggregationKeyMerge`、`TestRetryOfAMergedIntentDoesNotCreateARowAfterTheRowWasRead`、`TestMergeKeepsTheParamsOfTheNewerEventRegardlessOfProcessingOrder`、`TestSameRawAggregationKeyOfTwoTypesStaysTwoRows`、`TestLongKeysAreHashedToFit`。
- 收件人由型別的 `Recipient(參數)` 取出：事件的對象帳號與（STORE 受眾的）店 ID 在建立意圖時就寫進參數快照，worker 不再向 seller 讀「帳號目前的店」（事件發生當下的店才是收件人，之後換店不會改道）；`registry.Validate` 要求 STORE 一定有 store_id、BUYER 一定沒有。
- 投遞紀錄是推播與 email 的唯一帳本：worker 建 `PENDING` 的列（偏好關掉的渠道不建），並在同一個 transaction 為每筆新紀錄排一個 `notification.deliver` job（以投遞紀錄 ID 唯一；紀錄已存在的重試不再排）；這一步 job 不送出、紀錄留在 `PENDING`，A8-3 把 adapter 接進這個 job 並更新狀態，不另開第二份狀態、也不掃 `PENDING` 的列。守門：`TestDeliveryChannelsEachGetARecordAndAJobInTheSameTransaction`。

## 渠道 job 與 token（A8-3a）

| 新機制 | 必要條件 | 今天是否成立 | 決定 |
|---|---|---|---|
| FCM HTTP v1 adapter（`platform/fcm`，net/http 加 oauth2，只停用 UNREGISTERED、SENDER_ID_MISMATCH 的 token；單次呼叫不超過 500 個 token） | 要推播到 iOS，舊服務的 FCM SDK 與 Android 都不沿用 | 成立 | 不重試，結果分已送出、token 失效、暫時、永久、憑證錯誤，重試與停用由渠道 job 決定 |
| Resend adapter（`platform/resend`）與 Svix 簽章驗證 | 交易 email 與退信、投訴回報 | 成立 | 只回暫時性或永久性；冪等鍵用投遞紀錄 ID，job 當在送出後、寫回前時重試不重寄 |
| 渠道 job 三階段（鎖收件人準備、transaction 外呼叫 provider、再鎖一次記結果） | 單體規則 5（provider 不在 transaction 內）與 R1-4（清除後才執行的待送 job 不得寫回文字） | 成立 | 送出前收件人已不在：永久失敗不寫文字；送出後才不在：記已送出、不留文字 |
| `email_suppressions` | 寄給永久退信與投訴的位址會傷寄件網域信譽 | 成立（真實寄信延後到買網域，表與 webhook 先建） | D22：只存正規化位址的 HMAC-SHA256（`EMAIL_SUPPRESSION_HMAC_KEY`，輪替期間加 `_PREVIOUS` 一起查）、原因（`HARD_BOUNCE`、`COMPLAINT`）與時間，無明文位址、無限期保留；暫時與未定的退信不進名單；管理員只能經內部方法 `RemoveEmailSuppression` 移除 `HARD_BOUNCE`，投訴不可移除，投訴會把退信列升級成 `COMPLAINT`；webhook 驗簽後把事件縮成「原因加位址雜湊」才寫 `platform/inbox`（payload 從不含位址），inbox 依 `purge_after` 清除已處理的列；沒有密鑰時 dev 用固定 dev 密鑰、其他環境啟動失敗，email job 沒有密鑰不寄（守門：`TestSuppressionKeepsNoPlainAddressAnywhere`、`TestHardBounceIsRemovableButComplaintIsNot`、`TestSuppressedAddressIsNotEmailed`、`TestSuppressionKeysConfig`） |
| 停權、刪帳號的 token 同步刪除訂閱者 | 通知 ADR 決策 4：之後不能再有推播送到該帳號的裝置 | 成立 | 結構測試白名單，只准刪 token |
| 每日 60 天 token 清除 | App 每次啟動重新註冊，60 天沒註冊視為不用 | 成立 | `purge.StaleTokenDays` |

渠道 job 的不變量（守門測試在 `deliver_integration_test.go`、`suppression_integration_test.go`）：

- 三階段：transaction 內以 `intake.LockRecipient` 鎖收件人、依帳號 `preferred_locale` 組字、取 token 或已驗證信箱並查 suppression；transaction 外呼叫 provider；再鎖一次收件人並記結果。暫時性錯誤回錯誤讓 River 重試，最後一次嘗試改記永久失敗；推播至少一個 token 收下就算已送出。
- 送達保證：推播是 at-least-once（FCM 沒有冪等鍵；送出到定案之間程序死掉、或送到一半 ctx 取消而沒有任何 token 收下，重試會再推）；email 以投遞紀錄 ID 當 Resend 的 `Idempotency-Key`，不重複寄。送到一半取消但已有 token 收下時，`fcm.Send` 回傳部分結果、其餘記暫時性失敗，job 先定案 SENT；有 token 收下後先定案再刪失效 token，刪除失敗只記 log（`TestTokenDeleteFailureAfterASuccessfulPushStillSettlesSent`）。
- provider 金鑰被拒（FCM 401、403、`THIRD_PARTY_AUTH_ERROR`；Resend 401、403）是設定問題：log error 並重試，不記永久失敗。Resend 409 依錯誤名稱分：`concurrent_idempotent_requests` 暫時性、`invalid_idempotent_request` 永久。
- provider 替身只在 dev 環境（`requireDevEnvironment`）選得到；沒有設定 FCM 或 Resend 且不是 dev 環境時啟動失敗（`TestNotificationProviderFakesOnlyInDevEnvironment`），替身的假成功不會把正式環境的投遞標成 SENT。
- 推播圖片網址由 worker 以 `MEDIA_ORIGIN` 與 `rpc.PushImagePreset`（`card`）組（ADR 圖片 URI 樣板 D6；worker 與 serve 都要設 `MEDIA_ORIGIN`）。
- 送出前收件人已刪除、關閉或退場：永久失敗、不寫文字、不呼叫 provider。
- 推播目標只看受眾對應的 App（BUYER 受眾只推 BUYER App、STORE 受眾只推 SELLER App）；只有 UNREGISTERED、SENDER_ID_MISMATCH 的 token 被刪；憑證錯誤 log error 並重試。
- 推播 payload：可見 notification（收件人語系）、apns `mutable-content: 1`、有圖時 `apns.fcm_options.image`；data key 只有 `deep_link`、`state_refresh_hints`（逗號分隔的 proto enum 名稱）、`inbox_item_id`，沒有值就省略，常數在 `internal/deliver/payload.go`（`TestPayloadGolden`、`TestPushMessageGolden`）。
- 停權與刪帳號同 transaction 刪帳號的全部 token（店家停權不動）；`last_registered_at` 超過 60 天的由每日 job 刪。
- 只寄已驗證的信箱，位址的雜湊在 suppression 名單內不寄；退信與投訴 webhook（`POST /webhooks/resend`）先寫 `platform/inbox`（縮減後的 payload，無位址），只有永久退信與投訴進名單。

帳號 `preferred_locale`：identity 擁有（`identity.accounts.preferred_locale`，多語系 ADR D1），渠道 job 經 `Locales` 介面讀（`cmd/hawkr` 接 `*identity.Module`）。「最後一次生效的 UI 語系」的更新點：App 登入（`ExchangeIdToken`）與 `RegisterDeviceToken`（App 每次啟動、回前景都呼叫）以請求協商出的語系更新，只在與目前的值不同時寫（`TestRegisterDeviceTokenRecordsTheRequestLocaleForLaterDeliveries`）。網頁的 OAuth 回呼請求不帶 UI 語系，不更新。

## 治理與交易側型別（A8-3c）

- 舊服務的治理模板（`seller_account`、`content` 等）從未建立，這批型別是新的：店家面（停權、措施、KYC、詢問、撥款、代行）、買家作者（隱藏、還原、警告）、檢舉人（確認與結果）、刪帳確認信、詢價邀請；事件裁決見 `backend/docs/plans/2026-11-23-a8-notification/event-adjudication.md`。
- 一個意圖型別只有一個受眾、Spawned 型別沒有自己的冪等鍵：檢舉人是買家或店家，所以 `report_received`、`report_resolved` 各拆成買家與店家兩個型別（參數同為 `ReportSource`），`report_resolved` 一律經 fan-out 母意圖建立。計畫 D24 寫「統一為 `report_received`／`report_resolved`」，這裡是統一參數與文案、型別名稱依受眾拆開。
- 措施請求的裁定依 D3 一律通知 `store_measure_request_decided`（接受與駁回，文案依裁定）；接受申覆的撤銷與接受解除請求的結束由事件本身辨認（`MeasureRevoked.Reason` 是 `APPEAL_ACCEPTED`、`MeasureEnded.RequestID` 非零，seller 為此在 `MeasureEnded` 加 `RequestID`），intake 不另登記，同一次處置只通知一則；不需要 worker 端讀 seller。
- 補件期限提醒由 payment 的每小時掃描發布 `PayoutDeadlineApproaching`、notification 以「店家＋期限＋檔位」去重，取代計畫 D23 的「每日 job 排 `payout_account_deadline`」：期限在 payment 的表裡，notification 不讀別的模組的表。
- 結案的檢舉人 fan-out 的游標是檢舉人號碼（buyer_id 或 store_id，`Resolver.CaseReporters`）：買家與店家的母意圖各讀 moderation 自己那一邊（`ReporterSide`），一案一位檢舉人一筆（UNIQUE (case_id, reporter_*_id)）讓號碼可當 keyset，每頁從游標接著讀，不從頭走。
- `rfq_invited` 的偏好分類是 `PROCUREMENT`（D29：店家、預設開、可關）；`delegation_operation_completed` 的參數按計畫保留 `Capability`，文案只用動作（`Action`）。

## 與 舊服務的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。對外看得到或 schema 上的差異：

- 收件人是 `(account_id, 受眾 BUYER 或 STORE, store_id)`，取代 `(user_id, user_type)`；舊表混用 buyer_id 與 store_id。
- 收件匣存型別、參數快照、聚合鍵與 actor 帳號 ID 清單，讀取時組字；不存渲染後的標題與內文，不軟刪，180 天硬刪。
- 意圖表（冪等鍵 UNIQUE）與每渠道投遞紀錄（UNIQUE 意圖、收件人、渠道）取代 `push_notifications`；投遞紀錄只記推播與 email，收件匣列本身就是收件匣的紀錄。
- 裝置 token 以 token 為主鍵、綁一個帳號與一支 App，沒有 `platform`、`is_active`，記 `last_registered_at`。
- 偏好是 `(account_id, 受眾, 分類, 渠道, enabled)` 覆寫列；分類是 D4 的 14 個，渠道含收件匣與 email，不可關的規則在 `internal/model`。
- 通知型別、文案、深連結、聚合鍵、冪等鍵來源與渠道在 `internal/registry` 與 `internal/text`，沒有模板表；兩個型別 `account_suspended`、`account_unsuspended` 是舊模板沒有的新型別（舊模板只有店家版）。
- 已刪除的公告可能被派送期間重試的頁 job 重新建出收件匣或投遞紀錄（窗口只在派送進行中的重試）；接受，不記帳。
