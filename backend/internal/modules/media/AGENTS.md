# media 模組

圖片等靜態資產的上傳、轉正、引用與回收（設計書 08）。

## 現況：公開 API 與共用的引用同步

實作還是 Java 的 media-service，遷移計畫 B1 才改寫成 Go。在那之前：

- 根套件以型別別名公開 `internal/model` 的 `Uploader`、`Ref`、`Asset` 與 `Media` interface，以及引用同步的演算法（`internal/refsync`，由 `media.RefSyncer`、`media.SyncChanged`、`media.SyncReject`、`media.SyncWork`、`media.SyncInsertOpts` 與 `Sync*` 型別公開）；測試用的記憶體內 `Fake`（`media.NewFake`）在 `fake.go`。尚無搬入的資料，所以沒有 schema，也沒有 use case 子領域。
- `Media` 由 `internal/legacy/mediaservice` 呼叫 media-service 的 gRPC client 實作，在 `cmd/hawkr` 接線。
- 沒有 schema，所以登記在 `cmd/hawkr/modules.go` 的 `apiOnly`，不在 `modules()`。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。B1 搬入資料時新增子領域並同步更新圖。

## 公開 API 的規則

- 轉正時由呼叫方傳入上傳者（`Uploader`），media 不自己反查登入者（ADR 2026-09-24-go-modular-monolith D2）。
- `Media` 有四個呼叫，都在 DB transaction 之外：`Finalize`（轉正，不建立引用；回報 `Asset.Secure`，證件這類擁有者要求它為真）、`Link`（登記引用，冪等）、`Unlink`（只解除精確列出的資產）、`ReadSecure`（讀機敏資產的內容，三元組 `object_key`、`ref_type`、`ref_id` 全等才讀得到，不是機敏資產回 `ErrNotSecure`；呼叫方先自己授權並記稽核）。沒有「保留名單以外全刪」。
- 引用只由擁有該業務列的模組的同步 job 寫入：寫入端 `Finalize`，transaction 內記退場並排入同步；job 呼叫 `media.RefSyncer`（Link 目前的、Unlink 退場的，結算與完成 job 同一個 transaction）。擁有者各自以 `SyncQueries` 提供自己 schema 的 SQL（版本、退場、未同步的 ref）與目前資產的讀取；seq 協定、拒絕寫回、結算與保底掃描只有這一份。決策見 ADR `docs/adr/2026-10-03-media-references-owner-side.md`；設計書 08 §4.6.2。
- 不變式：業務列指著的每個資產，在 GC 寬限（資產建立起 7 天）內會有引用；引用只由該 ref 的同步 job 寫入；解除只解除退場的資產。同步由 River 重試、重試用盡或 job 遺失由各模組的保底掃描（`<模組>.media_sweep`）補排；監控訊號在擁有者那一邊：`media_ref_sync` 中 `synced_seq < seq` 的最舊 `updated_at` 超過 24 小時（掃描記 error log）。
- `ref_type.go` 由 CodeMeta 生成（`業務類型碼.md` 的 AssetReferenceType，`generate.sh --go-media`），不得手改。

B1 時補上職責、擁有的資料、不變量與讀取模型，並把實作搬進本模組：media 只存資產與生命週期（暫存 → 已認領 → 刪除）、用途與可見性，引用只在擁有者的資料表，刪除是擁有者以 outbox 發出的指令，GC 只過期未認領的暫存上傳，機敏下載由擁有者的端點授權、先記稽核再代理（`Media.ReadSecure`，B1 只換它後面的 adapter；不用簽章網址：記不到讀取，R2 存的是 media 加密過的密文）；同步 job 與各模組的退場表一起消失。
