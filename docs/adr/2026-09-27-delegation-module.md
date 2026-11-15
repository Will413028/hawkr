# 代行整個成為 delegation 模組：授權、session、稽核與編排不再拆在 identity

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-27 |
| 影響範圍 | 單體的 identity 與 delegation 模組、`internal/arch` 的依賴表、`internal/legacy/delegationservice`；設計書 56 |
| 取代 | `2026-09-24-go-modular-monolith.md` 的「delegation 整包留在 identity」與其「受影響的既有 ADR」中對 `2026-09-18-adm-delegation-service-ownership.md` §1 的沿用 |
| 路徑基準 | `backend/` |

## 脈絡

遷移計畫把代行拆成兩半：資料（授權、Operation Session、稽核三張表）在 A2 搬進 identity，編排（店家狀態檢查、對被代行模組的寫入）留給 A10 的 `delegation` 模組。identity 搬完之後，6,285 行裡約 2,400 行是代行；編排之後要讀寫這些資料，都得經過 identity 的公開 API（約 19 個代行專用的方法）。

把資料留在 identity 的理由（ownership ADR §1、主計畫 ADR）逐項對照現在的約束：

- (a) 要對管理員表下 FK：單體不建跨 schema FK（主計畫 ADR D3），所有模組引用帳號都只存 ID；代行沒有特別之處。不成立。
- (b) 管理員登出與代行撤銷要原子：`events.Publish` 在 transaction 內同步執行訂閱者，identity 發事件、代行在同一個 transaction 撤銷即可。以事件達成，不需要同一個模組。
- (c) 新服務的固定成本：模組不是服務，主計畫 ADR 已註明不再存在。

約束：

- 繼承：模組之間不得形成依賴環，只能 import 對方的根套件（主計畫 ADR D2）；仍成立。
- 繼承：設計書 56 的不變量（授權、session 的狀態機、稽核的冪等鍵）；仍成立，程式搬移不改語意。
- 外部：系統還沒上線，三張表沒有正式資料，可以直接在新 schema 重建。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：一個 bounded context 一個模組；身分（你是誰）與代表他人操作的授權（impersonation、support access）分成不同的 context，後者依賴前者 | 新 schema 與搬表；identity 改發「管理員登出、移除」事件；legacy 的 DelegationService 轉接改指向新模組 | 採用 |
| 維持：資料在 identity，編排在 delegation | 同一個概念拆在兩個模組；identity 為代行多開約 19 個方法，職責變成認證加授權 | 否決 |
| 資料與編排都放 identity | 編排要依賴 seller、ordering 等模組，identity 是最底層，依賴圖出現環（主計畫 ADR 已否決） | 否決 |

## 決策

代行的授權、Operation Session、稽核與編排整個是 `delegation` 模組（schema `delegation`），依賴 identity、seller 與被代行的模組；identity 不依賴它。identity 在管理員登出、被移除時於 transaction 內發事件，delegation 訂閱並在同一個 transaction 撤銷該管理員進行中的代行。對管理員與店家只存 ID。

## 後果

- 搬移與 0.12（ADR `2026-09-27-module-internal-structure.md`）一起做：代行的程式本來要搬進 identity 的 `internal/delegation`，改成搬進新模組的 `internal/`；排在 A2 的 C10 之後。
- identity 的 `IdentityAdminService.SignOut` 改成「發事件」，撤銷由 delegation 做；`internal/legacy/delegationservice` 改接 delegation 的公開 API；依賴表 `delegation` 的說明改為整個代行。
- A10 的編排直接加進這個模組。
- 重新評估的條件：代行的資料與身分資料出現必須同一個 transaction、而事件無法表達的寫入。

## 修訂：資格歸屬、登出與憑證核發（2026-09-28）

**脈絡**：原決策 (b) 假設 identity 的管理員登出有狀態寫入，須與撤銷代行保持原子。C10 改用 Cloudflare Access 後，登出由 Access 處理，identity 沒有登出寫入路徑；只發 `AdminSignedOut` 的 transaction 實際是在轉送 delegation 命令。identity 目前也沒有移除管理員的路徑。

**決策**：

