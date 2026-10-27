// Package buyer 提供買家模組公開 API。
package buyer

import "github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"

// FavoriteAdded 帶 UUIDv7、OccurredAt 與業務 ID 的 FavoriteAdded 事件。
type FavoriteAdded = model.FavoriteAdded

// FavoriteRemoved 帶 UUIDv7、OccurredAt 與業務 ID 的 FavoriteRemoved 事件。
type FavoriteRemoved = model.FavoriteRemoved

// SellerFollowed 帶 UUIDv7、OccurredAt 與業務 ID 的 SellerFollowed 事件。
type SellerFollowed = model.SellerFollowed

// ProductViewed 帶 UUIDv7、OccurredAt 與業務 ID 的 ProductViewed 事件。
type ProductViewed = model.ProductViewed

// RestockTargeted 帶 UUIDv7、OccurredAt 與業務 ID 的 RestockTargeted 事件。
type RestockTargeted = model.RestockTargeted

// FanOutInput 是一批補貨通知的輸入（上游事件的商品、顯示快照與發生時間）。
type FanOutInput = model.FanOutInput

// BuyerDeleted 帶 UUIDv7、OccurredAt 與業務 ID 的 BuyerDeleted 事件。
type BuyerDeleted = model.BuyerDeleted

// BuyerDeletionRequested 買家進入刪除中（刪帳申請，不能撤回）時，在同一個 transaction 發布一次。
type BuyerDeletionRequested = model.BuyerDeletionRequested
