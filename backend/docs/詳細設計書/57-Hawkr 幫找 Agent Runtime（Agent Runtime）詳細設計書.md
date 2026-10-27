# 57 — Hawkr 幫找 Agent Runtime（Agent Runtime）詳細設計書

| 項目 | 內容 |
|---|---|
| 範圍 | Hawkr 幫找採購 Agent 的 runtime：命令、run ledger、SSE 契約、公開 DTO、持久化、配額、executor、前端 adapter |
| 不在範圍 | 採購的產品行為與 slot engine（見 55 號）、交易／付款／結算／發票／KYC／售後（見各自設計書） |

檢索用盡後的大品項／小品項選擇（LLM 使用條件、分類與行情 ID 邊界、多階段續答、snapshot 的 `openConfirmation.question` 還原契約）見 [ADR 2026-09-13](../adr/2026-09-13-procurement-category-fallback.md)。

---

## 0. 文件效力與來源

### 0.1 本文件的地位

本文件是 **Agent runtime 的 canonical 規範**。55 號仍是採購助手的產品與 domain 設計書；本文件只處理 runtime 層，兩者互補不重疊。

### 0.2 上游來源與取捨

| 來源 | 效力 |
|---|---|
| `docs/開發規範/*`、`api/proto/`、CodeMeta、MQ catalog、DB schema | **高於本文件**。本文件不重述其內容，只引用 |
| issue #2549（Hawkr Agent V6，P0） | **本文件的敘述層 SSOT**。§D／§E／§F／§G 全文有效 |
| 外部文件《Hawkr 幫找 Agent — LLM Agent 開發規格》（HK-PA-SPEC-007 v6.0 摘錄） | **參考來源，不可直接實作**：它提出的 SSE envelope 送到現行前端會驗證失敗、它指涉的多個既有物件錯位、且缺少敘述層。逐項差異見 [ADR 2026-08-22](../adr/2026-08-22-agent-runtime-v6-divergence.md) |
| ADR：AG-UI 採用（2026-08-17）、對照官方套件的四項調整（2026-08-21） | 有效，本文件承接 |

---

## 1. 全域不變式

違反以下任一條的設計一律不接受，不因工期例外。

### 1.1 商務邊界

1. Agent 只協助理解需求、澄清、搜尋、整理、準備詢價與呈現可核對結果。
   **本條是觸發條件，不是能力禁令**——寫成「不得聯絡供應商」會被誤讀成要下架已上線的詢價寄送。正確的判準是：**下列動作不得在無人確認下由 agent 觸發**——送出詢價／報價邀請、建立訂單、寫入付款狀態、任何對外通知（MQ、email）。經買家明確確認後由 runtime 執行，不受本條限制。
   這四類動作**永遠不得由模型提案**：它們不出現在模型可見的 toolset 裡，因此模型結構上無法選擇它們（機制見 §1.5）。⚠️ 圖內目前沒有 llmagent，這條因此更強——連可提案的 toolset 都不存在；**掛回真 tool 時它回到「靠 toolset 組成強制」那個形狀**，屆時 toolset 的組成就是這條不變式的唯一防線。
2. **模型文字不得取代價格、庫存、運費、資格、付款或履約事實。** 四種價格事實嚴格區分，任何 wire／DB／event／UI／分析不得混稱：

   | 名稱 | 定義 |
   |---|---|
   | `referencePriceRange` | Pricebook 的市場參考上下價 |
   | `buyerRequestedRange` | Buyer 調整後要投入許願池的希望詢價區間 |
   | 供應商報價 | 供應商提交的正式報價。**形狀不是純量**：v6.0 定義為含實際品相／等級、unit basis、`goods`／`shipping`／`tax`／`total`、`validUntil` 的 **append-only `sellerResponseVersion`**，只能新增不能覆寫 |
   | `acceptedDealPrice` | Buyer 接受特定 `sellerResponseVersion`、且收到 server authoritative ACK 後才成立 |

   ⛔ **平台算出來的價不是任何一種價格事實。** DB 的 `seller_unit_price`／`buyer_unit_price`／`seller_total_price`／`buyer_total_price` 是平台在 Pricebook 公斤價上疊 markup（PLATFORM 0.06／EXTERNAL 0.12）算出來的，**四者皆不是**——不得被當成供應商報價或成交價，也不得遷移成它們。

   > **欄位形狀由商務線 #2538 定義，不在本文件範圍**（§10 已聲明本文件只涵蓋 runtime 層）。本文件只凍結「四種事實嚴格區分」這條不變式與上面那條 ⛔。

3. PLATFORM 與 EXTERNAL 是兩條獨立 candidate lane；候選預設未選取。配對理由只能使用可驗證的 category、region、keywords、公開資料來源與 freshness；缺乏證據時顯示「尚未確認」，**不得由模型補寫**。

   **identity-first 適用於兩層 entity，順序不可對調：**

   | 層 | 何時釘死 | 現況 |
   |---|---|---|
   | **品項** | 收集其餘條件**之前** | ✅ 已做——CropGate 三態（Matched／Ambiguous／Unavailable ＋「其他」逃生艙）；未釘死不得進 Pricebook 查價 |
   | **供應商** | 候選回來時 | ❌ 未做。每個候選只能攜帶**能被證明屬於它自己**的事實；跨 lane 身分空間不同（PLATFORM `store_id` vs EXTERNAL `candidate_leads.id`），不得互相冒充 |

   執行機制是 evidence contract（§1.6）。
4. 每一位 seller 的回覆彼此獨立。任一 seller 回覆不得把其他 recipient 標成 LOST。
5. Shortlist 建立零筆 order、零筆 payment。
6. Event ledger 是 browser replay 與 public audit projection，**不是** order／payment／RFQ 或財務 ledger。

### 1.2 身分

1. `account_id` 是登入 principal；`buyer_id` 是業務 domain ID。**procurement 所有業務表的擁有者欄位一律 `buyer_id`**（`db/schema/procurement.sql` 檔頭約定）。
2. **Agent 配額的計量軸也是 `buyer_id`**（stint-scoped），不是 account——換 stint 視為全新買家、額度歸零。落地要求見 §7.2。
3. `runId` 不得同時代表 `candidateSetId`、`requestId` 或 Temporal Workflow ID。

   `run_id` 是 `procurement_candidate_sets` 上與 `id` 各自獨立的一欄，建立時兩者各配一次 TSID。
   對外 `runId` 一律回 `run_id`（`wireRunID`），`candidateSetId` 另有自己的欄位。
   🔴 **`GetCandidateSearch` 的 `runId` 是 echo-back**：原樣回傳呼叫端在路徑上送進來的值，
   **不得**從查到的列重新推導——推導會讓 client 手上的值與 server 回值不匹配，而輪詢端的
   freshness guard 會把不匹配解讀成「還沒更新」，症狀是靜默卡死而不是錯誤。
4. 所有 ID（`eventId`／`seqId`／`messageId`／`artifactId`／`interruptId`／`conversationId`／`runId`／TSID）對外 JSON **一律 string**。

### 1.3 隱私

Data-class 矩陣：

| 資料 | Browser／history | Public event／artifact | Log／trace／Sentry | Server worker |
|---|---|---|---|---|
| Buyer 自己的輸入 | 允許（需 ownership ＋ `private, no-store`） | 不複製，只放 messageRef／safe summary | 禁止 | 依 ID 讀取 |
| Public narrative／activity | 允許 | 允許，typed／bounded | 只放 enum／count | 允許 |
| Candidate 公司名、分類、區域 | 允許 | 允許 | 僅低基數 count | 允許 |
| Candidate email／phone／social／聯絡人 | **禁止** | **禁止** | **禁止** | 依 candidate ID 解析 |
| Raw reasoning、system prompt、raw tool args／result | **禁止** | **禁止** | **禁止** | 不持久化成產品契約 |

補充三條，避免被讀成「已經乾淨」：

- **不得改名包裝**：把 raw Thought 換個欄位名送出，與直接送出同罪（#2549 P0 決策 1）。
  > **這條的主詞是 raw Thought，不是「任何模型產生的對外文字」。** 由獨立 prompt 產生、受 §3 約束的短意圖句不在本條射程內——那是 Public Activity 表面的內容，不是被改名的推理原文。判準：模型的**自由推理輸出**永遠不得上 wire；模型的**受約束意圖陳述**依 §3 的規則處理。
- **原始工具輸出目前仍在 network payload 上**：`search_candidate_sellers` 的 `candidates` 由 ai-service 整包 marshal、gateway 以 `TOOL_CALL_RESULT.content` 原樣透傳。唯一補償控制是 tool output 的欄位挑選層。收斂方式見 §9.4。
- **保留欄位的值仍可能是聯絡管道**：`public_website` 於 CRM1 來源多為社群網址；`research_summary`／`positioning`／`product_summary` 是自由文字。

**所有 authenticated 回應與 SSE 一律 `Cache-Control: private, no-store`**（判準與例外見 `docs/開發規範/API 設計規範.md`「回應快取政策」）。

### 1.4 Ownership matrix

| 邊界 | 擁有的真相 | 不得擁有 |
|---|---|---|
| Web | draft、optimistic mapping、canonical snapshot／event fold、UI focus | domain transaction、contact、raw reasoning、第二套 conversation runtime |
| Gateway | session 驗證、buyer actor projection、proxy、trace propagation、SSE flush | business status、信任 browser 自報的 `X-Buyer-Id` |
| ai-service public API | command claim、run／event／artifact projection、ownership、RFQ publication approval | order／payment truth、未經 allowlist 的 domain model 序列化 |
| PostgreSQL `hawkr_ai` | request／revision／candidate／quote 與 run ledger | 財務總帳 |
| Durable executor | lease、fencing、checkpoint、bounded retry | 覆寫 domain CAS、把 in-memory channel 當 queue SSOT |
| Notification worker | 依 recipient ID server-side 解析聯絡方式與發送 | 把 contact 放進 browser、event 或 trace |

### 1.5 不可繞過的 guard

控制流交給模型之後，「哪些東西模型碰不到」必須明講——在舊架構它是隱含的（`PlanDialogue` 是唯一入口），現在不是。

**下表每一項都必須是模型無 handle 的 Go 函式**（ADK graph 的 `FunctionNode`，或 `before_tool` callback），模型不能選擇跳過、也不能用參數改變它的判準：

| guard | 擋什麼 | 繞過的後果 |
|---|---|---|
| `Slots.Apply` 欄位驗證 | unit／city 不在白名單、數量預算超範圍 | 未經驗證的值進入 Pricebook 查價與候選搜尋 |
| CropGate 分類硬擋 | 品項不落在 Pricebook 分類 | 拿不存在的品項去查行情，或用舊品項行情對新品項報價 |
| price preview 的 pricing mode 判定 | `PRICEBOOK_GUIDED` 下 mapping 不可靠時的 typed rejection | 自動 fallback 成編造的數字 |
| tool registry 白名單 | 未知 tool 名 → 回錯誤給模型，**tool 從不執行** | 模型自行擴充可用動作集 |
| 高後果動作的授權層 | 送出詢價／報價邀請、建立訂單、寫入付款狀態、任何對外通知 | 見 §1.1-1；這四類**不出現在模型可見的 toolset 裡** |

