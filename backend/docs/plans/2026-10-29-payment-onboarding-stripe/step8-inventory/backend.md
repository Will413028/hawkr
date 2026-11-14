# 步驟 8 刪除盤點：backend Go（`backend/internal/**`、`backend/cmd/**`）

快照：worktree `hawkr-payment-onboarding`，分支 `payment-onboarding`，commit `499c5ae8`。路徑一律相對 `backend/`。`S/` = `internal/modules/seller/`。
唯讀盤點；沒有 build、沒有跑測試。**「讀過」= 我讀了該段程式；「推論」= 只看了 grep 命中行、函式簽名或檔名，沒讀本體。**

計畫依據（`docs/plans/2026-10-29-payment-onboarding-stripe.md`）：步驟 8 範圍 255 行、消費端 256、允許清單 257、驗收 258；D6（269）、D9 修訂（279）、D11 修訂（285）、D14（293）、D15（294）、D17（296）、D18（297）、D22（301）、D24 Q2（303）；不變量表 36–49。

重現用的 grep：

```
git grep -c -i payuni -- backend/internal backend/cmd ':!**/gen/**'            # 156 檔
git grep -c -E 'id_card|IDCard|IdCard|rep_id_card|bank_account|BankAccount|bank_code|BankCode|bank_book|BankBook|ID_CARD_NO|REP_ID_CARD_NO|BANK_ACCOUNT|kyc_identity_hashes|account_identity_hashes|CheckKYCKeys|Reveal|DocumentAccess|CheckNameUnlocked|IdentityLocked|KYCFlowStatus|BINDING|BIND_FAILED|FORWARDED|PENDING_INFO|merchant\.verification' -- backend/internal backend/cmd ':!**/gen/**'
rg -n '\bpayuni\.[A-Z]\w+' internal/modules/seller -g '!internal/payuni/**' -g '*.go' -o   # seller/internal/payuni 的呼叫點
rg -l 'platform/payuni"' internal cmd                                           # platform/payuni 的 importer（14 檔，12 個是測試）
```

---

## 1. 檔案表

類別縮寫：PAYUNI = PAYUNI-onboarding；PII = D22-PII；其餘照派工的名稱。

### 1.1 整包或整檔刪除

| 路徑 | 內容 | 類別 | 處置 | 會壞／要跟上的 | 測試 |
|---|---|---|---|---|---|
| `internal/platform/payuni/`（4 檔：`client.go`、`client_test.go`、`crypto.go`、`limiter.go`） | PayUni 代理商 API client、加解密、限流（推論：只看檔名與命中數） | PAYUNI | delete（D6 選 A） | importer：`cmd/hawkr/seller.go:21`、`S/internal/payuni/{payuni,observe,sync}.go`，以及 10 個 integration 測試（見 1.6） | `client_test.go` 一起刪 |
| `S/internal/payuni/`（10 檔：`intent.go` 352 行、`sync.go` 345、`observe.go` 338、`jobs.go` 185、`payuni.go` 109、`storedata.go` 78、`convert.go`、`AGENTS.md`、2 個測試） | 綁定 intent、River job、觀測、同步；對外符號（由呼叫點得知）：`RequestBind`、`BindIntent`、`IntentParams`、`CheckResubmit`、`IdentityLocked`、`CheckNameUnlocked`、`SellerViewOf`/`SellerView`、`MissingStoreFields`、`NewlyMissing`、`SyncedFieldsChanged`、`RequestSync`、`Resume`、`Rebind`、`Refresh`、`Receive`、`Observe`、`Poll`、`Sweep`、`Sync`、`Registration`、`Observations`、`KYCSource`、`BindingKYC`、`Client`、`ErrPermanent`（本體推論） | PAYUNI ＋ D17（`CheckNameUnlocked`、`IdentityLocked`）＋ D14（`Rebind`） | delete；**`MissingStoreFields`／`NewlyMissing` 先搬家或另決**（見 §3-U5） | 呼叫點（讀過）：`S/internal/kyc/admin.go:469`、`S/internal/kyc/kyc.go:172,176,181,259,274,323,343,351,413,416,501,504,645`、`S/internal/kyc/payuni.go`、`S/internal/profile/profile.go:381,399,486,487`、`S/internal/closing/closing.go:381`、`S/internal/devseed/devseed.go:621`、`S/internal/stint/flow.go`、`S/seller.go:30,99,148,642,659`、`S/payuni.go` | `jobs_test.go`（含不變量表的 `TestSyncJobIsUniquePerStoreInRiver`，推論）、`storedata_test.go` 一起刪 |
| `S/internal/model/payuni.go`（659 行） | `PayUniBindStatus`、`PayUniOutcome`、`PayUniSource`、`PayUniRemedy`（:89）、`PayUniProblem`、`ClassifyPayUniCode`、`PayUniRegistration`、`RebindPayUniInput`、`PayUniNotification`、`PayUniObservation*`、**`KYCFlowStatus` 11 值（:417–441）**、`IdentityLockedBy`（:455）、`KYCFlow`（:463）、`KYCFlowStatusOf`、`DeriveKYCFlow`、`FormatMerID`（:576）、事件 `PayUniForwarded/Approved/PendingInfo/UnderReview/Rejected/BindFailed`（:605–653）；**也放了非 PayUni 的 `StoreFieldNames`（:133）** | PAYUNI＋D15＋D17＋D14＋D18 | delete；`StoreFieldNames` 與 `StoreField*` 常數去留見 §3-U5 | `S/payuni.go` 的 alias、`internal/httpapi/seller/{kyc,admin_kyc,admin}.go`、`S/internal/stint/flow.go`、`S/internal/profile/admin.go:49–54,95–103`、`S/internal/model/{kyc.go:262,265,266; model.go:279,289; store_action.go:142,143,181,223; kyc_resubmit.go}`、`internal/legacy/sellerserver/server.go:187,437`（`FormatPayUniMerID`） | `payuni_test.go`（345 行）刪 |
| `S/internal/model/payuni_state.go`（175 行） | `PayUniState` 8 值、`SellerPayUni`、`SellerPayUniOf`、`LockedIdentityFields`（:175） | PAYUNI＋D17 | delete | `S/internal/kyc/diff.go:26`（`LockedIdentityFields`）、`kyc_resubmit.go:69`、httpapi `toSellerFlow` | `payuni_state_test.go`（372 行）刪 |
| `S/payuni.go`（207 行） | 模組公開 alias 與方法：`PayUniClient`、`KYCFlow*`、`SellerPayUni`、`KYCResubmitUnavailableReason`、`PayUniErr*`、`Remedy*`，以及 Rebind／Refresh／Receive／Observations 等方法（推論：只看 outline） | PAYUNI | delete | httpapi seller 的 `AdminModule` 介面、`internal/legacy/sellerconsumer`、`sellerrelay`、`cmd/hawkr/seller.go:67` | `payuni_integration_test.go`（336 命中）、`payuni_state_integration_test.go` 刪 |
| `S/internal/kyc/payuni.go`（46 行，讀過） | `BindingData`（解密身分證號給 bind_merchant，:15–36）、`CompanyPhone`（:40） | PAYUNI＋PII（national ID／rep ID） | delete | 只被 `payuni.KYCSource` 用 | — |
| `S/db/queries/payuni.sql`（179 行，18 支查詢：`PayUniRegistrationByStore` … `PayUniRegistrationsToResync`） | 見檔內 `-- name:` | PAYUNI | delete；`S/internal/store/payuni.sql.go` 由 `make sqlc` 消失 | `S/internal/profile/profile.go:403`（`PayUniRegistrationByStore`）、`S/internal/kyc/kyc.go:172,413,501`（`LockPayUniRegistration`）、devseed `:621,632,635` | — |
| `cmd/hawkr/payuni_config_test.go` | `readPayUni` 的設定測試（推論） | PAYUNI | delete | — | 本身 |
| `internal/legacy/sellerconsumer/`（2 檔，`consumer.go` 讀過） | 消費 RabbitMQ `seller.review.result`（Java payuni-webhook-service 的 AGENT_NOTIFY）→ `ReceivePayUniNotification`（:21–23,48–65）；queue `hawkr.monolith.seller.review.result`（:73） | D18／legacy-seam | delete 整個套件（唯一用途就是 PayUni 審核通知） | `cmd/hawkr/main.go:49,453–463`（`consumerDone` 容量 2 與第二個 goroutine） | `consumer_test.go` 刪 |
| `internal/legacy/sellerrelay/payuni_notices.go`＋`notices_test.go` | `payuniUnknown`、`payuniNotices` 對照表（:4,:10） | D18／legacy-seam | delete | `relay.go:393,403`（`pendingInfoReason`、`rejectedReason`） | `notices_test.go` 刪 |

### 1.2 seller 模組：改（移除符號）

