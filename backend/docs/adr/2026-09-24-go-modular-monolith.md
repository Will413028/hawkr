# 後端改為 Go modular monolith：架構決策集

| 項目 | 值 |
|---|---|
| 狀態 | accepted；D7 superseded by `2026-09-26-external-contract-protobuf-connect.md` |
| 修訂 | 2026-09-29：A3 盤點更正 seller 往上呼叫的描述；vendor portal 改屬 agent（Will 決定，計畫 `2026-09-29-a3-buyer-seller.md`） |
| 日期 | 2026-09-24 |
| 影響範圍 | 整個 `backend/`（22 個 deployable ＋ gateway）、`api/proto`、`infra/mq`、`k8s`、前端的 API client |
| 計畫 | `docs/plans/2026-09-24-go-modular-monolith.md` |
| 盤點資料 | `docs/plans/2026-09-24-go-modular-monolith/`（快照：monorepo `6c8bc0c15`） |

> 本檔只記 `WHY THIS DESIGN` / `WHY NOT THE ALTERNATIVES`。執行步驟、驗收條件、進度在計畫檔。

## 脈絡

- 公司 2026-09 結束營運，系統**從未上線**：沒有真實使用者，也沒有要保留的資料。程式碼併成個人 monorepo 繼續開發，開發者一人加 coding agent。
- 現況是 22 個 deployable（12 Go、10 Java）加 gateway：22 個資料庫、257 張表、730 個 gRPC RPC、212 筆 MQ 事件。
- 扣掉 gateway，有 16 支服務互相呼叫成一個環（15 組雙向呼叫）。一個業務動作被拆到多個資料庫，靠 outbox、saga、補償 worker、去重表拼湊一致性，而這些機制本身已有多處缺口（見 `flows.md`）。
- 不可動的約束：
  - 部署目標維持 k8s ＋ Cloudflare。
  - 不得連到舊公司的任何端點，prod 與 dev 都算（D9）。

## D1 單一 Go module、單一 binary 的 modular monolith

**考慮過**：(a) 維持微服務 (b) 只合併 Go 服務、保留 Java (c) 單體，但保留 `go.work` 底下的多個 Go module

**決策**：`backend/` 只有一個 `go.mod`，編出一個 binary，以 `serve`、`worker`、`migrate` 三種角色執行。Java 全部改寫成 Go。

- **否決 (a)**：微服務解決的是「多個團隊要各自部署」的組織問題，這裡不存在。它的成本卻全數存在：CI 與部署管線、MQ topology、跨庫一致性機制，以及回顧中記錄的事故類型（migrate 與 deploy 範圍不一致、topology 漂移、runner 排隊）。
- **否決 (b)**：兩套 toolchain、兩套慣例，而且 Java 和 Go 之間無法共用 transaction，D4 的主要收益拿不到。
- **否決 (c)**：多 module 會保留各自的 `go mod tidy`、版本與 CI matrix。邊界改由 `internal/` 和 arch test 強制（D2），不靠 module 切割。

## D2 模組邊界：24 個模組，依賴方向由 arch test 強制

**模組的定義**：一組業務規則（不變量）與這些規則所管資料的唯一擁有者。

- 位置：`internal/modules/<名稱>/`。
- 公開：`<名稱>.go`（對外方法與型別）、`events.go`（發出的事件）、`ports.go`（需要上層實作的介面）。
- 隱藏：`internal/` 底下的實作，其他模組 import 不到。
- 擁有：一個 Postgres schema，只有它能寫入。
- 說明：自己的 `AGENTS.md`，寫明職責、擁有的資料、不變量、讀取模型、不負責什麼。

**合併與分開的判準**：

- **合併**：同一條規則必須同時看兩邊的資料、而且沒有一邊能單獨擁有它；或者兩邊幾乎總是一起修改，且原因不是契約或資料副本造成的；或者兩邊講同一套語言。
- **分開**：有自己的生命週期、被多個模組使用；改變的原因不同（法規、外部 provider、演算法）；有隔離需求；或者對方只需要很窄的 API。
- **不是理由**：
  - 大小。單體裡的模組幾乎沒有成本。
  - 需要原子性。D4 允許跨模組共用 transaction，所以擁有規則的模組在呼叫方的 transaction 裡被呼叫即可。
  - 互相呼叫。這只是症狀，先檢查反向呼叫是不是放錯了地方。
  - 原本的部署切法、程式語言。

**模組與依賴**（只能依賴表內列出的模組；Go 本身禁止 import cycle）：

