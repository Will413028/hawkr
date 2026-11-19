# A9-0 盤點（領域、資料、機制切片）：`ai-service` → agent／採購模組

唯讀盤點，快照 `bc2c732c`（worktree `a9-plan`，branch dev），2026-11-19。路徑相對於 `backend/`，`AI=services/ai-service`，`AIi=services/ai-service/internal`，`PR=services/ai-service/internal/procurement`，`M=internal/modules`。
範本：`backend/docs/plans/2026-11-19-a6-review-im-crm/domain-inventory.md`。決策來源：遷移計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md` 的 A9 條（第 252–256 行）、ADR `docs/adr/2026-10-27-general-ecommerce-scope-cut.md` D2。
標記：【量】＝跑過指令或打開檔案；【推】＝讀程式推論。本檔只盤「領域、資料、機制」；RPC／REST／client 呼叫點是別的切片（但 §0 列出 RPC 數量差異，因為會影響範圍）。

---

## 0. 摘要

- **規模**：非測試、非生成 Go 行數 37,086【量】`find AI -name '*.go' ! -name '*_test.go' ! -name '*gen.go' ! -name '*.pb.go' ! -path '*/gen/*' | xargs cat | wc -l`；含測試 96,999【量】。其中採購（`PR`）23,901，占 64%【量】。
- **表**：40 張（`schema.sql` 9、`procurement.sql` 31）【量】`grep -c 'CREATE TABLE' AI/db/schema/*.sql`；65 支 migration【量】`ls AI/db/migrations/*.sql | wc -l`；0 個 extension【量】`grep EXTENSION`；sqlc 查詢 169 支（頂層 17＋procurement 152）【量】`cat AI/db/queries/*.sql AI/db/queries/procurement/*.sql | grep -c '^-- name:'`。procurement 31 張表**沒有任何 FK**（`grep -n REFERENCES AI/db/schema/procurement.sql` 無結果【量】）；`schema.sql` 只有 ADK 的 `events → sessions` 一條 FK（`AI/db/schema/schema.sql:35-38`【量】）。搬遷清單 `db-tables.csv:2` 寫 41 張【量】，差一張（我數到 40；清單可能含 Atlas 的 revision 表，【推】）。
- **三塊領域**（再加一塊清單沒點名的）：
  1. 通用 AI 助理（賣家／買家聊天、專職助手、coordinator，ADK＋Gemini）。
  2. 採購／RFQ（買家以對話建立詢價 → 候選搜尋 → 確認快照 → 發布 → 供應商回覆 → 買家接受 → 建單），64% 的程式碼。
  3. LLM provider 與 cache（分類樹索引、別名）。
  4. **AI 修圖**（`ai_styles`、`ai_usage`、`ai_quotas`、`ai_usage_reservations`，Gemini image，`TransformImage`＋後台 `AIAdminService` 7 支）與 `RecommendationService.PersonalizedRecommend`：任務文字沒提，但佔 4 張表、9 支 RPC（Q1）。
- **RPC 面**：`ai.pb.go` 實際 59 支（AIAdmin 7、AIAssistant 13、AIImage 2、Recommendation 1、AIProcurement 36）【量】`rg -o '\w+_\w+_FullMethodName += ' pkg/proto/ai/ai_grpc.pb.go | sed 's/_FullMethodName.*//' | awk -F_ '{c[$1]++}END{for(k in c)print k,c[k]}'`；搬遷清單 `rpc-servers.csv` 列 61 支，多出 `AIAssistantService.SellerPrice`、`BuyerSeasonal`（清單 `:13`、`:20`；`rpc-calls.csv:164,172` 還記著 gateway 呼叫點），但 proto、gateway、ai-service 都已經沒有這兩支（`rg 'SellerPrice|BuyerSeasonal' --glob '!*.csv'` 無結果【量】）。清單落後於 pricebook 移除（AD6）。
- **對外依賴**：gRPC 呼叫 order、product、category、seller、buyer、media 六個 client（`AIi/client/*`）【量】；`rpc-calls.csv` 另列 `PricebookService` 6 支（`:` ai-service→pricebook-service），但 `AIi/client` 沒有 pricebook 目錄、`rg -i pricebook AI`（排除 migration）無結果【量】，清單過時。
- **最重要的發現**（細節見 §5、§6）：
  1. **賣家收款統計用錯狀態碼**：`query_payment_summary` 把 `"30"` 當「已完成」撈已收款訂單（`AIi/agent/tools.go:307`），同檔的 `orderStatusMap` 寫 `30=配送中`、`50=已完成`（`tools.go:23-31`）；coordinator 的 prompt 又寫 `30=已完成, 40=已取消`（`coordinator.go:31,53`）。三處互相矛盾（AD1）【量】。
  2. **`category_alias_gen.go` 是用舊農產分類樹生成的**：檔頭「active 分類樹的 220 個 L2（排除 L1：花卉），全樹 2257 個節點，2026-09-13」（`AIi/cache/category_alias_gen.go:3`），851 筆別名對到 id 4 至 222 的農產分類（蔬菜、水果、蛋奶…）；一般電商分類樹沒有這些節點時，別名層會把買家的詞對到不存在或不相干的分類（AD3，Q4）。【量】檔頭與筆數；【推】後果。
  3. **採購執行沒有 sweeper**：process 在 commit 後、dispatch 前或中途 crash，run 永遠停在 QUEUED／RUNNING，沒有程式重驅（`PR/usecase/run_dispatch.go:310-316` 自己承認「Track 2 lease executor #2452 接手，v1 不做 sweeper」）；只有候選搜尋有 lazy reaper（`PR/repository/run.go:455`）。單體有 River job，這個缺口可以補（AD2、Q6）。
  4. **`cmd/resolver-eval` 的 `observe` 已經解析不到現行回應**：打舊 gateway REST 的 `/app/buyers/me/procurement/turns` 並讀 `{code, data}` 信封與 `display_value`（`cmd/resolver-eval/main.go:132-139,227-243`）；A9 之後 gateway 路由消失，這支工具整支要重寫成打 Connect（§4、Q9）。
  5. **兩份 LLM 錯誤排除機制只靠註解**：`llmprovider` 的 quirk 登記表靠 `scripts/check-llm-client-construction.py` 守門（`PR/usecase/surface_realizer.go:58` 引用），但這支腳本在整個 worktree 找不到（`find . -name 'check-llm-client-construction*'` 無結果【量】），守門不存在（AD5）。
- **沒有 outbox 以外的 MQ 消費者**：`rg 'Consume|Subscribe' AI --glob '!*_test.go'` 無結果【量】。搬遷清單 `event-pairs.csv` 還列 `order.completed`、`seller.membership_changed` 兩條 ai-service 消費（更新 ranking facts），但 `seller_ranking_facts` 表已被 `20261028100000_drop_seller_ranking_facts.sql` 刪掉，程式也沒有消費者（AD6）。ai-service 只**生產** 2 個事件（`events.csv` 列兩行）：`procurement.rfq_publication_invite.created`、`…external_created`（給 notification）等，經 `rfq_publication_outbox`（`PR/publicationoutbox`）。
- **Pricebook 殘留**：資料與程式已清乾淨（§4.1）；殘留在「文件清單」「`PRICE_REFERENCE_AVAILABLE` 這個 claim 的名字」「`crop`／農產命名」「分類別名」與「prompt 文案」，A9 不接 pricebook。
- **單體裡已有一個 `agent` 模組**（`M/agent`），但它是「供應商入口的邀請與 session」，不是 ai-service 的搬遷目的地（`M/agent/AGENTS.md:3`）【量】；A9 的「agent」名稱容易誤導，採購與助理要放哪個模組要先決定（Q2）。依賴表的 `agent` 列是 `{ordering, catalog, buyer, seller, media}`（`internal/arch/modules.go:39`）；**`ordering`、`catalog` 兩個模組在單體還不存在**（`ls internal/modules` 14 個，無此二者；`internal/legacy/orderservice`、`productservice` 還在）【量】，A9 的前置是 A4／A5 或繼續走 legacy client（Q3）。

---

## 1. 領域與資料

### 1.1 表

來源：`AI/db/schema/{schema,procurement}.sql`；sqlc 查詢在 `AI/db/queries/`，生成碼在 `AIi/repository/postgres/gen/`（修圖等）與 `PR/repository/gen/`（採購，後者手寫 `models.go` 包一層，`PR/repository/models.go:6`）。ID 一律由 Go 端 `go-pkg/tsid` 產生（`PR/module.go:29`、`AIi/usecase/style_usecase.go:51`）【量】。

