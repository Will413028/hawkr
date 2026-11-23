# media：擁有者 transaction 內只新增列（B1-D25 實作設計）

前提：B1-D25（plan `2026-11-21-b1-media.md:279`）、簡報 `lockorder-brief.md`。規則 4 與 `internal/platform/lockorder` 不改、不加豁免。
業界基準：Rails Active Storage（attach＝隨紀錄 INSERT `attachments`，不 UPDATE blob；`purge_later` 由 job 刪）。下文標「現況」的都附 file:line，標「推論」的未量測。

## 0. 現況：擁有者 transaction 內會鎖 media 既有列的四句
| 方法 | SQL | 位置 |
|---|---|---|
| Confirm | `MarkUploadConfirmed`（UPDATE staged_uploads READY→CONFIRMED） | `db/queries/uploads.sql:90-95`，呼叫 `internal/upload/upload.go:305` |
| Confirm | `ActivateAsset`（UPDATE assets STAGED→ACTIVE） | `assets.sql:19-22`，`upload.go:327` |
| MarkDeleted | `MarkAssetsDeleting`（UPDATE assets ACTIVE→DELETING） | `assets.sql:24-28`，`internal/reclaim/reclaim.go:82` |
| MarkDeleted | River `Enqueue(DeleteAssetsArgs)` 的 unique insert（river schema 內加鎖） | `reclaim.go:89-90` |
其餘對外方法 `ReadSecure`、`ReadStagedSecure`、`CopyPublic` 都 `RequireNoTx`（`secure_read.go:135,160`、`copy_public.go:22`），不在擁有者 transaction 內；所以「同 transaction 內 Confirm 後 ReadSecure」不存在，唯一的同 transaction 讀取是 Confirm 重入時的 `assetOf(requireActive)`（`upload.go:322,360-372`）。

## 1. Schema（`internal/modules/media/db/schema.sql`）
### 1.1 新表
```sql
-- token 的認領：Confirm 與 GC 搶同一個唯一鍵（upload_id），先 INSERT 的一方擁有這筆暫存。只新增、不更新、不刪。
CREATE TABLE media.upload_claims (
    upload_id   UUID        PRIMARY KEY,
    kind        TEXT        NOT NULL,          -- CONFIRMED：擁有者確認；RECLAIMED：GC 回收
    asset_id    UUID,                           -- CONFIRMED 一定有；RECLAIMED 在暫存還沒有資產時是 NULL
    claimed_xid XID8        NOT NULL DEFAULT pg_current_xact_id(),
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT upload_claims_upload_fkey FOREIGN KEY (upload_id) REFERENCES media.staged_uploads (id),
    CONSTRAINT upload_claims_asset_fkey  FOREIGN KEY (asset_id)  REFERENCES media.assets (id),
    CONSTRAINT upload_claims_asset_key   UNIQUE (asset_id),
    CONSTRAINT upload_claims_kind_check  CHECK (kind IN ('CONFIRMED', 'RECLAIMED')),
    CONSTRAINT upload_claims_asset_check CHECK (kind <> 'CONFIRMED' OR asset_id IS NOT NULL)
);
-- 擁有者的刪除請求：一個資產最多一筆（ON CONFLICT DO NOTHING 冪等）；applied_at 只由 media 的套用 job 寫。
CREATE TABLE media.asset_deletion_requests (
    asset_id     UUID        PRIMARY KEY,
    requested_at TIMESTAMPTZ NOT NULL,
    applied_at   TIMESTAMPTZ,
    CONSTRAINT asset_deletion_requests_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id)
);
CREATE INDEX asset_deletion_requests_pending ON media.asset_deletion_requests (requested_at) WHERE applied_at IS NULL;
```
`claimed_xid` 用 DEFAULT 而非參數：值一定是寫入它的 transaction，取代現在以列 xmin 判斷的 `UploadConfirmedInThisTx`（`uploads.sql:122-124`）。

