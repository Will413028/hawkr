# coordinator 與 all-tools fallback 的取捨

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-20 |
| 影響範圍 | ai-service 賣家／買家助理 |
| 來源 | issue #3519 D 段「coordinator 的 eval gate 從未真的跑過」 |

## 脈絡

`internal/agent/eval_notes.md` 訂了判準：「coordinator 未明顯優於回退路徑時，調指令不改架構」。
這個判準至今沒有實測資料支撐，於是兩條路徑（coordinator 與 `AI_USE_COORDINATOR=false`
的 all-tools fallback chat）一直並存——並存本身就是債。

## 量法

`cmd/coordinator-eval`：in-process 同時建兩條 runner（不必翻 server 端 env 重部署），
`ToolDeps` 全 nil（只花 LLM，不碰下游服務），觀測 `FunctionResponse.Name` 並把 tool 名
映回所屬 specialist，讓兩條路徑可比。28 條 query × 2 條路徑 = 56 次對話，Gemini。

corpus 的 `expect` 一律從 `specialists.go` 的 `Description` 推導——那正是 coordinator
看到的委派依據，所以「該不該路由到這裡」有客觀依據。

## 結果

| | coordinator | fallback |
|---|---|---|
| 單一意圖（n=21） | **21/21 (100%)** | 15/21 (71%) |
| 多步驟（n=7） | **6/7 (86%)** | 5/7 (71%) |
| 合計（n=28） | **27/28 (96%)** | 20/28 (71%) |
| 中位延遲（單一／多步驟） | 6.0s／12.6s | 2.8s／3.7s |

**fallback 的失敗模式比數字重要**：6 次單一意圖失敗裡，**5 次一個 tool 都沒叫**
（`actual=[]`）——all-tools chat 直接用 prompt 回答，沒去查資料。對使用者是「看起來
合理但沒有根據」的答案。剩下 1 次是把「哪些訂單還沒付款給我」路由到 order 而非 payment。

coordinator 唯一一次真失敗是多步驟漏叫一個 specialist（「待出貨的訂單有哪些，庫存能不能出」
只叫 order、漏 inventory）。

## 決定

1. **架構不動，coordinator 留著。** 判準要求的「明顯優於」成立（96% vs 71%，且兩個軸向都優）。
2. **那一次多步驟漏叫，走「調指令」而不是改架構**——正是判準原本規定的處置。
3. **刪除 all-tools fallback**（2026-09-20 Will 裁決，已執行）。它作為緊急開關的價值
   是負的：緊急時的行為是「不查資料就回答」，比不可用更糟——使用者無法分辨那是真資料
   還是模型自己講的。留著它也讓兩條路徑各自要維護 tool 集。

   移除範圍：`Agents.SellerChatFallback`／`BuyerChatFallback`、`Runners` 的對應欄位、
   `config.UseCoordinator`（`AI_USE_COORDINATOR`）、`ChatRunner` 的分支。
   **移除前實查：沒有任何環境把該旗標設成 false**（k8s values／compose／.env.template
   皆無），所以這次移除不改變任何現行環境的行為。

   ⚠️ 代價寫下來：coordinator 掛掉時沒有備援，只能回 9703。這是刻意選的——寧可明確
   不可用，不要讓使用者拿到編造的答案。

## 2026-09-20 追加：委派是隨機的，單次跑的數字不足以下結論

上表是**各跑一次**的結果。後續用 `--repeat 3` 重量賣家那組才發現：

| | 單次 | 3 輪重複 |
|---|---|---|
| seller_single | 12/12 | **36/36（零變異）** |
| seller_multi | 3/4 | **10/12**，變異集中在一條（1/3） |

**單一意圖是確定性的**（三輪完全一致），**多步驟不是**。所以上面那個 96% vs 71% 的比較
在單一意圖上站得住，在多步驟上只能當方向性參考。harness 已加 `--repeat`，並在摘要
明印「`--repeat 1` 的數字不能當結論」。

## 判準規定的「調指令」已執行，並量了效果

那一條多步驟漏叫（「待出貨的訂單有哪些，庫存能不能出」只叫 order）依判準走調指令。
`sellerCoordinatorInstruction` 的委派規則補上「先把問句拆成幾個獨立子問句，每個都要
各自處理完」與三個例子。同樣用 `--repeat 3` 量調整前後：

| seller_multi | 調整前 | 調整後 |
|---|---|---|
| 這週待收多少？順便看庫存夠不夠 | 3/3 | 3/3 |
| 待出貨的訂單有哪些，庫存能不能出 | **0/3** | **1/3** |
| 幫我看這個月的訂單和收款狀況 | **2/3** | **3/3** |
| 番茄行情如何？我手上的庫存夠賣嗎 | 3/3 | 3/3 |
| 合計 | **8/12 (67%)** | **10/12 (83%)** |

兩條各進步一級、無任何一條退步。但那句難的仍是 1/3——**不是修好，是從「不會」變成
「偶爾會」**。不再往下調：再調就是對單一 query 過擬合，而且下面那個 stub 干擾沒排除，
量不準。

⚠️ 指令裡的三個例子**刻意不使用 corpus 裡的句子**。第一版誤用了測試集原句，那會讓
eval 變成「量模型會不會照抄例子」的循環論證。

## 代價與未決

- coordinator 的延遲是 fallback 的 **2.1–3.4 倍**（多一跳委派）。單一意圖 6.0s 對聊天
  介面可接受；多步驟 12.6s 偏慢，是「調指令」時要一併看的數字。
- **樣本小且多步驟有隨機性**：多步驟 n=7（賣家那組 ×3 輪後為 n=12）。結論在單一意圖上
  穩（36/36），多步驟只能說方向。
- 🔴 **harness 的 stub 可能低估多步驟**：`ToolDeps` 全 nil ⇒ tool 回 error，coordinator
  拿到第一個 specialist 的錯誤後可能就不再委派第二個。其餘三條 3/3 說明這不是普遍影響，
  但那條 1/3 的可能正踩到。要排除這個干擾，得讓 stub 回**空資料**而不是 error——未做。
- 這一輪用 Gemini 量。**工具選對率是 model-specific 的**，換 provider 要重量。
- corpus 第一版有兩條的 `expect` 我自己寫錯（「訂單到哪了」同時落在 `buyer:order` 與
  `buyer:tracking` 的 Description 之間），修正後 buyer_multi 從 1/3 變 3/3。
  **那個 order／tracking 的語意邊界本身值得產品端釐清**，不是 eval 的問題。
