# A9-4 盤點：修圖、配額與後台風格（唯讀，2026-11-23，讀 `origin/dev`）

縮寫：`OLD=backend/services/ai-service`、`U=OLD/internal/usecase`、`H=OLD/internal/grpc/handler.go`、`GW=backend/gateway/internal`、`M=backend/internal/modules/imageai`、`P=api/hawkr/imageai/v1alpha1/imageai.proto`、`PA=api/hawkr/admin/imageai/v1alpha1/imageai.proto`、`IA=interface-inventory.md`、`DI=domain-inventory.md`、`CI=client-inventory.md`。
標記：【量】讀過原始碼／跑過指令（`git -C <repo> show|grep origin/dev …`）；【推】推論。

## 0. 數字與範圍

- 舊 RPC 9 支：`AIImageService` 2（`TransformImage`、`ListAiStyles`）＋`AIAdminService` 7（IA §1.1 #1–9）【量】。`RecommendationService.PersonalizedRecommend` 依 A9-D2 刪除，不在本步。
- gateway REST 9 條：`GW/router/routes_ai.go:14,18`（app 2）、`:25-28`（admin styles 4）、`:34-36`（admin quotas 3）【量】。
- 舊表 4 張（`OLD/db/schema/schema.sql:67-118`）：`ai_styles`、`ai_usage`、`ai_quotas`、`ai_usage_reservations`【量】。依 A9-D9 新 schema 應為 3 張（不建預約表）【推】。
- 新契約已在 `origin/dev`：P（`ImageAiAppService` 2 支）、PA（`ImageAiAdminService` 7 支）、`error_reason.proto:131-140` 的 `IMAGE_*` 4 個 reason；生成碼 `backend/internal/httpapi/gen/hawkr/imageai/...` 也在【量】。尚未掛載：`backend/internal/httpapi` 內除生成碼外無 `imageai` 命中（`git grep -n -i imageai origin/dev -- backend/internal/httpapi backend/cmd backend/internal/arch | rg -v /gen/` 只命中 `cmd/hawkr/modules.go:69`、`arch/modules.go:41,85`）【量】。`TestEveryServiceInTheContractIsMounted` 依計畫 3b-3d 列仍缺 imageai【量：計畫進度表】。
- 模組骨架 M 存在：`imageai.go`（空 `Deps`）、`internal/{feature,model,store}` 空殼、`CARRYOVER.md`（機制表 11 列、傳輸層表空）、`AGENTS.md`；`arch/modules.go:41` 依賴 `seller`、`media`；`cmd/hawkr/modules.go:69` 註記「骨架，表隨 A9-4 建立」（`withoutData`）【量】。行為規格 `backend/docs/modules/imageai.md` 只有骨架一句【量】。
- 計畫的一致性問題：缺陷表把 AD8、AD17 標在 A9-5（`plan.md` 缺陷表），但 A9-4 範圍寫「config（AD17）」與「同 tx 呼叫 `adminaudit.Record`（AD8）」【量】。本盤點以 A9-4 範圍為準，建議主 session 把缺陷表兩列改成 A9-4【推】。

## 1. 舊實作逐項

欄位：位置｜行為｜既有缺陷｜去向

### 1.1 TransformImage（app）

