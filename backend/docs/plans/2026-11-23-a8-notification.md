# 遷移計畫 A8：notification 搬進單體

**Goal：** notification-service 由 `internal/modules/notification` 取代，依 ADR `docs/adr/2026-10-05-notification-module.md`（以下稱「通知 ADR」，含 2026-10-17 多語系修訂）。完成後（可驗證的終態見 A8-9）：

- 通知型別定義在程式碼：typed 參數、受眾、偏好分類、渠道、zh-TW／en 文案、深連結，每個型別有固定輸出的比對測試；`notification_templates` 與 94 列模板退場。
- 收件人是 `(account_id, 受眾 BUYER|STORE, store_id)`；裝置 token 以 token 為主鍵、綁一個帳號與一支 App。
- 投遞：每個（事件、收件人、渠道）一個冪等鍵、每渠道一筆投遞紀錄（待送／已送／永久失敗），收件匣、推播、email 各自是 River job，暫時性錯誤指數退避重試。
- 單體模組的事件在產生的 transaction 內排入通知 job；還沒搬的服務（Java、invoice-service、ai-service）的事件由 `internal/legacy` 逐一訂閱個別 routing key，不用 wildcard。
- 對外 API 是 Connect（`api/` 的 notification service），即時串流是 server streaming（主計畫 D7）；四端改用生成的 client。
- 刪除：notification-service、gateway 的 `/app/notifications*`、通知偏好與後台通知路由、`openapi.yaml` 對應 operation、`backend/api/proto/notification` 與 Java 副本、`socialrelay`、`reviewrelay`（notification 那一半）、`imrelay`、`delegationservice/events.go` 的通知 wire、`pkg/mq` 的 social 常數。

**上層：** 主計畫 `2026-09-24-go-modular-monolith.md` 的 A8 段與「模組完成的定義」；本檔只寫 A8 特有的步驟與驗收。進度寫在本檔，主計畫 A8 段只指向這裡。

**附件：** `2026-11-23-a8-notification/`，盤點快照 dev `894625b5`。檔案異動：A8-1 到 A8-5 在 `backend/internal/modules/notification`、`api/` 新增；在事件來源模組、`internal/arch/modules.go`、`internal/legacy`、`internal/httpapi` 修改；A8-4、A8-8 刪除列出的 relay 與舊服務；A8-6、A8-7 修改四端並刪手寫呼叫。

| 檔 | 內容 |
|---|---|
| `inventory.md` | 16 支 RPC＋SSE 與四端呼叫點、6 張表、消費 18／發布 2 個事件、渠道 13 列、94 列模板（重放 migration 量得）、背景工作 10 列、缺陷 #1–#27、單體橋接 8 套件與預留事件 12 項、守門測試 4 組、不確定 U1–U13 |

- 盤點由唯讀 subagent 起草，每項附 `file:line`；主 session 抽查了：缺陷 #1、#10、#19 的描述與 ADR 脈絡一致；identity 已有寄件地址 API（`identity.go:255` `ContactEmail`，只寄 `ContactEmailVerified`）。盤點的「去向」欄由本檔步驟填，衝突時以本檔為準。

## 前提（核對日 2026-11-23，dev `894625b5`）

