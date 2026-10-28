# content 模組行為規格

本文是單體 `content` 模組行為的唯一來源。模組維護給前台顯示的內容：首頁 banner（設計書 25）與 App 版本門檻（設計書 54）。

## 1. 範圍與公開介面

- 擁有 schema `content`：`banners`、`app_version_gates`，以及 media 引用的同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。沒有 `pub_*`，沒有讀取模型。
- 根套件保留 `Module`、`New`、`Migrations`、公開型別別名與轉呼叫方法；整合測試以 `content_test` 經公開 API 驗證。
- 根套件的 `Deps` 是組裝 API；`internal/model` 只定義對外型別；`internal/banner`、`internal/appversion` 各自宣告最小 `Deps` 並實作 use case 和規則。
- `internal/images` 封裝 media 的圖片轉正；`internal/mediaref` 是 banner 圖片引用的同步；`internal/validation` 放共用的純輸入檢查；每頁筆數由 `internal/platform/pagination` 擁有；`internal/store` 是 sqlc 生成的查詢套件。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。
- 寫入方法收到的 `adminID` 只用來告訴 media 圖片是誰上傳的；認證與授權在 http 層。
- 部分更新（field mask）在 http 層：讀目前的資料、套上 mask 內的欄位，再以整筆的內容呼叫修改方法。
- 分頁 token 的編碼在 http 層；模組只收上一頁最後一筆的排序鍵（`BannerCursor`），多取一筆判斷有沒有下一頁，不回總數。每頁筆數：0 用預設值 50、超過上限 100 以上限計、負數是錯誤。
- 瀏覽統計沒有讀取端，不記錄。

## 2. banner

- 後台維護；前台只看得到啟用中、而且在顯示期間內的（`ListLiveBanners`）。後台清單依 `display_order` 排序，以 keyset 翻頁。
- 點擊行為一定完整：型別是 `popup`、`url`、`product`、`post` 之一，並帶著該型別需要的值。`popup` 不帶值（遺留的值會被丟掉），`url` 只接受 http、https，`product` 與 `post` 的值只檢查非空，不確認存在。
- 修改是整筆覆寫，空的行為會洗掉原本的設定，所以在寫入端檢查；DB 的 `action_type` 沒有預設值，`banners_action_check` 是第二道防線。
- 排程是 [開始, 結束)：開始的那一刻上架、結束的那一刻下架，結束必須晚於開始。寫入端檢查，DB 的 `banners_schedule_check` 是第二道；兩者的判定一致。開始或結束可以不設。
- 按鈕文字最多 12 個字（以 rune 計）。
- 啟用中的 banner 最多 20 則，超過時上架回 `FailedPrecondition`；計算不含正在修改的那一筆。
- 主鍵是應用層產生的 UUIDv7，不設 DB 預設值。

## 3. 圖片與引用同步

- banner 的預覽圖與彈窗圖各存 object key 與 asset ID（`preview_image_asset_id`、`popup_image_asset_id`），key 與 asset ID 成對、`NOT NULL`：圖片必填。
- 寫入只帶新圖的 staged token，空字串表示沿用；沿用的 key 與 asset ID 取自模組自己讀到的現有資料。
- 圖片只在 transaction 外 `Finalize`（轉正，不建立引用）；`Finalize` 不需要 banner 的 ID。轉正被 media 拒絕時，錯誤指向該圖片的欄位，且 content 自己的驗證錯誤先於 media 的錯誤。轉正失敗時不寫入任何東西；banner 不存在時不碰 media。
- 引用只由 `content.media_sync` 同步 job 寫入（`internal/mediaref`）：新增、換圖與刪除在 transaction 內寫業務列、把被換掉的圖記成退場（`retired_assets`），並遞增該 ref 在 `media_ref_sync` 的 `seq` 後排入同步。
- 同步 job 以 (ref_type, ref_id) 去重，Link 業務列目前的圖、精確 Unlink 退場的資產；沒有列的 ref（已刪除）沒有目前的資產。結算時只在 `seq` 沒變才記下 `synced_seq`，job 執行中業務列又變動則再跑一次。`media_ref_sync` 的列不刪除。
- 已退場的圖不能寫回同一個 ref。
- banner 的預覽與彈窗共用同一個 ref，同一個資產可以同時在兩個欄位：退場是「寫入前這個 ref 的所有資產減去寫入後的所有資產」（集合差，去重），不逐欄位比對；刪除用同一個函式。
- 同步 job 失敗由 River 重試；重試用盡或 job 遺失由保底掃描（`content.media_sweep`）補排 `synced_seq < seq` 的 ref。
- 外部呼叫（轉正）不在 DB transaction 內。

## 4. 並發：版本與 etag

- banner、版本門檻各有 `version`，建立時為 1，每次修改在 UPDATE 明寫加一；對外是 etag。`updated_at` 同樣在 UPDATE 明寫 `NOW()`，不用 trigger。
- 修改必須帶讀到的版本；刪除可以帶，帶了就比對。
- 轉正新圖之前先比對一次版本；最後以條件式 UPDATE（`WHERE version = $n`）為準，0 列時再分辨不存在與版本不符。版本不符回 `VERSION_CONFLICT`（`ABORTED`），不寫入。
- 回傳的內容與版本取自寫入當下（`RETURNING`），不在 commit 之後另外讀。

## 5. App 版本門檻

- 每種 App（`BUYER`、`SELLER`）一組門檻，以 `app_type` 為主鍵，不對外暴露 id；App 只發 iOS，沒有平台維度。欄位：最低版本 `minimum_version`、商店網址 `store_url`、提示標題與內文。
- 最低版本必須是三段數字（`major.minor.patch`），DB 的 `app_version_gates_minimum_version_check` 是第二道。版本比較每一段當數字比，只在這個模組做。
- 商店網址以空字串表示沒有設定（`NOT NULL DEFAULT ''`），有值時只接受 `https://`、`itms-apps://` 開頭。
- 建立只建立（`CreateAppVersionGate`，`ON CONFLICT DO NOTHING`）：已經存在回 `AlreadyExists`，業務原因與版本衝突相同。修改（`UpdateAppVersionGate`）要帶版本。同時建立同一組 key，只有一個成功。
- 沒有刪除，所以版本不必區分同一組 key 的不同建立。
- 檢查一律 fail-open：沒有設定、門檻或 App 送來的版本格式不對，都不要求更新。

## 6. 已知限制

- 新圖要在 media GC 的 7 天寬限（從資產建立起算）內被同步 job Link：轉正之後、同步完成之前，資產沒有引用。
- asset ID 欄位不 backfill：本機有舊列（沒有 asset ID）的開發資料庫要重灌。
