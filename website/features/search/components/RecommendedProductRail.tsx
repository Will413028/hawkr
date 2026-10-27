import Link from "@/components/shared/Link";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import { getProductPriceDisplay } from "@/lib/products/pricing";
import { parseRating, parseReviewCount } from "@/lib/products/rating";
import type { SearchProductHit } from "@/lib/search/repo";

interface RecommendedProductRailProps {
	products: ProductInput[];
}

type ProductInput = SearchProductHit & {
	hashtags?: Array<string | { name?: string }>;
	listPriceMin?: string;
	listPriceMax?: string;
	list_price_min?: string;
	list_price_max?: string;
	salePriceMin?: string;
	salePriceMax?: string;
	sale_price_min?: string;
	sale_price_max?: string;
	reviewCount?: string | number;
	reviews_count?: string | number;
};

function normaliseHashtags(raw: ProductInput["hashtags"], limit = 2): string[] {
	if (!Array.isArray(raw)) return [];
	const out: string[] = [];
	const seen = new Set<string>();
	for (const tag of raw) {
		const name = typeof tag === "string" ? tag : (tag?.name ?? "");
		const trimmed = name.trim();
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(trimmed);
		if (out.length >= limit) break;
	}
	return out;
}

export default function RecommendedProductRail({
	products,
}: RecommendedProductRailProps) {
	if (!products.length) return null;
	return (
		<section className="flex flex-col gap-3.5" aria-label="推薦給你的商品">
			<header className="flex items-end justify-between gap-3">
				<div>
					<strong className="block text-lg text-[var(--ink-heading)] mb-0.5 dark:text-[#f5e9c3]">
						推薦給您
					</strong>
					<small className="text-[0.8125rem] text-[var(--ink-soft)]">
						本週精選
					</small>
				</div>
				<Link
					className="text-[0.8125rem] text-[var(--action-primary)] no-underline font-medium whitespace-nowrap hover:underline"
					href="/market/products"
				>
					看市集 →
				</Link>
			</header>
			<div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-5 max-[680px]:grid-cols-2 max-[680px]:gap-3">
				{products.map((product, index) => {
					const id = String(product.id || index);
					// Empty when absent → MarketProductCard's <AppImage> shows the
					// design-system ImageFallback (folder-image) instead of a
					// branded stand-in.
					// B-238: render seller identity only from the canonical URL.
					const image = product.imageKey || product.imageKeys?.[0] || "";
					const sellerName =
						product.sellerName || product.seller?.name || "Hawkr 選品";
					const sellerAvatar = product.seller?.logoUrl?.trim() || "";
					// 兩個來源（marketplace hot_products = handler.ProductItem、搜尋命中
					// = handler.ProductHitData）都以 storeId 帶 store surrogate。
					const sellerId =
						String(product.storeId ?? product.seller?.storeId ?? "") ||
						undefined;
					// Don't echo the store name as 產地. Use a real origin/city, else 產地直送.
					const origin =
						product.seller?.city ||
						(product.origin && product.origin !== sellerName
							? product.origin
							: "") ||
						"產地直送";
					const rating = parseRating(product.rating);
					const reviews = parseReviewCount(
						product.reviewCount ?? product.reviews_count,
					);
					const priceDisplay = getProductPriceDisplay(product);
					const hashtagList = normaliseHashtags(product.hashtags, 2);
					const item: MarketProductItem = {
						id,
						name: product.name || "Hawkr 市集商品",
						image,
						sellerName,
						sellerAvatar,
						sellerId,
						origin,
						price: priceDisplay.effectiveText || "價格未定",
						originalPrice: priceDisplay.originalText || undefined,
						isOnSale: priceDisplay.isOnSale,
						hashtags: hashtagList,
						unit: "份",
						rating,
						reviews,
						href: `/products/${id}`,
						// Sort fields aren't read inside the card itself; provide
						// safe defaults so the type contract holds.
						priceValue: 0,
						createdAt: 0,
						salesCount: 0,
					};
					return (
						<MarketProductCard
							key={`recommend-${id}`}
							product={item}
							imageLoading="lazy"
						/>
					);
				})}
			</div>
		</section>
	);
}
