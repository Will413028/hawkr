# 遷移計畫 A8：notification 搬進單體

**Goal：** notification-service 由 `internal/modules/notification` 取代，依 ADR `docs/adr/2026-10-05-notification-module.md`（以下稱「通知 ADR」，含 2026-10-17 多語系修訂與 2026-11-23 修訂）。完成後（可驗證的終態見 A8-8）：

- 通知型別定義在程式碼：typed 參數、受眾、偏好分類、呈現分類、渠道、可選的聚合鍵、zh-TW／en 文案、深連結，每個型別有固定輸出的比對測試；`notification_templates` 與 94 列模板退場。
- 來源模組的 transaction 只排一個「通知意圖」job；worker 解析收件人、套偏好與略過規則、寫收件匣與每渠道投遞紀錄、排渠道 job。
- 收件人是 `(account_id, 受眾 BUYER|STORE, store_id)`；裝置 token 以 token 為主鍵、綁一個帳號與一支 App，記最後註冊時間，過期清除。
- 對外 API 是 Connect（`api/` 的 notification service）；即時串流只傳失效訊號（未讀數與最新 ID），client 收到後重拉列表。四端改用生成的 client。
- 只接單體模組的事件。還沒搬的服務（Java、invoice-service、ai-service）的通知型別隨發送端搬遷時定義（主計畫「模組完成的定義」），不寫 `internal/legacy` 轉接；從 A8-7 刪舊服務起到各發送端搬進單體前，那些事件沒有通知（系統未上線，Will 2026-11-23 接受）。
- 刪除：notification-service、gateway 的通知路由、`openapi.yaml` 對應 operation、`backend/api/proto/notification` 與 Java 副本、`socialrelay`、`imrelay`、`reviewrelay` 的 notification 一半、`delegationservice/events.go` 的通知 wire、`sellerrelay` 對 notification 的部分、`pkg/mq` 的 social 常數。

**上層：** 主計畫 `2026-09-24-go-modular-monolith.md` 的 A8 段與「模組完成的定義」；本檔只寫 A8 特有的步驟與驗收。進度寫在本檔，主計畫 A8 段只指向這裡。

**附件：** `2026-11-23-a8-notification/`，盤點快照 dev `894625b5`。檔案異動：A8-1 到 A8-4 在 `backend/internal/modules/notification`、`api/` 新增；在事件來源模組、`internal/arch/modules.go`、`internal/httpapi` 修改；A8-3、A8-7 刪除列出的 relay 與舊服務；A8-5、A8-6 修改四端並刪手寫呼叫。

| 檔 | 內容 |
|---|---|
| `inventory.md` | 16 支 RPC＋SSE 與四端呼叫點、6 張表、消費 18／發布 2 個事件、渠道 13 列、94 列模板（重放 migration 量得）、背景工作 10 列、缺陷 #1–#27、單體橋接 8 套件與預留事件 12 項、守門測試 4 組、不確定 U1–U13 |
| `baseline-brief.md` | 業界基線對照（自建 vs SaaS、River vs 工作流引擎、投遞模型、收件匣、偏好、串流、token、聚合）與前提問題（legacy 轉接 100% 重工）；Will 2026-11-23 同意全部建議 |

- 盤點由唯讀 subagent 起草，每項附 `file:line`；主 session 抽查了缺陷 #1、#10、#19 與 identity 的寄件地址 API。簡報由沒參與草案的 subagent 起草；主 session 核對了 B3 刪超商取貨（主計畫 B3 段）與 `internal/legacy` 只放暫時轉接（主計畫規則）。簡報的外部連結是依公開文件記憶寫的【推】，實作引用前核對。
- 盤點的「去向」欄由本檔步驟填，衝突時以本檔為準。

## 前提（核對日 2026-11-23，dev `894625b5`）

- **會改到或取代的部署是否在線**：不在線。系統從未上線（根目錄 `AGENTS.md`）；不受回滾窗口、資料搬移、雙跑約束；舊 `hawkr_notification` 資料不搬。已確認。
- **寄件地址**：主計畫 A8 段「identity 接 Clerk `user.updated` webhook」已失效：Clerk 已移除（`identity/db/migrations/20261030171858_drop_clerk_user_id.sql`，ADR `2026-10-30-identity-self-hosted-federated-login.md`），identity 在每次登入以 provider 回報的 email 更新寄件地址，`identity.ContactEmail`（`identity.go:255`）只把 `ContactEmailVerified` 當寄件地址。A8 與發票寄送都用它。已確認【量：讀碼】。
- **沒有 Apple 登入**：通知 ADR 決策 8 的 Apple 私密轉寄網域登記不做。已確認。
- **取貨簡訊的前提已消失**：B3 改賣家自寄、刪超商取貨（主計畫 B3 段）；通知 ADR 決策 9 的取貨簡訊候選失效。已確認。
- **外部前置（只有 Will 能做）**：新的 Firebase 專案（擋 A8-3 的真實推播與 A8-6 的真機驗收）；Resend 新帳號（不得用舊公司的）與交易信子網域（SPF、DKIM、DMARC）。Will 目前沒有網域（2026-11-23），真實寄信延後到買網域；其他步驟用替身。未完成。
- **單體是同一份程式、不是單一行程**（2026-11-24 更正，證據 `cmd/hawkr/main.go` 的 `runServe` 與 `runWorker`；原寫「單一行程：串流 hub 用記憶體即可」不成立）：`serve`（Connect 串流、可多個副本）與 `worker`（River，含意圖 worker）是不同行程，記憶體內的發布到不了別的行程。串流訊號改走 Postgres LISTEN/NOTIFY（先例 `modules/im` 的 `internal/live`、procurement 的共用 LISTEN）：寫入端在自己的 transaction 內 `pg_notify`（payload 只有業務鍵），每個 serve 行程一條專用 LISTEN 連線（`Deps.ListenConnConfig`，直連或 session 模式，不經 transaction pooling）扇出給本行程的 `(account_id, 受眾)` 訂閱者，內容由 serve 端讀取。每帳號串流上限是每個 serve 行程各自的，不跨副本。已確認。
- **一個帳號最多一家現行店、沒有店員**（ADR `2026-06-29-identity-defer-store-entity.md`）：店家受眾的收件人解析集中一處。仍成立。
- **工具鏈**（預設，可推翻）：照根目錄 `AGENTS.md`。

