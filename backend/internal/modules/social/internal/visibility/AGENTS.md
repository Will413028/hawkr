# social/internal/visibility

「公開可讀」與「可見」的唯一定義。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/social.md`。

## 不變量

- 公開可讀 = 店家可曝光、貼文已發布且沒有被內容下架、旗標 `counted` 為真；規則只在 `visibility`，公開表只經它的入口取得。守門：`TestPublicTablesAreOnlyReachedThroughVisibility`、`TestRawSQLEscapesOnlyAppearInGeneratedPubWrappers`、`TestEveryQueryIsRegistered`。規格：`docs/modules/social.md` §4。
- 個人封鎖是依觀看者的獨立條件，不併進 `counted`、不影響計數；匿名語句沒有封鎖子句。守門：`TestAnonymousReadsCarryNoBlockClause`、`TestViewerReadsCarryTheBlockPredicates`、`TestBlocksDoNotChangeCounts`。規格：§5。
