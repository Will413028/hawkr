# 節假日管理（Holiday Management）詳細設計書

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 對應程式碼 | `services/java/shipping-service/.../service/HolidayService.java`、`.../service/ShippingService.java`（日期驗證呼叫點）、`monitoring/sql-exporter/`、`monitoring/prometheus/rules/business-alerts.yml` |
| 相關 ADR | [2026-09-18-holiday-cache-and-coverage-monitoring](../adr/2026-09-18-holiday-cache-and-coverage-monitoring.md) |
| 相關 doc | [06-物流微服務](./06-物流微服務（Shipping%20Service）詳細設計書.md)、68 號《7-ELEVEN 大宗寄倉（B2C）》§6.1.3（第二個消費者） |

---

## 1. 概述

### 1.1 目的

管理台灣國定假日與例假日資料，提供**工作天驗證**與**可出貨日期查詢**功能。

### 1.2 業務背景

宅配出貨（黑貓 TAQBIN）的 `shipDate`（出貨日期）與 `deliveryDate`（配達日期）必須為**工作天**。工作天定義：非週六日、非台灣國定假日。

### 1.3 資料來源

行政院人事行政總處公開 CSV（Google 行事曆專用格式）：
- 網址格式：`https://www.dgpa.gov.tw/FileConversion?filename=dgpa/files/...csv`
- CSV 列出**所有非工作天**（含週六日「例假日」、國定假日、補假）
- 不在 CSV 中的日期即為工作天

### 1.4 使用場景

| 場景 | 說明 |
|------|------|
| `generate_home_label` 日期驗證 | shipDate / deliveryDate 必須為工作天 |
| 前台日曆元件 | 查詢可出貨日期範圍，標記非工作天為不可選 |
| B2C 大宗寄倉 `print_label` | `ShipDate ≥ 今日+1` 的工作天判定（68 號 §6.1.3 E3） |

---

## 2. DB Schema

### 2.1 holidays 表

隸屬 `hawkr_shipping` 資料庫。主鍵使用 BIGSERIAL。

| **欄位名稱** | **類型** | **必填** | **說明** |
|---|---|---|---|
| id | BIGSERIAL | PK | 自增主鍵 |
| holiday_date | DATE | YES | 放假日期（UNIQUE） |
| name | VARCHAR(50) | YES | 假日名稱（例假日 / 春節 / 補假 等） |
| year | SMALLINT | YES | 年份（方便按年匯入/清除） |
| created_at | TIMESTAMPTZ | YES | DEFAULT NOW() |

**索引：**
- `uk_holidays_date UNIQUE (holiday_date)` — 日期唯一，快速查詢
- `idx_holidays_year (year)` — 按年份查詢/清除

**設計決策：**
- 不需要 `updated_at`（紀錄型資料，只 INSERT/DELETE 不 UPDATE）
- 使用 BIGSERIAL 而非 TSID（內部管理表，不對外暴露）
- `year` 冗餘欄位，方便按年清除重匯

---

## 3. 快取機制

### 3.1 記憶體快取（首選）

holidays 資料特性：**量少（每年約 120 筆）、變動極低（每年匯入一次）**，採服務內記憶體快取，**不放 Redis**（選型理由見[相關 ADR](../adr/2026-09-18-holiday-cache-and-coverage-monitoring.md) D1）。

| 項目 | 說明 |
|------|------|
| 快取結構 | `ConcurrentHashMap<LocalDate, String>` — date → name |
| 載入時機 | `@PostConstruct` 服務啟動時從 DB 全量載入 |
| 更新時機 | `importHolidays` API 執行後立即刷新；每月 1 日 03:00 UTC 自動 reload（`@Scheduled`） |
| 查詢方式 | `holidayCache.containsKey(date)` — O(1) |

### 3.2 ComponentReadiness 契約（強制）

`HolidayService` 實作 `ComponentReadiness`（介面定義見 `Java 微服務開發規範.md §8.7`），於 `@PostConstruct` 完成 DB 載入後將 `cacheLoaded` 翻 true。

| 規則 | 行為 |
|------|------|
| `isReady()` | `cacheLoaded.get()` — false 表示尚未完成首次載入 |
| `name()` | `"shipping-holiday"` |
| 影響 | shipping-service `/actuator/health/readiness` 在 cache 未 ready 時報 `DOWN`，K8s 不導流量進來 |

**為什麼需要**：假日資料未載入時，`isBusinessDay()` 會錯誤地把所有日期判定為工作天，導致日期驗證失準。冷啟動視窗必須阻擋。這道阻擋保護兩條路徑：宅配 `generate_home_label` 與 B2C 大宗寄倉 `print_label`（68 §6.1.3 E3）。