**執行機制**：高後果動作以 `Mode: ModeSingleTurn` ＋ `DisallowTransferToParent/Peers=true` 從 toolset 移除，`transfer_to_agent` 因此不會被注入 request；未知 tool 名由 registry 在執行前回 `{"error": ...}` 給模型。**這是唯一在執行期強制的白名單機制**，不要改用 sub-agent transfer。

### 1.6 Evidence contract

每個候選必須攜帶「這筆資訊憑什麼可信」的結構化紀錄。這是 §1.1-3 供應商 identity 的執行機制。

| 欄位 | 語意 | 約束 |
|---|---|---|
| `matchedFacts` | **驗證過符合**的條件（配送區域、分類…） | 保證非 null；只能放能被證明屬於**該候選自己**的事實 |
| `unverifiableFacts` | **無法驗證**的維度 ＋ **原因** | 保證非 null；**必須列舉原因，不得只是空陣列** |
| `factsAsOf` | 這些事實的**資料來源時點** | ⛔ **不是查詢時間** |

**`factsAsOf` 的語意**：查詢時間不得冒充資料時間。EXTERNAL 半邊填 `seller_prospects` 該列的事實截止時間（`ProspectVendor.FactsAsOf`），不是 `time.Now()`——填查詢當下等於宣稱這批離線名單是今天更新的。PLATFORM 半邊查的是 product-service 的即時資料，來源時點與查詢時點同一個，填 `now` 即為正解。

**記錄層閘門**：`domain.ValidateCandidateEvidence` 在候選被寫進 candidate set 之前對三欄做 accept/reject，落點是兩條 lane 唯一的匯流點 `usecase.acceptEvidence`（新增 lane 不必各自再掛一次）。現行規則刻意只擋「結構上不可能被正確渲染」的形狀——`matchedFacts`／`unverifiableFacts` 為 JSON `null`、`factsAsOf` 為零值；**不擋空陣列**（下段的常設揭露由建構面保證 EXTERNAL 非空，PLATFORM 半邊要不要也常設一條來源揭露屬未定的產品決策，2026-09-06 裁決先寬鬆）。擋下的是單一候選不是整條 lane：一筆壞候選換掉其餘合法候選的可見性，代價與問題不成比例。

**不採用業界的 citation 模型**：Bedrock contextual grounding、Azure groundedness、Vertex checkGrounding、Anthropic Citations 全部是「拿自由文字 source material 算 entailment 或標 span-level citation」——**我們沒有可引用的文件**（候選來自自家商品 DB 與一次性離線匯入名單）。可移植的是記錄層閘門：在候選被呈現**之前**對結構化記錄做 accept/reject，而上面三欄已經是那個形狀。

**EXTERNAL lane 不做事前驗證**（2026-08-26 決策）：站外賣家只寄詢價信，不回覆即自我篩除，**email 往返本身就是篩選器**。代價是買家清單看起來像經過篩選而其實沒有——所以誠實揭露是這個策略成立的**前提**：EXTERNAL 候選必須攜帶一條常設的 `unverifiableFacts`（「外部名單，供應能力未經平台驗證，以賣家回覆為準」）＋ 正確的 `factsAsOf`。落地於 `domain.ExternalLaneDisclosure()`，key 為 `external_unverified`，一律排在逐維度的「需向賣家確認 X」之前——它講的是整張卡的來源性質，不是某個維度的缺口。文案改動要同步改這裡。

🔴 **`EvidenceRef` 的形狀未凍結，任何消費端都不得依賴它**——在形狀定案並接上 producer 之前，不得有程式碼讀它。EXTERNAL lane 唯一可能的來源 `public_website` 值多為社群網址（§1.3 已標為已知缺口）。

---

## 2. Wire 契約（本文件最重要的一節）

### 2.1 唯一不變式：全系統只有一套 browser wire，就是 AG-UI

POST 串流與未來的 GET replay **送同一種 frame**。不存在第二套 SSE envelope。

**這不是風格偏好，是實測結果。** 對 `@ag-ui/core@0.0.58` 實跑 `EventSchemas.safeParse`：

| Frame | 結果 |
|---|---|
| v6 §3.3 的 `{schemaVersion, seqId, type:"ACTIVITY_DELTA"}` | **FAIL**（`invalid_type: Required`） |
| v6 §4.2 完整 envelope | **FAIL** |
| 現行 `CUSTOM step_detail` | PASS |
| `CUSTOM` ＋ ledger 附加鍵（本文件採用） | **PASS**，附加鍵原樣存活於 `parsed.data` |

website `lib/ai-chat/stream-client.ts` 在分派前跑 `EventSchemas.safeParse`，**驗不過就走 `onError`**——採用 v6 §3.3 的 envelope 等於讓整條 replay 變成一連串使用者可見錯誤。

### 2.2 三層分離

| 層 | 值域 | 說明 |
|---|---|---|
| **Ledger event type**（DB 欄位） | §4.2 的 19 個值，進 CodeMeta | replay 與稽核的 SSOT；比 AG-UI 豐富 |
| **Wire carrier**（SSE frame） | AG-UI 原生型別 ＋ `CUSTOM name=<slug>` | 前端唯一認得的東西 |
| **Projector** | ledger row → AG-UI frame | 兩者之間唯一的轉換點，單一實作 |

AG-UI 沒有對應原生型別的 ledger event（`RUN_WAITING`、`ARTIFACT_UPSERTED`、`CONFIRMATION_REQUIRED`、lane progress…）一律走 `CUSTOM name="<slug>"`，與現行 `step_detail`／`draft_patch` 同一手法。**已實測**：`@assistant-ui/react-ag-ui@0.0.56` 的 adapter 把 `CUSTOM` 映射成 canonical data part（原始碼註解：「CUSTOM events become canonical data parts」）。

### 2.3 明確禁用的 AG-UI 型別

| 型別 | 為何不用 |
|---|---|
| `STEP_STARTED`／`STEP_FINISHED` 作為進度載體 | adapter 的 26 個 case **完全不含這兩個**，會落到 RAW 後被 RunAggregator 丟棄。可以送（無害），但不可依賴它顯示任何東西 |
| `THINKING_*`／`REASONING_*` | **維持禁用。** §1.3 禁止的資料類別；且 AG-UI 對 Reasoning 的定義是「meant to be **sent back to the agent**」，與 Activity 的 frontend-only 相反——用它會把使用者可見文字變成模型 context 的一部分 |

> **Activity 的 wire carrier 是 `CUSTOM name="activity"`，不是原生 `ACTIVITY_*`**（值域見 §4.2）。`SubmitTurnStream`（POST turn stream）與 GET replay 兩條路徑對 Activity 送出**逐字相同**的 frame，唯一允許的差異是 POST 路徑不寫 `id:` 行（那是 replay-only 的 ledger-seq cursor，見 §2.4）。
>
> ⚠️ 原生 `ACTIVITY_SNAPSHOT`／`ACTIVITY_DELTA` 本身沒有被禁用，只是不作為本系統的載體；若日後引入 assistant-ui 並改採原生型別，§4.2 的 carrier 定義要一起改。

### 2.4 SSE 傳輸層

- **無 `event:` 行。** `writeAGUIFrame` 只寫 `data: <json>\n\n`。backend golden fixture 與 website `agui-fixtures.unit.test.ts` 皆已斷言 `not.toMatch(/^event:/m)`。
- **`id:` 行等於 ledger `seq`**，只在 GET replay 端點出現。Heartbeat 用 SSE comment，不占 seq。
- Gateway 必須轉送 `Last-Event-ID`、保留 SSE profile，並在每次成功 write 後 flush。

#### 2.4.1 `threadId` 與 `runId` 的值域：每個端點家族取自己最穩定的識別碼

AG-UI 的 `RunStartedEvent`／`RunFinishedEvent` 把 `threadId` 與 `runId` 列為**必填字串**（`@ag-ui/core` 的 `z.ZodString`，不是 `ZodOptional`——同一個 schema 裡的 `parentRunId` 才是選填）。缺了 `threadId`，整個 frame 會在消費端的 zod 驗證被拒，症狀是「那一種事件從此收不到」而不是「少一個欄位」。

協定**只規定這兩個 key 必須存在且是字串，不規定裡面放什麼**。本系統三個端點家族各放不同的值，這是刻意的，不是漂移：

| 端點家族 | `threadId` | `runId` |
|---|---|---|
| 12 支 `AIAssistantService` SSE（賣家助手、seller-match…） | `req.SessionID` | gin middleware 的 `trace_id` |
| procurement `POST /turns/stream` | `req.RequestID` | 起始為 `trace_id`，收到 `RunAccepted` 後換成真正的 ledger `run_id` |
| procurement `GET /runs/{runId}/events`（replay） | run 的 `conversation_id`（`RunEvent.conversation_id`） | ledger `run_id` |

**不要把它們統一**，理由是各路徑在「必須送出這個欄位的那一刻」握有的資訊不同：

- POST turn stream 的 RUN_STARTED 是延後到收到 `RunAccepted` 才送的（`deferRunStarted`），而**第一輪** turn 正是對話還沒建立的那一輪——`conversationIDForRequest` 查不到時回 `0, nil`（不建立）。改用 conversationId 會讓助手開場送 `threadId: ""`，等於在吃 100% 買家流量的路徑上把一個有值的欄位換成空字串。
- replay 反過來：run 早已存在、對話幾乎必然已建立，而 run row 存的是 `conversation_id` 與 `domain_ref`（候選搜尋的 domainRef 是 `candidateSetId`），要拿 requestId 得另外 join。
- 沒有任何消費端跨 transport 對帳這兩個欄位（website 完全不讀 `threadId`，只要求它存在且是字串）。

**不變式**：原生 `RUN_STARTED`／`RUN_FINISHED` frame 必須帶 `threadId`；值可以是空字串（`z.ZodString` 接受，語意是「這個 run 沒有掛在任何 thread 上」），但 **key 不得省略**。CUSTOM frame **不得**帶 `threadId`——那個 schema 沒有宣告這個欄位，多送一個 key 會被 zod 的 strict 模式拒絕，製造出第二種「整個 frame 被丟掉」的失敗。

### 2.5 Replay 演算法（normative）

1. 驗證 buyer ownership。random ID 與他人的 ID 回**相同的 404 外形**。
2. `Last-Event-ID` 以十進位字串解析為 64-bit safe cursor，**不得經 JavaScript Number**。
3. 查詢 `seq > cursor` 並取得 high-water mark。
4. **先註冊 live wake channel（PostgreSQL `LISTEN` 或等價機制）再重查一次**，封住 query-to-subscribe race。
5. Wake 只表示「可能有新資料」；每次仍從 DB ledger 取下一批。in-memory channel、ring buffer 與 Gateway buffer **都不是** replay SSOT。
6. Cursor 早於 retention boundary → HTTP 410 ＋ `REPLAY_CURSOR_EXPIRED` ＋ snapshot reference；cursor 高於 high-water → HTTP 409。
7. Slow consumer 或 channel overflow 只中斷該 stream，不阻塞 worker。
8. Browser disconnect、route change 或關頁**只 abort subscription，不得改 run status**。