FK 的鎖：擁有者 INSERT claim／request 時，FK 檢查對 `staged_uploads`、`assets` 的被參照列取 `FOR KEY SHARE`。media job 對這兩表只做不改主鍵的 UPDATE（`FOR NO KEY UPDATE`），兩者相容、不互等；media 從不 DELETE 這兩表的列。classify 本就不算同 schema 的 FK 鎖（`lockorder/classify.go:22-24`）。這是保留 FK 的必要條件，若日後要 DELETE assets 列，需先拿掉這兩個 FK。（推論，PostgreSQL 列鎖相容表 §13.3.2。）

### 1.2 既有表
- `media.assets.state`：值域改為 `('STAGED','DELETING','DELETED')`，只表示**物理生命週期**、只由 media 自己的 transaction 改；`ACTIVE` 移除（由推導取代）。`assets_deleting` 索引不變。
- `media.staged_uploads.state`：`CONFIRMED` 保留，但改成 media 的 settle 步驟（§3.7）事後物化的值，只為讓 `staged_uploads_gc` 部分索引不累積已確認列；權威來源是 claim。CHECK 不變。
- `MarkUploadConfirmed`、`ActivateAsset`、`MarkAssetsDeleting`、`UploadConfirmedInThisTx` 四句刪除。

### 1.3 Migration
`origin/dev` 沒有任何 media migration（`git log origin/dev -- backend/internal/modules/media/db/migrations` 無輸出），兩支（`20261122185529_media_baseline.sql`、`20261123013219_media_delegated_store.sql`）都只在本分支，依 backend `AGENTS.md:33` 規則 6 **壓平成一支 baseline**：刪兩支與 `atlas.sum`，改完 schema.sql 後 `atlas migrate diff media_baseline --env dev` 重產，再 `make sqlc`；由 `TestSchemaFileMatchesMigrations` 守。

## 2. 狀態模型（SQL 推導；`c`=upload_claims、`r`=asset_deletion_requests、`a`=assets、`u`=staged_uploads）
| 狀態 | 謂詞 |
|---|---|
| STAGED（暫存列） | `u.state IN ('PENDING','UPLOADING','UPLOADED') AND NOT EXISTS (c WHERE c.upload_id=u.id)` |
| READY | `u.state='READY' AND NOT EXISTS (c WHERE c.upload_id=u.id)` |
| REJECTED | `u.state='REJECTED' AND NOT EXISTS (c WHERE c.upload_id=u.id AND c.kind='CONFIRMED')`（被拒不可能有 CONFIRMED claim：Confirm 的 INSERT…SELECT 只選 READY） |
| CONFIRMED（暫存列） | `EXISTS (c WHERE c.upload_id=u.id AND c.kind='CONFIRMED')`（不論 u.state 是 READY 還是已物化的 CONFIRMED） |
| 資產 CONFIRMED-ACTIVE | `a.state='STAGED' AND EXISTS (c WHERE c.asset_id=a.id AND c.kind='CONFIRMED') AND NOT EXISTS (r WHERE r.asset_id=a.id)` |
| 資產 DELETING | `a.state='DELETING' OR (a.state='STAGED' AND EXISTS (r WHERE r.asset_id=a.id))` |
| 資產 DELETED | `a.state='DELETED'` |
| 暫存 DELETING／DELETED | `u.state IN ('DELETING','DELETED')`（只由 GC 寫，前提是它贏得 RECLAIMED claim） |

落成一句 sqlc `GetAssetView :one`（回 assets 欄位＋`confirmed bool`＋`delete_requested bool`），Go 端 `model.DeriveAssetState(row)` 回 `AssetStaged|AssetActive|AssetDeleting|AssetDeleted`，`AssetActive` 常數保留但只從推導產生。取代 `GetAsset` 在 `upload.go:364`、`secure_read.go:141,181`、`copy_public.go:28` 的用法。

