# 電子發票服務（invoice-service）架構決策

| 項目 | 值 |
|---|---|
| 狀態 | accepted；ADR-01 superseded by `2026-09-24-go-modular-monolith.md`（獨立服務改為獨立模組，憑證邊界列為接受的代價）。遷移計畫 A5 落地前，現行程式仍依本檔運作 |
| 日期 | 2026-09-18 |
| 影響範圍 | `services/invoice-service`、`api/proto/invoice`、order／payment／aftersales／shipping／notification 的介接面 |
| 對應規格 | `docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md` |

> 本檔保存 `WHY THIS DESIGN` / `WHY NOT THE ALTERNATIVES`。
> **逐欄規格、端點簽章、狀態值域一律在 58 號詳細設計書**，本檔不複製。
> 58 號 §1.5 保留「決策編號 → 採用結論」的索引，`ADR-NN` 與 `D-NN` 的編號以該表為準。

---

## 脈絡

電子發票要解的問題是：**買家付款成功的那一刻，賣家（而不是平台）就負有開立統一發票的法定義務；
平台必須替每一位賣家、用他自己的統編與 AppKey、在法定時限內把這張憑證開出去，
並在退款發生時自動作廢或折讓。**

三個彼此獨立的硬約束共同決定了架構形狀：

1. **法定時限是硬的**：B2C 開立後 2 日內、B2B 7 日內必須完成上傳，逾時由國稅局對營業人（＝賣家）
   記點或罰款。⇒ 任何把開票排進非同步佇列的設計，都必須有「距離逾期剩幾天」的量測與人工介入路徑，
   不能只有重試次數。
2. **開票人是賣家不是平台**：每個賣家一組統編 ＋ AppKey。⇒ 平台必須持有 N 份可開立稅務憑證的密鑰，
   這是本專案最高等級的機敏資料。
3. **上游沒有任何可靠性保證**：AMEGO 無 webhook、無冪等鍵、無重試策略、無 SLA、無 rate limit 數字、
   維護窗以 4 小時為級距、HTTP 一律回 200。⇒ 可靠性必須 100% 由我方承擔。

---

## ADR-01 服務落點：獨立微服務

**決策**：`invoice-service` 是獨立 Go 微服務，獨立 DB `hawkr_invoice`。

**不採用**：掛 order-service／掛 payment-service／擴充 payuni-webhook-service。

| | 掛進既有服務會發生什麼 | 獨立服務怎麼解 |
|---|---|---|
| **保存期與 migration 節奏** | 發票憑證法定保存 5 年、且列為 PII 清理**例外**；訂單／金流資料有各自不同的保留與清理規則。共用 schema ⇒ 一次 migration 同時動到兩種保存義務的資料 | 獨立 DB，清理排程與保留期自成一套 |
| **憑證邊界** | per-seller AppKey 一旦放進 order-service，該 process 內**任何一條 code path**（含結帳 hot path 的所有 caller）都在同一個記憶體空間握有它 | 「誰能解密 AppKey」＝「誰能部署 invoice-service」 |
| **故障隔離** | AMEGO 單一上游、維護窗 4 小時級、無 SLA。放進 order/payment ⇒ 第三方中斷直接進到結帳 hot path | 最壞情況是「發票延後開立」（有法定時限的告警在守），不是「結帳掛掉」 |
| **多副本排程搶佔** | order-service／payment-service prod 是 2 副本。多加一個排程器＝要把那些服務既有的租約矩陣重新論證一次 | invoice-service 從第一版就是單副本 ＋ lease claim，租約算式只有自己這一組要顧 |
| **責任語意** | 「這筆訂單有沒有開票」若是 order 的欄位，order 就成了發票狀態的第二個 SSOT | 發票文件狀態的唯一 SSOT 是 `invoice_documents`；order 只提供快照來源 |

**明確被否決的兩個替代位置**：

- **擴充 `payuni-webhook-service` 的 provider 維度**：它的 `webhook_inbox` 唯一鍵已含 provider，
  技術上塞得進去。否決理由是 **AMEGO 根本沒有 webhook**——把一個沒有回呼的整合放進一個叫 webhook
  的服務，是用服務名說謊。
