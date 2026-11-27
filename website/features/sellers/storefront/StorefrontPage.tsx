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
}

export default async function StorefrontPage({ sellerId: id }: Props) {
	let productsData = await getCachedSellerStorefrontProducts(id, 1, 12).catch(
		prerenderSafe({ records: [], total: 0, totalPages: 0 }),
	);

	if (!productsData.records.length) {
		const fallback = await getCachedSellerProducts(id, 1, 12).catch(() => null);
		if (fallback?.records.length) productsData = fallback;
	}

	return (
		<SellerProductsClient
			sellerId={id}
			initialProducts={productsData.records}
			initialTotalPages={productsData.totalPages}
		/>
	);
}
