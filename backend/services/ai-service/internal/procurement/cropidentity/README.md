# cropidentity — 品項身分解析

買家打的一句品項文字（「蘋果」「小白菜」「豬絞肉」）解析成平台的分類身分，以及由此產生的消歧中斷。對外只有三個入口：

| 入口 | 用途 |
|---|---|
| `Resolver` | `Resolve(ctx, itemText) (DisambiguationResult, error)`；唯一實作是 `*DisambiguationUseCase` |
| `ComputeGate` | 一輪的 gate 判定，回 `GateResult` |
| `GateResult` | `Evaluated()` 說這一輪有沒有做出新判定；`ApplyTo` 把結果寫進 slots 與 patch |

`Module` 是 fx 接線，掛在 `internal/procurement/module.go`。

## 解析路徑

```
understandQuery（原文 → 變體）
  → taxonomyRetriever（分類樹；人工 synonym 命中＝pinned）
  → fuse.RRF（單一來源，名次即 retriever 的名次；pinned 排最前）
  → decide（匹配品質分層：恰好一個強訊號定案／唯一候選定案／其餘問）
  → 分類卡由 adaptive facet 決定問哪一層（facet.go）：L2 分組 ≥2 且最大組 <80% 問 L2，否則下鑽最大組問 L3；
    下鑽到模型建議的 L2 時才展開（expandGroup：一個子類直接問、多個走確定性比對→LLM ITEM）
```

沒有 pricebook：建構子只收分類樹與模型，`no_pricebook_test.go` 守解析路徑不得 import pricebook 的 client 或型別。
`Resolve` 只有兩種結果：`NeedsDisambiguation`（要買家挑分類）與 `Unavailable`（沒有更多要問的：分類已確認，
`Selection.CategoryID` 非空；或查無，`Selection` 沒有分類 → 買家自行描述品項）。分類已確認時 gate 直接通過並產生 `CATEGORY_CONFIRMED` 解析結果，不出卡片。

錯誤處理：分類樹是唯一的確定性來源，查詢失敗往上拋（`WithTransientRetry` 當場重試一次，再失敗交給 crop_gate 的跨輪重試）；
LLM 失敗降級成沒有建議。LLM 只在 `expandGroup` 與買家自由輸入找不到確定性命中時才呼叫；
模型建議的東西一律要買家確認：展開後即使只剩一個也出卡，逃生艙不帶模型猜的分類當「已確認」。
每條規則各有一支 `resolver_v2_test.go`／`facet_test.go` 釘住。

買家選完之後的 resume（`ResolveSelection`）走**同一條召回**，差別只在多一個約束：選了大品項就把候選收斂到
那個 L2 底下（`narrowToGroup`）再交給 `categoryCard`；收斂後空手才展開那個 L2。自由輸入同樣走召回，但**一律
出卡不定案**——打字是搜尋，只有明確按下選項才算確認。選到 L3 時直接定案，不再召回。

離線評測：`goldenset/offline_eval_test.go` 用種子分類樹（`backend/infra/seed-data/04-product.sql`）逐筆對
`goldenset/testdata/golden_queries.yaml` 呼叫 `Resolve`，不打任何外部服務。

## 品項解析錯了，修哪裡

**先判斷錯在哪一層**，兩條修法不可互相替代：

1. **某個別名對應到錯的分類** → 往 `internal/cache` 的 `synonyms` 加一行人工覆寫。那一層永遠壓過生成層，不需要重跑任何東西。`category_alias_gen.go` 是生成物，**不得手改**（該檔開頭自己寫著這件事）。

2. **整批別名品質下滑**（不是單一條） → 重跑生成器：

   ```bash
   cd services/ai-service && go run ./cmd/category-alias-gen
   ```

   跑完看 stderr 的丟棄報告，判的是**理由分布有沒有系統性問題**，不是逐列確認。單一條錯誤別名不要用這條路——用第 1 條。

## 為什麼 `crop_selection_resume.go` 不在這裡

它引用 resume 協議的 runtime 符號（`AnswerInput`、`CropGateOutput` 與中斷重建），屬於 turn runtime 而不是品項身分，留在 `internal/procurement/usecase`。
