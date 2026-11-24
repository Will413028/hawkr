# im/internal/live

即時通道：LISTEN 連線、Hub、訂閱者與在線狀態。

## 不變量

- LISTEN 連線必須是直連或 session 模式的 Postgres，不能經 transaction pooling：`Deps.ListenConnConfig` 指向專用連線，nil 時以主連線池的設定另開，不從池借；不在執行期偵測。
- Hub 的 `partyKey` 含 kind；扇出只合併待處理訊號、永不阻塞，超過上限回 `Resync`。每個 Party 的全域並行串流上限在 `Subscribe` 的 transaction 內以 advisory lock 數 `presence_streams`，寫入失敗讓 `Subscribe` 失敗。
- `presence_streams.expires_at` 與 `conversations.last_message_at` 只由資料庫時鐘產生與比較（查詢參數沒有時間型別）。在線狀態沒有最後上線時間，以共有對話授權，任一方向封鎖回離線。
- 串流只讀：停權與封鎖不結束串流，只有關店由 `Subscription.Revalidate` 結束；heartbeat、壽命、寫入 deadline、重驗節奏屬串流傳輸 helper，不在模組。
