# A9-3b-3 盤點：其餘買家採購方法（唯讀，2026-11-23）

縮寫：`W=.worktrees/a9-3b-run`、`OLD=W/backend/services/ai-service/internal`、`H=OLD/grpc/procurement_handler.go`、`SRH=OLD/grpc/procurement_seller_response_handler.go`、`M=W/backend/internal/modules/procurement`、`P=W/api/hawkr/procurement/v1alpha1/procurement.proto`、`IA=interface-inventory.md`。
標記：【量】讀過原始碼／跑過指令；【推】推論。

## 0. 數字與範圍

- `BuyerProcurementAppService` 在 P 只有 **23 支** rpc（`awk '/service BuyerProcurementAppService/,/^}/' P | grep -c rpc`），不是 36；36 是舊 `AIProcurementService`（IA §1.3，含賣家、vendor portal、SubmitTurnStream）【量】。
- 扣掉 3b-2f 的 6 支 → **3b-3 = 17 支**：GetSnapshot、GetCandidateSearch、ToggleSelection、SubmitAnswer、Rewind、PreviewRfqPrice、ConfirmRfqSnapshot、PreviewRfqPublication、ExecuteRfqPublication、ListConversations、GetConversationMessages、ListSellerResponses、PutShortlistEntry、DeleteShortlistEntry、AcceptSellerQuote、RenameConversation、DeleteConversation【量】。
- 3b-2f 的 handler 骨架 `W/backend/internal/httpapi/buyerprocurement/app.go`（worktree 新增未 commit）內嵌 `UnimplementedBuyerProcurementAppServiceHandler`，註解寫「其餘方法由 Unimplemented 回 unimplemented，掛載隨 A9-3b-3（A9-D13）」(app.go:4,37)【量】。
- `TestEveryServiceInTheContractIsMounted`（`W/backend/internal/httpapi/contract_test.go:49`）比對「全部」掛載清單 = api/ 宣告的全部 service；P 另有 `SellerProcurementAppService`（P:130），所以 3b-3 掛完 buyer 後此測試**仍紅**（seller 待 3c，另有 imageai 待 A9-4）。驗收應寫成「失敗訊息的缺少清單不再含 BuyerProcurementAppService」，而不是「轉綠」【量＋推】。
- 舊 handler 的 cross-cutting：全部先 `buyerIDFromContext`（H 各方法首行），ID 用 `parseInt64` 失敗回 INVALID_ARGUMENT【量】。新契約 ID 是 UUID（`M/db/schema.sql` 全表 `id UUID`），3b-2f 已有 `clientRequestID()` 解析（app.go:66）【量】。

## 1. 逐支表

欄位：舊實作與每個分支｜M 可沿用｜缺口｜跨模組｜client｜開放問題（→§3）

### 1.1 GetSnapshot
- 舊：H:426-475。檢查：requestId 解析→`snapshot.GetSnapshot(requestID,buyerID)`（不屬於回 not found）→revision `SlotsJSON` 反序列化失敗 Internal→`GetConversationIDForRequest`→`buildConversationRuntimeView`（H:495-551：列 conversation 的 runs，第一個非終態的 TURN/CLARIFICATION_RESUME 為 active；只有 active 是 WAITING 時查 open interrupt，crop 消歧 payload 轉成 question 與 crop candidates）→`FindLatestPublicationForRequest`（NotFound 視為無）【量】。
- 沿用：`M/internal/snapshot/snapshot.go`（`Reader.Get` 以 conversation 為鍵組 runs/interrupt/messages，3b-2e）；queries `GetRequestForBuyer`、`GetActiveRevision`、`GetConversationByRequest`、`ListRunsByRequest`、`GetOpenInterruptForRequest`、`GetLatestPublicationByRequest`（snapshot.go 已用）【量】。
- 缺口：以 request_id 為鍵的入口（Reader 現在只吃 conversationID）；`missing_required`、`item_disambiguation` 的組裝（slots/turn 已有 card 與 missing 計算，【推】可沿用 `turn/card.go`、`slots`）；`PublishedRfq.rfq_snapshot_hash` 要由 Go 重算（S5 刪 hash 欄，CARRYOVER:140）【量＋推】。
- 跨模組：無。client：website `useProcurementSession.ts`、`session-logic.ts`、`procurement-rfq-demo/load.ts`【量 rg】。
- 依賴 3b-2f/2e：是（共用 snapshot 讀取模型與 `RunSummary`/`OpenConfirmation` 轉換 convert.go）。