| 模組 | 內容（來源） | 可依賴 |
|---|---|---|
| identity | account-service：登入、session、角色、ADM 帳號（代行的 session 與稽核資料 2026-09-27 改屬 delegation，見 `2026-09-27-delegation-module.md`） | — |
| media | media-service | — |
| payment | payment-service：PayUni 收款、退款、撥款 API，會員訂閱計費 | — |
| buyer | buyer-service | identity, media |
| seller | seller-service（推薦賣家排行、vendor portal 除外） | identity, media |
| content | content-service | media |
| fulfillment | shipping-service（含 Uber、假日） | buyer, seller, media |
| catalog | product-service（含 category） | seller, buyer, media, fulfillment |
| productimport | product-import-service | catalog, media, fulfillment |
| pricebook | pricebook-service | catalog |
| invoice | invoice-service，並接手 order 的發票快照 | payment, fulfillment, buyer, seller, media |
| cart | cart-service | catalog, buyer, seller |
| coupon | coupon-service | identity, buyer, seller |
| ordering | order-service：訂單與 checkout | cart, coupon, catalog, fulfillment, payment, invoice, buyer, seller, identity, media |
| aftersales | aftersales-service | ordering, coupon, payment, fulfillment, buyer, seller, media |
| review | review-service | ordering, catalog, buyer, seller, media |
| im | im-service | ordering, buyer, seller, media |
| crm | crm-service | ordering, coupon, buyer, identity |
| finance | payment-service 的物流財務與責任判定，加上 order-service 的賣家撥款 | aftersales, ordering, payment, fulfillment |
| social | social-service | im, review, catalog, buyer, seller, identity, media |
| discovery | seller-service 的推薦賣家排行 | ordering, catalog, buyer, seller, review |
| delegation | 代行的授權、Operation Session、稽核（2026-09-27 起，`2026-09-27-delegation-module.md`）與 gateway 的代行編排：店家狀態檢查、對各模組的寫入 | identity, seller, ordering, catalog, social, media |
| accountdeletion | 刪除帳號的申請與流程：等待未結訂單、關閉買家（與賣家）、刪除帳號（2026-09-30 起，`2026-09-30-account-deletion-module.md`） | identity, buyer；A3b 加上 seller |
| agent | ai-service（含 procurement），以及 seller-service 的 vendor portal（採購向非會員供應商詢價的邀請與 session，A3 先建模組只放它） | ordering, catalog, pricebook, buyer, seller, media |
| notification | notification-service | buyer, seller；並訂閱各模組的事件 |
| （http） | gateway，以及 payuni-webhook 解散後的入口 | 全部模組 |

`platform`（config、db、River、log、otel、errors、codemeta 等）不是模組：模組可以依賴它，它不得依賴任何模組。

**反方向的需要，依序三選一**：

1. 只是畫面要顯示別人的資料：移到 http 層組裝。
2. 下層要通知上層：下層發事件，上層訂閱（D4）。
3. 下層需要上層做前置檢查：port。下層定義 interface，上層實作，在組裝點接線。目前已知 3 處：
   - seller 認證申請，需要確認「有已發布的貼文」，由 social 實作。
   - 刪除帳號（買家與賣家），需要確認沒有未結訂單：port 由 `accountdeletion` 定義，由 ordering 實作（2026-09-30 修訂，原本是 buyer、seller 各自定義，`2026-09-30-account-deletion-module.md`）。

**15 組雙向呼叫的定案**：

| 服務對 | 方向 | 反方向怎麼拿掉 |
|---|---|---|
| order↔payment | ordering → payment | 金額與 merId 由 ordering 傳入；付款結果是 payment 的事件；`SnapshotPaymentInfo` 與訂單上的付款副本刪除 |
| order↔shipping | ordering → fulfillment | 收件資料由 ordering 傳入；物流狀態是 fulfillment 的事件；訂單上的物流投影欄刪除 |
| aftersales↔order | aftersales → ordering | 開案、結案時由 aftersales 呼叫 ordering，設定「有進行中的售後」旗標並暫停或恢復倒數，ordering 不再反查；撥款前的 hold 查詢移到 finance |
| invoice↔order | ordering → invoice | 發票快照歸 invoice；付款成功時由 ordering 排入開票 |
| coupon↔order | ordering → coupon | 資格判斷需要的訂單統計由呼叫方傳入 |
| account↔seller | seller → identity | account 那一側其實是代行的店家狀態檢查，移到 delegation |
| media↔seller | seller → media | 呼叫方傳入 owner，media 不再反查 principal |
| buyer 與 order、product、seller、crm 四組 | 對方 → buyer；buyer 與 seller 互不依賴 | buyer 的往上呼叫都是清單補顯示欄位（追蹤、瀏覽、補貨），移到 http |
| seller 與 order、product、review、buyer 四組 | 對方 → seller | 推薦賣家排行移到 discovery；評價統計與粉絲數是補顯示欄位，移到 http；商品瀏覽由發出事件的一方帶 `store_id`；刪店前的未結訂單檢查用 port（見上）。原本寫「都是推薦排行」，A3 盤點更正 |
| seller↔social | social → seller | 用 port（見上） |

