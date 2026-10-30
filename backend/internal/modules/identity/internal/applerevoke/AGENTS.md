# identity/internal/applerevoke

Sign in with Apple 授權撤銷。

## 不變量

- Apple 憑證只在 ID token 與連結時的身分相同才保存；身分不符或帳號已刪除才當場撤銷，暫時性失敗不撤銷。守門：`TestExchangeRevokesWhenTheCodeBelongsToSomeoneElse`。規格：§7。
- 刪帳在同一個 transaction 取走 token 並排入撤銷。守門：`TestAppleTokenIsStoredReplacedAndRevokedOnDeletion`。規格：§7。