**驗收基線：** A8-0 對當時 HEAD 跑 backend `make lint-monolith`、`make vet-all`、`make test-all`、notification-service 的 unit 測試與四端的 CI 等價檢查，記在文末「基線」；既有失敗不在本階段修。CI 一次只推一批。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 來源 transaction 只排一個通知意圖 job，收件人與偏好在 worker 決定；例外只有明列的清除訂閱者：停權與刪帳號時同 tx 刪 token（通知 ADR 決策 4） | A8-1 的結構性測試：notification 的同步訂閱者只呼叫 `jobs.Enqueue`，白名單列出 token 清除訂閱者（A8-3 加入）；backend 單體規則 4 |
| 意圖冪等鍵 = 事件來源身分；投遞冪等鍵 =（意圖、收件人、渠道） | 投遞紀錄 UNIQUE、重送 runbook、未觸達查詢 |
| 投遞紀錄是每筆推播與 email 觸達與否的唯一來源（River 只保留完成的 job 24 小時）；收件匣不是渠道，收件匣列本身就是進收件匣的紀錄 | 重送、未觸達率查詢 |
| 收件匣參數快照不含第三方個資（只存 ID，讀取時解析） | 註冊表自檢、收件匣讀取、個資清除 |
| 收件匣只存型別＋參數快照＋聚合鍵，讀取時依請求語系組字 | 收件匣讀取 API、型別渲染程式的保留、個資清除鍵 |
| BUYER 受眾只推 buyer App、STORE 只推 seller App；串流訂閱鍵 `(account_id, 受眾)` | `device_tokens.app`、推播 job、串流 hub |
| 封鎖雙方之間不送互動通知；自我通知以帳號比對 | worker 的略過規則（經 buyer、seller 公開 API 讀封鎖）；A7-3 的同一個測試 |
| email 只寄 `identity.ContactEmail` 的 `ContactEmailVerified`，不寄 suppression 名單 | email job、發票寄送 |
| 通知型別隨發送端模組定義，不經 `internal/legacy` | 主計畫「模組完成的定義」；A8-8 的 `rg` 檢查 |
| 依賴方向：notification 依賴來源模組的公開事件型別與讀取 API，來源模組不 import notification | `internal/arch/modules.go:42-45` 的 notification 列；A8-1 加 `accountsuspension`、`moderation` |

## 規模（搬遷）

| | 數 | 來源 |
|---|---|---|
| RPC＋SSE | 16＋1 | `inventory.md` §1 |
| 表 | 6 | §2 |
| 單體送出的 route（A8 要接的部分） | 22 | §8.1 |
| 預留給 A8 的單體事件 | 12 項 | §8.3 |
| 不在 A8 接的 (exchange, key) | 51 組（73 組減單體 22），隨 B2–B10、A5、A9 搬遷 | `baseline-brief.md` §1(a) |
| relay 套件 | 7（`bizcalendarrelay` 無關） | §8.2 |

## 範圍

- 做：模組本體與資料模型；Connect API（收件匣、未讀數、已讀、刪除、偏好、裝置 token、串流、後台公告）；推播（FCM）與 email（Resend）adapter 和替身；單體模組事件的訂閱；主計畫 A8 段列的治理、KYC、收款帳戶、停權、檢舉結果通知；投遞重送手段與 runbook；未觸達率查詢；四端改 Connect；刪舊服務與橋。
- 不做：
  - `internal/legacy` 的 RabbitMQ 通知 consumer（100% 重工，`baseline-brief.md` §1(a)；Will 2026-11-23）。
  - 行銷 email、SMS、LINE、Web Push（通知 ADR 決策 9）；取貨簡訊（前提隨 B3 消失）。
  - 後台模板編輯、預覽、試發（模板退場）。
  - 時間窗摘要（每日摘要信）：沒有需求；互動洗版由聚合鍵處理。
  - `product.viewed`（每次瀏覽通知賣家）的型別。
  - 舊資料搬移（不在線）。
- 延後：
  - 接上 order、aftersales 的賣家工作區發布者（呼叫 `PublishSellerWorkspace`，D19）。觸發：order、aftersales 模組搬進單體。
  - 還沒搬的服務的通知型別。觸發：各發送端搬進單體（B2–B10、A5、A9），由「模組完成的定義」新增條目承接。
  - 真實寄信驗收（Resend 帳號、交易信子網域的 SPF、DKIM、DMARC，寄一封到測試信箱）。觸發：Will 買網域。
  - 食安事件通知近 6 個月購買者。觸發：B8。
  - SMS、LINE、Web Push 重新評估。觸發：未觸達率連續 4 週超過 10%（通知 ADR 待辦）。

## 步驟

- [ ] **A8-0 基線與事實查核**（被擋於：無）
  - 範圍：跑驗收基線；產出單體事件裁決表（§8.1 的 22 個 route＋§8.3 的 12 項：每個事件對應哪個型別、受眾、分類、呈現分類、是否聚合，或不做附理由），交 D3；查 U10（seller workspace 即時通道 `hawkr.seller_workspace` 是否與通知串流共用路徑或 hub，共用就寫進 A8-2 範圍）、U12（代行者是否呼叫通知路由）、U8（client 對 `type`／`refType`／`data.*` 是否做互動分支，影響 D6）。
  - 消費端：無（唯讀）。
  - 驗收：基線結果與四份產出在附件；裁決表與 `inventory.md` 去向欄無空格（新增一次性腳本 `2026-11-23-a8-notification/check_dispositions.py`，不進 `scripts/`）。
  - 停止條件：基線有會擋 A8 的失敗；U8 發現 client 依型別做互動（D6 要重議）。

