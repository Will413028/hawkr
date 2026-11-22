# 遷移計畫 A6：review、im、crm 搬進單體

**Goal：** review-service、im-service、crm-service 搬進 `internal/modules/review`、`internal/modules/im`、`internal/modules/crm`，完成後（可驗證的終態見 A6-7）：

- 聊天與評價的封鎖、檢舉入口改送對話或評價的參照；im、review 以 in-process 呼叫 buyer、seller 的「是否封鎖」API（`buyer.IsBlockingStore`、`buyer.IsBlockedBetween`、`seller.StoreBlocksBuyer`，A7-1b 建好）。
- moderation 加聊天訊息與評價兩種對象：檢舉快照、隱藏與還原都 in-process。
- CRM「黑名單」併入 seller 的店家封鎖。
- IM 的即時通道依 ADR `2026-09-26-external-contract-protobuf-connect.md` D7 改走 Connect。
- 四個 client 改用生成的 Connect client。

同時刪除：三個舊服務、gateway 的聊天／評價／CRM 路由（29 條打到這三個服務，另 4 條 CRM 路徑改由各擁有者的 Connect 方法提供）、`openapi.yaml` 對應的 operation、單體的 `internal/legacy/reviewservice`、`crmservice`、`pkg/blocklist`、`pkg/moderation`、`api/proto/{review,im,crm}` 與 `pkg/proto` 對應目錄。

**上層：** 主計畫 `2026-09-24-go-modular-monolith.md` 的 A6 段與「模組完成的定義」；本檔只寫 A6 特有的步驟與驗收。進度寫在本檔，主計畫 A6 段只指向這裡。

**附件：** `2026-11-19-a6-review-im-crm/`，盤點快照為 dev `94796248`。

| 檔 | 內容 |
|---|---|
| `domain-inventory.md` | 7 張表、擁有的資料與不變量、沿用機制 M1–M19、背景作業 B1–B10、缺陷 RD1–RD10、ID1–ID9、CD1–CD10、設計題 Q1–Q11 |
| `interface-inventory.md` | 34 支 RPC、14 列事件、對外呼叫、33 條 REST（含 1 條 WebSocket）、其他消費者、單體現況耦合 |
| `client-inventory.md` | 四端呼叫點與 rg 指令、壞掉的封鎖與檢舉入口、手寫表、WebSocket 狀態機、測試與 mock |
| `decision-brief.md` | 要 Will 拍板的 A6-D1 到 A6-D8 與建議 |

- 三份盤點由唯讀 subagent 起草，每項附 `file:line` 與【量】／【推】；主 session 抽查：RPC 數（修正 36→34）、`IsBlockingStore`／`IsBlockedBetween`／`StoreBlocksBuyer` 的位置、`orderservice` 3 支方法只有生成碼沒有 legacy client 方法、`pkg/blocklist` 寫入端只剩套件自身（沒有呼叫者）。
- 盤點的「去向」欄是 2026-11-19 的快照，與本檔步驟不一致時以本檔為準。

## 前提（核對日 2026-11-19，dev `94796248`）

- **會改到或取代的部署是否在線**：不在線。系統從未上線（根目錄 `AGENTS.md`）；不受回滾窗口、資料搬移、雙跑約束，舊資料不搬。
- **聊天與評價的封鎖、檢舉從 A7-4e 起本機失敗**：Will 2026-10-18 接受（A7 計畫 Q1），持續到 A6-6。已確認仍成立：gateway 與 openapi 沒有 `/app/blocks*`、`/app/reports`，buyer-app、seller-app 仍呼叫（`client-inventory.md` §6）。
- **`pkg/blocklist` 已沒有寫入者**：im、review 讀到的是空集合且 fail-open，所以 characterization 測試不得以舊服務的封鎖行為為基準；封鎖行為以 A7 的封鎖語意（`2026-10-18-a7-social/block-brief.md`）與本檔決定為規格。已確認【量】。
- **A7-D31**（社群內容下架的複本：每內容的 partial UNIQUE；修復鎖 measure 列經 `seller.pub_lock_content_takedowns`，seller 在 social 之前，Will 2026-11-19）已合併；A6 的評價隱藏若接內容下架照同一鎖序。
- **ordering、catalog、coupon、notification 仍在 Java 或舊服務**：review、im、crm 對它們的依賴在 A6 期間走 legacy client 或 relay（依賴表 `internal/arch/modules.go:28-30` 已列 ordering、catalog、coupon）。`internal/legacy/orderservice` 要補 `GetOrderForReview`、`GetOrderForAfterSales`、`HasOrderBetween`（生成碼已有，`internal/legacy/gen/order/orderconnect/order.connect.go:561,581,591`）。
- **Java 不呼叫這三個服務的 RPC**，只消費 review 事件（order、product）、發布 `order.completed`／`aftersales.refund.completed`（`interface-inventory.md` §0）。已確認【量】。
- **ID 型別**：buyer、store 仍是 bigint；review、im 擁有的實體依 ADR `2026-09-25-row-identity-by-data-kind.md` 用 UUIDv7（A6-D3 的 moderation `Target` 依此通用）。
- **工具鏈**（預設，可推翻）：照根目錄 `AGENTS.md`。

**驗收基線：** 各步驟開工時對當時 HEAD 跑 `make check` 等價檢查與三個舊服務的 unit 測試，記在文末「基線」；既有失敗不在本階段修。CI 一次只推一批。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 模組只 import 依賴表允許的模組根套件；moderation 依賴內容模組（現在 social），所以 review、im 不得依賴 moderation | `internal/arch/modules.go`；A6-4 在 moderation 的依賴加 review、im；隱藏理由由 review、im 自己存 |
| 封鎖放在發起者的模組，讀寫檢查都帶觀看者本人，不影響計數（A7 封鎖語意） | `buyer.IsBlockingStore`、`buyer.IsBlockedBetween`、`seller.StoreBlocksBuyer`、`seller.StoresBlockingBuyer`；im 送訊息守門、review 列表的 SQL 述詞（RD1） |
| 處分紀錄與效果在同一個模組、同一個 transaction，只有一個寫入者（10e R12） | review、im 的隱藏紀錄（取代 `hidden_at`）；moderation 只經 review、im 的模組 API 隱藏與還原 |
| 跨模組 transaction 內由下往上鎖：identity → buyer → seller → 上層模組 → 編排者 | moderation 隱藏評價或訊息；review、im 的刪帳訂閱者（在 buyer、seller 的事件 transaction 內，只鎖自己的列） |
| SQL 只碰自己的 schema；跨模組篩選 join 擁有者的 `pub_*` | review 列表的封鎖過濾 join `buyer.pub_blocked_*`；A6-D2 選 (ii) 時 CRM 的封鎖標示 join seller 的 `pub_*`（crm 依賴加 seller） |
| Connect 讀取標 `NO_SIDE_EFFECTS`（ADR 09-26 D3） | `MarkRead` 不能放在讀取方法裡 |
| A8 之前 notification-service 收到的 `chat.message.sent`、評價事件 wire 欄位與語意不變；Java order、product 收到的評價事件形狀不變 | A6-1 的 `reviewrelay`、A6-2 的 `imrelay`（`interface-inventory.md` §2） |
| 即時推送只在訊息 commit 後送達：送出在同一個 transaction 發 Postgres `NOTIFY`（只帶對話 ID 與 seq），串流收到後以 `MessagesAfter` 讀 DB（A6-D15） | im 的 `Send`、`Hide`／`Restore`（預覽變更）；2b 的 LISTEN 連線與訂閱表；斷線補洞與推送共用同一條讀取路徑 |
| 並行期單一生效者：舊服務在 gateway 後面處理流量到 A6-6e；模組的 relay 與 legacy consumer 只在測試中啟用，A6-6e 刪路由的同一個變更才在 `cmd/hawkr` 接上，避免重複發事件或與舊 crm-service 搶 queue | `reviewrelay`、`imrelay`、crm 的 `order.completed`／`aftersales.refund.completed` consumer 的註冊點；A6-6e 驗收檢查舊服務已從 compose 移除 |
| 刪帳的接線：review、im 訂閱 buyer、seller 的刪除事件（同 tx，只寫刪帳者複本與排清除 job，單體規則 4；social 前例 `social.go:625-638`）；accountdeletion 不認識 review、im，依賴表不加（Will 2026-11-19，A6-D9） | review、im 的 `OnBuyerDeletionRequested`／`OnBuyerDeleted` 等訂閱者與 `inactive_authors` 類複本；`cmd/hawkr` 的事件註冊 |

