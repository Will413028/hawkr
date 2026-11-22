# media 模組

## 職責

公開圖片與機敏檔案的上傳、處理、確認、刪除與回收，機敏資產的串流上傳與讀取，以及擁有媒體的模組（buyer、seller、content）共用的引用同步演算法。規格：`backend/docs/modules/media.md`；步驟與決策：`backend/docs/plans/2026-11-21-b1-media.md`；沿用盤點：`CARRYOVER.md`。

## 擁有的資料

schema `media`（`db/schema.sql`）：`assets`（`STAGED → ACTIVE → DELETING → DELETED`）、`staged_uploads`（token 只存 SHA-256；`PENDING → UPLOADED → READY｜REJECTED → CONFIRMED`，GC 走 `DELETING → DELETED`）；登記在 `cmd/hawkr/modules.go`。子領域：`internal/upload`（上傳、處理 job、機敏上傳與讀取）、`internal/image`（libvips 重新編碼）、`internal/secure`（STREAM AEAD、DEK 包裹、KEK 金鑰環）、`internal/storage`（`Store` port、R2 與 S3 相容 adapter、`Fake`）、`internal/reclaim`（`MarkDeleted`、GC、實體刪除）、`internal/model`（狀態、上限、期限的唯一定義）。讀取模型：無。

## 發布的讀取介面

無，沒有 `pub_*`。對外是 `module.go` 的 `Module`（`BeginUpload`、`CompleteUpload`、`GetUpload`、`Confirm`、`MarkDeleted`、`UploadSecure`、`PutSecure`、`PutPublic`、`ReadSecure`、`ReadStagedSecure`、`RewrapDEKs`）與引用同步 `media.Sync*`。`Confirm`、`MarkDeleted` 必須在擁有者的 transaction 內；其餘會碰物件儲存的呼叫不得在 transaction 內。舊 `Media` interface 仍由 Java media-service 實作（`internal/legacy/mediaservice`），隨 B1-5 退場。

## 模組內部結構

- 根套件以別名公開 `internal/model`；`fake.go` 是記憶體內 `media.NewFake`；`ref_type.go` 由 CodeMeta 生成，不得手改。
- 依賴 libvips（`vipsgen`）：CI 裝 `libvips-dev`，本機 `brew install vips`。新增直接依賴先經同意。
- 組裝點（`cmd/hawkr`）提供 `ObjectStore`（`MEDIA_STORAGE`，缺設定啟動失敗，不退回 fake）、`media.ParseKeyring(MEDIA_KEKS)`，並把 `media.Queues()` 經 `workerQueues` 併入 worker（queue 名重複啟動失敗）；模組不讀環境。
- 依賴圖唯一來源：`internal/arch/modules.go` 的 `InternalDependencies`。
- 內部直接寫入（B1-D14）：`PutSecure`（出貨單 PDF 與圖片，同 `UploadSecure` 管線）、`PutPublic`（只收圖片，同步重新編碼）建 `READY` 暫存並回 token，擁有者在自己的 transaction 內 `Confirm`，呼叫時 `Uploader` 帶操作者；沒確認的由 GC 回收。不新增資產狀態與身分型別，目前沒有呼叫方。

## 不變量

- 引用只由擁有者的同步 job 寫入；Unlink 只解除精確列出的資產；Finalize 不建立引用。守門：`TestSyncNeverUnlinksAnythingButRetiredAssets`（seller）、`TestFinalizeSignsCarriesTheUploaderAndCreatesNoReference`。規格：§2、§3、§5。
- ReadSecure 要 `object_key`、`ref_type`、`ref_id` 全等，非機敏回 `ErrNotSecure`；呼叫方先授權並記稽核。守門：`TestReadSecureSendsTheTripleAndSeparatesNotSecureFromNotFound`。
- `READY` 的公開物件已在 `public/<uuid v4>`，網址不可猜且只回給上傳者；Confirm 只接受 `READY`、未過期、上傳者相符，重入回同一資產。守門：`TestConfirm_RejectsNonUploader`、`TestConfirm_ReentryReturnsSameAsset`。
- GC 只刪過期未確認的暫存（含超過 `StuckUploadGrace` 仍 `UPLOADED` 的）與其物件；先標 `DELETING` 並提交，transaction 外刪物件，再標 `DELETED`，失敗留著重試；已確認資產只由 `MarkDeleted` 刪。守門：`TestGC_NeverDeletesConfirmedAssets`、`TestStorageIsNeverCalledInsideATransaction`。
- 期限與上限只在 `internal/model` 定義一次（`UploadTTL`、`PresignTTL`、`StuckUploadGrace`、`Max*Size`、`MaxImagePixels`、`MaxAnimatedPixels`、`MaxImageDimension`）。守門：`TestStuckUploadGraceOutlastsEveryProcessAttempt`。
- `public/` 的圖片一律經 libvips 重新編碼（JPEG、PNG、WebP、GIF；sRGB、已套用方向、無 metadata）；辨認不出、無法解碼、超過上限永久拒絕；`image.Limits` 非正回 `ErrInvalidLimits`。守門：`TestFakeImageIsRejected`、`TestHeightOverDimensionLimitIsRejectedFromTheHeader`、`TestNonPositiveLimitsAreRejected`。
- 機敏檔只以密文落地：`UploadSecure` 依 magic bytes 辨認、以實際讀到的位元組套用上限、STREAM 加密到暫存檔再 `Put`，沒有金鑰環就不寫。守門：`TestCiphertextHoldsNoPlaintext`、`TestUploadSecure_NoPlaintextInAnyPutOrStoredObject`（CI）。直接寫入同規則：`TestPutSecure_PDFIsEncryptedAndReadsBackAfterConfirm`（CI）、`TestPreparePublic_RejectsNonImagesAsInvalidInput`。

## 不負責

- 業務列、asset ID 欄位與各 ref 的同步 job：由 buyer、seller、content 擁有。
- 機敏下載的授權與存取紀錄：擁有者的端點負責。
- 圖片讀取出口（image gateway、CDN）；public 圖片無 EXIF 由 `internal/image` 達成（ADR `2026-11-22-media-public-image-reencode.md`）。

## 決策紀錄

沿用盤點見 `CARRYOVER.md`。ADR：`docs/adr/2026-10-03-media-references-owner-side.md`、`2026-11-22-media-secure-stream-aead.md`、`2026-11-22-media-upload-channels.md`、`2026-11-22-media-public-image-reencode.md`、`2026-09-24-go-modular-monolith.md`（D2）。