**證據**：同步呼叫圖（`rpc-*.csv`）、事件圖（`events.csv`）、一致性流程（`flows.md`），以及 455 個 PR 的 co-change（`cochange.py`）。

- co-change：77% 的 PR 只改一個服務；耦合度最高的一組只有 0.25（buyer＋seller）。cart＋order、coupon＋order、product-import＋product 一起被改的 PR 都不到 5 個。
- 結論：原本的業務邊界大致正確，問題出在依賴方向沒有規則，以及一個業務動作被拆到多個資料庫。

**否決**：

- **1:1 照搬服務**：會連同放錯位置的呼叫一起搬過來。seller 裡的推薦排行、buyer 裡的顯示組裝、account 裡的代行檢查，都會讓依賴圖出現環。
- **討論初期的 14 模組草案**：
  - crm 併進 buyer 不成立，因為 crm 依賴 ordering 和 coupon。
  - im 與 notification、social 與 review 與 content 之間沒有耦合，合併沒有理由。
  - cart、coupon 併進 ordering，以及 product-import 併進 catalog，理由都是「互相呼叫」或「只餵資料給對方」，屬於症狀而不是判準，co-change 數據也不支持。
- **delegation 整包留在 identity**：代行編排必須讀店家狀態並寫入各個被代行的模組，放在最底層的 identity 會讓依賴圖出現環。session 與稽核資料留在 identity，理由見下方「受影響的既有 ADR」。
  - > 2026-09-27 修訂：session 與稽核資料也改屬 delegation 模組，留在 identity 的理由不再成立（`2026-09-27-delegation-module.md`）。

## D3 單一 database、每模組一個 schema、禁止跨 schema SQL

**決策**：

- 一個 Postgres database，每個模組一個同名 schema。
- 模組的 SQL 只能碰自己的 schema，由 CI 用 Postgres parser 解析每一條 query 來強制。
- 不建跨 schema 的 FK；參照其他模組的資料一律存 ID。

**跨模組讀取的判斷順序**：

1. **先檢查資料是不是放錯了擁有者。** 用來篩選或排序的資料，歸查詢它的模組所有；事實發生在哪個模組，就由那個模組呼叫 API 更新。例如：銷量歸 catalog，由 ordering 更新；「進行中售後」旗標歸 ordering，由 aftersales 設定。
2. **依 ID 取資料或補顯示欄位**：批次呼叫對方的 API。
3. **必須拿下層模組的資料來篩選或分頁**：在自己的 schema 維護讀取模型。
   - 以來源實體為單位，例如 catalog 裡每家店一列的 `store_state`。
   - 由來源模組的事件在同一個 transaction 內更新，而且只有這一個寫入者。
   - backfill 寫在新增該表的同一個 migration。
   - 每一份都寫進模組的 `AGENTS.md`。

**Migration**：

- 用 Atlas，每個模組有自己的 `db/` 目錄：declarative schema、versioned migrations、`atlas.sum`。
- `hawkr migrate` 在部署前一次套用全部模組。
- 因為沒有跨 schema FK，模組之間沒有 migration 順序依賴。
- Flyway 退場。
- 系統沒有上線資料，所以每個模組都從新的 baseline 開始，不沿用舊的 migration 歷史。

**否決**：

- **保留各自的資料庫**：失去 D4 的單一 transaction。
- **允許 join 對方的表**：Fowler 所說的 Integration Database。GitLab 拆 CI 資料庫時，最大的障礙就是散落的跨表 join。
- **允許 join 對方公開的 view**：Sam Newman 把 Database View pattern 定位為拆庫期間的過渡做法。另外 2026-09-24 實測，Atlas v1.1.3 在沒有登入 Pro 時，會把 schema 裡的 view 從產出的 migration 中**靜默略過**，不報錯。
- **跨 schema FK**：讓模組之間產生 migration 順序依賴，也讓模組無法單獨搬移。
- **現行的 30 份非刻意副本**：它們經由 MQ 非同步更新，有的有兩個寫入者，會遺失事件，是既有 bug 的來源。這與上面「同一 transaction、單一寫入者」的讀取模型本質不同。20 份業務或法定必須的時點快照（例如訂單明細價格）是模組自己的資料，照常保留。

