# A8 通知：對照業界基線的設計取捨簡報

核對基準：dev `d6e324b5f`（2026-11-23）。證據標記：【量】實際讀碼或讀文件；【推】由已讀內容推得；【猜】未驗證。外部文件連結是依公開文件整理，**本次沒有逐一 fetch 驗證**，一律標【推】。
路徑縮寫：`PLAN` = `backend/docs/plans/2026-11-23-a8-notification.md`；`INV` = 同名目錄 `inventory.md`；`MASTER` = `backend/docs/plans/2026-09-24-go-modular-monolith.md`；`NADR` = `docs/adr/2026-10-05-notification-module.md`。

---

## 0. 結論先講

1. **A8-5（替還沒搬的 Java、invoice-service、ai-service 寫 `internal/legacy` 逐一 RabbitMQ consumer）應整步刪除。** 它要訂閱的發送端全部已排進後面自己的步驟，`internal/legacy` 在階段 C 整包刪除，重工比例 100%；其中至少約 10 組 key 的發送機制在後續步驟本來就會被刪掉，連「換成 in-process」的機會都沒有。排程分類是 SELF-IMPOSED。改成：**通知型別跟著發送端搬**——每個 B 步驟（與 A5、A9）在「模組完成的定義」加一項「本模組的通知型別與 in-process 訂閱」。
2. 連帶：D3（事件裁決）縮小成只裁決單體事件；D5（7-ELEVEN 取貨簡訊）的前提在 B3 消失，從 A8 移除。
3. 草案的大方向（型別寫在程式碼、帳號＋受眾定址、投遞紀錄＋冪等鍵、偏好＝分類×渠道、自建）和業界基線一致，保留。需要改的是幾個「沿用沒重評」的細節：fan-out 放在來源 transaction 內、串流傳資料而不是傳失效訊號、proto 以「型別」為 enum、token 沒有最後活躍時間、`product.viewed` 這類型別照搬、偏好把 follower/post 歸成行銷。

---

## 1. 前提問題：本階段要寫的程式，哪些已排定會被重寫、搬移或刪除？

### 1(a) A8-5 的 legacy consumer

「legacy consumer」：單體裡一段接 RabbitMQ 訊息、解碼成 Go 型別再交給通知模組的轉接程式，放在 `internal/legacy`。

**量化**（來源 INV §5.4 的 94 列模板，`INV:37-130`，用 awk 依 (exchange, routing key) 去重【量】）：

| 發送端 | 去重後的 key 數 | 模板列 | 主計畫裡搬它的步驟 | 搬遷時的處置 |
|---|---|---|---|---|
| aftersales（Java） | 24 | 37 | B9（`MASTER` B9 段） | B9 直接刪退貨物流單、超商取件逾時等 consumer 與排程，退貨改買家自寄【量：MASTER B9「不改寫、直接刪」】→ `return_at_store`、`return_at_store_24h`、`auto_completed_store_pickup_timeout` 等至少 3 組 key 的發送機制消失【推】 |
| invoice-service（Go） | 12 | 13 | A5，A5 等 B8（`MASTER:256`） | 搬進單體後改 in-process |
| order（Java） | 4 | 7 | B8 | 同上；`payout.completed` 移到 B10 finance【推：MASTER B10「賣家撥款從 ordering 移入」】 |
| shipping（Java） | 4 | 5 | B3 | B3 改賣家自寄，`ShipmentAnomalyService`、PayUni／Tcat 建單全部直接刪【量：MASTER B3】→ `shipment.anomaly.detected`、`shipment.created`（現在是 PayUni 建單完成）語意消失【推】 |
| product（Java，認證） | 3 | 3 | B4 | B4 明寫「通知模板 `product_certification_*` 契約上一起消失」【量：MASTER B4 延後項】→ 3 組全部丟 |
| coupon（Java） | 2 | 2 | B7 | `coupon.issued` 發布端是死碼（`INV:20`）|
| payment（Java） | 1 | 1 | B2／B10 | `payment.rescue.refund_completed` 是 PayUni 救援退款，B2 拿掉 PayUni【推】|
| ai-service | 2 | 2 | A9（**進行中**，`backend/docs/plans/2026-11-19-a9-agent.md`） | A9-D1 刪站外業者路線（`a9-agent.md:111`）→ `external_created.vendor` 丟；另一個 key 在 A9 期間就變成單體 `procurement` 模組的事件【推】|
| **合計** | **52 / 73 組（71%）** | **64 / 94 列（68%）** | 全部在 B1–B10、A5、A9 | 階段 C「刪除 `internal/legacy/`」（`MASTER` 階段 C 第 2 項）【量】|