### 1.2 GetCandidateSearch
- 舊：H:617-642 → `candidateSearch.GetRun(runID, requestID, buyerID)`；回 set 狀態、兩 lane 狀態、failure_reason、lanes（H:553-576）【量】。
- 新契約：無 lane；`status` 是 RunStatus、`failure_reason` 是 RunFailureReason；Candidate 帶 StoreCard、matched/unverifiable facts 讀取時由需求現行版本組出（A9-D29(3)）【量 P、plan:139】。
- 沿用：`GetRunForBuyer`、`GetCandidateSetForBuyer`、`ListCandidates`（search/executor.go 用）；`internal/search`、`internal/candidate`（facts 組裝【推】在 candidate.go）【量】。
- 缺口：run 不是 CANDIDATE_SEARCH 類型或不屬此 request → NOT_FOUND 的判定；搜尋未完成時 candidate_set 尚無（run.candidate_set_id 為空）的回應形狀；StoreCard 組裝在 httpapi（`seller.Module.StoreCards` 存在，`modules/seller/public_integration_test.go:33`）【量＋推】。
- 跨模組：seller（StoreCards，httpapi 層組）。client：website `AgentTaskCard.tsx`、`useProcurementSession.ts`。
- 依賴 3b-2f：是（StartCandidateSearch 回的 run_id；RunStatus/FailureReason 轉換）。

### 1.3 ToggleSelection
- 舊：H:644-683＋`OLD/procurement/usecase/selection.go:63-109`。分支：candidateSetId/lane/candidateId 必填；`set.RequestRevisionID != req.ActiveRevisionID`→ABORTED `CANDIDATE_FACTS_STALE`；以候選自身 lane 為準，`in.Lane != cand.Lane`→INVALID_ARGUMENT `LANE_MISMATCH`；Upsert selection 表【量】。
- 新契約：無 lane；「候選集不是有效候選集（需求已不在 MATCHING，或有更新的搜尋成功）回 ABORTED，conflictTarget=candidate_set」（P ToggleSelection 註解；A9-D29(4)）【量】。
- 沿用：`SetCandidateSelected`（candidates.sql:20，已帶 request/buyer join，目標狀態）【量】。
- 缺口：**「有效候選集」查詢**（需求 MATCHING 且是最新一筆 SUCCEEDED 的 CANDIDATE_SEARCH run 的 candidate_set）不存在（`grep LatestSucceeded|EffectiveCandidateSet` 無命中）；需與 `SetCandidateSelected` 同 tx 並鎖需求列或 run 防止與重搜競態【量＋推】。
- client：website（`CandidateCanvas.tsx` 等）。依賴 3b-2f：否（只依賴 3b-2d 已完成的資料）。

### 1.4 SubmitAnswer
- 舊：H:685-760。同步驗證：clientRequestId、slotKey 必填；QUANTITY（非 skip）不收 optionId、freeText 或 unitOptionId 至少一；CROP_DISAMBIGUATION 必 optionId、`crop-other` 必 freeText、其餘禁 freeText、禁 unitOptionId；其他 slot：禁 unitOptionId，optionId/freeText/skip 恰一；expectedRevisionId 解析【量】。
  usecase `OLD/procurement/usecase/answer.go:440-498`：先查冪等鍵（重送跳過守衛）→`GuardNoOpenInterrupt`（有 PENDING 中斷→ABORTED `APPROVAL_PENDING`）→`ClaimRun`（TURN run，hash 比對）→重送 replay；新 run 用 dispatcher 在 goroutine 跑並**同步等**結果（:470-488）。錯誤：STALE_REVISION、ANSWER_SCHEMA_INVALID、SLOT_NOT_SKIPPABLE（H:762-776）。pipeline 經 narrator/realizer（LLM）【量】。
