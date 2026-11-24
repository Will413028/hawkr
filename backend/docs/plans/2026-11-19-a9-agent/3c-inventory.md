# A9-3c 盤點（唯讀；worktree `.worktrees/a9-3c`，分支 a9-3c，含未合併的 A9-3c-0）

標記：【量】＝開檔或跑指令看過；【推】＝讀程式推論；【憶】＝憑記憶的業界做法，未查證。
路徑縮寫：PLAN=`backend/docs/plans/2026-11-19-a9-agent.md`；MOD=`backend/internal/modules/procurement`；OLD=`backend/services/ai-service/internal/procurement`；PROTO=`api/hawkr/procurement/v1alpha1/procurement.proto`；LEG=`backend/internal/legacy`。

## 0. SSOT 摘要（照原文，3c 範圍）

- A9-3c 步驟（PLAN:206）：賣家採購 6 支、經 legacy ordering 建單、`procurement.*` relay。驗收：relay 只在測試啟用（不變量「單一生效者」，PLAN:43）、把產生的 payload 與 golden fixture 逐欄比對；fixture 從舊 ai-service outbox 寫入程式以相同輸入產生，放附件 `fixtures/events/`；mutation：改一個欄位名比對要失敗。不能動：buyer、seller 模組；pricebook 不接回；舊 `backend/services/ai-service`（PLAN:205 的 3c-0 欄，及 3c 欄「不能動」）。
- 停止條件（PLAN:209）：A9-3c 開工時先定 WISH_POOL 查詢來源（A9-D44 已刪）、加賣家面讀 publication 的查詢（以 invite `store_id` 限定）、品相欄位是否屬範圍（A9-D44(3) 已換成 `spec_note`）；驗收加：注入建單失敗後 acceptance 轉建單失敗、買家可再接受（A9-D19 S6）。**這三個停止條件的決策都已由 A9-D44 吸收**，本盤點只剩下方第 6 節的新問題。
- A9-D19 S4（冪等鍵放實體，非建立型是目標狀態、不建收據表；CARRYOVER.md:32、95-111）；S5（hash 由 Go 重算，表裡無 hash 複本）；S6（刪沒有寫入者的狀態值，`write_closed_reason` 只剩 `DECLINED`，schema.sql:247-270）；S10（BuyerDeleted 保留賣家可見的匿名紀錄，`buyer_id` 設 NULL，db/queries/buyer_deleted.sql）；S6 修訂（建單 River job 重試用盡 → acceptance 轉 `ORDER_FAILED`、解除 live 鎖，PLAN:129）。
- A9-D29(5)（PLAN:139）：排除封鎖這位買家的店，搜尋時讀取失敗即搜尋失敗；**發布時再查屬 A9-3c**。
- A9-D35（PLAN:145）：發布核准存 `candidate_set_id`、價格區間、`buyer_note`；賣家可見內容由核准＋需求版本組出；`AcceptSellerQuote` 比對賣家實際看到內容的 hash。
- A9-D36（PLAN:146）：`rfqhash.SellerResponse`，SubmitSellerResponse 重用同一函式。
- A9-D44（PLAN:154）：(1) 刪 WISH_POOL，邀請列是賣家可見性唯一依據；(2) 賣家端讀 publication 以 invite 的 `store_id` 限定、不依賴 `buyer_id`、查不到一律 NOT_FOUND；(3) `OfferedQuality`→選填 `spec_note`，hash 原像升 v2。**實際狀態**：3c-0 提交 `5e1f6091f` 的訊息是「freeze hash versions only after launch」，`rfqhash.SchemaV1` 仍是唯一版本、`spec_note` 直接進 v1（`MOD/internal/rfqhash/rfqhash.go:6`、`response.go:30-41`）【量】。與 D44(3) 字面「升為 v2」不同，與「上線前不受相容性約束」（hawkr AGENTS.md）一致；建議在 PLAN 補一句更正（不是待決）。
- 不變量（PLAN:42-43）：notification 收到的 `procurement.*` wire 形狀不變（A8 之前）；relay 與 job 只在測試啟用，刪路由的同一個變更才在 `cmd/hawkr` 接上（A9-6，PLAN:232）。

## 1. SellerProcurementAppService 6 支

通則（PROTO:140-141 檔頭）：店家取自呼叫者目前的店家，需要賣家能力 `CAP_MARKETING`；只能看到發給自己的詢價與處理自己的回價，其他一律 `not_found`。現有慣例：能力檢查在模組內（`seller.Authorize`，寫入在 tx 內、讀取在查詢前），handler 只推導店家（`seller.CurrentStore`），宣告表 `declarations` 雙向對照 service 方法（`httpapi/crm/declaration.go:14-30`）【量】。`TestEveryServiceInTheContractIsMounted` 仍紅，等 3c 掛載（PLAN 3b-3d 列）。現有範本 `backend/internal/httpapi/buyerprocurement/`；賣家端沒有對應 package（`ls internal/httpapi` 無 sellerprocurement）【量】。

欄位與 field_behavior 引自 PROTO:1142-1285；錯誤引自 PROTO:150-170 與 `api/hawkr/common/v1alpha1/error_reason.proto:116-122`。

| RPC | 請求 | 回應 | 契約錯誤 |
|---|---|---|---|
| ListSellerPublications（NO_SIDE_EFFECTS） | `page_size`（0→20，>50→50，負數 INVALID_ARGUMENT）、`page_token` | `publications[]`（`SellerPublicationListItem`：summary、`SellerFacingRfq`、optional `latest_response_status`、`has_draft`）、`next_page_token` | 無店家能力→PERMISSION_DENIED／FAILED_PRECONDITION（seller.Authorize）；token 壞→INVALID_ARGUMENT |
| GetSellerPublication（NO_SIDE_EFFECTS） | `publication_id` REQUIRED | summary、optional `draft`、optional `latest_response`、`seller_facing_payload` | 不是發給呼叫者→NOT_FOUND |
| UpsertSellerResponseDraft | `publication_id` REQUIRED、`expected_draft_version`（空＝建立第一版）、`draft` REQUIRED（SellerQuote） | `draft`（`draft_version`、`update_time` OUTPUT_ONLY、SellerQuote） | 草稿已存在卻帶空、或版本不符→ABORTED／VERSION_CONFLICT（conflictTarget=draft）；賣家已婉拒→FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED；NOT_FOUND |
| SubmitSellerResponse | `publication_id`、`client_request_id`（UUID4）、`expected_publication_version`、`expected_seller_facing_payload_hash`（皆 REQUIRED）、`expected_seller_response_version`（空＝尚無正式回價）、`quote` REQUIRED | `seller_response`（SellerResponseVersion，含 `replayed`、`superseded_by_version`、`outside_buyer_requested_range`） | 詢價被改版→ABORTED（conflictTarget=publication）；expected 版本不是目前（含空卻已有）→ABORTED（conflictTarget=seller_response）；`spec_note`>1000 字、金額恆等式不成立→INVALID_ARGUMENT；停止寫入→FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED；NOT_FOUND |
| WithdrawSellerResponse | `publication_id`、`seller_response_id`、`client_request_id`、`expected_seller_response_version`（皆 REQUIRED）、`reason_code`（原樣記錄、不解析） | `closure`（`SellerResponseClosure`） | 契約只寫註解「撤回正式回價」，錯誤未列；依檔頭與 error_reason.proto:118-119 應有 RFQ_RECIPIENT_CLOSED（停止寫入）、ABORTED（版本不符） |
| DeclineRfqPublication | `publication_id`、`client_request_id`、`reason_code` | `closure` | 契約未列錯誤；NOT_FOUND |

