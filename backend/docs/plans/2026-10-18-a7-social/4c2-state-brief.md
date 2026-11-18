# A7-4c-2 社群畫面的狀態設計簡報（v2，已經獨立審查修正）

## 前提題
被改的檔是否排定重寫、搬移或刪除？否。4c-3（計畫 :537-539）會再碰 `comments_bottom_sheet`、貼文畫面（封鎖入口）並刪手寫 repository 死碼，是在同一批 widget 上加功能，不是重寫；4d 是 seller-app。→ 現在改不是拋棄式工作，而且應在 4c-3 之前做，否則 4c-3 會在舊結構上加入口。

## 範圍（量）
持有社群伺服器狀態的檔：`comments_bottom_sheet.dart` 1054、`merchant_chat_tab.dart` 1164、`merchant_home_tab.dart` 1077、`merchant_posts_feed_screen.dart` 1474、`post_detail_screen.dart` 1373 行，加 `post_feed_controller`、`post_detail_controller`。同一則貼文的讚數／留言數現在存在 feed 的 List（以 index 寫）、detail controller、home tab 的 `_posts` map、預覽卡參數四處。

## 根源
伺服器狀態存在 widget 欄位與多份副本，每個非同步路徑自己寫回。三輪審查 29 項中 R2/R3 有 14 項是時序或殘留狀態，每輪在不同路徑出現：缺的是結構，不是 if。

## 業界基準
1. 伺服器狀態放在以 key 定址的快取，widget 只顯示與發動作（TanStack Query、SWR、Apollo、Relay；Riverpod 官方 AsyncNotifier + family）。
2. 同一實體只存一份（正規化）：Apollo／Relay 以實體 id 存，列表只存 id；任何畫面改了讚數，所有畫面一起變。
3. 觀看者是快取 key 的一部分；登出、換帳號清掉使用者快取（TanStack `queryClient.clear()`、Apollo `resetStore()`）。本 App 通知、優惠券已用「provider watch session 自動重建」（`session_provider.dart:212-213`）。
4. 寫入走集中的 mutation：以 id 樂觀更新、失敗以 id 回滾，回來時若快取已屬於別的觀看者就丟棄。
5. 重新整理失敗保留舊資料（stale-while-revalidate），只把 not-found／permission 當不可見。
6. 先登入再做：多數 App 登入後回原頁、使用者再按一次；本 App `requireLogin` 就是這樣（回傳 false、不補做）。

## 實測（主 session，2026-11-18，臨時測試已刪）
Riverpod 3：AsyncNotifier 的 build `ref.watch` 一個值，mutation await 期間該值改變、build 重跑後，**舊 mutation 的 `ref.mounted` 仍為 true，寫入成功蓋掉新資料**（輸出 `mountedAfterAwait=true value=stale-write`）。→ 不能靠 `ref.mounted`；notifier 要自己記「這份資料屬於哪個觀看者／哪一代」，mutation 回來時比對。這件事集中在一個 base notifier 做一次，不是每個 widget、每條路徑各做。

## 選項
**A. 在現有 widget 狀態上繼續補**（序號延伸到寫入、修 R3）。小。失敗條件：任何新路徑忘了檢查就重演，三輪紀錄證明會漏；四份副本照樣不同步。可逆，但程式會丟掉。

**B. 業界基準：正規化實體快取 + 集中 mutation（推薦）**
- `postEntityProvider(postId)`：每則貼文一份（讚、讚數、留言數、viewer_can_*、viewer_actor）。feed、店家貼文列表、home tab 只存 postId 清單與分頁 token；detail、預覽卡、留言 sheet 都讀同一份實體。
- `commentsProvider(postId)`、`forumProvider(storeId)`：留言以 id 存，回覆以 parentId 分組，分頁在 notifier 內。
- 共用 base：每份快取記錄所屬觀看者與世代；觀看者變（`viewerIdProvider`）就重建並清空；所有 await 回來先比對世代，不符就丟棄——讀取、分頁、回覆、樂觀回滾全走這一處。
- 生命週期：feed 列表 keepAlive（保住已載入頁，現況 `post_feed_controller.dart:25`）；實體與留言 autoDispose（沒人看就釋放）；換觀看者時全部重建。
- 重讀失敗保留舊值；UI 先看 value 再看 error（不讓 `.when` 的錯誤分支蓋掉舊資料）。
- widget 改成 Consumer、只顯示；刪 `ViewerScopedLoads`、`_loadSeq`、`_postsRevision`、手寫 map；`merchant_home_tab` 不再收可為 null 的 `WidgetRef`。
- 改動：大（7 個檔的狀態部分，race 測試改成 `ProviderContainer` 層，較好寫也較好守）。一個 session 可能做不完，拆 4c-2b-1（實體快取 + base + feed／detail／預覽卡）與 4c-2b-2（留言 sheet、討論區）。
- 失敗條件：實體與列表的同步規則沒定好（例如刪貼文時列表沒移除 id）；世代比對漏掉某條 await 路徑——但只有 base 一處要守，可用一支測試涵蓋。
- 推翻條件：量到正規化快取在 feed 長列表上有明顯效能問題，或需要跨裝置即時同步（那時要的是推播，不是本地快取）。
- 可逆：可，沒有理由回去。

**C. 換帳號時重建整個 App**：只解決換帳號；同帳號內的重讀失敗、切店、預覽卡、多份副本不同步都還在。可與 B 並用當登出保險，不能取代 B。

## 約束清單
| 約束 | 推離基準？ | 分類 |
|---|---|---|
| 4c-2 是一個 session | 是 | SELF-IMPOSED（計畫分段）→ 改拆子步驟 |
| 與現有 widget 寫法一致、改動大小 | 是 | SELF-IMPOSED |
| 全域狀態、共用元件要先取得同意 | 否 | REAL（buyer-app/AGENTS.md）→ 本簡報就是請求 |
| 本機不跑完整測試、模擬器 | 否（影響驗收：全套與實機待 CI／手動） | REAL |
| seller-app 4d-3 會做同類畫面 | 否 | SELF-IMPOSED；B 的 base 可當 4d 範本 |

沒有 REAL 約束把方案推離 B。

## 建議
B。計畫把 4c-2 改成：4c-2a（已完成的 Connect 換線，保留為紀錄）、4c-2b-1、4c-2b-2；目前的 `ViewerScopedLoads` 由 4c-2b 刪除。驗收：R1–R3 29 項各在新結構下有測試＋mutation，另加「mutation 期間換觀看者，結果被丟棄」的 base 測試。終態：社群畫面不持有伺服器狀態，同一則貼文只有一份。觸發：現在，4c-3 之前。