**誰可以 UPDATE**（只在 media 自己開的 transaction，`RequireNoTx` 的方法或 job 內）：`staged_uploads`（上傳、處理、GC、settle）、`assets`（處理建列外的 state：GC、刪除套用、物理刪除、rewrap）、`asset_deletion_requests.applied_at`（套用 job）、`secure_chunks`（DELETE，物理刪除）。`upload_claims` 永不 UPDATE／DELETE。擁有者 transaction 內只 INSERT `upload_claims`、`asset_deletion_requests`，並 SELECT。

## 3. 方法與 job
### 3.1 Confirm（`upload.go:298`；ConfirmField(s) 經它，`confirm_fields.go:48,95` 不改）
1. `RequireTx`、`own()`（`GetUploadByTokenHash`，純 SELECT）照舊。
2. `ClaimUpload :one`：
```sql
INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at)
SELECT u.id, 'CONFIRMED', u.asset_id, sqlc.arg(now) FROM media.staged_uploads u
WHERE u.id = sqlc.arg(id) AND u.state = 'READY' AND u.expires_at > sqlc.arg(now)
ON CONFLICT (upload_id) DO NOTHING
RETURNING asset_id;
```
3. 有列 → `GetAssetView(asset_id)` 組 `ConfirmedAsset`（ID、ObjectKey、Visibility、ContentType、SizeBytes 都來自不變欄位，不加鎖）。
4. 無列 → `GetClaim(upload_id)`（SELECT，含 `claimed_xid = pg_current_xact_id() AS here`）：
   - 無 claim → `notConfirmable(GetUpload)`（被拒／未就緒／過期，訊息同 `upload.go:342-357`）。
   - `kind=RECLAIMED` → `MediaUploadExpired`（同現在 DELETING 的訊息）。
   - `kind=CONFIRMED AND here` → 同 transaction 重入：`GetAssetView` 推導，不是 ACTIVE（同 transaction 已 MarkDeleted）→ `MediaAssetUnavailable`，否則回同一資產（B1-D23）。
   - `kind=CONFIRMED AND NOT here` → `MarkConfirmedElsewhere(FailedPrecondition MediaUploadAlreadyUsed)`（`ErrUploadAlreadyConfirmed` 語意不變）。
5. 不再 UPDATE 任何列。

### 3.2 MarkDeleted（`reclaim.go:72`）
`RequireTx`、排序去重照舊；改為一句 `RequestDeletions :many`，不 Enqueue：
```sql
INSERT INTO media.asset_deletion_requests (asset_id, requested_at)
SELECT c.asset_id, sqlc.arg(now) FROM media.upload_claims c
WHERE c.asset_id = ANY(sqlc.arg(ids)::uuid[]) AND c.kind = 'CONFIRMED'
ORDER BY c.asset_id
ON CONFLICT (asset_id) DO NOTHING
RETURNING asset_id;
```
未確認、不存在的 ID 被 SELECT 濾掉（與現在「只 ACTIVE 才轉」一致）；已請求過的冪等。已 DELETING／DELETED 的資產一定已有請求（物理 DELETING 只會來自請求或 GC，GC 只碰沒有 CONFIRMED claim 的資產），ON CONFLICT 吸收。同 transaction 內剛 Confirm 的 claim 對自己可見，所以 **Confirm→MarkDeleted 同 transaction 會寫入請求，推導立刻是 DELETING，commit 後由 job 刪**；rollback 兩列一起消失、物件歸 GC。不洩漏。

