# 買家會員分級與 Agent 額度模型的設計裁決

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-08-27 ~ 2026-08-29 |
| 影響範圍 | `buyer-service`（membership 模組）、`ai-service`（procurement quota）、`api/proto/buyer/buyer.proto` |
| 規格 | [59-買家會員分級與 Agent 額度方案](../詳細設計書/59-買家會員分級與%20Agent%20額度方案（Buyer%20Membership%20&%20Entitlement）詳細設計書.md) |

## 脈絡

要為買家導入三級會員方案（free／basic／premium），額度上限必須可用資料變更調整，
且要能被 `ai-service` 既有的 reserve-commit-release 用量機制消費。當時不可動的約束：
`ai_usage_reservations` 與 57 號 §7 的計量機制已上線且有測試覆蓋；`account_id`（登入
principal）與 `buyer_id`（stint-scoped 業務身份）是兩條不可互換的軸。

---

## D1：方案／會員資料歸屬哪個 service

| 方案 | 代價 | 判定 |
|---|---|---|
| 放 `ai-service`（與 quota ledger 同服務） | 「買家是什麼會員」變成 AI runtime 的職責；未來非 agent 功能要看會員等級必須繞道 ai-service | ✗ |
| 放 `buyer-service`（與買家同服務） | 多一支跨服務同步 RPC 在 agent turn 路徑上 | ✓ |

**決策**：放 `buyer-service`。「這個買家（stint）是什麼會員方案」是買家 stint 本身的屬性，
與 AI runtime 的用量計量是兩件不同的事。

## D2：額度週期公式用全平台日曆月或買家錨點滾動

| 方案 | 代價 | 判定 |
|---|---|---|
| 全平台日曆月固定重置（57 號原規則） | 全平台在同一天湧入重置負載；續約日與額度重置日不同步 | ✗ |
| 買家自身生效日錨點滾動 | 每個買家的週期起訖不同，查詢需即時計算 | ✓ |

**決策**：錨點滾動。使續約與額度重置同步，並把負載分散到整月。

## D2b：錨點滾動的精確公式

| 方案 | 代價 | 判定 |
|---|---|---|
| (a) `period_end = period_start + 30 天` | 與 PayUni 定期定額的 `PeriodType=month`（每月同一天）語意不相容——扣款日會隨月份天數逐期漂移，等於要自建 cron 主動觸發扣款 | ✗ |
| (b) 下個月同一天，月底 clamp 到當月最後一天 | 月底起算的買家週期會被 clamp 且不自癒（1/31 → 2/28 之後長期停在 28 日） | ✓ |

**決策**：(b)。可直接沿用 PayUni 自帶排程，不需要 buyer-service 自建扣款 cron。
clamp 不回補漂移是刻意的——對齊主流訂閱制產品的既有慣例，行為可預期，
不需要額外狀態追蹤漂移量。相關約束見
[60-買家訂閱計費引擎](../詳細設計書/60-買家訂閱計費引擎（Buyer%20Subscription%20Billing%20Engine）詳細設計書.md)。

## D3：57 號 §7.2 的契約異動方式

| 方案 | 代價 | 判定 |
|---|---|---|
| 另闢欄位保留舊規則 | 同一個語意留兩套來源，必然漂移 | ✗ |
| 直接修改權威文件 | 無相容窗口 | ✓ |

**決策**：直接修改，維持單一 SSOT。

## D4：`buyer-service` 不可用時 `ai-service` 的 quota 查詢行為

| 方案 | 代價 | 判定 |
|---|---|---|
| fail-open（預設放行） | 額度控管在依賴掛掉時形同虛設 | ✗ |
| fail-closed（拒絕該次 turn） | `buyer-service` 成為 agent 路徑上的單點依賴 | ✓ |

**決策**：fail-closed。此功能流量極低，可用性風險可接受；流量上升時重新評估快取。

## D5：admin 指派是否受 `is_purchasable` 限制

**決策**：不受限。該欄位只擋一般自助購買通道，admin 指派是既有的人工例外／補償通道。

## D6：計量軸選 `buyer_id`（stint-scoped）或 `account_id`（帳號穩定）

| 方案 | 代價 | 判定 |
|---|---|---|
| `account_id`（跨 stint 沿用） | 與業務規則相反（換 stint 應歸零）；需要新插一條 account 軸並申請 account-id-ratchet 例外 | ✗ |
| `buyer_id`（stint-scoped，換 stint 歸零） | 買家刪除重建檔案會讓付費方案歸零 | ✓ |

**決策**：`buyer_id`。業務規則要求換 stint 時會員權益歸零；也與 ai-service procurement
本來就以 `buyer_id` 為主軸的慣例一致（`TurnInput.BuyerID`、`buyerIDFromContext`）。

⚠️ 已知代價：買家刪除重建買家檔案會讓付費方案憑空消失。這是刪除重建機制本身的既定
代價，不由本設計承擔。

## D7：`membership_plans` admin CRUD 開放哪些欄位

| 方案 | 代價 | 判定 |
|---|---|---|
| A. 只開放額度／價格 | `is_purchasable` 留下另一個手動 SQL 缺口（不可審計） | ✗ |
| B. 額度＋價格＋`is_purchasable`，`display_name`／`plan_code` 不開放 | `display_name` 仍需手動改 | ✓ |
| C. 全欄位（含 `display_name`） | 擴大到未被查證為缺口的範圍 | ✗ |

**決策**：B，並採 `has_*` explicit-presence 旗標做 partial update（比照既有
`UpdateAiStyleRequest`），而非全欄覆寫——只有 3 個可選欄位時 partial update 才是正確的
PATCH 語意，且能避免「前端漏帶某欄位、後端把它覆寫成零值」的事故。

## 後果

- `ai-service` 每次 agent turn 多一次同步 RPC（目標 P95 < 50ms）。
- `buyer-service` 進入 agent 的關鍵路徑，掛掉會讓 agent 全面不可用（D4 的直接後果）。
- 週期公式與 PayUni 定期定額對齊，訂閱計費不需要自建排程。

## 修訂：搬進單體後的核對（2026-10-01，A3a）

- D1：歸屬改成單體的 buyer 模組（`internal/modules/buyer`），`buyer-service` 已刪除；ai-service 經 legacy `BuyerService.GetBuyerEntitlement` 讀取。D4 的「buyer-service 不可用」變成單體不可用。
- D6：一個帳號一生一個買家（ADR `2026-09-30-account-created-provisions-buyer.md`），不再有「刪除後重建買家」，`buyer_id` 與帳號同壽，「換 stint 歸零」的情境不會發生；計量軸仍是 `buyer_id`。
- D5 的 admin 指派：後台入口沒有搬，`AssignMembership` 沒有呼叫方。Will 決定刪除指派的寫入路徑，entitlement 固定回預設方案；觸發條件記在計畫 `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 6。計費或指派入口要做時重新設計。

## 修訂：設計書 60 已刪除（2026-10-29）

會員計費不做，設計書 60（買家訂閱計費引擎）已刪除，上文指向它的連結不再有目標。D3 的決定（週期公式對齊 PayUni 定期定額）只剩週期公式這一半成立：沒有扣款流程，也就沒有由 PayUni 排程驅動的續約。
