# 物流財務事實的 domain ownership：shipping 發出成本事實，Logistics Finance 持有債權

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-11 |
| 影響範圍 | shipping-service／payment-service 財務事實 ownership |

- **脈絡**：`2026-09-11-uber-commerce-model-seller-direct.md` 把商流改成 `SELLER_DIRECT` 之後，新增了兩條資金線（線 B 平台→賣家收物流服務費、
  線 C 平台↔Uber 月結），而 66 詳設 §1.6.1 未指定它們的落點。
  本 ADR 之前，以下 financial fact 沒有任何一份文件指定 authority：
  賣家物流應收／provider 應付／provider 帳單／true-up／賣家貸記／收款通道對帳／帳本分錄。
  🔴 **每一份 financial fact 只能有一個 authority**——兩個 owner 都認為自己是權威，結果是對不起帳。
- **決定：按「事實 vs 債權」畫界，三個 authority 各司其職。**

⚠️ **authority 欄寫的是「可部署單元 ＋ module」**；[70 號](70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89詳細設計書.md)
以 **Runtime Owner** 稱呼同一件事。70 號是 **domain design**，它的存在不依賴任何一個服務名——
**domain 文件與 deployment boundary 是兩件事**，不要因為服務合併就以為 domain 被取消。

| financial fact | authority | 為什麼是它 |
|---|---|---|
| **Provider Fulfillment Facts**（配送履約事實：狀態、責任成因、provider 原生線索） | **shipping-service** | 唯一知道「這趟配送發生了什麼」；成因判定原料（`cancelation_reason`／`undeliverable_reason`）在 `uber_deliveries`；與 shipment 狀態可同交易 |
| **Provider Cost Facts**（暫估成本、實際 provider 成本的事實面） | **shipping-service** | 與履約事實同源、同交易；🔴 **只發事實，不持有應收** |
| **Seller Logistics Receivable**（建立／金額／狀態／true-up） | **payment-service / Logistics Finance module** | 賣家層級的債權，跨越單筆 shipment 生命週期 |
| **Seller Logistics Credit**（賣家物流貸記餘額） | **payment-service / Logistics Finance module** | `2026-09-11-logistics-cost-true-up.md` 的四條保證（永久可追蹤／不得過期／不得因無交易消失／平台最終仍欠）是**賣家層級餘額**，不是單筆 |
| **Collection Attempt**（收款嘗試的生命週期、實際手續費、失敗處置） | **payment-service / Logistics Finance module** | 餘額不足／次日失敗／重試是**收款生命週期**，與債權狀態同一個交易邊界 |
| **PayUni Collection**（`agent_fee` 提交、查詢、取實際手續費） | **payment-service / Logistics Finance module** | 所有 PayUni 通道收斂於 payment-service（`PayUniHttpClient.callAgentApi` 已被 `grant` 使用）；05 詳設 `:17` 已逐字認列本通道 |
| **Financial Ledger**（雙分錄帳本與分錄） | **payment-service / Logistics Finance module** | 五元組載體（`account_code` ＋ `owner_type` ＋ `owner_ref_id` ＋ `direction` ＋ `amount`）與 `2026-09-11-ledger-carrier-model-compatibility.md` 的 `txn_type` 互斥約束同一張表 |
| **Provider Financial Reconciliation**（provider 應付、帳單匯入、差異案件） | **payment-service / Logistics Finance module** | 月結對帳是**跨筆聚合**，單筆軸表達不了；`provider_settlement_lines` 無 `commerce_model` 限制 |
| **Invoice / Adjustment Document**（平台→賣家服務費發票與折讓） | **invoice-service** | 方向二已在 `allowedIssuanceCombos` 第三列 `{PLATFORM, SELLER, manualLines}` |
| **買家端金額**〔原表沿用，本次裁決未觸及〕 | **order-service** | 既有；🔴 且 reconciliation **一律不得回寫**（`2026-09-11-logistics-cost-true-up.md`） |

🔴 **界線的一句話**：「這趟配送的 Uber 成本是 120」是**物流事實**（shipping-service）；
「賣家因此欠平台 120」是**債權**（Logistics Finance module）。
**shipping-service 發出成本事實，不持有應收。**

