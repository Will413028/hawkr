// Package rows 把 sqlc 的列轉成對外型別；寫入路徑與讀取路徑共用。
package rows

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
)

// SyncProductRating 重算商品的公開評價平均與則數（與 ProductStats 同一份公開判定），在呼叫方的 transaction 內寫回 catalog。
func SyncProductRating(ctx context.Context, q *store.Queries, cat model.Catalog, productID uuid.UUID) error {
	st, err := q.ProductStats(ctx, productID)
	if err != nil {
		return err
	}
	return cat.SetRating(ctx, productID, st.AvgRating, int32(st.TotalCount)) //nolint:gosec // 單一商品的評價則數不會超過 int32 上限
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