| 項 | 位置 | 行為 | 既有缺陷 | 去向 |
|---|---|---|---|---|
| REST 入口 | `GW/handler/ai.go:247-292`；body 上限 `GW/middleware/body_guard.go:13,52`（10 MiB、120s） | multipart `image`＋`prompt`；`prompt` 空回 400；MIME 取 multipart header，空字串當 `image/jpeg`；回 `imageBase64` 等 4 欄【量】 | MIME 完全信任 client header，不驗 magic bytes（`:272-275`）【量】 | 刪（A9-6）；新入口是 Connect unary，MIME 由伺服器 sniff（`http.DetectContentType`）並以白名單驗證【推】 |
| gateway→ai gRPC | `GW/grpc/ai_service_conn.go:30-31` | 送收上限 20 MB【量】 | A9-1b 移交的「20MB 對規範 2 MiB」就是這個數字；實際生效的入口上限是 10 MiB（body_guard），20 MB 只是 gRPC 層【量】 | 刪；新上限見 §2.3、§5 Q1 |
| 身分 | `H:426-435`、method policy `H:477-479`（`ActorSeller`） | 從 gateway 注入的 metadata 取 store_id【量】 | 信任 header（M2/AD8 同源）【量：DI:115】 | 改：handler 由 session 取 account，經 `seller.CurrentStore`（`modules/seller/seller.go:464`）解出 store；非賣家回 permission_denied（P:11）【量＋推】 |
| admission | `U/transform_admission.go:11-68` | 每 pod 一個 byte 預算 semaphore（預設 256 MiB，需求＝圖大小×6），`TryAcquire` 失敗回 `InFlightLimitError`→gRPC ResourceExhausted（`H:440-443`）【量】 | AD17：`os.Getenv("TRANSFORM_INFLIGHT_BYTES_BUDGET")` 繞過 config（`:44`），且解析失敗靜默用預設（`:48-51`）【量】 | 改寫：入口上限固定後，byte 預算等價於「同時幾張」；改成併發數 semaphore，上限走 `platform/config`（AD17 修法），失敗回 resource_exhausted＋RetryInfo（P:18）【推】→ §5 Q5 |
| 未設定金鑰 | `U/transform_usecase.go:81-83` | 回未分類 error→handler 回 code 500 通用訊息【量】 | 與 LLM 停用語意不一致 | 改：`llm.Model` 停用時回 `llm.ErrDisabled`（unavailable）（`platform/llm/llm.go:69`）【量】 |
| 額度上限 | `U/transform_usecase.go:131-140`；`domain/usage.go` `DefaultWeeklyLimit = 10` | 有 `ai_quotas` 列用它，否則 10【量】 | 非 `ErrNoRows` 的 DB 錯誤才回錯，OK；但 limit 是程式常數，CARRYOVER M9 列寫「額度數字照現在 config 搬」與事實不符（不是 config）【量】 | 保留語意（個別覆寫＞預設）；預設值放哪 → §5 Q4 |
| 預約→確認／釋放 | `U/transform_usecase.go:101-121,142-237`；SQL `OLD/db/queries/usage.sql:1-37` | tx 內 `pg_advisory_xact_lock(hashtextextended('ai_usage_store', store_id))`，`confirmed+pending>=limit` 拒絕，否則插預約列（TTL 5 分）；Gemini 成功後另一 tx 刪預約＋`ai_usage`+1；失敗刪預約【量】 | (a) confirm 失敗只 log（`:118-121`），預約 5 分鐘後過期→該次修圖不計次（額度少扣）【量】；(b) 過期預約只在下一次 reserve 時順手刪 1 天前的（`usage.sql:23-24`）【量】；(c) `limit<=0` 時回 `Usage:0`（`:97-99`），訊息顯示 0/0【量】 | 改寫（A9-D9）：一列計數器＋條件 UPDATE，照 procurement `ConsumeUsage`／`RefundUsage`（§3.4）；呼叫前扣、失敗退；刪預約表與 advisory lock |
| Gemini 呼叫 | `U/transform_usecase.go:62,248-420` | 手寫 `net/http` POST `gemini-2.5-flash-image:generateContent`，`x-goog-api-key`，`responseModalities:["IMAGE"]`，60s timeout、不跟 redirect（`:292-297`）；prompt 外包一段中文系統說明（`:299-301`）；無圖時依 `blockReason`／`finishReason` 分類為 `ProviderRefusalError`，可重試的再呼叫一次（`:314-335`）【量】 | 手寫 HTTP 是 A9-1c 架構測試抓不到的建構點（計畫 A9-4 範圍）；最壞 2×60s＝120s，與 gateway 120s read deadline 同量級【量＋推】；模型名寫死（`:62`）【量】 | 改寫：經 `platform/llm` 建構（§3.1），模型名進登記表；retry 一次的規則保留（IMAGE_NOT_PRODUCED 才重試）；deadline 在呼叫端明確設定【推】 |
| 錯誤對應 | `H:393-468` | quota→`Code 4291`（REST 429）；refusal→9250/9251/9252；unavailable→9253（REST 409）；其他→500 通用訊息；文案寫死在 handler【量】 | 數字碼＋寫死中文（M18）【量】 | 改：`IMAGE_QUOTA_EXCEEDED`（resource_exhausted，metadata `remaining`、`resetTime`）、`IMAGE_SAFETY_BLOCKED`／`IMAGE_RECITATION_BLOCKED`（invalid_argument）、`IMAGE_NOT_PRODUCED`（unavailable）、provider 不可用（unavailable 無 reason）（`error_reason.proto:128-140`）【量】 |
| 週期 | `U/style_usecase.go:353-365` | 週一 00:00 Asia/Taipei 起算，`week_start DATE`【量】 | — | 保留；`resetTime` 要算下週一 00:00 Taipei 轉 UTC【推】 |
| prompt | `GW/handler/ai.go:266-270` | client 自由文字，只檢查非空【量】 | 無長度上限（成本與 prompt 注入面）【量】；但 seller-app／website 都有「自訂提示詞」功能（`image_editor_ai_tab.dart:169,315-325`、`AiImageEnhanceDialog.tsx:47`）【量】 | 保留自由文字（P:57 已定）；長度上限是產品數字 → §5 Q2 |
| 指標 | `U/transform_usecase.go:110,330` `observability.RecordAITransform*` | OTel 計數【量】 | — | 依 A9-D11：不搬指標，slog 記 outcome【量：計畫】 |

