import type { Review } from "@/lib/reviews/repo";
import type { SellerReviewItem } from "./ReviewList";

/** 評價資料層的 {@link Review} 整理成店家評價列表元件讀的欄位。 */
export function sellerReviewItem(review: Review): SellerReviewItem {
	return {
		id: review.id,
		author: review.username ?? "匿名",
		avatar: review.avatarUrl,
		rating: review.rating,
		createdAt: review.createdAt,
		content: review.content,
		images: review.imageUrls,
		skuName: review.skuName,
		productImageUrl: review.productImageUrl,
		productId: review.productId,
	};
}
