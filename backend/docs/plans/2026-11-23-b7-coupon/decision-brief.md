# B7 決策簡報：coupon 從零設計（業界基線 vs 現況）

快照 dev `df21e0d8a`。證據標記：【量】＝讀過 source 或跑過指令；【推】＝推論；【文件】＝公開文件的已知做法，沒在本 repo 實測。

## 前提題：被改的程式是否排定要重寫、搬走或刪除？

- **Java order-service（約 20,021 行，`find backend/services/java/order-service/src/main -name '*.java' | xargs cat | wc -l`【量】）與 aftersales-service**，排定在 B8、B9 改寫。
  - B8 的前置是 B2 payment port、B3 fulfillment、B4 catalog、B6 cart，目前都還沒完成（主計畫 `[ ]`【量】）。所以「B7 期間 ordering 仍在 Java」這個前置條件現在做不到，算 **REAL**（真正的限制）。
  - 把 B7 併進 B8 不可行：會把 B8 拉到 2 萬行以上的範圍，而且它的前置一樣還沒滿足。
- **會被丟掉的轉接程式（B8／B9 時刪除）**：
  - `legacy/couponserver`：只轉接 5 支 RPC。同類的 `sellerserver/server.go` 是 530 行【量】，估計 coupon 的轉接在 300–500 行【推】。
  - `legacy/orderservice` 上的 2 個 port 實作。
  - 合計約占 B7 的 10–15%【推】。
  - 這些是必須付的成本，但**設計上不能讓這些轉接程式決定模組的形狀**：模組的 API 照「同一個 transaction 內呼叫」來設計，轉接層只負責把它包一層。
- **coupon 自己是重寫**：現有的 Java 結構都不是限制。「和現有程式一致」不能拿來當理由（SELF-IMPOSED，自己加的限制）。

## 限制帳

| 限制 | 分類 | 來源 |
|---|---|---|
| B8／B9 之前，結帳與售後在 Java，跨進程呼叫 coupon | REAL：payment 先於 ordering 有明文規定（主計畫第 11 行），B2 還沒完成【量】；B3、B4、B6 是否為前置，是從 B8 條目「同一個 transaction 內鎖購物車、扣庫存」推得【推】 | 主計畫 |
| 依賴方向是 ordering→coupon | 與基線一致（不是偏離基線的限制） | `internal/arch/modules.go:25-26` |
| 系統沒上線，不搬資料、不受相容性約束 | 有利條件 | 根 `AGENTS.md` |
| Java 的資料模型、RPC 形狀、Redis 快取、排程 | SELF-IMPOSED，不沿用 | 盤點 |
| admin 前端沒有 coupon 畫面 | 事實；要不要做是範圍題 | `client-inventory.md` C6【量】 |

## 業界基線（從零設計的參考）

- **兩層模型**：促銷規則（campaign／template：折抵方式、資格、期間、總量），以及歸屬某位使用者的券（wallet voucher）。
  - 蝦皮、淘寶的「領券」模式：領取時就產生買家的券【文件】。
  - Stripe 用 `Coupon` 搭 `PromotionCode`，以 `max_redemptions` 原子計數【文件】。
  - Shopify 的 discount 在下單時套用，用量以訂單計【文件】。
- **折扣是訂單的一部分**：下單時把折抵金額與券的身分快照寫進訂單，並在**同一個 DB transaction** 內把券標成已用（或已鎖給這筆未付款訂單）。訂單取消或逾時未付，在同一個狀態轉換的 transaction 裡把券還回去。
  - 單體的標準做法是不需要 saga，也不需要對帳排程去查訂單狀態。
- **資格只有一份判定**：領券時判一次，套用（結帳）時再用同一個函式判一次。client 只傳「我的券 id」，不傳模板 id。
- **數量上限是原子的**：`UPDATE … SET issued = issued + 1 WHERE issued < limit`，或用 UNIQUE 列計數。不做「先讀、再判斷、再寫」。
- **由系統發的券**（補償、活動）以來源事件的身分當冪等鍵（例如售後單 id，加上 UNIQUE）。
- **到期是推導出來的**：`now() > end_at` 就是過期，讀取時計算。不需要排程去翻狀態欄；排程只做「到期前提醒」這類通知，並走 outbox／River。
- **提前結束活動**：只擋新的領取與套用。已下單的折扣跟著訂單快照，不受影響；已領未用的券，依產品規則作廢或保留到期。

## 逐題

### Q1 券的資料模型（原 D2）