- **會改到或取代的部署是否在線**：不在線。系統從未上線（根目錄 `AGENTS.md`）；不受回滾窗口、資料搬移、雙跑約束；舊 `hawkr_notification` 資料不搬（收件匣、token、偏好在上線前都是測試資料）。已確認。
- **寄件地址**：主計畫 A8 段「identity 接 Clerk `user.updated` webhook」已失效：Clerk 已移除（`identity/db/migrations/20261030171858_drop_clerk_user_id.sql`，ADR `2026-10-30-identity-self-hosted-federated-login.md`），identity 在每次登入以 provider 回報的 `email`／`email_verified` 更新寄件地址，`identity.ContactEmail` 只把 `ContactEmailVerified` 當寄件地址。A8 直接用它；發票寄送地址（主計畫 A8 段第 1 條）同用這支 API。已確認【量：讀碼】。收尾時通知 ADR 加修訂、主計畫刪 Clerk 那一條。
- **沒有 Apple 登入**（根目錄 `AGENTS.md` 身分驗證列）：通知 ADR 決策 8 的「Apple 私密轉寄寄件網域登記」不做。已確認。
- **外部前置（只有 Will 能做）**：新的 Firebase 專案（推播；不得用舊公司的）、寄信服務帳號與寄件網域（SPF、DKIM、DMARC）。擋 A8-3 的「接真實 provider」驗收與 A8-7 的真機驗收，不擋其他步驟：本機與 CI 用推播與寄信的替身。未確認（D8）。
- **舊服務是否實際在跑**（U1）：不影響做法（不在線、不雙跑），只影響「現在哪些通知已經送不到」。不查。
- **還沒搬的服務仍經 RabbitMQ 發事件**，wire 與「發布失敗只記 log」在邊界外（通知 ADR 繼承約束）。仍成立。
- **一個帳號最多一家現行店、沒有店員**（ADR `2026-06-29-identity-defer-store-entity.md`）。仍成立；店家受眾的收件人解析集中一處。
- **工具鏈**（預設，可推翻）：照根目錄 `AGENTS.md`；文案目錄照共用文案目錄的生成規則（`backend/internal/platform/uitext`）或模組 `internal/text`，由 A8-1 依多語系 ADR 的「目錄分界」選定，不另立第三種。

**驗收基線：** A8-0 對當時 HEAD 跑 backend `make lint-monolith`、`make vet-all`、`make test-all`、notification-service 的 unit 測試與四端的 CI 等價檢查，記在文末「基線」；既有失敗不在本階段修。CI 一次只推一批。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 通知 job 與產生事件的寫入在同一個 transaction（`events.Publish` 的同步訂閱者在同一個 tx 內 `jobs.Enqueue`） | 各模組的事件發布點（social、review、im、seller、payment、accountsuspension、delegation、procurement、moderation；刪除類由 identity、buyer、seller 的刪除事件觸發）、A8-4 的訂閱器 |
| 冪等鍵 =（事件來源身分、收件人、渠道） | 投遞紀錄 UNIQUE、legacy 訂閱器（RabbitMQ 至少一次投遞）、重送 runbook |
| 收件匣只存型別＋參數快照，讀取時組字 | 收件匣讀取 API、型別渲染程式的保留（型別資料刪光前不得刪渲染）、個資清除鍵 |
| BUYER 受眾只推 buyer App、STORE 只推 seller App | `device_tokens.app`、推播 job、即時串流訂閱鍵 `(account_id, 受眾)` |
| 封鎖雙方之間不送互動通知 | 原 `socialrelay` 的封鎖檢查與 A7-3 的測試，搬到 notification 建立通知處 |
| 自我通知以帳號比對（不比 buyer_id／store_id） | 型別宣告的 actor 欄位、建立通知處的略過規則（缺陷 #12） |
| email 只寄 `identity.ContactEmail` 的 `ContactEmailVerified` | email job、發票寄送 |
| 依賴方向：notification 依賴事件來源模組的公開事件型別，來源模組不 import notification | `internal/arch/modules.go:42-45` 的 notification 列；A8-1 加 `accountsuspension`、`moderation`（停權事件、檢舉處置事件），其餘來源已在列 |
| notification 的同步訂閱者只寫 notification 自己的表與排 job，不鎖其他模組的列 | A8-1 新增 `notification` 的 lockorder 測試：在 accountsuspension、moderation、social 的處置 transaction 內觸發通知訂閱者，斷言 notification 訂閱者只取 notification schema 的鎖（照 `accountdeletion_lockorder_integration_test.go` 的做法）；A8-4 每加一個來源模組就加一個案例 |

## 規模（搬遷）

| | 數 | 來源 |
|---|---|---|
| RPC＋SSE | 16＋1 | `inventory.md` §1 |
| 表 | 6 | §2 |
| 消費的 routing key（有模板） | 94 列模板（92 啟用），涵蓋 73 組 (exchange, key) | §3.4、§5.4 |
| 單體送出的 route | 22 | §8.1 |
| relay 套件 | 7（`bizcalendarrelay` 無關） | §8.2 |
| 預留給 A8 的單體事件 | 12 項 | §8.3 |
| client 呼叫點 | website、buyer-app、seller-app 各一組；admin（公告、模板管理） | §1.1 |

## 範圍