- 寫出來的 consumer：52 個 key 各一個 consumer＋wire 解碼測試（以 `tests/contract/golden/event_payload_keys.json` 為 golden）＋topology 測試【量：PLAN:109-111】。**最終保留 0 個**：每個發送端搬進單體時，consumer 由 in-process 訂閱取代，`internal/legacy` 在 C 整包刪除。重工比例 100%【推】。
- 更糟的部分：型別的 typed 參數是依照 Java 現在的 wire 欄位設計的；B 步驟會重新設計這些事件（例如 B3 出貨狀態機重寫、B9 退款收斂成一張命令表），參數 struct 也會被重寫。至少 3（認證）＋1（物流異常）＋1（PayUni 救援退款）＋1（站外業者）＋約 3（超商退貨）≈ 9–10 組 key 的發送機制會被刪掉，寫了等於白寫【推】。
- 估計規模：A8-5 加上 D3 對這 52 組的裁決、U4／U5（golden 與發送端不一致）的查證，約占 A8 工作量的 1/4【猜】。

**排程分類**：

| 排程 | 宣稱的理由 | 分類 | 根據 |
|---|---|---|---|
| A8 要替 Java 事件保持通知可用 | 「還沒搬的服務仍經 RabbitMQ 發事件，wire 在邊界外」（`PLAN:29`、`NADR:30`） | **SELF-IMPOSED** | Java、invoice-service、ai-service 都是我們自己的程式，排在自己的 B／A5／A9 步驟搬；「之後自己的步驟會搬」不是無法滿足的前提。系統從未上線（根目錄 `AGENTS.md`「上線前不受相容性約束」），A8 到 B9 之間訂單類通知暫時沒有，沒有任何使用者受影響【量：`PLAN:24` 已確認不在線】 |
| 食安事件通知購買者延到 B8 | 「查得到購買者」的前提在 B8 | **REAL** | 現在沒有單體的訂單資料可查（`PLAN:68`）|
| 推播與 email 的真實 provider 驗收 | 需要 Will 開 Firebase、寄信帳號與 DNS | **REAL** | 外部帳號只有 Will 能開（`PLAN:27`）|

**建議**：刪除 A8-5。改成在 `MASTER`「模組完成的定義」加一項：「本模組產生的、使用者需要知道的事件，在 notification 模組定義型別並以 in-process 訂閱；搬遷時 `INV` §5.4 對應列的去向欄填好」。各 B 步驟、A5、A9 自己承接。過渡期（A8-8 到各步驟之間）Java 事件不產生通知——明寫進 PLAN 前提，由「不在線」這條事實擔保。
- 推翻這個建議需要的事實：A8 到 B9 之間有真實使用者或公開 demo 需要訂單通知。之後反轉的代價：低，屆時寫 legacy consumer 的成本和今天一樣。
- 另一個選項「讓舊 notification-service 繼續跑、只服務 Java 事件」：不採。舊服務自己有 device token 表與收件匣（`INV` §2），會和新模組形成兩個 token 來源與兩個收件匣，正是 NADR 要消除的東西【推】。

### 1(b) gateway REST 與舊服務在 A8 內刪除

- gateway 的 `/app/notifications*`、偏好、後台通知路由、`openapi.yaml` operation、notification-service 都在 A8-8 刪（`PLAN:125`）。A8 不在 gateway 或舊服務寫任何新程式（A8-2「舊 REST 在 A8-8 前仍可用」只是不動）【量：PLAN:92】。**重工 0**，這是正確的退場，不是重工。
- 要被丟掉、但草案仍要動的：A8-4 刪 `socialrelay`、`imrelay`、`reviewrelay` 的 notification 一半等 7 個 relay 套件（`INV` §8.2）——都是刪除，沒有新寫。✓
- 唯一的例外：A8-5 驗收要求「topology 測試：沒有 `#` 綁定」（`PLAN:111`），這是為 RabbitMQ topology 新寫的測試，階段 C 刪 `infra/mq/` 時一起丟【量：MASTER 階段 C 第 5 項】。隨 A8-5 刪除。