### 3.3 月排程 refresh（強制）

| 項目 | 設定 |
|------|------|
| 觸發時機 | `@Scheduled(cron = "0 0 3 1 * *")` — 每月 1 日 03:00 UTC |
| 動作 | 重新從 DB 全量載入 `holidayCache` |
| 失敗處理 | log warn，保留既有 cache，不更新 `cacheLoaded`（已為 true） |

⚠️ `importHolidays` API 會立即刷新該 pod 的快取；月排程是兜底機制，不是主要路徑。運維直接改 `holidays` 表時沒有立即刷新，必須等月排程或重啟。

---

## 4. API 設計

### 4.1 Admin API — 匯入假日（CSV URL）

```
POST /api/v1/admin/holidays/import
```

| 項目 | 說明 |
|------|------|
| Namespace | admin（僅限管理員） |
| 權限 | requireAccountType(4)（ADMIN） |
| 用途 | 傳入政府 CSV URL，後端下載解析後寫入 DB。**檔案內容上傳版見 §4.3**（ADM 後台一鍵補資料首選）；本版並存供既有整合/自動化沿用 |

**Request Body：**
```json
{
  "csvUrl": "https://www.dgpa.gov.tw/FileConversion?filename=..."
}
```

**Response：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "year": "2026",
    "importedCount": "120",
    "deletedCount": "115"
  }
}
```

**處理流程：**
1. 下載 CSV（OkHttpClient，30s timeout）
2. 解析 CSV：跳過 header 和空行，取 `Subject`（col 0）+ `Start Date`（col 1）
3. 從 `Start Date` 推算年份（**取第一筆有效資料的年份 — URL 版隱式年份**；顯式年份版見 §4.3）
4. **先刪除該年份舊資料**（`DELETE FROM holidays WHERE year = ?`）
5. 批次 INSERT 新資料（過濾至該年份 + 去重，見 §7.2）
6. 刷新記憶體快取
7. 回傳匯入統計

### 4.2 App API — 查詢可出貨工作天

```
GET /api/v1/app/shipping_business_days
```

| 項目 | 說明 |
|------|------|
| Namespace | app（登入使用者） |
| 權限 | requireAccountType(2, 3)（SELLER / SELLER_AND_BUYER） |
| 用途 | 前台日曆元件取得可出貨日期清單 |

**Query Parameters：**
- 無（固定回傳以今日為起點的 7 個工作天）

**Response：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "businessDays": [
      {"date": "2026-03-18", "dayOfWeek": "三"},
      {"date": "2026-03-19", "dayOfWeek": "四"},
      {"date": "2026-03-20", "dayOfWeek": "五"},
      {"date": "2026-03-23", "dayOfWeek": "一"},
      {"date": "2026-03-24", "dayOfWeek": "二"},
      {"date": "2026-03-25", "dayOfWeek": "三"},
      {"date": "2026-03-26", "dayOfWeek": "四"}
    ]
  }
}
```

**邏輯：**
- 起點：明日（shipDate 必須 > 今日）
- 如果明日是非工作天，跳到下一個工作天
- 往後連續找 7 個工作天回傳

### 4.3 Admin API — 匯入假日（上傳 CSV 內容）

```
POST /api/v1/admin/holidays/import_content
```

| 項目 | 說明 |
|------|------|
| Namespace | admin（僅限管理員） |
| 權限 | requireAccountType(4)（ADMIN） |
| 用途 | ADM 後台上傳從政府網站下載的 CSV 檔：前端讀成文字內容 + **顯式指定年份**後送入。免 admin 自找/貼 URL。與 §4.1 URL 版並存 |

**Request Body：**
```json
{
  "year": "2026",
  "csvContent": "Subject,Start Date,Start Time,...\n中華民國開國紀念日,2026/1/1,,...\n例假日,2026/1/3,,..."
}
```

**Response：** 同 §4.1（`year` / `importedCount` / `deletedCount`）。`importedCount` = 過濾至宣告年份後實際寫入筆數。

**處理流程：**
1. 解析 `csvContent`（同 §7 解析規則，免下載）
2. **斷言宣告 `year` 在 CSV 中至少 1 筆** → 否則回 `HOLIDAY_CSV_YEAR_MISMATCH`（抓「選錯年 / 傳錯檔」的 admin 操作錯誤）
3. 過濾出宣告 `year` 的假日（政府 CSV 常跨年，見 §7.2/§7.3；**只匯入宣告年，與 URL 版同 filter 邏輯**）
4. **先刪除該年份舊資料**（`DELETE FROM holidays WHERE year = 宣告year`）
5. 批次 INSERT（過濾後 + 去重）
6. 刷新記憶體快取
7. 回傳統計