## 規模

| | 數 | 來源 |
|---|---|---|
| RPC | 34（review 9、im 13、crm 12）；8 支沒有呼叫端（`HideReview`、`HideMessage` 等）直接刪 | `interface-inventory.md` §0、§1 |
| 表 | 7（review 1、im 2、crm 4） | `domain-inventory.md` §1.1 |
| 事件 | 14 列（發布 7、消費 7） | `interface-inventory.md` §2 |
| REST | 33 operation（含 WebSocket 1） | `interface-inventory.md` §4 |
| client | website 聊天、評價回覆、CRM；admin 0；buyer-app 聊天、評價；seller-app 聊天、評價、CRM | `client-inventory.md` §1 |
| 缺陷 | RD1–RD10、ID1–ID9、CD1–CD10 | 下方「既有缺陷」 |

約 12～15 session【推，A7 類比：非測試 9,036 行 vs social 9,388 行】。

## 範圍

**做：** 三個模組；moderation 加評價、聊天訊息對象；CRM 黑名單併入店家封鎖；IM 即時通道改 Connect；relay：`reviewrelay`（給 Java order、product 與 notification）、`imrelay`（`chat.message.sent` 給 notification）；crm 的 `order.completed`、`aftersales.refund.completed` legacy consumer；coupon legacy client；orderservice 補 3 支；四端改 Connect 與封鎖、檢舉入口改送參照。

**不做：**
- 舊服務不再改；舊資料不搬。
- 沒有呼叫端的 RPC（`BroadcastMessage`、`IsUserOnline`、`GetConnectionStats`、`DisconnectUser` 等）不搬。
- CRM 投影不重新設計成讀時聚合（資料在 Java ordering，Q7 的建議；ordering 搬遷時退場）。

**延後**（收尾時搬回主計畫）：
- 評分聚合改由 review 發布 `pub_*`、catalog 與 seller 讀取，刪 Java 的全量覆寫副本。觸發：catalog 搬遷。
- CRM 投影改讀 ordering 的 `pub_*`。觸發：ordering 搬遷。
- notification 消費 review、im 事件、刪 relay。觸發：A8。
- 先評價、後全額退款的評價不計分（A6-D7 (a)）。觸發：ordering 搬遷。

## 既有缺陷（處置）

證據在 `domain-inventory.md` §6；處置裡的 A6-Dn 見下方「決定」。

| # | 處置 | 步驟 |
|---|---|---|
| RD1 封鎖過濾只在商品列表且在分頁後 | 修（A6-D4）：過濾進 SQL 述詞＋回歸測試 | A6-1 |
| RD2 長度以位元組計 | 修：rune 計，上限寫進 proto＋回歸測試 | A6-1 |
| RD3 隱藏無法還原 | 修：隱藏紀錄表，含還原＋回歸測試 | A6-1、A6-4 |
| RD4 刪帳不處理評價 | 修（A6-D6）：刪除評價並重算評分＋回歸測試 | A6-1 |
| RD5 先評價、後全額退款的評價仍計入評分 | 接受（A6-D7 (a)，Will 2026-11-19）：延後到 ordering 搬遷 | — |
| RD6 快照採請求值 | 修：全取訂單項目＋回歸測試 | A6-1 |
| RD7 圖片截斷死碼 | 修：刪死碼、DB CHECK ≤4 | A6-1 |
| RD8 聚合競態 | 接受到 catalog 搬遷：Java `ReviewStatsConsumer` 不讀版本（`rg -n -i 'version' backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/mq/ReviewStatsConsumer.java` 無命中，2026-11-19），加版本會改 wire 形狀、違反不變量；relay job 執行時讀當下狀態重算（全量覆寫的消費者因此收斂到最後狀態，`step-1-design.md` §4.2） | A6-1 |
| RD9 暫時性故障誤報 NotFound | 修＋回歸測試 | A6-1 |
| RD10 單筆讀取不過濾隱藏 | 修：共用公開可讀述詞＋回歸測試 | A6-1 |
| ID1 隱藏訊息仍在預覽 | 修：同 tx 更新對話預覽＋回歸測試 | A6-2、A6-4 |
| ID2 presence 無授權 | 修（A6-D5）：只對有對話的對方、不給最後上線時間＋回歸測試 | A6-2 |
| ID3 刪除事件以帳號為單位抹除 | 修：依身分抹除；帳號刪除才全抹＋回歸測試（開工先確認能否只刪單一身分） | A6-2 |
| ID4 hub 只在本進程 | 修（A6-D15）：改 Postgres LISTEN/NOTIFY，多進程也成立 | A6-2b |
| ID5 停業查詢失敗放行 | 修：in-process 查 seller，失敗回錯誤 | A6-2 |
| ID6 以 target_id 猜房 | 修：從 session 取身分、以對話 ID 授權＋回歸測試 | A6-2 |
| ID7 背景 goroutine 發布 | 修：同 tx outbox（relay） | A6-2 |
| ID8 圖片引用釋放缺口 | 接受：同 tx 經 media 釋放，GC 在 media | A6-2 |
| ID9 OFFSET 與游標並存 | 修：以 `seq` keyset | A6-2 |
| CD1 黑名單沒有效果 | 修（A6-D2 (i)）：刪 `blocklist` 值與分頁 | A6-3 |
| CD2 改分類不驗值 | 修：值域、0 列 NotFound、CHECK＋回歸測試 | A6-3 |
| CD3 標籤名稱無唯一 | 修：`UNIQUE(store_id, name)`、`ON CONFLICT`＋回歸測試 | A6-3 |
| CD4 FK 不保證同店 | 接受：入口只有名稱路徑；新 schema 用複合 FK | A6-3 |
| CD5 部分成功回成功 | 修：同 tx 整批＋回歸測試 | A6-3 |
| CD6 consumer 錯誤分類不一致 | 修：統一 permanent／retry＋回歸測試 | A6-3 |
| CD7 `total_spent` 可為負 | 修：退款依 ledger 冪等、下限語意寫進 CHECK＋回歸測試；下限由每單淨額取 `GREATEST(…,0)` 結構保證，順帶吸收「部分退款排在全額後」的殘餘風險 | A6-3 |
| CD8 賣家看得到買家電話 Email | 修（A6-D8）：CRM 不回電話與 Email＋回歸測試 | A6-3 |
| CD9 分頁無次序鍵 | 修：keyset `(sort_key, id)`、刪快取 | A6-3 |
| CD10 tx 外呼叫 order | 接受：ordering 搬遷時消失 | A6-3 |
| client：三端手寫聊天預覽偵測（`client-inventory.md` §8） | 修：伺服器回訊息種類 enum，文字進共用文案目錄 | A6-5、A6-6 |

## 預設（可推翻；沒有真正的替代方案）