### 1.1 舊實作逐項（OLD/commerce/seller_response.go；gRPC handler `backend/services/ai-service/internal/grpc/procurement_seller_response_handler.go`）

principal：`SellerPrincipal{StoreID|InviteID}`，InviteID 是站外業者（vendor portal，seller_response.go:73-95）。**整條 EXTERNAL lane（`evidenceFor`、`Verification`、`ErrVerificationRequired`、Vendor Portal 6 支）由 A9-D1 刪除，不搬**。

- **ListPublications**（:141-170，handler :74-101）：`limit` 非正或 >100 → 50（無分頁）；查 `ListProcurementRfqPublicationsForStore`（invite 以 `store_id`、join 核准取賣家可見 JSON、left join 回價列／草稿／最新版，依 `published_at DESC, id DESC`，不過濾 delivery_state）【量 `services/ai-service/db/queries/procurement/publication.sql:115-133`】；每列 `LatestResponseStatus` 用 `SellerResponseVersionStatus(latest, latest, withdrawn, validUntil, now)` 推導；壞掉的 payload JSON 視同沒有內容、不讓整份清單失敗（repository/publication.go:436-440）。
- **GetPublication／GetPublicationDetail**（:172-205、:253-314）：`FindPublicationInviteForStore(publicationID, storeID)`，找不到→`ErrInviteNotFound`→NOT_FOUND（"resource not found"，與猜 ID 同外形，handler :676-681）。詳情會 **ensureIdentity（讀取也寫入：建立回價身分列）**；讀草稿（解不開當沒有）、讀最新版與其推導狀態；賣家可見內容取不到時回零值、不讓整支失敗。
- **UpsertDraft**（:217-251）：驗可見性→`parseOptionalVersion`→JSON 原樣存；tx 內 ensureIdentity、`WriteClosed`→`ErrSellerResponseWriteClosed`；`UpsertProcurementSellerResponseDraft`：`INSERT … ON CONFLICT DO UPDATE … WHERE version = $3`，沒列回來→`ErrSellerResponseDraftStale`（ABORTED，reason STALE_SELLER_RESPONSE）。草稿不驗金額恆等式、不計正式一次、買家讀不到。
- **Submit**（:331-455）。順序：(1) 空 `ClientRequestID`→400；(2) 可見性→NOT_FOUND；(3) `checkPublicationCAS`（版本字串與 `seller_facing_payload_hash`，任一不符→`ErrStalePublication`，空字串略過檢查）；(4) `Normalize(now)`：responseType 二擇一、**品相必填**、`taxInclusionBasis` 二擇一、**currency 必為 TWD**、金額字串須非負整數（不接受小數與科學記號）、**total == goods+shipping+tax**、`unitBasis`／`offeredQuantity` 非空且無 e/E、單位非空、`leadTimeDays` 非負整數（可空）、`promisedDeliveryAt` RFC3339（可空）、**`validUntil` 必須在 now 之後**，算 `PayloadHash`；(5) `expectedVersion`；(6) 算 `outsideBuyerRequestedRange`（只標示不阻擋；單位／基準數量不同基準一律 false；經 publication→snapshot→price preview 讀區間，:669-711）；(7) tx：ensureIdentity；**⛔ 先查冪等重送（`(seller_response_id, client_request_id)`）：payload hash 不同→`ErrSellerResponseIdempotencyConflict`，相同→回放第一次的版本（Replayed）；排在 CAS 與 write-closed 之前，否則重試會被判成已提交**（:373-386）；(8) `WriteClosed`→RFQ_RECIPIENT_CLOSED；(9) expected 空但已有正式回價→`ErrSellerResponseAlreadySubmitted`（RFQ_RESPONSE_ALREADY_SUBMITTED）；expected 非空且 ≠ latest→`ErrSellerResponseStale`；(10) `next = latest+1` 插入版本列（寫入 hash、`PublicationVersion`、`SellerFacingPayloadHash` 複本）；撞冪等鍵（插入回傳既有列且 version≠next）→回放、**不得再 bump**；(11) `BumpSellerResponseLatestVersion`（列版本 CAS）。回應狀態用 `SellerResponseVersionStatus`。
- **Withdraw**（:457-540）：空冪等鍵→400；可見性；tx：ensureIdentity；路徑上 `sellerResponseId` 不是自己的列→`ErrSellerResponseNotFound`；**先查冪等（command 紀錄）**：內容不同→conflict、相同→由紀錄還原當時結果；`WriteClosed`→RFQ_RECIPIENT_CLOSED；已撤回的版本等於 expected→回既有（目標狀態冪等）；`latest ≠ expected` 或沒有正式回價→STALE；插入 command(WITHDRAW)、`WithdrawSellerResponse`（設 `withdrawn_version`）。reason_code 存在 command 列。
- **Decline**（:542-608）：先查冪等；已 `WriteClosed`（任何原因）→回既有列（不是錯誤）；插入 command(DECLINE)、`CloseSellerResponseWrite(DECLINED)`。reason_code 存在 command 列。已提交的回價在婉拒後保留，買家端 `QuoteAcceptability` 變 INELIGIBLE（seller_quote.go:225-243）。
- 錯誤對映（handler :670-713）：NOT_FOUND 一律 `RESOURCE_NOT_FOUND`；STALE_PUBLICATION／STALE_SELLER_RESPONSE／RFQ_RESPONSE_ALREADY_SUBMITTED／**RFQ_RECIPIENT_CLOSED 全部是 ABORTED**；QUOTE_AMOUNT_INCONSISTENT、QUOTE_QUALITY_REQUIRED 是 FAILED_PRECONDITION；欄位不合是 INVALID_ARGUMENT。

### 1.2 被 A9-D44／A9-D35／S5／S6 作廢，與新契約的對映

