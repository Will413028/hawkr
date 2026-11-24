# B2 付款設計簡報：和業界做法比較、治本的重構方向（2026-11-24，v2）

標記：【實測】本次跑指令看到；【文件】附出處（Stripe 頁面都是本次實際抓取）；【推論】沒有直接證據。Repo：`.worktrees/b2-int`（PR #484，B2 步驟 3a–5）。
v2 依獨立審查 `b2-design-brief-review.md` 修訂：B7 coupon 的事實、限制分成兩列、D-F 改為修訂既有決定（結論改成維持現狀）、D-E 改用 24 小時規則、Stripe 出處改成實際抓取的頁面。

## 結論先講

1. **前提寫錯了。** 計畫的不變量寫「一筆付款只對一家店（結帳依 `storeId` 拆單）」，但結帳根本不拆單：Java `CheckoutValidator.java:65-69` 遇到不同賣家直接回 `DIFFERENT_SELLER`；`confirm()` 只建一張單（`CheckoutService.java:1064,1095`）；proto 註解寫 `order_nos … Phase 1: 1 筆`（`backend/api/proto/order/order.proto:1017`）；website 購物車一次只能選一家店（`website/features/cart/cartSelection.ts:12`）【實測】。所以 step 5 的 `orderNos[0]` 不會漏單，但買家買三家店要結帳三次、付款三次。
2. **業界基準：一次結帳付一次款，依賣家拆單，每張訂單各自轉帳、各自退款。** Stripe SCT（separate charges and transfers，付款與轉帳分開做）的頁面寫「You can split a single charge between multiple transfers」，`transfer_group` 只用來把相關物件串在一起【文件，docs.stripe.com/connect/separate-charges-and-transfers】。Shopee、Etsy、Amazon 也都是一次結帳、依店拆單【推論，沒有逐一核對】。
3. **D-A 建議：** payment 模型現在改成「一筆付款對 N 張訂單應收」。群組鍵是 payment 自己產生的 `payment_id`，轉帳、退款上限、撤回都以訂單應收為單位。在 PR #484 合併前改。Java 過渡期 N=1，對外介面（legacy gRPC、事件）仍以 `order_no` 為鍵，不動。多賣家結帳在 B8 用 Go 做；前提是 B6 cart 和 B3 運費規則，這兩個都還沒開工。
4. **D-F 結論改了（v1 → v2）：** 維持現行「401／403＝結果不明，用同一把鍵重送」。這是 D60 的金鑰輪替設計加上 D68 的三分結果。只加告警。v1 主張「401 一定沒執行」，但 Stripe 文件沒有直接寫，只能算推論；要改分類，前提是 sandbox 先證明。
5. 其他：D-B 在 B8 改成結帳頁內嵌付款，用 deferred intent（先渲染付款表單，按下單時才建 PaymentIntent）；D-C 賣家入帳帳本 RPC、D-D 後台異常結案 UI 與術語表，兩項都要在步驟 6 之前完成；D-E 超過 24 小時不重送，先用強一致的 list 證明「不存在」，否則進 ANOMALY。

## 0. 前提問題：要改的程式是否已排定重寫、搬移或刪除？

| 程式 | 排定的步驟 | 重做量 | 分類 |
|---|---|---|---|
| Java `order/checkout/*`（3,644 行【實測 wc】） | B8 搬進 Go，Java 刪除（主計畫 B8 列） | 現在讓 Java 支援多賣家結帳，B8 會 100% 丟掉 | **SELF-IMPOSED**，否決：程式是我們自己的，做完就丟 |
| 多賣家結帳的 Go 版 | B8 | — | **REAL**，具名前提現在做不到：B6 cart 未開工（主計畫 :381 `[ ]`）、B3 運費規則未開工（:356 `[ ]`）；`internal/modules` 裡沒有 cart、ordering、fulfillment【實測 ls】。B7 coupon 的 Go 模組**已存在**（`internal/modules/coupon`；B7-2／3 完成，見 `2026-11-23-b7-coupon.md`），不是前提【實測】 |
| coupon `Quote/Lock/Redeem/Release`，以 `order_no` 為鍵（`coupon.go:213-228`），`LockInput{OrderNo, StoreID, OrderAmount}` | B7 進行中（B7-4／5） | 現在不必改，見 D-A「群組鍵與分攤」 | — |
| payment 模組（PR 內 `backend/internal` 有 52 個檔案【實測 git diff】） | 不會重寫，是終局程式 | 一個分支內的 migration（壓平）＋ use case 的鍵從 `payment_id` 改成 `payment_order_id`【推論】 | **現在改** |
| `internal/legacy/paymentserver`（gRPC，步驟 4） | B8／B9 刪除 | 以 `order_no` 為鍵，N=1 時對應到唯一的訂單應收，proto 不改【推論】 | REAL（Java 呼叫端活到 B8／B9） |
| 三個 client 的付款頁（PR 共 171 檔、+22k 行【實測】） | B8 重寫結帳 | D-B 要動這裡 | 付款頁在終局仍是重試入口，不會丟掉 |
| B10 | 已退場（主計畫 :391） | 手續費不在範圍內 | — |

