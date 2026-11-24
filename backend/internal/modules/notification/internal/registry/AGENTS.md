# notification/internal/registry

型別註冊表與文案：每個型別一個 `Definition[P]`，`Define` 變成 `Entry`。

## 不變量

- 每個型別有冪等鍵來源（意圖冪等鍵＝型別名稱＋事件來源身分）、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。守門：`registry.Validate`、`TestEveryRegisteredTypePassesTheSelfCheck`、`TestSelfCheckRejectsBrokenDefinitions`。
- 每個型別在 zh-TW、en 有固定輸出，en 沒有執行期退回 zh-TW，文字的語系取自 `Rendered.Locale`、各部分不一致即錯誤；冪等鍵與聚合鍵過長時改存型別名稱加雜湊。守門：`TestGoldenRendering`、`TestGoldenIdempotencyKeys`、`TestEveryRenderableValueHasEnglishText`、`TestRenderRejectsMixedLocales`、`TestLongKeysAreHashedToFit`。
- 收件人解析有三種寫法，互斥：`Recipient`（參數裡已有帳號）、`Expand`（只帶 `store_id`、`buyer_id`，worker 經 `Resolver` 解析，回 nil 表示沒有存活收件人，形狀不合法是 `ErrBadParams` 永久錯誤）、`Broadcast`＋子型別 `Spawned`（fan-out，冪等鍵由派送決定）。`Expand` 型別的 actor 以 `Principal`（身分面加該面的號碼，買家與店是兩個號碼空間）宣告，供封鎖檢查與自我略過。`Expand` 與 `Spawned` 的範例參數要能經自檢用的 Resolver 展開成合法收件人；母意圖不能有呈現欄位。
- 整份註冊表的測試同時對正式清單與含一組 Broadcast＋Spawned 的清單跑（`wholeRegistries`）。
- 守門：`TestSelfCheckRejectsBrokenResolutionDefinitions`、`TestEveryBroadcastChildIsASpawnedRegisteredType`、`TestExpandRejectsAnInvalidShapeAsPermanent`。
- 內容側型別：深連結的 store_code 在讀取時經 `Load` 批次取得（店家號碼不合法的列留空連結，不讓整頁失敗）；貼文相關通知連到 `/sellers/{storeCode}/posts`；`post_published` 母意圖的冪等鍵是貼文 ID（重新發布不再通知）。守門：`TestGoldenContentRendering`、`TestContentLoadDropsInvalidStoreIDsBeforeTheLookup`、`TestPostPublishedIdempotencyKeyIgnoresTheEventID`。
- 型別清單按領域分檔（`account.go`、`announcement.go`、`content.go`、`governance.go`），`catalog.go` 的 `all` 只串各領域的清單；新增型別只改自己領域檔。
- 治理與交易側型別在 `governance.go`：店家面型別以 `StoreRecipient`、買家面以 `BuyerRecipient` 在 worker 解析；參數只放 ID、列舉字串與事件時間，列舉在 worker 的 `Expand` 就驗證（不認得的值是 `ErrBadParams`，不寫文案壞掉的列）；來源模組的列舉值與註冊表常數的一致由 intake 的 `TestRegistryValuesMatchTheOwnersEnums` 對照。確認信型別（`store_pending_deletion`、`buyer_deletion_requested`）只有 email 渠道，位址在寄送時才讀。
- 走意圖管線的型別只有一個受眾，Spawned 型別沒有自己的冪等鍵：所以檢舉人的通知拆成買家與店家兩個型別（`report_received`、`store_report_received`、`report_resolved`、`store_report_resolved`），`report_resolved` 一律經 fan-out 母意圖建立（`seller_report_resolved` 只展開一位，`moderation_case_closed_buyers`、`moderation_case_closed_stores` 各讀自己那一邊的檢舉人，Target.ID 與游標是檢舉人號碼）。措施請求的裁定（接受與駁回）都是 `store_measure_request_decided`，參數帶 `Outcome`，文案依種類與裁定（D3）。
- 守門：`TestGoldenGovernanceRendering`、`TestGoldenGovernanceIdempotencyKeys`、`TestCaseClosedFanoutReadsOnlyItsSideAndPagesFromTheCursor`、`TestExpandRejectsValuesThatHaveNoText`、`TestEveryAppealPathRendersAndAnUnknownOneFails`。
