# Phase 1 手動驗證

## SSE 多層事件流（coordinator → agenttool specialist）
需 GEMINI_API_KEY + 依賴服務（或 mock）。本機起 ai-service：
1. `POST /seller/assistant/chat` body `{"message":"這週待收多少？順便看庫存夠不夠"}`（帶 X-Store-Id，非登入 account_id——gateway 已解析為現任 store_id 才轉發）。
2. 觀察 SSE：應看到 coordinator 串流出的最終彙整文字；中間 specialist 呼叫以 FunctionResponse 事件出現（name=payment_specialist / inventory_specialist），不應有重複或吞字。
3. 對照單一意圖：`{"message":"查我的待出貨訂單"}` 應只呼叫 order_specialist 一次。

## Eval gate（tool 選對率 / 多步驟成功率）

> **2026-09-20 已實跑。** `cmd/coordinator-eval`（in-process 同時建兩條 runner，
> `ToolDeps` 全 nil，觀測 `FunctionResponse.Name`）。28 條 × 2 條路徑：
> coordinator 27/28 (96%)、fallback 20/28 (71%)，中位延遲 6.0s／12.6s 對 2.8s／3.7s。
> 判準「明顯優於」成立 ⇒ 架構不動。結論與代價見
> `docs/adr/2026-09-20-coordinator-vs-all-tools-fallback.md`。
> 下面這段是當初的做法描述，保留備查。


準備代表性 query 集（單一意圖 x N、多步驟 x N），對 coordinator 與 fallback(all-tools) 各跑一輪，人工/腳本比對：
- 是否呼叫了正確 specialist（單一意圖）。
- 多步驟是否呼叫齊全部 specialist 並彙整。
記錄 token / 延遲。coordinator 未明顯優於 fallback 時，調 coordinator instruction（委派規則），不改架構。
