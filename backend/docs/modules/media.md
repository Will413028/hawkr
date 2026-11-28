# media 模組行為規格

本文是單體 `media` 模組行為的唯一來源：公開圖片與機敏檔案的暫存上傳、處理、確認、刪除與回收，以及機敏資產的串流上傳與讀取。上傳的 wire 契約在 `api/hawkr/media/v1alpha1/`；步驟與決策在 `backend/docs/plans/2026-11-21-b1-media.md`。

## 1. 範圍與公開介面

- 實作在 `internal/modules/media`，自己的 `media` schema 與 migration，登記在 `cmd/hawkr/modules.go` 的 `modules()`。組裝點（`cmd/hawkr/media.go`）提供物件儲存（`MEDIA_STORAGE`）與 KEK 金鑰環（`MEDIA_KEKS`），把 `media.Queues()`、`AddWorkers`、`PeriodicJobs` 接進 worker；缺設定或 KEK 格式錯誤時啟動失敗，不退回 fake。
- 擁有媒體的模組（buyer、seller、content、social、review、im、moderation）直接依賴 `*media.Module` 的方法子集，不經轉接層。Java media-service 已刪除（B1-10）；`pkg/media` 只剩還沒搬進單體的舊 Go 服務（invoice、ai）使用。
- 根套件公開 `Uploader`、`ConfirmedAsset`、`PutPublicResult` 等別名與 `ErrNotSecure`、`ErrNoTransaction`、`ErrInTransaction`、`ErrNoKeyring`；`ConfirmFields`、`ConfirmField` 是擁有者確認 token 時把不合格的 token 指到欄位錯誤的共用函式；`RetiredAssets`、`AssetIDs` 是擁有者算「寫入前的資產減寫入後的資產」交給 `MarkDeleted` 的共用 helper（B1-D23）。
- 測試用的記憶體內替身：`Fake`（`media.NewFake`，擁有者模組的 `Confirm`、`MarkDeleted`、`ReadSecure`、`CopyPublic`）與 `MemoryStore`（物件儲存）。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 2. 暫存與確認

- client 以 `BeginUpload` 取得預簽 PUT 與只出現一次的 token（資料表只存雜湊），上傳後 `CompleteUpload`，`GetUpload` 輪詢到 `READY` 或 `REJECTED`（`RejectReason`）。機敏檔案走 `BeginSecureUpload` 與 `PutSecureChunk` 分段上傳；伺服器內部以 `PutPublic`、`PutSecure` 直接寫入，同樣得到 `READY` 的 token。
- 擁有者在寫入業務列的 transaction 內呼叫 `Confirm(ctx, uploader, token, want)`：只接受 `READY`、未過期、上傳者相符、可見性等於 `want`（公開圖片或機敏檔，必填）的暫存，可見性不符回 InvalidInput 且不寫認領，回傳 `ConfirmedAsset`（資產 UUID、object key、可見性、型別、大小）。只在同一個 transaction 內重入冪等（回同一個資產）；token 已在別的 transaction 確認過回 FailedPrecondition（B1-D23），避免一個資產被兩個擁有者共用、一方 `MarkDeleted` 弄壞另一方，所以擁有者以全部欄位的前後差集決定 `MarkDeleted`，client 不重送未變的 token。確認與業務列一起提交或回滾；回滾時認領列一起消失、暫存仍是 `READY`，同一個 token 可以重送。確認只在擁有者的 transaction 內 INSERT 一列 `media.upload_claims`（`CONFIRMED`），不更新 media 既有的列（B1-D25；單體規則 4 的鎖順序檢查器不允許擁有者 transaction 鎖 media 的既有列）。與 GC 搶同一個唯一鍵 `upload_id`：先插入的一方擁有這筆暫存，輸給 GC 的 Confirm 讀到 `RECLAIMED` 回「已過期」。
- `Confirm` 回 NotFound（不存在或不屬於上傳者，兩者無法分辨）、FailedPrecondition（處理中、被拒、過期）、InvalidInput（可見性不符）時，擁有者以 `ConfirmFields` 轉成指向該 token 欄位的 InvalidInput，其他錯誤原樣回傳。
- `Uploader` 由呼叫方傳入：帳號與管理員的 ID 各自發號、會同號，所以帶上 `Admin`；代行營運帶被代行的店家 `Store`，token 的每個動作都要帶同一個值。
- 沒有被確認的暫存與資產由 GC 回收：GC 以 `RECLAIMED` 認領過期暫存，每列各用一個 `lock_timeout`（100ms）的 transaction，撞上擁有者還沒提交的認領就跳過、留給下一輪，所以 GC 與多 token 的擁有者 transaction 之間不會死結、也不會被拖住。
- 狀態由 SQL 推導（B1-D25），列上的 `assets.state` 只記物理生命週期（`STAGED`、`DELETING`、`DELETED`）：

| 狀態 | 推導 |
|---|---|
| 暫存 STAGED／READY／REJECTED | 暫存列 `state` 對應值且沒有任何認領 |
| 暫存 CONFIRMED | 有 `CONFIRMED` 認領（列上的 `CONFIRMED` 是 settle 事後物化，只為讓 GC 索引不累積） |
| 資產 ACTIVE | `state='STAGED'` 且有 `CONFIRMED` 認領且沒有刪除請求 |
| 資產 DELETING | `state='DELETING'`，或 `state='STAGED'` 且有刪除請求 |
| 資產 DELETED | `state='DELETED'` |

  推導的唯一定義是 `GetAssetView` 加 `model.DeriveAssetState`；讀取、確認重入與測試 helper（`media.AssetStateOf`）都經它。

## 3. 刪除

