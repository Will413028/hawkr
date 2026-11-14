# Agent Runtime 與外部 v6.0 規格摘錄的分歧

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-08-22 |
| 影響範圍 | ai-service procurement runtime、gateway SSE、hawkr-website |

## 脈絡

外部文件《Hawkr 幫找 Agent — LLM Agent 開發規格》（HK-PA-SPEC-007 v6.0 摘錄）是 Agent runtime
的參考來源，但它**不能直接實作**。本 ADR 保存逐項的分歧與否決理由，讓後來的人不必重新推導
——尤其是「為什麼不照那份規格做」這個問題。

三類結構性問題：

- **(a)** 它提出的 SSE envelope 送到現行前端會**驗證失敗**（實測 `@ag-ui/core` 的
  `EventSchemas.safeParse` 不通過，症狀是前端走 `onError`）。
- **(b)** 它指涉的既有物件多處錯位：配額掛錯端點、扣次掛在跨不了 transaction 的儲存體、
  引用不存在的權限。
- **(c)** 它成文於 2026-08-16，早於敘述層的定案四天，缺少整個敘述層。

## 逐項分歧

| v6 節 | v6 說 | 我們的做法 | 否決理由 |
|---|---|---|---|
| §1.2 | 配額掛 `/assistant/chat` | 掛 `/procurement/turns(/stream)` | 端點錯位；與 v6 自己的 §2.3／§3.1 打架 |
| §1.2 | `event: done` frame | `RUN_FINISHED.result.quota` | 該 frame 不存在，監聽它的 listener 永遠收不到 |
| §1.2 | 新增 `turnId` | 沿用 `clientRequestId` | 欄位已存在；`turn_id` 在 wire 上語意相反 |
| §1.2 | `PROCUREMENT_OPS` 權限 | 既有 `requireAdmin()` | 該權限不存在 |
| §1.3.2 | `(account_id, turn_id)` 冪等 | `(buyer_id, client_request_id)` | 避免兩把鎖跨 stint 分岔 |
| §1.3.3 | 先寫 ADK event 再同 transaction 扣次 | 與 revision／answer_patch 同 transaction | ADK session service 走獨立連線，**跨不了 transaction**；且對象也錯（對話不在 ADK `events` 表），而該表每小時被硬刪 |
| §1.3.4 | `RELEASED_NO_CHARGE` ＋ 10 分鐘回收 | expiry 隱式釋放 | 既有 pattern 更好，不需要 reaper |
| §1.4 | `quota_owner_account_id` | `buyer_id` | 角色前綴 pattern 會撞 JSON contract gate；且計量軸本來就該是 stint |
| §1.4 | `assistant_event_id` 為 committed 憑證 | `result_revision_id` | ADK events 每小時被清 |
| §3.3／§4.2 | 自訂 `hawkr.agent.event.v1` envelope | AG-UI frame ＋ ledger 附加鍵 | **實測驗證失敗** |
| §3.3 | `event: agent-event` 行 | 無 `event:` 行 | golden fixture 與 website 測試皆已斷言 |
| §4.2 | `ACTIVITY_SNAPSHOT`／`ACTIVITY_DELTA` | `AGENT_ACTIVITY_*`（ledger）＋ `CUSTOM name="activity"`（wire） | 與 AG-UI 原生型別同名不同義 |
| §4.6 | 9701–9705 數字碼 | `ProcurementErrorReason` additive 值 | CodeMeta 已明文否決第二套數字碼 |
| §5 | run 主表 v1 帶 lease／fencing 四欄 | v1 不帶，Track 2 expand 加回 | 與 executor 的 Track 切分對撞 |
| §5 | `failure_code`、TEXT 主鍵、匿名 CHECK | `failure_reason`、TSID 主鍵、具名 `chk_*` | 對齊既有 schema 慣例 |
| §6.2 | DB lease executor 為 v1 baseline | Track 2，gated | 與 executor 的 Track 切分對撞 |
| §6.5 | 新增 `procurement_candidate_lane_runs` | 沿用既有 lane 欄位 | 現行欄位已是對外契約，v6 未給搬移路徑 |
| §10.2.1 | swagger 產 Agent event 型別 | JSON Schema ＋ golden fixtures | SSE 在 swagger 只有 `{string}` |
| §10.3 | 固定短句 allowlist | claim-class allowlist | 短句降級為 template 示例 |
| §10.4 | 兩個表面 | **三個**表面（新增 Narrative） | 敘述層的決策 |
| §7 全節 | pricing preview／RFQ confirmation／publication approval 的完整交易流程 | 承接但分線交付，見 57 號 §5.7 | v6 §1.0 漏列此節，導致它既沒被承接也沒被否決 |

## 後果

- 57 號詳細設計書是 runtime 的 canonical 規範；v6 摘錄只能當參考，且任何引用都必須先按本表校正。
- 本 ADR 的結論不改。若某一項日後被推翻，新增一份 ADR 並在此標 superseded。