### 1.2 ListAiStyles（app）

| 項 | 位置 | 行為 | 既有缺陷 | 去向 |
|---|---|---|---|---|
| REST | `GW/handler/ai.go:176-220`；路由 `routes_ai.go:14`（PrivateNoStore） | 把 `thumbnailKey` 組成 `thumbnailUrl`【量】 | — | 刪；新回 `hawkr.common.v1alpha1.Image`（P:46） |
| handler | `H:108-154` | 未知或空 category 一律當 `product`（`:118-121`）【量】 | 靜默改寫輸入 | 改：未設＝PRODUCT，`UNSPECIFIED` 回 invalid_argument（P:70）【量】 |
| use case | `U/style_usecase.go:62-79,283-323,327-351` | 讀 5 分鐘行程內快取；default 那列只提供 `default_prompt` 不列入；停用的不列；沒有 default 列時用寫死的 `fallbackDefaultPrompts`（`:30-33`，含「農產品圖片」）【量】 | (a) 行程內快取多 pod 不一致，寫入只清本 pod（`:25-27,272-275`）【量】；(b) quota／usage 讀取吞掉所有 DB 錯誤當預設（`:339-348`）【量】；(c) 農產文案（Q4 of DI）【量】 | 改：不搬快取（風格數量有限、讀一次索引查詢）【推】；DB 錯誤上拋；fallback 文案改寫成一般電商或要求每個用途必有 default → §5 Q9 |

### 1.3 後台 7 支

| 方法 | 位置 | 行為 | 既有缺陷 | 去向 |
|---|---|---|---|---|
| 全部 | method policy `H:481-486`（`ActorAdmin`）；actor `OLD/internal/actor/actor.go:25-37` | 以 `x-account-type == "4"` 判斷 admin【量】 | AD8【量：DI:236】 | 改：`adminauth` 逐方法宣告（§3.2） |
| ListAiStylesForAdmin | `H:156-189`；`U/style_usecase.go:82-107`；SQL `style.sql:1-4` | category 空＝全部，依 `sort_order, id`【量】 | — | 保留；回 `AiStyle`（含 `etag`、`Image`）（PA:22-25,101-108） |
| CreateAiStyle | `H:191-229`；`U/style_usecase.go:130-151` | 先 `GenID()`，有 staged token 時先 `FinalizeAndLink`（跨服務）再寫 DB；或接受既有 `thumbnail_key`（驗 `public/` 前綴）；`sort_order` 解析失敗當 0（`:118-127`）；回應無資料【量】 | (a) media link 在 DB insert 之前、不同 tx：insert 失敗留下孤兒引用（`H:197-202`）【量】；(b) `UNIQUE(category,name)` 衝突以未分類 DB 錯誤回傳【推】；(c) 同時把 is_default=true 建立時不先清舊 default，撞 partial unique 才失敗【推：`style.sql` 只在 Update 清】 | 改：UUIDv7 ID；縮圖只收 staged token（PA:27,62-63「建立時必填」）；`media.FinalizeField`＋`media.SyncChanged` 與寫列同 tx（content banner 的作法：`modules/content/internal/images/images.go:63`、`internal/mediaref/mediaref.go:25,88`）【量】；名稱衝突回 already_exists【推】；`adminaudit.Record` 同 tx |
| UpdateAiStyle | `H:231-290`；`U/style_usecase.go:154-243`；SQL `style.sql:14-53` | `has_*` 旗標部分更新；設 default 時同 tx 先清同分類 default 再標記（為避開 partial unique 的逐列檢查）；只有 `is_default=true` 會寫（false 被忽略）；換縮圖後 best-effort 釋放舊引用【量】 | (a) 無樂觀鎖【量】；(b) `is_default=false` 被靜默忽略（`:155-166`）【量】；(c) 舊縮圖釋放不在 tx 內（`H:282-287`）【量】；(d) DB 錯誤包成 code 400 通用訊息（`H:278-279`）【量】 | 改：`etag`＋`update_mask`（PA:5-9,119-123）；`default_style` 設 false 的語意要定 → §5 Q7；清舊 default＋標新 default 的兩句順序保留（`style.sql:14-43` 的理由仍成立）【量】；縮圖用 refsync 同 tx |
| DeleteAiStyle | `H:292-313`；`U/style_usecase.go:246-266` | default 不可刪（400）；查詢錯誤一律回「style not found」400；刪後 best-effort 釋放引用【量】 | DB 故障偽裝成 not found（`:248-257`）【量】；釋放不在 tx【量】 | 改：etag 選填（PA:129-133）；not_found 與內部錯誤分開；default 不可刪的錯誤形狀未在契約 → §5 Q7；refsync 同 tx |
| GetAiQuota | `H:315-344`；`U/quota_usecase.go:32-70` | 無個別設定回預設 10、`is_custom=false`；store_id 不驗存在；DB 錯誤吞成預設（`:38-60`）【量】 | 不存在的店也回 200（預設）；吞錯【量】 | 改：回 `AiQuota`（PA:82-99）；店不存在的處理 → §5 Q8；DB 錯誤上拋 |
| UpdateAiQuota | `H:346-370`；`U/quota_usecase.go:73-91`；SQL `quota.sql:7-13` | `weekly_limit` 必須 0–1000（`:74-84`），upsert，無樂觀鎖【量】 | 0–1000 是程式常數，無規格來源【量】 | 改：PA:40-43 的 etag 語意（無列時不帶 etag 建立；有列沒帶 etag 回 ALREADY_EXISTS；過期 ABORTED）；上限 1000 寫進模組規格（§5 Q4） |
| DeleteAiQuota | `H:372-391`；`quota.sql:15-16` | 無列也成功【量】 | — | 改：無個別設定回 NOT_FOUND（PA:45-46）【量】 |

