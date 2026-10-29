# social 模組

## 職責

貼文、留言、讚、店家討論區、hashtag、計數、瀏覽與分享，以及買家的社群處分（`sanctions`）與隱藏紀錄（`hides`）。目前只有根套件 API 與 job，沒有對外入口。行為規格的唯一來源是 `docs/modules/social.md`。

## 擁有的資料

schema `social`（`db/schema.sql`）：貼文與 hashtag、留言與讚、討論區、`hides`、`sanctions`、`inactive_authors`（買家刪帳的複本）、`content_takedowns`（seller 內容下架措施的複本）、三張 stats 表、瀏覽與分享的去重表、media 引用的同步狀態；函式與 trigger 在 `db/objects.sql`，沒有 view。完整表單見 `docs/modules/social.md` §1。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，不發布 `pub_*`（`internal/pubsql` 是讀 buyer、seller 的 `pub_*` 的包裝）。對外是 Go 公開 API 與模組事件（`PostPublished`、`CommentCreated`、`CommentReplied`、`PostLiked`），形狀見 `docs/modules/social.md` §1。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與事件。
- `internal/model` 型別與共用規則；`post`、`comment`、`forum`、`engage`、`sanction`、`stats`、`purge` 各管自己的 use case；`actor` 推導互動者；`hashtag`、`textcheck` 正規化輸入；`mediaref` 同步圖片引用；`readcache` 排快取失效；`store` 是 sqlc 的查詢套件。
- `visibility` 是「公開可讀」「可見」唯一的定義；`reads` 是帶這些規則的 go-jet 讀取（登記表加守門測試）；`jet` 與 `pubsql` 是 `make jet` 的生成物，`text` 的 `text_gen.go` 是 `make uitext` 的生成物，都不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。依賴表：`buyer`、`seller`、`media`。

## 不變量

- 公開可讀 = 店家可曝光、貼文已發布且沒有被內容下架、旗標 `counted` 為真；規則只在 `visibility`，公開表只經它的入口取得。守門：`TestPublicTablesAreOnlyReachedThroughVisibility`、`TestRawSQLEscapesOnlyAppearInGeneratedPubWrappers`、`TestEveryQueryIsRegistered`。規格：`docs/modules/social.md` §4。
- 個人封鎖是依觀看者的獨立條件，不併進 `counted`、不影響計數；匿名語句沒有封鎖子句。守門：`TestAnonymousReadsCarryNoBlockClause`、`TestViewerReadsCarryTheBlockPredicates`、`TestBlocksDoNotChangeCounts`。規格：§5。
- 買家身分的寫入一律經 `actor.Resolver` 的 `ForWrite`／`ForAuthor`／`ForBuyerWrite` 檢查店家封鎖；`Deps.StoreBlocks` 缺了 `New` 就 panic。守門：`TestNewSocialWiresStoreBlocks`（`cmd/hawkr`）、`TestBuyerBlockingAStoreHidesItsContentAndStopsWrites`。規格：§5。
- 計數只由 trigger 與 `reevaluate_*` 維護，「是否計入」只有那一份定義；stats 有 `CHECK (>= 0)`。守門：`TestStatsMatchTheDatasetAfterRandomWrites`、`TestConcurrentReevaluatesDoNotCountTwice`、`TestReevaluateHasNoDeadlocksAndEndsConsistent`、`TestStatsRejectNegativeCounts`。規格：§6。
- 每日對帳先修旗標再覆寫計數，且不蓋掉並行的加減。守門：`TestReconcileRepairsFlagsBeforeCounts`、`TestRecountDoesNotOverwriteConcurrentLikes`。規格：§6。
- 買家刪帳的事實有複本，由訂閱者在同一個 transaction 同步寫並重新求值。守門：`TestBuyerDeletionRequestUncountsTheContentInTheSameTransaction`、`TestDirectCloseStillHidesUncountsAndPurges`。規格：§7。
- 內容下架的事實有複本 `content_takedowns`，由 seller 措施事件的同 transaction 訂閱者維護，每日對帳雙向核對 `seller.pub_content_takedowns()`；建立時訂閱者核對內容存在、未刪除、作者是措施的店家（`TakedownContentError`）。守門：`TestTakedownRejectsContentThatIsNotTheMeasureStoresOwn`、`TestReconcileAddsTheTakedownsTheSubscriberMissed`。規格：§4、§6。
- 跨模組鎖順序見 `backend/AGENTS.md` 單體規則 4（social 在 seller 之後）；本模組內：內容列 → 貼文 stats → 父項 stats，各依 ID 由小到大。守門：`TestMarkAuthorInactiveLocksStatsInTheSameOrderAsTheTriggers`、`TestEditLocksTheBuyerBeforeTheContentRow`。規格：§6、§7。
- 隱藏與還原先鎖目標內容列。守門：`TestDirectHideRowRacingADeleteCountsOnce`。規格：§8。
- 回覆的 partial index 述詞是旗標。守門：`TestReplyIndexesAreOnTheCountedFlag`。規格：§4。
- keyset 游標是單一列比較，沒有布林開關。守門：`TestKeysetQueriesCompareOneRowAndHaveNoBooleanSwitch`、`TestKeysetSQLCQueriesHaveNoBooleanCursorSwitch`。規格：§4。
- 使用者面的句子都在 `internal/text`，`apperr.*` 不帶字串字面值。守門：`TestSocialApperrCallsCarryNoSentenceLiterals`（`internal/arch`）。規格：§3。
- 隱藏、還原、警告的理由先 `trimtext.Trim`。守門：DB 的 `trimtext.Check` CHECK、`TestTrimmedTextChecksUseTheCanonicalPattern`（`cmd/hawkr`）。規格：§8。
- 熱門標籤的窗口最長 30 天。守門：`TestTrendingHashtagsWindowIsCappedAtThirtyDays`。規格：§2。

## 不負責

- 檢舉、案件與處置：moderation；social 只存隱藏與警告的紀錄，案件以不透明的 `source_ref` 參照。
- 排程發布、Connect 與 handler、推播 relay：尚未在本模組（見 `docs/plans/2026-10-18-a7-social.md`）。
- 店家與買家的名稱、頭像：讀取時由 seller、buyer 批次補，social 不存快照。

## 決策紀錄

從 social-service 沿用的每個機制、當時的決定與理由，以及與 social-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-10-23-social-counts-in-stats-tables.md`（計數、旗標與買家刪帳複本，含文末兩則修訂）、`docs/adr/2026-10-17-social-actor-identity.md`、`docs/adr/2026-09-27-module-internal-structure.md`（go-jet 讀取的修訂）。