## D-A 多賣家購物車的付款粒度（最重要）

- **業界基準**：一次結帳只有一筆 charge（扣款）。每家店一張訂單，訂單完成後各自建一筆 Transfer，帶同一個 `source_transaction` 和同一個 `transfer_group`；退款以訂單為單位，再對那張訂單的 transfer 做 reversal（撤回轉帳）。Stripe 的相關規則：
  - 同一個 charge 可以拆給多筆 transfer，總額不超過 source charge【文件，SCT 頁；抓取時這段被截斷，來自摘要】。
  - 「refunding a charge has no impact on any associated transfers」：退款不會自動撤回轉帳，要逐筆 transfer 撤回【文件，SCT 頁，摘要】。
  - reversal 指定在某一筆 transfer 上，可以分多次部分撤回，直到全部撤回為止；部分撤回只限轉給 Stripe 帳戶的 transfer，recipient 帳戶符合【文件，docs.stripe.com/api/transfer_reversals/create】。
- **現有機制從哪裡來**：不變量表（計畫第 32 行）和範圍裡的「不做多賣家合併付款」，都是 PayUni 時期的設定：PayUni 一筆付款只對一個商店代號（第 20 行）。SCT 下這個限制已經不存在，計畫第 20 行自己也寫了【文件】。schema 裡的 `payments_order UNIQUE(order_no)`、`transfers_active_payment UNIQUE(payment_id)`、`refunds_request(payment_id, …)`（`schema.sql:138,214,294`）都是一付款一訂單的假設【實測】。
- **選項**
  - **A. 一筆付款對 N 張訂單應收（建議）**。`payments`（群組，記總額，掛 `payment_attempts`）＋新表 `payment_orders(payment_id, order_no UNIQUE, store_id, amount)`；`transfers`、`refunds`、`refund_reversals` 改以 `payment_order_id` 為鍵。唯一索引改成「一張訂單最多一筆有效轉帳」；退款上限＝訂單應收金額−該訂單非失敗退款的總和；鎖順序是付款列→訂單應收列。D11（先退款再收回、補差額）和 D5（自動退款）的範圍從付款縮到訂單應收。失敗條件：產品永遠不做多店結帳，那 `payment_orders` 永遠只有一列，等於多一層沒用的間接（成本是一張表）。
  - **B. 維持一張訂單一筆付款，多店就付 N 次**。失敗條件：只要產品支援多店結帳就會出問題，買家要做 N 次 3DS 驗證或錢包確認，中途失敗就只付了一部分【推論】。
  - **C. 一張訂單一個 PaymentIntent，由 client 依序串起來**。失敗條件同 B，而且多了跨 PaymentIntent 的部分成功狀態。
- **群組鍵與分攤（審查第 7 點）**
  - 群組鍵：B8 之前由 payment 擁有。Java 不產生結帳群組 ID，所以 `CreatePayment(order_no)` 在 payment 內建一個群組（`payment_id`）掛 1 筆訂單應收。B8 改成 ordering 傳入 N 張訂單（或 checkout ID），payment 仍產生 `payment_id`。
  - N=1 時的對應：`GrantPayment(order_no)`、`CreateRefund(order_no)`（AutoRefund 和售後都走它）、`GetActivePaymentStatusByOrderNo` 等 legacy RPC 都以 `order_no` 找到唯一的訂單應收，proto 不改【推論】。
  - 每張單的金額以訂單方為準：現在經 `GetOrderForPayment` 讀 Java 的應付金額（計畫步驟 4 表），B8 換成 in-process。payment 不自己算。
  - 優惠分攤由 ordering 負責：賣家券天生只屬於一張單（`campaigns_seller_scope_check`，`coupon/db/schema.sql:60`）【實測】。平台券（`issuer_type='ADMIN'`）跨店時，要由 ordering 依比例把折扣分攤到各張單，分攤後的應付金額就是 payment 的「訂單應收」，也就是退款上限。Shopee 的平台券是按比例分攤到各店訂單【推論】。
  - coupon 介面：只要平台券仍限「一次結帳只套在一張單」，現在的 `Lock(order_no)` 就能用；要讓平台券跨多張單，就要改成鎖在結帳群組上再分攤。這是 B8 的產品決定，不需要在 B7 先改（改了也沒有呼叫者）。
