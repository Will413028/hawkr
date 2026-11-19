# ai-service 搬進單體：只留平台內採購、三個模組、串流改 Connect

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-19（Will 決定） |
| 影響範圍 | ai-service、gateway 的 AI 與採購路由、單體 `agent` 模組、website、admin、buyer-app、seller-app |
| 執行計畫 | `backend/docs/plans/2026-11-19-a9-agent.md`（決定 A9-D1–D5、D7） |

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
