# agent 模組行為規格

本文是單體 `agent` 模組行為的唯一來源。模組提供沒有帳號的供應商從採購詢價（RFQ）邀請信進來查看詢價、寫草稿、正式送出時的存取模型：邀請、兌換、session、撤銷與清理。

## 1. 範圍與公開介面

- 使用者：對外入口 `internal/httpapi/agent` 的 `VendorPortalPublicService`（以邀請 secret 兌換）與 `VendorPortalInviteeService`（以每張邀請各一個的 cookie 認證）、`internal/legacy/gatewayproxy` 轉給 gateway 的 RFQ 路由、notification 的邀請信（經 legacy gRPC adapter `internal/legacy/sellerserver/invites.go`，對應模組搬進單體後刪除），以及採購。
- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與錯誤，加上 `ports.go` 的 `Prospects`（由 `*seller.Module` 實作；`ProspectContact` 在呼叫方的 transaction 內對 prospect 列取共享鎖，Ensure 靠它與封存序列化）。
- 事件的接線在 `cmd/hawkr/agent_events.go`；模組本身不發事件。

## 2. 邀請（EnsureInvite、MarkInviteDelivered）

- 每個 RFQ 收件人一張邀請，存 prospect、寄出時的 email 快照與 secret 的 SHA-256。
- Ensure 在一個 transaction 內以共享鎖讀 prospect、再鎖住那張邀請：只有進行中的 prospect 能建立或換 secret；沒有邀請就建立，還沒寄出就換 secret 並重讀 email。
- 結果以 `errors.Is` 辨認：已撤銷 `ErrInviteRevoked`、prospect 不是進行中（封存、被合併、已轉成店家）`ErrProspectNotActive`、已寄出 `ErrInviteAlreadyDelivered`、沒有可寄的 email `ErrNoDeliverableEmail`。
- 既有邀請掛的 prospect 與輸入不同時回 InvalidInput，邀請原樣。
- `ExpiresAt` 必須晚於現在、不超過現在起 180 天，否則 InvalidInput。
- Mark 帶寄出的那一把 secret，以條件 UPDATE 比對 SHA-256，只有它仍是目前的 secret 才標記；過時的回 `ErrInviteSecretSuperseded`，同一把重送冪等。
- 併發：同一個 RFQ 收件人的 Ensure、Mark、兌換與撤銷以邀請列鎖序列化；Ensure 以唯一鍵去重，呼叫端看不到 unique violation。鎖的順序固定是 prospect 再邀請，與封存一致。

## 3. 兌換與預覽（ExchangeInvite、PreviewInvite）

- 邀請在到期或撤銷之前可重複兌換，每次發新的 session；兌換不撤銷任何既有 session。
- 預覽沒有副作用，回遮罩的 email、到期時間，以及邀請與收件人的 ID（只給入口判斷請求帶的 session 是不是這張邀請的，不進對外 API）。
- 兌換與預覽的不可用（未知、格式錯、過期、撤銷）一律同一個 NotFound。secret 不是剛好 32 bytes 的標準編碼就不查資料庫。

## 4. session（ValidateSession、VerifySession）

- 期限：idle 24 小時滑動（寫入節流 5 分鐘）、絕對 7 天，且不超過邀請的到期時間（建立時被邀請的到期截斷；idle 不超過絕對期限，CHECK）。
- 一張邀請同時最多 5 個有效 session。超過時在同一個 transaction 內（邀請列加鎖）撤銷多出來的：候選排除本次新發的（它一定未驗證），先撤最舊的未驗證，全部都已驗證才撤最舊的已驗證。
- 有效的條件只有一處：session 沒撤銷、idle 與絕對期限都沒到，且邀請沒撤銷、沒到期；寫在 `ValidateSession` 與 `VerifySession` 的 SQL 述詞裡，兩者一致。滑動與驗證在同一個 statement，寫入失敗就是 `ValidateSession` 失敗。
- `ValidateSession` 的不可用一律同一個 Unauthenticated；`VerifySession` 的不符或不可用一律同一個 PermissionDenied。
- `VerifySession` 是一個條件 UPDATE（CAS）：session 有效、還沒驗證、驗證的 email 與邀請寄出時的快照去前後空白且不分大小寫相等。相同 email 重送冪等。驗證紀錄（方式、驗證的 email、時間）綁單一 session，DB 的 CHECK 要求三者同有同無；方式目前只有 `GOOGLE`。比對對象是快照，prospect 之後改 email 不影響。
- 模組只收呼叫端已驗證的 email，不驗 Google token。

