/**
 * Storefront landing — `/sellers/:id`.
 *
 * Buyer-facing page that shows a seller's profile, banner, and product
 * grid. Logic stays here; the route file (app/sellers/[id]/page.tsx)
 * keeps only generateMetadata + the param extraction shell.
 */

import SellerProductsClient from "@/features/sellers/storefront/SellerProductsClient";
import {
	getCachedSellerProducts,
	getCachedSellerStorefrontProducts,
} from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";

interface Props {
	sellerId: string;
	/** Seller-center buyer-view preview: product cards open a read-only drawer. */
	previewMode?: boolean;
}

export default async function StorefrontPage({
	sellerId: id,
	previewMode = false,
}: Props) {
	// Only the first page is server-rendered (SEO/ISR body). Later pages are
	// fetched client-side by SellerProductsClient off the `?page` URL param, so
	// this route reads no dynamic API and stays statically renderable.
	//
	// Source the grid from the rich market endpoint so the cards match 市集
	// (sale price + favoriteCount). Fall back to the leaner seller endpoint
	// if it comes back empty (e.g. backend filter miss). An empty result
	// renders an empty grid — no mock products.
	let productsData = await getCachedSellerStorefrontProducts(id, 1, 12).catch(
		prerenderSafe({ records: [], total: 0, totalPages: 0 }),
	);

	if (!productsData.records.length) {
		const fallback = await getCachedSellerProducts(id, 1, 12).catch(() => null);
		if (fallback?.records.length) productsData = fallback;
	}

	// Profile header / banner / page shell live in the persistent
	// (storefront) layout — this page owns only the product grid, so tab
	// switches swap just this subtree (no scroll-fade: content appearing
	// where unchanged chrome sits still should not drift in).
	return (
		<SellerProductsClient
			sellerId={id}
			initialProducts={productsData.records}
			initialTotalPages={productsData.totalPages}
			previewMode={previewMode}
		/>
	);
}
