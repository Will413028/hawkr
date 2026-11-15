# A7-1f 開工決定簡報（排程發布）

- 對象：`backend/docs/plans/2026-10-18-a7-social.md` 步驟 A7-1f；修訂 A7-D6（Will 2026-10-18，`decision-brief.md` §3）。
- 快照：dev `cb7f4f14`（2026-11-15）。【量】＝讀過程式碼或文件；【推】＝推論；【未查證】＝一般知識。
- 狀態：方向已決（Will 2026-11-15，A7-D22）。

## 0. 結論（A7-D22）

排程貼文的擁有者是店家，執行時判斷「授權現在還在不在」，不判斷「當初的 session 還活著嗎」。

1. 排程唯一的真相是 `social.post_schedules`（一次排程一列，取消原因凍結在列上）；River job 只是到點的觸發器（§2）。
2. 執行時重驗：等同當下重跑一次開代行的授權判斷，只略過時間與並行上限（§1）。
3. 授權失效的四個觸發點，在同一個 transaction 內先取消排程、再改 delegation 的列（§1 表）。
4. 後台登出改為「結束」管理員進行中的 session，不再「撤銷」（§3）。
5. 店主看得到並能取消代行排的貼文：入口不在 A7-1f，排進 A7-2d（§4）。

## 前提題：這一步改的程式碼之後會被重寫、搬走或刪掉嗎？

不會。delegation 的撤銷、結束、稽核與 social 模組都是單體的最終位置（`backend/AGENTS.md`「單體的結構」）。gateway 還在用的兩段式 `RecordDelegationAuditIntent`／`ConfirmDelegationAudit` 不動（計畫 A7-1f「不能動」），A10 才處理。重工量：0。

## 業界基準【未查證】

| 系統 | 延後的動作綁在誰身上 | 授權被撤時 |
|---|---|---|
| Buffer、Hootsuite 團隊排程 | 頻道或組織，記下建立者 | 成員離開照發；頻道授權被撤就失敗 |
| Meta Business Suite | 粉專 | 代理商失去存取就不能管理；粉專停權不發 |
| Salesforce 排程工作 | 建立者本人 | 建立者停用就失敗（公認缺陷，官方建議改用整合帳號） |
| OAuth 延後動作 | 授權（grant），不是 access token | 授權撤銷後執行時失敗 |

共通原則：資源主人擁有動作、操作者只是建立者；執行時以當下授權判斷，撤銷時取消只是讓畫面早點反映；資源主人看得到並能取消待執行的動作。

## 1. 執行時重驗與立即取消

**事實【量】：**
- 代行 session 最長 30 分鐘（`model.DelegationTTL`，`delegation_rules.go:22`），排程最長 90 天後（舊規格 `SchedulePublishMaxLead`，`services/social-service/internal/domain/post_schedule.go:28-38`）。執行時 session 幾乎一定已結束。
- 批次撤銷只撤進行中的 session（`delegation/db/queries/delegation_sessions.sql:157-186` 的 `status = 'ACTIVE' AND ended_at IS NULL …`）；`SessionForAudit` 對 scheduled origin 只擋 `REVOKED`（`internal/session/query.go:45-60`）。照 A7-D6 原文，撤回店家授權或拔掉管理員權限後，先前排的貼文照發、稽核寫 SUCCEEDED。
- 設計書 56 第 4821 行把「目標店家轉為不可代行」（§8.1.3 第 8 項 `store_not_operable`）列為取消來源之一；重驗等同重跑開代行的判斷，自然涵蓋。

**執行時重驗**（接法：social 定義排程授權介面、`cmd/hawkr` 注入 delegation 實作，job 在同一個 transaction 先鎖貼文列再問授權，拒絕就把排程轉 `CANCELED`、原因 `AUTHORITY_LOST`、發 `PostScheduleCanceled`、不重試；開工設計審查後 Will 2026-11-15 選定，取代「訂閱者回錯誤讓發布 rollback」，見 ADR `2026-09-27-delegation-module` 2026-11-15 修訂）：
- 該 session 沒有被治理撤銷（`REVOKED`）；已結束或到期不擋。
- 店家授權仍有效且含 `posts.publish`，店家仍可代行；管理員仍存在、仍開通資格、角色仍有 `delegation.sessions.create`。
- 這組條件從 `preflight`（`session.go:57-140`）抽成一個授權述詞，`StartDelegation` 與重驗共用；並行上限（`GetActiveDelegationSessionOfActor/Store`）、TTL、閒置留在 `StartDelegation`，不進述詞。管理員不存在是拒絕（轉 `AUTHORITY_LOST`），不是錯誤（現在 `preflight` 回錯誤，`session.go:61-64`，會讓 job 無限重試）。
- 重驗一律用**不鎖的讀取**：job 已經鎖了 social 的貼文列，再以 `FOR SHARE` 讀 identity 角色（`AdminAllows` → `ListAdminRolesForShare`，`identity/db/queries/admin_roles.sql:23`）會倒退鎖順序，和「撤角色 → 取消（鎖貼文列）」形成死結。正確性靠貼文列的鎖：取消路徑也要鎖同一列，所以要嘛 job 先拿到鎖（發布發生在撤銷 commit 之前，取消時排程已是 `PUBLISHED`），要嘛取消先 commit（job 拿到鎖後讀到 `CANCELED`，或讀到已撤的授權而拒絕）。identity 為此提供不鎖的權限讀取（例如 `AdminAllowsNow`），`StartDelegation` 繼續用鎖的版本。
- 成本【推】：每次執行多讀授權、資格、角色三份資料（角色經 identity `AdminAllows`），延長貼文列鎖住的時間，量級是幾次索引查詢。