- 做：模組本體與資料模型；Connect API（收件匣、未讀數、已讀、刪除、偏好、裝置 token、即時串流、後台公告）；推播（FCM）與 email adapter 和替身；單體模組事件的 in-process 訂閱；還沒搬的服務事件的 legacy 逐一訂閱；四端改 Connect；主計畫 A8 段列的治理、KYC、收款帳戶、停權、檢舉結果通知；投遞重送手段與 runbook；刪舊服務與橋。
- 不做：
  - 行銷 email、SMS、LINE、Web Push（通知 ADR 決策 9；取貨簡訊依 D5）。
  - 後台模板編輯、預覽、試發（模板退場；缺陷 #21 隨之消失）。
  - 舊資料搬移（不在線）。
  - 食安事件通知購買者：前提「查得到購買者」在 B8（主計畫 A8 段）。
- 延後：
  - 食安事件通知近 6 個月購買者。觸發：B8 ordering 搬進單體。
  - 賣家瀏覽／追蹤通知的節流與聚合（`buyer/CARRYOVER.md:65`）。觸發：出現追蹤者超過 1,000 的店家，或收到洗版回報。
  - SMS、LINE、Web Push 重新評估。觸發：通知 ADR 待辦的未觸達率門檻（連續 4 週超過 10%）。

## 步驟

- [ ] **A8-0 基線與事實查核**（被擋於：無）
  - 範圍：跑驗收基線（backend：`make lint-monolith`、`make vet-all`、`make test-all`；backend Makefile 沒有 `make check`）；查 7-ELEVEN 是否自行寄取貨簡訊給收件人（D5 的事實，查公開文件與 `backend/docs/第三方整合/` 的 7-ELEVEN 總覽）；讀 `event-anomalies.md` §1–§3 與盤點 §3.4，產出「事件裁決表」草稿（每個 routing key：訂閱並對應哪個型別、不訂閱附理由），交 D3；查 U10（seller workspace 即時通道 `hawkr.seller_workspace` 是否與通知串流共用路徑或 hub；共用時在 A8-2 把它一起搬到 Connect 串流或拆開，寫進 A8-2 範圍）、U12（代行者能否呼叫通知路由，交 D9）與 U8（client 對 `type`／`refType`／`data.*` 的分支），寫進附件。
  - 消費端：無（唯讀）。
  - 驗收：基線結果與四份產出在附件；事件裁決表與 `inventory.md` 去向欄無空格（新增一次性腳本 `2026-11-23-a8-notification/check_dispositions.py`，只在本階段用，不進 `scripts/`）。
  - 停止條件：基線有會擋 A8 的失敗（例如 River 或 Connect 基礎設施壞掉）。

- [ ] **A8-1 模組本體（walking skeleton）**（被擋於：A8-0、D1、D2、D4）
  - 範圍：`internal/modules/notification`：schema（收件匣、投遞紀錄、裝置 token、偏好、公告）；型別註冊表（typed 參數、受眾、分類、渠道、文案、深連結）；文案依 D1 的位置生成 zh-TW／en；建立通知的入口（冪等、偏好、自我略過、封鎖略過）；收件匣 job；保留期清除 job（D2）；個資清除：訂閱 `identity.AccountDeleted`（`identity/events.go:9`，以帳號清買家參數與 token）、`buyer.BuyerDeleted`（`buyer/events.go:25`）、`seller.StoreRetired`（`seller/events.go:31`，以 store_id 清店家參數）；accountdeletion 不加事件、不 import notification。`internal/arch/modules.go` 的 notification 列加 `accountsuspension`、`moderation`（理由寫在行尾註解）。一個型別端到端：`accountsuspension.Suspended` → 收件匣（最小、已有事件、沒有舊 wire）。`CARRYOVER.md` 沿用盤點表（盤點 §2、§6 每列）。
  - 消費端：`rg -n "accountsuspension\.(Suspended|Unsuspended)" backend/internal`；`internal/arch/modules.go`。
  - 不能動：accountsuspension 的事件型別與發布 transaction。
  - 驗收：每個型別的固定輸出比對測試（zh-TW、en）；integration：三個刪除事件後參數與 token 被清；同一事件投遞兩次只有一列（冪等）、偏好關閉的分類不建收件匣但必要分類仍建、自我通知被略過、保留期到期列被硬刪；`make lint-monolith`、`make test-all`、`GOWORK=off go vet -tags integration ./...`。mutation（進 `mutation/`）：拿掉冪等鍵 UNIQUE → 冪等測試紅；把自我略過改回比 buyer_id → 自我略過測試紅；必要分類改成可關 → 必要分類測試紅。
  - 停止條件：型別註冊表無法同時表達通知 ADR 決策 1 的所有屬性而不加執行期反射。

