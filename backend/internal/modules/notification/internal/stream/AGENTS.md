# notification/internal/stream

WatchNotifications 的訊號 hub：依 `(帳號, 受眾)` 訂閱（STORE 另帶店，賣家工作區事件以店為單位），每帳號同時開著的串流有上限。

## 不變量

- serve 與 worker 是不同行程，訊號走 LISTEN/NOTIFY（頻道 `notification_signals`）：寫入端（意圖 worker、公告派送的收件匣寫入、`PublishSellerWorkspace`）在寫入的同一個 transaction 內 `pg_notify`，commit 後才送達，rollback 不送。payload 只有業務鍵與提示，沒有內容。守門：`TestInboxSignalCrossesTheProcessBoundaryThroughTheDatabase`、`TestAnnouncementFanOutSignalsTheRecipientsStream`、`TestSellerWorkspaceNotifyFollowsTheCallersTransaction`。
- 每個 serve 行程一條專用 LISTEN 連線（`Deps.ListenConnConfig`：直連或 session 模式，不經 transaction pooling；nil 以主池設定另開），扇出給本行程的訂閱者；worker 不跑 `Run`。每次連線成功發完整重讀訊號，`Run` 結束關閉全部訂閱。守門：`TestListenReconnectEmitsAFullRereadSignal`。
- 訊號的內容（未讀數、最新一則）由 serve 端 `Next` 讀，讀取函式由模組根以 `app` 用例提供：與 `GetUnreadCount` 同一條規則，只算註冊的型別（含公告）。守門：`TestStreamSignalCountsOnlyRegisteredTypesLikeUnreadCount`。
- 上限（resource_exhausted）是每個 serve 行程各自計算；訊號可合併、可遺漏，client 以列表為準，沒有續傳與心跳。
- LISTEN 連線斷線以指數退避重連，連線維持 `stableAfter` 以上才重設退避（連上就斷的不重設，否則每次重連的完整重讀訊號約每秒數次）；讀取內容失敗時已取走的重讀標記與提示放回去。守門：`TestRunBackoffKeepsGrowingWhileTheConnectionFlaps`、`TestRunBackoffResetsAfterAStableConnection`、`TestFailedReadKeepsTheDirtyMarkAndHints`。
