import type { Metadata } from "next";
import { notFound } from "next/navigation";
import ViewItemTracker from "@/components/analytics/ViewItemTracker";
import JsonLd from "@/components/seo/JsonLd";
import { ProductDetailPage } from "@/features/product";
import type { GA4Item } from "@/lib/analytics/types";
import { getCachedProduct } from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { buildImageUrl } from "@/lib/media/cdn";
import { breadcrumbSchema, productSchema } from "@/lib/seo/jsonld";
import {
	localeFromParam,
	productSeoTitle,
	socialImages,
} from "@/lib/seo/metadata";
import { productSeoDescription } from "@/lib/seo/product-summary";
import { absoluteUrl, productUrl } from "@/lib/seo/urls";

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

// On-demand ISR：不在 build 期列舉真實商品（後端對任一商品的 5xx 不會炸 build），
// 全部首訪生成、快取 revalidate 秒數後背景刷新，並由 POST /api/revalidate 帶
// productTag 即時失效。傳統模式 dynamicParams 預設 true，空陣列即「零預建、全 on-demand」。
export const revalidate = 60;

export function generateStaticParams() {
	return [];
}

// 商品 id 是純數字 TSID；掃描器打的垃圾路徑（如 /products/styles.css.map）
// 直接 404，不讓非法 id 流進後端打出 400 ApiClientError。
function assertValidId(id: string) {
	if (!/^\d+$/.test(id)) notFound();
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	assertValidId(id);
	const product = await getCachedProduct(id).catch(prerenderSafe(null));

	if (!product) return { title: "找不到商品" };

	// One title and one summary for every surface (see lib/seo/metadata.ts
	// `productSeoTitle`, lib/seo/product-summary.ts): the page <title>, the
	// share card and the JSON-LD below must never tell three stories.
	const pageLocale = localeFromParam(locale);
	const title = productSeoTitle(
		{ name: product.name, sellerName: product.seller?.name },
		pageLocale,
	);
	const description = productSeoDescription(product);
	const imgUrl = buildImageUrl(product.imageKey, "detail");
	const canonical = productUrl(id, pageLocale);

	return {
		title: { absolute: title },
		description,
		alternates: { canonical },
		openGraph: {
			title,
			description,
			url: canonical,
			images: socialImages(imgUrl),
			type: "website",
		},
	};
}

interface ProductData {
	name: string;
	/** §231: `imageKey` is the objectKey SSOT; buildImageUrl turns it into the
	 *  requested preset URL without a peel-and-rebuild round trip. */
	imageKey?: string;
	seller?: { name?: string };
	/** Seller's 詳細說明 (`{ version, blocks }`); summarised for SEO surfaces. */
	description?: unknown;
	origin?: string;
	categoryLv1Name?: string;
	categoryLv2Name?: string;
	categoryLv3Name?: string;
	categoryName?: string;
	/** Product-level stock total, when the wire reports one. */
	stock?: number | string;
	salePriceMin?: number | string;
	salePriceMax?: number | string;
	priceMin?: number | string;
	priceMax?: number | string;
	price?: number | string;
	skus?: { stockQty?: number | string; price?: number | string }[];
	rating?: number | string;
	reviews?: unknown[];
	reviewCount?: number | string;
	publicationState?: string;
	purchasable?: boolean;
}