| 舊行為 | 狀態 | 新處置 |
|---|---|---|
| WISH_POOL 拉模式、ListPublications 只列 DIRECT（seller_response.go:141-142 註解） | A9-D44(1) 作廢 | 邀請列即可見性；3c-0 已刪 |
| 品相必填（`ErrQuoteQualityRequired`）、`offered_quality_*` | A9-D44(3) 作廢 | `spec_note` 選填、≤1000 字（schema.sql 版本表 CHECK）；QUOTE_QUALITY_REQUIRED 不再需要 |
| EXTERNAL lane、`Verification` 欄、vendor portal 6 支 | A9-D1 作廢 | 不搬；版本表已無 verification 欄 |
| `seller_response_commands`（Withdraw／Decline 的冪等與 reason_code 紀錄） | S4 刪表 | 目標狀態冪等；**reason_code 無處存**（見 Q4） |
| 版本表的 `payload_hash`、`seller_facing_payload_hash` 複本 | S5 刪欄 | 由 `rfqhash.SellerResponse`（A9-D36）重算；publication 的 hash 由 `rfq.compile/assemble` 重算 |
| 身分列的列版本（`seller_responses.version`）CAS | 新 schema 沒有該欄 | 以 `SELECT … FOR UPDATE` 鎖身分列取代（Accept 已用 `LockSellerResponseForBuyer`，quote.sql:23-28），Submit／Withdraw／Decline 必須鎖同一列才與 Accept 串行 |
| `write_closed_reason ∈ {PUBLICATION_CLOSED, OTHER_WON}` | S6 刪（沒有寫入者，舊程式 `CloseSellerResponseWrite` 只有 DECLINED 呼叫，【量】rg 全 ai-service 無其他呼叫） | 但 proto 註解仍寫「已結案或他人成交」（PROTO:160、error_reason.proto:118），見 Q3 |
| 回價時算 `outside_buyer_requested_range` 並存欄 | A9-D35(2)：買家端改現算 | 版本表欄位仍 NOT NULL，**無讀取者**（3b-3d 留的題，PLAN 3b-3d 列），見 Q6 |
| confirmed snapshot／price preview 取區間 | A9-D35(1) 刪表 | 區間在發布核准（`range_lower_amount`、`range_upper_amount`），`quote.OutsideRange` 已有 |
| RFQ_RESPONSE_ALREADY_SUBMITTED、STALE_PUBLICATION 兩個 reason | 新契約沒有（PROTO:156-160） | 合併成 ABORTED／VERSION_CONFLICT 的 conflictTarget=seller_response／publication |
| 金額恆等式不符 FAILED_PRECONDITION | 新契約 INVALID_ARGUMENT（PROTO:159） | 改欄位違規 |
| 列表無分頁（limit） | 新契約 keyset 分頁 | 新寫（見第 4 節） |

### 1.3 新模組已有什麼【量】

- 表：`seller_responses`（schema.sql:247，`(publication_id, store_id)` 複合 FK 到邀請列、UNIQUE、`latest_version`、`withdrawn_*`、`write_closed_*`、CHECK）、`seller_response_versions`（:271，`(seller_response_id, version)` 與 `(seller_response_id, client_request_id)` 唯一、各文字欄 CHECK＋`trimtext`、`total = goods+shipping+tax` CHECK、`currency` NOT NULL、`outside_buyer_requested_range` NOT NULL、`publication_version` NOT NULL）、`seller_response_drafts`（:316，JSONB payload＋`version`）、`rfq_publication_invites`（:229）。
- 查詢（`db/queries`）：`InsertSellerResponse :exec`（無 ON CONFLICT，rfq.sql:73）、`GetSellerResponse`（以 `(publication_id, store_id)`，rfq.sql:77）、買家端的 `ListCurrentSellerResponses`、`LockSellerResponseForBuyer`、`GetSellerResponseVersion`、`MarkInviteSent`／`MarkInviteFailed`（rfq.sql:65-71）。**沒有任何寫版本、草稿、撤回、婉拒的查詢，也沒有賣家面的 publication 讀取**。
- Go：`rfqhash.SellerResponse`（response.go:30，`SpecNote` 已在）、`quote.QuoteOf`／`PayloadHash`／`VersionStatus`／`Acceptability`／`OutsideRange`（quote.go:62-176）、`quote.idempotencyMismatch`、`quote.versionConflict`、`TargetSellerResponse`／`TargetPublication`。`rfq.compile`（compile.go:24）與 `assemble`（:45，純函式）組賣家可見內容；但 `compile` 的查詢帶 `BuyerID`（`GetRevision`、`ListApprovalRecipients`），賣家端不能用，要用 `assemble` 加不帶 buyer 的查詢。
- 缺口：Currency 常數 "TWD"（SellerQuote proto 沒有 currency 欄，版本表要）；數量需驗「正、≤3 位小數、NUMERIC(14,3) 範圍」；`SellerQuote` proto 的 `lead_time_days` 是非 optional int32（PROTO:901-917），0 與未填無法分（見 Q7）。

## 2. 經 legacy ordering 建單

### 2.1 舊 ai-service【量】

- 接受時：`commerce/buyer_acceptance.go`（BuyerAcceptance 在同一 tx 寫 acceptance 與 `procurement_order_provisioning_outbox` 列，payload `OrderShellRequestedPayload`，buyer_acceptance.go:89）。不上 MQ，內部補單。
- Worker（`OLD/orderprovisioning/worker.go`）：`pkg/outbox.Worker`，`maxAttempts=5`、`baseBackoff=1m`、`maxBackoff=1h`、`claimLease=3m`、`fetchBatch=50`（:27-45）。`process`（:200）：載入 acceptance（找不到→終態拒絕）；已 `ORDER_CREATED`／`ORDER_FAILED`→不做事；`buildCommand`（解析店 id：PLATFORM 用 acceptance.StoreID、EXTERNAL 用 `PROCUREMENT_SETTLEMENT_STORE_ID`；讀版本列與 confirmed snapshot；`ComposeRfqOrderLineName(canonical payload, offeredQuantity, offeredQuantityUnit)`＝"品名 數量 單位"，組不出來是終態、不得用 sentinel；`UnitPrice = goods+tax`、`TotalAmount = acceptedDealPrice`、`ShippingFee`、`Lane`）；呼叫 `CreateRfqOrder`；`classifyResponse`（:388）：code 0 成功、**500 可重試**、`FulfillmentCommerceModelUnknown`→UNKNOWN_LANE、`PlatformCollectionNotConfigured`→CONFIGURATION_MISSING、其餘終態 ORDER_SERVICE_REJECTED；成功但缺 orderNo／paymentDeadline→終態拒絕；`AttachOrderToAcceptance`（CAS，stale 視為已處理）。終態拒絕直接 `MarkAcceptanceOrderFailed(reason)`；暫時性錯誤回 error 由 outbox 重試，**用盡後 outbox 列轉 failed、`order_failure_reason=RETRY_EXHAUSTED`**（:60，hook `OnTerminalFailed`）。
- 冪等鍵：`AcceptanceID`（int64，TSID）→ Java `orders.acceptance_id BIGINT` 唯一索引 `uk_orders_acceptance_id`（`services/java/order-service/.../OrderEntity.java:141-147`、`V15__add_order_acceptance_id.sql:17-24`）；重送回既有 `orderNo` 加 `replayed:true`（`OrderGrpcService.java:423`）。呼叫是 internal-only（HMAC 四 header，:418）。
- Java 端解析：`parseLongRequired(request.getAcceptanceId())`（OrderGrpcService.java:439）【量】。