| 路徑 | 內容（file:line） | 類別 | 處置 | 會壞／要跟上的 | 測試 |
|---|---|---|---|---|---|
| `S/internal/kyc/admin.go`（648 行，讀過 107–169、300–648） | `Approve` 內 `payuni.RequestBind`（:469）；`RevealSensitive`（:304–364，解密三個密文欄位並建立 15 分鐘授權）；`ReadDocument`（:566–613）；`ListReveals`（:181）、`ListDocumentAccesses`（:214）；`adminViewWith` 的 `bankShared`（:126,135）；`MatchCertHolder` 讀 `row.RealName`（:647） | PAYUNI（:469）；PII（三個密文）；shared-keep（授權、讀檔、稽核：公司文件仍要用） | edit：刪 :469；`RevealSensitive` 的明文部分刪除，**授權與文件定位的去留見 §3-U1**；`bankShared` 見 §3-U3 | proto：`adminpb.RevealKYCSensitiveResponse` 的身分證號／銀行帳號欄位、`KYCApplication.shares_bank_account_with_sanctioned_account`（名稱推論） | `internal/httpapi/seller/admin_kyc_test.go`、`admin_kyc_integration_test.go`、`S/kyc_integration_test.go`、`S/admin_integration_test.go` |
| `S/internal/kyc/kyc.go`（662 行，讀過 40–662） | import payuni（:23）；`SaveDraft` 的 `LockPayUniRegistration`／`CheckResubmit`／`checkIdentityUnchanged`（:170–184）；`draftBase` 的 `CheckResubmit`（:259）；`checkIdentityUnchanged`（:265–282）；`sellerView`／`viewWithReviews`／`viewWith` 的 `payuni.SellerViewOf`、`out.Flow`、`out.PayUni`、`out.ResubmitReasons`（:322–381）；`Submit`（:409 允許從 APPROVED 重送、:413–418）；`Cancel` 的 `IdentityLocked`（:500–510）；`missingStoreFields` → `payuni.MissingStoreFields`（:645）；`requirements` 已只剩公司名稱與統編（:591–594）；`Deps.Keys`（:50） | PAYUNI＋D15＋D17 | edit：移除上列；`Submit` 的來源狀態與「已核准可否重送」見 §3-U6；`Keys` 見 §3-U4 | `model.KYCApplication.{Flow,PayUni,ResubmitReasons}`；proto `KYCFlow`、`SellerPayUni`、`KYCApplication.flow` | `internal/kyc/complete_test.go`（`TestRequirementsAreMarketplaceDataOnly` 保留）、`S/kyc_integration_test.go`（112 命中） |
| `S/internal/kyc/draft.go`（274 行，讀過 64–274） | `clearIndividual`（:67）、`clearCompany` 的 rep 部分（:74）；`mergeDraft` 的 `RealName`、`BirthDate`、`IDCardIssue*`、`EnglishName`、`BankName`（:119–139）；個人不得填 `RepIDCardNo`、公司不得填 `IDCardNo`（:110–116）；`applyIdentity` 的身分證號（:189–198）、統編 hash（:199–208，**保留**）；`setID`（:212）；`applyBank`（:229–274） | PII（national ID／rep ID／bank）；`RealName` 等見 §3-U2 | edit：刪 `setID`、`applyBank`、`clearIndividual`、身分證號兩段；統編段保留 | `model.KYCDraftInput` 的欄位；httpapi `kycDraftInput`；proto `KYCApplication` 的 `individual.id_card_no`、`business.rep_id_card_no`、`bank_code`、`bank_account` | `kyc_integration_test.go`、`kyc_identity_hashes_integration_test.go` |
| `S/internal/kyc/hashes.go`（188 行，讀過） | `fieldIDCard`、`fieldRepIDCard`、`fieldBank`（:16–19）、`hashFields`（:26）、`linkedIdentityOf`（:29–41）、`storedHash`、`hashInput`（解密，:67–96）、`writeHashes`（同時寫 `kyc_identity_hashes` 與 `account_identity_hashes`，:100–118）、`syncHashes`、`RehashIdentities` | PII；`fieldTaxID`、`fieldBankFingerprint` 是 shared-keep | edit：`hashFields` 剩 `TAX_ID`；`storedHash`／`hashInput` 的三個分支刪；`linkedIdentityOf` 的舊種類見 §3-U7 | `kyc.sql` 的 `KYCApplicationsMissingHashKey`（:322–341） | `kyc_identity_hashes_integration_test.go`、`account_identity_hashes_integration_test.go`、`fingerprints_test.go`（`TestBankFingerprintIsNotAnApplicationHashField`） |
| `S/internal/kyc/identity.go`（97 行，讀過） | `match`：解密身分證號、負責人身分證號、銀行帳號重算 hash（:24–47）；`blocked`／`bankShared`（:69–74） | PII | edit：只剩統編（:36–38）；`bankShared` 永遠 false → §3-U3 | `admin.go:126,454`、`kyc.go:433` | `kyc_integration_test.go`、`termination_lifecycle_integration_test.go`（推論） |
| `S/internal/kyc/protect.go`（100 行，讀過） | `colIDCardNo`、`colRepIDCardNo`、`colBankAccount`（:15–19）；`protector.keys`、`protect`、`open`、`aad`（:22–60）；`idInput`、`bankInput`（:72,76）；`CheckKeys`（:81–100，錯誤訊息提到 `KYC_ENCRYPTION_KEYS`） | PII；`taxInput`、`hashes`、`mac` shared-keep | edit：三個密文欄位沒了之後 `keys`／`protect`／`open`／`aad` 沒有呼叫者 → §3-U4 | `ReferencedKeyIDs`（`kyc.sql:364–387`）的 `ENCRYPTION` 段 | `platform/seal/seal_test.go`（推論：測的是 seal 本身，不受影響） |
| `S/internal/kyc/convert.go`（讀過 19–118） | `toApplication` 的遮蔽（:34–52）與 `RealName`…`BankCode` 的複製（:25–27）；`insertParams`／`updateParams` 的密文與 hash 欄位（:84–118） | PII | edit | sqlc 參數型別 `InsertKYCApplicationParams`、`UpdateKYCDraftParams` | — |
| `S/internal/kyc/diff.go`（讀過） | `draftDiff.identity()`（:22–32，用 `model.LockedIdentityFields`）；`fieldDiffs` 的 `bank_code`、`bank_account`、`individual.id_card_no`、`business.rep_id_card_no`（:83–84,88,97）；`secret()`（:53，解密比對） | D17＋PII | edit：`identity()` 與 `secret()` 刪；`fieldDiffs` 跟 `KYCFieldNames` 同步 | `diff_test.go` 的 `TestFieldDiffsCoverEveryNonDocumentField` 會守住 | `diff_test.go` |
| `S/internal/kyc/documents.go`（264 行，讀過 26–105） | `singles`：`IDCardFront`、`IDCardBack`、`BankBook`（:35–41）；`documentField`（:45–56）；公司文件清單（:93–104，**保留**） | PII（ID-card image／bank book）；公司文件 shared-keep | edit：`singles` 變空 → 連帶的單張合併邏輯是否整段刪見 §3-U1 | `model.KYCDraftInput.{IDCardFront,IDCardBack,BankBook}`；proto `KYCApplication.id_card_front/back`、`bank_book` | `kyc_integration_test.go`、`media_sync_integration_test.go`（推論） |
| `S/internal/kyc/issues.go`（180 行，outline） | `checkIssuesAgainstApplication`、`issueResolved` 依 `KYCFieldNames`／文件位置 | shared-keep | keep；值域跟著 `KYCFieldNames` 縮（§3-U8） | `kyc_review_issues_field_check` | `issues_test.go`、`kyc_issues_integration_test.go`（25 命中） |
| `S/internal/kyc/devseed.go`、`fingerprints.go`、`registrycheck.go` | `SeedApproved`（:16）；fingerprint 寫入器；登記查詢 | shared-keep | keep；`devseed.go` 的輸入型別跟著 `KYCDraftInput` | — | — |
| `S/internal/model/kyc.go`（711 行，讀過 94–711） | `SlotIDCardFront/Back`、`SlotBankBook`（:57–61）；`ReviewRebind`（:87）；`SanctionIdentityKind` 三值（:98–110）；`kycDocumentFields`（:163–169）；`KYCApplication` 的 `IDCardNoMasked`、`BankAccountMasked`、`RepIDCardNoMasked`、`BankName`、`BankCode`、`Flow`、`PayUni`、`ResubmitReasons`（:243–266）；`KYCAdminApplication.SharesBankAccountWithSanctionedAccount`（:362）；`KYCReveal`、`KYCDocumentAccess`、`KYCSensitive`（:298–393）；`KYCDraftInput`（:418–433）；`NormalizeIDCardNo`（:560）、`NormalizeBankAccount`（:593）、`ValidBankCode`（:599）、`IdentityInput`（:603）、`BankIdentityValue`（:622）、`MaskIDCardNo`（:625）、`MaskBankAccount`（:635）、`NormalizeMobile`（:658，註解提 PayUni）、`ValidEnglishName`（:664）、`KYCFieldNames`（:670–679）；事件註解提 `merchant.verification.*`（:516–527） | PII＋PAYUNI＋D14（`ReviewRebind`）＋D15 | edit：刪身分證號／銀行帳號的正規化、遮蔽、`KYCSensitive` 的三個明文欄位、三個 slot、`ReviewRebind`；`SanctionIdentityKind` 的舊值見 §3-U7 | `kyc_reviews_action_check`（含 `'REBIND'`）、`kyc_documents_slot_check`、`kyc_document_accesses_slot_check`、`measure_identities_kind_check` | `kyc_test.go`（13 命中）、`measure_test.go` |
| `S/internal/model/kyc_resubmit.go`（116 行） | `KYCResubmitUnavailableReason` 9 值，其中 7 個 `PAYUNI_*`（:21–33）、2 個平台審核（`UNDER_REVIEW_CANCELABLE`、`UNDER_REVIEW`，:17–19）；`ResubmitUnavailableReasons` 依 `KYCFlow.CanResubmit` 與 `SellerPayUni`（:46–66） | D15＋PAYUNI | UNDECIDED（§3-U6）：7 個 PayUni 值必刪；剩 2 個值與 `CanResubmit` 要由誰推導計畫沒寫 | proto `KYCResubmitUnavailableReason`、`KYCFlow.resubmit_unavailabilities`；`S/internal/text/zh-TW.json` 的 `kyc.resubmit_unavailable.*` | httpapi `kyc_test.go` |
| `S/internal/model/store_action.go`（225 行） | `StoreActionRebindPayUni`（:27）；`StoreReasonPayUniMerchantTerminal/BindInProgress/Bound`（:47–51）；規則（:142–143）；`RebindUnavailableReasons`（:181）；`AdminPayUniRegistration`（:221–223） | D14＋PAYUNI | edit：刪上列；`StoreReasonKYCNotApproved`（:53）只剩 rebind 在用與否未核對（推論） | proto `StoreAction` 的 `REBIND_PAYUNI`、`StoreActionUnavailableReason` 的 `PAYUNI_*`；`S/store_action.go:18,41–43` | `store_action_test.go`（16 命中）、`S/store_action_integration_test.go`（29） |
| `S/internal/model/agreement.go`、`agreement_gen.go` | `AgreementPayUniTerms = "PAYUNI_TERMS"`（`agreement.go:20`）、`AgreementKinds()`（:25）；`agreement_gen.go:15–17`（由 `website/scripts/generate-legal.ts` 生成，不手改） | D11 | edit `agreement.go`；`agreement_gen.go` 由 website 的 manifest 重生 | `agreement_acceptances_kind_check`、`agreement_acceptances_source_check`；`S/seller.go:280`；proto `AgreementKind`；計畫 141 行：`GetSellerAgreements` 要求每個種類都有現行版本，**兩者必須同一個 commit** | `agreement_test.go`、`S/agreement_integration_test.go`（9） |
| `S/internal/model/model.go` | `StoreListInput.PayUniBindStatus`（:278–279）；`StoreListItem.KYCFlow KYCFlowStatus`（:289）；註解（:323） | PAYUNI＋D15 | edit：刪 bind 篩選；清單的 `KYCFlow` 改成什麼見 §3-U6 | proto `admin ListStoresRequest.payuni_bind_status`、`StoreSummary.kyc_flow_status`（`internal/httpapi/seller/admin.go:137,173`） | `httpapi/seller/admin_test.go` |
| `S/internal/model/measure.go` | `LinkedNationalID`、`LinkedResponsiblePersonID`、`LinkedBankAccount`（:485–498）；`CountsAsGround`（:503）；註解提 PayUni 自由文字（:193） | shared-keep | UNDECIDED（§3-U7） | proto `LinkedIdentity`；`S/governance.go:219–222` | `measure_test.go` |
| `S/internal/model/validation.go:118–125`、`capability.go:21`、`card.go:9` | 註解與「聯絡欄位長度上限 = PayUni 上限」 | PAYUNI（規則來源） | edit：計畫 255 行「CARRYOVER 聯絡欄位長度一列改成平台自己的上限並結案」；**新的上限數字沒寫**（§3-U5） | `S/CARRYOVER.md`；DB 的 stores CHECK（未讀） | `S/seller_integration_test.go`（推論） |
| `S/internal/stint/flow.go` | `ToPayUniRegistration`（:11）、`FlowStatusesOf`（:37） | PAYUNI＋D15 | delete 檔或改成只回 `KYCStatus`（§3-U6） | `S/internal/exposure/exposure.go:41`（`KYCFlows`）→ `S/internal/profile/admin.go:95` | — |
| `S/internal/exposure/exposure.go` | `KYCFlows`（:41）；註解（:6,39–40） | D15 | edit | 同上 | `exposure_*_integration_test.go` |
| `S/internal/profile/profile.go` | import（:20）；`Deps.StorefrontBaseURL`（:39）；`CheckNameUnlocked`（:379–383）；`checkRequiredNotCleared`（:396–415，以 `PayUniRegistrationByStore` 有沒有列為條件）；`SyncedFieldsChanged`／`RequestSync`（:485–487） | D17＋PAYUNI | edit：:379–383、:485–487 刪；`checkRequiredNotCleared` 見 §3-U5 | `seller.Deps.StorefrontBaseURL`（`S/seller.go:64`）、`cmd/hawkr/seller.go:87,132` | `S/seller_integration_test.go`、`S/public_integration_test.go`（18） |
| `S/internal/profile/admin.go` | bind 狀態篩選（:49–54）；`KYCFlow`（:95–103） | PAYUNI＋D15 | edit | `stores.sql:213–218` 的 `bind_status` 參數 | `S/admin_integration_test.go`（7）、`stores_queries_test.go`（推論） |
| `S/internal/closing/closing.go` | `payuni.Resume`（:381）；註解（:362） | PAYUNI | edit | — | `internal/closing/closing_integration_test.go`（推論） |
| `S/internal/purge/purge.go`（讀過 outline） | `PurgeKYCApplications`（:247）、`PurgeKYCIdentityHashes`（:251）、`PurgeKYCRegistrationChecks`（:255）、`PurgeKYCDocuments`（:258）、**`PurgePayUniRegistrations`（:274）、`PurgePayUniObservations`（:281）**、`PurgeAgreementSources`（:278）、`Payment.ForgetOwners`（:286）、`expireIdentityHashBatch`（:294–324） | PAYUNI（:274,281）；其餘 shared-keep | edit：刪 :274、:281 與檔頭註解（:7–8） | `purge.sql:159–171` | `S/purge_integration_test.go`（21 命中）、`purge_stages_integration_test.go`、`purge_hold_race_integration_test.go` |
| `S/internal/devseed/devseed.go` | import（:24）；`PayUniMerStatus`／`PayUniMemberStatus`（:40）；清表 SQL（:111–112,120–121）；`lockPayUni`（:142–143,420,465–466）；核准檢查（:581–582）；寫註冊（:621–637） | PAYUNI | edit：整段 PayUni 種子刪 | `cmd/hawkr/dev_seed.go:28,43` | `devseed_test.go`、`authority_integration_test.go`、`cmd/hawkr/dev_seed_integration_test.go`（6） |
| `S/seller.go` | import（:30）；`Deps.PayUni`（:74）、`KYCKeys`（:72）、`StorefrontBaseURL`；`payuni.New`（:148–149）；`AddWorkers`（:642）；`PeriodicJobs`（:659）；`AgreementPayUniTerms`（:280）；註解（:63–64,163,650,678,699–701） | PAYUNI＋D11 | edit | `cmd/hawkr/seller.go:129–132` | `ctor_test.go`（推論） |
| `S/events.go` | `PayUniForwarded/Approved/PendingInfo/UnderReview/Rejected/BindFailed` alias（:76–92） | D18 | edit：刪 6 個 | `internal/legacy/sellerrelay/relay.go:163–187`、`cmd/hawkr/seller_events.go:35–39` | — |
| `S/kyc.go`（332 行） | `KYCFieldNames`（:200）、`StoreFieldNames`（:209–210）、`MatchCertHolder`（:252）、`RehashKYCIdentities`（:323）、`CheckKYCKeys`（:330）；Reveal／ReadDocument 的公開方法（推論） | shared-keep＋PII | edit：跟著 kyc 套件 | httpapi、`cmd/hawkr/main.go:231,234,396,399` | — |
| `S/store_action.go` | alias（:18,41–43） | D14 | edit | httpapi | — |
| `S/governance.go:219–222` | `Linked*` alias | shared-keep | 跟 §3-U7 | — | — |
| `S/internal/text/zh-TW.json`、`en.json`、`text_gen.go` | 模組句子：`kyc.resubmit_unavailable.payuni_*`（7 鍵，:19–43）、`payuni.state.*`（7 鍵，:176–200）、`store_action.unavailable.*.rebind_payuni`（5 鍵，:600–644）；`text_gen.go` 生成 | PAYUNI＋D14＋D15 | edit json，`text_gen.go` 重新生成 | `model/kyc_resubmit.go:81–`、`payuni_state.go:42`、`store_action.go:197` | — |
| `S/AGENTS.md`、`S/CARRYOVER.md`（74 命中）、`S/internal/kyc/AGENTS.md`、`S/internal/purge/AGENTS.md`（未讀） | 規則與沿用盤點表 | — | edit（沒讀內容，推論）；`CARRYOVER.md` 是否屬允許清單見 §3-U10 | — | — |

