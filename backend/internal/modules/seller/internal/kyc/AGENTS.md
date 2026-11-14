# seller/internal/kyc

賣家身分認證（KYC）與防詐身分雜湊。

## 不變量

- 儲存草稿與送出只看申請的狀態（`model.CanEditKYC`）：草稿與被退回的可以；審核中只能取消；已核准的不能編輯也不能重送。店名與介紹沒有鎖。守門：`TestCanEditKYCOnlyForDraftAndRejected`、`TestSavingAndSubmittingDependOnlyOnTheApplicationStatus`。規格：§13。
- 平台不收身分證號、銀行帳號、證件影像與個人姓名（Stripe 計畫 D22、D37）：申請的表與輸入、檢視型別都沒有這些欄位。公司補充文件是機敏檔，有後台的 KYC 讀取權限就能讀，每次讀取先寫一列 `kyc_document_accesses`（誰、哪一張、何時）才讀內容。守門：`TestKYCHasNoPersonalIdentityOrBankColumnsOrFields`、`TestKYCDocumentAccessIsRecordedBeforeTheBytesAreRead`。規格：§13。
- 防詐身分雜湊（`account_identity_hashes`，只有 blind index）的寫入只有兩處：申請的統編由 `writeHashes` 在申請寫入的同一個 transaction 寫入（申請沒有自己的 hash 欄位或表，草稿「有沒有改」直接比明文統編，D42），provider 的銀行帳號 fingerprint（`BANK_FINGERPRINT`）由 `Fingerprints.RecordBankFingerprints` 在收款同步的 transaction 寫入；`BANK_FINGERPRINT` 不在 `hashFields`，修剪與補算不碰它。守門：`TestBankFingerprintIsNotAnApplicationHashField`。
- 防詐身分雜湊不隨個資清理（`model.StoreRetention`）刪除，保存期是 `model.IdentityHashRetention`；措施快照從它取，命中的身分是快照加上被處分帳號還在保存表的雜湊（定義只在 SQL 的 `measure_identity_hits(at)`，`match` 不判斷措施是否生效）；銀行帳號命中只列出、不能建立關聯終止。守門：`TestIdentityHashesOutliveThePIIPurgeAndFeedLaterSnapshots`、`TestSharedProviderBankFingerprintIsListedButNeverTerminates`。規格：§13。
