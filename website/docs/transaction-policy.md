# Hawkr 交易後端實作快照（2026-07-02，非現行商務契約）

> **狀態**：本文件保留 2026-07-02 的後端查核結果，只用來辨識尚未完成的工程功能，不再作為費率、發票角色或簽約內容的權威來源。現行商務約定以 `docs/legal/hawkr-seller-platform-service-agreement-template.md` v1.2（三方案版本化）為準；三層發票流程以 `docs/planning/INVOICE_SETTLEMENT_DELIVERY_REFERENCE.md` 為準。
> **禁止誤用**：下列「後端尚未定義 6%」或「發票尚未實作」是實作落差，不會推翻已確認的三方案成交服務費（Starter 6%／Growth 3%／Pro 0%，另收方案月費 0／990／2,990）、金流手續費另計，或「統一合作法人開給賣家、賣家開給消費者、Hawkr 開給統一合作法人」的營運規則。本快照撰寫時只有單一 6% 費率，任何以 6% 為唯一費率的敘述均已被 v1.2 方案表取代。
> **用途**：作為產品與工程驗收的歷史事實依據。
> **資料來源**：`<org>/backend-payuni` @ `main`——詳細設計書 + Java 程式碼常數 + 運行設定（`application.yml`）交叉驗證。
> **核對日期**：2026-07-02。
> **驗收提醒**：後端幾乎所有時效數字都是 `${環境變數:預設值}` 形式，正式環境可能被環境變數覆寫。標「⚠️」者為文件與運行設定不一致或尚未實作的項目，發布前應由產品與工程以 production-like 測試自行確認。

---

## 🚨 簽約前必讀：三個與常見認知不符的後端事實

| # | 常見假設 | 後端實際 |
| --- | --- | --- |
| 1 | 平台抽 **6% 服務費** | **後端完全未定義任何平台抽成／6%**。搜 `6%`/`0.06`/`platformFee`/`serviceFee`/`分潤`/`抽成` 全 repo 0 命中。賣家實際只被 **PayUni 金流手續費 + 物流代收費**扣款。平台「結算分帳」功能**設計待規劃、尚未實作**。 |
| 2 | 商品發票賣家開、運費發票平台開 | **後端完全未實作任何發票功能**（無服務／RPC／欄位／載具）。發票開立主體在設計書**未定義**。 |
| 3 | 平台代收貨款後再撥款分帳 | **Phase 1：款項（含運費）直接進「賣家自己的 PayUni 商店帳戶」**。平台代理商代收分帳為**未來 Phase 2+**，尚未啟用。撥款入帳天期由 PayUni 依收款方式決定。 |

---

## 一、交易時效總表（快速查閱）

| 環節 | 政策數值 | 逾時後果 | 可信度 |
| --- | --- | --- | --- |
| **付款期限** | 建單後 **24 小時** | 自動取消、歸還庫存/券（`cancel_reason 9101`） | 高 |
| **出貨期限｜超商店到店** | 物流單建立 + **72 小時**（自然日，不順延） | 自動取消 + 原路退款（`9102`） | 高 |
| **出貨期限｜宅配（黑貓）** | **第 3 個工作日 23:59:59（台北）**（跳過週末/國定假日） | 自動取消 + 原路退款（`9102`） | 高 |
| **出貨期限｜Uber 即時配** | **不設期限** | — | 高 |
| **收貨後自動完成** | 送達後 **60 小時** → 觸發撥款 | 訂單轉 COMPLETED、發動撥款（售後爭議會暫停倒數） | 高 |
| **售後可申請窗口** | 送達後 **60 小時**內 | 逾時無法申請售後 | 高 |
| **賣家回覆售後期限** | **24 小時**（採運行值）⚠️ | 逾時系統**代為拒絕**（非自動同意），買家可申訴 | 中（見 §4.2） |
| **買家寄回期限** | 同意退貨起 **7 天** | 自動關案、**不退款、款項照撥賣家**（狀態 33） | 高（已上線） |
| **賣家確認退貨收貨** | 退貨送達後 **48 小時** | 自動確認並啟動退款（狀態 32） | 高 |
| **超商退貨到店待取** | 到店後 **72 小時** | 自動完成並啟動退款（狀態 31） | 高 |
| **申訴期限** | 被拒/關案後 **72 小時**內（限一次） | 逾時喪失申訴權 | 高 |
| **退款時效** | 付款完成後 **180 天**內（PayUni 限制） | 超過無法退款 | 高 |
| **撥款入帳｜信用卡** | PayUni 請款日 + **7 日** | — | 高（PayUni 費率表） |
| **撥款入帳｜ATM/超商代碼/先買後付** | 付款日 + **7 日** | — | 高（PayUni 費率表） |
| **撥款入帳｜超商取貨付款** | 成功取貨日 + **7 日** | — | 高（PayUni 費率表） |
| **撥款入帳｜黑貓貨到付款** | 已配達日 + **15 日** | — | 高（PayUni 費率表） |
| **平台服務費（抽成）** | **未定義／未實作** | — | 高（確定無） |
| **電子發票** | **未定義／未實作** | — | 高（確定無） |