- 未讀數＝`messages.seq > participant.last_read_seq`，單欄游標，不維護計數（domain Q5）。
- Connect 列表用 keyset，不回總數，除非畫面需要（Q9）。
- 對話在首則訊息時建立、CRM 客戶在首筆完成訂單時建立（Q11）。
- CRM 以 legacy consumer 保留 ledger 與投影（Q7）。

## 步驟

每個子步驟一個 session，各自有驗收。每步的派工附檔案表（新增／修改／刪除），至少含 `internal/arch/modules.go`、`cmd/hawkr` 的註冊、`openapi_coverage_test.go` 的 `maxRESTOperations`、compose 與 `.github/workflows`。缺陷的 mutation 以缺陷表的「處置」欄為準：改回舊寫法（證據欄的 `file:line`），對應的回歸測試要失敗。每步開工照 feature playbook；驗收含「改回舊寫法，測試要失敗」的 mutation（在複本跑）。

### A6-1 review 模組（被擋於：A6-D4、A6-D6、A6-D7）

實作設計 `2026-11-19-a6-review-im-crm/step-1-design.md`（v2）。拆成三個子步驟，各一個 session：

- **A6-1a** 設計書與開工 design-review（完成）。
- **A6-1b** 模組本體：schema、提交、回覆、讀取、統計、隱藏與還原、`orderservice.OrderForReview`、RD1–RD3、RD6、RD7、RD9、RD10 的回歸測試與 mutation、`CARRYOVER.md`、`AGENTS.md`。
- **A6-1c**（被擋於 A6-1b）刪帳訂閱者與 purge job（RD4）、媒體同步、`revalidate.ProductTag`、`reviewrelay`（wire＋golden）、完成前 design-review。review 模組在 `cmd/hawkr` 建構並接刪帳訂閱者與媒體同步 worker；`legacy/reviewservice` 的 3 個呼叫端不換（A6-D11，移到 A6-6e）。


- 範圍：`internal/modules/review`（schema、評價建立與回覆、公開可讀述詞、隱藏紀錄與還原 API、封鎖過濾 SQL 述詞）；`orderservice.GetOrderForReview`；`reviewrelay`（形狀同舊事件）；RD1–RD10 處置；`CARRYOVER.md` 沿用盤點（M 列）。
- 消費端：`rg -n 'reviewservice\.' backend/internal --type go`（3 個呼叫端，`interface-inventory.md` §5.3）；Java `rg -l 'review' backend/services/java --glob '*.java' | rg -i consumer`。
- 刪帳：依 A6-D6、A6-D9，訂閱 buyer 的刪除事件，寫 `review.inactive_authors` 並排 purge job（`step-1-design.md` §3.4）；驗收含 `accountdeletion_lockorder_integration_test.go` 涵蓋 review 列在 buyer、seller 之後才鎖。
- 退款：A6-D7 選 (a)，照舊：只放行訂單狀態 50（已完成），退款後的訂單（60、70）本來就不能評；先評後退款的評價仍計分，延後到 ordering 搬遷。
- 不能動：舊 review-service；gateway 路由（A6-6e 才刪）。
- 驗收：unit＋integration（真 Postgres 在 CI）；relay 的 wire 形狀對舊事件做 golden 比對；RD 各回歸測試的 mutation。

### A6-2 im 模組（被擋於：A6-D1、A6-D3 通道、A6-D5）

拆 2a、2b：

- **2a 資料與送出**，照 A6-1 拆成兩個 session：**2a0** 設計書 `2026-11-19-a6-review-im-crm/step-2a-design.md` 與開工 design-review；**2a1**（2a0 完成，決定見 A6-D14）實作與完成前 design-review。內容：schema（UUIDv7、`seq`、`last_read_seq`）、送出守門（停權→停業→封鎖，含 A6-D1 的例外）、`orderservice` 的 `OrderParties`、`HasOrderBetween` 與組合兩者的 `OpenOrderBetween`（A6-D14 W1 (c)）、帳號與身分刪除的抹除（ID3，訂閱 `buyer.BuyerDeleted`、`seller.StoreRetired`）、`imrelay`、buyer、seller 事件改 in-process 訂閱（不訂 identity，設計書 §9）。
- **2b 即時通道**，照 2a 拆兩個 session：**2b0** 設計書 `2026-11-19-a6-review-im-crm/step-2b-design.md` 與開工 design-review；**2b1**（2b0 完成）實作與完成前 design-review。內容：依 A6-D3 server streaming 收；commit 後推送依 A6-D15（LISTEN/NOTIFY）；presence 依 A6-D5，以 unary 輪詢（A6-D16）；不做打字中（A6-D15）；只有關店讓模組結束串流，停權與封鎖不結束，session 失效、帳號刪除由 A6-5 的串流 helper 定期重驗結束（A6-D16 W4）。心跳、壽命、寫入期限歸 A6-5 的 helper。
- 刪帳：ID3 依身分抹除；依 A6-D9 訂閱 buyer、seller 的刪除事件（同 tx 只寫複本與排抹除 job），lockorder integration 測試涵蓋 im。
- 消費端：gateway `G/handler/chat.go`、`gatewayproxy` 的 WebSocket 轉發；notification-service 的 `chat.message.sent`。
- 驗收：守門的表格測試（每種封鎖方向 × 有無訂單）；streaming 斷線以游標補洞的 integration；ID 各回歸測試的 mutation。

### A6-3 crm 模組（被擋於：A6-D2、A6-D8；A7-1b 已完成）

照 A6-1、A6-2 拆成兩個 session：**A6-3a** 設計書 `2026-11-19-a6-review-im-crm/step-3-design.md` 與開工 design-review；**A6-3b**（A6-3a 完成、設計書的待決題已決）實作與完成前 design-review。

- 範圍（A6-D18 修訂）：`internal/modules/crm`（設計書 `step-3-design.md`）；`order.completed`、`aftersales.refund.completed` legacy consumer（只在測試啟用，A6-6e 接線），ledger 冪等鍵：`order.completed` 取 `event_id`，退款取 `external_ref_id`（缺時 `case_no`）——Java 每次重發退款都換新 `event_id`（`RefundCompletedPayload.java:59`）；黑名單依 A6-D2 (i) 刪除；刪帳與店家退場訂閱者（`inactive_parties`，依賴表加 seller，理由：訂閱 `seller.StoreRetired`）；依賴表 crm 去 `identity`、`coupon`；建好 `crm.VIPStatuses`，但 `GetBuyerVipStatuses` 換成 in-process 與刪 `internal/legacy/crmservice` 移到 A6-6e。不建 coupon legacy client。
- 驗收：consumer 重放冪等測試；同客戶不同訂單並發；CD 各回歸測試的 mutation（設計書 §11）。

### A6-3c 停用身分改由擁有者守門（A6-D19）

照 A6-1 拆兩個 session：**A6-3c0** 設計書（以 `step-3c-brief.md` 為輸入，含 buyer、seller 新 API 的完整清單、各模組改點、migration）與開工 design-review；**A6-3c1** 實作與完成前 design-review。

- 範圍：buyer、seller 新增共享守門與狀態 `pub_*`，業務寫入鎖降為 `FOR NO KEY UPDATE`（A6-D20 W4）；sweep 唯一性 helper 收齊全部 14 處（A6-D20 W6）；review、im、crm 改用擁有者守門、刪 `inactive_*` 表與 advisory lock、保底掃描改讀擁有者 `pub_*`；`internal/arch` 依賴與 lockorder 登記；各模組 `CARRYOVER.md` 更新。
- 不能動：social 的 `inactive_authors` 與每日對帳；各模組抹除 job 的刪除範圍；im「刪帳申請中仍可聊天」語意；守門鎖取在 RPC 之後（`im` 的 `send.go` 先呼叫 order 再進 tx）。
- 消費端：`rg -n "inactive_parties|inactive_authors|LockParty|LockPartyShared|LockActive" backend/internal`。
- 驗收：lockorder integration（反序要失敗）；每模組「停用後立即看不到、寫入被擋、重放不重建」integration；mutation：拿掉守門鎖 → 停用與寫入交錯測試失敗；拿掉讀取 join → 可見性測試失敗；im 刪帳申請中可送訊息的回歸測試。
- 觸發改回複本：量到熱門店家 multixact 或鎖等待有問題時，該擁有者改 advisory 並替 lockorder 加分類器。