> 既有可援先例：gateway `internal/sse` 已有 ring buffer ＋ `Since(lastEventID)` ＋ `bufferMiss` 降級；website `lib/sse/client.ts` 已完整實作 `Last-Event-ID` header 帶入與字串 cursor。

---

## 3. 公開輸出的三個分類

raw reasoning 永久不恢復。使用者看得到的「AI 在想什麼／做什麼」只由這些表面承接，**不可互相冒充**：

| 表面 | 文字由誰產生 | 時機 | 可保存 | 約束 |
|---|---|---|---|---|
| **Public Activity**（步驟列） | Backend projector（deterministic）**或**模型的受約束短意圖句 | 進行中 | 否 | 見 §3.1。`safeArgs` 僅限核准欄位名、bounded count、公開 artifact／status |
| **Public Reply**（進行中） | 模型（`SurfaceRealizer`，於 terminal 一次產生） | 整輪結束後一次送出（見 §11 的串流現況） | 是 | 純文字；**不承載事實**（見 §3.2） |
| **結構化 blocks** | 模型（受 DialoguePlan 約束）／validator reject 時 backend deterministic | 隨時 | 是 | 候選卡、報價單、消歧選項等；`PublicAgentMessageV1.blocks[]`，值域見 §3.3，validator 與 `RenderMode` 見 §3.3.1 |


### 3.1 Activity 表面接受模型短句的四條約束

Activity 列上的短句**只重述 guard 已經驗過的結構化結果**：`extract_slots` 的 ShortIntent 過 `guardShortIntent`，其餘步驟由 `narrateStepOutcome` 拿伺服器算好的摘要生成、失敗退回 `StepLabels`。權威不來自「誰決定下一步」，來自「句子只能轉述已發生的事」（見 §3.2）。四條約束缺一不可：

1. **發送時機綁在 guard 之後。** 短句只在對應的 tool call **通過 deterministic guard**（`before_tool` callback／node `ValidateOutput`）之後才送出。
   > 這是與外部參考實作唯一的差別，也是讓這件事安全的那一刀：失效模式不會因為短句改由模型產生而消失，只是換了位置——模型講完意圖後 guard 仍可能擋掉那個 tool call（未知 tool 名、`Slots.Apply` reject、CropGate 硬擋），此時使用者已看到「翻找可配送至【台北市】的【香蕉】賣家」而動作沒有發生。綁在 guard 之後就結構性不可能。
2. **長度上限。** 短意圖句，不是敘述段落。
3. **不承載事實。** 不得出現數字＋單位、店家名稱、價格、庫存、資格。事實只在結構化 blocks。
4. **缺席或違規一律確定性退回。** 模型沒吐、超長、含事實或值域外 → 退回既有的 `StepLabels` 伺服器固定字串表。該表因此從終態降為 **fallback 層**，不得刪除。

> ⚠️ **驗證不得放在 `after_model` callback。** ADK Go 的 `runAfterModelCallbacks` 在 streaming loop 裡呼叫且**沒有 `Partial` 守衛**，`StreamingModeSSE` 下會在每個 token chunk 觸發，無法檢查完整斷言。放 node 的 `ValidateOutput` 或 `before_tool`。

### 3.2 Activity 列的權威從哪裡來

那一列是對「系統此刻正在做什麼」的**事實宣稱**，講錯等於宣稱了未發生的副作用。

**權威來自輸入，不來自控制流**：每一句短句的輸入都是 guard 之後、由伺服器算好的結構化摘要（`summarizeValidateSlots`／`summarizeCropGate`／`summarizeNextQuestion` 等），模型只能換句話說；空句、超長、含事實值域外內容一律退回 `StepLabels` 固定字串（§3.1-4）。所以無論控制流由 deterministic planner 還是模型決定，這一列都不會宣稱未發生的動作——**本節不依賴控制流由誰決定**。

🔴 **這條放寬成立的唯一條件是 §3.1 四條約束全部在執行期強制。** 業界主流（OpenAI Agents SDK、Vercel AI SDK、LangGraph、LangChain 等）的進度面一律是 runtime 產生的 typed state，AG-UI 協定本身也把唯一那個人類可讀短句欄位 `THINKING_START.title` 廢棄且沒有補回替代事件，Activity payload 刻意是 JSON object 而非字串——**拿掉任何一條約束，這個表面就退化成無人驗證的模型散文**。

### 3.3 Block kind 值域

以 #2549 §F 為準：`UNDERSTOOD | CONTEXT | RATIONALE | WARNING | NEXT_STEP | COMPLETE`。

> ⚠️ 本值域**取代** website `narrative.ts` 現行的 `UNDERSTOOD|SEARCHED|ORGANIZED|WARNING|COMPLETE|SUMMARY`（只有三個重疊）。同一次交付要改 `narrative.ts` 的 union 與 `NARRATIVE_ICON`——該表是封閉的，未知 kind 不渲染 icon，漏改是靜默掉圖示。

#### 3.3.1 三層 Validator 與 `RenderMode` 值域

`surface_realizer.go`（三個 provider 共用的 ADK 骨架）產生候選 blocks，`surface_pipeline.go` 的 `realizeOrFallback` 是 `/turns`（graph terminal node）、`/answers`、`/rewinds` 三個 compose 點共用的唯一進入點，交給 `surface_validator.go` 的 `validateSurfaceBlocks` 做三層驗證：

1. **Claim code 語意驗證**：每個 block 必須自報至少一個 claim code，且落在 `plan.AllowedClaimCodes` 內、不在 `plan.ForbiddenClaimCodes` 內。
2. **危險片語黑名單**（`forbiddenPhrases`）：完成貌用語（搜尋、找到候選／賣家、送出、發布、通知賣家、成交、下單、扣款等）的多種口語講法。每個語意詞同時列「已X」與「已經X」（含「已幫你X」「已經幫你X」），因為純字面 `strings.Contains` 不會自動涵蓋另一種講法；刻意不列裸字（「送出」「找到」），否則「尚未送出」「還沒找到」這些**合法且必要的否定敘述**會一起被誤傷。
3. **Fact-echo**（`domain/fact_tokens.go`）：數字逐 token 相等；城市與單位以封閉值域比對，城市另摺疊「臺／台」異體字（`cityAliases` 把兩種寫法映成兩個 canonical 值，不摺會讓臺北／臺中／臺南／臺東四個都會系統性誤擋）。模型文字裡的事實值必須落在**當輪 committed 的 `domain.Slots` ∪ `SafeRenderArgs` 四欄**內——取 Slots 是因為 `SafeRenderArgs.SalientFactLabels` 只有「這一輪」recorded 的最多 2 筆（`/rewinds` 恆為空），而 prompt 會把最近 2 則歷史回覆整段餵給模型，只比對 `SafeRenderArgs` 會把任何承接前文的自然句誤判成竄改。被更正過的舊值（quantity 50→80）不在 committed Slots 內，因此仍然會被擋——這是刻意不改用 `RecentNarratives` 當來源的理由。

任一層 reject、`Realize` 回傳 error，或超過 `RealizeTimeout`（4s），一律整包退回 `domain.RenderPlan(plan, args)` 的原樣結果。

`RenderMode` 值域只有兩個合法值：`"MODEL"`（realize 成功且通過三層驗證）、`"TEMPLATE"`（任一步失敗或退場）。欄位路徑：`turnPayload.RenderMode` → `TurnResult.RenderMode` → `aipb.DialoguePlanSummary.render_mode` → gateway wire `dialoguePlan.renderMode`。`PolicyVersion` 已升至 `"phase3-v1"`；`TemplateVersion`（`TemplateRegistryVersion`）維持 `"phase2-v1"`——版本號只綁定 registry 的結構契約（Phase 2 把 `RenderFunc` 回傳型別從 `string` 換成 `[]Block`），不綁文案；文案修正不觸發升版。

### 3.4 逐欄位抽取進度

`EXTRACT_SLOTS` 是單一次 LLM 呼叫，佔一輪 wall-clock 的 **82%**（實測 5.6s；live sampling 修正為 7.3–9.5s）。呼叫本身進行中依然零事件，但這段窗口的頭尾現在各掛了一筆 Activity 快照（見下），不再是全程死寂。

**entry／completion 兩階段快照（normative）**：`extract_slots`／`validate_slots`／`crop_gate` 三個 node 各自在本體一開頭先 emit 一筆 entry Activity 快照（`text` 為空字串），做完真正的工作（LLM 抽取／`Slots.Apply`／crop 分類）之後再 emit 一筆 completion 快照——兩筆**共用同一個 `rowId`／`retryGroupKey`**（identity 模型見 §4.4），消費端依 `rowId` 合併成「先出現、後補內容」的同一列。⚠️ **呼叫進行中（entry 送出後、completion 送出前）沒有任何細粒度更新**，它不是下段逐欄位進度的替代品。completion 的 `text` 只有 `extract_slots` 是模型短句；`validate_slots`／`crop_gate` 帶 `StepLabels` 固定短句，這兩步的模型敘述（`StepNarrator`，見 §4.3／`step_narrator.go`）在節點內只算摘要，留到 turn 尾與 `UNDERSTAND_INPUT`／下一題那兩筆同一個平行群組生成，只寫進持久化的 `steps[]`，不回填 ledger 的 Activity 事件——每輪因此少兩次序列 LLM 呼叫，代價是**進行中的 Activity rail**（與重讀同一個 run 事件流時）這兩列只有固定短句；`GET .../messages` 的對話 replay 讀的是持久化 payload，模型敘述完整保留。刻意取捨，不以第二筆事件補發（`AGENT_ACTIVITY_DELTA` 見 spec §0 的排除理由）。敘述失敗、逾時或不符合 guard（空白、超長）時退回固定字串，紀律同 §3.1-4。

逐欄位進度仍然需要，形狀依 §4.4 的 identity 模型：**一個中性 code ＋ 每列自己的 row id ＋ `safeArgs{fieldName}`**，不是 8 個 `CAPTURE_*` 字面碼（那是舊 by-code 收斂逼出來的產物）。文案必須是**中性動詞**：`onField` 發生在 `Slots.Apply()` 之前，而 Apply 會 reject（unit／city 不在白名單、數量預算超範圍），pre-commit 的 fact claim 會出現在該欄位其實被丟棄的那一輪——這與 §3.1-1「綁在 guard 之後」是同一條規則的兩個面。

⚠️ **entry／completion 與逐欄位進度是互補不是取代**：前者一個 node 兩筆事件，後者每個欄位一筆。實作逐欄位進度時必須處理 `hasRejected` 輪的矛盾（只清 degraded 輪不夠），否則矛盾會被寫進終態 `steps[]` 與 `payload_json`。

---

## 4. Public DTO、Event 與 Artifact

### 4.1 DTO boundary

Public transport 必須使用具名 allowlist DTO，**不得**直接序列化 domain model、ADK event、tool result、DB row 或無界 `map[string]any`。

