# A9-3b-3 舊 reason 對新 code 與 reason 對照表

只列 3b-3a 的 6 支方法用到的；後續子步驟（3b-3b、3c、3d）各自擴充這張表。舊值是 ai-service 的 gRPC code 與 `procurementStatus` 的 reason 字串，新值是 `apperr` 分類（對外是 Connect code）與 `ErrorReason`（`api/hawkr/procurement/v1alpha1/error_reason.proto`）。

| 方法 | 舊 code／reason | 新 code／reason | 欄位違規 |
|---|---|---|---|
| 全部（ID 格式） | INVALID_ARGUMENT／空 reason（`parseInt64` 失敗） | INVALID_ARGUMENT，無 reason | 對應的請求欄位（`request_id`、`conversation_id`、`run_id`） |
| 全部（不是本人、不存在） | NOT_FOUND／`REQUEST_NOT_FOUND` | NOT_FOUND，無 reason；不洩漏存在，非本人與不存在同一個回應 | 無 |
| GetCandidateSearch（run 不是搜尋類型、不屬於這個需求） | 舊實作依 `GetRun` 的 `(runID, requestID, buyerID)` 查不到就 NOT_FOUND | NOT_FOUND，無 reason | 無 |
| ListConversations（壞 cursor） | 靜默當第一頁 | INVALID_ARGUMENT，無 reason（A9-D31 Q7） | `page_token` |
| ListConversations（負數 page_size） | 無檢查（`<1` 改成預設值） | INVALID_ARGUMENT，無 reason | `page_size` |
| RenameConversation（標題空白、超過 40 字、含換行） | FAILED_PRECONDITION／`CONVERSATION_TITLE_INVALID` | INVALID_ARGUMENT，無 reason（新契約沒有 TITLE_INVALID，A9-D31 Q8） | `title` |
| RenameConversation、DeleteConversation（缺 `clientRequestId`） | INVALID_ARGUMENT／空 reason | INVALID_ARGUMENT，無 reason | `client_request_id` |
| DeleteConversation（已刪除重送） | NOT_FOUND／`REQUEST_NOT_FOUND` | OK（A9-D31 Q9）；從未存在或不是本人仍是 NOT_FOUND | 無 |
