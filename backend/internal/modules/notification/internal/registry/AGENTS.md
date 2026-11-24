# notification/internal/registry

型別註冊表與文案：每個型別一個 `Definition[P]`，`Define` 變成 `Entry`。

## 不變量

- 每個型別有冪等鍵來源（意圖冪等鍵＝型別名稱＋事件來源身分）、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。守門：`registry.Validate`、`TestEveryRegisteredTypePassesTheSelfCheck`、`TestSelfCheckRejectsBrokenDefinitions`。
- 每個型別在 zh-TW、en 有固定輸出，en 沒有執行期退回 zh-TW，文字的語系取自 `Rendered.Locale`、各部分不一致即錯誤；冪等鍵與聚合鍵過長時改存型別名稱加雜湊。守門：`TestGoldenRendering`、`TestGoldenIdempotencyKeys`、`TestEveryRenderableValueHasEnglishText`、`TestRenderRejectsMixedLocales`、`TestLongKeysAreHashedToFit`。
