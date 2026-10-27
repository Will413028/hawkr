# @hawkr/api

對外 API（根目錄 `api/` 的 Protobuf＋Connect）的 TypeScript 共用套件，website 與 admin 共用。對應 Dart 版是 `packages/hawkr_api`。

## 內容

| 路徑 | 內容 |
|---|---|
| `src/gen/` | `buf generate` 的輸出（protoc-gen-es，`target=ts`，含 `google/rpc/error_details`）。**不得手改** |
| `src/connect/errors.ts` | 錯誤分類器 `classifyError`、轉接層 `factsFromError`、`userMessage`、一定有字的 `errorText`（取消當 internal，與 `submitError`、Dart 的 `errorText` 同一條規則；預設文字由呼叫端注入的 `DefaultErrorText`，型別只在本套件定義，文字在 `@hawkr/ui-text`）、`metadataList`（逗號連接的清單值）；`ErrorFacts.metadata` 與 reason 分支的 `ErrorDecision.metadata` 帶 `ErrorInfo.metadata`（ADR `backend/docs/adr/2026-09-25-api-error-model.md` D7） |
| `src/connect/submit.ts` | 寫入失敗的呈現分類 `submitError`（版本衝突、欄位錯誤、文字）與 `fieldMessage` |
| `src/connect/retry.ts` | `retryInterceptor`：瀏覽器預設退避重試最多兩次；`{ server: true }` 只立即重試一次、不 sleep |
| `src/proto/date.ts` | `parseDate`（只收存在的 `YYYY-MM-DD`）、`formatDate`；同 Dart `packages/hawkr_api/lib/dates.dart` |
| `src/proto/field-mask.ts` | `fieldMask(schema, paths)`：依訊息 descriptor 檢查 proto 路徑後建立 `update_mask`；同 Dart `lib/field_mask.dart` |
| `src/text/server-text.ts` | `trimGoSpace`（同 Go `strings.TrimSpace`）與 `goTrimmedRuneCount`（修剪後以 code point 計）：後端的文字長度驗證規則，client 用同一組規則計數與阻擋；同 Dart `packages/hawkr_api/lib/go_text.dart` |

app 專屬的東西留在各 app：transport、`authedFetch`、`sellerSuspendedInterceptor`、`authExpiredInterceptor`、`reportInterceptor`（website）、`sessionInterceptor`、`adminTransport`。

## 使用

只發佈 TS 原始碼，不 build。消費端在 `next.config.ts` 設 `transpilePackages: ["@hawkr/api"]`。用 subpath import，**沒有 barrel index**（website 不會因為 import 一個模組而帶進 admin 的生成碼）：

```ts
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { classifyError } from "@hawkr/api/connect/errors";
import { retryInterceptor } from "@hawkr/api/connect/retry";
```

## 生成

改 `api/` 的 proto 後，在根目錄執行：

```sh
pnpm --filter @hawkr/api gen:api
```

buf CLI 的版本固定在 `api/.buf-version`；`buf.gen.yaml` 的 `clean: true` 會先清空 `src/gen`。CI（`.github/workflows/api-contract.yml` 的「TS 套件」job）重新生成並檢查 drift。

## 檢查

```sh
pnpm --filter @hawkr/api exec tsc --noEmit
pnpm --filter @hawkr/api exec biome check .
pnpm --filter @hawkr/api test   # vitest；錯誤分類以 api/testdata/error-dispatch.json 驅動
```