- [ ] **A8-2 Connect API 與即時串流**（被擋於：A8-1、D6）
  - 範圍：`api/` 新增 notification service：收件匣列表（cursor 分頁、分類篩選，`total` 跟著篩選）、未讀數、已讀（不存在或非本人回 NotFound，缺陷 #18）、全部已讀、刪除、偏好讀寫（受眾與分類白名單一個來源，缺陷 #9–#11）、裝置 token 註冊與刪除（`ios`、`android` 依 D7）、即時串流 server streaming（`(account_id, 受眾)` 訂閱、續傳游標、每帳號串流上限，缺陷 #19、#20）、後台公告（建立、列表；公告是資料，送給受眾全體，不只有 token 的人，缺陷 #5–#7）。收件匣回應帶伺服器組好的標題、內文、型別 enum 與深連結（D6）。BFF 代理與 `httpapi` 的停權放行、限流豁免一起改（守門測試 §8.4 #4）。
  - 消費端：`rg -n "notifications|notification-preferences|device-token" website/ admin/ buyer-app/lib seller-app/lib backend/gateway backend/internal/httpapi`。
  - 不能動：舊 REST 在 A8-8 前仍可用（client 還沒遷）。
  - 驗收：`buf lint`、`buf breaking`（新 service）、TS／Dart 生成無 drift；Connect 特性測試涵蓋每支方法；串流 integration：建立通知後收到事件、斷線以游標續傳不漏、另一受眾收不到。mutation：串流鍵拿掉受眾 → 跨受眾測試紅；`MarkAsRead` 改回非本人也成功 → NotFound 測試紅。

- [ ] **A8-3 推播與 email 渠道**（被擋於：A8-1、D8；接真實 provider 的驗收另擋於 Will 的帳號）
  - 範圍：FCM adapter（錯誤分類沿用：只在 Unregistered、SenderIDMismatch 停用 token；ThirdPartyAuthError 告警；每次最多 500 個 token 分批，缺陷 #26）；email adapter（對外只回暫時性／永久性，不重試；退信與投訴 webhook 寫 suppression list）；兩者的本機替身；推播 job、email job（送出時依 `preferred_locale` 組字、投遞紀錄存實際文字與 locale）；token 綁定規則（登入 upsert 改綁、登出刪、帳號停權與刪帳號同 transaction 刪；店家停權不動）；投遞重送手段（重排永久失敗紀錄的後台操作或指令）與 runbook。
  - 消費端：`rg -n "BanUser|accountsuspension|accountdeletion" backend/internal/modules --type go -l`（token 刪除掛點）。
  - 驗收：integration（替身）：暫時性錯誤重試後成功只送一次、永久性錯誤記永久失敗不重試、Unregistered 停用 token、BUYER 受眾不推 seller App、帳號停權後 token 消失、suppression 名單內不寄；`ReportReceived` 確認信地址在 job 完成與丟棄後不在參數裡（主計畫 A8 段，`seller/events.go`）。mutation：永久錯誤改成暫時 → 不重試測試紅；拿掉受眾 App 過濾 → 受眾測試紅。真實 provider：Will 帳號就緒後各送一則到測試裝置與信箱，結果貼進度表。
  - 需要人做的事：Firebase 專案、寄信帳號、寄件網域 DNS（Will）。