**立即取消**（單體規則 4：social 在 delegation 之前，取消一律在鎖或改 delegation 任何列之前）：

| 觸發點 | 範圍 | transaction 內的順序 |
|---|---|---|
| 撤回店家授權（`authorization.go:390`，現在第一句是 `lockAuthorization` FOR UPDATE） | 該店所有 `PENDING` 且建立者是代行的排程（social 以 `store_id` 取消，不列舉 session） | `social.CancelDelegatedSchedulesOfStore` → 鎖授權列 → 撤銷進行中 session |
| 撤回管理員代行資格（`session.go:486-505`，現在先改資格列） | 該管理員所有未撤銷 session 的排程（不鎖的讀取列出 session ID） | 讀 session → 取消 → 改資格列 → 撤銷進行中 session |
| 管理員角色被撤（`OnAdminRoleRevoked`，在 identity 的 transaction 內） | 同上 | identity 角色列（DELETE，再 `AdminAllows` FOR SHARE）→ 讀 session → 取消 → 撤銷進行中 session（之前沒有鎖 delegation 的列，identity 在 social 之前，合法） |
| 治理撤銷指定 session（`RevokeDelegation`） | 這個 session 的排程 | 取消 → UPDATE session |

不鎖讀取與取消之間新開的 session、以及之後新增卻沒接取消的治理路徑，都由執行時重驗擋下；立即取消只負責讓畫面早點反映。這取代 A7-D6 的「撤銷收斂到一個內部撤銷點、同 transaction 呼叫 `CancelScheduledBy(session)`」與「過期不用處理」；A7-D6 的方向（social 擁有排程、delegation 呼叫 social、單段稽核）不變。

**約束帳：**

| 約束 | 分類 | 來源 |
|---|---|---|
| social 不能 import delegation | SELF-IMPOSED，符合基準，保留；重驗在 delegation 的訂閱者內 | `internal/arch/modules.go` |
| session 歷史不可事後改寫（不把已結束的 session 補成 REVOKED） | SELF-IMPOSED，符合稽核基準，保留 | 結束摘要依賴 `ended_at`（`query.go` `DelegationSessionSummary`） |
| 鎖順序 social 在 delegation 之前 | SELF-IMPOSED（單體規則 4），照它排 | `platform/lockorder.Modules` |
| A7-D6「過期不用處理」 | 已決，事實前提不成立，由 A7-D22 修訂 | `decision-brief.md` §3 A7-D6 |

**推翻條件：** 若要「排程只依當初的授權、之後撤回不影響」，就退回 A7-D6 原文；這和業界基準與 A7-D6 的「撤銷時取消」方向都相反。**可逆性：** 上線前可逆，只動 delegation 內部與 social 的取消介面。

## 2. 資料形狀：`social.post_schedules`

**事實【量】：** 舊系統把排程放在貼文列，取消原因分三種凍結，理由是「hidden_at 可被 RestorePost 清成 NULL，事後反查會得到看起來沒被下架過的錯誤答案」（`post_schedule.go:10-16`）。現在的 `social.posts` 沒有排程欄（`social/db/schema.sql:23-43`）；`CARRYOVER.md:27,46` 寫明 A7-1f 再加，輪詢、租約、六態欄位不搬。River 會定期清掉完成與取消的 job【未查證保留時長】，不能當長期紀錄。

| 選項 | 失效條件 |
|---|---|
| a 貼文列加 `scheduled_at`、`scheduled_by`，取消就清空 | 事後問「這篇為什麼沒發」沒有地方查 |
| b 貼文列再加最後一次結果與原因 | 重排後前一次原因被覆蓋；貼文列多 4 個只給排程用的欄 |
| **c（採用）** 新表，一次排程一列 | 列出某店排程要多一個 join；表隨排程次數成長（遠小於貼文量【推】） |

