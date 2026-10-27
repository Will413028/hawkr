import { getCachedProductReviews } from "@/lib/api/cached";
import ProductReviewsSection from "./components/ProductReviewsSection";
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
	const reviewsResult = await getCachedProductReviews(productId, 1, 6).catch(
		() => null,
	);
	const { reviews, total } = mapLiveReviews(reviewsResult);
	return <ProductReviewsSection reviews={reviews} reviewTotal={total} />;
}
