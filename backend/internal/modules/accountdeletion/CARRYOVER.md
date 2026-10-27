# accountdeletion 模組的沿用盤點

## 沿用盤點

從 buyer-service 的 `delete_buyer_account.go`、`complete_pending_deletion.go`、order client，以及 gateway 的 `admin_account.go` 逐項判斷。系統從未上線，沒有正式資料搬移。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| TSID 稽核 ID | 分散式產號 | 不成立；新實體依 row-identity ADR | UUIDv7 | 申請 id 用應用層 UUIDv7，無 DEFAULT |
| buyer 狀態與 pending | 未結訂單阻止刪除 | 成立 | 流程管理者保存申請，參與者保存自身狀態 | 申請 PENDING_ORDERS；buyer PENDING_DELETION；完成時同交易關閉 |
| order 的未結狀態集合 | Java ordering 仍在服務 | 過渡期成立 | 擁有訂單者提供查詢 API | Orders port 由 legacy/orderservice 以 CodeMeta 狀態集合實作；未知狀態失敗 |
| buyer_id 查訂單 | 訂單以 buyer_id 為業務鍵 | 成立 | 以下單時的身分 ID 查 | 由 buyer 公開 API 解析帳號唯一的 current buyer_id，傳給 Orders port |
| pending 重送回 9204 | 舊 REST 相容 | 不成立，未上線且新 Connect 契約回狀態 | 冪等請求回當前狀態 | 重送沿用同一 pending 狀態列，另追加不可變的 request_attempts；管理員後續申請亦留痕 |
| pending 時移除角色及撤銷 token | 舊跨服務角色處理 | 不成立；等待期間仍是買家 | 待辦流程保留參與者直到可關閉 | pending 期間仍是買家；完成時 buyer.CloseForDeletion 關閉買家，identity 再刪帳（買家不是角色，W5，不移除角色） |
| buyer_deletion_audit | 管理員申請須留操作者與原因 | 成立，但稽核屬帳號流程 | 流程管理者的持久狀態紀錄 | accountdeletion.deletion_requests 保存狀態與時間；request_attempts 每次保存本人或管理員、operator_admin_id、reason；buyer 刪舊表 |
| 管理員先 seller 後 buyer | 兩個遠端服務藉順序避免提早 cascade | 不成立：seller 在單體，三者同一個 transaction | 單一 transaction 由流程管理者協調參與者 | 管理員的申請在已鎖帳號列的 transaction 內呼叫 seller 的 `BeginDeletion`／`CloseForDeletion`（與 buyer 同一個 transaction，順序不再有語意）；帳號本人的申請仍要先關店。店家的未結訂單由 Orders port 的店家側簽名查（7c 階段 3，見下一列） |
| 管理員刪帳號略過店家訂單、`OverrideActiveOrders`（7a 暫時行為與 gateway 的強制覆寫） | 店家有未結訂單時不能刪 | 成立；覆寫不成立（強制刪除讓進行中的訂單失去賣家，要立即處理惡意店家走停權） | 流程管理者等待參與者可以關閉；擁有訂單者提供查詢 | **7c 階段 3**：Orders 加 `HasOpenStoreOrders(storeID)`（與 `seller.closing` 的自助關店同一個 legacy 實作，`GetSellerOrders` 含未付款加 `aftersales_count`，fail-closed）；管理員的申請與 `CompletePending` 都查，買家側先查、已有未結訂單就不問店家；任一失敗不寫入；不搬覆寫旗標；交易內確認仍是同一個 stint |
| order.completed 加速只認買家 | 訂單結案時等待中的刪帳要盡快完成 | 店家是商家的訂單也阻擋刪帳，事件有 `store_id` | 事件只加速收斂，對帳是保底 | `buyerconsumer` 的 job 帶 `store_id`，經 `seller.Store` 解析帳號後各自呼叫 `CompletePending`，一側失敗不擋另一側；沒有設定解析器卻有 `store_id` 是錯誤、不默默略過 |
| 無角色 fallback 刪帳 | 帳號可沒有 buyer/seller | 不成立；帳號活著 ⇔ 買家活著（ADR `2026-09-30-account-created-provisions-buyer`） | 不變量被破壞時失敗並告警，不以替代路徑掩蓋 | 不搬；未刪除的帳號沒有現任買家回 internal 並記 error log，不寫入、不刪帳 |
| order 查詢在 buyer transaction 內 | 舊遠端呼叫併發競態 | 不成立；跨服務交易應避免遠端呼叫 | 外部前置查詢先於本地交易，未來同 DB 可納入交易 | Orders port 先查，查詢失敗 unavailable 且無寫入 |
| outbox、RabbitMQ、角色遠端 RPC | 微服務跨庫可靠傳遞 | 不成立 | 單體 transaction 內 events.Publish、River job | 不搬；buyer 與 identity 公開 API 共用 db.InTx |
| PII、地址、收藏清理 | 關閉買家身分需要 | 成立 | 由擁有資料的 participant 做 | buyer.CloseForDeletion 保留既有清理與事件；鎖順序改由本模組保證（交易內先 `identity.LockAccount` 再呼叫它），buyer 不再自己鎖帳號，也不訂閱 `AccountDeleted`：關閉買家只有本模組一條路徑（ADR `2026-09-30-account-deletion-module.md` 修訂） |
| CodeMeta／bizcode | 舊 wire 契約 | 只有過渡訂單狀態成立 | 模組使用具名字串型別與 CHECK，邊界轉 Connect code | Requester／State 用 UPPER_SNAKE＋DB CHECK；錯誤用 apperr |
| 定期收斂 pending | order.completed 事件可能早於申請 commit 或遺失 | 成立 | 定期掃描持久流程狀態並重試 | 每小時 River job 啟動即補跑、有界 keyset 掃描、逐筆錯誤不中斷；步驟 4 event consumer 只加速，先經 buyer.ProfileByBuyerID 由 buyer_id 解析 account_id 再呼叫 CompletePending |
| 同帳號申請與角色寫入的序列化 | 交易外查遠端訂單後，SELLER 可能在交易開始前授予、帳號可能被刪除；買家不會換成另一個（一個帳號一生一個買家） | 成立 | 所有參與者共用同一個帳號列鎖（PostgreSQL row lock，`SELECT … FOR UPDATE`）；不另建只保護單一模組的 advisory lock | 交易內先呼叫 `identity.LockAccount`（與 GrantRole、RemoveRole、DeleteAccount 同一把鎖），再重讀帳號與買家；帳號本人的申請在 SELLER 出現時拒絕、管理員的申請改為連同店家一起處理，已刪除就記完成。buyer_id 比對與重試迴圈已刪除（走不到）；交易內不再重讀「還有其他角色」（SELLER 剛在同一把鎖下檢查過）；partial UNIQUE 保底 |
| schema FK 跨模組 | 跨 schema 資料擁有權 | 不成立 | 模組資料庫邊界以 ID 關聯 | account_id bigint，不設跨 schema FK；partial UNIQUE 保護 pending |