## D4 Transaction 由最上層的 use case 開，跨模組共用

**決策**：

- use case 用 `platform` 提供的 `InTx(ctx, fn)` 開 transaction。
- 被呼叫的模組從 ctx 取得同一個 transaction，跨模組的寫入因此一起 commit 或一起 rollback。
- 下層通知上層時，發 in-process 事件：事件型別定義在發布方的 `events.go`，訂閱在組裝點註冊，handler 在同一個 transaction 內同步執行，失敗就讓整個 transaction rollback。
- 同 transaction 的事件只用在需要原子性的狀態變更；其餘一律改成 River job（D6）。

**否決**：每個模組各開 transaction，再用 saga 或 outbox 串接，也就是現況。

- 盤點出的機制中，約一半只因為拆服務而存在，例如：
  - `stock_tx_records` 與補償 worker
  - `cart_cleanup_retry_tasks`、`coupon_consumption_tasks`
  - `payment_event_dedup`、第二層 webhook inbox
  - RFQ outbox、RefundSyncWorker
  - 代行的 intent→confirm 兩段式
- 這些機制造成的缺口：
  - 建單後券沒被鎖住，可以重複使用。
  - 物流事件遺失時，已建物流單的訂單被取消並退款。
  - 建物流單失敗留下的空殼，讓補建重試全部失效。
  - aftersales rollback 後，訂單已取消卻沒有退款任務。
- 改用單一 transaction 後，上述缺口在結構上不可能發生。

## D5 外部 provider 呼叫不在 DB transaction 內

**決策**：

- 呼叫 PayUni、AMEGO、Uber、Apple、R2、FCM、LLM 時，先 commit 一筆帶冪等鍵的 intent 列，再由 River job 在 transaction 外呼叫，最後在新的 transaction 裡寫入結果。
- 外部進來的 webhook 一律先寫 `webhook_inbox`、先 commit，再交給負責的模組處理。
- 這是 `platform` 提供的單一機制，模組不得自己實作。

**理由**：外部呼叫無法 rollback。放在 transaction 內，DB 失敗時外部已經生效；放在 transaction 外又沒有 intent 列，崩潰時無從得知外部狀態。現有程式有三處違反，且合併資料庫也不會自動修好：

- payment 的 `createRefund` 在 transaction 內呼叫 PayUni。
- shipping 在 transaction 內、insert 之前呼叫 Uber；ADR `2026-09-14-uber-create-idempotency-precommit-key` 要求的第一層沒有實作。
- seller 的 `Approve` 在 transaction 外呼叫 PayUni，沒有 intent 列。

## D6 非同步：RabbitMQ 換成 River

**決策**：用 River（跑在 Postgres 上的 Go job queue）。job 和業務寫入在同一個 transaction 內排入；重試、排程 job、唯一 job 都是內建的；失敗的 job 可以查看、可以重試。212 筆事件的去向：

| 類別 | 筆數 | 去向 |
|---|---|---|
| 推動另一模組的狀態（STATE） | 67 | 上層直接呼叫下層；下層通知上層則用同 transaction 事件（D4） |
| 同步資料副本（PROJECTION） | 20 | 刪除，改成查詢或讀取模型（D3） |
| 通知、外部副作用（SIDE_EFFECT） | 83 | 在同一個 transaction 內排入 River job |
| webhook 轉送 | 9 | http 層寫 inbox 後，交給負責的模組 |

- notification 改成逐一明確訂閱有模板的事件。現在的 wildcard 訂閱會把 39 種沒有模板的事件默默丟掉，不能照搬。
- hot row 計數（social 的讚數與留言數、銷量）維持非同步，改成 River job。

**否決**：

- **保留 RabbitMQ**：資料寫了、訊息沒送出的 dual-write 問題仍在；topology 要另外用 Terraform 維運；96 個 DLQ 沒有 consumer。
- **只用 in-process goroutine**：不 durable，行程重啟就遺失。
- **換成其他 broker（NATS、Kafka）**：一樣有 dual-write 問題，而且多一個要維運的元件。

## D7 對外 HTTP 契約：OpenAPI spec-first

> **superseded by `2026-09-26-external-contract-protobuf-connect.md`**（2026-09-26）：對外契約改為 Protobuf＋Connect。以下保留原本的決策與理由。

**決策**：

