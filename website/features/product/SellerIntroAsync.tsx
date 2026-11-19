import { getCachedStore } from "@/lib/api/cached";
import { imageUri } from "@/lib/media/image";
import { fetchStoreCards } from "@/lib/seller/fetchStoreCards";
import ProductSellerIntro from "./components/ProductSellerIntro";

/**
 * Streamed seller card. The card's identity (avatar / name / location) renders
 * instantly from the product payload's embedded `seller` block (see the
 * <Suspense> fallback in ProductDetailPage); only the rating / review-count /
 * subscriber numbers need a store read, which gates this component. The read is
 * `GetStore` by the block's store_code; a block without a code is resolved by
 * `BatchGetStores` on the store id, which also supplies the code for the link
 * but carries no follower count. Keeping it here pulls that upstream round-trip
 * off the page's critical TTFB path.
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
	const card = await fetchStoreCards([sellerId])
		.then((cards) => cards.get(sellerId) ?? null)
		.catch(() => null);
	return (
		<ProductSellerIntro
			sellerId={sellerId}
			seller={
				card
					? {
							storeCode: card.storeCode,
							name: card.name,
							logoUrl: imageUri(card.logo, "avatar") ?? "",
							reviewCount: card.reviewCount,
						}
					: null
			}
			sellerStats={
				card?.ratingAverage !== undefined && card.reviewCount !== undefined
					? { avgRating: card.ratingAverage, totalCount: card.reviewCount }
					: null
			}
			followerStats={null}
			productSeller={productSeller}
		/>
	);
}
