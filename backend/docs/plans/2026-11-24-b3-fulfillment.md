# B3 fulfillment：賣家運費設定進 seller 模組，刪除 Java shipping 與 PayUni 物流

**Goal：** `internal/modules/seller` 擁有賣家運費設定（固定運費、免運門檻、處理天數）與純函式報價，經 Connect 對外；Java shipping-service、`payuni-webhook-service`、Uber 程式、Java 與 Go 舊服務裡依賴它們的程式、gateway 物流路由、CodeMeta 物流值域與 client 的超商／標籤／開通／異常介面全部刪除；`modules.go` 沒有 `fulfillment`。出貨記錄與含出貨的訂單狀態機在 B8 ordering 交付。驗收見步驟 8。

**上層：** `2026-09-24-go-modular-monolith.md` 的 B3（全域約束與「模組完成的定義」在該檔）。ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`（含 2026-11-24 修訂）。

**附件：** `2026-11-24-b3-fulfillment/`（快照 dev `496b048e2`）：`interface-inventory.md`（Java shipping）、`caller-inventory.md`（呼叫端）、`client-inventory.md`（client）、`premise-review.md`（P1–P6，已決）、`decision-brief.md`（v1 簡報；D1–D6、D9、D15 已被 P1–P6 取代，其餘題目在下方「決定」重列）。

## 前提（開工時核對過，2026-11-24）

- 會改到或取代的既有部署是否在線：不在線，系統從未上線（根 `AGENTS.md`），不受相容性窗口、資料搬移與 App 舊版相容約束；回滾就是 revert。
- 出貨只做賣家自寄；出貨歸訂單（B8）、運費設定歸 seller、物流 port 延後（ADR 2026-11-15 與其 2026-11-24 修訂，Will；已確認）。
- B3 到 B8 之間 Java 訂單停在已付款、結帳運費固定 0；訂單完成、轉帳觸發、crm 的 `order.completed` 與有已付款訂單的賣家關店在這段期間不會發生（P3，Will 2026-11-24；已確認）。
- Java 端依賴 shipping 的程式一律刪除，不留回空的 stub、不寫轉接（P3；已確認）。
- gateway `shipping_ambient.go` 與配送溫層（`deliveryTemp`、`ShippingTemperature`）歸 B4／B6，B3 不碰（Will 2026-11-24；已確認）。
- B6 cart 依賴本計畫的報價函式（Will 2026-11-24；已確認）。
- B4 D10 已依 P6 修訂：catalog 不存配送方式、重量尺寸，不依賴運費設定（`db9d2e5d4`，Will 2026-11-24；已確認）。
- 設計書 69 已刪，主計畫與 ADR 的引用過期（`caller-inventory.md` §6；已確認）。主計畫假日段的 4 支匯入 RPC 不存在（`interface-inventory.md` D6；已確認）。

**驗收基線：** 未跑；步驟 0 建立。Java shipping-service 沒有整合測試（`interface-inventory.md` §0）。不寫特性測試：新設計裡沒有任何 Java 出貨或運費端點保留同一語意（運費模型與出貨擁有者都換了），主計畫「開工前先有特性測試」的對象在本階段是空集合；出貨的特性測試責任移到 B8。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 運費報價只有一個純函式，結帳把結果快照進訂單 | seller 模組的報價函式；B6 cart 預覽、B8 ordering 結帳 |
| 運費設定只有 seller 一個擁有者 | seller 的運費設定表；catalog 不存運費或配送方式（P6） |
| 沒有模組反向讀出貨狀態；出貨下游由訂單擁有者呼叫 | `backend/internal/arch/modules.go`（無 `fulfillment`）；B8 的 ordering→invoice、ordering→payment |
| 依賴方向依 `modules.go` | cart、ordering → seller（既有邊，`modules.go:24,26`） |

## 規模

| | Java shipping | payuni-webhook-service | Java 其他服務的 shipping 依賴 | gateway | client | 產生計數的位置 |
|---|---|---|---|---|---|---|
| 量 | 144 檔、52 RPC、19 表、9 排程 | 27 主程式檔 | 19 處 gRPC 呼叫、3 個 client（order、aftersales、product）＋ Go invoice-service | 31 路由、openapi 30 path | website 161、admin 19、buyer-app 83、seller-app 105 檔（含順帶提到） | 三份盤點 §0 |

## 範圍

- 做：seller 運費設定與報價（schema、Connect、賣家設定畫面）；刪 Java shipping、`payuni-webhook-service`、Uber、Java order／aftersales／product 與 Go invoice-service 依賴 shipping 的程式、gateway 物流路由與 openapi、CodeMeta 物流值域（溫層除外）、MQ topology 的 shipping 部分、client 的超商／標籤／開通／異常介面；`modules.go` 刪 `fulfillment`；文件改寫與 ADR superseded。
- 不做：出貨記錄、出貨狀態機、賣家出貨與買家確認收貨畫面（B8，P1）；退貨物流（B9）；物流 port 與模擬 adapter（延後，P5）；超商、門市地圖、標籤、追蹤自動查詢、即時運費（ADR D4）。
- 延後（收尾時搬回主計畫）：
  - B8 ordering：出貨記錄（0..N 筆）、含出貨的訂單狀態機、逾期取消（用運費設定的處理天數）、自動完成、確認收貨、「未收到」回報、承運商清單（v1 D7）、client 出貨與收貨畫面（v1 D12 第 2、3、7 組）、ordering→invoice 的開票呼叫。觸發：B8 開工。
  - B9 aftersales：退貨物流（買家自寄回填）。觸發：B9 開工。
  - 物流 port 與真實 adapter。觸發：決定上線並簽物流合約。
  - 溫層欄位與 `shipping_ambient.go`（B4／B6）。
  - 主計畫 B1 延後項裡的託運單 `PutSecure` 接線：託運單隨物流商退場不存在，這個觸發作廢（步驟 1 改主計畫文字）；shipping 的 `MediaGrpcClient` 隨步驟 4 刪除。

## 步驟

檔案變動類型：步驟 2、3 是新增（seller 模組與 client 設定畫面內）與修改；步驟 4、5、6 是刪除（步驟 4 另修改 Java 結帳為固定 0 運費）；步驟 0、1、7 是修改。

**並行軌道（2026-11-24 重排）：** 步驟之間只剩下列相依，四條軌道可各開一個 session 同時做，每個 session 由主 session 派 Sonnet 實作、各用自己的 worktree（`<repo>/.worktrees/<branch>`），合進 dev 時後到者 rebase：

| 軌道 | 步驟 | 開工條件 | 主要檔案範圍（避免撞檔） |
|---|---|---|---|
| A 後端刪除 | 0 → 4 → 5（4、5 必須連著做、同一批合進 dev：gateway import 步驟 4 要刪的 `pkg/proto/shipping`） | 現在 | `backend/services`、`backend/gateway`、`backend/pkg`、`backend/internal/{arch,platform,legacy,modules/notification}`、`api/proto`、compose、`.github` |
| B seller 運費 | 2 → 3（3 要 Will 看過線框） | 現在 | `backend/internal/modules/seller`、`api/hawkr/seller`、生成物；website 與 seller-app 的賣家設定 |
| C client 刪除 | 6 | 現在（呼叫端先刪、伺服器後刪，符合「呼叫端遷完再刪舊」；系統未上線，順序反過來也不壞） | website、admin、buyer-app、seller-app 的物流畫面（不含 B 軌道的設定頁） |
| D 文件 | 1 + 7（同一個 session） | 現在 | `backend/docs`（不含 `plans/`）、`docs/adr` |

- 撞檔點：步驟 4 刪 notification 分類與步驟 6 刪 client 開關要同一批合進 dev（步驟 4 已寫）；步驟 3 與步驟 6 都改 seller-app 上手清單，6 只刪「物流設定」步驟、3 加「運費設定」步驟，後到者 rebase；主計畫檔只由 D 軌道改。
- `第三方整合/PayUni 物流官方 API 總覽.md`、`Uber Direct API 總覽.md` 被 `PayUniInboundWireContractTest`、`UberInboundWireContractTest` 讀取，改由步驟 4 與這兩個測試同一個 commit 刪除（Will 2026-11-24，讓 D 軌道先合）；D 軌道已拿掉它們在 `_sidebar.md`、`index.md` 的條目。
- 步驟 8 在四條軌道都完成後做。

- [ ] **0. 基線**（軌道 A 第一件事；被擋於：無）
  - 範圍：在當下 dev 重跑三份盤點 §0 的計數指令，差異補進附件；跑 backend 的 CI 等價檢查，記既有失敗。B、C、D 軌道開工時各自跑自己會碰到的檢查當基線，記在進度表。
  - 驗收：進度表記錄基線與計數差異。
- [x] **1. 文件前置**（被擋於：R4）
  - 範圍：主計畫 B1、B3、B6、B8、B9 條目依 ADR 2026-11-24 修訂改寫（B8 加出貨、B6 報價來源、B9 退貨物流、B1 的託運單觸發作廢）；對設計書 69 的引用改指 ADR 修訂；假日段修正。
  - 驗收：`rg -n '設計書 ?69' backend/docs docs` 只剩歷史紀錄；`python3 scripts/check-doc-paths.py` 綠。
- [ ] **2. seller 運費設定與報價**（被擋於：R2、R6）
  - 範圍：seller 模組加運費設定表（atlas migration；以賣家業務鍵為主鍵、固定運費 TWD 非負、免運門檻可空、處理天數 1–7 預設 3，CHECK 約束）；純函式報價（輸入賣家設定與商品小計，回運費與距免運差額）；Connect：`api/hawkr/seller/v1alpha1/seller.proto` 的 `SellerAppService` 加讀寫運費設定；報價以 seller 根套件的函式提供給 B6／B8（in-process，不開對外 RPC；B6 的購物車 RPC 自己帶報價結果）；生成 Go／TS／Dart；文字走模組 `internal/text`；`backend/docs/modules/seller.md` 補運費設定。
  - 消費端：`rg -n 'SellerAppService' website admin buyer-app seller-app packages -g '!**/gen/**' -g '!node_modules'`；B6、B8 計畫（尚未開工）。
  - 不能動：seller 既有 RPC 與表（R6 的能力改名除外）。
  - 驗收：`gofmt -l`、`make lint-monolith`、`GOWORK=off go vet -tags integration ./...`、`buf lint`、三端生成無 drift；單元測試覆蓋報價邊界（小計等於門檻、門檻空、運費 0）；integration 測試覆蓋設定讀寫與 CHECK 拒絕（`backend-integration-tests.yml`，待 CI）；mutation 清單寫進 `mutation/`（門檻比較 `>=` 改 `>`、門檻空時當 0、處理天數 CHECK 拿掉、更新不檢查擁有者），`python3 scripts/mutation/run.py --validate`，結果待 CI。
- [ ] **3. 運費設定畫面**（被擋於：步驟 2、R1 第 4 組）
  - 範圍：website 賣家設定與 seller-app 設定加「運費設定」（固定運費、免運門檻、處理天數）；先交線框取得 Will 同意；在上手清單取代「物流設定」步驟。
  - 消費端：`client-inventory.md` §4。
  - 驗收：website `pnpm` lint、typecheck、unit；seller-app analyze、format、custom_lint、生成檢查與改到的測試檔（完整 `flutter test` 交 CI）；mutation：送出不帶處理天數、錯誤不顯示伺服器文字，各自要有測試失敗。
- [ ] **4. 刪 Java shipping 與依賴它的後端程式**（被擋於：步驟 0、R3、R5）
  - 範圍：
    - 刪 Java shipping-service（含 `HolidayService`、`holidays`）、`payuni-webhook-service`、Uber 全部（主計畫 B3 清單）。
    - Java order：刪 `ShippingEventConsumer`、`AutoCompleteScheduler`、`ShippingFeeGrpcFetcher`、`ShipmentCreationBackstopScheduler`、shipping gRPC client 與它的呼叫點（訂單列表、詳情的物流欄位一起刪）；結帳運費固定 0、配送選項移除。
    - Java aftersales 刪 shipping client 與退貨物流呼叫；product-service 刪 shipping client 的 3 支呼叫。
    - Go invoice-service 刪 `internal/client/shipping`、`internal/grpc/shipment_status.go` 與依賴它們的流程。
    - notification 依 R3 處理賣家新訂單推播。
    - MQ topology 刪 `hawkr.shipping` 與 `shipping.*` queue；CodeMeta 物流值域（溫層除外）與 bizcode 9024；`sellerserver` 的 `MerId` 與 `GetSellerShippingProfile`；compose、`.env.template`、CI 路徑；`modules.go` 刪 `fulfillment` 與指向它的全部邊（含 catalog→fulfillment，與 B4 協調見 R5），同步刪 `internal/platform/lockorder/lockorder.go` 的 `fulfillment`（`internal/arch` 的 lockorder 測試要求兩者一致）。
    - Java `common-core` 的物流碼值 enum 與 helper、`java-proto` 的 `shipping.proto`、`backend/pkg/proto/shipping` 與 `backend/api/proto/shipping`、`pkg/proto/order` 裡只服務 shipping 的欄位；contract golden 隨之更新。
    - `backend/docs/第三方整合/PayUni 物流官方 API 總覽.md`、`Uber Direct API 總覽.md`（從步驟 7 移來，見並行軌道段）。
    - 舊種子 `06-shipping.sql`；seller `CARRYOVER.md` 裡「固定 `store_code` 是為了 shipping 的 `mer_id`」那條理由改寫（理由不存在後，重新回答 `store_code` 是否還要固定，寫進 `CARRYOVER.md`）。
    - notification：單體的 `CategoryShipment`、`CategoryShipmentAnomaly` 與舊服務的偏好、`shipment.delivered`／`shipment.anomaly.*` 模板刪除（配送異常不再存在；出貨通知在 B8 隨出貨重建），與步驟 6 刪 client 開關同一批合進 dev。
    - 搬遷清單裡 shipping 與 payuni-webhook 的 RPC、事件、副本填 `disposition`（單體規則 8）。
  - 消費端：`rg -n 'ShippingServiceGrpc|shippingGrpcClient|client/shipping|hawkr\.shipping|shipment\.' backend --glob '!**/target/**' --glob '!**/gen/**' --glob '!backend/docs/**'`
  - 不能動：`shipping_ambient.go`；溫層值域；Java order 的其他狀態轉換與 payment 呼叫（B2 的轉接依賴它們）。
  - 驗收：`backend-java-tests.yml`、`backend-go-checks.yml`、`backend-monolith.yml`、`backend-migration-inventory.yml` 等價檢查綠（`python3 scripts/check-migration-inventory.py`）；上列消費端 `rg` 的命中只剩「步驟 4 開工時列出的允許清單」：已套用的 migration（只能往前，不改）、步驟 5 才刪的 gateway 檔與測試、步驟 7 的文件；允許清單寫進進度表，每項附理由；Java order 的單元測試斷言結帳運費為 0 且不呼叫任何 shipping client；mutation：結帳運費改成非 0 常數要變紅。
  - 完成後的狀態：Java 訂單不會離開已付款（P3）。
- [ ] **5. gateway 與 openapi**（被擋於：步驟 4、R5；與步驟 4 連著做，同一批合進 dev）
  - 範圍：刪 gateway shipping 路由與 handler、`routes_webhook_simulator.go`、`routes_payment.go` 的 PayUni 物流代理（與 B2 協調：先到者刪）、`reverse_proxy.go` 的 `payuni-webhook` 項、openapi 對應 path；對 `shipping_ambient.go` 的引用隨 handler 刪除，檔案本身留給 B4（R5）。
  - 消費端：`rg -n -i 'shipping|shipment|store_map|payuni-webhook' backend/gateway --glob '*.go' -g '!shipping_ambient.go'`
  - 驗收：`backend-gateway-json-contract.yml` 等價檢查（`scripts/gateway-json-contract-check.py`、`openapi_coverage_test.go`）；上列 `rg` 只剩溫層相關。
- [ ] **6. client 刪除**（被擋於：R1、R2，已決）
  - 範圍：R1 第 1、5、6 組與第 4 組的刪除部分——超商取貨、門市地圖與地址簿門市分頁、配送方式分組、物流開通頁與設定畫面、標籤列印與黑貓／叫車／7-11 對話框、異常看板與 badge、admin `shipping` tab、通知偏好的配送異常開關、商品即時報價；訂單畫面的物流貨態區塊刪除（B8 再加出貨資訊）；3003 守門測試依 R2 刪或改寫。
  - 消費端：`client-inventory.md` §1–§6。
  - 不能動：溫層欄位與它所在的邏輯（B4／B6）；同檔衝突依 R5。
  - 驗收：同步驟 3 的 client 檢查；`rg -n 'store_map|storePickup|store-pickups|shipment_anomaly|shipping_quotes|shipping_onboarding' website admin buyer-app seller-app -g '!node_modules' -g '!.next' -g '!**/gen/**'` 為 0；e2e 交 CI。
- [ ] **7. 文件**（被擋於：R4，已決；與步驟 1 同一個 session）
  - 範圍：設計書 65、06、20–23 改寫（自寄、出貨歸訂單、運費設定歸 seller）；03、09、29、36、58 刪物流商與 Uber 段落；`backend/docs/runbooks/uber-quote-retention*` 刪；R4 列出的 ADR 標 superseded。
  - 範圍補充：`backend/docs/開發規範/` 裡描述物流商的段落刪除；`backend/docs/第三方整合/` 的 7-ELEVEN、黑貓、PayUni 物流總覽整份刪除（整合不存在；PayUni 物流與 Uber 兩份移到步驟 4 刪）；R4 名單外仍提到物流商的 ADR 只在文末加一行「物流商已依 ADR 2026-11-15 退場」，不改正文。
  - 驗收：`python3 scripts/check-doc-paths.py`、`python3 scripts/check-payuni-residue.py` 綠；`rg -l -i 'Uber|黑貓|7-ELEVEN|超商取貨' backend/docs docs` 的命中（不計步驟 4 才刪的兩份總覽）只在 `docs/adr/`（superseded 或加了退場註記的 ADR）與 `backend/docs/plans/`（計畫與盤點的歷史紀錄）。
- [ ] **8. 收尾**（被擋於：步驟 1–7）：主計畫「模組完成的定義」逐項核對（seller 運費設定部分）；design-review skill 跑在累積 diff；延後項附觸發條件搬回主計畫 B1、B8、B9；需要 Will 做的事列出。驗收：各 workflow 每個 job 綠、`mutation` CI job 綠、主計畫勾選。

## 決定

前提重審 P1–P6：已決 2026-11-24 Will（照建議），內容見 `premise-review.md` 與 ADR 修訂。

v1 題目的去向：D1–D6、D9、D15 被 P1–P6 取代；D7（承運商清單）延後到 B8；D8（退貨物流歸 B9）隨 P1 成立，延後到 B9；D3（cart 依賴邊）不需要（P4）。剩下要拍板的：

| # | 問題 | 建議 | 狀態 | 擋住 |
|---|---|---|---|---|
| R1 | UI 改動（v1 D12 的第 1、4、5、6 組在 B3；第 2、3、7 組移到 B8） | 全同意；第 4 組的運費設定新畫面先交線框 | 已決 2026-11-24 Will（照建議） | 3、6 |
| R2 | 配送方式從契約拿掉（v1 D10） | 拿掉；3005 交給 procurement 自己表達；B6、B8 的契約不帶配送方式 | 已決 2026-11-24 Will（照建議） | 2、6 |
| R3 | 賣家新訂單推播（v1 D11） | 改綁 `order.paid`（過渡期由 Java order 發），不再依賴 `shipment.created` | 已決 2026-11-24 Will（照建議） | 4 |
| R4 | 文件與 ADR superseded（v1 D13） | 設計書依步驟 7 改寫；4 份 Uber ADR、`free-shipping-subsidy-attribution`、`logistics-finance-domain-ownership`、`shipping-fee-cache-no-warmup-no-scheduler`、`checkout-free-shipping-and-degradation` 標 superseded | 已決 2026-11-24 Will（照建議） | 1、7 |
| R6 | seller 既有 `SELLER_CAPABILITY_SHIPPING_SETUP`（`CapShippingSetup`，上手清單的「物流設定」能力） | 改成「運費設定已填」：賣家建好運費設定才算完成這個能力，名稱改為 `SHIPPING_FEE_SETUP`；運費設定的寫入以既有賣家身分檢查（只寫自己的 `store`）；或刪掉這個能力、不擋上手流程 | 已決 2026-11-24 Will（照建議） | 2、3 |
| R5 | 與 B4 的刪除次序（v1 D14） | B3 刪 gateway handler 時帶走對 `shipping_ambient.go` 的引用，檔案本身由 B4 刪；誰先合進 dev，後者 rebase | 已決 2026-11-24 Will（照建議） | 4、5 |

既有缺陷（`interface-inventory.md` §12，24 條）：全部隨 Java 刪除消失；出貨相關的 D1、D2、D4、D7、D10 在 B8 設計出貨時當反例參考。拍板 R1–R5 時一併確認。

R1–R6 已決 2026-11-24 Will（照建議；R6 選「改成運費設定已填、改名 `SHIPPING_FEE_SETUP`」）。既有缺陷處置一併確認。

計畫審查（v2，2026-11-24）：findings 7（blocking 1）；修 6、升為決定題 1（F3→R6）、駁回 0。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 盤點與前提重審 | 完成 | 三份盤點（快照 `496b048e2`）；v1 計畫審查 2 輪；前提重審批判 1 輪 | v2 計畫審查 1 輪，修正後未再審（只改驗收文字與補範圍） | `3d8ef0bc0`、`b1bb27a92` |
| 0 基線 | 完成 | dev `e43d89b87`。計數：三份盤點 §0 重跑，後端數字與快照相同（差異見 `caller-inventory.md` §0 末）。檢查：`./mvnw test -B -fae` 綠；go.work 6 個模組 `go vet`（含 `-tags integration`）與 `go test` 綠；`make lint-monolith` 0 issues；單體 `go vet`（三種 tag）綠；`check-migration-inventory.py`、`gateway-json-contract-check.py`、`TestEveryRouteIsInOpenAPISpec` 綠；`gofmt -l` 空。既有失敗（A8-3d 合進 dev 帶來，非本計畫）：單體 `go test` 的 `internal/modules/notification` `TestSubscriptionsMatchTheEventAdjudication`、`TestErasureSubscriptionsCoverTheDeletionEvents`，`internal/httpapi/adminauth` `TestDeclarationsMatchTheServiceDescriptors`（112≠110）、`TestEveryDeclarationHasAnAuditModeAndPaymentWritesAreAudited`（多 `ResendDeliveries`） | Java `-Dgroups=integration`、Go integration、`backend-integration-tests.yml`：起容器，交 CI | |
| 4 刪 Java shipping | 進行中 | 已做（分支 `b3-a-backend-delete`）：刪 shipping-service、payuni-webhook-service、`api/proto/shipping`、`pkg/proto/shipping`、java-proto 副本；product：刪 shipping client、`CalculateShippingOptions`（含 proto）、上架時的配送方式驗證、公開讀取的賣家開通過濾與快取鍵裡的開通指紋；aftersales：刪 shipping client、退貨標籤重試（含 `RetryPrintReturnLabel` RPC 與 worker）、四個 shipping 事件 consumer 與 queue、退貨物流 provider 狀態聚合，`RETURN_REFUND` 決議回 `INVALID_PARAM`（只有已送達訂單能開售後，P3 期間不會發生），`return_shipping_method` 請求欄 reserved。product、aftersales `mvn compile` 綠 | order-service 未改（編譯紅）；其餘範圍全部未做；測試未跑 | 見下方「步驟 4 交接」 |
| 1 文件前置 | 完成 | 基線：`check-doc-paths.py` 綠；完成後 `rg -n '設計書 ?69' backend/docs docs` 只剩本計畫與附件 | 無 | 分支 `b3-d-docs` |
| 7 文件 | 程式完成待驗收 | 基線：`check-doc-paths.py`、`check-payuni-residue.py` 綠，物流商字眼 45 檔（不含 plans、ADR）；完成後兩支腳本綠，剩 16 檔，皆非物流商：`Kubernetes`、`Uber Fx` 子字串，PayUni 金流付款類型（7-ELEVEN ibon），CodeMeta 來源 md 與 `generate.sh`（物流值域歸步驟 4） | CodeMeta 4 份 md 與 `generate.sh` 的物流值等步驟 4 | 分支 `b3-d-docs` |


### 步驟 4 交接（2026-11-24）

order-service 的設計已定，下個 session 照做：

- 結帳：`CheckoutShippingFee`、`GetUberQuoteForCheckout`、`GetOrderForShipping` 三支 RPC 與其訊息刪除；`CheckoutValidate` 不回 `available_shipping_options`（`AvailableShippingOption` 刪除），保留溫層交集與整單錯誤（`ALL_SHIPPING_EXCEEDED` 刪）；preview／confirm 不收配送方式與超商欄位、運費固定 0、沒有免運判定，preview 回應的 `shipping_method`、`shipping_label`、`shipping_fee_payer`、免運兩欄刪除（欄號 reserved）。溫層欄位留給 B4／B6。
- `OrderPolicyResolver.resolve()` 不再收配送方式，一律 `SELLER_MANAGED`（賣家自寄）；confirm 的政策漂移比對隨之刪除。RFQ 建單同一支。
- 收件人姓名與地址長度改以 `orders` 欄寬（`recipient_name` 50、`shipping_address` 255）為上限，不再用 `PayUniShippingConstraints`；地址一律必填。
- `CreateOrderParams` 刪 `shippingMethod`、`feePayer`、`cvsStoreId／Name`、`shippingArea`、`senderCity／District`、`dimensionSum`；新訂單這些欄位寫 NULL（欄位可空，`chk_orders_fulfillment_mode_consistent` 對 NULL 成立）。DB 欄位本身留給 B8。
- 刪：`ShippingGrpcClient`、`ShippingUberQuoteAssessor`、`UberOrderEligibilityGuard`、`ShippingFeeCacheClient`、`ShippingFeeGrpcFetcher`、`ShipmentCreationBackstopScheduler`、`AutoCompleteScheduler`、`ShippingEventConsumer` 與其 queue、只服務它們的 `OrderShippingProgress`；`OrderGrpcService` 列表與詳情的物流欄（`shipping_status`、`label_*`、`partner_id`、`odno`、`validation_no`、`shipment_photo_keys`、`processing_deadline`）從 proto 刪除。
- 之後依計畫步驟 4 範圍繼續：invoice-service、notification（R3）、MQ topology、common-core 物流碼值、CodeMeta、compose 與 CI、`modules.go`／`lockorder.go`、種子、seller `MerId`／`GetSellerShippingProfile`、搬遷清單 disposition；`payment-service` outbox 測試與 `monitoring` 的 `hawkr.shipping`（`caller-inventory.md` §0 末）。每改一個 proto 都要跑 `scripts/gen-java-proto.sh`、`scripts/gen-go-proto.sh`（docker 裡的 protoc）、`make connect`。
- 步驟 5 要跟著刪 gateway 的 `RetryPrintReturnLabel`、`ReturnShippingMethod` 對應、`CalculateShippingOptions` 路由（`/app/shipping_quotes`）。
