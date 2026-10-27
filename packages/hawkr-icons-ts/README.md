# @hawkr/icons

Material Symbols（Apache-2.0）的 SVG 圖形與繪製元件，website 與 admin 共用。兩支 App 用 Flutter 內建的 `Icons`，三端圖形同一個家族、授權同一份。

## 內容

| 路徑 | 內容 |
|---|---|
| `src/names.ts` | 手寫：用到的 Material Symbols 名稱 |
| `src/material-symbols.gen.ts` | 名稱 → outlined 與 filled 的 path data，由 `tools/gen.ts` 從 `@material-symbols/svg-400`（鎖定 0.48.1）生成。**不得手改** |
| `src/material-symbol.tsx` | `MaterialSymbol` 元件與 `MaterialSymbolName` 型別 |
| `LICENSE` | 上游的 Apache-2.0 全文（登記在根目錄 `THIRD_PARTY_NOTICES.md`） |

## 使用

只發佈 TS 原始碼，不 build。消費端在 `next.config.ts` 設 `transpilePackages: ["@hawkr/icons"]`。用 subpath import，沒有 barrel index：

```tsx
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

<MaterialSymbol name="search" />                      // 裝飾用，aria-hidden；沒傳 size 時是 1em，跟著字級
<MaterialSymbol name="search" size={20} />            // number 是 px，string 是任何 CSS 長度
<MaterialSymbol name="star" fill title="已收藏" />     // filled 版，有 accessible name
```

website 的產品 UI 不直接用它，一律經 `website/components/shared/AppIcon.tsx`。

## 新增圖示

在 `src/names.ts` 加名稱（要與上游 `outlined/<name>.svg` 同名），然後在 repo 根目錄執行：

```sh
pnpm --filter @hawkr/icons gen
```

輸出只由名稱清單與鎖定的上游版本決定，重跑沒有 diff；`tests/drift.test.ts` 在記憶體重跑生成器並與 checked-in 的生成檔逐字比對，CI（`.github/workflows/ts-packages.yml`）以此守住。

## 檢查

```sh
pnpm --filter @hawkr/icons lint
pnpm --filter @hawkr/icons test
```