### 1.4 資料表

| 舊表 | 位置 | 去向 |
|---|---|---|
| `ai_styles`（TSID、`category CHECK`、`UNIQUE(category,name)`、`UNIQUE(category) WHERE is_default`、`thumbnail_key`） | `schema.sql:67-85` | 保留成 `imageai.styles`：`id UUID`（v7）、加 `version`（etag）、`category` 改 Go 型別＋CHECK、`thumbnail` 改 media 引用欄＋refsync 所需欄位（照 content banner）【推】 |
| `ai_usage`（`(store_id, week_start)`、`usage_count`） | `schema.sql:87-96` | 保留成 `imageai.usage`：`used`、`CHECK (used >= 0)`，照 `procurement.usage`（`modules/procurement/db/schema.sql:474-482`）【量＋推】 |
| `ai_quotas`（`store_id` PK、`weekly_limit`、`memo`） | `schema.sql:98-105` | 保留成 `imageai.quotas`：加 `version`（etag）、`CHECK (weekly_limit BETWEEN 0 AND <上限>)`【推】 |
| `ai_usage_reservations` | `schema.sql:108-118` | 刪（A9-D9）【量：計畫】 |

`store_id` 維持 bigint 外部鍵（M1）【量：CARRYOVER M1】。

### 1.5 設定（AD17）

- `GEMINI_API_KEY` 經 `OLD/internal/config/config.go:13` 讀【量】；單體已由 `llm.ReadConfig` 讀同名變數（`platform/llm/llm.go:116-125`）【量】→ 共用，不新增。
- `GEMINI_MODEL`（`config.go:14`）是文字模型，修圖不讀它；修圖模型寫死在 `transform_usecase.go:62`【量】→ 新增 `LLM_IMAGEAI_MODEL`（選填）或只用登記表預設 → §3.1。
- `TRANSFORM_INFLIGHT_BYTES_BUDGET` 直接 `os.Getenv`（AD17）【量】→ 改成 `platform/config` 讀的併發上限（§5 Q5）。

## 2. 新契約

### 2.1 ImageAiAppService（P）

| 方法 | 請求 | 回應 | 錯誤 |
|---|---|---|---|
| `TransformImage` | `bytes image_data`（REQUIRED）、`string mime_type`（REQUIRED）、`string prompt`（REQUIRED） | `bytes image_data`、`mime_type`、`int32 weekly_usage`、`int32 weekly_limit` | resource_exhausted＋`IMAGE_QUOTA_EXCEEDED`；invalid_argument＋`IMAGE_SAFETY_BLOCKED`／`IMAGE_RECITATION_BLOCKED`；unavailable＋`IMAGE_NOT_PRODUCED`；provider 不可用 unavailable；併發超限 resource_exhausted＋RetryInfo；失敗退額度（P:13-20） |
| `ListAiStyles`（NO_SIDE_EFFECTS） | `optional AiStyleCategory category` | `default_prompt`、`weekly_usage`、`weekly_limit`、`repeated AiStyle styles`（id、name、label、prompt、`Image thumbnail`、`display_order`） | UNSPECIFIED 回 invalid_argument（P:70） |

### 2.2 ImageAiAdminService（PA）