- [x] **A8-1 模組本體（walking skeleton）**（被擋於：無）
  - 範圍：`internal/modules/notification`：
    - schema：收件匣（型別、參數快照、聚合鍵、actor 帳號 ID 清單、已讀）、通知意圖、每渠道投遞紀錄（待送／已送／永久失敗、實際送出的文字與 locale）、裝置 token（`last_registered_at`）、偏好覆寫列 `(account_id, 受眾, 分類, 渠道, enabled)`。公告表隨 A8-2、suppression 表隨 A8-3 建（第一個讀寫者出現時）。
    - 收件匣不是投遞渠道：投遞紀錄只記推播與 email；收件匣列本身就是「已進收件匣」的紀錄，聚合時多個意圖合併成一列。
    - 參數快照不存他人的個資：actor 與其他第三方只存帳號 ID（或業務 ID），名稱、頭像在讀取時向擁有模組解析，已刪除的顯示通用稱呼；註冊表自檢禁止型別參數宣告第三方名稱欄位。投遞紀錄的「實際送出文字」是例外（送出時的事實），由保留期與下方個資清除處理。
    - 舊模板機制的去向（盤點 §3–§5）：`payload_filter` 改成型別內的 typed 條件（Go 程式，不是資料）；silent 推播改成 D6 的狀態重讀提示（沒有可見文字的型別不建）；`image_key_template` 改成 D14 的可選圖片欄位；逐列寫進 `CARRYOVER.md`。
    - 型別註冊表：typed 參數、受眾、偏好分類（D4）、呈現分類（D6）、渠道、可選聚合鍵（讚、追蹤用）、冪等鍵來源、actor 欄位、文案（模組 `internal/text`，zh-TW／en）、深連結、可選的推播圖片（商品類型別帶，D14）；預設偏好與不可關的分類寫在註冊表。
    - 入口：同步訂閱者只排意圖 job；worker 解析收件人、套偏好、自我略過（帳號比對）、封鎖略過、寫收件匣（有聚合鍵時未讀列 upsert）與投遞紀錄、排渠道 job。
    - 保留期清除 job（收件匣、投遞紀錄與通知意圖都 180 天硬刪，D2、D15；意圖表同期限，因為它只為冪等存在，超過 River 重試期即無用，用 `jobs.SweepOpts`）；個資清除：訂閱 `identity.AccountDeleted`（`identity/events.go:9`）、`buyer.BuyerDeleted`（`buyer/events.go:25`）、`seller.StoreRetired`（`seller/events.go:31`），排 job 刪該收件人的收件匣列、偏好、投遞紀錄的送出文字與意圖參數（token 的同步刪除在 A8-3）；worker 在寫收件匣的同一個 transaction 先以 `FOR SHARE` 鎖收件帳號列（依 `platform/lockorder`，identity 帳號列最先）並確認未刪除，消除「檢查後才 commit 刪除」的競態。
    - `internal/arch/modules.go` 的 notification 列改成實際依賴：只留已在單體、且 A8 會訂閱或讀取的模組（identity、buyer、seller、review、im、social、procurement、delegation、payment、accountsuspension、moderation），刪掉還沒搬或已退場的（catalog、ordering、fulfillment、invoice、coupon、aftersales、finance）；那些隨各自搬遷時加（D13）。
    - 一個型別端到端：`accountsuspension.Suspended` → 收件匣。`CARRYOVER.md` 沿用盤點表（盤點 §2、§6 每列）。
  - 子步驟（一個 session 做不完，2026-11-23 拆分；各自有驗收，依序做）：
    - **A8-1a 骨架、schema、註冊表**：`make new-module` 產骨架並登記 `cmd/hawkr/modules.go`；上列全部表（`schema.sql`＋migration＋sqlc）；型別註冊表與 `internal/text`（先只有 `account_suspended`、`account_unsuspended` 兩型別）；`modules.go` 依賴改成實際依賴；`CARRYOVER.md`（盤點 §2–§6 每列）、模組 `AGENTS.md`。驗收：兩型別 zh-TW／en 固定輸出比對測試、註冊表自檢測試（每型別有冪等鍵來源、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別、參數不含第三方名稱欄位）、`TestSchemaFileMatchesMigrations` 等既有守門、lint／vet。mutation：不可關分類改成可關 → 註冊表測試紅。
    - **A8-1b 意圖管線與第一個端到端型別**：同步訂閱者→意圖 job→worker（收件人解析、偏好、自我略過、收件匣寫入與聚合 upsert、投遞紀錄；渠道 job 先只排不送，A8-3 接 adapter；寫收件匣的 transaction 內以 `FOR SHARE` 鎖收件帳號列並確認未刪除）；`accountsuspension.Suspended`／`Unsuspended` 訂閱；結構性測試。驗收：下方 integration 的冪等、偏好、自我略過、聚合四項、刪除後到達的意圖不建收件匣與 Suspended→收件匣；mutation 其餘三項。
    - **A8-1c 保留期與個資清除**：180 天清除 job、三個刪除事件的清除 job（D16 已決）。開工時查到兩處缺口（2026-11-23，主 session 依 D15 與本步驗收裁決）：(1) `deliveries.intent_id` 的 `ON DELETE CASCADE` 讓刪過期意圖連帶刪掉未滿 180 天的投遞紀錄（投遞的 `created_at` 晚於意圖）：FK 改 `ON DELETE RESTRICT`，清除 job 先刪過期投遞，再刪過期且已無投遞的意圖；(2) worker 只鎖 identity 帳號列，`BuyerDeleted`／`StoreRetired` 之後處理的意圖仍會替已清除的收件人建列：worker 在同一個 transaction 依 `platform/lockorder` 再以 `FOR SHARE` 鎖 BUYER 收件人的 buyer 列、STORE 收件人的 store 列（經 buyer、seller 公開方法），已刪除或已退場就略過。驗收：下方 integration 的清除兩項（刪除事件後清除、到期硬刪）。
  - 消費端：`rg -n "accountsuspension\.(Suspended|Unsuspended)" backend/internal`；`internal/arch/modules.go`。
  - 不能動：accountsuspension 的事件型別與發布 transaction。
  - 驗收：每個型別的固定輸出比對測試（zh-TW、en）；結構性測試：notification 的同步訂閱者只呼叫 `jobs.Enqueue`；integration：同一事件投遞兩次只有一列、偏好關閉的分類不建收件匣但不可關的分類仍建、自我通知被略過、同聚合鍵的未讀通知合併成一列、三個刪除事件後收件匣、偏好、投遞送出文字與意圖參數被清、刪除後到達的意圖不建收件匣、180 天到期的收件匣／投遞紀錄／意圖列被硬刪；`make lint-monolith`、`make test-all`、`GOWORK=off go vet -tags integration ./...`。mutation（進 `mutation/`）：拿掉意圖冪等 UNIQUE → 冪等測試紅；自我略過改回比 buyer_id → 略過測試紅；不可關分類改成可關 → 測試紅；聚合 upsert 改成 insert → 合併測試紅。
  - 停止條件：型別註冊表無法不靠執行期反射表達上列屬性。

