# social/internal/actor

推導互動者與店家封鎖檢查。

## 不變量

- 買家身分的寫入一律經 `actor.Resolver` 的 `ForWrite`／`ForAuthor`／`ForBuyerWrite` 檢查店家封鎖；`Deps.StoreBlocks` 缺了 `New` 就 panic。守門：`TestNewSocialWiresStoreBlocks`（`cmd/hawkr`）、`TestBuyerBlockingAStoreHidesItsContentAndStopsWrites`。規格：§5。
