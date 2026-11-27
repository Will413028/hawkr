"use client";

import * as React from "react";
import StorefrontProductGrid from "@/features/sellers/storefront/StorefrontProductGrid";
import { fetchProducts, fetchSellerProducts } from "@/lib/api/index";
import { useUrlPageParam } from "@/lib/hooks/useUrlPageParam";
import { mapProduct, type ProductRecord } from "@/lib/market/listFilters";

const PAGE_SIZE = 12;

interface SellerProductsClientProps {
	sellerId: string;
	/** First page products, rendered in the SSR/ISR HTML (SEO). */
	initialProducts: ProductRecord[];
	initialTotalPages: number;
}

/**
 * Client-safe (soft, non-strict) page fetch that mirrors the server storefront
 * source order: the rich market endpoint first (so cards keep sale price +
 * favoriteCount), then the leaner seller endpoint as a fallback. Never throws —
 * an outage or genuinely empty page just leaves the current grid unchanged.
 */
async function fetchStorefrontPage(
	sellerId: string,
	page: number,
	signal: AbortSignal,
): Promise<{ records: ProductRecord[]; totalPages: number }> {
	const envelope = (await fetchProducts(
		"",
		page,
		PAGE_SIZE,
		undefined,
		undefined,
		{ sellerStoreIds: [sellerId], signal },
	)) as {
		data?: { records?: ProductRecord[]; total?: number; totalPages?: number };
	};
	const data = envelope?.data;
	let records = (data?.records ?? []) as ProductRecord[];
	const total = Number(data?.total ?? 0) || 0;
	let totalPages =
		Number(data?.totalPages ?? 0) || Math.ceil(total / PAGE_SIZE) || 0;
	if (!records.length) {
		const fallback = await fetchSellerProducts(sellerId, page, PAGE_SIZE);
		if (fallback.records.length) {
			records = fallback.records as ProductRecord[];
			totalPages = fallback.totalPages || totalPages;
		}
	}
	return { records, totalPages };
}

export default function SellerProductsClient({
	sellerId,
	initialProducts,
	initialTotalPages,
}: SellerProductsClientProps) {
	const [currentPage, setPage] = useUrlPageParam();

	const [products, setProducts] = React.useState(initialProducts);
	const [totalPages, setTotalPages] = React.useState(initialTotalPages);

	React.useEffect(() => {
		if (currentPage <= 1) {
			setProducts(initialProducts);
			setTotalPages(initialTotalPages);
			return;
		}
		const controller = new AbortController();
		let active = true;
		fetchStorefrontPage(sellerId, currentPage, controller.signal)
			.then((data) => {
				if (!active) return;
				setProducts(data.records);
				if (data.totalPages) setTotalPages(data.totalPages);
			})
			.catch(() => {});
		return () => {
			active = false;
			controller.abort();
		};
	}, [sellerId, currentPage, initialProducts, initialTotalPages]);

	const items = React.useMemo(
		() => products.map((product, index) => mapProduct(product, index)),
		[products],
	);

	return (
		<>
			<span data-hk-page-content-ready="seller-storefront" hidden />
			<StorefrontProductGrid
				items={items}
				currentPage={currentPage}
				totalPages={totalPages}
				onPageChange={setPage}
			/>
		</>
	);
}
