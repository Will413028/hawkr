# A7-1b 封鎖：讀取與寫入語意對照業界基準

Will 2026-10-23：照業界做法（下面結論 1 到 3 全照建議）。

## 業界基準

原始碼量測（shallow clone，commit：lemmy `45b6505800a5`、lemmy-ui `23193d63dd92`、atproto `3b3bb0541199`、mastodon `aafbc4b6b64b`、discourse `23949cfdec87`）；商業平台是二手資料。

| 系統 | 方向 | 被封鎖者在回覆串裡的節點 | 其下別人的回覆 | 計數 | 寫入 | 過濾的層 |
|---|---|---|---|---|---|---|
| Bluesky threads v2 | 雙向 | 丟掉（`packages/bsky/src/views/index.ts:2170-2177`） | 丟掉 | 全域（`data-plane/server/indexing/plugins/post.ts:525-545`，reference 實作） | 協定擋不了 | 應用層 hydration |
| Bluesky v1 | 雙向 | 無內文的 `blockedPost{uri, author}`（同檔 `:1455-1456`） | 丟掉 | 同上 | 同上 | 同上 |
| Mastodon | 雙向 | 丟掉（`app/models/concerns/status/threading_concern.rb:71-94`、`app/lib/status_filter.rb:11-14`） | API 保留，web 走不到（`selectors/contexts.ts:22-57`） | 全域 | 回覆前 `authorize(@thread, :show?)`；讚與轉推 `!blocking_author?`（`status_policy.rb:11,21,25`）；`show?` 只看作者封鎖了觀看者 | 應用層逐筆 |
| Lemmy | 單向（觀看者→作者） | 丟掉（`crates/db_views/comment/src/impls.rs:289-291`） | API 保留，lemmy-ui 建樹時丟掉（`src/shared/utils/app.ts:110-115`） | 全域（trigger） | 不擋，只擋通知與私訊 | DB join |
| Discourse ignore | 單向、軟性 | 可展開佔位「N hidden replies」（`lib/topic_view.rb:1140-1147`） | 保留（線性串） | 全域（讚依觀看者扣） | 不擋，只擋私訊與通知 | DB＋前端 |
| Reddit | 雙向 | 封鎖者：摺疊；被封鎖者：[unavailable]【二手】 | 未查 | — | 擋【二手】 | — |
| 店或頁封鎖 | 單向 | X 2024：被封鎖者仍看得到、不能互動；Etsy：不能傳訊、仍可購買；Facebook 粉專新版：看不到【皆二手】 | | | | |

Bluesky 從 v1 的佔位改成 v2 的丟掉。可展開的佔位是 ignore／mute 的軟性語意。

## 現行設計的對照

| 項目 | 現行 | 基準 | 判定 |
|---|---|---|---|
| 買家↔買家雙向互不可見 | D2 | Bluesky、Mastodon、Reddit | 一致 |
| 買家→店家：看不到該店貼文、討論區 | D2 | Facebook 粉專新版 | 一致 |
| 店家→買家：單向，只擋寫入 | D2 | X 2024、Etsy | 一致 |
| 計數不依觀看者 | A7-D11 | 四個系統全域 | 一致 |
| SQL 述詞內 join buyer 的 `pub_blocked_buyers()`、`pub_blocked_stores()`，不建投影 | A7-1b 範圍 | Lemmy DB join；過濾在分頁之前（SD15） | 一致 |
| 讀不到就不能寫 | SD3 | Mastodon `show?` | 一致 |

## 結論

1. **被封鎖者的頂層留言或討論區頂層項目，連同底下的回覆，對觀看者整則不出現，不留佔位**（Bluesky v2、Mastodon、Lemmy）。刪除與隱藏的佔位不變：那是對所有人一致的狀態，佔位保住第三人的對話；封鎖是觀看者選擇不看這個人，連帶不看與他的對話。代價：觀看者看不到店家或別人回給他的內容，含自己在那串裡的回覆。不加 enum 值、不改 `api/` 契約。
2. **不採 Bluesky 的第三方封鎖規則**（回覆者與上層或根作者有封鎖時，回覆對所有人隱藏，`hydration/hydrator.ts:926-960`）：D2 的語意是雙方互不可見，不要求第三人也看不到。接受的剩餘風險：封鎖之前留下的舊回覆，其他人仍看得到。觸發：營運出現「封鎖後舊回覆被拿來騷擾」的檢舉。
3. **封鎖不影響計數**：所有人看到同一個數字，觀看者看到的則數可能較少。

## 實作要點（照基準，不另問）

- 封鎖是獨立的條件，不併進 `Viewable()`／`Counted()`：被封鎖者的頂層項目不能因為「有可見回覆」變成佔位（`reads/comments.go` 的 `hasVisibleReply` 與 `state`、`reads/forum.go` 同）。
- 讀取與寫入檢查一律帶觀看者本人，不用 `visibility.Anonymous`（含 `ReplyTargetIsReadable`、`ForumReplyTargetIsReadable`、`PostIsPublic`），讀寫對「看得到」的定義一致。
- 店主以店家身分讀自己的店（`ListOwner*`）不套個人封鎖（D2）；觀看者自己的內容列表不套封鎖。
- 「回覆 @某人」的目標看不到時，比照目標已刪除。
- 封鎖後不通知對方：通知過濾在 A7-3（Lemmy、Discourse 的封鎖至少擋通知與私訊）。
