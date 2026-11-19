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
