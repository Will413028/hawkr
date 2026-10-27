# 節假日資料的快取位置與覆蓋度監測方式

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-06-30 |
| 影響範圍 | `shipping-service`（`HolidayService`）、`monitoring/sql-exporter`、`monitoring/prometheus/rules` |
| 規格 | [24-節假日管理](../詳細設計書/24-節假日管理（Holiday%20Management）詳細設計書.md) |

## 脈絡

台灣國定假日資料量極小（每年約 120 筆）、一年匯入一次，但工作天判定在出貨路徑上是
每次呼叫都要做的同步查詢；同時「下年度資料沒人上傳」會讓整條出貨鏈卡死，需要在耗盡前
就被發現。

---

## D1：快取放記憶體還是 Redis

| 方案 | 代價 | 判定 |
|---|---|---|
| Redis | 每次工作天判定多一次網路往返，換不到任何東西——資料 < 1KB、只有 shipping-service 讀、前台透過 gRPC 取得而不直接讀 Redis | ✗ |
| 服務內記憶體 `ConcurrentHashMap` | 每個 pod 各持一份，更新要靠 `importHolidays` 觸發 ＋ 月排程兜底；冷啟動視窗要另外擋 | ✓ |

**決策**：記憶體快取。冷啟動視窗以 `ComponentReadiness` 契約擋住（未載入完成前
readiness 報 DOWN），更新以「匯入即刷新 ＋ 月排程兜底」兩層處理。

## D2：覆蓋度監測用 SQL 直查還是 application gauge

| 方案 | 代價 | 判定 |
|---|---|---|
| A2：`HolidayService` 暴露 Micrometer gauge | 把資料完整性監測寫進 application code，違反《後端服務責任邊界規範》 | ✗ |
| A1：obs 層 `sql_exporter` 直查 DB | 監測與被監測者分離，shipping-service 零改動 | ✓ |

**決策**：A1。資料完整性監測歸 observability，不進 application code。

## D3：告警門檻用「離年底天數」還是「覆蓋剩餘天數」

原始需求是「T-60／T-30／T-7 各推一次 ＋ T-7~0 每日」，以離 12/31 的天數倒推。

**決策**：改用「覆蓋剩餘天數」門檻分級（`< 60 且 >= 7` warning、`< 7` critical）。
兩者等價（覆蓋剩 N 天 ≈ 下年度未上傳時距年底 N 天），但門檻分級是 Prometheus 慣用形狀，
一次涵蓋各節點，避免「精準在 T-60 推一次」這種非慣用的一次性告警。

## 後果

- 假日資料的更新在 pod 之間不是即時一致的：`importHolidays` 只刷新處理該請求的那個 pod，
  其餘 pod 要等月排程。運維直接改 DB 時同理。
- 告警的 repeat 節奏由 Alertmanager 的環境設定決定，不是固定 24 小時。