#### 1.1.1 ADK session（通用助理與採購共用）

| # | 表 | 主鍵 | 說明 | 寫入／讀取 | 去向 |
|---|---|---|---|---|---|
| 1 | `sessions` | `(app_name, user_id, id TEXT)` | ADK 的 GORM model 的手寫 DDL（`schema.sql:3-11`）；`state JSONB` | ADK `session/database`（`AIi/agent/di.go:49-67` 的 GORM＋pgx）；清理 `AIi/agent/cleaner.go`（每小時，刪 24 小時前）；採購用 `AppNameProcurement`、助理用 `AppNameAssistant`，分區鍵不同（`AIi/agent/adk_app_name.go`） | 取決於 Q5（ADK 是否保留） |
| 2 | `events` | `(id, app_name, user_id, session_id)` | 28 欄，ADK 事件（`schema.sql:13-39`）；`actions BYTEA` | 同上；FK `ON DELETE CASCADE` 到 `sessions` | 同上 |
| 3 | `app_states` | `app_name` | `schema.sql:48` | ADK | 同上 |
| 4 | `user_states` | `(app_name, user_id)` | `schema.sql:54` | ADK | 同上 |

【量】`grep -n 'CREATE TABLE' AI/db/schema/schema.sql`。`user_id` 是 TEXT：賣家助理以 `store_id`、買家以 `buyer_id` 字串（【推】`AIi/agent/tools.go:719-757` 以 state 取 `store_id`／`buyer_id`，註解說避免 19 位 TSID 經 JSONB 往返失真）。

#### 1.1.2 AI 修圖與配額（`schema.sql`）

| # | 表 | 主鍵 | 值得注意 | 寫入／讀取 | 去向 |
|---|---|---|---|---|---|
| 5 | `ai_styles` | `id BIGINT` TSID | `prompt TEXT`（存修圖 prompt）、`category CHECK IN ('product','post')`、`UNIQUE(category,name)`、`UNIQUE(category) WHERE is_default`（`:67-85`） | `AIi/usecase/style_usecase.go`（後台 CRUD）；讀 `ListAiStyles`；`thumbnail_key` 是 media 引用，`ReleaseReferences`（`AIi/grpc/ai_style_media_release_test.go` 有測） | 取決於 Q1 |
| 6 | `ai_usage` | `(store_id, week_start)` | 每店每週用量（`:87-96`） | `AIi/usecase/quota_usecase.go`；`transform_usecase.go` | 同上 |
| 7 | `ai_quotas` | `store_id` | admin 單店額度覆寫（`:98-105`） | 同上 | 同上 |
| 8 | `ai_usage_reservations` | `id` TSID | 預約制：呼叫 Gemini 期間持有，`expires_at` 到期視為失效，無清理 job（`:108-118`） | `transform_usecase.go`；advisory lock 命名空間見 M9 | 同上 |
| 9 | `candidate_leads` | `id` | **已無讀寫者**：只剩 `schema.sql:125` 與 `procurement.sql` 的註解、`AIi/agent/tools.go:498,528` 的註解（`rg candidate_leads --glob '!db/migrations/*' AI` 只命中 schema 與註解【量】）；`schema.sql:120-124` 自己寫「本表保留但不再被讀取（expand-contract 的 expand 段尚未走到 drop）」 | 無 | **不搬**（AD4；上線前不受相容性約束，直接刪） |

兩套配額機制：修圖 `ai_usage` 家族（store＋週）與採購 `procurement_usage` 家族（buyer＋期間）形狀相同、各自實作預約制（`PR/…/quota.sql` 開頭自述「比照 ai_usage 家族」）；共用 advisory lock 命名空間靠字串雜湊分隔（`db/queries/procurement/quota.sql:6-13`）。見 M9、Q7。

#### 1.1.3 採購（`procurement.sql`，31 張，1,088 行；欄數為 `awk` 粗估【量】）

| 群組 | 表（欄數） | 主要語意 | 寫入（file） |
|---|---|---|---|
| 需求本體 | `procurement_requests`（7）、`…_request_revisions`（9，append-only，rewind 時舊分支標 SUPERSEDED）、`…_answer_patches`（9） | request 的 `status`、`active_revision_id`、`version`（樂觀鎖）；revision 帶 slots JSON | `PR/repository/revision.go`、`answer`／`rewind` usecase |
| 對話 | `…_conversations`（6）、`…_conversation_commands`（6，冪等） | 買家的對話列表、重新命名、刪除 | `PR/repository/conversation.go`、`PR/usecase/conversation.go` |
| 候選 | `…_candidate_sets`（15，含 `lease_expires_at`、`fencing_token`）、`…_candidates`（18）、`…_candidate_selections`（10） | 兩條 lane：PLATFORM（平台賣家）與 EXTERNAL（站外業者 prospect）；candidate 帶賣家快照（name、logo、store_code）（`replicas.csv:50`） | `PR/repository/candidate.go`、`PR/usecase/candidate_search*.go` |
| 試算與確認 | `…_price_previews`（12）、`…_confirmed_rfq_snapshots`（11，`canonical_payload_json`） | 試算只剩 `buyer_requested_range_json` 與 `quote_total_range_json`（數量×單價推算）；冪等鍵 `UNIQUE(buyer_id, client_request_id)`（`procurement.sql:319-337`） | `PR/pricing/price_preview.go`、`PR/commerce/rfq_confirmation.go` |
| 發布 | `…_rfq_publication_approvals`（28）、`…_approval_recipients`（5）、`…_publication_commands`（10）、`…_publications`（17）、`…_publication_invites`（11）、`…_publication_outbox`（10） | 買家核准 → 發布 → 給每個收件人一張邀請；outbox 發 `procurement.events` | `PR/commerce/publication_*.go`、`PR/publicationoutbox` |
| 供應商回覆 | `…_seller_responses`（15）、`…_seller_response_versions`（33）、`…_seller_response_commands`（11）、`…_seller_response_drafts`（5） | 報價版本化、草稿、撤回、拒絕；`outside_buyer_requested_range` 只標示（`PR/commerce/seller_response.go:658`） | `PR/commerce/seller_response.go` |
| 買家決策 | `…_buyer_shortlist_entries`（7）、`…_buyer_quote_acceptances`（28） | shortlist、接受報價（含快照與建單所需欄位） | `PR/commerce/buyer_acceptance.go` |
| 建單 | `…_order_provisioning_outbox`（10） | acceptance 之後非同步建 order（gRPC `CreateRfqOrder`） | `PR/orderprovisioning` |
| Agent run 帳本 | `…_agent_runs`（19）、`…_run_inputs`（6）、`…_run_events`（9，`last_event_seq`／`min_available_seq`）、`…_run_artifacts`（12）、`…_run_commands`（10）、`…_run_interrupts`（11） | 持久化的 run 狀態機（`run_type` 5 種、`status` 9 種，`procurement.sql:955-960`）、事件重放、CLARIFICATION 中斷與 resume | `PR/repository/run.go`（約 1,000+ 行）、`PR/usecase/run_dispatch.go` |
| 配額 | `…_usage`（4）、`…_usage_reservations`（5） | 買家期間用量 | `PR/repository/quota.go` |

57 個 index【量】`grep -c 'CREATE.*INDEX' procurement.sql`；CHECK 與 value-domain 約束很多（`grep -c 'CHECK\|REFERENCES'` = 47【量】）；**無 enum 型別、無函式、無 trigger**（`grep ENUM|CREATE TYPE|FUNCTION|TRIGGER` 無結果【量】）。ID 空間：`buyer_id` 是 buyer 業務鍵、`store_id`、`prospect_id`（seller）、`order_no`（order），搬遷清單 `idrefs.csv:19` 與 `replicas.csv:50-51` 記錄了對外的 ID 與副本欄位。

### 1.2 不變量（程式強制處）