- OpenAPI 3.x 規格檔是對外 HTTP 契約的唯一來源。
- http 層的 server 介面由規格生成；website、admin 的 TS client 與兩支 App 的 Dart client 也由規格生成。CI 擋住生成物漂移。
- 初始規格由現有的 swag 註解（約 600 個 endpoint）轉出，並以 gateway 的 JSON contract baseline 驗證，確保與現有前端的 wire 格式一致：camelCase、統一 envelope、ISO 8601 UTC。
- 生成工具在階段 0 選定並驗證。

**否決**：

- **維持 swag 的 code-first 註解**：註解和實作會漂移，前端的 client 至今全是手寫的。
- **用 proto 加 grpc-gateway**：內部的 proto 會在 D1 之後移除，不應該為了對外契約保留它。
- **TypeSpec**：多一套 toolchain；它最終也是編譯成 OpenAPI，日後需要時可以再加上。

## D8 遷移策略：strangler，並存只為了開發不中斷

**決策**：

- **階段 A**：先把 12 支 Go 服務和 gateway 併進單體，Java 服務照常運作。
  - > 2026-09-26 修訂：gateway 不在階段 A 併進單體，改當過渡入口、隨模組搬遷縮小（`2026-09-26-external-contract-protobuf-connect.md` D6）。
  - 單體需要呼叫 Java 時，先建好目標模組的公開 API，再由一個呼叫 Java gRPC 的暫時實作滿足它。呼叫方從第一天就照最終 API 寫。
  - 單體替仍在運作的 Java 服務提供 gRPC server，並做 RabbitMQ 的收發。
  - 以上暫時程式碼全部集中在 `internal/legacy/`。模組不得 import 它，只有組裝點接線。
- **階段 B**：Java 依 D2 的依賴圖，從依賴最少的往核心改寫。每個模組改寫完，就能直接以 in-process 的方式呼叫已經是 Go 的依賴，當下套用 D4 與 D5，不需要為了過渡而用 Go 重寫一次 saga 或 outbox。
- **並存的性質**：系統從未上線，所以沒有資料搬移、沒有雙跑、沒有流量切換。並存只是為了讓開發環境一直能跑、測試一直是綠的。
- **安全網有兩層**，任何一層沒過都不能刪掉舊實作：
  1. **HTTP 層的特性測試**：依設計書與既有行為寫情境測試，先對舊實作跑過，再對新實作跑。
  2. **integration 測試對照表**：現有的 205 個 Java integration 測試，逐一對應到 Go 測試，或標記為「依本 ADR 某決策退場」並註明理由，由 CI 檢查。
- **不搬**：設計書有寫、但程式沒接上的功能，包括 recurring-service、會員計費的 provider adapter、平台向賣家開的物流費發票、`openUnresolved`、`SellerMerchantIdResolver`。要不要做，另外決定。

**否決**：

- **一次全部重寫**：沒有任何一步能驗證，出錯無法定位。
- **先改寫 Java、再合併 Go**：Java 新模組會先被迫呼叫仍在遠端的 Go 服務，等於用新程式碼實作舊的跨服務機制。

## D9 禁止連線到舊公司的端點

**決策**：CI gate 以 denylist 擋下 runtime 程式碼與設定中的舊公司端點。

- 範圍：舊公司網域的所有 host、公司的 infra IP、prod DB 的 PlanetScale host、公司的 R2 帳號、舊公司的 ghcr.io 組織、公司的 Workers 子網域與 OSS bucket。
- 新單體的路徑零容忍。其他既有路徑以 ratchet baseline 起算，只能減少、不能增加。
- 本機與 CI 一律使用本機服務或測試替身。

## 受影響的既有 ADR

- **整份被取代**：`2026-09-18-rfq-acceptance-order-atomicity.md`。它選 outbox 的前提是「沒有跨服務 transaction」，D4 之後這個前提不存在，接受報價與建單在同一個 transaction 內完成。
- **部分被取代**：
  - `2026-09-18-adm-delegation-service-ownership.md` §2（intent → 寫入 → confirm 兩段式）改為同一個 transaction。§1 仍然成立：session 與稽核留在 identity，理由 (a) 要能對 `admin_accounts` 下 FK，在 D3 下依然有效（2026-09-27 修訂：§1 由 `2026-09-27-delegation-module.md` 取代；D3 不建跨 schema FK，(a) 其實不成立）。理由 (c)「新服務的固定成本」不再存在，但它本來就只是否決新建服務的理由之一。§3 到 §5 不變。
  - `2026-09-18-invoice-service-architecture.md` ADR-01（獨立微服務）改為獨立模組。其他性質都保住：
    - 保存期與 migration 節奏：靠自己的 schema 和 migration 目錄。
    - AMEGO 故障隔離：外部呼叫只在 invoice 自己的 River queue 裡執行，結帳路徑只負責排入 job（D5）。
    - 單副本排程：由 River 認領。
    - 發票狀態的唯一 SSOT：發票快照也移進 invoice，更強。
    - **唯一失去的是憑證邊界**：AppKey 會和其他 provider 的憑證在同一個 process 裡，列為接受的代價。