- [ ] **A8-4 單體模組的 in-process 訂閱**（被擋於：A8-1、D3、D4；推播與 email 渠道另擋於 A8-3）
  - 範圍：依 D3 的事件裁決表，為每個單體事件定義型別並訂閱：social（貼文、留言、讚、追蹤）；moderation 新增事件（消費者此時出現，照「消費者出現時再加事件」）：內容被隱藏、作者收到警告（Q6）、案件結案（帶每位檢舉人帳號，A7-D20），在 moderation 處置的同一個 transaction 發布、review、im（離線推播）、seller（檢舉收到與結果、詢問事件：此時才加 `CaseInquiryCreated`、措施與治理、KYC 審核結果、`MeasureRequestSubmitted` 依 D3）、payment `PayoutAccountChanged`（含首次啟用時間、補件期限欄位與期限前提醒，主計畫 A8 段）、accountsuspension 停權與解除、delegation 代行完成、procurement `RfqPublished`、帳號刪除的個資清除。封鎖略過搬進建立通知處，帶 A7-3 的同一個測試。刪除 `socialrelay`、`imrelay`、`reviewrelay` 的 notification 一半、`delegationservice/events.go` 的通知 wire、`sellerrelay` 對 notification 的部分、相關 wire／golden 測試、`pkg/mq` social 常數。
  - 消費端：`rg -n "socialrelay|imrelay|reviewrelay|sellerrelay|delegationservice" backend/`；`rg -n "hawkr\.(content|im|review|delegation)" backend/ --type go`。
  - 不能動：各模組事件型別的既有欄位（只加不改）；`reviewrelay` 給 Java product 評分聚合的那一半（A6 延後項，觸發 catalog 搬遷）。
  - 驗收：測試證明每個被訂閱的事件都有對應型別、每個型別都有訂閱或明列為公告／手動（通知 ADR 決策 2）；integration：每個型別一個「事件 → 收件匣」案例；封鎖雙方不送；relay 刪除後 `rg` 0 hit。mutation：拿掉封鎖略過 → A7-3 測試紅；移除一個訂閱 → 覆蓋測試紅。
  - 完成後的狀態：社群、評價、聊天的通知改由新模組送；舊服務只剩 legacy 事件。

- [ ] **A8-5 還沒搬的服務的 legacy 訂閱**（被擋於：A8-4、D3）
  - 範圍：`internal/legacy` 對 D3 判定要訂閱的每個 routing key（Java order、aftersales、shipping、product、coupon、payment，invoice-service，ai-service）各一個明確 consumer，解碼成 typed 參數後走同一入口；冪等鍵取自事件 ID 或 D3 寫明的來源身分；取貨簡訊依 D5。發票寄送 email 的地址來源改呼叫 `identity.ContactEmail`。
  - 消費端：`rg -n "routing_key|RoutingKey" backend/internal/legacy --type go`；`backend/internal/legacy/rabbitmq/testdata/routes.json`；topology 定義。
  - 驗收：每個 key 的 wire 解碼測試以 `backend/tests/contract/golden/event_payload_keys.json` 為 golden；重複投遞只建一筆；topology 測試：沒有 `#` 綁定。mutation：冪等鍵改成隨機 → 重複投遞測試紅。
  - 停止條件：某個 key 的 golden 與實際發送端不一致（U4、U5）且無法從發送端原始碼裁決。

- [ ] **A8-6 website 與 admin**（被擋於：A8-2）
  - 範圍：新增 website 的串流續傳測試（斷線後帶游標重連不漏通知）；website 收件匣、未讀數、即時串流、偏好改用 `@hawkr/api` 的 Connect client，刪手寫呼叫與 SSE hook；admin 公告頁改 Connect，刪模板管理頁。UI 改動只限跟隨契約（版面不動）。
  - 消費端：`rg -n "notification" website/ admin/ -g '!node_modules' -l`。
  - 驗收：website、admin CI 等價檢查（lint、typecheck、unit）；website e2e 交給 CI（不在本機跑）。mutation：串流續傳游標不帶 → 續傳測試紅。

- [ ] **A8-7 buyer-app 與 seller-app**（被擋於：A8-2、D7；真機推播驗收另擋於 Firebase 專案）
  - 範圍：兩支 App 的收件匣、偏好、device token 註冊（含 android 依 D7）、登出刪 token 改用 `packages/hawkr_api` 的 Connect client；換新 Firebase 專案設定。每支 App 一個 session。
  - 消費端：`rg -n "notification|device_registrar|device-token" buyer-app/lib seller-app/lib`。
  - 驗收：analyze、format、custom_lint、生成檢查與改動到的測試檔（完整 `flutter test` 交 CI）；真機：登入收到推播、登出不再收到、換帳號登入不收前一人的通知。

