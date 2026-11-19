# review 模組

## 職責

訂單項目的買家評價：提交（訂單資格與快照取自訂單）、店家回覆、平台隱藏與還原、公開與作者與店家的讀取、評價統計。給 httpapi（A6-5）、moderation（A6-4，呼叫 `Hide`／`Restore`）、discovery 與 seller 的店家評分（`StoreRatingStats`）使用。行為規格：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-1-design.md`；A6-1b 只有模組本體，沒有對外入口。

## 擁有的資料

schema `review`（`db/schema.sql`）：`reviews`（`order_item_id` UNIQUE）、`review_images`（最多 4 張）、`hides`（隱藏紀錄，含還原）、`inactive_authors`（刪帳買家的複本）、`media_ref_sync`、`retired_assets`。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與 `Orders` port。
- `internal/write`（提交、回覆、隱藏、還原）、`internal/read`（列表、單筆、統計）、`internal/rows`（列轉對外型別）、`internal/mediaref`（媒體引用同步，照 social）、`internal/text`（伺服器句子目錄）、`internal/model`、`internal/store`（sqlc）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。
- 訂單由 `Orders` port 取（`cmd/hawkr/review.go` 以 `legacy/orderservice` 實作，ordering 搬進單體後換）；`Buyers`（查上傳者帳號）與 `Stores`（取 `store_code`）傳 `*buyer.Module`、`*seller.Module`。

## 不變量

- 公開可讀 = 沒有未還原的隱藏、作者不在 `inactive_authors`；`db/queries/public.sql` 每支查詢以同一段述詞開頭（`testdata/public_predicate.sql`），讀 `review.reviews` 的非公開查詢要列在 `nonPublicReads`（`TestEveryPublicQueryCarriesThePublicPredicate`、`TestNonPublicReviewReadsAreListed`）。新的公開讀取一律放進 `public.sql`。
- 觀看者的封鎖（買家之間任一方向、封鎖店）在 SQL 述詞內、keyset 之前；匿名與有觀看者分兩支查詢；統計不過濾觀看者；店家封鎖買家不影響可見性。
- 一個訂單項目至多一則評價（`reviews_order_item_key`）；`product_id`、`sku_name`、`price`、商品圖只取自訂單項目，`SubmitInput` 沒有這些欄位；評價建立後不變。
- 長度以字元計（上限 1000）；附圖最多 4 張（`review_images_position_check`、`MaxImages`）；超過回 InvalidInput，不截斷。
- 只有訂單狀態 50 能評價，期限 30 天；`Orders` 回 `ErrOrderNotFound` 才是 NotFound，其他錯誤一律 Unavailable。
- `hides` 同一則評價至多一筆未還原（部分唯一索引）；`Hide`、`Restore` 只鎖評價自己的列，新隱藏與還原在同 transaction 發 `ReviewAggregateChanged` 並排快取失效；重複隱藏不發事件。
- 媒體引用只由同步 job 寫；transaction 內只寫業務列、同步版本並排 job，不呼叫 media。
- 本步的 transaction 內不呼叫 provider；`Submit` 撞唯一鍵用 `ON CONFLICT DO NOTHING`，不 catch 23505。

## 不負責

買家名稱與頭像（httpapi 以 `buyer.BatchDisplay` 補）；隱藏的決定與審核（moderation，A6-4）；刪帳訂閱者與 purge、`revalidate.ProductTag`、`reviewrelay`、換掉 `legacy/reviewservice`（A6-1c）；Connect 與 handler（A6-5）；退款後評價的排除（接受到 ordering 搬遷，A6-D7）。

## 決策紀錄

從 review-service 沿用的每個機制、當時的決定與理由，以及與 review-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。