### 1.3 SQL 與生成物（seller）

| 路徑 | 內容 | 類別 | 處置 |
|---|---|---|---|
| `S/db/queries/kyc.sql`（400 行，讀過 142–400） | `InsertKYCApplication`／`UpdateKYCDraft`（:19–82，含密文與 hash 欄位，未逐行讀）；`InsertKYCReveal`、`ActiveKYCGrant`、`InsertKYCDocumentAccess`、`ListKYCDocumentAccesses`、`ListKYCReveals`（:148–192）；`AdminKYCApplications*` 以 `real_name` 搜尋（:204,223）；`SnapshotMeasureIdentities`（:246–252）；`LinkedAccountHits`（:269–291）；`KYCApplicationsMissingHashKey`（:325–341）；`ReferencedKeyIDs`（:367–387） | PII＋shared-keep | edit：`KYCApplicationsMissingHashKey` 與 `ReferencedKeyIDs` 的身分證號／銀行段刪（計畫 255 行「`CheckKYCKeys` 只檢查仍在用的金鑰」）；`LinkedAccountHits`、`SnapshotMeasureIdentities` 的 CASE 見 §3-U7；`real_name` 搜尋見 §3-U2 |
| `S/db/queries/purge.sql`（171 行，讀過） | `PurgeKYCApplications`（:96–128，逐欄清 NULL）、`PurgePayUniRegistrations`（:159–165）、`PurgePayUniObservations`（:167–171） | PAYUNI＋PII | edit：刪後兩支；第一支拿掉被 drop 的欄位 |
| `S/db/queries/stores.sql:213–218` | admin 清單的 `bind_status` 篩選（`EXISTS … payuni_registrations`） | PAYUNI | edit |
| `S/db/schema.sql`、`objects.sql`、`pub.sql` | 見 §2 | — | edit（宣告式 schema，與新 migration 同步） |
| `S/internal/store/*.sql.go`、`models.go` | sqlc 生成（不在 `gen/` 底下但是生成物）：`payuni.sql.go` 消失，`kyc.sql.go`（264 命中）、`purge.sql.go`、`stores.sql.go`、`models.go` 重新生成 | — | `make sqlc` |
| `S/db/migrations/*.sql`（75 檔，15 檔提到 payuni）＋`atlas.sum` | 已套用，不得修改（計畫 22 行） | allowed-residue？（§3-U10） | keep；改動寫新 migration |

