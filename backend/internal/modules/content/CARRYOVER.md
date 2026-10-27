# content 模組的沿用盤點

從 content-service 沿用的每個機制重新判斷的結論（遷移計畫「模組完成的定義」），以及與 content-service 的差異。模組現行的職責、資料與不變量在同目錄的 `AGENTS.md`。

## 沿用盤點

content-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）。對外 API（舊 gateway 的 content handler 與 go-pkg）的機制在下一節「傳輸層的沿用盤點」。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 主鍵用 TSID（`go-pkg/tsid`）與 `BIGSERIAL` | 多個資料庫各自發號，靠 node 編號避免撞號 | 不成立：單一資料庫、多個 process 寫入，也沒有 node 編號 | UUIDv7（RFC 9562） | 改 UUIDv7（ADR `2026-09-25-row-identity-by-data-kind.md`） |
| 沿用圖片時 client 送 key，只檢查格式（`go-pkg/mediaurl.ValidateObjectKey`） | 契約裡有 key，client 送來的 key 只可能格式錯 | 不成立：對外契約沒有 key（ADR `2026-09-25-api-image-uri-template.md` D1），格式對的 key 也可能屬於別筆資料 | 更新只表達「換新或不換」，不在 mask 裡的欄位不變（AIP-134） | 寫入只帶新圖的 staged token，空字串表示沿用；沿用的 key 由模組取自自己讀到的現有資料 |
| 圖片的順序契約：先轉正並登記引用、再寫業務列、之後釋放舊引用（`go-pkg/media`） | media 在另一個服務，不能和業務列同一個 transaction | 登記與釋放引用的先後要由擁有業務列的一方序列化；「先登記、後釋放」加 keep 快照在並發下會刪掉剛登記的引用 | 引用與業務列同一個 transaction（Active Storage）；引用在別的系統時，以單一寫入者序列化並只精確移除被取代的資產 | `Finalize` 在 transaction 外、不建立引用；業務列存 asset ID，新增、換圖、刪除在 transaction 內記退場並排入 `content.media_sync`，job 是唯一的寫入者（`docs/adr/2026-10-03-media-references-owner-side.md`）；B1 後引用與業務列同一個 transaction，job 與退場表消失 |
| 釋放舊引用在 commit 之後同步呼叫、失敗只記 log | 釋放要在業務列寫入之後 | 不成立：同步呼叫失敗就永遠漏放；原本否決 job 的理由（job 延遲拉長競態窗口）在單一寫入者之下不存在 | 在寫入的 transaction 內排入帶重試、以目前狀態收斂的 job | 改為同步 job（上一列）：寫入者不 Link、不 Unlink，延遲只影響「新圖多久被登記」，受 7 天寬限約束，不影響已登記引用的安全 |
| banner 只換一張圖時不釋放舊引用 | 保留名單只認 asset ID，業務列只存 object key | 不成立：業務列存 asset ID 之後，被換掉的那一張可以精確退場 | 精確解除被取代的資產 | 兩張圖各自比對：被換掉的記成退場、沿用的不動（`TestUpdateBannerRetiresExactlyTheReplacedImages`） |
| 逐位置比對決定哪張圖退場（舊的 `ReleaseReferences` 以 keep 名單，等同逐位置語意） | banner 兩張圖用同一個 ref | 不成立：預覽與彈窗可以是同一個資產，換掉其中一張時舊資產仍被另一張指著，退場它會讓同步失敗 | 以集合表示 ref 的資產：退場 = 寫入前的集合減寫入後的集合 | `images.Retired(old, after)` 取集合差並去重，刪除用同一個函式（`TestSharedAssetAcrossBannerColumnsIsNotRetiredWhileStillPointedAt`、`TestRetiredIsASetDifferenceAcrossAllSlotsOfTheRef`） |
| asset ID 欄位允許 NULL（相容舊列、不 backfill） | 上線前的開發資料沒有 asset ID | 不成立：系統從未上線，沒有相容性約束（根 AGENTS.md） | 欄位依不變式宣告：與 key 成對、`NOT NULL`（buyer 的 `buyers_avatar_asset_check` 是先例） | `preview_image_asset_id`、`popup_image_asset_id` 都是 `NOT NULL`，不 backfill，本機舊列的開發資料庫重灌（`TestImageKeysAndAssetIDsArePaired`） |
| 點擊行為存成 JSONB（`banners.action`），完整性只在 Go 端檢查 | admin 經 gateway 送來整段 JSON 字串，原樣存下 | 不成立：契約是 proto 的 `oneof`，四種變體固定 | 固定的變體用欄位加 CHECK，不變量由 DB 保證（PostgreSQL 設計規範 §六：JSONB 要說明為何不用欄位） | 改成 `action_type`、`action_target` 加 `banners_action_check`（migration `banner_action_columns`） |
| 點擊行為的 url 只檢查非空 | 後台填的網址一定合法 | 不成立：設計書 25 🔴 規定 url 只能 http、https，不檢查等於讓後台對 App 觸發任意 scheme；同模組的商店網址已在寫入端限制 scheme | 寫入端限制 scheme | 模組只收 http、https（`BannerAction.validate`），DB 的 CHECK 是第二道 |
| 點擊行為的商品、貼文 ID 只檢查非空，不確認存在 | 擁有者模組能被查詢 | 不成立：catalog、social 還是舊服務，ID 格式也會隨它們搬進單體從 TSID 改成 UUID | 經擁有者模組的公開 API 確認存在（ADR D2） | 現在只檢查非空；catalog、social 搬進單體時，依賴表加上它們並以公開 API 確認存在 |
| 商店網址 `store_url` 可為 NULL，NULL 表示沒有設定 | 需要區分 NULL 與空字串 | 不成立：沒有地方區分，proto 也以空字串表示沒有設定 | 「沒有設定」只有一種表示法 | 改 `NOT NULL DEFAULT ''`（同一個 migration） |
| 站點文案 `site_contents` 的後台 CRUD | 前台有地方讀 | 不成立：website 的 `lib/site-content` 已退役並被 `check-architecture.ts` 禁止 import，App 沒讀，gateway 也沒有公開的 endpoint。唯一的讀取方是 gateway 的退貨文案 SSE（key `return-policy`，沒有值時退回 ai-service 的固定文案 `ReturnPolicyFacts`），盤點時漏列；後台頁刪除後已經寫不進值 | 同上 | 拿掉（Will 2026-09-25，同一個 migration）；admin 的文案頁在試點步驟 6 刪除，退貨文案 SSE 在刪除 content-service 時改成一律由 ai-service 回固定文案。真的需要以 key 管理文案時，連讀取端一起設計 |
| 並發修改：最後寫入者獲勝（設計書 50 §7.4、54 §5.3，不加樂觀鎖） | 修改頻率低，而且衝突無害 | 不成立：頻率低只讓衝突少見；整筆覆寫會靜默蓋掉別人的修改，換圖時還會讓 GC 刪掉正在顯示的圖 | 資源帶 etag，修改時比對（Google AIP-154、RFC 9110 的 `If-Match`） | 改成 etag（ADR `2026-09-25-optimistic-concurrency-etag.md`，Will 2026-09-25 決定）：`version` 欄與條件式 UPDATE（migration `etag_versions`），見 `AGENTS.md` 的「不變量」 |
| 版本門檻的 upsert（`ON CONFLICT DO UPDATE`） | 設定只有一份，覆寫無害 | 不成立：兩個人同時設定時，後寫入的一方靜默蓋掉前一方 | 以 key 建立只建立、修改帶版本（ADR 的 D3；RFC 9110 的 `If-None-Match: *` 與 `If-Match`） | 拆成 `CreateAppVersionGate` 與 `UpdateAppVersionGate` |
| 次要索引：`banners` 的 `(is_active, display_order)` | 表大到全表掃描太慢 | 不成立：由後台維護、筆數少 | 小表不建次要索引，等 EXPLAIN 證明需要 | 拿掉（migration `drop_unmeasured_indexes`）；EXPLAIN 顯示需要時再加 |
| 分頁用頁碼加 `OFFSET`（`go-pkg/pagination`） | 前端以頁碼跳頁、需要總數 | 不成立：沒有畫面跳頁或讀總數，後台清單寫死第一頁 | 對外用不透明的 page token（Google AIP-158）；內部用 keyset，翻頁途中有新增或刪除也不重複、不漏 | 模組收上一頁最後一筆的排序鍵（`BannerCursor`），多取一筆判斷有沒有下一頁；token 的編碼在 http 層。每頁筆數照 AIP-158（0 用預設值、超過上限以上限計、負數是錯誤）。不回總數 |
| `updated_at` 在 UPDATE 語句明寫 `NOW()`，不用 trigger | 設計書 50 規定明寫；PostgreSQL 設計規範 §四 原本規定 trigger | 兩份唯一來源矛盾，2026-09-25 由 Will 決定採明寫 | 兩種都常見：trigger 保證任何 UPDATE 都刷新；明寫才能讓計數這類更新不刷新 | 沿用明寫；規範 §四 改寫成單體的規則。`version` 同樣在每支 UPDATE 明寫加一 |
| 文字欄位 `VARCHAR(n)` 的上限（255、100、50） | 產品上需要長度上限 | 沒有證據：255 是 MySQL 時期的慣用值（content-service 的 migration 註解寫「migrated from MySQL」），設計書 54 的 `VARCHAR(255)` 是欄位型別，不是產品上限；找得到的產品上限只有後台 banner 按鈕文字的 12 字，與 DB 的 100 不一致 | 上限從產品規格來，沒有就用 `text`（PostgreSQL 設計規範 §一、PG wiki「Don't use varchar(n) by default」） | 改 `text`（Will 2026-09-25 決定，migration `text_columns`）；只留產品規格有的上限：banner 按鈕文字 12 字，與後台編輯器一致。其他欄位的大小由傳輸層的 request 上限擋 |
| 錯誤用 `go-pkg/errors`（對應 gRPC status） | 錯誤經 gRPC 回給 gateway | 不成立：單體內直接呼叫 | 模組只回報錯誤的種類，狀態碼由傳輸層決定 | 改成 `platform/apperr` |
| log 用 zap（`go-pkg/logger`）；時間字串用 `go-pkg/timeutil` 解析 | 共用套件 | 不成立 | 標準庫 `log/slog`；時間在傳輸層解析，模組收 `time.Time` | 改 slog；timeutil 不搬 |
| App 版本比較：x.y.z 三段數字、格式不對一律 fail-open | App 版本號格式固定，檢查失敗不能擋住使用者 | 成立（設計書 54） | 同 | 沿用 |
| 管理員上傳圖片的身分用 CodeMeta 的 `AccountType`（`codemeta.AccountTypeAdmin`） | Java media-service 的舊契約用數字帳號類型判斷上傳者 | content 已不用 CodeMeta；傳 `media.Uploader{Admin: true}`，僅 `internal/legacy/mediaservice` 轉成舊數字碼 | 上傳者的輸入型別由 media 定義，舊 wire 值留在轉接層 | 改用 `media.Uploader`；B1 改寫 Java media-service 時移除轉接層的數字碼 |
| 圖片引用的類型用 CodeMeta 的 `AssetReferenceType` 數字碼（banner 7109，`media.RefType*`） | 引用類型由 Java 的 media-service 解讀，值域要跨語言一致 | 成立到 B1：media 仍是 Java 服務 | 引用類型屬於 media，由 media 模組定義 | 沿用；`media.RefType*` 是 `generate.sh --go-media` 的生成物（遷移計畫 0.2），B1 改寫 media 時值域移進 media 模組，並決定是否保留數字碼 |