- **被強化**：`2026-09-11-logistics-finance-domain-ownership.md`。原本的「不得被結帳與退款路徑 import」只剩 package 邊界和人工檢查，現在由 finance 模組加上依賴表，交給編譯器與 arch test 強制。
- **其餘 ADR**：預設仍然有效。每個模組搬遷時，逐一檢查與它相關的 ADR。
- **`docs/開發規範/後端服務責任邊界規範.md` §七**：outbox 與 saga 的例外條款在單體內大多不再成立，於階段 0 改寫。

## 後果

**接受的代價**：

- 一個 process 握有全部的憑證。
- 無法只部署一部分，一次有問題的部署會影響全部。
- 單一資料庫是共同的故障點。
- 約 17 萬行 Java 要改寫。
- 階段 A 與 B 期間，`internal/legacy/` 裡會有暫時的轉接程式碼。

**應該重新評估的條件**：

- 某個模組需要獨立擴展，或需要獨立部署的節奏。
- 參與開發的人數增加，需要按團隊劃分所有權。
- 定期重跑 `cochange.py`，發現某兩個模組的耦合度持續偏高。此時先檢查邊界，再考慮合併。

## 補充：D9 的理由與 D3 的重新評估條件（2026-10-03）

決策不變。以下是 2026-09-24 討論時寫在另一份紀錄、本檔原文沒有寫下的內容，併回這裡，讓本檔是唯一的一份。

- **D9 的理由**：舊公司的網域、infra 與第三方帳號都不屬於這個 repo，連上去可能把資料送進別人的帳號。所以用 denylist 加只減不增的 baseline，新單體的路徑零容忍。
- **D3 再加一個重新評估的條件**：改用 Atlas Pro，或換成能正確處理 view 的 migration 工具時，可以重新評估 D3 否決的「允許 join 對方公開的 view」（否決理由之一是 Atlas 在沒有登入 Pro 時會靜默略過 view）。

## 修訂：反方向 port 的已知處只剩一處（2026-10-18，Will）

原決策段保留歷史。D2 的「下層需要上層做前置檢查：port」目前已知處只剩刪除帳號（`accountdeletion` 定義，ordering 實作）。原列的「seller 認證申請需要確認有已發布的貼文，由 social 實作」已不存在：該前置條件在 2026-10-03 刪除（`internal/modules/seller/CARRYOVER.md` 第 237 行），`backend/internal` 沒有 `HasPublishedPost`。

## 修訂：跨模組讀取改用擁有者發布的 pub_*、D4 收窄（2026-10-18，Will）

原決策段保留歷史；本段取代 D3 的「跨模組讀取的判斷順序」第 3 點與否決清單的「允許 join 對方公開的 view」，並收窄 D4。依據與業界比較：`docs/plans/2026-10-18-a7-social/d3-read-model-brief.md`（v3）。

**D3 不變的部分**：一個 database、每模組一個 schema、只有擁有者寫入、不建跨 schema FK、不 join 對方的內部表。

**跨模組讀取的判斷順序（取代原順序）**：

1. 資料的擁有者對嗎？用來篩選或排序的資料歸查詢它的模組所有，事實發生的模組呼叫 API 更新。
2. 只是依 ID 取資料或補顯示欄位：批次呼叫對方的 Go API。
3. 各來源都擁有全部篩選與排序鍵：在 httpapi 依同一個全序做 keyset composition。
4. 要用下層模組的屬性篩選或排序：join 下層擁有者發布的唯讀 `<owner>.pub_*`（view，或帶時間參數的 `BEGIN ATOMIC` SQL 函式，時間由請求的 clock 傳入）。依賴表（`internal/arch/modules.go`）允許的上層模組才能 join。`pub_*` 必須算「現在的狀態」，不能只是最後公告的值；規則只有一個家，擁有者的 Go 程式讀同一份 SQL。
5. 讀取模型只在量測觸發時才建：某支讀取在正式資料量下 p95 超過門檻，`EXPLAIN` 顯示瓶頸在跨 `pub_*` 的 join 而且加索引解決不了；或 D1 的重評條件成立、要拆某個模組。建的時候用 outbox 非同步投影（來源 transaction 內 `InsertTx` 排入投影 job，只帶 ID 與版本；唯一 job 的狀態集合不含 Completed，結算時比對來源版本，照 `media/internal/refsync`；加週期 sweep 與全量重建指令；指令端的不變量一律同步問擁有者，不讀副本）。這是終局，不是過渡。

