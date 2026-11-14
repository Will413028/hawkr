# 步驟 9a：完成前的設計審查帳本

審查對象：`cfcfeca1..6f066b08`（719 檔）。兩位沒有參與撰寫的唯讀審查者，各審一半：後端（`backend/internal`、`backend/cmd`）與對外契約加 client（`api`、website、admin、seller-app、packages）。只看設計（照搬、將就、只記待決），不找正確性 bug。

第 1 輪：findings 21 (blocking 0)：修 16（排進 9b–9d）、記錄 5、交 Will 11（全部選「改」，D48–D54，已併入「修」）、駁回 0。

狀態欄：open＝排進子步驟、還沒做；fixed＝做完並寫出證明它的檢查；recorded＝寫進所列位置並附觸發條件。子步驟完成時回來改這一欄。

## 後端

| # | 位置 | 嚴重度 | 發現 | 處置 | 狀態 |
|---|---|---|---|---|---|
| B1 | `internal/platform/stripe/client.go`、`webhook.go` | should | 手寫 Stripe HTTP client 與驗簽，沒用官方 stripe-go，也沒寫理由 | 改（D51，9b）：先確認 SDK 支援 Accounts v2 與釘住的 preview 版本 | fixed（`fa8ad019`、`6d681a2e`）：`internal/platform/stripe` 建在 stripe-go v87.1.0 上，contract 測試重放 `2026-10-28.endive` 的錄製；`git grep -n 'Stripe-Version' -- backend/internal backend/cmd` 只剩註解與測試的斷言 |
| B2 | 分支新增的 migration 24 支（seller 19 支） | should | 建了又刪的表與欄位全留成 migration；只在本分支與 `diag/*` 套用過 | 改（D48，9d）：壓成淨差異 | fixed（`47438c57`、`bd226d9e`、`c83dc30a`、`9a5d2b7d`）：CI 診斷 run 43567659142，壓平前後 `pg_dump --schema-only` 的差異只有 `accounts_poll` 2 行；`TestPaymentOnboardingSquashedMigrationDeletesRetiredRowsAndKeepsTheRest`（8 個 mutation 各自讓它失敗） |
| B3 | `payment/internal/account/service.go` 的 `Purge` | should | 個資清理刪掉本地對應，外部帳戶永遠開著、之後找不到 | 改（D52，9b）：關閉成功才刪對應 | fixed（`603bc2b8`、`6d681a2e`、`9fa7d985`、`ab467882`）：`TestForgetOwnersClosesTheExternalAccountThenDeletesTheRow`、`TestAFailedCloseKeepsTheRowMarkedExcludedFromPollingAndRetriedByTheSweep`、`TestPurgeOfARowWithoutExternalIDStoresTheResolvedIDBeforeClosing`（CI run 43471966103） |
| B4 | `cmd/hawkr/payment.go` 的 `PAYMENT_PROVIDER=fake` | should | fake provider 讓所有帳戶直接啟用，沒有 dev 環境保護；repo 已有 `requireDevEnvironment` | 改（9b） | fixed（`95343fc8`）：`TestFakePaymentProviderOnlyInDevEnvironment` |
| B5 | `.github/workflows/backend-db-proto-checks.yml` | should | `internal/platform/inbox/db` 的 migration 不在 CI 的 atlas 檢查內 | 改（9d） | fixed（`80759b94`）：run 43567659702 驗了 `internal/platform/inbox/db`；改壞 `atlas.sum` 的 run 43563920293 失敗 |
| B6 | `payment.accounts.contact_email` | should | 信箱快照是帳戶還在 seller 時的決定，搬家後多一條已知限制 | 改（D50，9b）：建立外部帳戶當下經 `Owners` 讀 | fixed（`603bc2b8`、`8d2f34e7`，D55）：`contact_email` 欄刪除；`TestEnsureReadsTheOwnersContactEmailAtCreationTime`、`TestARetriedCreateWithAChangedEmailKeepsOneExternalAccount`（CI run 43471966103） |
| B7 | `account/jobs.go` 的 `Sweep` | nit | 每 15 分鐘輪詢所有未啟用帳戶，沒有年齡上限或退避 | 記：payment `CARRYOVER.md`，觸發：上線前定上限（9b 寫入） | recorded（`95343fc8`）：payment `CARRYOVER.md`「輪詢補漏」列，觸發：上線前 |
| B8 | `payment/db/schema.sql` 的 `accounts_poll` | nit | 索引前導欄位 `fetched_at` 沒有查詢用到 | 改（9d，隨壓平） | fixed（`80759b94`）：索引是 `(store_id)`；唯一用到的查詢 `PollablePaymentAccountStoreIDs` 以 `store_id` 篩選與排序 |
| B9 | `docs/modules/payment.md` §8 第 2 點 | nit | 已關閉店家的建立失敗待辦要管理員手動清 | 記：同處補觸發條件「上線前，或後台待辦出現已關閉店家時」（9b 寫入） | recorded（`95343fc8`）：`docs/modules/payment.md` §8 補觸發條件 |
| B10 | `account/sync.go`、`service.go` 的 provider 分支 | nit | provider 不符就靜默略過，換 provider 後舊列沒有去向 | 記：payment `CARRYOVER.md`，觸發：接第二家金流商（9b 寫入） | recorded（`95343fc8`）：payment `CARRYOVER.md`「`provider` 欄位」列，觸發：接第二家金流商 |
| B11 | `internal/platform/inbox/inbox.go` 的 `Purge`、`Sweep` | nit | 未處理事件永不刪、沒有終局狀態，每個新 handler 要自己補掃 | 記：payment `CARRYOVER.md`，觸發：第二個 handler 接上 inbox 時（9b 寫入） | recorded（`95343fc8`）：payment `CARRYOVER.md`「`inbox.events`」列，觸發：第二個 handler 接上 inbox 時 |
| B12 | `internal/platform/emailaddr` | nit | `agent/internal/model/email.go`、`seller/internal/governance/report.go` 仍各寫一份信箱規則 | 改（9b，待 Will 確認兩個行為改變；不同意就改成記錄） | fixed（`7305492a`，Will 2026-11-13、11-14 同意五項行為改變）：`git grep -n 'mail\.ParseAddress' -- backend/internal backend/cmd` 只剩 `emailaddr`；`TestValidateSubmitAnonymousEmailRule` |
| B13 | `payment/CARRYOVER.md` | nit | 漏列 Stripe client、不倒退條件、重讀名額、fake provider、`Owners`、`provider` 欄位 | 改（9b） | fixed（`95343fc8`、`8d2f34e7`）：payment `CARRYOVER.md` 補列 Stripe client、不倒退條件、重讀名額、`requirements`、fake provider、`Owners`、`provider` 欄位、兩步建立的延後項 |

