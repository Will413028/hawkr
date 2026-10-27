import type { Metadata } from "next";
import "@/styles/features/cart.css";
/* MarketProductCard still leans on .market-product-card styles. */
import "@/styles/features/market.css";
import "@/styles/features/cart-recommendations.css";
import CartClient from "@/features/cart/CartClient";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import type { MarketplaceProduct } from "@/lib/api";
import { getCachedMarketplace } from "@/lib/api/cached";
import { getProductPriceDisplay } from "@/lib/products/pricing";
import { parseRating, parseReviewCount } from "@/lib/products/rating";

export const metadata: Metadata = {
	title: "購物車",
	description:
		"檢視已加入購物車的商品、調整數量、套用優惠碼，並選擇配送方式準備結帳。",
};

function normaliseHashtags(raw: MarketplaceProduct["hashtags"]): string[] {
	if (!Array.isArray(raw)) return [];
	const out: string[] = [];
	const seen = new Set<string>();
	for (const tag of raw) {
		const name = typeof tag === "string" ? tag : (tag?.name ?? "");
		const trimmed = name.trim().replace(/^#/, "");
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(trimmed);
		if (out.length >= 2) break;
	}
	return out;
}

/**
 * Map a marketplace `hot_products[]` entry into the market grid's card
 * shape so the "KEEP SHOPPING" rail renders the same `MarketProductCard`
 * the Hawkr 市集 list uses (favourite heart, qty stepper, add-to-cart).
 * `priceValue`/`createdAt`/`salesCount` only feed the market page's
 * client-side sort fallback, which this static rail never invokes.
 */
function toRecommendedProduct(
	product: MarketplaceProduct,
	index: number,
): MarketProductItem {
	const id = String(product.id || index);
	const priceDisplay = getProductPriceDisplay(product);
	return {
		id,
		name: (product.name ?? "").trim() || "推薦商品",
		// Empty when absent → MarketProductCard's <AppImage> shows the
		// design-system ImageFallback (folder-image) rather than a branded
		// stand-in.
		image: product.imageUrl || "",
		sellerName: (product.sellerName ?? "").trim(),
		sellerAvatar: product.seller?.logoUrl?.trim() || undefined,
		sellerId: product.storeId ? String(product.storeId) : undefined,
		origin: "",
		price: priceDisplay.effectiveText || "價格未定",
		originalPrice: priceDisplay.originalText || undefined,
		isOnSale: priceDisplay.isOnSale,
		hashtags: normaliseHashtags(product.hashtags),
		unit: product.skuNames?.[0] || "份",
		rating: parseRating(product.rating),
		// reviewCount 由 /public/products 提供（#1156 後端修復後為真實值）；
		// rating 為 null 時卡片仍顯示「尚無評價」。
		reviews: parseReviewCount(product.reviewCount),
		href: `/products/${id}`,
		priceValue: 0,
		createdAt: 0,
		salesCount: 0,
	};
}

export default async function CartPage() {
	// Pull a small slice of marketplace recommendations so the
	// "KEEP SHOPPING" rail surfaces real products. Best-effort: if the
	// call fails or returns nothing we render NOTHING — never fall back
	// to hard-coded placeholder bundles. An empty cart-recommendation
	// rail showing fake "五月蔬果" cards is worse than no rail at all.
	const marketplace = await getCachedMarketplace(0, 0, 6).catch(() => null);
	const hotProducts = marketplace?.hotProducts ?? [];
	const cards: MarketProductItem[] = hotProducts
		.slice(0, 6)
		.map(toRecommendedProduct);

	return (
		<main className="cart-shell-page relative isolate overflow-hidden bg-transparent pt-[var(--page-start-gap,6px)] text-hk-body max-md:px-0 max-md:py-[52px] max-md:pt-[var(--page-start-gap,var(--hkui-space-4))]">
			<div className="w-[min(1440px,calc(100%-48px))] mx-auto max-md:w-[min(100%-28px,680px)]">
				<CartClient />

				{cards.length > 0 ? (
					<>
						<section
							className="max-w-[760px] mx-auto mb-8 text-center"
							style={{ marginTop: 48 }}
						>
							<span className="inline-flex w-fit mb-3.5 text-hk-primary cart-type-support tracking-normal uppercase">
								Keep shopping
							</span>
							<h2 className="m-0 mb-3.5 text-hk-heading cart-type-section-title">
								挑完購物車，再看看這週剛上架的
							</h2>
						</section>
						<section
							aria-label="購物車商品推薦"
							className="cart-recommendations grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-5 max-[680px]:grid-cols-2 max-[680px]:gap-3"
						>
							{cards.map((card, index) => (
								<MarketProductCard
									key={card.id}
									product={card}
									index={index}
									listContext={{
										listId: "cart_keep_shopping",
										listName: "購物車為你推薦",
									}}
								/>
							))}
						</section>
					</>
				) : null}
			</div>
		</main>
	);
}