- [ ] **A8-2 Connect API 與串流**（被擋於：A8-1、D6）
  - 子步驟（2026-11-23 拆分，各有驗收、各自一個 session；範圍、消費端、不能動、驗收照下方原文分配）：
    - A8-2a 契約：`api/` 的 notification service 全部方法（含串流與後台公告）、D6 的呈現分類與狀態重讀提示 enum、`ErrorReason`；`seller.proto:306` 白名單註解；backend `make connect`、TS、Dart 生成。`ErrorReason`：不需新增（標準 code 足夠）。驗收：`buf lint`、`buf breaking`、三端生成無 drift；handler 未掛時 `TestEveryServiceInTheContractIsMounted` 的紅是預期，在 A8-2b 消除。
    - A8-2b 收件匣、偏好、token 的 unary handler：`model.Delivers` 改由型別註冊表推導每個分類實際送出的渠道（現只排除公告的 email，例：互動類沒有 email），偏好列表與更新依它過濾與拒絕；D9 代行者拒絕、`httpapi` 停權放行與限流豁免、掛載。驗收：每支方法的 Connect 特性測試；mutation：`MarkAsRead` 非本人成功、`total` 不套篩選。
    - A8-2c 後台公告：公告表、建立與列表、收件匣與推播分開的 job。驗收：沒有 token 的帳號也收到；特性測試。
    - A8-2d 串流與賣家工作區：hub、`(account_id, 受眾)` 訂閱與上限、LISTEN/NOTIFY 跨行程扇出（意圖 worker 在寫收件匣的同一 transaction `pg_notify`；serve 以 `serveWithRun` 跑 `notificationModule.Run`，worker 不跑；LISTEN 重連後對所有本地訂閱者發完整重讀訊號）、`seller_workspace` 事件的單體內發布介面（D19，發布端隨 order、aftersales 搬遷，在自己的 transaction 內呼叫）、website `useSellerWorkspaceRealtime` 改接。驗收：串流 integration（worker 路徑寫入後另一個 hub 的訂閱者經 LISTEN 收到訊號、賣家工作區發布後訂閱者收到事件且 rollback 不送、另一受眾收不到、超過上限被拒、LISTEN 連線被終止後重連發重讀訊號）；mutation 作用在 `pg_notify` 路徑（拿掉意圖 worker 的 notify、拿掉工作區 notify、串流鍵拿掉受眾、拿掉上限、拿掉重連重讀、notify 不在呼叫端 transaction）。
  - 範圍：`api/` 新增 notification service：收件匣列表（cursor 分頁、分類篩選，`total` 跟著篩選）、未讀數、已讀（不存在或非本人回 NotFound）、全部已讀、刪除、偏好讀寫（受眾與分類只來自註冊表）、裝置 token 註冊與刪除（不帶 `platform`：兩支 App 只有 iOS，Android 已於 2026-09-26 移除，FCM 也不需要；App 每次啟動呼叫註冊，更新 `last_registered_at`）、串流（server streaming，`(account_id, 受眾)` 訂閱，只送未讀數與最新收件匣 ID，保留每帳號串流上限）、後台公告（建立、列表；收件匣與推播分開，沒有 token 的人也收得到；公告表在本步建）。收件匣回應帶伺服器依請求語系組好的標題、內文、深連結與呈現分類 enum。代行者不能呼叫（D9），`api/hawkr/seller/v1alpha1/seller.proto:306` 的白名單註解一起改。BFF 代理、`httpapi` 的停權放行與限流豁免一起改（§8.4 #4）。推播 payload 與串流訊號帶 D6 的狀態重讀提示 enum。賣家工作區即時通道與通知串流共用路徑、SSE 連線與 topic `notification:user:{storeId}`（A8-0 U10，`fact-checks.md`；`seller_workspace_channel.go:14-34`），一起搬：同一個串流與 hub 承載 `seller_workspace` 事件、其發布端（`ResolveTopic` 讀 `payload.sellerId`）與批次事件名，website `useSellerWorkspaceRealtime` 改接新串流。
  - 消費端：`rg -n "notifications|notification-preferences|device-token" website/ admin/ buyer-app/lib seller-app/lib backend/gateway backend/internal/httpapi`。
  - 不能動：舊 REST 在 A8-7 前仍可用。
  - 驗收：`buf lint`、`buf breaking`、TS／Dart 生成無 drift；Connect 特性測試涵蓋每支方法；串流 integration：建立通知後收到訊號、另一受眾收不到、超過上限被拒、呼叫賣家工作區發布介面（D19）後賣家工作區訂閱者收到事件。mutation：拿掉發布介面到 hub 的轉送 → 賣家工作區測試紅；串流鍵拿掉受眾 → 跨受眾測試紅；`MarkAsRead` 改回非本人也成功 → NotFound 測試紅；`total` 不套分類篩選 → 篩選計數測試紅；拿掉串流上限檢查 → 上限測試紅。