- **讓 PayUni 在建立交易時代開發票**（PayUni 的 `TradeInvoice` 支付工具旗標）：這條路我方不持有
  AppKey、開票時機綁在交易當下，且 PayUni 代開是用**平台**的發票資格開，而發票人必須是賣家。

---

## ADR-02 服務語言：Go

**決策**：Go（fx ／ sqlc ／ Atlas ／ pgx）。**不採用**：Java（Spring Boot）。

選 Go 是為了買下四樣現成的東西 ＋ 一道 Java 側不存在的 CI 保護：

| # | Go 側現成 | Java 側現況 | 對發票的具體意義 |
|---|---|---|---|
| 1 | **MQ consumer ↔ topology 登記一致性 gate**（`scripts/mq-consumer-registration-check.py`，雙向檢查） | **無任何 CI 攔得住**（該 gate 只掃 `**/*.go`） | 服務端已 passive declare ⇒ 漏登記 topology 條目時不 crash、不進 DLQ、該訂閱靜默失效。而發票的失效症狀是「漏開」——**漏開不會有人發現，直到客戶要報稅** |
| 2 | `pkg/outbox` 泛型 lease worker（`Repository[T]` 五方法契約、逐筆取租約基準、scheduler 內建 panic recovery） | 各服務自寫 task worker | 《多副本排程搶佔規範》合規的搶佔閘門直接沿用，不必重新論證 |
| 3 | `pkg/crypto` AES-256-GCM，**fail-closed**（空 secret 直接 `ErrEmptySecret`，永不退化成明文） | 無共用欄位級加密元件 ⇒ 選 Java 要先把 AES-GCM 抽進 `common-core` | AppKey 加密存 DB 是本專案的前提，不是選配 |
| 4 | `pkg/mq.NewResilientConsumer` 的 retry-count ＋ DLX 已完整 wire | 各服務自行組 `manualAckContainerFactory` | 入向事件的重試與死信零手工 |
| 5 | `pkg/tsid.NewGenerator` **無 Node ID 概念** | 必須挑一個未用的 `node-id-base` | 少一個全域唯一資源要管 |

**接受的代價**：order／aftersales／payment 三個上游都是 Java，跨語言呼叫走 gRPC（本來就是 gRPC，
無額外成本）；但 Java 側的 `CodeMetaValidator`（寫 DB 前強制驗碼）在 Go 側對應物是
`pkg/errors/bizcode` ＋ 手寫 validator，形狀不同 ⇒ 「對應 CodeMeta 的輸入寫入 DB 前必須走共用
validator」這條規則在 Go 側要自己落實（該義務寫在 58 號 §4.1.2）。

**改用 Java 的隱藏成本**（四項同時要補）：Java 側 topology 登記的人工 checklist、
Java 側 AES-GCM 共用元件（抽進 `common-core`）、TSID `node-id-base` 的登記、Java 版 lease worker。

---

## ADR-03～ADR-36 決策與被否決的方案

