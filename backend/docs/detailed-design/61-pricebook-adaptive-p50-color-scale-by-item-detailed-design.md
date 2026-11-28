# Pricebook 各品項自適應 P50 色階詳細設計書

---

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 維護者 | Will |
| 對應程式碼 | `services/pricebook-service/internal/domain/spatial/adaptive_scale.go`、`services/pricebook-service/internal/domain/spatial/surface.go`、`services/pricebook-service/internal/usecase/spatial_surface.go`、`api/proto/pricebook/pricebook.proto` |

---

## 1. 概述與目標

### 1.1 色階來源

網頁賣家中心價格地圖的色階 breaks **依「這個 selection ＋ 這個 target date」的統計快照動態產生**（policy `selection-p50-v1`），不是依 MOA 類別共用固定值。

`reference/price_scale.v1.csv` **不提供 breaks**，只作為 **eligibility gate**：沒有對應審核列的 selection 整個回 `unavailable`（fail-closed，不建立任何 surface）。

domain 只使用同一次 response 已經算出的三個統計量：`SpatialStatistics.minimum_p50`、`p50`（成交量加權）、`maximum_p50`。**不引入新的統計計算，也不讓瀏覽器自行計算或插值**——四層（cell 染色、等高線切點、縣市 rollup、圖例）必須共用同一組 server 端算出的 breaks，任何一層自己算都會產生漂移。

### 1.2 Goals / Non-Goals

| 類型 | 項目 |
|---|---|
| Goal | 同一 selection 的 cell 染色、等高線切點、縣市 rollup、圖例共用同一組 server 端 breaks，四層不各自漂移。 |
| Goal | 平坦或量化後無法形成 ≥2 個有效門檻的 domain，回 `selection-p50-v1-flat-fallback` ＋ `breaks: []`，不製造假差異。 |
| Goal | 同一 selection／target date／horizon／model snapshot 重取，`scaleId` 與 `breaks` 完全可重現。 |
| Non-Goal | 網站端的圖例文案與揭露設計屬 `hawkr-website`，不在本文件。本文件只定義 §1.3 的消費契約。 |
| Non-Goal | 不新增或修改任何 proto 欄位——`PriceScaleInfo` / `SpatialStatistics` 的現有欄位已足夠承載 adaptive 與 flat-fallback 兩種輸出。 |

### 1.3 前端消費契約

以下四條是 backend 對消費端（`hawkr-website`）的保證，任一條被打破都會讓前端的 runtime contract check 直接 throw：

1. `policyVersion == "selection-p50-v1"` ⇒ `breaks` **至少 2 個**，且每個值**嚴格落在** `(minimum_p50, maximum_p50)` 開區間內。
2. `policyVersion == "selection-p50-v1-flat-fallback"` ⇒ `breaks` 為**空陣列**。
3. policy-version 字串必須逐字元相符，不得出現第三種值。
4. 🔴 `flat-fallback` **不得**與「三個 P50 統計量皆為 null」同時出現在一個成功回應裡。這個組合只允許出現在 `status="unavailable"`（`TestSpatialSurface_Get_AllMarketsClosed_ReturnsUnavailableNotError` 釘住此不變量）。

---

## 2. 演算法設計

### 2.1 輸入

`AdaptiveScaleDomain{Minimum, Median, Maximum}`——對應同一次 `Get()` 呼叫算出的 `weightedMedianP50` 三個回傳值，`Minimum <= Median <= Maximum` 是呼叫端（`weightedMedianP50` 本身排序陣列頭尾與中位數）的結構性保證。

### 2.2 breaks 生成

把 `(0, Minimum)`、`(0.5, Median)`、`(1, Maximum)` 當成三個已知的 (分位, 價格) 錨點，對固定的 5 個分位 `[0.20, 0.35, 0.50, 0.65, 0.80]` 做分段線性內插（`f<=0.5` 在 `[Minimum, Median]` 內插，`f>0.5` 在 `[Median, Maximum]` 內插，`f=0.5` 精確等於 `Median`）。每個候選值量化到小數點後 4 位（與既有 cell/county 價格量化同一單位），依序只保留「嚴格大於前一個已保留值、嚴格小於 `Maximum`」的候選——分布越靠近 `Minimum` 或 `Maximum` 一側，量化後 collapse 掉的候選越多，這就是「2～5 個門檻」隨資料分布變化的機制，不是另外一條「決定要幾條」的規則。