### 1.4 httpapi、權限、待辦

| 路徑 | 內容 | 類別 | 處置 | proto 符號 | 測試 |
|---|---|---|---|---|---|
| `internal/httpapi/seller/admin_kyc.go`（27 個 payuni 命中） | `GetPayUniRegistration`（:237）、`RebindPayUni`（:250）、`ListPayUniObservations`（:266）、`RefreshPayUniStatus`（:308）、`toAdminPayUni`（:468–）、`toObservation`（:496）；`RevealKYCSensitive`（:154）、`ListKYCReveals`（:179）、`ListKYCDocumentAccesses`（:205）；`toAdminKYC`（:344，`Flow` :385–386）；`documentFieldPB`（:336） | PAYUNI＋D14；Reveal 三支 shared-keep／UNDECIDED（§3-U1） | edit：刪 4 支 PayUni handler 與轉換 | `SellerAdminService.{GetPayUniRegistration,RebindPayUni,ListPayUniObservations,RefreshPayUniStatus}`、`adminpb.PayUniRegistration`、`PayUniObservation`、`pb.PayUniBindStatus`、`pb.KYCFlowStatus`、`pb.PayUniProblem`、`RevealKYCSensitive*` | `admin_kyc_test.go`（36＋48）、`admin_kyc_integration_test.go`（40＋37） |
| `internal/httpapi/seller/admin.go` | `AdminModule` 介面（:30–34 等）；`msg.GetPayuniBindStatus()`（:137–138）；`KycFlowStatus`（:173）；`out.Kyc.FlowStatus`（:221–222） | PAYUNI＋D15 | edit | `ListStoresRequest.payuni_bind_status`、`StoreSummary.kyc_flow_status`、`Store.kyc.flow_status`（訊息名推論） | `admin_test.go`、`admin_integration_test.go` |
| `internal/httpapi/seller/kyc.go`（389 行，outline） | `kycDraftPaths`／`kycSubmitPaths`（:38,49）；`kycDraftInput`（:100–190）；`toKYC`（:245–310）；`storeFieldPB`（:328）；`toFlow`（:333–334，`CanResubmit`、`IdentityLocked`）；`toSellerFlow`（:338）；`toProblem`（:361） | PII＋D15＋D17＋PAYUNI | edit | `pb.KYCApplication` 的身分證號、銀行、證件欄位；`pb.KYCField`（22 值）；`pb.KYCFlow`（`status`、`can_resubmit`、`identity_locked`、`resubmit_unavailabilities`）；`pb.SellerPayUni`、`pb.PayUniState`、`pb.PayUniProblem`、`pb.PayUniRemedy`；`pb.StoreField`；`SaveKYCDraftRequest.update_mask` | `kyc_test.go`（26＋32）、`kyc_integration_test.go`（16＋28）、`kyc_etag_test.go` |
| `internal/httpapi/seller/admin_document.go`（讀過 outline） | 後台讀證件圖片的 HTTP 端點：`documentURI`（:29）、`ServeHTTP`（:52）、`documentRequest`（:72） | shared-keep（公司文件仍經它讀） | keep, narrowed：slot 值域縮成 `COMPANY_DOCUMENT` | — | `admin_kyc_integration_test.go` |
| `internal/httpapi/seller/declaration.go:62–71` | 註解；身分比對的限流理由「身分證號與統編存在與否的探測面」 | shared-keep | edit 註解；限流規則留（統編仍在） | — | `declaration_test.go` |
| `internal/httpapi/rpc/fieldmask.go:57` | 註解以 `individual.id_card_no` 當巢狀路徑的例子 | — | edit 註解；**oneof 子訊息的巢狀路徑支援在 `individual` 沒有欄位之後是否還有使用者，未核對**（§3-U9） | `KYCApplication.individual` | `rpc/updatemaskfields_test.go`（7 命中，推論：用 KYC 欄位當 fixture） |
| `internal/platform/adminperm/adminperm.go` | `SellerPayUniRegistrationsGet/Rebind/Refresh`、`SellerPayUniObservationsList`（:58–61,131）；`SellerKYCRevealsList/Create`、`SellerKYCDocumentAccessesList`、`SellerKYCDocumentsGet`（:54–57,130） | PAYUNI（4 個）；shared-keep（4 個） | edit：刪 4 個 `seller.payuni_*`；另外 4 個跟 §3-U1 | — | `httpapi/identity/admin_roles_integration_test.go`（1 命中，推論：列舉權限字串） |
| `internal/httpapi/adminauth/adminauth.go` | 宣告表 :79–82（PayUni 4 支）、:73,74,78（Reveal 類） | PAYUNI | edit：刪 :79–82；宣告表與 service descriptor 不一致時單體不啟動（不變量表 45 行），所以**和 proto 生成物同一個 commit** | `SellerAdminService…Procedure` 常數 | adminauth 的測試（未列，推論） |
| `internal/httpapi/reviewqueue/review_queue.go` | `rq.PayUniBindRetry: adminperm.SellerPayUniRegistrationsGet`（:46）；kind 對應（:248）；註解（:34） | D14 | edit | `ReviewQueueItemKind.REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY` | `review_queue_test.go`（4） |
| `internal/modules/reviewqueue/internal/model/model.go:25,33`、`reviewqueue.go:51` | `PayUniBindRetry Kind = "PAYUNI_BIND_RETRY"` | D14 | edit | 同上 | `reviewqueue_integration_test.go`（1）、`S/pub_review_items_integration_test.go`、`S/review_policy_integration_test.go`（26） |
| `internal/arch/modules.go:108,112,114,115,122` | seller 內部套件圖的 `payuni` 節點與 4 個依賴它的套件；計畫 46 行說還有 `platform/payuni`（我只看到這 5 行命中，platform 那筆未定位） | PAYUNI | edit | — | `arch` 的測試 |
| `internal/arch/storedcodes_test.go:17` | 提到 `kyc_review_issues_asset_check`（「id_card_front 等」） | shared-keep | edit 說明文字 | — | 本身 |

### 1.5 cmd、legacy、platform 其他