### A6-4 moderation 加評價、聊天對象（被擋於：A6-1、A6-2、A6-D3 對象）

拆三個 session：**A6-4a** 設計書 `step-4-design.md` 與開工 design-review（完成）；**A6-4b** REVIEW、CHAT_MESSAGE 對象、快照（依種類分開，GetCase 對外）、經 review／im 的隱藏與 `ReverseHide`（A6-D21 W1、W4）；**A6-4c** 店家內容走 seller 下架（`CHAT_MESSAGE`、`REVIEW_REPLY`）與店家檢舉聊天訊息（A6-D21 W2、W3），完成前 design-review 涵蓋 4b＋4c。
- 鎖序前提已核對：隱藏與還原不取擁有者守門，計畫的 buyer → seller → review/im → moderation 不變（設計書 §5；4b 的並行 integration `TestHideRacesWithEraseAndSendWithoutDeadlock` 在 lockorder 檢查器下通過，mutant「CloseCase 移到 im.Hide 前」被抓到，已證明）。

- 範圍：`TargetKind` 加 REVIEW、CHAT_MESSAGE（proto、Go、CHECK migration）；快照形狀（聊天含前後文，A6-D3）；隱藏與還原經 review、im API 同 tx；作者為店家時走 seller 措施（A7-D5′）；依賴表 moderation 加 review、im；`reviewqueue` 分支不變（來源仍是 moderation）。
- 驗收：檢舉→隱藏→還原的 integration；鎖序測試；mutation：還原時不清隱藏紀錄、隱藏時不更新對話預覽（ID1），測試各要失敗。

### A6-4d 店家退場清店家檢舉人資料（被擋於：A6-D22；不擋 A6-5）

- 範圍：moderation 訂閱 `seller.StoreRetired`（同 transaction，沿用買家刪帳訂閱者的模式），清該店全部店家檢舉的說明、冪等紀錄（request_hash）與 `reporter_daily_counts` 的店家列；檢舉列留著。CARRYOVER 第 29 列補店家路徑。
- 消費端：`rg -n "StoreRetired" backend/internal`。
- 驗收：integration 測試證明退場後三者清空、檢舉列仍在、重送事件冪等；mutation（CI）：拿掉計數列的清除，測試要失敗。

### A6-5 對外 API（被擋於：A6-1～4）

拆四個子步驟，各自一個 session：
- **5a** 契約設計書 `2026-11-19-a6-review-im-crm/step-5-design.md`：34 支逐支對到 Connect 方法（扣刪除 8 支）、`ReportTarget` 參照形狀（含店家檢舉人的 `CHAT_MESSAGE` 入口）、訊息種類 enum、卡片訊息的伺服器參照（A6-D14 W6）、串流 helper（心跳、壽命、寫入期限、session 重驗，A6-D16 W4）、CRM 的 lookup／meta／orders／coupons 由哪個擁有者提供；取捨列成 W 題交 Will，開工 design-review。
- **5b** `ReviewService` 與評價檢舉（5a 已決）。完成（PR #425）。
- **5c** `ChatService`（含 streaming helper、接 `cmd/hawkr`）與聊天檢舉（5a 已決）。開工第一件：在 `internal/httpapi` 加一支 unit 測試，以 AST 找出 repo 內所有 `httpapi.Deps{...}` 組合字面值（含 `*_integration_test.go` 與 `cmd/new-module` 範本），斷言 `NewHandler` 必填的依賴欄位都有設；mutation：刪掉任一處的 `Review:`，測試要失敗。理由：5b 加必填 `Review` 後漏了 9 個 integration 測試與 1 個範本，執行期才報錯，CI 連續三輪失敗（Will 2026-11-22）。
- **5d** `SellerCrmService` 與擁有者方法、CD8 驗收（5a 已決）；完成前 design-review 涵蓋 5b–5d。

- Connect：`ReviewService`、`ChatService`（含 streaming）、`SellerCrmService`；檢舉送參照（`ReportTarget` 帶評價或訊息 ID）；封鎖入口沿用 buyer、seller 的既有方法；訊息種類 enum 與文案目錄；CRM 的 `meta` 由 `GetCustomer` 取代，`lookup`、`coupons` 刪除，`orders` 走店家訂單列表（A6-D18 W3、A6-D23）。
- 必做方法清單：設計書 `step-5-design.md` §1 的 34 列去向表（對外 25 支舊方法 → 24 個 Connect 方法，加 W1 的店家版方法）；派工附上整張表。
- 驗收：`buf lint`；handler 測試涵蓋清單的每個方法；生成檔無 drift；mutation：拿掉檢舉參照的擁有者檢查，測試要失敗。
- CD8 驗收：`SellerCrmService` 回應欄位測試證明永不回 phone、email；mutation：把聯絡欄位加回回應，測試要失敗（A6-3 設計書 §11.2）。

### A6-5e 商品評價統計（A6-D24 (2)；被擋於：5b；擋 6c2 的商品評價顯示）

- 範圍：review 模組加商品層級的評價數與平均分讀取（照 `StoreRatingStats` 的形狀與公開述詞），`ReviewPublicService` 加對應方法；TS／Dart 重新生成。
- 消費端：`rg -n "StoreRatingStats|ProductRating" backend/internal api buyer-app/lib website/src`。
- 驗收：handler 與模組測試（平台隱藏、作者刪帳的評價不計入，與公開列表同一述詞；店家回覆被下架不影響評價計數），回應含星等分佈（同店家統計）；`buf lint`、生成無 drift；mutation：統計不套公開述詞，測試要失敗。

### A6-6 client（被擋於：見下；Q1 的缺口在這裡結束）

- 被擋於（Will 2026-11-22 改）：6b admin、6c1 只用評價與聊天，被 5b、5c 擋（已完成），可與 5d 並行；6c2 另被 A6-5e 擋；6a、6c、6d 合併到整合分支 `a6-clients`，與 6e 同批進 dev（A6-D25）；6a website、6d seller-app、6e gateway 用到或刪除 CRM，被 5d 擋。並行時後合併的分支重跑 `@hawkr/api`、`packages/hawkr_api` 生成，不手解生成檔衝突。

拆 6a website（測試清單＝`client-inventory.md` §7 全部檔案）、6b admin（評價、聊天檢舉的後台處理）、6c buyer-app（拆 **6c1** 評價、檢舉、封鎖改接 Connect，聊天整段維持舊路徑含其檢舉與封鎖，不出現半新半舊；**6c2** 聊天改串流 client、回覆引用拿掉（A6-D24）、商品評價統計改讀 A6-5e）、6d seller-app、6e gateway 與 openapi 清單（刪 33 條、`maxRESTOperations` 扣除），並把 `httpbuyer.Ports.CRM` 換成 `crm.VIPStatuses`、刪除 `internal/legacy/crmservice`（A6-D18），把 `legacy/reviewservice` 的 3 個呼叫端（`cmd/hawkr/discovery.go`、`cmd/hawkr/seller.go`）換成 `review.StoreRatingStats` 後刪除 `internal/legacy/reviewservice`（A6-D11）。每端：改生成 client、封鎖與檢舉入口改送參照、WebSocket 狀態機換成 streaming client、手寫預覽偵測刪除；e2e 的 `/app/chat/*` mock 一併改（採購助理 4 支 spec）。