| DTO | 欄位 SSOT | 明確禁止 |
|---|---|---|
| `PublicAgentMessageV1` | ✅ **#2549 §F**（14 個欄位群） | reasoning、system prompt、raw model response |
| `PublicAgentConfirmationV1` | ✅ 本文件 §4.6（最小欄位集） | free-form side-effect payload、模型生成的核准 |
| `PublicActivityV1` | ✅ **#2549 §G** | query text、SQL、prompt、unbounded error text |
| `PublicAgentRunV1` | ✅ 本文件 §4.1.1 | prompt、reasoning、lease owner、fencing token、private error |
| `PublicAgentEventV1` | ✅ 本文件 §4.1.2 | raw、Thought、raw args／result、contact |
| `PublicAgentArtifactV1` | ✅ 本文件 §4.1.3 | generic unconstrained data、private contact |
| `PublicCandidateV1` | ✅ 本文件 §4.1.4 | email、phone、social、聯絡人、BD notes |

> 下列四張欄位表的來源優先序：**已上線的既有 wire 型別**（`PublicCandidateV1` 直接對齊 gateway `ProcurementCandidateItemDTO`）＞**本文件其他章節已凍結的值域**（§4.2 ledger event、§4.3 狀態、§4.5 artifact type）＞外部參考文件（必須先按 [ADR 2026-08-22](../adr/2026-08-22-agent-runtime-v6-divergence.md) 校正，不可直接照抄）。對應 Go 型別定義：`services/ai-service/internal/procurement/domain/artifact.go`。

#### 4.1.1 `PublicAgentRunV1`

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `runId` | string | Y | Run 身分（§1.2-3：不得等於 `candidateSetId`／`requestId`／Temporal Workflow ID） |
| `runType` | string | Y | 例如 `TURN`（預設對話輪）、`CANDIDATE_SEARCH`（§5.3）。開放值域，client 需 default 分支 |
| `conversationId` | string | Y | 所屬對話（§5.6 runtime-snapshot 的 thread 重建鍵） |
| `status` | string | Y | `ProcurementRunStatus` 值域（CodeMeta，9 值） |
| `failureReason` | string | N | `ProcurementRunFailureReason` 值域；非 `FAILED`／`EXPIRED` 時為空字串，不含自由文字錯誤訊息 |
| `resumedIntoRunId` | string | N | 僅 `status=RESUMED` 時出現，指向接續的 child run（同 Temporal `CONTINUED_AS_NEW`） |
| `lastSeqId` | string | Y | 本 run 目前已知最新 ledger seq（§5.6 replay 邊界） |
| `minAvailableSeqId` | string | Y | 本 run 目前可 replay 的最小 seq；cursor 早於此值回 410 `REPLAY_CURSOR_EXPIRED`（§2.5-6） |
| `createdAt` | string（ISO 8601 UTC） | Y | Run 建立時間 |
| `updatedAt` | string（ISO 8601 UTC） | Y | 最近一次狀態轉移時間 |

明確禁止：`prompt`、`reasoning`、`lease_owner`、`fencing_token`、除 `failureReason` 機器碼外的 private error message。

#### 4.1.2 `PublicAgentEventV1`

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `eventId` | string | Y | 事件身分（TSID） |
| `runId` | string | Y | 所屬 run |
| `seq` | string | Y | run 內單調遞增序號（§4.2 不變式 1）；GET replay 的 `id:` 行即此值（§2.4） |
| `type` | string | Y | `ProcurementLedgerEventType` 值域（CodeMeta，19 值，§4.2） |
| `occurredAt` | string（ISO 8601 UTC） | Y | **事件發生時間，不是 DB audit time**——兩者不得互相冒充 |
| `data` | object | N | 依 `type` 而定形狀，只含該事件型別的 allowlisted 欄位（例如 `STEP_*` 帶 `Step`；`ARTIFACT_UPSERTED` 帶 `artifactId`／`type`／`version` 參照，不內嵌完整 `PublicAgentArtifactV1`） |

明確禁止：RAW、`REASONING_*`／Thought、raw tool args／result、system prompt（同 §4.2 不變式 5）、candidate 聯絡方式。

#### 4.1.3 `PublicAgentArtifactV1`

對應本文件 §4.5 的 Artifact envelope、`services/ai-service/internal/procurement/domain/artifact.go` 的 `Artifact` struct。

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `artifactId` | string | Y | Artifact 身分（TSID） |
| `type` | string | Y | `ProcurementArtifactType` 值域（CodeMeta，13 值，§4.5） |
| `version` | int | Y | 本 `artifactId` 的第幾次產製（instance revision，對齊 `UNIQUE (artifact_id, version)`），不是 data schema 版本 |
| `checksum` | string | Y | `data` 內容雜湊；供商務線 CAS（`expectedRfqSnapshotHash`）與前端變更偵測使用 |
| `status` | string | Y | `ProcurementArtifactStatus` 值域（CodeMeta，5 值：`DRAFT`／`READY`／`STALE`／`CONSUMED`／`FAILED`） |
| `asOf` | string（ISO 8601 UTC） | Y | 資料事實時間（freshness）；依據事實已更新時 `status` 應轉 `STALE` |
| `domainRef` | string | N | 指向產製來源的 allowlisted domain 識別值（例如 `candidateSetId`），供 owner restore 使用 |
| `data` | object | Y | 依 `type` 而定形狀。10 型（`REQUEST_SNAPSHOT`／`CLARIFICATION`／`CANDIDATE_RESULTS`／`ERROR_RECOVERY`）已可從既有 domain 結構推導；其餘 9 型（§4.5 note 列出）**尚未展開**，projector 對未展開／unknown schema 必須 fail-closed，不得臆測渲染 |

明確禁止：generic unconstrained `data`（Track A／B 不得共用 schema，§4.5）、private contact。

#### 4.1.4 `PublicCandidateV1`

= 現行 `ProcurementCandidateItemDTO`（`gateway/internal/handler/procurement_response_mapping.go`）凍結版本，讓 `CANDIDATE_RESULTS` artifact 與既有 `GET .../candidate-searches/{runId}` 端點共用同一份契約，不另開第二套候選形狀。

| 欄位 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `candidateId` | string | Y | 候選身分 |
| `sourceId` | string | Y | 來源列身分（`PLATFORM`＝product-service SKU／`EXTERNAL`＝`candidate_leads` row） |
| `lane` | string | Y | `PLATFORM`／`EXTERNAL` |
| `displayName` | string | Y | 顯示名稱 |
| `score` | number | Y | 排序分數 |
| `matchedFacts` | array | Y | 可驗證配對事實（§1.1-3），保證非 null |
| `unverifiableFacts` | array | Y | 無法驗證的需求維度＋原因，保證非 null |
| `factsAsOf` | string（ISO 8601 UTC） | Y | 事實的**資料來源時點**（⛔ 不是查詢時間，語意見 §1.6） |
| `selected` | boolean | Y | 買家是否已勾選 |
| `storeId` | string | N（`EXTERNAL` lane 整個省略） | 商家 store-stint surrogate；**不是授權判準**（§1.3、issue #2377） |
| `sellerName` | string | N（同上） | |
| `storeCode` | string | N（同上） | 賣家頁 canonical 短碼；空字串時消費端不得退回 `storeId` 組連結 |
| `logoKey` | string | N（同上） | 商家 logo objectKey（SSOT） |
| `logoUrl` | string | N（同上） | 由 Gateway 依 `logoKey` 組出（49 §6.6.1 flat-pair 慣例，Gateway 是唯一 URL 組裝點） |

明確禁止：`email`、`phone`、`social`、聯絡人、BD notes（`priorityLetter`／`priorityStars` 屬內部評分，同樣不對外）、商品層圖片與標價（§1.1-2、issue #2377 裁決）。⚠️ `publicWebsite` 類欄位刻意不在此 DTO 中——那是 `CandidateLead`（離線名單來源）的原始欄位，§1.3 已揭露其值多為社群網址（已知缺口，獨立議題，不在本次範圍關閉），本 DTO 只收斂 `CandidateItem` 這條已上線、已通過隱私審查的路徑。

### 4.2 Ledger event 值域（19 個，進 CodeMeta）

| 類別 | Ledger event type | Wire carrier |
|---|---|---|
| Run | `RUN_ACCEPTED`, `RUN_STARTED`, `RUN_WAITING`, `RUN_CANCEL_REQUESTED`, `RUN_FINISHED`, `RUN_FAILED`, `RUN_CANCELLED`, `RUN_RESUMED`, `RUN_EXPIRED` | 原生三個：`RUN_STARTED`→`RUN_STARTED`、`RUN_FINISHED`→`RUN_FINISHED`、**`RUN_FAILED`→`RUN_ERROR`**。其餘六個走 `CUSTOM`，且**各自具名**：`run_accepted`／`run_waiting`／`run_cancel_requested`／`run_cancelled`／`run_resumed`／`run_expired` |
| Step | `STEP_STARTED`, `STEP_FINISHED` | `CUSTOM name="step_detail"`（原生 STEP_* 可並送但不可依賴） |
| Tool | `TOOL_STARTED`, `TOOL_FINISHED` | 原生 `TOOL_CALL_*` |
| Activity | `AGENT_ACTIVITY_SNAPSHOT`, `AGENT_ACTIVITY_DELTA` | `CUSTOM name="activity"` |
| Message | `MESSAGE_COMPLETED` | 原生 `TEXT_MESSAGE_*` |
| Artifact | `ARTIFACT_UPSERTED` | `CUSTOM name="artifact"` |
| Confirmation | `CONFIRMATION_REQUIRED`, `CONFIRMATION_RESOLVED` | `CUSTOM name="confirmation"` |

> 🔴 **Run 這一列的 carrier 逐字如下**（`RUN_FAILED` 歸 `CUSTOM`，六個 `CUSTOM` 型別共用一個 `name="run_status"`）：
>
> 1. **`RUN_FAILED` 走原生 `RUN_ERROR`**。`RUN_ERROR` 是 AG-UI 協定裡「這個 run 以錯誤結束」的原生事件（`@ag-ui/core` `RunErrorEventSchema`：`message` 必填、`code` 選填、schema 為 passthrough），不是傳輸層專用構造。原則：**協定已經模型化的概念用協定的原生事件，`CUSTOM` 只留給協定真的沒有的概念。**
> 2. **`CUSTOM` 的 `name` 必須能判別**。`activity`／`step_detail`／`artifact`／`confirmation` 四個 family 可以兩個型別共用一個 `name`，因為它們的 payload 自描述；Run 家族的 public payload 多半是字面 `{}`，共用一個桶會讓消費端結構上分不出是哪一種事件（實測：候選搜尋失敗與被接受在 client 完全不可分辨）。
>
> `RUN_FAILED` 的 public payload 因此必須帶 `failureReason`（`domain.RunFailedPayload`），gateway 以它組出 `RUN_ERROR.reason`。**機器碼走 `reason` 不走 `code`**——`code` 被 website 以 `/^-?\d+$/` 過濾，本 repo 既有慣例是 UPPER_SNAKE 機器碼一律走 `reason`；這個分工同時給了消費端一個天然判準：**傳輸失敗**的 `RUN_ERROR` 帶 HTTP 字串 `code`，**業務失敗**帶 `reason`。值域外的原因直接省略 `reason`，不憑空補值（CodeMeta 是該值域 SSOT）。

> Activity 兩個值刻意加 `AGENT_` 前綴，避免與 AG-UI 原生的 `ACTIVITY_SNAPSHOT`／`ACTIVITY_DELTA` **同名不同義**。