| 路徑 | 內容 | 類別 | 處置 | 會壞／要跟上的 |
|---|---|---|---|---|
| `cmd/hawkr/seller.go` | import（:21）；`payuniSetup`（:65–69）；`readPayUni`（:71–108）；env：`PAYUNI_BASE_URL`、`PAYUNI_AGENT_ID`、`PAYUNI_HASH_KEY`、`PAYUNI_HASH_IV`、`PAYUNI_TIMEOUT`、`PAYUNI_MAX_QPS`、`PAYUNI_STOREFRONT_BASE_URL`（:73–87）；`newSeller` 的 `pay` 參數（:129–132）；`kycKeys.enc`、`KYC_ENCRYPTION_KEYS`、`KYC_HASH_KEYS`（:27–57） | PAYUNI；金鑰是 PII／shared-keep | edit：刪 `readPayUni` 與 7 個 env；`KYC_ENCRYPTION_KEYS` 見 §3-U4；`PAYUNI_STOREFRONT_BASE_URL` 只給 MerURL 用（`S/seller.go:64`） | `main.go:159,341`、`seller_commands.go:53`；本機 `.env` 範例與部署文件（不在我的範圍） |
| `cmd/hawkr/main.go` | `readPayUni`（:159,341）；`CheckKYCKeys`／`RehashKYCIdentities`（:231–234,396–399，**保留**）；`sellerconsumer`（:49,453–463）；`sellerrelay.NewWorker`（:439，保留） | PAYUNI＋D18 | edit | — |
| `cmd/hawkr/seller_commands.go:53`、`seller_events.go:27,35–39` | `readPayUni`；5 個 `sellerrelay.OnPayUni*` 訂閱；`OnKYCRejected`（:40）見 §3-U11 | PAYUNI＋D18 | edit | `cmd/hawkr/seller_relay_integration_test.go`（1 命中） |
| `cmd/hawkr/dev_seed.go:28–47` | 種子的 `PayUniMerStatus`、`IDCardNo: "A123456789"`、`BankCode`、`BankAccount`、`RealName`、`BirthDate`、`IDCardIssue*`、`EnglishName`、`BankName` | PAYUNI＋PII | edit；`RealName` 等跟 §3-U2 | `dev_seed_integration_test.go`（6） |
| `cmd/hawkr/payment_config_test.go:25` | `"PAYMENT_PROVIDER": "payuni"` 當「不支援的 provider」例子 | allowed-residue？ | UNDECIDED（§3-U10）：換別的字串或列入允許清單 | — |
| `internal/httpapi/entry_test.go:285`、`internal/httpapi/rpc/web_origin_test.go:37` | 測試資料 `https://payuni.example`、`https://www.payuni.com.tw/t`；`/payment/return` 路由在 `internal/httpapi/origin.go:14`（買家付款的瀏覽器回跳，D 類，推論） | D-class／allowed-residue？ | UNDECIDED（§3-U10） | — |
| `internal/legacy/sellerrelay/relay.go`（48 命中） | Args 型別與 `Kind()`：`legacy.seller_payuni_{forwarded,approved,pending_info,under_review,rejected}`（:81–121）；`OnPayUni*`（:163–187）；`AddWorkers`（:228）；`merchant()` 發 `merchant.verification.<key>`（:361–366）；`payUni*` worker（:368–384）；`kycRejected`（:388）；`pendingInfoReason`、`rejectedReason`（:393,403） | D18／legacy-seam | edit：刪 5 個 PayUni 事件；`kycRejected`（管理員退回也發 `merchant.verification.rejected`）見 §3-U11 | `relay_test.go`（22＋7）、`wire_test.go`、`relay_integration_test.go` |
| `internal/legacy/sellerserver/server.go` | `PayuniMerId: seller.FormatPayUniMerID(s.StoreCode)`（:187，`sellerMeta`）；`MerId: seller.FormatPayUniMerID(st.StoreCode)`（:437，`GetSellerShippingProfile`）；註解（:177） | legacy-seam | edit：:187 回空值（計畫 30、255 行）；**:437 計畫沒提**（§3-U12）；`PayuniMerId`／`MerId` 是 Java 契約生成的欄位名，屬允許清單 | `server_test.go`（3）、`server_integration_test.go`（11＋3，import `platform/payuni`）、`S/legacy_integration_test.go`（8） |
| `internal/legacy/gen/**`（`seller.pb.go`、`order.pb.go` 等） | Java 契約生成物 | allowed-residue | keep（B2 刪） | — |
| `internal/platform/codemeta/code_meta.go`（71 命中） | 生成檔（檔頭「DO NOT EDIT」，:1–5）：`PayuniMerchantVerificationStatus`（:545–590，商店審核狀態，屬開通）、`RefundStatus`（:500–）、`PayUniSubmitStatus`（:1047–，物流）、`LabelPrintStatus` 等。`internal`、`cmd` 沒有任何 Go 檔 import `codemeta`（`rg -l 'codemeta\.' internal cmd -g '!internal/platform/codemeta/**'` 0 筆） | D-class（退款、物流）＋PAYUNI（`PayuniMerchantVerificationStatus`） | UNDECIDED（§3-U13）：計畫寫「CodeMeta 照本機脈絡檔規則」，我讀到的規則是 `backend/AGENTS.md:68`（凍結、不手改生成檔） | — |
| `internal/platform/uitext/catalog/zh-TW.json`（92 個 payuni 命中）、`en.json`、`config.json` | 鍵：`seller.KYCField.*` 22 個（:193–277）；`seller.KYCFlowStatus.*` 與 `.action/.banner_*/.gate_*`（:281–456，及 :1051–1209 的 enum 清單與排除表）；`seller.PayUniState.*` 8 個（:777–805）；`seller.SellerPayUni.fix_fields_heading`、`reason_heading`（:933,937）；`seller.StoreField.*` 5 個（:969–985）。文字值裡的 PayUni 字樣未逐筆讀 | PAYUNI＋D15＋PII | edit，之後 `make uitext`（`catalog_gen.go`、`golden_calls_gen_test.go`、`testdata/golden.gen.json` 與 TS、Dart 生成物重生）；`KYCFlowStatus` 的 gate／banner 文案搬去哪見 §3-U6 | `uitext_test.go`（4 命中）；兩端 `check-enum-text` |
| `cmd/uitext-gen/names.go:11` | 註解以 `seller.KYCFlowStatus.gate_message` 為例 | — | edit 註解 | — |
| `internal/modules/payment/AGENTS.md`（1 命中） | 未讀 | — | 推論：敘述性文字，edit | — |
| `internal/modules/{buyer,content,discovery}/CARRYOVER.md` | 指向 B 階段的列（buyer、content 在允許清單；**discovery 不在**，257 行） | allowed-residue | keep；discovery 那 2 筆見 §3-U10 | — |
| `internal/platform/stripe/**`、`internal/modules/payment/internal/account/stripe*.go` | `bank_account`、`ListBankAccountFingerprints`：Stripe 外部帳戶 | shared-keep | keep（步驟 6 的 fingerprint） | — |
| `internal/modules/delegation/**`（`AuditOrderPIIReveal`） | 訂單個資的檢視稽核，與 KYC 無關 | shared-keep | keep | — |

### 1.6 受影響的測試（未讀內容，依 grep 命中分類）

- 整檔刪（10）：`internal/platform/payuni/client_test.go`、`S/internal/payuni/{jobs,storedata}_test.go`、`S/internal/model/{payuni,payuni_state}_test.go`、`S/payuni_integration_test.go`、`S/payuni_state_integration_test.go`、`cmd/hawkr/payuni_config_test.go`、`internal/legacy/sellerconsumer/consumer_test.go`、`internal/legacy/sellerrelay/notices_test.go`。
- import `platform/payuni` 的 integration 測試（要改組裝）：`S/{store_action,revalidate,exposure,purge,legacy,public,exposure_announce}_integration_test.go`、`internal/legacy/sellerserver/server_integration_test.go`。
- 以 PayUni 條件建資料的曝光測試（計畫 105 行點名）：`S/exposure_rule_dataset_integration_test.go`（8）、`exposure_integration_test.go`（12）、`exposure_announce_integration_test.go`（12）、`exposure_plan_integration_test.go`（3）。
- PII：`S/kyc_integration_test.go`（112，含 `TestKYCNeverReturnsOrStoresPlaintext`，計畫 258 行要改寫成「沒有這些欄位」）、`kyc_identity_hashes_integration_test.go`（21）、`account_identity_hashes_integration_test.go`（19）、`bank_fingerprint_integration_test.go`（18）、`kyc_issues_integration_test.go`（25）、`migration_integration_test.go`（13＋4）、`termination_lifecycle_integration_test.go`（12＋3）、`review_policy_integration_test.go`（26）、`agreement_integration_test.go`（9）。
- 其餘見 §5 的命令（63 個測試檔）。

---

## 2. DB

已套用的 migration 全部不得修改（計畫 22 行）；以下都寫進**新** migration，並同步 `S/db/schema.sql`、`objects.sql`、`pub.sql`。行號是 `schema.sql`。

### 2.1 必刪（計畫有寫）