---

## 二、訂單生命週期與時效

**狀態流**：`0 待付款 → 10 已付款 → 20 待出貨 → 30 配送中 →(32 待取貨/35 物流異常)→ 40 已送達 → 50 已完成`；旁支 `→ 80 已取消`、`→ 60 部分退款 / 70 全額退款`。終態＝ `80 取消`、`70 全額退款`。

### 2.1 付款期限——24 小時

- 未付款訂單建單後 **24 小時（86400 秒）** 自動取消，歸還庫存與釋放優惠券。
- `cancel_reason = 9101 PAYMENT_TIMEOUT`，`cancel_by_role = SYSTEM`；排程每 15 秒掃描。
- 出處：`03-訂單微服務設計書 §8.5`、`OrderConfig.java paymentDeadlineSeconds=86400`、`application.yml payment-deadline-seconds:${PAYMENT_DEADLINE_SECONDS:86400}`。
- 注意：這是「訂單」層級的 24h。PayUni 付款頁本身另有短效期（信用卡/Apple Pay 約 10 分鐘、ATM 約 2 天），僅代表**該次付款嘗試**過期，不等於訂單取消。

### 2.2 賣家出貨期限——依配送方式不同（**不是單一天數**）

| 配送方式 | 期限 | 計算 | 「已處理」定義 |
| --- | --- | --- | --- |
| 超商店到店（7-11） | **72 小時**（自然日） | 物流單 `createdAt + 72h`，**不順延假日** | `labelPrinted=true`（已觸發取號） |
| 宅配（黑貓 TAQBIN） | **第 3 個工作日 23:59:59 台北** | `nextBusinessDay(建單日, 3)`，**跳過週末/國定假日** | `tcatCalled=true`（已呼叫黑貓取件） |
| Uber 即時配 | **不設期限** | Uber 自動處理 | — |

- 逾期由排程每 5 分鐘掃描 → 訂單轉 `CANCELLED(80)`，`cancel_reason = 9102 SHIPMENT_TIMEOUT`，**原路退款**給買家。
- 起算點＝物流單建立時間（≈ 付款時間，物流單由 `order.paid` 事件毫秒級觸發）。
- **賣家無法自訂出貨期限**（系統固定值，無設定入口）。
- 出處：`09-售後微服務設計書 §5.9.1/§5.9.6`、`24-節假日管理設計書`、`OrderCancelReason.java`。
- 補充計時器——**建單兜底**：付款後若物流單一直建不起來，寬限 30 分、每 30 分重試、上限 5 次仍失敗 → 取消 + 退款（`cancel_reason 9103 SHIPMENT_CREATE_TIMEOUT`）。

### 2.3 收貨後自動完成——60 小時（撥款起點）

- 買家收貨（訂單轉 `DELIVERED(40)`）後 **60 小時**自動完成：`auto_complete_at = delivered_at + 60h`。計時起點是**送達**，非賣家出貨。
- 自動完成（或買家主動確認收貨）→ 訂單轉 `COMPLETED(50)` → **觸發撥款流程**（見 §3.1）。
- **售後/爭議會暫停倒數**：買家於 60h 內發起售後或申訴時，`countdown_paused_at` 記錄暫停時點，倒數停止＝款項不撥付；案件結束後補回剩餘時間再繼續。
- 出處：`03 §5.4/§5.5/§5.8`、`OrderConfig.java autoCompleteHours=60`、`application.yml auto-complete-hours:${AUTO_COMPLETE_HOURS:60}`。

---

## 三、款項、費用與撥款

### 3.1 撥款觸發與流程

- **觸發時點**：買家確認收貨（或 60h 自動完成）→ 訂單 `COMPLETED` → Order Service 排程 `GrantPaymentWorker`（每 30 秒掃 `grant_payment_tasks`）呼叫 Payment Service `GrantPayment(order_no)` → 呼叫 **PayUni Grant API** 發撥款指示。
- 代理商模式：所有交易建立時帶 `GrantExport=1`，PayUni **不自動撥款**，需由本系統（代理商）在買家確認收貨後主動發動。
- **系統端不做任何金額扣減**——送出 PayUni Grant API 的是**全額** `Amt`；平台不在此步驟抽成或算淨額。
- 出處：`05-支付服務設計書 §5.6`、`GrantPaymentWorker.java`、`payment.proto GrantPayment`。

