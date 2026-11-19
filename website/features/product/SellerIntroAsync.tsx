import { getCachedStore } from "@/lib/api/cached";
import ProductSellerIntro from "./components/ProductSellerIntro";

/**
 * Streamed seller card. The card's identity (avatar / name / location) renders
 * instantly from the product payload's embedded `seller` block (see the
 * <Suspense> fallback in ProductDetailPage); only the rating / review-count /
 * subscriber numbers need a store read, which gates this component. The read is
 * `GetStore` by the block's store_code; a block without a code has no storefront
 * link and its stats stay unset. Keeping the read here pulls that upstream
 * round-trip off the page's critical TTFB path.
 */
export default async function SellerIntroAsync({
	sellerId,
	productSeller,
}: {
	sellerId: string;
	productSeller: unknown;
}) {
	// Seller enrichment is optional to the PDP. Keep the embedded product-seller
	// identity usable when the store read is temporarily unavailable.
	const storeCode = String(
		(productSeller as { storeCode?: unknown } | null)?.storeCode ?? "",
	).trim();
	if (storeCode) {
		const store = await getCachedStore(storeCode).catch(() => null);
		return (
			<ProductSellerIntro
				sellerId={sellerId}
				seller={store ? { ...store, logoUrl: store.logo_url } : null}
				sellerStats={
					store?.ratingAverage !== undefined && store.reviewCount !== undefined
						? { avgRating: store.ratingAverage, totalCount: store.reviewCount }
						: null
				}
				followerStats={
					store?.followerCount !== undefined
						? { followers: store.followerCount }
						: null
				}
				productSeller={productSeller}
			/>
		);
	}
	return (
		<ProductSellerIntro
			sellerId={sellerId}
			seller={null}
			sellerStats={null}
			followerStats={null}
			productSeller={productSeller}
		/>
	);
}