- **可能推翻 D-A 的 Stripe 限制（審查第 6 點）**：(1) 一個 charge 的退款跨多筆 transfer：退款本來就不碰 transfer，逐筆撤回正好就是 A 的模型，不推翻【文件】。(2) 部分退款之後，`source_transaction` 的轉帳總額上限會不會跟著下降：文件沒寫【文件，未載明】。A 的轉帳金額在送出時已經扣掉該單的退款（D69），所以加總≤charge−已退款，兩種解讀都成立【推論】；要在 sandbox 驗一次（3d 的樣本）。
- **推翻建議的事實**：Will 決定產品永遠一次結帳只能一家店；或 sandbox 發現多筆 `source_transaction` 轉帳加上部分退款時會被拒。
- **可逆性與成本**：合併前改，就是分支內壓平 migration（單體規則 6 允許）【文件，backend/AGENTS.md】；合併後再改，要多一個往前的 migration 加第二輪審查；拖到 B8 才改，transfer、refund、reversal 的鍵全部要搬【推論】。
- **限制帳本**：PayUni 一付款一商店：已退場，不是限制。Java 只建單店訂單：讓 Java 改成多店是 SELF-IMPOSED，否決；Go 版要等 B6、B3，是 REAL。recipient 帳戶只能收 transfer：REAL，SCT 符合【文件】。幣別：單一幣別 USD，訂單整數照搬成 cent（D9／ADR D67），不是 TWD；多張單同一幣別，沒有換匯問題【文件，計畫 D9】。手續費：B10 已退場，不在範圍。部分請款（partial capture）：用的是自動請款，不適用【推論】。
- **終局與觸發**：本 PR 改成 1→N；B8 的 Go 結帳拿掉 `DIFFERENT_SELLER`，`CreatePayment` 涵蓋 N 張單。觸發條件：B8 開工，而且 B6、B3 已經完成。

## D-B 買家結帳與付款的 UX

- **業界基準**：結帳頁直接內嵌付款。Stripe 的 deferred intent：「render the Payment Element prior to creating a PaymentIntent」，客人送出後才在伺服器建 PaymentIntent【文件，docs.stripe.com/payments/accept-a-payment-deferred】。
- **現況**：確認訂單（舊 REST）之後跳到 `/orders/{orderNo}` 付款（`CheckoutClient.tsx:781-801`）；付款方式選擇已經拿掉，GA4 的 `payment_type` 寫死成 `"stripe"`（:791-795）；零元訂單直接進成功頁（:798-800）【實測】。
- **選項**：A 維持兩段式；B 在 B8 改成內嵌 deferred intent，付款頁只留給「付款失敗重試」和「待付款訂單去付款」。
- **建議 B，放在 B8**：要在同一次請求裡建單又建 PaymentIntent，得等結帳 API 搬進 Go（REAL：Java `confirm` 活到 B8）。現在的付款頁在終局仍是重試入口，不會丟掉。失敗條件：Go 結帳沒辦法在請求內同步建 PaymentIntent。GA4 照舊：`add_payment_info` 在送出前觸發，`purchase` 依 `GetPayment` 觸發。可逆性高。

## D-C 賣家收入檢視

- **業界基準**：賣家看到的是一份帳本：入帳（transfer）、退款扣回（reversal）、補差額、撥款到銀行（payout）。Shopee「我的收入」、Etsy Payment account 都是這種形式【推論】。
- **現況**：seller-app 的 `fetchPaymentHistory` 還在打舊 REST（`order_repository.dart:110`、`shared_order_detail_screen.dart:1051`）【實測】。D8 決定刪掉它，但新契約沒有替代的 RPC，步驟 6 刪舊服務後這個畫面會壞【推論】。
- **建議**：新增賣家用的「這張單的入帳／本店轉帳列表」RPC，讀 `transfers` 和 `refund_reversals`（D-A 之後以訂單應收為鍵）；seller-app 改用它，刪掉付款歷程。要在步驟 6 之前做完。失敗條件：Will 決定賣家只看 Stripe 的 dashboard（recipient 帳戶能不能用，【未核對】）。