| 不變量 | 強制處 | 備註 |
|---|---|---|
| 所有寫入冪等：以 `(buyer_id, client_request_id)` 為仲裁 | `UNIQUE` 索引（price_previews `procurement.sql:331`、各 `*_commands` 表）＋`!IsNew` 分支不重派（`PR/usecase/run_dispatch.go:300-312`） | 【量】；重試不得併發重跑同一個 child run（註解說明已裁決） |
| 同一 request 同時只有一個 ACTIVE revision | `procurement.sql:23` 附近註解；`PR/repository/revision.go`（rewind 把舊分支整條標 SUPERSEDED） | 【量】註解；【推】由 usecase 在 tx 內維持，DB 沒有 partial unique（`grep 'WHERE status' procurement.sql` 未驗） |
| NOTIFY 一定在 commit 之後 | `PR/repository/run.go:406,459`；`candidate.go:404`；`publication.go:324` | 【量】；LISTEN 用專屬連線（`PR/repository/run_watch.go:62-100`） |
| DB 是事件的 SSOT，wake channel 不帶內容 | `run_watch.go:44-50` | 【量】 |
| 候選搜尋的終態寫入用 `fencing_token` 防舊 owner 覆寫 | `procurement.sql:169-170`；`PR/usecase/candidate_search.go:223-240` | 【量】 |
| 買家只能碰自己的 request／run | 各查詢 `WHERE buyer_id = $x`（`grep -c buyer_id AI/db/queries/procurement/*.sql`） | 【推】；`AIi/grpc/method_policy.go` 登記每支 RPC 的 actor（buyer／seller／admin；`registerMethodPolicy` 共 55 處，`rg -c` 【量】） |
| 供應商回覆不得外洩 buyer 的個資給 seller-facing 結構 | `PR/domain/seller_facing_payload.go`；測試 `seller_facing_payload_test.go:69-80` 斷言 `marketQuote` 不出現 | 【量】 |
| 候選 DTO 不得帶 email、phone、social | `PR/domain/artifact.go:136-143` 註解；`PublicWebsite` 可能是社群網址是已知缺口（`artifact.go:143`、`AIi/agent/tools.go:525-535`） | 【量】；屬 AD7 |
| 發布前要核准、核准有效期與重確認 | `PR/commerce/publication_*.go`、`procurement_rfq_publication_approvals`（28 欄） | 【推】 |
| 每個 LLM 輸出都要經 validator，錯誤降級成模板 | `PR/usecase/surface_validator.go`、`surface_realizer.go:60-90`、`public_text_policy.go` | 【量】檔案；LLM 失敗不致命（降級） |
| 擁有者欄位用 `buyer_id`／`store_id`，不用 account_id | `procurement.sql:5-6` 註解；根 AGENTS「身分」 | 【量】 |

### 1.3 碰到但不擁有的資料

| 資料 | 擁有者 | ai-service 怎麼碰 | A9 後 |
|---|---|---|---|
| 訂單清單／單筆／物流（助理工具） | ordering（Java order-service；單體 `legacy/orderservice`） | `AIi/client/order/adapter.go`（`GetMyOrders`、`GetOrder`、`GetSellerOrders`、`GetSellerOrderDetail`、`CreateRfqOrder`；`rpc-calls.csv`） | ordering 不在單體：用 legacy client 或等 A4（Q3） |
| 商品、庫存、分類樹 | catalog（Java product-service） | `AIi/client/product/adapter.go`（`GetProduct`、`GetProductList` 3 處、`GetStock`）、`client/category`（`GetCategoryTree`，timeout 10s） | 同上；分類樹是別名、召回的資料基礎（Q4） |
| 賣家 meta、stats、prospect 搜尋 | seller | `AIi/client/seller/adapter.go`（`GetSellerStats`、`BatchGetSellerMeta`、`SearchProspects`） | `seller` 已在單體；`seller_prospects` 由 seller 擁有（`M/seller/db`） |
| 買家 entitlement | buyer | `AIi/client/buyer/adapter.go:58`（`GetBuyerEntitlement`） | `buyer` 模組公開 API |
| 圖片上傳／引用 | media | `AIi/client/media/client.go`（`FinalizeImages`、`LinkImages`、`ReleaseReferences`） | `media` 已在單體，與寫列同 tx |
| 供應商邀請與 session | `agent`（單體既有） | `rfq_recipient_id` 屬 ai-service，agent 模組只驗正整數（`M/agent/AGENTS.md`、已刪除的 agent 模組規格第 67 行） | 採購進單體後，`agent` 的 `Prospects` port 與採購的接線要定（Q2） |
| 通知 | notification（舊） | ai-service 經 `procurement.events` exchange 發事件（`events.csv`） | A8 notification 搬遷時要接新事件（Q8） |

---

## 2. 領域區塊（資料、不變量、背景作業）

背景作業編號 B1…Bn。

### 2.1 通用 AI 助理（`AIi/agent`、`AIi/actor`、`AIi/handler`、`AIi/grpc/assistant_handler.go`）

- **結構**：每個角色（seller、buyer）一個 coordinator（ADK `llmagent`，`agenttool` 包 specialist；`AIi/agent/coordinator.go:60-75`）＋「按鈕」專職 runner（SellerOrders／Inventory／Payment、BuyerOrders／Tracking／ReturnPolicy／SellerMatch）＋ `OrderPresenter`、`ProductAutofill`（Gemini structured output，in-memory session）、`ProductListing`（`AIi/agent/di.go:99-145`）【量】。工具 8 個：`get_seller_orders`、`get_order_detail`、`check_inventory_shortage`、`query_payment_summary`、`get_my_orders`、`get_buyer_order_detail`、`track_my_package`、`search_candidate_sellers`、`search_category`、`update_draft`（`tools.go` 各 `Name:`）【量】。
- **Actor**：`internal/actor` 155 行，從 gRPC metadata 讀 `x-store-id`、`x-buyer-id`、`x-account-type`（`actor.go:15-60`）。**信任 gateway 注入**：`AccountTypeAdmin = "4"` 且註解自承無法區分真 admin 與代行 session（`actor.go:25-37`）。單體內改為從 session 取（M12、AD8）。
- **資料**：只用 ADK session 表（§1.1.1）；沒有自己的業務表。
- **不變量**：coordinator prompt 要求「禁止編造」「涉及退款賠償食安一律請客服」（`coordinator.go:23-33`）；多步驟委派是隨機的（`cmd/coordinator-eval/main.go:56` 實測 1/3 通過同一條 query）；2026-09-20 eval：coordinator 27/28、fallback 20/28（`AIi/agent/eval_notes.md`）【量】。
- **背景作業**：
  - **B1** session cleaner：`outbox.NewScheduler` 每 1 小時、啟動時先跑一次，刪 24 小時前的 session（`AIi/agent/cleaner.go:13-50`）【量】。單體以 River periodic job 取代。
- **資料流副作用**：SSE 串流是 AG-UI 格式，gateway 轉 12 支 SSE（計畫 A9 條第 253 行）；ai-service 自己只提供 gRPC server streaming（`assistant_handler.go:45-75`）。**`internal/handler` 的 `buyer_assistant.go`、`seller_assistant.go` 仍在，但 `rg 'NewBuyerAssistant|NewSellerAssistant'` 非測試無呼叫者**（【量】只命中註解；【推】HTTP 版 handler 是殘留，被 gRPC 取代）（AD9）。

### 2.2 採購（`PR`）

子區塊與規模（非測試非生成行數【量】`find PR/<dir> -name '*.go' ! -name '*_test.go' … | xargs cat | wc -l`）：

| 子區塊 | 行數 | 職責 |
|---|---|---|
| `usecase` | 8,041 | turn、answer、rewind、graph（ADK graph runner）、candidate search（platform／external 兩條 lane）、extract（LLM slot 抽取）、surface realizer／validator（LLM 潤飾敘述）、step narrator、run dispatch／lifecycle、interrupt、resume |
| `repository` | 6,157 | pgx＋sqlc 手寫包裝、run 帳本、LISTEN/NOTIFY（`run_watch.go`） |
| `domain` | 2,949 | slots、dialogue planner、claim codes、artifact 型別、run／step 狀態、fact tokens、normalize |
| `commerce` | 2,507 | RFQ 確認、發布預覽／執行、供應商回覆、買家接受 |
| `cropidentity` | 1,394 | 品項文字 → 平台分類樹節點（`resolver_v2.go`、`facet.go`、`taxonomy_recall.go`、`crop_gate.go`、`crop_category_model.go`） |
| `orderprovisioning`＋`publicationoutbox` | 1,384 | 兩支 outbox worker |
| `goldenset` | 778 | 離線評測（黃金查詢集、情境 corpus、報告） |
| `pricing` | 239 | 價格試算（區間 × 數量，**無行情**） |
| `graphspike` | 0 | **只有 `resume_spike_test.go`**（【量】`ls PR/graphspike`；spike 殘留，AD10） |