**與 §4.1 URL 版差異：**

| 項目 | §4.1 URL 版 | §4.3 內容上傳版 |
|---|---|---|
| 年份來源 | CSV 首筆隱式推算 | **admin 顯式指定**（根治「壞 CSV 只有少數有效行 → 刪光整年只寫少數」的隱式風險） |
| CSV 來源 | 後端 OkHttp 下載 URL | 前端 FileReader 讀檔成文字 |
| 傳輸 | JSON `{csvUrl}` | JSON `{year, csvContent}` |

**破壞性覆寫防護：**
- 匯入語意 = 「該年份資料完全取代」（先刪後寫，同一 `@Transactional` 原子，中途失敗整批 rollback）。
- ADM 前端送出前經**二次確認**：以 §4.4 coverage 查得該年現有筆數（＝將刪除數，advisory）＋ 前端本地解析筆數（＝將寫入數，須以宣告年過濾後計）→ 顯示「將刪除 N / 寫入 M」→ admin 確認再送。
- `deletedCount` 為匯入後的事後 ground truth（真實刪除數）。

### 4.4 Admin API — 覆蓋現況查詢

```
GET /api/v1/admin/holidays/coverage
```

| 項目 | 說明 |
|------|------|
| Namespace | admin（僅限管理員） |
| 權限 | requireAccountType(4)（ADMIN） |
| 用途 | ADM 後台顯示各年度假日資料覆蓋現況，呼應 §8 監測，供 admin 判斷是否需補下年度資料、支撐 §4.3 的二次確認 |

**Response：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "coverageDays": "180",
    "years": [
      {"year": "2025", "count": "115", "maxHolidayDate": "2025-12-31"},
      {"year": "2026", "count": "120", "maxHolidayDate": "2026-12-31"}
    ]
  }
}
```

**欄位：**
- `coverageDays`：距耗盡天數 = `COALESCE(MAX(holiday_date) - CURRENT_DATE, -1)`，**與 §8 監測 metric `hawkr_holiday_coverage_days` 同源同公式**（見 §8.4，逐字一致）。空表 / 資料耗盡回 `"-1"`。
- `years[]`：各年度 `count`（該年筆數）+ `maxHolidayDate`（該年最大假日日期，`YYYY-MM-DD`），依 year 遞增。
- `years` 為空即表示尚無任何假日資料。

**一致性註記：** `coverageDays` / `count` 為 **advisory 快照**（非交易級鎖定）；並發匯入下可能瞬時 stale，正確性以 §4.3 匯入 response 的 `deletedCount` / `importedCount` 為準。

---

## 5. gRPC RPC 定義

```protobuf
// ==================== 假日管理 ====================

// 匯入假日（Admin，CSV URL）
rpc ImportHolidays(ImportHolidaysRequest) returns (ImportHolidaysResponse);

// 匯入假日（Admin，上傳 CSV 內容）
rpc ImportHolidaysFromContent(ImportHolidaysFromContentRequest) returns (ImportHolidaysResponse);

// 查詢假日覆蓋現況（Admin）
rpc GetHolidayCoverage(GetHolidayCoverageRequest) returns (GetHolidayCoverageResponse);

// 查詢可出貨工作天（App）
rpc ListBusinessDays(ListBusinessDaysRequest) returns (ListBusinessDaysResponse);

message ImportHolidaysRequest {
    int64 admin_account_id = 1;     // Gateway JWT 注入（E.5.5 例外）
    string csv_url = 2;             // 政府 CSV 下載 URL
}

message ImportHolidaysResponse {
    int32 code = 1;
    string message = 2;
    ImportHolidaysData data = 3;
}

message ImportHolidaysData {
    string year = 1;                // 匯入年份
    string imported_count = 2;      // 匯入筆數
    string deleted_count = 3;       // 刪除舊資料筆數
}

message ImportHolidaysFromContentRequest {
    int64 admin_account_id = 1;     // Gateway JWT 注入（E.5.5 例外）
    string year = 2;                // 顯式指定年份（前端可控，string）
    string csv_content = 3;         // CSV 檔案文字內容（前端 FileReader 讀取）
}
// Response 重用 ImportHolidaysResponse / ImportHolidaysData

message GetHolidayCoverageRequest {
    int64 admin_account_id = 1;     // Gateway JWT 注入（E.5.5 例外）
}

