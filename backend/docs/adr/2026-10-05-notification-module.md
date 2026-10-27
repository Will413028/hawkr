# 通知模組：通知型別定義在程式碼、收件人是帳號加受眾、email 是交易渠道

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-05） |
| 日期 | 2026-10-05 |
| 影響範圍 | 新的 `internal/modules/notification`（取代 notification-service）、各模組的通知事件、`internal/legacy`（舊服務事件的明確訂閱）、website、兩支 App 的推播註冊與收件匣、`pkg/mail` |
| 計畫 | `docs/plans/2026-09-24-go-modular-monolith.md` A8 |
| 修訂其他 ADR | `2026-09-18-adm-delegation-suspension-and-notification.md` §2（文末修訂）、`2026-09-27-identity-clerk.md` 的 email 用途（文末修訂） |

## 脈絡

notification-service（設計書 12、13）的現況：

以下路徑相對 `backend/services/notification-service/`，另有標示的除外。

- 訂閱 15 個 exchange 的 `#`，以 (exchange, routing key) 查 DB 模板，查不到就以 debug log 丟掉（`internal/worker/consumer.go:41-57`、`internal/worker/template_handler.go:32-40`）：沒有模板的 39 個 routing key 也被收進來再丟掉（`backend/docs/plans/2026-09-24-go-modular-monolith/event-anomalies.md` §3）；該送卻送不到的在同檔 §1（例如 governance 的 5 個 key 送往沒有建立的 exchange、`job.parsed` 沒有消費者）。
- 模板在 `notification_templates`：自製的 `#{field|default}` 字串替換，沒有型別、payload 欄位錯了要到執行期才知道；唯一修改途徑是 Atlas migration，後台只剩開關、預覽、試發（`backend/gateway/internal/router/routes_admin_notification.go:11`）。
- 收件匣存渲染後的 title、body（`db/schema/schema.sql:15-34`）；沒有被刪除的列永久保留（`internal/worker/cleanup.go:69-72` 只清已軟刪除的）。
- 收件人以 surrogate 定址：buyer 存 buyer_id、seller 存 store_id；token 的唯一鍵是 (user_id, user_type, token)，同一支手機的 token 可以同時掛在兩個使用者下。
- 投遞：推播失敗只記 log 後 ACK；冪等檢查把失敗的也當成處理過，永遠不重送（`internal/usecase/send_push.go:44-58`）；收件匣與推播不是原子；公告只送給有裝置 token 的人（`internal/grpc/server_broadcast.go:83`）。偏好只擋推播、不擋收件匣。
- FCM 的錯誤分類是對的：只在 Unregistered、SenderIDMismatch 時停用 token（`internal/infrastructure/firebase.go:165-190`）；ThirdPartyAuthError 不停用 token，註解寫應該告警，但沒有實作。
- 偏好 10 類（`internal/domain/notification_preference.go:6-15`），`shipment_anomaly` 對賣家強制開啟。

約束：
- 外部：推播只有 FCM（APNs 經 FCM）。不得使用舊公司的 Firebase 專案、Resend 帳號與 `example.com` 寄件網域（單體 ADR `2026-09-24-go-modular-monolith.md` D9；Will 2026-10-05 確認 Resend 與網域屬於舊公司），新的推播專案、寄信服務帳號與寄件網域是本步驟的前置。
- 外部：登入只有 Google、Apple、LINE（ADR `2026-09-27-identity-clerk.md` D2）；LINE 不保證回傳 email，部分帳號沒有 email。Apple 私密轉寄地址只收已在 Apple Developer 登記的寄件網域。
- 外部：個資法第 20 條（為行銷利用個資時，當事人表示拒絕就停止）；Gmail、Yahoo 2024 年起的大量寄件者規範（SPF、DKIM、DMARC，行銷信符合 RFC 8058 一鍵退訂）。
- 繼承：還沒搬進單體的服務（Java 等）到各自的步驟前仍經 RabbitMQ 發事件，wire 格式與「發布失敗只記 log」都在邊界外。仍成立，舊事件能達到的可靠度受它限制。
- 繼承：一個帳號同時最多一家現行店、沒有店員（ADR `2026-06-29-identity-defer-store-entity.md`，重開條件在該檔）。仍成立；本決策的結構讓日後加店員只改一處。
- 不再成立、不遵守：DB 模板與以 migration 修改（公司時期營運要在後台改文案；沒有營運團隊、編輯路由已撤）、wildcard 訂閱、收件匣存渲染快照、surrogate 定址（微服務拿不到帳號）、「賣家通知只做站內」（前提是沒有寄信能力，見修訂）。