### 2.2 單體現況【量】

- `LEG/orderservice/buyerorders.go:39-196` 已有 `RfqOrderCommand`、`RfqOrderResult` 與 `Client.CreateRfqOrder`（`AcceptanceID int64`、`BuyerID`、`StoreID`、`TotalAmount`、`ShippingFee`、`UnitPrice`、`SkuName`、`Lane`；非 0 code 不轉 error 由呼叫端分辨；成功但無 orderNo 回 error；deadline RFC3339）。**不帶 `sku_id`**（Java 側 null 合法，OrderGrpcService.java:441-445；注意生成碼註解 `order.pb.go:1749-1750` 寫「worker 先向 ProductService 建好才投遞」，與 Java 實作與舊 worker 不符，是過期註解）。
- `quote.Accept` 已在同 tx 寫 acceptance 並 `Jobs.EnqueueJob(OrderProvisioningArgs{AcceptanceID uuid})`（accept.go:99；quote.go:46-51；kind `procurement.order_provisioning`）；`MarkOrderCreated`／`MarkOrderFailed`（rfq.sql:99-109，免 buyer_id、以 acceptance id 定位；`MarkOrderFailed` 同時轉 `state='ORDER_FAILED'` 解除 live 鎖）。procurement 依賴表已含 `ordering`（`internal/arch/modules.go:39`）；模組只依賴 port，adapter 在 `cmd/hawkr`（前例：`cmd/hawkr/buyer_ports.go:25`、`discovery.go:30`）。
- 平台沒有 River discard hook：`jobs.go` 只有 Enqueue／CancelJob；run 帳本用 reconciler 掃 Discarded／Cancelled job 標失敗（`MOD/internal/run/reconcile.go:55`），採購 run 的 job 用 `MaxAttempts:1`（run/submit.go:22-31）。

### 2.3 缺口

1. **冪等鍵型別不相容（阻塞）**：單體 acceptance id 是 UUIDv7，Java 與 `legacy/orderservice` 都要 int64。CARRYOVER 盤點 M1 寫「對 Java 只有 order_no、prospect_id、store_id、buyer_id 是外部鍵」（CARRYOVER.md:9），漏了 `acceptance_id`（`idrefs.csv:19` 之外的 Java 消費者，Java 程式碼證據如上）。見 Q1。
2. 沒有 worker：註冊 `procurement.order_provisioning` 的 `river.Worker`，只在測試啟用（不變量）。
3. 沒有 ordering port 與 adapter（`internal/legacy/orderservice` 的 `CreateRfqOrder` 有，但 procurement 沒有 port；`Deps` 沒有欄位）。
4. 品名 `SkuName` 的來源：舊用 confirmed snapshot 的 canonical payload；新模組沒有 snapshot（A9-D35(1)）。來源是 acceptance→publication→核准→需求版本（`ItemResolutionLabel`，空則 `Item`）加被接受版本的 `offered_quantity` 與單位。需要不帶 `buyer_id` 的讀取（BuyerDeleted 後 `buyer_id` 為 NULL，buyer_deleted.sql 與 CARRYOVER.md:119 同一個原因）。數量字串需去尾零（舊註解說 `NumericToCanonicalString`；`quote.decimal3` 是固定三位小數，不能直接當品名）。
5. Lane 不再存在：EXTERNAL 退場（A9-D1），`Lane` 固定 `"PLATFORM"`（Java 驗 lane，未知值 fail-closed）；`PROCUREMENT_SETTLEMENT_STORE_ID`、CONFIGURATION_MISSING、UNKNOWN_LANE 可刪（`order_failure_reason` 現在是無 CHECK 的 TEXT，rfq.sql:105-109、schema.sql:357）。
6. 錯誤分類：沿用「500 與 transport 錯誤重試、其他 code 終態」；Java 的 `UNAUTHORIZED` 映射（OrderGrpcService.java:466-）也應是終態還是重試要定（HMAC secret 設定錯是永久錯，屬「設定缺失」類）。
7. **S6 修訂的落點**：重試用盡時轉 ORDER_FAILED 並釋放 live 鎖。plan 3b-3d 列已寫「3c 的 worker 在 job 最終失敗（重試用盡被丟棄）時必須 `MarkOrderFailed`」（PLAN:280）。機制沒定（Q2）。
8. 買家在建單前被刪（`buyer_id` NULL）：`CreateRfqOrder` 需要 BuyerID，無法呼叫；沒有 SSOT（Q9）。
9. 驗收注入失敗的測試要在 integration（CI）；本機不跑容器（全域規則）。

## 3. `procurement.*` relay

### 3.1 舊 outbox 寫的事件【量】

`interface-inventory.md:112-134`、`OLD/publicationoutbox/worker.go:57-68`、`OLD/commerce/publication_execute.go:240-262,434-445`。MQ 只有 2 個 routing key，exchange `procurement.events`（`backend/pkg/mq/exchanges.go:20,88,90`）；order provisioning outbox 不上 MQ。**沒有消費的事件**（`interface-inventory.md:19`）。

| # | routing key | 何時寫 | payload（無 envelope，原樣 JSON，body 就是它） | 消費者 |
|---|---|---|---|---|
| E1 | `procurement.rfq_publication_invite.created` | `ExecuteRfqPublication` 同 tx，每個 PLATFORM invite 一列 outbox；worker 先發 MQ、後 `MarkInviteSent` | `{"inviteId","publicationId","storeId"}`，皆字串化 TSID（struct `inviteCreatedPayload`，publication_execute.go:434-438；golden `backend/tests/contract/golden/event_payload_keys.json:1209-1220` 的 `emitted_keys` 三個，`identity_spaces.storeId=store_id`） | notification-service 模板 `procurement.rfq_publication_invite.created.seller`，`target_user_field=storeId`、`reference_id_template=#{inviteId}`（去重靠它，每個 invite 唯一）、`data_fields` 帶 `inviteId`、`publicationId`（`services/notification-service/db/migrations/20260910120000_procurement_rfq_publication_invite_template.sql`；含「欄名大小寫不可互抄，抄錯靜默 0 投遞」的警告） |
| E2 | `procurement.rfq_publication_invite.external_created` | EXTERNAL invite | `{"inviteId","publicationId","prospectId","expiresAt"}` | **consumer 已刪**（`5c0292db2`），模板是 disabled 的 inert 標記（`20260911100000_…golden_marker.sql`；`a8-notification/inventory.md:105,295,340`）。EXTERNAL 隨 A9-D1 退場，**relay 不需要映射 E2** |

relay 要映射：每個 `RfqPublished.Invites[]` 一則 E1，`routing key` 與三個鍵名不變。

### 3.2 3b-3 已寫的模組事件