message GetHolidayCoverageResponse {
    int32 code = 1;
    string message = 2;
    HolidayCoverageData data = 3;
}

message HolidayCoverageData {
    repeated HolidayYearCoverage years = 1;
    string coverage_days = 2;       // 距耗盡天數（與 §8 metric 同源公式）；空表 "-1"
}

message HolidayYearCoverage {
    string year = 1;
    string count = 2;               // 該年筆數
    string max_holiday_date = 3;    // 該年最大假日日期（YYYY-MM-DD）
}

message ListBusinessDaysRequest {
    int64 account_id = 1;           // Gateway JWT 注入（E.5.5 例外）
}

message ListBusinessDaysResponse {
    int32 code = 1;
    string message = 2;
    repeated BusinessDay business_days = 3;
}

message BusinessDay {
    string date = 1;                // ISO 8601 日期（YYYY-MM-DD）
    string day_of_week = 2;         // 星期幾（一/二/三/四/五）
}
```

---

## 6. 日期驗證邏輯（`generate_home_label` ＋ `print_label` 使用）

> 本章的工作天判定有**兩個消費者**。[68 §6.1.3 E3](68-7-ELEVEN%20大宗寄倉（B2C）詳細設計書.md)
> 的 B2C 標籤列印 `/api/logistics/print_label` 要求「`ShipDate` 不得為當日（`ShipDate ≥ 今日+1`），
> 且必須是可出貨工作天 ⇒ 走 `ShippingBusinessDaysPath` 的節假日判定」，違反回
> `B2C_LABEL_SHIP_DATE_INVALID`。
>
> **兩者共用 §6.1 的工作天判定，但額外約束不同**（細節不在本書重複，一行指標）：
>
> | 消費者 | `shipDate`／`ShipDate` 額外約束 | 權威 |
> |---|---|---|
> | `generate_home_label`（宅配） | 本章 §6.1 全部 5 條（含「7 個工作天內」上限） | 本書 |
> | `print_label`（B2C 大宗寄倉） | `≥ 今日+1` ＋ 工作天；另有「同一次呼叫四參數一致、≤50 筆」的分組規則 | [68 §6.1.3](68-7-ELEVEN%20大宗寄倉（B2C）詳細設計書.md) |
>
> ⚠️ 68 §8 另立一條不變式：`ShipStatusTime` **只存不判**，只有 `ShipDate`（列印用）走節假日判定，
> **兩者是不同的東西不可混用**。

### 6.1 shipDate 驗證

| # | 規則 | 錯誤訊息 |
|---|------|---------|
| 1 | 必填 | `shipDate 為必填欄位` |
| 2 | 格式 YYYYMMDD | `shipDate 格式錯誤，應為 YYYYMMDD` |
| 3 | 必須 > 今日 | `出貨日期必須大於今日` |
| 4 | 必須為工作天 | `出貨日期 {date} 為假日（{name}），請選擇工作天` |
| 5 | 必須在 7 個工作天內 | `出貨日期超出可選範圍（最遠至 {maxDate}）` |

### 6.2 deliveryDate 驗證

| # | 規則 | 錯誤訊息 |
|---|------|---------|
| 1 | **選填** — 未填則自動設為 shipDate + 1 工作天 | （無錯誤） |
| 2 | 格式 YYYYMMDD | `deliveryDate 格式錯誤，應為 YYYYMMDD` |
| 3 | 必須 > shipDate | `配達日期必須大於出貨日期` |
| 4 | 必須為工作天 | `配達日期 {date} 為假日（{name}），請選擇工作天` |

### 6.3 HolidayService 工具方法

```java
public class HolidayService {
    /** 檢查日期是否為工作天 */
    public boolean isBusinessDay(LocalDate date);

    /** 取得假日名稱（非假日返回 null） */
    public String getHolidayName(LocalDate date);

    /** 從指定日期起算，找到第 N 個工作天 */
    public LocalDate nextBusinessDay(LocalDate from, int n);

