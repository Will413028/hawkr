# 店家可曝光以事件傳給 Java，legacy 的店家能力拆成上架與接單

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-05） |
| 日期 | 2026-10-05 |
| 影響範圍 | `internal/modules/seller`（事件與對帳 job）、`internal/legacy`（SellerService 的 `BatchGetSellerMeta`、事件 relay）、`api/proto/seller`、Java product-service、order-service |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 9（9a、9c） |

## 脈絡

Java 判斷店家狀態只有一個布林 `can_accept_orders`（舊 seller-service 推導為「PayUni 已啟用 ∧ 店家營運中」，`services/seller-service/internal/domain/seller.go:70-72`），兩處使用：商品轉 ACTIVE（`ProductService.java` 的 `requireSellerAcceptingOrdersForActivation`，9240）與結帳（`CheckoutService.java` 的 `requireSellerAcceptingOrders`，9241）。買家看得到的商品列表在讀取時只過濾 `seller_deleted_at`（`ProductPublicationPolicy`）；停權靠 product 的 consumer 把商品下架並存快照、解除時還原。

單體的 seller 把店家狀態分成能力（`model.CapabilitiesOf`，停權與等待刪除拿掉經營類）與可公開曝光（`model.Exposable`：營運中、沒有停權、KYC 流程核准（含 PayUni）、店名、logo、鄉鎮代碼；ADR `2026-10-03-public-store-identity-and-visibility.md`）。可曝光不是單調的：PayUni 核准後每次觀測都重算結果（`internal/payuni/observe.go`），停權到期在讀取時推導、沒有寫入。計畫第 83 行原本定「等待刪除的店下架由 Java 的 consumer 依 lifecycle 事件處理」。

約束：
- 繼承：Java 的 catalog、ordering 到 B4、B8 才改寫，`services/java/` 最後整個刪除（主計畫 B4、B8 與收尾）。仍成立：在 Java 裡做完整的讀取模型，B4 時整份丟掉。
- 繼承：B4 已排定「建立 `store_state` 讀取模型，取代停權快照」。仍成立，它需要 seller 的狀態事件餵入，現在沒有人發。
- 上線前不受相容性約束：legacy proto 與 Java 呼叫點可以直接改。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：三件事分開、各由擁有者的當下狀態判斷——上架看店家的營業能力，買家可見性在讀取時由店家狀態推導（讀取模型由狀態事件餵入，event-carried state transfer，Fowler〈What do you mean by "Event-Driven"?〉；Amazon 的 listing suppression、Etsy 的休假模式都不改商品本身），結帳當下查詢收款與接單能力（Stripe Connect 的 `charges_enabled`） | Java 端要有一份可見性狀態 | ✓ 依 Java 的退場調整：seller 端完整做，Java 端只做最小的過濾 |
| 單一布林改成 `Exposable` | 開店前不能先準備商品；錯誤文案寫成停權；可曝光後來失去時已上架的商品照樣可見 | ✗ |
| 契約拆成兩個欄位，可見性留到 B4，等待刪除另加 consumer | B4 之前，PayUni 被撤、從未核准的店的商品出現在列表；consumer 複製快照作法 | ✗ |
| Java 列表在既有的 `batchGetSellerMetas` 之後過濾 | 先分頁後過濾，筆數不準；詳情、搜尋各自要補 | ✗ |
| 在 Java 建完整的 `store_state` 並把所有停權反應改寫 | 等於在要刪的程式裡先做一次 B4 | ✗ |

## 決策

1. seller 發 `StoreExposureChanged{StoreID, Exposable, Version}`：可曝光翻轉時在寫入的 transaction 內發出；時間造成的變化（停權到期）由既有的定期 job（`stamp_first_exposed` 那一支）比對後補發。seller 記下最後公告的值與版本，事件依版本排序，讀取端丟棄舊版本。
2. legacy relay 轉成 `seller.exposure.changed`。Java product-service 的 consumer 記下每家店可不可曝光，`ProductPublicationPolicy` 在讀取時把不可曝光的店的商品當成不公開；product 以快照下架、還原的停權 consumer 與快照欄位刪除。coupon 停用優惠券、order 取消未付款單照舊，它們是對停權的業務反應，不是可見性。
3. legacy 的 `SellerMeta` 拆成兩個欄位：`can_manage_products`（`CapProductManage`）給商品轉 ACTIVE，`can_accept_orders`（`Exposable`）給結帳。賣家可以在店家可曝光之前上架商品，買家看不到。錯誤碼維持 9240、9241：CodeMeta 已凍結（ADR `2026-09-25-codemeta-retirement.md`）、client 依它們分支，依原因區分由 B4、B8 的 Connect `ErrorReason` 取代（Will 2026-10-05）；第 2 項上線後，不可曝光的店的商品買家看不到，9241 只剩購物車裡的舊商品會遇到。
4. 等待刪除不另設 routing key：店家不可曝光，由第 2 項隱藏、第 3 項擋結帳。取代計畫第 83 行的作法。

