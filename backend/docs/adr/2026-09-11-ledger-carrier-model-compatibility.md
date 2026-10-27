# 帳本載體：放寬 `chk_ledgtxn_model`，並用 txn_type × commerce_model 相容性守住互斥

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-11 |
| 影響範圍 | payment-service 帳本 schema |

- **脈絡**（🔴 **歷史敘述**：下述路徑已隨 `settlement-service` 於 2026-09-12 刪除，該 schema 的現行落點是 `services/java/payment-service/src/main/resources/db/migration/V9__add_logistics_finance_tables.sql`）：實讀當時的 `services/settlement-service/db/schema/schema.sql` 發現一條先前未被任何文件指出的約束：
  ```sql
  CONSTRAINT "chk_ledgtxn_model"  CHECK ("commerce_model" = 'PLATFORM_CONSIGNMENT'),
  ```
  🔴 **商流改成 `SELLER_DIRECT` 之後，整張 `ledger_transactions` 在 DB 層寫不進任何一列。**
  「零消費者」這個措辭太溫和：這不是沒人用，是**結構性排他**。
  原本的「不得改接到 `SELLER_DIRECT`」，
  它的機械形式就是這條 CHECK。⇒ 線 B／線 C 的帳務**必須**先解決載體問題。
- **選項**：
  - **A 另建一套新帳本表**：兩張長得幾乎一樣的帳本並存；「平台這個月賺賠多少」要 UNION 兩張表；
    守恆 trigger 要寫兩份
  - **B 放寬 CHECK 成值域，讓同一張帳本承載兩種商流**（採用）
  - C 刪掉舊帳本改寫成新的：破壞性，且「要不要保留受託代銷這個未來選項」是產品決定
- **決定：B —— 雙分錄模型本身是商流中立的**（`account_code` ＋ owner 五元組），
  把商流鎖進表級 CHECK 是 **fail-closed 的權宜**，不是永久設計。
  放寬時**同批**補一條相容性 CHECK，讓兩套商流的分錄在 DB 層互斥：

```sql
-- 放寬（值域，不是單值）
CONSTRAINT chk_ledgtxn_model CHECK (commerce_model IN ('PLATFORM_CONSIGNMENT','SELLER_DIRECT'))

-- 🔴 同批新增：兩套商流的 txn_type 不得交叉。漏了這條，放寬就退化成「什麼都能寫」
CONSTRAINT chk_ledgtxn_model_type CHECK (
  (commerce_model = 'PLATFORM_CONSIGNMENT' AND txn_type IN (
     'ORDER_ACCRUAL','REFUND_REVERSAL_GOODS','REFUND_REVERSAL_SHIPPING','PAYOUT_FEE',
     'PLATFORM_SERVICE_FEE','HOLD_APPLY','HOLD_RELEASE','PAYOUT_SETTLE','MANUAL_ADJUSTMENT',
     -- 🔴 這兩個**兩邊都要有**，理由見下方「一個容易做錯的地方」
     'LOGISTICS_COST_ACCRUAL','PROVIDER_SETTLEMENT'))
  OR
  (commerce_model = 'SELLER_DIRECT' AND txn_type IN (
     'LOGISTICS_COST_ACCRUAL','PROVIDER_SETTLEMENT','LOGISTICS_RECEIVABLE_ACCRUAL',
     'LOGISTICS_COLLECTION_SETTLED','LOGISTICS_TRUE_UP','LOGISTICS_CREDIT_ISSUE',
     'LOGISTICS_CREDIT_APPLY','COLLECTION_FEE_ACCRUAL','MANUAL_ADJUSTMENT'))
)
```

- 🔴 **放寬 CHECK 不會讓撥款鏈復活**：撥款的守門在 `PayoutEligibility` 的 **G0 軸**
  （`commerce_model = PLATFORM_CONSIGNMENT`），那一層完全不受本 ADR 影響；
  而上面的相容性 CHECK 讓 `PAYOUT_*`／`HOLD_*` 這些代銷專屬 txn_type **不可能**在
  `SELLER_DIRECT` 下寫入。**兩道防線方向一致，不互相取代。**
- **新增科目（`LedgerAccountCode`，補上「應收」這一側）**：現況 13 值**零 RECEIVABLE**，
  兩個 PAYABLE 都是「平台欠別人」；`PLATFORM_LOGISTICS_RECOVERY` 名字像應收但 §5.3 定義它是
  **收入科目＝買家付的運費** ⇒ 65 `:1539` 逐字「平台今天沒有『向賣家收取一筆已經花掉的錢』的能力」。

| 🆕 `account_code` | owner | 借方增加代表 | 貸方增加代表 |
|---|---|---|---|
| `SELLER_LOGISTICS_RECEIVABLE` | SELLER | **增加**賣家應收（應收成立） | 收款完成／貸記沖抵 |
| `PLATFORM_LOGISTICS_SERVICE_INCOME` | PLATFORM | 收入沖回（貸記／退還） | **平台的物流服務費收入** |
| `PLATFORM_COLLECTION_FEE_EXPENSE` | PLATFORM | **收款通道手續費**，🔴 **僅限平台負擔的例外情況**（`PLATFORM_CAUSED` 的反向沖銷，`2026-09-11-responsibility-based-cost-attribution.md`） | 沖回 |
| `SELLER_LOGISTICS_CREDIT` | SELLER | 貸記動用（抵扣／現金返還） | **貸記發生**（平台欠賣家，`2026-09-11-logistics-cost-true-up.md` 四條保證的載體） |
| 🆕 `PLATFORM_OPERATING_CASH` | PLATFORM | 平台**自有**現金流入（收到賣家的物流費） | 平台自有現金流出（付給 provider） |