**撤回「允許 join 對方公開的 view」的否決**：

- Newman 把 view 定位為拆庫期間的過渡做法，前提是要拆庫。D1 否決微服務，本專案沒有拆庫的目標，前提不成立；「改動隔離、資料擁有權、不變量只有一個家」的理由與拆不拆無關，由 `pub_*` 的契約形狀保住。
- Atlas 社群版靜默略過 view 與函式（2026-10-18 再量過，函式也一樣）：Atlas 是我們自己選的工具，這不是外部約束。改為 `pub_*` 的 migration 手寫（表照用 Atlas）；漏建由 `TestSchemaFileMatchesMigrations` 擋，底層欄位改型別或刪除由 Postgres 對 `BEGIN ATOMIC` 函式與 view 的依賴追蹤擋（integration CI）。
- 2026-10-20 更正上一點（Will）：Atlas v1.3.3 社群版不是略過 view，而是在 desired state 裡遇到 view 就報錯（`views are available to logged-in users only`），seller 的 `schema.sql` 放進 view 之後 `atlas migrate diff` 就無法執行。因此每個模組的 desired state 分成兩個檔：`db/schema.sql` 只放表、約束與索引，是 Atlas（`atlas.hcl` 的 `src`）唯一讀的檔；`db/objects.sql` 放 view、函式與複合型別，以 `atlas migrate new` 手寫 migration。sqlc 與 `TestSchemaFileMatchesMigrations` 依序讀兩個檔，`internal/arch` 的測試擋物件放錯檔。不採 `atlas login`（多一個第三方帳號與憑證），也不採腳本截斷 `schema.sql`（依賴檔內順序）。
- 否決「跨 schema FK」與「join 對方的內部表」維持不變。
- 2026-10-03 補充的「D3 重新評估條件」（Atlas Pro 或能處理 view 的工具）作廢，被本段取代。

**強制機制的措辭更正**：原文「CI 用 Postgres parser 解析每一條 query」不準確。實際是 sqlc（內建 Postgres parser）只讀自己的 `schema.sql` 加依賴表允許的模組的 `db/pub.sql`，參照其他物件在生成時失敗，不另做 parser。擁有者的 `db/pub.sql` 契約檔只宣告 `pub_*` 的形狀（函式宣告成 `RETURNS SETOF <pub 表>`，因為 sqlc 不型別化 `RETURNS TABLE`）。三道檢查：

- `internal/arch` 測試：每個模組 `sqlc.yaml` 的 schema 清單只能是自己的 `schema.sql` 加依賴表允許的模組的 `pub.sql`；`pub.sql` 裡每個 `CREATE` 的名字都必須是 `<自己的 schema>.pub_*`。
- 契約測試（integration）：`pub.sql` 的每個物件和套用 migration 後真實物件的欄位、型別、參數比對，兼作消費者契約測試。
- 既有的 `sqlc-foreign-schema` fixture：沒列在清單的 schema 會失敗。

`scripts/gen-monolith-sqlc.sh` 要先把所有模組複製進 stage 再逐一 generate。

**D4 保留，收窄三處**：

- (a) 同 transaction 的訂閱者只做兩種事：只寫自己的列、而且無法在讀取時推導的原子狀態變更；或者只排入 job、寫 inbox（relay 形狀）。能在讀取時推導的可見性，改為 join `pub_*`。
- (b) 會被別的模組呼叫的公開方法不得用 `InReadSnapshot`，另給加入呼叫方 transaction 的入口（例如 `CreateInTx`，不收 `request_id`，冪等由呼叫方的狀態轉移擁有）；撞唯一鍵後不得在同一個 transaction 繼續下 SQL。`InTx` 可重入，既有呼叫點不用改。
- (c) 跨模組 transaction 的全域鎖順序由下往上，條文如下：