## 傳輸層的沿用盤點

舊的對外 API 是 gateway 的 content handler 加上 content-service 的 gRPC；目前的 Connect 入口在 `internal/httpapi`（慣例在〈API 設計規範（Connect）〉），gateway 只服務尚未搬入單體的 REST、SSE 與 WebSocket。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 身分以明文的 `X-Account-Id`、`X-Account-Type` 由 gateway 傳入 | gateway 是對外入口，單體信任它帶來的身分 | 不成立：入口已移到單體，顧客 Bearer／Clerk cookie 與後台 Access JWT 在入口驗證；明文 header 只由單體送給本機與 CI 的 legacy gateway | 在唯一入口驗憑證，內部轉接只傳已驗證的 principal | 已改：content 收經入口授權的 adminID；gateway 的身分 header 僅在 `internal/legacy/gatewayproxy` 產生，舊服務不部署 |
| 錯誤是 envelope（`code`、`message`、`traceId`），gateway 把 gRPC 錯誤轉成 HTTP | REST＋envelope 的契約 | 不成立：契約是 Connect | Connect 的錯誤碼加 details（AIP-193） | 模組回 `apperr`（驗證錯誤以 `InvalidInputOn` 指出模組輸入的欄位名，handler 以 `rpc.WithFieldPaths` 對應到請求的路徑，對應不到時以 error 記 log），`internal/httpapi/rpc` 是唯一的轉換點：`google.rpc` 的標準 details，使用者文字放 `LocalizedMessage`，欄位錯誤是 `BadRequest`，client 需要分支時才帶 `ErrorInfo`（reason 是 `api/` 的 `ErrorReason`）；下游的錯誤一律 `internal`，下游的訊息只進 log；interceptor 之前的錯誤由 `rpc.LogUnhandled` 補 log（ADR `2026-09-25-api-error-model.md`） |
| gateway 的中介層寫 REST envelope，Connect 以 ResponseWriter 轉接錯誤 | Connect 先經 gateway 的 REST 中介層 | 不成立：gateway 的 Connect 代理已刪，單體是 Connect 入口 | 入口中介層直接寫 Connect 錯誤 | 已移到 `internal/httpapi`，用 `connect.NewErrorWriter` 寫認證與限流等錯誤；不再維護 gateway 的攔截式轉換 |
| 回應組 `*ImageUrl`（gateway 以 `MEDIA_BASE_URL` 組原圖網址） | client 需要圖片網址 | 部分成立：client 需要的是各 preset 的網址，而網域與路徑不該進 App 的 binary | 伺服器回可用的網址或樣板，client 只選尺寸（Twitch、Spotify、Shopify、Cloudflare Images） | 回 `Image.uri_template`，由 `internal/httpapi` 以 `MEDIA_ORIGIN` 組，client 只換 `{preset}`；key 不對外（ADR `2026-09-25-api-image-uri-template.md`，取代試點步驟 3 的「只回 key」） |
| App 類型與平台用 CodeMeta 的 AppType、AppPlatform（字串的 Value，proto 以 `string` 帶） | 多個服務、多種語言共用一份代碼表 | content 不成立：content 的契約是 proto，client 由 proto 生成。CodeMeta 的 AppType 仍被 gateway、payment、aftersales 使用（PayUni 回調導向買家或賣家版），到遷移計畫階段 C 才退場；AppPlatform 只有 gateway 的舊版本檢查與 content 使用 | 對外列舉在 IDL 定義（AIP-126），儲存的碼由擁有資料的模組定義 | content 定義自己的 `AppType`（值＝proto 值名去掉前綴＝DB 存的值），對外是 proto enum；CHECK 與 Go 的值域由 integration 測試比對，proto 與 Go 由 `rpc.EnumMismatches` 雙向比對（ADR `2026-09-25-codemeta-retirement.md` D3）。AppType 在 CodeMeta 與 content 是同一個概念（買家版或賣家版），payment 搬進單體時移到 `common`（同一 ADR 的 D1：兩個以上模組用到）。App 只發 iOS，沒有平台維度：不設 AppPlatform、`platform` 欄位與 proto 的 `AppPlatform`，CodeMeta 的 AppPlatform 已刪除 |
| 公開讀取不設 `Cache-Control`、沒有 ETag | 不需要 HTTP 快取 | 不成立：讀取改成 GET，要能讓 CDN 與瀏覽器重新驗證 | `Cache-Control` 加 ETag 與條件請求 | 公開讀取 `public, no-cache` 加回應雜湊的弱 ETag、304；後台 `private, no-store`。`max-age` 連同 website 的失效流程在 CD 驗證 Cloudflare 時決定（API 設計規範（Connect）§九） |
| 請求大小由 gateway 的 `BodyGuard` 擋（2 MiB） | 服務自己不擋 | 不成立：模組的文字欄位不設上限，大小要由傳輸層擋 | 服務自己設上限 | 單體 `connect.WithReadMaxBytes` 2 MiB，與 gateway 相同 |
| 公開讀取的限流級別（舊 gateway 的 `/api/v1/public` 是 moderate） | 匿名的公開讀取與其他路由分開限流 | 成立：Connect 的公開 service 仍需獨立額度 | 依 service 的 audience 分級，入口集中計數 | 單體的 `internal/httpapi/ratelimit.go` 以 `rpc.AudienceOf` 在 `tierOf` 選 `tierPublic`，不再使用 gateway 的 `classifyTier` |
| trace：gateway 開 span，下游以 gRPC metadata 接續 | gateway 是第一個可信入口 | 不成立：單體是入口，client 的 `traceparent` 不可信 | 在可信入口開新 trace，轉送 legacy 時注入 W3C Trace Context | 已由單體開新 trace；`internal/legacy/gatewayproxy` 轉送時注入 `traceparent`，不接續 client 的值 |
| 呼叫 media 的逾時與重試：轉正 10 秒、建立引用每次 5 秒最多 3 次（`go-pkg/media`） | 呼叫方沒有整體的 deadline | 不成立：interceptor 給每個請求 15 秒的 deadline | 每次呼叫的預算是上限，實際以請求剩下的時間為準 | 沿用這些值當上限；它們由請求的 context 截斷，兩張圖都很慢時請求以 deadline_exceeded 失敗、不寫入業務列（轉正過的圖由 GC 依孤兒規則回收）。B1 後沒有這段遠端呼叫 |
| 呼叫 media 以 grpc-go 與服務間 HMAC 簽章（`go-pkg`） | 單體要呼叫 Java 的 gRPC | 成立到 B1 | 同一套 RPC 框架 | 以 connect-go 的 gRPC 協定（h2c）呼叫，stub 由 buf 生成到 `internal/legacy/gen`，簽章照設計書 52 的格式自己算；單體不依賴 grpc-go 與 go-pkg |
| 修改時 transport 讀一次（套 field mask）、模組再讀一次（轉正前比對版本） | field mask 在 transport、版本比對在模組 | 成立 | 模組接收 patch 自己合併，或 transport 把讀到的版本交給模組 | 接受：後台的修改少，多一次主鍵讀取的成本可以忽略；field mask 是契約的語意，留在 transport |
| gateway 的 Connect 代理清單與單體的清單各寫一份 | gateway 與單體分屬 Go module，且 gateway 必須代理 Connect | 不成立：gateway 已不代理 Connect | 只在實際入口登記 Connect service | 已刪 gateway 的代理清單；單體的 `connectServices` 對齊 `api/` 契約 |

