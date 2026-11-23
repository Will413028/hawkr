# B1 integration 測試對照表（test-map）

本檔是 B1-1 的產出（計畫 `../2026-11-21-b1-media.md` 的 B1-1；盤點 `interface-inventory.md` §4 的骨架）。內容三部分：

1. 179 個 Java 測試逐列的去向：「Go 對應」寫計畫中的 Go 測試或行為與負責步驟（B1-2 核心、B1-3 Connect 上傳 handler、B1-5 擁有者切換），「退場理由」寫為什麼這個測試隨著機制消失。每列兩欄恰好填一欄，另一欄寫 `—`。
2. 機敏上傳與機敏預覽的行為規格（B1-D12）：CI 的 Java 沒有物件儲存設定，這兩個端點對 Java 測不到，規格以 Java 原始碼與既有單元測試為準，逐條附 `file:line`，B1-3 的 Connect 情境直接依它寫。
3. `secure_download` 的承接（端點刪除，但它的行為由擁有者代理端點接手）。

**證據等級**：179 的數字與每列的 `file:line` 是【量】，由 `check-test-map.py` 掃 Java 測試原始碼（`@Test`）得出，並與本表逐列比對；「Go 對應」與「退場理由」是【推】：依 Java 測試方法名與被測原始碼判斷，沒有逐一讀測試本體，B1-2 與 B1-3 實作時若發現測試的真實斷言與這裡不符，直接改本表（手動跑檢查腳本，會在列數、行號或方法名漂移時失敗）。

**規則**：Java 路徑相對 `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/`；Java 原始碼的 `file:line` 相對 `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/`。測試名稱前綴是計畫中的 Go 測試名，落地時可以改名，但行為要在；改名時一併更新本表。

**檢查**：`python3 backend/docs/plans/2026-11-21-b1-media/check-test-map.py`（任何空格、缺列、多列、行號或方法名對不上、兩欄都填或都沒填，回非零）。這是手動檢查，在各步驟驗收時跑；沒有任何 CI 會自動跑它，B1-8 決定它要不要移到 `scripts/` 並接進 CI。

**步驟代號**：B1-2 = Go media 核心（schema、儲存、加密、生命週期、GC）；B1-3 = Connect 契約與上傳 handler；B1-5 = 擁有者模組切換與過渡機制刪除。

## 一、對照表

