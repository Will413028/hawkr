# B1-0 介面盤點：Java media-service 改寫成 Go 單體的 media 模組（唯讀，快照 dev-based `b1-media` 分支 `17bb31c5`）

標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式或方法名推論。路徑縮寫：`B=backend`、`J=backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media`（下稱 `J/…`）、`JT=backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media`、`JR=backend/services/java/media-service/src/main/resources`、`M=backend/internal`、`G=backend/gateway/internal`。所有指令從 worktree 根目錄執行。
終態以 ADR `docs/adr/2026-10-03-media-references-owner-side.md` 的「決策」與文末「修訂」為準：media 不認識業務，只存資產與生命週期（暫存→確認→刪除）、用途與可見性；沒有 `asset_reference`、沒有 `(ref_type, ref_id)`；刪除是擁有者經 outbox 送出的明確指令；GC 只過期未認領的暫存上傳；機敏下載由擁有者端點代理（修訂：不是簽章網址）。
去向詞彙：保留／改寫成…／刪除＋理由；「待決」＝真的有取捨，說明放在 §0。方法名與新介面是我擬的【推】，不是已決定的契約。

## 0. 待決項（有取捨，需 Will 或設計簡報決定）

| # | 待決 | 取捨 | 我的傾向（簡報前提，不是決定） |
|---|---|---|---|
| 0.1 | 暫存→確認由誰、何時做 | 今天 `FinalizeImages` 在 DB transaction 之外轉正（`J/service/MediaAssetService.java:98`），資產一建立就存在、沒有引用，靠 7 天 GC 補（ADR 後果段）。終態說「GC 只過期未認領的暫存上傳」，所以「轉正」必須變成擁有者的明確確認：(A) 擁有者在自己的 transaction 內呼叫 media 的 `Confirm(tx, …)`（同一個 Postgres，沒有遠端，transaction 外呼叫的理由 `M/modules/media/AGENTS.md`「不變量」第三條已不成立），業務列與確認原子；(B) 維持兩步（先確認、後寫業務列）→ 孤兒確認資產永遠沒人回收，因為 GC 不再刪已確認資產 | A。B 會重新創造「確認了卻沒掛上」的洩漏，又沒有 GC |
| 0.2 | staged token 與上傳會話的形狀 | 今天是 HS JWT（`J/service/StagedTokenService.java:90`，claims=session、account、scope、exp）加 `upload_session` 表（狀態 INIT/COMPLETED/FINALIZED/EXPIRED）。單體內 token 不需要簽章也能是資料庫列的 ID：(A) 不透明 ID＋查表（撤銷與過期都在表），(B) 沿用 JWT。上線前沒有相容性約束 | A；JWT 只在「token 離開伺服器信任域又被帶回」才有價值，這裡有表就不需要第二份真相 |
| 0.3 | 資產的「用途」模型（取代 22 個 `ref_type`） | media 不能再持有業務類型清單。選項：(A) 只有 `visibility`（public／secure），object key 不含 category；(B) media 持有封閉的 `purpose` 列舉（又是業務知識）；(C) 擁有者傳自由字串 purpose。今天 final key 是 `{public|secure}/{category}/{ownerId}/{yyyymmdd}/{file}`（`J/storage/AbstractStorageService.java:43`），category 來自 ref_type 的小寫 code | A。用途資訊在擁有者的表本來就有；R2 沒有依 category 的生命週期規則（【推】，沒找到設定，見不確定性） |
| 0.4 | 機敏檔密文格式與讀取介面 | 今天單段 AES-256-GCM＋每檔金鑰以主密鑰包裝（`J/crypto/AesStreamCipher.java:75,117,147`），解密串流在讀到結尾才驗 tag，所以串流給 HTTP 時，被竄改的檔案已經送出明文（缺陷 5.6）。`Media.ReadSecure` 回 `[]byte`（`M/modules/media/internal/model/model.go`），100MB 上限全進記憶體。上線前沒有明文與舊密文的相容問題。選項：(A) 分塊 AEAD（STREAM 構造）＋`io.ReadCloser`，(B) 沿用單段 GCM、讀完驗 tag 再回（≤100MB 進記憶體）。新增依賴要先列名稱版本、核對滿兩週再同意 | A，但取決於能否只用標準庫（`crypto/cipher`）手寫 STREAM；否則先 B 並記為限制，由 Will 接受 |
| 0.5 | 上傳通道：預簽 PUT 或經單體 | 公開圖用預簽 PUT 直傳 R2，大小上限無法在 R2 端綁定，只能事後 HEAD 回查（`J/storage/R2StorageService.java:41-44` 註解）；MIME 只信客戶端宣告與 R2 回報的 Content-Type，沒有 magic-byte 檢查（缺陷 5.4）。機敏檔已經是經服務上傳（`PUT /secure_upload/{id}`）。選項：(A) 全部經單體串流（可先驗 magic bytes、限長度、直接剝除寫入最終位置，省掉 tmp→final 複製與刪除），(B) 保留預簽 PUT＋finalize 時回查 | A。代價是上傳流量過單體；10MB 圖的流量在作品集規模可忽略（【推】，沒量） |
| 0.6 | 客戶端契約：REST `/api/v1/app/media/*` 或 Connect | 今天 gateway 代理 `upload_sessions`、`upload_sessions/{id}/complete`、`secure_upload`、`secure_preview`、`secure_download`（§2.2），四個 client 共 19 個檔直接打這些路徑（§2.3）。ADR `2026-09-26-external-contract-protobuf-connect` 規定對外契約是 Protobuf＋Connect。改契約要同一次變更更新全部消費端（AGENTS.md「跨專案契約」）。選項：(A) 新增 `api/` 的 media 服務（Connect）並一次改四個 client；(B) 單體先提供同路徑的 REST 轉接，之後再換 | A，但可拆子步驟（單體端先落、client 逐一換）；是否拆由簡報決定 |
| 0.7 | （已由計畫 B1-D7 取代，2026-11-21）Java 呼叫端還在（product、order、aftersales、shipping）時怎麼退場 Java media | 這四個 Java 服務仍呼叫 media-service 的 `FinalizeImages/LinkImages/UnlinkImages/ReleaseReferences/ServerUpload*`（§2.1），它們各自在 B3 fulfillment（shipping）、B4 catalog（product）、B8 ordering（order）、B9 aftersales（`docs/plans/2026-09-24-go-modular-monolith.md:293-325`）。Java media 的 DB 與 Go media 的 DB 是兩份，asset ID 不通用。選項：(A) B1 只搬 Go 擁有者模組＋建立 Go media；Java media-service 繼續服務這四個 Java 服務直到它們遷移（兩套並存，資產不互通）；(B) 單體加一個舊契約的 gRPC 轉接面給 Java 呼叫端（要實作 Link/Unlink/Release 的語意，與終態矛盾）；(C) 把 B1 排在 Java 呼叫端遷移之後 | A，並明寫 Java media 的退場條件＝最後一個 Java 呼叫端遷出 |
| 0.8 | 資產 ID 方案 | 今天 TSID（BIGINT，序列化成數字字串，node-id 100-199，`JR/application.yml` 的 `hawkr.tsid`），擁有者表存 TEXT。「沿用也是決策」：從零設計會用 UUID（v7）。上線前沒有相容性約束 | UUID v7；擁有者欄位仍是 TEXT 或 uuid，由各模組 migration 決定（影響 7 個模組的 `*_asset_id` 欄位與 sqlc） |
| 0.9 | 刪除指令的傳輸：outbox 事件或 transaction 內呼叫 | ADR 寫 outbox。媒體與擁有者同一個 Postgres，in-tx 的 `MarkDeleted(tx, assetID)` 與 outbox 的差別是：in-tx 沒有「刪除指令遺失」的視窗、也不需要 media 認識事件；outbox 讓 media 可以獨立部署。單體內沒有獨立部署的計畫 | in-tx 標記刪除（與 0.1 同一個 `Tx` 介面），實體刪 R2 物件由 media 自己的 job 做；若維持 ADR 字面的 outbox，需說明哪個條件使 in-tx 不足 |

## 0.A 數字與指令

見 §6「規模」。重點：RPC 10 個（0 個 RabbitMQ 事件）、REST 端點 4 個（含 `/health`）、表 3 個（`upload_session`、`media_asset`、`asset_reference`；`import_asset` 在 V10 已刪）、Java 測試 179 個且**全是單元測試，沒有資料庫或 Testcontainers**（`JT/repository/MediaAssetGcExclusionTest.java:24` 自己寫明），所以「integration 測試對照表」的 Go 對應要補真 DB 的 characterization，不是 1:1 翻譯。

## 1. Java media-service（`backend/services/java/media-service`）

### 1.1 gRPC RPC（proto：`B/api/proto/media/media.proto`，與 `B/services/java/shared/java-proto/src/main/proto/media/media.proto` 逐字相同【量：diff 無輸出】；實作 `J/grpc/MediaGrpcService.java`）

回應都是 200 加 body 內的 `code`（0＝成功，其餘 CodeMeta 碼），不用 gRPC status（`MediaGrpcService.java:852-862`）。除 §1.1.A 標示外，沒有任何 RPC 在服務內檢查呼叫端是否內部服務（Link、Unlink、Release、ServerUploadAndFinalize、ServerUploadSecureAndFinalize 靠網路與 `INTERNAL_AUTH_MODE` 攔截器，見 §1.7 與缺陷 5.9）。

