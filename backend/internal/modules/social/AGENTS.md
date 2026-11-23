# social 模組

## 職責

貼文、留言、讚、店家討論區、hashtag、計數、瀏覽與分享，以及買家的社群處分（`sanctions`）與隱藏紀錄（`hides`）。行為規格的唯一來源是 `backend/docs/modules/social.md`。

## 擁有的資料

schema `social`（`db/schema.sql`；函式與 trigger 在 `db/objects.sql`，沒有 view），依子領域：貼文與 hashtag、排程發布（`post_schedules`）、留言與讚、討論區、計數、處分與隱藏、買家刪帳與內容下架的複本、瀏覽與分享。貼文圖的資產 ID 是 UUID，資產由 media 擁有。表單見規格 §1。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，不發布 `pub_*`（`internal/pubsql` 是讀 buyer、seller 的 `pub_*` 的包裝）。對外是 Go 公開 API 與模組事件，形狀見 `backend/docs/modules/social.md` §1。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與事件。
- `visibility` 是「公開可讀」「可見」唯一的定義；`reads` 是帶這些規則的 go-jet 讀取（登記表加守門測試）；`jet` 與 `pubsql` 是 `make jet` 的生成物，`text` 的 `text_gen.go` 是 `make uitext` 的生成物，都不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。依賴表：`buyer`、`seller`、`media`。

## 不變量

- 買家刪帳的事實有複本，由訂閱者在同一個 transaction 同步寫並重新求值。守門：`TestBuyerDeletionRequestUncountsTheContentInTheSameTransaction`、`TestDirectCloseStillHidesUncountsAndPurges`。規格：§7。
- 內容下架的事實有複本 `content_takedowns`，由 seller 措施事件的同 transaction 訂閱者維護，每日對帳雙向核對 `seller.pub_content_takedowns()`；建立時訂閱者核對存在的內容作者是措施的店家（`TakedownContentError`）；已軟刪的自家內容與已清除的內容成立但不寫複本（A7-D7 項目 10；已清除內容的歸屬由 moderation 以案件快照核對）。守門加 `TestTakedownOfPurgedContentSucceedsWithoutReplica`。守門：`TestTakedownOfDeletedOwnContentSucceedsWithoutReplica`、`TestTakedownRejectsContentThatIsNotTheMeasureStoresOwn`、`TestReconcileAddsTheTakedownsTheSubscriberMissed`。規格：§4、§6。
- 跨模組鎖順序見 `backend/AGENTS.md` 單體規則 4（social 在 seller 之後）；本模組內：內容列 → 貼文 stats → 父項 stats，各依 ID 由小到大。守門：`TestMarkAuthorInactiveLocksStatsInTheSameOrderAsTheTriggers`、`TestEditLocksTheBuyerBeforeTheContentRow`。規格：§6、§7。
- 代行寫入（A7-D27）：貼文寫入方法明確收 `actor *delegatedactor.Actor`（店主傳 nil），在自己的 transaction 內為每個成功寫入發一般領域事件（`PostCreated`、`PostUpdated`、`PostPublished`、`PostUnpublished`、`PostScheduled`；冪等 no-op 發 `PostPublishRepeated`、`PostUnpublishRepeated`），店主寫入也發、Actor 為 nil。Actor 非 nil 時以 `events.PublishRequired(…, delegatedactor.AuditHandler)` 發布，指名的稽核 handler 不在就失敗關閉；代行建立只能是草稿。守門：`TestPostWritesPublishOneGeneralEventEachAndFailClosedWithoutASubscriber`。
- 排程發布：唯一的真相是 `post_schedules`，River job 只是到點的觸發器；鎖順序貼文列 → 排程列，所有取消都先鎖貼文列。別的模組（delegation）取消排程必須在自己的 transaction 內、鎖或改自己的列「之前」呼叫 `CancelScheduledBy`、`CancelDelegatedSchedulesOfStore`（social 在 delegation 之前）。細節見 `internal/post/AGENTS.md`。規格：§2、§6。
- 使用者面的句子都在 `internal/text`，`apperr.*` 不帶字串字面值。守門：`TestSocialApperrCallsCarryNoSentenceLiterals`（`internal/arch`）。規格：§3。
- 單一子領域的不變量在 `internal/<子領域>/AGENTS.md`（碰該目錄時才載入）；跨子領域的才放這裡。

## 不負責

- 檢舉、案件與處置：moderation；social 只存隱藏與警告的紀錄，案件以不透明的 `source_ref` 參照。
- 推播 relay：在 `internal/legacy/socialrelay`（A7-3，A8 刪除），本模組只發事件與提供 `CommentForRelay`；代行排程的到點授權由 delegation 實作、`cmd/hawkr` 注入，social 不 import delegation。
- 店家與買家的名稱、頭像：讀取時由 seller、buyer 批次補，social 不存快照。

## 決策紀錄

從 social-service 沿用的每個機制、當時的決定與理由，以及與 social-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-10-23-social-counts-in-stats-tables.md`（計數、旗標與買家刪帳複本，含文末兩則修訂）、`docs/adr/2026-10-17-social-actor-identity.md`、`docs/adr/2026-09-27-module-internal-structure.md`（go-jet 讀取的修訂）。
