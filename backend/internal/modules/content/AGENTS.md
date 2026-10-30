# content 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.instruction-lint.toml`）。行為規格與已知限制寫在 `docs/modules/content.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

營運維護、給前台顯示的內容：首頁 banner 與 App 版本門檻。banner 由後台維護，前台只看得到啟用中、在顯示期間內的；版本門檻每種 App（買家、賣家）一組，版本比較只在這裡做。行為規格的唯一來源是 `docs/modules/content.md`。

## 擁有的資料

schema `content`（`db/schema.sql`）：`banners`、`app_version_gates`，以及 media 引用的同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，沒有 `pub_*`。

## 模組內部結構

- 根套件保留 `Module`、`New`、`Migrations`、公開型別別名與轉呼叫方法；整合測試以 `content_test` 經公開 API 驗證。
- 根套件的 `Deps` 是組裝 API；`internal/model` 只定義對外型別；`internal/banner`、`internal/appversion` 各自宣告最小 `Deps` 並實作 use case 和規則。
- `internal/images` 封裝 media 的圖片轉正；`internal/mediaref` 是 banner 圖片引用的同步；`internal/validation` 放共用的純輸入檢查；每頁筆數由 `internal/platform/pagination` 擁有；`internal/store` 是 sqlc 生成的查詢套件。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加子領域時同步修改圖。

## 不變量

- banner 的點擊行為一定完整，url 只接受 http、https。守門：`TestBannerActionMustBeComplete`、DB 約束 `banners_action_check`。規格：`docs/modules/content.md` §2。
- banner 的排程是 [開始, 結束)，結束必須晚於開始。守門：`TestBannerScheduleCheckMatchesTheGoRule`、DB 約束 `banners_schedule_check`。規格：§2。
- 啟用中的 banner 最多 20 則。守門：`TestBannersCanOnlyBeActivatedUpToTheLimit`。規格：§2。
- 圖片只在 transaction 外 `Finalize`，不建立引用；key 與 asset ID 成對且必填。守門：`TestCreateBannerFinalizesBothImagesAndTheSyncJobLinksThem`（不建立引用）、`TestImageKeysAndAssetIDsArePaired`、DB 的 `NOT NULL`；transaction 外這一點無守門（只有規格）。規格：§3。
- 引用只由 `content.media_sync` 同步 job 寫入，job 以 (ref_type, ref_id) 去重；業務列寫入只記退場、不 Link 也不 Unlink。守門：`TestSyncJobIsUniquePerRefAcrossWrites`、`TestSyncRunsAgainWhenTheBannerChangesDuringTheRun`、`TestSyncStaysCorrectWhileWritesInterleaveWithRealRiver`。規格：§3。
- 退場是寫入前後該 ref 所有資產的集合差；已退場的圖不能寫回。守門：`TestRetiredIsASetDifferenceAcrossAllSlotsOfTheRef`、`TestSharedAssetAcrossBannerColumnsIsNotRetiredWhileStillPointedAt`、`TestUpdateBannerRetiresExactlyTheReplacedImages`。規格：§3。
- 重試用盡或 job 遺失由保底掃描補排。守門：`TestSweepRecoversALostEnqueueWithRealRiver`。規格：§3。
- 修改必須帶讀到的版本，以條件式 UPDATE 為準，回傳取自 `RETURNING`；刪除可以帶版本。守門：`TestUpdateBannerRejectsAStaleVersionBeforeFinalizing`、`TestUpdateBannerLosesToAWriteAfterItsVersionCheck`、`TestDeleteChecksTheVersionWhenGiven`。規格：§4。
- 版本門檻以 app_type 識別，建立只建立、修改要帶版本。守門：`TestAppVersionGateCreateOnlyCreatesAndUpdateNeedsTheVersion`、`TestConcurrentCreatesOfTheSameGate`。規格：§5。
- 版本門檻一律 fail-open：沒有設定、門檻或 App 送來的版本格式不對，都不要求更新。守門：`TestCheckAppVersion`。規格：§5。
- 新圖要在 media GC 的 7 天寬限內被同步 job Link。守門：`TestSyncBetweenFinalizeAndCommitKeepsThePointedAtBannerImages`。規格：§6。

## 不負責

- 圖片的儲存與回收：media。這裡只存 object key；對外的 `Image.uri_template` 由 `internal/httpapi` 組（ADR `2026-09-25-api-image-uri-template.md`）。
- 認證與授權、部分更新（field mask）、分頁 token 的編碼：http 層。模組只收上一頁最後一筆的排序鍵（`BannerCursor`）。
- 瀏覽統計：沒有讀取端，不記錄。

## 決策紀錄

從 content-service 沿用的每個機制、當時的決定與理由，以及與 content-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-10-03-media-references-owner-side.md`（引用由同步 job 單一寫入、7 天寬限與保底掃描）、`docs/adr/2026-09-25-optimistic-concurrency-etag.md`（版本與 etag；D3 版本門檻建立只建立）、`docs/adr/2026-09-25-row-identity-by-data-kind.md`（UUIDv7）。
