"use client";

import * as React from "react";
import Pagination from "@/components/shared/Pagination";
import { fetchSellerReviews } from "@/lib/api/index";
import { useUrlPageParam } from "@/lib/hooks/useUrlPageParam";
import ReviewList, { type SellerReviewItem } from "./ReviewList";

interface ReviewsClientProps {
	sellerId: string;
	/** Static rating-summary card (SEO body); does not paginate. */
	statsCard: React.ReactNode;
	/** First page reviews, rendered in the SSR/ISR HTML (SEO). */
	initialReviews: SellerReviewItem[];
	initialTotalPages: number;
}

export default function ReviewsClient({
	sellerId,
	statsCard,
	initialReviews,
	initialTotalPages,
}: ReviewsClientProps) {
	// The first page is the ISR/SEO body already in the server HTML; later pages
	// are fetched client-side (below). `useUrlPageParam` reads `?page` WITHOUT
	// useSearchParams so the route is not bailed out to client-side rendering.
	const [currentPage, setPage] = useUrlPageParam();

	const [reviews, setReviews] = React.useState(initialReviews);
	const [totalPages, setTotalPages] = React.useState(initialTotalPages);
	const [pending, setPending] = React.useState(false);

	React.useEffect(() => {
		// Page 1 is already in the server HTML — restore it without a fetch.
		if (currentPage <= 1) {
			setReviews(initialReviews);
			setTotalPages(initialTotalPages);
			setPending(false);
			return;
		}
		let active = true;
		setPending(true);
		fetchSellerReviews(sellerId, currentPage)
			.then((data) => {
				if (!active) return;
				setReviews((data.records as SellerReviewItem[]) ?? []);
				if (data.totalPages) setTotalPages(data.totalPages);
			})
			.catch(() => {
				// Soft path: keep whatever is on screen if a page fetch fails.
			})
			.finally(() => {
				if (active) setPending(false);
			});
		return () => {
			active = false;
		};
	}, [sellerId, currentPage, initialReviews, initialTotalPages]);

	return (
		<>
			<span data-hk-page-content-ready="seller-storefront" hidden />
			<div className="glass-card glass-card--lg" style={{ marginTop: 24 }}>
				{statsCard}
				<ReviewList reviews={reviews} busy={pending} />
			</div>
			<Pagination
				current={currentPage}
				total={totalPages}
				onPageChange={setPage}
			/>
		</>
	);
}
