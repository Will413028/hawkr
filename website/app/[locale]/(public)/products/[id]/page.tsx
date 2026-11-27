import type { Metadata } from "next";
import { notFound } from "next/navigation";
import ViewItemTracker from "@/components/analytics/ViewItemTracker";
import JsonLd from "@/components/seo/JsonLd";
import { ProductDetailPage } from "@/features/product";
import type { GA4Item } from "@/lib/analytics/types";
import { getCachedProduct } from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { SENTINEL_ENTITY_ID } from "@/lib/api/static-params";
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

export function generateStaticParams() {
	return [{ id: SENTINEL_ENTITY_ID }];
}

function assertValidId(id: string) {
	if (!/^\d+$/.test(id)) notFound();
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	assertValidId(id);
	const product = await getCachedProduct(id).catch(prerenderSafe(null));

	if (!product) return { title: "找不到商品" };

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