| # | 決策 | 採用 | 不採用 | 否決理由／代價 |
|---|---|---|---|---|
| ADR-03 | 開票時機 | 付款成功 | 出貨／確認收貨／售後期滿／撥款後 | 法定時限自付款起算，晚於付款的任何錨點都會壓縮到逾期窗口 |
| ADR-04 | 發票模型 | 通用 issuer → recipient | 綁死賣家 → 買家 | 綁死後四方向（平台→賣家、賣家→買家…）無法表達 |
| ADR-05 | provider 串接 | 自建 AMEGO client | PayUni `TradeInvoice` 代開 | 走 PayUni ⇒ 發票綁在「一次付款」上、我方不持有 AppKey、無法表達「賣家自行開票」與「免用統一發票」、無法做 ADM 手動開票；退款的作廢／折讓判定會落在 PayUni 而非我方，5 年保存與合意證據無處可放。等於放棄通用模型 |
| ADR-06 | 上傳結果取得 | 主動輪詢 | 建 `POST /webhooks/amego` | AMEGO 無 webhook／callback，端點建了也沒有人呼叫 |
| ADR-07 | 冪等記錄落點 | PostgreSQL | Redis／兩者並用 | Redis 無容量預算且逐出會讓冪等記錄靜默消失 |
| ADR-08 | ADM 授權 | 既有 `requireAdmin` ＋ 強制填理由 ＋ 二次確認 ＋ 完整稽核 | 細粒度 capability／代行封閉九值 | 塞進代行封閉九值會撞兩個 DB CHECK ＋ boot 斷言；另蓋第二套授權層成本高且與 56 號規劃搶跑 |
| ADR-09 | 發票端點 namespace | 一般 `/api/v1/admin/*` | 進 ADM 代行 namespace／修改代行禁字清單 | 同 ADR-08 |
| ADR-10 | 稅別來源 | 賣家層級預設值 ＋ 可抽換 resolver | 商品名判定／SKU 加欄／ADM 逐單填 | 依商品名自動判定免稅會產生法律責任；SKU 加欄要動全平台商品模型 |
| ADR-11 | 通知管道 | Phase 1：`MailSender` 介面 ＋ `LogMailSender`；Phase 2 接雲端 email 服務 | 光貿代寄／自建 SMTP／只做站內／Phase 1 就接雲端供應商 | AMEGO 的信只涵蓋一種模式、測試環境不寄、重寄無 API；自建 SMTP 要自己承擔投遞率；Phase 1 就接雲端供應商會把依賴治理三件套（用量／上限 series、兩條告警、預算數字）拉進第一版 |
| ADR-12 | 執行模型 | due-date driven（事件建工作、排程推進狀態） | consumer 直接打 AMEGO | consumer 直接打上游 ⇒ 沒有「距離逾期剩幾天」的量測面，失敗只剩重試次數 |
| ADR-13 | 狀態建模 | 三軸分離 | 單一 `status` 欄／二軸 | 憑證狀態、provider 動作狀態、財政部上傳狀態三者的改寫者不同，合併會讓一方覆蓋另一方 |
| ADR-14 | 資料一致性維護 | metric ＋ 告警 ＋ 維運 SQL | reconciliation job／drift scanner／backfill CLI | 《後端服務責任邊界規範》禁止把補償任務基礎設施寫進 application code |
| ADR-15 | 發票配號 | `f0401` 自動配號 | `f0401_custom` 我方配號 | ① 順時順號是硬約束，自行配號時我方要保證號碼分配與時間單調一致（併發下很難）；② `track_get` 沒有冪等鍵，網路中斷時無法安全重試、可能永久漏掉一個號段，而號段作廢會造成財政部端字軌不連續。改採自行配號須先解決併發下的號碼單調分配，並為 `track_get` 設計「逾時後靠 `track_status` 的 `now` 比對」的補救 |
| ADR-16 | AMEGO 呼叫粒度 | 一次一張 ＋ 我方批次層 | AMEGO 批次多張 | 批次介面的部分失敗無法逐張歸因 |
| ADR-17 | 故障備援 | 受控人工出口（ADM 查證 ＋ 雙人確認後解鎖） | 完全不設人工備援／系統自動轉人工 | 《統一發票使用辦法》§24-1 要求「無法開立時依規定改以人工開立……必須另有經確認的故障應變程序」。改為完全不設人工備援須先由會計師、國稅局或 AMEGO 書面確認；系統自動轉人工會在結果不明時造成重複開票 |
| ADR-18 | AppKey 保護 | 單層 AES-256-GCM ＋ current／previous 兩版本 | KMS／envelope encryption／不加密 | 見下方「AppKey 加密的選型」 |
| ADR-19 | 退款處理 | 毛額開立 ＋ 作廢／折讓 | 開淨額（剩餘快照） | ① 金額守恆式的等號右邊是「賣家開給買家的發票／收據總額」；② 法規要求保存退款合意證據，開淨額則無折讓憑證、留不下退款軌跡；③ 法制已為「先開票、退貨再折讓」提供完整工具。改開淨額需重寫守恆式、重新設計退款軌跡的保存方式，並回答「買家同意訊息保存在哪裡」 |
| ADR-20 | 優惠券歸屬 | 一律賣家負擔 ＋ 平台出資券 fail-closed 轉 ADM | 新增 `funded_by` 欄／一律賣家負擔而無兜底 | 真要補貼，守恆式必須先改寫成「賣家銷售額 ＝ 買家實付 ＋ 平台補貼」，且要同時設計補貼回沖的金流——不是加一個欄位就好 |
| ADR-21 | 零元訂單 | 不開票，訂單詳情標示原因 | 開 0 元發票 | 同業做法一致；AMEGO 對 `TotalAmount = 0` 是否合法沒有正面敘述。若改為要開，需先實開一張 0 元測試發票確認 AMEGO 接受 |
| ADR-22 | 載具 | 買家未主動選擇即開無載具發票，平台不代配 | 拿買家手機／email 去開光貿 `amego` 載具／申請 Hawkr 專屬會員載具 | 借光貿載具沒有解決兩件事：買家不是光貿會員 ⇒ 中獎通知無人負責、歸戶責任不明；且載具發票中獎前拿不到 PDF ⇒ 等於用「可能拿不到的中獎通知」換掉「確定拿得到的發票檔案」。要提供平台代管載具，正確形態是明確 opt-in ＋ 告知識別碼來源，且須先解決中獎通知與歸戶責任 |
| ADR-23 | 副本與搶佔 | 單副本 ＋ lease claim | 單副本無 claim／多副本 | 無 claim 在調高副本數那一刻引爆；多副本要先把租約矩陣全部論證過 |
| ADR-24 | 表名 | `invoice_*` ＋ provider 欄 | `amego_*`／`seller_invoice_*` | provider 與 issuer 皆須中立，表名綁 provider 等於把換 provider 變成 migration |
| ADR-25 | 賣家能力 | 新增**兩個** capability | 零個（沿用既有）／一個 | 不放行「稅務身分／補件」與「自行開票責任條款」會死結：稅務逾期本身導致停權，而解除停權的唯一路徑是回去補件 |
| ADR-26 | 接單上限執行點 | 結帳鏈既有收斂點回 `9326` | 改 `can_accept_orders`／走治理停權 | `can_accept_orders` 的既有語意只吃憑證與賣家狀態，塞入發票語意會讓兩個領域共用一個布林 |
| ADR-27 | 稅務資格 SSOT | invoice-service（`seller_tax_profiles`） | seller-service | 稅務資格的生命週期與發票開立綁定，放 seller-service 會產生第二個 SSOT |
| ADR-28 | proto 型別與共用 | 字串 ＋ CodeMeta；Request 獨立、Response 形狀完全相同才共用 | proto 定義 enum；Response 全拆／全共用 | proto enum 與 CodeMeta 會成為兩份碼表；Response 全共用則一方加欄位會外溢 |
| ADR-29 | 錯誤碼號段 | `9300~9399`；CodeMeta Value 段 `3801~3899` 保留不登記 | `2001~2099`／`6001~6099`／`9290~9299`／`9701~9999` | `2001~2099` 人眼會與既有號段撞車；`9290~9299` 已留給代行營運擴段（禁用） |
| ADR-30 | 保存期 | 5 年，列為 PII 清理與 media GC 例外 | 沿用既有清理規則 | GC 是不可逆刪除（媒體 7 天、PII T+30），等外部確認回來資料已經沒了。縮短保存期需法務確認，且要處理「已經被例外化保留的資料如何合法刪除」 |
| ADR-31 | 折讓合意 | 退貨／退款申請即合意；賣家單方下修另給異議期 | 每次折讓前逐次確認／不需再確認 | 單方下修時「買家申請 A 金額、實際折讓 B 金額」用「申請即合意」說不過去。改為每次折讓前都要買家點擊確認會拉長退款鏈；改為不需再確認則被查核時無法舉證。任一方向都需要法務意見 |
| ADR-32 | Phase 1 範圍 | 逐項分流：月結匯出與賣家申請作廢移出、S05 補做 | 全做／全不做 | 前兩者是「有 API 沒畫面」的半成品，而作廢折讓已全自動、賣家本來就不需要那個入口；S05 相反，它是 `PLATFORM_RECEIPT` 模式的授權取得入口，沒有它整條免用發票線走不通 |
| ADR-33 | IP 白名單 | 兩模式建構子，Phase 1 走直連 | 只寫直連／只走 egress-proxy | 只寫直連 ⇒ 對方啟用白名單那天要改 code；只走 egress-proxy ⇒ 為一個未確認的約束付固定 IP 的維運成本 |
| ADR-34 | AMEGO 配額治理 | 自訂 5 QPS 並當成 `max` series | 等對方給數字／不接監控 | 《硬上限依賴接入治理規範》禁止先接上去再補監控 |
| ADR-35 | 和單 | 賣家已進 `AMEGO_AUTO` 時對 `consolidated=true` 拒絕 | 不處理／全域禁用和單 | 不處理 ⇒ 一張發票跨多賣家、開票人無法決定；全域禁用會傷到未啟用發票的賣家 |
| ADR-36 | 治理期限起算點 | 自賣家**送出審核申請**起算 | 自接受條款起算／自認證核准起算 | 自「接受條款」起算會讓期限與接單限制被賣家自己的節奏牽著走。⚠️ 主路徑上兩個錨點同時發生、看起來等價，**但不得據此改錨**：補簽路徑仍會在事後改寫 `attested_at`，改錨等於把起算點交還給賣家 |

