# media 模組

## 職責

圖片等靜態資產的轉正、引用登記與解除、機敏資產的讀取，以及擁有媒體的模組（buyer、seller、content）共用的引用同步演算法。實作還是 Java 的 media-service（Java media-service 的規格見設計書 08）。行為規格的唯一來源是 `docs/modules/media.md`。

## 擁有的資料

無 schema，也沒有 use case 子領域；資產與引用資料在 Java media-service。登記在 `cmd/hawkr/modules.go` 的 `apiOnly`，不在 `modules()`。讀取模型：無。

## 發布的讀取介面

無。沒有 schema，所以沒有 `pub_*`。對外是 Go 公開 API：`Media` interface（`Finalize`、`Link`、`Unlink`、`ReadSecure`）與引用同步（`media.RefSyncer`、`media.SyncChanged`、`media.SyncReject`、`media.SyncWork`），形狀見 `docs/modules/media.md` §1。

## 模組內部結構

- 根套件以型別別名公開 `internal/model` 的 `Uploader`、`Ref`、`Asset`、`SecureAsset` 與 `Media`；`internal/refsync` 是引用同步的演算法，由 `media.Sync*` 公開；`fake.go` 是測試用的記憶體內 `media.NewFake`。
- `Media` 由 `internal/legacy/mediaservice` 呼叫 media-service 的 gRPC client 實作，在 `cmd/hawkr` 接線。
- `ref_type.go` 由 CodeMeta 生成（`generate.sh --go-media`），不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加子領域時同步修改圖。

## 不變量

- 轉正的上傳者由呼叫方傳入，media 不自己反查登入者；Finalize 不建立引用。守門：`TestFinalizeSignsCarriesTheUploaderAndCreatesNoReference`、`TestFinalizeCallerBugsAreCaughtLocally`。規格：`docs/modules/media.md` §2。
- Unlink 只解除精確列出的資產，沒有「保留名單以外全刪」。守門：`TestLinkAndUnlinkAreSignedWithoutAnAccountAndCarryExactAssets`、`TestSyncNeverUnlinksAnythingButRetiredAssets`（seller）。規格：§3。
- `Media` 的四個呼叫都在 DB transaction 之外。守門：無（只有規格）。規格：§3。
- ReadSecure 要 `object_key`、`ref_type`、`ref_id` 全等才讀得到，非機敏資產回 `ErrNotSecure`；呼叫方先授權並記稽核。守門：`TestReadSecureSendsTheTripleAndSeparatesNotSecureFromNotFound`。規格：§4。
- 引用只由擁有者的同步 job 寫入；業務列指著的資產在 GC 寬限（7 天）內會有引用。守門：`TestSyncBetweenFinalizeAndCommitKeepsThePointedAtAsset`（buyer）、`TestSyncBetweenFinalizeAndCommitKeepsThePointedAtBannerImages`（content）、`TestNextRetryNeverWaitsPastTheGCGrace`。規格：§5。
- 重試用盡或 job 遺失由保底掃描補排；落後超過 24 小時記 error log。守門：`TestSweepRecoversALostEnqueueWithRealRiver`（content）。規格：§5。
- `CheckFinalizeArgs` 的封閉集合等於 `ref_type.go` 的常數。守門：`TestRefTypesMatchTheGeneratedConstants`。規格：§6。

## 不負責

- 業務列、asset ID 欄位與每個 ref 的同步 job、版本與退場表：由擁有媒體的模組（buyer、seller、content）各自擁有，各用自己的 schema。
- 機敏下載的授權與存取紀錄：擁有者的端點負責（路徑與傳輸規則見〈API 設計規範（Connect）〉「檔案回應」）。
- 儲存層與圖片讀取出口：Java media-service 與 49 號設計書。

## 決策紀錄

實作還在舊服務，尚無 `CARRYOVER.md`（實作搬進單體時建立）。相關 ADR：`docs/adr/2026-10-03-media-references-owner-side.md`（引用由擁有者端的同步 job 單一寫入，終點是 media 不認識業務；文末修訂：機敏下載由擁有者端點代理）、`docs/adr/2026-09-24-go-modular-monolith.md`（D2）。