- [ ] **A8-3 渠道與單體事件訂閱**（被擋於：A8-1；真實 provider 驗收另擋於 Will 的帳號）
  - 範圍：
    - FCM adapter（只在 Unregistered、SenderIDMismatch 停用 token；ThirdPartyAuthError 告警；每批最多 500 個 token）；Resend adapter（對外只回暫時性／永久性，不重試；退信與投訴 webhook 先寫 `platform/inbox` 再寫 suppression 表（本步建））；兩者的本機替身。推播、email job：送出時依帳號 `preferred_locale` 組字；送出前在同一個 transaction 以 A8-1c 的同一組鎖確認收件人未刪除、未退場，否則把投遞標成永久失敗、不寫送出文字（A8-1c 審查 R1-4：清除之後才執行的待送 job 不得寫回送出文字）。
    - token：登入 upsert 改綁、登出刪、帳號停權（`accountsuspension.Suspended`）與刪帳號（`identity.AccountDeleted`）時由同 transaction 的同步訂閱者刪列，加進 A8-1 結構測試的白名單、每日 job 刪 60 天未更新的 token；店家停權不動。
    - 重送：依投遞紀錄重排永久失敗的手段與 runbook；未觸達查詢：近 N 天交易必要型別沒有任何渠道送達的數量（後台唯讀）。
    - 依 D3 的裁決表為單體事件定義型別並訂閱：social（留言、讚、追蹤的店發文；讚用聚合鍵）、buyer 的追蹤店家（`cmd/hawkr/buyer_events.go:20` 現經 `buyerrelay`；用聚合鍵）、review、im（離線推播）、seller（檢舉收到與結果、seller 新增公開事件 `CaseInquiryCreated`（`seller/events.go`，在 `CreateCaseInquiry` 的同一個 transaction 發布）、措施與治理、KYC 審核結果）、payment `PayoutAccountChanged`（payment 先加首次啟用時間、補件期限欄位；期限前提醒是另一型別 `payout_account_deadline`）；型別清單以 `event-adjudication.md` 與 D3 為準（收藏不做、im 只推播不寫收件匣、解除請求只送 Decided）、accountsuspension 停權與解除、delegation 代行完成、procurement `RfqPublished`。moderation 新增事件（消費者此時出現）：內容被隱藏、作者收到警告、案件結案（帶每位檢舉人帳號），在處置的同一個 transaction 發布。`ReportReceived` 的確認信：意圖參數只放 report 參照，worker 執行時向 seller 讀地址。
    - 刪除 `socialrelay`、`imrelay`、`reviewrelay` 的 notification 一半、`delegationservice/events.go` 的通知 wire、`sellerrelay` 對 notification 的部分、`buyerrelay` 的追蹤、`product.viewed` 與 `product.restock.target` wire（`buyer.deleted`、`favorite.*` 有 Java 消費者，保留）、相關 wire／golden 測試、`pkg/mq` social 常數。
  - 消費端：`rg -n "socialrelay|imrelay|reviewrelay|sellerrelay|buyerrelay|delegationservice" backend/`；`rg -n "CreateCaseInquiry" backend/internal`；`rg -n "hawkr\.(content|im|review|delegation)" backend/ --type go`。
  - 不能動：各模組事件型別的既有欄位（只加不改）；`reviewrelay` 給 Java product 評分聚合的那一半（A6 延後項）。
  - 驗收：測試證明每個被訂閱的事件都有對應型別、每個型別都有訂閱或明列為公告；integration（替身）：暫時性錯誤重試後只送一次、永久性錯誤不重試、Unregistered 停用 token、BUYER 受眾不推 seller App、停權與刪帳號後 token 在同一個 transaction 消失、60 天未更新的 token 被清、suppression 名單內不寄、確認信地址不在任何 job 參數裡、每個型別一個「事件 → 收件匣」案例、封鎖雙方不送；relay 刪除後 `rg` 0 hit。mutation：永久錯誤改暫時 → 測試紅；拿掉受眾 App 過濾 → 測試紅；拿掉封鎖略過 → A7-3 測試紅；移除一個訂閱 → 覆蓋測試紅。真實 provider：Firebase 就緒後送一則推播到測試裝置，結果貼進度表；真實寄信見「延後」。
  - 需要人做的事：Firebase 專案（Will）。
  - 完成後的狀態：單體模組的通知改由新模組送。

- [x] **A8-4 主計畫承接條目**（被擋於：無；可與 A8-1 並行）
  - 範圍：主計畫「模組完成的定義」加一條：搬進單體的模組，舊服務發往 notification 的每個事件逐一裁決（定義通知型別並訂閱，或不做附理由），不經 `internal/legacy`。通知 ADR 加 2026-11-23 修訂。
  - 驗收：`rg -n "通知型別" backend/docs/plans/2026-09-24-go-modular-monolith.md` 命中新條目；ADR 修訂段存在。

- [ ] **A8-5 website 與 admin**（被擋於：A8-2）
  - 範圍：website 收件匣、未讀數、串流（收到訊號重拉列表；重連後先拉一次）、偏好改用 `@hawkr/api`，刪手寫呼叫與 SSE hook；admin 公告頁改 Connect、新增未觸達查詢頁（照既有後台列表元件），刪模板管理頁。版面不動。
  - 消費端：`rg -n "notification" website/ admin/ -g '!node_modules' -l`。
  - 驗收：website、admin CI 等價檢查；新增 website 測試：收到訊號後重拉列表、重連後拉一次；e2e 交給 CI。mutation：收到訊號不重拉 → 測試紅；重連後不拉 → 重連測試紅。

- [ ] **A8-6 buyer-app 與 seller-app**（被擋於：A8-2；真機推播驗收另擋於 Firebase 專案）
  - 範圍：兩支 App 的收件匣、偏好、token 註冊（每次啟動註冊，不送 `platform`）、登出刪 token 改用 `packages/hawkr_api`；換新 Firebase 專案設定。推播改用可見通知（APNs alert，FCM `notification` payload），加 iOS Notification Service Extension 下載推播圖片（D14）；刪掉以背景 isolate＋`flutter_local_notifications` 顯示 data-only 推播的路徑（iOS 會節流）。每支 App 一個 session。
  - 消費端：`rg -n "notification|device_registrar|device-token" buyer-app/lib seller-app/lib`。
  - 驗收：analyze、format、custom_lint、生成檢查與改動到的測試檔（完整 `flutter test` 交 CI）；真機：登入收到推播、登出不再收到、換帳號登入不收前一人的通知。

