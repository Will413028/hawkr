# B2 步驟 7：模組完成的定義逐項核對

基準：dev `ad526d5db`（PR #503 合併後）。條目來自 `backend/docs/plans/2026-09-24-go-modular-monolith.md`「模組完成的定義」。CI 證據是 `ad526d5db` 上的 dev run，每個 job 的結論都看過。

| 條目 | 結論 | 證據 |
|---|---|---|
| 目錄與 `AGENTS.md` | 完成 | `backend/internal/modules/payment/AGENTS.md`；`TestRepositoryConformsToArchitecture` 通過（本機 `go test ./internal/arch/`） |
| 沿用盤點 | 完成 | `backend/internal/modules/payment/CARRYOVER.md`；`TestEveryMovedModuleHasACarryOverAudit` PASS（本機 `go test -v -run TestEveryMovedModuleHasACarryOverAudit ./internal/arch/`） |
| 公司時期未完成項 | 完成 | `inventory.md` §9 逐項有處置（步驟 0 驗收：`check-migration-inventory.py` 無空格） |
| 獨立設計審查 | 完成，2 題交 Will | 完成前 design-review（diff `1a4e2668f^..ad526d5db`，付款相關路徑）：findings 5（blocking 0、should 2、nit 3）。處理：死信延後 → 交 Will；平台持有資金期限的觸發改成上線前查官方文件（計畫「範圍」延後段）；收款帳戶輪詢的觸發加上可量測條件（`CARRYOVER.md`）；D13 補修訂；job helper 各一份 → 駁回（第三份不存在，觸發未成立） |
| schema 在模組的 `db/`、arch 與 SQL 檢查 | 完成 | `backend/internal/modules/payment/db/`（Atlas、sqlc）；backend monolith run 45571204849 三個 job 皆 success |
| 公開 API 的依賴 | 完成 | 同上，arch 測試 |
| RPC、事件、副本去向 | 完成 | `python3 backend/scripts/check-migration-inventory.py --assume-gone payment-service`：「通過：已刪除的服務，它的 RPC、事件、副本都有去向。」；migration inventory run 成功 |
| 外部 provider 呼叫符合 D5 | 完成 | 付款、退款、轉帳、撤回、取消都是「先寫列與 job，job 呼叫外部」；integration 的 `Violations` 斷言（例：`TestCloseOrderPaymentCancelsAnAwaitingIntentThroughAJob` 的 `f.noViolations`），monolith integration job success |
| 代碼值與錯誤碼去向 | 完成 | `inventory.md` §8（20 列）；步驟 6 刪掉沒有消費者的付款值域與 `PaymentMethodIneligibleReason`（PR #503） |
| 測試與 Java 對照表 | 完成 | §7a 33 列全部對到存在的 Go 測試（步驟 7 回填 20 列；逐列 `rg -c '^func <名稱>\('` 皆為 1）；integration 在 monolith run 45571204849 success；Java tests run 45571205271 success |
| 特性測試 | 依 D2 改寫 | D2：不做對舊 Java 的 REST 特性測試，改為對照表加 Connect 測試（`internal/httpapi/payment/*_test.go`）；backend characterization run 成功 |
| 對外 API | 完成 | `api/hawkr/payment/v1alpha1`、`api/hawkr/admin/payment/v1alpha1`；`backend/internal/httpapi/server.go:554-555` 掛在 `connectServices` |
| client | 完成 | 步驟 5、6：四端用生成 client，付款 REST 與 ATM 流程刪除；website e2e run 45571204874（8 shard）、buyer-app 45571205154、seller-app 45571204998、website CI、admin CI 皆 success |
| 刪除舊服務 | 完成 | `git ls-files backend/services/java/payment-service` 為 0；compose 與 workflow 無 `payment-service`（`rg -n payment-service backend/docker-compose*.yml .github/workflows` 0 筆）；`payuni-webhook-service` 只剩物流與 Uber（B3） |
| 通知型別 | **未完成，交 Will** | 舊 `payment.rescue.refund_completed` 的買家通知改由模組事件 `AutoRefundCompleted` 承接（`inventory.md` §3 #9），但 notification 沒有訂閱它（`rg AutoRefundCompleted backend/internal/modules/notification` 0 筆）；`CARRYOVER.md` 寫的觸發「A8 訂閱之前」已過（A8 已收尾）。新增通知文案屬 UX 文案，需要同意 |
| D9 | 完成（行為約束） | D9 gate 腳本在作品集公開步驟 9 依計畫移除（`7fb87b1d6`），規則本身仍在；payment 只連 `https://api.stripe.com`（`backend/internal/platform/stripe/client.go:24`） |
| 相關 ADR | 完成 | `adr-review.md`：43 份皆有結論；`2026-09-11-payuni-channel-fee-bearer`、`2026-09-11-channel-fee-grossup-in-adapter` 改 superseded；B2 造成的殘句在步驟 8 改寫 |

## 步驟 3d、4、5 的驗收補證

- 3d adapter 分類：`internal/modules/payment/internal/charge/stripe_test.go` 的 `TestStripeMoneyErrorsAreClassifiedIntoThreeWays` 等 19 個測試，本機 `go test ./internal/modules/payment/...` ok。
- mutation（workflow_dispatch run 45572988704，dev `ad526d5db`，`b2-step3d/4/5.json`）：go 38/40、go-integration 9/10、admin 10/10 KILLED。未殺的 3 條：
  - `currency-not-usd` 存活：測試拿常數 `StripeCurrency` 比對自己。步驟 7 改成比對字面值 `"usd"`；本機在副本換成 `"twd"`，`TestStripeCreatePaymentSendsCentsUSDGroupMetadataAndKey` 失敗（killed）。
  - `resolve-wrong-actor`、`close-job-gives-up-while-creating` 是清單本身編譯失敗（變數或 import 沒用到），步驟 7 修清單。修正後 run 45577634782：go 40/40、go-integration 10/10、admin 10/10 全殺。