## 考慮過的方案

| 題目 | 方案 | 判定 |
|---|---|---|
| 通知的定義放哪 | 基準：交易型通知的內容與規則放在程式碼，隨版本審查與測試（Novu Framework 的 `workflow()`；GitHub、Discourse 的通知型別寫在程式碼）。後台可編輯的模板（Courier、Braze、Shopify 給商家的 Liquid 模板）是給有營運或行銷團隊頻繁改文案的組織 | ✓ |
| | DB 模板搬進單體 | ✗ 必要條件（營運編輯）不存在，只剩缺點 |
| | 程式碼定義結構、文案放 DB 可覆寫 | ✗ 同上，還多一個漂移來源 |
| 收件匣存什麼 | 基準一：存型別與資料、讀取時渲染（Discourse 的 `notification_type`＋`data`、GitHub 以 subject 參照） | ✓ 採折衷：參數存當下的不可變快照，讀取時只組文字、不回查資源 |
| | 基準二：存渲染好的快照（Knock、Novu 的 message） | ✗ 文案修正與日後多語系不能回溯，收件匣與程式碼文案不同源 |
| | 存型別＋ID、讀取時回查資源 | ✗ 通知是對過去事件的陳述，金額或名稱改了舊通知會跟著漂移；讀取要 N+1 解析 |
| 收件人與 token | 基準：收件人是人（帳號）；token 屬於一個 App 安裝、綁目前登入的帳號，登出解綁、換人登入改綁（FCM 的 token 管理建議）；同一人有多種身分時依受眾送到對應的介面（Uber 乘客與司機共用帳號、通知分開；Knock 的 tenant） | ✓ |
| | 維持 surrogate 定址 | ✗ 同一支手機換人登入會收到前一人的通知 |
| | 只用帳號、不分受眾 | ✗ 買家 App 會收到店家通知 |
| Email | 基準：電商對交易事件寄 email（Amazon、Etsy、蝦皮、momo），帳號與安全事件一定寄（Google、GitHub）；推播可以被拒絕、網頁沒有推播 | ✓ 只寄交易與安全類 |
| | 只做站內 | ✗ 網頁買家不開網站就不知道出貨、退款；關店後的帳務事件在 App 裡看不到 |
| | 交易與行銷都寄 | ✗ 需要同意紀錄、退訂與頻率控制，沒有營運需求前是空投資 |
| SMS、LINE | 基準：台灣電商的 SMS 用在 OTP 與取貨通知，日常通知靠推播、email、LINE 官方帳號；LINE Notify 2025-03-31 停止服務 | 一般通知不做；取貨簡訊見決策 9 |

## 決策