- [ ] **A8-7 刪除舊服務與舊路由**（被擋於：A8-3、A8-5、A8-6）
  - 範圍：（全部是刪除或修改）刪 `backend/services/notification-service`、compose 項目、gateway 通知路由與 handler、`openapi.yaml` 對應 operation、`backend/api/proto/notification` 與 Java `java-proto` 副本（先 diff 確認無其他使用，U11）、只為舊通知存在的守門測試（§8.4 #1、#2）、`notifyAllowlist`、沒人讀的 `MAIL_*`／`RESEND_API_KEY`。
  - 消費端：`rg -n "notification-service|hawkr_notification|notificationpb|notification\.proto" --hidden -g '!.git' -g '!**/docs/**' -g '!**/*.md'`。
  - 驗收：上述 `rg` 0 hit（附輸出）；賣家工作台即時通道的既有測試仍綠（U10）；全部 workflow 的 CI 等價檢查；gateway 對 openapi 的對齊檢查通過。
  - 完成後的狀態：還沒搬的服務的事件沒有通知，到各自搬遷為止。

- [ ] **A8-8 收尾**（被擋於：A8-7）：主計畫「模組完成的定義」逐項核對；`inventory.md` 去向欄與缺陷 #1–#27 處置無空格（`check_dispositions.py`）；design-review skill 對整體 diff；長期檢查（訂閱↔型別覆蓋、同步訂閱者只排 job、notification 不 import `internal/legacy`）接進 CI；設計書 12、13 改寫；主計畫 A8 段打勾；延後項附觸發搬回主計畫；需要人做的事列給 Will。

## 既有缺陷處置

依 `inventory.md` §7 編號；「消失」指新結構讓它不可能發生，由對應步驟的測試守。

| # | 處置 | 步驟 |
|---|---|---|
| 1 吞錯誤後 ACK、2 冪等非原子、3 無冪等鍵、4 冪等鍵殘留字面 | 修：意圖 job＋投遞紀錄 UNIQUE＋River 重試；型別必須宣告冪等鍵來源 | A8-1、A8-3 |
| 5–7 廣播分頁、同步逾時、計數不實、只送有 token 的人 | 修：公告是資料，收件匣與推播分開 job | A8-2、A8-3 |
| 8 丟 Code、9 白名單不一致、10 分類不在清單、11 未驗證受眾、24 `push_mode` 無 CHECK | 消失：Connect 錯誤；分類與受眾只在註冊表 | A8-1、A8-2 |
| 12 自我略過比錯身分空間 | 修：帳號比對 | A8-1 |
| 13 token 同機多人、14 刪帳號只清 chat | 修：token 主鍵與綁定、個資清除 | A8-1、A8-3 |
| 15 只收 iOS | 不是缺陷：Android 已移除；刪 `platform` 欄位與兩處驗證 | A8-2、A8-7 |
| 16 永久保留 | 修：180 天 | A8-1 |
| 17 `total` 不受篩選、18 已讀無 NotFound | 修 | A8-2 |
| 19 串流跨身分、20 串流 best-effort | 消失：`(account_id, 受眾)`；串流只是訊號，資料由列表保證 | A8-2 |
| 21 試發路徑、22 死碼死設定、23 孤兒模板、25 行程內模板快取 | 刪除 | A8-7 |
| 26 FCM 500 上限 | 修：分批 | A8-3 |
| 27 只有 zh-TW | 修 | A8-1 |
| 盤點 §6 的 90 天殭屍 token 清理 | 改寫：`last_registered_at`＋60 天清除 | A8-3 |

## 決定

業界對照與每題的取捨見附件 `baseline-brief.md`；以下只留結論。

