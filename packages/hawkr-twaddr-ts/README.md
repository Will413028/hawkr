# @hawkr/twaddr

內政部 8 碼鄉鎮市區代碼表與查詢（縣市 5 碼、鄉鎮市區 8 碼、郵遞區號），website 與 admin 共用。對應 Dart 版是 `packages/hawkr_twaddr`，Go 版是 `backend/internal/platform/twaddr`（ADR `backend/docs/adr/2026-10-04-address-town-code.md`）。

## 內容

| 路徑 | 內容 |
|---|---|
| `src/twaddr.gen.ts` | 資料表（`COUNTIES`、`TOWNS`），由 `backend/internal/platform/twaddr/generate.go` 從 CSV 生成。**不得手改** |
| `src/lookup.ts` | 手寫的型別與查詢：`townByCode`、`countyByCode`、`townsByCounty`、`countyCodeOfTownCode`、`postalCodeByTownCode`、`formatAddress` |

## 使用

只發佈 TS 原始碼，不 build。消費端在 `next.config.ts` 設 `transpilePackages: ["@hawkr/twaddr"]`。用 subpath import，沒有 barrel index：

```ts
import { townsByCounty } from "@hawkr/twaddr/lookup";
```

## 生成

改 CSV 後，在 `backend/internal/platform/twaddr` 執行 `go run generate.go`（一次輸出 Go、TS、Dart 三份）。CI（`.github/workflows/twaddr-generated-diff.yml`）重新生成並檢查 drift。

## 檢查

```sh
pnpm --filter @hawkr/twaddr lint
pnpm --filter @hawkr/twaddr test
```
