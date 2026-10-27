# market_registry.v1 / taiwan_boundary（category-aware 市場地理登錄）

issue #2248（spatial-surface BFF）的基礎層——`internal/domain/geography` 用
這兩份鏡射檔解析 `(market_code, category_code, valid_on)` → 實體設施座標與
所屬縣市。設計書：`docs/superpowers/specs/2026-08-08-pricebook-market-geography-registry-design.md`。

## 為什麼需要兩份檔案而不是延用既有 `market` 表

`market` 表的 `market_code` 是 `UNIQUE`，一列只帶一組座標。但 `moa:400`／
`moa:514`／`moa:800` 三個市場代碼在蔬果（N04/N05）與花卉（N06）類別下是
「不同實體地址」，不是同一地點的不同休市排程——`market_registry.v1.csv` 的
`moa-facility-400-produce`（臺中市西屯區）與 `moa-facility-400-flower`
（臺中市南屯區）就是實測到的兩個不同座標。`market` 表的單一 market_code+
lat/lng 結構無法承載這個事實。

## 資料來源與授權

兩份檔案皆逐位元組鏡射自 `<org>/Hawkr_Transformer_Pricebook`
（該 repo owner: hawkr-price-ai），commit
`1374695cd071cafd112f244abc68101b7ef555f6`：

| 檔案 | 上游路徑 | 上游 contract | 授權 |
|---|---|---|---|
| `market_registry.v1.csv` | `reference/market_registry.v1.csv` | `contracts/reference/market_registry.v1.yaml`（自稱 `hawkr_authoritative_market_registry`） | 上游內部資料，非公開授權（同 repo 內部使用） |
| `taiwan_boundary.taiwan-atlas-2021.9.20.geojson` | `reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson` | `contracts/reference/taiwan_boundary.v1.yaml` | 衍生自 [dkaoster/taiwan-atlas](https://github.com/dkaoster/taiwan-atlas)（MIT，Copyright (c) 2020 Daniel Kao），上游資料為內政部國土測繪中心－直轄市、縣市界線（TWD97 經緯度），政府資料開放授權條款－第 1 版 |

這個套件不重新蒐集或修正任何地理資料，只鏡射、驗證、查詢。`V`/`F`/`L` ↔
`N04`/`N05`/`N06` 的對照已在 `internal/domain/forecast/calendar.go` 鏡射過
（用於休市日曆），這裡直接沿用，不重複定義。

## manifest 的用途

`market_registry.v1.manifest.json` / `taiwan_boundary.v1.manifest.json` 各自
記錄對應檔案的 sha256、列數/feature 數與上游 commit。`geography.LoadMarketRegistry`
/ `geography.LoadBoundary` 載入時會重算 sha256 並與 manifest 比對，**不符時
回 error、不是警告**——理由同 `product_crop_seed.csv` 既有慣例：檔案被改過
但 manifest 沒同步更新，代表繞過了審查流程。

## 改動流程（上游發布新 snapshot 時）

1. 確認新的上游 commit SHA：
   ```bash
   gh api repos/<org>/Hawkr_Transformer_Pricebook/commits/main --jq '.sha'
   ```
2. 用該 SHA 重新執行本檔案所屬 plan 的 Task 1 Step 1/5（`gh api .../contents/<path>?ref=<新SHA>`），
   覆蓋兩份鏡射檔。
3. 重算 sha256（`shasum -a 256 <file>`），更新對應 `.manifest.json` 的
   `sha256`／`row_count`／`feature_count`／`upstream_commit`。
4. 跑 `cmd/pricebook-geography-check` 確認新快照下每個 active 設施仍能正確
   解析（見 `internal/domain/geography` 的套件文件）。
5. 四個檔案（兩份資料 + 兩份 manifest）一起 commit（缺一個會讓下次
   `geography.Load` 直接拒收）。

目前沒有自動偵測「上游有新版本」的機制——這是手動流程，等這個成為真正的
維運痛點再補自動化（同設計書 Open Follow-ups）。
