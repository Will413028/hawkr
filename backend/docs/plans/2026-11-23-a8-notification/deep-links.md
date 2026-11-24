# 通知深連結格式與路徑表（D20）

來源：多語系 ADR `docs/adr/2026-10-17-multi-locale-architecture.md` D1（伺服器產生的連結是 website URL，帶收件人語系前綴）；計畫 D6、D20。路由核對日 2026-11-24，dev `c686bec9d`，讀 `website/app/**`。

## 格式

- `deep_link` 是 website 相對路徑：以 `/` 開頭、沒有 host、沒有 scheme；可帶 query。推播 data 的 `deep_link` 鍵是同一個值。
- 語系前綴：收件人語系是 en 時前綴 `/en`，zh-TW 不加前綴。下表的路徑不含前綴。
- 路徑必須是下表中已存在的 website 路由；伺服器不產生沒有路由的路徑。沒有目的地的型別，`deep_link` 為空字串。
- App 把路徑對應到自己的畫面（universal link／App Links 接受 `/en/…`）；遇到不認得的路徑開收件匣。
- `/sellers/{storeCode}` 的路徑段是店家的 store_code，不是 store id；伺服器必須用店家的 store_code 組這些路徑。
- App 的聊天 banner 抑制從路徑判斷聊天室（D6）：路徑符合下表聊天列時，取最後一段 `{targetId}`。

## 路徑表

| 目的地 | 路徑 | 受眾 | 備註 |
|---|---|---|---|
| 買家訂單詳情 | `/account/orders/{orderNo}` | BUYER | 另有 `/orders/{orderNo}` 結帳後頁面，通知用前者 |
| 買家售後詳情 | `/account/after-sales/{caseNo}` | BUYER | |
| 聊天（買家） | `/account/messages/{targetId}` | BUYER | |
| 聊天（賣家） | `/seller-center/messages/{targetId}` | STORE | `targetId` 是買家 ID（`website/lib/chat/types.ts` 的 `targetUserId` 說明）；買家端是店家 ID |
| 店家論壇串（留言、讚） | `/sellers/{storeCode}/messages/{threadId}` | BUYER、STORE | 公開頁 |
| 貼文 | 無獨立路由 | - | 只有店家貼文列表 `/sellers/{storeCode}/posts`；追蹤的店發文用此列表 |
| 店家頁 | `/sellers/{storeCode}` | BUYER | |
| 商品頁 | `/products/{id}` | BUYER | 補貨通知 |
| 評價 | 買家端：`/sellers/{storeCode}/reviews`；賣家端：無獨立路由 | BUYER、STORE | 賣家中心只有 `preview/reviews`（預覽），不用 |
| 賣家訂單詳情 | `/seller-center/orders/{orderNo}` | STORE | |
| 賣家售後詳情 | 無獨立路由 | - | 賣家中心沒有售後頁 |
| 賣家撥款、錢包 | `/seller-center/wallet` | STORE | 收款帳戶啟用 |
| 賣家店家狀態（停權） | `/seller-center/suspended` | STORE | |
| 賣家 KYC | `/seller/onboarding` | STORE | |
| 店家措施 | `/seller/measures/{measureId}` | STORE | 申訴 `/appeal`、補正 `/remediation` 為其子路徑 |
| 店家案件詢問 | `/seller/inquiries/{inquiryId}` | STORE | |
| 檢舉結果 | `/account/reports/{id}` | BUYER | |
| 採購詢價（RFQ 邀請） | 無賣家端路由 | - | 只有買家採購助理 `/account/messages/assistant/procurement` 與 demo 頁，邀請對象是賣家，不用 |
| 帳號停權／解除 | 無 | - | 停權帳號無可用頁面 |
| 代行完成 | 無 | - | 代行區 `/adm/{storeId}/…` 無語系前綴且只給代行者，通知收件人是店家，不用 |
| 收件匣（含後台公告） | 買家：`/account/notifications`；賣家：`/seller-center/notifications` | BUYER、STORE | 未知路徑與公告的目的地 |