- **建議：錢包模型，加上推導式到期。**
  - `coupon_campaigns`（原 templates）與 `coupon_vouchers`（每位買家持有的券）。
  - 領券、admin 發券、系統補償都只是「建立 voucher」。
  - 狀態只有 `available`／`locked(order)`／`redeemed`／`voided`，到期從 `end_at` 推導。
  - `coupon_claims` 併入 voucher；`coupon_target_accounts` 保留為資格名單。
- **為什麼現在是那樣**：Java 在結帳 reserve 時才建立 instance，領取另存一張表。這是增量長出來的結果，造成 BD3、BD4、BD6、BD18。
- **選項**：
  - A：錢包模型（建議）。失敗條件：產品需要「不用領、輸入代碼就能用」的公開碼。這時改成加一張 promotion code 表，一樣在套用時建立 voucher。
  - B：保留兩張表，只修 bug。失敗條件：每加一條規則，都要在 claim 與 instance 兩處同步。
- **可逆性**：上線前隨時可改，成本是 migration 與測試。
- **什麼事實會推翻建議**：產品要的是代碼型折扣（Shopify 式），而不是錢包型券。

### Q2 結帳與券的介面（原 D1、D3 的根）

- **建議**：
  - 模組 API 設計成在呼叫方的 transaction 內執行：`Lock(ctx, tx, voucherID, orderRef)`、`Redeem(...)`、`Release(...)`，全部冪等。
  - B8 的 ordering 直接在結帳的 transaction 內呼叫它們。
  - B7 期間，`legacy/couponserver` 以自己開的 transaction 包一層給 Java 用（REAL 限制下的轉接，B8 刪除）。
- **B7 期間的釋放**（Will 2026-11-23 選 A，取代原本的推導式釋放）：鎖沒有到期時間，只有持鎖訂單的 Release 或 Redeem 能解除，別單不能奪鎖。
  - 被否決的推導式釋放：鎖過期後被別單（Java 訂單 B，已寫入折扣、不再重驗）奪走，結果不是同一張券核銷兩次，就是 B 付款後沒折扣。
  - 代價：Java 漏送 Release 時券卡在鎖定，由 admin 作廢單張券處理；spike 1 查 Release 有無重試。
  - B8 刪除這段過渡：釋放跟著訂單狀態轉換，在同一個 transaction 裡完成。
- **不需要反查 order 的對帳 job**，也不需要 `OrderStatusReader` port。

- **MEMBER 資格**（在該店有消費紀錄的買家）：
  - 業界做法是讀取方持有投影：ordering 在訂單完成時發事件，coupon 維護 `store_buyer_membership` 投影。這樣不用反向查詢，也不會「order 掛掉時券全部消失」（BD28）。
  - B7 期間，事件由 Java order 的既有 MQ（spike：有沒有訂單完成事件）經 relay 進來；B8 改成同 transaction 的事件。
  - 替代方案：用 port 同步查詢（ordering 實作），比較簡單，但 order 掛掉時領券跟著失敗。
  - 什麼事實會推翻這個建議：spike 2 發現 Java 沒有「訂單完成」事件。這時改用同步 port，到 B8 再換成投影。
  - 可逆性：B8 換事件來源時，投影要從 ordering 的歷史訂單 backfill 一次；系統還沒上線，成本低。
- **可逆性**：介面放在模組內，可改。

- **轉接層對 Java 的對應**（D2 A 的附帶規則）：Java 結帳送的是 `couponTemplateId`。轉接層會把它對到「該買家在該活動下、可用、最早到期」的那張 voucher。
  - 沒領過的券，如果活動不需要領取，就在同一個 transaction 裡當場發一張再鎖定；需要領取的，回「未領取」。
  - 這套對應只存在轉接層，B8 時改成 client 直接傳 voucher id。
  - 替代方案：現在就改成傳 voucher id。範圍是 Java 結帳與兩個 client，含 website、buyer-app 的結帳請求，共 9 處（`client-inventory.md` C10【量】）。
    - B8 時會丟掉的程式：Java 這邊的改動。
    - 這不是真實限制，只是工作量。
    - 建議仍採用「轉接層對應，且不當場發券」：沒領過的券一律回「未領取」，client 要先領券。這樣錢包模型不會被打破；代價是 B8 前結帳有兩個步驟。
- **刪帳、停權等事件**：buyer、seller 已經在單體裡，會在 transaction 內發出事件（`modules/buyer/events.go:25`、`modules/seller/events.go:12,15`【量，審查者】）。
  - 基線做法是 coupon 直接訂閱這些模組事件，同 transaction 處理，不經 RabbitMQ 繞一圈。
  - `buyerrelay`／`sellerrelay` 裡轉給 coupon 的部分隨 Java 刪除。