- 驗收：各端 CI 等價檢查；e2e 推到 CI；6e 接上 relay 與 consumer 後，以 backend-e2e-verify 擷取 `reviewrelay`、`imrelay` 的 RabbitMQ wire 與 crm consumer 的入帳；封鎖與檢舉按鈕以單體實打（backend-e2e-verify）。
- UI：封鎖名單分頁改名或拿掉（A6-D2）先取得同意。
- 6a 必做：website 公開店面評價第 2 頁之後遇到伺服器錯誤顯示成空清單（`fetchSellerReviews` 的 softFail，A3 步驟 10d 第 6 刀 design-review 記下）；改讀 review 模組的 Connect 時，讀取失敗要和「沒有評價」分開呈現，附一個讀取失敗時不顯示空清單的測試。

### A6-7 收尾（被擋於：A6-6）

- 刪三個舊服務、`pkg/blocklist`、`pkg/moderation`（`internal/platform/textfilter` 成為唯一詞典）、`api/proto/{review,im,crm}`、`pkg/proto` 對應目錄、compose 與 CI 相關設定。
- 需要人做的事（已知）：A6-D2 的 UI 同意、兩支 App 的真機登入驗證。
- `check-inventory.py`：自 A7 複製，改盤點檔路徑與缺陷前綴（RD、ID、CD）。
- 完成定義逐項核對；design-review；`check-inventory.py`（自 A7 複製改參數）確認去向與處置沒有空格；延後項搬回主計畫；需要人做的事列給 Will。

## 決定

各題的理由與否決方案收在三份 ADR，下表只留結論：`docs/adr/2026-11-22-im-realtime-delivery-connect-streaming.md`、`docs/adr/2026-11-22-review-chat-crm-policy-in-monolith.md`、`docs/adr/2026-11-22-deactivated-identity-guard-at-owner.md`。

全部已決（Will 2026-11-19；D9 起逐題附日期）：D7 選 (a)，其他照 `decision-brief.md` 的建議；D9 在 A6-1a 開工審查時追加。

| 題 | 結論 | 用在 |
|---|---|---|
| A6-D1 | 有進行中訂單或售後未結案時仍可聊，雙向一致 | A6-2a |
| A6-D2 | (i) 刪 `blocklist` 值與 CRM 分頁，封鎖只在店家封鎖名單 | A6-3、A6-6 |
| A6-D3 | server streaming 收、unary 送，不留 WebSocket；後台只看被檢舉訊息前後各 5 則的固定快照 | A6-2b、A6-4 |
| A6-D4 | 觀看者封鎖作者或店家就看不到；店家封鎖買家不影響公開；統計不過濾 | A6-1 |
| A6-D5 | (ii) 只對有對話的對方顯示在線，不給最後上線時間 | A6-2b |
| A6-D6 | 刪除評價並重算評分 | A6-1 |
| A6-D7 | (a) 照舊；先評後退款仍計分，延後到 ordering 搬遷 | A6-1 |
| A6-D8 | CRM 只顯示暱稱與訂單統計 | A6-3 |
| A6-D9 | 刪帳改由 review、im 訂閱 buyer、seller 的刪除事件，不由 accountdeletion 直接呼叫（Will 2026-11-19，A6-1a 開工審查第 5 項） | A6-1c、A6-2a |
| A6-D10 | 提交評價先檢查訂單擁有者，不是自己的訂單回 NotFound，不照舊碼先查狀態（Will 2026-11-19，A6-1b code-review 第 4 項） | A6-1b |
| A6-D11 | 店家評分統計的讀取（discovery、seller 的 `StoreRatingStats`）留在舊 review-service 到 A6-6e，與刪路由同一個變更才換成 review 模組並刪 `legacy/reviewservice`：A6-6e 前評價仍寫進舊服務，提早換會讓評分讀到空表，違反「並行期單一生效者」（Will 2026-11-20，A6-1c 開工） | A6-1c、A6-6e |
| A6-D12 | `review.purge_author` 每批刪除後，同 tx 對該批涉及的 (店, 商品) 發 `ReviewAggregateChanged`；刪帳當下的 `ReviewAuthorDeactivated` 保留。理由：purge 先於 relay 跑完時 relay 列不到目標，Java 聚合會停在舊值；每批工作量有上限，不違反設計審查第 9 項（Will 2026-11-20，A6-1c 實作發現） | A6-1c |
| A6-D13 | 刪帳的評分重算只走 purge 每批的 `ReviewAggregateChanged`（A6-D12），刪除 `ReviewAuthorDeactivated` 與 relay 的逐組展開，取代設計審查第 9 項的單一事件：purge job 與訂閱者同 tx 排入、可立即執行，兩條路徑延遲同級，留兩條只會每組重送（Will 2026-11-20，A6-1c 完成前 design-review 第 1 項）。purge 另加週期掃描補排失敗的 job（第 2 項） | A6-1c |
| A6-D15 | 即時推送走 Postgres LISTEN/NOTIFY（送出同 tx 發 `NOTIFY`，commit 後才送達；串流以 `MessagesAfter` 讀 DB），不在 `platform/events` 加 commit 後掛點，取代「hub 單進程扇出」與 ID4 的接受；不做打字中，A6-6 刪三端的打字中（Will 2026-11-20，A6-2b 開工） | A6-2b、A6-6 |
| A6-D16 | A6-2b 設計書 §13 W1–W5 全照建議：NOTIFY payload 加帶 buyer_id、store_id 路由鍵，內容不經通知（W1 (ii)）；presence 以 unary 輪詢（W2 (a)）；任一方向封鎖即對被封鎖方顯示離線（W3 (i)）；停權、封鎖不結束串流，只有關店由模組結束，session 失效與帳號刪除由 A6-5 helper 重驗結束（W4 (i)）；保留已讀指示，`MarkAllRead` 不逐對話推給對方（W5 (i)）（Will 2026-11-20） | A6-2b1、A6-5 |
| A6-D17 | im 寫入 transaction 設時間上限（statement_timeout、idle_in_transaction_session_timeout），重連補洞起點往回放同一常數加時鐘誤差（Will 2026-11-20） | A6-2b1 |
| A6-D18 | A6-3 設計書 §13 W1–W5 全照建議：VIP 查詢切換與刪 `legacy/crmservice` 延到 A6-6e（W1 (a)，同 A6-D11）；刪帳與店家退場時清除 crm 資料並以 `inactive_parties` 擋重放（W2 (b)）；不建 coupon legacy client、CRM 不回優惠券數（W3 (a)）；標籤上限每店 200、每客戶 20（W4 (a)）；拿掉 CRM 客戶列表的關鍵字搜尋，A6-6 改 UI（W5 (a)，Will 已同意這項 UI 改動）（Will 2026-11-20） | A6-3b、A6-6 |
| A6-D19 | 停用身分的事實與守門只在擁有者：buyer 加「未關閉」、seller 加「未退場」的共享列鎖（`FOR KEY SHARE`）與狀態 `pub_*`，獨占端沿用既有的 `LockForDeletion`、`RetireDueStores`；review、im、crm 刪自己的 `inactive_*` 複本與 advisory lock，讀取改 join 擁有者 `pub_*`；抹除 job 留在各模組；social 保留複本（存旗標，規則 2 例外）；不做平台框架。簡報 `2026-11-19-a6-review-im-crm/step-3c-brief.md`（Will 2026-11-21） | A6-3c、A6-4 |
| A6-D20 | A6-3c 設計書 §10 W1–W7：W1 (a′) 不存在的 ID 當已停用並跳過；NotFound 時查 pub 分辨，已關閉／已退場記 info、不存在記 error（修訂 Will 2026-11-21）；W2 (a) 新增 `pub_closed_buyers()`；W3 (a) `LockActive`／`LockBuyerShared` 降為 `FOR KEY SHARE`；W4 (b) 業務寫入降為 `FOR NO KEY UPDATE`、只有轉移保留 `FOR UPDATE`；W5 (a) 不加索引、門檻寫進 CARRYOVER；W6 (b) sweep 唯一性 helper 本步收齊全部 14 處；W7 (b) 整合測試替身斷言 legacy RPC 不在 tx 內（Will 2026-11-21） | A6-3c1 |
| A6-D21 | A6-4 設計書 §W：W1 (a) moderation 新增 `ReverseHide`，案件維持結案，另記還原欄位與稽核紀錄，同 tx 呼叫 review／im／social 的 `Restore`；W2 (a) 店家寫的聊天訊息與評價回覆照 A7-D5′ 走 seller 內容下架，新增 `CHAT_MESSAGE`、`REVIEW_REPLY` 對象，im 讀取 join `seller.pub_content_takedowns()`；W3 (a) 檢舉人改為買家或店家二選一，只有 `CHAT_MESSAGE` 接受店家檢舉；W4 (a) `GetCase` 回應加依種類分開的快照欄位，快照 struct 改依種類分開，後台頁面在 A6-6（Will 2026-11-21） | A6-4 |
| A6-D22 | 店家退場（`seller.StoreRetired`）時比照買家刪帳：同 transaction 訂閱者清該店以店家檢舉人身分送出的檢舉說明、冪等紀錄與每日計數列，檢舉列與檢舉人鍵保留（Will 2026-11-21） | A6-4d |
| A6-D23 | A6-5a 設計書 W1–W4：W1 (g) 聊天依集合分買家版與店家版方法，店家由呼叫者此刻的店推導；W2 (c) 店家檢舉身分由訊息歸屬推導；W3 (a) 卡片延到 B8；W4 (a) 送出帶 `request_id` 去重；W5 (a) 刪 CRM `lookup`，優惠券從客戶名單選；W6 (a) CRM 不暴露訂單，改用店家訂單列表加買家篩選（Will 2026-11-21） | A6-5c、A6-5d、A6-6 |
| A6-D24 | A6-6c 實作中浮現的三題：(1) 聊天「回覆引用」（舊 App 以文字前綴拼成，契約沒有）先拿掉，列入延後，要做時在契約加 `reply_to` 欄位；(2) 商品層級評價數與平均分由 review 模組新增統計方法提供（照 `StoreRatingStats`），client 不自行分頁加總（A6-5e）；(3) 評價列不提供封鎖，只留檢舉（契約只能封鎖回覆的店家）（Will 2026-11-22） | A6-5e、A6-6c |
| A6-D25 | client 切換與 A6-6e 同批進 dev：6a website、6c buyer-app、6d seller-app 各自完成並開 PR 跑 CI，但合併到整合分支 `a6-clients`（從 dev 開，定期 merge dev），與 6e（接上 relay、consumer、評分讀取換 review 模組、刪路由）一起以單一 PR 進 dev；理由：並行期單一生效者，任一 client 先切會讓資料分在單體與舊服務兩邊，提前接 crm consumer 會與舊 crm-service 搶 queue。6b admin 與後端步驟（5d、5e）不受影響，照常進 dev（Will 2026-11-22） | A6-6a、6c、6d、6e |
| A6-D14 | A6-2a 設計書 §10 W1–W6 全照建議：「進行中」＝雙方間訂單狀態 10–40，以現有 RPC 組出、不改 Java，B8 換精確查詢（W1 (c)）；停權賣家的例外用同一定義（W2）；店家等待刪除仍可聊，只有關閉才擋（W3）；買家抹除等 `BuyerDeleted`（W4 (ii)，前提等待刪除的買家仍能登入，2a1 開工確認）；店家可主動開啟對話（W5）；2a 不支援卡片訊息，形狀在 A6-5 以伺服器參照定（W6 (b)）（Will 2026-11-20） | A6-2a1、A6-5 |