- [ ] **A8-8 刪除舊服務與舊路由**（被擋於：A8-4 到 A8-7）
  - 範圍：（全部是刪除或修改，沒有新增）刪 `backend/services/notification-service`、compose 項目、gateway 通知路由與 handler、`openapi.yaml` 對應 operation、`backend/api/proto/notification` 與 Java `java-proto` 副本（先 diff 確認無其他使用，U11）、`internal/legacy` 中只為舊通知存在的 client 與守門測試（§8.4 #1、#2）、`notifyAllowlist`、prod 設定裡沒人讀的 `MAIL_*`／`RESEND_API_KEY`（缺陷 #22）。
  - 消費端：`rg -n "notification-service|hawkr_notification|notificationpb|notification\.proto" --hidden -g '!.git'`。
  - 驗收：上述 `rg` 0 hit（附指令與輸出）；賣家工作台即時通道的既有測試仍綠（`rg -n "seller_workspace" website/lib backend/gateway` 對應的測試，U10）；全部 workflow 的 CI 等價檢查；gateway 對 openapi 的對齊檢查通過。

- [ ] **A8-9 收尾**（被擋於：A8-8）：主計畫「模組完成的定義」逐項核對；`inventory.md` 去向欄與缺陷 #1–#27 處置無空格（`check_dispositions.py`）；design-review skill 對整體 diff；長期檢查（訂閱↔型別覆蓋、無 wildcard 綁定）接進 CI；通知 ADR 加修訂（Clerk 改為 identity 登入同步、刪 Apple 私密轉寄、D 題結論）；設計書 12、13 的模板、收件匣、token 章節改寫；主計畫 A8 段打勾並移除過時的 Clerk 條；延後項附觸發搬回主計畫；需要人做的事列給 Will。

## 既有缺陷處置

依 `inventory.md` §7 編號；「新設計消失」指新模組的結構讓它不可能發生，由對應步驟的測試守。

| # | 處置 | 步驟 |
|---|---|---|
| 1 吞錯誤後 ACK、2 冪等非原子、3 無冪等鍵、4 冪等鍵殘留字面 | 修：投遞紀錄＋冪等鍵 UNIQUE＋River 重試（通知 ADR 決策 7）；型別宣告冪等鍵來源，沒有就編譯不過 | A8-1、A8-3 |
| 5–7 廣播分頁、同步逾時、計數不實、只送有 token 的人 | 修：公告是資料，收件匣與推播分開 job（決策 6） | A8-2、A8-3 |
| 8 gateway 丟 Code、9 白名單不一致、10 分類不在清單、11 未驗證受眾、24 `push_mode` 無 CHECK | 新設計消失：Connect 錯誤、分類與受眾只在型別註冊表一處定義 | A8-1、A8-2 |
| 12 自我略過比錯身分空間 | 修：以帳號比對（Will 2026-11-17） | A8-1 |
| 13 token 同機多人、14 刪帳號只清 chat、15 只收 iOS | 修：token 主鍵與綁定規則、個資清除鍵（決策 4、5）；Android 依 D7 | A8-1、A8-3 |
| 16 永久保留 | 修：保留期（D2） | A8-1 |
| 17 `total` 不受篩選、18 已讀無 NotFound | 修 | A8-2 |
| 19 串流跨身分、20 串流 best-effort | 修：`(account_id, 受眾)`、server streaming 與游標 | A8-2 |
| 21 試發與正式路徑不同、22 死碼死設定、23 孤兒模板、25 行程內模板快取 | 刪除：模板退場；孤兒事件由 D3 裁決 | A8-0、A8-8 |
| 26 FCM 500 上限 | 修：分批 | A8-3 |
| 27 只有 zh-TW | 修：zh-TW／en 文案（多語系修訂） | A8-1 |

## 決定

- **D1 通知文案放哪**（狀態：未決；擋住：A8-1；需要的事實：無）
  - A：共用文案目錄 `internal/platform/uitext/catalog`——三端已有生成管線；但目錄分界規定只放「只由契約值決定」的文字，通知文案帶參數且只有伺服器用，放進去會讓 TS／Dart 生成一堆用不到的函式。
  - B：模組自己的 `internal/text`（多語系 ADR 的「模組的伺服器句子」）——符合分界；伺服器組字後送給 client，client 不需要這些字串（建議）。
- **D2 收件匣保留期**（未決；擋住：A8-1）
  - A：90 天——最省；買家查「上個月的出貨通知」仍在，季度以上的查詢靠訂單頁。
  - B：180 天——涵蓋退貨、爭議與撥款週期（建議）。
  - C：1 年——成本與個資保留都較高，沒有對應需求。
