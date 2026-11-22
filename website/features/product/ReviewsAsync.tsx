import { getCachedProductReviews } from "@/lib/api/cached";
import ProductReviewsSection from "./components/ProductReviewsSection";
import ProductReviewsUnavailable from "./components/ProductReviewsUnavailable";
import { mapLiveReviews } from "./deriveDetailLayoutProps";

/**
 * Streamed PDP reviews section. The reviews fetch (`getCachedProductReviews`)
 * is kept off the page's critical TTFB path: the shell renders from the product
 * payload alone, and this server component streams the reviews in behind a
 * <Suspense> boundary once they resolve.
 */
export default async function ReviewsAsync({
	productId,
}: {
	productId: string;
}) {
	// 讀取失敗不顯示成「還沒有評價」，也不讓整段消失：顯示讀取失敗與重試。
	const reviewsResult = await getCachedProductReviews(productId, 6).catch(
		() => undefined,
	);
	if (reviewsResult === undefined) return <ProductReviewsUnavailable />;
	const { reviews, total } = mapLiveReviews(reviewsResult);
	return (
		<ProductReviewsSection reviews={reviews} reviewTotal={total} reportable />
	);
}
