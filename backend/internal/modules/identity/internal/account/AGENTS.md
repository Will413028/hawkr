# identity/internal/account

帳號建立、角色、停權與刪除的生命週期。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/identity.md`。

## 不變量

- 新帳號不授予角色（買家由 buyer 訂閱 `AccountCreated` 建立）。Clerk 時期的 `clerk_user_id` UNIQUE 在 Clerk 退場前保留。規格：§2。
- 刪帳與（有 Clerk 使用者時）排入刪除它的 job 在同一個 transaction，刪帳留下墓碑並發出 `AccountDeleted`；停權不排 Clerk job，認證結果帶 `SuspendedAt`、不回錯誤。規格：§4。
- 停權紀錄與 `suspended_at` 同一個 transaction，未解除的紀錄最多一筆；停權、解除入口只由 `accountsuspension` 呼叫。規格：§4。
- 帳號生命週期 ACTIVE → PENDING_DELETION → DELETED；等待刪除時 `GrantRole` 回 FailedPrecondition。規格：§4。
- 授予、移除角色與刪帳都先鎖帳號列；`DeleteAccount` 只由 accountdeletion 呼叫。守門：無（只有規格）。規格：§3、§4。
- 不是角色的值（例如 `BUYER`）授予或移除回 InvalidInput。規格：§3。