## 基線

2026-11-19，`762c28e6`，本機 unit 測試，五組都沒有失敗。

| 範圍 | 指令（在 `backend/`） | 結果 |
|---|---|---|
| 單體 | `GOWORK=off go test ./...` | exit 0，114 個套件 ok |
| gateway | `(cd gateway && go test ./...)` | exit 0，16 個套件 ok |
| review-service | `(cd services/review-service && go test ./...)` | exit 0，8 個套件 ok |
| im-service | `(cd services/im-service && go test ./...)` | exit 0，10 個套件 ok |
| crm-service | `(cd services/crm-service && go test ./...)` | exit 0，9 個套件 ok |

- 還沒跑：integration（要起容器，本機不跑，看 CI）、`make lint-monolith`、client 測試；各子步驟對當時 HEAD 重跑。

## 進度

| 步驟 | 狀態 | 已跑的驗收 | 未跑與原因 |
|---|---|---|---|
| A6-0 盤點與計畫 | 完成（2026-11-19） | 盤點抽查；壓力測試兩輪（第 1 輪 11 項、第 2 輪 4 項，全部改進，沒有駁回）；Will 拍板 D1–D8 | — |
| A6-1a review 設計 | 完成（2026-11-19） | 基線（上表）；開工 design-review 1 輪：2 輪，累計 findings 9 (blocking 0)，fixed 8、raised 2（A6-D9、第 9 項，Will 已決）；處理對照在設計書 §10 | — |
| A6-1b review 模組本體 | 完成（2026-11-20） | 本機 build、vet（含 integration tag）、`go test ./...`、`make lint-monolith`、`make sqlc` 無 drift；CI（PR #407）含 testcontainers integration 全綠；mutation：本機 RD2、RD9（submit 與 legacy client）、公開述詞守門、token 空白與重複、OrderForReview 欄位檢查、Hide/Restore utf8、A6-D10 皆 killed，CI 一次批次（run 44570770267）RD1、RD2、RD3、RD6、RD7、RD9、RD10、F2、F3、F6 共 13 個 mutant 皆 killed；審查：`/code-review` 1 輪＋diff 審查 1 輪，findings 12 (blocking 0)，fixed 10、recorded 1、raised 1（A6-D10，Will 已決）、rejected 0 | F2（拿掉 `UNIQUE(review_id, asset_id)`）的 kill 沒確認是斷言失敗還是 migration checksum 先錯；同一性質另有單元層「重複 token 被拒」的 mutation 證明。媒體同步的 worker 與定期清理已在本步寫好、未接 `cmd/hawkr`，A6-1c 只剩接線與驗證（diff 審查 should，recorded） |
| A6-1c 刪帳、媒體同步、reviewrelay | 完成（2026-11-20） | 本機 build、vet（含 integration tag）、`go test ./...`、`make lint-monolith`、`make sqlc` 無 drift；CI run 44649008948 四個 job 全綠（含 testcontainers integration）；mutation：CI run 44650114789 共 12 個 mutant 皆 killed（作者鎖兩側、Submit 交易內重查、排 purge、每批 `ReviewAggregateChanged`、purge 刪除、三處 `inactive_authors` 過濾、relay 送出 job 唯一、保底掃描兩個）；審查：`/code-review` 1 輪＋diff 審查 2 輪 findings 9 (blocking 0)，fixed 8、rejected 1；完成前 design-review findings 4 (blocking 0)，fixed 2（第 1 項 A6-D13 Will 決定）、recorded 1、rejected 1 | 鎖序 mutant 沒做（改動不是單一檢查）；評分讀取仍走舊 review-service（A6-D11，A6-6e 換）；social 的 purge 同樣缺保底掃描，未改 |
| A6-2a0 im 資料與送出的設計 | 完成（2026-11-20）：設計書 `step-2a-design.md`，W1–W6 已決（A6-D14） | 開工 design-review 2 輪：findings 9 (blocking 0)，fixed 7、raised 1（W6）、rejected 1；處理對照在設計書 §11 | 無程式、無測試（設計步驟） |
| A6-2a1 im 資料與送出的實作 | 完成（2026-11-20，PR #411）：im 模組 schema、送出守門、抹除（訂 `BuyerDeleted`、`StoreRetired`；W4 前提已核對：等待刪除的帳號仍可登入）、`imrelay`（未接線，A6-6e）、`orderservice.OpenOrderBetween`；順帶修 review purge 的 River 唯一性（缺 Running、掃描被已完成列擋住）與 `review.hidden` 的通知 allowlist；與設計書不同處記在設計書 §12 | 完成前 design-review 7 項 (blocking 0)：fixed 5、raised 1（`mediaref` 第 7 份，待 Will）、rejected 1（較舊訂單多擋，W1 (c) 已接受）；code-review 8 項 (important 2)：fixed 8；mutation §8 15 項＋修正輪 6 項 | 已跑：`go build`、`go vet -tags integration`、`go test ./...`、`make lint-monolith`、CI 全部 job（含 testcontainers integration）；未跑：無 |
| A6-2b0 即時通道的設計 | 完成（2026-11-20）：設計書 `step-2b-design.md`；A6-D15（Will 開工時決定）、W1–W5 已決（A6-D16） | 開工 design-review 2 輪：findings 11 (blocking 0)，fixed 10、recorded 1、raised 0、rejected 0；處理對照在設計書 §15 | 無程式、無測試（設計步驟） |
| A6-2b1 即時通道的實作 | 完成（2026-11-20，PR #412）：`im/internal/live`（LISTEN 連線、扇出、訂閱、presence）、寫入端 `NOTIFY`、寫入 transaction 上限（A6-D17）；未接 `cmd/hawkr`（A6-5）；與設計書不同處記在設計書 §16 | 本機 `go build`、`go vet -tags integration`、`go test ./...`、`go test -race`（im）、`make lint-monolith`、`make sqlc` 無 drift；CI run 44750191321 im 全部 integration 通過；mutation：本機單元 15 個 killed，CI 批次 run 44743745834（22 killed）＋44749188726（補強後 M12、寫入上限 5 路徑、M15 共 6 個 killed）；審查：`/code-review` 1 輪＋完成前 design-review 1 輪＋diff 審查 1 輪（第 2 輪），findings 16 (blocking 0)，fixed 14、recorded 2、raised 1（L2，A6-D17 已決後修）、rejected 1 | M7、M20 的 kill 是逾時或 cleanup 卡住，不是乾淨的斷言失敗；M4、M13、M22 integration 版沒跑（M4 無單點改法、M13 不存在、M22 單元已覆蓋）；補洞時間界線不含 COMMIT 本身耗時（只靠 2 秒時鐘誤差，未量測）；巢狀於外層 transaction 的 im 寫入不設上限（目前無呼叫端，觸發條件在 CARRYOVER）；同一 CI run 的 seller `TestSellerInquiryListIsNewestFirstFilteredAndCounted` 失敗是 dev 既有（run 44731436154），不屬本步 |
| A6-3a crm 設計 | 設計書與開工 design-review 完成，完成（2026-11-20）：W1–W5 已決（A6-D18），§10 計畫修改已套用 | 開工 design-review 2 輪：findings 11 (blocking 0)，fixed 10、recorded 0、raised 0、rejected 1；處理對照在設計書 §14；兩處計畫與事實衝突已核對原始碼（`RefundCompletedPayload.java:59` 每次新 UUID、`membership.go:90` VIP 唯一呼叫端） | 無程式、無測試（設計步驟）；— |
| A6-3b crm 實作 | 完成（2026-11-21）：`internal/modules/crm`、`internal/legacy/crmconsumer`（只在測試啟用）；與設計書的差異見 `step-3-design.md` §15；§16 #1 依 A6-D19 交 A6-3c 處理 | go build／vet、unit、`make sqlc` 無 drift（主 session 重跑）；完成前 design-review＋/code-review＋第 2 輪：findings 14 (blocking 0)，fixed 9、recorded 0、raised 1、rejected 4（設計書 §16）；CI `backend monolith` run 44850261836（5ad73cbe）三個 job 全綠，含 integration test（testcontainers） | 36 個 mutation 只在實作者的原生 Postgres 跑過（CI 不跑 mutation） |
| A6-3c0 擁有者守門設計 | 完成（2026-11-21）：設計書 `step-3c-design.md`，W1–W7 已決（A6-D20） | 開工 design-review 1 輪：findings 6 (blocking 0)，fixed 1、recorded 3、raised 2（W4、W6）、rejected 0；Postgres 列鎖衝突表以官方 explicit-locking 頁核對 | 第 2 輪未派：拍板改的是 W4、W6 的選項，設計書已有 (b) 的完整描述，實作規定寫在 §10 末；`SKIP LOCKED` 語意與 pub 函式內聯待 3c1 的 integration 證明 |
| A6-3c1 擁有者守門實作 | 完成（2026-11-21）：buyer `LockNotClosed`／`pub_closed_buyers`、seller `LockNotRetired`／`pub_retired_stores`；W3、W4 鎖強度；`platform/jobs` 的 `UniqueByArgs`／`SweepOpts` 收齊 14 處；review、im、crm 刪 `inactive_*` 與 advisory lock | CI run 44862272441（87b0a3da）全部 job 綠，含 integration（`SKIP LOCKED` 跳過、pub 函式內聯 EXPLAIN 在真實 Postgres 通過）；mutation run 44863292682：19 個全 KILLED（§8.3 M1–M13、W4 兩個、修正輪 F4／F6／F8 各一、im relay join 一個；M1 以測試逾時被殺）；審查 2 輪：findings 13 (blocking 1)，fixed 10、recorded 1（seller ScheduleExpiry 唯一性狀態，`seller/CARRYOVER.md`）、raised 1（crm 守門 NotFound 一律記 error log，見 W1；Will 2026-11-21 決定 (a′)，已在後續 commit 實作）、rejected 1（buyer 孤兒註解不存在） | `TestSubmitIsRejectedAfterDeletionRequestAndWaitsForTheGuard` 在守門被拿掉時以逾時而非斷言失敗 |
| A6-4a 設計 | 完成（2026-11-21）：設計書 `step-4-design.md`，W1–W4 已決（A6-D21） | 開工 design-review 1 輪：findings 7 (blocking 2)，fixed 5、recorded 0、raised 2（W2／W3，Will 已決）、rejected 0 | 第 2 輪未派：改動只有 `MessageForReport` 加入呼叫方 tx，由規則 3 與先例 `seller/governance.go:600` 定案；無程式、無測試（設計步驟） |
| A6-4b 評價與聊天對象、快照、ReverseHide | 完成（2026-11-21，PR #420）：`TargetKind` 加 REVIEW、CHAT_MESSAGE；`review.ReviewForReport`、`im.MessageForReport`；快照依種類分開、`GetCase` 帶 snapshot oneof；HIDE 經 review／im 同 tx；`ReverseHide`（案件維持 CLOSED、`reversed_*`＋adminaudit 含 `restored`，只還原本案件 `source_ref` 的隱藏，內容已清除仍記錄）與權限 `moderation.cases.reverse`；admin 對象選單加兩值。與設計書不同處：還原依 `source_ref`（改寫 §2）、聊天 HIDE 測試直接建案件（店家作者 HIDE 待 4c） | 本機 build、vet（含 integration tag）、`go test ./...`、`make lint-monolith`、sqlc／connect／uitext 無 drift（主 session 重跑）；CI PR #420 在 585fc2a4 全部 job 綠，含 integration；mutation CI run 44875013657（17／19 killed）＋44875872312、44877005095 補測試後 #18、#19 killed，共 19 個全 KILLED；審查：`/code-review` 1 輪 findings 7 (blocking 0)：fixed 5、recorded 1（F4 聊天前後文含檢舉人訊息的保留，觸發：上線前隱私審查）、rejected 1（F3 舊扁平快照，未上線不相容）；diff 審查第 2 輪 findings 5 (blocking 0)：fixed 5 | `NOTIFY` 次數未斷言（im 自身測試涵蓋）；抹除前後文的墓碑只間接覆蓋；完成前 design-review 依計畫在 4c 涵蓋 4b＋4c |
| A6-4c 店家內容下架與店家檢舉人 | 完成（2026-11-21，PR #421）：`contentkind` 加 CHAT_MESSAGE、REVIEW_REPLY；seller `measure_contents` CHECK 與快照連結規則；`pub_content_takedowns()` 只回 ENFORCED；im 讀取與 review 回覆遮蔽 join 它，`ReplyToReview` 在下架生效時拒絕；im／review 的 `OnMeasureCreated`／`OnMeasureRevoked` 核對歸屬；moderation TAKEDOWN 用於 CHAT_MESSAGE（店家作者）與新對象 REVIEW_REPLY，同 tx 建 seller 措施；`reports`／`report_requests`／`reporter_daily_counts` 加 `reporter_store_id` 與恰好一欄 CHECK，店家只能檢舉 CHAT_MESSAGE、額度各 20；admin `CaseReport` 檢舉人 oneof、處置選單對齊 `AllowedOn`。Will 2026-11-21 決定見 `step-4c-design.md` §6 | 本機 build、vet（含 integration tag）、`go test ./internal/... ./cmd/...`、`make lint-monolith`、各生成無 drift（主 session 重跑）；CI PR #421 在 567257bd 全部 job 綠，含 integration 與 lockorder 併發測試（鎖序 seller → im/review → moderation 已證明）；mutation CI run 44909500419＋44910476574：12 個（#4 拆 schema 與 Go 兩個）全 KILLED；審查：design-review（完成前，涵蓋 4b＋4c）findings 4 (blocking 0)；`/code-review` findings 9 (blocking 0)；合併去重 13：fixed 11、recorded 2（店家退場不清店家檢舉人資料，觸發 A6-5 開工前由 Will 決定；檢舉人 exclusive arc 取捨）、raised 0、rejected 0；第 2 輪 diff 審查 NO DEFECTS FOUND | EXPLAIN 未跑（`measure_contents_one_enforced` 的使用是推論）；admin `pnpm build` 未在本機跑（CI admin job 綠）；舊微服務檢舉路徑未逐項對照（design-review 標為覆蓋缺口，舊服務在 A6-7 刪除） |
| A6-4d 店家退場清店家檢舉人資料 | 完成（2026-11-21，PR #423）：`Retention.OnStoreRetired`（同 transaction 清店家檢舉人的說明、冪等紀錄與每日計數列，檢舉列留著）、`reports` 店家檢舉人 partial index、送出先取檢舉人擁有者守門（`seller.LockNotRetired`／`buyer.LockNotClosed`）、檢舉人專用的找不到文字、`cmd/hawkr` 接線與 AST 測試 | 本機 build、vet（含 integration tag）、unit、`make lint-monolith`、`make sqlc`；CI PR #423 全部 job 通過（含 integration）；mutation CI run 44920513102 共 7 個全 KILLED（店家守門、買家守門、兩處接線、三條清除 SQL）；審查：`/code-review` 2 輪，findings 11 (blocking 0)，fixed 11 | 送出與退場同時發生的競態沒有測試（只測退場後再送）；index 無行為 mutation；`TestTakedownAndAccountDeletionRunConcurrentlyWithoutDeadlock` 的刪帳端用不存在的帳號，刪帳回 NotFound 就通過，沒走到真正的刪除流程【推，既有，未改】 |
| A6-5b ReviewService 與評價檢舉 | 完成（2026-11-22，PR #425）：`api/hawkr/review/v1alpha1`（`ReviewPublicService` 3 支、`ReviewAppService` 5 支，§1 的 5b 列全做）、`ErrorReason` 15–18、`httpapi/review`；`ListOwnStoreReviews` 由 `seller.CurrentStore` 推導（A6-D23 W1 (g)）；`ReplyReview` 由評價所屬店推導，以不套公開述詞的 `review.ReviewStore` 取店（店主遇隱藏得 `REVIEW_NOT_REPLYABLE`，非店主遇不公開 not_found、公開 permission_denied，Will 2026-11-22 選 (a)）；`ListMyReviews` 加 `order_item_id` 篩選；`BlockAuthor`／`BlockBuyer` 加 `review_id`，`httpapi/blocks` 依參照種類問 social、review | 本機 build、vet（含 integration tag）、`go test -race ./...`、`make lint-monolith`、`buf lint`、`make sqlc`／`make connect` 無 drift；CI PR #425 全部 job 通過（integration run 44986178830，第一次因 testcontainers Reaper 失敗重跑）；mutation CI run 44986178958 共 15 個全 KILLED；審查：`/code-review` 1 輪＋diff 審查 2 輪，findings 13 (blocking 3)，fixed 13（D1–D3 由第 2 輪抓到，Will 決定 D1 方向後修） | 經 web-host 反代的實網測試不屬本步；client 改走新 API 在 A6-6 |
| A6-5c ChatService 與聊天檢舉 | 完成（2026-11-22，PR #427，merge `e45a32a9`）：`httpapi.Deps` 字面值 AST 守門測試；`im.Send` 以 `request_id` 冪等（`UNIQUE (conversation_id, sender_kind, request_id)`）；`Presence` 每次呼叫固定 2 次往返（新增 `seller.BuyersBlockedByStore`）；`api/hawkr/chat/v1alpha1`（`ChatAppService` 15 支 unary＋2 支串流，買家版與店家版分開，單一對話上的方法由 `im.ConversationParties` 推導身分）、`ErrorReason` 19–22、預覽文案進共用目錄；串流 helper（心跳、壽命、寫入期限、重驗）與 `im.Module.Run` 接進 serve；`httpapi.Deps` 必填加 `IM`、`Identity`；封鎖入口加 `message_id`；`SubmitReport` 的聊天檢舉人由訊息歸屬推導、`ListMyStoreReports` | 本機 build、vet（含 integration tag）、`go test -race ./...`、gofmt、`make lint-monolith` 0 issues、`buf lint`、生成無 drift、TS／Dart 套件檢查、mutation 清單 48 筆格式檢查（主 session 重跑 Go 與 lint，其餘為實作者回報）；審查：`/code-review` 1 輪＋diff 審查 2 輪，第 1 輪 findings 11 (blocking 0) fixed 11，第 2 輪 findings 4 (blocking 1) open 4，第 3 輪修正經 Will 2026-11-22 同意，只改測試與 mutation 清單：D1–D4 與串流店家身分 mutant 逾時 fixed 5；CI（head `2cdd53ea`）：backend monolith 45030840950、backend db proto checks 45030840765、mutation 45030840913 全綠，mutation (go)、mutation (go-integration) 無 SURVIVED／TIMEOUT | 經 web-host 反代與 Cloudflare 的串流實測（不緩衝、閒置超過 60 秒不被切）是需要人做的事 |
