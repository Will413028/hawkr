// Package rows 把 sqlc 的列轉成對外型別；寫入路徑與讀取路徑共用。
package rows

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
)

// RequestRatingRecompute 在呼叫方的 transaction 內登記商品的評分待重算（D3）：評分由 worker 非同步寫進 catalog，這裡不碰 catalog。
func RequestRatingRecompute(ctx context.Context, q *store.Queries, now time.Time, productIDs ...uuid.UUID) error {
	return q.RequestRatingRecompute(ctx, store.RequestRatingRecomputeParams{ProductIds: productIDs, Now: now})
}

// Review 把評價列、它的附圖與回覆組成對外型別。reply 的內容與時間都在列上（DB 保證兩者同有同無）。
func Review(r store.ReviewReview, images []model.Image) model.Review {
	out := model.Review{
		ID: r.ID, OrderNo: r.OrderNo, OrderItemID: r.OrderItemID, BuyerID: r.BuyerID, StoreID: r.StoreID, ProductID: r.ProductID,
		Rating: int(r.Rating), Images: images, SkuName: r.SkuName, ItemImageKey: r.ItemImageKey, Price: r.Price, CreatedAt: r.CreatedAt,
	}
	if out.Images == nil {
		out.Images = []model.Image{}
	}
	if r.Content != nil {
		out.Content = *r.Content
	}
	if r.ReplyContent != nil && r.RepliedAt != nil {
		out.Reply = &model.Reply{Content: *r.ReplyContent, RepliedAt: *r.RepliedAt}
	}
	return out
}

// Images 依評價 ID 分組附圖（輸入已依評價與位置排序）。
func Images(list []store.ReviewReviewImage) map[uuid.UUID][]model.Image {
	out := make(map[uuid.UUID][]model.Image)
	for _, im := range list {
		out[im.ReviewID] = append(out[im.ReviewID], model.Image{AssetID: im.AssetID, ObjectKey: im.ObjectKey})
	}
	return out
}