| 方法 | 請求 | 回應 | 錯誤／語意 |
|---|---|---|---|
| `ListAiStylesForAdmin` | `optional category` | `repeated AiStyle ai_styles` | — |
| `CreateAiStyle` | `AiStyle`（name、label、prompt、category REQUIRED；`thumbnail_staged_token` 必填） | `AiStyle` | — |
| `UpdateAiStyle` | `AiStyle`（id、etag 必填）＋`update_mask` | `AiStyle` | 沒帶 etag invalid_argument；不符 ABORTED `VERSION_CONFLICT` |
| `DeleteAiStyle` | `id`、`etag`（選填） | 空 | 刪除並釋放縮圖 |
| `GetAiQuota` | `store_id` | `AiQuota`（store_id、weekly_limit、weekly_usage、memo、custom、update_time、etag） | 無個別設定回預設、`custom=false` |
| `UpdateAiQuota` | `AiQuota`（store_id、weekly_limit 必填）＋`update_mask` | `AiQuota` | 無列不帶 etag＝建立；有列沒帶 ALREADY_EXISTS；過期 ABORTED |
| `DeleteAiQuota` | `store_id`、`etag`（選填） | 空 | 無個別設定 NOT_FOUND |

AiStyle（admin）：`name`、`category` IMMUTABLE；`default_style`、`enabled`、`display_order`、`create_time`、`update_time`、`etag`（PA:50-79）【量】。

### 2.3 新舊差異（對外看得到的）

| # | 差異 | 證據 | 處置 |
|---|---|---|---|
| C1 | **請求大小**：舊入口 10 MiB（body_guard）、gRPC 20 MB；單體 `connect.WithReadMaxBytes(2<<20)` 全域（`backend/internal/httpapi/server.go:148,218`），規範 §十「2 MiB」、§十一「檔案上傳用 presigned URL，API 只處理 metadata」（`docs/共通規範/API 設計規範（Connect）.md:195,202`）【量】。P 把原圖放 `bytes`，與 §十一 抵觸 | 如左 | §5 Q1（需 Will） |
| C2 | 回應 `bytes image_data`：Connect JSON codec 下是 base64（膨脹約 1/3），binary codec 無膨脹【推】；§十三 只為「回應是檔案本身」開了 `/api/files/` 例外（`:209-214`）【量】 | 如左 | 隨 Q1 一起決定 |
| C3 | 數字碼 4291／9250–9253 → `ErrorReason`；`IMAGE_QUOTA_EXCEEDED` 新增 metadata `remaining`、`resetTime` | `error_reason.proto:128-131` | 實作照契約 |
| C4 | 字串數字 → `int32`；`is_default`→`default_style`、`sort_order`→`display_order`、`thumbnailKey/Url`→`Image` | P、PA | 照契約 |
| C5 | ID TSID → UUIDv7 字串；`store_id` 仍是數字字串 | CARRYOVER M1 | 照契約 |
| C6 | 後台新增 etag 與 update_mask；Create／Update 回資源；DeleteAiQuota 無列 NOT_FOUND（舊為成功） | PA:5-9,40-46 | 照契約 |
| C7 | Create 縮圖必填 staged token（舊可空或給既有 key） | PA:27,62 | 照契約；admin 頁要改成先上傳（A9-5） |
| C8 | 未知 category：舊靜默當 product；新 UNSPECIFIED 回 invalid_argument | `H:118-121`、P:70 | 照契約 |
| C9 | 契約沒寫：default 風格可否刪、`default_style=false` 的語意、prompt 長度上限、店不存在時 GetAiQuota 的結果、weekly_limit 上限 | PA、P | §5 Q2、Q4、Q7、Q8 |

## 3. 單體可重用點

### 3.1 `internal/platform/llm`

