# ai-service 搬進單體：只留平台內採購、三個模組、串流改 Connect

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-19（Will 決定） |
| 影響範圍 | ai-service、gateway 的 AI 與採購路由、單體 `agent` 模組、website、admin、buyer-app、seller-app |
| 執行計畫 | `backend/docs/plans/2026-11-19-a9-agent.md`（決定 A9-D1–D5、D7；實作期修訂見文末：A9-D12、D14、D22、D24、D26、D27） |

## 脈絡

ai-service 有 59 支 RPC、40 張表、約 37k 行，其中採購約 24k 行；對外 59 條 REST，含 12 條 AG-UI SSE。盤點（計畫附件，dev `bc2c732c`）顯示：

- 有活 UI 的只有買家助理（buyer-app）、買家採購（website）、賣家報價（seller-app）、AI 修圖與後台風格。賣家助理、刊登助理在三端都被 flag 關閉；`product-autofill` 沒有呼叫端；website 的 seller-match 是死碼；個人化推薦一律以匿名呼叫。
- 採購有一條「站外業者」路線：邀請沒有帳號的業者經供應商入口報價，以結算 store（`PROCUREMENT_SETTLEMENT_STORE_ID`）建單。它是農產時期代為採購的形狀。
- 三端都手寫 AG-UI 解析，沒有用 AG-UI 的 client 函式庫或 CopilotKit；兩支 App 各一份逐字相同的 `agui_sse.dart`。
- 約束：系統從未上線（根目錄 `AGENTS.md`）；pricebook 已刪、不接回（`2026-10-27-general-ecommerce-scope-cut.md` D2）；串流契約依 `2026-09-26-external-contract-protobuf-connect.md` D7 在本階段決定。

## 決定

| # | 決定 | 否決的方案與理由 |
|---|---|---|
| D1 | 刪除站外業者路線，採購只對平台賣家；單體 `agent` 模組（供應商入口）一併刪除 | 保留：一般電商沒有代採業務，結算 store 本身是臨時解；B2B 詢價平台（Alibaba RFQ、Amazon Business）只對平台內賣家 |
| D2 | 搬：買家助理、買家採購、賣家報價、AI 修圖與後台風格。刪：賣家助理、刊登助理、`product-autofill`、seller-match、個人化推薦 | 全部照搬：上線前沒人用的功能要一起改 Connect 與三端，重開時以新契約重做比維護死碼便宜 |
| D3 | 三個模組：`procurement`、`assistant`、`imageai` | 全併進一個模組：RFQ 狀態機、對話助理、修圖配額沒有共用資料，依聚合邊界切 |
| D4 | 串流改 Connect server streaming，AG-UI 事件語意映射成 proto `oneof`；續傳用游標欄位取代 `Last-Event-ID` | 保留 AG-UI SSE 為第二套契約：AG-UI 的價值在接現成前端函式庫，本專案沒用，只是另一套要手寫的格式；與 A6 的 IM（A6-D3）一致 |
| D5 | 保留 ADK 的 agent 執行層，session 改成單體以 pgx 實作 ADK 公開的 `session.Service`，以 ADK 的 `RunServiceTests` 當驗收（A9-0 spike 證實不改 ADK 原始碼即可替換，Will 2026-11-19） | 沿用 ADK 的 gorm session：單體不用 gorm，多一套 ORM 與連線池；自寫 agent 迴圈：要重做 tool 呼叫與事件語意，spike 沒找到非換不可的理由 |
| D7 | 刪除農產分類別名層（851 筆），只用分類樹＋人工同義詞召回；評測與生成工具不搬 | 為新分類樹重新生成別名：上線前沒有召回率基準可守，等有量測需求再加 |

依賴表的變更（三個新模組的依賴列、刪 `agent` 列與 `notification` 對 `agent` 的依賴）在計畫 A9-1 寫進 `2026-09-24-go-modular-monolith.md` 的修訂段。

## 後果

- 採購少掉外部收件人、lane 欄、`rfq_publication_invite.external_created` 事件與結算 store；供應商入口的 email 個資一起消失。
- 被刪的功能要重開時，依新的 proto 契約重做，不回頭找舊程式。
- 四端改用生成的 Connect client，刪除手寫的 AG-UI 解析與 `@ag-ui/core` 依賴。
- 召回品質沒有基準；計畫的延後項「召回 eval 與別名層」在需要基準時觸發。

