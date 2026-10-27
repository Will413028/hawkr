# Pricebook 的新鮮度只描述資料多舊，不改寫預測點的狀態

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-09-14） |
| 日期 | 2026-09-14 |
| 影響範圍 | `services/pricebook-service`（`SeriesForecast`、`GroupForecast`）、ai-service 的 `PreviewRfqPrice`（被動受益）、website 採購助手的新鮮度提示 |
| 來源 | `backend/docs/superpowers/specs/2026-09-14-pricebook-freshness-override-removal-design.md`（私人封存）、`backend/docs/superpowers/plans/2026-09-14-pricebook-freshness-override-removal.md`（私人封存） |

## 脈絡

`SeriesForecast` 與 `GroupForecast` 原本各有一段 Step 8（原設計書 §8）：最新可見觀測距 origin 超過 72h（`FreshnessConfig.StaleWithin`）時，把該 series 全部 7 個預測點的 `Status` 覆寫成 `unavailable`。覆寫的是狀態不是值：`p10/p50/p90` 照算並留在 payload，`missing_reason` 是空的，只是下游 `usablePredictionStatus`（只認 `ready`、`partial`、`market_closed`）把整列濾掉。

實測（`cmd/pricebook-forecast`，`moa:338:LA1` 桃農／甘藍-初秋，`data_through_date` 都是 2026-09-09）：origin 在 72h 內回 `ready`、D+1 p50 28.70；在 72h 外回 `unavailable`、p50 29.05。被丟棄的 29.05 對照 MOA 2026-09-13 實際成交 29.2，誤差 0.5%。

後果鏈：桃園市只有桃農一個批發市場 ⇒ `regional-comparison` 該縣市 `supportingMarketCount=0`、`unavailable` ⇒ ai-service `PreviewRfqPrice` 的 `mappingReliable` 不成立 ⇒ 422 `MAPPING_MISSING` ⇒ 買家詢價拿不到參考價。

約束：

- 新鮮度訊號已經另外存在：`SeriesForecastOutput.Freshness`、`GroupForecastOutput.Freshness`、`dataMeta.freshnessStatus`、`sourceDataAsOf`；觀測路徑的 `usabilityStatus`（`ObservedUsabilityFromFreshness`）與預測狀態機無關。
- 品質閘門（`CalibrationGateReason`、`EvaluatePointGates`、`EvaluateHorizonSet`、`market_closed`）已經決定「這個點能不能用」。
- 沒有偵測停業市場的資料或機制。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 現狀：>72h 整條覆寫成 `unavailable` | 模型算得出來的預測整條消失；單一市場的縣市完全拿不到參考價 | ✗ |
| 把 72h 拉長到 14 天（對齊 `MaxIngestAttempts`） | 仍是「過線就整條消失」，只是換位置 | ✗ |
| 降級不消失，加一條遠更寬的硬上限 | 保留一條防「停業市場永遠報價」的線；仍是在服務端替消費端決定能不能用 | ✗（Will 選擇完全移除，接受下列後果） |
| **基準：移除覆寫，不設替代上限**：「有多舊」由新鮮度欄位表達，「能不能用」由品質閘門與消費端判斷（產出方描述資料年齡、消費端決定是否採用，例如 HTTP 的 `Age`／`Last-Modified`、Prometheus 的 staleness marker） | 停業或長期無成交的市場會持續報價，沒有機制發現 | ✓ |

## 決策

- 兩處 Step 8 都移除覆寫；只移其一會造成「單一市場曲線有值、大類曲線消失」。新鮮度照算並如實回報，逐點 `Status` 只由品質閘門決定。
- 新鮮度以 `in.Origin` 計算，不用牆鐘時間（as-of 重放時可預測、可測試）。
- 不改 `usabilityStatus` 與品質閘門；`market_closed`（真休市）不受影響。
- website 採購助手的提示：`stale` 與 `unavailable` 都顯示「價格為模型推估，僅供參考。」。原本 `unavailable` 的「這個區域目前沒有可用的行情資料。」在覆寫存在時不可達，移除後可達卻會與顯示中的價格矛盾。`fresh` 的「僅供設定詢價範圍，不是供應商報價。」講的是另一件事，保留。

範圍外：沒有批發市場的縣市（仍是 `MAPPING_MISSING`）、artifact 裡沒有的 (市場, 品項)、退到大類或全國的 fallback、ingest 排程的延遲。

## 後果

- 以 `LA1` D+1 實測，`regional-comparison` ready 縣市 8 → 11（桃園、宜蘭、臺東新增）。台中市是既有數字被改動：台中市果菜加入 volume 加權後由 33.81 升到約 39.32，背後沒有新成交。
- `spatial-surface` 的 `excludedMarketCodes` 由 5 個變空；插值刻意不做成交量加權，所以永靖鄉（`512`，最後成交 2026-07-28）以 7.57 成為獨立低點並拉低周圍等值線。
- **已知風險**：停業或長期無成交的市場會持續報價，沒有任何機制會發現。Will 2026-09-14 知悉並接受。終點：A4 把 pricebook 搬進單體時，在 `CARRYOVER.md` 重新回答「從零設計會怎麼做」；觸發條件是 A4 開工，或出現一個依賴新鮮度做顯示決策的消費端。

結果（可驗證）：

- `e46ce81d5`（SeriesForecast）、`a38dc5f40`（GroupForecast）移除覆寫；`82e8e15bd` 釘住新鮮度與預測值正交；`0b063f633` 改 website 文案。
- 測試：`services/pricebook-service/internal/usecase/forecast_test.go` 的 `TestSeriesForecast_FreshnessUnavailable_DoesNotOverridePointStatus`、`TestGroupForecast_FreshnessUnavailable_DoesNotOverridePointStatus`、`TestSeriesForecast_FreshnessDoesNotChangePredictedValues`（覆寫加回時會失敗）。
- `website/features/procurement-assistant/RfqDraftFlow.tsx` 的 `FRESHNESS_NOTICE`。