    /** 從明日起算，取得接下來 N 個工作天 */
    public List<LocalDate> getNextBusinessDays(int count);
}
```

---

## 7. CSV 解析規則

### 7.1 Google 行事曆 CSV 格式

```
Subject,Start Date,Start Time,End Date,End Time,All Day Event,Description,Location
中華民國開國紀念日,2026/1/1,,2026/1/1,,TRUE,,
例假日,2026/1/3,,2026/1/3,,TRUE,,
```

### 7.2 解析邏輯

1. 跳過第一行（header）
2. 跳過空行（Subject 為空）
3. 取 `Subject`（index 0）→ `name`
4. 取 `Start Date`（index 1）→ 解析為 `LocalDate`（格式 `yyyy/M/d`）
5. 從日期推算 `year`
6. 忽略重複日期（CSV 可能有重複）

### 7.3 錯誤處理

| 情況 | 處理 |
|------|------|
| URL 下載失敗 | 回傳 `CSV 下載失敗: HTTP {code}` |
| CSV 格式不符 | 回傳 `CSV 格式錯誤: 第 {line} 行無法解析` |
| 解析結果為空 | 回傳 `CSV 無有效假日資料` |
| 年份混合（跨年 CSV，§4.1 URL 版） | 以第一筆有效資料的年份為準，只匯入該年份 |
| 年份混合（跨年 CSV，§4.3 內容版） | 以 admin 顯式宣告的 `year` 為準，過濾至該年匯入；若宣告 year 在 CSV 無任何資料 → `HOLIDAY_CSV_YEAR_MISMATCH` |

---

## 8. 資料覆蓋度監測

`holidays` 由 admin 人工從政府 CSV 匯入（§4.1）。若下年度資料未在跨年前上傳，§6 的 `shipDate`／`deliveryDate` 工作天驗證會 fail：宅配 `generate_home_label` 與 B2C `print_label` **兩條路徑一起卡死**。故需監測「假日資料還剩幾天覆蓋」並在耗盡前提醒。

### 8.1 監測資料來源

依 [§22 後端服務責任邊界](../開發規範/後端服務責任邊界規範.md)：**資料完整性監測歸 observability，不寫進 application code**。故採 SQL 直查（非 `HolidayService` 暴露 Micrometer gauge）：

- **產出**：obs 層 `sql_exporter`（co-located 各 infra 主機，連本地 `hawkr_shipping`）跑
  `SELECT COALESCE(MAX(holiday_date) - CURRENT_DATE, -1) FROM holidays` → gauge
  `hawkr_holiday_coverage_days`（尚有幾天覆蓋；空表回 -1）。
- **shipping-service 零改動**：不得在 `HolidayService` 加 gauge——資料完整性監測寫進 application code 違反責任邊界規範。
- 設定：`monitoring/sql-exporter/sql_exporter.yml` 的 `holiday` collector。

### 8.2 告警規則與推播

`monitoring/prometheus/rules/business-alerts.yml`（`holiday-coverage-alerts` group），`env` 由 Prometheus scrape target 提供、`channel: business` 路由：

| Alert | 條件 | 級別 | 說明 |
|---|---|---|---|
| `HolidayDataCoverageLow` | `< 60 且 >= 7` | warning | 與 Critical 互斥 |
| `HolidayDataCoverageCritical` | `< 7` | critical | repeat 依 Alertmanager 各環境週期（dev 2h／prod 30m），非固定 24h |

門檻分級的選型理由見[相關 ADR](../adr/2026-09-18-holiday-cache-and-coverage-monitoring.md) D3。

### 8.3 推播目的地與路由

business 告警 → Discord `dev-警報`（dev）/ `prod-警報`（prod），與 infra 系統告警頻道 `monitor-alert` 分離。路由基建（Alertmanager `channel` label + discord-webhook-gateway 多頻道）見 [基礎建設：業務告警分流與 dev-警報 路由設定](../基礎建設/業務告警分流與%20dev-警報%20路由設定.md)。補資料本身走 §4.3 ADM 後台上傳（或 §4.1 URL 版），非自動抓取。

### 8.4 監測與 ADM 覆蓋現況查詢同源

§4.4 覆蓋現況查詢（`GET /admin/holidays/coverage`）供 ADM 後台可視化，其 `coverageDays` 與本節監測 metric `hawkr_holiday_coverage_days` **使用同一 SQL 公式** `COALESCE(MAX(holiday_date) - CURRENT_DATE, -1)`，確保「告警看到的」與「admin 查到的」一致，不產生兩套算法漂移（**單一定義 SSOT**）。

| 面向 | §8.1 監測 | §4.4 查詢 |
|---|---|---|
| 觸發 | sql_exporter 被動輪詢 + Prometheus 告警 | admin 主動開頁查看 |
| 介面 | 無 UI（Discord 推播） | ADM 覆蓋現況頁 |
| 距耗盡天數定義 | `hawkr_holiday_coverage_days` | `coverageDays`（同公式） |

**完整閉環**：覆蓋將耗盡 → §8 監測告警 Discord → admin 開 ADM 頁看各年度覆蓋現況（§4.4）→ 上傳次年 CSV 補資料（§4.3，含二次確認防覆寫）→ 覆蓋恢復、告警自動解除。
