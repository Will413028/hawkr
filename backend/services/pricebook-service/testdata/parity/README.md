# pricebook Go↔Python parity fixture

給 `backend-payuni/services/pricebook-service` 的 Go 推論實作（Plan B / P3）做逐項比對用。
2026-07-29 產出，來源是 `Hawkr_Transformer_Pricebook` 的 demo artifact。

## 為什麼需要中間層

`forecast.csv` 只有最終 p10/p50/p90。Go port 的真正難點是**特徵建構**——bitemporal
revision tie-break、rolling window 的可見性、以及那個 offset 陷阱（lag 與 rolling 都從
offset 1 起算，**只有 `last_price` 從 0 起算**，所以 `days_since_last_trade = 0` 是合法值）。
只比最終價格的話，parity 一旦不過，分不出是特徵算錯還是線性代數算錯。

所以每個 case 都存三個比對點：

| 比對點 | 欄位 | 對應 Python |
|---|---|---|
| 1. 特徵向量 | `features`（36 float） | `build_feature_vector_from_daily_points(...).as_array()` |
| 2. ridge 中心值 | `ridge_center` | `RidgeParameters.predict(...)` 的原始回傳 |
| 3. 最終分位數 | `price_p10/p50/p90` | 加 `residual_quantiles` → `max(0)` → 單調修正之後 |

建議容差：特徵層 `1e-12`、價格層 `1e-9`（設計書 §6 的規定）。注意 Python 用 `@`（BLAS dot），
Go 若用 naive loop 會有 ULP 級差異，容差不能設 0。

## 檔案

| 檔案 | 說明 |
|---|---|
| `parity-fixture.json` | 主 fixture。14 cases（13 `direct_ridge` + 1 休市 `not_applicable`）。**已驗證與 demo 既有的 `forecast.csv` 逐欄位完全一致** |
| `model-demo.json` | 主 fixture 對應的模型（demo 原檔，2 series × 7 horizons） |
| `parity-fixture-fallback.json` | fallback fixture。14 cases（7 `direct_ridge` + 6 `seasonal_naive` + 1 休市 `not_applicable`） |
| `model-fallback.json` | fallback fixture 對應的模型（重訓，門檻 115） |
| `history.json` | demo 與 fallback fixture 共用的原始歷史觀測點（240 筆，`PricePoint.canonical_dict()` 格式）。Go 測試用它重建 `points_by_date`，餵進 `BuildFeatureVectorFromDailyPoints`/`SeasonalNaiveFromDailyPoints` 做「特徵層」與 `seasonal_naive` 分支的 parity 驗證——沒有這份資料就只能驗 ridge 數學，驗不到特徵建構本身 |
| `calendar.json` | 傳給 `forecast_seven_days` 的三個休市／日曆輸入（`closed_dates_by_market` / `closed_dates_by_market_category` / `verified_calendar_market_categories`）。Go 的編排層 parity test 用它重建休市輸入，才驗得到 `not_applicable` 判定本身而非只驗形狀。**這份資料同時是「休市必須用 (market, category) 兩層鍵」的證據**：`moa:109:A1` 是 N05、`moa:109:LA1` 是 N04，同市場同一天 2026-07-21 只有 N04 休市，用單層 market 鍵會把 A1 一起關掉 |

## 推論公式（Go 端要複製的全部）

```
center = intercept + Σ (x[i] − feature_mean[i]) / feature_scale[i] × coefficients[i]
p10 = center + residual_quantiles[0]
p50 = center + residual_quantiles[1]
p90 = center + residual_quantiles[2]
→ 全部 max(0) → 強制單調遞增（累積 max）
```

**必須逐字複製的一行**（`baseline.py:70`，訓練期）：
```python
scale = np.where(scale < 1e-12, 1.0, scale)
```
零變異欄位鎖 1.0。`model.json` 存的 `feature_scale` 已經是保護過的值，但 Go 端若自行重算
標準化參數就必須複製這行——漏了會在常數欄位除以極小值。

主 fixture 裡就有現成例子：`price_lag_1_missing`（index 4）在這兩條 series 從沒缺過值，
`feature_scale[4] = 1.0`、`coefficients[4] = 0.0`。

## 分支邏輯

`forecasting.py` 的判斷順序：