## 傳輸層的沿用盤點

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| gateway 管理員 rich delete | 舊 REST 與雙服務編排 | A3b 之前不成立 | 後台入口呼叫同一流程 API | 後台刪帳於步驟 8 接 Request(ADMIN)，目前不建入口 |
| 自助帳號刪除的認證 | 已驗證 principal | 成立 | handler 只傳 account_id，流程模組重新查角色 | IdentityAppService.DeleteAccount 傳 ACCOUNT_OWNER |
| 舊 gRPC envelope／錯誤碼 | 舊 client 分支 | 不成立 | Connect code 加 zh-TW LocalizedMessage | failed_precondition、unavailable 由 apperr 映射；不帶舊碼 |
| 分頁／快取／請求大小／限流／trace | 外部 API 入口 | 成立但非流程職責 | 共用入口中介層 | 沿用 httpapi/rpc；模組不重做 |
| order-service 五秒 deadline | 過渡期遠端查詢可能卡住 | 成立 | client 設定有界期限 | legacy/orderservice 保留五秒 timeout；錯誤由流程轉 unavailable |
| 重試 | pending 與外部訂單查詢 | 成立 | 冪等操作可重送，每次完成前重查訂單 | Request 重送共用 pending 狀態並追加 attempt；CompletePending 重查且可重送；River 每小時補跑，order.completed consumer 只加速 |

## 與舊服務的差異

- 稽核從 buyer stint 移到帳號申請；本人申請也留一筆，pending 與 completed 有明確欄位及約束；每次申請有獨立 append-only attempt。
- pending 期間仍是買家；完成刪除在單一資料庫 transaction 內進行。
- 帳號本人的申請在有 SELLER 時拒絕；管理員的申請把店家當參與者，在同一個 transaction 關閉。管理員的申請把店家的未結訂單與買家的一樣當成等待條件（7c 階段 3），本人的關店在 seller 的 `CloseCurrent` 檢查同一個 port，不保留 gateway 那種永久的強制覆寫。
- 申請進入等待時帳號變成 PENDING_DELETION（identity 保存的狀態），等待期間不能授予角色。
- 不依賴 Redis、outbox、舊公司服務或既有正式資料 migration。
