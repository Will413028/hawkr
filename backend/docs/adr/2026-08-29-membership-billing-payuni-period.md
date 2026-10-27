# 會員訂閱計費採 PayUni 續期收款，支付頁模式，不設重試

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-08-29 |
| 影響範圍 | payment-service、buyer-service、payuni-webhook-service、PCI 稽核範圍 |

## 1. 採哪一條 PayUni 產品線

**考慮過**：

| 方案 | 代價 | 判定 |
|---|---|---|
| (a) 信用卡幕後 Token 交易（`POST /api/credit`，我方自控排程） | 要自建 cron 主動觸發每期扣款、自己維護「現在輪到誰」的排程狀態 | 否決 |
| (b) 續期收款（`POST /api/period`，PayUni 自帶排程） | 綁定 PayUni 的週期語意 | **採用** |

**理由**：會員週期公式是「每月同一天」，與 (b) 的 `PeriodType=month` 原生語意相容；
選 (b) 之後 buyer-service **一支 cron 都不需要**——觸發權在 PayUni，我方只對結果做反應。

⚠️ 兩條產品線要**分別申請審核**並各自綁定幕後 IP，不是同一個開關。

## 2. 訂閱建立走支付頁還是幕後

**考慮過**：(a) 支付頁（hosted redirect）／(b) 幕後（server-to-server，我方收發卡號）

**決策：(a)。** 依既有立場「raw card data NEVER touches our backend」——(b) 會把持卡人資料
帶進我方 PCI 稽核範圍，還額外要求 IP 白名單；(a) 兩者都不需要。換卡同理走 hosted redirect。

## 3. Webhook 是否新增型別

**考慮過**：(a) 沿用既有 `PAYMENT_UOP`／(b) 新增 `PAYMENT_RECURRING`

**決策：(b)。** 續期收款的 Notify payload 是**完全不同的欄位形狀**（帶 `PeriodTradeNo`／
`PeriodOrderNo`／`ThisPeriod`），沿用既有解析等於在一個型別裡塞兩種 schema。

## 4. 訂閱扣款是否透過 order-service

**決策：不透過。** 訂閱扣款不產生訂單——它的品項來源是**方案快照**而不是訂單快照，
發票側（58 號）本來就把「訂閱制」線設計成以方案快照為品項來源，且平台 issuer 沒有 `store_id`。
硬走 order-service 會為了複用一條路徑而造出一筆語意上不存在的訂單。

## 5. 續約失敗要不要設緩衝期／重試

**考慮過**：(a) 寬限期 ＋ 重試編排／(b) 失敗即刻降回免費

**決策：(b)，緩衝天數 ＝ 0。**

**接受的代價**：若 PayUni 對同一期有我方不知情的內部重試、而它隨後成功，會出現
「我方已降級、PayUni 端最終收款成功」的不一致——買家付了錢卻沒拿到方案，需要人工處理。
這是刻意用「極少數誤降級」換「完全不建置重試／等待機制」。

**連帶移除**：整套重試編排（renewal cycle 表、per-period 重新授權 RPC、5 態重試狀態機、
buyer-service 排程 cron），以及「寬限狀態要不要反映在 `buyer_memberships`」這個問題本身
——沒有寬限期，該問題不存在。

## 後果

- `buyer-service` **不得**為此建立任何 cron 或重試機制；收到結果就是最終處置。
- `membership_charge_attempts` 的 first-terminal-wins 是這個決策的機械保證：
  晚到的矛盾結果只留成稽核列，不改寫已做出的處置。
- 若日後要加回重試，**先回來推翻這份 ADR**——重試會同時改變資料模型（需要 Task 表）
  與 `buyer_memberships` 的狀態語意。
