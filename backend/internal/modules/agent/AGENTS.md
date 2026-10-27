# agent 模組

agent 模組放「代理人」類的功能；現在只有 vendor portal：沒有帳號的供應商從採購詢價（RFQ）的邀請信進來看詢價、寫草稿、正式送出時的存取模型，給對外入口（`internal/httpapi/agent` 的 `VendorPortalPublicService`（以邀請 secret 兌換）與 `VendorPortalInviteeService`（以每張邀請各一個的 cookie 認證），以及 `internal/legacy/gatewayproxy` 轉給 gateway 的 RFQ 路由）、notification 的邀請信（步驟 9 的 legacy gRPC adapter，A8 之後直接呼叫）與採購（A9）使用。來源是 seller-service 的 `vendor_portal_*`（A3b 步驟 7d）；決策在 ADR `docs/adr/2026-10-03-vendor-portal-invite-scoped-access.md`。

## 職責

- 邀請（`EnsureInvite`、`MarkInviteDelivered`）：每個 RFQ 收件人一張，存 prospect、寄出時的 email 快照與 secret 的 SHA-256；Ensure 在一個 transaction 內以共享鎖讀 prospect、再鎖住那張邀請：只有進行中的 prospect 能建立或換 secret；沒有就建立、還沒寄出就換 secret 並重讀 email；已撤銷、prospect 不是進行中、已寄出、沒有可寄的 email 各回不同的結果（`ErrInviteRevoked`、`ErrProspectNotActive`、`ErrInviteAlreadyDelivered`、`ErrNoDeliverableEmail`，用 `errors.Is`）；既有邀請掛的 prospect 與輸入不同回 InvalidInput。Mark 帶寄出的那一把 secret，只有它仍是目前的 secret 才標記，過時回 `ErrInviteSecretSuperseded`，同一把重送冪等。
- 兌換與預覽（`ExchangeInvite`、`PreviewInvite`）：邀請到期或撤銷之前可重複兌換，每次發新的 session；預覽回遮罩的 email、到期時間，以及邀請與收件人的 ID（只給入口判斷請求帶的 session 是不是這張邀請的，不進對外 API），沒有副作用。
- session（`ValidateSession`、`VerifySession`）：idle 24 小時滑動（寫入節流 5 分鐘）、絕對 7 天、不超過邀請的到期時間；一張邀請同時最多 5 個有效 session。驗證紀錄（方式、驗證的 email、時間）綁單一 session。
- 撤銷與合併（`OnProspectArchived`、`OnProspectMerged`）：seller 在封存的 transaction 內發事件，handler 在同一個 transaction 撤銷該 prospect 的全部邀請與底下的 session；合併時 handler 把輸家的邀請改掛贏家，不撤銷。
- 清理（River 每天、啟動即執行）：絕對期限過了超過 7 天的 session；到期或撤銷超過 90 天的邀請連同 email。

## 擁有的資料

schema `agent`（`db/schema.sql`）：

- `vendor_portal_invites`：`invite_id`（UUIDv7）、`rfq_recipient_id`（UNIQUE，ai-service 的 ID）、`prospect_id`（seller 的 ID）、`invited_email`（寄出時的快照，這個模組唯一的個資）、`secret_hash`、`secret_delivered_at`、`expires_at`、`revoked_at` 與 `revoke_reason`（`PROSPECT_ARCHIVED`）。
- `vendor_portal_sessions`：`session_id`（UUIDv7）、`token_hash`、`invite_id`（FK，`ON DELETE CASCADE`）、`verification_method`（`GOOGLE`）、`verified_email`、`verified_at`、`idle_expires_at`、`absolute_expires_at`、`revoked_at`、`last_seen_at`。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與錯誤，加上 `ports.go`（`Prospects`，由 `*seller.Module` 實作；`ProspectContact` 在呼叫方的 transaction 內對 prospect 列取共享鎖，Ensure 靠它與封存序列化）。
- `internal/model` 放對外型別、代碼值、期限常數、錯誤與 email 的純函式；`internal/vendorportal` 是第一個子領域（邀請、兌換、session、撤銷、清理與排程）；`internal/store` 是 sqlc 的查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。
- 事件的接線在 `cmd/hawkr/agent_events.go`；模組本身不發事件。