| 物件 | 現況 | 定義它的 migration | 動作 |
|---|---|---|---|
| 函式 `seller.review_payuni_bind_retry(at)` | `objects.sql:254–264` | `20261021105752_seller_payuni_bind_retry_review`（後續在 `20261027164917`、`20261102122621`、`20261106120300` 重建過） | DROP；`seller.pub_review_items` 的 UNION ALL 拿掉最後一段（`objects.sql:308–309`），`pub.sql:25` 的註解同步（「10 種」變 9 種） |
| 表 `seller.payuni_observations` | :1364–1392；索引 `payuni_observations_store`、唯一鍵 `payuni_observations_event_key` | `20261002142021_payuni` | DROP TABLE |
| 表 `seller.payuni_registrations` | :1301–1355；`payuni_registrations_mer_id`、`_poll`、`_unsynced`；`payuni_registrations_purged_check`（:1348）；`bind_failed_at`（`20261021105717`）；FK → `kyc_applications` | `20261002142021_payuni`、`20261002143416_seller_pii_purge`、`20261007072959_seller_contact_to_store`、`20261021105717` | DROP TABLE（先 drop 上面的函式）。不變量表 43 行的 `payuni_registrations_purged_check` 隨表消失；取代它的是 payment 模組的最終清理（`purge.go:286`） |
| `agreement_acceptances_kind_check` | `kind IN ('SELLER_TERMS','KYC_PRIVACY_NOTICE','PAYUNI_TERMS')`（:1601） | `20261007142254`，後在 `20261015074609`、`20261027164917` 改過 | 改成兩值（D11：不轉換舊紀錄）。**既有 `PAYUNI_TERMS` 列怎麼處理計畫只說「不轉換」**：新 CHECK 會被既有列擋下，migration 要先 DELETE 或 CHECK 保留舊值（§3-U14） |
| `agreement_acceptances_source_check` | `kind IN ('KYC_PRIVACY_NOTICE','PAYUNI_TERMS') AND submission_review_id IS NOT NULL`（:1605–1608） | 同上 | 跟著改 |
| `kyc_reviews_action_check`、`kyc_reviews_reviewer_check` | 含 `'REBIND'`（:1185–1189）；註解 :1171 | `20261002133159_seller_kyc`、`20261002214348_seller_review_fixes` | 刪 `REBIND`（D14）；既有列同 §3-U14 |
| `kyc_identity_hashes_field_check` | `('ID_CARD_NO','REP_ID_CARD_NO','TAX_ID','BANK_ACCOUNT')`（:1048） | `20261016090100_kyc_identity_hashes` | 剩 `'TAX_ID'`；先 DELETE 三種的既有列（計畫 255 行點名這三種） |
| `kyc_applications` 密文與 hash 欄位 | `id_card_no_enc`、`id_card_hash`、`id_card_hash_key`（:931–933）；`bank_account_enc`、`bank_account_hash`、`bank_account_hash_key`（:940–942）；`rep_id_card_no_enc`、`rep_id_card_hash`、`rep_id_card_hash_key`（:953–955） | `20261002133159_seller_kyc`（及之後的修訂） | DROP COLUMN ×9；`bank_code`（:939）是銀行帳號 blind index 的輸入，一起刪 |
| `kyc_applications` 的 CHECK | `kyc_applications_id_card_check`（:985）、`_rep_id_card_check`（:988）、`_bank_account_check`（:991）、`_bank_code_check`（:973）；`_hash_length_check`（:995–998）；`_type_fields_check`（:1000–1005，提到 `rep_id_card_no_enc`、`id_card_no_enc`）；`_purged_check`（:1015–1025） | `20261002133159`、`20261006231005_kyc_company_type_drop`、`20261002143416`、`20261007072959`、`20261102232910`、`20261103124804` | 前四個 DROP；後三個重新定義 |
| `kyc_documents_slot_check`、`kyc_documents_position_check` | `('ID_CARD_FRONT','ID_CARD_BACK','BANK_BOOK','COMPANY_DOCUMENT')`（:1087–1088） | `20261002133159_seller_kyc` | 剩 `COMPANY_DOCUMENT`；既有三種 slot 的列要先刪並把 asset 記成退場（media 引用由同步 job 解除，`documents.go:254` 的 `retireApplicationDocuments` 是現成做法；migration 內做不到「記成退場」→ §3-U14） |
| `kyc_document_accesses_slot_check`、`_position_check` | 同四值（:1275–1276） | `20261003165655_seller_kyc_document_grants` | append-only 稽核表，既有列是否保留舊 slot 值 → §3-U1 |
| `kyc_review_issues_field_check`、`_asset_check` | 22 個欄位路徑（:1213–1236）；asset 限 `id_card_front`、`id_card_back`、`bank_book`、`business.company_documents`（:1239–1242） | `20261011180100_kyc_review_issues`，後在 `20261015074609`、`20261102232910`、`20261103124804` 改 | 跟 `KYCFieldNames` 同步（§3-U8）；append-only，既有列同 §3-U14 |

### 2.2 計畫沒寫、但跟著動或要決定的

| 物件 | 現況 | 備註 |
|---|---|---|
| `kyc_applications.real_name`、`birth_date`、`id_card_issue_date`、`id_card_issue_type`、`id_card_issue_location`、`english_name`、`bank_name` 與各自的 CHECK（:929–938,966–972） | 明文欄位，7a 之後不再必填、表單已刪（`step7-screen-drafts.md:18`） | §3-U2 |
| `measure_identities_kind_check` | `('ID_CARD','TAX_ID','BANK_ACCOUNT')`（:904）；定義在 `20261012214337_seller_measures` | §3-U7 |
| `account_identity_hashes_field_check` | 五值（:1071）；`20261030090000`、`20261103021937` | 計畫 255 行「已有的列保留到 5 年清理」→ CHECK 必須留舊值；之後不再有寫入者 |
| `kyc_reveals`（:1248–1260）、`kyc_document_accesses`（:1264–1278） | 顯示全文的授權與讀圖稽核 | §3-U1 |
| view `seller.store_requirement_flags` | `objects.sql:18–36`：只讀 `kyc_applications` 與 `payout_capabilities`，**已不引用 PayUni**（最後定義在 `20261110160932_store_requirement_flags_two_sources`） | keep，不用動；`objects.sql:14` 的註解仍提 `DeriveKYCFlow`，要改 |
| `seller.review_kyc_application`、`pub_exposable_stores` | 不引用 PayUni（`objects.sql:238–249`） | keep |
| 索引註解 `kyc_registration_checks_pending`（:1164–1165） | 提 `payuni_registrations_unsynced` 的先例 | 改註解 |
| `stores` 的聯絡欄位長度 CHECK | 未讀；`validation.go:118` 說上限就是 PayUni 的 | §3-U5 |

### 2.3 purge 套件讀什麼（`S/internal/purge/purge.go`，三段）

1. 第一段退場（30 天，`model.StoreRetireAfterHours`）：`RetireDueStores`、`StoreHolds`、`RetireStores`（`purge.sql:1–84`）。不碰 KYC 或 PayUni。
2. 第二段個資清理（184 天，`model.StoreRetentionHours`），同一個 transaction（`purge.go:187–286`）：`PurgeStores` → `PurgeKYCApplications`（:247）→ `PurgeKYCIdentityHashes`（:251）→ `PurgeKYCRegistrationChecks`（:255）→ `PurgeKYCDocuments`（:258，回傳的 asset 記成退場）→ **`PurgePayUniRegistrations`（:274）**→ `PurgeAgreementSources`（:278）→ **`PurgePayUniObservations`（:281）**→ `Payment.ForgetOwners`（:286）。粗體兩支刪。
3. 防詐 hash 過期（5 年，`model.IdentityHashRetentionHours`）：`ExpiredIdentityHashStores`、`DeleteExpiredAccountIdentityHashes`（`purge.go:294–324`）。保留；這是舊種類列唯一的刪除路徑。

---

## 3. 計畫沒決定的（不替 Will 決定，只列選項與證據）

**U1 顯示全文、限時授權、讀圖稽核在只剩公司文件之後的形狀。** 現況：`RevealSensitive` 同時做兩件事——回三個明文欄位，並建立 15 分鐘授權讓同一位管理員讀證件圖片（`admin.go:301–364`；`model.RevealGrantTTL`，`kyc.go:296`）；`ReadDocument` 沒有授權一律 PermissionDenied（`admin.go:577–580`），每次讀寫一列 `kyc_document_accesses`（:603–606）。D24 Q2 保留公司補充文件（303 行），D22 只說刪「證件」（301 行）。選項：(a) 保留整套（RPC 改成只發授權、回文件位置；`KYCSensitive` 剩 `GrantID`、`ExpiresAt`、`Documents`）；(b) 公司文件不是 D22 的證件，改成有後台權限就能讀、仍記稽核，刪 `kyc_reveals` 的授權語意；(c) 連讀取稽核也不留。連帶：4 個權限字串（`adminperm.go:54–57`）、3 支 RPC（`adminauth.go:73,74,78`）、兩張 append-only 表既有列的 slot 值、`documents.go` 的單張位置邏輯（`singles` 變空，:35–41）。

**U2 `real_name`、`birth_date`、`id_card_issue_*`、`english_name`、`bank_name` 是否屬於 D22。** 管轄的句子：D22「平台不再收身分證號、銀行帳號與證件」（301 行）；步驟 8「刪身分證號、銀行帳號、證件的 proto 欄位、request 驗證、密文欄位與證件檔參照」（255 行）；D9 修訂「平台不收身分與銀行資料…平台審核只管市集政策與統編」（279 行）；7a 只拿掉必填並寫「不動：proto 欄位、密文欄位、證件檔、enum」（139、149 行）；表單刪掉「身分證明、銀行帳戶、…英文名稱」（`step7-screen-drafts.md:18`），留下的清單沒有姓名與生日（:20–26）。這七欄不在步驟 8 的列舉裡。選項：全刪（與 D9 修訂一致）；或部分保留。**刪 `real_name` 有三個連帶**：`MatchCertHolder` 以 `row.RealName` 比對政府證書的業者名稱（`admin.go:620–647`，個人賣家沒有 `company_name` 時它是唯一基準）；後台審核佇列以 `real_name` 搜尋（`kyc.sql:204,223`）；種子（`cmd/hawkr/dev_seed.go:30,45`）。`english_name` 與 `ValidEnglishName`（`model/kyc.go:663`）是 PayUni 的商店英文名稱，`bank_name` 沒有其他讀者（未逐一核對，推論）。

**U3 `SharesBankAccountWithSanctionedAccount`。** 現在由 `match` 以申請的銀行帳號重算 hash 得出（`identity.go:39–47,72–73`；`admin.go:126,135`；`model/kyc.go:355–362`）。申請沒有銀行帳號後永遠是 false。選項：刪欄位（proto、admin 畫面）；或改以 provider fingerprint 推導（`account_identity_hashes` 的 `BANK_FINGERPRINT` 對 `measure_identities` 的 `BANK_ACCOUNT`，現成的連接只在 `LinkedAccountHits` 的第二段，`kyc.sql:282–289`，方向是措施→帳號，不是申請→措施）。