- 代行資格是 delegation 的授權事實，由 `delegation.admin_eligibilities` 保存管理員 ID，不再是 `identity.admins.delegation_enabled`。撤回資格與撤銷該管理員進行中的代行在同一個 DB transaction；建立 session 時鎖住資格列，避免與撤回交錯後留下 active session。identity 的 `Admin` 不暴露代行資格。
- 後台登出的 HTTP handler 直接呼叫 delegation 撤銷，不再經 identity 的 `SignOutAdmin`／`AdminSignedOut`。`IdentityAdminService.SignOut` 仍留在原 Connect service：對外動作是管理員登出，撤銷代行是它的直接效果，沒有必要把登入與登出拆到不同 service。只有 identity 日後出現管理員移除路徑、且它自己的狀態變更需要通知 delegation 時，才重新引入事件。
- 建立只提供 session 及 handoff code；ADM BFF 以管理員身分按 session ID 讀取完整摘要並對帳，ADM host 不接收 operation credential。兌換時以 `crypto/rand` 產生至少 256 bit 的 operation credential，資料庫只存 SHA-256，不按 session ID 以 HMAC 推導；單句 CAS 清除 code hash 並寫入 credential hash，且只接受尚無 credential 的 session。重放同一 code 依 9282 失效語意處理；同鍵重新建立在兌換前改簽 code，兌換後只回既有 session 摘要，不再提供 code 或改寫 credential hash。

## 修訂：代行的排程發布（2026-11-15）

**脈絡**：A7 計畫的 A7-D6 讓 social 擁有排程、delegation 在撤銷 session 時同 transaction 取消，執行時只擋已撤銷的 session。開工盤點量到兩個事實，前提不成立：代行 session 最長 30 分鐘，排程最長 90 天，執行時 session 幾乎一定已結束；三條批次撤銷（撤回店家授權、撤回資格、角色被撤）只撤進行中的 session。照原設計，撤回授權之後先前排的貼文仍以代行名義發布。約束：social 不得 import delegation（依賴表，inherited，單向依賴仍成立）；跨模組鎖由下往上、social 在 delegation 之前（單體規則 4，inherited，仍成立）。系統未上線，無 external 約束。

**考慮過的方案**（執行時的授權由誰判斷、怎麼接）：

- 甲：delegation 的同 transaction 事件訂閱者兼當閘門，回錯誤讓發布 rollback，job 依錯誤類別轉已取消。缺點：訂閱者漏接時排程直接照發且不報錯；取消要在 rollback 後另開 transaction；單體現有的訂閱者都是事後反應，沒有當前置閘門的。
- 乙（業界基準，依賴反轉：使用端定義介面、組裝點注入實作，Robert C. Martin《Clean Architecture》的 Dependency Inversion Principle）：social 定義排程授權介面，依建立者種類（`DELEGATION`、`OWNER`）註冊；`cmd/hawkr` 注入 delegation 的實作。缺點：單體第一個「上層模組實作下層模組介面」的例子（現有的 port 都由被依賴的一方實作，`internal/modules/agent/ports.go`）。
- 丙：只靠撤銷時取消，不在執行時判斷。批次撤銷蓋不到已結束的 session，撤回授權後照發，不採用。

**決策**：乙（Will 2026-11-15，計畫 A7-D22）。

- 排程貼文屬於店家；排程存在 `social.post_schedules`，一次排程一列，取消原因凍結在列上，River job 只帶排程 ID。
- 到點時 social 在同一個 transaction 先鎖貼文列，再依 `scheduled_by_kind` 呼叫注入的授權實作；拒絕就把排程寫成 `CANCELED`（`AUTHORITY_LOST`）、不重試，通過才發布。缺少某個 kind 的實作時單體啟動失敗。
- delegation 的實作以不鎖的讀取判斷「現在還能開這個代行嗎」：session 沒被治理撤銷（已結束不擋）、店家授權仍含 `posts.publish` 且可代行、管理員存在且資格與角色仍在；判斷與 `StartDelegation` 共用同一個授權述詞，並行上限、TTL、閒置不在述詞內。不鎖是為了不倒退鎖順序；正確性靠取消與發布鎖同一列貼文。
- 撤回店家授權、撤回資格、角色被撤、治理撤銷指定 session 四個觸發點，在同一個 transaction 內先呼叫 social 取消，再鎖或改 delegation 的列。立即取消只讓畫面早點反映，誤發由執行時判斷擋下。
- 稽核仍由 delegation 訂閱 `PostPublished{origin=SCHEDULED}` 寫單段 SUCCEEDED，`idempotency_key = "schedule:<schedule_id>"`。
- 後台登出改為結束管理員進行中的 session（`expires_at` 拉回現在，憑證立刻失效），不再撤銷；撤銷只留給治理動作。上一段修訂寫的「撤銷代行是 `SignOut` 的直接效果」改為「結束」。

