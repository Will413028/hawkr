# notification/internal/registry

型別註冊表與文案：每個型別一個 `Definition[P]`，`Define` 變成 `Entry`。

## 不變量

- 每個型別有冪等鍵來源（意圖冪等鍵＝型別名稱＋事件來源身分）、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。守門：`registry.Validate`、`TestEveryRegisteredTypePassesTheSelfCheck`、`TestSelfCheckRejectsBrokenDefinitions`。
- 每個型別在 zh-TW、en 有固定輸出，en 沒有執行期退回 zh-TW，文字的語系取自 `Rendered.Locale`、各部分不一致即錯誤；冪等鍵與聚合鍵過長時改存型別名稱加雜湊。守門：`TestGoldenRendering`、`TestGoldenIdempotencyKeys`、`TestEveryRenderableValueHasEnglishText`、`TestRenderRejectsMixedLocales`、`TestLongKeysAreHashedToFit`。
- 收件人解析有三種寫法，互斥：`Recipient`（參數裡已有帳號）、`Expand`（只帶 `store_id`、`buyer_id`，worker 經 `Resolver` 解析，回 nil 表示沒有存活收件人，形狀不合法是 `ErrBadParams` 永久錯誤）、`Broadcast`＋子型別 `Spawned`（fan-out，冪等鍵由派送決定）。`Expand` 型別的 actor 以 `Principal`（身分面加該面的號碼，買家與店是兩個號碼空間）宣告，供封鎖檢查與自我略過。守門：`TestSelfCheckRejectsBrokenResolutionDefinitions`、`TestEveryBroadcastChildIsASpawnedRegisteredType`、`TestExpandRejectsAnInvalidShapeAsPermanent`。
- 型別清單按領域分檔（`account.go`、`announcement.go`、`content.go`、`governance.go`），`catalog.go` 的 `all` 只串各領域的清單；新增型別只改自己領域檔。