---

## 2. 逐題分析

每題格式：業界基線 → 既有機制為什麼存在 → 選項與失敗條件 → 推翻條件與反轉代價 → 限制清單 → 建議。

### Q0 自建 vs 通知基礎設施（Novu、Knock、Courier）——草案漏題

「通知基礎設施」：把通知的工作流、偏好、收件匣、渠道路由做成產品的 SaaS 或開源服務，應用只呼叫 `trigger(workflow, recipient, payload)`。

- 業界基線：Knock（[docs.knock.app](https://docs.knock.app)：workflow、batch、preferences、in-app feed）、Novu（[docs.novu.co](https://docs.novu.co)，開源可自架，依賴 MongoDB、Redis 與多個 Node 服務）、Courier（[courier.com/docs](https://www.courier.com/docs)）。中小團隊、多渠道、需要非工程人員改文案時常用 SaaS；GitHub、Discourse、Shopify 這類規模的產品自建【推】。
- 既有機制為什麼存在：notification-service 是公司時期自建的微服務，原因是當時的微服務架構【推】。
- 選項：
  - A 自建在單體模組（草案預設）。失敗條件：需要非工程人員編輯文案、或要 10 種以上渠道。
  - B Knock／Courier SaaS。失敗條件：要離線開發與 CI（替身要另寫）、要把通知資料留在自己的 DB 做個資清除（NADR 決策 5 的清除鍵）、免費額度。
  - C 自架 Novu。失敗條件：單一 binary 部署（多出 MongoDB、Redis、Node 服務）。
- 推翻條件：出現營運團隊要改文案，或渠道擴到 SMS、LINE 多家。反轉代價：中；型別寫在程式碼，渠道是 adapter，日後換成 trigger SaaS 只換「投遞」那一層。
- 限制：
  - 單一 binary、單體部署：**REAL**（ADR `docs/adr/2026-09-24-go-modular-monolith.md`，已接受的架構決策）。
  - 沒有營運團隊：**REAL**（NADR:32）。
  - 個資清除要在自己的 DB 執行：**REAL**（NADR 決策 5）。
- **建議：A 自建**，並在 ADR 補這一題（草案與 NADR 都沒有寫「為什麼不用 SaaS」）。

### Q0b 工作流引擎 vs River job——草案漏題

「工作流引擎」：Temporal 之類把多步驟流程（等待、分支、重試）持久化的執行引擎。「River」：專案已用的 Postgres job 佇列（`backend/go.mod:15` `river v0.51.0`【量】）。

- 業界基線：Discourse 用 Sidekiq job（`Jobs::PostAlert`、`Jobs::UserEmail`）【推】；Knock、Novu 內部是 workflow＋step 佇列【推】；Temporal 用在跨服務長流程。
- 選項：A River job（草案）；B Temporal。B 失敗條件：多一個要部署的叢集，通知沒有跨天的多步驟補償。
- 延遲、摘要、提醒都能用 River 的 `ScheduledAt` 與 unique job 表達【推：River 文件 Scheduled jobs、Unique jobs】。
- 限制：單一 binary（REAL，同上）。
- **建議：A River**，與草案相同。

### Q1 投遞模型：fan-out 放在哪、投遞紀錄、outbox（草案 A8-1／A8-3、不變量第 1、9 列）

「outbox」：在業務 transaction 內寫一筆「待送」紀錄，transaction 提交後再由背景程式送出，保證「業務寫了就一定會送」。「fan-out」：一個事件展開成多個收件人、多個渠道。

- 業界基線：transactional outbox（[microservices.io: Transactional outbox](https://microservices.io/patterns/data/transactional-outbox.html)）；River 的 `InsertTx` 本身就是 outbox：job 列和業務列在同一個 Postgres transaction【推：River 文件 Transactional enqueueing】。Discourse 在建立貼文後排一個 `PostAlert` job，在 job 裡解析收件人與寫通知列，不在請求的 transaction 內展開【推】。Knock、Novu 也是 trigger 後非同步展開【推】。
- 既有機制：單體的 `events.Publish` 在 transaction 內同步呼叫訂閱者（`internal/platform/events/bus.go:23,71`【量】）；backend 規則 4 允許訂閱者「`Enqueue` job／寫 inbox」，並把 notification 的列放在全域鎖順序最後（`backend/AGENTS.md` 單體規則 4【量】）。
- 草案：同步訂閱者在來源 transaction 內「建立通知」：冪等、偏好、自我略過、封鎖略過（`PLAN:83`），然後排收件匣、推播、email job；為此要新增 lockorder 測試，每加一個來源模組加一個案例（`PLAN:47`）。
- 選項：
  - A（草案）來源 tx 內做收件人解析＋偏好＋封鎖判斷＋排渠道 job。失敗條件：fan-out 大的型別（`post.created` 通知所有追蹤者、公告）在來源 tx 內展開，tx 變長；每個來源 tx 都要讀 notification、seller（店主解析）、social（封鎖）的資料，鎖順序與依賴面變大。
  - B 來源 tx 只 `InsertTx` 一個「通知意圖」job（型別＋參數快照＋冪等鍵＝事件來源身分）；worker 在自己的 tx 內解析收件人、套偏好與略過規則、寫收件匣列與每渠道投遞紀錄、排渠道 job。失敗條件：收件匣出現有秒級延遲（River 的輪詢間隔）；偏好在事件發生後、worker 執行前被改的話，用的是較新的偏好（業界都這樣，可接受）。
- 推翻條件：量測到 worker 延遲讓收件匣明顯落後（例如 > 2 秒），且必須和業務寫入同時可見。反轉代價：低，入口函式同一個，只改呼叫時機。
- 限制：
  - 「同 tx 訂閱者只做原子狀態變更或排 job」：**REAL**（backend 規則 4，由測試強制）——B 完全符合，A 也符合但用得最大。
  - 「必須在來源 tx 內決定收件人」：**SELF-IMPOSED**，沒有來源；可靠性只需要「意圖」在 tx 內落地。
- **建議：B**。好處：來源模組的 tx 只多一列 River job；lockorder 測試從「每個來源模組一個案例」縮成「訂閱者只呼叫 `jobs.Enqueue`」一條結構性測試；封鎖、店主解析都在 worker 經對方公開 API 讀。投遞紀錄保留：River 只保留完成的 job 24 小時（`MASTER` A8 段第 7 條引用 River 原始碼【量】），不能當「每筆通知是否觸達」的帳本，而 NADR 待辦的未觸達率指標與重送 runbook 都需要它。重送＝依投遞紀錄重排 job，紀錄是唯一來源。
- `ReportReceived.ConfirmationEmail` 的地址：在 B 之下，意圖 job 的參數**只放 report 參照**，worker 執行時向 seller 讀（MASTER A8 段第 7 條的第二個選項），比「完成後清參數」少一個要測的清理路徑。

### Q2（D3）事件裁決

- 草案：A8-0 產出 73 組 key＋event-anomalies 的裁決表，一次拍板（`PLAN:158`）。
- 依第 1 節，Java、invoice、ai-service 的 52 組不在 A8 裁決，交給各搬遷步驟。A8 只裁決單體現在送出的 22 個 route（`INV` §8.1）與預留的 12 項事件（`INV` §8.3）。
- 草案的原則「只訂閱使用者需要知道且有發送端的事件；沒有發送端不寫型別」與 GitHub、Discourse 的做法一致（型別隨功能出現）【推】。✓
- 補一條：`product.viewed`（賣家每次被瀏覽就收一則通知，`INV:84`）與 `buyer.favorite.added`：業界電商（Etsy、Shopee 賣家端）以統計頁呈現瀏覽數，不逐次通知【猜：依公開產品行為，未查文件】。這是照搬。建議 D3 直接判「不做型別」，連帶 `buyer/CARRYOVER.md:65` 的節流延後項消失。
- 限制：「Java 事件要在 A8 裁決」——SELF-IMPOSED（同 1(a)）。
- **建議**：D3 縮成單體事件的裁決表；`product.viewed` 不做型別；收藏改成可選的聚合型別或不做（交 Will）。

### Q3（D1）文案放哪

- 業界基線：伺服器渲染的通知文案放在伺服器的 i18n 目錄（Discourse `server.en.yml` 的 `notifications` 段、GitHub 伺服器端模板）【推】。
- 選項 A 共用文案目錄：失敗條件是三端生成用不到的函式，且違反多語系 ADR 的「目錄分界」（只放由契約值決定的文字）【量：`docs/adr/2026-10-17-multi-locale-architecture.md` 目錄分界修訂，經根目錄 AGENTS.md 共用文案目錄列轉述】。選項 B 模組 `internal/text`：失敗條件是 client 需要自己組通知句子（Q5 選伺服器組字後不成立）。
- 限制：目錄分界——REAL（已接受的 ADR）。
- **建議：B**，與草案相同。

### Q4（D2）收件匣保留期

- 業界基線：GitHub 通知未儲存者約 5 個月後刪除（[GitHub Docs: Managing notifications from your inbox](https://docs.github.com/en/account-and-profile/managing-subscriptions-and-notifications-on-github/viewing-and-triaging-notifications/managing-notifications-from-your-inbox)）【推】；Facebook、Discourse 也會修剪舊通知【猜】。
- 既有機制：舊服務永久保留（缺陷 #16）。
- 選項：90／180／365 天。90 失敗條件：退貨、爭議、撥款跨季。365 失敗條件：個資保留期拉長、渲染程式要保留更久（NADR 後果第 3 條）。
- 限制：個資最小保留——REAL（個資法）；保留期也決定舊型別渲染程式的保留成本——REAL（NADR 決策 5）。
- **建議：180 天**，與草案相同；另把「已讀＋超過 N 天」不另設，避免第二條規則。反轉代價低（一個常數＋清除 job）。

### Q5（D6）收件匣回應的形狀＋proto 的型別 enum——草案框得太窄

- 業界基線：GitHub 通知 API 回 `reason`、`subject.type`（Issue、PullRequest…，少量穩定值）與 URL，文字由 subject 本身提供【推：GitHub REST「List notifications」】；Discourse 回 `notification_type`（整數 enum，數十個）＋`data`，client 渲染【推】。
- 草案：伺服器組好標題、內文、深連結，「加型別 enum 讓 client 選圖示」（`PLAN:168`）。
- 問題：型別 enum 是「每個通知型別一個 proto enum 值」，依第 1 節，B 步驟會陸續加型別，每次都要改 proto、三端重新生成；client 唯一用途是選圖示。這是把舊模板的 `data.type`（`INV` §5.4 的 data.type 欄，約 50 個值【量】）照搬成 enum。
- 選項：A 型別 enum（草案）；B 只回「呈現分類」enum（例如 ORDER、SHIPMENT、REFUND、CHAT、SOCIAL、ACCOUNT、ANNOUNCEMENT，約 8–10 個，和偏好分類對齊）＋深連結＋伺服器組好的文字。B 失敗條件：client 需要依型別做不同互動（不只圖示）。A8-0 的 U8（client 對 `type`／`refType`／`data.*` 的分支）就是驗證這個條件的事實。
- 限制：「同一個 enum 的對照表不在兩個 client 各寫」——REAL（根目錄 AGENTS.md 共用文案目錄列）；B 也符合（圖示對照表很小，可放共用 design token）。
- **建議：伺服器組字（同草案）＋B 的呈現分類 enum**；U8 若發現 client 依型別做互動，才回到 A。

### Q6 收件匣模型——草案沒有單獨成題

- 業界基線：Discourse `notifications` 表（user_id、notification_type、data JSON、read、high_priority）【推】；GitHub 以 thread 為單位，同一 subject 的後續事件更新同一列（`updated_at`、`reason`），不新增列【推】；Facebook「X 和其他 3 人說讚」以聚合鍵合併【推】。
- 草案：存型別＋參數快照，讀時組字（NADR 決策 5）。✓ 但沒有「聚合鍵」。
- 選項：A 每個事件一列（草案）；B 型別可宣告 `group_key`（例如「某則貼文的讚」），同鍵未讀時更新同一列並累加 actor。B 失敗條件：多一個 upsert 路徑與「actor 清單」欄位。
- 推翻條件：沒有任何互動類型別（讚、追蹤、留言）。反轉代價：上線前低（不受相容性約束）；上線後要資料遷移。
- 限制：「節流聚合延到有 1000 追蹤者的店」（`PLAN:71`）——SELF-IMPOSED：觸發條件是洗版發生後才修，但資料模型的選擇現在就在做。
- **建議：型別註冊表現在就支援可選的 `group_key`，只給讚、追蹤這兩種用**；不做跨型別摘要。

### Q7（D4）偏好模型

- 業界基線：Slack、GitHub、Knock 都是「分類（或 workflow）×渠道」矩陣，只存使用者覆寫值，預設寫在程式碼【推：Knock Preferences 文件、GitHub Notification settings】；交易與安全通知不可關（GitHub 安全通知、Amazon 訂單信）。
- 草案分三群（`PLAN:159-163`）。問題：
  1. 把 follower、post（追蹤我的店有人追蹤、追蹤的人發文）歸「行銷性質」。這兩類是社群互動，不是平台利用個資做行銷；個資法第 20 條的 opt-out 只約束行銷。歸錯會讓「行銷」群混入互動通知，日後加行銷 email 同意紀錄時邊界不清【推】。
  2. 沒寫儲存模型：存完整矩陣或只存覆寫。
- 建議：分類＝互動（chat、review、social：留言、讚、追蹤、追蹤的人發文）、交易必要、行銷（coupon、restock）；表只存 `(account_id, 受眾, 分類, 渠道, enabled)` 的覆寫列，預設與「不可關」在型別註冊表。限制：個資法第 20 條——REAL，只適用行銷群。

### Q8 即時串流（D6 之外、A8-2）

- 業界基線：GitHub 通知 API 是輪詢，伺服器以 `X-Poll-Interval` 控頻【推】；Discourse 用 MessageBus（長輪詢，訊息帶遞增 ID 可續傳）傳「有新通知」與未讀數，client 再抓列表【推】；Slack 用 WebSocket 傳事件。
- 草案：Connect server streaming，`(account_id, 受眾)` 訂閱、續傳游標、每帳號串流上限（`PLAN:90`）；串流傳通知本體並要求「斷線以游標續傳不漏」（`PLAN:93`）。
- 問題：收件匣本來就是 DB 裡的持久資料，「不漏」由收件匣列表保證；讓串流也保證不漏，等於在串流上重做一次佇列語意。A9 採的記憶體環形緩衝（`a9-agent.md:122`）是因為助理事件不落表，通知沒有這個前提。
- 選項：
  - A 串流傳通知本體＋游標續傳（草案）。失敗條件：要維護每帳號的緩衝或回查 DB 的續傳邏輯。
  - B 串流只傳「失效訊號」（未讀數＋最新收件匣 ID），client 收到後呼叫 unary `ListNotifications`；重連後先打一次列表即可，不需要續傳游標。失敗條件：需要在通知抵達時立即顯示內容卡片（多一次請求，延遲 < 100 ms，可接受）。
  - C 不做串流，輪詢未讀數。失敗條件：網頁需要即時（聊天已有自己的通道）。
- 限制：單體是單一進程——REAL（A9 計畫不變量，`docs/adr/2026-11-19-ai-service-monolith-scope.md`【量：rg 命中】），所以 hub 用記憶體即可，不需要 Postgres LISTEN/NOTIFY 或 Redis。
- **建議：B**。缺陷 #19（跨身分）由訂閱鍵 `(account_id, 受眾)` 修，#20（best-effort）由「訊號＋列表」本身消除。U10（seller workspace 通道是否共用 hub）仍在 A8-0 查。

### Q9 email 寄件與 suppression（D8、A8-3）

「suppression list」：退信或投訴過的地址名單，名單內不再寄。

- 業界基線：Postmark、SES、Resend 都在 provider 端自動維護 suppression（Postmark Suppressions、SES account-level suppression list）【推】；Gmail／Yahoo 2024 寄件者規範要求 SPF、DKIM、DMARC，行銷信要 RFC 8058 一鍵退訂（[Google: Email sender guidelines](https://support.google.com/a/answer/81126)）【推】；交易信與行銷信分 stream 或子網域（Postmark message streams）【推】。
- 草案：adapter 只回暫時／永久錯誤、webhook 寫本地 suppression list（`PLAN:96`）。
- 本地 suppression 的必要條件：換 provider 時名單不遺失，以及「不排 job」比「排了被 provider 擋」少一筆投遞紀錄。條件成立但弱；可接受。
- D8 provider：量極小（作品集），選擇看免費額度與 webhook。Postmark（送達率口碑）與 Resend 都可行；adapter 介面不受影響，反轉代價低【推】。
- 限制：不能用舊公司 Resend 帳號——REAL（NADR:27）；沒有 Apple 登入——REAL（根目錄 AGENTS.md），NADR 的 Apple 私密轉寄網域登記刪除（草案已處理 ✓）。
- **建議**：同草案；補「交易信用獨立子網域（例如 `mail.<域名>`）」寫進 Will 的 DNS 待辦，日後若有行銷信不共用信譽。

### Q10 推播 token 生命週期（D7、A8-3）——草案漏了時間戳

- 業界基線（Firebase「Best practices for FCM registration token management」，[firebase.google.com/docs/cloud-messaging/manage-tokens](https://firebase.google.com/docs/cloud-messaging/manage-tokens)）【推】：伺服器存 token 與**最後更新時間戳**；App 每次啟動取得 token 並回報（至少每月更新）；超過約一個月沒更新視為 stale，不再推送並移除；收到 `UNREGISTERED`／`INVALID_ARGUMENT` 時刪除；token 閒置 270 天會失效。
- 既有機制：舊服務有「`is_active` 且 90 天未更新就停用」的殭屍 token 清理（`INV` §6 #3c【量】）。
- 草案：token 主鍵、App、帳號；登入 upsert、登出刪、停權與刪帳號同 tx 刪（`PLAN:96`）。**沒有 `last_seen_at` 與 stale 清除**——舊的殭屍清理沒有寫去向，新設計也沒有對應【量：PLAN 全文無 stale／last_seen】。
- 「停權與刪帳號在同一個 tx 刪 token」：在 Q1 的 B 之下仍成立（是 identity／accountsuspension 處置 tx 內的 notification 寫入，或改成同 tx 的事件訂閱者刪列，屬原子狀態變更，符合規則 4）。
- 限制：FCM 是唯一推播渠道——REAL（NADR:27）。
- **建議**：token 表加 `last_registered_at`；App 每次啟動呼叫註冊（upsert 更新時間）；每日 job 刪超過 60 天未更新的 token【猜：60 天是在 Firebase 建議的「一個月 stale」與 270 天失效之間取的值，交 Will 定】。D7 一起收 Android：同意草案（兩支 App 是 Flutter、FCM 原生支援）。

### Q11 摘要／批次與節流——草案延後

- 業界基線：Knock batch function（時間窗內合併成一則）【推】；GitHub 不做時間摘要，以 thread 合併（Q6）【推】。
- 建議：Q6 的 `group_key` 已處理互動洗版；時間窗摘要（例如每日 email 摘要）沒有需求，不做。「延到 1000 追蹤者」的延後項改由 Q6 取代並刪除。限制：無 REAL 限制推離基線。

### Q12 多語系

- 業界基線：伺服器依收件人 locale 渲染推播與 email，不用 FCM `body_loc_key`（多語系 ADR 已決）。✓
- 補一點：收件匣讀取時「依讀者語系」組字，語系來源要寫明（請求的 locale header vs 帳號 `preferred_locale`）。建議讀取用請求語系、送出用帳號 `preferred_locale`，與 NADR 修訂一致【量：NADR:87】。

### Q13（D5）取貨簡訊

- 前提在 B3 消失：B3 改賣家自寄，刪 PayUni、超商門市地圖、`BuyerStorePickupService`（`MASTER` B3【量】）。平台不再建 7-ELEVEN 取貨單，就沒有「可取貨」事件可以發簡訊。
- 限制：「生鮮在超商逾期是損失」——前提已被範圍裁切推翻（ADR `2026-10-27-general-ecommerce-scope-cut.md`）。
- **建議**：D5 從 A8 刪除；NADR 決策 9 的取貨簡訊候選加修訂「隨 B3 賣家自寄失效」。

### Q14（D9）代行者能否讀店家通知

- 業界基線：Shopify collaborator／staff 權限把通知與帳務分開授權；support impersonation 通常唯讀且稽核【推】。
- 草案建議 A（不能），與 NADR 決策 3、4 一致。U12 在 A8-0 查 client 是否依賴。同意。限制：NADR 決策 3——REAL（已接受）。注意 `seller.proto:306` 的白名單註解要同步改【量：PLAN:179 引用】。

### Q15（D7）Android、（D2）略——見 Q10、Q4。

### Q16 必要通知的渠道保證（NADR 待辦「至少送達一個渠道」）——草案漏題

- NADR 待辦要求 A8 起記錄「每筆交易通知是否至少送達一個渠道」（`NADR:80`）。草案的投遞紀錄有每渠道狀態，但沒有步驟產出這個指標或查詢，也沒有驗收【量：PLAN 全文無「觸達率」】。
- 建議：A8-3 加一個管理員唯讀查詢（或 slog 彙總）「近 N 天交易必要型別的未觸達數」，否則 SMS、LINE 的重評觸發條件永遠不會被量測。

---

## 3. 草案中「在既有結構上修補」或「沿用未重新評估」的地方

| # | 位置 | 問題 | 分類 |
|---|---|---|---|
| 1 | A8-5 全步、`PLAN:8`、不變量第 2 列「legacy 訂閱器」 | 為即將搬走的發送端寫轉接，100% 重工（第 1 節） | 沿用「wire 在邊界外」的繼承約束而沒重評 |
| 2 | D3 範圍含 Java 的 52 組 key | 同上 | 同上 |
| 3 | D5 取貨簡訊 | 前提已被 B3 賣家自寄推翻 | 沿用 NADR 決策 9 沒重評 |
| 4 | 不變量第 1、9 列＋A8-1 lockorder 測試 | 在來源 tx 內做完整通知建立，需要為每個來源加鎖順序案例 | 在既有 `events.Publish` 結構上擴大用法，基線是 tx 內只排意圖 job（Q1） |
| 5 | D6「型別 enum 讓 client 選圖示」 | 舊 `data.type` 照搬成 proto enum，B 步驟每加型別都改契約 | 照搬（Q5） |
| 6 | A8-2 串流「游標續傳不漏」 | 在串流上重做持久佇列語意；照搬 SSE `Last-Event-ID` 的設計 | 照搬（Q8） |
| 7 | `product.viewed`、收藏通知 | 舊模板照搬成型別候選；並衍生「節流延後項」 | 照搬（Q2） |
| 8 | 延後項「1000 追蹤者才做節流聚合」 | 資料模型選擇現在發生，延後只會在上線後遷移 | 只記待決（Q6） |
| 9 | token 表無最後活躍時間 | 舊殭屍清理的去向沒寫，新設計也沒有 FCM 建議的 stale 處理 | 沿用未盤點（Q10） |
| 10 | D4 把 follower、post 歸行銷 | 沿用舊 10 類的名稱分群，沒依「是否行銷利用個資」重分 | 沿用（Q7） |
| 11 | NADR 未觸達率指標無步驟 | 待辦的量測沒有落地 | 只記待決（Q16） |
| 12 | NADR 沒有「自建 vs SaaS」與「River vs 工作流引擎」 | 決策沒寫被否決的方案 | 漏題（Q0、Q0b） |

## 4. 對步驟的具體改法（供 Will 拍板後改 PLAN）

- 刪 A8-5；PLAN `:8` 改寫；`MASTER`「模組完成的定義」加「通知型別隨發送端搬」；B2、B3、B4、B7、B8、B9、B10、A5、A9 各加一行承接。
- A8-1：入口改成「事件 → 意圖 job」；lockorder 測試改成結構性測試；型別註冊表加 `group_key`；token 表加 `last_registered_at`。
- A8-2：呈現分類 enum 取代型別 enum；串流改成失效訊號，移除續傳游標。
- A8-3：stale token 清除 job；未觸達查詢。
- D3 縮小、D5 刪除；D4 重分三群。