- **可機械執行的驗證（一）**：`shipping-service` 的任何一張表**不得出現** receivable／credit／collection／
  `agent_fee` 語意的欄位。一旦出現，就是這條界線被跨過了。
  ⚠️ 這條驗證對現況成立：shipping-service 全部 migration 只有 **3 個金額欄**
  （`shipments.trade_amt`、`shipping_method_configs.commission_min`、`shipping_fee_tiers.fee`；
  另有 `commission_rate` 費率欄與 `settlement_days` 天期欄，**兩者都不是金額**），零 receivable 概念。
- 🔴 **可機械執行的驗證（二）——合併後才需要的那一條**：同一個可部署單元**不等於**同一個 domain。
  `io.github.will413028.hawkr.payment.logistics.**` 不得被結帳／退款的 hot path import，
  且 Logistics Finance 的 9 張表不得與 business payment 的表互相 JOIN。
  **business payment hot path 與 logistics reconciliation 不得彼此直接耦合**（裁決第 2 條）。
  ⚠️ 拆成獨立服務時，網路邊界會替你擋住這件事；併入同一個服務之後**只剩 package 邊界與 gate**
  ——這是本次裁決換來的代價，明寫於此。

- **為什麼不放 shipping-service**（三個具體理由，不是風格偏好；**這一段不受 2026-09-12 裁決影響**，
  它論證的是「不該由 Fulfillment 持有債權」，與債權由誰執行無關）：
  1. `seller_logistics_credit` 的**四條保證**（`2026-09-11-logistics-cost-true-up.md`）是**賣家層級餘額**，
     而 shipping 的資料模型全部以 shipment 為軸
  2. 月結對帳是**跨筆聚合**，單筆軸表達不了
  3. `agent_fee` 的失敗處置（餘額不足／次日失敗／重試）是**收款生命週期**，與配送狀態機正交
     ⇒ 塞進 shipping 會讓 `shipments` 長出第二個狀態機
- 🔴 **為什麼是 payment-service 而不是一個獨立的 accounting 服務**
  （2026-09-12 取代原版的「為什麼不新建 accounting boundary」——原版用「schema 已經寫好了」當理由，
  那是 circular reasoning：schema 是**設計產物**不是既成事實，而且該服務從未被建立過）：
  1. **收款執行本來就在 payment-service**。債權與通道分屬兩個部署單元時，「送出收款」與「記應收」
     跨網路 ⇒ 失敗窗口要靠額外的補償機制才收得住；在同一個服務內它們可以同交易。
  2. **獨立服務要付部署／監控／契約三份成本，而線 B 不產生任何使用者可見價值**
     ⇒ 成本與收益不成比例，且**最容易在排程時被壓縮**，壓縮的後果是線 B 整條沒有 owner。
  3. **代價**：domain 邏輯要從 Go 重寫成 Java（V9 已把 9 張表落在 `hawkr_payment`），
     並靠 `io.github.will413028.hawkr.payment.logistics.{domain,application,repository,infrastructure,scheduler}`
     維持內部分層。
- **已落實於**：`2026-09-11-ledger-carrier-model-compatibility.md`／`2026-09-11-channel-fee-grossup-in-adapter.md`；66 詳設 §1.6.1 的 punt 由本 ADR 承接；
  [70 號](70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89詳細設計書.md) §1 的 Runtime Owner 標示；
  `services/java/payment-service/src/main/resources/db/migration/V9__add_logistics_finance_tables.sql`。

## 修訂：設計書 65A、66、68、69 已刪除（2026-10-29）

物流履約前端實作規格（65A）、Uber Direct 整合（66）、7-ELEVEN 大宗寄倉（68）、商家自有出貨（69）的詳細與基本設計書已刪除：65A、68、69 未實作且不在任何計畫內，66 的 Uber 已由 ADR `2026-10-27-general-ecommerce-scope-cut.md` D4 決定刪除。本文中「65A」「66」「68」「69」號設計書與它們的章節編號指向已刪除的文件，內容見 git 歷史。設計書 65 只保留已實作的部分。
