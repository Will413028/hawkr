# delegation 模組

代行營運的授權、工作階段、操作稽核與編排歸本模組；現階段舊 gateway 仍編排跨服務寫入，經 `internal/legacy/delegationservice` 呼叫公開 API，A10 才將編排搬進單體。依據設計書 56、`docs/adr/2026-09-27-delegation-module.md` 與遷移計畫 0.12。

## 職責

- 判定店家授權與管理員資格、建立與撤銷工作階段、核發 credential、兌換一次性 handoff code（設計書 56 §5.1、§7）。
- 管理店家的代行授權及只追加的治理歷史；撤回授權時連帶撤銷進行中的工作階段（§5.1.2）。
- 記錄兩階段操作稽核與唯讀回看；確認成功時在同一個 transaction 發出 `DelegationOperationCompleted`（§5.1.3、§8.4）。
- 管理員代行資格由本模組管理；撤回資格時在同一個 transaction 撤銷進行中的工作階段。後台登出的 Connect handler 直接呼叫本模組撤銷。
- 開始代行要有管理員角色的權限 `delegation.sessions.create`（`platform/adminperm`）：`StartDelegation` 在自己的 transaction 內以 identity 的 `AdminAllows`（`FOR SHARE`）檢查，沒有時回 `DenialAdminNotAllowlisted`（permission denied）。訂閱 identity 的 `AdminRoleRevoked`（`OnAdminRoleRevoked`，組裝點 `newDelegation` 訂閱），剩下的角色不再允許 `delegation.sessions.create` 時，在撤銷角色的同一個 transaction 撤銷該管理員進行中的代行。代行資格（`admin_eligibilities`）仍獨立於角色，A10 搬代行時併入角色。

## 擁有的資料

schema `delegation`（`db/schema.sql`）：

- `admin_eligibilities`：管理員代行資格，只存 identity 的管理員 ID；沒有列即未開通。
- `delegated_operation_sessions`：操作者、對象、店家快照、理由、能力與期限；credential、handoff code 只存雜湊。過渡期主鍵是 bigint IDENTITY。
- `seller_delegation_authorizations`：每店終生一列的授權；全域列有定義但營運不建立。
- `delegation_authorization_audit_logs`：授權治理歷史及冪等紀錄，只追加。
- `admin_operation_audit_logs`：先寫 PENDING intent、再以 CAS 確認一次；`resource_id` 原樣保存為 text。

五表只保存 identity 的管理員 ID 與其他模組的帳號、店家 ID，不對其他 schema 下 FK。管理員身分資料仍由 identity 擁有。TTL、閒置、handoff、分頁與保留天數是 `internal/model` 的常數；credential 在 handoff 兌換時由 CSPRNG 核發。

## 模組內部結構

- 根套件只組裝 `Module`、轉呼叫、公開 `internal/model` 的型別別名與事件；整合測試在根套件，只經公開 API。
- 根套件的 `Deps` 是組裝 API，各子領域宣告最小 `Deps`。`internal/session` 管工作階段的寫入、狀態與管理員登出撤銷；`internal/authorization` 管授權與治理歷史，撤回時經 session 撤銷；`internal/audit` 管稽核和回看，經 session 判斷工作階段是否可用。
- `internal/model` 放公開型別及不碰 I/O 的規則；`internal/store` 是 sqlc 生成的資料存取葉節點。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- credential 與 handoff code 只以 SHA-256 存；handoff 以一條 CAS 兌換一次，兩者不進 log。管理員同時只代行一家、店家同時只被一位管理員代行，由 partial unique 及寫入檢查守住。
- 狀態只儲存 active、revoked；hard 與閒置到期在讀取時推導，所有「進行中」查詢都含兩個到期述詞。輪詢不延長閒置期限，僅代行資源反查會延長。
- 九項能力是封閉列舉，Go 規則與 DB CHECK 均拒絕履約、退款與金流能力。店家層級授權列一旦存在，即遮蔽全域列，無論是否有效。
- 授權寫入與治理歷史同 transaction；撤回同時撤銷該店進行中的工作階段。稽核 intent 是寫入前置；confirm 只從 PENDING 轉移一次，成功且需通知時在同 transaction 發事件。
- 撤銷的原因是系統寫入的固定值（`internal/model` 的 `RevokeReason*`，`revoke_reason` 只有長度的 CHECK）：登出 `admin_logout`、撤回資格 `delegation_disabled`、撤回店家授權 `authorization_revoked`、撤銷管理員角色 `admin_role_revoked`。角色撤銷的 `revoked_by_admin_id` 是 NULL（系統因角色變動而撤銷，沒有操作者；誰撤的在 identity 的 `admin_role_events`）。
- 後台登出時由 `internal/httpapi/identity` 直接呼叫 `RevokeDelegationsOfAdmin`；撤回代行資格時刪除資格與撤銷 active session 同 transaction，任一處失敗整個 transaction 回滾。

## 已知限制

- 工作階段 bigint ID 是舊 social-service、gateway 與 admin 契約的過渡型別；最後一個數字 ID 消費端遷走後，在階段 C 改 UUID（row-identity ADR）。
- 店家狀態由模組直接呼叫 seller 的公開 API 讀取（`Deps.Seller`，根套件的 `storeStateOf`）：只有不存在是 `DenialStoreNotDelegable`（legacy 轉接層對應 9265）；已關閉的店家存在，狀態是 `Found`＋`Closed`，拒絕是 `DenialStoreClosed`（9265 只在 legacy `bizCodes`）；可營運與原因都來自 seller 的 `StoreActionDelegate`（`Store.ActionUnavailableReasons`，由 `CapStorefrontWrite` 缺的條件對應：已關閉、等待刪除、停權；等待刪除與停權是 `DenialStoreNotOperable`、9272），preflight 的標籤與 `StartDelegation` 被拒時的錯誤是 seller 給的句子（`seller.StoreActionUnavailableText`），delegation 不自己寫這些句子，不可營運而沒有句子是內部錯誤；seller 的其他錯誤原樣傳出（不是拒絕，也沒有「讀不到」的狀態：seller 在行程內）；`Deps.Seller` 必填，nil 時建構就 panic（只讀店家：`serve` 傳 seller 模組，運維子命令 `delegation-eligibility`、`admin-role` 與 characterization 測試傳 `seller.NewReader(db, now)`，只要資料庫）。
- 兩階段稽核保留到 A10；跨模組寫入與稽核屆時可在單體 transaction 內處理。現行保留 730 天只下發給畫面，清理仍是維運 SQL，不含 session。
- 代行資格沒有後台畫面；營運以 `hawkr delegation-eligibility enable|disable` 呼叫本模組的 `SetAdminEligibility`，撤回時同步撤銷進行中的工作階段（設計書 56 §8.5.3）。

## 讀取模型

無；清單、時間線及摘要由本模組的表直接查詢，管理員顯示資料經 identity 的公開 API 取得。

## 不負責

- 管理員身分與 Access 驗證：identity。identity 不 import delegation。
- 店家狀態、商品、貼文、訂單等業務資料：各資料擁有者；本模組只保存必要快照，店家狀態在需要時向 seller 讀取。
- HTTP 認證、限流與錯誤映射：`internal/httpapi`；過渡期舊契約映射在 `internal/legacy/delegationservice`。

## 決策紀錄

從 account-service 沿用的每個機制、當時的決定與理由，以及與 account-service 的差異在同目錄的 `CARRYOVER.md`。改動或移除沿用的機制前，先讀那一列。
