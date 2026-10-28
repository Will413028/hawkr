# delegation 模組行為規格

本文是單體 `delegation` 模組行為的唯一來源。代行營運的授權、工作階段、操作稽核與管理員代行資格。

## 1. 範圍與公開介面

- 判定店家授權與管理員資格、建立與撤銷工作階段、核發 credential、兌換一次性 handoff code。
- 管理店家的代行授權及只追加的治理歷史（§4）。
- 記錄兩階段操作稽核與唯讀回看；確認成功時在同一個 transaction 發出 `DelegationOperationCompleted`（§6）。
- gateway 編排跨服務寫入，經 `internal/legacy/delegationservice` 呼叫模組的公開 API；舊契約的業務碼與文字映射在該轉接層。gateway、領域服務與兩個前端的代行行為見 [詳細設計書 56](../詳細設計書/56-ADM%20代行營運（Delegated%20Operations）詳細設計書.md)。
- 根套件只組裝 `Module`、轉呼叫、公開 `internal/model` 的型別別名與事件。
- 模組沒有 `db/pub.sql`，不發布 `pub_*`。

## 2. 資料

五張表在 schema `delegation`（`db/schema.sql`），只保存 identity 的管理員 ID 與其他模組的帳號、店家 ID，不對其他 schema 下 FK；管理員身分資料由 identity 擁有，顯示資料經 identity 的公開 API 取得。

- `admin_eligibilities`：管理員代行資格，只存管理員 ID；沒有列即未開通。
- `delegated_operation_sessions`：操作者、對象、店家快照、理由、能力與期限；credential、handoff code 只存雜湊。主鍵是 bigint IDENTITY（舊 gateway 與 admin 契約以數字 ID 指稱）。
- `seller_delegation_authorizations`：每店終生一列的授權；全域列有定義但營運不建立。
- `delegation_authorization_audit_logs`：授權治理歷史及冪等紀錄，只追加。
- `admin_operation_audit_logs`：先寫 PENDING intent、再以 CAS 確認一次；`resource_id` 原樣保存為 text。

TTL、閒置、handoff、分頁與保留天數是 `internal/model` 的常數，不是環境變數。

讀取模型：無。清單、時間線及摘要由本模組的表直接查詢。

## 3. 工作階段

- 狀態只儲存 `active`、`revoked`。hard 到期（1800 秒）與閒置到期（900 秒）在讀取時推導，所有「進行中」查詢都含兩個到期述詞。
- 結束代行把 `ended_at` 設為現在、`expires_at` 取 `LEAST(expires_at, 現在)`（不早於 `issued_at` 加一秒），`revision` 加一；對已終止的 session 重放是冪等。
- `revision` 只在結束與撤銷時增加；輪詢、活動延長與代行讀寫不改它。session 列永不硬刪，只有 `admin_eligibilities` 會 `DELETE`。
- 輪詢（`ResolveDelegation` 的 activity 為假）不延長閒置期限；代行資源的反查（activity 為真）才會延長。
- 管理員同時只代行一家、店家同時只被一位管理員代行，由 partial unique index 與寫入檢查守住。
- 建立只交付一次性 handoff code；handoff code 與 credential 都只存 SHA-256。兌換以一條 CAS 同時清掉 code 雜湊並寫入 credential 雜湊，credential 是兌換時以 CSPRNG 核發的 256 bit 隨機值。同一個 code 再兌換回 9282；已兌換 session 的同鍵重放只回摘要，不再發 code 或改寫雜湊。
- handoff code 與 credential 不進 log。
- 同一個冪等鍵併發建立共用一個 session；尚未兌換時的重放重簽 handoff code（新的雜湊與期限，不改 `revision`），已核發 credential 的列不重簽。
- 建立時，同一個 transaction 先結束同一管理員與同一店家已到期（hard 或閒置）卻仍佔著 partial unique index 的列，再檢查衝突。
- handoff code 有效 60 秒，前綴 `dhof_`；credential 前綴 `dops_`。
- 能力是封閉的九項；Go 規則與 DB CHECK 都拒絕履約、退款與金流能力。
- `display_code` 格式為 `OPS-` 加 13 碼 0-9A-Z，以 CSPRNG 產生，撞號重試。
- 開始代行要有管理員角色的權限 `delegation.sessions.create`（`platform/adminperm`）：`StartDelegation` 在自己的 transaction 內以 identity 的 `AdminAllows`（`FOR SHARE`）檢查，沒有時回 `DenialAdminNotAllowlisted`（permission denied）。代行資格（`admin_eligibilities`）獨立於角色。

### 撤銷