欄位：`id`、`post_id`、`store_id`、`publish_at`、`scheduled_by_kind`（`DELEGATION`、`OWNER`）、`scheduled_by_ref`（不透明字串，由發起者解析；delegation 放 session ID）、`state`（`PENDING`、`PUBLISHED`、`CANCELED`）、`cancel_reason`（`AUTHORITY_LOST`、`DELETED`、`TAKEN_DOWN`、`REPLACED`、`OWNER_CANCELED`）、`created_at`、`resolved_at`。索引：`(post_id) WHERE state = 'PENDING'` 唯一；`(scheduled_by_kind, scheduled_by_ref) WHERE state = 'PENDING'`；`(store_id) WHERE state = 'PENDING'`。River job 只帶 `schedule_id`，執行時先鎖貼文列再讀排程列，不是 `PENDING` 就不做事（冪等鍵＝`schedule_id`）。

## 3. 後台登出改為「結束」

**事實【量】：** 登出呼叫 `RevokeDelegationsOfAdmin`（`httpapi/identity/admin.go:146`），寫 `REVOKED`、理由 `ADMIN_LOGOUT`（`session.go:450-452`）。設計書 56 第 2007 行要求登出時中止進行中的代行；第 5079 行把它列為「後台換人登入、代行 cookie 還在」的三道補償防線之一；第 4819 行寫登出撤銷讓待執行排程被拒是「同一條機制的自然結果」；ADR `docs/adr/2026-09-27-delegation-module.md:53` 寫撤銷是 `SignOut` 的直接效果。

**決定：** 登出改為結束管理員所有進行中的 session（寫 `ended_at`，不寫 `REVOKED`）。
- 防線要的是憑證立刻失效。推導狀態不看 `ended_at`（`model/rules.go:11-19`），結束能擋住憑證是因為 `EndDelegationSession` 把 `expires_at` 拉回現在（`delegation_sessions.sql:104-112`），之後 `SessionForAudit`（`query.go:51-55`）與 `ResolveDelegation`（`session.go:376-399`）都判為到期。新的「結束某管理員所有進行中 session」查詢必須用同一個 `expires_at` 夾值；只寫 `ended_at` 會讓第 5079 行的防線失效。邊界：夾值是 `GREATEST(issued_at + 1s, now)`，核發後 1 秒內登出時憑證最多再有效 1 秒，接受（比 REVOKED 弱的唯一差別）。
- 第 4819 行的「排程被拒」是舊機制的連帶結果，不是設計目標；撤銷只保留給治理動作（不再信任這個人或授權）。
- 成本：新增查詢「結束某管理員所有進行中 session」（現有 `EndDelegationSession` 只收單一 ID，`store/delegation_sessions.sql.go:95-105`）；改 `internal/modules/delegation/AGENTS.md` 的不變量與守門測試 `TestAdminSignOutRevokesItsDelegations`（改名並改斷言為結束）、`docs/modules/delegation.md:46,49`、設計書 56 第 2007、4819、5079 行與 §7.2.7、delegation ADR 修訂段；`RevokeReasonAdminLogout` 若再無使用處就刪除。

## 4. 店主看得到並能取消

業界基準的第三條。A7-1f 提供 `CancelSchedule(post, by OWNER)`（原因 `OWNER_CANCELED`）與讀取排程的根套件 API；店主的 Connect 入口與畫面排進 A7-2d（Will 2026-11-15）。

## 5. 實作預設（可推翻）

- 可排程的狀態：`DRAFT`、`UNPUBLISHED`（舊規格 `SchedulePostPublish` 的 `publish_status <> PUBLISHED`）；排程中可編輯，編輯後仍要滿足公開要件（`social/CARRYOVER.md:83`）；手動立即發布或重新排程時舊排程轉 `REPLACED`；刪除與內容下架同 transaction 取消（`DELETED`、`TAKEN_DOWN`）。
- 時點區間 5 分鐘到 90 天（`post_schedule.go:28-38` 的理由仍成立）。
- social 內鎖順序：貼文列 → 排程列；發布 job 與所有取消都先鎖貼文列（同店多列依 ID 升序）。
- 單段 SUCCEEDED 稽核的 `idempotency_key = "schedule:<schedule_id>"`；job 重試撞唯一鍵回原結果。
- 取消函式回傳被取消的排程 ID；空的 ref 清單是 no-op；每個被取消的排程發一個 `PostScheduleCanceled{schedule_id, post_id, reason}`。
- `PostPublished` 帶 `origin`（`DIRECT`、`SCHEDULED`）與 `scheduled_by`；不另發 `PostScheduled`（沒有消費者）。

## 6. 計畫事實更正

計畫 A7-1f 寫撤銷「也會在店家停權、關店的 transaction（seller）內觸發」，驗收寫「以 `RevokeActiveDelegationsOfStore`（在 seller 的 transaction 內）」。實際上 `RevokeActiveOfStore` 只被撤回店家授權呼叫（`rg -n "RevokeActiveOfStore" backend/internal --type go` → `authorization.go:425` 與介面宣告；設計書 56 第 4810 行一致）。店家轉為不可代行改由執行時重驗涵蓋；驗收改成「撤回店家授權與排程 job 並行不死結」。