**U4 加密金鑰環 `KYC_ENCRYPTION_KEYS`。** 三個密文欄位是 `seal.Keyring` 在 seller 唯一的用途（`protect.go:40–60`；`rg 'seal\.Keyring'` 只有 `cmd/hawkr/seller.go:27`、`S/seller.go:72`、`S/internal/kyc/{kyc.go:50,protect.go:23}`）。計畫只寫「`CheckKYCKeys` 只檢查仍在用的金鑰」（255 行）。選項：移除 `Deps.KYCKeys`、env 與 `parseKYCKeys` 的加密半邊（`cmd/hawkr/seller.go:33–57`），`KYC_HASH_KEYS` 留；或保留 env 不用。`platform/seal` 的 `Keyring` 型別是否還有別的使用者：`internal`、`cmd` 的非測試檔 0 個（上述 rg）。

**U5 店家聯絡欄位的規則（原本是「PayUni 必填」）。** 三處：(1) 送出前必填 `MissingStoreFields`（`kyc.go:425–431,639–646`）——計畫 154 行明寫不能動，但實作在 `seller/internal/payuni`，`StoreFieldNames` 在 `model/payuni.go:133`，要有新家；(2) `checkRequiredNotCleared`：有 PayUni 註冊之後不能清掉（`profile.go:396–415`），條件是 `PayUniRegistrationByStore` 有列——表沒了之後，選項是刪規則、或改以「申請已送出／已核准」為條件；(3) 長度上限「就是 PayUni 的上限」（`validation.go:118–125`：聯絡人 20 字、信箱 50 字、手機 09 開頭）——計畫只寫「改成平台自己的上限並結案」（255 行），數字沒寫（維持現值改理由，或放寬）。

**U6 `KYCFlow`／`CanResubmit`／清單的流程欄位由誰取代。** D15 把賣家端狀態拆成平台審核五值與收款帳戶（294 行），步驟 8 只列「`KYCFlowStatus` 的 PayUni 值與 `BINDING`／`BIND_FAILED`／`FORWARDED`／`UNDER_REVIEW`／`PENDING_INFO`」（255 行）。沒寫的：(a) 剩下的 `NOT_STARTED/DRAFT/SUBMITTED/REJECTED/APPROVED` 是留一個縮小的 `KYCFlowStatus`，還是整個 enum 刪掉改用 `KYCStatus`（7a 已加 `seller.KYCStatus` 的徽章文字，144 行）；共用文案目錄有 `seller.KYCFlowStatus.{action,banner_*,gate_*}` 一整組（`zh-TW.json:325–456`），刪 enum 就要搬家；(b) `KYCFlow.CanResubmit` 與 `KYCResubmitUnavailableReason` 剩 2 個平台審核值（`kyc_resubmit.go:17–19`）——留在 `KYCApplication`，或併入 7e1 的 `OnboardingTask`；(c) `Submit` 現在允許 `APPROVED` 的申請重送（`kyc.go:409`，原因是「已核准但 PayUni 拒絕」），沒有 PayUni 之後已核准的申請能不能再編輯或重送；(d) admin 清單的 `StoreListItem.KYCFlow`（`model.go:289`、`profile/admin.go:95–103`）與 `bind_status` 篩選。7c 的備註也點到 `MyKyc.fromApplication` 仍要求 `flow.status`（339 行）。

**U7 舊身分種類在防詐管線的去留。** 計畫只說 `account_identity_hashes` 既有列留 5 年（255 行）。實際行為（讀過）：`SnapshotMeasureIdentities` 把 `ID_CARD_NO`／`REP_ID_CARD_NO` 複製成快照的 `ID_CARD`（`kyc.sql:246–252`）；但能命中它的兩條路都會消失——`match` 靠申請的密文重算（`identity.go:24–35`），`LinkedAccountHits` 第一段連的是 `kyc_identity_hashes`（`kyc.sql:273`，不是 `account_identity_hashes`）。所以留下的舊列只會被複製進快照，再也不會被比中。選項：(a) 照字面保留列與全部列舉值（`SanctionIdentityKind` 的 `ID_CARD`、`measure_identities_kind_check`、`LinkedIdentity` 的 `NATIONAL_ID`／`RESPONSIBLE_PERSON_ID`、proto 與 admin 文字），接受它們是死值；(b) 保留列但把 `LinkedAccountHits` 改連 `account_identity_hashes`，讓舊列仍能列出關聯帳號；(c) 刪舊種類的列與列舉（與「保留 5 年」牴觸，要改決定）。未上線、只有種子資料（285 行）是相關事實。`LinkedBankAccount` 因 fingerprint 仍在用（`hashes.go:37`）。

**U8 退回問題的欄位值域與既有列。** `KYCFieldNames` 22 值（`model/kyc.go:670–679`）＝ proto `KYCField` ＝ `kyc_review_issues_field_check`（`schema.sql:1213–1236`）＝ 文案 `seller.KYCField.*`。刪欄位後要一起縮；`kyc_review_issues` 是 append-only（`schema.sql:1198–1201`），既有列指到被刪的欄位時 CHECK 會擋。原因碼 `UNREADABLE`、`EXPIRED`（`model/kyc.go:131–136`）是為證件圖片設的，公司文件仍可能用到，計畫沒提要不要縮。

**U9 `individual` oneof 分支。** `individual.id_card_no` 是 `individual.` 底下唯一的欄位（`KYCFieldNames`）；刪掉後 proto 的 `individual` 子訊息是空的。留空訊息、刪 oneof、或改成單純的 `seller_type`，計畫沒寫（proto 由別的 agent 盤點）；backend 端連動 `FieldAppliesTo`（`model/kyc.go:683–691`）、`rpc/fieldmask.go:57` 的巢狀路徑支援與 `updatemaskfields_test.go`。

**U10 允許清單沒涵蓋、但改不掉或該不該改的 payuni 字樣。** 驗收是 `git grep -n -i payuni -- backend/internal backend/cmd api/hawkr` 逐筆屬於允許清單（258 行），清單見 257 行。清單外的：(1) 已套用的 migration 15 檔與 `atlas.sum`（不得修改），以及新的 drop migration 本身；(2) 註解引用 ADR 檔名 `2026-10-19-seller-kyc-payuni-consolidated`：23 檔 35 處（`git grep -c 'seller-kyc-payuni-consolidated' -- internal cmd ':!**/gen/**'`），含 `schema.sql`；(3) 測試資料：`cmd/hawkr/payment_config_test.go:25`、`internal/httpapi/entry_test.go:285`、`internal/httpapi/rpc/web_origin_test.go:37`；(4) `internal/modules/discovery/CARRYOVER.md`（2 筆；清單只列 buyer 與 content）；(5) `S/CARRYOVER.md` 的 74 筆（沿用盤點表是歷史紀錄）；(6) `codemeta/code_meta.go` 71 筆（見 U13）；(7) `onboarding.go:57` 這類說明 store_code 由來的註解。選項：擴充允許清單、改 ADR 檔名、或逐筆改寫。

**U11 管理員退回的 `merchant.verification.rejected`。** D18「停發 `merchant.verification.*`」（297 行）；`sellerrelay` 的 `kycRejected` 也用同一個 routing key（`relay.go:106,193,388`，`cmd/hawkr/seller_events.go:40`），它是平台審核退回，不是 PayUni 事件。照字面是一起停；一起停之後 `seller.KYCRejected` 在 legacy 這邊沒有訂閱者。計畫的兩處寫法（255 行「`sellerrelay` 的 `merchant.verification.*`」與 D18）都沒單獨點名它。

**U12 `GetSellerShippingProfile` 的 `MerId`。** 計畫只寫 `sellerserver` 的 `payuni_mer_id` 回空值（30、255 行，對應 `server.go:187`）。`server.go:437` 另有物流用的 `MerId`（PayUni 物流以商店代號下單，B3）。`model.FormatMerID` 隨 `model/payuni.go` 刪除時，這一處要決定：也回空值，或把格式化函式留在 `sellerserver`。

**U13 CodeMeta。** 計畫寫「CodeMeta 照本機脈絡檔規則」（255 行）；我讀得到的規則只有 `backend/AGENTS.md:68`（凍結、不手改生成檔）與根 `AGENTS.md`。`code_meta.go` 是生成檔，`PayuniMerchantVerificationStatus`（:545–590）屬開通，但單體沒有任何 Go 檔 import `codemeta`。選項：不動（當成 D 類／凍結，進允許清單）；或改 `docs/代碼管理（CodeMeta）/*.md` 後重新生成（會動到 Java、client 的生成物，超出本計畫範圍）。

**U14 新 CHECK 與既有列（migration 的資料步驟）。** 縮值域的 CHECK（`agreement_acceptances_kind_check`、`kyc_reviews_action_check`、`kyc_identity_hashes_field_check`、`kyc_documents_slot_check`、`kyc_document_accesses_slot_check`、`kyc_review_issues_field_check`）遇到既有列會失敗。計畫的前提是未上線、不受資料搬移約束（22 行），D11 寫「不轉換舊紀錄」（285 行），但沒說是刪列還是 CHECK 留舊值。另外，migration 直接刪 `kyc_documents` 的身分證與存摺列時，對應的 media 資產不會被記成退場（正常路徑是 `mediaref` 加同步 job，`documents.go:254`、`purge.go:258`）——本機種子資料以外是否需要一次性的資產清理，計畫沒寫。