### 論證寫在規格正文的決策（本檔只記被否決的那一半）

| 決策 | 採用 | 不採用 | 否決理由 |
|---|---|---|---|
| Port 分配 | `10040` HTTP／`10041` gRPC／`10042` 檔案串流 | Java 頻段 `9952/9953`；`9928/9929`；PDF 掛回 `10040` 共用 router | `9928/9929` 已被 inventory-service 佔用；PDF 共用 router 會讓串流與一般 API 搶同一組逾時設定 |
| MQ routing key | `order.paid` | `order.paid.v1` | 該 routing key 在 broker 上不存在，綁了靜默收不到訊息 |
| 入向 queue | `invoice.order.paid`、`invoice.order.refund.finalized`、`invoice.order.cancelled` 三條 | 綁 `order.refunded` | 此 routing key 不存在 |
| proto `package` | `hawkr.invoice` | 照抄 `payment.proto` 的 `io.github.will413028.hawkr.payment.grpc` | 違反 Proto 管理規範的 package 命名 |
| 發票 PDF 取得 | objectKey ＋ 授權下載 | 公開 CDN URL | 發票是稅務憑證，公開 URL 等於無授權外流 |
| Tier A 越權回應 | 404，與「不存在」不可區分 | 403 | 403 等於確認訂單存在，可枚舉他人訂單號 |
| 發票日期取值 | `COALESCE(provider_paid_at, pay_time)`，期別以 UTC+8 雙月制換算 | 只取 `pay_time`；在 UTC 算期別 | `provider_paid_at` 是 PayUni 認為的真實付款時間；在 UTC 算期別會在跨期邊界把單據算進錯誤期別。改取其他時間需重新論證跨期邊界行為，並補邊界日測試 |
| 表頭稅別 | 一律 `DetailVat=1`（含 B2B），明細帶含稅價 | B2B 走未稅明細 | 兩套明細語意會讓尾差出現在兩個不同的地方 |
| 折扣分攤 | 最大餘數法、全整數、先分攤再算稅 | 浮點分攤；先算稅再分攤 | 浮點違反金額規範；先算稅再分攤會讓尾差散在每一行，而先分攤再算稅的尾差只有一處、且由 AMEGO 用同一條式子復現 |