單體 procurement 唯一的 `events.Publish` 是 `model.RfqPublished`（`MOD/internal/rfq/publication_execute.go:105`；定義 `MOD/internal/model/rfq.go:79-85`）：`PublicationID, RequestID, ApprovalID uuid.UUID`、`BuyerID int64`、`PublishedAt`、`Invites []RfqInvite{InviteID uuid.UUID, StoreID int64}`；重送冪等不再發。沒有其他事件（`rg events.Publish MOD` 只此一處）【量】。接受報價沒有事件（舊服務也沒有）。

### 3.3 relay 的形狀與前例【量】

前例 `LEG/reviewrelay`、`imrelay`、`socialrelay`、`sellerrelay`：`OnX(q jobs.Enqueuer)` 在發布事件的 tx 內把事件轉成 River job（args 帶 `event_id`、`occurred`），worker 組 wire payload 再 `rabbitmq.PublishArgs` 排一個通用 publish job（`reviewrelay/relay.go:184-213`）。守門：`wire_test.go`（逐欄位釘 wire）、`golden_test.go`（對 `event_payload_keys.json`，`reviewrelay/golden_test.go`）、`rabbitmq/publish_routes_test.go`＋`testdata/routes.json`（掃描所有 `PublishArgs` 字面值，exchange 與 routing key 必須在 `infra/mq/topology/topology.yaml` 宣告；新增發送者要 `-update` 重生 routes.json）。`cmd/hawkr` 接線在 `cmd/hawkr/review.go:40-41`、`main.go:510-511`。procurement 的 relay 照 PLAN:43 只在測試啟用。

### 3.4 缺口

1. `procurementrelay` 套件（`LEG/procurementrelay`）不存在；`rfq_publication_invites` 的 `QUEUED→SENT／FAILED` 目前**沒有任何寫入者**（`MarkInviteSent/Failed` 只有查詢，無呼叫者；schema 註解寫「與 River job 同 transaction 建立」，但 `ExecutePublication` 沒有排 job，只有 `events.Publish`）。見 Q8。
2. ID 值形狀：舊 wire 是十進位 TSID 字串，新是 UUID 字串（鍵名、字串型別不變）。模板只當不透明字串用【推：沒有讀 notification 內部解析 `inviteId` 為數字的程式，只看了模板與契約測試】。見 Q10。
3. **golden fixture 目錄不存在**：`fixtures/` 目前只有 `assistant/`（【量】`ls 2026-11-19-a9-agent/fixtures`）。要建 `fixtures/events/`。舊 payload 是 `json.Marshal` 在 `ExecutePublication` 的 tx 內行內產生，struct 是未匯出的，**無法在本機以舊程式「相同輸入」呼叫產生而不起 DB（integration 在 CI）**，見 Q11。
4. `tests/contract/golden/event_payload_keys.json` 這兩條 publisher 仍是 `ai-service`；`TestGolden…` 類測試要求每條 golden 都有模板；3c 不改它（A8 才改）。

## 4. 賣家面讀 publication 的查詢（BuyerDeleted 安全）

前提：`rfq_publications.buyer_id`、`requests.buyer_id`、核准 `buyer_id` 在 BuyerDeleted 後是 NULL（buyer_deleted.sql:29-38）；已發布的需求與 revision、核准、發布、邀請、回價全部保留。現有 `GetPublication`、`GetRevision`、`ListApprovalRecipients` 都帶 `buyer_id`，賣家端不可用（CARRYOVER.md:119 已寫這點）。

需要的查詢（新增於 `db/queries/seller.sql`，名稱為建議）：
- `GetSellerPublication(publication_id, store_id)`：`rfq_publication_invites i JOIN rfq_publications p JOIN rfq_publication_approvals a JOIN request_revisions rev ON rev.id = p.request_revision_id`，`WHERE i.publication_id=@publication_id AND i.store_id=@store_id`；`LEFT JOIN seller_responses sr ON (publication_id, store_id)`、`seller_response_drafts d`、`seller_response_versions lv ON version = sr.latest_version`。無列→NOT_FOUND（含不存在、他店、買家已刪都同形狀，A9-D44(2)）。一次回齊 `assemble` 要的 `a` 與 `rev`，不經 `compile`（compile 載入受眾名單只為算 audience hash，賣家端不需要，且受眾名單不得給賣家）。
- `ListSellerPublications(store_id, cursor, page_size)`：同 join，`ORDER BY p.published_at DESC, p.id DESC` keyset（游標格式照 `quote/list.go:29-55` 的 `(created_at,id)` token）；一次 join 取核准與 revision，避免每列 N+1。
- `LockOrEnsureSellerResponse(publication_id, store_id)`：`INSERT … ON CONFLICT (publication_id, store_id) DO NOTHING` 後 `SELECT … FOR UPDATE`（現有 `InsertSellerResponse` 無 ON CONFLICT，併發兩個首次寫入會有一個撞唯一鍵）；FK 保證只有被邀請的店能建列。讀取方法**不要**像舊實作那樣建列（舊 GetPublicationDetail 會寫入，且 NO_SIDE_EFFECTS 契約不允許，PROTO:145-151）。
- 寫入：`UpsertDraft`（`ON CONFLICT … WHERE version=@expected`）、`InsertSellerResponseVersion`、`BumpLatestVersion`、`SetWithdrawn(expected)`、`CloseWrite(DECLINED)`。
- BuyerDeleted 後的寫入是否仍允許見 Q3。

賣家端組合只含白名單 `SellerFacingRfq`（來自 `assemble` 的 `model.SellerFacing`，含 `buyer_requested_range`，賣家本來就看得到區間，PROTO:758-777）；`publication_version` 現永遠是 `rfq_publications.version`（無重發）。

## 5. 發布時的封鎖檢查（A9-D29(5)）

現況【量】：
- 搜尋時：`candidate.BlockedStores`（`MOD/internal/candidate/candidate.go:46-49`）由 `procurement.go:284-300` 的 `blockingStores` 適配 `seller.StoresBlockingBuyer`（依 seller 批次上限分批，任一批失敗整個失敗），營業中另走 `openStores`＋`seller.ExposableStores`（procurement.go:56-59）。
- 預覽（`rfq.PreviewPublication`，publication_preview.go:41）：`resolveStores` 只把候選 ID 對到店家（:153-172），**不查封鎖、不查營業中**；`procurement.go:517` 的註解寫「發布時的封鎖檢查在 A9-3c」。
- 執行（publication_execute.go:32-117）：領核准→驗需求版本→`search.IsEffective`→`compile`→`InsertPublication`→`InsertInvite`，**也沒有封鎖與營業中檢查**。`rfq.Service` 沒有 `SellerStores` 欄位。
- 舊服務發布時沒有封鎖檢查（`rg -i block OLD/commerce/publication_*.go` 只命中註解），所以這是新行為，不是搬遷。

位置建議（細節見 Q5）：Execute 內（領核准之後、`InsertPublication` 之前）為權威；Preview 同檢查提早失敗。

## 6. SSOT 沒定的取捨（含「沿用也是決策」）

> 每題：問題、選項、建議（業界做法標【憶】，未查證）。

