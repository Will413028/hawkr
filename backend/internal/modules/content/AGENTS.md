# content 模組

營運維護、給前台顯示的內容：首頁 banner、App 版本門檻。來源是 content-service（遷移計畫 A1）。

## 職責

- banner（設計書 25）：後台維護；前台只看得到啟用中、而且在顯示期間內的。
- App 版本門檻（設計書 54）：每種 App（買家、賣家）一組最低版本；App 只發 iOS，沒有平台維度（2026-09-26 拿掉）。版本比較只在這裡做。

## 擁有的資料

schema `content`（`db/schema.sql`）：`banners`、`app_version_gates`，以及 media 引用的同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。

## 模組內部結構

- 根套件保留 `Module`、`New`、`Migrations`、公開型別別名與轉呼叫方法；整合測試以 `content_test` 經公開 API 驗證。
- 根套件的 `Deps` 是組裝 API；`internal/model` 只定義對外型別；`internal/banner`、`internal/appversion` 各自宣告最小 `Deps` 並實作 use case 和規則。
- `internal/images` 封裝 media 的圖片轉正；`internal/mediaref` 是 banner 圖片引用的同步（退場資產、版本與 `content.media_sync` job）；`internal/validation` 放共用的純輸入檢查；AIP-158 每頁筆數由 `internal/platform/pagination` 擁有；`internal/store` 是 sqlc 生成的查詢套件。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- banner 的點擊行為一定完整：型別是 popup、url、product、post 之一，並帶著該型別需要的值；url 只接受 http、https（設計書 25）。修改是整筆覆寫，空的行為會洗掉原本的設定，所以在寫入端檢查；DB 的 `action_type` 沒有預設值，`banners_action_check` 是第二道防線。
- banner 的排程是 [開始, 結束)：開始的那一刻上架、結束的那一刻下架，所以結束必須晚於開始。寫入端檢查，DB 的 `banners_schedule_check` 是第二道，兩者的判定由 `TestBannerScheduleCheckMatchesTheGoRule` 綁在一起。
- 圖片只在 transaction 外 `Finalize`（轉正，不建立引用）；banner 的兩張圖存 object key 與 asset ID（`preview_image_asset_id`、`popup_image_asset_id`，與 key 成對、`NOT NULL`：圖片必填）。引用只由 `content.media_sync` 同步 job 寫入（`internal/mediaref`）：新增、換圖與刪除在 transaction 內寫業務列、把被換掉的圖記成退場並排入同步；job 以 (ref_type, ref_id) 去重，Link 業務列目前的圖、精確 Unlink 退場的資產，沒有列的 ref（已刪除）沒有目前的資產。已退場的圖不能寫回。banner 的主鍵是應用層產生的 UUIDv7，所以 `Finalize` 不需要 ID。決策見 `docs/adr/2026-10-03-media-references-owner-side.md`。
- 修改與刪除以版本做樂觀鎖（ADR `2026-09-25-optimistic-concurrency-etag.md`）：banner、版本門檻各有 `version`，每次修改加一，對外是 etag。修改必須帶讀到的版本，刪除可以帶；轉正新圖之前先比對一次，最後以條件式 UPDATE（`WHERE version = $n`）為準，0 列時再分辨不存在與版本不符。回傳的內容與版本取自寫入當下（`RETURNING`），不在 commit 之後另外讀。
- 版本門檻以 key（app_type）識別：建立只建立（`ON CONFLICT DO NOTHING`，已經存在回 `AlreadyExists`，業務原因與版本衝突相同），修改要帶版本（ADR 的 D3）。沒有刪除，所以版本不必區分同一組 key 的不同建立。
- 版本門檻一律 fail-open：沒有設定、門檻或 App 送來的版本格式不對，都不要求更新。

## 已知限制

- 新圖要在 media GC 的 7 天寬限（從資產建立起算）內被同步 job Link：轉正之後、同步完成之前資產沒有引用。同步 job 失敗由 River 重試，重試用盡或 job 遺失由保底掃描（`content.media_sweep`）補排；監控訊號與落後上限見 ADR。B1 之後引用與業務列同一個 transaction，這個依賴消失。
- banner 的預覽與彈窗共用同一個 ref，同一個資產可以同時在兩個欄位：退場是「寫入前這個 ref 的所有資產減去寫入後的所有資產」，不逐欄位比對（`TestSharedAssetAcrossBannerColumnsIsNotRetiredWhileStillPointedAt`）。
- asset ID 欄位不 backfill：本機有舊列（沒有 asset ID）的開發資料庫要重灌（上線前沒有相容性約束）。

## 讀取模型

無。

## 不負責

- 圖片的儲存與回收：media。這裡只存 object key；對外的 `Image.uri_template` 由 `internal/httpapi` 組（ADR `2026-09-25-api-image-uri-template.md`）。
- 認證與授權：http 層。寫入方法收到的 `adminID` 只用來告訴 media 圖片是誰上傳的。
- 部分更新（field mask）：http 層讀目前的資料、套上 mask 內的欄位，再以整筆的內容呼叫修改方法。
- 分頁 token 的編碼：http 層。模組只收上一頁最後一筆的排序鍵（`BannerCursor`）。
- 瀏覽統計：沒有讀取端，不記錄（見 `CARRYOVER.md` 的沿用盤點）。

## 決策紀錄

從 content-service 沿用的每個機制、當時的決定與理由，以及與 content-service 的差異在同目錄的 `CARRYOVER.md`。改動或移除沿用的機制前，先讀那一列。