## D-D 後台異常結案 UI 與用語

- **現況**：`ResolvePaymentAnomaly` 已經有 API（`api/hawkr/admin/payment/v1alpha1/payment.proto:58-60`），admin 端沒有呼叫的頁面（`rg ResolvePaymentAnomaly admin` 沒有結果）；術語表（`docs/共通規範/使用者面用語術語表.md`）沒有「撥款、轉帳、收回、補差額」，但 admin 的 `anomalies/page.tsx`、`PaymentAccountCard.tsx` 已經在用這些詞【實測】。
- **業界基準**：一個待處理異常的佇列，每筆核對證據後結案；用詞對齊 Stripe 的 Transfer、Reversal、Refund、Payout【推論】。ADR D68 本身就預設人工結案【文件】。
- **建議**：步驟 6 之前補上結案頁；術語表加一節：轉帳＝transfer、收回＝reversal、補差額＝top-up transfer、撥款＝payout。不做的話，ANOMALY 會永遠佔著唯一索引，只能直接改 DB。

## D-E 結果不明時的查回

- **現況**：`FindPaymentIntentByAttempt` 從新到舊翻頁，最多 50 頁，比對 `metadata.attempt_id`（`internal/platform/stripe/payment.go:19,123-150`）【實測】。
- **Stripe 規則**：24 小時內用同一把鍵、同樣參數重送；超過 24 小時，「don't retry with either the old key or a new one」，要先查物件、Dashboard、webhook，查不出來就找 Stripe 支援【文件，docs.stripe.com/error-low-level】。冪等鍵至少保留 24 小時【文件，docs.stripe.com/api/idempotent_requests】。Search API「don't use search for read-after-write」，平常 1 分鐘內可查到，list API 沒有這個延遲【文件，docs.stripe.com/search】，所以 Search 不能用來證明「不存在」。List PaymentIntents 有 `created` 篩選；抓到的內容沒有列出 `gte`、`lte` 子參數名稱【文件，docs.stripe.com/api/payment_intents/list；子參數待核】。
- **建議**：24 小時內只用同一把鍵重送。超過 24 小時不重送；先用強一致的 list（帶 `created` 下限，下限取我們呼叫前記下的時間減去時鐘誤差，port 加 `createdAfter`）證明不存在，證明了才換新鍵重建，否則進 ANOMALY。50 頁上限保留當保險。這應該和 D10／ADR D68 的「換新鍵前先強一致查回」一致，只是補上時間窗和下限【推論，待核對 §11 的實作】。失敗條件：記下的時間晚於 Stripe 的建立時間，要用測試守住「呼叫前先記時間」。

## D-F 錯誤分類：401／403（修訂既有決定，結論改成維持）

- **既有決定與出處**：`APIError.Permanent()` 刻意把 401／403 當成非終局，理由是金鑰填錯、輪替或權限被調整屬於設定問題，修好就能成功，清理的 7 天期限（D60）就是為了涵蓋這種情況（`internal/platform/stripe/client.go:173-184`）【實測】。payment 的實際分類 `classifyMoney`：429 和 `balance_insufficient` 歸可重試拒絕（呼叫方換新鍵，`payment.go:422`）；其他 4xx 歸終局；401／403、5xx、逾時歸「不明」（`internal/modules/payment/internal/charge/stripe.go:41-56`）【實測】。「不明」依 D10／D68 處理：同一把鍵重送，再查回，最後進 ANOMALY。
- **v1 的提案和它的問題**：v1 想把 401／403 改成「確定沒執行」。Stripe 文件只寫了「參數驗證失敗或撞到並行請求」時不保存冪等結果，沒有提到認證失敗【文件，idempotent_requests】。所以「401 沒執行」只是推論。
- **選項**：A 維持「不明」，同一把鍵重送：金鑰修好後重送就會成功，就算第一次其實執行了也安全，只有金鑰壞超過 24 小時才會進 ANOMALY。B 改成「確定沒執行」換新鍵：好處是少走一次查回；推論如果錯了，就可能重複搬錢。
- **建議 A，加告警**：401／403 發設定錯誤告警（金鑰或權限壞了，需要人修）。B 的前提是 sandbox 實測：用無效金鑰或權限不足的 restricted key 送帶冪等鍵的請求，修好金鑰後用同一把鍵重送，確認沒有回傳先前的結果。即使驗證通過，B 的好處也很小。推翻條件：Stripe 文件或 sandbox 證明 401／403 一定在執行前就失敗，而且金鑰壞超過 24 小時的情況常見到值得處理。