> **Activity 的 `CUSTOM name="activity"` carrier 由 POST turn stream（`SubmitTurnStream`）與 GET replay 兩條路徑共同送出。** 傳輸機制：proto `StreamEvent.activity`（`ActivityEvent{data_json}`，`StreamEvent` oneof 第 10 個 case）由 gateway `writeStreamEvent` 映射成 frame，與 replay 路徑的 `writeRunEvent` **逐字相同**，僅少了 replay-only 的 `id:` 行（§2.3、§2.4）。

**Event 不變式：**

1. `(run_id, seq)` unique；seq 在同一 run 內單調遞增。
2. State transition、event append 與 `last_event_seq` 更新在**同一 transaction**。
3. Live broadcaster 只在 DB commit **之後**送 wake hint。
4. Unknown optional event 可略過並推進 cursor；unknown major schema 必須停止 fold、重新 snapshot。
5. 禁止 RAW、`REASONING_*`、ADK Thought、system prompt、raw tool args／result。

### 4.3 狀態值域

**不在本文件重述。** Run（9 值）、Step（7 值）、Lane（7 值）、Artifact（5 值）、Confirmation（8 值）的 SSOT 是 CodeMeta `業務狀態碼.md`。

`RUN_FAILED` 是 terminal ledger event，`FAILED` 是 run status；AG-UI wire 的 `RUN_ERROR` 是 frame 型別。三者不得混稱。

### 4.4 Row identity

Activity row identity 是**一個 opaque stable id 當 row key ＋ `attempt` 當該 row 上的整數屬性**，不是複合鍵。**「heartbeat」不是資料庫更新語意**：ledger（`procurement_agent_run_events`）本身維持全程 append-only（欄位只有 `id/run_id/seq/event_type/schema_version/public_payload_json/occurred_at/trace_id/created_at`，沒有 `row_id` 欄位，寫入路徑 `AppendProcurementAgentRunEvent` 也是單純 `INSERT`，沒有任何 UPSERT）。「heartbeat 只更新同一 attempt，**不以 code 合併不同工作**」落地成：**同一個 row 生命週期內的多筆事件（例如下方 graph 節點的 entry／completion 兩筆）共用同一個 `rowId`，各自獨立 append 進 ledger，由消費端依 `rowId` 在記憶體裡合併成一列**——不是後面那筆去改寫前面那筆的 DB row，也不跨 attempt 合併（跨 attempt 走 `retryGroupKey`，見下）。

- **已落地的欄位形狀**：`AGENT_ACTIVITY_SNAPSHOT` 事件的 `data_json`（即 `public_payload_json`）是 `{rowId, attempt, maxAttempts, retryGroupKey, code, label, text, status, parentRowId?, count?}`。Wire carrier 是 AG-UI `CUSTOM name="activity"`，gateway 對 `data_json` 原樣透傳（`customSlugForRunEventType` 判斷 slug、`rawJSONOrString(ev.GetDataJson())` 帶原始 bytes，不做任何二次轉譯或裁切）。唯一的 marshal 點是 `usecase.ledgerEventTypeForActivity`。

  - `label` 是 server 端固定字串（`domain.StepLabels[code]`），**恆非空**；`text` 是「這一次實際發生了什麼」，**可為空**——空的時候消費端顯示 `label`，不是反過來。⚠️ `text` 帶的不一定是模型輸出，帶的時候也不一定來自 `StepNarrator`：`extract_slots` 的 completion 帶的是**抽取那次呼叫自帶的 `ShortIntent`**（過 `guardShortIntent`，該節點的建構子根本沒有 narrator）；`validate_slots`／`crop_gate` 帶 `StepLabels`（敘述延到 turn 尾，見 §3.4）；候選搜尋 run 恆空。**`label` 恆非空這件事是消費端的契約**：沒有它，消費端在 entry 事件那一刻（`text` 恆空）只有一個 `code`，會被迫自備第二份 code→中文對照表。
  - `status` 是 `codemeta.ProcurementStepStatus`（`PENDING`／`RUNNING`／`SUCCEEDED`／`FAILED`…），**恆非空**。🔴 **失敗住這一軸，不住 code 軸**：`domain.StepCode` 沒有 `FAILED`，不得回填——靠 entry／completion 的出現位置表達「執行中 vs 已完成」表達不了 `PENDING`（還沒跑到）與 `FAILED`（跑失敗）。
  - `parentRowId` 只在**巢狀子列**出現（目前只有候選搜尋的兩條 lane 掛在 `SEARCH_CANDIDATES` 底下），只支援一層。`count` 只在該列真的有計數概念時出現。兩者**缺席與帶零值是不同的意思**：`count` 缺席＝這一列沒有計數，`count: "0"`＝查到 0 家（有效結果）。兩者都是十進位字串（id 與計數依 Proto 管理規範 §6）。

- **兩個 producer，同一份形狀**：
  1. **graph 節點**（`extract_slots`／`validate_slots`／`crop_gate`）：`twoPhaseActivity` 送 entry（`RUNNING`，`text` 空）＋ completion（`SUCCEEDED`）兩筆；completion 的 `text` 只有 `extract_slots` 非固定（抽取呼叫自帶的 `ShortIntent`），`validate_slots`／`crop_gate` 是 `StepLabels` 固定短句（敘述延到 turn 尾，§3.4）。
  2. **候選搜尋 run**（`candidate_search_rail.go`）：`CANDIDATE_SEARCH` 這個 run type 自己的 ledger run，脫離 ADK graph 的背景 goroutine，直接 `AppendRunEvent`。`code` 是 `SEARCH_CANDIDATES`（父列）／`SEARCH_PLATFORM`／`SEARCH_EXTERNAL`（兩條 lane 子列）／`SUMMARIZE`（finalize），清單由 `domain.CandidateSearchRailPlan` 定義。這條 run 沒有 `StepNarrator`，`text` 恆空。

- **前瞻清單（`PENDING`）只對固定 pipeline 成立**：候選搜尋 run 在 claim 成功後、任何一步開始前，先把 `CandidateSearchRailPlan` 的每一列以 `status=PENDING` 送出一次，買家因此看得到「還沒跑到的步驟」。🔴 **turn run 目前不送前瞻清單，且在有決策之前不得逕自推廣**（turn run 的節點鏈雖然固定、技術上做得到，但要不要送是產品決定，追蹤於 #2549）。

- **rail 橫跨兩個 ledger run**：clarify 階段的步驟屬 `TURN` run，候選搜尋屬 `CANDIDATE_SEARCH` run。前端本來就同時握有兩個 `runId`（`POST .../candidate-searches` 回傳 `runId`），**合併由前端做**，後端不另造一層跨 run 的投影——兩個 run 的終態語意才保持獨立。

- **刻意不接線的 6 個 code**：`READ_SELLER`／`COMPARE`／`VERIFY_CERT`／`VERIFY_ORIGIN`／`CHECK_STOCK`／`RANK_CANDIDATES` 在 `CandidateSearchUseCase.run` 沒有任何對應工作（該函式全部內容是 claim → 兩條 lane 併發 → finalize；排序在各 lane 的 `Search` 內部就做完，不是跨 lane 的獨立階段）。為不存在的工作發步驟＝替買家編造進度，違反 §1.1 的 evidence contract。`domain.TestCandidateSearchRailPlan_OmitsStepsWithNoRealWork` 是這條規則唯一會轉紅的地方。

| row 類型 | row id 來源 |
|---|---|
| tool call | **直接用原生 `toolCallId`**，不另鑄 |
| 非 tool（抽取、驗證、收斂） | server 鑄一個 id |

- `attempt` ＋ `maxAttempts` 是**整數欄位**，1-indexed。
- **另加 `retryGroupKey`**：模型驅動的 retry 會鑄**全新**的 tool call id，`toolCallId` 單獨永遠無法把多次嘗試歸成一組。
- **載體走 `CUSTOM name="activity"` 或原生 `ACTIVITY_*`，不得騎在 `STEP_STARTED`／`STEP_FINISHED` 上。** AG-UI 的 Step 事件**只帶 `stepName`、沒有 id**，而其 reference verifier 對同名第二次 `STEP_STARTED` 直接 hard-throw `Step "X" is already active`——「N 列共用一個 code」在 AG-UI 是**協定違規**，retry 也不能建模成兩個同名重疊 step。


🔴 **兩條進度路徑並存，不得混淆**：上面的 Activity row identity 之外，`STEP_STARTED`／`STEP_FINISHED`（`step_detail` slug，`turn.go`／`HandleStream`）仍是 POST turn stream 的活路徑。它**沒有 row id**：`ai.proto` 的 `Step` 只有 `code/status/label/duration_ms`；gateway `stepDetailPayload` 是 `turnId/seq/code/status/label/durationMs`；website `mergeLiveStep` 以 `s.code === next.code` 收斂。⇒ **這條路徑只能以 code 收斂，消費端不得假裝它有 row id**；需要 row identity 的功能必須走 Activity 那條。

### 4.5 Artifact

每個 artifact type 必須在 JSON Schema 定義完整、versioned data schema、`checksum`、`freshness` 與 allowlisted domain reference。

- **Track A**：`REQUEST_SNAPSHOT`、`CLARIFICATION`、`CANDIDATE_RESULTS`、`PRICING_PREVIEW`、`DEAL_TEMPERATURE`、`CONFIRMED_RFQ`、`RFQ_PUBLICATION_APPROVAL`、`RFQ_PUBLICATION_RESULT`、`SELLER_QUOTE_RESULT`、`ERROR_RECOVERY`
- **Track B**：`PURCHASE_PLAN`、`CART_APPROVAL`、`CART_RESULT`

**兩個 Track 不得共用一個 generic `APPROVAL` data schema。**

**寫入路徑對 13 型完全共用**（`repository.UpsertRunArtifact`，見 §6.2），各型的差異只在自己的 `data` 形狀。🔴 **形狀尚未定案的型別不得預先發明 `data`**——本文件只凍結已展開的型別，未展開者的 schema 屬上游業務決定。已有 producer 的型別各自產製於自己的終態前：`CANDIDATE_RESULTS`（`usecase.produceCandidateResultsArtifact`，候選搜尋 `SUCCEEDED` 時）、`ERROR_RECOVERY`（`usecase.produceErrorRecoveryArtifact`）、`RFQ_PUBLICATION_RESULT`（`commerce` 的發布交易內）。

`asOf` 取這版 artifact 所引用事實裡**最舊**的那一個（`CANDIDATE_RESULTS` ＝全部候選 `factsAsOf` 的最小值）：一份混合資料的快照只能宣稱它最不新鮮的那一部分的新鮮度，取最新值等於用平台商品的即時性替一批離線名單背書。`checksum` 一律 `domain.ArtifactChecksum`（對**已序列化的 data 位元組**做 sha256 hex，與 `Slots.Hash` 同一套；對 Go 值重新 marshal 算出的值不保證描述真正被存下來的內容）。

### 4.6 Confirmation

`ConfirmationRequired` 至少含 `interruptId`、`confirmationKind`、`artifactRef`、`allowedActions`、`expiresAt`、`expectedVersion`。

**自由文字、模型回覆或按鈕 label 不得被 server 解析成正式核准。** RFQ Publication Approval 與 Cart Approval **不得走 generic resume**。