- **背景作業與 goroutine**（B 編號延續）：
  - **B2** 候選搜尋非同步：`SafeGo` 派發、`candidateSearchBudget`、兩條 lane 各一個 goroutine fan-in（`PR/usecase/candidate_search.go:223-292`）；終態以 fencing token 寫入；lazy reaper 於讀取時回收過期 lease（`PR/repository/run.go:455`）。
  - **B3** run 派發：`RunDispatcher.Dispatch` 用 `SafeGo(context.WithoutCancel)`＋`runDispatchBudget` timeout（`run_dispatch.go:122-150`）；**沒有 crash 後重驅**（AD2）。
  - **B4** 回答／rewind 的平行群組（`answer.go:201-239`、`rewind.go:150-181`、`graph_terminal.go:220-288`，`errgroup`）；目的是把 realize 與 narrate 併排以壓低延遲（`surface_realizer.go:22-25`）。
  - **B5** `WatchRun`：每條 stream 佔一條專屬 pgx 連線做 `LISTEN procurement_agent_run_<id>`，以 goroutine 讀通知，channel 容量 16，連線壞掉就關 channel（`run_watch.go:55-100`）。**每個 watcher 一條連線，沒有上限**（AD11）。
  - **B6** `rfq_publication_outbox` drain：每 1 分鐘 `outbox.NewScheduler`，`claimLease` 3 分鐘、`maxAttempts` 5、指數 backoff 1 分鐘至 1 小時、`fetchBatch` 50，發布到 RabbitMQ `procurement.events`（`PR/publicationoutbox/{di,worker}.go`；`publishConfirmTimeout` 低於 lease）；另有一個 monitor scheduler（`di.go:164`）【量】。
  - **B7** `order_provisioning_outbox` drain：同形狀，每次 gRPC `CreateRfqOrder`，5 秒 deadline（`grpcclient.Timeout5s`），5 次上限；`CONFIGURATION_MISSING` 需人工重放（`PR/orderprovisioning/worker.go:23-60`）。
  - **B8** category index TTL refresh：`SafeGo` 起 goroutine＋`singleflight`，30 秒 timeout，過期後下一次讀取觸發，失敗保留舊 index（`AIi/cache/category_index.go:215-232`）。
  - **B9** 同步 fire-and-forget NOTIFY 帶 timeout（`run_watch.go:14-30`）。
- **不變量補充**：
  - **Turn 在 HTTP 斷線時不取消**：`TestSubmitTurnStream_ClientDisconnectDoesNotCancelUsecaseContext`（`AIi/grpc/procurement_stream_test.go:116`）【量】。
  - 對外文字由 deterministic planner 先決定 act code 與 claim allowlist（`PR/domain/dialogue_planner.go:170-195`），LLM 只潤飾，驗證不過就退模板【量】。
  - 兩套 controller 的痕跡：`controller_kind IN ('DURABLE','GRAPH')`（`procurement.sql:960`），`ClaimAndDispatchTurn`（DURABLE）已無定義與呼叫者，turn 一律 `ClaimAndDispatchTurnViaGraph`，只有 publication 與 child run 寫 DURABLE（`PR/repository/publication.go:300`、`run.go:317`）【量】`rg 'ClaimAndDispatchTurn\('`（AD12）。

### 2.3 LLM provider（`AIi/llmprovider`）

- 兩個入口：`NewProcurement`（採購，由 `EXTRACTOR_MODEL` 決定）與 `NewAssistant`（助理，固定 `gemini`，`GEMINI_MODEL` 預設 `gemini-2.5-flash`）；皆**永遠不回 error**，缺 key 回 `disabledLLM` 並讓呼叫端回 `9703`（`provider.go:122,418-445`）【量】。
- 登記表 4 個模型：`gemini-2.5-flash`、`deepseek-flash`、`deepseek-v4-pro`、`google/gemma-4-26b-a4b-it:free`（OpenRouter）（`provider.go:112-119`）。
- 另有兩個**不經 `llmprovider` 的 LLM 呼叫**：修圖用手寫 `http.Client` 直打 Gemini `gemini-2.5-flash-image:generateContent`，`x-goog-api-key` header，60 秒 client timeout（`AIi/usecase/transform_usecase.go:62,293,358-364`）；`cmd/*` 的工具走 `llmprovider`。
- 指標：`ai_procurement_llm_enabled`、`…_config_incomplete`、`…_info{model,provider}`、`…_budget_limit`（刻意缺席而非 0，`provider.go:150-205`）（M8）。

### 2.4 cache（`AIi/cache`，521 行非生成＋863 行生成）

- `memory.go`：`sync.Map`＋TTL 的通用 cache（`New(ttl)`），非測試的使用者待查：`rg 'cache\.New\(' --glob '!*_test.go' AI`（未跑；【推】供 `recommend`／`style` 使用）。
- `category_index.go`：分類樹索引（`leafEntry`、`pathByID`、`tree`），`categoryIndexTTL`；`GetCategoryTree` 對 `status != "active"` 直接打 client；B8。
- `category_alias.go`＋`category_alias_gen.go`：別名兩層——人工 `synonyms`（永遠優先）＋生成層 `generatedCategoryAliases`（851 筆，`category_index.go:301` 查表）；`category_ancestors.go`、`category_tree_snapshot.go`（離線 gate，`testdata/category_tree_snapshot.json`）。
- 單體對應：`catalog` 模組的分類樹 API 取代 `client/category`＋TTL index（Q4）。

---

## 3. 沿用的機制 M1…Mn

「必要條件」＝這個機制存在需要什麼為真；「今天是否成立」打勾或叉；「業界做法」是從零設計會怎麼做。

