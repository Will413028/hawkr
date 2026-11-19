"use client";

import { useQuery } from "@tanstack/react-query";
import type { SellerReviewItem } from "@/features/sellers/reviews/ReviewList";
import ReviewStatsCard, {
	type SellerReviewStats,
} from "@/features/sellers/reviews/ReviewStatsCard";
import ReviewsClient from "@/features/sellers/reviews/ReviewsClient";
import { fetchSellerReviewStats, fetchSellerReviews } from "@/lib/api/index";
import { PREVIEW_REVIEWS_UNAVAILABLE } from "@/lib/seller/storefront-preview";
import { PreviewPublicOnly, usePreviewStore } from "./PreviewStoreContext";

/** 只在店家能公開時才掛載：讀買家看得到的公開評價與統計。 */
function PublicReviews() {
	const { view } = usePreviewStore();
	const query = useQuery({
		queryKey: ["seller-preview", "reviews", view.storeId],
		queryFn: async () => {
			const [reviews, stats] = await Promise.all([
				fetchSellerReviews(view.storeId, 1, true),
				fetchSellerReviewStats(
					view.storeId,
					true,
				) as Promise<SellerReviewStats | null>,
			]);
			return { reviews, stats };
		},
		staleTime: 30_000,
	});
	if (!query.data) {
		return query.isError ? (
			<p className="seller-profile__error" role="alert">
				評價暫時載入不到，請稍後再試。
			</p>
		) : (
			<p role="status" aria-busy="true">
				正在載入評價…
			</p>
		);
	}
	const { reviews, stats } = query.data;
	return (
		<ReviewsClient
			sellerId={view.storeId}
			statsCard={<ReviewStatsCard stats={stats} />}
			initialReviews={reviews.records as SellerReviewItem[]}
			initialTotalPages={reviews.totalPages}
		/>
	);
}

export default function PreviewReviewsTab() {
	return (
		<PreviewPublicOnly unavailable={PREVIEW_REVIEWS_UNAVAILABLE}>
			<PublicReviews />
		</PreviewPublicOnly>
	);
}
