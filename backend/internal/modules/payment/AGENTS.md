# payment 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `docs/modules/payment.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

收款與付款。現階段只擁有收款帳戶（Stripe 計畫 D28）：外部帳戶的本地快取、建立與同步 job、輪詢、provider 的 port 與 Stripe／fake adapter、收款事件的 inbox handler、後台的讀取、重新整理與重試建立。給 seller（核准、開通 session、個資清理）與後台用。行為規格的唯一來源是 `docs/modules/payment.md`。

## 擁有的資料

schema `payment`（`db/schema.sql`）：`accounts`（收款帳戶，擁有者鍵 `store_id`，不建跨 schema FK）。view 與函式在 `db/objects.sql`。webhook inbox 不在這裡：`inbox.events` 由 platform 擁有（`internal/platform/inbox`，D29）。讀取模型：無。

## 發布的讀取介面

- `payment.pub_review_items(at)`：後台待辦的 payment 部分（現在只有 `PAYMENT_ACCOUNT_CREATE_FAILED`），只有店家 ID，給 reviewqueue（契約在 `db/pub.sql`）。規格 §6。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件（`events.go`）與錯誤。
- `internal/account`：收款帳戶的 use case、port 與 adapter、job（子領域規則見 `internal/account/AGENTS.md`）。`internal/model`：對外型別與事件。`internal/store`：sqlc 生成，不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go`：payment 是葉節點，不 import 任何模組；擁有者是否營運中與聯絡信箱經組裝點注入的 `Owners`（信箱建立外部帳戶時才讀，不存副本）。

## 不變量

- 外部呼叫不在 transaction 內（單體規則 5）。守門：fake provider 的 `Violations`（integration 測試）。規格 §2。
- 擁有者的 transaction 對 payment 只 INSERT 帳戶列（`ON CONFLICT DO NOTHING`）與排 job，不鎖、不改既有列；既有列只由本模組的 job 與後台指令寫（全域鎖順序 payment 先於 seller）。守門：integration 的 `lockorder` 檢查、`TestRequestKeepsAnExistingRow`。規格 §2、§7。
- 擁有者個資清理時先關閉外部帳戶、成功才刪本地列（沒有外部 ID 的列以冪等鍵取得帳戶再關閉，D55）；job 先標 `purge_pending_at`（呼叫 provider 之前），失敗留列，不進同步輪詢，由週期掃描重排；ForgetOwners 的 transaction 仍只排 job。守門：`TestForgetOwnersClosesTheExternalAccountThenDeletesTheRow`、`TestAFailedCloseKeepsTheRowMarkedExcludedFromPollingAndRetriedByTheSweep`。規格 §2。
- 同步在寫快照的 transaction 內發 `PayoutAccountChanged`；payload 與明文 fingerprint 不寫 log、不保存。守門：`TestSyncPublishesPayoutAccountChangedInsideTheWrite`、`TestDuplicateProviderEventIsRecordedOnceAndDispatchedOnce`。規格 §3、§5。
- 收款事件的 handler 只排同步、不標記已處理；同步只把讀取開始前收到的事件標為已處理。守門：`TestAnEventArrivingWhileTheAccountIsBeingReadIsNotLostWhenItsSyncIsCoalesced`。規格 §3。

## 不負責

- 開通閘門、防詐 hash、可曝光公告：seller（讀 seller 自存的副本）。
- inbox 的表、派送 job、補掃與清理：platform `internal/platform/inbox`。
- 確認呼叫者是擁有者：seller 的入口（D30）。

## 決策紀錄

沿用機制的判斷在同目錄的 `CARRYOVER.md`。相關 ADR：`docs/adr/2026-09-24-go-modular-monolith.md`（文末修訂「payment 模組先只擁有收款帳戶」：payment 先只有收款帳戶、seller 依賴 payment、規則 5 的 inbox 由 platform 提供）、`docs/adr/2026-10-19-seller-kyc-payuni-consolidated.md`（收款帳戶的擁有者）。計畫：`docs/plans/2026-10-29-payment-onboarding-stripe.md` D28–D30。
