# 後台動作的紀錄放在平台層：每個後台寫入在領域變更的同一個 transaction 內留一筆

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will，2026-11-14；收款開通 ADR D62） |
| 日期 | 2026-11-14 |
| 影響範圍 | `internal/platform/adminaudit`（新）、`internal/httpapi/adminauth`（宣告表加紀錄模式）、`internal/modules/payment`（三個後台寫入接上、刪除自有紀錄表）、`cmd/hawkr`（`admin-audit` 營運子命令） |
| 相關 | 「決策的稽核是領域列本身」與「讀取紀錄與修改紀錄目前沒有對外的列表方法」兩句的修訂寫在 `2026-10-18-seller-governance-consolidated.md` 文末；後台權限與宣告表見 `2026-10-04-admin-roles.md`；payment 的後台動作見 `2026-11-13-payment-onboarding-provider-port-stripe.md`；收款開通 ADR D60、D62（編號表在 `2026-11-13-payment-onboarding-provider-port-stripe.md` 的「決定編號」） |

## 脈絡

payment 的「已在金流商處理，刪除對應」要留下誰、何時刪了哪一列。D60 在 payment 內建了 `payment.account_mapping_deletions`，`/code-review` 指出它沒有讀取端：寫了沒有人讀得到的紀錄不算稽核。同一個問題會在每個會刪掉自身對象的後台動作重複出現（seller 的審核與處分各自在領域列上留決策欄位，但領域列只留「現在的狀態」與少數決策事實，不是一份誰做了什麼的總帳）。

約束：
- 外部：保存期未定（隱私政策未定）；紀錄裡不得放 secret 或個資原文。
- 繼承：領域模組不互相讀寫表、schema 由擁有者獨佔（`backend/AGENTS.md` 單體規則 4、6）；仍成立，所以紀錄表要有擁有者，不能由各模組寫別人的表。
- 繼承：同一個 transaction 內的寫入同生同滅（`internal/platform/db`）；仍成立，紀錄與領域變更要在同一個 transaction，不能事後補。
- 繼承：後台 RPC 的授權由 `adminauth` 宣告表逐支宣告、沒宣告啟動失敗（`2026-10-04-admin-roles.md`）；仍成立，紀錄模式沿用同一張表。
- 繼承：系統從未上線，沒有相容性與資料搬移約束（根 `AGENTS.md`）；分支內尚未併入 `dev` 的 payment migration 可直接折進（D48）。
- 不再成立而拿掉：「各模組自己留自己的紀錄」。理由：業界做法是平台層統一的稽核紀錄，由平台保證每個管理寫入都留一筆。
- 現況：只有一位管理員。

## 考慮過的方案

「基準」是同樣條件下、假設還沒有程式碼時的業界做法；標「推測」的是沒有核到官方出處。

| 決定 | 基準（業界做法） | 採用 | 未採 |
|---|---|---|---|
| 紀錄放哪一層 | Google Cloud Admin Activity、GitHub 稽核日誌、Shopify 與 Stripe 的統一稽核紀錄：誰、動作、對象、時間、結果，由平台保證每個管理寫入都有一筆（各家文件，2026-11-14 與 Will 比較時的描述，未逐項核出處） | 平台套件 `internal/platform/adminaudit`，平台自有 schema、只增不改；use case 在領域變更的同一個 transaction 內寫一筆 | 各模組自建（下一個會刪掉自身對象的動作再建一張表，每張表各自要讀取端）；只在 RPC 邊界由 interceptor 寫（與領域變更不同 transaction，可能紀錄了沒做成或做成了沒紀錄，也拿不到對象） |
| 誰保證每個寫入都有紀錄 | 平台層的宣告：每個管理方法標明留紀錄、唯讀或尚未遷移 | `adminauth` 宣告表逐支標 `audited`、`readOnly`、`auditPending`，沒有標的啟動失敗；`audited` 的 RPC 由各模組的整合測試驗證真的寫了一筆 | 只靠慣例與 code review |
| 讀取端 | 稽核日誌通常有查詢畫面 | `cmd/hawkr` 的 `admin-audit` 營運子命令（依管理員、動作、對象、時間範圍篩選），不做畫面 | 後台畫面與 proto（只有一位管理員，沒有人需要看） |
| 動作與對象的表示 | 動作是 API 方法名稱 | `action` 是後台 RPC 的 procedure 名稱，`target_type` 加 `target_id` 是對象在它自己模組的種類與 ID 文字，`detail` 只放小的結構化內容 | 為每個動作建一個列舉；detail 放整份前後快照 |
| 紀錄的語意 | 紀錄「請求已受理」而不是外部結果 | 重試建立、重試關閉寫「已要求」（outcome = requested）；provider 呼叫在之後的 job 裡，結果由領域列與 log 呈現 | 等 job 完成才寫（紀錄與領域變更不同 transaction） |

## 決策

新增平台套件 `internal/platform/adminaudit`：schema `adminaudit`，表 `records`（id、occurred_at、admin_id、action、target_type、target_id、detail jsonb），只增不改（套件只有 `Record` 與 `List`；沒有資料庫層的不可改約束，Atlas 社群版的 desired state 不放 trigger）。`Record` 必須在呼叫方的 `db.InTx` transaction 內呼叫，沒有 transaction 回 `ErrNoTransaction`，欄位不合回 `invalid_input` 並使呼叫方的 transaction 失敗：沒有紀錄的後台寫入不成立。紀錄跟領域變更一起 commit 或一起 rollback。

`adminauth` 的宣告表每支 admin 方法加一個紀錄模式：`audited`（寫入，use case 在同一個 transaction 內寫一筆）、`readOnly`（不改領域狀態）、`auditPending`（既有的寫入還沒遷移）。沒有標的啟動失敗（`Check`）；`readOnly` 只給讀取與明列的例外（`RefreshPaymentAccount`：只重讀 provider 並覆寫快取，結果由 provider 決定，不是管理員的決定；`SignOut`：結束管理員自己的 session），新的寫入方法不得宣告 `auditPending`（`backend/AGENTS.md` 一條規則）。

本分支：payment 的三個後台寫入（`RetryPaymentAccountCreate`、`RetryPaymentAccountPurge`、`DeletePaymentAccountMapping`）宣告 `audited` 並接上，刪除 `payment.account_mapping_deletions` 與它的讀寫（折進分支內尚未併入 `dev` 的 payment migration，D48）。領域會讀的決策事實仍留在領域列（例如 `purge_blocked_at`）：本決定改的是「誰做了什麼」的總帳，不是領域列的狀態欄位。其餘 41 支既有的寫入方法宣告 `auditPending`。

後果與限制：
- 紀錄的 `detail` 不放 secret、個資原文與金流商的原始說明（終局原因只放代碼）。
- 沒有資料庫層的不可改約束：有資料庫寫入權限的人可以改紀錄。
- 保存期未定（隱私政策未定）；上線前決定。

## 延後

- 其餘後台 RPC 與 seller、delegation、identity 既有紀錄表遷入平台紀錄。觸發條件：上線前（41 支方法目前是 `auditPending`，`adminauth.AuditPendingProcedures()` 列出它們）。
- 檢視畫面與讀取 proto。觸發條件：第二位管理員。
- 資料庫層的不可改約束與保存期。觸發條件：保存期隨隱私政策決定時；或需要紀錄不可被資料庫管理員改寫時。