## 不變量

- **存取範圍是一張邀請**：session 只屬於一張邀請，驗證紀錄只屬於一個 session；兌換不撤銷別張邀請的 session。由 `vendor_portal_sessions.invite_id` 與所有查詢以 session 為鍵保證，`TestExchangeCapsSessionsPerInviteAndLeavesOtherInvitesAlone` 綁住。
- **秘密不落地**：邀請 secret 與 session token 是 32 bytes 亂數（base64url），資料庫只存原始 bytes 的 SHA-256（CHECK 長度 32、UNIQUE）；明文只在 `IssuedInvite.Secret`、`IssuedSession.Token` 出現一次。不透明字串，呼叫端不得解析。
- **不分原因的錯誤**：兌換、預覽的不可用（未知、格式錯、過期、撤銷）一律同一個 NotFound；`ValidateSession` 一律同一個 Unauthenticated；`VerifySession` 的不符或不可用一律同一個 PermissionDenied。猜 secret 的人分不出成因。
- **有效的條件只有一處**：session 沒撤銷、idle 與絕對期限都沒到，且邀請沒撤銷、沒到期；寫在 `ValidateSession`、`VerifySession` 的 SQL 述詞裡（兩者一致），session 的絕對期限在建立時被邀請的到期截斷。
- **驗證是 CAS**：`VerifySession` 一個條件 UPDATE：有效、還沒驗證、驗證的 email 與邀請快照去前後空白且不分大小寫相等；DB 的 CHECK 要求方式、email、時間同有同無。模組只收呼叫端已驗證的 email，不驗 Google token。
- **邀請的狀態**：撤銷時間與原因同有同無（CHECK）；已撤銷的邀請不得再 rotate 或兌換；撤銷不可逆（沒有還原的路徑，prospect 還原不復活邀請）；重送撤銷事件不改原本的時間與原因。
- **封存之後沒有有效的邀請**：Ensure 只替進行中的 prospect 建立或換 secret，並與封存序列化（讀 prospect 帶 `FOR SHARE`，鎖的順序固定是 prospect 再邀請；封存的撤銷與 Ensure 不會讓封存的 prospect 留下未撤銷的邀請）；非進行中（封存、被合併、已轉成店家）回 `ErrProspectNotActive`。`TestEnsureInviteRequiresAnActiveProspect`、`TestProspectArchiveAndMergeRevokeVendorPortalAccess`。
- **一張邀請屬於一個 prospect**：既有邀請的 `prospect_id` 與 Ensure 的輸入不同時 InvalidInput，邀請原樣（`TestEnsureInviteRejectsAnotherProspectsRecipient`）。合併是唯一改變 `prospect_id` 的路徑：輸家的邀請改掛贏家，email 快照、secret、session、驗證不動，不撤銷（`TestProspectMergedRepointsTheSourceInvitesToTheWinner`）。
- **Mark 只認目前的 secret**：條件 UPDATE 比對 SHA-256，Ensure 換過 secret 之後過時的 Mark 不生效（`TestMarkInviteDeliveredIgnoresASupersededSecret`、`TestMarkInviteDeliveredRacesWithRotation`）。
- **驗證紀錄只用於授權，證據由步驟 8 保存**：session 上的紀錄隨 session 清理；正式送出、撤回、婉拒時，入口以 `X-Vendor-Portal-*` header 把 `SessionInfo` 的 `SessionID` 與 `Verification{Method, Email, At}` 交給 gateway（API 設計規範（Connect）§二），gateway 轉給 ai：正式送出存在不可變的報價版本列，撤回與婉拒存在 ai 只追加的指令紀錄（ADR「驗證的證據」與 2026-10-04 修訂）。`ValidateSession`、`VerifySession` 交出這四項（`TestSessionInfoCarriesTheVerificationEvidence`）。
- **session 上限不擠掉較高保證的 session**：超過 5 個有效 session 時，撤銷候選排除本次新發的（它一定未驗證），先撤最舊的未驗證，全部都已驗證才撤最舊的（`TestExchangeOverCapRevokesTheOldestUnverifiedFirst`、`TestExchangeOverCapKeepsTheNewSessionAndRevokesTheOldestVerifiedWhenAllAreVerified`）。
- **併發**：同一個 RFQ 收件人的 Ensure、Mark、兌換與撤銷以邀請列鎖序列化；Ensure 以唯一鍵去重，不會讓呼叫端看到 unique violation；有效 session 的上限在同一個 transaction 內執行。
- **撤銷在發布方的 transaction 內**：handler 在 transaction 外被呼叫時拒絕；發布方 rollback 時撤銷一併 rollback。
- **到期時間**：`EnsureInvite` 的 `ExpiresAt` 必須晚於現在、不超過現在起 180 天（InvalidInput）；固定上限是沒有 RFQ 截止日可用時的代用品，理由與 A9 的觸發見 `CARRYOVER.md`。
- **email 是個資**：只存在 `invited_email`，隨邀請在到期或撤銷滿 90 天後刪除。
- 代碼值是 UPPER_SNAKE，Go 的值集合與 CHECK 由 `TestChecksMatchGo` 綁在一起；資料庫另以 `TestDatabaseRejectsInconsistentRows` 驗證寫入端出錯時的第二道防線。