- **D3 事件裁決**（未決；擋住：A8-4、A8-5；需要的事實：A8-0 的裁決表）：`backend/docs/plans/2026-09-24-go-modular-monolith/event-anomalies.md` §1 的 13 個沒有消費者的 key、§3 的 39 個沒有模板的 key、`order.delivered`、`coupon.issued`（發布端死碼）、5 個孤兒 `merchant.verification.*`（改由 seller KYC 事件取代）。建議原則：只訂閱「有使用者需要知道」且有發送端的事件；沒有發送端的不寫型別；`MeasureRequestSubmitted`（提醒管理員）不進通知（管理員不是收件人，決策 3），留在後台佇列。A8-0 完成後整表一次拍板。
- **D4 偏好分類**（未決；擋住：A8-1）：現有 10 類加模板裡的 `content`、`seller_account`。建議：
  - 交易必要（收件匣不可關、email 不可關）：訂單、出貨取貨、退款售後、撥款結算、帳號與治理（含 KYC、停權、處分）。
  - 互動（可關）：聊天、評價、社群互動（留言、讚）。
  - 行銷性質（預設開、可隨時關，個資法第 20 條）：coupon、restock、follower、post。
  - 賣家的 `shipment_anomaly` 推播不可關（沿用）。
- **D5 取貨簡訊**（未決；擋住：A8-5 的取貨型別渠道；需要的事實：A8-0 查 7-ELEVEN）
  - A：7-ELEVEN 自己寄就不做 SMS（建議，若事實成立）。
  - B：不寄就在「可取貨」「取貨即將逾期」加 SMS 渠道（要選 SMS provider，成本另估）。
- **D6 收件匣回應的形狀**（未決；擋住：A8-2）
  - A：伺服器組好標題、內文、深連結，加型別 enum 讓 client 選圖示——client 不渲染、多語系一處（建議）。
  - B：只回型別＋參數，client 渲染——三端各寫一份對照表，違反「同一個 enum 的對照表不在兩個 client 各寫」。
- **D7 Android 推播**（未決；擋住：A8-2 的 platform 值、A8-7）
  - A：這次一起收 android token（兩支 App 是 Flutter，FCM 本來就支援；成本是真機驗收多一台）（建議）。
  - B：維持只收 iOS，Android 延後到上架計畫。
- **D8 寄信服務**（未決；擋住：A8-3 的真實 provider 驗收；不擋替身開發）
  - A：Resend 新帳號——API 簡單、有 webhook；舊公司用過，需確認是新帳號。
  - B：Amazon SES——最便宜、設定與信譽管理較重。
  - C：Postmark——交易信送達率口碑最好、價格中等（建議：只寄交易信，量小，送達率優先）。
  - 帳號與網域由 Will 開；adapter 介面不受選擇影響。

- **D9 代行者能否讀寫店家的通知**（未決；擋住：A8-2；需要的事實：A8-0 查 U12，`api/hawkr/seller/v1alpha1/seller.proto:306` 註解把「通知收件箱、偏好與裝置」列在代行白名單）
  - A：代行者不能讀收件匣、不能改偏好與 token——通知 ADR 決策 3、4：代行者不收店家通知、代行頁不訂閱店家串流（建議）。
  - B：代行者可讀收件匣（協助店家處理）、不能改偏好與 token——要改決策 3 並在代行稽核記讀取。

拍板順序：D1、D2、D4、D6、D7、D8、D9 現在可以一次拍（D9 的建議與通知 ADR 一致，A8-0 若發現 client 依賴再回報）；D3、D5 等 A8-0 的產出。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| A8-0 基線與事實查核 | 未開始 | | | |
| A8-1 模組本體 | 未開始 | | | |
| A8-2 Connect API 與串流 | 未開始 | | | |
| A8-3 推播與 email | 未開始 | | | |
| A8-4 in-process 訂閱 | 未開始 | | | |
| A8-5 legacy 訂閱 | 未開始 | | | |
| A8-6 website、admin | 未開始 | | | |
| A8-7 兩支 App | 未開始 | | | |
| A8-8 刪舊 | 未開始 | | | |
| A8-9 收尾 | 未開始 | | | |

## 基線

（A8-0 填）