## 後果

- 單體與 Java 對「買家看得到哪些店的商品」只有一個定義（`Exposable`），落後最多一個 job 週期（時間造成的變化）或一次事件傳遞。
- Java 端的 consumer、可見性欄位與過濾在 B4 刪除；seller 的事件與對帳 job 由 B4 的 `store_state` 直接沿用。
- Java 的讀取模型可以安全重送：寫入依版本，舊版本與重複都不改資料；暫時性錯誤有上限地重試，進死信的訊息可以直接重送。seller 的對帳只比對自己最後公告的值，Java 的表被重建（9d 重寫種子）或訊息遺失時要由 seller 全量重新公告（9d 的一步）。
- 「買家看得到」的條件在 Java 分散在各支公開 SQL 與 Policy：Java 在 B4 刪除，不先集中；B4 的 catalog 只定義一個公開條件，不照抄這些 SQL。
- 結帳的閘仍在 Java，B8 改成 ordering 呼叫 seller 的公開 API（ADR `2026-09-29-seller-suspension-and-business-identity-in-monolith.md` 後果段）。

## 待辦

- 9a：`BatchGetSellerMeta` 的兩個欄位與 Java 兩個閘。
- 9c：事件、對帳 job、relay 的程式（完成）。
- 9d：relay 接線、Java product 的 consumer 與過濾、刪除快照 consumer，與切換同一個 commit（兩份店家資料沒有同步，不讓兩個擁有者並存；Will 2026-10-05）。
- B4：`store_state` 改由 catalog 消費 `StoreExposureChanged`，刪除 Java 端的對應物。
- 未確認：等待刪除的店是否也要停用優惠券。結帳已被擋，現在不做；B7 coupon 改寫時決定。

## 勘誤：脈絡與原始碼不符的兩處（2026-10-05，步驟 9c 開工的盤點，決策不變）

- 脈絡寫「停權到期在讀取時推導、沒有寫入」：處分的效果確實在讀取時推導，但有 `ends_at` 的違規處分另有排在到期時間的 `seller.sanction_expiry` 與每小時的 `seller.reconcile_suspensions`，兩者經 `suspension.Recompute` 發 `StoreUnsuspended`（seller 的 `internal/suspension/jobs.go`）。到期與 job 執行之間的空窗仍只能由對帳 job 補發，決策 1 照舊；`stamp_first_exposed` 現在只掃 `first_exposed_at IS NULL` 的店，改成對帳 job 時候選集合要涵蓋所有未關閉的店與最後公告為可曝光的店。
- 脈絡寫買家看得到的商品列表在讀取時以 `ProductPublicationPolicy` 過濾 `seller_deleted_at`：Policy 只用在單一商品（公開詳情、`toDetailDTO`、brief）；列表、搜尋、各種計數走 SQL，只看 `status`，不經 Policy，也沒有過濾 `seller_deleted_at`。決策 2 的「讀取時當成不公開」因此同時落在 Policy（單一商品）與這些 SQL 的條件（列表與計數，分頁在 SQL 內才準）。
- 買家看得到的 hashtag 數量（自動完成、商品上的 hashtag）原本讀預先維護的 `hashtag.product_count`，靠停權下架時逐筆調整；改成讀取時以同一個公開條件計算，計數欄位與它的維護刪除（product-service V21）。重新評估：B4 catalog 改寫時，或 hashtag 查詢的 p95 超過 200 ms。

## 修訂：契約把可曝光呈現為 `PUBLIC_LISTING` 能力（2026-10-08，Will）

決策不變，內部機制照舊：`model.MissingExposure`／`Exposable` 仍是唯一規則，公告、`StoreExposureChanged`、`seller.exposure.changed` 與 `can_accept_orders` 都不動。變的只有對賣家 client 的契約：`GetMyStore` 不再有 `exposure`，改為 `capabilities` 中的 `PUBLIC_LISTING`（`active` ⇔ 可曝光，`missing` 就是 `MissingExposure`，條件型別是 `SellerRequirement`）。條件的判斷改成互不重疊：等待刪除的店缺 `NOT_PENDING_DELETION`，不再缺 `STORE_ACTIVE`（`STORE_ACTIVE` 只表示店家沒有關閉）；`Exposable` 的結果不變。見 `2026-09-18-seller-capability-authorization.md` 的同日修訂。
