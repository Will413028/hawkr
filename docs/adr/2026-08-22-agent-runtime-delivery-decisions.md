# Agent Runtime 的五項交付裁決

| 項目 | 值 |
|---|---|
| 狀態 | accepted（D2／D4 已 superseded，見各節） |
| 日期 | 2026-08-22（D2／D4 於 2026-08-26 推翻） |
| 影響範圍 | ai-service procurement runtime、proto `Step`、hawkr-website renderer |

## D1 持久化訊息採 `blocks[]`，以 expand-contract 交付

**考慮過**：① 只凍 `reply: string` ② `textFallback` ＋ `blocks[]`

**決策：②。** `RATIONALE`／`WARNING`／`NEXT_STEP` 這幾個 block kind 在扁平字串裡表達不出來，
而敘述層就是靠它們。fixtures 若先凍結成 `reply`-only，之後補 `blocks[]` 要再付一次 wire ＋
golden fixture ＋ 兩支前端消費端的成本（**二次凍結**）。

**交付方式**：additive 先加 `blocks[]` ＋ `textFallback`，`reply: string` 保留；前端切換完成
後才 DROP `reply`。同一次要處理 website `narrative.ts` 的 `legacyReplyToBlocks` fallback。

## D2 Activity row identity 用 opaque row id ＋ `attempt` 屬性

**決策**：一個 opaque row id ＋ `attempt` 整數屬性 ＋ `retryGroupKey`，proto `Step`
**不加** `stepId`／`attemptId`，tool row 直接用原生 `toolCallId`（現行規格見 57 號 §4.4）。

**考慮過並否決**：proto `Step` additive 新增 `stepId`／`attemptId` 兩欄，前端過渡期以
`stepId ?? code` 當 row key（理由是 by-code 收斂下「N 列共用同一個 code」結構上不可能）。

**否決理由**：三個 runtime 獨立收斂到同一形狀——Temporal（`ActivityID` 跨 retry 不變、
`PendingActivityInfo` 同時送 `activity_id`＋`attempt`＋`maximum_attempts`）、LangGraph
（deterministic `task_id` ＋ `ExecutionInfo.node_attempt`，1-indexed）、ADK v2
（`NodeState.Attempt` 是 retry budget 不是 identity）。複合鍵是我們自己發明的第四種。

## D3 artifact／event registry 以工作版本凍結 fixtures

**決策**：不等更新版本的確認，以當時的 registry delta 為工作版本凍結 fixtures，
**但 fixtures 與 schema 檔必須註明它未經二次確認**，之後若有更新版本以 additive 方式處理。

**理由**：等不到確認就等於無限期擋住整條 Contract Freeze，而 additive 的回頭成本遠低於停工成本。

## D4 前端邊界放寬的範圍由四條約束界定，不由表面界定

**決策**：放寬不限於 narrative 表面；Activity 表面同樣可接受伺服器文字，
但受四條約束限制（57 號 §3.1），其中「發送綁在 guard 之後」是其核心。

**考慮過並否決**：Activity rail 維持前端固定文案表（deny-by-default 不變），
只有 Narrative 表面接受經 semantic validator 驗證的伺服器文字，
**明確寫死放寬範圍僅限 narrative，否則會被援引去鬆 activity**。

**否決理由**：這條裁決預判的正是後來發生的事——Activity 表面確實被放寬了。但它不是被援引，
是它所依據的前提本身改變了：權威來源從「誰決定控制流」改成「句子只能轉述 guard 已驗過的
結構化結果」。放寬受四條約束限制（57 號 §3.1），其中「發送綁在 guard 之後」正是本裁決原本
要防的失效模式的**結構性**解法——使用者不可能看到「正在翻找 X」而那個動作沒有發生。

## D5 assistant-ui runtime 選型延後

**決策**：`ExternalStoreRuntime` vs `useAssistantTransportRuntime` 不在此時決定。

**理由**：`@assistant-ui/react-ag-ui` 仍是 0.0.x，現在選定，等真正遷移時 API 很可能已變。

## D6 Executor Track 2 維持 gated，但要給 PoC 時間窗

**決策**：Track 2（DB lease executor）維持 gated on Temporal PoC，**但「無限期 gated」不是計畫**
——PoC 需要排入時間窗，否則本項改為直接手刻 DB lease executor。

**理由**：Track 1（ledger／replay／cancel／resume）不依賴它，但 candidate search 的
`SafeGo(context.Background())` durability 缺口會一直開著。