- 新契約：回 `SubmitAnswerResponse{TurnResult}`（同步）；有中斷時 **FAILED_PRECONDITION**＋APPROVAL_PENDING（舊為 ABORTED）；冪等鍵在 `turns (buyer_id, client_request_id)`（CARRYOVER:98）；`SLOT_KEY_ITEM_DISAMBIGUATION` 已由 ResumeRun 處理【量】。
- 沿用：`turn/compute.go` 的 `decide(prior, patch)`、`finish`、`nextStatus`、`rfqState`；`turn/executor.go:175 commit`（寫版本、輪次、狀態、標題、預覽）；`turns_source_check` 已收 'ANSWER'（schema:91）；`GetTurnByClientRequest`；`slots` 正規化【量】。
- 缺口：commit 現在綁 run（先鎖自己的 run）；SubmitAnswer 若不開 run 需抽出 run 無關的 commit；option_id→值的映射表（舊 `resolveAnswerValue`、`buildQuantityPatch` answer.go:287-325）新模組無對應；`SLOT_NOT_SKIPPABLE`、`ANSWER_SCHEMA_INVALID`、`STALE_REVISION` 不在新 ErrorReason（error_reason.proto 只有 VERSION_CONFLICT、APPROVAL_PENDING 等），需改映射為 INVALID_ARGUMENT 欄位違規／ABORTED VERSION_CONFLICT conflictTarget=request_revision【量＋推】。
- 跨模組：ITEM 自由文字需 resolver（Taxonomy，legacy category）。client：website `ClarifyQuestionCard.tsx`、`answer-validation.ts`、`useProcurementSession.ts`。
- 依賴 3b-2f：否（但依賴 3b-2c 的 decide/commit）。→ Q1。

### 1.5 Rewind
- 舊：H:778-814＋`OLD/procurement/usecase/rewind.go:53-110,226-252,311-370`。檢查：clientRequestId、slotKey 必填；冪等重送（request 不同→IDEMPOTENCY conflict）；`ListBranchRevisions`→`resolveRewindTarget`：找最後一次改變該 slot 的 revision 的前一版，從沒寫過→FAILED_PRECONDITION `ANSWER_SCHEMA_INVALID`；crop gate；UNIT 問題併到 QUANTITY；有中斷→APPROVAL_PENDING。**舊實作沒有 expected_revision 檢查**【量】。
- 新契約：`expected_revision_id` REQUIRED，不符 ABORTED VERSION_CONFLICT conflictTarget=request_revision（P RewindRequest:4）【量】。
- 沿用：`ListRevisions`（append-only revisions，S2）、`InsertRevision`、`SupersedeActiveRevision`、`LockActiveRevision`、turns source 'REWIND'【量】。
- 缺口：rewind 目標計算（逐版比較 slot 值）新模組沒有；倒回 ITEM/ITEM_RESOLUTION 時 MATCHING→退回、舊候選集失效（A9-D28(4)/D29(2) 的 `searchAffectingFields`）需套用；倒回後新版本是「複製舊版內容的新列」還是「重新啟用舊列」——S2 說只留 active 一種表示法【推】→ Q2。
- client：website `useProcurementSession.ts`、`errors.ts`。依賴 3b-2f：否。

### 1.6 PreviewRfqPrice
- 舊：H:816-863＋`OLD/procurement/pricing/price_preview.go:87-231`。分支：clientRequestId 必填；冪等命中且輸入 hash 不同→IDEMPOTENCY conflict；revision≠expected→ABORTED STALE_REVISION；range 驗證（TWD、上下界正整數、lower≤upper）→FAILED_PRECONDITION BUYER_REQUESTED_RANGE_REQUIRED；`referenceBasis`（GRAM→1000）；有單位與數量卻算不出→QUANTITY_UNIT_UNCONVERTIBLE；溢位檢查 `scaleByQuantity`；TTL 15 分【量】。
- 新契約：不合法 range → **INVALID_ARGUMENT**（P PreviewRfqPrice 註解）；無 version（A9-D21）；delivery_city 欄位仍在【量】。
- 沿用：`InsertPricePreview`（`ON CONFLICT DO NOTHING`）、`GetPricePreviewByClientRequest`、`GetLatestPricePreviewForRevision`；表有具名欄位 range/total（rfq.sql:1-7）。目前只有 integration 測試呼叫【量 grep】。
- 缺口：domain 計算（純函式，直接搬）；payload_hash 由 Go 重算（S5）；冪等「內容不同」比對改比欄位（沒有 input hash 欄）；delivery_city 存哪裡——表沒有此欄【量 rfq.sql:2-4】→ Q6。
- client：website `CandidateCanvas.tsx`、`RfqDraftFlow*`。依賴：無。