### 4.7 錯誤契約

**不開 97xx 數字碼。** CodeMeta `業務狀態碼.md` 已明文：procurement 對外 envelope 的 `code` 就是 HTTP status，沒有也不需要第二套數字業務碼。

新增錯誤一律作為 `ProcurementErrorReason` 的 additive 值：

| 需求 | 作法 |
|---|---|
| 月配額用盡 | 新值 `AGENT_MONTHLY_QUOTA_EXCEEDED`（429），回 `remaining=0` ＋ `resetAt` |
| 同 turn 進行中 | 新值 `AGENT_TURN_IN_PROGRESS`（409） |
| If-Match 過期 | revision 的 stale 用 **`STALE_REVISION`**、approval 的 stale 用 **`APPROVAL_STALE`**（兩者皆 409，`pkg/codemeta/registry.go` 的 `ProcurementErrorReasonStaleRevision`／`ProcurementErrorReasonApprovalStale`）。兩者不得互相替代：前者是「你改的那版已經不是最新」，後者是「你帶的 `expectedVersion` 與現行版本不符」 |
| 同 key 不同 payload | 已凍結為 **`IDEMPOTENCY_KEY_CONFLICT`**（不是 v6 摘錄與 #2446 內文寫的 `IDEMPOTENCY_KEY_REUSED`——同鍵同內容在本系統是成功的 replay 路徑，用 REUSED 會語意錯誤） |
| schema／capability／限流 | `SCHEMA_VERSION_UNSUPPORTED`、`CAPABILITY_UNAVAILABLE`、`RATE_LIMITED` 已存在，逐字沿用 |

錯誤 envelope 形狀沿用 `gateway/internal/handler/procurement_errors.go` 的既有實作，不重新設計。

---

## 5. 命令與 run 生命週期

### 5.1 Durable command

```
POST /app/buyers/me/procurement/turns
Accept: application/vnd.hawkr.agent-command.v1+json
```

成功固定回 **202 Accepted**。`RUN_ACCEPTED` 只代表 command、buyer message、private run input、run row 與第一筆 event 已在同一 transaction commit；worker 尚未 claim。

```
BEGIN
  claim command by (buyer_id, client_request_id) + command kind + canonical hash
  if new:
    create request/initial revision/conversation when required
    create buyer message and private run input reference
    create run and bind domain refs
    append RUN_ACCEPTED seq=1
COMMIT
return 202；commit 後才 wake executor
```

ACK 遺失後以同 key 重送**必須直接回原 run，不重跑模型**。

### 5.2 冪等鍵

**唯一權威鍵是 `(buyer_id, client_request_id)`**，且是 DB 唯一索引。

> 🔴 **不得改用 `(account_id, turn_id)` 這類組合**：同一請求會有兩把鎖、兩個唯一性判定，buyer stint 輪替後兩者分岔。
>
> 也**不新增 `turnId` 欄位**——`clientRequestId` 已存在且語意相同（`TurnRequest.client_request_id`，gateway 強制必填，website 每輪產生）。而 `turn_id` 這個名字在 wire 上已被占用且語意相反：`StepEvent.turn_id` 是伺服器回填的 `clientRequestId` echo。

可複用：`repository/revision.go` 的 `FindByIdempotencyKey`、`recoverIdempotentAppend`（23505 交易外復原）與 `revision_concurrency_test.go` 已釘住併發行為。

### 5.3 Candidate search

用具體 domain command（`POST .../requests/{requestId}/candidate-searches`），回同一個 RunAccepted envelope 但 `runType=CANDIDATE_SEARCH`，帶 allowlisted `domainRef` 指向 `candidateSetId`。

**不得新增讓 browser 任意指定 `runType` 的 generic mutation endpoint。**

### 5.4 Cancel

`cancelDisposition` 只允許 `REQUESTED`、`ALREADY_CANCELLED`、`ALREADY_TERMINAL`、`TOO_LATE`。**side effect 已 commit 時回 authoritative `SUCCEEDED` ＋ `TOO_LATE`**，不得謊報取消成功。

### 5.5 Resume

```
BEGIN
  validate owner, interrupt, schema, expiry, expected version
  claim resume clientRequestId + canonical response hash
  parent WAITING_FOR_INPUT -> RESUMED
  create one child run QUEUED
  bind interrupt.resolved_by_child_run_id
  append parent RUN_RESUMED and child RUN_ACCEPTED
COMMIT
```

同一 interrupt **只能有一個 child**。Parent 在接受 resume 後轉 `RESUMED`（handoff terminal），真正的 active execution 在 child run。

### 5.6 Snapshot

`GET .../conversations/{conversationId}/runtime-snapshot` 是 thread 重建的權威 public projection：`messages`、`runs`、`activeRunId`、`artifacts`、`openConfirmation`，以及每個 run 的 `lastSeqId`／`minAvailableSeqId`／terminal marker。

v1 每個 conversation 同時間只有一個 active run subscription。

---

### 5.7 RFQ 定價預覽、確認與發布核准（尚未展開）

v6 摘錄 §7 的這條流程（pricing preview → immutable RFQ snapshot confirmation → publication approval → execute）**本文件承接但尚未展開細節**。

🔴 **這條流程橫跨兩條線，切點在「聯絡供應商」之前**——

| 半邊 | Owner | 屬於 |
|---|---|---|
| pricing preview ＋ **immutable RFQ snapshot confirmation** | #2403 | **Agent 線**（＝§10 的完成定義，詢價單本體） |
| publication approval ＋ execute | #2450 | **商務線**（「不得自動聯絡供應商」的另一邊） |

先記下本文件其他章節已經對它下的約束，避免「約束存在、流程未定義」被讀成「可以自由設計」：

1. **四種價格事實不得混稱**（§1.1-2）。⚠️ 現況 DB 已有 `buyer_unit_price`／`seller_unit_price`——那是平台加成後的計算價，**既不是 `buyerRequestedRange` 也不是 `sellerQuotedPrice`**。展開這條流程時要先決定舊欄位的歸屬或改名，否則會變成八個名字四個概念。
2. **artifact type 固定**（§4.5）：`PRICING_PREVIEW`／`CONFIRMED_RFQ`／`RFQ_PUBLICATION_APPROVAL`／`RFQ_PUBLICATION_RESULT`。Track A／Track B 不得共用 generic `APPROVAL` schema。
3. **核准不得由自由文字或模型產生**（§4.6）；RFQ Publication Approval 與 Cart Approval **不得走 generic resume**（§5.5）。
4. **execute 必須是一個 DB transaction**（§6.4-6，同屬 ai-service DB 時）。
5. **兩種定價模式互斥**，且都禁止自動價格核准；映射缺失時走 buyer 自行輸入，不由系統臆測。

## 6. 持久化

### 6.1 通則

- DDL 走 **Atlas declarative schema**（`db/schema/` 目錄形式），不手寫 migration。
- 主鍵一律 **TSID（BIGINT）**；`artifact_id` 這類需要對外穩定字串者另加欄位，不改 PK 型別。
- CHECK constraint 一律具名 `chk_*`，不用匿名 inline。
- 刻意**不與既有 AI 表（`sessions`／`events`／`ai_*`）建外鍵**——未來抽成獨立服務時整檔搬走。
- Seq 分配：**run row lock 內原子分配，禁止 `MAX(seq)+1`**。可直接沿用 `AppendRevision` 的既有 pattern（row lock 配號 ＋ unique index 當最終防線 ＋ 23505 交易外復原）。

### 6.2 v1 表

| 表 | 用途 |
|---|---|
| `procurement_agent_runs` | run 主表 |
| `procurement_agent_run_inputs` | private input ref（raw buyer input 不進 public event） |
| `procurement_agent_run_events` | event ledger，`UNIQUE (run_id, seq)` |
| `procurement_agent_run_artifacts` | typed artifact，`UNIQUE (artifact_id, version)`。寫入路徑＝`repository.UpsertRunArtifact`：**身分鍵是 `(run_id, artifact_type)`**，同一型重新產製沿用既有 `artifact_id`、版本 +1；版本配號與 `ARTIFACT_UPSERTED` 的 append 在同一個 run row lock ／同一個交易內完成（配號規則同 §6.1 的 seq，PK 是最終防線）。舊版本不 UPDATE——artifact 是不可變快照。租戶邊界只有一條：對 run row 帶 `buyer_id` 的 `FOR UPDATE` 查詢（artifact 表本身沒有 `buyer_id`）|
| `procurement_agent_run_commands` | command claim，`UNIQUE (buyer_id, client_request_id)` |
| `procurement_agent_run_interrupts` | interrupt／confirmation |

**`procurement_agent_runs` v1 不含 `lease_owner`／`lease_expires_at`／`fencing_token`／`attempt` 四欄**（見 §8 的 Track 切分），只保留 `version BIGINT` 做 CAS。四欄以 expand migration 於 Track 2 加回。

失敗原因欄位名是 **`failure_reason`**。⚠️ 不得新增 `failure_code` 或 `error_code`——後者曾存在並已被 DROP，重新引入會讓同一語意有兩個欄位。

### 6.3 Lane

**不新增 `procurement_candidate_lane_runs`。** 現行 `procurement_candidate_sets` 已有 `platform_lane_status`／`external_lane_status`，且**已是 hawkr-website 直接消費的對外契約**。要改成獨立 lane 表必須走 expand-contract 並提出搬移路徑；v1 不做。

Lane 行為要求不變：兩條 lane 各有 stable ID 與獨立 status／version／attempt／checkpoint；每次 transition 同 transaction 更新 lane state 並 append activity event；**第一條 lane 完成就立即 commit／event／flush，不等另一條**；`processedCount`／`totalCount` 只有 authoritative 值才回，unknown total 為 `null`（**不得顯示假 percent**）。

### 6.4 Schema 不變式

1. Idempotency scope 固定 `(buyer_id, client_request_id)`。
2. 同 key ＋ 同 canonical hash 回原 run／result；同 key ＋ 不同 hash 回 409 **且零 mutation**。
3. Terminal run 不可 reopen。
4. Raw buyer input 只在 private message storage／run input ref 保存；public event 不複製。
5. Candidate row 的「同一批候選內一家一筆」已由兩條 partial UNIQUE 落地（migration `20260905170006`）：`uq_procurement_candidates_set_store (candidate_set_id, store_id) WHERE store_id IS NOT NULL` 與 `uq_procurement_candidates_set_prospect (candidate_set_id, prospect_id) WHERE prospect_id IS NOT NULL`。🔴 **鍵是業者身分，不是 `source_id`**：以 `(candidate_set_id, lane, source_id)` 為鍵對 PLATFORM 等於對 product_id 去重（同店多商品仍會產生多筆），對 EXTERNAL 等於對 primary key 去重（零效果）。**應用層去重是機制、DB 約束是防線**：PLATFORM 依 store 選代表、EXTERNAL 靠上游 `seller_prospects` 一列即一業者，約束只在機制失效時擋下。不要再補第三條索引。
6. RFQ publication approval、run、event、artifact、publication、audience projection 與 outbox 同屬 ai-service DB 時，execute **必須是一個 DB transaction**。

---

## 7. Agent 回答配額

### 7.1 問題

`/procurement/turns` 的 LLM 呼叫本身沒有次數計量，本節補上。

