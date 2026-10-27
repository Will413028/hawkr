# 內容審核（檢舉 · 封鎖 · 平台下架）的架構收斂裁決

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-07-08 ~ 2026-07-10 |
| 影響範圍 | `social-service`、`account-service`（`SuspendAccount`）、`hawkr-admin` |
| 規格 | [31-內容審核（檢舉 · 封鎖 · 平台下架）](../詳細設計書/31-內容檢舉與封鎖（Content%20Reporting%20&%20Block）詳細設計書.md) |

## 脈絡

收斂之前，平台上同時存在**三套半**內容審核路徑：

- A：統一的 `reports` ／ `moderation_actions` schema（結構完整，但 app 端沒有呼叫它）
- B：legacy 的 `ReportPost`（買家實際唯一走得到的入口）
- C：商家治理的 `violation_reports`（另一個 bounded context）
- D：內容管理介面的「直接下架」——繞過 `reports`，**不寫任何稽核紀錄**

同時帶著兩個根 bug：`latest_report_reason` 誤用 `MAX(reason_code)`（按字串排序取最大，
語意上毫無意義）、`moderation_actions.user_id` 實際存的是內容 ID 而不是使用者 ID。

Apple App Store Guideline 1.2 對含 UGC 的 App 有硬性要求（內容過濾、檢舉、封鎖使用者、
24 小時內處理），而 D 路徑的無稽核軌跡直接構成合規缺口。

---

## D1：架構怎麼收斂

| 方案 | 代價 | 判定 |
|---|---|---|
| A. 沿用既有 `reports` ／ `moderation_actions`，把其他路徑併進來 | 要處理既有資料與 6 種 targetType 的跨服務接點 | ✓ |
| B. 全新 append-only ledger 三表 | 既有 schema 已具多數 best practice，重建是不必要的重工，且一樣要動 6 種 targetType 的跨服務接點 | ✗ |
| C. 只修兩個 bug、不收斂 | 三系統並存的根因原封不動，D 路徑仍然不寫稽核 | ✗ |

**決策**：A。

## D2：商家治理（`violation_reports`）要不要併進來

| 方案 | 代價 | 判定 |
|---|---|---|
| 併入共用底層 | 對象（內容作者 vs 商家）、驅動法規（Apple 1.2 vs 食安法遵）、生命週期、reason 詞彙皆異 ⇒ 共用底層是不對稱抽象 | ✗ |
| 維持獨立 bounded context | 兩套 schema、兩個 admin 佇列 | ✓ |

**決策**：不併。本模組治理「誰貼了違規內容」，32 號治理「商家資格」。

## D3：BAN_USER 要不要做成真停權

| 方案 | 代價 | 判定 |
|---|---|---|
| 維持 Redis-only 的假停權 | 重登／OAuth 路徑繞得過去，等於沒有停權 | ✗ |
| account-service 新增 `SuspendAccount` | 要新增跨服務 RPC 與 durable 狀態 | ✓ |

**決策**：account-service 新增真停權。查證後 account 完全沒有停權能力，而借用軟刪除會
混淆語意（「被停權」與「已刪除」是兩件事）。

## D4：admin 直接下架時 `reason_code` 的驗證強度

| 方案 | 代價 | 判定 |
|---|---|---|
| A. hard-fail（同使用者檢舉） | 下架動作會因為分類沒填或打錯而失敗——而下架本身是止血動作 | ✗ |
| B. 空值或非法值 fallback 到 `OTHER` | admin 的分類品質較低，統計時要知道這件事 | ✓ |

**決策**：B。與使用者檢舉刻意不同：使用者送錯分類應該被擋，admin 止血不應該被分類擋住。

## 後果

- 所有下架路徑（含 admin 直接下架）都會留下 `moderation_actions`，Apple 合規證據完整。
- `reporter_id = 0` 成為 admin-direct 的 sentinel，統計「檢舉數」時必須排除它。
- 商家治理與 UGC 審核永遠是兩套 schema，不要再有人提議合併。