1. 該 (series, horizon) 沒有 model → `not_applicable`（不建特徵）
2. 建特徵後 `last_price_missing == 1` → `not_predicted` / `insufficient_recent_data`
3. `horizon_model.kind == "direct_ridge"` → ridge 路徑
4. 否則 → `seasonal_naive_from_daily_points(...)`

`kind` 是**訓練期**決定的（`training.py:744`）：
`len(targets) >= min_samples_per_model and proper_training_count >= 2`。

fallback fixture 就是把 `min_samples_per_model` 從 30 調到 115 重訓逼出來的——demo 的
sample_count 是 118(D+1) 遞減到 112(D+7)，所以 D+1~D+4 留在 ridge、D+5~D+7 落 seasonal_naive。

**已實測**：fallback fixture 的 7 個 ridge case 與主 fixture 的對應 case 在
features / ridge_center / p10 / p50 / p90 上**完全一致**。該門檻只決定走哪條分支，不影響
ridge 本身的訓練結果（calibration 切分由 `calibration_fraction` 與 `min_calibration_samples`
決定，兩者未動）。兩份 fixture 的 ridge 部分因此互為驗證。

## 已知限制

- **只有 2 條 series、1 個市場**（`moa:109` 台北一的 A1 與 LA1）。過了不代表 production
  的數千 series 會過。上線前仍需要 production `model.json`（約 154MB，在 VM／Windows
  訓練機上，不在 repo 裡）跑一次全量比對。
- **`history.json` 只涵蓋 2 條 series、1 個市場**，跟其他 fixture 同一個已知限制（見上）。
- **沒有 `insufficient_calibration_data` / `invalid_calibration_lineage` 的 case**。這兩個
  gate 在 `forecasting.py` 裡存在，但 demo 資料觸發不到。
- **`not_predicted` / `insufficient_recent_data` 也沒有覆蓋**（需要 `last_price_missing == 1`，
  即該 series 在 `max_days_since_last_trade` 天內完全沒有成交）。
- 休市只覆蓋到 1 個 case（`moa:109:LA1` D+3，2026-07-21）——但該 case 自 P4 起是**真的由 Go 算出來**的（`calendar.json` 提供輸入），不再只驗形狀。仍未覆蓋到的是 `closed_by_market`（市場層級休市，需要 `is_market_closed` 且 `category_code` 為 NULL 的觀測列，demo 資料裡是 0 筆）。

## 怎麼重新產生

腳本用 monkey-patch 從外部攔截，**不修改 `Hawkr_Transformer_Pricebook` 的任何檔案**
（已用 `git status --porcelain` 與 `diff -rq` 雙重確認）：

腳本在 `regenerate/`。需要 資料團隊那個 repo 的本機 clone 與其 venv（**不在本 repo 內，也不會進
CI**——這是本機重現用的，不是自動化測試的一部分）。

```bash
PB=~/Hawkr_Transformer_Pricebook

# 先複製一份 demo-output 再指過去：RevisionStore 會對 sqlite 開 WAL，
# 不要直接對來源 repo 跑
cp -R "$PB/demo-output" /tmp/pb-demo

cd services/pricebook-service/testdata/parity/regenerate
DEMO_OUTPUT=/tmp/pb-demo "$PB/.venv/bin/python" gen_parity_fixture.py
DEMO_OUTPUT=/tmp/pb-demo "$PB/.venv/bin/python" gen_fallback_fixture.py
```

環境變數：`PRICEBOOK_REPO`（預設 `~/Hawkr_Transformer_Pricebook`）、
`DEMO_OUTPUT`（預設 `$PRICEBOOK_REPO/demo-output`）、`FIXTURE_OUT`（預設本目錄的上一層，
也就是直接覆寫這批 fixture）。

**2026-07-29 已實測**：改成環境變數版本後重跑，四個產物與本目錄現存版本逐 byte 一致。

兩支腳本都會自我驗證：主 fixture 逐欄位對照 demo 既有的 `forecast.csv`，不一致會列出來。
第一版就是靠這個檢查抓到「忘了傳休市日曆參數」——當時 14 個 case 全部產出數字、看起來
完全正常，但其中一個本該是 `not_applicable`。沒有那步比對就會交出一份靜默錯誤的 golden。
