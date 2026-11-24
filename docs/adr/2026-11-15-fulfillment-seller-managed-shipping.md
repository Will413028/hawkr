# 出貨以賣家自寄為主線，物流商只留 port 與模擬 adapter

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-15（Will 決定） |
| 影響範圍 | 單體計畫 B3 fulfillment（Java shipping-service 改寫）、`payuni-webhook-service` 的物流部分、website、兩支 App 的出貨與結帳流程 |
| 執行計畫 | `backend/docs/plans/2026-09-24-go-modular-monolith.md` 的 B3 |

## 脈絡

公司時期的出貨由平台串 PayUni 物流（超商、宅配建單、列印、貨態 webhook），每個賣家帶 PayUni 子商店的 MerID。收款改成 Stripe Connect 測試模式後（`docs/adr/2026-11-13-payment-onboarding-provider-port-stripe.md` D1），單體計畫原本預定 B3 把物流商改成綠界。B3 開工前查了物流商的條件。

約束：
- 外部：台灣物流商的多賣家模式都要簽約。綠界物流的 `PlatformID` 只給「專案合作的平台商」，要簽約開通，物流測試資訊也沒有平台商測試編號（<https://developers.ecpay.com.tw/7398/>、<https://developers.ecpay.com.tw/7418/>）；綠界公開的測試帳號只有單一商店（B2C 2000132、C2C 2000933）。PayUni 代理商 API 要簽約，Will 2026-10-29 決定不簽。
- 外部：國外的多賣家出貨 API 也不是自助開通。Shippo 的 Platform Account 要由 Shippo 人工把帳號轉成平台帳號（<https://docs.goshippo.com/platform-accounts/platform-upgrade-account.md>）；EasyPost 建子帳號的 Users API 只能用正式金鑰（<https://docs.easypost.com/docs/users>）。兩家都不支援台灣超商。
- 外部：系統從未上線（根目錄 `AGENTS.md`「上線前不受相容性約束」）。
- 繼承：「物流改綠界」是 2026-10-24 作品集公開時的前提，當時沒查多賣家條件；是我們自己的決定，可推翻。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：marketplace 由賣家自寄並回填物流業者與追蹤號碼，平台代買標籤是加值功能（Etsy、eBay 的做法；本專案設計書 69「商家自有出貨」已定義 `fulfillment_mode = SELLER_MANAGED`） | 沒有超商取貨與門市地圖；運費不能即時報價 | 採用 |
| 綠界單一帳號：所有物流單掛平台的測試商店，port 留每個賣家的外部帳戶 ID | 示範的是「平台替所有賣家寄」，不是多賣家；上線要換成平台商綁定，模擬的那條路不會被用到 | 否決 |
| 國外出貨 API（Shippo Platform、EasyPost 子帳號） | 要人工開通或綁付款；仍沒有台灣情境，台灣賣家地址能否出標未知 | 否決 |
| 簽綠界平台商或 PayUni 代理商 | 作品集要簽物流合約 | 否決：和 PayUni 不簽的理由相同 |

## 決策

- **D1** 出貨唯一上線的方式是賣家自寄：賣家回填物流業者與追蹤號碼；出貨狀態由賣家更新或買家確認收貨；逾期未出貨、逾期自動完成、配送異常轉售後由伺服器的狀態機決定。
- **D2** 運費只用賣家自訂的固定規則，不做即時報價。
- **D3** 保留供應商中立的物流 port（建單、取標籤、貨態 webhook 先寫 inbox 並以事件 ID 去重），每個賣家帶外部帳戶 ID。作品集只放模擬 adapter，用來測建單到貨態回傳的流程。真實物流商的 adapter 在決定上線並簽約後才寫。
- **D4** 不搬：PayUni 物流與 `payuni-webhook-service` 的超商、宅配建單與貨態；超商門市電子地圖、追蹤號碼自動查詢、即時運費。

## 後果

- 作品集看不到超商取貨這個台灣電商的主要出貨方式；示範重點是出貨單與訂單的狀態機、模組分界與事件。
- client 依賴超商、宅配建單的介面在 B3 刪除或改成自寄流程；UI 流程改動先取得同意。
- 設計書 65–70 裡假設平台串物流商的段落，B3 開工前盤點並改寫或刪除。
- `internal/legacy/sellerserver` 回給 Java 的物流 `MerId` 隨 Java shipping 退場刪除。
- 模擬 adapter 只證明 port 的形狀，port 是否適合綠界平台商模式要到寫真實 adapter 時才驗證；那時以本 ADR 的修訂記錄 port 的調整。

## 什麼情況要重新檢視

- 決定在台灣上線：簽物流合約，寫真實 adapter，並決定超商取貨是否回到主線。
- 出現能自助開通、測試模式可用的多賣家物流 API。

## 修訂：出貨歸訂單、運費設定歸 seller、物流 port 延後（2026-11-24，Will 決定）

B3 開工前的前提重審（`backend/docs/plans/2026-11-24-b3-fulfillment/premise-review.md` P1–P6）比對了業界做法：Etsy `createReceiptShipment`、eBay `createShippingFulfillment`、Amazon 賣家自配送 `confirmShipment`、Shopee `ship_order` 都把賣家自寄的出貨記在訂單底下（0..N 筆，含承運商與追蹤號碼），訂單狀態由出貨記錄推導；Shopify 的 `FulfillmentOrder` 那一層是為多地點、第三方倉的路由，本專案沒有。獨立出貨模組的必要條件（串多家物流商的建單與貨態）已隨本 ADR 消失，留著它只會重現 Java 訂單與出貨兩份狀態以事件同步的缺陷。

- **D1 修訂**：出貨記錄（0..N 筆，作品集先開放整單一筆）、由它推導的訂單狀態、逾期取消、自動完成、確認收貨與「未收到」回報由 ordering 擁有，在 B8 交付；不建 `fulfillment` 模組，`internal/arch/modules.go` 刪除該模組與指向它的邊。下游（invoice 開票、aftersales、轉帳）由訂單擁有者呼叫或讀訂單，不反向讀出貨狀態。
- **D2 修訂**：運費設定放在 seller 模組，每個賣家一份：固定運費、免運門檻（可空）、處理天數（預設 3 個工作日、可改 1–7）；報價是純函式，cart 預覽與 ordering 結帳共用，結帳把結果快照進訂單。
- **D3 修訂**：物流 port 與模擬 adapter 延後到決定上線並簽約時；webhook inbox 去重的示範由 B2 的 payment 承擔。
- **過渡期**：B3 刪除 Java shipping 與 Java 端所有依賴它的程式（不留 stub、不寫轉接）；B3 到 B8 之間 Java 訂單停在已付款、結帳運費固定 0，訂單完成、轉帳、crm 的完成事件與賣家關店在這段期間不會被觸發（系統未上線）。
- 「後果」段的作品集重點改為：訂單的狀態機（含出貨）在 B8 交付，B3 交付運費設定與舊物流的清理。
- 對外契約不帶「配送方式」：每筆訂單都是賣家自寄，`ShippingMethod` 3001／3002／3003 刪除，3005（RFQ 站外出貨）由 procurement 以自己的欄位表達；B6、B8 的契約不帶配送方式（B3 R2，Will 2026-11-24）。