- **出站依賴**：Account、Buyer、Seller 那 6 支（暱稱、建立時間、身分解析、店家資訊）改為直接呼叫模組 API，不搬 Java 的「失敗就回 -1 或空 map」降級。讀取失敗就回錯誤；顯示資料（店名、logo）與主資料同時讀取，任何一項讀取失敗就整個請求失敗，不做部分降級。

### Q3 補償券（原 D4）

- **建議**：補償是「系統發 voucher」，以 `(source_type, source_id)` 加 UNIQUE 冪等。轉接 RPC 加必填欄位，由 Java aftersales 傳售後單 id，並在同一個變更裡修改 Java。B9 改成同 transaction 呼叫。
- **沒有真正的替代方案**：不加鍵，就無法防止重複發放。
- 什麼事實會推翻這個建議：無。
- 可逆性：加了 UNIQUE 鍵之後很容易再拿掉。

### Q4 提前結束活動（原 D5）

- **建議**：結束只擋新的領取與鎖定。已鎖定的券照常完成或釋放；已領未用的券，**產品題**，兩種都是業界做法：
  - 作廢：蝦皮賣家結束活動時，未用的券失效【文件，未逐條核對】。
  - 保留到期：較保護買家。
- **推薦作廢**：賣家停止活動通常是因為預算或錯誤設定。
- **什麼事實會推翻**：Will 想以買家保護為主。
- **可逆性**：上線前可以換。上線後作廢已領的券幾乎無法補救；保留到期則隨時可以改成作廢。

### Q5 admin 方法沒有畫面（原 D6）

- **前提攻擊**：沒有消費端的方法就是死碼，業界基線（YAGNI，不為還沒有的需求先寫程式）是不做。
- **但營運真的需要的能力**：補償券（售後之外的人工補償）、賣家配額、作廢單張券，有實際用途。
- **建議**：
  - 本階段只做「有消費端或營運必要」的方法，加上 admin 畫面：建立與發布平台活動、指定發放、作廢、配額。
  - admin UI 屬於 UI 改動，需要你同意版面。
  - 若不想擴大範圍，次選是後端與畫面都不做，延後到要做營運券管理時。
  - **不建議**只做後端不做畫面：那等於有方法但沒有人能用。
- **失敗條件**：作品集展示需要平台券流程，卻沒有畫面。
- **可逆性**：延後的話，之後再加成本不變；先做的話，畫面可能隨需求改版。

### Q5b 到期提醒的通知（A8-D13 已決，不再是決定題）

- 現況：`coupon.expiring` 經 MQ 送給 Java notification-service；單體還沒有 notification 模組【量】。
- 已決：A8 計畫 D13（Will 2026-11-23）規定「legacy 通知轉接不做；通知型別隨發送端搬遷」，D4 把 coupon 歸為行銷類通知。
- 基線做法：coupon 在 transaction 內發出 in-process 事件 `VoucherExpiring`，由 notification 模組訂閱，並寫入自己的 outbox。
- 依 D13，先後只剩一個條件：B7-5 刪掉 Java 的那一刻，notification 模組必須已經存在（A8-1、A8-3）。
  - 理由：A8 是我們自己排的進度，屬於自訂限制（SELF-IMPOSED），而且 A8 已經在進行。
  - 所以 B7-5 被 A8-3 擋住，不另寫轉接。
  - B7-0 到 B7-4 可以和 A8 並行。

### Q6 其餘照搬機制的去向（不需拍板，依基線）

- **不搬**：Redis 模板快取（同進程查 DB）、`coupon.issued` 事件（沒有消費者）、`coupon:avail:*`、`usage_limit_per_account`（沒實作）、AutoExpire 排程（改成推導）。
- **改寫**：到期提醒改成 River job，加上 notification intent。
- 時間一律 `timestamptz`／UTC。

## 需要的 spike（B7-0 內做，只讀）

1. Java order 的付款逾時時間 vs 鎖定期限（Q2 的推導式釋放安全嗎？）。
2. Java order 有沒有「訂單完成」或「首次消費」事件可接 MEMBER 投影（Q2）。

## 終態與觸發

- B7 結束：模組 API 已是 B8 要的形狀；Java 經轉接使用它。
- B8 觸發：刪掉 couponserver 的 order 部分（釋放改由訂單轉換在同一個 transaction 內完成），MEMBER 投影改由同 transaction 事件餵入。
- B9 觸發：刪掉補償轉接。