**Q1　Java `acceptance_id` 是 BIGINT，單體 acceptance id 是 UUIDv7。**
選項：(a) `buyer_quote_acceptances` 加 `order_ref BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE`，只當對 Java 的冪等鍵，隨 ordering 搬進單體（B8）刪欄；(b) 改 Java（migration 把 `orders.acceptance_id` 改 varchar／uuid、改 gRPC 解析），Java 是要退場的服務，且既有列是 BIGINT 要混存；(c) 把 UUID 壓成 63 位元整數（有碰撞，否）。
建議 (a)：identity 欄有前例（`buyer.buyer_id`、`identity.accounts`），值域與舊 TSID（~10^17）不會相交，並行期新舊服務不撞鍵【推：兩種生成方式的值域，未實測】；同時更正 CARRYOVER M1 那列（它漏了 acceptance_id）。

**Q2　建單 job 重試用盡時誰去 `MarkOrderFailed`（S6 修訂的機制）。**
`platform/jobs` 沒有 discard hook。選項：(a) worker 內判斷 `job.Attempt >= job.MaxAttempts` 時先 `MarkOrderFailed` 再回錯；(b) 只靠 reconciler 掃 `CREATING_ORDER` 且 job 已 Discarded 的列（run 帳本的做法）；(c) 兩者都有。
建議 (c)：(a) 涵蓋正常重試用盡與可分類的終態拒絕（直接 `river.JobCancel` ＋標失敗，不再空轉重試）；(a) 蓋不到 job 被 rescuer 丟棄或 worker 被殺（River 的 stuck job 被 rescue 時不一定走到最後一次 `Work`）【推】，需要 (b) 作保險，且重用 `run/reconcile.go` 的做法（業界：saga 加逾時掃描【憶】）。MaxAttempts 沿用舊值 5、退避由 River 管（舊 1m 起、封頂 1h，推論值）。

**Q2b　風險（只講一次）**：用盡轉失敗後買家可再接受；若 Java 其實已建單、只是回應逾時，同一 acceptance 的冪等鍵不再被重送，買家再接受會產生第二張訂單（孤兒單）。S6 已定，本盤點只標示；補救是營運對帳 `orders.acceptance_id` 與 `order_no` 為 NULL 的接受單，不在 3c 範圍。

**Q3　賣家寫入何時算「已停止」（`RFQ_RECIPIENT_CLOSED`）。**
契約註解（PROTO:160、error_reason.proto:118）說「賣家已婉拒、詢價已結案或他人成交」，但 S6 只留 `DECLINED`，沒有結案或他人成交的寫入者，也沒有「詢價結案」概念（`rfq_publications.state` 只有 PUBLISHED）。再加上：回價期限 `response_deadline` 過了呢？買家已刪（`buyer_id` NULL）呢？
選項：(a) 只擋 DECLINED，改註解成與實作一致；(b) 另外用推導（不存欄位）：需求已有 `ACTIVE` 接受單（他人成交）、發布的 `buyer_id IS NULL`（買家已刪，沒人能接受）時寫入回 RFQ_RECIPIENT_CLOSED，讀取照常；(c) 再加 `response_deadline` 過了就擋。
建議 (b)：成交後與買家已刪時再收新報價沒有任何讀取者，擋下是業界做法（Ariba 活動結案後投標關閉【憶】）；它不需要新欄位，符合 S6 的「刪沒有寫入者的狀態值」。`response_deadline` 暫不強制：舊服務只顯示不強制，買家設定的是「期限」，擋與不擋要產品決定，若要 (c) 請 Will 拍板。已提交的回價在 `ORDER_FAILED` 後買家能再接受別家，所以 (b) 的「他人成交」只認 `state='ACTIVE'`。

**Q4　Withdraw／Decline 的 `client_request_id` 與 `reason_code`。**
CARRYOVER.md:111 已寫「若 A9-3c 的請求加了內容欄位，要回頭補鍵」：兩支現在都有內容（expected_version、reason_code），而 S4 刪了 command 表，**reason_code 沒有儲存位置**，`client_request_id` 沒有存處也就無法回「同鍵內容不同→INVALID_ARGUMENT」（`procurement.proto` 檔頭契約）。
選項：(a) 兩支從 proto 刪 `client_request_id`（目標狀態方法；上線前可直接改契約），`reason_code` 加欄 `withdrawn_reason_code`、`write_closed_reason_code`（可空）；(b) 補回收據表；(c) 保留欄位但不比對。
建議 (a)：Withdraw 以「指定版本已撤回」天然冪等，Decline 以「已婉拒」天然冪等（AIP-135 風格的目標狀態；Stripe 的 idempotency key 只用在建立型呼叫【憶】）；(c) 是死欄位，違反「不留技術債」。reason_code 是否真有讀取者（買家端沒有顯示）要先確認；若無，**寧可不存、不收欄位**，但契約現在說「伺服器原樣記錄」，所以最小做法是存。需要 Will 決定是否改 proto（影響 TS／Dart 生成物與 website、seller-app 在 A9-5 的 client）。

**Q5　發布時封鎖與營業中檢查的行為。**
選項：(a) Execute 內檢查，任何一家被封鎖或不再營業→整筆失敗；(b) 靜默剔除被封鎖的店再發（受眾變了，核准的 `audience_hash` 不再成立，違反「買家同意的就是發出去的」，A9-D35）；(c) 只在 Preview 檢查；(d) Preview 與 Execute 都檢查。
建議 (d)＋失敗用現有 `ABORTED／VERSION_CONFLICT（candidate_set）`：不新增 ErrorReason，也不告訴買家是誰封鎖了他（搜尋時就是靜默過濾，同一個隱私立場）；client 的處理是重新搜尋（新候選集會濾掉他們）。讀取失敗一律失敗（fail-closed，同 A9-D29(5)）。實作上 `rfq.Service` 加 `BlockedStores` 與 `OpenStores` port，重用 `procurement.go:284` 的批次適配；在 tx 內讀 seller 是模組公開 API 的讀取，不違反規則 5（規則 5 管外部 provider）【推】。

**Q6　版本表兩個沒有讀取者的欄：`outside_buyer_requested_range`、`publication_version`（「沿用也是決策」）。**
3b-3d 已把買家端改成現算（A9-D35(2)），欄位仍 NOT NULL 且 SubmitSellerResponse 的回應要回 `outside_buyer_requested_range`（PROTO:1159-1175 的欄位 8）。`publication_version` 永遠等於 `rfq_publications.version`（沒有重發，state 只有 PUBLISHED），而 hash CAS 已在 `expected_seller_facing_payload_hash`。
建議：刪 `outside_buyer_requested_range` 欄（回應與買家端都現算，區間在核准，`quote.OutsideRange` 已有；存下來的值會與買家端現算漂移，且寫入當下與讀取當下的區間相同，沒有「存起來才對」的理由——necessity condition 不成立）；`publication_version` 欄與 `expected_publication_version` 請求欄**先留**（契約欄位；日後若有重發會用到），但要在 CARRYOVER 寫明目前沒有重發、版本檢查只是形式，且只有 hash 實際有效。