**理由與取捨**：乙讓授權判斷成為發布路徑上的必要依賴，漏接在啟動時就失敗，拒絕與取消同一個 transaction；代價是介面的方向和現有 port 相反，需要這份紀錄說明。依賴表與 import 方向不變（social 不 import delegation）。

**撤銷條件**：若之後排程要改成「只依建立當下的授權、之後撤回不影響」，或 social 與 delegation 合併，重新評估。

**相關**：`backend/docs/plans/2026-10-18-a7-social.md`（A7-D6、A7-D22、步驟 A7-1f）、`backend/docs/plans/2026-10-18-a7-social/step-1f-decision-brief.md`。

## 修訂：代行 Connect（2026-11-15，2026-11-16 依 A7-D25 改寫）

**脈絡**：A7-2d 把代行貼文的 6 條 gateway 路由換成單體的 Connect（A7 計畫 S1）；A10 的商品、訂單切片要沿用同一種形狀。gateway 的做法是三段式（稽核 intent → 寫入 → confirm）由 gateway 編排跨服務呼叫；單體裡 delegation 與 social 在同一個行程、同一個資料庫，不再需要跨服務的兩段。約束：依賴方向 delegation → social 合法、social 不得 import delegation（依賴表，inherited，仍成立）；跨模組鎖由下往上、social 在 delegation 之前（單體規則 4，inherited，仍成立）；系統未上線，無相容性約束；gateway 仍在用的 `RecordDelegationAuditIntent`、`ConfirmDelegationAudit` 不動（商品、訂單到 A10 才搬）。

**考慮過的方案**（代行 Connect 由誰擁有）：

- 甲：把代行方法併進 `SocialAppService`。缺點：一條 interceptor 鏈要同時處理顧客 session 與代行 session 兩種憑證，宣告表的「能力」一半是賣家能力、一半是代行能力。
- 乙（2026-11-15 的版本，A7-D24，被取代）：delegation 擁有 `DelegationDelegatedService`（`hawkr.delegation.v1alpha1`），use case 也在 delegation。缺點：契約與資源的寫入分屬兩個模組；A10 的商品、訂單每加一個方法 delegation 就多一份資源專屬的膠水；service 名稱的重複感是前提錯位的症狀。
- 丙（採用，A7-D25）：資源模組擁有 service（Google AIP 依資源切 service，act-as 的差別放在憑證層，不另開資源 API），audience 後綴沿用本專案的 `AppService`／`AdminService`／`InviteeService` 形式：`hawkr.social.v1alpha1.SocialDelegatedService`，A10 的商品、訂單各自有 `<模組>DelegatedService`；delegation 只提供共用的 interceptor 與同 transaction 的稽核＋能力檢查元件。
- 丁：沿用兩段式稽核，由 handler 先 intent 再呼叫 social 再 confirm。缺點：兩個 transaction 之間 social 成功而 confirm 失敗會留下 PENDING 列；單體沒有跨服務呼叫要補償。

**決策**：丙（Will 2026-11-16，A7-D25），以及下面的形狀（A10 沿用或修訂）。