**U15 種子資料。** `cmd/hawkr/dev_seed.go:30–47` 寫入身分證號 `A123456789`、銀行帳號與 PayUni 核准的註冊；`devseed.go:111–121` 清 `payuni_*` 與兩張 hash 表。刪除後種子店家要保留哪些 KYC 欄位取決於 U2；`account_identity_hashes` 的舊種類列在重跑種子時會被清掉（`devseed.go:120`），與「保留 5 年」不衝突（種子不是正式資料），但值得知道。

**U16 `LockPayUniRegistration` 帶來的鎖順序。** `SaveDraft`、`Submit`、`Cancel` 目前的順序是「店家、申請、註冊」（`kyc.go:170–172,413,500–501`）。註冊列消失後這一層鎖沒了；取消時「身分已鎖定不能取消」（:504–510）也沒了。D17 刪鎖已決，這裡只是提醒 `platform/lockorder` 的檢查表可能有 `payuni_registrations`（未核對）。

---

## 4. backend 內的刪除順序（每一步都能編譯）

proto 的刪除會改生成的 service 介面，所以「proto＋三端生成＋httpapi handler＋`adminauth` 宣告表＋`adminperm`」必須同一個 commit；以下假設 proto 由對應的步驟一起帶。

1. **搬走非 PayUni 的共用件**（先做，不刪東西）：`MissingStoreFields`、`NewlyMissing`、`StoreFieldNames`／`StoreField*` 從 `seller/internal/payuni`、`model/payuni.go` 搬到 model 的非 PayUni 檔（去留依 U5）。
2. **切斷 seller 內部對 `internal/payuni` 的呼叫**：`kyc/admin.go:469`、`kyc/kyc.go`（CheckResubmit、IdentityLocked、SellerViewOf、LockPayUniRegistration）、`profile/profile.go:379–383,485–487`、`closing/closing.go:381`、`devseed`、`stint/flow.go`、`exposure.KYCFlows`、`profile/admin.go`；同一步處理 `model.KYCApplication.{Flow,PayUni,ResubmitReasons}` 與 httpapi 的 `toFlow`／`toSellerFlow`／`toProblem`（需要 U6 的決定與 proto）。
3. **對外入口**：admin 4 支 PayUni RPC、`REBIND_PAYUNI`、`PAYUNI_BIND_RETRY`（reviewqueue 的 model、httpapi、`adminperm`、`adminauth`），proto 與生成物同 commit。
4. **legacy 與組裝**：`sellerrelay` 的 5 個 PayUni 事件與 `payuni_notices.go`、`sellerconsumer` 整包、`cmd/hawkr/{seller_events.go,main.go}` 的訂閱與 consumer、`sellerserver` 的 MerID；然後 `S/events.go` 的 6 個 alias。
5. **刪本體**：`S/internal/payuni/`、`S/internal/kyc/payuni.go`、`S/internal/model/payuni*.go`、`S/payuni.go`、`S/seller.go` 的 Deps 與 worker／週期 job 註冊、`cmd/hawkr/seller.go` 的 `readPayUni` 與 env、`internal/platform/payuni/`、`internal/arch/modules.go`。
6. **PayUni 的 SQL**：刪 `queries/payuni.sql`、`purge.sql` 的兩支、`stores.sql` 的篩選與 `purge.go:274,281` → 新 migration（先 drop `review_payuni_bind_retry` 與 `pub_review_items` 的成員，再 drop 兩張表，改 `agreement_acceptances_*`、`kyc_reviews_*` 的 CHECK）→ `schema.sql`／`objects.sql`／`pub.sql` → `make sqlc`。sqlc 以 `schema.sql` 驗查詢，所以查詢與 schema 同 commit。
7. **`PAYUNI_TERMS`**：`model/agreement.go`、website manifest 重生的 `agreement_gen.go`、CHECK，同一個 commit（計畫 141 行的理由）。
8. **PII 管線**（由外往內）：proto 欄位與 httpapi `kycDraftInput`／`toKYC` → `model.KYCDraftInput`／`KYCApplication`／`KYCSensitive` → `kyc/{draft,convert,diff,hashes,identity,protect,documents,admin}.go` → `kyc.sql`／`purge.sql` 的欄位與 `ReferencedKeyIDs`、`KYCApplicationsMissingHashKey` → 新 migration drop 欄位與 CHECK → `make sqlc` → `cmd/hawkr/seller.go` 的加密金鑰（U4）→ 種子。
9. **文案與生成物**：`uitext/catalog/*.json`、`S/internal/text/*.json` → `make uitext`；`AGENTS.md`／`CARRYOVER.md`；最後跑允許清單的 grep。

步驟 2 與 8 各自需要 §3 的決定才能開工（U1、U2、U5、U6、U7）。

---

## 5. 計數與命令

| 項目 | 數字 | 命令 |
|---|---|---|
| 提到 payuni 的檔（非 `gen/`） | 156 | `git grep -l -i payuni -- backend/internal backend/cmd ':!**/gen/**' \| wc -l` |
| 其中 `gen/` 底下另有 | 10（7 個 `httpapi/gen` 會重生，3 個 `legacy/gen` 保留） | `git grep -l -i payuni -- backend/internal backend/cmd \| grep '/gen/'` |
| payuni ∪ PII 模式命中的檔（非 `gen/`、非 migration） | 160 | 見下方 (A) |
| 其中測試檔 | 63 | 見下方 (B) |
| 已套用 migration 提到 payuni | 15（另有 `atlas.sum`） | `git grep -l -i payuni -- backend/internal/modules/seller/db/migrations \| wc -l` |
| 引用 ADR 檔名的檔／處 | 23／35 | `git grep -c 'seller-kyc-payuni-consolidated' -- internal cmd ':!**/gen/**' \| wc -l`；`git grep -h -o …  \| wc -l` |

處置的粗估（依 §1 的分類手算，不是命令直接輸出）：

- **delete：29 檔**（其中測試 10、生成物 1）。命令：`git ls-files backend/internal/platform/payuni backend/internal/modules/seller/internal/payuni 'backend/internal/modules/seller/internal/model/payuni*' 'backend/internal/modules/seller/payuni*' backend/internal/modules/seller/internal/kyc/payuni.go backend/internal/modules/seller/db/queries/payuni.sql backend/internal/modules/seller/internal/store/payuni.sql.go backend/cmd/hawkr/payuni_config_test.go backend/internal/legacy/sellerconsumer backend/internal/legacy/sellerrelay/payuni_notices.go backend/internal/legacy/sellerrelay/notices_test.go | wc -l`。`kyc_resubmit.go`、`stint/flow.go` 依 U6 可能再加 2。
- **edit：約 120 檔**＝ 160 − 29 − keep 約 11；其中測試約 53、生成物約 8（`store/{kyc,purge,stores}.sql.go`、`models.go`、`text_gen.go`、`catalog_gen.go`、`golden_calls_gen_test.go`、`golden.gen.json`）、文件 5。
- **keep：約 11 檔在 160 之內**（`codemeta/code_meta.go`、3 個 `CARRYOVER.md`、3 個測試資料檔依 U10、`platform/stripe` 與 `payment/internal/account` 的檔未計入 160 的不算），另加 15 個 migration、`atlas.sum`、3 個 `legacy/gen`。
- **新增：至少 1 個 migration**（PayUni 與 PII 可以各一個，配合 §4 的步驟 6、8）。

(A) `(git grep -l -i payuni -- backend/internal backend/cmd ':!**/gen/**' ':!backend/internal/modules/seller/db/migrations/**'; git grep -l -E 'id_card|IDCard|IdCard|rep_id_card|bank_account_enc|BankAccountEnc|BankAccountMasked|bank_code|BankCode|bank_book|BankBook|ID_CARD_NO|REP_ID_CARD_NO|kyc_identity_hashes|CheckKYCKeys|RevealKYC|KYCReveal|kyc_reveals|DocumentAccess|CheckNameUnlocked|IdentityLocked|KYCFlowStatus|merchant\.verification' -- backend/internal backend/cmd ':!**/gen/**' ':!backend/internal/modules/seller/db/migrations/**') | sort -u | wc -l`

(B) 同 (A) 的兩個模式（第二個少了 `bank_account_enc|BankAccountEnc|kyc_reveals|CheckNameUnlocked`），pathspec 換成 `'backend/internal/**_test.go' 'backend/cmd/**_test.go'`。

---

## 6. 沒涵蓋的

- 沒讀本體、只看簽名或命中行：`internal/platform/payuni/*`、`S/internal/payuni/*`、`S/payuni.go`、`internal/httpapi/seller/{kyc.go,admin_kyc.go}` 的欄位對應、`sellerrelay/relay.go` 的 worker 本體、全部測試檔、`S/CARRYOVER.md`、各 `AGENTS.md`、uitext 目錄的文字值、`kyc.sql:1–141`。
- 沒核對：`platform/lockorder` 的表（U16）、`internal/arch/modules.go` 的 `platform/payuni` 那一筆、`stores` 表聯絡欄位的 CHECK、`StoreReasonKYCNotApproved` 是否只有 rebind 在用、`backend-db-proto-checks` 與 migration inventory 對新 migration 的要求。
- 不在範圍：`api/hawkr` 的 proto、website、admin、兩支 App、`packages/*`、`backend/docs/**`、`backend/services/**`（Java 與 Go 舊服務）、`.env` 範例與部署設定。
- 沒有執行任何 build、測試、sqlc 或 atlas；「會壞」都是由呼叫點推得，未經編譯器確認。
