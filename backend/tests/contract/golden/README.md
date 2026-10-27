# 事件 payload golden 契約（issue #1176）

本目錄是 notification template ↔ publisher payload-key 契約 guardrail 的 golden 來源。
設計背景見 `docs/詳細設計書/13-通知模板引擎與收件箱（Notification Template）詳細設計書.md §14`
與 `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md §7.6.4`。

## 檔案

| 檔案 | 內容 |
|------|------|
| `event_payload_keys.json` | 每個 `event_type` → 對應 publisher 實際發出的 payload key 集合（`emitted_keys`）+ publisher / lang / routing_key |
| `known_pending.json` | 已知 template↔publisher 不一致、但由後續 PR 修復的 `event_type/target_user_type`（契約測試 skip 但保留追蹤） |

## guardrail 如何運作

`services/notification-service/internal/worker/template_contract_test.go`（`//go:build integration`）：

1. testcontainers 起真 Postgres，依序套用**全部** Atlas migrations（含 `template_ssot_baseline`，
   其開頭 `DELETE FROM notification_templates` 使重放結果 deterministic 等於 dev 快照）。
2. `SELECT` 全部 `enabled` template。
3. 逐 row 驗證它讀的 payload key —— `target_user_field`、無 default 的 `#{placeholder}`
   （title / body / image / reference，復用 handler `RenderTemplate` 的解析）、`data_fields.field` 值、
   `payload_filter` key —— 都在該 event 的 `emitted_keys` 內。
4. 任一 read-key 對不上 → 測試紅 → 擋 merge。廣播 template（`fcm_topic` 非空 + 無 target_user_field）豁免；
   `known_pending.json` 列的 skip。

CI：`.github/workflows/contract-tests.yml`（PR 觸發，paths filter 涵蓋 `pkg/mq/**`、Java `**/mq/**`、
`services/notification-service/db/**`、本目錄）。

## 維護規則

- **改 template**（migration 新增 `#{placeholder}` / `data_fields.field` / `target_user_field`）：
  若讀了新 key，必須同步在 `event_payload_keys.json` 的對應 event 補該 key，且該 key 必須是 publisher 真的會發的。
- **改 publisher payload**（Java `payload()` / record，Go struct json tag）：
  同 PR 更新 `event_payload_keys.json` 對應 `emitted_keys`。
- **新增 event_type + template**：在 `event_payload_keys.json` 新增條目，否則契約測試對該 template 報「無 golden 條目」。
- **刪除 template**：同步刪掉它的 golden 條目。本檔是**契約的測試 fixture**，不是事件目錄（那是 36 號設計書）——沒有 template 消費的條目對契約零貢獻，卻會讓讀的人以為它受測試保護，而且沒有人會回頭重驗它。2026-08-16 清掉三支這樣的幽靈條目（`return.shipped`／`order.shipped.merchant`／`payment.rescue.refund_completed.seller`）。**這條由 `TestTemplateContract` 的反向斷言守住**：golden 每個 event_type 都必須在 `notification_templates` 全表（含 `enabled = false`）找得到 template。退役事件本身的 payload 記錄歸 36 號。
- `emitted_keys` 是 publisher 實發 key 的**快照**。理想以各服務 publisher-side golden 測試
  （Java `*GoldenTest`、Go `golden_test`）斷言 serializer 輸出 == 本檔，隨事件鏈遷移逐鏈補上
  （§14.6）；在補齊前，本檔為手動維護、以此測試 review 把關。