| # | 檔:行 | 測試方法 | Go 對應 | 退場理由 |
|---|---|---|---|---|
| 1 | `controller/SecureMediaControllerTest.java:132` | `secureUpload_usesUniquePerAttemptObjectKey_notSessionCreationKey` | B1-3：`TestSecureUpload_EachAttemptWritesOwnObjectKey`（每次嘗試寫獨立暫存 key，不重用建立會話時的 key） | — |
| 2 | `controller/SecureMediaControllerTest.java:163` | `secureUpload_whenConcurrentAttemptAlreadyWon_deletesOwnUploadAndDoesNotOverwriteWinner` | B1-3：`TestSecureUpload_ConcurrentLoserDeletesOwnObject`（落敗者刪自己的物件、不覆寫贏家） | — |
| 3 | `controller/SecureMediaControllerTest.java:190` | `secureUpload_whenR2UploadTimesOut_keepsSessionRetryableAndDoesNotIssueStagedToken` | B1-3：`TestSecureUpload_StorageTimeoutKeepsStagingRetryable`（儲存逾時：暫存列維持可重試、不發 token、不刪既有物件） | — |
| 4 | `controller/SecureMediaControllerTest.java:245` | `securePreview_setsContentLengthFromActualR2ObjectSize_notDeclaredSessionFileSize` | B1-3：`TestStagingPreview_ContentLengthFromStoredObject`（Content-Length 取儲存端實際大小減密文開銷，不取宣告值） | — |
| 5 | `controller/SecureMediaControllerTest.java:291` | `securePreview_usesInlineDisposition_andRfc5987EncodesCjkFilename` | B1-3：`TestStagingPreview_InlineDispositionRFC5987`（inline，中文檔名走 RFC 5987） | — |
| 6 | `controller/SecureMediaControllerTest.java:326` | `secureDownload_usesAttachmentDisposition_andRfc5987EncodesCjkFilename` | — | `secure_download` 端點刪除（盤點 §1.2 #3、B1-D6）；Disposition 與 Content-Length 行為由擁有者代理端點的測試承接（B1-5，seller 的 KYC 與治理佐證讀取） |
| 7 | `controller/SecureMediaControllerTest.java:380` | `secureDownload_filenameFromWritePathCannotBreakQuotedString` | — | `secure_download` 端點刪除；檔名不得截斷 quoted-string 的規則由 `AssetFilename` 的搬移測試（列 148–169）與擁有者代理端點測試（B1-5）承接 |
| 8 | `controller/SecureMediaControllerTest.java:416` | `secureDownload_setsContentLengthFromActualR2ObjectSize_notStaleSizeBytes` | — | `secure_download` 端點刪除；Content-Length 取實際大小的規則同列 4，由擁有者代理端點測試承接（B1-5） |
| 9 | `grpc/ServerReadSecureAssetTest.java:185` | `returnsBytesWhenTripleMatches` | B1-2：`TestReadSecure_ReturnsPlaintextForAssetAcrossSegments`（以資產 ID 讀，行程內呼叫；授權留在擁有者） | — |
| 10 | `grpc/ServerReadSecureAssetTest.java:200` | `filenamePrefersOriginalOverObjectKeyTail` | B1-2：`TestAssetDisplayName_OriginalFilenameWins`（顯示檔名優先取原始檔名） | — |
| 11 | `grpc/ServerReadSecureAssetTest.java:220` | `filenameFallbackStripsAttemptSuffixForLegacyRows` | — | 回退到 object key 尾段並剝 `.attempt-` 後綴只服務舊列；正式 key 不含 `.attempt-`（缺陷 5.15）也不含呼叫端檔名（5.16），上線前沒有舊列 |
| 12 | `grpc/ServerReadSecureAssetTest.java:243` | `decryptsEncryptedObject` | B1-2：`TestRoundTrip_SegmentBoundaries`（B1-D4 的分塊 AEAD 往返；整合面的往返在 `TestReadSecure_ReturnsPlaintextForAssetAcrossSegments`） | — |
| 13 | `grpc/ServerReadSecureAssetTest.java:264` | `mismatchedRefIdIsIndistinguishableFromMissing` | — | 三元組（ref_type、ref_id、asset_id）隨 `asset_reference` 消失；「非擁有者不可讀」由擁有者授權測試守（B1-5） |
| 14 | `grpc/ServerReadSecureAssetTest.java:283` | `mismatchedRefTypeIsIndistinguishableFromMissing` | — | 同列 13：ref_type 隨 `asset_reference` 消失 |
| 15 | `grpc/ServerReadSecureAssetTest.java:295` | `rejectsNonInternalCallers` | — | gRPC 內部閘門（`x-internal-service`）隨 gRPC 介面消失，Go 的 `ReadSecure` 是行程內呼叫，沒有外部呼叫者 |
| 16 | `grpc/ServerReadSecureAssetTest.java:308` | `rejectsEmptyTripleFields` | — | 三元組欄位隨 `asset_reference` 消失；`ReadSecure` 只收資產 ID，空 ID 由 `TestReadSecure_UnknownAssetNotFound`（列 9 同檔）涵蓋 |
| 17 | `grpc/ServerReadSecureAssetTest.java:322` | `rejectsNonSecureAsset` | B1-2：`TestReadSecure_RejectsPublicAsset`（公開資產不可走機敏讀取） | — |
| 18 | `grpc/ServerReadSecureAssetTest.java:334` | `rejectsTruncatedRead` | B1-2：`TestReadSecure_TamperedTruncatedOrTaglessCiphertextFailsBeforeAnyPlaintext`（B1-D4：竄改或截斷在送出明文前失敗） | — |
| 19 | `grpc/ServerReadSecureAssetTest.java:350` | `referenceStubOnlyAcceptsExactTriple` | — | 三元組 stub 的測試替身隨 `asset_reference` 消失 |
| 20 | `repository/MediaAssetGcExclusionTest.java:31` | `selectOrphanedAssetsExcludesStatutoryRefTypes` | — | 孤兒資產 GC 與 `ref_type` 法定排除清單（7120、7111）一起消失：GC 不再刪已確認資產；「發票與證件不呼叫刪除」由擁有者側測試守（B1-5） |
| 21 | `repository/MediaAssetGcExclusionTest.java:42` | `claimAssetForGcExcludesStatutoryRefTypes` | — | 同列 20：`claimAssetForGc` 隨孤兒資產 GC 刪除 |
| 22 | `repository/MediaAssetGcExclusionTest.java:54` | `exclusionCoversStatutoryTypes` | — | 同列 20：法定 `ref_type` 清單隨 `ref_type` 消失 |
| 23 | `scheduler/MediaGarbageCollectionJobClaimTest.java:98` | `sessionClaimLost_deletesNothing` | B1-2：`TestStagingGC_ClaimLostDeletesNothing`（暫存 GC 搶不到租約就不刪物件） | — |
| 24 | `scheduler/MediaGarbageCollectionJobClaimTest.java:114` | `assetClaimLost_deletesNothing` | — | 孤兒資產 GC 的 asset claim 隨孤兒資產 GC 刪除；實體刪除 job 的租約見列 30 |
| 25 | `scheduler/MediaGarbageCollectionJobClaimTest.java:130` | `claimWon_claimsBeforeDeleting` | B1-2：`TestStagingGC_ClaimsBeforeDeleting`（先標記並提交，再刪物件） | — |
| 26 | `scheduler/MediaGarbageCollectionJobClaimTest.java:156` | `perRowMarking_onlyMarksSucceeded` | B1-2：`TestStagingGC_MarksOnlySucceededRows`（逐列標記，只標刪除成功的） | — |
| 27 | `scheduler/MediaGarbageCollectionJobClaimTest.java:181` | `leaseLengthIsBoundedByGcPeriod` | B1-2：`TestGC_LeaseBoundedByPeriod`（租約長度不超過 GC 週期） | — |
| 28 | `scheduler/MediaGarbageCollectionJobClaimTest.java:209` | `leaseBaselineIsPerItem` | B1-2：`TestGC_LeaseBaselinePerItem`（租約起算點逐項計算） | — |
| 29 | `scheduler/MediaGarbageCollectionJobClaimTest.java:248` | `sessionClaimCommitsBeforeSideEffect` | B1-2：`TestStagingGC_ClaimCommitsBeforeStorageCall`（fake 儲存斷言呼叫時沒有開著的 transaction，單體規則 5） | — |
| 30 | `scheduler/MediaGarbageCollectionJobClaimTest.java:288` | `assetClaimCommitsBeforeSideEffect` | B1-2：`TestDeleteJob_ClaimCommitsBeforeStorageCall`（刪除中標記先提交，transaction 外才刪物件） | — |
| 31 | `scheduler/MediaGarbageCollectionJobClaimTest.java:339` | `assetClaimRestatesNoReferencePredicate` | — | 「無引用」述詞隨 `asset_reference` 消失；實體刪除 job 只認 `MarkDeleted` 標記 |
| 32 | `scheduler/MediaGarbageCollectionJobTest.java:86` | `shouldCleanExpiredInitSessions` | B1-2：`TestStagingGC_CleansExpiredInitRows` | — |
| 33 | `scheduler/MediaGarbageCollectionJobTest.java:112` | `shouldCleanExpiredCompletedSessions` | B1-2：`TestStagingGC_CleansExpiredCompletedRows` | — |
| 34 | `scheduler/MediaGarbageCollectionJobTest.java:131` | `shouldNotCleanNonExpiredSessions` | B1-2：`TestStagingGC_KeepsUnexpiredRows` | — |
| 35 | `scheduler/MediaGarbageCollectionJobTest.java:148` | `shouldCleanOrphanedAssets` | — | 孤兒資產 GC 刪除：資產要嘛已確認（擁有者持有，只有 `MarkDeleted` 才刪），要嘛仍是暫存（走過期暫存 GC） |
| 36 | `scheduler/MediaGarbageCollectionJobTest.java:167` | `shouldNotCleanAssetWithReferences` | B1-2：`TestGC_NeverDeletesConfirmedAssets`（GC 不刪已確認資產） | — |
| 37 | `scheduler/MediaGarbageCollectionJobTest.java:179` | `shouldNotCleanRecentOrphanedAssets` | — | 孤兒資產保留天數隨孤兒資產 GC 刪除 |
| 38 | `scheduler/MediaGarbageCollectionJobTest.java:195` | `shouldKeepDbRowWhenStorageDeleteFails` | B1-2：`TestDeleteJob_StorageFailureKeepsRowRetryable`（缺陷 5.17：刪物件失敗後列仍可重試） | — |
| 39 | `scheduler/MediaGarbageCollectionJobTest.java:223` | `shouldBreakBatchLoopOnPartialFailure` | B1-2：`TestDeleteJob_FailureDoesNotBlockOtherRows`（單列失敗不影響已完成的列，下次重試） | — |
| 40 | `scheduler/MediaGarbageCollectionJobTest.java:247` | `shouldSkipWhenDisabled` | B1-2：`TestGC_DisabledSwitchSkipsRun`（停用開關） | — |
| 41 | `scheduler/MediaGarbageCollectionJobTest.java:264` | `cleanMethodsAreWrappedInTransactionTemplate` | B1-2：`TestGC_StorageCalledOutsideTx`（transaction 邊界） | — |
| 42 | `scheduler/MediaGarbageCollectionJobTest.java:283` | `midFlightThrowTriggersRollback` | B1-2：`TestGC_MidFlightErrorRollsBackMarkOnly`（中途失敗只回滾標記 transaction） | — |
| 43 | `service/AssetReferenceServiceTest.java:55` | `shouldLinkSuccessfully` | — | `Link` 隨 `asset_reference` 消失；成功確認的同等行為見 `Confirm`（列 63） |
| 44 | `service/AssetReferenceServiceTest.java:77` | `shouldBeIdempotent` | B1-2：`TestConfirm_ReentryInTheSameTransactionReturnsSameAsset、TestConfirm_ConfirmedInAnotherTransactionIsRejected`（B1-D2 補充：重入回暫存列上已記的同一資產） | — |
| 45 | `service/AssetReferenceServiceTest.java:98` | `shouldTolerateDuplicateAssetIds` | B1-2：`TestMarkDeleted_DuplicateIDsTolerated`（重複的資產 ID 不報錯） | — |
| 46 | `service/AssetReferenceServiceTest.java:120` | `shouldThrowWhenRefTypeEmpty` | — | `ref_type` 隨 `asset_reference` 消失 |
| 47 | `service/AssetReferenceServiceTest.java:129` | `shouldThrowWhenRefIdEmpty` | — | `ref_id` 隨 `asset_reference` 消失（引用只存在擁有者的資料表） |
| 48 | `service/AssetReferenceServiceTest.java:138` | `shouldThrowWhenAssetIdsEmpty` | — | `Link` 的資產清單參數隨 `Link` 消失；`MarkDeleted` 空清單視為 no-op（列 45、51） |
| 49 | `service/AssetReferenceServiceTest.java:147` | `shouldThrowWhenAssetNotFound` | B1-2：`TestConfirm_UnknownTokenNotFound`（找不到暫存列：未知 token） | — |
| 50 | `service/AssetReferenceServiceTest.java:165` | `shouldUnlinkSuccessfully` | — | `Unlink` 隨 `asset_reference` 消失；刪除語意見 `MarkDeleted`（列 45、51） |
| 51 | `service/AssetReferenceServiceTest.java:180` | `shouldBeIdempotentOnNonExistent` | B1-2：`TestMarkDeleted_NonExistentIsNoop`（刪除指令冪等） | — |
| 52 | `service/AssetReferenceServiceTest.java:195` | `shouldThrowWhenRefTypeEmpty` | — | `ref_type` 隨 `asset_reference` 消失 |
| 53 | `service/AssetReferenceServiceTest.java:209` | `shouldReleaseSuccessfully` | — | `Release`（依 ref 全放）隨 `asset_reference` 消失；擁有者自己列出要刪的資產 ID 再呼叫 `MarkDeleted`（B1-5） |
| 54 | `service/AssetReferenceServiceTest.java:226` | `shouldPassKeepAssetIdsDownToRepository` | — | keep 清單隨 `Release` 消失 |
| 55 | `service/AssetReferenceServiceTest.java:244` | `shouldTreatNullKeepListAsReleaseAll` | — | null keep 清單語意隨 `Release` 消失 |
| 56 | `service/AssetReferenceServiceTest.java:256` | `shouldThrowWhenKeepAssetIdMalformed` | B1-2：`TestMarkDeleted_RejectsMalformedAssetID`（B1-D8 的 ID 格式） | — |
| 57 | `service/AssetReferenceServiceTest.java:264` | `shouldThrowWhenRefIdsEmpty` | — | `ref_ids` 隨 `asset_reference` 消失 |
| 58 | `service/AssetReferenceServiceTest.java:272` | `shouldThrowWhenRefTypeInvalid` | — | `ref_type` 的合法值檢查隨 `ref_type` 消失（B1-D3） |
| 59 | `service/BatchGetAssetMetadataTest.java:71` | `returnsOnlyAuthorizedAssetsAndCannotDistinguishDenialFromMissing` | — | 授權過濾與「查無與越權不可分辨」隨 gRPC `BatchGetAssetMetadata` 消失；授權在擁有者（B1-5 的擁有者端點測試） |
| 60 | `service/BatchGetAssetMetadataTest.java:98` | `originalFilenameTakesPrecedenceOverObjectKeyTail` | B1-2：`TestAssetMetadata_OriginalFilenameWins` | — |
| 61 | `service/BatchGetAssetMetadataTest.java:117` | `fallsBackToObjectKeyTailForLegacyRows` | B1-2：`TestAssetMetadata_EmptyFilenameWhenNoneRecorded`（正式 key 不含檔名，缺陷 5.16，沒有尾段可回退） | — |
| 62 | `service/BatchGetAssetMetadataTest.java:131` | `emptyInputIsANoop` | B1-2：`TestAssetMetadata_EmptyInputIsNoop` | — |
| 63 | `service/MediaAssetServiceTest.java:146` | `shouldFinalizeSuccessfully` | B1-2：`TestConfirm_Success`（擁有者 transaction 內確認，資產轉為已確認） | — |
| 64 | `service/MediaAssetServiceTest.java:192` | `copiesOriginalFilenameFromSession` | B1-2：`TestConfirm_CopiesOriginalFilenameFromStaging` | — |
| 65 | `service/MediaAssetServiceTest.java:214` | `leavesOriginalFilenameNullWhenSessionHasNone` | B1-2：`TestConfirm_LeavesFilenameNullWhenStagingHasNone` | — |
| 66 | `service/MediaAssetServiceTest.java:236` | `shouldReturnExistingAssetWhenAlreadyFinalized` | B1-2：`TestConfirm_ReentryInTheSameTransactionReturnsSameAsset、TestConfirm_ConfirmedInAnotherTransactionIsRejected`（同列 44） | — |
| 67 | `service/MediaAssetServiceTest.java:278` | `shouldThrowPermissionDeniedWhenOwnerMismatch` | B1-2：`TestConfirm_RejectsNonUploader`（mutation：`Confirm` 不比對上傳者 → 此測試紅） | — |
| 68 | `service/MediaAssetServiceTest.java:295` | `shouldThrowInvalidArgumentWhenSessionNotFound` | B1-2：`TestConfirm_UnknownStagingNotFound` | — |
| 69 | `service/MediaAssetServiceTest.java:312` | `shouldThrowInvalidArgumentWhenSessionExpired` | B1-2：`TestConfirm_RejectsExpiredStaging` | — |
| 70 | `service/MediaAssetServiceTest.java:338` | `shouldThrowInvalidArgumentWhenSessionStatusIsInit` | B1-2：`TestConfirm_RejectsStagingNotYetUploaded`（暫存列尚未完成上傳） | — |
| 71 | `service/MediaAssetServiceTest.java:357` | `shouldRejectWhenClaimsScopeMismatchSession` | B1-2：`TestConfirm_RejectsTokenStagingMismatch`（B1-D2：token 與暫存列一對一） | — |
| 72 | `service/MediaAssetServiceTest.java:379` | `shouldRejectWhenSessionAccountDiffersFromClaims` | B1-2：`TestConfirm_RejectsTokenAccountMismatch` | — |
| 73 | `service/MediaAssetServiceTest.java:395` | `shouldRejectAndCleanupWhenHeadCheckFails` | B1-2：`TestConfirm_RejectsWhenStorageStatFails`（HEAD 回查失敗：拒絕並清理） | — |
| 74 | `service/MediaAssetServiceTest.java:417` | `shouldFinalizeWhenHeadCheckPasses` | B1-2：`TestConfirm_PassesWhenStorageStatMatches` | — |
| 75 | `service/MediaAssetServiceTest.java:450` | `shouldDeleteTmpOnlyAfterDbCommit` | B1-2：`TestConfirm_DeletesStagingObjectOnlyAfterCommit`（暫存物件在 DB 提交後才刪） | — |
| 76 | `service/MediaAssetServiceTest.java:470` | `shouldNotInsertWhenLosingStatusCas` | B1-2：`TestConfirm_ConcurrentLoserDoesNotInsert`（狀態 CAS 落敗不寫資產列） | — |
| 77 | `service/MediaAssetServiceTest.java:499` | `shouldCompensateAlreadyCopiedObjectsWhenLaterTokenFails` | B1-2：`TestConfirm_FailureLeavesNoUnownedFinalObject`（後續失敗時補償已複製的物件） | — |
| 78 | `service/MediaAssetServiceTest.java:547` | `shouldSkipCompensationWhenAnotherAssetOwnsTheObject` | B1-2：`TestConfirm_CompensationSkipsObjectOwnedByOtherAsset` | — |
| 79 | `service/MediaAssetServiceTest.java:569` | `shouldDeleteUploadedObjectWhenInsertFails` | B1-2：`TestPutPublic_InsertFailureDeletesObject`（後端直傳寫列失敗時刪物件；`PutPublic` 接線延後，B1-8） | — |
| 80 | `service/MediaAssetServiceTest.java:588` | `persistsSanitizedFilenameButKeepsRawInObjectKey` | B1-2：`TestPutPublic_PersistsSanitizedFilenameKeyHasNoFilename`（缺陷 5.16：key 不含呼叫端檔名） | — |
| 81 | `service/MediaAssetServiceTest.java:629` | `uploadsToSecurePrefixAndLinksReference` | B1-2：`TestPutSecure_WritesSecurePrefixWithoutReference`（寫機敏前綴；不建引用） | — |
| 82 | `service/MediaAssetServiceTest.java:655` | `compensatesWhenPersistFails` | B1-2：`TestPutSecure_CompensatesWhenPersistFails` | — |
| 83 | `service/MediaAssetServiceTest.java:672` | `persistsSanitizedFilenameButKeepsRawInObjectKey` | B1-2：`TestPutSecure_PersistsSanitizedFilenameKeyHasNoFilename` | — |
| 84 | `service/MediaUploadValidatorTest.java:30` | `declaredValidImage` | B1-3：`TestCreateUpload_AcceptsAllowedImage` | — |
| 85 | `service/MediaUploadValidatorTest.java:37` | `declaredRejectsUnknownMime` | B1-3：`TestCreateUpload_RejectsUnknownMime` | — |
| 86 | `service/MediaUploadValidatorTest.java:44` | `declaredPdfOnlyForSecure` | B1-3：`TestCreateUpload_PdfOnlyForSecure` | — |
| 87 | `service/MediaUploadValidatorTest.java:52` | `declaredRejectsOversize` | B1-3：`TestCreateUpload_RejectsOversizeDeclared`（mutation：拿掉大小上限檢查 → 「超量上傳被拒」紅） | — |
| 88 | `service/MediaUploadValidatorTest.java:59` | `declaredRejectsNonPositiveSize` | B1-3：`TestCreateUpload_RejectsNonPositiveSize` | — |
| 89 | `service/MediaUploadValidatorTest.java:68` | `finalizedRejectsMissingObject` | B1-2：`TestConfirm_RejectsMissingObject` | — |
| 90 | `service/MediaUploadValidatorTest.java:75` | `finalizedRejectsActualOversize` | B1-2：`TestConfirm_RejectsActualOversize` | — |
| 91 | `service/MediaUploadValidatorTest.java:83` | `finalizedRejectsSizeMismatch` | B1-2：`TestConfirm_RejectsSizeMismatch` | — |
| 92 | `service/MediaUploadValidatorTest.java:91` | `finalizedRejectsActualMimeNotWhitelisted` | B1-2：`TestConfirm_RejectsActualMimeNotAllowed` | — |
| 93 | `service/MediaUploadValidatorTest.java:99` | `finalizedRejectsMimeMismatch` | B1-2：`TestConfirm_RejectsMimeMismatch` | — |
| 94 | `service/MediaUploadValidatorTest.java:107` | `finalizedPasses` | B1-2：`TestConfirm_ValidObjectPasses` | — |
| 95 | `service/MediaUploadValidatorTest.java:115` | `finalizedSecurePassesWithGcmTagOverhead` | B1-2：`TestUploadSecure_CiphertextOverheadIsOutsideTheRecordedPlainSize`（密文開銷依 B1-D4 的格式改寫：資產記明文大小，物件大小是明文加 header 與每段 tag） | — |
| 96 | `service/MediaUploadValidatorTest.java:127` | `finalizedSecureRejectsWhenMissingGcmTag` | B1-2：`TestReadSecure_TamperedTruncatedOrTaglessCiphertextFailsBeforeAnyPlaintext`（形狀改變：不再在確認時比大小，缺 tag 改成讀取時驗證失敗，與竄改、截斷同一個測試） | — |
| 97 | `service/MediaUploadValidatorTest.java:135` | `finalizedSecureSkipsContentTypeCheck` | B1-2：`TestUploadSecure_ObjectTypeIsOctetStreamWhileAssetKeepsRecognisedType`（形狀改變：不再有確認時的型別檢查，改斷言機敏物件的實際型別一律是 octet-stream、資產列記辨認出的型別） | — |
| 98 | `service/ServerUploadSecureEncryptionTest.java:112` | `uploadsCiphertextNotPlaintext` | B1-2：`TestPutSecure_StoresCiphertextNotPlaintext` | — |
| 99 | `service/ServerUploadSecureEncryptionTest.java:139` | `roundTripsThroughTheRealCipher` | B1-2：`TestPutSecure_RoundTripsThroughReadSecure` | — |
| 100 | `service/ServerUploadSecureEncryptionTest.java:169` | `legacyPlaintextAssetsStayReadable` | — | 舊明文機敏資產只存在於 Java 的歷史資料；系統未上線、開發資料不搬（計畫前提「不搬資料」），Go 沒有明文機敏資產要讀 |
| 101 | `service/ServerUploadSecureEncryptionTest.java:199` | `refusesToWritePlaintextWhenMasterKeyMissing` | B1-2：`TestUploadSecure_RefusesWhenKeyringMissing`（無金鑰環拒寫） | — |
| 102 | `service/StagedTokenServiceTest.java:41` | `missingSecretFailsClosed` | B1-2：`TestStagedToken_MissingSecretFailsClosed`（缺 secret 啟動即失敗） | — |
| 103 | `service/StagedTokenServiceTest.java:48` | `missingSecretWithAllowEphemeralUsesEphemeralKey` | — | ephemeral key 是 Java 的本機後門（`TOKEN_ALLOW_EPHEMERAL`）；Go 缺 secret 一律啟動失敗（列 102），不提供隨機金鑰的退路 |
| 104 | `service/StagedTokenServiceTest.java:65` | `explicitSecretRoundTrip` | B1-2：`TestStagedToken_RoundTrip`（格式依 B1-D2） | — |
| 105 | `service/StagedTokenServiceTest.java:81` | `forgedSignatureRejected` | B1-2：`TestStagedToken_ForgedSignatureRejected` | — |
| 106 | `service/StagedTokenServiceTest.java:101` | `tokenSignedWithOldKeyRejectedAfterRotation` | B1-2：`TestStagedToken_OldKeyRejectedAfterRotation` | — |
| 107 | `service/StagedTokenServiceTest.java:118` | `expiredTokenRejected` | B1-2：`TestStagedToken_ExpiredRejected` | — |
| 108 | `service/UploadSessionServiceTest.java:65` | `persistsSanitizedOriginalFilename` | — | 不保留原始檔名（B1-D16、缺陷 5.16）：沒有檔名欄位可持久化，也就沒有消毒後的檔名要驗；Go 的暫存列不存呼叫端的檔名 |
| 109 | `service/UploadSessionServiceTest.java:77` | `objectKeyGenerationReceivesRawFilename` | — | 「key 要吃原值、消毒須排在 key 生成之後」是 Java 的順序陷阱；Go 的 key 不含呼叫端檔名（缺陷 5.16），沒有這個順序依賴 |
| 110 | `service/UploadSessionServiceTest.java:95` | `storesNullWhenNothingSurvivesSanitization` | — | 不保留原始檔名（B1-D16）：沒有檔名消毒，也就沒有「消毒後什麼都不剩」的分支 |
| 111 | `service/UploadSessionServiceTest.java:107` | `keepsCleanCjkFilename` | — | 不保留原始檔名（B1-D16）：檔名不進任何欄位，中日韓字元是否原樣保留不成立；需要顯示名時由擁有者依文件類型產生 |
| 112 | `storage/AbstractStorageServiceTest.java:45` | `generateObjectKey_normalExtension` | B1-2：`TestObjectKey_ShapeForPublicStaging`（暫存 key 格式 `tmp/{date}/{id}`；key 不含呼叫端輸入） | — |
| 113 | `storage/AbstractStorageServiceTest.java:53` | `generateObjectKey_rejectsBackslashInExtension` | B1-2：`TestObjectKey_NeverContainsCallerInput`（含反斜線的副檔名不進 key；路徑穿越拒絕保留為「key 不含呼叫端輸入」） | — |
| 114 | `storage/AbstractStorageServiceTest.java:60` | `generateObjectKey_rejectsSlashInExtension` | B1-2：`TestObjectKey_NeverContainsCallerInput`（含斜線的副檔名，同列 113 的表格案例） | — |
| 115 | `storage/AbstractStorageServiceTest.java:67` | `finalKeyPrefixTracksCodeMetaCode` | — | 正式 key 的 category 前綴隨 `ref_type` 消失（B1-D3、`spike.md` (d)：沒有程式依前綴決定行為） |
| 116 | `storage/AbstractStorageServiceTest.java:86` | `buyerAvatarPrefixUnchanged` | — | `buyerAvatar` 等 category 前綴隨 `ref_type` 消失 |
| 117 | `storage/AbstractStorageServiceTest.java:99` | `importFiveSegmentKey_usesDateNotImportLiteral` | — | 匯入用的五段 key（`import` 暫存）隨用途模型改變消失；暫存 key 只有一種形狀 |
| 118 | `storage/AbstractStorageServiceTest.java:113` | `normalThreeSegmentKey_unchanged` | B1-2：`TestObjectKey_ThreeSegmentUnchanged` | — |
| 119 | `storage/AbstractStorageServiceTest.java:123` | `secureThreeSegmentKey_usesSecurePrefix` | B1-2：`TestObjectKey_SecureUsesSecurePrefix` | — |
| 120 | `storage/AbstractStorageServiceTest.java:133` | `noDateSegment_fallsBackToToday` | — | 沒有日期段時回退到今天，是為了轉換多種 Java key 形狀；Go 只產生一種形狀，不轉換 |
| 121 | `storage/ImageMetadataStripperTest.java:106` | `jpegIsStrippedLosslessly` | B1-2：`TestStrip_JpegLossless`（不變量第 1 條） | — |
| 122 | `storage/ImageMetadataStripperTest.java:128` | `jpegKeepsOnlyOrientation` | B1-2：`TestStrip_JpegKeepsOnlyOrientation` | — |
| 123 | `storage/ImageMetadataStripperTest.java:140` | `jpegWithDefaultOrientationHasNoExif` | B1-2：`TestStrip_JpegDefaultOrientationHasNoExif` | — |
| 124 | `storage/ImageMetadataStripperTest.java:147` | `truncatedJpegIsRejected` | B1-2：`TestStrip_TruncatedJpegRejected` | — |
| 125 | `storage/ImageMetadataStripperTest.java:170` | `pngIsStripped` | B1-2：`TestStrip_PngStripped` | — |
| 126 | `storage/ImageMetadataStripperTest.java:222` | `webpIsStripped` | B1-2：`TestStrip_WebpStripped` | — |
| 127 | `storage/ImageMetadataStripperTest.java:246` | `webpWithDefaultOrientationHasNoExif` | B1-2：`TestStrip_WebpDefaultOrientationHasNoExif` | — |
| 128 | `storage/ImageMetadataStripperTest.java:257` | `gifIsStripped` | B1-2：`TestStrip_GifStripped` | — |
| 129 | `storage/ImageMetadataStripperTest.java:282` | `unknownFormatIsNull` | B1-2：`TestStrip_UnknownFormatRejected`（mutation：拿掉剝除前的格式辨認 → 「假圖片被拒」紅） | — |
| 130 | `storage/R2StorageServiceTest.java:67` | `uploadStreamPassesObjectKey` | B1-2：`TestR2_PutObjectPassesKey` | — |
| 131 | `storage/R2StorageServiceTest.java:81` | `uploadStreamWrapsR2ApiTimeout` | B1-2：`TestR2_PutObjectWrapsTimeout` | — |
| 132 | `storage/R2StorageServiceTest.java:123` | `deletePassesObjectKey` | B1-2：`TestR2_DeletePassesKey` | — |
| 133 | `storage/R2StorageServiceTest.java:133` | `copyPassesKeys` | B1-2：`TestR2_CopyPassesKeys` | — |
| 134 | `storage/R2StorageServiceTest.java:162` | `copyToPublicStripsMetadata` | B1-2：`TestR2_PromoteToPublicStripsMetadata` | — |
| 135 | `storage/R2StorageServiceTest.java:183` | `copyToPublicRejectsCorruptImage` | B1-2：`TestR2_PromoteToPublicRejectsCorruptImage` | — |
| 136 | `storage/R2StorageServiceTest.java:199` | `copyToPublicNonImageUsesServerSideCopy` | B1-2：`TestR2_PromoteNonImageUsesServerSideCopy` | — |
| 137 | `storage/R2StorageServiceTest.java:213` | `uploadToPublicStripsMetadata` | B1-2：`TestR2_PutPublicStripsMetadata` | — |
| 138 | `storage/R2StorageServiceTest.java:226` | `uploadToTmpIsUntouched` | B1-2：`TestR2_PutStagingIsUntouched` | — |
| 139 | `storage/R2StorageServiceTest.java:247` | `statObjectReturnsStat` | B1-2：`TestR2_StatReturnsStat` | — |
| 140 | `storage/R2StorageServiceTest.java:263` | `statObjectMissingReturnsNull` | B1-2：`TestR2_StatMissingReturnsNotFound` | — |
| 141 | `storage/R2StorageServiceTest.java:271` | `presignUpload` | B1-3：`TestPresignPut_SignsContentLengthContentTypeIfNoneMatch`（B1-D5，`spike.md` (e)） | — |
| 142 | `storage/R2StorageServiceTest.java:283` | `presignDownload` | B1-2：`TestR2_PresignDownload` | — |
| 143 | `storage/R2StorageServiceTest.java:295` | `notConfiguredFallback` | — | 「R2 未配置時回模擬 URL」是 Java 的開發模式後備；Go 缺 R2 設定啟動即失敗（缺陷 5.12，`TestConfig_MissingR2FailsStartup`） |
| 144 | `storage/R2StorageServiceTest.java:307` | `copyMissingSourceThrows` | B1-2：`TestR2_CopyMissingSourceFails` | — |
| 145 | `storage/StorageBackendConfigTest.java:20` | `defaultIsR2` | — | Go 沒有 Spring 條件裝配；唯一後端是 R2，沒有「預設值」要測 |
| 146 | `storage/StorageBackendConfigTest.java:29` | `explicitR2` | — | 同列 145 |
| 147 | `storage/StorageBackendConfigTest.java:38` | `legacyOssNotWired` | B1-2：`TestConfig_RejectsUnknownStorageBackend`（非 r2 值啟動即失敗） | — |
| 148 | `util/AssetFilenameTest.java:25` | `keepsCleanNameUnchanged` | B1-2：`TestAssetFilename_KeepsCleanNameUnchanged`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 149 | `util/AssetFilenameTest.java:31` | `takesBasename` | B1-2：`TestAssetFilename_TakesBasename`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 150 | `util/AssetFilenameTest.java:42` | `stripsCrLf` | B1-2：`TestAssetFilename_StripsCrLf`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 151 | `util/AssetFilenameTest.java:51` | `stripsNul` | B1-2：`TestAssetFilename_StripsNul`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 152 | `util/AssetFilenameTest.java:61` | `truncatesKeepingExtension` | B1-2：`TestAssetFilename_TruncatesKeepingExtension`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 153 | `util/AssetFilenameTest.java:74` | `truncatesByCharactersNotDisplayWidth` | B1-2：`TestAssetFilename_TruncatesByRunesNotDisplayWidth`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 154 | `util/AssetFilenameTest.java:86` | `returnsNullWhenNothingLeft` | B1-2：`TestAssetFilename_ReturnsEmptyWhenNothingLeft`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 155 | `util/AssetFilenameTest.java:97` | `returnsNullForNullInput` | B1-2：`TestAssetFilename_ReturnsEmptyForEmptyInput`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 156 | `util/AssetFilenameTest.java:103` | `stripsDoubleQuote` | B1-2：`TestAssetFilename_StripsDoubleQuote`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 157 | `util/AssetFilenameTest.java:120` | `outputNeverContainsQuoteOrBackslash` | B1-2：`TestAssetFilename_OutputNeverContainsQuoteOrBackslash`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 158 | `util/AssetFilenameTest.java:140` | `trimsSurroundingWhitespace` | B1-2：`TestAssetFilename_TrimsSurroundingWhitespace`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 159 | `util/AssetFilenameTest.java:151` | `originalFilenameWins` | B1-2：`TestAssetFilename_DisplayName_OriginalWins`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 160 | `util/AssetFilenameTest.java:160` | `fallsBackToObjectKeyTail` | — | 回退到 object key 尾段的規則隨「正式 key 不含檔名」消失（缺陷 5.16、列 11、61）；顯示檔名只取原始檔名 |
| 161 | `util/AssetFilenameTest.java:168` | `stripsAttemptSuffixOnFallback` | — | 回退到 object key 尾段的規則隨「正式 key 不含檔名」消失（缺陷 5.16、列 11、61）；顯示檔名只取原始檔名 |
| 162 | `util/AssetFilenameTest.java:178` | `doesNotStripNonUuidAttemptSuffix` | — | 回退到 object key 尾段的規則隨「正式 key 不含檔名」消失（缺陷 5.16、列 11、61）；顯示檔名只取原始檔名 |
| 163 | `util/AssetFilenameTest.java:185` | `objectKeyWithoutSlash` | — | 回退到 object key 尾段的規則隨「正式 key 不含檔名」消失（缺陷 5.16、列 11、61）；顯示檔名只取原始檔名 |
| 164 | `util/AssetFilenameTest.java:191` | `blankOriginalIsTreatedAsAbsent` | B1-2：`TestAssetFilename_DisplayName_BlankOriginalIsAbsent`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 165 | `util/AssetFilenameTest.java:198` | `fallbackValueIsSanitized` | — | 回退到 object key 尾段的規則隨「正式 key 不含檔名」消失（缺陷 5.16、列 11、61）；顯示檔名只取原始檔名 |
| 166 | `util/AssetFilenameTest.java:218` | `originalFilenameBranchIsSanitizedToo` | B1-2：`TestAssetFilename_DisplayName_OriginalIsSanitized`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 167 | `util/AssetFilenameTest.java:228` | `returnValueNeverContainsQuoteOrBackslash` | B1-2：`TestAssetFilename_DisplayName_NeverContainsQuoteOrBackslash`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 168 | `util/AssetFilenameTest.java:247` | `returnsEmptyWhenFallbackSanitizesToNothing` | — | 回退到 object key 尾段的規則隨「正式 key 不含檔名」消失（缺陷 5.16、列 11、61）；顯示檔名只取原始檔名 |
| 169 | `util/AssetFilenameTest.java:253` | `returnsEmptyStringWhenNothingAvailable` | B1-2：`TestAssetFilename_DisplayName_EmptyWhenNothingAvailable`（`AssetFilename` 逐案搬到 Go，規則不變） | — |
| 170 | `util/AssetFilenameTest.java:267` | `stripsQueryAndFragment` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 171 | `util/AssetFilenameTest.java:280` | `doesNotDoubleDecode_percent` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 172 | `util/AssetFilenameTest.java:290` | `doesNotDoubleDecode_plus` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 173 | `util/AssetFilenameTest.java:297` | `doesNotDoubleDecode_pathTraversal` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 174 | `util/AssetFilenameTest.java:307` | `trailingSlashYieldsNull` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 175 | `util/AssetFilenameTest.java:314` | `opaqueUriYieldsNull` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 176 | `util/AssetFilenameTest.java:320` | `illegalUriYieldsNullNotThrow` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 177 | `util/AssetFilenameTest.java:329` | `nullInputYieldsNull` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 178 | `util/AssetFilenameTest.java:337` | `stripsDecodedNul` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |
| 179 | `util/AssetFilenameTest.java:344` | `takesLastPathSegment` | — | `AssetFilename.fromSourceUrl` 沒有呼叫端（`rg -l fromSourceUrl backend/services/java` 只命中定義它的 `AssetFilename.java`），匯入路徑的 URL→檔名不在 media 的範圍；函式與它的案例一起不搬 |