### 1.7 ConfirmRfqSnapshot
- 舊：H:893-947＋`OLD/procurement/commerce/rfq_confirmation.go:192-356`。分支：buyer_note 長度（MaxBuyerNoteLen）→INVALID_ARGUMENT；冪等命中但 request/preview/version 不同→conflict；revision≠expected→STALE；preview 不屬買家→NOT_FOUND；preview.request≠request→STALE；preview.revision≠active→STALE；過期→STALE（「報價預覽已過期」）；version/hash 不符→STALE【量】。
- 新契約：刪 price_preview_version（A9-D21）；全部 ABORTED VERSION_CONFLICT（conflictTarget 待定）；`expire_time` optional【量】。
- 沿用：`InsertConfirmedSnapshot`、`GetConfirmedSnapshot(ByClientRequest)`、`GetPricePreview`；`buyer_note` 具名欄（S3，trimmedColumns 登記【推】已在 3a）【量】。
- 缺口：canonical payload 的重算（舊存 `canonical_payload_json`，新表刪，CARRYOVER:140）；hash 演算法需定義且跨 Preview/Confirm/PublicationPreview/Accept 一致 → Q5。

### 1.8 PreviewRfqPublication
- 舊：H:949-1043＋`OLD/procurement/commerce/publication_preview.go:175-366`。分支：previewClientRequestId 必填；revision≠expected→STALE_REVISION；snapshot 不屬買家→NOT_FOUND；snapshot.request/revision 不符→APPROVAL_STALE；`compileAudience`：DIRECT 時 candidate set 不符／候選不在集合→INVALID_ARGUMENT、身分解析失敗→Internal；WISH_POOL 帶 policy id/version；deadline 格式；`CompileSellerFacingPayload`；insert approval（多個 hash 與快照複本）、TTL【量】。
- 沿用：`InsertApproval`、`InsertApprovalRecipient`、`ListApprovalRecipients`、`GetApprovalByClientRequest`（rfq.sql:26-）；recipients 表以 store_id 唯一【量】。
- 缺口：audience 編譯（DIRECT：候選需屬「有效候選集」，同 §1.3 查詢）；WISH_POOL 政策來源是 3c 開工題（plan:194）→ preview 時 recipient_count=0 可先做；seller-facing payload 組裝（SellerFacingRfq）；發布時查封鎖屬 3c（A9-D29(5)）【量＋推】。
- 跨模組：seller（DIRECT 收件者是否 OPEN／封鎖）【推】。

### 1.9 ExecuteRfqPublication
- 舊：H:1045-1116＋`OLD/procurement/commerce/publication_execute.go:165-446`。單一 tx：claim command（重送回原 publication）→gate：revision≠expected→STALE；`ClaimPublicationApprovalForExecute`（CAS PENDING→EXECUTING）失敗→APPROVAL_STALE；payload hash 不符→APPROVAL_STALE；approval.request 不符→APPROVAL_STALE；approval.revision≠active→STALE；過期→APPROVAL_STALE；verifyAudience（DIRECT 收件者空或 hash 不符→APPROVAL_STALE）→insert publication→invites＋**publication outbox**（INVITE_CREATED／EXTERNAL）→**produceResultArtifact**→Mark SUCCEEDED【量】。
- 沿用：`ClaimApprovalForExecute`、`MarkApprovalSucceeded`、`InsertPublication`、`GetPublicationByClientRequest`、`InsertInvite`、`ListInvites`；`agent_runs.run_type` 收 'PUBLICATION'、`publication_id` FK（schema:421,436）【量】。
- 缺口：outbox → `events.Publish`＋River（CARRYOVER:21），`procurement.*` relay 屬 3c（plan:191）；artifact 已刪（A9-D24(5)），GetRuntimeSnapshot 的「發布結果」改讀 `GetLatestPublicationByRequest`；是否建 PUBLICATION run 未定 → Q3、Q4。
- 跨模組：notification（經事件，3c relay）。client：website `RfqDraftFlow.tsx`、`rfq-publication-contract.ts`。