export default async function Page({ params }: Props) {
	// PDP renders as on-demand ISR (traditional model, cacheComponents off), same
	// as the sister route /sellers/[id]/products/[pid]. Entity reads stay
	// tag-invalidated via `getCachedProduct` / ProductDetailPage; the route-level
	// ISR entry gives cheap cached renders (no per-request render → no
	// resume-hang / OOM exposure) and full server-rendered SEO.
	const { id, locale: rawLocale } = await params;
	assertValidId(id);
	const locale = localeFromParam(rawLocale);
	const product = (await getCachedProduct(id).catch(
		prerenderSafe(null),
	)) as ProductData | null;
	if (!product) notFound();
	const canonical = productUrl(id, locale);

	let jsonLd: React.ReactNode = null;
	let offerLow = 0;
	if (product) {
		// Price: prefer sale range, fall back to list range, then to SKU prices;
		// collapse to a single Offer when low === high. The backend reports "no
		// sale price" as the string "0" (not null), so a nullish chain would stop
		// there and emit no offer at all — pick the first POSITIVE number instead.
		// Only emit a price when one exists (never a fabricated 0).
		const skuPrices = (Array.isArray(product.skus) ? product.skus : [])
			.map((s) => Number(s.price))
			.filter((n) => Number.isFinite(n) && n > 0);
		const firstPositive = (
			...vals: Array<number | string | undefined>
		): number => {
			for (const v of vals) {
				const n = Number(v);
				if (Number.isFinite(n) && n > 0) return n;
			}
			return 0;
		};
		const low = firstPositive(
			product.salePriceMin,
			product.priceMin,
			product.price,
			skuPrices.length ? Math.min(...skuPrices) : undefined,
		);
		const high =
			firstPositive(
				product.salePriceMax,
				product.priceMax,
				product.price,
				skuPrices.length ? Math.max(...skuPrices) : undefined,
			) || low;
		offerLow = low;
		const hasRange =
			Number.isFinite(low) && Number.isFinite(high) && high > low && low > 0;
		const hasPrice = Number.isFinite(low) && low > 0;
		// Availability: only from stock the wire actually reports — per-SKU
		// `stockQty` first, the product-level `stock` total when no SKU says.
		// Unknown stays unknown (no `availability` in the schema): an
		// unreported quantity used to be advertised as InStock, which is a claim
		// the page never makes.
		const skus = Array.isArray(product.skus) ? product.skus : [];
		const reported = (
			value: number | string | undefined,
		): number | undefined => {
			if (value === undefined || value === null || value === "")
				return undefined;
			const n = Number(value);
			return Number.isFinite(n) ? n : undefined;
		};
		const skuStock = skus
			.map((s: { stockQty?: number | string }) => reported(s.stockQty))
			.filter((n): n is number => n !== undefined);
		const knownStock = skuStock.length
			? Math.max(...skuStock)
			: reported(product.stock);
		const availability =
			knownStock === undefined
				? undefined
				: knownStock > 0
					? ("InStock" as const)
					: ("OutOfStock" as const);
		// Rating: emit only with a real review count (never fabricated).
		const ratingValue = Number(product.rating);
		const reviewCount = Number(product.reviewCount ?? 0);
		const image = buildImageUrl(product.imageKey, "detail");

		jsonLd = (
			<JsonLd
				data={[
					productSchema({
						name: product.name,
						url: canonical,
						image,
						description: productSeoDescription(product),
						...(hasRange
							? { priceRange: { low, high } }
							: hasPrice
								? { price: low }
								: {}),
						...(availability ? { availability } : {}),
						purchasable: product.purchasable,
						...(ratingValue > 0 && reviewCount > 0
							? { rating: { value: ratingValue, count: reviewCount } }
							: {}),
					}),
					breadcrumbSchema([
						{
							name: "Hawkr 市集",
							url: absoluteUrl("/market/products", locale),
						},
						{ name: product.name, url: canonical },
					]),
				]}
			/>
		);
	}

	const viewItem: GA4Item | null = product
		? {
				item_id: id,
				item_name: product.name,
				// Same first-positive semantics as the JSON-LD offer above — the
				// backend's "0" sale-price sentinel must not suppress the real price.
				...(offerLow > 0 ? { price: offerLow } : {}),
				...(product.seller?.name ? { item_brand: product.seller.name } : {}),
			}
		: null;

	return (
		<>
			{jsonLd}
			{viewItem && <ViewItemTracker item={viewItem} />}
			<ProductDetailPage productId={id} />
		</>
	);
}