> 在跨模組的 transaction 內，可能被兩個以上模組的路徑鎖到的列，一律依下列層級由下往上鎖：
> 1. identity 帳號列：錨點。任何涉及一個帳號、跨模組的流程，第一個鎖它（`Identity.LockAccount`）。
> 2. buyer 的列。
> 3. seller 的列，照 seller 內部既有順序：店家／stint → 申請 → 註冊 → 公告列；措施與通報在店家列之後。
> 4. 上層模組的列，依 `modules.go` 依賴表由下往上（例如 social 內容在 seller 之後）。
> 5. 編排者（moderation、accountdeletion）自己的列最後。
>
> 「鎖」指 `FOR UPDATE`／`FOR SHARE`、`UPDATE`、`DELETE`、會衝突的 `INSERT`。READ COMMITTED 下的一般 `SELECT` 不鎖列，驗證型讀取不受這條限制。上層模組不得在呼叫下層之前鎖住自己會被下層同步訂閱者碰到的列。

這和 accountdeletion 現有的順序一致（identity → buyer → seller →自己的列）。D4 對「由上往下、不變量跨模組」寫入的理由（四個實際缺口）不變。

**曝光規則**：`seller.store_exposures` 存的是最後公告的值，不是現在可不可以曝光，所以不能直接包成 `pub_*`。規則的每個述詞搬進 seller 擁有的 SQL（`seller.pub_exposable_stores(at)`），Go 的 `Classify`、`CapabilitiesOf` 讀同一份。`store_exposures`、`StoreExposureChanged` 與 15 分鐘對帳，只在還有行程外的 legacy 消費者時保留，B4 把 product 搬進單體後一併刪除。

**後果**：A7 計畫少 social 的曝光與封鎖兩份投影；遷移計畫 B4 不建 `store_state`；A7-1c、1d 少兩個同步訂閱者。單體規則 2、3、4、6 已更新（`backend/AGENTS.md`）。

**2026-10-19 修訂（A′(i) 審查）**：上文「`pub.sql` 裡每個 `CREATE` 的名字都必須是 `<自己的 schema>.pub_*`」改為：`pub.sql` 另外可以有一條 `CREATE SCHEMA <自己>`（sqlc 讀契約檔時要先有 schema），其餘每個 `CREATE` 仍必須是 `<自己的 schema>.pub_*`（`internal/arch` 的 `CheckPubSQL`）。消費端的查詢只呼叫 `pub_*` 函式，不得直接 `FROM` 契約檔為了 sqlc 宣告成表的回傳型別（同一個 arch 測試）。

## 修訂：用 go-jet 建 query 的模組，規則 2 的強制機制（2026-10-23，Will；A7-1a）

上文「強制機制」三道檢查針對 sqlc 讀的 query。模組改用 go-jet 建的讀取（共用可見性述詞的 query，`2026-09-27-module-internal-structure.md` 的修訂）沒有 sqlc 的 schema 清單擋跨模組參照，規則 2 的目標不變，強制改為（`social` 是第一個）：

- 模組只有自己的表型別：`make jet`（`cmd/jetgen tables`）對只套用該模組 migration 的暫時 Postgres 生成，所以 query 無法用型別參照別的模組的表。
- 跨模組只經擁有者 `pub.sql` 生成的包裝（`internal/pubsql`，`cmd/jetgen pub`，擁有者要在依賴表內）；原始 SQL 的逃生口（`CustomExpression`、`Token`、`Raw*`、`NewTable`）只許出現在這些生成檔（AST 測試）。
- 每支登記的 query 渲染出來的 SQL 由 `internal/platform/querycheck` 以 pg_query 解析：表只能是自己的 schema，函式只能是自己的 schema、內建白名單，或依賴表允許的 `<擁有者>.pub_*`。
- CI 的 `make sqlc` drift 步驟旁加 `make jet` drift 步驟。

規則 6 的例外：曾有「模組的程式碼生成器建立的 plpgsql 函式」一條（`migrate.Module.Generated`），2026-10-23 已刪除（social 的「計入」改成只參照自己的表的 `BEGIN ATOMIC` 函式，見 `2026-09-27-module-internal-structure.md` 最後一則修訂）；規則 6 只剩 `RETURNS trigger` 的例外。


## 修訂：模組 `AGENTS.md` 的角色（2026-10-28，Will）

原決策「說明：自己的 `AGENTS.md`，寫明職責、擁有的資料、不變量、讀取模型、不負責什麼」改為：模組 `AGENTS.md` 只放改這個模組前必須知道的規則（固定七節，每條不變量一句規則加守門測試與規格位置），大小上限 6,000 bytes；行為規格的唯一來源是 `docs/modules/<模組>.md`。原決策段保留歷史。理由與否決的方案見 ADR `2026-10-28-agents-md-layering.md`。