### 1.10 ListConversations
- 舊：H:1118-1141＋`usecase/conversation.go:97-140`：pageSize 預設 20、上限（maxConversationPageSize）、壞 cursor 靜默當第一頁、keyset (updated_at,id)、hasMore 用 +1【量】。
- 新契約：page_size 0→20、>50→50、**負數 INVALID_ARGUMENT**；next_page_token【量 P】。
- 沿用：`ListConversations`（requests.sql:76，只有 LIMIT，無 cursor）；索引 `conversations_buyer_active (buyer_id, updated_at DESC, id DESC)` 已就緒【量】。
- 缺口：keyset 條件的 query；壞 token 行為（舊靜默）→ Q7（小）。依賴：無。

### 1.11 GetConversationMessages
- 舊：H:1143-1168＋conversation.go:163-191：每個 answer_patch 拆 buyer＋assistant 兩則（id `<patch>-buyer/-assistant`）【量】。
- 沿用：`snapshot.messagesOf(turns)`（snapshot.go:118）、`ListTurnsByRequest`、`GetConversationForBuyer`——與 GetRuntimeSnapshot 的 messages 同源【量】。缺口：幾乎無，只需公開入口。依賴 3b-2e/2f：是（共用 convert）。

### 1.12 RenameConversation
- 舊：H:1226-1252：clientRequestId 必填；trim 後空、>40 rune、含 \n\r → FAILED_PRECONDITION `CONVERSATION_TITLE_INVALID`；not found→NOT_FOUND【量】。
- 沿用：`RenameConversation`（requests.sql:82，title_source='USER'，目標狀態不建收據 CARRYOVER:106）【量】。
- 缺口：上限不一致：舊 40 rune、DB CHECK 200（schema:72-85）、A9-D28(3) 自動標題 40 rune；新 ErrorReason 無 TITLE_INVALID → INVALID_ARGUMENT 欄位違規【量】→ Q8。重送已刪除對話＝0 列＝NOT_FOUND（合理）。

### 1.13 DeleteConversation
- 舊：H:1254-1274：found=false→NOT_FOUND【量】。
- 沿用：`SoftDeleteConversation`（`deleted_at IS NULL` 條件）【量】。
- 缺口：**重送冪等**：第二次 0 列，若照舊回 NOT_FOUND 就違反「重送回第一次結果」（CARRYOVER:107 宣稱目標狀態冪等）；需另查「存在但已刪」回 OK。刪除時進行中的 run 是否取消、已發布的詢價是否仍可在其他入口看到 → Q9。

### 1.14 ListSellerResponses
- 舊：SRH:390-419＋`commerce/buyer_acceptance.go:126-218`：列 request 的所有回價版本、shortlist 標記、`SellerResponseVersionStatus`（latest／withdrawn／valid_until）、`QuoteAcceptability`、publication_id 選填後過濾（記憶體）、`BatchGetSellerMeta` 失敗只 warn【量】。
- 新契約：publication_id **REQUIRED**、分頁（0→50、>100→100）、StoreCard、`outside_buyer_requested_range`【量 P】。
- 沿用：表 `seller_responses`、`seller_response_versions`、`buyer_shortlist_entries`；**沒有任何列表或 shortlist query**（rfq.sql 只有 `InsertSellerResponse`、`GetSellerResponse`）【量】。
- 缺口：列表 query＋分頁；status／acceptability／outside-range domain；資料寫入者（賣家回價）在 3c，3b-3 只能用直接寫表的測試資料【推】→ Q10。
- 跨模組：seller（StoreCards）。client：website `useBuyerQuoteComparison.ts`、`useRfqRuntimeSnapshot.ts`。

### 1.15 PutShortlistEntry ／ 1.16 DeleteShortlistEntry
- 舊：SRH:421-472＋buyer_acceptance.go:219-249：Put 先 `loadOwnedVersion`（response 不屬 buyer/request→NOT_FOUND，版本不存在→NOT_FOUND）；Delete 不檢查版本、只以 (request, response) 刪，回 present=false（冪等）；version ≤0→INVALID_ARGUMENT【量】。
- 沿用：表 `buyer_shortlist_entries`，唯一 `(request_id, seller_response_id)`（schema:360-372）—— 一則回價只會有一個版本在清單【量】。
- 缺口：put（upsert 版本）／delete／list query 全無；Put 換版本時是覆蓋還是拒絕【推】→ 併 Q10。