## D-G 較小的項目

| 項目 | 業界或文件 | 建議 |
|---|---|---|
| `payment.expired` 換成 `payment.closed` | 一個關閉事件帶原因（取消或逾期）【推論】 | 保留 closed 並帶 reason；relay 對 Java 的對應寫進 CARRYOVER |
| `GrantPayment` 重試時回 FailedPrecondition | 冪等 API 對重複請求應回同一個成功結果（Stripe 冪等的語義【文件，idempotent_requests】） | 已經有有效轉帳時回成功，否則 Java worker 會一直重試或誤判失敗【推論，未逐行核對】 |
| 建立中的嘗試遇到關閉時，放棄最後一次嘗試 | 應等建立收斂後再取消【推論】 | 排 job 收斂後再關，避免孤兒 PaymentIntent；晚到的成功目前只能靠 D5 自動退款補救 |
| Apple Pay 網域 | 每個顯示 Apple Pay 的網域都要經 `POST /v1/payment_method_domains` 註冊；SCT 用平台金鑰、不帶 `Stripe-Account`；上線前要註冊 live 網域【文件，docs.stripe.com/payments/payment-methods/pmd-registration】。`website/public` 只有 `apple-app-site-association`（universal links 用）【實測】 | 列進「需要人做的事」，用平台金鑰註冊網域 |
| 公開金鑰從 env 經 `PaymentSession.publishable_key` 下發 | 公開金鑰本來就可以公開；由伺服器下發可確保和 secret key 屬於同一個帳戶 | 維持 |

## 摘要表

| 決定 | 建議 | 改動範圍 | 擋住哪一步 |
|---|---|---|---|
| D-A 付款粒度 | 付款 1→N 訂單應收；群組鍵 `payment_id` 由 payment 產生；轉帳、退款、撤回以訂單為單位；N=1 時 legacy 介面不變；分攤由 ordering 負責 | payment schema（分支內壓平）、use case、admin 查詢 | 步驟 4 收尾、6；B8 啟用 N>1（等 B6、B3） |
| D-B 結帳 UX | B8 內嵌 deferred intent；付款頁留作重試入口 | B8 結帳 API、三端結帳頁 | B8 |
| D-C 賣家收入 | 賣家入帳帳本 RPC，seller-app 改用 | api、payment 讀取、seller-app | 步驟 6 |
| D-D 異常 UI／術語 | admin 結案頁＋術語表金流一節 | admin、docs | 步驟 6 |
| D-E 查回 | 24h 內同一把鍵重送；超過 24h 先用 list＋`created` 下限證明不存在，否則 ANOMALY | port、adapter、job | 步驟 7 之前 |
| D-F 401／403 | 維持「不明」（D60、D68），加告警；sandbox 證明後才考慮改 | 告警 | 無 |
| D-G | closed＋reason；GrantPayment 冪等成功；關閉等收斂；Apple Pay 網域註冊 | 小改 | 步驟 4／6 |

## 不留技術債的排序

1. **在 PR #484 內重做（合併前）**：D-A 資料模型、D-E 的 24 小時規則與 `created` 下限、D-G 的 `GrantPayment` 冪等與關閉收斂、D-F 告警。這些都是終局程式，合併後再改就要多做往前的 migration 和第二輪審查。
2. **步驟 6 之前（同一個 PR 或步驟 5b）**：D-C 賣家帳本、D-D 異常結案 UI 與術語表。不做，刪舊服務後就是壞掉的畫面。
3. **計畫修正**：不變量第 32 行改成「一筆付款對 1..N 張訂單；Java 過渡期只送 1 張」，更正「依店拆單」；範圍的「不做多賣家合併付款」改成「B8 做（前提 B6、B3）」；B8 列加上多店結帳、平台券分攤、deferred intent 結帳頁；3d 的 sandbox 樣本加上「部分退款後的多筆 `source_transaction` 轉帳」。
4. **B8**：Go 版結帳拿掉 `DIFFERENT_SELLER`，購物車改成可以多店選取，`CreatePayment` 涵蓋 N 張單，平台券分攤，結帳頁內嵌付款。

**只有 Will 能回答**：(1) 產品要不要做多店一次結帳？要做，D-A 現在改；永遠不做，D-A 就不成立。(2) 平台券要不要跨店分攤？這會決定 B8 時 coupon 要不要改成鎖在結帳群組上。(3) 賣家要不要直接看 Stripe dashboard？會影響 D-C。