## 與 content-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline：

- 食譜功能已刪除（ADR `docs/adr/2026-10-27-general-ecommerce-scope-cut.md`），表由 migration `drop_recipes` 移除。
- 拿掉 `banners.click_count`：從來沒有被遞增，也沒有對外暴露，而且每次修改 banner 都被寫回 0。
- 點擊行為從 JSONB 的 `banners.action` 改成 `action_type`、`action_target` 兩欄加 CHECK；url 只接受 http、https（原本只檢查非空）。
- `banners.id`（TSID）改成應用層產生的 UUIDv7，不設 DEFAULT（ADR `2026-09-25-row-identity-by-data-kind.md`）。
- 排序不再拿 ID 代表建立時間：banner 依 `display_order, created_at`，ID 只當最後的 tiebreaker。
- 文字欄位從 `VARCHAR(n)` 改成 `text`，長度上限只留 banner 按鈕文字的 12 字（原本 DB 與寫入端都是 100，後台編輯器限 12）。
- 刪除不存在的 banner 回 NotFound（原本回成功）。
- 修改時不能指定要沿用的 key，只能換新或沿用這筆資料現有的圖（原本送 key、只檢查格式，可以指向別筆資料的圖；對方換圖或刪除後，GC 會刪掉這邊還在顯示的圖）。
- 版本檢查收到不認得的 app_type 回 InvalidInput（原本當成沒有設定而放行）。設計書 54 §4.4 規定值域由 content 守；App 收到錯誤時本來就 fail-open。
- 拿掉站點文案（`site_contents`）：沒有讀取端（見沿用盤點）。後台清單以 keyset 翻頁，不回總數。
- 商店網址沒有設定時存空字串，不再是 NULL。
- banner 的排程改成 [開始, 結束)，開始等於結束不再合法；DB 加上 `banners_schedule_check`（原本只有寫入端檢查，而且允許開始等於結束）。
- banner、版本門檻加上版本（etag），修改必須帶讀到的版本；版本門檻的 upsert 拆成只建立與帶版本的修改。
- 版本檢查沒帶 App 的版本時回 InvalidInput（原本由 gateway 回 400）。