## 已知限制

- `rfq_recipient_id` 屬於 ai-service，A9 之前無法驗證它存在，只檢查是正整數。A9 採購搬進單體時，邀請改在 RFQ 發布的 transaction 內建立並驗證它；RFQ 發布選出收件人時也要排除封存的業者（Ensure 已拒絕非進行中的 prospect，但發布端要在寄信之前就排除）。
- Ensure 與 Mark 的兩段式（`secret_delivered_at`、換 secret）在 A8 notification 搬進單體前成立；Send 成功但 Mark 失敗時的重送會換 secret，先寄出的信失效（同一把 secret 重寄的取捨見 `CARRYOVER.md`）。A8 時改成同一個 transaction 加 outbox，拿掉兩者，並加入 email OTP 作為第二種驗證方式。
- 兩個併發的 Ensure 對同一張還沒寄出的邀請都會成功，後到的換掉先到的 secret（先到的呼叫方拿到的連結失效，它的 Mark 回 `ErrInviteSecretSuperseded`）；同一個 RFQ 收件人一次只該有一個寄信的呼叫方，A8 的 outbox 之後不再有這個問題。

## 讀取模型

無。

## 不負責

- 對外的 Connect API、cookie、`Idempotency-Key`、兌換與驗證的限流、Google token 的驗證（經 identity）、正式送出與撤回的「要求已驗證 session」檢查：步驟 8 的入口。
- notification-service 呼叫的 `EnsureVendorPortalInvite`、`MarkVendorPortalInviteSecretDelivered` 的 gRPC adapter 在 `internal/legacy/sellerserver/invites.go`（對應模組搬進單體後刪除）。
- email OTP 與寄信：A8（notification）。
- RFQ 的內容、草稿、送出、`rfq_recipient_id` 的存在與歸屬、買家取消發布時撤銷邀請：ai-service，A9 搬進單體後由採購負責。
- prospect 的資料與 email：seller（`ProspectContact`）。

## 決策紀錄

從 seller-service 沿用的每個機制、當時的決定與理由，以及與 seller-service 的差異在同目錄的 `CARRYOVER.md`。改動或移除沿用的機制前，先讀那一列。