### 3.3 刪除套用 job（新，`reclaim.ApplyDeletions`；River periodic `media.apply_deletions`）
media-only transaction，keyset 分批 200：
```sql
-- name: ApplyDeletionRequests :many
WITH picked AS (
  SELECT asset_id FROM media.asset_deletion_requests
  WHERE applied_at IS NULL ORDER BY requested_at, asset_id LIMIT sqlc.arg(max_rows) FOR UPDATE SKIP LOCKED),
marked AS (
  UPDATE media.assets a SET state = 'DELETING', updated_at = sqlc.arg(now)
  FROM picked p WHERE a.id = p.asset_id AND a.state = 'STAGED' RETURNING a.id)
UPDATE media.asset_deletion_requests r SET applied_at = sqlc.arg(now)
FROM picked p WHERE r.asset_id = p.asset_id RETURNING r.asset_id;
```
冪等：assets 只從 STAGED 轉、已 DELETING／DELETED 不動；applied_at 只寫一次；兩個 worker 由 SKIP LOCKED 分開。之後呼叫既有 `DeleteAssets`（`reclaim.go:148`）做物理刪除，三段式不變。順序：先 commit 標記，再刪物件，失敗留 DELETING 由下一輪重試（現況語意，`reclaim.go:3-6`）。
- 間隔：新常數 `ApplyInterval`（建議 30s，見 §8），`RunOnStart: true`，`jobs.SweepOpts(ApplyInterval)` 合併；`SweepInterval` 的 GC 也先呼叫一次 ApplyDeletions 當補掃。不另做退避：job 失敗由 River 重試（`MaxAttempts`），下一個週期自然再掃。
- 延遲上界（推論）：commit 到物件刪除 ≤ ApplyInterval＋一輪 DeleteAssets。期間推導狀態已是 DELETING，ReadSecure／CopyPublic 已拒絕；公開物件的 CDN URL 在物件刪掉前仍可取（現在 commit 到 job 跑完也一樣，只是窗口從秒級變 ≤30s）。

### 3.4 GC（`CollectGarbage`，`reclaim.go:103`）
與 Confirm 以同一唯一鍵互斥，取代 `FOR UPDATE SKIP LOCKED` 對 Confirm 的仲裁（`uploads.sql:97-110`）：
1. `ReclaimExpiredUploads :many`：`INSERT INTO upload_claims (upload_id,kind,asset_id,created_at) SELECT u.id,'RECLAIMED',u.asset_id,now FROM staged_uploads u WHERE (<現行過期條件>) AND NOT EXISTS (claim) ORDER BY u.expires_at,u.id LIMIT n ON CONFLICT DO NOTHING RETURNING upload_id, asset_id`。
2. 同 transaction `UPDATE staged_uploads SET state='DELETING' WHERE id = ANY(贏得的) AND state IN (...)`、`UPDATE assets SET state='DELETING' WHERE id = ANY(...) AND state='STAGED'`（沿用 `ClaimAssetsOfUploads`）。
3. 輸給 Confirm（ON CONFLICT 無列）＝已確認，不碰。輸給 GC 的 Confirm 讀到 RECLAIMED → 過期。
- 處理 job 的 `MarkUploadReady` 與 GC 對 staged_uploads 的競爭不變（都是 media-only，條件式 UPDATE，`uploads.sql:78-82`）。PENDING／UPLOADING 列也走 claim，讓 `Complete`、分段推進看到 GC 已擁有時的判斷維持在 `u.state='DELETING'`（GC 同 transaction 寫入），不需改 `upload.go:276`、`secure_upload.go:409`、`direct.go:73`。

### 3.5 讀取
- `ReadSecure`（`secure_read.go:134`）：`GetAssetView` → 推導非 ACTIVE 回 NotFound，其餘不變。
- `CopyPublic`（`copy_public.go:21`）：同上，推導 ACTIVE 且 PUBLIC 才複製；來源只讀。
- `ReadStagedSecure`（`secure_read.go:159`）：條件加「無 claim」（READY 推導），已確認回 `MediaSecurePreviewUnavailable`（現在靠 u.state≠READY，`:176`）。
- `GetUpload`／`Get`（`upload.go:284`）：有 CONFIRMED claim 回 `StateConfirmed`，有 RECLAIMED 且 u.state 仍 READY（同一 commit 內不會發生，防禦性）回 DELETING。
- `assetOf`（`upload.go:360`）改用 `GetAssetView`＋推導。