- 服務與類別：服務名 `<模組>DelegatedService`（audience 後綴），package 是擁有資源的模組的 `hawkr.<模組>.v1alpha1`；`rpc.AudienceDelegated`；單體入口不經 adminauth、不讀顧客或管理員身分（停權的帳號不影響它），限流、log、錯誤轉換與其他 service 相同。同一個 package 的 request／response 訊息名不得重複，所以 `SocialDelegatedService` 的方法是 `ListStorePosts`、`GetStorePost`、`CreateStorePost`、`UpdateStorePost`、`PublishStorePost`、`UnpublishStorePost`、`ScheduleStorePost`（`SocialAppService` 已有 `CreatePost` 等）。
- 傳輸（`backend/internal/httpapi/delegation`，共用）：一支泛用的 interceptor，每個 `<模組>DelegatedService` 掛自己的一支、帶自己的宣告表（`Declaration`：service descriptor，與每個方法的代行能力、稽核動作與資源種類；宣告表在擁有 service 的 httpapi 套件，社群是 `httpapi/social`），在 `rpc.NewInterceptor` 之內。流程：讀請求標頭 `X-HK-Operation-Session`，以 `delegation.ResolveDelegation(credential, activity=false)` 唯讀解析；解析不到 session（憑證缺少或無效）回 `UNAUTHENTICATED`、不寫稽核列（沒有 session 可歸屬），只記一筆結構化的安全 log（slog warn：procedure、trace ID、憑證是否存在，不含憑證的值）；解析到之後以 `delegation.AuthorizeDelegated` 授權：session 不是 ACTIVE（到期、已結束、已撤銷，`FAILED_PRECONDITION`）或沒有該方法宣告的能力（`PERMISSION_DENIED`）被拒絕，拒絕另寫一列 FAILED 稽核；通過就把代行（session 與方法宣告）放進 ctx，請求處理成功之後才以 `delegation.TouchDelegation(session_id)`（單一 UPDATE、不回列）順延閒置逾時：在 interceptor、handler 或共用元件任何地方被拒絕或失敗的請求都不算活動。宣告表與 service 方法一一對應，缺漏或多出都不能啟動。目標店家只來自 session，請求不帶 `store_id`。
- 資源模組的 handler（`httpapi/social` 的 `Delegated`）只做傳輸：解析請求、呼叫 delegation 的元件、把結果轉成回應；貼文的寫入仍是 social 的方法，delegation 不 import 它們的細節（delegation 的元件是泛用的，不含貼文專屬的方法）。
- 共用元件（delegation 模組的公開 API，不含任何資源專屬的程式；A7-D26 修訂，取代「在同一個 `db.InTx` 內」的寫法：CI 量到 social 在自己的 transaction 前轉正圖片、確認商品，被外層 transaction 包住後違反單體規則 5「外部呼叫不在 transaction 內」與規則 3「transaction 由最上層 use case 開」）：`DelegatedWrite(ctx, caller, write, do)` 不開 transaction。流程：(1) 重送偵測：查稽核表 `(session_id, request_id)`，內容相同回第一次的結果、不呼叫 `do`，內容不同回 `DenialIdempotencyKeyReused`（見「冪等」）；(2) 檢查 session 此刻還能用並帶有能力（公開 API 自己檢查，不依賴 interceptor，也不能等到訂閱者：資源模組的業務驗證不得先於授權；重送已在 (1) 回第一次的結果），被拒絕回拒絕並寫 FAILED、不呼叫 `do`；(3) 讀店家此刻可不可代行（只有第一次寫入），不可代行回拒絕；(4) 把代行者（`platform/delegatedactor.Actor`：session、管理員、店家、方法宣告、`request_id`、操作原因、`notify_seller`、內容雜湊、寫入前版本、trace ID）放進 ctx，呼叫 `do`（資源的寫入）。資源模組（social）在自己的 transaction 內為每個寫入發帶代行者的領域事件（`PostWrittenByActor`：建立、修改、發布、下架、排程各一次，含冪等的 no-op；與排程發布的 `PostPublished{origin=SCHEDULED}` 同形），沒有代行者（店主）不發，有代行者卻沒有訂閱者時失敗關閉；delegation 的訂閱者 `OnPostWrittenByActor` 在同一個 transaction 內核對事件與代行者一致、以 transaction 內讀到的 session 檢查此刻還能用並帶有能力（interceptor 在請求進來時、`DelegatedWrite` 在呼叫 `do` 之前各檢查過一次，這是提交時權威的一次，共用 delegation 內部同一個檢查函式，對的是不同時間點的 session）、再次確認店家此刻可代行，然後寫一列單段 `SUCCEEDED` 的稽核（`origin = INTERACTIVE`，`request_id` 對到 `(session_id, idempotency_key)`，資源 ID 與寫入後版本取自事件），要通知賣家時同一個 transaction 排入完成事件。任何一步回錯誤，social 的 transaction 整個 rollback，稽核列跟著消失，所以「有 SUCCEEDED 稽核」等於「寫入成功」。啟動時 `cmd/hawkr` 檢查 `PostWrittenByActor` 有訂閱者，沒有就不能啟動。代行者的型別放 `platform`（資源模組不得 import delegation）。`DelegatedRead` 是公開 API，自己做一次 session 與能力檢查（縱深防禦，interceptor 的授權是第一道；讀取沒有 transaction，這是模組內唯一的一次），再讀店家狀態後執行讀取，任何拒絕寫 FAILED，成功的讀取不稽核。稽核列的內容雜湊由元件在資源模組算的請求內容之外，再綁上操作原因與 `notify_seller`（同一把 `request_id` 帶不同的原因或通知設定回 `DenialIdempotencyKeyReused`）；FAILED 列寫入用不帶取消的 ctx（有期限），請求被取消仍留下一列，並帶請求已知的資源名稱與 ID。
- 冪等：每個代行寫入的請求帶 `request_id`（UUIDv4，AIP-155；代行寫入一律必填，與 gateway 時期每個 mutation 都要 `Idempotency-Key` 一致），對到稽核表既有的 `(session_id, idempotency_key)` 唯一鍵；同一個 session 以同一個 `request_id` 重送，內容相同（動作、資源種類、資源 ID 與請求內容雜湊）不呼叫 `do`、回第一次的結果（`Replayed`，呼叫方據資源 ID 讀回資源目前的樣子），與店家此刻的狀態無關（AIP-155：重送回原結果；店家可不可代行只在第一次寫入檢查），也不為已有 SUCCEEDED 的 `request_id` 寫 FAILED；內容不同回 `DenialIdempotencyKeyReused`（failed_precondition）。`request_id` 同時寫進稽核列的 `request_id`；`correlation_id` 是請求的 trace ID。並行的重送（兩個請求同時通過查詢）：後到的訂閱者撞稽核表唯一鍵、回 `ErrOperationKeyTaken` 讓它那一側 social 的 transaction rollback，元件再讀稽核列收斂成重送（`request_id` 以 session 為範圍，偏離 AIP-155「由資源 service 去重」的字面：稽核表已有唯一鍵，social 的 request_id 只在店主路徑）。失敗的嘗試不占用 `request_id`（FAILED 列沒有冪等鍵），同一個 `request_id` 之後仍可成功。
- 稽核模型（成功與失敗都留紀錄，OWASP Logging Cheat Sheet、NIST SP 800-53 AU-2）：成功的寫入是上面同 transaction 的 `SUCCEEDED`；被拒絕的請求（能力不足、session 已失效、店家此刻不可代行，**讀取也寫**）與寫入的業務失敗，在另一個 transaction 寫一列 `FAILED`（被稽核的寫入 rollback 後這一列仍要留下）：新欄位 `failure_reason`（A7-D25 第 4 點）記失敗原因的字串：代行拒絕是拒絕的代碼（`CAPABILITY_DENIED`、`SESSION_REVOKED` 等，表在模組 model 的 `DenialCodes`），業務失敗是回傳錯誤的 `ErrorReason` 值名（`VERSION_CONFLICT` 等），沒有原因的失敗是明確的 `INTERNAL`；CHECK 要求 FAILED 一定有值、值是 UPPER_SNAKE 且不是 `UNSPECIFIED`、非 FAILED 為 NULL（ErrorReason 由 apperr 與 proto 擁有、會隨資源模組增加，所以 CHECK 守格式而不列舉）。數字 `biz_code` 只留給舊 gateway 的兩段式 confirm（過渡邊界）：legacy 轉接層把舊碼對成 `failure_reason`（對不到的 gateway 自有碼寫 `INTERNAL`，原始數字仍存 `biz_code`），讀取時 `biz_code` 優先、沒有則由拒絕代碼對回 9260–9299（`DenialBizCodes`）；`biz_code` 隨 gateway 兩段式在 A10 刪除。被拒絕的讀取用 `*_READ` 動作（`POST_READ`），不通知賣家；`operation_reason` 只在值域合法（1–100 字）時寫入。FAILED 列寫不進去只記 log，不改變回給呼叫方的錯誤。憑證解析不到 session 不寫列，只記安全 log（見傳輸）。成功的讀取不稽核。
- 請求時發現店家不可代行（不存在、已關閉、等待刪除、停權）：只拒絕（`DenialStoreNotDelegable`／`DenialStoreClosed`／`DenialStoreNotOperable`），寫 FAILED，不撤銷、不結束 session；狀態判斷用 seller 的 `StoreActionDelegate`，句子與 `StartDelegation` 被拒時同一句。
- 第五個授權失效的觸發點：delegation 訂閱 seller 的 `StoreSuspended`、`StoreClosed`、`StorePendingDeletion`（等待刪除也是不可代行），在同一個 transaction 內先取消該店待發布的代行排程（原因 `AUTHORITY_LOST`，與其他四個觸發點相同：先取消、再改 delegation 的列），再結束進行中的 session（`EndActiveDelegationsOfStore`；結束同後台登出：`expires_at` 拉回現在、憑證立刻失效，不寫 `REVOKED`，Microsoft CAE、RFC 7009）。只有對 delegation 自己列的 UPDATE，鎖順序是 seller 之後 delegation（由下往上）。別家店的 session 不受影響。
- 鎖順序：稽核寫入只有 INSERT（外鍵對 session 列的 `FOR KEY SHARE` 與所有 session 寫入的 `NO KEY UPDATE` 相容）與對自己剛寫的列的 UPDATE，沒有先鎖 delegation 既有的列；`do` 在其後呼叫資源模組，social 的貼文列鎖仍在任何與之衝突的 delegation 列鎖之前。`platform/lockorder` 不分類單純 INSERT。
- 驗證：操作原因（去除首尾空白後 1–100 字、不得含控制字元）與 `request_id` 格式在稽核寫入之前檢查（模組內 `validateOperation`，傳輸層先擋一次並指向欄位；傳輸層擋下的格式錯誤沒有稽核列：請求還沒有可歸屬的動作）；內容的值域（長度、標籤數、圖片）由 social 判斷、單一來源。
- 店主側：`SocialAppService` 的 `ListPendingSchedules`、`CancelPostSchedule`，讓店主看見並取消代行排的貼文（取消原因 `OWNER_CANCELED`）；能力宣告依 social 模組實際的檢查（取消是 `CAP_MARKETING`，清單是模組沒有能力檢查的讀取，明列豁免與理由）。
- gateway 的兩段式 `RecordDelegationAuditIntent`、`ConfirmDelegationAudit` 與它的 6 條貼文路由留到 A10（商品、訂單搬進單體時統一到這個模型），之後刪除。

