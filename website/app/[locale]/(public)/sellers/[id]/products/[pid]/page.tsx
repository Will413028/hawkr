import type { Metadata } from "next";
import { ProductDetailPage } from "@/features/product";
import { getCachedProduct } from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { SENTINEL_ENTITY_ID } from "@/lib/api/static-params";
import { buildImageUrl } from "@/lib/media/cdn";
import {
	localeFromParam,
	productSeoTitle,
	socialImages,
} from "@/lib/seo/metadata";
import { productSeoDescription } from "@/lib/seo/product-summary";
import { productUrl } from "@/lib/seo/urls";

interface Props {
	params: Promise<{ locale: string; id: string; pid: string }>;
}

export function generateStaticParams() {
	return [{ id: SENTINEL_ENTITY_ID, pid: SENTINEL_ENTITY_ID }];
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { pid, locale } = await params;
	const product = await getCachedProduct(pid).catch(prerenderSafe(null));
	if (!product) return { title: "找不到商品" };

	// Same composer as the canonical /products/[id] route: this URL canonicalises
	// there, so the two must serve one title and one summary.
	const pageLocale = localeFromParam(locale);
	const title = productSeoTitle(
		{ name: product.name, sellerName: product.seller?.name },
		pageLocale,
	);
	const description = productSeoDescription(product);
	const imgUrl = buildImageUrl(product.imageKey, "detail");
	const canonical = productUrl(pid, pageLocale);

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

export default async function Page({ params }: Props) {
	const { pid } = await params;
	return <ProductDetailPage productId={pid} />;
}
