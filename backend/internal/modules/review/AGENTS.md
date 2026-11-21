# review 模組

## 職責

訂單項目的買家評價：提交（訂單資格與快照取自訂單）、店家回覆、平台隱藏與還原、公開與作者與店家的讀取、評價統計。給 httpapi（A6-5）、moderation（A6-4，呼叫 `Hide`／`Restore`）、discovery 與 seller 的店家評分（`StoreRatingStats`）使用。行為規格：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-1-design.md`；沒有對外入口（A6-5 才加）；`cmd/hawkr` 只接刪帳訂閱者與 worker。

## 擁有的資料

schema `review`（`db/schema.sql`）：`reviews`（`order_item_id` UNIQUE）、`review_images`（最多 4 張）、`hides`（隱藏紀錄，含還原）、`media_ref_sync`、`retired_assets`。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與 `Orders` port。
- `internal/write`（提交、回覆、隱藏、還原）、`internal/read`（列表、單筆、統計）、`internal/rows`（列轉對外型別）、`internal/mediaref`（媒體引用同步，照 social）、`internal/purge`（刪帳的訂閱者邏輯與清除 job）、`internal/text`（伺服器句子目錄）、`internal/model`、`internal/store`（sqlc）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。
- 訂單由 `Orders` port 取（`cmd/hawkr/review.go` 以 `legacy/orderservice` 實作，ordering 搬進單體後換）；`Buyers`（查上傳者帳號）與 `Stores`（取 `store_code`）傳 `*buyer.Module`、`*seller.Module`。

## 不變量

- 公開可讀 = 沒有未還原的隱藏、作者不在 `buyer.pub_inactive_buyers()`（join，不複製狀態）；`db/queries/public.sql` 每支查詢以同一段述詞開頭（`testdata/public_predicate.sql`），讀 `review.reviews` 的非公開查詢要列在 `nonPublicReads`（`TestEveryPublicQueryCarriesThePublicPredicate`、`TestNonPublicReviewReadsAreListed`）。新的公開讀取一律放進 `public.sql`。
- 觀看者的封鎖（買家之間任一方向、封鎖店）在 SQL 述詞內、keyset 之前；匿名與有觀看者分兩支查詢；統計不過濾觀看者；店家封鎖買家不影響可見性。
- 一個訂單項目至多一則評價（`reviews_order_item_key`）；`product_id`、`sku_name`、`price`、商品圖只取自訂單項目，`SubmitInput` 沒有這些欄位；評價建立後不變。
- 長度以字元計（上限 1000）；附圖最多 4 張（`review_images_position_check`、`MaxImages`）；超過回 InvalidInput，不截斷。
- 訂單不是提交者的，回與不存在相同的 NotFound（A6-D10，先於狀態檢查）；只有訂單狀態 50 能評價，期限 30 天；`Orders` 回 `ErrOrderNotFound` 才是 NotFound，其他錯誤一律 Unavailable。
- `hides` 同一則評價至多一筆未還原（部分唯一索引）；`Hide`、`Restore` 只鎖評價自己的列，新隱藏與還原在同 transaction 發 `ReviewAggregateChanged` 並排快取失效；重複隱藏不發事件。
- 店家回覆可被 seller 的內容下架措施（`REVIEW_REPLY`，`content_id` 是評價 ID，A6-D21）下架：review 不存複本，`TakenDownReplyIDs` join `seller.pub_content_takedowns()`，列表與單筆讀取把 `Reply` 設為 nil（店家自己的視圖也一樣），評價照常、統計不變；`OnMeasureCreated`／`OnMeasureRevoked` 同 transaction 依 id 升序鎖評價列並核對評價屬於措施的店家（不符回 `TakedownContentError`），不發事件（回覆不進評分彙總）（`TestReviewReplyTakedownHidesOnlyTheReply`）。
- 媒體引用只由同步 job 寫；transaction 內只寫業務列、同步版本並排 job，不呼叫 media。
- 買家刪帳（A6-D6、A6-D9）：`OnBuyerDeletionRequested`／`OnBuyerDeleted` 同 transaction 只排 `review.purge_author`（`EnqueuePurge`）（以買家為唯一鍵），不刪列；列由 job 依 id 升序分批刪（附圖 CASCADE），每批記退場資產、排媒體同步與 `revalidate`，每批每組 (店, 商品) 發一個 `ReviewAggregateChanged`（A6-D12），可重跑；`review.purge_sweep` 週期掃描補排失敗被丟棄的清除 job（A6-D13）。守門：`TestDeactivationHidesReviewsImmediatelyViaThePubJoin`、`TestRD4PurgeDeletesRowsAndImagesAndSyncsMediaAndIsRerunnable`、`TestDeletionLocksBuyerAndSellerBeforeReviewRows`（accountdeletion）。
- `Submit` 的 insert transaction 第一句 `Buyers.LockActive`（買家列 `FOR KEY SHARE`，與買家轉移的 `FOR UPDATE` 互斥），NotFound 與 FailedPrecondition 都回 `ReviewAuthorInactive`；清除 job 的唯一性不含執行中的狀態。守門：`TestSubmitIsRejectedAfterDeletionRequestAndWaitsForTheGuard`。
- 本步的 transaction 內不呼叫 provider；`Submit` 撞唯一鍵用 `ON CONFLICT DO NOTHING`，不 catch 23505。

## 不負責

買家名稱與頭像（httpapi 以 `buyer.BatchDisplay` 補）；隱藏的決定與審核（moderation，A6-4）；換掉 `legacy/reviewservice`（A6-6e，A6-D11）；`legacy/reviewrelay` 只在測試啟用，A6-6e 才接進 `cmd/hawkr`；Connect 與 handler（A6-5）；退款後評價的排除（接受到 ordering 搬遷，A6-D7）。

## 決策紀錄

從 review-service 沿用的每個機制、當時的決定與理由，以及與 review-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。