### 3.2 實際入帳天期（由 PayUni 決定）

款項進賣家 **PayUni UNi 帳戶**的天期，由 PayUni 依收款方式決定（PayUni 官方費率表）：

| 收款方式 | 入帳天期 |
| --- | --- |
| 信用卡 | 請款日 + 7 日 |
| 超商代碼 / ATM 轉帳 / 先買後付（AFTEE） | 付款日 + 7 日 |
| 超商取貨付款 | 成功取貨日 + 7 日 |
| 黑貓宅配貨到付款 | 已配達日 + 15 日 |

- 「請款日」＝ PayUni 每日自動或賣方手動發動日；「付款日」＝ 買方完成付款日。
- DB 另有 per-物流方式的 `settlement_days`（7-11=D+7、黑貓=D+15），用途是**對齊/顯示** PayUni 天期，**不是**系統自行延遲撥款的排程參數。
- 出處：`PayUni 費用一覽.md`「撥款 UNi 帳戶」、`shipping V1__init 表 settlement_days`。
- ⚠️ 這些天期是 **PayUni 第三方作業**，非 Hawkr 可保證；合約如需明列，應以 PayUni 合約/後台為準。

### 3.3 平台服務費（抽成）——**未定義／未實作**

- 後端**沒有任何** Hawkr 平台抽成、服務費、分潤費率。**「6%」不存在於程式碼與設計書。**
- 賣家綁定 PayUni 商店時，系統**未帶入 `Fee`（費率）參數** → PayUni 套用其「平台預設」（該預設值是 Hawkr 與 PayUni 私下議定，**不在此 repo**）。
- `SellerSettlementEvent` 雖在事件目錄宣告 `platform_fee` 欄位，但**無公式、無值、無任何服務 publish**（空殼）。
- 對合約的意涵：**若要向賣家收「平台服務費 6%」，後端需先新增設計與實作**；目前系統不會扣這筆。

### 3.4 金流手續費（PayUni 收，賣家負擔）

賣家實際被扣的是 PayUni 的金流服務手續費（單筆計收、不足 1 元以 1 元計）：

| 付款方式 | 手續費 |
| --- | --- |
| 信用卡（國內卡） | 2.8% |
| 信用卡（國外卡） | 3.5% |
| 信用卡分期 | 3 期 2.9% / 6 期 3.5% / 12 期 6.5% / 24 期 11%（期數越高越高） |
| ATM 轉帳（單繳帳號） | 1%（上限 15 元） |
| 超商代碼（ibon） | 25 元／筆 |
| 電子錢包 | icash Pay 2.5% / LINE Pay 0.2% / 街口 2.5% / AFTEE 2.5% |
| 退款手續費 | 超商代碼/ATM 各 10 元 |
| 提領匯款手續費 | 15 元（每月前 5 次免） |

- 負擔方：賣家（自 UNi 帳戶扣）。出處：`PayUni 費用一覽.md`「金流服務費用」。

### 3.5 物流代收手續費（貨到付款 COD，PayUni 收）

| 配送方式 | 代收手續費 |
| --- | --- |
| 7-11 店到店 | 0.5%／筆（不足 1 元以 1 元計） |
| 黑貓宅配 | 1%／筆（不足 20 元以 20 元計） |

- 出處：`shipping V1__init shipping_method_configs.commission_rate`（seed 0.0050 / 0.0100）。這是 COD 代收金流費，**與「誰付運費（fee_payer）」是兩回事**。

### 3.6 賣家淨額 / 結算明細——**未實作**

- 後端**沒有** `sellerNetAmount` / `platformFee` 計算邏輯；撥款是全額。
- 「賣家結算（含分帳）」與 `payout.completed` 撥款完成通知：**設計待規劃、無 publisher、尚未實作**（`36-MQ 與事件總覽 §9.7`）。
- 對合約的意涵：目前**沒有平台端對帳明細/淨額計算**；賣家看到的金流細節＝ PayUni 帳戶對帳。

---

## 四、售後 / 退款政策

> 售後為獨立於訂單主流程的一套計時器（互不換算）。所有狀態碼見 `09-售後微服務設計書`。

### 4.1 售後可申請窗口——送達後 60 小時

- 買家須在 `orders.delivered_at + 60 小時`內申請售後，逾時無法申請。
- 出處：`09 §1.5 R2 / §2.6`、`application.yml apply-deadline-hours:${…:60}`。