### 1.17 AcceptSellerQuote
- 舊：SRH:474-505＋buyer_acceptance.go:268-429。分支：clientRequestId、expected snapshot hash 必填；冪等命中比對 request/response/version/snapshot hash；tx：鎖 seller_response（FOR UPDATE），不屬→NOT_FOUND；latest≠expected→STALE；該版已撤回→STALE；write closed→RFQ_RECIPIENT_CLOSED；版本過 valid_until→RFQ_RESPONSE_EXPIRED；snapshot hash 不符→STALE_RFQ_SNAPSHOT；insert acceptance（live 唯一索引）＋**order provisioning outbox**；撞 live→`classifyLiveAcceptance`：PAYMENT_FINALIZED→RFQ_ALREADY_AWARDED，否則 RFQ_CHECKOUT_ALREADY_ACTIVE【量】。
- 沿用：`InsertAcceptance`（live 鎖只擋 ACTIVE，rfq.sql:103-105）、`GetAcceptanceByClientRequest`、`GetLiveAcceptance`、`MarkOrderCreated`／`MarkOrderFailed`（建單 worker 用）【量】。
- 缺口：建單 River job＋legacy ordering 呼叫屬 3c（plan:191「經 legacy ordering 建單」）；**RFQ_ALREADY_AWARDED 無資料來源**：S6 刪了 PAYMENT_FINALIZED（schema `state IN ('ACTIVE','ORDER_FAILED')`），判定需問 ordering 訂單已付款【量】→ Q11；`GetSellerResponse` 以 (publication, store) 查，缺「以 id 鎖列」query【量】。
- 跨模組：ordering（3c）。client：website `useBuyerQuoteComparison.ts`。

## 2. 共通缺口
1. 新 ErrorReason 沒有舊的 STALE_REVISION／APPROVAL_STALE／CANDIDATE_FACTS_STALE／SLOT_NOT_SKIPPABLE／ANSWER_SCHEMA_INVALID／CONVERSATION_TITLE_INVALID／BUYER_REQUESTED_RANGE_REQUIRED／QUANTITY_UNIT_UNCONVERTIBLE（error_reason.proto grep 只見 VERSION_CONFLICT=1、REQUIREMENT_NOT_READY=27、APPROVAL_PENDING=28、APPROVAL_ALREADY_RESOLVED=29、RFQ_*=31-34、PROCUREMENT_RUN_IN_PROGRESS=42）；需要一張「舊 reason → 新 code＋reason＋conflictTarget」對照表【量】。conflictTarget 值集（request_revision、candidate_set、seller_response、rfq_snapshot 已見於 P；price_preview、publication_approval 未見）→ Q5 一併。
2. RFQ 鏈（preview→confirm→approval→publication→accept）在 M 只有 queries 與 3a 的 table 測試，**沒有 domain／usecase 套件**（`grep -rl '\.InsertPricePreview('` 只命中 `procurement_integration_test.go`）【量】。
3. hash：S5 刪掉所有 hash 欄，payload_hash／audience_hash／seller_facing_payload_hash／rfq_snapshot_hash 都要由 Go 從欄位重算且穩定【量 CARRYOVER:140】。
4. StoreCard 組裝放 httpapi（`seller.Module.StoreCards`）——GetCandidateSearch、ListSellerResponses、可能 PreviewRfqPublication 共用【推】。
5. website 目前全走舊 REST（`website/app/api/v1/[...path]/route.ts`；`rg BuyerProcurementAppService website` 無命中）；client 切換是 A9-5，不在 3b-3【量 plan:207】。buyer-app 沒有採購（client-inventory §1 摘要）【量】。

## 3. 決定題（計畫／ADR／proto 沒寫、且有實際取捨）