## 修訂（2026-11-22，實作期決定）

約束：系統從未上線（external：沒有線上資料與使用者，舊形狀不受相容性約束）；單體是單一進程（inherited，`backend/AGENTS.md` 現行架構，部署未變）；外部 LLM 不在 transaction 內呼叫（inherited，單體規則 5，仍適用）。

| # | 決定 | 否決的方案與理由 |
|---|---|---|
| D5 修訂 | ADK 只用於買家助理；採購管線是一般 Go 程式，中斷的 payload 是接續的唯一狀態。ADK session 的 pgx 實作收回 assistant 模組內部（`internal/modules/assistant/internal/adksession`），不再放 platform（計畫 A9-D24(2)、A9-D27） | 採購也用 ADK 圖＋checkpoint：採購是固定順序的欄位收集與搜尋，checkpoint 只為了中斷接續，卻要多養一套 session 表與圖狀態；platform 共用套件：只剩一個使用者時，schema 參數與跨模組一致性測試沒有存在條件 |
| E1 | 助理送出是 unary（回輪次 ID），另一支訂閱串流帶游標續看；事件放進程記憶體的每輪環形緩衝，重啟後輪次失效、client 收到 NOT_FOUND 重送（A9-D12） | 送出即串流（舊 `SubmitTurnStream`）：website 經 Vercel 的串流上限 120 秒，斷線就丟掉進行中的輪次；事件存表：多一張高寫入表，只換到重啟後續看，上線前不值得 |
| E2 | 同一對話進行中再送出直接拒絕（`ASSISTANT_TURN_IN_PROGRESS`）（A9-D14） | 排隊（LangGraph 預設 enqueue，[docs](https://langchain-ai.github.io/langgraph/cloud/how-tos/enqueue_concurrent/)）：聊天介面回覆中本就鎖送出，排隊會吃掉 100 秒串流窗口；不防：ADK `session.Service` 沒有條件寫入，兩輪會交錯寫進同一段歷史 |
| E3 | 每個採購 run 一個 River job（同交易插入、MaxAttempts 1）；狀態只經單一守衛轉換函式，終態吸收；reconciler 依 job 狀態收斂（job 消失→失敗退額度）（A9-D22） | 自寫 sweeper 依 QUEUED 10 分、RUNNING 5 分逾時判定（A9-D21 原案）：逾時是猜測存活，和 executor 形成兩個不協調的終態寫入者；River 已記錄 job 的真實狀態與 rescue（durable job queue 的標準做法） |
| E4 | 只有 TURN 扣額度；不搬 LLM narrator 與 realizer，使用者面文字走模板與 ui-text；每買家 2 個進行中、每分鐘 10 次、同需求 1 個進行中（A9-D24） | 每種 run 都扣：候選搜尋與回答提問是同一輪的延續，扣兩次等於懲罰回答；搬 narrator：多一次 LLM 呼叫只為改寫文字，且違反「只由契約值決定的文字放共用文案目錄」 |
| E5 | 中斷「回答成功才解決」：提問的 run 停在 WAITING_FOR_INPUT；回答開普通 run 指向中斷；回答 run 成功（或自己再提問）的同一交易內，中斷轉 RESOLVED、提問 run 轉 SUCCEEDED；回答失敗或取消不做任何還原。取代 A9-D24(7) 的「子 run 失敗時重新開放」（A9-D26） | 回答時就解決、失敗再重開（3b-2b 初版）：要讓已結束的 run 破例改回等待，違反終態吸收，且取消路徑另需規則；同一個 run 內接續（OpenAI Assistants `requires_action`）：牴觸 E3 的每 run 一個 job。業界基準是 LangGraph 的 interrupt／resume（[docs](https://langchain-ai.github.io/langgraph/concepts/human_in_the_loop/)）：checkpoint 只在 resume 成功時前進 |

取捨：E1、E2 用進程記憶體換掉資料表，代價是多進程部署時要重做（觸發：單體改多副本）；E3 把存活判斷交給 River，代價是依賴它的 job 保留與 rescue 行為；E5 讓「處理中」要由「中斷 PENDING＋有進行中回答 run」推導，不是一個欄位。

撤銷條件：單體改成多副本（E1、E2 的記憶體狀態要改成共享儲存）；採購流程改成需要分支與回溯的多步驟 agent（D5 修訂要重評 ADK 圖）。
