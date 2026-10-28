# 公開店家的識別與可見性

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-03；同日與 2026-10-07 修訂，見文末） |
| 日期 | 2026-10-03 |
| 影響範圍 | `api/hawkr/seller`（Public service）、`internal/httpapi/seller`、`internal/modules/seller`（以 `store_code` 讀取、批次讀取）、`internal/httpapi/buyer` 與 discovery 的店家卡片、website 與 buyer-app 的店面頁與店家卡片 |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 8（8c） |

## 脈絡

舊的 `GET /public/sellers/{id}` 同時做三件事：店面頁、各處店家卡片的補資料（收藏、追蹤、聊天、訂單、貼文）、把 `store_id` 或 `store_code` 解析成店家。它不分狀態，回傳任何店家，包括已關閉與還沒通過 KYC 的。buyer-app 的收藏與追蹤逐店呼叫它。

7c 決定以「可公開曝光」取代 KYC 的內容前置條件（`model.Exposable`：營運中、沒有停權、KYC 與 PayUni 流程核准、店名＋logo＋所在地齊全）；seller 模組規格（`docs/modules/seller.md` §1.3）寫明已關閉店家的資料怎麼對外呈現由本 ADR 決定。

約束：
- 繼承：`store_id` 在過渡期是 `bigint IDENTITY`（Java 以 BIGINT 保存），連號、可以逐號列舉；最後一個 Java 消費者退場後換成 uuid（ADR `2026-09-25-row-identity-by-data-kind.md` 的過渡規則）。仍成立，這是外部約束。
- 繼承：`store_code` 是每個 stint 鑄一次的亂數（PayUni MerID＝`HAWK`＋code，只能綁一次；ADR `2026-07-11-identity-store-stint-symmetry.md`）。仍成立。
- 繼承：關店滿保留期後清除店名與說明（ADR `2026-10-19-seller-kyc-payuni-consolidated.md`「關店與個資保留」；寫這份 ADR 時保留期是 30 天）。仍成立。
- 不再成立：「兩種鍵都要收」——它是為了公司時期已經發出去的 `store_id` 連結；系統從未上線。

## 考慮過的方案

**識別鍵**：row-identity ADR 已經回答——契約裡的參照是不透明的 `id`，給人看的編號另開 UNIQUE 欄位。URL 因此用 `store_code`（亂數、不可列舉），API 參照用 `id`；不另列方案。

**不可曝光或已關閉的店家**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：探索與店面頁只放公開的店家；訂單、聊天、收藏等「參照卡片」對停業的對方仍顯示名稱並標上狀態（Shopify 開店前以密碼頁不公開；Etsy、Amazon 的訂單在店家停業後仍顯示賣家名） | 卡片要能區分狀態 | 依外部約束調整後採用 |
| 一律只回可曝光的店家，其他 not_found | 店家停權或關閉時，買家的收藏、聊天、訂單上的店名消失 | ✗ |
| 卡片不分狀態全部回傳 | 以連號的 `store_id` 逐號查詢，就能列舉出還沒公開的店名 | ✗（外部約束：`store_id` 在過渡期可列舉） |

## 決策

- `SellerPublicService.GetStore` 以 `store_code` 讀取，只回可公開曝光的店家，其他一律 `not_found`（不區分不存在與不公開）。賣家預覽自己還沒公開的店面讀 App 的 `GetMyStore`。
- `BatchGetStores(ids)` 給卡片用：曾經 KYC 核准的店家回店名、logo 與狀態（營業中、暫停、已關閉）；從沒核准過的不回；個資清除後名稱欄位不設，由 client 顯示「已關閉的店家」。
- 公開讀取不接受 `store_id` 當 URL 的鍵；website 的 `store_id` → `store_code` 轉址在步驟 10 刪除。
- 讀取不附帶寫入：舊 `GetSeller` 內的店家來訪紀錄（`seller_store_visitors`）不搬。

## 後果

- 重新入駐換新的 `store_code`，店面網址跟著變；這和 stint 模型一致（新 stint 是新店家）。
- 「曾經核准」這個過濾只為了連號可列舉存在：`store_id` 換成 uuid 後重新評估，卡片可以改成依基準不論核准與否都顯示。
- 自選的店家網址（handle）是另一個產品功能，上線前做 SEO 規劃時再決定。

## 修訂：卡片以「曾經公開曝光」過濾，契約只有一種店家卡片（2026-10-03，Will 決定）