| # | 機制 | 位置與做法 | 必要條件 | 今天是否成立 | 業界做法 | 建議去向 |
|---|---|---|---|---|---|---|
| M1 | **ID 方案**：全 TSID BIGINT（Go 端產生），ADK 表用 TEXT | `procurement.sql:4`、`PR/module.go:29`；`style_usecase.go:51` | Java 需要認得 ID（舊服務互通） | 否：對 Java 只有 `order_no`、`prospect_id`、`store_id`、`buyer_id` 是外部鍵（`idrefs.csv:19`），run／request／candidate 等自己的實體沒有 Java 消費者【推】 | 自己的實體用 UUIDv7（row-identity ADR）；外部鍵維持 bigint 到最後一個 Java 消費者退場（同 A6 M1） | 實體 UUIDv7；buyer／store／prospect 引用維持 bigint |
| M2 | **gRPC 內部服務＋method policy＋metadata 注入身分** | `AIi/grpc/method_policy.go`、`actor/actor.go`、gateway 注入 `x-store-id` 等 | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層（`internal/httpapi`）從 session 取身分，模組公開 API 收 `buyer.ID`／`store.ID` 參數 | 刪 gRPC 與 metadata；method policy 改為 Connect 層 interceptor 宣告（同 `adminauth`） |
| M3 | **ADK v2＋GORM session service＋pgx**（`google.golang.org/adk/v2 v2.6.0`、`gorm.io/gorm`）：session、event 持久化在自己 4 張表 | `AIi/agent/di.go:49-67`、`schema.sql:1-66`；採購用 graph runner（`PR/usecase/graph*.go`） | 需要 ADK 的 agent loop 與 AG-UI 事件 | 部分：助理 coordinator 靠 ADK；採購的狀態機其實由 run 帳本（31 表之一群）自持，ADK session 只當 graph 的 checkpoint（【推】`graph_session.go`） | 狀態機在業務表，agent loop 輕量；若留 ADK，用它的 pgx 原生儲存而非 GORM 第二套 ORM | Q5：保留 ADK 還是自寫；若保留，GORM 只在這一處且不外溢 |
| M4 | **LLM SDK**：Gemini 走 `google.golang.org/genai`；DeepSeek、OpenRouter 走 `openai-go/v3` 的 `/responses`；修圖手寫 HTTP | `llmprovider/provider.go:395-410`、`transform_usecase.go` | 需要多 provider 切換 | 部分：只有 `EXTRACTOR_MODEL` 一個開關；登記表 4 個、其中 gemma 是 `:free`（無 SLA）【量】 | 一個 `LLM` port＋provider adapter；明確的 timeout、retry、budget；修圖也走同一個 port | 保留 `llmprovider` 為 `platform/llm`（或模組內 `llm` 子套件）；修圖併入（Q1） |
| M5 | **金鑰與設定**：env 讀取（`config.go`，反射＋strict env 警告）；`GEMINI_API_KEY`、`DEEPSEEK_API_KEY`、`OPENROUTER_API_KEY`、`LLM_API_KEY`… | `AIi/config/{config,load,strict_env}.go` | 單一進程讀 env | 否：單體有統一設定載入（`cmd/hawkr`） | 設定集中在 composition root；secret 不進 repo | 併入 `cmd/hawkr` 設定；**`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL`（預設 `gpt-3.5-turbo`）全 repo 無讀取者**（`rg 'LLMApiURL\|LLMApiKey\|LLMModel' --glob '!*_test.go'` 只命中 `config.go:10-12`【量】）→ 刪（AD13） |
| M6 | **timeout／retry**：extractor 單次 30s（`extract.go:24`）＋`retryingExtractor` 一次（`extractor_provider.go:34-54`）；SDK 自帶重試關掉（`provider.go:88-106`）；realizer 4s、narrator 3s、crop 建議 timeout；run 預算 `runDispatchBudget`、`candidateSearchBudget`；gRPC client 5s／10s；修圖 60s；assistant 30s | 如左 | 外部 provider 要有 deadline | 成立：每個外部呼叫都有 deadline【量】 | 同；加 provider 層熔斷與併發上限 | 保留並集中到 `llm` 子套件；timeout 常數與 `observability` 的直方圖 bucket 鏡像要保持同步（`observability/metrics.go:12-60` 用測試守） |
| M7 | **prompt 儲存**：全部以 Go 字串常數寫在程式裡（coordinator、specialist、extractor、realizer、narrator、crop model、`return_policy.go`）；唯一存在 DB 的是修圖的 `ai_styles.prompt` | `AIi/agent/{coordinator,specialists,agents,return_policy}.go`、`PR/usecase/{extract,surface_realizer,step_narrator}.go`、`cropidentity/crop_category_model.go` | 需要不經部署修改 prompt | 否：沒有 prompt 版本欄位；`return_policy` 有「gateway 先讀 key `return-policy`、沒有值才退 ai-service 固定文案」的過渡（計畫 `:188`） | prompt 進程式碼並隨版本控管，加 eval gate；後台可改的文案走 content 模組 | 留在程式碼；`agent/return_policy.go` 與 content 的關係在 A9 結案（Q8）；**coordinator prompt 內有「生鮮賣家小幫手」「食安責任」「`農產品圖片`」等農產文案**（`coordinator.go:11`、`style_usecase.go:31`；`rg -c '生鮮\|農\|食安'` 在 agent 12 處）→ 隨 scope cut 改寫（Q4） |
| M8 | **observability**：OTel meter（24 個 instrument）＋Prometheus `promauto` gauge（`llmprovider` 6 個、兩支 outbox 各 1 個）；Loki／告警規則在 repo 外（`monitoring/…`，`provider.go` 註解引用） | `AIi/observability/metrics.go`；`llmprovider/provider.go:124-205` | 有 Prometheus／OTel pipeline | 單體：待定（計畫 A7 延後項說「單體告警管道」尚未建立，`go-modular-monolith.md:241-242`） | 單一 metrics 入口、標籤封閉基數 | 沿用指標名（告警規則依賴）；等單體告警管道（Q10） |
| M9 | **預約制配額（reserve→confirm／release）＋advisory lock 字串雜湊命名空間** | `db/queries/usage.sql`、`db/queries/procurement/quota.sql:6-13`；表 `ai_usage_reservations`、`procurement_usage_reservations` | 兩個進程併發，需要先佔再扣 | 部分：單體仍多 pod；但可用 `SELECT … FOR UPDATE` 於一列 counter，不需要預約表與雜湊鎖 | 一列 counter＋條件 UPDATE（`WHERE used < limit`），失敗即拒絕；預約只在長呼叫（Gemini 修圖，數秒）才有意義 | 採購配額簡化（counter）；修圖保留預約（呼叫外部 provider 不在 tx 內，單體規則 5）（Q7） |
| M10 | **LISTEN/NOTIFY 作 run 事件喚醒**，每條 stream 一條專屬連線 | `PR/repository/run_watch.go` | 多 pod 要跨進程喚醒 | 成立（單體多 pod），但連線數無上限（AD11） | 同；或 DB 輪詢（`last_event_seq`）＋單連線 listener 扇出 | 保留；單體用一條共用 listener 扇出到 in-process channel（Q6） |
| M11 | **兩支 outbox（`pkg/outbox` Scheduler＋claim lease CAS）**：rfq publication、order provisioning | `PR/publicationoutbox`、`PR/orderprovisioning` | 要可靠地外送 MQ／gRPC | 否：單體規則 4、5：tx 內 `events.Publish`＋`jobs.Enqueue`（River），provider 在 tx 外由 worker 呼叫 | River job 帶冪等鍵，重試與退避由 River 管 | 兩支 outbox 改 River job；`procurement.events` exchange 的舊形狀在 notification 搬完之前由 relay 維持（前例 `legacy/sellerrelay`、`socialrelay`）（Q8） |
| M12 | **用 gRPC client 取別的服務資料**（6 個 client） | `AIi/client/*` | 資料在別的服務 | 部分：buyer、seller、media 已在單體；ordering、catalog 還在 Java | 模組公開 API；未搬的走 `internal/legacy/*` | buyer／seller／media 換成模組 API；order／product／category 走 legacy 或等 A4／A5（Q3） |
| M13 | **cmd 工具**：`category-alias-gen`（526）、`prospect-category-gen`（557）、`resolver-eval`（812）、`coordinator-eval`（298）、`extractor-benchmark`（160）、`grpc-healthcheck`（46）、`ai-service`（7） | `AI/cmd/*` | 需要離線評測與生成 | 部分：`resolver-eval` 打舊 REST（§4.2）；`prospect-category-gen` 直連舊 `hawkr_seller` DB 並讀 `INFRA_VM_IP`（`cmd/prospect-category-gen/prospects.go:23`，公司時期基礎設施變數名，違反根規則 7 的精神，AD14）；`grpc-healthcheck` 單體不需要 | 評測工具重寫成打 Connect 或 in-process；生成器吃檔案而非連線 | `grpc-healthcheck` 刪；`coordinator-eval`、`extractor-benchmark` 搬到 `cmd/` 下以 in-process 呼叫；`resolver-eval` 重寫；`category-alias-gen`、`prospect-category-gen` 依 Q4 |
| M14 | **生成檔 `category_alias_gen.go`**：851 筆別名，由 LLM（deepseek-flash／samples=3／minAgree=2／temperature=0.7）從農產分類樹生成 | `AIi/cache/category_alias_gen.go:1-6`；`cmd/category-alias-gen` | 分類樹穩定、別名準 | **否**：針對農產 L2 分類（id 4–222）；seed 分類樹是 id 4–33 的小樹（`backend/infra/seed-data/04-product.sql:93-125`），分類 ID 不對得上【量】 | 召回用 embedding 或搜尋索引＋人工 synonym；別名表隨分類樹資料化 | Q4：重新生成、改資料化或刪除；離線 gate（`category_alias_gate_test.go`）是好的設計，留 |
| M15 | **離線評測資產**：`goldenset`（778 行＋`testdata/golden_queries.yaml`、情境 corpus）、`cmd/coordinator-eval/testdata/corpus.yaml` | `PR/goldenset` | 需要 regression 基準 | 成立，但 golden 查詢仍是農產品項（「蘋果」「小白菜」「豬絞肉」，`cropidentity/README.md:5`）【量】 | 一般電商品項重建 golden set | Q4 |
| M16 | **簡繁轉換 `opencc`**（`longbridgeapp/opencc`）：LLM 抽取結果強制轉繁體 | `PR/zhtw/convert.go`（36 行） | LLM 輸出語域不穩 | 成立（2026-08-15 實測案例） | 同 | 保留，併入採購 |
| M17 | **fx 組裝＋`pkglogger`（zap）＋`syncutil.SafeGo`** | `AIi/**/di.go`、`go-pkg` | 單進程 DI | 否：單體 composition root 手動接線 | 同單體規則 | 全改成 `cmd/hawkr` 接線；`go-pkg/{identity,tsid,outbox,mq,errors/bizcode,grpcclient}` 要換成單體的 `platform` 對應 |
| M18 | **bizcode 數字碼＋CodeMeta**（`9703` 等） | `AIi/domain/errors.go`、`llmprovider/provider.go:122`、`AIi/grpc/procurement_errors.go` | 舊 REST 契約 | 否：新契約用 `ErrorReason` | `api/` 的 `ErrorReason`＋Go 型別 | 全部轉 `ErrorReason`；`procurement_error_reason_registry_test.go` 已有註冊表概念，可沿用【量】 |
| M19 | **雙軌欄位：`controller_kind`（DURABLE／GRAPH）** | `procurement.sql:960`、`domain/run.go:60-68` | 兩條執行路徑並存 | 否：`ClaimAndDispatchTurn` 已刪，欄位成了單值 | 不留 | 刪欄位或固定 `GRAPH`（AD12） |
| M20 | **lazy reaper（讀取時回收過期 candidate set）** | `PR/repository/run.go:455` | 沒有背景 sweeper | 否：單體有 River | River periodic job 掃 `lease_expires_at`（索引已有，`procurement.sql:208-210`） | 併入 B3 的 sweeper（Q6） |

