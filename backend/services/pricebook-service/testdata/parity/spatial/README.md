# pricebook Go↔Python spatial parity fixture

給 `internal/domain/spatial`（`Hawkr_Transformer_Pricebook@2b64d9b` 的
`spatial/interpolation.py` + `spatial/reference.py` 的 Go port）做端到端逐欄位比對用。
理由與 regenerate script 慣例同隔壁的 `testdata/parity/`（ridge model），只是換成
IDW / TWD97 / marching-squares 這條引擎。

## 為什麼需要這份 fixture

Task 1-6 每個函式（投影、IDW、grid、marching-squares、banding）在實作當下都已各自對過
Python 參考輸出。這份 fixture 補的是**組合層**：`BuildRegionGrid → IDWAt → BuildSurfaceCells
/ BuildContours` 串起來跑，對照 Python 的 `build_spatial_surface` 編排結果。單函式各自對得上、
串起來卻不對的失效模式（跨函式邊界的單位換算、EPSG 沒一路帶下去、迭代順序不同導致 feature
排序不同）只有這一層驗得到。

## Scenario

一個合成的 20km×20km 正方形 region（**不是真實台灣地形**——county rollup 對真實
`reference/taiwan_boundary.*.geojson` 的驗證是分開的），4 個合成市場觀測點構成非共線四邊形，
`grid_resolution_m=2500`、`contour_breaks=(15, 25, 35)`、`epsg=3826`。

產出：49 個 surface feature、21 個 contour feature、4 個 point feature，`status="ready"`。

## 檔案

| 檔案 | 說明 |
|---|---|
| `spatial-fixture.json` | golden fixture。`region_ring_wgs84`（測試 region 的 WGS84 外環）、`status`、`surface_geojson` / `contours_geojson` / `points_geojson` 三個 FeatureCollection，以及產生時用的 `config`。由 `regenerate/gen_spatial_fixture.py` 直接呼叫 Python 的 `build_spatial_surface` 產生，**沒有任何手寫的期望值** |
| `regenerate/gen_spatial_fixture.py` | 重現腳本。唯讀存取 Transformer clone，不 monkey-patch、不改對方 repo 任何檔案 |

## 容差

沿用 `testdata/parity/README.md` 訂的政策（設計書 §6）：特徵層 `1e-12`、價格層 `1e-9`。
本 fixture 兩邊的輸出都已先經過 rounding（座標 6 位、價格 4 位、`nearest_distance_km` 3 位，
Python `_rounded()` ↔ Go `spatial.RoundTo()`），所以實際上取較嚴的 `1e-9` 一體適用；
真正的比對負擔落在「rounding 之前的值是否一致到不會跨進位邊界」。

⚠️ 計畫書草稿裡寫的「features 1e-6, prices 1e-4」在 `testdata/parity/README.md` 裡**不存在**，
是轉述時的誤植。以本節與該 README 為準。

## 比對點

`parity_test.go` 對 **49 個 surface feature 與 21 個 contour feature逐一**比對，不是只比數量：

| 層 | 欄位 |
|---|---|
| surface | `price_twd_per_kg`、`band_index`、`lower/upper_price_twd_per_kg`（含 null）、`supporting_neighbor_count`、`nearest_distance_km`、Polygon 外環 5 個點 × 2 座標 |
| contour | LineString 兩端點 × 2 座標、`level_price_twd_per_kg` |

`supporting_neighbor_count`（4 角取 min）與 `nearest_distance_km`（4 角取 max，且 Go 的
`IDWAt` 回傳**公尺**、Python 回傳**公里**）是刻意納入的：那是跨函式邊界的單位換算點，
單函式測試驗不到。

## 已知限制

- 同 ridge-model fixture 的免責聲明：**4 個點 / 1 個 region**，不是 production 的
  ~數百市場 / 22 縣市 / 3 個 surface_region。過了不代表 production 規模會過。上線前仍需要跑
  一次全量比對（Go 的 `spatial_surface` usecase 輸出 vs Transformer live 的
  `/api/v1/spatial-surfaces/{crop_id}`，挑一個市場數多的真實作物），比照 ridge-model port
  在信任 production `model.json` 之前所做的一次性人工檢查。
- **只覆蓋 `status="ready"` 這條路徑**。`degraded` / `unavailable` 與各種
  `quality_reason_codes`（`insufficient_spatial_points`、`collinear_spatial_points`、
  `unsupported_land_area`、`flat_price_field`…）不在這份 fixture 裡，由 usecase 層的
  單元測試各自覆蓋。
- **location 合併的加權方式，測試端與 production 端刻意不同**。`parity_test.go` 的
  `parityLocations` 照 Python `_consolidate_locations`（`interpolation.py:372-382`）做
  **volume 加權**平均；但 production 的呼叫端 `usecase.consolidateLocations` 刻意用
  **未加權**平均。

  ⚠️ 這裡是 pipeline 的**兩個不同步驟**，不要混為一談：
  1. `aggregateMarkets`（前一步，把一個大類的子品項折成「這個市場一個數字」）**有做 volume
     加權**，走 `variantWeights` → `Forecast.ComputeGroupVolumes` → `forecast.WeightedAverage`，
     跟 GroupForecast / TrendSummary 同一套權重。crop path 一個市場只有一條 series，加權是
     恆等運算，所以刻意不付那次查詢。
  2. `consolidateLocations`（本節講的這步，把投影後座標相同的市場併成一個內插點）**維持未加權**
     ——到這一步每個市場的 volume 已經在上一步折進它自己的值裡了，兩個剛好共座標的**不同**
     設施之間沒有剩下有意義的權重可用。Python 在這一步併的是還帶著各自 `volume_kg` 的原始
     觀測值，做的是不同的事。

  **這個分歧因此完全不在本 fixture 的驗證範圍內**，而且這份 scenario 的 4 個點各自落在不同
  格點、每組只有 1 個成員，加權與未加權在數值上恰好相同，連間接察覺都不可能。真要驗這段
  精度得另建多市場共點的 scenario。（同一個函式還有一處更小的分歧：分組鍵 production 用
  `math.Round`（四捨五入），Python `round()` 與本測試用的是 banker's rounding，只在座標
  剛好落在 .5 公尺時才有差。）
- **contour 的投影與去重不在 `domain/spatial` 裡**。Go 刻意把 `BuildContours` 的輸出留在
  TM2 公尺空間，投影成 WGS84 + 丟掉「兩端點 round 到小數 6 位後相同」的退化線段這兩步在
  呼叫端（`internal/usecase/spatial_surface.go` 的 `projectContours`，對應 Python 的
  `if left_wgs84 == right_wgs84: continue`）。`parity_test.go` 在測試裡複製了這個呼叫端契約——
  也就是說這份 fixture 驗的是「domain 函式 + 已知的呼叫端契約」的組合，不是 domain 函式單獨。

## 怎麼重新產生

需要 資料團隊那個 repo 的本機 clone（**不在本 repo 內，也不會進 CI**——本機重現用，
不是自動化測試的一部分）。腳本只讀不寫來源 repo，不需要它的 venv，只需要 `numpy`。

```bash
cd services/pricebook-service/testdata/parity/spatial
PRICEBOOK_REPO=~/Hawkr_Transformer_Pricebook \
    python3 regenerate/gen_spatial_fixture.py
```

環境變數：`PRICEBOOK_REPO`（預設 `~/Hawkr_Transformer_Pricebook`）。
輸出固定寫回本目錄的 `spatial-fixture.json`。

**2026-08-08 已實測**：連跑兩次，產物 sha256 一致（`e80512af…`），且與本目錄現存版本逐 byte 相同。