### 4.2 賣家回覆售後期限——24 小時 ⚠️

- **採運行值 24 小時**（`application.yml process-deadline-hours: 24`）。⚠️ 設計書與程式碼預設寫 **72 小時**，兩者不一致——**以正式環境運行值為準，簽約前請與後端最終確認**。
- 逾時後果：系統**代為拒絕**（狀態 → `REJECTED(60)`、`reject_reason = SELLER_NO_RESPONSE`、actor = SYSTEM），**非自動同意**；買家可走申訴。（可切換備援策略 `AUTO_ESCALATE` = 升級 Admin 仲裁。）
- 出處：`09 §2.6`、`AfterSalesProperties.java processDeadlineHours=72` vs `application.yml:24`。

### 4.3 買家寄回期限——7 天（已上線）

- 賣家同意退貨進入 `RETURN_PENDING(20)` 起 **7 天**（`ship_back_deadline_at`）內須寄回；剩 24h 一次預警。
- 逾時 → 自動關案 `AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)`，**退款資格解除、訂單款項照常撥付賣家**（不退款終態，不建 refund_task）。
- 買家要拿回錢須走申訴（見 §4.5）。政策 2026-06-15 由「30 天僅升級」改為「7 天自動關案」，**已上線 main**。
- 出處：`09 §2.6/§5.3.8`、`application.yml return.timeout-days:${…:7}`、`ReturnTimeoutWorker.java`。

### 4.4 賣家確認退貨收貨——48 小時 / 超商到店 72 小時

- 宅配退貨或超商賣家已取貨（`RETURN_DELIVERED(27)`）：**48 小時**未確認 → 自動確認並啟動退款（狀態 32）。
- 超商退貨到店待取（`RETURN_AT_STORE(26)`，僅超商）：**72 小時**未取 → 自動完成並啟動退款（狀態 31）。
- 賣家可主動驗貨確認並下修退款金額（驗貨扣款）；但逾時仍會系統自動確認退款。
- 出處：`09 §2.6`、`application.yml auto-confirm-receipt-hours:48 / store-pickup-timeout-hours:72`。

### 4.5 申訴 / 平台仲裁——72 小時

- 買家被拒（`REJECTED(60)`）或待寄回逾時關案（33）後，可在 **72 小時**內提申訴（**一案限一次**），由 Admin 裁決三選一：`FULL_REFUND / PARTIAL_REFUND / REJECT_APPEAL`。
- 起算：60 → `last_rejected_at + 72h`；33 → `closed_at + 72h`。申訴期間案件凍結（擋所有改寫）。**被拒後申訴是唯一救濟（reopen 已淘汰）**。
- ⚠️ **Admin 裁決本身無 SLA**（設計書未定義平台須在幾天內裁決）。
- 出處：`09 §10`、`AfterSalesProperties AppealConfig.deadlineHours=72`。

### 4.6 退款金流

- **原路退回 PayUni**：信用卡即時退款；ATM 非即時（買家須至 PayUni 頁面填銀行帳號，靠 webhook 推進）。
- **退款上限＝買家實付** `total_actual_amount`（＝商品小計 + 運費 − 折扣）；優惠券折抵不退還；超額 PayUni 拒退 `REFUND_AMOUNT_EXCEEDED`。
- **退款時效**：付款完成後 **180 天**內（PayUni 限制）。
- **撥款保護**：透過「暫停訂單 60h 自動完成倒數」達成——爭議中訂單尚未進到撥款步驟，故款項不會先撥給賣家。**非**「已排定撥款再凍結」的顯式機制。
- 撥款前退款：走 PayUni「取消授權」（比退款快、無手續費）。
- 出處：`09 §2.5/§2.6/§4.4/§5.4`、`05-支付服務設計書 §5`。

---

## 五、運費政策

### 5.1 收款主體

- **Phase 1（現行）**：運費併入訂單總額，由**賣家自己的 PayUni 商店（`payuni_mer_id`）直接收款**。
- **Phase 2+（未來）**：平台代理商統一收款後再分帳（`PAYUNI_AGENT_MER_ID`），**尚未啟用**。
- 金額公式：`total_actual_amount = 商品小計 − 折扣 + 運費（若買方負擔）`。
- 出處：`20-結帳模組設計書 §4.13/§4.10`。

### 5.2 誰付運費（fee_payer）與免運

