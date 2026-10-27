# 食安事件 Postmortem — {YYYY-MM-DD} / {ticket-id}

> **用法**：複製本檔到 `docs/postmortems/food-safety/YYYY-MM-DD-{ticket-id}.md`，逐節填寫。
> **時限**：事件 `status='resolved'` 後 **7 天內** PR 開出（不必 merged）。
> **流程**：`git branch postmortem/{ticket-id}` → PR → governance lead review → merge。
> 🔴 **八個章節一節都不能刪**——內容空時填「無」／「N/A」也要保留標題。
> 章節定義的權威是 [`food-safety-incident-runbook.md` §5.3](../../runbooks/food-safety-incident-runbook.md)，
> 本檔只是它的空白表單；兩者不一致時以 runbook 為準。

| 欄位 | 值 |
|---|---|
| ticket-id | |
| 事件狀態轉 `resolved` 時間（UTC+8） | |
| 本 postmortem 撰寫人 | |
| governance lead reviewer | |

---

## 1. 事件摘要

> 1–2 段。時間、地點、商品、影響人數、最嚴重結果（住院／群聚／死亡）。
> 回答「**發生什麼？**」

## 2. 時間線

> bullet list，**每一條都要有 admin handle ＋ UTC+8 timestamp**。
> 標準節點：通報者首次反映 → 客服收件 → admin ack → 聯繫通報者 → verify →
> 建 violation → 通知賣家 → FDA 通報 → resolved。

- `YYYY-MM-DD HH:mm (UTC+8)` — @{admin} — {發生什麼}

## 3. 涉案資訊

> 回答「**誰、什麼東西、什麼狀況？**」

| 項目 | 值 |
|---|---|
| `seller_id` / 商家名稱 | |
| `product_id` / 商品名稱 | |
| 批號（若有） | |
| 通報者人數與症狀 | |
| 購買時間 | |
| 食用時間 | |

## 4. 處置動作

> 回答「**平台做了什麼？**」

- ack 時間：
- status 推進路徑：
- 是否建 violation（`penalty_type` / `suspension_days`）：
- 是否下架商品：
- 是否通知賣家：
- 賣家是否申訴、申訴決議：

## 5. FDA 通報情況

- 是否通報：
- 管道（1919 / 線上 / 傳真）：
- `reference_no`：
- FDA 後續回應（裁罰／不立案／進度待回）：

> ⚠️ **未通報時必須填判斷依據**，不得留白。

## 6. 賣家通知紀錄

- 通知時間：
- 通知管道（push / email / 簡訊）：
- 賣家回應（承認／否認／沉默）：
- 賣家是否提交申訴：

> ⚠️ **RabbitMQ publish 失敗時**要補貼 ops 介入紀錄（runbook D7）。

## 7. 後續預防措施

> bullet，**每一項都要有 owner ＋ due date**，沒有 owner 的不算行動項。

**平台層**
- [ ] {行動} — owner @{} — due {YYYY-MM-DD}
  - 是否需要新增 product 上架檢核？
  - 是否新增此類別的自動偵測？
  - admin UI 是否要加新欄位？

**賣家層**
- [ ] {行動} — owner @{} — due {YYYY-MM-DD}
  - 是否要 contract 此賣家加強 SOP？
  - 是否要追蹤該賣家後續案件率？

**流程層**
- [ ] {行動} — owner @{} — due {YYYY-MM-DD}
  - runbook 是否要更新？
  - 是否要新增 SQL SOP？

## 8. Lesson learned

> 1–2 段。學到什麼？下次會做不一樣？是否有 wiki／tech 等級的踩坑值得抽出？
> 回答「**organizational memory**」。