- 唯一建構點；登記表只有 `gemini-2.5-flash`、`deepseek-flash`、`deepseek-v4-pro`（`llm.go:37-45,87-91`），角色只有 `Assistant`、`Procurement`（`:47-55,93-96`），`DefaultModel` 是全域單一常數（`:39`）【量】。**不支援圖片模型**【量】。
- 回傳的是 ADK `model.LLM`（`:151`），請求是 `model.LLMRequest`（內含 `genai.Content` 與 `genai.GenerateContentConfig`）【量：import `google.golang.org/adk/v2/model`、`genai`】。用它送 inline image part＋`ResponseModalities: ["IMAGE"]`、從 `LLMResponse.Content.Parts[].InlineData` 取圖在型別上可行【推：未跑 spike】。
- 架構測試 `arch/restricted_calls.go:85-90` 只擋 `genai.NewClient`；手寫 HTTP 抓不到（計畫已述）【量】。
- 需要的改動【推】：新增 `Role ImageAI`（只允許 Gemini）、登記 `gemini-2.5-flash-image`、讓預設模型依角色決定（目前 `DefaultModel` 全域，`build` 在 `:176-179` 取它）、`ReadConfig` 加 `LLM_IMAGEAI_MODEL`；`ErrDisabled` 文案是「AI 助理尚未開通」（`:69`），修圖要用中性或依角色的訊息。另需一個 spike 確認 ADK gemini model 對 `GenerateContent` 非串流＋IMAGE modality 會原樣回 `InlineData` 與 `FinishReason`／`PromptFeedback.BlockReason`（舊分類邏輯 `transform_usecase.go:387-419` 需要這兩欄）。
- **spike 結果（A9-4a，2026-11-23，`platform/llm/image_test.go` 的 `TestADKImagePathKeepsInlineDataAndFinishReasonButDropsBlockReason`，對本機 httptest 替身，不連網）【量】**：ADK `gemini` model 非串流時 `InlineData` 與 `FinishReason` 原樣回來（候選被擋時 `Content` 為 nil、`FinishReason` 有值）；但 prompt 被擋、回應沒有候選時 `generate` 直接回 `empty response` 錯誤（`gemini.go:129-132`），`PromptFeedback.BlockReason` 不見。舊分類邏輯需要 `BlockReason`，所以 ADK 路徑擋住，採備案：`llm` 回窄介面 `ImageGenerator`（`GenerateImage(ctx, ImageRequest) (ImageResult, error)`，`ImageResult` 帶 `Image`、`MimeType`、`FinishReason`、`BlockReason`），內部用 `genai.Client`，`llm` 仍是唯一建構點（`NewImage`／`buildImage`）。登記表加 `gemini-2.5-flash-image`（只給 `ImageAI` 角色，且該角色只接受圖片模型），預設模型改依角色（`roleDefaults`），`ReadConfig` 讀 `LLM_IMAGEAI_MODEL`，停用時回 `ErrImageDisabled`。
- 防止手寫 HTTP 再出現：可在 `arch/restricted_calls` 加「`internal/modules/**` 不得出現 `generativelanguage.googleapis.com` 字串」之類守門【推】（不屬必做，列給主 session）。

### 3.2 `adminauth` 與 `adminperm`

- `adminauth.declarations`（`httpapi/adminauth/adminauth.go:65` 起）逐 procedure 宣告 `requires(perm, auditAudited|auditReadOnly)`；新寫入方法不得用 `auditPending`（`:32-43`）；啟動時對照 descriptor（`:1-4`）【量】。
- `adminperm` 命名 `<模組>.<資源>.<動詞>`（`platform/adminperm/adminperm.go:3-4`），要新增 7 個常數並加入 `Catalog()`（`:119-151`）；唯一角色 `OWNER` 擁有全部（`:18-22`）【量】。建議名稱：`imageai.ai_styles.{list,create,update,delete}`、`imageai.ai_quotas.{get,update,delete}`【推】。
- admin service 掛載見 `server.go:448` 註解：每個 admin service 掛自己的宣告表 interceptor【量】。

### 3.3 `adminaudit.Record`

- `platform/adminaudit/adminaudit.go:95-115`：必須在 tx 內（否則 `ErrNoTransaction`），`Entry{Actor{AdminID, Action=procedure}, TargetType, TargetID, Detail}`【量】。範例：`modules/payment/internal/account/service.go:872`、`modules/moderation/internal/handle/handle.go:171`【量】。
- 建議 TargetType：`ai_style`（ID＝UUID）、`ai_quota`（ID＝store_id）【推】。

### 3.4 procurement 的配額計數器（A9-3b）

- SQL：`modules/procurement/db/queries/runs_usage.sql:19-31`——`ConsumeUsage` 是 `INSERT … SELECT … WHERE @quota_limit > 0 ON CONFLICT DO UPDATE SET used = used + 1 WHERE used < @quota_limit RETURNING used`（回 0 列＝拒絕）；`RefundUsage` `used - 1 WHERE used > 0`【量】。
- 表：`modules/procurement/db/schema.sql:473-482`（`used >= 0` CHECK）【量】。
- 呼叫點：扣在 `internal/run/submit.go:139`（與建 run 同 tx），退在 `internal/run/settle.go:22-25`（只在 `QuotaPeriodStart` 非 NULL）【量】。
- 對 imageai 的差異【推】：procurement 的「扣」與 run 列同 tx、「退」跟著 run 結算，crash 有 sweeper 收尾；imageai 沒有 run 列，扣在獨立短 tx、Gemini 在 tx 外、退在另一個短 tx（`context.WithoutCancel`＋timeout）。若在扣與退之間 crash，該次額度不退 → §5 Q3。
- 複製 SQL 而不是抽共用套件：兩模組各自擁有 schema（依賴方向），DI Q7 建議的共用 `quota` 子套件需跨模組 SQL，與模組擁有資料的規則衝突【推】；SQL 僅 ~15 行，複製成本低。

