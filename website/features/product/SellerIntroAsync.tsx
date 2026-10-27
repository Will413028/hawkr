import {
	getCachedSeller,
	getCachedSellerReviewStats,
	getCachedSellerStats,
} from "@/lib/api/cached";
import ProductSellerIntro from "./components/ProductSellerIntro";

/**
 * Streamed seller card. The card's identity (avatar / name / location) renders
 * instantly from the product payload's embedded `seller` block (see the
 * <Suspense> fallback in ProductDetailPage); only the rating / review-count /
 * subscriber numbers need the seller + stats fetches, which gate this component.
 * Keeping them here pulls that second upstream round-trip off the page's
 * critical TTFB path.
 */
export default async function SellerIntroAsync({
	sellerId,
	productSeller,
}: {
	sellerId: string;
	productSeller: unknown;
}) {
	// Seller enrichment is optional to the PDP. Keep the embedded product-seller
	// identity usable when any public stats endpoint is temporarily unavailable.
	const [seller, sellerStats, followerStats] = await Promise.all([
		getCachedSeller(sellerId).catch(() => null),
		getCachedSellerReviewStats(sellerId).catch(() => null),
		getCachedSellerStats(sellerId).catch(() => null),
	]);
	return (
		<ProductSellerIntro
			sellerId={sellerId}
			seller={seller}
			sellerStats={sellerStats}
			followerStats={followerStats}
			productSeller={productSeller}
		/>
	);
}