**Q7　`SellerQuote.lead_time_days` 與草稿的「未填」表示。**
proto 的 `int32 lead_time_days`（非 optional）無法區分「未填」與「0 天」；版本表欄位可空。草稿用 JSONB 存 SellerQuote：proto3 JSON 預設不輸出零值，部分填寫的 int64 金額也無法分辨未填與 0。
選項：(a) `lead_time_days` 改 `optional int32`，草稿用 `protojson`（`EmitUnpopulated:false`，讀回得零值即可預填表單）；(b) 0＝未填並寫進註解。
建議 (a)：天數 0（當天出貨）是合理報價；改 proto 的成本與 Q4 同批做。草稿另需大小上限與每個文字欄的長度檢查（draft 不驗商業規則，但要驗形狀與長度，否則 JSONB 無上限；`trimtext` 的規則 9 管 DB CHECK 欄位，JSONB 裡的字串不在其中，要在寫入端 `trimtext.Trim`）。

**Q8　邀請 `delivery_state`（QUEUED→SENT／FAILED）誰寫、語意是什麼（necessity condition）。**
舊語意：先發 MQ、再轉 SENT；重試用盡轉 failed。單體的 relay 前例是兩跳（relay job 排通用 `PublishArgs` job），SENT 就只能代表「已交給投遞管道」（正好是 proto 註解 PROTO:324-334 的字面意思），但 FAILED（重試耗盡）到不了，因為通用 publish job 的失敗與 invite 無關。A8 之後 relay 退場，notification 的 `rfq_invited` 意圖 job 接手，寫入者要再換。
選項：(a) 專用 job 直接呼叫 `rabbitmq.Sender`（舊語意：發成功→`MarkInviteSent`，重試用盡→`MarkInviteFailed`）；(b) 兩跳，SENT＝已排入 publish job，FAILED 不寫；(c) 刪 `delivery_state` 欄與 proto `InviteDeliveryState`。
建議 (a)：保留賣家看得到的投遞狀態語意（舊服務有），寫入者先是 relay、A8-3 改由 `rfq_invited` 意圖 job；(c) 是必要條件不成立時才選，目前賣家端的 summary 有此欄（PROTO:1142-1148），先不刪。要在 CARRYOVER 寫明寫入者交接點給 A8-3。

**Q9　建單時買家已被刪（acceptance `buyer_id` IS NULL）。**
`CreateRfqOrder` 必須帶買家，無法呼叫。選項：(a) 終態失敗（reason `BUYER_DELETED`）不呼叫 Java；(b) 無限重試；(c) 用 NULL 之外的備援買家。建議 (a)：刪帳流程已經檢查 `HasOpenOrders`（`legacy/orderservice.HasOpenOrders`），`CREATING_ORDER` 還不是訂單，是競爭窗口，屬 BuyerDeleted 與接受的競態；結果是 ORDER_FAILED 且沒有人能再接受，無害。

**Q10　relay payload 的 ID 值形狀（TSID→UUID）與「wire 形狀不變」。**
鍵名與字串型別不變，值是 UUID 字串；`storeId` 仍是 bigint 字串。選項：(a) 接受 UUID（模板只當不透明字串與去重鍵；A8 會整個改寫成 typed 型別，`notification/CARRYOVER.md:34,101`）；(b) 給舊形狀的數字 ID（需要新增數字 ID 欄，只為一個將被 A8 刪除的 relay）。建議 (a)；`reviewrelay` 的前例是 review_id 直接送 0（`reviewrelay/relay.go` 檔頭），我們不用這麼做，因為 `#{inviteId}` 的去重要求每個 invite 值唯一。需要 Will 確認「不變量：wire 形狀不變」的字面是否包含值的格式。

**Q11　golden fixture 的產生方式。**
驗收要求「從舊 ai-service 的 outbox 寫入程式以相同輸入產生」。舊程式不能動、`inviteCreatedPayload` 未匯出且在需要 DB 的 `ExecuteRfqPublication` tx 內組裝，本機不能跑 integration（全域規則）。
選項：(a) 在草稿工作區用一個單獨的 `go run` 小程式，**逐字複製**該 struct（引用 `publication_execute.go:434-438` 的行號與 commit），以固定輸入輸出 JSON，存 `fixtures/events/invite_created.json`；(b) 在 CI 由舊 integration 測試（`publication_execute_integration_test.go`）取回 outbox 列 payload 當 fixture，不能動舊服務，所以也要不改它就取得，做不到；(c) 手寫 JSON。
建議 (a) 並在 fixture 旁記錄來源，再加兩道對照：notification 模板的 `data_fields`／`target_user_field` 與 `event_payload_keys.json`（三鍵）。這是推論級的 provenance（不是舊程式真正跑出來的值），要在驗收報告講明。

**Q12　讀取方法的副作用與冪等。**
舊 GetPublicationDetail 建回價身分列；新契約標 NO_SIDE_EFFECTS。選項：讀取不建列（建議，本盤點第 4 節）；列由第一次寫入（草稿或正式）建立。此題不需 Will 決定，列為實作約束。

**Q13　`ListSellerPublications` 不過濾投遞失敗的邀請，也不過濾已成交或買家已刪的詢價。**
建議照舊（不過濾；賣家看得到自己收到過的詢價，狀態由 `latest_response_status`、`delivery_state` 表達），已成交與買家已刪的詢價，賣家仍可讀不可寫（Q3 (b)）。無需決定，列為預設。

「沿用也是決策」其他項（已答、不需決定）：
- 版本推導狀態（持久層沒有 status 欄）：成立（版本不可變）；沿用 `quote.VersionStatus`。
- 目前版本只靠 `withdrawn_version` 單值：版本 1 撤回、版本 2 送出後撤回，版本 1 會顯示成 SUPERSEDED 而不是 WITHDRAWN（只剩最後一次撤回的版本）。買家端只看目前版本，沒有讀取者，不處理，列在 CARRYOVER 即可【推】。
- 正式回價後保留草稿（舊行為，GetSellerPublication 同時回草稿與最新版）：沿用。
- 能力 `CAP_MARKETING`：`backend/docs/modules/seller.md:238` 的能力表沒有列「賣家採購回價」，文件要補一行（它列的是舊的行銷經營）。

## 7. 建議的子步驟切分

前置：Q1、Q4、Q7（可能要改 proto 與 schema）先由 Will 定；Q2、Q5、Q8 定了才能開 3c-2、3c-3。

