# Uber webhook 落點：在 payuni-webhook-service 新增垂直切片，共用 inbox/outbox 兩張表

| 項目 | 值 |
|---|---|
| 狀態 | accepted；Java 的 Uber 程式刪除時（主計畫 B3）標 superseded by `2026-10-27-general-ecommerce-scope-cut.md`（D4）；在那之前 Java 仍依本檔運作 |
| 日期 | 2026-09-06 |
| 影響範圍 | payuni-webhook-service／gateway 路由 |

## 脈絡

Uber Direct 的 webhook 要落在哪裡？現有的 webhook 入口是 `payuni-webhook-service`，
它已經有 `webhook_inbox`／`webhook_outbox` 兩張表、去重索引與 raw envelope 保存。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **① `payuni-webhook-service` 加一個垂直切片，共用 inbox/outbox 兩張表** | 該服務的 `provider` 維度從此不再恆為 `PAYUNI`，下游路由要加 provider 維度；`webhook_inbox.mer_id` 要放寬 | ✅ **採用** |
| ② `shipping-service` 自己開一個 HTTP 入口 | shipping-service 是 Java 業務服務，開對外 HTTP 入口等於讓它同時是入口與業務層，違反「微服務不暴露 HTTP」 | ❌ 否決 |
| ③ 新建 `uber-webhook-service` | 為一個 provider 開一個服務，六類登記與 NetworkPolicy 的成本遠大於一個 slice | ❌ 否決 |

## 決策

**①。** 主要理由：

- **inbox 的去重與稽核已經做好了**：`UNIQUE (provider, webhook_type, event_key)` ＋ raw envelope 保存 ＋ outbox 派發，
  重寫一份等於重新製造同一組 bug。
- **不是新服務**：同 image、同 values ⇒ 部署面唯一新增是 Gateway 一條 root 層 proxy 路由。

## 後果

逐欄的 schema 與 CodeMeta 代價寫在 65 詳設 §4.5.1（三件，缺一即靜默失效），本 ADR 不複述。
最大的行為後果是 envelope 的 `provider` 欄不再恆為 `"PAYUNI"`——
下游「用 `webhookType` 路由」的地方必須加 provider 維度，否則兩個 provider 的同名 `webhookType` 會撞在一起。

## 什麼情況要重新檢視

第三個 provider 也要接 webhook，且它的 inbox 語意與既有兩者不相容時。