合計：Go 對應 122 列、退場 57 列、共 179 列【量：`check-test-map.py` 輸出】。

## 二、機敏上傳的行為規格（B1-D12）

來源：`PUT /app/media/secure_upload/{id}`，`controller/SecureMediaController.java:78-185`（gateway 以 `/api/v1/app/media/secure_upload/{id}` 轉入，`gateway/internal/router/routes_media.go`）；單元測試 `SecureMediaControllerTest`（列 1–3）。Java 的 HTTP 錯誤狀態由 `exception/MediaServiceException.java:37-73` 與 `exception/GlobalExceptionHandler.java:26-66` 決定：invalidArgument 400、permissionDenied 403、notFound 404、failedPrecondition 409、internal 500。CI 的 Java 沒有 R2 設定，這些規則對 Java 實跑不到，B1-3 的 Connect 情境依本節寫（用 fake 儲存）。

| 規則 | 內容 | 來源 | B1-3 怎麼處理 |
|---|---|---|---|
| U1 授權 | 必須帶帳號 ID 與帳號類型；類型要涵蓋 BUYER、SELLER 或 ADMIN 其一，否則 403（缺少類型、無效類型、類型不足各有訊息） | `SecureMediaController.java:80-81,88,358-371` | 沿用：由入口的 session 取身分，不再讀 header |
| U2 會話 ID 格式 | 路徑的 `id` 以 `UUID.fromString` 解析 | `:93` | 不沿用：Java 沒有攔 `IllegalArgumentException`，格式錯誤落到通用 handler 回 500（`GlobalExceptionHandler.java:62-66`）；Go 回 invalid argument。ID 形狀依 B1-D8 |
| U3 會話歸屬 | 以（會話 ID、帳號 ID）查；查無（含別人的會話）一律 404、同一句訊息，兩種情況不可分辨 | `:96-99` | 沿用 |
| U4 必須是機敏會話 | 會話的 `is_secure` 不是 true → 400「此會話不是機敏上傳」 | `:102-105` | 沿用（若 D5 把機敏與公開合成同一條通道，這條改成「通道依會話型別選」） |
| U5 狀態 | 會話狀態必須是 INIT，否則 409 | `:108-111` | 沿用 |
| U6 過期 | `expires_at` 早於現在 → 400「上傳會話已過期」 | `:114-116` | 沿用 |
| U7 大小 | 檔案大小取 `Content-Length` header，沒有才取 request 的長度；**不**與建立會話宣告的大小比對、**不**檢查上限（只有 Spring multipart 的 100MB 與 gateway 的 20MiB body 上限） | `:118-119`；`application.yml` `spring.servlet.multipart`；`gateway/internal/middleware/body_guard.go:14,53` | 補強，不沿用：上限單一來源（缺陷 5.8），宣告大小與實際大小要相符（B1-3 驗收的「超量上傳被拒」） |
| U8 加密金鑰 | 每次呼叫產生新的檔案金鑰（256-bit 金鑰＋96-bit IV），以主密鑰 AES-GCM 包裝成一個字串存回會話；主密鑰沒設定時回 500「加密主密鑰未配置」 | `:122`；`crypto/AesStreamCipher.java:22-30,75-104,195-197` | 依 B1-D4 的分塊 AEAD 改寫；缺主密鑰拒寫（列 101） |
| U9 每次嘗試獨立 key | 物件 key 是 `{會話的 key}.attempt-{uuid}`，不重用建立會話時的 key，兩個並發嘗試天生寫到不同位置 | `:124-127`；測試列 1 | 保留「每次嘗試獨立 key」；但正式資產的 key 不含 `.attempt-`（缺陷 5.15） |
| U10 密文大小與型別 | 密文長度＝明文長度＋16 bytes tag；寫入的 Content-Type 一律是 `application/octet-stream`，與 request 的 `Content-Type` header 無關（header 讀進來但從沒使用） | `:82,130-139`；`AesStreamCipher.java:44` | 密文開銷依 B1-D4；型別一律 octet-stream 沿用；request 的 Content-Type 要與宣告比對（補強，缺陷 5.3） |
| U11 串流 | 邊收邊加密邊寫儲存，不整檔進記憶體 | `:132-140` | 沿用（B1-D4 的分塊格式本來就為此） |
| U12 儲存失敗 | 儲存寫入失敗（含逾時）→ 500「檔案上傳失敗」；會話維持 INIT、沒有加密金鑰、沒有 staged token，也不刪任何既有物件 | `:132-140`；測試列 3（`SecureMediaControllerTest.java:190-214`）；`storage/R2StorageService.java:171-195` | 沿用 |
| U13 CAS 轉狀態 | 以 INIT→COMPLETED 的條件更新，同一個更新寫入：`expires_at` 延長為現在＋預覽 TTL（預設 3600 秒）、包裝後的金鑰、這次嘗試的 key | `:62-63,142-151` | 沿用；預覽 TTL 只有一處定義（缺陷 5.14、B1-2 驗收） |
| U14 並發落敗 | CAS 更新 0 列（別的嘗試先贏）→ 刪自己的嘗試物件（刪失敗只記錯誤 log、留下無主物件），回 409「並發上傳衝突」；贏家的物件與金鑰不被覆寫 | `:153-163`；測試列 2 | 沿用；刪失敗要有補償機制（過期暫存 GC 掃到無主物件），不只記 log |
| U15 成功回應 | 回 staged token 與預覽路徑 `/api/v1/app/media/secure_preview/{id}`。token 的 claims：會話 ID、帳號 ID、是否機敏、scope、到期時間（＝延長後的預覽到期時間） | `:165-184`；`service/StagedTokenService.java:90-105` | token 形狀依 B1-D2；預覽路徑依 B1-3 的契約 |
| U16 Java 沒有的檢查 | 沒有 magic bytes 驗 PDF／圖片（缺陷 5.3）；沒有 Content-Type 與宣告比對；沒有大小上限（U7） | 上列來源皆無對應程式 | 計畫 B1-3 要求補上（mutation：拿掉 magic-byte 檢查 → 「宣告 PDF 實為任意位元組被拒」紅） |