- 模組自己寫入的 `revoke_reason` 是固定值（`internal/model` 的 `RevokeReason*`），DB 只有長度的 CHECK：登出 `admin_logout`、撤回資格 `delegation_disabled`、撤回店家授權 `authorization_revoked`、撤銷管理員角色 `admin_role_revoked`。
- `RevokeDelegationSession` 的理由由呼叫端帶入（1–200 字）：gateway 在授權鏈第 8 項命中時帶 `store_not_operable`（`pkg/delegation`），人工撤銷帶操作者填的文字。
- 角色撤銷的 `revoked_by_admin_id` 是 NULL：系統因角色變動而撤銷，沒有操作者；誰撤的在 identity 的 `admin_role_events`。
- 後台登出時由 `internal/httpapi/identity` 直接呼叫 `RevokeDelegationsOfAdmin`。
- 撤回代行資格時，刪除資格與撤銷進行中的 session 在同一個 transaction，任一處失敗整個回滾。
- 訂閱 identity 的 `AdminRoleRevoked`（`OnAdminRoleRevoked`，組裝點 `newDelegation` 訂閱）：剩下的角色不再允許 `delegation.sessions.create` 時，在撤銷角色的同一個 transaction 撤銷該管理員進行中的代行。
- 撤銷保留較早的結束時間。

## 4. 店家授權

- 授權寫入與治理歷史同 transaction；撤回同時撤銷該店進行中的工作階段。
- 店家層級授權列一旦存在，即遮蔽全域列，無論它是否有效。
- 治理需要已開通資格的管理員；授權的店家必須存在。
- 授權的三處冪等鍵以唯一鍵與 `ON CONFLICT DO NOTHING` 守住；重放比對 action、resource 與 payload hash，不同 payload 回衝突。
- `revision` 僅在結束或撤銷時增加；授權寫入以互斥 CAS 判定。

## 5. 店家狀態

- 店家狀態由模組直接呼叫 seller 的公開 API 讀取（`Deps.Seller`，根套件的 `storeStateOf`）。`Deps.Seller` 必填，nil 時建構就 panic。
- 只讀店家的場合（運維子命令 `delegation-eligibility`、`admin-role` 與 characterization 測試）傳 `seller.NewReader(db, now)`，只要資料庫；`serve` 傳 seller 模組。
- 只有店家不存在是 `DenialStoreNotDelegable`（legacy 轉接層對應 9265）。已關閉的店家存在，狀態是 `Found` 加 `Closed`，拒絕是 `DenialStoreClosed`（9265 只在 legacy `bizCodes`）。
- 可營運與原因來自 seller 的 `StoreActionDelegate`（`Store.ActionUnavailableReasons`，由 `CapStorefrontWrite` 缺的條件對應：已關閉、等待刪除、停權）。等待刪除與停權是 `DenialStoreNotOperable`、9272。
- preflight 的標籤與 `StartDelegation` 被拒時的錯誤是 seller 給的句子（`seller.StoreActionUnavailableText`）；delegation 不自己寫這些句子，不可營運而沒有句子是內部錯誤。
- seller 的其他錯誤原樣傳出：不是拒絕，也沒有「讀不到」的狀態（seller 在行程內）。

## 6. 操作稽核

- 兩階段：寫入前先寫 PENDING intent；confirm 只從 PENDING 轉移一次，成功且需通知時在同一個 transaction 發出事件。事件發不出去則 confirm 回滾。
- 讀取、揭露與更正類動作不能通知店家；confirm 只在成功時檢查能力，`SUCCEEDED` 列的能力必須在該 session 的授權快照內。
- confirm 只更新 `result`、`resource_id`、`after_snapshot`、`changed_fields`、`resource_version_after`、`biz_code`、`confirmed_at` 與 `notified_at`，其餘欄位寫入後不變；稽核列永不硬刪。
- `origin=scheduled` 是排程執行的 intent：session 已到期仍可寫入，已撤銷仍拒絕；只支援貼文、必須帶 `resourceId`；操作理由繼承同一 session、同一資源最近一筆 `POST_SCHEDULE` 的理由，找不到回 9279。
- 事件只有 `DelegationOperationCompleted`；結束與撤銷不發事件。
- intent 需要可用的工作階段與有效輸入；冪等鍵綁定指令、資源與 payload。
- 批次上限 50。
- 動作（19 個）、原因碼與能力都有 DB CHECK。
- 保留 730 天，只下發給畫面；清理是維運 SQL，不含 session。

## 7. 管理員代行資格

- 代行資格沒有後台畫面；營運以 `hawkr delegation-eligibility enable|disable` 呼叫 `SetAdminEligibility`，撤回時同步撤銷進行中的工作階段。
- 建立 session 時鎖住資格列，避免與撤回交錯後留下 active session。

## 8. 分頁

- 列表以 OFFSET 分頁並回總數；分頁參數經驗證，不截斷。
