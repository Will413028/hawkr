# seller/internal/kyc

賣家身分認證（KYC）與防詐身分雜湊。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- KYC 個資只存密文，型別不帶明文，顯示全文先稽核。守門：`TestKYCNeverReturnsOrStoresPlaintext`。規格：§13。
- 防詐身分雜湊（`account_identity_hashes`，只有 blind index）與 `kyc_identity_hashes` 同一個 transaction 寫入、不隨 184 天個資清理刪除，來源 stint 關店滿 5 年才刪；措施快照從它取；銀行帳號命中只列出、不能建立關聯終止。守門：`TestIdentityHashesOutliveThePIIPurgeAndFeedLaterSnapshots`、`TestBankAccountHitsAreListedButNeverTerminate`。規格：§13。