### 3.5 media（縮圖）

- 單體 `modules/media`：`FinalizeField`（staged token 轉正，`finalize.go:32-35`）、`SyncChanged`（同 tx 記退場並排同步 job，`media.go:56-59`）、`RefTypeAiStyle = "7114"`（`ref_type.go:23`）【量】。content banner 是同形範本（`content/internal/images/images.go:63`、`content/internal/mediaref/mediaref.go:25,35,88`）【量】。
- media 的 `Media` 介面只有 Finalize／Link／Unlink／ReadSecure（`media/internal/model/model.go:42-56`），**沒有讀一般 staged 物件 bytes 的方法**【量】→ Q1 的「presigned 上傳原圖」選項要新增 media API。
- 修圖結果不落地（舊版回 base64，client 自己再走上傳）【量：`ai-image.ts:427`、`H:458-467`】。

## 4. 消費端（A9-5 改 Connect）

指令：`git -C <repo> grep -n -E "image_transformations|ai_styles|ai_quotas|AiImageEnhanceDialog|ai-image" origin/dev -- website admin seller-app buyer-app ':!*generated*' ':!**/openapi*' | rg -v "e2e|\.test\.|_test\.dart"`【量】。buyer-app 0 命中【量】。

| client | 呼叫點 | 備註 |
|---|---|---|
| website | `lib/seller/ai-image.ts:267`（`/ai_styles`）、`:364`（`/image_transformations`）；UI `features/seller/components/AiImageEnhanceDialog.tsx:37`，被 `ProductImageUploader.tsx:194`、`features/seller-center/posts/PostComposeClient.tsx:723` 使用 | 含內建風格 fallback 清單（CI:102）；`ai-image.ts` 與 dialog 內 `rg -i "canvas|toBlob|resize|maxDimension"` 0 命中 → **上傳前不縮圖**【量】；文件 `website/docs/ai-style-thumbnails.md` 描述佔位縮圖 |
| seller-app | `lib/features/photo_editor/data/ai_image_service.dart:127`（`/app/ai_styles`）、`:195`（`/app/image_transformations`）；錯誤碼鏡射 `:27-41`（9250–9253、4291）；model `ai_style_model.dart` | 上傳前 `FlutterImageCompress` 限 1920px、q85 JPEG（`:181-189`）【量】；寫死 fallback 風格清單 `presentation/image_editor_ai_tab.dart:56-67,94-97`【量】 |
| admin | `src/app/admin/ai-styles/page.tsx:99,181,200,219,228`（styles）、`:244,267,284`（quotas）；入口 `src/components/Layout/Sidebar.tsx:180` | 頁內直接 `api.*`，型別內聯（CI:111）【量】 |

兩個 client 的寫死 fallback 風格清單違反根 AGENTS「同一個對照表不在兩個 client 各寫／不寫寫死的後備」精神【推】→ A9-5 刪除，改為 API 失敗時顯示錯誤（§5 Q9）。

## 5. 開放問題（SSOT 沒定、需要 Will）

**Q1（契約＋成本）TransformImage 的原圖怎麼送？** 單體全域 2 MiB、規範 §十一 要 presigned 上傳；P 用 inline `bytes`。
- (a) 維持 inline bytes＋2 MiB，client 上傳前縮圖（長邊約 1536–2048px、JPEG），伺服器超過回 invalid_argument（帶 field）；在 P 的 `image_data` 註解寫上上限。成本：website 要補縮圖（目前沒有）；契約只改註解。
- (b) inline bytes，`ImageAiAppService` 單獨放寬 `WithReadMaxBytes`（例如 8 MiB）；規範 §十 要加例外。成本：例外條款、記憶體尖峰（admission 仍要）。
- (c) 照 §十一：原圖先走 media staged 上傳，`TransformImage` 帶 staged token；需新增 media「讀 staged 物件」API 與 imageai→media 讀取，結果仍 inline 或另存。成本最高，跨模組新 API。
- 建議 **(a)**【推】：Gemini 圖片模型的輸出解析度約 1K 級，大於 2 MiB 的原圖不帶來品質，seller-app 已壓到 1920px；修圖是暫態運算而非檔案保存，§十一 的 presigned 是為「要存的檔案」設計，在 P 註解寫明這個區分即可，不需要例外。順帶確認回應 bytes（C2）在 binary codec 下傳回，JSON codec 會膨脹但仍 < 4 MiB【推】。

**Q2（產品數字）prompt 長度上限？** 舊版無上限，兩個 client 都有自訂提示詞。選項：不設（僅 2 MiB 總量）／設定上限（例如 500 rune，用 `@hawkr/api` `text/server-text` 的計長規則）。建議設上限並寫進 P 註解與模組規格【推】；數字需 Will。