### 3.6 Rewrap、處理、上傳
`ListAssetsToRewrap` 條件 `state <> 'DELETED'` 不變（值域縮小仍正確）；`RewrapAsset` 是 media-only UPDATE assets，非主鍵欄位，與擁有者 FK 的 KEY SHARE 相容。`InsertAsset`／`InsertSecureAssetFromUpload` 以 `'STAGED'` 建列不變。`process.go:121-190`、`secure_upload.go`、`direct.go` 不改。

### 3.7 settle（物化，併在 ApplyDeletions 同一 job、另一個 transaction）
`UPDATE staged_uploads u SET state='CONFIRMED' FROM upload_claims c WHERE c.upload_id=u.id AND c.kind='CONFIRMED' AND u.state='READY'`（分批）。必要條件：`staged_uploads_gc` 部分索引含 READY（`schema.sql:134`），已確認列不物化會永遠留在 GC 掃描範圍。推導不依賴它；漏跑只影響 GC 掃描量，不影響正確性。

## 4. 強制方式
- **主防線：執行期檢查器**（`lockorder/checker.go:34`）在所有整合測試照常跑；本設計移除的四句正是它 CI 抓到的約 100 筆違規來源（plan `:279`）。驗收＝這些違規歸零。
- **單元級守門（新，`internal/modules/media/appendonly_integration_test.go`）**：在 testdb 掛一個 recording `pgx.QueryTracer`，於一個 transaction 內跑 Confirm（新確認、重入、跨 transaction 拒絕三條）、ConfirmFields、MarkDeleted（含空清單、已刪），對每句實際送出的 SQL 呼叫 `lockorder.Classify`，任何非空 → 失敗；並斷言沒有任何 SQL 觸及 `river` schema。
  - 能擋：Confirm／MarkDeleted 內加一句 UPDATE／FOR UPDATE／DO UPDATE、或又把 Enqueue 放回來。
  - 擋不住：沒被測試走到的分支（以三條路徑涵蓋全部 return 點，mutant 驗證）；未來新增的對外方法（要把它加進表——在 media `AGENTS.md` 寫成不變量）；FK 的 KEY SHARE 與唯一鍵等待（classify 依設計不算，見 §5）。
- 不採靜態 AST 掃 `q(ctx).X(` 呼叫：要追 helper 遞迴、對 sqlc 名稱對 SQL 常數，複雜度高於 tracer 而覆蓋相同；tracer 看的是真正送出的 SQL。

## 5. INSERT … ON CONFLICT 的等待邊
- 條件：transaction B 對 `upload_claims.upload_id`（或 `asset_deletion_requests.asset_id`）INSERT 時，另一個**尚未 commit** 的 transaction A 已插入同一鍵 → B 等 A 結束（A commit：B 得衝突；A rollback：B 成功）。
- 發生情境：(1) 兩個請求同時用同一 token（client 重送或惡意）；(2) GC 與 Confirm 撞同一過期邊界的 token；(3) 兩個 transaction 同時 MarkDeleted 同一資產。
- 為何可接受：(1) B1-D23 本來就判其中一方失敗；若 A 之後還要等 B 持有的擁有者列，形成環，PostgreSQL 在 `deadlock_timeout`（預設 1s）後中止一方，結果與 D23 相同（一方失敗），屬罕見的錯誤路徑。(2) GC 是 media-only transaction，插入後只 UPDATE 擁有者不鎖的 media 列，不會等擁有者 → 不成環。(3) 一個資產只有一個擁有者（D23），同資產的兩個刪除者是同一擁有者對同一列的操作，已先被擁有者列序列化。檢查器不建模此邊，屬已知且接受的盲點。（推論，PostgreSQL §13.3 與 INSERT 文件；由 §6 的並行測試量測行為，不量測頻率。）