## 三、機敏預覽的行為規格（B1-D12）

來源：`GET /app/media/secure_preview/{session_id}`，`controller/SecureMediaController.java:202-273`；單元測試列 4、5（`SecureMediaControllerTest.java:245,291`）。預覽只給上傳者本人、在確認（`Confirm`）之前使用；確認後的讀取走擁有者端點（第四節）。

| 規則 | 內容 | 來源 | B1-3 怎麼處理 |
|---|---|---|---|
| P1 授權 | 同 U1 | `:204-205,210` | 同 U1 |
| P2 會話 ID 格式 | 同 U2（`UUID.fromString`，格式錯誤 Java 回 500） | `:215` | 同 U2 |
| P3 歸屬 | 以（會話 ID、帳號 ID）查；別人的會話與不存在不可分辨，都是 404；管理員也只能預覽自己上傳的 | `:217-221` | 沿用 |
| P4 狀態 | 會話必須是 COMPLETED，否則 409「僅已完成的會話可預覽」 | `:223-227` | 沿用 |
| P5 過期 | `expires_at` 早於現在 → 400「預覽已過期」；`expires_at` 是上傳完成時延長的預覽到期時間（U13） | `:229-232` | 沿用 |
| P6 缺金鑰 | 會話沒有加密金鑰 → 500「此會話缺少加密金鑰」 | `:235-238` | 沿用（資料不一致才會發生） |
| P7 串流解密 | 以會話記錄的 key（贏家的嘗試 key）從儲存讀密文，邊讀邊解密邊寫給 client，緩衝 8192 bytes | `:240-241,263-268` | 沿用；補強：Java 的 GCM tag 在串流結尾才驗，密文被竄改時 client 已收到明文【推：依 GCM 串流解密的性質，沒有實測】，Go 依 B1-D4 要「竄改在送出任何明文前失敗」（列 18） |
| P8 回應型別 | `Content-Type` 取會話宣告的型別；宣告為空時依檔名副檔名推斷（jpg、jpeg、png、gif、webp、pdf，其餘 octet-stream） | `:248-253,398-415` | 沿用（宣告在建立時已驗過白名單） |
| P9 Content-Disposition | 一律 inline（改成 attachment 會把預覽變成下載）；檔名用 RFC 6266／5987 編碼，中文檔名不亂碼 | `:255-259`；測試列 5 | 沿用 |
| P10 顯示檔名 | 取會話的原始檔名，沒有才回退到 object key 尾段（剝 `.attempt-{uuid}`）；兩條路徑都經消毒：移除控制字元、雙引號、反斜線，取 basename，最長 255 字元並保留副檔名；都沒有就是空字串 | `:244-246`；`util/AssetFilename.java:28,38,64,83,106-123` | 沿用消毒規則（列 148–169）；回退尾段隨 key 不含檔名消失（列 11、160–163） |
| P11 Content-Length | 取儲存端 HEAD 查得的密文大小減 16 bytes tag，不取宣告大小；查不到物件就不設 header，由後面的讀取拋出真正的錯誤 | `:261,373-393`；測試列 4 | 沿用（宣告值可能與實際不符，會截斷或卡住 client） |