| # | RPC | 行為（來源） | 呼叫端（§2） | 去向 |
|---|---|---|---|---|
| 1 | `CreateUploadSession` | 驗宣告（MIME 白名單、大小上限，`MediaUploadValidator.java:51`），生成 tmp／secure object key，寫 `upload_session`（TTL 30 分），公開上傳回預簽 PUT URL，機敏上傳不回 URL（`UploadSessionService.java:51`；gRPC 層 `MediaGrpcService.java:375`；需 BUYER/SELLER/ADMIN 與 account id，`:384`） | gateway `handler/media.go:73`、`admin_delegated_media.go:93` | 改寫成 Go 的 `CreateUpload`（由 0.5、0.6 決定是預簽 PUT 或串流上傳、Connect 或 REST）。保留：宣告驗證、30 分 TTL、僅上傳者可續用 |
| 2 | `CompleteUploadSession` | INIT→COMPLETED（CAS），機敏檔不可走此流程，回 staged token 與預簽 GET 預覽 URL（`UploadSessionService.java:117`；`MediaGrpcService.java:470`）。不檢查物件是否真的上傳（要到 Finalize 才 HEAD） | gateway `media.go:139`、`admin_delegated_media.go:133` | 改寫：0.5 選 A 時與 CreateUpload 合併成一次串流上傳（物件存在才有 token）；選 B 時保留並補「HEAD 確認存在」 |
| 3 | `FinalizeImages` | 驗 token、帳號與 session 一致、狀態 COMPLETED、未過期；HEAD 回查大小／MIME；copy tmp→final（public 路徑剝除 metadata）；單一 tx 內 CAS COMPLETED→FINALIZED、建 `media_asset`；之後刪 tmp；失敗補償刪已複製物件；FINALIZED 重入回同一資產（`MediaAssetService.java:98,166,243,264`；`MediaGrpcService.java:57`）。空 ref_type 退成 `OTHER`（`:79`） | 9 處 Java／Go 呼叫端，見 §2.1 | 改寫成 `Confirm`（0.1）：保留驗證順序、冪等重入、補償；刪除 ref_type 參數（0.3） |
| 4 | `LinkImages` | `INSERT … ON CONFLICT DO NOTHING` 進 `asset_reference`；`ref_type` 要是 `isValidValue`；資產須存在（`AssetReferenceService.java:44`；`MediaGrpcService.java:142`） | 同上 | 刪除：引用只在擁有者表（ADR 決策）。Go 擁有者端的 `media_sync` job 與 `media_ref_sync`／`retired_assets` 一併刪（§3.2） |
| 5 | `UnlinkImages` | 精確刪 `(asset, ref_type, ref_id)`（`AssetReferenceService.java:98`；`MediaGrpcService.java:222`） | product、shipping、seller（legacy）、Go 單體 | 刪除，同上；被換掉的資產改由擁有者在 tx 內發 `MarkDeleted`（0.9） |
| 6 | `ReleaseReferences` | 依 `(ref_type, ref_ids)` 批次刪引用，`keep_asset_ids` 保留名單（`AssetReferenceService.java:210`；`MediaGrpcService.java:302`） | product、seller／im／ai／buyer 等 legacy | 刪除；「帳號刪除」語意改成擁有者對自己列出的資產發刪除 |
| 7 | `ServerUploadAndFinalize` | 服務間直傳公開檔：key `public/{category}/{yyyymmdd}/{uuid}/{filename}`（filename **未消毒**，`MediaAssetService.java:434`）、直接建 `media_asset`、不建引用、**不驗 MIME／大小**（`:429-470`；gRPC `MediaGrpcService.java:550`）。上傳者欄位為 NULL | shipping `ShipmentLabelService.java:95,187`（託運單 PDF，ref SHIPPING） | 改寫成 `PutPublic`（服務內部 port）；補驗證與 filename 消毒；B3 fulfillment 才有呼叫端，B1 可只保留 port 不接線（待簡報確認，屬 0.7 的一部分）。注意：公開 PDF 與「公開圖片限 image/*」規則衝突（`MediaUploadValidator.java:22`） |
| 8 | `ServerUploadSecureAndFinalize` | 服務間直傳機敏檔：key `secure/…`、AES-GCM 加密、同一 tx 建資產並建引用；ref_type／ref_id 必填；無主密鑰拒絕寫明文（`MediaAssetService.java:498-570`；`MediaGrpcService.java:641`） | shipping `MediaUberPodStorage.java:53`；Go invoice `client/media/client.go:206` | 改寫成 `PutSecure`：保留「無主密鑰就不寫」；刪除建引用（擁有者自己存 asset id） |
| 9 | `ServerReadSecureAsset` | **唯一**檢查 `isInternalCall()` 的 RPC（`MediaGrpcService.java:723`）；三元組 `(object_key, ref_type, ref_id)` 全等才讀，查無與越權回同一訊息；整檔進記憶體、驗解密長度（`MediaAssetService.java:608,710`） | Go `legacy/mediaservice/mediaservice.go:124`（seller KYC、governance evidence）、invoice `client/media/client.go:263` | 改寫成 `ReadSecure(ctx, assetID) → io.ReadCloser`（0.4）：三元組述詞刪除（ADR）；呼叫方先授權並記存取紀錄的規則（ADR 修訂）保留。三元組在 owner 側換成「owner 的列指著這個 asset」 |
| 10 | `BatchGetAssetMetadata` | 依 object_key 批次（上限 100）回 asset_id／檔名／大小／時間／MIME；授權同 #9，查無與越權不出現（`MediaAssetService.java:656`；`MediaGrpcService.java:777`，內部閘門 `:783`） | invoice `client/media/client.go:318` | 改寫：擁有者以 asset id 查 metadata；授權述詞刪除；object_key→asset_id 反查若仍要則保留在擁有者側（擁有者自己存了 key） |

另有 proto 沒有、Java 也沒有的 RPC：`ImportDownloadAsset`、`FinalizeImportAsset`、`BindStagedAsset` 在 `rpc-servers.csv` 標 removed，proto 與 Java 已無（`rg "rpc " api/proto/media/media.proto` 只有 10 個【量】）。

### 1.2 REST 端點（`J/controller`）

| # | 端點 | 行為 | 去向 |
|---|---|---|---|
| 1 | `PUT /app/media/secure_upload/{id}`（`SecureMediaController.java:78`） | 需 account id／type headers；session 屬於呼叫者、為 secure、狀態 INIT、未過期；每次嘗試用獨立 key `{key}.attempt-{uuid}`、串流加密寫 R2；CAS INIT→COMPLETED 並把新 key 與包裝金鑰寫回；落敗者刪自己的物件；回 staged token 與預覽路徑。gateway 另有 20MiB body 上限（`G/middleware/body_guard.go:14,53`）而 Java multipart 設 100MB（`application.yml`） | 改寫：保留「每次嘗試獨立 key＋CAS＋落敗補償」；上限單一來源化（缺陷 5.8） |
| 2 | `GET /app/media/secure_preview/{session_id}`（`:202`） | 上傳者在 COMPLETED 且未過期時串流解密預覽，inline disposition | 改寫：上傳預覽改成擁有者尚未確認前，只給上傳者；放在 media 的「暫存預覽」端點（若 0.5 選 A 可改成直接回縮圖 URL） |
| 3 | `GET /app/media/secure_download/{asset_id}`（`:284`） | 授權＝`uploader == account` 或「該資產有引用且上傳者是 account」，後者被前者蓋過、等於只有上傳者（缺陷 5.5）；串流解密、attachment disposition | **刪除**：ADR 修訂規定機敏下載由擁有者端點代理；gateway `routes_media.go` 的對應路由同刪。客戶端依賴見 §2.3（website `lib/invoices/seller-repo.ts:439,1068`、seller-app `media_repository.dart:321`） |
| 4 | `GET /health`（`J/controller/HealthController.java:15`） | 存活探測（另有 actuator；compose healthcheck 用 `http://localhost:9936/actuator/health`，`docker-compose-local.yml:1142`） | 刪除（單體有自己的健康檢查）【推】 |

### 1.3 資料表與 Flyway（`JR/db/migration`，共 10 檔、525 行【量】）

| 檔 | 內容 | 去向 |
|---|---|---|
| `V1__init_schema.sql:7,28,47` | 建 `upload_session`（`account_id`、`object_key`、`is_secure`、`content_type`、`file_size`、`encryption_key`、`asset_id`、`status` 7001-7004、`expires_at`；索引 `(status, expires_at)`、`account_id`）、`media_asset`（`id` BIGINT TSID、`account_id`、`object_key`、`encryption_key`、`ref_type`、`url`、`mime_type`、`size_bytes`、`is_secure`）、`asset_reference`（`asset_id` FK、`ref_type`、`ref_id`、UNIQUE 三元組、兩個索引） | 改寫：`upload_session`→媒體的暫存表（狀態集合與 0.2 一起重定）；`media_asset`→資產表（去 `ref_type`）；`asset_reference` **刪除** |
| `V2`／`V3`／`V5` | `import_asset`（商品匯入的委派下載，含 source_type 與 staged_session_id 等） | 刪除：V10 已 drop；B5 已整個移除 |
| `V4__drop_media_asset_url.sql` | 移除 `media_asset.url`（URL 讀取時組裝） | 保留此原則（只存 object key） |
| `V6__split_media_asset_account_id.sql` | `account_id` 拆成 `uploader_account_id` 與 `import_store_id` | 改寫：只留 `uploader`（帳號或管理員要帶種類，`Uploader{ID, Admin}`，`M/modules/media/internal/model/model.go`）；`import_store_id` 已由 V10 刪 |
| `V7__unique_media_asset_object_key.sql` | 依 object_key 去重並搬引用，加唯一索引 | 改寫：Go 的 schema 直接建 `UNIQUE(object_key)`；去重 migration 刪除（無舊資料，上線前） |
| `V8__add_media_gc_lease_columns.sql` | `upload_session` 與 `media_asset` 加 `gc_lease_until`＋索引 | 改寫：保留 `upload_session` 的租約（過期暫存 GC 要防多實例重複）；`media_asset` 的租約刪除（不再 GC 資產）。單體若只有一個實例，也可改 `SELECT … FOR UPDATE SKIP LOCKED`（較簡單，待簡報） |
| `V9__add_original_filename.sql` | `upload_session.original_filename`（後來 `media_asset` 也有，見 `MediaAssetEntity`） | 保留 |
| `V10__drop_import_asset.sql` | drop `import_asset`、`media_asset.import_store_id` | 保留結論（無此表） |

Flyway 用 `baseline-on-migrate: true`、`baseline-version: 0`（`application.yml`）；Go 端改 atlas／sqlc（`make sqlc`、`atlas migrate diff`，本機 Docker 一次性容器可跑）。Java 的 `hawkr_media` 資料庫整個放棄（上線前，Will 2026-10-03 起開發資料庫可重灌，ADR 後果段）。

### 1.4 RabbitMQ 與其他非同步

| 項 | 結果 |
|---|---|
| 發布／消費的事件 | **0**。`rg -in "rabbit\|amqp" backend/services/java/media-service` 無輸出【量】；`pom.xml` 沒有 amqp 依賴【量】。`docs/plans/2026-09-24-go-modular-monolith/events.csv` 還列了 `import.asset.download.internal`、`import.asset.downloaded`、`gc.expire_job`（media 側的匯入事件），那是商品匯入移除（B5）前的過期列，Java 已沒有 → CSV 的這三列應在 B1 同步標 removed |
| 被動接收 | `account.deleted` 等事件由擁有者模組消費後呼叫 `ReleaseReferences`，media 本身不訂閱 |
| 去向 | 刪除（沒有東西可搬）；終態新增的「刪除指令」由 0.9 決定（outbox 或 in-tx） |

### 1.5 排程工作（`J/scheduler/MediaGarbageCollectionJob.java`）

單一 `@Scheduled(cron = "0 0 3 * * ?")`（`:141`），`GC_ENABLED` 可關，三階段依序各在自己的 tx（`runGarbageCollection`，`:141-176`）：

| # | 階段 | 行為 | 去向 |
|---|---|---|---|
| 1 | `cleanExpiredSessions`（`:178`） | 狀態 INIT／COMPLETED 且過期、租約可搶（60 分鐘，`GC_LEASE_MINUTES`，`:85`）→ 先在獨立 tx 搶租約 → 刪 R2 物件 → 標 EXPIRED；R2 失敗保留列待下輪 | 保留（終態 GC 唯一職責）；改寫成 River periodic job（單體慣例，`M/modules/*/internal/mediaref` 已用 `river.PeriodicJobOpts`）；cron 寫在設定而非常數 |
| 2 | `cleanOrphanedAssets`（`:238`） | 無引用且建立滿 7 天的資產：搶租約→刪 R2→刪列；R2 失敗就提前結束本輪（`:292`）；**排除 `ref_type IN ('7120','7111')`**（發票 PDF、商家認證文件的法定保存，`MediaAssetRepository.java:38`） | 刪除：終態不 GC 已確認資產；法定排除清單隨 `ref_type` 一起消失，保存義務由擁有者「不發刪除指令」承擔，需有擁有者側的測試守（seller 的 certification、invoice） |
| 3 | `cleanStaleReferences`（`:318`） | 刪指向不存在資產的 `asset_reference` | 刪除（沒有此表） |
| — | 新增 | 終態另需：消費擁有者刪除指令後刪 R2 物件與列（重試、冪等） | 新增（0.9） |

`triggerManualGc`（`:329`）沒有任何呼叫端，連測試都沒有（`rg -c triggerManualGc backend/services/java` 只有定義一處）【量，見缺陷 5.11】。

### 1.6 R2／S3 儲存與圖片處理

| 項 | 行為（來源） | 去向 |
|---|---|---|
| 後端 | 唯一實作 R2（S3 相容，AWS SDK v2，region `auto`）；`STORAGE_BACKEND` 非 `r2` 啟動即失敗（`J/storage/R2StorageService.java:48-52` 條件裝配，`StorageBackendConfigTest`）。未設定 R2 時各方法回模擬值（`:126-135,172-176,241-250`，缺陷 5.12） | 改寫：Go 用 S3 client；未設定時應拒絕啟動而非靜默跳過 |
| Key 佈局 | tmp：`tmp/{yyyymmdd}/{uuid}{ext}`；secure：`secure/{yyyymmdd}/{uuid}{ext}`；final：`{public\|secure}/{category}/{ownerId}/{yyyymmdd}/{filename}`（`AbstractStorageService.java:32,43`）；副檔名含 `/`、`\`、`..` 拒絕（`:75`） | 改寫（0.3）：保留副檔名消毒；去 category |
| 預簽 URL | PUT（綁 Content-Type，無法綁大小）、GET（預覽，TTL `MEDIA_PREVIEW_TTL` 預設 3600）（`R2StorageService.java:126,151`） | 待決 0.5 |
| 公開圖 metadata 剝除 | 白名單逐段搬運、不重新編碼：JPEG（保留 Orientation，重寫最小 EXIF）、PNG、WebP、GIF；無法辨認格式回 null 原樣處理；辨認得出但壞掉拋例外並拒絕寫入（`J/storage/ImageMetadataStripper.java:41-401`，469 行；呼叫點 `R2StorageService.java:180,314,336`）。所有寫入 `public/` 的路徑（轉正 copy、後端直傳）都經過 | **保留並搬進 Go**（`M/modules/media/AGENTS.md`「不負責」與計畫 B1 第二條都指定要搬）；測試 9 個（§4）一併搬，Go 版要有同樣的損毀拒絕。注意缺陷 5.4（無法辨認回 null 即原樣寫入） |
| 縮放／縮圖 | **沒有**。`rg -ic "resize\|BufferedImage\|ImageIO\|thumbnail" J` 只有 `MediaUploadValidator.java` 兩處（為註解文字）【量】；尺寸變體由 Cloudflare Image Gateway 依 `pkg/mediaurl/url.go:26` `BuildImageURL(base, preset, objectKey)` 組 URL（設計書 49） | 不適用（不搬）；`BuildImageURL` 是 Go 現有、保留 |
| 內容檢查 | 宣告階段：MIME 白名單 `image/jpeg|png|webp|gif`，PDF 僅限機敏；大小 `MEDIA_MAX_IMAGE_SIZE_BYTES`=10MiB、`MEDIA_MAX_SECURE_SIZE_BYTES`=100MiB（`MediaUploadValidator.java:51,31-42`）。物件階段：HEAD 大小≤上限、等於宣告（機敏加 16 bytes tag）、公開檔的實際 Content-Type 在白名單且等於宣告（`:77-135`） | 保留規則；magic-byte 檢查是新增（0.5、缺陷 5.4） |
| 加密 | AES-256-GCM，每檔 32B key＋12B IV，以主密鑰 AES-GCM 包裝成 Base64 存在 `encryption_key`；密文＝明文＋16B tag（`J/crypto/AesStreamCipher.java:18-30,75`）；主密鑰 `ENCRYPTION_MASTER_KEY` 缺則 `isAvailable()=false`（`:168`）。**沒有主密鑰輪替**（缺陷 5.7） | 待決 0.4 |
| 顯示檔名 | `AssetFilename.sanitize`（basename、去 CR/LF/NUL/引號、255 字上限含副檔名）、`resolveDisplayName`（優先原始檔名，舊資產回退 key 尾段並剝 `.attempt-{uuid}`）（`J/util/AssetFilename.java:83,106`，186 行） | 保留並搬（測試 32 個） |

### 1.7 設定與環境變數（`JR/application.yml`；compose：`B/docker-compose-local.yml:1082` 起）

| 變數／鍵 | 預設 | 用途 | 去向 |
|---|---|---|---|
| `SERVER_PORT`／`GRPC_PORT` | 9936／9937 | HTTP 與 gRPC | 刪除（單體單埠） |
| `GRPC_MAX_INBOUND_MSG_SIZE` | 10MiB | 後端直傳 PDF 上限 | 刪除；直傳大小在 `PutPublic/PutSecure` 驗 |
| `NACOS_*`（6 個） | `NACOS_ENABLED=false` | 服務發現 | 刪除 |
| `DB_HOST/DB_POOL_PORT/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD/DB_SSLMODE` | `hawkr_media` | Java 專用資料庫 | 刪除（改單體 DB 的 `media` schema） |
| `FLYWAY_ENABLED` | true | migration | 刪除 |
| `R2_ACCOUNT_ID`／`R2_ACCESS_KEY_ID`／`R2_SECRET_ACCESS_KEY`／`R2_BUCKET_NAME` | 空 | 儲存 | 保留（單體設定；金鑰不進版控） |
| `TOKEN_SECRET`／`TOKEN_ALLOW_EPHEMERAL` | 空／false | staged token 簽章，缺則啟動失敗 | 待決 0.2（選 A 則刪） |
| `ENCRYPTION_MASTER_KEY` | 空 | 機敏檔主密鑰 | 保留（0.4） |
| `MEDIA_PREVIEW_TTL` | 3600 | 預覽 URL 秒數 | 保留 |
| `MEDIA_MAX_IMAGE_SIZE_BYTES`／`MEDIA_MAX_SECURE_SIZE_BYTES` | 10MiB／100MiB | 上傳上限 | 保留（單一來源，缺陷 5.8） |
| `media.upload.session-ttl-minutes` | 30（無環境變數） | 上傳會話 TTL | 保留（改成設定） |
| `GC_ENABLED`；`media.gc.cron`、`orphan-asset-days`=7、`batch-size`=100（無環境變數） | true | GC | 改寫：只留過期暫存的 cron、批次大小；`orphan-asset-days` 刪除 |
| `SELLER_SERVICE_HOST`／`SELLER_SERVICE_GRPC_PORT` | 127.0.0.1／9941 | `SellerGrpcClient`（呼叫 `ResolvePrincipals`，`J/client/SellerGrpcClient.java:69`） | 刪除：**沒有任何呼叫端**（缺陷 5.10） |
| `ACCESS_LOG_SAMPLE_RATE`、`POD_INDEX`、`LOG_LEVEL` | — | 觀測 | 刪除（單體的 observability） |
| `INTERNAL_AUTH_MODE`、`INTERNAL_SERVICE_HMAC_SECRET(_PREV)`（compose `docker-compose-local.yml:1094-1096`，`.env.template:227-231`） | `permissive` | 服務間 HMAC 簽章，`GrpcAuthContext.isInternalCall()` 讀攔截器判定（`common-core/.../GrpcAuthContext.java:101`） | 刪除（單體內是函式呼叫）；Go 舊 client 的 `grpcclient.Signer` 隨 `legacy/mediaservice` 刪 |
| `MEDIA_BASE_URL` | 必填 | 公開 URL 網域（`common-core MediaUrlAutoConfiguration`） | 保留；Go 端 `pkg/mediaurl` 已有 |

## 2. 呼叫端（所有語言）

### 2.1 服務端呼叫端

指令：`rg -l "MediaGrpcClient|mediaGrpcClient" backend/services/java --glob '*.java' --glob '!**/media-service/**' --glob '!**/src/test/**'`＝**13 檔**【量】；`ls backend/services/*/internal/client/media/client.go`＝**5 檔**；`rg -l 'modules/media"' --glob '*.go' --glob '!*_test.go' backend/internal backend/cmd | grep -v '^backend/internal/modules/media/'`＝**43 個非測試檔**，測試檔 **48**；`B/gateway/internal/grpc/media_client.go`（119 行）與 `B/pkg/media/client.go`（178 行）各 1。

| # | 呼叫端 | 用到的 RPC（來源 file:line） | ref_type | 去向 |
|---|---|---|---|---|
| 1 | Java product-service | Finalize（`ProductService.java:1911,1953,2464,2519`）、Link（`:2554,2565,2587,2622`；`ProductMediaReferenceLinker.java:75,86`）、Unlink（`:2596,2631`）、Release（`:2648,2653`）；client `grpc/client/MediaGrpcClient.java:69-182` | PRODUCT、PRODUCT_DESC | 不動（0.7：B4 catalog 才遷）；Java media 要活到此時 |
| 2 | Java order-service | Finalize、Link（`OrderGrpcService.java:694,729`；client `MediaGrpcClient.java:54,75`） | ORDER_RECEIPT | 不動（B8） |
| 3 | Java aftersales-service | Finalize、Link（`EvidenceService.java:43,52`；client `MediaGrpcClient.java:52,68`） | AFTERSALE | 不動（B9） |
| 4 | Java shipping-service | ServerUploadAndFinalize（`ShipmentLabelService.java:95,187`）、Unlink（`:120`）、Link（`ShippingService.java:847,1797`）、Finalize（`:1782`）、ServerUploadSecureAndFinalize（`MediaUberPodStorage.java:53`） | SHIPPING、DELIVERY_POD | 不動（B3） |
| 5 | Go legacy seller-service | Finalize／Link／Release（`services/seller-service/internal/client/media/client.go:65,89,96`），用於 `usecase/seller.go`、`certification.go`、`violation*.go`、`seller_prospect_media.go`、`mq/account_deleted_consumer.go` | SELLER_LOGO／COVER、CERTIFICATION、GOVERNANCE_EVIDENCE、PROSPECT_* | 單體的 seller 模組已搬（§2.2）；此目錄是公司時期遺留，B1 不改，待該服務目錄整個刪除時一併消失【推：未查 seller-service 是否仍被 compose 啟動】 |
| 6 | Go legacy invoice-service | FinalizeEvidence／ReadUnlockEvidence／ReleaseEvidence／UploadSecure／ReadSecure／ReadTaxEvidence／BatchGetEvidenceMetadata（`services/invoice-service/internal/client/media/client.go:99-318`，352 行，含 `ServerReadSecureAsset`、`BatchGetAssetMetadata`，`ServerUploadSecureAndFinalize`） | CERTIFICATION（稅籍佐證，`:36`）、INVOICE_DOCUMENT（`:189`） | **B1 之後仍需要 media 的機敏讀寫**：invoice 是 B10 finance；列入 0.7 |
| 7 | Go legacy im／ai／review | `FinalizeAndLink`、`ReleaseReferences`（`im-service/.../client.go:35,42`；`ai-service/.../client.go:47,54`；`review-service/.../client.go:33`），經 `pkg/media/client.go:44,98,160` | CHAT_MESSAGE、AI_STYLE、ORDER_REVIEW | im、review 已搬進單體；ai 是 `imageai`（`docs/modules/imageai.md`）未搬。同 #5 |
| 8 | gateway（Go） | `CreateUploadSession`、`CompleteUploadSession`（`G/handler/media.go:73,139`、`admin_delegated_media.go:93,133`）；`media_client.go:81,94,108` 的 `FinalizeImages`／`LinkImages`／`UnlinkImages` **沒有呼叫端**（`rg "\.(FinalizeImages\|LinkImages\|UnlinkImages)\(" backend/gateway/internal --glob '!*_test.go'` 只有 client 自己的定義【量】） | — | 上傳會話兩個 RPC：隨 0.6 決定；其餘三個方法刪除（死碼） |
| 9 | Go 單體（`M/`） | 透過 `media.Media` port：Finalize 9 處（review `internal/write/write.go:115`、im `internal/write/send.go:55`、seller `internal/profile/profile.go:423`、`internal/mediaref/mediaref.go:307`、`internal/kyc/documents.go:90`、`internal/prospect/images.go:108`、buyer `internal/profile/profile.go:305`、social `internal/post/post.go:133`、content `internal/images/images.go:63`）；Link／Unlink 只在 `internal/modules/media/internal/refsync/refsync.go:193,198`；ReadSecure 在 seller `internal/mediaref/mediaref.go:271`（經 `internal/governance/evidence.go:59`、`internal/kyc/admin.go:486`）；`httpapi/seller/files.go` 是端點代理。裝配在 `cmd/hawkr/main.go:132,182,339,384`、`seller_commands.go:68`（`MEDIA_SERVICE_ADDR`＋HMAC secret） | §3 的 12 種 | 改寫：port 換成單體內的 media（`internal/legacy/mediaservice`＋`internal/legacy/gen/media` 刪除）；9 個 Finalize 呼叫改成 `Confirm`（0.1）；Link／Unlink 呼叫與 `refsync` 整個刪除 |

### 2.2 gateway 路由與 openapi

指令：`rg -n "/media/|secure_upload|secure_download|secure_preview|upload_sessions" backend/gateway/internal --glob '*.go' | grep -v _test`；`rg -n "^  /api/v1/.*media" backend/api/openapi/openapi.yaml`。

| # | 路由／項 | 來源 | 去向 |
|---|---|---|---|
| 1 | `POST /api/v1/app/media/upload_sessions`、`/{id}/complete`（JWT） | `G/router/routes_media.go`（`constants.go:178-179`）；openapi `openapi.yaml:13291,13339`（**僅這兩條在 openapi**） | 改寫（0.6） |
| 2 | `PUT /secure_upload/:id`、`GET /secure_preview/:session_id`、`GET /secure_download/:asset_id`（HTTP 反代到 media，`FailClientFacing`） | `routes_media.go`、`constants.go:181-183`；body 上限 `middleware/body_guard.go:14,53`；**不在 openapi**，只在 `router/openapi_coverage_test.go:62-66` 的例外表（binaryProxy） | upload／preview：改寫（0.5、0.6）；download：刪除（§1.2 #3），含 `admin_secure_media_removed_test.go`（斷言後台沒有通用機敏下載，保留此不變量） |
| 3 | 代行營運：`POST /admin/delegated-operations/media/upload_sessions`（及 `:session_id/complete`） | `G/router/routes_delegated.go:85-86`、`seller_route_assert.go:97-98`、`handler/admin_delegated_media.go:64,115` | 改寫（0.6）；website 的 BFF 轉接在 `website/app/(delegated)/adm/api/media/upload_sessions/**` |
| 4 | 發票 PDF 檔案面 | `routes_invoice.go:193-199`（註解說明 media 的 secure_download 授權＝上傳者本人，所以發票用 invoice 自己的串流面） | 不屬 B1，但是 ADR 修訂方向（擁有者代理）的既有範例 |

### 2.3 Client 端（website／admin／buyer-app／seller-app／packages）

指令：`rg -l "upload_sessions|secure_upload|secure_download|secure_preview" website admin seller-app buyer-app --glob '!**/node_modules/**'`＝website 12、admin 3、seller-app 3、buyer-app 1（含測試）；`rg -c "stagedToken|staged_token" website admin seller-app buyer-app packages --glob '!**/node_modules/**'` 檔數與命中：website 65 檔／266、admin 9／44、seller-app 33／186、buyer-app 16／64、packages 21／69（含 `packages/hawkr-api-ts`、`hawkr_api` 的生成物）【量】。`rg -n ... packages` 對上傳路徑**沒有**命中（packages 只帶 staged token 欄位的契約，沒有上傳流程）。

| # | Client | 流程（file:line） | 去向 |
|---|---|---|---|
| 1 | website 公開圖 | `website/lib/media/upload.ts:3-82`（三步：POST upload_sessions→PUT 預簽 URL→POST complete；賣家 root `/app/media/upload_sessions`，代行 root `delegated-repos.ts:37` `/media/upload_sessions`） | 改寫（0.5、0.6） |
| 2 | website 機敏檔 | `website/lib/media/secure-upload.ts:66,86`（POST 建會話 isSecure＝true→PUT `secure_upload/{id}`，直接得 stagedToken） | 改寫 |
| 3 | website 發票／證件下載 | `website/lib/invoices/seller-repo.ts:439,1068` 組 `/api/v1/app/media/secure_download/{assetId}`；`seller-types.ts:113` 註解 | **依賴待遷**：secure_download 刪除後要改成擁有者端點（0.6、§1.2 #3） |
| 4 | admin | `admin/src/services/media-upload.ts:24,57,95`（upload_sessions、secure_upload、complete）；測試 `admin/tests/media-upload.test.ts`、`invoice-unlock-evidence-secure-upload.test.ts` | 改寫 |
| 5 | seller-app | `seller-app/lib/features/media/data/media_repository.dart:100,155,283,321,326`（含組 secure_download、secure_preview URL）；`private_document_uploader.dart` 註解稱「走 legacy media 的 secure_upload」 | 改寫；`:321,:326` 兩個 URL 組裝隨端點刪除 |
| 6 | buyer-app | `buyer-app/lib/core/networking/oss_service.dart:74,92` | 改寫 |
| 7 | 圖片顯示 URL | 讀取時由 object key＋preset 組成（`B/pkg/mediaurl/url.go:26`）；client 不依賴 media-service | 保留（0.3 去 category 不影響 URL 組裝：URL 只吃 object key） |

## 3. `ref_type`／`AssetReferenceType`

### 3.1 值與擁有者

唯一來源：CodeMeta `backend/docs/代碼管理（CodeMeta）/業務類型碼.md`（`generate.sh --go-media` 生成 `M/modules/media/ref_type.go`，**22 個值**；`pkg/media/ref_type.go` 也是 22 個；Java `common-core/.../AssetReferenceType.java` 22 個【量：`rg -c` 三處皆 22】）。另手寫鏡像：`M/modules/media/finalize_args.go:13-17` 的封閉集合（`TestRefTypesMatchTheGeneratedConstants` 守）。**一共 5 個地方要同步刪**（CodeMeta 文件、`ref_type.go`、`pkg/media/ref_type.go`、Java enum、`finalize_args.go`）。

| 碼 | 名稱 | 擁有者（現況） | 在單體 | 去向 |
|---|---|---|---|---|
| 7101 | PRODUCT | Java product-service | 否（B4） | 單體 media 不認識；Java media 繼續 |
| 7102 | PRODUCT_DESC | Java product-service | 否（B4） | 同上 |
| 7103 | SELLER_LOGO | seller 模組（`seller/internal/mediaref`） | 是 | 刪（擁有者表存 asset id） |
| 7104 | SHIPPING | Java shipping | 否（B3） | 同 7101 |
| 7105 | SELLER_COVER | seller | 是 | 刪 |
| 7106 | IDENTITY_VERIFICATION | 無使用者（`rg` 除 `finalize_args.go` 與 legacy 測試外無命中【量】） | — | 刪（死值） |
| 7107 | CHAT_MESSAGE | im（`im/internal/mediaref`） | 是 | 刪 |
| 7108 | POST | social（`social/internal/mediaref`） | 是 | 刪 |
| 7109 | BANNER | content（`content/internal/mediaref`） | 是 | 刪 |
| 7110 | AFTERSALE | Java aftersales | 否（B9） | 同 7101 |
| 7111 | CERTIFICATION | seller（KYC，`mediaref.go:61`）＋legacy invoice 稅籍佐證 | 是（seller） | 刪；**法定保存**排除（§1.5 #2）改由 owner 測試守 |
| 7112 | BUYER_AVATAR | buyer（`buyer/internal/mediaref`） | 是 | 刪 |
| 7113 | ORDER_RECEIPT | Java order | 否（B8） | 同 7101 |
| 7114 | AI_STYLE | legacy ai-service | 否 | 同 7101 |
| 7115 | ORDER_REVIEW | review（`review/internal/mediaref`） | 是 | 刪 |
| 7118 | GOVERNANCE_EVIDENCE | seller（`mediaref.go:66`）與 moderation（`moderation/internal/mediaref`） | 是 | 刪 |
| 7119 | RECIPE | 無（content 食譜已刪，`rg RECIPE\|RefTypeRecipe` 只剩生成物與 `finalize_args.go:15`【量】） | — | 刪（計畫 B1 延後項指定） |
| 7120 | INVOICE_DOCUMENT | legacy invoice（`client.go:189`） | 否（B10） | 同 7101；**法定保存**排除同 7111 |
| 7121 | DELIVERY_POD | Java shipping（`MediaUberPodStorage`） | 否（B3） | 同 7101 |
| 7122 | SELLER_PROSPECT_LOGO | seller（prospect） | 是 | 刪 |
| 7123 | SELLER_PROSPECT_COVER | seller（prospect） | 是 | 刪 |
| 7199 | OTHER | media 自己：空 ref_type 的預設（`MediaGrpcService.java:79`，ServerUploadAndFinalize 同樣的 fallback 在其參數驗證段；`finalize_args.go:17`） | — | 刪（0.3：沒有「未分類」，用途不在 media） |

### 3.2 擁有者模組的 `media_ref_sync`／`retired_assets`／`media_sync`

指令：`rg -n "CREATE TABLE.*(media_ref_sync|retired_assets)" --glob schema.sql backend/internal`＝**7 個模組各 2 張表（共 14 張）**；`rg -n "func \(\w+ \w*Args\) Kind" backend/internal/modules/*/internal/mediaref`＝**7 組 `<模組>.media_sync`＋`<模組>.media_sweep` job（共 14 種）**【量】。

| 模組 | `media_ref_sync`／`retired_assets`（schema.sql 行） | job Kind、檔 | 處理的 ref | 去向 |
|---|---|---|---|---|
| buyer | `:150`／`:162` | `buyer.media_sync`、`buyer.media_sweep`（`buyer/internal/mediaref/mediaref.go:31,40,139`，196 行） | BUYER_AVATAR | 刪表、刪 job、刪 sqlc 查詢（`buyer/db/queries/media_refs.sql`）、migration 用 atlas 新增 drop；改成 buyer 自己的「換圖時 `MarkDeleted`」 |
| content | `:69`／`:81` | `content.media_sync`、`content.media_sweep`（`mediaref.go:31,40,153`，210 行） | BANNER | 同上；`content/db/migrations/20261003095157_media_ref_sync.sql` 是建立它的 migration，不改歷史，另加 drop |
| im | `:88`／`:98` | `im.media_sync`、`im.media_sweep`（`mediaref.go:30,39,134`，191 行） | CHAT_MESSAGE | 同上 |
| moderation | `:145`／`:156` | `moderation.media_sync`、`moderation.media_sweep`（`mediaref.go:33,42,153`，210 行） | GOVERNANCE_EVIDENCE（不可改不可刪，無退場） | 同上；沒有刪除路徑的 ref 在新模型下只需「從不發刪除」 |
| review | `:90`／`:100` | `review.media_sync`、`review.media_sweep`（`mediaref.go:30,39,134`，191 行） | ORDER_REVIEW | 同上 |
| seller | `:1362`／`:1374` | `seller.media_sync`、`seller.media_sweep`（`mediaref.go:35,44`，321 行，4 類 ref） | LOGO、COVER、PROSPECT_LOGO／COVER、CERTIFICATION、GOVERNANCE_EVIDENCE | 同上；KYC 與 governance evidence 的 `mediaref.ReadSecure` 保留成 owner 端點代理（`httpapi/seller/files.go`） |
| social | `:377`／`:389` | `social.media_sync`、`social.media_sweep`（`mediaref.go:30,39,139`，196 行） | POST | 同上 |

共用演算法 `M/modules/media/internal/refsync/refsync.go`（321 行）＋測試 32 行，`media.RefSyncer`／`SyncChanged`／`SyncReject`／`SyncWork` 全刪（ADR 後果段已預告）。`media.NewFake`（`fake.go`，209 行，模擬 UNIQUE＋精確 Unlink＋逾時後才生效的 RPC）整個改寫成新介面的記憶體 fake。ADR 要求保留的「監控訊號：`synced_seq < seq` 最舊 `updated_at` 超過 24 小時記 error」隨之消失；新模型的對應監控是「待刪除指令積壓年齡」。

## 4. Java 測試對照表（「integration 測試對照表」骨架）

指令：`find backend/services/java/media-service/src/test -type f`＝17 個 `*Test.java`、4,442 行、`rg -c "@Test"` 合計 **179**【量】；`rg "SpringBootTest|Testcontainers"` 只命中一條註解（`JT/repository/MediaAssetGcExclusionTest.java:24`，明寫「本模組沒有 Testcontainers」）。所以這 179 個都是 Mockito 單元測試，**沒有一個碰真資料庫或真 R2**（R2 以 mock S3Client）。
本表的「斷言」欄是從方法名拆出來的【推：沒有逐一讀測試本體】；每個檔的第一列（粗體）是該檔範圍與整體去向，個別測試的去向沿用該檔。「Go 對應」欄留空，由 B1 的實作者在補 characterization 時填。SQL 語意（`selectOrphanedAssets`、`claimAssetForGc`、`deleteOrphanedReferences`、`ON CONFLICT`）沒有任何 Java 測試能驗，Go 版要用真 Postgres integration 驗（這類測試依全域規則不在本機跑、由 CI 跑，結果出來前標「待 CI」）。

| # | 檔:行 | 測試方法 | 斷言（依方法名） | Go 對應 | 去向／備註 |
|---|---|---|---|---|---|
| | **controller/SecureMediaControllerTest.java** | | SecureMediaController（HTTP 機敏上傳、預覽、下載） | | 改寫：機敏上傳的每次嘗試獨立 key、並發落敗補償、Content-Length 取實際大小、RFC5987 檔名、Disposition 一律搬；下載端點本身刪除（見 §1.2），但 owner 代理端點要有同等測試 |
| 1 | `controller/SecureMediaControllerTest.java:132` | `secureUpload_usesUniquePerAttemptObjectKey_notSessionCreationKey` | secure Upload uses Unique Per Attempt Object Key not Session Creation Key | | |
| 2 | `controller/SecureMediaControllerTest.java:163` | `secureUpload_whenConcurrentAttemptAlreadyWon_deletesOwnUploadAndDoesNotOverwriteWinner` | secure Upload when Concurrent Attempt Already Won deletes Own Upload And Does Not Overwrite Winner | | |
| 3 | `controller/SecureMediaControllerTest.java:190` | `secureUpload_whenR2UploadTimesOut_keepsSessionRetryableAndDoesNotIssueStagedToken` | secure Upload when R2 Upload Times Out keeps Session Retryable And Does Not Issue Staged Token | | |
| 4 | `controller/SecureMediaControllerTest.java:245` | `securePreview_setsContentLengthFromActualR2ObjectSize_notDeclaredSessionFileSize` | secure Preview sets Content Length From Actual R2 Object Size not Declared Session File Size | | |
| 5 | `controller/SecureMediaControllerTest.java:291` | `securePreview_usesInlineDisposition_andRfc5987EncodesCjkFilename` | secure Preview uses Inline Disposition and Rfc5987 Encodes Cjk Filename | | |
| 6 | `controller/SecureMediaControllerTest.java:326` | `secureDownload_usesAttachmentDisposition_andRfc5987EncodesCjkFilename` | secure Download uses Attachment Disposition and Rfc5987 Encodes Cjk Filename | | |
| 7 | `controller/SecureMediaControllerTest.java:380` | `secureDownload_filenameFromWritePathCannotBreakQuotedString` | secure Download filename From Write Path Cannot Break Quoted String | | |
| 8 | `controller/SecureMediaControllerTest.java:416` | `secureDownload_setsContentLengthFromActualR2ObjectSize_notStaleSizeBytes` | secure Download sets Content Length From Actual R2 Object Size not Stale Size Bytes | | |
| | **grpc/ServerReadSecureAssetTest.java** | | ServerReadSecureAsset／內部閘門 | | 改寫成 `Media.ReadSecure`／owner 端點測試；三元組述詞因 ADR 消失，「查無與越權不可分辨」改成「非擁有者不可讀」由 owner 授權測試守；解密與截斷測試保留 |
| 9 | `grpc/ServerReadSecureAssetTest.java:185` | `returnsBytesWhenTripleMatches` | returns Bytes When Triple Matches | | |
| 10 | `grpc/ServerReadSecureAssetTest.java:200` | `filenamePrefersOriginalOverObjectKeyTail` | filename Prefers Original Over Object Key Tail | | |
| 11 | `grpc/ServerReadSecureAssetTest.java:220` | `filenameFallbackStripsAttemptSuffixForLegacyRows` | filename Fallback Strips Attempt Suffix For Legacy Rows | | |
| 12 | `grpc/ServerReadSecureAssetTest.java:243` | `decryptsEncryptedObject` | decrypts Encrypted Object | | |
| 13 | `grpc/ServerReadSecureAssetTest.java:264` | `mismatchedRefIdIsIndistinguishableFromMissing` | mismatched Ref Id Is Indistinguishable From Missing | | |
| 14 | `grpc/ServerReadSecureAssetTest.java:283` | `mismatchedRefTypeIsIndistinguishableFromMissing` | mismatched Ref Type Is Indistinguishable From Missing | | |
| 15 | `grpc/ServerReadSecureAssetTest.java:295` | `rejectsNonInternalCallers` | rejects Non Internal Callers | | |
| 16 | `grpc/ServerReadSecureAssetTest.java:308` | `rejectsEmptyTripleFields` | rejects Empty Triple Fields | | |
| 17 | `grpc/ServerReadSecureAssetTest.java:322` | `rejectsNonSecureAsset` | rejects Non Secure Asset | | |
| 18 | `grpc/ServerReadSecureAssetTest.java:334` | `rejectsTruncatedRead` | rejects Truncated Read | | |
| 19 | `grpc/ServerReadSecureAssetTest.java:350` | `referenceStubOnlyAcceptsExactTriple` | reference Stub Only Accepts Exact Triple | | |
| | **repository/MediaAssetGcExclusionTest.java** | | GC 法定排除（7120、7111） | | 刪除：GC 不再刪已確認資產，排除清單消失；改由 owner 側「發票與證件不呼叫刪除」測試守 |
| 20 | `repository/MediaAssetGcExclusionTest.java:31` | `selectOrphanedAssetsExcludesStatutoryRefTypes` | select Orphaned Assets Excludes Statutory Ref Types | | |
| 21 | `repository/MediaAssetGcExclusionTest.java:42` | `claimAssetForGcExcludesStatutoryRefTypes` | claim Asset For Gc Excludes Statutory Ref Types | | |
| 22 | `repository/MediaAssetGcExclusionTest.java:54` | `exclusionCoversStatutoryTypes` | exclusion Covers Statutory Types | | |
| | **scheduler/MediaGarbageCollectionJobClaimTest.java** | | GC 租約 | | 改寫：租約搶佔與先 commit 租約再做副作用保留給「過期暫存」GC；assetClaim* 三個隨孤兒資產 GC 一起刪 |
| 23 | `scheduler/MediaGarbageCollectionJobClaimTest.java:98` | `sessionClaimLost_deletesNothing` | session Claim Lost deletes Nothing | | |
| 24 | `scheduler/MediaGarbageCollectionJobClaimTest.java:114` | `assetClaimLost_deletesNothing` | asset Claim Lost deletes Nothing | | |
| 25 | `scheduler/MediaGarbageCollectionJobClaimTest.java:130` | `claimWon_claimsBeforeDeleting` | claim Won claims Before Deleting | | |
| 26 | `scheduler/MediaGarbageCollectionJobClaimTest.java:156` | `perRowMarking_onlyMarksSucceeded` | per Row Marking only Marks Succeeded | | |
| 27 | `scheduler/MediaGarbageCollectionJobClaimTest.java:181` | `leaseLengthIsBoundedByGcPeriod` | lease Length Is Bounded By Gc Period | | |
| 28 | `scheduler/MediaGarbageCollectionJobClaimTest.java:209` | `leaseBaselineIsPerItem` | lease Baseline Is Per Item | | |
| 29 | `scheduler/MediaGarbageCollectionJobClaimTest.java:248` | `sessionClaimCommitsBeforeSideEffect` | session Claim Commits Before Side Effect | | |
| 30 | `scheduler/MediaGarbageCollectionJobClaimTest.java:288` | `assetClaimCommitsBeforeSideEffect` | asset Claim Commits Before Side Effect | | |
| 31 | `scheduler/MediaGarbageCollectionJobClaimTest.java:339` | `assetClaimRestatesNoReferencePredicate` | asset Claim Restates No Reference Predicate | | |
| | **scheduler/MediaGarbageCollectionJobTest.java** | | GC 行為 | | 改寫：只留過期 session 與 R2 失敗保留列、停用開關、tx 邊界；孤兒資產、孤立引用相關案例刪除 |
| 32 | `scheduler/MediaGarbageCollectionJobTest.java:86` | `shouldCleanExpiredInitSessions` | should Clean Expired Init Sessions | | |
| 33 | `scheduler/MediaGarbageCollectionJobTest.java:112` | `shouldCleanExpiredCompletedSessions` | should Clean Expired Completed Sessions | | |
| 34 | `scheduler/MediaGarbageCollectionJobTest.java:131` | `shouldNotCleanNonExpiredSessions` | should Not Clean Non Expired Sessions | | |
| 35 | `scheduler/MediaGarbageCollectionJobTest.java:148` | `shouldCleanOrphanedAssets` | should Clean Orphaned Assets | | |
| 36 | `scheduler/MediaGarbageCollectionJobTest.java:167` | `shouldNotCleanAssetWithReferences` | should Not Clean Asset With References | | |
| 37 | `scheduler/MediaGarbageCollectionJobTest.java:179` | `shouldNotCleanRecentOrphanedAssets` | should Not Clean Recent Orphaned Assets | | |
| 38 | `scheduler/MediaGarbageCollectionJobTest.java:195` | `shouldKeepDbRowWhenStorageDeleteFails` | should Keep Db Row When Storage Delete Fails | | |
| 39 | `scheduler/MediaGarbageCollectionJobTest.java:223` | `shouldBreakBatchLoopOnPartialFailure` | should Break Batch Loop On Partial Failure | | |
| 40 | `scheduler/MediaGarbageCollectionJobTest.java:247` | `shouldSkipWhenDisabled` | should Skip When Disabled | | |
| 41 | `scheduler/MediaGarbageCollectionJobTest.java:264` | `cleanMethodsAreWrappedInTransactionTemplate` | clean Methods Are Wrapped In Transaction Template | | |
| 42 | `scheduler/MediaGarbageCollectionJobTest.java:283` | `midFlightThrowTriggersRollback` | mid Flight Throw Triggers Rollback | | |
| | **service/AssetReferenceServiceTest.java** | | Link／Unlink／Release | | 刪除（asset_reference 消失）；idempotent 與錯誤碼對應改成 Delete 指令的冪等測試 |
| 43 | `service/AssetReferenceServiceTest.java:55` | `shouldLinkSuccessfully` | should Link Successfully | | |
| 44 | `service/AssetReferenceServiceTest.java:77` | `shouldBeIdempotent` | should Be Idempotent | | |
| 45 | `service/AssetReferenceServiceTest.java:98` | `shouldTolerateDuplicateAssetIds` | should Tolerate Duplicate Asset Ids | | |
| 46 | `service/AssetReferenceServiceTest.java:120` | `shouldThrowWhenRefTypeEmpty` | should Throw When Ref Type Empty | | |
| 47 | `service/AssetReferenceServiceTest.java:129` | `shouldThrowWhenRefIdEmpty` | should Throw When Ref Id Empty | | |
| 48 | `service/AssetReferenceServiceTest.java:138` | `shouldThrowWhenAssetIdsEmpty` | should Throw When Asset Ids Empty | | |
| 49 | `service/AssetReferenceServiceTest.java:147` | `shouldThrowWhenAssetNotFound` | should Throw When Asset Not Found | | |
| 50 | `service/AssetReferenceServiceTest.java:165` | `shouldUnlinkSuccessfully` | should Unlink Successfully | | |
| 51 | `service/AssetReferenceServiceTest.java:180` | `shouldBeIdempotentOnNonExistent` | should Be Idempotent On Non Existent | | |
| 52 | `service/AssetReferenceServiceTest.java:195` | `shouldThrowWhenRefTypeEmpty` | should Throw When Ref Type Empty | | |
| 53 | `service/AssetReferenceServiceTest.java:209` | `shouldReleaseSuccessfully` | should Release Successfully | | |
| 54 | `service/AssetReferenceServiceTest.java:226` | `shouldPassKeepAssetIdsDownToRepository` | should Pass Keep Asset Ids Down To Repository | | |
| 55 | `service/AssetReferenceServiceTest.java:244` | `shouldTreatNullKeepListAsReleaseAll` | should Treat Null Keep List As Release All | | |
| 56 | `service/AssetReferenceServiceTest.java:256` | `shouldThrowWhenKeepAssetIdMalformed` | should Throw When Keep Asset Id Malformed | | |
| 57 | `service/AssetReferenceServiceTest.java:264` | `shouldThrowWhenRefIdsEmpty` | should Throw When Ref Ids Empty | | |
| 58 | `service/AssetReferenceServiceTest.java:272` | `shouldThrowWhenRefTypeInvalid` | should Throw When Ref Type Invalid | | |
| | **service/BatchGetAssetMetadataTest.java** | | BatchGetAssetMetadata | | 改寫成 owner 端點的 metadata 查詢（asset 的 id、檔名、大小、時間）；授權部分刪除 |
| 59 | `service/BatchGetAssetMetadataTest.java:71` | `returnsOnlyAuthorizedAssetsAndCannotDistinguishDenialFromMissing` | returns Only Authorized Assets And Cannot Distinguish Denial From Missing | | |
| 60 | `service/BatchGetAssetMetadataTest.java:98` | `originalFilenameTakesPrecedenceOverObjectKeyTail` | original Filename Takes Precedence Over Object Key Tail | | |
| 61 | `service/BatchGetAssetMetadataTest.java:117` | `fallsBackToObjectKeyTailForLegacyRows` | falls Back To Object Key Tail For Legacy Rows | | |
| 62 | `service/BatchGetAssetMetadataTest.java:131` | `emptyInputIsANoop` | empty Input Is ANoop | | |
| | **service/MediaAssetServiceTest.java** | | Finalize、後端直傳 | | 改寫：轉正流程各錯誤分支、重入、HEAD 回查、tmp 刪除時機、補償、原始檔名消毒保留；ref 相關案例（uploadsToSecurePrefixAndLinksReference）改成不建引用 |
| 63 | `service/MediaAssetServiceTest.java:146` | `shouldFinalizeSuccessfully` | should Finalize Successfully | | |
| 64 | `service/MediaAssetServiceTest.java:192` | `copiesOriginalFilenameFromSession` | copies Original Filename From Session | | |
| 65 | `service/MediaAssetServiceTest.java:214` | `leavesOriginalFilenameNullWhenSessionHasNone` | leaves Original Filename Null When Session Has None | | |
| 66 | `service/MediaAssetServiceTest.java:236` | `shouldReturnExistingAssetWhenAlreadyFinalized` | should Return Existing Asset When Already Finalized | | |
| 67 | `service/MediaAssetServiceTest.java:278` | `shouldThrowPermissionDeniedWhenOwnerMismatch` | should Throw Permission Denied When Owner Mismatch | | |
| 68 | `service/MediaAssetServiceTest.java:295` | `shouldThrowInvalidArgumentWhenSessionNotFound` | should Throw Invalid Argument When Session Not Found | | |
| 69 | `service/MediaAssetServiceTest.java:312` | `shouldThrowInvalidArgumentWhenSessionExpired` | should Throw Invalid Argument When Session Expired | | |
| 70 | `service/MediaAssetServiceTest.java:338` | `shouldThrowInvalidArgumentWhenSessionStatusIsInit` | should Throw Invalid Argument When Session Status Is Init | | |
| 71 | `service/MediaAssetServiceTest.java:357` | `shouldRejectWhenClaimsScopeMismatchSession` | should Reject When Claims Scope Mismatch Session | | |
| 72 | `service/MediaAssetServiceTest.java:379` | `shouldRejectWhenSessionAccountDiffersFromClaims` | should Reject When Session Account Differs From Claims | | |
| 73 | `service/MediaAssetServiceTest.java:395` | `shouldRejectAndCleanupWhenHeadCheckFails` | should Reject And Cleanup When Head Check Fails | | |
| 74 | `service/MediaAssetServiceTest.java:417` | `shouldFinalizeWhenHeadCheckPasses` | should Finalize When Head Check Passes | | |
| 75 | `service/MediaAssetServiceTest.java:450` | `shouldDeleteTmpOnlyAfterDbCommit` | should Delete Tmp Only After Db Commit | | |
| 76 | `service/MediaAssetServiceTest.java:470` | `shouldNotInsertWhenLosingStatusCas` | should Not Insert When Losing Status Cas | | |
| 77 | `service/MediaAssetServiceTest.java:499` | `shouldCompensateAlreadyCopiedObjectsWhenLaterTokenFails` | should Compensate Already Copied Objects When Later Token Fails | | |
| 78 | `service/MediaAssetServiceTest.java:547` | `shouldSkipCompensationWhenAnotherAssetOwnsTheObject` | should Skip Compensation When Another Asset Owns The Object | | |
| 79 | `service/MediaAssetServiceTest.java:569` | `shouldDeleteUploadedObjectWhenInsertFails` | should Delete Uploaded Object When Insert Fails | | |
| 80 | `service/MediaAssetServiceTest.java:588` | `persistsSanitizedFilenameButKeepsRawInObjectKey` | persists Sanitized Filename But Keeps Raw In Object Key | | |
| 81 | `service/MediaAssetServiceTest.java:629` | `uploadsToSecurePrefixAndLinksReference` | uploads To Secure Prefix And Links Reference | | |
| 82 | `service/MediaAssetServiceTest.java:655` | `compensatesWhenPersistFails` | compensates When Persist Fails | | |
| 83 | `service/MediaAssetServiceTest.java:672` | `persistsSanitizedFilenameButKeepsRawInObjectKey` | persists Sanitized Filename But Keeps Raw In Object Key | | |
| | **service/MediaUploadValidatorTest.java** | | 宣告與實際物件驗證 | | 保留（MIME 白名單、大小上限、GCM tag offset 隨加密格式改寫） |
| 84 | `service/MediaUploadValidatorTest.java:30` | `declaredValidImage` | declared Valid Image | | |
| 85 | `service/MediaUploadValidatorTest.java:37` | `declaredRejectsUnknownMime` | declared Rejects Unknown Mime | | |
| 86 | `service/MediaUploadValidatorTest.java:44` | `declaredPdfOnlyForSecure` | declared Pdf Only For Secure | | |
| 87 | `service/MediaUploadValidatorTest.java:52` | `declaredRejectsOversize` | declared Rejects Oversize | | |
| 88 | `service/MediaUploadValidatorTest.java:59` | `declaredRejectsNonPositiveSize` | declared Rejects Non Positive Size | | |
| 89 | `service/MediaUploadValidatorTest.java:68` | `finalizedRejectsMissingObject` | finalized Rejects Missing Object | | |
| 90 | `service/MediaUploadValidatorTest.java:75` | `finalizedRejectsActualOversize` | finalized Rejects Actual Oversize | | |
| 91 | `service/MediaUploadValidatorTest.java:83` | `finalizedRejectsSizeMismatch` | finalized Rejects Size Mismatch | | |
| 92 | `service/MediaUploadValidatorTest.java:91` | `finalizedRejectsActualMimeNotWhitelisted` | finalized Rejects Actual Mime Not Whitelisted | | |
| 93 | `service/MediaUploadValidatorTest.java:99` | `finalizedRejectsMimeMismatch` | finalized Rejects Mime Mismatch | | |
| 94 | `service/MediaUploadValidatorTest.java:107` | `finalizedPasses` | finalized Passes | | |
| 95 | `service/MediaUploadValidatorTest.java:115` | `finalizedSecurePassesWithGcmTagOverhead` | finalized Secure Passes With Gcm Tag Overhead | | |
| 96 | `service/MediaUploadValidatorTest.java:127` | `finalizedSecureRejectsWhenMissingGcmTag` | finalized Secure Rejects When Missing Gcm Tag | | |
| 97 | `service/MediaUploadValidatorTest.java:135` | `finalizedSecureSkipsContentTypeCheck` | finalized Secure Skips Content Type Check | | |
| | **service/ServerUploadSecureEncryptionTest.java** | | 機敏後端直傳加密 | | 改寫：密文非明文、往返、主密鑰缺失拒絕寫明文；legacyPlaintext 案例刪（上線前無明文舊資產，待決 §0.5 確認） |
| 98 | `service/ServerUploadSecureEncryptionTest.java:112` | `uploadsCiphertextNotPlaintext` | uploads Ciphertext Not Plaintext | | |
| 99 | `service/ServerUploadSecureEncryptionTest.java:139` | `roundTripsThroughTheRealCipher` | round Trips Through The Real Cipher | | |
| 100 | `service/ServerUploadSecureEncryptionTest.java:169` | `legacyPlaintextAssetsStayReadable` | legacy Plaintext Assets Stay Readable | | |
| 101 | `service/ServerUploadSecureEncryptionTest.java:199` | `refusesToWritePlaintextWhenMasterKeyMissing` | refuses To Write Plaintext When Master Key Missing | | |
| | **service/StagedTokenServiceTest.java** | | staged token（HS JWT） | | 改寫：token 格式待決（見 §0.2）；偽造、過期、金鑰輪替、缺 secret fail-closed 保留 |
| 102 | `service/StagedTokenServiceTest.java:41` | `missingSecretFailsClosed` | missing Secret Fails Closed | | |
| 103 | `service/StagedTokenServiceTest.java:48` | `missingSecretWithAllowEphemeralUsesEphemeralKey` | missing Secret With Allow Ephemeral Uses Ephemeral Key | | |
| 104 | `service/StagedTokenServiceTest.java:65` | `explicitSecretRoundTrip` | explicit Secret Round Trip | | |
| 105 | `service/StagedTokenServiceTest.java:81` | `forgedSignatureRejected` | forged Signature Rejected | | |
| 106 | `service/StagedTokenServiceTest.java:101` | `tokenSignedWithOldKeyRejectedAfterRotation` | token Signed With Old Key Rejected After Rotation | | |
| 107 | `service/StagedTokenServiceTest.java:118` | `expiredTokenRejected` | expired Token Rejected | | |
| | **service/UploadSessionServiceTest.java** | | 原始檔名消毒 | | 保留 |
| 108 | `service/UploadSessionServiceTest.java:65` | `persistsSanitizedOriginalFilename` | persists Sanitized Original Filename | | |
| 109 | `service/UploadSessionServiceTest.java:77` | `objectKeyGenerationReceivesRawFilename` | object Key Generation Receives Raw Filename | | |
| 110 | `service/UploadSessionServiceTest.java:95` | `storesNullWhenNothingSurvivesSanitization` | stores Null When Nothing Survives Sanitization | | |
| 111 | `service/UploadSessionServiceTest.java:107` | `keepsCleanCjkFilename` | keeps Clean Cjk Filename | | |
| | **storage/AbstractStorageServiceTest.java** | | object key 生成 | | 改寫：category 前綴隨 ref_type 消失；路徑穿越副檔名拒絕保留 |
| 112 | `storage/AbstractStorageServiceTest.java:45` | `generateObjectKey_normalExtension` | generate Object Key normal Extension | | |
| 113 | `storage/AbstractStorageServiceTest.java:53` | `generateObjectKey_rejectsBackslashInExtension` | generate Object Key rejects Backslash In Extension | | |
| 114 | `storage/AbstractStorageServiceTest.java:60` | `generateObjectKey_rejectsSlashInExtension` | generate Object Key rejects Slash In Extension | | |
| 115 | `storage/AbstractStorageServiceTest.java:67` | `finalKeyPrefixTracksCodeMetaCode` | final Key Prefix Tracks Code Meta Code | | |
| 116 | `storage/AbstractStorageServiceTest.java:86` | `buyerAvatarPrefixUnchanged` | buyer Avatar Prefix Unchanged | | |
| 117 | `storage/AbstractStorageServiceTest.java:99` | `importFiveSegmentKey_usesDateNotImportLiteral` | import Five Segment Key uses Date Not Import Literal | | |
| 118 | `storage/AbstractStorageServiceTest.java:113` | `normalThreeSegmentKey_unchanged` | normal Three Segment Key unchanged | | |
| 119 | `storage/AbstractStorageServiceTest.java:123` | `secureThreeSegmentKey_usesSecurePrefix` | secure Three Segment Key uses Secure Prefix | | |
| 120 | `storage/AbstractStorageServiceTest.java:133` | `noDateSegment_fallsBackToToday` | no Date Segment falls Back To Today | | |
| | **storage/ImageMetadataStripperTest.java** | | metadata 剝除 | | 保留且搬（不變量，AGENTS.md 已指定）；Go 版需同樣的 JPEG、PNG、WebP、GIF 案例與損毀拒絕 |
| 121 | `storage/ImageMetadataStripperTest.java:106` | `jpegIsStrippedLosslessly` | jpeg Is Stripped Losslessly | | |
| 122 | `storage/ImageMetadataStripperTest.java:128` | `jpegKeepsOnlyOrientation` | jpeg Keeps Only Orientation | | |
| 123 | `storage/ImageMetadataStripperTest.java:140` | `jpegWithDefaultOrientationHasNoExif` | jpeg With Default Orientation Has No Exif | | |
| 124 | `storage/ImageMetadataStripperTest.java:147` | `truncatedJpegIsRejected` | truncated Jpeg Is Rejected | | |
| 125 | `storage/ImageMetadataStripperTest.java:170` | `pngIsStripped` | png Is Stripped | | |
| 126 | `storage/ImageMetadataStripperTest.java:222` | `webpIsStripped` | webp Is Stripped | | |
| 127 | `storage/ImageMetadataStripperTest.java:246` | `webpWithDefaultOrientationHasNoExif` | webp With Default Orientation Has No Exif | | |
| 128 | `storage/ImageMetadataStripperTest.java:257` | `gifIsStripped` | gif Is Stripped | | |
| 129 | `storage/ImageMetadataStripperTest.java:282` | `unknownFormatIsNull` | unknown Format Is Null | | |
| | **storage/R2StorageServiceTest.java** | | R2 adapter | | 改寫成 Go S3 client adapter 測試；剝除、上傳超時包裝、presign 保留 |
| 130 | `storage/R2StorageServiceTest.java:67` | `uploadStreamPassesObjectKey` | upload Stream Passes Object Key | | |
| 131 | `storage/R2StorageServiceTest.java:81` | `uploadStreamWrapsR2ApiTimeout` | upload Stream Wraps R2 Api Timeout | | |
| 132 | `storage/R2StorageServiceTest.java:123` | `deletePassesObjectKey` | delete Passes Object Key | | |
| 133 | `storage/R2StorageServiceTest.java:133` | `copyPassesKeys` | copy Passes Keys | | |
| 134 | `storage/R2StorageServiceTest.java:162` | `copyToPublicStripsMetadata` | copy To Public Strips Metadata | | |
| 135 | `storage/R2StorageServiceTest.java:183` | `copyToPublicRejectsCorruptImage` | copy To Public Rejects Corrupt Image | | |
| 136 | `storage/R2StorageServiceTest.java:199` | `copyToPublicNonImageUsesServerSideCopy` | copy To Public Non Image Uses Server Side Copy | | |
| 137 | `storage/R2StorageServiceTest.java:213` | `uploadToPublicStripsMetadata` | upload To Public Strips Metadata | | |
| 138 | `storage/R2StorageServiceTest.java:226` | `uploadToTmpIsUntouched` | upload To Tmp Is Untouched | | |
| 139 | `storage/R2StorageServiceTest.java:247` | `statObjectReturnsStat` | stat Object Returns Stat | | |
| 140 | `storage/R2StorageServiceTest.java:263` | `statObjectMissingReturnsNull` | stat Object Missing Returns Null | | |
| 141 | `storage/R2StorageServiceTest.java:271` | `presignUpload` | presign Upload | | |
| 142 | `storage/R2StorageServiceTest.java:283` | `presignDownload` | presign Download | | |
| 143 | `storage/R2StorageServiceTest.java:295` | `notConfiguredFallback` | not Configured Fallback | | |
| 144 | `storage/R2StorageServiceTest.java:307` | `copyMissingSourceThrows` | copy Missing Source Throws | | |
| | **storage/StorageBackendConfigTest.java** | | 儲存後端設定 | | 刪除：Go 沒有 Spring 條件裝配；「非 r2 值啟動即失敗」改成設定驗證測試 |
| 145 | `storage/StorageBackendConfigTest.java:20` | `defaultIsR2` | default Is R2 | | |
| 146 | `storage/StorageBackendConfigTest.java:29` | `explicitR2` | explicit R2 | | |
| 147 | `storage/StorageBackendConfigTest.java:38` | `legacyOssNotWired` | legacy Oss Not Wired | | |
| | **util/AssetFilenameTest.java** | | 顯示檔名消毒與回退 | | 保留且搬 |
| 148 | `util/AssetFilenameTest.java:25` | `keepsCleanNameUnchanged` | keeps Clean Name Unchanged | | |
| 149 | `util/AssetFilenameTest.java:31` | `takesBasename` | takes Basename | | |
| 150 | `util/AssetFilenameTest.java:42` | `stripsCrLf` | strips Cr Lf | | |
| 151 | `util/AssetFilenameTest.java:51` | `stripsNul` | strips Nul | | |
| 152 | `util/AssetFilenameTest.java:61` | `truncatesKeepingExtension` | truncates Keeping Extension | | |
| 153 | `util/AssetFilenameTest.java:74` | `truncatesByCharactersNotDisplayWidth` | truncates By Characters Not Display Width | | |
| 154 | `util/AssetFilenameTest.java:86` | `returnsNullWhenNothingLeft` | returns Null When Nothing Left | | |
| 155 | `util/AssetFilenameTest.java:97` | `returnsNullForNullInput` | returns Null For Null Input | | |
| 156 | `util/AssetFilenameTest.java:103` | `stripsDoubleQuote` | strips Double Quote | | |
| 157 | `util/AssetFilenameTest.java:120` | `outputNeverContainsQuoteOrBackslash` | output Never Contains Quote Or Backslash | | |
| 158 | `util/AssetFilenameTest.java:140` | `trimsSurroundingWhitespace` | trims Surrounding Whitespace | | |
| 159 | `util/AssetFilenameTest.java:151` | `originalFilenameWins` | original Filename Wins | | |
| 160 | `util/AssetFilenameTest.java:160` | `fallsBackToObjectKeyTail` | falls Back To Object Key Tail | | |
| 161 | `util/AssetFilenameTest.java:168` | `stripsAttemptSuffixOnFallback` | strips Attempt Suffix On Fallback | | |
| 162 | `util/AssetFilenameTest.java:178` | `doesNotStripNonUuidAttemptSuffix` | does Not Strip Non Uuid Attempt Suffix | | |
| 163 | `util/AssetFilenameTest.java:185` | `objectKeyWithoutSlash` | object Key Without Slash | | |
| 164 | `util/AssetFilenameTest.java:191` | `blankOriginalIsTreatedAsAbsent` | blank Original Is Treated As Absent | | |
| 165 | `util/AssetFilenameTest.java:198` | `fallbackValueIsSanitized` | fallback Value Is Sanitized | | |
| 166 | `util/AssetFilenameTest.java:218` | `originalFilenameBranchIsSanitizedToo` | original Filename Branch Is Sanitized Too | | |
| 167 | `util/AssetFilenameTest.java:228` | `returnValueNeverContainsQuoteOrBackslash` | return Value Never Contains Quote Or Backslash | | |
| 168 | `util/AssetFilenameTest.java:247` | `returnsEmptyWhenFallbackSanitizesToNothing` | returns Empty When Fallback Sanitizes To Nothing | | |
| 169 | `util/AssetFilenameTest.java:253` | `returnsEmptyStringWhenNothingAvailable` | returns Empty String When Nothing Available | | |
| 170 | `util/AssetFilenameTest.java:267` | `stripsQueryAndFragment` | strips Query And Fragment | | |
| 171 | `util/AssetFilenameTest.java:280` | `doesNotDoubleDecode_percent` | does Not Double Decode percent | | |
| 172 | `util/AssetFilenameTest.java:290` | `doesNotDoubleDecode_plus` | does Not Double Decode plus | | |
| 173 | `util/AssetFilenameTest.java:297` | `doesNotDoubleDecode_pathTraversal` | does Not Double Decode path Traversal | | |
| 174 | `util/AssetFilenameTest.java:307` | `trailingSlashYieldsNull` | trailing Slash Yields Null | | |
| 175 | `util/AssetFilenameTest.java:314` | `opaqueUriYieldsNull` | opaque Uri Yields Null | | |
| 176 | `util/AssetFilenameTest.java:320` | `illegalUriYieldsNullNotThrow` | illegal Uri Yields Null Not Throw | | |
| 177 | `util/AssetFilenameTest.java:329` | `nullInputYieldsNull` | null Input Yields Null | | |
| 178 | `util/AssetFilenameTest.java:337` | `stripsDecodedNul` | strips Decoded Nul | | |
| 179 | `util/AssetFilenameTest.java:344` | `takesLastPathSegment` | takes Last Path Segment | | |

## 5. 既有缺陷與怪異處（Java 行為）

處置詞彙：修＝新實作不帶這個行為；接受＝照搬；待決＝見 §0。多數缺陷隨表與 RPC 消失而 moot，仍列出以免被當成要保留的行為。

| # | 位置 | 描述 | 處置 |
|---|---|---|---|
| 5.1 | `J/service/MediaAssetService.java:388-406`（`findExistingAsset`） | 重入或並發落敗時，若 session 沒有 `asset_id` 或資產列不存在，退回「該上傳者最新的任一資產」（註解稱相容舊資料）。可能回傳別次上傳的資產 | 修：上線前沒有舊資料；`Confirm` 重入只依 session 的 asset，找不到就報內部錯誤 |
| 5.2 | `J/service/MediaAssetService.java:210-216` | FINALIZED 重入忽略這次請求的 `ref_type`，直接回舊資產 | 接受（`ref_type` 隨 0.3 消失，不再有此情形） |
| 5.3 | `J/service/MediaUploadValidator.java:100-106` | 機敏檔的實際 MIME 不驗（密文一律 `application/octet-stream`），只信建立會話時的宣告；PDF 宣告後實際位元組完全不檢查 | 修：加密前依 magic bytes 驗 PDF／圖片（0.5） |
| 5.4 | `J/storage/ImageMetadataStripper.java:41-57`、`R2StorageService.java:180-181,314-318` | `strip` 對無法辨認的格式回 `null`，呼叫端就原樣寫入 `public/`；宣告 `image/png` 但內容是任意位元組也能通過（HEAD 只比 Content-Type 與大小）。剝除不變量對「不是圖片的圖片」不成立 | 修：寫入 `public/` 前必須辨認為四種圖片格式之一，否則拒絕（搬 stripper 時一併做，並補測試） |
| 5.5 | `J/controller/SecureMediaController.java:314-315`、`AssetReferenceRepository.java:153-161`（`hasAccessByAccount`） | 下載授權＝上傳者，或「有引用且上傳者是該帳號」；第二個條件是第一個的子集，整句等於只有上傳者。gateway 註解已承認（`routes_invoice.go:193-196`） | 刪（端點刪除，§1.2 #3） |
| 5.6 | `SecureMediaController.java:241,330`、`J/crypto/AesStreamCipher.java:147`、`MediaAssetService.java:717` | 串流解密在讀到結尾才驗 GCM tag，HTTP 回應的標頭與主體已送出；被竄改的密文會先送出明文片段，錯誤只出現在連線中斷。整檔讀取版（`readAllBytes`）不受影響 | 待決 0.4 |
| 5.7 | `AesStreamCipher.java:30,75-101` | 包裝後的金鑰沒有金鑰版本欄位；換 `ENCRYPTION_MASTER_KEY` 會讓所有既有機敏資產無法解密，且沒有輪替機制 | 修：密文信封帶 key id，至少支援兩把主密鑰並存（0.4） |
| 5.8 | `SecureMediaController.java:119-131`；`G/middleware/body_guard.go:14,53`；`application.yml`（multipart 100MB、`MEDIA_MAX_SECURE_SIZE_BYTES`） | 機敏上傳的上限在三處各自設定（gateway 20MiB、Java multipart 100MB、驗證器 100MiB）。`secure_upload` 寫入 R2 之前不比對宣告大小與上限，超量要到 Finalize 的 HEAD 才被拒（已寫入的暫存物件靠 GC 回收）；若請求沒有 `Content-Length`，`getContentLengthLong()` 回 -1，`ciphertextSize` 變 15，傳給 S3 的長度是錯的 | 修：單一上限（設定一處）；串流中以 `LimitReader` 強制，超量就中止並刪除 |
| 5.9 | `J/grpc/MediaGrpcService.java:142,222,302,550,641`（無閘門）對 `:723,783`（有閘門）；`.env.template:231`、`docker-compose-local.yml:1094,1082` 起（埠 9936/9937 發布到主機） | 只有讀取兩支檢查內部呼叫；建引用、解引用、批次釋放、兩個直傳都沒有，攔截器預設 `INTERNAL_AUTH_MODE=permissive`。能連到 9937 的人能對任何資產 Link／Unlink／Release，也能直傳檔案 | 接受（RPC 面整個刪除；本機 compose 的埠暴露屬環境）。新 `PutPublic/PutSecure` 是 Go 內部 port，不暴露在任何 HTTP／Connect 面 |
| 5.10 | `J/client/SellerGrpcClient.java:19`（77 行） | `resolvePrincipals` 沒有任何呼叫端（`rg "sellerGrpcClient\|SellerGrpcClient" -g '*.java' media-service` 只在該檔自己）；`SELLER_SERVICE_*` 設定與 `rpc-calls.csv` 的 `media-service→seller-service ResolvePrincipals` 列都是死的 | 刪 |
| 5.11 | `J/scheduler/MediaGarbageCollectionJob.java:329` | `triggerManualGc` 沒有呼叫端也沒有測試 | 刪 |
| 5.12 | `R2StorageService.java:128,174,243,271`；`MediaAssetService.java:244` | R2 沒設定時，presign／上傳／複製回模擬成功、`statObject` 回 null、`verifyUploadedObject` 整個跳過：設定漏了服務照常運作，卻什麼都沒存 | 修：缺 R2 設定時啟動即失敗（測試用 fake 儲存） |
| 5.13 | `J/service/AssetReferenceService.java:71-84` | `linked_count` 回的是「嘗試次數」，不是實際插入數（`ON CONFLICT DO NOTHING` 的 0 列也被算） | 刪（moot） |
| 5.14 | `UploadSessionService.java:42` 對 `SecureMediaController.java:62` | 同一個屬性 `hawkr.media.preview.ttl` 兩處預設不同（86400 與 3600）；目前 yaml 與 compose 都設 3600 所以沒爆 | 修：單一常數或設定，一處定義 |
| 5.15 | `SecureMediaController.java:127`（`.attempt-{uuid}` 後綴）、`J/storage/AbstractStorageService.java:43-60`（final key 取原 key 的最後一段） | 機敏上傳的正式 object key 永遠帶著 `.attempt-{uuid}` 後綴；`AssetFilename.resolveDisplayName`（`:106`）只在顯示時剝掉 | 修：正式 key 用新的 id 生成，不繼承暫存名 |
| 5.16 | `MediaAssetService.java:434,503` | 直傳的 key 直接拼入原始 `filename`（顯示名才消毒，`:435`）；`/`、`..` 會產生任意 key 片段，也沒有 `generateObjectKey` 那道副檔名拒絕 | 修（PutPublic／PutSecure 由 media 生成 key，不吃呼叫端檔名） |
| 5.17 | `MediaGarbageCollectionJob.java:149-155`；`JT/scheduler/MediaGarbageCollectionJobTest.java:283`（`midFlightThrowTriggersRollback`） | 三階段各是一個長 tx，內含 R2 刪除；中途丟例外時資料庫回滾但已刪的 R2 物件不會回來（租約在 REQUIRES_NEW 已提交，所以不會重複搶，但列指著不存在的物件）。測試只驗「有回滾」，不驗這個後果 | 修：新的過期暫存 GC 逐列處理，先標記→刪物件→提交（與租約設計一致） |
| 5.18 | `UploadSessionService.java:117-183` | `CompleteUploadSession` 不確認物件真的存在，就簽發 staged token | 接受或 moot：0.5 選 A 時物件存在才有 token |
| 5.19 | `MediaGrpcService.java:852-862`、`legacy/mediaservice/mediaservice.go:161-177` | 錯誤用「200＋body code」表達，Go 端要自己解碼 400／403／404 並猜意義；`ErrNotSecure`、`ErrStagedTokenRejected` 是這層轉譯的產物 | 刪（單體內用型別化錯誤，不再有碼表轉譯；`ErrNotSecure` 是否保留看 0.4 的 `ReadSecure` 簽名） |
| 5.20 | `docs/plans/2026-09-24-go-modular-monolith/events.csv`、`rpc-pairs.csv` | 仍列 media 的匯入事件與 `media-service→seller-service ResolvePrincipals`，與 Java 現況不符 | 修（B1 同步更新 CSV；`scripts/check-migration-inventory.py` 的詞彙） |

## 6. 規模

快照：`git rev-parse HEAD`＝`17bb31c545a64d476f7bf218a38f4b61c2ee0aef`（分支 `b1-media`，worktree `/Users/will/second-brain/.projects/personal/hawkr/.worktrees/b1-media`）。

| 項目 | 量到 | 指令 |
|---|---|---|
| gRPC RPC | **10**（10 個都有呼叫端；`ImportDownloadAsset`、`FinalizeImportAsset`、`BindStagedAsset` 已不在 proto） | `rg -c "^\s*rpc " backend/api/proto/media/media.proto` |
| REST 端點 | **4**（`secure_upload`、`secure_preview`、`secure_download`、`/health`） | `rg -n "Mapping" backend/services/java/media-service/src/main -g '*.java'` |
| RabbitMQ 事件 | **0** | `rg -in "rabbit\|amqp" backend/services/java/media-service` |
| 排程 | **1** 個 cron、3 個階段 | `rg -n "@Scheduled" backend/services/java/media-service/src/main` |
| 表（Java） | **3**（`upload_session`、`media_asset`、`asset_reference`） | `rg -n "CREATE TABLE" JR/db/migration` 加 V10 的 drop |
| Flyway 檔／行 | 10 檔／525 行 | `ls JR/db/migration`、`cat JR/db/migration/*.sql \| wc -l` |
| Java main（含生成前手寫） | 37 檔／5,888 行 | `find J -name '*.java' \| xargs cat \| wc -l` |
| Java test | 17 檔／4,442 行／179 個 `@Test`／0 個整合測試 | `rg -c "@Test" JT`、`rg "SpringBootTest\|Testcontainers" JT` |
| ref_type 值 | **22**（CodeMeta、`ref_type.go`、`pkg/media/ref_type.go`、Java enum 各一份，加 `finalize_args.go` 手寫集合＝5 處） | `rg -c '\("[A-Z_]+", "7' AssetReferenceType.java`、`rg -c '"7[0-9]{3}"' ref_type.go` |
| 有 `media_ref_sync`／`retired_assets` 的 Go 模組 | **7**（buyer、content、im、moderation、review、seller、social），14 張表、14 種 job | `rg -ln media_ref_sync --glob schema.sql backend/internal` |
| Go media 模組 | 非測試 845 行／測試 217 行；`legacy/mediaservice` 177／293 行 | `find backend/internal/modules/media -name '*.go' …`；`wc -l` |
| 單體呼叫 media 的非測試檔／測試檔 | 43／48；Finalize 呼叫點 9、Link／Unlink 呼叫點 2（皆在 `refsync.go`）、ReadSecure 呼叫點 1（`seller/internal/mediaref/mediaref.go:271`） | 見 §2.1 指令 |
| Java 呼叫端檔 | 13 檔（product、order、aftersales、shipping 四個服務） | 見 §2.1 指令 |
| Go legacy 服務的 media client | 5 檔（seller、invoice、im、ai、review）＋gateway＋`pkg/media` | `ls backend/services/*/internal/client/media/client.go` |
| client 檔（上傳路徑） | website 12、admin 3、seller-app 3、buyer-app 1 | §2.3 指令 |
| 設定／環境變數 | 約 30 個鍵（§1.7） | `application.yml` 與 compose `:1082` 起 |

## 7. 證據契約

- **讀過的檔**：ADR `docs/adr/2026-10-03-media-references-owner-side.md`（決策、後果、修訂）；Java media-service 的 `application.yml`、10 個 Flyway 檔、`MediaGrpcService`（簽名與閘門）、`MediaAssetService`（finalize、直傳、讀取）、`UploadSessionService`、`AssetReferenceService`／`Repository`、`MediaGarbageCollectionJob`、`SecureMediaController`、`MediaUploadValidator`、`AbstractStorageService`、`R2StorageService`（上傳、複製、stat、剝除呼叫點）、`ImageMetadataStripper`（開頭與簽名，沒逐行讀 JPEG／PNG／WebP／GIF 解析）、`AesStreamCipher`（簽名與註解）、`StagedTokenService`（簽名）；`api/proto/media/media.proto` 全文；Go 的 `modules/media/AGENTS.md`、`media.go`、`ref_type.go`、`finalize.go`、`finalize_args.go`、`internal/model/model.go`、`internal/legacy/mediaservice/mediaservice.go`；gateway `routes_media.go`、`media_client.go`；七個 `mediaref` 的 Kind 行；A6 盤點的格式（前 80 行）。
- **跑過的指令類型**：`rg`（符號、呼叫端、計數）、`find`／`wc -l`、`git rev-parse`、`awk`／python 從測試檔抽方法名。沒有網路、docker、live API，沒有讀 `.env` 或 secrets（只看了 `.env.template` 的變數名與預設）。
- **剩餘不確定**：(1) §4 的斷言欄是從方法名推的，未讀測試本體；(2) 沒找到 R2 bucket 的生命週期規則設定（0.3 的「沒有依 category 的規則」是缺席證據，不是確證）；(3) legacy Go 服務目錄（seller、im、ai、review、invoice）在 compose 是否仍會被啟動沒查，§2.1 #5-#7 的去向因此標【推】；(4) `INTERNAL_AUTH_MODE=permissive` 時攔截器如何設定 `isInternalCall()` 沒讀（攔截器在 `common-core` 之外的 observability 或 grpc 模組，缺陷 5.9 的嚴重度因此是上限）；(5) CodeMeta 文件 `業務類型碼.md` 本身沒打開，22 這個數字來自三處生成物的計數；(6) `docs/詳細設計書/08`、`49` 號設計書沒讀（只引用計畫與 AGENTS.md 對它們的指向）。
