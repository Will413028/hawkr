# A9 決策簡報

要 Will 拍板的題目。證據在同目錄三份盤點（`domain-inventory.md` 的 Q1–Q13、`interface-inventory.md` §7、`client-inventory.md` §6）。拍板順序依相依：D1、D2 決定範圍，先拍；D3、D4、D6–D10 在範圍內成立，可同批拍；D5 等 A9-0 spike；D11 一併確認。

## 前提（決定之前要知道的事實）

- 系統從未上線，舊資料不搬、契約可以直接改（根目錄 `AGENTS.md`）。
- 活的 UI 只有兩組：buyer-app 助理 4 條 SSE、website 採購 2 條 SSE 加約 20 支 unary。seller 助理、刊登助理三端都被 flag 關掉；website 的 seller-match 是死碼；`product-autofill` 無呼叫端（`client-inventory.md` §4）。
- 59 支 RPC 都只被 gateway 呼叫（`interface-inventory.md` §1）。
- 單體 `agent` 模組現在只管「供應商入口」（`internal/modules/agent/AGENTS.md`），依賴表已允許 ordering、catalog、buyer、seller、media（`internal/arch/modules.go:39`）。
- ordering、catalog 仍在 Java；`internal/legacy` 沒有 `GetMyOrders`、`CreateRfqOrder`、`GetStock`、`GetProductList`、`GetCategoryTree` 的 client 方法【量：`rg "func .*\) <名稱>\(" backend/internal/legacy`】。

## A9-D1 站外業者（EXTERNAL lane）還要嗎（範圍最大的槓桿）

- 問題：採購候選除了平台賣家，還能邀請沒有帳號的站外業者報價（邀請信、供應商入口、結算 store `PROCUREMENT_SETTLEMENT_STORE_ID`）。這是農產時期「代為採購」的形狀；一般電商是否還需要？
- (a) 保留：A9 搬整套，單體 `agent` 模組（供應商入口）繼續有用。
- (b) 刪除：採購只剩平台賣家。刪 `candidate_external_lane.go`、lane 欄、外部收件人、`external_created` 事件、結算 store；單體的 `agent` 模組（供應商入口）跟著刪。約少 1.5–2.5k 行【猜，未量】。
- 業界：B2B 詢價平台（Alibaba RFQ、Amazon Business）只對平台內賣家；邀請站外廠商是採購代理（procurement agent）業務。
- 建議 **(b)**：一般電商沒有代採業務；結算 store 本身就是臨時解（AD15）。
- 擋住：A9-1 之後全部。

## A9-D2 哪些功能進 A9、哪些刪

| 功能 | 現況 | 建議 |
|---|---|---|
| 買家助理（4 條 SSE、`sessions/latest`） | buyer-app 有活 UI | 搬 |
| 賣家助理（5 條 SSE、2 unary） | 三端 flag 關閉 | 刪（重開時依新契約重做） |
| 刊登助理（product-listing SSE） | website、seller-app flag 關閉 | 刪 |
| `product-autofill` | 無呼叫端 | 刪 |
| 買家採購（2 SSE＋約 25 unary） | website 有活 UI | 搬 |
| seller-match | web 死碼 | 刪 |
| 賣家採購（6 unary，回報價） | seller-app 有 | 搬（採購的另一半） |
| AI 修圖＋配額（2 RPC、4 表） | website、seller-app 有 | 搬，拆成子階段 A9b |
| 後台風格管理（7 RPC） | admin 有 | 跟修圖 |
| 個人化推薦（1 RPC） | 固定匿名、沒有個人化 | 刪（等 catalog 搬遷時由 catalog 做） |

- 業界：上線前不搬沒人用的功能，重開時以新契約重做比維護死碼便宜。
- 建議如表。若 Will 想保留某個 flag 關閉的功能，它的成本是要一起改 Connect 與三端。

## A9-D3 模組邊界

- (a) 採購 `procurement`、買家助理 `assistant`、修圖 `imageai` 三個模組；`agent` 依 D1 刪除或保留。
- (b) 全併進 `agent`。
- 業界：以聚合邊界切；RFQ 狀態機、對話助理、修圖配額沒有共用資料。
- 建議 **(a)**。依賴表新增三列（`procurement`: ordering、catalog、buyer、seller、notification；`assistant`: ordering、catalog、buyer；`imageai`: seller、media），改依賴表等於改架構，拍板後寫進主計畫 ADR 的修訂。

## A9-D4 串流契約（主計畫 A9 交給本階段的 D7 題）

- (a) Connect server streaming，事件寫成 proto（AG-UI 的事件語意映射成 `oneof`）。
- (b) 保留 AG-UI SSE 為第二套對外契約。
- 現況：三端都手寫 AG-UI 解析（website 只用 `@ag-ui/core` 的型別與 schema；兩支 App 各一份 217 行逐字複本），沒有用 AG-UI 的 client 函式庫或 CopilotKit；只有 `runs/:runId/events` 有斷線續傳（`Last-Event-ID`）。
- 業界：AG-UI 的價值在接 CopilotKit 等現成前端；沒用時它只是另一套要手寫的格式。A6 的 IM 已選 server streaming（A6-D3）。
- 建議 **(a)**，續傳用游標欄位取代 `Last-Event-ID`（同 A8 的通知串流）。順帶消掉 App 的兩份複本與 `@ag-ui/core` 依賴。