## 5. 秘密

- 邀請 secret 與 session token 都是 32 bytes 的 `crypto/rand`，以 base64url（無 padding）輸出；資料庫只存原始 bytes 的 SHA-256（`bytea`，CHECK 長度 32、UNIQUE）。明文只在 `IssuedInvite.Secret`、`IssuedSession.Token` 出現一次，對呼叫端是不透明字串，不得解析。
- 查詢以 hash 為鍵，不需要常數時間比較。

## 6. 撤銷與合併

- seller 在封存 prospect 的 transaction 內發 `ProspectArchived`；`OnProspectArchived` 在同一個 transaction 撤銷該 prospect 的全部邀請與底下的 session，並記 `revoke_reason`（`PROSPECT_ARCHIVED`）。
- 撤銷時間與原因同有同無（CHECK）；撤銷不可逆（沒有還原的路徑，prospect 還原不復活邀請）；已撤銷的邀請不得再換 secret 或兌換；重送撤銷事件不改原本的時間與原因。
- `OnProspectMerged` 不撤銷：把輸家的邀請改掛贏家（`prospect_id` 改成贏家，email 快照、secret、session、驗證都不動）。這是唯一改變 `prospect_id` 的路徑；`rfq_recipient_id` 是 UNIQUE，改掛不會與贏家既有的邀請衝突。
- 兩個 handler 在 transaction 外被呼叫時拒絕；發布方 rollback 時撤銷一併 rollback。
- 封存之後沒有有效的邀請：Ensure 與封存序列化（讀 prospect 帶 `FOR SHARE`），封存先到則 Ensure 回 `ErrProspectNotActive`，Ensure 先到則封存的撤銷會撤掉這張邀請。

## 7. 驗證證據

- session 上的驗證紀錄只用於授權，隨 session 清理，不是證據。
- 正式送出、撤回、婉拒時，入口以 `X-Vendor-Portal-*` header 把 `SessionInfo` 的 `SessionID` 與 `Verification{Method, Email, At}` 交給 gateway（API 設計規範（Connect）§二），gateway 轉給 ai：正式送出存在不可變的報價版本列，撤回與婉拒存在 ai 只追加的指令紀錄。`ValidateSession`、`VerifySession` 交出這四項。

## 8. 清理

- River 每天一個 job（啟動即執行），分批、每批一個 transaction：刪除絕對期限過了超過 7 天的 session；刪除到期或撤銷超過 90 天的邀請連同 `invited_email`（session 由 `ON DELETE CASCADE` 帶走）。
- `invited_email` 是這個模組唯一的個資，只存在邀請列。

## 9. 資料

- 代碼值是 UPPER_SNAKE；Go 的值集合與 CHECK 一致，資料庫對寫入端出錯的不一致列有第二道防線。
- `vendor_portal_invites`：`invite_id`（UUIDv7）、`rfq_recipient_id`（UNIQUE，ai-service 的 ID）、`prospect_id`（seller 的 ID，無 FK）、`invited_email`、`secret_hash`、`secret_delivered_at`、`expires_at`、`revoked_at`、`revoke_reason`。
- `vendor_portal_sessions`：`session_id`（UUIDv7）、`token_hash`、`invite_id`（FK，`ON DELETE CASCADE`）、`verification_method`、`verified_email`、`verified_at`、`idle_expires_at`、`absolute_expires_at`、`revoked_at`、`last_seen_at`。

## 10. 已知限制

- `rfq_recipient_id` 屬於 ai-service，模組無法驗證它存在，只檢查是正整數。RFQ 發布選出收件人時，發布端要自己排除封存的業者（Ensure 已拒絕非進行中的 prospect，但發布端要在寄信之前就排除）。
- Ensure 與 Mark 是兩段式（`secret_delivered_at`、換 secret）：Send 成功但 Mark 失敗後的重送會換 secret，先寄出的信失效。同一把 secret 重寄目前做不到。
- 兩個併發的 Ensure 對同一張還沒寄出的邀請都會成功，後到的換掉先到的 secret：先到的呼叫方拿到的連結失效，它的 Mark 回 `ErrInviteSecretSuperseded`。同一個 RFQ 收件人一次只該有一個寄信的呼叫方。
- 驗證方式只有 Google；email OTP 尚未提供。
- 邀請的壽命上限是固定的 180 天，不由 RFQ 截止日推導。
- 買家取消 RFQ 發布時，模組不會撤銷邀請。