- `feePayer` 存於**每個 SKU 的 `shipping_method` JSONB**：`"3101" = 買家付`（預設）、`"3102" = 賣家付（＝免運）`。
- 免運規則：`feePayer=SELLER(3102)` 是免運前提；`freeThreshold<=0` ＝無條件免運，`freeThreshold>0` ＝門檻免運（依優惠券分攤後單品實付判定）。任一商品達標 → 整筆配送免運。**平台無「免運券」概念，免運＝賣家上架時勾「我負擔運費」。**
- 出處：`20 §4.7/§4.7.3/§4.9`（依 issue #1147 產品裁定）。

### 5.3 運費費率

- **三維費率**：`配送方式 × 溫層 × 配送區域` 決定級距，全部存於 **Shipping Service DB**（`shipping_fee_tiers` × `shipping_method_configs`），含材積/重量上限、撥款天期、代收手續費率。
- 出處：`21-運費計算模組設計書 §0.2`。

---

## 六、發票——**後端未實作**

- 後端**完全沒有**電子發票功能：無 invoice 服務／RPC／DTO 欄位／DB migration；付款請求從不帶發票載具（`CarrierType`）。
- `InvoiceIssuedEvent` / `BuyerInvoiceNotifyEvent` 僅在事件目錄宣告，**無 producer／consumer**，Payment Service 發布事件清單（§4.4）不含發票事件。
- **商品發票、運費發票的開立主體在設計書未定義**；賣家在後端也無稅籍/發票字軌欄位。
- **退款作廢／折讓（credit note）後端亦無**任何邏輯。
- 對合約的意涵：發票相關條款目前**無系統支撐**，須註明「以正式上線公告／賣家自行依法開立為準」，或先排開發。
- 出處：`05-支付服務設計書 §1.1/§4.4`、`payment.proto`、`CreatePaymentRequest.java`（皆無發票欄位）。

---

## 七、簽約前務必確認清單（Action Items）

1. **平台服務費**：是否要向賣家收（例如 6%）？若要，**後端須新增設計與實作**（目前完全沒有）。合約費率＝商業決策，非現行系統行為。
2. **賣家回覆售後期限**：正式環境跑 **24h（yml）** 還是 **72h（doc/code 預設）**？兩者打架，需後端一槌定音。
3. **所有 `${ENV:default}` 時效**：確認 production 是否有 env var 覆寫（付款 24h、出貨 72h/3工作日、自動完成 60h、寄回 7 天、確認收貨 48h/72h、申訴 72h）。
4. **發票**：合約如含發票條款，須確認開發時程或改為「以正式公告為準」。
5. **撥款入帳天期**：信用卡 D+7、黑貓貨到付款 D+15 等屬 PayUni 作業，合約引用須對齊 PayUni 現行費率表/合約。
6. **金流/物流手續費費率**：PayUni「平台預設費率」不在本 repo，須向 PayUni 合約確認賣家實際被扣費率。
7. **運費/貨款收款主體**：現行 Phase 1 = 賣家自己 PayUni 帳戶收款；若合約假設平台代收分帳，需說明那是 Phase 2 未來式。

---

## 附錄：資料來源檔案（backend-payuni @ main）

**設計書**
- `docs/詳細設計書/03-訂單微服務（Order Service）詳細設計書.md`——訂單狀態機、付款期限、自動完成、撥款觸發
- `docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md`——撥款、退款、發票（未實作）
- `docs/詳細設計書/09-售後微服務（AfterSales Service）詳細設計書.md`——出貨期限、售後全時效、退款金流
- `docs/詳細設計書/20-結帳模組（Checkout Module）詳細設計書.md`——運費收款主體、fee_payer、免運
- `docs/詳細設計書/21-運費計算模組（Shipping Fee Module）詳細設計書.md`——三維運費費率、代收手續費
- `docs/詳細設計書/24-節假日管理（Holiday Management）詳細設計書.md`——宅配出貨期限工作日順延
- `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md`——結算/撥款完成事件（待規劃）
- `docs/第三方整合/PayUni 費用一覽.md`——金流手續費、撥款入帳天期

**程式碼 / 設定**
- `services/java/order-service/.../config/OrderConfig.java`、`application.yml`——付款 24h、自動完成 60h、建單兜底
- `services/java/order-service/.../worker/GrantPaymentWorker.java`——撥款觸發
- `services/java/aftersales-service/.../config/AfterSalesProperties.java`、`application.yml`——售後全時效
- `services/java/aftersales-service/.../scheduler/ReturnTimeoutWorker.java`——7 天寄回自動關案
- `services/java/shared/common-core/.../code/OrderCancelReason.java`——取消原因碼 9101/9102/9103
- `services/seller-service/internal/client/payuni/client.go`——綁定 PayUni（未帶抽成費率）
- `shared/events/events.json`——SellerSettlementEvent / InvoiceIssuedEvent（空殼、未實作）