## A9-D5 ADK v2＋GORM 的對話 session

- 買家助理的跨輪記憶靠 ADK session（4 張表、GORM 第二套 ORM）；採購的狀態在自己的 31 張表，不靠 ADK。
- (a) 保留 ADK，session 改用 pgx 的儲存實作，GORM 不進單體。
- (b) 保留 ADK 與 GORM，隔離在子套件。
- (c) 拿掉 ADK，自己存對話（一張 `messages` 表＋直接呼叫 LLM 的 tool loop）。
- 業界：agent 框架用在多 agent 編排；單一助理加幾支 tool 時，自寫 loop 是常見做法，依賴少。
- 建議 **(a)**，若 ADK 沒有可替換的 session 介面則 **(c)**；由 A9-0 spike 回答，spike 之後再交。

## A9-D6 run 的 crash 恢復（AD2）

- 現況：commit 後進程 crash，run 永遠停在 QUEUED／RUNNING，買家端一直轉圈。
- (a) River sweeper 把逾時的 run 標成失敗，買家重送。
- (b) sweeper 重新驅動 run。
- 業界：LLM 呼叫不冪等，重驅會產生不同結果與重複扣額度；標失敗＋重送是常見做法。
- 建議 **(a)**。

## A9-D7 分類別名與農產殘留（AD3）

- 851 筆別名、golden set（蘋果、小白菜）、prompt 的「生鮮」「食安」都是農產時期的；別名命中會覆寫買家的詞。
- (a) 刪別名層，只用分類樹＋人工同義詞召回；golden set 與 prompt 改成一般電商。
- (b) 為新分類樹重新生成別名（要 LLM 呼叫與 eval 成本）。
- 建議 **(a)**：上線前沒有召回率基準可守，先用最簡單的；有量測需求時再加別名層（延後，觸發：召回 eval 低於門檻）。`resolver-eval`、`prospect-category-gen`、`category-alias-gen` 三支工具不搬（前兩支打舊 gateway 或直連舊 DB）。

## A9-D8 候選資料的外洩邊界（AD7）

- 候選賣家的 `research_summary`、`positioning` 等自由文字原樣給 LLM 與買家，屏障只有 DTO 挑欄位（兩份鏡像）。
- (a) 自由文字不給買家，只給 LLM；買家看結構化欄位。
- (b) 消毒後都給。
- 建議 **(a)**，在單一 DTO 以型別區分「給 LLM」與「給買家」。

## A9-D9 配額的資料模型

- 現況：修圖（每店每週）與採購（每買家每期）各一套「預約→確認→退回」機制，各有預約表。
- (a) 各自保留，但改成一列計數器＋條件 UPDATE，外部呼叫失敗時退回；不建預約表。
- (b) 照舊搬預約表。
- 業界：預約制只用在長時間外部呼叫且需要跨進程協調時；單一進程、呼叫在 transaction 外，計數器＋退回即可。
- 建議 **(a)**。額度數字照現在 config 搬，不改產品數字。擋住 A9-4。

## A9-D10 LLM provider 範圍

- 現況：Gemini（含圖片模型）、DeepSeek、OpenRouter（免費的 `google/gemma-4-26b-a4b-it:free`，無 SLA，只在實驗用）。
- (a) 只搬 Gemini 與 DeepSeek。
- (b) 三家都搬。
- 建議 **(a)**；要再加 provider 時走同一個建構點。擋住 A9-1。

## A9-D11 一併確認（有合理預設）

- 改名：`crop_*`、`PRICE_REFERENCE_AVAILABLE`、`Unavailable`、`categoryDescribeStage` 等在寫新 proto 時一次改掉；`GateOption` 的 variadic 換明確 options（主計畫延後項的結論）。
- 指標：沿用 `ai_procurement_llm_*`；主計畫已有的「單體告警管道」延後項多記一個消費者，不改它的範圍或觸發。
- 舊事件：notification（A8 之前）消費的兩條 `procurement.*` 事件依 D1 處理；保留的那條以 relay 維持 wire 形狀（前例 `legacy/sellerrelay`）。
- ordering、catalog：經 `internal/legacy` 補 client，模組只依賴 port；不等 B8。

## 題目之間的連動

- D1 選 (b)：單體 `agent` 模組刪除，`notification` 依賴列的 `agent`、`cmd/hawkr` 的 agent 接線、`legacy/sellerserver` 的 invites 一起改（A9-1）；D3 的依賴表跟著少一列。
- D3 拍板後寫進 ADR `2026-09-24-go-modular-monolith.md` 修訂段（A9-1 第一項）。
- D5 等 A9-0 spike 後交。
