"use client";

import * as React from "react";
import Pagination from "@/components/shared/Pagination";
import Button from "@/components/ui/Button";
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
	/** 包住分頁的外殼；賣家預覽用它標出邊界放行的翻頁區。 */
	paginationWrapper?: (pagination: React.ReactNode) => React.ReactNode;
	/** 嚴格模式（賣家預覽）：翻頁讀取失敗要顯示錯誤，不當成空清單；公開店面不傳。 */
	strict?: boolean;
}

export default function ReviewsClient({
	sellerId,
	statsCard,
	initialReviews,
	initialTotalPages,
	paginationWrapper,
	strict = false,
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
		setPending(true);
		setFailed(false);
		fetchSellerReviews(sellerId, currentPage, strict)
			.then((data) => {
				if (!active) return;
				setReviews((data.records as SellerReviewItem[]) ?? []);
				if (data.totalPages) setTotalPages(data.totalPages);
			})
			.catch(() => {
				// Public path keeps whatever is on screen; strict mode surfaces it.
				if (active && strict) setFailed(true);
			})
			.finally(() => {
				if (active) setPending(false);
			});
		return () => {
			active = false;
		};
	}, [
		sellerId,
		currentPage,
		initialReviews,
		initialTotalPages,
		strict,
		reloadKey,
	]);

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
						{strict
							? (paginationWrapper ?? ((node) => node))(retryButton)
							: null}
					</div>
				) : (
					<ReviewList reviews={reviews} busy={pending} />
				)}
			</div>
			{paginationWrapper ? paginationWrapper(pagination) : pagination}
		</>
	);
}
