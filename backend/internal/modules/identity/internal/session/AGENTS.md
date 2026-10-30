# identity/internal/session

顧客 session 的存放、逾時與換發。

## 不變量

- session 只存 token 的 SHA-256；閒置與絕對逾時是 `WebIdle`、`AppIdle`、`Absolute`，`last_used_at` 最多每 `RenewEvery` 寫一次。守門：`TestSessionIdleAndAbsoluteExpiry`、`TestSessionRenewalWritesAtMostOncePerDay`。規格：§1。
- 登入一律換發新 token。守門：`TestLoginIssuesAFreshSessionAndRevokesThePreviousOne`。規格：§1。
- 停權與刪帳在同一個 transaction 刪該帳號全部 session。守門：`TestSuspensionAndSessionDeletionAreAtomic`、`TestDeletionIsAtomicAcrossSessionsAndIdentities`。規格：§4。