**理由與取捨**：單段稽核省掉 PENDING 的中間狀態與 confirm 的補償；失敗另開 transaction 讓被拒絕與失敗的嘗試有紀錄，代價是 FAILED 列不與業務寫入同 transaction（失敗列寫不進去時只記 log）。資源模組擁有 service 讓 A10 的每個資源各自演進，代價是每個 `<模組>DelegatedService` 要自己的宣告表與 handler；共用的部分（interceptor、稽核元件）只有一份。（A7-D26）稽核由事件在資源模組的 transaction 內寫、不由 delegation 開外層 transaction：代價是 delegation 要訂閱資源模組的每種寫入事件（A10 的商品、訂單沿用同形），好處是資源模組的外部呼叫留在 transaction 之外、稽核與寫入仍同 transaction。

**撤銷條件**：若稽核需要與業務寫入同 transaction 的失敗紀錄（合規要求失敗列也不可遺失），FAILED 改為與拒絕同步寫入並失敗時拒絕請求；若 social 與 delegation 合併，重新評估鎖順序的敘述。

**相關**：`backend/docs/plans/2026-10-18-a7-social.md`（S1、A7-D22、A7-D24（已被取代）、A7-D25、步驟 A7-2d）、`api/hawkr/social/v1alpha1/social.proto`（`SocialDelegatedService`）。