---

## AppKey 加密的選型（ADR-18 展開）

**決策**：k8s Secret 持有 master key ＋ `pkg/crypto` AES-256-GCM ＋ current／previous 兩個 key 版本。

| 不採用 | 理由 |
|---|---|
| KMS | repo 內沒有任何 KMS／Vault 基礎設施，為一個服務引入等於新增一條有配額與帳單的硬上限依賴 |
| per-record DEK（envelope encryption） | 金鑰輪替的複雜度換不到對應的威脅模型改善——能讀 DB 的攻擊者在本架構下同時要能讀 k8s Secret |
| 不加密 | AppKey 是可開立稅務憑證的密鑰，等同賣家的稅務身分 |

**衍生的三個選擇**：

- **輪替時不直接覆蓋**：舊值保留為 previous 版本，讓「輪替期間仍在途的請求」與「輪替後才發現新
  key 錯誤」兩種情況都有回退路徑，且輪替本身不是破壞性操作。
- **fingerprint 含統編而不是裸 `app_key`**：裸值的 fingerprint 在兩個賣家共用同一把 key 時會碰撞，
  而那正是要偵測的異常。
- **fingerprint 用 HMAC 不是裸 SHA-256**：裸雜湊對低熵輸入（統編是 8 位數字）可被窮舉反推；
  lookup key 與 master key 分開，讓「能做等值查詢」不蘊含「能解密」。

