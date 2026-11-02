# seller/internal/payment

收款開通：provider 中立的 port、帳戶快取、同步 job 與 webhook inbox。

## 不變量

- 外部呼叫（建立帳戶、重讀帳戶、開通 session）不在 transaction 內；一店一個建立 job、一店一個同步 job（唯一鍵 `store_id`）。守門：`TestPaymentJobsAreUniquePerStoreInRiver`。
- 狀態只由帳戶快照推導（`StatusOf`），沒有觀測歷史與事件時間排序，重讀即最新；`transfers_active` 是閘門的唯一輸入（view `store_requirement_flags.onboarding_complete`）。
- 外部事件先寫 inbox（`payment_provider_events`，(provider, event_id) 唯一），重送的事件不再排 job。
- 帳戶列不存個資；一店一列，關店再開沿用。