## 6. 擁有者端與測試
**擁有者端**：26 處呼叫（簡報 `:96`）簽章與語意不變，**不需改**。`media.Fake`（`fake.go:22-145`）語意不變（Confirm 後 MarkDeleted 標為已刪），不改。

**同步斷言 DELETING 的整合測試**（`rg -n "DELETING" -g '*_integration_test.go' internal`）：擁有者端
`im/im_integration_test.go:637`、`social/writes_integration_test.go:471,478`、`social/purge_integration_test.go:145`、`content/media_integration_test.go:125,168`、`review/purge_integration_test.go:144`、`buyer/media_integration_test.go:59,165`、`seller/seller_integration_test.go:617,686-687`、`seller/prospect_integration_test.go:849,980,997`、`seller/kyc_integration_test.go:677,690-691,1084,1112-1113`、`seller/purge_integration_test.go:165`。
改寫：各模組的 `assetState`／`wantAsset` helper（如 `seller/media_helpers_integration_test.go:83`）改查推導 view（新增 `media` 匯出的測試 helper `mediatest.AssetState(ctx, db, id)`，以 §2 謂詞回 `ACTIVE|DELETING|...`），斷言字串不變——推導在擁有者 commit 後立刻是 DELETING，不需等 job。直接寫 `UPDATE media.assets SET state='DELETING'` 造狀態的 `seller/governance_evidence_integration_test.go:178`、`seller/kyc_integration_test.go:1555`，改成 INSERT `asset_deletion_requests`（透過 helper `mediatest.RequestDeletion`）。

media 自己：`lifecycle_integration_test.go:1025,1094,1116-1121,1143,1204` 的物理 DELETING 斷言改為「MarkDeleted 後推導 DELETING、物理仍 STAGED；跑 ApplyDeletions 後物理 DELETING／DELETED」；`secure_abort_integration_test.go:97,177` 造的是暫存列 DELETING（GC 路徑），改為同時 INSERT RECLAIMED claim。

**新測試**（media 整合測試，CI 跑）：
1. `TestConfirmThenMarkDeletedSameTxDeletionWins`：同 transaction Confirm→MarkDeleted→再 Confirm 回 Unavailable；commit 後推導 DELETING；ApplyDeletions＋DeleteAssets 後物件不存在、DELETED。rollback 版：無 claim、無請求，資產留給 GC。
2. `TestConfirmReentrySameTxReturnsSameAsset`、`TestConfirmAcrossTxRejected`（`errors.Is(ErrUploadAlreadyConfirmed)`）。
3. `TestConfirmBlocksOnUncommittedClaim`：A Confirm 未 commit，B Confirm 在 goroutine 等；A commit → B FailedPrecondition；A rollback → B 成功。
4. `TestGCAndConfirmMutuallyExclusive`：過期邊界上 GC 與 Confirm 並行 N 次，恰一方得 claim、另一方得預期錯誤或略過；`TestGC_NeverDeletesConfirmedAssets` 保留並改成 claim 語意。
5. `TestApplyDeletionsIdempotent`：重跑、兩 worker 並行、請求指向已 DELETED 資產、applied_at 只寫一次。
6. `TestReadSecureAfterConfirmCommit` 與 `TestReadSecureAfterMarkDeletedCommitIsNotFound`（job 未跑前就 NotFound）；`CopyPublic` 同兩條；`ReadStagedSecure` 確認後回 PreviewUnavailable。
7. §4 的 append-only tracer 測試。
8. `queues_test.go`：PeriodicJobs 含 `media.apply_deletions`，間隔＝`ApplyInterval`。