現有的三道護欄各管一件事，**互不取代**：

| 護欄 | 數什麼 | 軸 | 位置 |
|---|---|---|---|
| `tierDefault` 限流 | 請求數（300 / 60 秒） | 限流身分（位址） | gateway `middleware/ratelimit.go`；三棵 AI 路由樹皆在此 tier，分類由 `ratelimit_ai_routes_test.go` 對實際路由表雙向比對釘住 |
| `proxy.streamTurnDeadline` | 每輪時間（120 秒，經 gRPC deadline 傳到 ai-service） | 單一請求 | gateway proxy |
| **本節的月配額** | **成功保存的回答數** | **buyer stint** | ai-service |

⚠️ **AI 串流沒有並發連線上限**：`MaxConnPerUser` 只活在 `gateway/internal/sse` 的 Hub（通知／貼文留言 SSE，預設 3），而 AI 助手與 `/turns/stream` 走 `proxy.ServeAIAssistantStream` 直接寫 `c.Writer`，**不經過 Hub、不佔任何位子**。本節的月配額不涵蓋並發（追蹤於 #2697）。

### 7.2 規則

- 每個 **buyer（stint）** 每期最多 N 次成功保存的回答或澄清問題（N＝該買家目前生效方案的 `monthly_answer_limit`，見 59 號 §5.1）。
- **週期採錨點滾動**：以該買家 stint 自己的錨點起算，逐月推進到涵蓋 `NOW()` 的那一格。🔴 **是「每月同一天」不是「每 30 天」**——`period_end` ＝ `period_start` 下個月同一天，該日在目標月份不存在時 clamp 到該月最後一天（1/31 → 2/28），且**clamp 不自癒**（之後各期從 clamp 後的日期繼續算）。完整規則見 59 號 §5.1。
- 額度是 **buyer stint 本身的屬性**：換 stint（買家檔案刪除重建，產生新 `buyer_id`）視為全新買家，額度歸零重新起算，不隨帳號沿用。
- 系統失敗、無結果、同 turn 重送**不扣**。
- 計量軸是 **buyer_id**（stint-scoped，換 stint 歸零，非跨 stint 穩定）。落地要求：
  1. 欄位名 **`buyer_id`**——直接沿用 `procurement.sql` 檔頭既有的 `buyer_id`-owner 慣例，不需要 escape hatch 別名，**不觸發** `scripts/account-id-ratchet.py`，不需要跑 ratchet 流程或更新其 baseline。
  2. 對外 DTO 只回裸名 `buyerId`。

### 7.3 掛在哪

**`POST /app/buyers/me/procurement/turns` 與 `/turns/stream`**（兩支共用同一筆 usage row）。

⚠️ **`/buyers/me/assistant/chat` 是另一條買家泛用助手線，不在本節配額內**，要不要計量另案決定。

### 7.4 配額結果怎麼回

放進 `RUN_FINISHED.result` 的 `quota` 子物件（procurement 路徑該欄位已是完整 `TurnResult`，屬 additive），或走 `CUSTOM`。

🔴 **不存在 `event: done` 這個 frame**：兩條 SSE 路徑都是純 `data:`（§2.4）。監聽 `done` 事件的 client 永遠收不到東西。

### 7.5 扣次原子性

1. Gateway 從同一 JWT 注入 principal account 與 current buyer id；client header 一律覆寫（**已是現況**）。
2. 以 `(buyer_id, client_request_id)` INSERT reservation，同 transaction 鎖當月 counter；`COMMITTED + 未過期 reservation < limit` 才可進 Agent。
3. **terminal 結果的持久化（revision／answer_patch 的 append）與 usage RESERVED→COMMITTED 在同一個 `database.TxManager` transaction 內完成。**
4. Committed 憑證指向 procurement 自己的 durable row（`result_revision_id` → `procurement_request_revisions.id`），**不是 ADK `assistant_event_id`**。

🔴 **不得拿 ADK `events` 當 committed 憑證，也不得試圖與它同 transaction**：ADK session service 走 gorm 獨立連線、procurement 走自己的 pgxpool，兩者**跨不了 transaction**；且 procurement 的對話根本不在 ADK `events` 表（在 `procurement_answer_patches`／`procurement_request_revisions`）。ADK `events` 另有 `SessionCleaner` 每小時硬刪 24 小時前資料 ⇒ 拿它當憑證會讓「重送取回原答案」在 24 小時後**保證失敗**。

### 7.6 釋放

採用既有 `ai_usage_reservations` 的 **expiry 隱式釋放**（查詢時濾掉過期未確認者），不引入 `RELEASED_NO_CHARGE` 狀態與 reaper job。理由見該表註解：「不需清理 job；避免 process 被殺導致額度永久洩漏」。

> 實作參考：三態 reserve／confirm／release ＋ TTL ＋ `pg_advisory_xact_lock` 的既有形狀見 `internal/usecase/transform_usecase.go`。

### 7.7 Admin

用既有 `requireAdmin()`。**不得為此新增 `PROCUREMENT_OPS` 之類的專屬權限**——per-capability admin RBAC 是獨立且不小的 scope，要另案處理。

---

## 8. Executor

### 8.1 Track 切分

| Track | 範圍 | 條件 |
|---|---|---|
| **Track 1（本期）** | run ledger、command claim、202 ACK、runtime snapshot、GET SSE replay、cancel／resume | 無前置 |
| **Track 2（gated）** | DB lease executor、`FOR UPDATE SKIP LOCKED` claim、heartbeat／fencing、lease metrics、移除 `SafeGo(context.Background())` | **gated on #2452 Temporal PoC 拍板** |

**Track 2 未拍板前，v1 不交付 DB lease executor。** 過渡期的 stuck run 以 read-time staleness 終態化處理，不做 takeover。

> 這是既有決策（D1／D5／D6）。v6 §6.2 把整節寫成 v1 baseline，與之對撞；**必須以本文件為準**，否則實作者會拿到兩份互相矛盾的權威文件。

### 8.2 Port

定義 `DurableRunExecutor` port；handler、domain service 與 public DTO **不依賴 DB 或 Temporal 實作型別**。

### 8.3 Temporal PoC 邊界

1. 前置為 run ledger、executor port、DB fencing、lane checkpoint 與 minimal OTel。
2. **只在 DEV 啟用，Production flag 固定 OFF。**
3. Temporal SDK 型別只存在 adapter 與 worker command。
4. Postgres 仍是 SSOT；Temporal history 不是 business SSOT。
5. Workflow 不直接做 HTTP／DB／LLM／wall-clock／random，全部經 Activity。
6. Temporal 不可用時 **fail closed**；禁止同一 run fallback 到另一 executor 同時執行。
7. Workflow ID 固定 `candidate-search:<candidateSetId>`。
8. **Durable start intent**：DB commit 與 `StartWorkflow` 之間有 crash gap，必須以一張 PoC-only 的 start intent 表（狀態機 ＋ lease ＋ fencing ＋ 重試）封住，且該表**不列入 §6.2 的 v1 六表**——它隨 PoC 的 adopt／reject 一起去留。
8. PoC 的 ADR 結論只能是 adopt／conditional adopt／reject。

### 8.4 Agent loop 預算

🔴 **目前圖內沒有 llmagent，唯一的界是 gateway 的 `streamTurnDeadline` 120s 與各段自己的逾時常數。** 本節的 loop 預算規範在**圖內重新出現 AgentNode 時**（掛真 tool）生效，數值與機制照本節，不重新討論。

**為什麼現在不需要迭代上限**：圖是**無環的線性鏈**（單一條 `workflow.Chain`，無 `Route`、無回邊），沒有可以無限轉的迴圈——loop 預算界的是 llmagent 內部的 tool-calling 迴圈（`Flow.Run()` 的裸 `for {}`），沒有 llmagent 就沒有那個迴圈。⚠️ **各段逾時常數相加（約 83s）是快樂路徑的加總，不是嚴格上界**：`NewCropGateNode` 的 `workflow.NodeConfig` 帶 `DefaultRetryConfig()`，`cropidentity.DisambiguationUseCase.Resolve` 內部還有多段 Pricebook 呼叫，兩者都會乘上去。**硬上限只有 `streamTurnDeadline` 120s。**

掛回 AgentNode 之後 loop 必須有界，而**框架不會幫你**：ADK Go 的 `agent.RunConfig` 只有 `StreamingMode` 與 `SaveInputBlobsAsArtifacts` 兩個欄位，`MaxLLMCalls` 只存在於非本路徑使用的 `LiveRunConfig` 且從未被強制執行；`workflow` graph 套件也沒有任何迭代上限。

1. **wall-clock 是權威，不是 turn count。** 上限 **90s**。它必須先於 gateway 的 `proxy.streamTurnDeadline`（120s）觸發，並留足時間讓優雅收尾（route→終端→ledger 收尾→序列化→回傳）走完——**貼著 120s 設等於讓這層防線形同虛設**。同一條不變式也管抽取：`ExtractTimeout` 與 `RetryTimeout` 合計必須 ≤ 120s 的一半（各 30s，`extract_test.go` 有測試釘住）。⚠️ 代價：wall-clock 不反映實際工作量，慢 provider 會被誤殺，但範圍縮小到「真的異常慢」。
2. **保留最後一輪。** 接近上限時（deadline 前 15 秒）把 search／relax 類 tool 從 toolset 移除，讓最後一次呼叫**保證吐文字**而不是再開一個動作。
3. **per-tool 預算**（例：re-search ≤ 3）。觸頂時給模型**回饋**（tool 回錯誤讓它換做法），不是直接砍掉整輪。未自訂預算的 tool 套用**預設上限 10**（`loopBudgetDefaultToolCap`，`graph_loop_budget.go`）。`IsLongRunning()` 回 true 的 tool **不受這個預算約束**——ADK 對 long-running tool 走另一條內部路徑，從不呼叫 `BeforeToolCallbacks`。
   🔴 **計數器語意必須是 per-turn／per-loop-invocation，不是 per-conversation。** 現有計數器鍵只用 tool 名字組成，而 ADK session state 跨 turn 存續 ⇒ **掛真 tool 時若原封不動沿用同一組鍵，計數器會跨 turn 累加，從第二個 turn 起把某個 tool 永久鎖死**。state-key 設計必須自己處理跨 turn 重置。
4. **觸頂時回正常訊息**（「還沒收斂，請縮小範圍」），**raw error 不得到達買家**。
5. **迭代次數自己在 session state 數並用它路由**——graph engine 不提供，conditional cycle 可以無限轉。
6. **每個吃 LLM 輸出的 router node 必須掛 `workflow.Default`。** ADK scheduler 對沒配到的 route 是**靜默死路**——ADK 文件逐字：「the workflow silently dead-ends at this node… treated as a deliberate decision not to continue, **not an error**」。回邊也必須帶 Route，否則 `workflow.New` 直接以 `ErrUnconditionalCycle` 拒絕。

---

## 9. 前端契約

### 9.1 Adapter boundary

assistant-ui runtime（若採用）**只能作 Hawkr canonical store 的 view adapter**，不得成為 message／run／artifact／confirmation 的第二套真相。

不得同時保留一套 `useProcurementSession` state 與另一套 assistant-ui thread state 各自決定 active run。