**脈絡**：原決策寫「曾經 KYC 核准的店家回店名、logo 與狀態」，理由卻是「還沒公開過的店名不能以連號的 `store_id` 列舉」。兩者在 8c 實作時分歧：人工核准之後 PayUni 還要審核，這段期間店家從沒公開過，以人工核准判斷就會讓店名、logo 與 `store_code` 被逐號讀出（8c code-review F1）。同時，店家卡片在契約裡有三種形狀（buyer 的 `StoreCard`、seller 的 `StoreCard`、discovery 的 `RecommendedSeller`），buyer 那份已經和實作不一致：封面永遠不設、`active` 分不出暫停與關閉（8c design-review）。

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：對外 ID 不可猜，卡片不需要可見性過濾（Stripe、Shopify 的不透明 ID） | `store_id` 換成 uuid 要改 Java 的 BIGINT 欄位 | 外部約束擋住，作為終點 |
| 存第一次公開的時間（`published_at` 模式），只回「已公開過或此刻可公開」的店 | 一個欄位、由定期 job 單一寫入；讀取以「此刻可公開」補足 job 的間隔 | 採用 |
| 人工核准加上 PayUni 曾經核准 | 不用 migration；兩者都核准、但缺 logo 或所在地、從沒公開過的店仍可列舉 | ✗ |
| 維持人工核准 | PayUni 審核期間的店名可列舉，這是常態不是邊角 | ✗ |

卡片的形狀：被參照的實體在契約裡只有一種表示（GraphQL 的欄位展開、Stripe 的 `expand[]`、AIP-122 的參照加 BatchGet 都是一種形狀）。`internal/httpapi` 是畫面的讀取組裝層，由伺服器內嵌同一張卡片；清單只回 ID、由 client 再批次讀取會讓手機每頁多一次往返，四個 client 各自合併，否決。

**決策**：
- 店家的發布時間是 seller 保存的事實 `first_exposed_at`：由 seller 的定期 job 在店家第一次可公開曝光時設定一次，不覆寫；可曝光的判斷仍只有 `model.Exposable` 一處；已可曝光、job 還沒設定的店家，發布時間以當下計。所有「開始公開」的用途只讀它（業界的 `published_at`），discovery 的新店加分不再從 KYC 與 PayUni 的核准時間推算。
- 卡片只回已發布、或此刻可公開曝光的店家。原決策的「曾經 KYC 核准」由此取代。
- 店家卡片只有 `hawkr.common.v1alpha1.StoreCard`（跨模組共用的訊息放 common，〈API 設計規範（Connect）〉§一）：`BatchGetStores`、buyer 的清單、discovery 的推薦都內嵌它。狀態是「營業中＝此刻可公開曝光」、暫停、已關閉；封面保留（錢包的會員卡使用）。
- buyer 讀店家改成直接依賴 seller 模組的卡片，刪除為舊 seller-service client 留下的轉接型別。

**後果**：`store_id` 換成 uuid 時（觸發條件：ADR `2026-09-25-row-identity-by-data-kind.md` 的最後一個 Java 消費者改寫），只刪掉卡片的這個過濾，卡片依基準不論是否公開過都回傳；`first_exposed_at` 是發布時間，保留。兩次 job 之間短暫公開又下架的店家不會被記為公開過，漏的方向是不顯示。

## 修訂：店家卡片帶所在地與評分摘要（2026-10-07，Will 決定）

**脈絡**：`StoreCard` 只有 ID、`store_code`、店名、logo、封面與狀態；client 的追蹤、收藏、推薦卡片讀所在地與評分，舊 client 逐卡讀整筆 profile 取得（A3a 第 4 輪 N8），逐卡改呼叫 `GetStore` 是本 ADR 否決的 N+1。業界基準（Etsy、蝦皮）的店家卡片顯示所在地、星等與評論數，由伺服器組裝。評分屬於 review（主計畫 A6），legacy 已有批次讀取 `ListStoreRatingStats`（discovery 的排行在用），所以多出的讀取是既有的。

**決策**：
- `StoreCard` 加 `town_code`（seller 自己的資料；個資清除後不設）與評分摘要（平均分數、評論數），由 `internal/httpapi` 以批次讀取組裝；評分讀不到時不設，不當成 0 分。
- 卡片不帶「已認證」、簡介與得獎：能出現在卡片上的店一定已 KYC 核准，「已認證」沒有資訊量。
- seller 不存評分；review 搬進單體時，評分改讀它的公開 API，契約與 client 不變。需要依評分排序或篩選店家時，由使用端以 review 的事件維護自己的讀取模型，不複製進 seller。