**Mutants**（寫進 `mutation/`，CI 跑）：`ClaimUpload` 去掉 `AND u.state = 'READY'`（預期紅：拒絕/過期測試）；去掉 `expires_at >`（GC 互斥）；`here` 判斷反轉（2）；`RequestDeletions` 去掉 `kind='CONFIRMED'`（未確認 ID 忽略測試）；推導 ACTIVE 去掉 `NOT EXISTS r`（1、6）；ApplyDeletions `a.state='STAGED'`→刪條件（5，DELETED 被改回 DELETING）；ReclaimExpiredUploads 去掉 `NOT EXISTS claim`（4）；在 MarkDeleted 加回 Enqueue（7）。

## 7. 文件與驗收
- `docs/…/media.md`（模組文件）：狀態圖改成 §2 表；刪除流程改「請求→套用 job→物理刪除」。
- media `AGENTS.md`：不變量「擁有者 transaction 內只 INSERT `upload_claims`、`asset_deletion_requests`；對外方法新增時加進 append-only tracer 測試」。
- `CARRYOVER.md:28`：MarkDeleted 一列改為「INSERT 刪除請求，job 輪詢套用（B1-D25），不在擁有者 transaction 內 Enqueue」；新增一列「Confirm 以 UPDATE 轉狀態 → 認領列（B1-D25）」。
- `schema.sql` 表頭註解（`:14-16,47-52`）同步；plan B1-D9 註記「標記＝INSERT 請求列」（D25 已說明）、進度表。
- 驗收（backend/）：`make sqlc`、`gofmt -l .`、`make lint-monolith`、`GOWORK=off go vet -tags integration ./...`、`python3 scripts/mutation/run.py --validate`、`go test ./internal/modules/media/... ./internal/platform/lockorder/...`（單元）；整合測試與 mutation 由 CI 跑，lockorder 違規數歸零為主要判準。

## 8. 待決（D25 未決定的真取捨；2026-11-23 全部決定）
1. **ApplyInterval**（已決：30s，可調常數 `reclaim.ApplyInterval`）：建議 30s（刪除延遲 vs 每 30s 一次空查詢，部分索引下成本可忽略）；替代 5s（更接近現在 commit 後立即 job）或沿用 5 分鐘（簡單但刪除延遲大）。
2. **claim／request 列保留**（已決：與暫存／資產列同生命週期，media 的物理刪除 job 在同一個 transaction 刪除，沒有獨立保留期；因此 §1.1 的 FK 前提不受影響，被刪的是子列）：原建議永久保留（每筆數十 bytes，是 D23 判斷與稽核紀錄；暫存列與資產列本來也保留作墓碑，`schema.sql:16`）；替代是資產 DELETED 後 N 天由 media job 刪——但會違反「claim 永不 DELETE」並需先處理 §1.1 的 FK 前提。
3. **settle 物化 vs 改 GC 索引**（已決：§3.7 物化）：建議 §3.7 物化；替代是把 `staged_uploads_gc` 索引改成不含 READY、GC 另走 claims 反查，省一個 UPDATE 但 GC 查詢較複雜。

## 9. 審查修正（2026-11-23，已實作）
- §3.4 GC 絕不等擁有者未提交的認領：候選用純 SELECT 列出，每列一個 transaction、`SET LOCAL lock_timeout`（`GCLockTimeout` 100ms），逾時（SQLSTATE 55P03）跳過、下一輪再試；擁有者不取 advisory lock。測試：`TestConfirmFieldsAndGCNeverDeadlockOrStall`。
- §3.2 物理 `DELETING` 的已確認資產一定有刪除請求：fixtures 改為經 `MarkDeleted` 造狀態；`TestOrphanDeletingInvariantHoldsAndTheCheckIsNotVacuous` 斷言並附反例。物理刪除完成（`DELETED`）時請求與認領列一併刪除（決定 2），所以不變量只能對 `DELETING` 檢查。
- 測試 helper：`media.AssetStateOf`／`CountAssetsInState`（根套件，因為其他模組的測試只能 import 根套件）經 `GetAssetView` 與 `DeriveAssetState`，不重寫謂詞。