## 契約與 client

| # | 位置 | 嚴重度 | 發現 | 處置 | 狀態 |
|---|---|---|---|---|---|
| C1 | `website/lib/seller/payment-account.ts`、`PaymentAccountSection.tsx`、seller-app `payment_account_action.dart` | should | 兩個 client 仍從 `PaymentAccount.status` 推導下一步，與 `OnboardingTask.action` 重複；目錄有兩套按鈕文字 | 改（D49，9c） | fixed（D56；`0c953d3a`、`4c631f1c`）：`git grep -n -E 'paymentAccountCanOnboard|paymentAccountLink|PaymentAccountActionKind|REFRESH_PAYMENT_ACCOUNT' -- backend/internal backend/cmd api/hawkr admin website seller-app packages` 0 筆；`TestOnboardingTaskStatesAndActions`、`TestPlanSessionKeepsThePreconditionsOnTheCannotEmbedPath`、seller-app `wallet_screen_test.dart` 的「收款帳戶狀態面板」群組（三個 mutation 各自讓它們失敗） |
| C2 | `CreatePaymentOnboardingSessionRequest.return_url`／`refresh_url`、seller-app `HAWKR_WEB_ORIGIN` | should | 回跳網址由 client 組，伺服器接受任何 https 網址；後端已有 `PUBLIC_WEB_ORIGIN` | 改（D49，9c） | fixed（`cb3b91ae`、`8a33414b`）：`TestOnboardingReturnPagesExistInTheWebsiteRoutes`、`TestWebLocalesMatchTheWebsiteRouting`；`git grep -n -E 'return_url|refresh_url|HAWKR_WEB_ORIGIN|onboardingUrls'` 在程式 0 筆 |
| C3 | admin payment 的 `RefreshPaymentAccount` 與 admin 的輪詢 | should | 後台重讀是排 job 加 client 輪詢，賣家端已有同步重讀 | 改（D49，9c） | fixed（`fdaab813`）：`TestAdminRefreshRereadsSynchronouslyThroughTheSellerPath`（CI mutation run 43541657688 失敗）；`git grep -n REFRESH_REREAD -- admin` 0 筆 |
| C4 | seller `PaymentAccount` 的 `requirement_collection`、`transfers_active`、`requirements`，session 的 `expire_time` | should | 賣家端沒有消費端；`requirements` 是金流商原始 JSON | 改（D49，9c） | fixed（`9f807664`）：seller `PaymentAccount` 只剩 `status`、`onboarding_surface`、`payouts_active`、`fetch_time`，session 回應沒有 `expire_time` |
| C5 | `api/hawkr/admin/payment/v1alpha1/payment.proto` import seller | should | 收款帳戶的 enum 還在 seller 的 package，方向與模組擁有者相反 | 改（D49，9c） | fixed（`a78c016f`）：`git grep -n 'hawkr/seller/v1alpha1/seller.proto' -- api/hawkr/admin/payment` 0 筆；enum 定義在 `api/hawkr/payment/v1alpha1/payment.proto` |
| C6 | `EmbeddedPaymentOnboarding.tsx`、`NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY` | should | 中立的 `EMBEDDED` 在 website 一律當成 Stripe，公開金鑰要與後端人工對齊 | 改（D53，9c） | fixed（`63d3c7e1`）：`TestStripePaymentConfig`；website 不認得的 provider 顯示錯誤狀態的兩個 unit 測試；`git grep -n NEXT_PUBLIC_STRIPE -- website` 0 筆 |
| C7 | `api/hawkr/**` 新增的 48 行 `reserved` | nit | 系統未上線，不需要保留編號與名稱 | 改（D54，9c） | fixed（`96c6c0ef`、`ac7fe9a0`）：`git grep -n -E '^[[:space:]]*reserved ' -- api/hawkr` 0 筆；`check_inventory.py` 對加回的 `reserved` 失敗 |
| C8 | `website/lib/seller/contact-email.ts`、seller-app `contact_email.dart` | nit | 聯絡信箱的格式規則在 TS 與 Dart 各一份，與客服信箱的 regex 也不同 | 記：延後，觸發：下一次改聯絡信箱規則時（計畫「延後」） | recorded |

## 審查者沒有涵蓋的範圍

- 後端：httpapi 各 handler 的本體；seller 的 `kyc`、`measure`、`purge` 與 8d、8e SQL 函式的 body；`stripe/client.go` 的重試與分頁；`adminperm`、uitext；測試檔與生成檔。
- client：`CertificationsClient.tsx`、`kyc.ts`、seller-app 的 KYC 精靈與錢包畫面、admin 的 `KycApplicationCards.tsx`、CSS、法務文件、測試；`governance.proto`、`measure.proto` 只看前段；三個子專案的 `AGENTS.md` 與兩份 ADR 全文沒有讀。

這些範圍各自在 7、8 的子步驟有 diff 審查（進度表），沒有設計視角的審查。
