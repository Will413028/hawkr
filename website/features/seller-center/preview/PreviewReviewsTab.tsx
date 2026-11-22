"use client";

import { useQuery } from "@tanstack/react-query";
import type * as React from "react";
import ReviewStatsCard from "@/features/sellers/reviews/ReviewStatsCard";
import ReviewsClient from "@/features/sellers/reviews/ReviewsClient";
import { sellerReviewItem } from "@/features/sellers/reviews/reviewItem";
import {
	fetchStoreReviewStats,
	fetchStoreReviewsPage,
	STORE_REVIEWS_PAGE_SIZE,
} from "@/lib/reviews/repo";
import { PREVIEW_REVIEWS_UNAVAILABLE } from "@/lib/seller/storefront-preview";
import PreviewReadingControl from "./PreviewReadingControl";
import { PreviewPublicOnly, usePreviewStore } from "./PreviewStoreContext";

const wrapPagination = (pagination: React.ReactNode) => (
	<PreviewReadingControl>{pagination}</PreviewReadingControl>
);

/** 只在店家能公開時才掛載：讀買家看得到的公開評價與統計。 */
function PublicReviews() {
	const { view } = usePreviewStore();
	const query = useQuery({
		queryKey: ["seller-preview", "reviews", view.storeId],
		queryFn: async () => {
			const [reviews, stats] = await Promise.all([
				fetchStoreReviewsPage(view.storeId),
				fetchStoreReviewStats(view.storeId),
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
			initialReviews={reviews.reviews.map(sellerReviewItem)}
			initialTotalPages={Math.ceil(stats.totalCount / STORE_REVIEWS_PAGE_SIZE)}
			paginationWrapper={wrapPagination}
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
