# seller/internal/kyc

賣家身分認證（KYC）與防詐身分雜湊。

## 不變量

- KYC 個資只存密文，型別不帶明文，顯示全文先稽核。守門：`TestKYCNeverReturnsOrStoresPlaintext`。規格：§13。
- 防詐身分雜湊（`account_identity_hashes`，只有 blind index）與 `kyc_identity_hashes` 同一個 transaction 寫入、不隨個資清理（`model.StoreRetention`）刪除，保存期是 `model.IdentityHashRetention`；措施快照從它取；銀行帳號命中只列出、不能建立關聯終止。守門：`TestIdentityHashesOutliveThePIIPurgeAndFeedLaterSnapshots`、`TestBankAccountHitsAreListedButNeverTerminate`。規格：§13。