存活 `< 2` → `selection-p50-v1-flat-fallback` + `breaks: []`；存活 `>= 2` → `selection-p50-v1` + 存活值；依構造必然嚴格遞增、非負，且落在 `(Minimum, Maximum)` 開區間內。

### 2.3 Scale ID

對 `SelectionKind|SelectionCode|TargetDate|HorizonDays|ModelVersion|BasePolicyID|PolicyVersion|Minimum|Median|Maximum|Breaks` 做 sha256，取前 8 hex 字元接在 `BasePolicyID`（`reference/price_scale.v1.csv` 該列的 `ScaleID`）後面，例如 `N05-price-scale-v1-adaptive-a1b2c3d4`。前綴可追溯回是 CSV 的哪一條審核列讓這個 selection 有資格上線；雜湊對任何輸入差異敏感，同輸入必然同輸出。

### 2.4 呼叫時序

`Get()` 必須在 `buildSurface` / `countyRollups`（兩者都消費 `scale.Breaks`）**之前**，先以 `u.marketPrices(markets, nil)` ＋ `weightedMedianP50` 算出三個統計量餵給 `DeriveAdaptiveScale`。

`finish()` 內另有一次 `weightedMedianP50` 呼叫（寫入 `out.Statistics`），兩次結果必然相同——`weightedMedianP50` 不依賴 `usedMarketCodes`。**這是刻意的重複，不要為了去重而把它抽成共用欄位**：抽掉之後 breaks 與 statistics 的來源就不再可獨立驗證。

### 2.5 空 breaks 的處置

`domain/spatial.band()` 對空 `breaks` 一律回 `(0, nil, nil)`。**這個 guard 不可移除**——`flat-fallback` 必然帶空 `breaks`，少了它會在 `breaks[len(breaks)-1]` 產生 `index out of range [-1]` panic。

`contour.go` 的 `BuildContours` 對空 `breaks` 本身即安全。

---

## 3. 相容性

- `reference/price_scale.v1.csv` 檔案本身與 `LoadPriceScales` / `priceScaleFor` 的 eligibility 判斷**不受本色階機制影響**——沒有審核列的 selection 一律 `unavailable`。
- **所有**成功建立 surface 的 response，`scale.policyVersion` 只會是 `selection-p50-v1` 或 `selection-p50-v1-flat-fallback`，**不會是 `v1`**。網站端對 `v1` 的渲染路徑是給舊快取用的防禦性相容，不是 backend 仍會輸出的值。
- 無 DB migration、無 MQ 事件、無 proto wire 變更——純計算邏輯，隨一般部署流程上線，無額外的部署順序要求。

## 4. 驗收行為

| # | 行為 | 驗收方式 |
|---|---|---|
| 1 | 兩個不同 selection 產生**不同**的 breaks | `TestSpatialSurface_Get_DifferentSelections_ProduceDifferentAdaptiveBreaks` |
| 2 | `breaks` 非負、有限、嚴格遞增，且每個值落在 `(Minimum, Maximum)` 開區間 | 演算法構造保證（§2.2）＋單元測試 |
| 3 | 平坦或量化後不足 2 門檻 ⇒ `selection-p50-v1-flat-fallback` ＋ `breaks: []` | 單元測試 |
| 4 | cells／contours／rollups 共用同一組 breaks（同一個 `base.Scale.Breaks` 餵給 `buildSurface` 與 `countyRollups`） | 單元測試 |
| 5 | 同一 snapshot 重取，`scaleId` 與 `breaks` 完全相同 | `TestSpatialSurface_Get_IsDeterministicAcrossRuns` |
| 6 | 全市場休市 ⇒ `status="unavailable"`，不是 error | `TestSpatialSurface_Get_AllMarketsClosed_ReturnsUnavailableNotError` |