1. **通知型別定義在程式碼**：每一種通知是一個型別，宣告 typed 參數 struct、受眾、偏好分類、渠道（收件匣、可見推播、只帶資料的推播、email）、文案與深連結。zh-TW 文案集中在一個訊息目錄。每個型別有固定輸出的比對測試。`notification_templates` 表與設計書 12、13 的模板設計退場。
2. **明確訂閱**：單體內的模組以 in-process 事件觸發，在產生事件的 transaction 內排入 River job；舊服務的事件由 `internal/legacy` 逐一訂閱個別 routing key，不用 wildcard。每個被訂閱的事件都有對應的通知型別，測試檢查。`event-anomalies.md` §1 的送不到的事件，以及 `order.delivered`（沒有消費者）、`coupon.issued`（發布端是死碼）等矛盾，在 A8 開工時逐一裁決。
3. **收件人是帳號加受眾**：`(account_id, 受眾 = BUYER | STORE, store_id)`。店家通知在送出時解析「誰收這家店的通知」，今天只有店主帳號；日後有店員時只改這個解析。管理員不是收件人（業務告警維持 Discord 與後台佇列）；代行者不收店家的通知；只有聯絡方式、沒有帳號的外部收件人有兩種：vendor portal 的業者（邀請與 A8 加入的 email OTP，ADR `2026-10-03-vendor-portal-invite-scoped-access.md`）與匿名通報人（處理結果，ADR `2026-10-18-seller-governance-consolidated.md`，依 DSA Art.16 告知通報人）。匿名通報人留 email 時寄結果；只留電話時不通知，列為決策 9 的 SMS 候選。
4. **裝置 token**：以 token 為主鍵，記錄 App（buyer、seller）與綁定的帳號；登入時 upsert 改綁，主動登出時刪除；帳號停權（Clerk BanUser）與刪帳號在同一個 transaction 刪除該帳號的 token（被動登出時 App 不會呼叫刪除）；店家停權不動 token，店家仍要收到處分與申訴相關的通知。BUYER 受眾只推 buyer App，STORE 受眾只推 seller App。網頁的即時串流以 `(account_id, 受眾)` 訂閱，受眾由 website 的頁面決定（買家頁、賣家中心）；管理員的代行頁不訂閱店家的串流。即時串流依主計畫 D7 改成 Connect server streaming。
5. **收件匣**：存型別、參數（渲染當下的不可變值，例如金額、名稱快照）與 ID，讀取時只組文字。推播實際送出的文字存在投遞紀錄。收件匣有保留期（A8 定天數）後硬刪，舊型別的渲染程式保留到該型別的資料刪光。參數裡的個資逐欄寫明清除鍵：店家資料跟著 seller 關店後的個資清除以 store_id 清，買家資料以帳號清。店關閉後，舊店的通知留在原 store_id 底下，新開的店不繼承；關店後才發生的事件（退款、撥款）以 email 為主要渠道。
6. **公告是資料**：營運撰寫的公告一則一列存在 DB，收件匣以參照連結；收件匣的發送與推播分開，沒有裝置 token 的人也收得到。
7. **投遞**：每個（事件、收件人、渠道）一個冪等鍵；每個渠道一筆投遞紀錄，狀態為待送、已送、永久失敗；收件匣與推播是不同的 job，一個失敗不影響另一個；暫時性錯誤以指數退避重試。FCM 的錯誤分類沿用現行（只在 Unregistered、SenderIDMismatch 停用 token），ThirdPartyAuthError（APNs 憑證問題）補上告警。寄信 adapter 對外只回暫時性與永久性兩種錯誤，provider 的原始錯誤不往上丟：原樣上拋會讓呼叫端的重試長進某一家 provider 的狀態碼，換 provider 就要全部重寫。adapter 自己不重試，重試只在投遞 job；永久性錯誤直接記為永久失敗，持續重送死信箱會拉低寄件網域的信譽，而交易信共用同一個網域（沿用 vendor portal E1 設計書 §3.3，`backend/docs/superpowers/specs/2026-09-11-vendor-portal-e1-invite-email-design.md`（私人封存））。
8. **偏好與 email**：偏好是分類乘以渠道；哪些分類的收件匣不可關（交易必要）、哪些推播不可關（例如賣家的 `shipment_anomaly`）定義在程式碼。行銷性質的分類（現有的 coupon、restock、follower、post 在 A8 逐一歸類）預設開啟、可隨時關閉（opt-out，個資法第 20 條）；email 渠道跟著同一份偏好矩陣，交易必要的 email 不可關。email 只寄交易與安全類，初版清單：
   - 買家：訂單成立、被取消（賣家取消或逾時）、付款失敗或待付款即將逾期、出貨、可取貨、取貨即將逾期或已退回、退款結果。
   - 賣家：新訂單、即將逾時未出貨、退貨或爭議申請、撥款與結算、KYC、PayUni、7-ELEVEN 開通結果、停權與治理處分、代行完成。
   - 電子發票的通知信依 ADR `2026-09-18-invoice-service-architecture.md` 的 ADR-11 由平台寄送（不用 AMEGO 代寄），屬交易類，由本模組的 email 渠道投遞。行銷 email 不做。
   寄件地址是帳號的已驗證主要 email，由 identity 以 Clerk 的 `user.updated` webhook 同步（見 Clerk ADR 的修訂；不在寄信時呼叫 Clerk，寄信路徑不多一個外部依賴）；沒有 email 的帳號只收站內通知，註冊時不強迫補填。寄信的平台工作在 A8 完成：新的寄信服務帳號與寄件網域、SPF、DKIM、DMARC、退信與投訴 webhook 寫入 suppression list、Apple 私密轉寄的寄件網域登記。
