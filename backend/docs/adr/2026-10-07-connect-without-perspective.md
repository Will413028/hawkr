# Connect 不使用視角，`X-Active-Role` 只留在舊 REST

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-07） |
| 日期 | 2026-10-07 |
| 影響範圍 | identity（`PerspectiveOf` 移出公開 API）、`internal/httpapi`（session、`rpc.Principal`）、`internal/legacy/gatewayproxy`、`api/` 的 identity 契約、website、admin、`packages/hawkr_api`、兩支 App |
| 取代 | `2026-09-30-account-created-provisions-buyer.md` W5 修訂的 `Account.perspective`；計畫 `2026-09-29-a3-buyer-seller.md` 第 67 行的 `X-Perspective` 改名 |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 10a（T5） |

## 脈絡

W5 之後視角只有兩個用途：入口轉給舊 gateway 時合成 `X-Login-Role`（gateway 的 handler 與 IM 在同一條路由內以它選擇以買家或賣家的身分動作），以及 `GetSession` 把 client 送來的 `X-Active-Role` 依規則回成 `Account.perspective`，website 再以它當工作區的預設——client 送、伺服器回，是繞一圈。沒有任何 Connect 模組以視角授權或改變行為：Connect 的服務名稱（`SellerAppService`、`BuyerAppService`）本身就表示以哪個身分呼叫。業界基準（Etsy、Shopify、Amazon）是買家與賣家走不同的 API 面，「正在看哪個工作區」是 client 的 UI 狀態，不是每個請求的 header。原本計畫把 header 改名成 `X-Perspective`，前提是它是長期契約，不成立。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| header 改名 `X-Perspective`，Connect 繼續讀 | 改名本身要動四端；保留一個沒有 Connect 消費者的每請求 header 與伺服器回傳的繞圈 | ✗ |
| Connect 不使用視角，header 只留在舊 REST | 刪除 `Account.perspective`；website 的預設規則移到 client | 採用（基準） |

## 決策

- 刪除 `Account.perspective`（號碼 reserved）與 `rpc.Principal.Perspective`；Connect transport 不送 `X-Active-Role`；website 工作區的預設（有賣家角色且上次停在賣家工作區才是賣家，否則買家）在 client 決定。
- `X-Active-Role` 只留在舊 REST 與 chat ws-ticket 的送出端；名稱不改，它是舊 wire 的一部分。`PerspectiveOf`（提示是 SELLER 且帳號有賣家角色時是賣家，否則買家；管理員沒有視角）從 identity 的公開 API 移到 `internal/legacy`，`gatewayproxy` 照舊轉成 `X-Login-Role`。

## 後果

退場條件是最後一個讀 loginRole 的 gateway handler 或 IM 消費端退場，屆時 header 與 `internal/legacy` 的推導一起刪除。

## 修訂：服務名稱表示身分只適用於資源不分身分的模組（2026-10-17，Will）

原決策段保留歷史；決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10d-a7-social-brief.md` D1（Will 全照建議）。

脈絡：「服務名稱本身就表示以哪個身分呼叫」寫成時只有 buyer、seller 兩個模組碰到身分。social 的同一個動作（留言）依資源歸屬決定身分：店主在自己的貼文是店家，在別人的貼文是本人；照服務名稱表示身分，client 要自己判斷「這是我的貼文」才知道打哪個 service，等於 client 在決定身分。專案的命名規則其實是「模組＋對象」（`<模組>AppService` 給登入的帳號用，買家賣家都算）。

決策：`BuyerAppService`、`SellerAppService` 的名稱表示身分，只適用於資源本身分買家與賣家的模組（buyer、seller）。資源被兩種身分共用的模組（social；簡報 D1 提到 A6 的聊天是同一個形狀，到 A6 再決定）用 `<模組>AppService`，身分由伺服器依資源歸屬推導，client 不選；以店家身分動作時由擁有資料的模組推導身分，再向 seller 檢查能力。「Connect 沒有視角、client 不送 `X-Active-Role`」不變。