---

## 4. Pricebook／農產行情殘留與延後項

### 4.1 殘留盤點（ADR D2 後 A9 不得接回）

`rg -n -i 'pricebook|price_book|行情|market_ref|market_basis|MarketPrice|marketquote' AI --glob '!db/migrations/*'` 的非測試命中只有：`AIi/usecase/style_usecase.go:31`（修圖 prompt「農產品圖片」）；其他命中皆測試／註解（`AIi/cache/category_index_test.go:473` 註解「17 元/kg 的葡萄柚行情」、`PR/domain/seller_facing_payload_test.go:69-80` 斷言 `marketQuote` 不出現）【量】。DB：`20261027181357_drop_procurement_market_reference.sql`、`20261028164119_drop_quality_knowledge_state.sql`、`20261028100000_drop_seller_ranking_facts.sql` 已刪行情／品質／排行相關表欄【量】；`procurement_price_previews` 現只剩區間欄位（`procurement.sql:319-333`）【量】。

仍要處理的殘留（皆非「接回」，是名實不符）：

| # | 項目 | file:line | 內容 | 建議 |
|---|---|---|---|---|
| P1 | `ClaimPriceReferenceAvailable = "PRICE_REFERENCE_AVAILABLE"` 與 `ActPricePreviewAvailable` | `PR/domain/claim_codes.go:17`；`PR/domain/dialogue_planner.go:189`；`testdata/dialogue_plan/price_preview_available_*.json:25` | 「價格參考可用」的 claim 在行情移除後只剩「試算區間可用」；名字暗示有參考價 | 【推】改名為 `PRICE_PREVIEW_AVAILABLE` 或確認文案不誤導；walkthrough 見 Q9 |
| P2 | `crop` 命名（農作物）貫穿程式：`cropidentity`、`CropSelection`、`CropDisambiguation`、`SlotCropDisambiguation = "crop_disambiguation"`（`PR/domain/pick_next_question.go:33`）、`CropOtherOptionID = "crop-other"`、`procurement_agent_run_interrupts` 的 payload kind、gRPC 欄位 | `PR/cropidentity/*`、`PR/domain/crop_*.go`、`AIi/grpc/procurement_handler.go:711-718` | 一般電商沒有「作物」；`slotKey`、`optionId` 是對外契約字串 | 【推】A9 定新 Connect proto 時一併改名（`item_disambiguation`、`other`）；上線前不受相容性約束 |
| P3 | 農產 prompt 與別名 | `AIi/agent/coordinator.go:11`（「生鮮賣家小幫手」）、`style_usecase.go:31`、`category_alias_gen.go` | 見 M7、M14 | Q4 |
| P4 | 搬遷清單過時 | `rpc-pairs.csv:24`（`ai-service→pricebook-service` 6 支）、`rpc-calls.csv:` PricebookService 6 行、`rpc-servers.csv:13,20`、`event-pairs.csv:45,56`、`replicas.csv:36`（`seller_ranking_facts`）、`db-tables.csv:2`（41 張） | 與現況不符（AD6） | 在 A9 開頭由 `scripts/check-migration-inventory.py` 對帳後修清單 |

### 4.2 延後項審查（計畫 `go-modular-monolith.md:256`）

| 項目 | 結論 | 證據 |
|---|---|---|
| **`cmd/resolver-eval` 的 `observe` 仍照拿掉行情前的回應形狀解析** | **成立，且比預期嚴重：整支工具打的是舊 gateway REST，不只 `observe`**。它：① 用 `JWT_SECRET` 自鑄 HS256 dev token（`main.go:21-48`），單體的身分是 session（ADR `identity-self-hosted-federated-login`），此法失效；② POST `/api/v1/app/buyers/me/procurement/turns`、`/app/buyers/me`（`main.go:232-248`）；③ 解析 `{code, data}` 信封（`code != 0` 視為失敗，`main.go:250-254`）；④ `slots.item_resolution.display_value` 與 `label` 二選一（`main.go:134-139`）——現行 `ItemResolution` 只有 `label`（`PR/domain/item_resolution.go:14-20`），`display_value` 是舊欄位的退路；⑤ 以 `question.slotKey == "crop_disambiguation"`、`optionId` 前綴 `category-`、`crop-other` 辨識（`main.go:142-166`）；⑥ 分類樹來自 `categoryPaths(*base)`（public REST）。`observeLive`（`scenario.go:141-168`）同樣讀 `slots`、`missingRequired` 的舊 JSON 形狀。 | 【量】`cmd/resolver-eval/main.go`、`scenario.go`；`git log` 最近一次動它是 `5d591063`、`a849cc9c`（行情移除當時）【量】 |
| **`Unavailable` 的命名** | **不再名實相符**。結構體文件寫「沒有更多東西要問（品項已定到分類樹的某個節點，或分類樹找不到）」（`PR/cropidentity/crop_disambiguation.go:9-14`）；`crop_gate.go:79-90` 對它的處理是「`Selection` 有 `CategoryID` → `CATEGORY_CONFIRMED`；沒有 → pending，原因 `CropPendingReasonUnavailable = "UNAVAILABLE"`（`domain/crop_resolution.go:8`）」。兩個不同語意（已定案、查無）擠在同一個布林，名字是「不可用」，是行情時代「行情不可用」的遺留。 | 【量】`rg 'Unavailable' PR/cropidentity`；README `:24` |
| **`categoryDescribeStage` 的命名** | **不再名實相符**。它回傳的就是 `DisambiguationResult{Unavailable: true, Selection: …}`（`crop_category_fallback.go:340-346`），被 `resolver_v2.go`、`multi_item.go`、`crop_category_fallback.go` 共 9 處呼叫；語意是「這個品項不用再問，進入『由買家自行描述』或定案」。「describe stage」是舊三階段（行情卡、分類卡、描述）中的最後一階，前兩階已刪。 | 【量】`rg -n categoryDescribeStage` 9 處（呼叫者見 `resolver_v2.go:15,63,96,103,126`、`multi_item.go:59`、`crop_category_fallback.go:353-412`） |
| **`cropidentity.GateOption` 的 variadic 舊簽章** | **成立，且理由消失**。註解寫「用 variadic 是為了不動既有呼叫端與測試的簽章」（`crop_gate.go:38`）；只有一個選項 `AwaitingBuyerAnswer()`（`:47`）。呼叫者只有 3 個：`usecase/graph_nodes.go:625-628`（傳選項）、`answer.go:125`、`rewind.go:81`（不傳）。從零設計是 `ComputeGate(ctx, resolver, before, after, ComputeOptions{AwaitingBuyerAnswer bool})` 或兩個函式，不為「不改簽章」留可變參數。 | 【量】`rg -n 'ComputeGate\(\|AwaitingBuyerAnswer' --glob '!*_test.go'` |

---

## 5. 既有缺陷 AD1…ADn

disposition：修＝A9 內順手修；接受＝記進 CARRYOVER；等決定＝要 Will 決定。