## 四、`secure_download` 的承接

`GET /app/media/secure_download/{asset_id}`（`controller/SecureMediaController.java:284-353`）端點刪除（盤點 §1.2 #3、B1-D6）；單元測試列 6–8 因此寫退場理由。它的授權是「上傳者本人，或該資產有引用且上傳者是呼叫者」，後者被前者蓋過、等於只有上傳者（缺陷 5.5）；回應規則是 Content-Type 取資產的 MIME、Content-Disposition 一律 attachment、檔名同 P10、Content-Length 同 P11、串流解密（`:333-348`）。這些回應規則由擁有者代理端點（seller 的 KYC 與治理佐證讀取，B1-5）以 `Media.ReadSecure` 實作並各自測試，授權與存取紀錄留在擁有者。

## 五、不在本表的 Java 行為

- 盤點的缺陷清單（5.1–5.20）裡，沒有 Java 單元測試覆蓋的 SQL 語意（`selectOrphanedAssets`、`claimAssetForGc`、`deleteOrphanedReferences`、`ON CONFLICT`）：Go 版用真 Postgres 的 integration 測試驗，在 CI 跑，結果出來前標「待 CI」（計畫 B1-2 驗收）。
- 建立上傳會話與完成（公開）兩個端點不在本表：它們有真實的特性測試，情境在 `backend/internal/httpapi/media/characterization/scenarios_test.go`，由 `.github/workflows/backend-characterization.yml` 對 Java 跑。