🔴 **`PLATFORM_COLLECTION_FEE_EXPENSE` 不是 `2026-09-11-channel-fee-grossup-in-adapter.md` 的正常落點**（2026-09-11 驗算後更正）：
`2026-09-11-payuni-channel-fee-bearer.md` 定案手續費**由賣家負擔** ⇒ 對平台它不是費用，而是「**應收未收足的原因**」。
正常路徑**不 post 手續費分錄**，未收足的部分自然留在 `SELLER_LOGISTICS_RECEIVABLE` 餘額裡
——**那就是本 ADR 要結轉的東西**。多 post 一筆會把餘額沖平 ⇒ 結轉機制在帳上失效。
實際手續費金額記在 `logistics_collection_attempts.actual_fee` 供對帳。逐條分錄與驗算見 70 §3.8。
⚠️ **原稿把這段夾在表格中間**，GFM 遇空行即結束表格 ⇒ 第 4、5 列會掉出表格渲染成裸文字（2026-09-11 複驗抓到）。

🔴 **第 5 個科目 `PLATFORM_OPERATING_CASH` 是 2026-09-11 獨立驗收抓出來的必要補充**，
它補的是一個會讓帳單匯入直接 500 的洞：

既有 `PROVIDER_SETTLEMENT` 的分錄是 **DEBIT `PROVIDER_PAYABLE` / CREDIT `PLATFORM_COLLECTED_CASH`**（§5.3.1a）。
而 `PLATFORM_COLLECTED_CASH` 的語意是「平台**代理商戶收到買家款**」——
🔴 **`SELLER_DIRECT` 下平台沒有代收，這個科目在新商流裡沒有語意**。
⇒ 付錢給 Uber 的現金必須有一個**平台自有**的對手科目，否則 `PROVIDER_SETTLEMENT` 在新商流下
**湊不出合法分錄**，而 `trg_ledger_balanced` 會在 COMMIT 當下擋下 ⇒ 帳單匯入 500。
⚠️ 連鎖：`provider_settlement_lines` 的 `chk_provsettle_ledger` 要求
`MATCHED` ⇔ `ledger_txn_no IS NOT NULL` ⇒ **不 post 分錄就無法轉 `MATCHED`** ⇒ 整條對帳卡死。

⚠️ **`SELLER_LOGISTICS_RECEIVABLE` 與 `SELLER_LOGISTICS_CREDIT` 是兩個科目、不是一個科目的正負**：
前者是「賣家欠平台」、後者是「平台欠賣家」，兩者**可以同時存在**（這期欠、上期多收），
合成一個淨額之後 `2026-09-11-logistics-cost-true-up.md` 的第 1 條保證（永久可追蹤）就失效了。

- 🔴 **一個容易做錯的地方：`LOGISTICS_COST_ACCRUAL` 與 `PROVIDER_SETTLEMENT` 必須留在兩個分支**
  （2026-09-11 獨立驗收抓到；本 ADR 初稿把它們獨佔配給 `SELLER_DIRECT`，那是錯的）：
  - `LOGISTICS_COST_ACCRUAL` 當初是在 **§5.3.1a** 引入的，而該節章首逐字寫「本節只適用於
    `commerce_model = PLATFORM_CONSIGNMENT`」，且自述「屬純會計完整性補強，**不是商流變更**」
    ⇒ 把它移走等於回頭改壞舊商流的帳本完整性
  - **物流成本這件事本身與商流無關**：兩種商流都會付錢給 provider。
    差別只在**對手科目**（舊商流是 `PLATFORM_COLLECTED_CASH`、新商流是 `PLATFORM_OPERATING_CASH`）
  ⇒ 🔴 **`txn_type` 的歸屬看「這個動作在哪個商流下發生」，不看「它是誰引入的」。**
  ⚠️ 判準：**那兩個 txn_type 在兩個商流下的分錄科目對不同** ⇒ 它們是同名的兩種分錄，
  不是同一種。科目對由 `ledger_transactions.commerce_model` 決定，逐條見
  [70 §3.8](70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89詳細設計書.md)。

- **後果**：
  1. 守恆式 `Σ(DEBIT) = Σ(CREDIT)` 與 `trg_ledger_balanced` **維持單一實作**，不需要第二份
  2. 「平台這個月賺賠多少」是單表查詢
  3. 🔴 `chk_ledgentry_owner` 與新 txn_type 是**同一批改動的兩半**（§5.3.1 已有此警語）：
     只改一半 ⇒ 分錄在 INSERT 當下被 `23514` 擋下，而症狀是「收款流程 500」
  4. **舊商流的分錄從此在 DB 層不可能與新商流混寫** ⇒ 稽核時兩套帳互不污染
- **已落實於**：`V9__add_logistics_finance_tables.sql` 的 `ledger_transactions`（放寬後的 `chk_ledgtxn_model` 值域 ＋ `chk_ledgtxn_model_type` 互斥）；
  owner 分配見 `2026-09-11-logistics-finance-domain-ownership.md`，手續費落點見 `2026-09-11-channel-fee-grossup-in-adapter.md`。