- **Q1 SubmitAnswer 同步還是走 run**：(a) 同步、不開 run、不扣額度：沒有 LLM（A9-D24(4) 已刪 narrator/realizer），以 `decide`＋抽出的 commit 在一個 tx 內完成，守衛＝同需求無進行中 run＋無 PENDING 中斷＋expected revision；(b) 開 TURN 類 run 並同步等（舊做法，handler 卡住 worker）；(c) 改成像 SubmitTurn 回 run_id（要改 proto）。**建議 (a)**：契約回 TurnResult 同步，舊的 run 只是為 LLM 存在；TurnResult.run_id／quota 留空。ITEM 自由文字需 resolver 讀分類樹，放 tx 外。
- **Q2 Rewind 的新版本表示**：(a) 插入一筆複製目標版內容的新 revision（source=REWIND，append-only，歷史可稽核）；(b) 重新啟用舊 revision。**建議 (a)**，符合 S2「append-only、單一 active」；並定倒回 ITEM/ITEM_RESOLUTION 時 MATCHING→REQUIREMENT_READY／CLARIFYING、候選集失效（沿用 A9-D29(2) 的集合）。
- **Q3 ExecuteRfqPublication 是否建 PUBLICATION run**：schema 留了 `run_type='PUBLICATION'` 與 `agent_runs.publication_id`，但 A9-D24(5) 刪了 artifact。(a) 不建 run，發布結果由 `GetLatestPublicationByRequest` 讀（GetSnapshot.published_rfq、RuntimeSnapshot 已這樣做）；(b) 建一個立即 SUCCEEDED 的 run 讓對話時間軸有一筆。**建議 (a)**，並在 3b-3 刪 `PUBLICATION` run_type 與 `publication_id` 欄（無寫入者，S6 原則）——要 Will 確認是否還有 WatchRun 想看到發布事件。
- **Q4 發布的外送事件邊界**：3b-3 只在 tx 內 `events.Publish` 一個模組事件（如 `RfqPublished`，invite 列 QUEUED），relay 與通知在 3c；或 3b-3 完全不發事件、3c 再補。**建議前者**：事件形狀屬 procurement 本身，tx 原子性要在寫 publication 的同一處；golden 比對留 3c。
- **Q5 hash 與 VERSION_CONFLICT conflictTarget 規格**：重算用哪些欄位、演算法（SHA-256 over canonical proto/JSON）、price_preview 與 publication_approval 的 conflictTarget 名稱。**建議**：一個 `rfqhash` 小套件，以 proto `SellerFacingRfq`／range 的 deterministic marshal 算，conflictTarget 新增 `price_preview`、`publication_approval`，寫進 P 註解。
- **Q6 PreviewRfqPrice.delivery_city**：表沒有欄位、舊實作傳進 input 只參與 input hash。(a) 從 proto 刪（縣市已在需求 slots）；(b) 加欄位。**建議 (a)**。
- **Q7 壞 page_token**：舊靜默當第一頁；AIP-158 要 INVALID_ARGUMENT。**建議 INVALID_ARGUMENT**（小題，可直接定）。
- **Q8 對話標題上限**：舊 40 rune（rename）、自動標題 40、DB CHECK 200。**建議 40** 並把 CHECK 收到 40（或註明 200 是寬鬆上限），INVALID_ARGUMENT 欄位違規。
- **Q9 DeleteConversation 語意**：重送回 OK（需區分已刪與不存在）；刪除時進行中 run 是否一併取消；已發布的需求是否仍接受回價／可被接受。**建議**：重送 OK；同 tx 取消進行中 run（沿用 CancelRun 路徑）；已發布的詢價不受影響（賣家面不變，買家面入口消失），比照 S10 精神。
- **Q10 3b-3 與 3c 的分界（買家讀賣家回價）**：ListSellerResponses、Put/DeleteShortlist、AcceptSellerQuote 的資料寫入者（SubmitSellerResponse）與建單 job 都在 3c。(a) 3b-3 全做，測試直接寫表、AcceptSellerQuote 只入列 River job（job worker 與 legacy ordering 在 3c）；(b) 這 4 支搬到 3c，3b-3 先以 Unimplemented 掛載（掛載測試只看 service 是否掛，不看方法）。**建議 (a)**：plan 已把它們歸 3b-3，且 handler 掛載本來就要；但若要縮小 3b-3，(b) 不違反 A9-D13 的「不掛空殼」嗎？——A9-D13 禁空殼，(b) 部分方法 unimplemented 屬灰區，需 Will 定。
- **Q11 RFQ_ALREADY_AWARDED 的判定來源**：S6 刪了 PAYMENT_FINALIZED。(a) 撞 live 鎖時以 `order_no` 問 ordering 是否已付款；(b) 訂閱 ordering 的付款事件回寫 acceptance（加回狀態，抵觸 S6）；(c) 契約刪 RFQ_ALREADY_AWARDED，一律 CHECKOUT_ALREADY_ACTIVE（metadata 帶 orderNo，client 去訂單頁自然看到已付）。**建議 (c)**（或 (a) 若 ordering port 已存在）；需 Will 定，因要改 proto 與 error_reason.proto 註解。
- **Q12 ListSellerResponses 分頁鍵與 Shortlist 換版本**：鍵用 (submit_time DESC, id)；Put 不同版本時覆蓋舊版本（唯一索引只允許一筆）。**建議覆蓋**並回新版本。

