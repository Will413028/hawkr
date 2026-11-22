"use client";

import * as React from "react";
import Pagination from "@/components/shared/Pagination";
import Button from "@/components/ui/Button";
import { useUrlPageParam } from "@/lib/hooks/useUrlPageParam";
import { fetchStoreReviewsAtPage } from "@/lib/reviews/repo";
import ReviewList, { type SellerReviewItem } from "./ReviewList";
import { sellerReviewItem } from "./reviewItem";

interface ReviewsClientProps {
	sellerId: string;
	/** Static rating-summary card (SEO body); does not paginate. */
	statsCard: React.ReactNode;
	/** First page reviews, rendered in the SSR/ISR HTML (SEO). */
	initialReviews: SellerReviewItem[];
	initialTotalPages: number;
	/** 包住分頁的外殼；賣家預覽用它標出邊界放行的翻頁區。 */
	paginationWrapper?: (pagination: React.ReactNode) => React.ReactNode;
}

export default function ReviewsClient({
	sellerId,
	statsCard,
	initialReviews,
	initialTotalPages,
	paginationWrapper,
}: ReviewsClientProps) {
	// The first page is the ISR/SEO body already in the server HTML; later pages
	// are fetched client-side (below). `useUrlPageParam` reads `?page` WITHOUT
	// useSearchParams so the route is not bailed out to client-side rendering.
	const [currentPage, setPage] = useUrlPageParam();

	const [reviews, setReviews] = React.useState(initialReviews);
	const [totalPages, setTotalPages] = React.useState(initialTotalPages);
	const [pending, setPending] = React.useState(false);
	const [failed, setFailed] = React.useState(false);
	const [reloadKey, setReloadKey] = React.useState(0);

	React.useEffect(() => {
		void reloadKey;
		// Page 1 is already in the server HTML — restore it without a fetch.
		if (currentPage <= 1) {
			setReviews(initialReviews);
			setTotalPages(initialTotalPages);
			setPending(false);
			setFailed(false);
			return;
		}
		let active = true;
		const ac = new AbortController();
		setPending(true);
		setFailed(false);
		// 讀取失敗一律顯示錯誤與重試，不當成沒有評價；總頁數沿用伺服器端依統計算出的值。
		fetchStoreReviewsAtPage(sellerId, currentPage, { signal: ac.signal })
			.then((data) => {
				if (!active) return;
				setReviews(data.reviews.map(sellerReviewItem));
			})
			.catch(() => {
				if (active) setFailed(true);
			})
			.finally(() => {
				if (active) setPending(false);
			});
		return () => {
			active = false;
			ac.abort();
		};
	}, [sellerId, currentPage, initialReviews, initialTotalPages, reloadKey]);

	const retryButton = (
		<Button
			type="button"
			color="primary"
			onClick={() => setReloadKey((value) => value + 1)}
		>
			重新載入
		</Button>
	);
	const pagination = (
		<Pagination
			current={currentPage}
			total={totalPages}
			onPageChange={setPage}
		/>
	);
	return (
		<>
			<span data-hk-page-content-ready="seller-storefront" hidden />
			<div className="glass-card glass-card--lg" style={{ marginTop: 24 }}>
				{statsCard}
				{failed ? (
					<div>
						<p className="seller-profile__error" role="alert">
							評價暫時載入不到，請稍後再試。
						</p>
						{(paginationWrapper ?? ((node) => node))(retryButton)}
					</div>
				) : (
					<ReviewList reviews={reviews} busy={pending} />
				)}
			</div>
			{paginationWrapper ? paginationWrapper(pagination) : pagination}
		</>
	);
}