| # | file:line | 描述 | 證據 | 建議 |
|---|---|---|---|---|
| AD1 | `AIi/agent/tools.go:307` | 收款統計以 `"30"` 查「已完成訂單」，但 `orderStatusMap` 的 `30=配送中`、`50=已完成`、`80=已取消`（`tools.go:23-31`）；工具 schema 的註解也寫 `20=待出貨`（`tools.go:50`）；coordinator prompt 另寫 `30=已完成, 40=已取消`（`coordinator.go:31,53`），`tools.go:129` 還寫「0/10/20/30/40」。賣家問「收了多少款」得到的是配送中訂單；LLM 讀到的狀態碼表是錯的，可能把 30 當完成、40 當取消。 | 【量】同檔三處矛盾；對照 `pkg/codemeta/labels.go:54-58` 與 review 的 `OrderStatusCompleted = "50"` | 修（單體內用 ordering 的公開型別；狀態碼表只定義一處） |
| AD2 | `PR/usecase/run_dispatch.go:310-316`、`turn.go`（註解同） | 沒有 sweeper：commit 後 dispatch 前／中 crash，run 永遠停在 QUEUED／RUNNING（`WAITING_FOR_INPUT` 會經 interrupt 到期？未驗），買家端看到永遠轉圈；child run 同樣。`procurement.sql:914` 寫 lease 欄位「留給 Track 2」。 | 【量】註解自承；【推】impact | 修（River periodic sweeper，Q6） |
| AD3 | `AIi/cache/category_alias_gen.go:3`、`category_index.go:301` | 別名層對到舊農產分類 id（4–222），與一般電商分類樹無關；`generatedCategoryAliases[key]` 命中就覆寫買家的詞，會把「蘋果」對到不存在或不相干的分類。 | 【量】檔頭；【推】後果依單體 catalog 的分類樹內容而定（未讀 `catalog`，因為模組不存在） | 等決定（Q4） |
| AD4 | `AI/db/schema/schema.sql:120-143` | `candidate_leads` 已無讀寫者，仍在 schema。 | 【量】`rg candidate_leads` | 修（不搬；單體 schema 不建） |
| AD5 | `PR/usecase/surface_realizer.go:58`、`AIi/llmprovider/provider.go:60-65` 註解 | 引用不存在的守門腳本 `scripts/check-llm-client-construction.py`。其目的是防止第二份 LLM client 建構（曾漏掉 DeepSeek `reasoning.effort=none`，正式端點 100% HTTP 400，靜默退模板，2026-09-11 事故）。 | 【量】`find . -name 'check-llm-client-construction*'` 無結果 | 修：改成 arch 測試（只有 `llmprovider` 可 import `openaimodel`／`gemini.NewModel`），或確認腳本在別的 repo |
| AD6 | `docs/plans/2026-09-24-go-modular-monolith/*.csv` | 搬遷清單與現況不符：多 2 支 RPC、6 支 pricebook RPC、2 條事件、1 個副本、1 張表（§0、§4.1 P4）。`AGENTS.md` 規則 8 要求刪舊服務前清單 `disposition` 填滿 | 【量】 | 修（A9-0 對帳） |
| AD7 | `PR/domain/artifact.go:136-143`、`AIi/agent/tools.go:525-535` | 候選的 `PublicWebsite` 可能是社群網址（CRM1 名單的「連結」欄映射），`research_summary`、`positioning`、`product_summary` 是原樣透傳的自由文字；兩份 DTO 鏡像（gateway 一份、ai-service 一份）靠欄位挑選當唯一屏障。 | 【量】註解自承「已知缺口，另案處理」 | 等決定（Q10 的資料分級）：A9 把 DTO 只留一份、值層過濾或改欄位語意 |
| AD8 | `AIi/actor/actor.go:25-37` | admin 判斷 `x-account-type == "4"`，註解自承分不出真 admin 與能力受限的代行 session；今天靠路由在 `requireAdmin()` 之後不可達 | 【量】 | 修（單體內 `adminauth` 宣告權限；A10 delegation 之前不得開放） |
| AD9 | `AIi/handler/{buyer,seller}_assistant.go`（341 行） | HTTP SSE 版 handler 無非測試呼叫者，只剩 gRPC 版與註解引用（`AIi/grpc/assistant_handler.go:171,277`） | 【量】`rg 'NewBuyerAssistant\|NewSellerAssistant' --glob '!*_test.go'` 無呼叫；【推】殘留 | 修（不搬） |
| AD10 | `PR/graphspike/resume_spike_test.go` | 只有測試檔的 spike 套件 | 【量】`ls` | 修（不搬） |
| AD11 | `PR/repository/run_watch.go:55-100` | 每條 `WatchRunEvents` stream 佔一條專屬 pgx 連線做 LISTEN，pool 上限內沒有 per-process 或 per-buyer 的 watcher 上限；連線耗盡會連帶影響一般查詢（註解承認「洩漏 pool 一個連線」風險） | 【量】程式；【推】impact 未壓測 | 修（共用 listener 扇出，Q6） |
| AD12 | `procurement.sql:960`、`PR/domain/run.go:60-68` | `controller_kind` 兩值，`DURABLE` 路徑的 turn 入口已刪，欄位在 turn 上永遠 `GRAPH` | 【量】`rg 'ClaimAndDispatchTurn\('` 無結果 | 修（刪欄位或固定） |
| AD13 | `AIi/config/config.go:10-12` | `LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL`（預設 `gpt-3.5-turbo`）無讀取者；`EXTRACTOR_PROVIDER`（`StaleExtractorProvider`）只用於警告（`provider.go:287`） | 【量】 | 修（刪） |
| AD14 | `cmd/prospect-category-gen/prospects.go:23` | 以 `INFRA_VM_IP` 組 `hawkr_seller` 的 DSN（含 `DB_PASSWORD` 環境變數）直連舊 DB；公司時期的基礎設施命名 | 【量】 | 修（改吃檔案或呼叫 seller API，Q4） |
| AD15 | `AIi/agent/tools.go:572-600`、`PR/usecase/candidate_external_lane.go` | EXTERNAL lane 的訂單賣方用 `PROCUREMENT_SETTLEMENT_STORE_ID`（`config.go:33-40`），未設定時 EXTERNAL lane 的 provisioning 終態失敗（`PR/orderprovisioning/di.go:58`）；根據計畫該 lane 的「受託代銷」形狀已於 2026-09-21 退場（`worker.go:52`），但 settlement store 仍是設定值 | 【量】 | 等決定（Q11：EXTERNAL lane 在一般電商是否還成立） |
| AD16 | `PR/orderprovisioning/worker.go:52`、`PR/usecase/graph_dispatch.go:787` | 註解仍引用 `PAYUNI_AGENT_MER_ID`、`backend-payuni CLAUDE.md`（公司時期） | 【量】 | 修（改寫註解；根規則 7） |
| AD17 | `AIi/usecase/transform_usecase.go:201,224`、`transform_admission.go:44` | 修圖 usecase 以 `context.WithoutCancel` 加 10s timeout 做收尾；`os.Getenv(envInFlightBytesBudget)` 繞過 `config`（`strict_env.go` 另列為例外） | 【量】 | 接受（搬時併入 config） |

---

## 6. 設計問題 Q1…Qn（需要 Will 決定）

排序：先擋 A9 範圍與資料模型的，後面是細項。每題附建議，但建議不是結論。