## 4. 子步驟切分（每段約一個 session）

| 段 | 內容 | 驗收 | 依賴 |
|---|---|---|---|
| **3b-3a 讀取與對話管理（6 支）** | GetSnapshot、GetConversationMessages、ListConversations（keyset query）、RenameConversation、DeleteConversation、GetCandidateSearch；httpapi StoreCard 組裝；舊 reason→新 code 對照表落地 | integration：非本人一律 NOT_FOUND；ListConversations page_size 0/51/-1、token 翻頁不漏不重；Delete 重送 OK；Rename 上限；GetCandidateSearch QUEUED／FAILED／SUCCEEDED 三形狀；mutation：拿掉 keyset 條件、拿掉 buyer_id 條件，測試要失敗 | **依賴 3b-2f**（`App` 骨架、convert、`Module.GetRuntimeSnapshot`／snapshot Reader、StartCandidateSearch 的 run） |
| **3b-3b 釐清編修（3 支）** | ToggleSelection（有效候選集查詢）、SubmitAnswer（同步、抽出 run 無關 commit）、Rewind（目標計算、新版本） | 對照舊 `answer.go`/`rewind.go` 的分支逐項測（略過必填、選項不在表、從未寫過的欄位、expected revision 過時→VERSION_CONFLICT、有 PENDING 中斷→FAILED_PRECONDITION APPROVAL_PENDING、同需求有進行中 run→PROCUREMENT_RUN_IN_PROGRESS）；Toggle 在重搜成功後／退出 MATCHING 後回 ABORTED candidate_set；冪等重送與內容不同；mutation：拿掉有效候選集檢查、拿掉 expected revision 檢查 | 依賴 3b-2c/2d（已完成）；handler 檔依賴 3b-2f 的 App；Q1、Q2 先定 |
| **3b-3c 詢價發布鏈（4 支）** | rfq domain 套件（range→total、hash 重算、seller-facing payload、audience）；PreviewRfqPrice、ConfirmRfqSnapshot、PreviewRfqPublication、ExecuteRfqPublication（tx 內寫 publication、invites、模組事件） | 舊 `price_preview.go`/`rfq_confirmation.go`/`publication_*.go` 每個錯誤分支一測；hash 決定性（同輸入同 hash、改任一欄 hash 變）；Execute 重送 replayed=true 不重複建 invite；approval 過期／被執行過→VERSION_CONFLICT；mutation：拿掉 preview revision 檢查、拿掉 approval CAS | 不依賴 3b-2f 的 Module 方法（只依賴 App 骨架）；Q3–Q6 先定 |
| **3b-3d 回價比較與接受（4 支）＋掛載** | ListSellerResponses、Put/DeleteShortlistEntry、AcceptSellerQuote（入列建單 job，worker 留 3c）；`cmd/hawkr`／httpapi 掛 BuyerProcurementAppService | 直接寫表造回價：status／acceptability／分頁／publication 過濾；Accept 的過期、撤回、改版、snapshot 不符、live 鎖、冪等；`TestEveryServiceInTheContractIsMounted` 的缺少清單只剩 SellerProcurementAppService（與尚未完成的 imageai）；mutation：拿掉 live 鎖分類、拿掉 valid_until 檢查 | 依賴 3b-3a（StoreCard 組裝）；Q10、Q11、Q12 先定 |

掛載放最後一段：A9-D13「不掛空殼」，17 支都實作後才掛【推】。若 Q10 選 (b)，3b-3d 縮成只掛載，4 支移 3c。

**依賴 3b-2f 新公開 Module 方法的段**：3b-3a（GetRuntimeSnapshot／snapshot、StartCandidateSearch 的 run、convert 的 RunSummary/OpenConfirmation）；其餘段只依賴 3b-2f 的 `App` 骨架與 `caller`/`clientRequestID` helper（app.go:53,66），不依賴其 Module 方法【量＋推】。
