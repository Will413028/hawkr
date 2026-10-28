# 商品瀏覽統計歸 seller-service，並以時間序列表而非累計計數器儲存

| 項目 | 值 |
|---|---|
| 狀態 | accepted；A3b 7c（Will 2026-10-02）修訂儲存形式：歸 seller 不變，改成只追加的原始事件加上背景彙總成每日桶 `(store_id, 台北日期)`，儀表板只讀彙總。本檔否決累計計數器的理由是「做不了時段」，日桶下不成立；商品層級與平台細分仍是非目標（計畫 `2026-09-29-a3-buyer-seller.md` 7c 的決定） |
| 日期 | 2026-09-18 |
| 影響範圍 | `services/seller-service/`、`services/java/product-service/`、賣家 Dashboard |

## 脈絡

賣家 Dashboard 要顯示「賣家旗下所有商品的總瀏覽次數」，來源有兩個：APP 已登入買家（既有 MQ `product.viewed` 事件）與網頁匿名訪客（公開打點 API）。

兩個決策必須在動手前定下來，因為兩者事後改都要搬資料：這份統計由哪個服務擁有，以及它怎麼存。

## 考慮過的方案

### 一、統計歸哪個服務

| 方案 | 代價 | 判定 |
|---|---|---|
| **seller-service** | 需要向 product-service 查 `product → seller_id`（網頁路徑），多一次跨服務呼叫 | **採用** |
| product-service | 商品瀏覽事件的天然歸屬地，但賣家層級的聚合要跨服務 JOIN；且它是 Java/Spring Boot，加 MQ consumer 等於再開一套 consumer 架構 | 否決 |

seller-service 已經擁有 Dashboard gRPC，賣家層級的聚合數據集中在同一個服務，Dashboard 查詢不需要跨服務。

### 二、怎麼存

| 方案 | 代價 | 判定 |
|---|---|---|
| **時間序列表**（每次瀏覽插一列，查詢時 `COUNT(*)` 帶時間範圍） | 寫入量等於瀏覽量；表會持續成長，需要保留策略 | **採用** |
| materialized counter（單一累計欄位，每次 `+1`） | 寫入極省，但**無法做任何時段篩選**——「本週 vs 上週成長率」這類查詢做不出來，要補就得再加一張時間序列表 | 否決 |

## 決策

1. 商品瀏覽統計由 **seller-service** 擁有：表、consumer、聚合查詢都在該服務內；網頁路徑由 seller-service 自行向 product-service 解析 `seller_id`，Gateway 不承擔這個查詢。
2. 以**時間序列表** `seller_product_view_log` 儲存，每次瀏覽插入一列；聚合在查詢時以時間範圍 `COUNT(*)` 計算。

## 後果

- **接受**：寫入量等於瀏覽量，表會持續成長。目前規模下（30 天約 10 萬列／賣家）以 `(seller_id, viewed_at DESC)` index 支撐；量級再上一個數量級時需要導入分區或彙總表。
- **接受**：網頁打點路徑多一次 product-service 呼叫（50ms timeout，逾時靜默忽略）。
- **重新評估條件**：瀏覽量成長到查詢時聚合撐不住，或出現「只要總數、不要時段」的單一需求。

現行規格（表結構、consumer 行為、統計時窗、失敗處置）見
`docs/modules/seller.md` §16。
