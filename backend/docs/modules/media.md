# media 模組行為規格

本文是單體 `media` 模組行為的唯一來源。圖片等靜態資產的轉正、引用登記與解除、機敏資產的讀取，以及擁有媒體的模組共用的引用同步。

## 1. 範圍與公開介面

- 實作是 `internal/legacy/mediaservice`：以 gRPC client 呼叫 Java 的 media-service，在 `cmd/hawkr` 接線。模組沒有自己的 schema 與 use case 子領域，登記在 `cmd/hawkr/modules.go` 的 `apiOnly`，不在 `modules()`。
- 根套件以型別別名公開 `internal/model` 的 `Uploader`、`Ref`、`Asset`、`SecureAsset` 與 `Media` interface，並公開 `ErrNotSecure`、`ErrStagedTokenRejected`。
- 引用同步的演算法在 `internal/refsync`，由 `media.RefSyncer`、`media.SyncChanged`、`media.SyncReject`、`media.SyncWork`、`media.SyncInsertOpts` 與 `Sync*` 型別公開。
- 測試用的記憶體內 `Fake`（`media.NewFake`）在 `fake.go`：模擬 `UNIQUE(asset, ref_type, ref_id)` 加 ON CONFLICT DO NOTHING、精確 Unlink，並能讓逾時之後才生效的 RPC 延後執行。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 2. 轉正（Finalize）

- `Media.Finalize(ctx, uploader, stagedToken, refType)` 把 staged token 轉成資產，不建立引用；冪等：同一個 token 在有效期間內回傳同一個資產。
- 上傳者（`Uploader`：`ID` 與是否為管理員）由呼叫方傳入，media 不自己反查登入者。帳號與管理員的 ID 各自發號、會同號，所以要帶上是哪一種。
- 回報 `Asset.Secure`；證件這類擁有者要求它為真。業務列只存 `ObjectKey`，顯示用的 URL 在讀取時組裝。
- 轉正了卻沒有任何引用的資產，由 media 的 GC 在資產建立滿 7 天後回收。
- 呼叫方自己傳錯的參數（缺上傳者、無效的 `ref_type`、空白的 token）由 `CheckFinalizeArgs` 在送出前擋下，是未分類的程式錯誤。
- media 拒絕某個 staged token（無效、過期、不屬於上傳者、已轉正的物件不符）時，錯誤可用 `errors.Is(err, ErrStagedTokenRejected)` 辨認，屬 InvalidInput 一類的使用者錯誤，重新上傳即可。Finalize 只在這個情況回使用者錯誤。
- 一次轉正多個 token 的清單（`Item`、`Field`）：每個被拒絕或失敗的元素一次全部回報，欄位指向被拒絕的 token；遇到非 token 的錯誤就停止。`Field.Message` 是該欄位給使用者看的文字，空字串沿用 media 的文字。

## 3. 登記與解除（Link／Unlink）

- `Link(ref, assetIDs)` 登記 ref 對資產的引用。冪等：已存在的引用不重複建立，可以重試。Link 不比對上傳者。
- `Unlink(ref, assetIDs)` 只解除精確列出的資產，不碰 ref 的其他引用；沒有的引用忽略。沒有「保留名單以外全刪」。
- Link 與 Unlink 都是單次呼叫，沒有內層重試；以服務身分簽署，不帶帳號。
- 四個呼叫（Finalize、Link、Unlink、ReadSecure）都在 DB transaction 之外：背後是 R2 與一個會逾時的服務，交易內等它會佔住連線池。

## 4. 讀取機敏資產（ReadSecure）

- `ReadSecure(objectKey, ref)` 回傳機敏資產（`secure/` 前綴、已解密）的內容、`ContentType` 與顯示用檔名（media 儲存的原始檔名，舊資產是 object key 的尾段）。
- 授權述詞是 `object_key`、`ref_type`、`ref_id` 三者與轉正當時全等；查無與越權無法分辨，都是 NotFound。
- 資產不是機敏資產時回 `ErrNotSecure`，與 NotFound 分開；呼叫方把它當成設定或資料的錯誤。
- 呼叫方必須先自己授權並記存取紀錄；media 只確認三元組。

## 5. 引用同步

引用只由擁有該業務列的模組的同步 job 寫入。

- 寫入端只做 Finalize；transaction 內寫業務列、把被換掉的資產記成退場（`SyncChanged`）、遞增 ref 的版本並排入同步 job。
- 同步 job 呼叫 `RefSyncer`：Link 業務列目前的資產，Unlink 退場的資產；結算與完成 job 在同一個 transaction。
- 擁有者各自以 `SyncQueries` 提供自己 schema 的 SQL（版本、退場、未同步的 ref）與目前資產的讀取。seq 協定、`SyncReject`（拒絕把已退場的資產寫回 ref，InvalidInput，須在鎖住業務列的 transaction 內呼叫）、結算與保底掃描只有 media 這一份。
- job 以 `(ref_type, ref_id)` 去重，最多重試 `SyncMaxAttempts`（25）次；重試退避封頂（`MaxBackoff`），下一次重試不晚於 GC 寬限。
- 重試用盡或 job 遺失，由各模組的保底掃描（`<模組>.media_sweep`）補排；排入有唯一性，重複補排安全。
- 目前指著的資產同時是退場資產時，同步仍然 Link 目前的、略過這些資產的解除、記 error log 並完成。

不變式：

- 業務列指著的每個資產，在 GC 寬限（資產建立起 7 天）內會有引用。
- 引用只由該 ref 的同步 job 寫入。
- 解除只解除退場的資產。

可觀測：同步失敗記 error log「media 引用同步失敗，會重試」（帶 `ref_type`、`ref_id`、`attempt`）。監控訊號在擁有者那一邊：各模組 `media_ref_sync` 中 `synced_seq < seq` 的最舊 `updated_at` 超過 `SyncMaxLag`（24 小時）時，保底掃描記 error log「media 引用同步落後過久」。

## 6. 引用型別

`ref_type.go` 的 `RefType*` 常數（AssetReferenceType）由 CodeMeta 生成（`業務類型碼.md`，`generate.sh --go-media`），不得手改。`CheckFinalizeArgs` 以這組常數為封閉集合。

## 7. 已知限制

- 實作是 Java 的 media-service；沒有 Go 端的 schema，資產與引用資料都在 Java 那一側。
- Java 沒有條件式刪除：第二次解除之後才在遠端生效的逾時 `LinkImages`（遲到超過約 15 秒），會留下一份退場資產的引用。方向是漏、不會誤刪，資產永遠不被 GC。