- 擁有者在自己的 transaction 內以 `MarkDeleted(ctx, assetIDs)` 請求刪除不再被指著的資產（換圖、終態）：只 INSERT `media.asset_deletion_requests`（B1-D25），不更新既有列、不在 transaction 內排 job；冪等，未確認、不存在的 ID 忽略。同 transaction 內 `Confirm` 後 `MarkDeleted`，推導狀態立刻是 `DELETING`（再 Confirm 回 `MediaAssetUnavailable`）。
- 流程是「請求 → 套用 job → 物理刪除」：`media.apply_deletions`（River 週期 job，`ApplyInterval` = 30 秒）在 media 自己的 transaction 內把請求所指的資產標為 `DELETING`（冪等，`FOR UPDATE SKIP LOCKED`），同一個 job 隨後做三段式實體刪除（commit → 刪物件 → 標 `DELETED`），並 settle 已確認的暫存列。擁有者 commit 到物件被刪最多 `ApplyInterval` 加一輪刪除；期間推導狀態已是 `DELETING`，`ReadSecure`、`CopyPublic` 已拒絕。
- 認領列與刪除請求列與它們的暫存／資產同生命週期：物理刪除完成的同一個 transaction 一併刪掉（沒有獨立保留期），暫存與資產列保留作墓碑。
- 治理佐證只 `Confirm`，從不 `MarkDeleted`；moderation 的證據快照以 `CopyPublic` 複製成自己確認的新資產，來源被刪也不影響快照。

## 4. 讀取機敏資產

- `ReadSecure(ctx, assetID)` 回傳機敏資產（`SECURE` 且推導為 `ACTIVE`）解密後的串流與上傳時記下的型別；公開資產回 `ErrNotSecure`，其餘 NotFound。
- 授權與存取紀錄由擁有者先做（KYC 證件、治理佐證在 seller）；media 不認識業務。在 DB transaction 之外呼叫。

## 5. 不變式

- `Confirm` 與 `MarkDeleted` 必須在帶 transaction 的 ctx 內（`ErrNoTransaction`）；會碰物件儲存的呼叫不得在 transaction 內（`ErrInTransaction`）。
- 資產由確認它的擁有者擁有；media 不記引用，也不認識 ref 型別。
- 擁有者的 transaction 內只 INSERT `upload_claims`、`asset_deletion_requests`、`asset_gates`、`asset_holds`、`asset_hold_releases` 並 SELECT（B1-D25；`Hold`、`ExtendHold`、`ReleaseHold` 也只 INSERT 與 SELECT；`CopyToSecure` 另 INSERT `assets`（副本資產）、`staged_uploads`、`asset_copy_requests` 與來源原圖上的 `asset_holds`，單體規則 4 的鎖順序檢查器不允許擁有者 transaction 更新或鎖 media 既有的列）；物理 `DELETING` 的已確認資產一定有刪除請求。守門：`appendonly_integration_test.go` 的 tracer 測試與 `TestOrphanDeletingInvariantHoldsAndTheCheckIsNotVacuous`。
- 保全（D34，只登記在 SECURE 證據副本）：被保全的資產永不離開原狀態。與刪除的互斥靠 `asset_gates.asset_id` 的唯一鍵，做法同 Confirm 與 GC：擁有者的 `Hold` INSERT `HOLD` 閘門（`ON CONFLICT DO NOTHING`）、SELECT 閘門（是 `DELETE` 回 `ErrAssetGone`）、再 INSERT 外鍵指向 `HOLD` 閘門的保全列；`ExtendHold` INSERT 較晚的新列，`ReleaseHold` INSERT 解除列。INSERT 保全列撞上外鍵（閘門在讀之後被切成 `DELETE` 或被撤掉）時擁有者 transaction 已中止、分不出原因，`Hold`、`ExtendHold`、`ReleaseHold` 回 `ErrHoldContended`（`Unavailable`；只認 `asset_holds_gate_fkey` 與 `asset_hold_releases_hold_fkey` 兩個外鍵，其他外鍵違反照原樣回錯），擁有者必須重試整個 transaction，重試時成功或讀閘門得到 `ErrAssetGone`。`Hold` 只收已確認（有 `CONFIRMED` 認領）的 `SECURE` 資產，否則回 `ErrAssetNotFound` 或 `ErrNotSecure`。`CopyToSecure` 的結果可能之後失敗：`CopyStatus(copyID)` 回 `PENDING`、`COMPLETED` 或 `FAILED`（未知的 ID 回 `ErrAssetNotFound`），`FAILED` 時 media 已用一般刪除流程請求刪除這個內部副本資產，擁有者要重新請求複製。複製中途放棄或提交結果不明而刪不掉的物件記入 `orphan_objects`，由刪除 job 先確認沒有 `secure_chunks` 引用才重試刪除。media 的 worker（刪除請求套用、GC 認領）在帶 `lock_timeout`（100ms）的 transaction 內先取得 `DELETE` 閘門：鎖 `HOLD` 閘門、清掉到期與已解除的保全列、沒有剩餘保全就撤掉 `HOLD` 閘門、INSERT `DELETE` 閘門，讀回不是 `DELETE` 就放棄；撞上擁有者未提交的閘門或保全列就跳過、下一輪再試。到期整理（`media.expire_holds`）與複製完成的釋放也只在 worker transaction 內刪保全列與閘門。`deleteUpload`、`deleteAsset` 刪物件前的鎖列再檢查（`FOR NO KEY UPDATE`，帶 `lock_timeout`，撞上鎖就跳過）保留，作為最後一道。
- 沒有 KEK 金鑰環時，機敏資產的上傳與讀取拒絕執行，不寫任何物件。