採用時必須明列禁用入口（`setMessages`、branch API、`messageRepository` 等會長出第二套真相的路徑），否則無法驗收。

> 選型未定：`useAssistantTransportRuntime` 自帶 `runId`／`resumeApi`／`resumeStateApi`／`pendingCommands`／replay boundary，形狀比 `ExternalStoreRuntime` 更貼 §5.5／§5.6／§2.5。**採用前要先比較兩者**，不要因為 v6 寫了 ExternalStoreRuntime 就照抄。

### 9.2 型別來源

| 對象 | 來源 |
|---|---|
| REST DTO | gateway swagger ＋ swagger-typescript-api |
| **Agent Event／Artifact／Confirmation** | **#2446 發布的 versioned JSON Schema ＋ golden fixtures** |

**Fixture 落點沿用既有慣例，不另起一套**：backend `gateway/internal/proxy/testdata/agui-frames/*.sse`（由 `agui_golden_test.go` 逐位元組比對），website 的測試直接讀同一份，不另存拷貝。新增的 event／artifact fixture 一律加進這一組。

> 🔴 **不得以 Backend OpenAPI 產生 Agent 的 wire types**：SSE 端點在 swagger 裡只有 `@Success {string} string`，codegen 一個字都產不出來；且 `gateway/docs/swagger.json` 不進版控。型別來源見本節上方。

Zod schema **必須對照 wire fixture 撰寫，不得由 swagger 衍生**——那是兩道獨立防線。

### 9.3 責任

1. Reducer 以 `runId + seqId` 去重、排序與 fold。
2. Artifact renderer 依 `artifactType + schemaVersion` dispatch。
3. SSE 用 streaming fetch ＋ 標準 parser（**已是現況**，非 native EventSource）；`Last-Event-ID` 要自己帶。
4. Turn draft 最多保留到 committed ACK，**不進 localStorage、analytics 或 trace**。
5. Browser close 只 abort stream；**Stop 按鈕才呼叫 cancel command**。
6. **三個消費端必須讀 `RUN_FINISHED.outcome`**（website ＋ 兩支 Flutter parser）。這是整個 epic 的第一條硬約束：interrupt 走官方 `RUN_FINISHED.outcome`（`outcome` 是原生欄位、嚴格 discriminated union、`reason` 必填），但三個消費端目前**只讀 `result`**——**合規的 interrupt frame 會通過 zod 驗證後被靜默丟棄且不觸發 `onError`**。後端單獨交付 §5.5 的 resume 等於沒交付。

### 9.4 Public interaction language

改用 **claim-class allowlist**（#2549 §C 的 `allowedClaimCodes`／`forbiddenClaimCodes` ＋ §E 的六項驗證），而不是固定短句清單。

禁止：宣稱未發生的副作用（「庫存已確認」）、主觀最高級（「最佳」「推薦」）、任何指向內部推理的入口（「查看完整思考」「推理內容」）。

**前端維持 deny-by-default**：未知 code／未知 kind 整筆丟棄。要顯示模型書寫的敘述時，邊界從「固定字串表」改為「經 validator 驗證的伺服器文字 ＋ templateKey allowlist」——這是實質放寬，**需要獨立 ADR**。

🔴 **claim-class allowlist 是目標，不是現行防線。** 現行的三層驗證在 `surface_validator.go`
（`validateSurfaceBlocks`：claim allowlist ＋ 片語 blocklist ＋ fact-echo），值域與退場規則見 §3.3.1。
短句與 reply 的安全性另靠 §3.1 的四條約束，加上「**事實只在結構化 blocks，prose 不承載事實**」這條
**沒有 gate 執行的設計原則**。

⚠️ **prose 捏造的缺口只關掉一塊，這是設計書必須誠實寫下的寬度**——三層驗證做到的就是這麼多：

| 狀態 | 範圍 |
|---|---|
| **已關** | 阿拉伯數字（逐 token 相等，非子字串）＋ 城市與單位（`AllowedCities`／`AllowedUnits` 封閉值域）。允許值域取自當輪 committed 的 `domain.Slots` ∪ `SafeRenderArgs`，另含「還缺幾個欄位」這個推導計數 |
| **仍開 ①** | **中文數字**（「三個賣家」「兩箱」）不進 `arabicNumberPattern`，fact-echo 全盲。system prompt 已要求數量一律用阿拉伯數字，但模型不照做時只剩列舉式的 `forbiddenPhrases` 擋得住 |
| **仍開 ②** | **店家／賣家名**不是封閉值域，沒有任何 token 檢查 |
| **仍開 ③** | **價格**若沒進 committed slots（例如平台算出的參考價），fact-echo 沒有對照來源 |

把中文數字納入 `NumberTokensIn` 需要實測校準（「兩項」「下週」這類合法用法會被誤傷），**刻意不做**。
出現第一起相關客訴時，優先加一條 deterministic blocklist（prose 不得出現數字＋單位與店家名 token）。

🔴 **LLM client 只能從 `internal/llmprovider` 取得**：四個採購側消費者與通用聊天 agent 收到的是已判定開通與否的
`llmprovider.Procurement`／`Assistant`，**自己不建、也不判**。兩個缺陷類別各出過一次事故，都源自手刻拷貝：
**provider quirk 漂移**（漏掉 DeepSeek 的 `reasoning.effort=none` ⇒ 強制 tool_choice 一律 HTTP 400 ⇒ 敘述 100%
退回模板，而測試與 CI 全綠）與**缺金鑰拖垮啟動**（建構期回 error ⇒ fx abort ⇒ 全服務 RPC 皆滅）。
ADK 是外部套件、擋不住直接 import，那條殘餘的縫由 `scripts/check-llm-client-construction.py`（static-gates）守。

**上述殘餘必須可量測**：`realizeOrFallback` 的 reject 與 error 兩條退場路徑各記一筆 `WARN`——reject 帶 `reason`、`act_code` 與**被擋下的 block 原文**（含模型自報的 claim codes；第一層的 `claim_violation` 只看 Text 判斷不出原因），error 帶 `act_code` 與 provider 錯誤。
⚠️ **只有 `procurement_surface_validation_reject_total{reason}` 這個計數不夠**：它回答得了「擋了多少」，回答不了「擋了什麼」，而收緊或放寬任一層都需要後者。
🔴 **不取樣**：reject 是例外路徑，取樣會在最需要證據的那一筆把它丟掉。成功路徑不寫 log。

### 9.5 OSS 邊界

| 元件 | 採用範圍 | 不得負責 |
|---|---|---|
| `@assistant-ui/react` | view adapter | 第二套 conversation store |
| AI Elements | Task／Tool／Confirmation anatomy 參考 | 引入 Reasoning UI 或第二套 AI transport |
| AG-UI protocol | **全系統唯一 browser wire** | 接管 ADK、PostgreSQL、durable run ledger |
| Temporal | DEV RunExecutor adapter | business SSOT 或 Production 預設 |
| OpenTelemetry／otelhttp | Gateway CLIENT span、TraceContext-only | 記 prompt／contact／body |

**CopilotKit、LangGraph runtime、CrewAI 與其他完整 Agent 平台不得疊加**（已確認 website 目前皆未安裝）。

版本以 lockfile 為準，本文件不釘死版本號。

---

## 10. 交付範圍與完成定義

**本文件只涵蓋 agent runtime。** 交易／付款／結算那條線（RFQ publication approval、投遞、
賣家報價版本、買家接受、建單、成交溫度計）**不在本文件範圍內**——見 §0.1 與 §5.7。

**Agent 線的完成定義**（依 §1.1-1 的「準備詢價」邊界）：

> 買家能得到一份**可核對、不可變、帶版本與 hash 的詢價單**，並清楚知道它還沒有送出。

🔴 **「清楚知道它還沒有送出」是交付項不是註腳。** 投遞路徑（`publicationoutbox`）必須真的把
outbox 列送出去；任何「Warn 後回 nil、把列標成 `done`」的 stub 都會讓買家按下送出看到
「已送出」而實際什麼都沒發生。

⚠️ **凍結 confirmed RFQ 的 snapshot／hash／版本形狀時，要讓商務線的 CAS 用得上**——賣家報價的
買家接受會 CAS `expectedRfqSnapshotHash`。凍完再改要付第二次凍結的代價（golden fixtures ＋
wire ＋ 兩支前端消費端）。

---

## 11. Capability flags

| Flag | 位置 | 預設 | Rollback |
|---|---|---|---|
| `PROCUREMENT_REPLAY_SSE_ENABLED` | gateway（`k8s/values/gateway.yaml` ＋ dev／prod overlay） | values 預設 `false`；dev 與 prod overlay 皆 `true` | 先關 Web；ledger 保留 |
| `WEB_PROCUREMENT_AGENT_RUNTIME_V1` | website `features/procurement-assistant/capabilities.ts` | OFF | 回 legacy durable procurement UI |
| `WEB_PROCUREMENT_AGENT_UI_V1` | 同上 | OFF | 保留 canonical runtime，退回既有 renderer |
| `WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1` | 同上 | OFF | 關閉市場參考、詢價上下限與 update-trial flow |
| `WEB_PROCUREMENT_RFQ_APPROVAL_V1` | 同上 | OFF | 關閉 Quote companion |

**規則**：每個 flag 有 owner、allowlist、expiry／review date、metric 與 kill-switch runbook；
**Backend capability 先開、Web flag 後開**；同一 run 的 executor binding 建立後固定。

🔴 **`NEXT_PUBLIC_*` 是 build-time 值，不是即時 kill switch**：website 側的 flag 由 Server
Component 解析後往下傳，client 元件不讀 `process.env`；預設值一律全 OFF，解析失敗時 fail-safe
到「什麼新表面都不開」。

⚠️ **durable dispatch 沒有 runtime toggle**：它是唯一路徑，不由 flag 控制。rollback 的唯一手段是
把 K8s deployment 回退到 cutover 之前的 image／commit。

🔴 **`SubmitTurnStream` 目前沒有逐步串流**：graph 路徑要等整輪（slot-extraction、turn 尾平行的
StepNarrator 群組與 realize）全部跑完才一次補送所有 frame，**對 100% 買家流量皆然**，不是哪個
flag 關閉時的少數情境；失敗時買家只收到裸 gRPC error，在那之前不會有任何 frame（`RunAccepted`
或 step frame）送出。

**新增 capability flag 的門檻**：per-turn 自動 fallback（Semantic Validator）已是壞輸出的安全機制
——壞輸出不會因為 flag 開關而改變，只會因為驗證通不通過而改變。⇒ **不為模型行為新增 flag**；
只有「純停用某個外部呼叫」這種窄用途才考慮，且要先說明 fallback 為何不夠。

---

## 12. 交付裁決

五項交付裁決（訊息採 `blocks[]`、row identity、registry 凍結版本、assistant-ui 選型、executor
Track 2 的 gating）的理由、被否決方案與兩項已被推翻的裁決，見
[ADR 2026-08-22 交付裁決](../adr/2026-08-22-agent-runtime-delivery-decisions.md)。

與外部 v6.0 規格摘錄的逐項分歧見
[ADR 2026-08-22 v6 分歧](../adr/2026-08-22-agent-runtime-v6-divergence.md)。