| 題 | 結論 | 狀態 |
|---|---|---|
| D0 自建 vs Novu／Knock／Courier | 自建：Novu 要另部署 MongoDB、Redis 與多個 Node 服務；SaaS 讓通知資料離開自己的 DB，個資清除做不到 | 已決 2026-11-23，Will |
| D0b River vs 工作流引擎（Temporal 等） | River | 已決 2026-11-23，Will |
| D1 文案位置 | 模組 `internal/text`（伺服器組字，client 不需要） | 已決 2026-11-23，Will |
| D2 收件匣保留期 | 180 天硬刪 | 已決 2026-11-23，Will |
| D3 事件裁決 | 範圍只有單體事件；原則：有使用者需要知道且有發送端才做型別；`product.viewed` 不做；`MeasureRequestSubmitted` 不進通知（管理員不是收件人）。逐列表見附件 `event-adjudication.md`，開放問題 1–7 照建議：收藏不做；聊天只推播不寫收件匣；`MeasureCreated(SUSPENSION)` 不另發（由 `StoreSuspended` 通知）；解除請求被接受時只送 `MeasureRequestDecided`（worker 對同一 `MeasureID` 去重）；`PayoutAccountChanged` 加首次啟用時間與補件期限欄位，期限提醒另立 `payout_account_deadline`；`content_hidden` 只給買家作者；`rfq_invited` 歸互動。問題 8 見 D3b | 已決 2026-11-23，Will |
| D3b 範圍外的四個 seller／buyer 事件 | `StoreClosed`、`StoreReopened` 不做（賣家自己的操作）；`StorePendingDeletion`、`BuyerDeletionRequested` 做 email 確認型別（交易必要，刪帳申請寄確認信是業界基線），A8-3 加入 | 已決 2026-11-23，Will |
| D4 偏好分類 | 交易必要（訂單、出貨、退款售後、撥款、帳號與治理）：收件匣、email 不可關；互動（聊天、評價、留言、讚、追蹤、追蹤的店發文）：可關；行銷（coupon、restock）：預設開、可關（個資法第 20 條）；賣家 `shipment_anomaly` 推播不可關。只存覆寫列 | 已決 2026-11-23，Will |
| D5 取貨簡訊 | 刪除（B3 賣家自寄） | 已決 2026-11-23，Will |
| D6 收件匣回應 | 伺服器組字＋10 個呈現分類 enum（`ORDER`、`REFUND`、`PAYOUT`、`MESSAGE`、`SOCIAL`、`REVIEW`、`PROMOTION`、`PROCUREMENT`、`ACCOUNT`、`SYSTEM`；不是每型別一個值）＋深連結。重議（A8-0 U8：App 推播依型別重讀訂單／售後／賣家狀態、在聊天室時不彈 banner，見 `fact-checks.md`）：推播與串流另帶狀態重讀提示 enum（`ORDER`、`AFTER_SALE`、`SELLER_STATUS`，可為空），聊天抑制以深連結指向的聊天室判斷；不公開每型別 enum | 已決 2026-11-23，Will；重議同日 |
| D7 Android | 不做：兩支 App 的 Android 已於 2026-09-26 移除（簡報誤以為仍在，2026-11-23 更正）；token 不存 platform，記 `last_registered_at`，60 天未更新清除 | 已決 2026-11-23，Will；更正同日 |
| D8 寄信服務 | Resend（免費額度足夠、不需審核）＋交易信專用子網域；adapter 介面不綁 provider。真實寄信等買網域 | 已決 2026-11-23，Will |
| D9 代行者讀店家通知 | 不能 | 已決 2026-11-23，Will |
| D10 投遞模型 | 來源 tx 只排意圖 job，worker 展開；投遞紀錄保留為唯一帳本 | 已決 2026-11-23，Will |
| D11 串流內容 | 只傳失效訊號，不做續傳游標 | 已決 2026-11-23，Will |
| D12 收件匣聚合 | 型別可宣告聚合鍵，未讀時合併；不做時間窗摘要 | 已決 2026-11-23，Will |
| D14 推播圖片 | 註冊表有可選圖片欄位，商品類型別帶圖；FCM payload 帶圖片 URL；iOS 加 Notification Service Extension（A8-6） | 已決 2026-11-23，Will |
| D15 投遞紀錄保留期 | 180 天硬刪，與收件匣同一個清除 job（舊系統只刪 failed／skipped 超過 30 天） | 已決 2026-11-23，Will |
| D13 legacy 通知轉接 | 不做；通知型別隨發送端搬遷 | 已決 2026-11-23，Will |
| D17 後台公告語系 | 一律只寫英文：單一 `title`／`body`，不分語系、不依請求語系選擇（偏離多語系 ADR 的 zh-TW／en，限公告） | 已決 2026-11-24，Will |
| D18 公告的偏好分類 | 新增偏好分類 `ANNOUNCEMENT`（受眾 BUYER 與 STORE）：收件匣不可關，推播可在偏好關閉，不寄 email（註冊表 `model.Delivers` 不列公告的 EMAIL）；proto 的 `NOTIFICATION_CATEGORY_ANNOUNCEMENT` 與註冊表（`internal/model`）同步，偏好分類由 D4 的 14 個變成 15 個。原問題：公告沒有分類會是 UNSPECIFIED，不能篩選也不能設偏好 | 已決 2026-11-24，Will |
| D19 賣家工作區事件的發布端 | notification 公開單體內的發布介面 `PublishSellerWorkspace(storeID, change)`，給之後搬進單體的 order、aftersales 模組在自己的 transaction 內呼叫（`pg_notify` 隨 commit 送出、rollback 不送；簽名 `PublishSellerWorkspace(ctx, storeID, change)`）；不寫 RabbitMQ 轉接（`seller.order.updated`、`seller.after_sale.updated` 的發布端是還沒搬的 Java order-service、aftersales-service，與 D13 同理）；website `useSellerWorkspaceRealtime` 現在就改接 `WatchNotifications`（STORE 受眾），order、aftersales 搬進單體之前賣家中心沒有即時同步（系統未上線，接受）。原問題：發布端在還沒搬的服務，單體內沒有可掛鉤的事件 | 已決 2026-11-24，Will |
| D16 投遞紀錄「實際送出文字」裡的第三方名稱 | (a) 接受：送出文字是送出時的事實紀錄，受 D15 的 180 天硬刪約束；只有收件人自己刪帳號時提前清。原問題：actor 刪帳號後，別人投遞紀錄的送出文字仍含其名稱，最長 180 天。選項：(a) 接受，送出文字是「送了什麼」的事實紀錄，期限受 D15 約束；(b) 不存送出文字，只存型別＋參數（ID）＋locale，需要時重組，代價是重組結果可能與當時不同、客訴時無法證明送了什麼；(c) actor 刪帳號時也掃描清除。建議 (a)：業界（SendGrid、Postmark 的 message activity）保留送出內容到保留期，GDPR 以有限保留期滿足最小化。 | 已決 2026-11-23，Will |

