# GA4 跨平台分析：蒐集面與 property 結構

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-07-12 |
| 影響範圍 | `hawkr-website/lib/analytics`、`hawkr_buyer`／`hawkr_seller` 的 analytics 層、`services/java/order-service`（MP backstop） |
| 對應規格 | `docs/詳細設計書/47-GA4 分析追蹤（GA4 Cross-Platform Analytics）詳細設計書.md` |

> 本檔保存 `WHY THIS DESIGN` / `WHY NOT THE ALTERNATIVES`。
> 事件目錄、參數契約、backstop 的逐欄規格一律在 47 號詳細設計書，本檔不複製。
>
> website repo 另有一份更早的 ADR 記錄同一組決策的 web 側脈絡：
> `hawkr-website/decisions/2026-06-07-ga4-client-side-server-mp-deferred.md`。

---

## 脈絡

三種 client（web、buyer App、seller App）要把使用者行為送進同一組 GA4 資源，
而 GA4 的資源結構本身帶著兩個不可繞過的約束：

1. **Firebase 專案與 GA4 property 是 1:1** ——
   buyer 與 seller App 共用同一個 Firebase 專案，就無法各自擁有 property。
2. **GA4 的跨來源自動去重不可靠** ——
   同一筆 `purchase` 若 client 與 server 各送一次，不能假設 GA4 會自己合併。

---

## ADR-01 蒐集面：client-side 先行，server-side MP 延後

**不採用**：① 純 server-side Measurement Protocol ② 一開始就雙送。

**決策**：三端先做 client-side 埋點；server-side MP 縮成一個**只補特定付款方式**的 backstop，延後實作。

理由是「先有資料 > 等完美歸因」：純 server-side 拿不到頁面層行為（瀏覽、加購物車、篩選），
而那些正是漏斗分析的主體。

## ADR-02 Consent：不送 denied 指令，但留 hook

**不採用**：① 主動送 `denied` ② 連 hook 都不留。

台灣站現階段無 GDPR 義務，主動送 denied 只會讓資料憑空少一塊。
但保留 `setConsent()` hook 的成本近乎零，未來要開就只是換一個呼叫參數。

## ADR-03 purchase 計法：一個 orderNo 一筆 ＋ client 端去重

**不採用**：① 多筆合併成一筆 ② 靠後端去重。

`transaction_id = orderNo` 讓每筆訂單在 GA4 有穩定且唯一的身分，
client 端自己以 `sessionStorage` 去重擋住重複觸發。
這同時替未來的 backstop 鋪路：同一個 `transaction_id`，GA4 client 端的行為可預期。

## ADR-04 Mobile property 結構：buyer 與 seller 共用一個 property

**不採用**：buyer、seller 各自一個 property。

prod 的 buyer 與 seller App 共用同一個 Firebase 專案，而 Firebase↔GA4 property 是 1:1，
技術上拆不開。改以 `role` user property 在同一個 property 內分流。

## ADR-05 Backstop 範圍：只補 ATM 與零元訂單

**不採用**：所有付款方式都由後端雙送。

client 送即時付清的（信用卡／Apple Pay／Google Pay），backstop 只送 ATM 與零元——
兩者**互斥分流**，因此完全不需要跨來源 dedup。
這是被 GA4 去重不可靠這個約束逼出來的形狀。

## ADR-06 Backstop 歸因：帶 `app_instance_id` ＋ platform

**不採用**：不歸因（unattributed）。

不歸因的事件在 GA4 裡成為孤兒 session，營收數字對得起來但漏斗對不起來。
改由 buyer App 在結帳時送 Firebase app instance id、後端存進 order，
MP 送出時用對應平台的 `firebase_app_id`。代價是結帳請求要多帶一個欄位。

## ADR-07 Backstop 掛載點：獨立的 `order.paid` consumer

**不採用**：直接掛在 PayUni webhook 處理流程裡。

獨立 consumer（`analytics.order.paid` queue）讓 analytics 的失敗完全隔離在訂單流程之外，
並且免費拿到 MQ 的 retry 與 DLQ。掛在 webhook 上則是把一個 best-effort 的分析動作
放進金流關鍵路徑。

## ADR-08 Web property：直接在 GA4 Console 建，不從 Firebase 端建

**不採用**：比照 App 那條路從 Firebase 端 enable。

從 Firebase 端建會踩到 Firebase↔property 1:1 限制，
把 web 意外綁進 buyer/seller 共用的 mobile property。
代價是 web 與 mobile 分屬兩個 property，無法在單一 GA4 View 內看完整跨平台旅程。

---

## 後果

- web 與 mobile 資料分屬兩個 property：跨平台漏斗只能各看各的。
  要合併必須重新評估歷史資料斷點成本。
- backstop 未實作期間，ATM 與零元訂單的 `purchase` 不會進 GA4（ADR-05 的分流是互斥的，
  沒有人補那一半）。
- 什麼條件下該重新評估 ADR-01：ATM／零元訂單占比高到營收缺口有人在意時。