- **Q1（範圍）AI 修圖＋配額＋後台風格管理＋個人化推薦要不要進 A9？** 任務與計畫只說「agent」；ai-service 實際還有 `AIImageService`（2 支）、`AIAdminService`（7 支）、`RecommendationService`（1 支）與 `ai_styles`、`ai_usage`、`ai_quotas`、`ai_usage_reservations` 4 張表（約 1,850 行，`AIi/usecase`）。建議：A9 分成 A9a（助理＋採購）與 A9b（修圖與推薦），修圖歸 `media` 或獨立 `imageai` 模組（它的資料是 store 配額與風格，不是採購）。這決定 A9 的 schema 邊界。
- **Q2（模組邊界）採購與通用助理放哪、叫什麼？** 單體的 `agent` 已是「供應商入口存取」（`M/agent/AGENTS.md:3`）。選項：(a) 採購一個模組 `procurement`（RFQ 全流程）、助理一個模組 `assistant`、`agent` 維持不變；(b) 全部併進 `agent`。業界做法：以聚合邊界分，RFQ 狀態機與聊天助理沒有共用資料；建議 (a)，且依賴表要先改（`internal/arch/modules.go:39` 的 `agent` 列只有 `ordering,catalog,buyer,seller,media`，採購還要 `notification` 的反向事件與 `agent` 的 `Prospects` port）。
- **Q3（前置）ordering、catalog 還沒搬進單體，A9 怎麼接？** 助理工具與採購建單、候選搜尋都靠 order、product、category。選項：(a) 經 `internal/legacy/orderservice`、`productservice` 的 port（模組只依賴 interface，A4／A5 完成再換）；(b) A9 排在 A4／A5 之後。建議 (a)，但要先確認 legacy 有沒有 `GetSellerOrders`、`GetMyOrders`、`CreateRfqOrder`、`GetStock`（`internal/legacy` 現有清單只見 `orderservice`、`productservice`，方法清單未逐一核對，【推】要補）。
- **Q4（農產殘留）一般電商分類樹不同於農產樹時，分類別名、召回、golden set、prompt 要不要重做？** 現況：別名 851 筆全是農產 L2（M14、AD3）、golden queries 是蘋果／小白菜（M15）、prompt 內有「生鮮」「食安」（M7）。選項：(a) 先搬、以 feature flag 關掉別名層，之後重生成；(b) 搬之前先重做（成本：LLM 呼叫與 eval）；(c) 刪別名層，只用人工 `synonyms`＋分類樹召回。建議 (a)＋刪別名層的可行性實驗（先量化別名層對 golden set 的貢獻，因為若貢獻小，(c) 最簡單）。需要 Will 回答：新的一般電商分類樹是什麼？
- **Q5（技術）ADK v2＋GORM session 要保留嗎？** 影響：4 張 session 表、GORM 第二套 ORM、AG-UI 事件格式。採購狀態機其實在 31 張業務表自持（run 帳本），助理的對話記憶才真的靠 ADK。選項：(a) 保留 ADK，session 表換成它的 pgx 原生儲存；(b) 助理不留跨輪記憶（`SellerLatestSession` 靠它，`assistant_handler.go:279-301`，需要查產品語意）；(c) 自寫。建議 (a)，理由：重寫成本大於維護成本，但 GORM 要隔離在一個子套件。
- **Q6（可靠性）run 的 crash 恢復與 `WatchRun` 的實作？** AD2、AD11：單體有 River，可以做 sweeper（掃 QUEUED 超時、RUNNING 無心跳）與共用 LISTEN 連線。問題：sweeper 重驅的語意——重跑 LLM 呼叫可能產生不同結果（冪等？），還是直接標 `FAILED`／`EXPIRED` 並讓買家重送？建議「標失敗＋提示重送」，不重驅（LLM 不是冪等的，ADR D6 的精神）。
- **Q7（成本／資料模型）配額機制要不要合併？** 修圖（store＋週）與採購（buyer＋期間）各一套預約制（M9）。業界做法：一列 counter＋條件 UPDATE；預約只留給長外部呼叫。同時要問：採購每期上限與修圖每週上限是產品數字，現在寫在哪（`quota_usecase.go`、`procurement_usage` 的 config）？它們會不會因一般電商改變？建議：兩者合併成 `quota` 子套件（key＝(kind, subject_id, period)），採購不用預約。
- **Q8（契約）AG-UI 的 12 支 SSE 與 `procurement.events` 的舊事件形狀怎麼辦？** 計畫第 253 行已把 D7 的決定丟給 A9：改 server streaming，還是保留 SSE 為第二套契約。額外：notification（A8 之前）仍消費 `procurement.rfq_publication_invite.created`／`external_created`（`events.csv`），A9 要用 relay 維持舊 routing key 與 payload（前例 `legacy/sellerrelay`），A8 完成後換成單體事件。建議：助理與採購事件都 server streaming；瀏覽器要雙向才留 WebSocket；舊事件形狀用 relay 到 A8。
- **Q9（命名與契約字串）要不要在 A9 把 `crop_*`、`PRICE_REFERENCE_AVAILABLE`、`Unavailable`、`categoryDescribeStage`、`crop-other`、`slotKey` 全改名？** 上線前不受相容性約束，A9 也要重做 Connect proto，是最便宜的時機（§4.1 P1、P2；§4.2）。建議全部改，並把 `ComputeGate` 的 variadic 換成明確的 options 型別。
- **Q10（資料分級）候選廠商的聯絡資訊與自由文字怎麼保證不外洩？** AD7：現況只靠 DTO 欄位挑選，兩份鏡像。單體只有一個 DTO，但值層過濾（社群網址辨識）與 `research_summary` 等自由文字仍要決定：整欄不給 LLM／買家，或消毒後給。同題：prompt 與回覆能不能含 seller_prospect 的哪些欄位。
- **Q11（產品）EXTERNAL lane（站外業者，不是平台賣家）在一般電商還成立嗎？** 成立則：邀請信（`agent` 模組，`vendor_portal_invites` 的 email 是模組唯一個資）、`PROCUREMENT_SETTLEMENT_STORE_ID` 的結算 store（AD15）、站外供應商報價後的建單賣方都要保留；不成立則採購只剩 PLATFORM lane，砍掉 `candidate_external_lane.go`、`candidate_selections` 的 lane 欄、`approval_recipients` 的外部收件人、`publication_invites` 的 external 分支與 `procurement.rfq_publication_invite.external_created` 事件，A9 大幅縮小。這是最大的範圍槓桿，建議先決定。
- **Q12（可觀測性）單體尚無告警管道，`ai_procurement_llm_*` 指標與 Loki／Prometheus 規則（`provider.go` 註解引用 `monitoring/prometheus/rules/service-alerts.yml`，本 worktree 未驗證存在）怎麼接？** 計畫 A7 延後項已記「單體告警管道建立時」（`go-modular-monolith.md:241-242`）。建議 A9 沿用指標名並列為該延後項的消費者，不另建。
- **Q13（模型）登記表的 `google/gemma-4-26b-a4b-it:free`（OpenRouter 免費額度）要不要進單體？** 無 SLA，免費額度常被限流；只在實驗用。建議不搬（只留 gemini 與 deepseek 兩家，各一個預設模型）。

---

## 7. 規模（非測試、非生成 LOC）

指令：`find <dir> -name '*.go' ! -name '*_test.go' ! -name '*_gen.go' ! -name '*gen.go' ! -name '*.pb.go' ! -path '*/gen/*' ! -name 'testing.go' -print0 | xargs -0 cat | wc -l`（在 `AIi` 下跑；`cmd` 在 `AI` 下跑）。【量】

| 區塊 | 目錄 | 行數 |
|---|---|---|
| 採購 usecase | `procurement/usecase` | 8,041 |
| 採購 repository | `procurement/repository` | 6,157 |
| 採購 domain | `procurement/domain` | 2,949 |
| 採購 commerce | `procurement/commerce` | 2,507 |
| 採購 cropidentity | `procurement/cropidentity` | 1,394 |
| 採購 outbox 兩支 | `procurement/{orderprovisioning,publicationoutbox}` | 1,384 |
| 採購 goldenset（評測資產） | `procurement/goldenset` | 778 |
| 採購 pricing／zhtw／handler／testkit／module | `procurement/{pricing,zhtw,handler,testkit}`＋`module.go` | 約 520 |
| **採購小計** | `procurement` | **23,901** |
| gRPC server（含採購 handler） | `grpc` | 3,713 |
| 助理 agent＋actor＋HTTP handler | `agent`、`actor`、`handler` | 2,154（其中 HTTP handler 341 為殘留，AD9） |
| 修圖／配額／推薦／候選 usecase | `usecase` | 1,850（`style` 365、`transform` 420、`quota` 96、`recommend` 232、`candidate_search` 152） |
| 外部 client | `client` | 886 |
| LLM provider | `llmprovider` | 447 |
| cache（不含生成） | `cache` | 521（生成 `category_alias_gen.go` 863） |
| config／infra／app／observability | | 960 |
| domain（助理共用） | `domain` | 119 |
| cmd 工具 | `AI/cmd/*` | 2,406（`resolver-eval` 812、`prospect-category-gen` 557、`category-alias-gen` 526、`coordinator-eval` 298、`extractor-benchmark` 160、`grpc-healthcheck` 46、`ai-service` 7） |
| **合計** | `AI` | **37,086**（以單一 `find` 量；各區塊加總約 36.3k，差額在 `procurement` 根與小檔） |

預估搬遷體量（【推】）：A9a 採購＋助理約 28k 行中，`goldenset`、`cmd` 工具、`graphspike`、HTTP handler、`candidate_leads`、`grpc` 層（被 Connect 取代）可不搬或重寫，真正要「沿用盤點」的是 `usecase`、`repository`、`domain`、`commerce`、`cropidentity` 約 21k 行；Q11 若砍 EXTERNAL lane，約再少 1.5–2.5k（【猜】，未量）。

---

## 8. 這份盤點沒做的事

- 沒讀 proto 與 gateway 的 SSE 處理（`gateway/internal/handler/ai.go`、`procurement_*`），12 支 SSE 的形狀未驗證。
- 沒有逐一核對 `internal/legacy/orderservice`、`productservice` 是否已有 A9 需要的方法（Q3 的前提未驗證）。
- 沒有跑任何測試、服務或外部 API；`PR/usecase` 的 graph 程式（`graph*.go`，約 3k 行）只讀了註解與簽章，graph 內部不變量未盤。
- `monitoring/` 與 `k8s/` 在本 worktree 的位置未查，告警規則是否存在未驗證（M8、Q12）。