**Q3（資料一致性）扣額度後 crash 不退。** A9-D9 選了計數器、不建預約表。選項：(a) 接受：crash 時該賣家少 1 次，寫進模組規格已知限制；(b) 加一張修圖紀錄表＋River job 收尾（等於又建預約表，與 A9-D9 衝突）；(c) 先呼叫 Gemini 成功後才扣（超用風險：併發時可超過上限，且 Gemini 成本已花）。建議 **(a)**【推】，因為 crash 機率×1 次額度的損失遠小於 (b) 的機制成本；但「只有 Will 接受限制」，需確認。

**Q4（產品數字放哪）預設每週 10 次、後台上限 0–1000。** 舊版是程式常數（`domain/usage.go`、`quota_usecase.go:75`），CARRYOVER M9 列誤寫「照現在 config 搬」。選項：模組常數＋規格／`platform/config` 環境變數／隨賣家方案（像 procurement 讀 buyer entitlement）。建議模組常數並寫進 `backend/docs/modules/imageai.md`，順手更正 CARRYOVER 那列【推】。

**Q5（設定，AD17）併發上限的形狀。** 舊 byte 預算（256 MiB、×6）。入口上限固定為 2 MiB 後，byte 預算等於「同時 N 張」。建議改成併發數 semaphore，N 由 `platform/config` 讀（預設依 256 MiB／(2 MiB×6) ≈ 21 取整，未量測，4a 以記憶體量測校正；每 pod 各一份，管不到 Gemini 專案層級速率限制，429 轉 `RESOURCE_EXHAUSTED`）【推】；這是實作細節，可不問 Will，列出以免默默沿用。

**Q6（llm 套件形狀）** 圖片模型併入 `platform/llm`：新增 `Role ImageAI`、登記 `gemini-2.5-flash-image`、預設模型改成依角色。需要先做一個小 spike（ADK gemini 非串流＋IMAGE modality 回 InlineData、FinishReason、BlockReason）。若 ADK 抽象擋住這些欄位，備案是 `llm` 套件另回一個 `*genai.Client` 的窄介面（仍在唯一建構點內）【推】。不涉契約，主 session 可決定。

**Q7（契約）default 風格的刪除與 `default_style=false`。** 舊：default 不可刪（400）、`is_default=false` 靜默忽略。PA 沒寫。選項：(a) 刪 default 回 failed_precondition（要不要新 ErrorReason？新增 reason＝契約變更）；(b) 允許刪，該用途回到 server fallback prompt；`default_style=false` 選項：拒絕（invalid_argument，要改 default 只能把另一個設成 true）或允許（用途可沒有 default）。建議 (a) 用 failed_precondition＋不新增 reason（`localized_message` 說明）、`default_style=false` 回 invalid_argument，並寫進 PA 註解【推】；屬對外契約，需 Will。

**Q8（契約）GetAiQuota／UpdateAiQuota 對不存在的店。** 舊版一律成功。模組依賴已含 `seller`（`arch/modules.go:41`）。建議經 seller 驗證店存在，不存在回 not_found，寫進 PA 註解【推】；需 Will（契約語意）。

**Q9（client 文字與後備）** server 的 `fallbackDefaultPrompts`（農產文案，`style_usecase.go:30-33`）與兩個 client 的寫死 fallback 風格清單。建議：server 端要求每個用途有一筆 default 風格（migration seed 或後台建立；沒有時 `default_prompt` 用一句中性常數），client 的 fallback 清單在 A9-5 刪除【推】。seed 與否需 Will（資料模型）。

**Q10（資料生命週期）店刪除時的 quotas／usage 清理。** 舊版無。procurement 有 `buyer_deleted.sql:35 DeleteBuyerUsage`【量】。若 seller 有店刪除事件可照做；否則延後。**已解決**：seller 發布 `StoreRetired`（`seller/events.go:30`）【量】，計畫 A9-4a 訂閱它刪該店配額與用量。

## 6. 建議的子步驟切分【推】

- A9-4a：schema（3 表）、`platform/llm` 的 ImageAI 角色＋spike、配額 SQL＋並發與退款測試（計畫驗收的兩個 mutation）。
- A9-4b：TransformImage、ListAiStyles use case＋handler＋掛 `ImageAiAppService`。
- A9-4c：後台 7 支＋`adminperm` 7 個權限＋`adminauth` 宣告＋`adminaudit` 同 tx＋refsync 縮圖＋掛 `ImageAiAdminService`（啟動檢查 mutation）；`TestEveryServiceInTheContractIsMounted` 的缺漏清單不再含 imageai。