9. **SMS、LINE、Web Push**：不做，渠道以 adapter 介面預留。例外候選是取貨簡訊：生鮮在超商逾期未取就是損失，以訂單上的收件人電話快照（Java order-service 的 `recipient_phone`，B 階段後是 order 模組的對應欄位；不是會被修改或刪除的買家地址簿）寄取貨通知屬於配送目的；A8 開工時查 7-ELEVEN 是否自行寄送取貨簡訊給收件人，不寄就在「可取貨」「取貨即將逾期」兩個型別加 SMS 渠道。

## 後果

- 通知文案、收件人與渠道只有程式碼一個來源，改文案要部署；沒有營運團隊之前這是想要的性質。
- 加新渠道只加 adapter；加店員只改店家受眾的收件人解析。
- 收件匣的渲染程式要保留到舊型別資料過期，保留期決定這個成本的上限。
- 舊服務發的事件仍是 fire-and-forget，通知的可靠度在那些服務搬遷前受它限制。
- 設計書 12、13 的模板、收件匣與 token 章節在 A8 改寫；主計畫「模板契約測試改成模組內 Go 測試」改成每個通知型別的比對測試。

## 待辦

- A8 開工前：開新的 Firebase 專案與本機推播替身、新的寄信服務帳號與寄件網域（Will）。
- A8 開工時定：收件匣保留天數；逐一裁決矛盾的事件；查 7-ELEVEN 取貨簡訊（決策 9）。
- 重新評估 SMS、LINE、Web Push 的觸發條件：A8 起記錄每個帳號的推播授權狀態、有沒有 email，以及每筆交易通知是否至少送達一個渠道；交易通知的未觸達率連續 4 週超過 10%，或取貨逾期退回率超過 A8 訂的門檻時重開。

## 修訂：D1 的訊息目錄有兩個語系（2026-10-17，Will 決定）

原決策段保留歷史。決定見 `2026-10-17-multi-locale-architecture.md`（D1、D3）。

- 決策 1 的「zh-TW 文案集中在一個訊息目錄」：改為每個通知型別的文案在集中的訊息目錄中有 `zh-TW`、`en` 兩份（ICU 受限子集，生成有型別的 Go 函式；英文內容在 `en` 發佈前可以先空）。仍是程式碼單一來源，仍有固定輸出的比對測試，測試涵蓋每個已發佈語系。
- 決策 5「收件匣存型別、參數，讀取時只組文字」：讀取時依讀者的語系組字；推播與 email 在送出時依收件人 `preferred_locale` 組字，投遞紀錄存實際送出的文字與 locale。推播文字由伺服器組好，不用 APNs `loc-key`／FCM `body_loc_key`。
- 通知中的 website 連結依收件人 `preferred_locale` 加語系前綴（`/en/…`）。
