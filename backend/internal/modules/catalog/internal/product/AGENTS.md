# catalog/internal/product

商品的寫入與讀取：整體修改、狀態、版本（etag）、建立的冪等、庫存篩選與計數、搜尋與列表；庫存的冪等與設定在 `../stock`。

## 不變量

- SKU 以選項值的組合辨識，每次 SKU 寫入同 transaction 重算 `min_price`／`max_price`（D19）；組合落在選項值內、不重複，沒有軸的商品剛好一個預設 SKU。庫存不在商品寫入裡改（既有組合不動，新組合給初始值）；改既有 SKU 的庫存走 `AdjustStock`（冪等、限定自己店家、不為負）。守門：`TestAdjustOnAnotherStoresSkuOrDeletedSkuIsNotFound`、`TestPriceRangeFollowsSkuChanges`。
- etag 是 `products.content_version`：只在賣家可編輯的內容與狀態改變時加一，庫存、銷量、收藏數、評分不加；`UpdateProduct`、`SetProductStatus` 必帶版本並在鎖住商品列之後比對，不符回 `ErrVersionConflict`。守門：`TestStaleEtagIsRejectedAndStockChangesDoNotMoveIt`。
- 冪等：建立靠 `(store_id, request_id)` 唯一加 `request_hash`，賣家的 `ADJUST`／`SET` 靠 `stock_operations` 的 `(store_id, key)` 加 `request_hash`；同鍵不同內容回 `ErrIdempotencyKeyReused`，同鍵併發靠 `ON CONFLICT DO NOTHING` 後讀回比對。`SetStock` 以條件式 `UPDATE … WHERE stock_qty = expected` 比對預期值。守門：`TestCreateIsIdempotentByRequestID`、`TestConcurrentCreatesWithOneRequestIDMakeOneProduct`、`TestSetStockReplaysOnceAndRejectsADifferentRequest`、`TestConcurrentSetStockWithTheSameExpectedValueLetsOneWin`。
- 低庫存以 SKU 判斷、只算 ACTIVE、門檻是商品的 `low_stock_threshold`（沒設 5），售完另計。守門：`TestLowStockCountIsPerSkuActiveOnlyWithPerProductThreshold`。
- 搜尋與賣家關鍵字用 `searchable_text`（含選項軸名稱與值）的 `ILIKE`（GIN trigram 索引），相關度排序 `word_similarity`；keyset 游標是 (排序鍵, ID)。守門：`TestPublicSearchFindsAndPagesWithoutRepeatOrGap`、`TestSellerQueryHitsAcrossPagesAndMatchesSkuOptions`。
- 指定店家的公開列表空清單是空頁，不是不篩選（`ListPublicProductsOfStores`）。守門：`TestPublicFiltersAndStoreScopedList`。
- 代行寫入（B4-5d，A7-D27）：`Create`、`Update`、`SetStatus` 明確收 `actor *delegatedactor.Actor`（店主傳 nil），成功的寫入在同 transaction 內以 `delegatedactor.Publish` 發一個事件（`ProductCreated`、`ProductUpdated`、目標 ACTIVE／ARCHIVED 的 `ProductPublished`／`ProductUnpublished`，已在目標狀態的是 `*Repeated`）；帶 actor 的事件沒有名為 `delegatedactor.AuditHandler` 的 handler 就失敗關閉，目標 DRAFT 不接受 actor。守門：`TestProductWritesPublishOneGeneralEventEachAndFailClosedWithoutASubscriber`、`TestDelegatedWritesCarryTheActorAndShareTheWriteTransaction`。
- 批次下架（D29）`UnpublishMany`：一個 transaction、商品依 ID 升序鎖列、一個 `ProductsUnpublished`；任一項版本不符整批回 `*BatchVersionConflictError`（列出全部衝突）且不寫入，已 ARCHIVED 的項目是成功、不比對版本、不改版本。守門：`TestUnpublishProductsIsAllOrNothing`。

