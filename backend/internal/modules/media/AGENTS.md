# media 模組

## 職責

公開圖片與機敏檔案的上傳、處理、確認、刪除與回收，機敏資產的串流上傳與讀取。規格：`backend/docs/modules/media.md`；步驟與決策：`backend/docs/plans/2026-11-21-b1-media.md`；沿用盤點：`CARRYOVER.md`。

## 擁有的資料

schema `media`（`db/schema.sql`）：`assets`（列上 `state` 只記物理生命週期 `STAGED → DELETING → DELETED`，`ACTIVE` 由認領與刪除請求推導）、`staged_uploads`（token 只存 SHA-256；`PENDING → UPLOADED → READY｜REJECTED → CONFIRMED`，`CONFIRMED` 由 settle 物化，GC 走 `DELETING → DELETED`）、`upload_claims`（Confirm 與 GC 搶的認領）、`asset_deletion_requests`（擁有者的刪除請求）；登記在 `cmd/hawkr/modules.go`。

子領域：`internal/upload`（上傳、處理 job、機敏上傳與讀取）、`internal/image`（libvips 重新編碼）、`internal/secure`（STREAM AEAD、DEK 包裹、KEK 金鑰環）、`internal/storage`（`Store` port、R2 與 S3 相容 adapter、`Fake`）、`internal/reclaim`（刪除請求、套用 job、GC、實體刪除；不變量見其 `AGENTS.md`）、`internal/model`（狀態、上限、期限的唯一定義）。讀取模型：無。

## 發布的讀取介面

無，沒有 `pub_*`。對外是 `module.go` 的 `Module`（`BeginUpload`、`CompleteUpload`、`GetUpload`、`Confirm`、`MarkDeleted`、`BeginSecureUpload`、`PutSecureChunk`、`PutSecure`、`PutPublic`、`ReadSecure`、`ReadStagedSecure`、`RewrapDEKs`）與確認 helper `ConfirmFields`、`ConfirmField`。`Confirm`、`MarkDeleted` 必須在擁有者的 transaction 內；其餘會碰物件儲存的呼叫不得在 transaction 內。Java media-service 已刪除（B1-10），單體內沒有它的 client。

## 模組內部結構

- 根套件以別名公開 `internal/model`；`fake.go` 是記憶體內 `media.NewFake`（擁有者模組的替身）與 `MemoryStore`；`confirm_fields.go` 是擁有者確認 token 的共用函式。
- 依賴 libvips（`vipsgen`）：CI 裝 `libvips-dev`，本機 `brew install vips`。新增直接依賴先經同意。
- 組裝點（`cmd/hawkr`）提供 `ObjectStore`（`MEDIA_STORAGE`，缺設定啟動失敗，不退回 fake）、`media.ParseKeyring(MEDIA_KEKS)`，並把 `media.Queues()` 經 `workerQueues` 併入 worker（queue 名重複啟動失敗）；模組不讀環境。
- 依賴圖唯一來源：`internal/arch/modules.go` 的 `InternalDependencies`。
- 內部直接寫入（B1-D14）：`PutSecure`、`PutPublic` 建 `READY` 暫存並回 token，不變量見 `internal/upload/AGENTS.md`。

## 不變量

- media 不記引用也不認識業務：資產由確認它的擁有者擁有，擁有者在自己的 transaction 內 `Confirm`、`MarkDeleted`；治理佐證只 `Confirm`，moderation 的快照以 `CopyPublic` 複製。守門：`TestMarkDeleted_RemovesEveryChunkObjectOfASecureAsset`。規格：§2、§3。
- 擁有者 transaction 內只 INSERT `upload_claims`、`asset_deletion_requests` 並 SELECT，不得更新或鎖 media 既有的列、不得 Enqueue（B1-D25；規則 4）；新對外方法要加進 `appendonly_integration_test.go` 的 tracer 測試。
- `ReadSecure` 只讀 `SECURE` 且推導為 `ACTIVE` 的資產，公開資產回 `ErrNotSecure`、其餘 NotFound，且拒絕 transaction；呼叫方先授權並記稽核。守門：`TestReadSecure_RejectsPublicAsset`、`TestReadSecure_NotFoundAfterOwnerDeletes`。
- `READY` 的公開物件已在 `public/<uuid v4>`，網址不可猜且只回給上傳者；Confirm 只接受 `READY`、未過期、上傳者相符，只在同一個 transaction 內重入回同一資產（同 transaction 已請求刪除則 Unavailable），已在別的 transaction 確認過回 FailedPrecondition（B1-D23）。守門：`TestConfirm_RejectsNonUploader`、`TestConfirm_ReentryInTheSameTransactionReturnsSameAsset`、`TestConfirm_ConfirmedInAnotherTransactionIsRejected`。
- GC 只刪過期未確認的暫存與其物件，已確認資產只由 `MarkDeleted` 刪；GC 絕不等擁有者未提交的認領（`lock_timeout`），細則見 `internal/reclaim/AGENTS.md`。守門：`TestGC_NeverDeletesConfirmedAssets`、`TestStorageIsNeverCalledInsideATransaction`。
- 期限與上限只在 `internal/model` 定義一次（`UploadTTL`、`PresignTTL`、`StuckUploadGrace`、`Max*Size`、`MaxImagePixels`、`MaxAnimatedPixels`、`MaxImageDimension`）。守門：`TestStuckUploadGraceOutlastsEveryProcessAttempt`。
- `public/` 的圖片一律經 libvips 重新編碼，細則見 `internal/image/AGENTS.md`。
- 機敏檔只以密文落地：`PutSecureChunk` 逐段（`model.SecureChunkSize`）收，第 0 段依 magic bytes 辨認，每段核對長度、在記憶體加密後寫成新 key 的物件，沒有金鑰環就不寫。守門：`TestCiphertextHoldsNoPlaintext`、`TestSecureUpload_NoPlaintextInAnyPutOrStoredObject`（CI）。
- 機敏資產讀到的每一段都是推進成功那次請求寫的物件（B1-D17）：條件式推進，輸家刪自己的物件；密文 v2 的 AAD 綁資產、段序號、段總數；`object_key` 是前綴，各段在 `secure_chunks`，列數不等於 `chunk_count` 回 `secure.ErrCorrupt`。守門：`TestChunks_SwappedChunksFailToDecrypt`、`TestSecureUpload_ConcurrentSameChunkReadsBackTheWinnerAndDeletesTheLoserObject`（CI）。

## 不負責

- 業務列、asset ID 欄位與各 ref 的同步 job：由 buyer、seller、content 擁有。
- 機敏下載的授權與存取紀錄：擁有者的端點負責。
- 圖片讀取出口（image gateway、CDN）；public 圖片無 EXIF 由 `internal/image` 達成（ADR `2026-11-22-media-public-image-reencode.md`）。

## 決策紀錄

沿用盤點見 `CARRYOVER.md`。ADR：`docs/adr/2026-10-03-media-references-owner-side.md`、`2026-11-22-media-secure-stream-aead.md`、`2026-11-22-media-upload-channels.md`、`2026-11-22-media-public-image-reencode.md`、`2026-09-24-go-modular-monolith.md`（D2）。
