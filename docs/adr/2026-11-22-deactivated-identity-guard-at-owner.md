# 停用身分的事實與守門只在擁有者

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-21（Will 決定） |
| 影響範圍 | 單體 `buyer`、`seller`（新增守門與狀態 `pub_*`）；`review`、`im`、`crm`（刪 `inactive_*` 表與 advisory lock）；`platform/jobs` 的 sweep helper；`social` 不動 |
| 執行計畫 | `backend/docs/plans/2026-11-19-a6-review-im-crm.md`（決定 A6-D19、A6-D20；簡報 `backend/docs/plans/2026-11-19-a6-review-im-crm/step-3c-brief.md`） |

## 脈絡

買家關閉帳號或店家退場後，上層模組要做三件事：立即看不到其內容、擋新寫入與舊事件重放、抹除自己的資料。A6-1 到 A6-3 讓 review、im、crm 各自存一份 `inactive_*` 複本加 advisory lock，social 也有自己的一份。複本的唯一用途是過濾與串行化，不是資料庫端存的旗標。

- 外部：Postgres 列鎖相容：`FOR KEY SHARE` 只和 `FOR UPDATE` 衝突，不和 `FOR NO KEY UPDATE` 衝突；多個交易同鎖一列會建 multixact。簡報依記憶寫，3c1 的 integration 在真實 Postgres 驗證。
- 繼承：規則 2 要求跨模組篩選先 join 擁有者的 `pub_*`，只有「資料庫端存旗標且函式只能參照自己的表」才複製（`backend/docs/開發規範/單體開發規範.md`）。這個必要條件只對 social 成立，review、im、crm 不成立，所以它們的複本沒有理由。
- 繼承：跨模組鎖序由下往上，`platform/lockorder` 只機械檢查列鎖，不涵蓋 advisory lock；仍成立，是正確性要求。
- 繼承：等待刪除的買家仍可用聊天（`2026-11-22-review-chat-crm-policy-in-monolith.md` D2），所以擁有者要提供「未關閉」而不只是「營運中」的守門。
- 繼承：Java ordering 還沒搬，舊事件會在刪除後才到；它是我們自己的程式，不是外部約束，擁有者的狀態檢查在它搬之前一樣擋得住。

## 決定

| # | 決定 | 否決的方案與理由 |
|---|---|---|
| D1 | 事實與串行化只在擁有者：buyer 加「未關閉」、seller 加「未退場」的共享列鎖（`FOR KEY SHARE`）與狀態 `pub_*`，獨占端沿用 `LockForDeletion`、`RetireDueStores`；review、im、crm 刪 `inactive_*` 與 advisory lock，讀取 join `pub_*`；不做平台框架（A6-D19）。基準：事實只有擁有者一份，消費者讀現況並取擁有者的守門，擁有者轉移時取獨占鎖並發事件（Shopify componentization；GDPR 由持有資料者各自刪除，簡報【推測】依記憶，未查證） | 維持各模組複本：每加一個消費者就多一份，同一個 River 唯一性缺陷要逐份修（A6-2a1 修過 review 那份）。平台 lifecycle 框架：各消費者的觸發時點與抹除形狀都不同（見 D3），框架會長出參數且本身仍是複本。crypto-shredding：資料不在不能改寫的儲存。soft-delete 加 RLS：policy 仍要參照擁有者狀態，抹除還是得做 |
| D2 | 守門用列鎖，不用 advisory lock（A6-D19） | advisory lock：`lockorder` 的分類器明寫不涵蓋，要涵蓋得另寫鍵前綴對應；buyer 的 `LockActive` 已是列鎖。代價 multixact 先接受，量到熱門店家有問題時改 advisory 並替 lockorder 加分類器 |
| D3 | 抹除 job 留在各模組；social 保留買家複本與每日對帳（規則 2 的例外）（A6-D19） | 抹除統一收回：社群依序鎖 stats 再重算、review 每批發 `ReviewAggregateChanged`、im 標記 `erased_at` 不刪列、crm 分批刪，形狀不同。social 也改 join：它存的是資料庫端旗標，必要條件仍成立 |
| D4 | 守門遇到不存在的 ID 當已停用並跳過；NotFound 時查 `pub_*` 分辨，已關閉或已退場記 info，不存在記 error（A6-D20 W1 (a′)） | 守門回可區分的原因：契約多一個分類，所有消費端都要處理。crm 另查買家存在：多一次讀取。買家與店家列不會被刪，不存在只可能是壞資料，重試不會讓它存在 |
| D5 | 新增獨立的 `pub_closed_buyers()`（A6-D20 W2 (a)） | 在 `pub_inactive_buyers` 加欄位：牽動 social 的 sqlc、對帳查詢與契約測試；沒有第三個消費者形狀時不抽象 |
| D6 | `LockActive`、`LockBuyerShared` 降為 `FOR KEY SHARE`（W3 (a)）；業務寫入降為 `FOR NO KEY UPDATE`，只有轉移（`BeginDeletion`、`CloseForDeletion`、`LockAccountStores`、`RetireDueStores`）保留 `FOR UPDATE`（W4 (b)） | 買家兩種共享強度：多一支鎖查詢。接受守門與業務寫入互等、量到再處理（簡報的建議，Will 選了 (b)）：每則訊息、每筆投影的共享鎖會與持 `FOR UPDATE` 的業務寫入互等，沒有量測；降鎖只一到兩行，且串行化不變（官方表：`FOR UPDATE` 與 `FOR NO KEY UPDATE` 衝突） |
| D7 | sweep 從擁有者的 `pub_*` 出發，不加索引，門檻（單輪超過 30 秒或 closed 買家超過 10 萬列）寫進 CARRYOVER（W5 (a)） | 先加部分索引：沒有量測就改兩個擁有者的 schema。從消費者端 `SELECT DISTINCT` 再 join：全表掃描 reviews |
| D8 | sweep 唯一性 helper 本步收齊全部 14 處（W6 (b)）；實作第一步逐處列出，能歸成兩種以內才各做一個 helper，超過停下回報 | 只改 review、im、crm 的 4 處：留下 10 處相同的五狀態清單，日後同一缺陷要逐處修，違反「不留技術債」 |
| D9 | 「legacy client 在 transaction 內被呼叫」以整合測試替身斷言沒有 tx（W7 (b)），本步 im 一個測試 | `internal/arch` 的 AST 規則：只抓 closure 內直接呼叫，另案；兩者都做：本步範圍外 |

## 後果

- 接受：熱門店家同時多筆共享鎖會建 multixact，沒有量測數字。重新評估：multixact 增長或守門等待有問題時，該擁有者改 advisory lock 並加分類器。
- 接受：im 的守門鎖必須在 RPC 之後、在 `inTx` 內取，取到後重新檢查狀態；守門移進 transaction 之前的 RPC 會讓列鎖跨網路呼叫。
- 接受：sweep 隨 pub 函式結果集單調增加而變慢。重新評估：單輪超過 30 秒或 closed 買家超過 10 萬列時加部分索引。
- 重新評估：第 5 個消費者且形狀與既有相同時重新評估平台框架；某消費者需要資料庫端存旗標時走 social 的形狀；ordering 搬遷時拿掉 crm 投影端的守門。