---

## D 系列決策的隱藏成本

58 號 §3.7 保留 `D1`–`D17` 的**現行約束**；以下只記「改動它要付什麼代價」中屬 alternatives 的部分。

| # | 改動它的代價 |
|---|---|
| D1（Go） | 見 ADR-02 的四項隱藏成本 |
| D2（自串 AMEGO） | 等於放棄通用發票模型與 In-scope 的四項能力 |
| D3（`f0401` 自動配號） | 見 ADR-15 |
| D4（發票日期取值） | 需重新論證跨期邊界行為，並補邊界日測試（至少 UTC 15:59 vs 16:00 各一、單月 1 日與雙月最後一日各一） |
| D5（零元不開票） | 若改為要開，需先實開一張 0 元測試發票確認 AMEGO 接受 |
| D6（不代配載具） | 要提供平台代管載具，正確形態是明確 opt-in ＋ 告知識別碼來源，且須先解決中獎通知與歸戶責任 |
| D7（不申請專屬載具） | 需求書要「預設會員載具」，其意圖是「買家用會員身分就看得到自己的發票」；本設計以發票中心滿足該意圖（跨訂單、看得到 PDF、下載得了、通知信夾帶）。改回載具要補申請流程 ＋ 中獎通知與歸戶的完整設計 |
| D8（毛額開立） | 見 ADR-19 |
| D9（券一律賣家負擔） | 見 ADR-20 |
| D9a（平台出資券 fail-closed） | 拿掉這道檢查必須先有「平台補貼回沖賣家」的完整金流與改寫後的守恆式，不能只把 if 刪掉 |
| D10（保存 5 年） | 縮短保存期需法務確認；且要處理「已經被例外化保留的資料如何合法刪除」 |
| D11（沿用 `requireAdmin`） | 要在 `gateway/internal/authz` 開第三個 registry（與代行封閉九值完全隔離），成本高且會與 56 號的規劃搶跑 |
| D12（Phase 1 只做寄信介面） | Phase 2 接真 provider 時只換 `MailSender` 的實作與 `MAIL_PROVIDER` 的值；屆時才需要跑依賴治理三件套與退信處理設計 |
| D13（折讓合意） | 見 ADR-31 |
| D14（受控人工出口） | 見 ADR-17；改為「強制回報否則視為違規」需要新的違規類型與申訴路徑 |
| D15（治理參數） | 全部是可調參數，改值只需改設定；但改變起算點的語意要重新論證漏洞（見 ADR-36） |
| D16（賣家能力層） | 見 ADR-25 |
| D17（Phase 1 範圍分流） | 見 ADR-32 |

---

## 後果

- 平台承擔 AMEGO 的全部可靠性缺口（無 webhook、無冪等鍵、無 SLA），對應成本是一整套
  due-date driven 的排程、量測與人工出口設計。
- 持有 N 份賣家 AppKey 使 invoice-service 成為平台機敏等級最高的服務，部署權限即解密權限。
- 「發票人是賣家」這條約束否決了所有由平台代開的捷徑（PayUni 代開、平台載具、平台統編），
  任何未來想走回捷徑的提案，要先推翻的是這一條而不是個別 ADR。