| 子步驟 | 內容 | 驗收（本機：gofmt、`make lint-monolith`、`GOWORK=off go vet -tags integration ./...`、`python3 scripts/mutation/run.py --validate`；integration 與 mutation 在 CI） |
|---|---|---|
| **A9-3c-1　賣家面讀取與草稿**（Upsert／Get／List 共 3 支＋ `SellerProcurementAppService` 掛載與宣告表） | 第 4 節的查詢；`assemble` 用於賣家端；keyset 分頁；`UpsertSellerResponseDraft`（含長度與大小檢查、Q7）；`httpapi/sellerprocurement` 與 `declarations`（`CAP_MARKETING`，模組內 `seller.Authorize`）；若 Q4 選 (a) 先改 proto 與 `make connect`、TS／Dart 生成 | buf lint／breaking；sqlc、connect、uitext 無 drift；integration：他店與買家已刪（`buyer_id` NULL）的 publication 回 NOT_FOUND／可讀，未被邀請的店 NOT_FOUND；讀取不建列；草稿 CAS（空版本建立、已存在帶空→VERSION_CONFLICT draft）；mutation：把 `store_id` 條件拿掉 → 他店可讀的測試要失敗；`TestEveryServiceInTheContractIsMounted` 仍紅直到 3c-2 |
| **A9-3c-2　正式回價與撤回、婉拒** | Submit（鎖身分列、冪等回放先於 CAS 與 closed 檢查、`Normalize` 等價的驗證、currency TWD、hash 用 `rfqhash.SellerResponse`、Q3(b) 的 closed 推導、Q6 刪欄）、Withdraw、Decline；`TestEveryServiceInTheContractIsMounted` 轉綠 | integration：冪等重送回放（`replayed=true`、不再加版本）、同鍵內容不同 INVALID_ARGUMENT（`client_request_id`）、expected 版本不符與「空卻已有」→VERSION_CONFLICT seller_response、金額恆等式、`spec_note` 1001 字、婉拒後寫入 RFQ_RECIPIENT_CLOSED、與 `Accept` 的並發（兩個 goroutine 鎖同一列）；mutation：回放檢查移到 CAS 之後、hash 漏 `spec_note`、expected 比較 `<`→`<=` |
| **A9-3c-3　發布時封鎖檢查**（Q5） | `rfq.Service` 加 `BlockedStores`／`OpenStores` port；Preview 與 Execute 檢查；失敗為 candidate_set 衝突 | integration（假 seller 實作）：封鎖／歇業的店在 Preview 與 Execute 都失敗、讀取失敗 fail-closed、不洩漏是哪一家；mutation：Execute 略過檢查要失敗 |
| **A9-3c-4　建單 worker 與 ordering port**（Q1、Q2、Q9） | `order_ref` 欄與 migration；`OrderCreator` port（`cmd/hawkr` 的 adapter 包 `legacy/orderservice.Client`）；`procurement.order_provisioning` worker：組品名、呼叫、分類（500／transport 重試，其餘終態）、`MarkOrderCreated`；用盡與 reconciler 路徑 `MarkOrderFailed`；只在測試註冊 | unit：品名、分類表、`UnitPrice = goods+tax`；integration（假 `OrderCreator`）：成功、`replayed` 成功、終態拒絕、**注入失敗到用盡→acceptance `ORDER_FAILED`、live 鎖解除、同一需求可再接受（PLAN:209 的驗收）**、買家已刪；mutation：用盡時不標失敗、分類把 500 當終態 |
| **A9-3c-5　relay 與 fixture** | `LEG/procurementrelay`（`OnRfqPublished` 在 tx 內排 job；worker 發 E1；Q8 的 `MarkInviteSent／Failed`）；`fixtures/events/invite_created.json`（Q11）；`wire_test`、`golden_test`（對 `event_payload_keys.json`）；`routes.json` 以 `-update` 重生；不在 `cmd/hawkr` 接線（A9-6） | 測試內啟用 relay，產生的 payload 與 fixture 逐欄比對（鍵名、字串型別、無多餘鍵）；`publish_routes_test` 通過；mutation：改一個鍵名（`storeId`→`store_id`）比對要失敗；`python3 backend/scripts/check-migration-inventory.py` 不受影響（舊服務 3c 不刪） |
| **A9-3c-6　收尾** | CARRYOVER（M1 補 acceptance_id、M11、Q3／Q6／Q8 的結論、`withdrawn_version` 單值）、模組 AGENTS.md、PLAN 進度列與 D44(3) 更正、`backend/docs/modules/seller.md:238` 能力表、`backend/docs/modules/procurement.md` | 設計審查（design-review，大型遷移前後各一次）；`/code-review`；CI 全綠（逐 job） |

可並行：3c-1 與 3c-3；3c-4 與 3c-5 互不依賴，但都等 Q 決定。mutation 清單寫在 `mutation/a9-3c.json`（CI 跑，hawkr 規則）。

## 8. 消費端與驗證用 rg 指令

```bash
# 賣家 RPC 的消費者（client 隨 A9-5 才換，3c 不動）
rg -n "sellers/me/procurement|me/procurement/publications|response-draft|seller-responses" website seller-app/lib admin/src --glob '!node_modules' --glob '!.next' --glob '!**/gen/**' --glob '!**/*.g.dart'
rg -n "SellerProcurementAppService|ListSellerPublications|SubmitSellerResponse|WithdrawSellerResponse|DeclineRfqPublication|UpsertSellerResponseDraft|GetSellerPublication" api backend/internal packages website seller-app/lib --glob '!**/gen/**' --glob '!**/*.g.dart'
# 3c-0 殘留與 Q6 欄位
rg -n "WISH_POOL|WishPool|OfferedQuality|offered_quality|quality_grade|inspection_notes" api backend/internal packages
rg -n "outside_buyer_requested_range|OutsideBuyerRequestedRange|publication_version|PublicationVersion" backend/internal api packages
# 事件與 relay 的消費者
rg -n "procurement\.rfq_publication_invite|ExchangeProcurement|procurement\.events" backend --glob '!*_test.go' --glob '!**/docs/**'
rg -n "RfqPublished|OrderProvisioningArgs|procurement\.order_provisioning" backend/internal backend/cmd
rg -n "acceptance_id|acceptanceId|AcceptanceID" backend/internal/legacy backend/services/java/order-service/src/main
rg -n "MarkInviteSent|MarkInviteFailed|MarkOrderCreated|MarkOrderFailed" backend/internal
# 發布時封鎖
rg -n "StoresBlockingBuyer|ExposableStores|BlockedStores|StoreIDsBlockingBuyer" backend/internal
# 守門測試
rg -n "TestEveryServiceInTheContractIsMounted|TestPublishRoutesManifest|event_payload_keys" backend --glob '*_test.go'
```

## 9. 未驗證與限制

- 全部讀取靜態；沒有跑任何測試或指令以外的 rg／sed，沒有量 Java 實際行為（只讀了 Java 原始碼）。
- Q1 的值域不相交是推論（TSID 量級與 identity 欄起點，未實測 dev 資料）。
- Q2 對 River rescuer 行為的描述是推論，未查 River 版本文件。
- Q10 沒有讀 notification-service 的解析程式（只讀了模板、契約與 golden），「當作不透明字串」是推論。
- 舊 `ListPublications` 在 gateway REST 層是否有額外過濾沒看（只看了 ai-service 的 SQL 與 usecase）。