## 六、B1-3d：Connect driver 對 B1-1 情境的對照

Connect driver（`backend/internal/httpapi/media/characterization/connect_test.go`）對單體的 `MediaAppService` 與機敏段端點跑 B1-1 同一份 `scenarios`（`TestMonolith`），只有單體測得到的機敏分段上傳另跑 `monolithScenarios`（`TestMonolithSecure`）。兩者都是 `integration` tag，由 `backend-monolith` 的 integration 步驟在 CI 跑（單體的 media 要真的 Postgres；`backend-characterization` 的單體在 B1-5 之前還接 Java 的假服務，Go media 未接線）。結果出來前標「待 CI」。

與 Java 刻意不同的情境（`scenarios_test.go` 的 `d.Variant() == Monolith` 分支寫了決策代號與新行為，沒有刪情境）：

| 情境 | 單體的新行為 | 依據 |
|---|---|---|
| create public session returns presigned url and 30 minute ttl | session ID 是不透明 staged token；到期時間是 `media.UploadTTL`（1 小時），不是 1800 秒 | B1-D2；B1-D1，CARRYOVER 沿用盤點「暫存 TTL」列 |
| create secure session has no upload url and allows pdf | 機敏會話是 `BeginSecureUpload`，不收宣告的型別，PDF 在第 0 段依 magic bytes 辨認 | B1-D17；單體情境 `real pdf bytes are accepted`、`declared pdf that is arbitrary bytes is rejected` |
| complete returns staged token and preview url | 完成後 token 不變（不再簽 JWT）、沒有 previewUrl，`GetUpload` 的狀態是 UPLOADED | B1-D2；B1-D1 |
| complete twice fails | 重送完成一律成功（冪等，不重複排 job） | B1-D1 補充、proto `CompleteUpload` 註解 |
| complete rejects secure session | 現況：機敏 token 的 `CompleteUpload` 成功但無作用（只有 PENDING 會轉 UPLOADED）；計畫沒有決定要不要拒絕，情境照現況寫 | B1-D17；`upload.Service.Complete`、`MarkUploadUploaded` |
| complete rejects malformed session id | 形狀不對的 token 回 not_found（與不存在不可區分），不再是 invalid | B1-D2 |

單體專屬情境對應 U 規則：`declared pdf that is arbitrary bytes is rejected`（B1-3 驗收，補強 U4／U7）、`real pdf bytes are accepted`（B1-D17）、`oversize secure upload is rejected`（U7，缺陷 5.8）、`chunk over segment size is rejected`（U7）、`chunk length that differs from the declared size is rejected`（U7）、`chunk of another accounts session is not found`（U3）、`chunk index gap is a conflict`（B1-D17）。

只有代行（delegated）或後台上傳才有的 B1-1 情境：沒有。B1-1 的 11 個情境都以買家身分呼叫；代行營運上傳的情境隨 B1-3c3 補進同一份清單。
