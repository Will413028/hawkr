# coupon 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與理由寫在計畫與 ADR，這裡只指過去。 -->

## 職責

優惠券：活動（折抵規則、總量、資格）與買家持有的券（領券、指定發放、系統補償、鎖定、核銷、釋放、作廢、提前結束活動）。給 httpapi（買家、賣家、admin）與 ordering（結帳與訂單狀態轉換，B8）用。B7 期間 Java order、aftersales 經 `internal/legacy/couponserver`（B7-4）呼叫。行為規格的唯一來源是 `backend/docs/plans/2026-11-23-b7-coupon.md` 的決定（D1–D8）與缺陷處置（BD1–BD34）。

## 擁有的資料

schema `coupon`（`db/schema.sql`）：`coupon_campaigns`（活動）、`coupon_vouchers`（買家的券）、`coupon_target_accounts`（DESIGNATED 與補償的資格名單）、`seller_coupon_quotas`（賣家活動數配額）、`coupon_redemption_logs`（稽核，只追加）。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與 `ports.go` 的 port。
- `internal/model` 型別、原因與純函式；`internal/store` 是 sqlc 的查詢套件；`internal/rows` 資料列轉型別。
- `internal/read` 買家的券與可領活動、賣家的活動列表與明細（keyset 分頁；資格只問 `qualify`）。
- `internal/qualify` 資格判定的唯一實作（`Check`：狀態、時間窗、資格；`Fit`：店家範圍、門檻、折抵）；`internal/campaign` 活動的建立、發布、結束、配額；`internal/voucher` 券的領取、發放、補償、Lock、Redeem、Release、作廢。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

- 領券與鎖券用同一個判定 `qualify.Check`，鎖券每次都重驗（`TestLockRechecksEligibility`）。
- 發行數只經 `IncrementIssued` 的條件式 UPDATE 遞增，DB 的 `campaigns_issued_count_check` 是後備（`TestConcurrentClaimNeverOverIssues`）。
- 一張券同時只被一筆訂單持有：只有 AVAILABLE 能被鎖；同一筆訂單重送冪等，別單失敗；一筆訂單只持有一張券（`vouchers_order_no_key`）（`TestConcurrentLockSucceedsOnce`、`TestLockIsIdempotentForTheSameOrder`）。
- Redeem、Release 以 `order_no` 為鍵，重送冪等；Release 在活動已結束時把券作廢而不是回到可用（`TestRedeemAndReleaseReplayAreIdempotent`、`TestReleaseAfterEndVoidsInsteadOfRestoring`）。
- 結束活動只擋新的領取與鎖定、作廢 AVAILABLE；LOCKED 照常核銷或釋放（`TestEndCampaignKeepsLockedVouchersRedeemable`）；店家只能結束自己發的非補償活動，其餘 NotFound（`TestEndCampaignRejectsStoreEndingACompensationCampaign`）。
- 補償券以 `(source_type, source_id)` 冪等：活動與券各有 UNIQUE，重送只建一張（`TestCompensationResendCreatesOneVoucher`）。
- 賣家的活動數配額在鎖住配額列之後檢查（BD15）；現金折抵不得超過門檻，門檻 0 是無門檻（BD32，`TestCreateCampaignRejectsDiscountAboveThreshold`、`campaigns_discount_within_threshold_check`）。
- 每個狀態轉換與稽核紀錄在同一個 transaction，寫入失敗整筆失敗；所有寫入在呼叫方的 transaction 內（`db.InTx` 加入它）。
- 時間用注入的 `Now`，欄位一律 `timestamptz`；到期從 `end_at` 推導，沒有改狀態的排程。

## 不負責

- 結帳與訂單狀態：ordering（B8）；B8 前由 `legacy/couponserver` 以自己的 transaction 包 Lock、Redeem、Release。
- 店家停權狀態、MEMBER 的消費紀錄：`Stores`、`Members` port 由組裝點提供（seller 的 API、B7-4 的投影）；本模組不 import `ordering` 與 `internal/legacy`。
- 刪帳事件的訂閱、MEMBER 投影、到期提醒 job 與 `VoucherExpiring` 事件：B7-4。admin 的 Connect API：B7-3b。通知：notification 模組訂閱事件。
- Connect handler 在 `internal/httpapi/coupon`（賣家能力宣告表、顯示資料組裝）；`Stores` 的正式 adapter 在 `cmd/hawkr/coupon.go`。

## 決策紀錄

從 coupon-service 沿用的每個機制、當時的決定與理由，以及與 coupon-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。