ADR：通知 ADR 加 2026-11-23 修訂（D0、D0b、D5、D10–D13、寄件地址改 identity 登入同步、刪 Apple 私密轉寄），由 A8-4 寫。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| A8-0 基線與事實查核 | 完成 | 基線見文末；`event-adjudication.md`、`fact-checks.md`、`inventory.md` 去向；`python3 backend/docs/plans/2026-11-23-a8-notification/check_dispositions.py` → 0 個空格（填表前 243 個） |  | 本次 |
| A8-1a 骨架、schema、註冊表 | 完成 | 本機：gofmt、lint-monolith（0 issues）、vet integration、notification／arch／cmd／uitext unit -race、mutation --validate；開工與完成前設計審查、code-review＋合約審查兩輪（findings 13，blocking 1 已修） ；CI `d049c19ab`：notification integration ok、ui-text generated diff、mutation、其餘 16 個 workflow 綠 | backend monolith 紅只因既有的 `httpapi TestEveryServiceInTheContractIsMounted`（imageai、procurement 未掛，A9 範圍；`6298a02b1` 起已紅） | `fe89c9128`..`9075c1bc1` |
| A8-1b 意圖管線 | 完成 | 本機：gofmt、lint（0 issues）、vet integration、notification／identity／arch／lockorder／cmd unit -race、mutation --validate；code-review＋合約審查兩輪（findings 10，blocking 0；fixed 8、rejected 1 nit：同時間戳由處理順序決定，機率可忽略） ；CI：mutation 綠（`f62fa160e`）；integration 在 run 45195973731（`52498bd7b`，含本步）notification、notification/internal/intake、identity ok | backend monolith 紅只因既有的 `httpapi TestEveryServiceInTheContractIsMounted`（A9）；repo hygiene 紅是 B2 的 PayUni 殘留（`payment/CARRYOVER.md`），推送前已紅 | `1d3383a9b`..`c213cfec3` |
| A8-1c 保留期與個資清除 | 完成 | 本機：gofmt、lint-monolith（0 issues）、vet integration、notification／buyer／seller／arch／lockorder／cmd unit -race、mutation --validate（8 個）；合約審查 NO DEFECTS；code-review＋第 2 輪審查（findings 10，blocking 0；fixed 8：R1-1/2/3/5/6、D1–D3，recorded 1：R1-4 寫進 A8-3，accepted 1：D4 店家鎖順序由 seller 自己的測試涵蓋）；CI run 45204116577（`8e72287e2`）：notification、intake、purge、buyer、seller integration ok；mutation 綠（8 個） | backend monolith 紅只因既有的 `httpapi TestEveryServiceInTheContractIsMounted`（A9）；repo hygiene 紅是既有的 B2 PayUni 殘留（`payment/CARRYOVER.md`） | `90d5530ca`..`f23cac6ef` |
| A8-2a 契約 | 完成 | 本機：buf lint、buf breaking、gofmt、lint-monolith（0 issues）、vet integration、notification unit；契約審查＋code-review 兩輪（findings 14，blocking 0；fixed 13、rejected 1 nit）；CI `37664db6d`：API contract、TS／Dart packages、website、admin、兩支 App 綠；ui-text generated diff 紅，後續 `a4640c3c9` 起綠；arch 與 mutation validate 的回歸由 `2b207ba9f` 修 | backend monolith 那輪被後續推送取消，以 A8-2b 的 CI 為準 | `47e131815`..`37664db6d`、`2b207ba9f` |
| A8-2b unary handler | 程式完成待驗收 | 本機：gofmt、lint-monolith（0 issues）、vet integration、arch／httpapi／notification／cmd unit -race（只剩預期的 `TestEveryServiceInTheContractIsMounted`：NotificationAdminService 屬 A8-2c、SellerProcurementAppService 屬 A9）、mutation --validate；契約審查＋code-review 兩輪（findings 13，blocking 1 屬跨步：公告列被收件匣隱藏，於 A8-2c 註冊公告型別解決；fixed 8、recorded 2、rejected 3） | integration 與 mutation（`mutation/a8-2b.json`）待 CI；recorded：分頁依最後活動時間（CARRYOVER）、BUYER 查詢在 generic plan 下可能不走索引排序（未量測，觸發：收件匣查詢變慢或可跑 EXPLAIN 時；根治是依受眾拆查詢）；A8-1 留在 dev 的註解計畫標籤於 A8-8 清 | 本次 |
| A8-2c 後台公告 | 程式完成待驗收 | 本機（主 session 重跑）：gofmt、lint-monolith（0 issues）、vet integration、arch／httpapi／modules／cmd unit -race（只剩 A9 的 SellerProcurementAppService 未掛）、mutation --validate 全 ok；跨步 blocking（A8-2b 收件匣隱藏公告列）以註冊 Fanout 型別 `announcement` 解決；契約審查＋code-review 兩輪（findings 13，blocking 0；fixed 11、recorded 1：派送中重試可能重建使用者剛刪的公告（CARRYOVER）、rejected 1 nit：FailedPrecondition／PermissionDenied 視為永久略過） | integration 與 mutation（`mutation/a8-2c.json`）待 CI | 本次 |
| A8-2d 串流與賣家工作區 | 程式完成待驗收 | 本機（主 session 重跑）：gofmt、lint-monolith（0 issues）、vet integration、arch／httpapi／modules／cmd unit -race（只剩 A9 的 SellerProcurementAppService 未掛）、mutation --validate 全 ok、website lint 與 test:unit；前提更正（serve／worker 分開，改 LISTEN/NOTIFY，2026-11-24）；契約審查＋code-review 兩輪（findings 15，blocking 0；fixed 11、rejected 4：S3 依 D19 接受、3 nit） | integration、mutation（`mutation/a8-2d.json`）與 sqlc drift 待 CI（`internal/store/app.sql.go` 因 Docker 停擺手改，審查核對與 SQL 一致，CI 的 sqlc drift 會驗）；限流每分鐘 120 次是估計值 | 本次 |
| A8-3 渠道與事件訂閱 | 未開始 | | | |
| A8-4 主計畫承接條目 | 完成 | `rg -n "通知型別" backend/docs/plans/2026-09-24-go-modular-monolith.md` 命中；ADR 修訂段存在 | | 本次 |
| A8-5 website、admin | 未開始 | | | |
| A8-6 兩支 App | 未開始 | | | |
| A8-7 刪舊 | 未開始 | | | |
| A8-8 收尾 | 未開始 | | | |

## 基線

A8-0，2026-11-23，dev `bcc906501`（工作區另有其他 session 未提交的 B7 文件改動，不影響 Go 與 client 程式）：

| 檢查 | 結果 |
|---|---|
| backend `make lint-monolith` | 通過 |
| backend `make vet-all` | 通過 |
| backend `make test-all`（不含 integration tag） | 通過 |
| `services/notification-service` `go test ./...` | 通過 |
| website `lint`、`design:check --apps-root ..`、`contrast:check`、`test:unit`、`test`（vitest unit） | 通過 |
| admin `lint`、`test` | 通過 |
| buyer-app、seller-app `fvm flutter analyze`、`fvm dart format --set-exit-if-changed lib test` | 通過 |

未跑：integration（起容器，交 CI）、website `build`／`test:build-output`／e2e、Flutter 完整 `flutter test`（交 CI）。既有失敗：無。
