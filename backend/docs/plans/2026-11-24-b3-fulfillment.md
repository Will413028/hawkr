# B3 fulfillment：出貨改成賣家自寄的 Go 模組，刪除 Java shipping 與 PayUni 物流

**Goal：** `backend/internal/fulfillment` 擁有出貨單、出貨狀態機與賣家運費規則，對外經 Connect（`api/`）；賣家回填承運商與追蹤號碼，逾期、自動完成與異常由伺服器決定；Java shipping-service、`payuni-webhook-service`、Uber 程式與其 gateway 路由、client 介面全部刪除。驗收見步驟 9。

**上層：** `2026-09-24-go-modular-monolith.md` 的 B3（全域約束與「模組完成的定義」在該檔，本檔不重複）。ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`。

**附件：** `2026-11-24-b3-fulfillment/`（快照 dev `496b048e2`）：`interface-inventory.md`（Java shipping：52 RPC、19 表、9 排程、24 缺陷）、`caller-inventory.md`（Java／Go／gateway／CodeMeta／設計書呼叫端）、`client-inventory.md`（四個 client 與 packages）、`decision-brief.md`（決定題的選項與建議）。

## 前提（開工時核對過，2026-11-24）

- 會改到或取代的既有部署是否在線：不在線，系統從未上線（根 `AGENTS.md`），不受相容性窗口、資料搬移與 App 舊版相容約束；回滾就是 revert。
- 出貨只做賣家自寄；物流商只留 port 與模擬 adapter（ADR 2026-11-15，Will 2026-11-15；已確認）。
- Java shipping 除運費規則與出貨狀態外直接刪，order 的 `ShipmentCreationBackstopScheduler`、`ShippingFeeGrpcFetcher` 直接刪（Will 2026-11-23；已確認，主計畫 B3）。保留的兩塊也是重寫：現有運費是平台級距，不是賣家規則（`interface-inventory.md` §13-1）。
- 範圍分界：gateway `shipping_ambient.go` 與配送溫層（`deliveryTemp`、`ShippingTemperature`）歸 B4／B6，B3 不碰（Will 2026-11-24；已確認）。
- B6 cart 依賴 B3 的運費報價；契約要讓購物車預覽與結帳快照用同一個函式（Will 2026-11-24；已確認）。
- Java order、aftersales、notification 與 Go invoice-service 在 B3 完成後仍在線到 B8／B9（主計畫順序；已確認）。
- 設計書 69 已刪，主計畫與 ADR 對它的引用過期（`caller-inventory.md` §6；已確認，處理見 D13）。
- 「B3 計畫文字的 4 支假日匯入 RPC」在 proto 與全 repo 都不存在（`interface-inventory.md` D6；已確認）：主計畫假日段刪除時以實際存在的 `HolidayService` 與 `holidays` 表為準。

**驗收基線：** 未跑（寫計畫當下只做盤點）；步驟 0 建立。Java shipping-service 沒有整合測試，只有 393 個單元／契約測試（`interface-inventory.md` §0、§14）。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 出貨狀態只經 CAS 轉換，轉換與事件在同一交易寫 outbox | fulfillment 的狀態轉換函式、outbox relay、過渡期 legacy relay 給 Java 的 `shipment.*` |
| 每張已付款訂單恰好一張出貨單（`order_id` UNIQUE，不含退貨） | 建單 consumer（`order.paid`）、`GetShipmentByOrderNo` 過渡轉接、逾期掃描 |
| 撥款只由「買家確認收貨或自動完成」觸發，賣家標記不算（收貨 Gate） | `shipment.completed` 事件、B2／B10 的撥款接線 |
| 運費報價只有一個函式，結帳把結果快照進訂單 | `QuoteShipping`、cart 預覽（B6）、ordering 結帳（過渡期 Java order 經轉接） |
| 外部 webhook 先寫 inbox（事件 ID UNIQUE）再處理 | 物流 port、模擬 adapter |
| 依賴方向依 `backend/internal/arch/modules.go` | fulfillment → buyer、seller、media；（D3 拍板後）cart → fulfillment |

## 規模（搬遷）

| | Java shipping | payuni-webhook-service | gateway | client | 產生計數的位置 |
|---|---|---|---|---|---|
| RPC／路由 | 52 RPC | 27 主程式檔 | 31 路由、openapi 30 path | website 161、admin 19、buyer-app 83、seller-app 105 檔 | 三份盤點 §0 |
| 表／事件／排程 | 19 表、5 consumer、9 publisher、9 排程 | — | — | — | `interface-inventory.md` §0 |

## 範圍

- 做：fulfillment 模組（出貨單、狀態機、運費規則、報價、逾期與自動完成、異常回報、物流 port＋模擬 adapter）；Connect 契約；過渡期 legacy 轉接；四個 client 的自寄流程；刪 Java shipping、`payuni-webhook-service`、Uber、相關 gateway 路由、CodeMeta 值域與 client 介面；文件改寫與 ADR superseded。
- 不做：超商取貨、門市地圖、標籤列印 UI、追蹤號碼自動查詢、即時運費（ADR D4）；退貨物流（D8 建議歸 B9）；賣家自訂處理天數、多運費 profile（D2、D5 建議延後）。
- 延後：
  - 商品 `shipping_method` JSONB 的 `freeThreshold`、`feePayer`：B3 起不再讀（運費規則改在 fulfillment），欄位在 B4 catalog 改寫時不帶（觸發：B4；見 D14）。
  - 溫層欄位與 `shipping_ambient.go` 刪除（B4／B6 擁有）。
  - 撥款觸發接 `shipment.completed`（觸發：B2／B10 撥款步驟）。
  - Java 端過渡 legacy 轉接刪除（觸發：B8 ordering、B9 aftersales 搬進單體）。
  - media 殘留清理裡 shipping 的 `MediaGrpcClient`（主計畫 B1 延後項；觸發：最後一個 Java 呼叫端遷出）。
  - 真實物流 adapter（觸發：決定上線並簽約）。

## 步驟

檔案變動類型：步驟 2、3 是新增；步驟 6、7 是刪除；步驟 0、1、4、5、8 是修改（步驟 4 另刪 Java `ShipmentCreationBackstopScheduler`、`ShippingFeeGrpcFetcher`）。

- [ ] **0. 基線與特性測試**（被擋於：基線部分無；特性測試部分被 D4、D5 擋）
  - 範圍：在當下 dev 重跑三份盤點 §0 的計數指令（快照 `496b048e2` 之後 dev 已前進），差異補進附件；跑 backend、website、admin、兩支 App 的 CI 等價檢查記基線；對自寄後仍保留的行為（訂單出貨查詢、買家確認收貨、運費試算）寫經 gateway 對 Java 的 REST 特性測試，只覆蓋 D4／D5 拍板後語意不變的部分。
  - 消費端：無（只加測試）。
  - 不能動：Java 程式。
  - 驗收：基線結果寫進進度表；特性測試對 Java 綠，由 `backend-characterization.yml` 在 CI 跑（本機不起容器），結果出來前標「待 CI」。
  - 停止條件：拍板後保留行為與 Java 全不相同 → 回報，特性測試改為只對單體寫。
- [ ] **1. 文件前置**（被擋於：D13）
  - 範圍：主計畫與 ADR 對設計書 69 的引用改指本計畫；主計畫假日段修正（4 支 RPC 不存在）。設計書改寫放步驟 8。
  - 驗收：`rg -n '設計書 ?69|69 號' backend/docs docs` 只剩歷史紀錄；`python3 scripts/check-payuni-residue.py`、`python3 scripts/check-doc-paths.py` 綠。
- [ ] **2. Connect 契約**（被擋於：D2、D3、D4、D7、D10）
  - 範圍：`api/` 新增 fulfillment 服務：賣家出貨（回填承運商＋追蹤號碼）、買家確認收貨與回報未收到、出貨單查詢、賣家運費設定讀寫、`QuoteShipping`；生成 Go／TS／Dart。
  - 消費端：`rg -n 'shipping|shipment' api backend/internal packages/hawkr-api-ts/src packages/hawkr_api/lib -g '!**/gen/**'`
  - 不能動：B4／B6 擁有的溫層欄位；其他服務的 proto。
  - 驗收：`buf lint`、`buf breaking`（上線前允許 breaking，記錄）、三端生成無 drift。
  - 停止條件：B6 計畫已定的 cart 契約與 `QuoteShipping` 形狀衝突 → 停下。
- [ ] **3. fulfillment 模組**（被擋於：步驟 2、D1、D4、D5、D6、D9）
  - 範圍：schema（atlas migration）、狀態機（CAS＋outbox）、運費規則與報價、建單 consumer（訂閱 `order.paid`；在 MQ topology 定義檔與 `exchanges.go` 新增 fulfillment 的 queue 綁定）、逾期掃描與自動完成（River job，`bizcalendar`）、異常回報事件、文字走 `internal/text`／uitext、物流 port＋模擬 adapter＋inbox；`CARRYOVER.md` 沿用盤點表（從 `interface-inventory.md` 去向欄轉入）。
  - 消費端：`rg -n 'fulfillment' backend/internal backend/cmd`
  - 不能動：`modules.go` 既有邊（D3 的新邊除外）。
  - 驗收：`gofmt -l`、`make lint-monolith`、`GOWORK=off go vet -tags integration ./...`、integration 測試（狀態轉換每條邊、非法轉換、同事件重送冪等、逾期以工作日計、報價免運邊界）；mutation 清單寫進 `mutation/`（拿掉 CAS 前置狀態、outbox 改交易外、免運比較改 `>`、逾期不跳假日、inbox 拿掉 UNIQUE，各自預期變紅的測試），`python3 scripts/mutation/run.py --validate`，結果待 CI。
- [ ] **4. 過渡期 legacy 轉接**（被擋於：步驟 3、D1、D8）
  - 範圍（Java 結帳那一句依 D15 定稿）：`internal/legacy` 提供 Java order、aftersales、invoice-service 仍呼叫且自寄後仍成立的 shipping RPC（由 `caller-inventory.md` §1.1 逐支列出並在派工裡定稿）與 `shipment.*` 事件 relay；Java 端刪掉 PayUni 專屬呼叫與 `ShipmentCreationBackstopScheduler`、`ShippingFeeGrpcFetcher`；Java 結帳：D15 選 A 時改經轉接呼叫 `QuoteShipping`，選 B 時固定 0 運費。
  - 消費端：`rg -n 'ShippingServiceGrpc|ShippingGrpcClient' backend/services --glob '*.java' --glob '!**/target/**'`；`rg -n 'shipping' backend/services/invoice-service --glob '*.go'`
  - 不能動：Java order 的狀態值與事件 routing key（B8 前仍有消費者）。
  - 驗收：Java 相關模組 `mvn test`（`backend-java-tests.yml`）；端到端情境寫成 `backend-integration-tests.yml` 跑的測試（本機不起容器，待 CI）：「付款 → 建出貨單 → 賣家出貨 → 確認收貨 → Java 訂單完成」與「逾期 → 取消退款」，斷言 DB 與 MQ wire；mutation：relay 漏發一個事件、轉接回錯狀態、Java 結帳不帶 `QuoteShipping` 的運費（D15 選 A 時；選 B 時改為斷言運費固定 0）。
  - 完成後的狀態：Java 退貨物流不可用到 B9（D8）。
- [ ] **5. client 自寄流程**（被擋於：步驟 2、3、D12）
  - 運費讀取：client 經 Connect 直接呼叫單體的 `QuoteShipping`；結帳金額仍以伺服器（過渡期 Java order）回傳的快照為準。
  - 範圍：K 盤點 D12 第 1、2、3、4、6、7 組的新增與改寫；3、4 先交線框取得同意。
  - 消費端：`client-inventory.md` §1–§5 的檔案清單。
  - 不能動：溫層欄位與 `ShippingMethodGroup` 中溫層邏輯（B4／B6）；若同檔衝突，依 D14 原則協調。
  - 驗收：website、admin `pnpm` lint／typecheck／unit；兩支 App analyze、format、custom_lint、生成檢查與改到的測試檔（完整 `flutter test` 與 e2e 交 CI）；新增行為的 mutation（追蹤號碼未填仍可送出、確認收貨不呼叫 RPC）。
- [ ] **6. client 刪除**（被擋於：D12）
  - 範圍：超商地圖、門市選擇、物流開通、標籤列印、異常看板與 badge、admin `shipping` tab、通知偏好開關、3003 守門測試（D10）。
  - 驗收：同步驟 5；`rg -n 'store_map|storePickup|shipment_anomaly|shipping_quotes' website admin buyer-app seller-app -g '!node_modules' -g '!.next'` 為 0。
- [ ] **7. 切路由與刪後端**（被擋於：步驟 4、5、6、D11、D14）
  - 範圍：gateway shipping 路由與 handler、`routes_webhook_simulator.go`、`routes_payment.go` 的 PayUni 物流代理（與 B2 協調：先到者刪）、openapi 對應 path；刪 Java shipping-service（含 `HolidayService`、`holidays`）、`payuni-webhook-service`、Uber 全部（主計畫 B3 清單）、CodeMeta 物流值域（溫層除外）與 bizcode 9024、`sellerserver` 的 `MerId` 與 `GetSellerShippingProfile`、compose 與 `.env.template` 設定、MQ topology 裡舊 `shipping.*` queue 與 `hawkr.shipping` exchange；notification 改訂閱 `order.paid`（D11），附測試（收到 `order.paid` 發賣家推播）與 mutation（拿掉訂閱綁定要變紅）。
  - 消費端：`rg -n -i 'shipping-service|payuni-webhook|uber' backend docker-compose*.yml .github -g '!go.sum' -g '!backend/docs/**' | rg -v 'go\.uber\.org'`
  - 不能動：`shipping_ambient.go`（B4）。
  - 驗收：全部 backend workflow 等價檢查；gateway 對齊 openapi 的 CI；上列 `rg` 為 0（溫層與 `shipping_ambient.go` 除外）。
- [ ] **8. 文件**（被擋於：D13）
  - 範圍：設計書 65、06、20–23 改寫成自寄；03、09、29、36、58 刪物流商與 Uber 段落；`backend/docs/runbooks/uber-quote-retention*` 刪；D13 列出的 ADR 標 superseded。
  - 驗收：`python3 scripts/check-doc-paths.py`；`rg -n -i 'payuni.*物流|7-ELEVEN|黑貓|Uber' backend/docs docs` 只剩 superseded ADR 與歷史紀錄。
- [ ] **9. 收尾**：主計畫「模組完成的定義」逐項核對；design-review skill 跑在累積 diff；長期檢查（狀態機邊測試、mutation 清單）已在 CI；延後項附觸發條件搬回主計畫 B3、B6、B8、B9；需要 Will 做的事列出。驗收：主計畫勾選、`mutation` CI job 綠、各 workflow 每個 job 綠。

## 決定

選項與取捨在 `decision-brief.md`。

| # | 問題 | 建議 | 狀態 | 擋住 |
|---|---|---|---|---|
| D1 | 過渡期 Java 怎麼看到出貨 | legacy 最小 gRPC＋事件 relay | 未決 | 3、4、7 |
| D2 | 運費規則模型與擁有者 | fulfillment 擁有賣家層級固定運費＋免運門檻，單一 `QuoteShipping` | 未決 | 2、3、5 |
| D3 | cart→fulfillment 依賴邊 | 加 | 未決 | 2 |
| D4 | 出貨狀態機 | 新字串 enum，5 狀態 | 未決 | 2、3 |
| D5 | 期限與自動完成 | 付款後 3 工作日、出貨後 10 天、保留收貨 Gate | 未決 | 3 |
| D6 | 配送異常 | 買家回報未收到，發事件；不自動偵測 | 未決 | 3 |
| D7 | 承運商欄位 | 固定清單＋OTHER 文字，追蹤號碼必填 | 未決 | 2、5 |
| D8 | 退貨物流 | 歸 B9；B3–B9 間 Java 退貨物流不可用 | 未決 | 4、7 |
| D9 | 物流 port 必要性 | 照 ADR D3 做最小 port（或修訂 ADR 延後） | 未決 | 3 |
| D10 | 配送方式碼 | 契約拿掉配送方式；3005 留給 procurement 自己表達（與 B4 D10 衝突，兩題一起拍） | 未決 | 2、5、6 |
| D11 | 新訂單推播 | 改訂閱 `order.paid` | 未決 | 7 |
| D12 | UI 改動（7 組） | 全同意，新畫面先交線框 | 未決 | 5、6 |
| D13 | 文件與 ADR | 引用改指本計畫，列出的 ADR 標 superseded | 未決 | 1、8 |
| D14 | 與 B4 刪除次序 | B3 刪 handler 時帶走引用，檔案留 B4；商品運費欄位由 B4 不帶 | 未決 | 7 |
| D15 | 過渡期 Java 結帳怎麼拿運費 | A：Java `CheckoutService` 運費來源換成轉接的 `QuoteShipping`、刪運費快取（建議）；B：過渡期結帳固定 0 運費直到 B8 | 未決 | 4 |

拍板順序：D1、D2、D4、D10 → D15 → D3、D5、D6、D7、D8 → 其餘可同時；D9 獨立。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 盤點 | 完成 | 三份盤點（快照 `496b048e2`） | — | |
| 0 基線與特性測試 | 未開始 | | | |
