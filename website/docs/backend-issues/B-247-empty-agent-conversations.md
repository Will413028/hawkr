## 問題描述

`GET /app/buyers/me/procurement/conversations` 會回傳一批 **`title` 與 `lastMessagePreview` 都是空字串**的對話。

實際情形（買家 web，2026-08-13）：側欄「Hawkr Agent」區段一次列出 6 個對話，其中 **5 個**是這種空殼，每一列都只能顯示為「未命名對話」，彼此無法分辨，點進去也沒有任何內容。

```
好的，芭樂收到！請問要…    1 天前   ← 正常
未命名對話                2 天前   ← title="" preview=""
未命名對話                （無時間）  ← title="" preview=""
未命名對話                （無時間）  ← title="" preview=""
未命名對話                3 天前   ← title="" preview=""
未命名對話                3 天前   ← title="" preview=""
```

這不只是「不好看」：側欄一次只能列這麼多，空殼把賣家對話整段擠出畫面（前端版面問題已另修）。使用者看到的是「我的採購助理裡有一堆我不記得建立過的空對話」。

## 前端目前怎麼處理

`ConversationSidebar` 已加上前端過濾：`title` 與 `preview` 皆空的對話**不進列表**（正在檢視中的那一個例外，避免使用者以為對話被刪了）。

**這是遮蔽症狀，不是修好。** 資料庫裡的空列還在，任何其他 client（App、後台、日後的匯出）都會再看到它們。後端停止產生並清乾淨之後，前端這段過濾應該移除。

## 需要後端確認的事

1. **這些列是怎麼產生的？** 前端這側，對話是在**第一輪 turn 的回應**裡才拿到 `conversationId`（`useProcurementSession` 收到後才把 id 寫進網址），前端沒有任何「先建立空對話」的呼叫。所以空列應該來自後端某條路徑，可能是：
   - turn 開始時先寫 conversation row，assistant 回覆失敗／中斷就留下空殼
   - B-244 的 backfill（前端註解已記錄「backfill 出來的舊對話 title 是空字串」）順帶建了沒有訊息的列
   - 其他我看不到的路徑

   以上都是推測，請以後端實際程式碼為準，不要直接採信。

2. **能不能不要產生？** 沒有任何訊息的 conversation 不應該進列表端點的結果；理想上根本不該落地。

3. **既有的空列要清掉。** 需要一次清理（軟刪或實刪由後端決定），範圍是「沒有任何 message 且 title 為空」的 conversation。

## 驗收條件

1. `GET /app/buyers/me/procurement/conversations` 不再回傳「沒有任何訊息且無標題」的對話
2. 中斷／失敗的 turn 不會留下空 conversation（請補一個「turn 失敗」的測試蓋到）
3. 既有空列已清理完成，且清理不影響有訊息的對話（含只有買家訊息、assistant 尚未回覆的那種）
4. 清理與新規則都不影響 `hasMore` / `nextCursorId` 的分頁正確性
5. 確認後告知前端，好移除 `ConversationSidebar` 的暫時性過濾

## 備註

- **影響頁面**：官網 Web `/account/messages`（買家訊息中心側欄）
- **前端檔案**：`features/messages/components/ConversationSidebar.tsx`（暫時過濾）、`features/procurement-assistant/conversations.ts`（列表契約）
- **關聯**：#2343（B-244 Hawkr Agent 多對話側欄）、B-245（自動標題門檻過嚴／backfill）
- **與 B-245 的差別**：B-245 是「有內容但標題沒產生」，這張是「連內容都沒有」。兩者症狀在 UI 上都顯示為「未命名對話」，但成因與處理方式不同
- **發現日期**：2026-08-13
