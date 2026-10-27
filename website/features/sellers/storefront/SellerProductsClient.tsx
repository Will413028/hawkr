"use client";

import * as React from "react";
import MasonryGrid, {
	type MasonryBreakpoint,
} from "@/components/shared/MasonryGrid";
import Pagination from "@/components/shared/Pagination";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import { fetchProducts, fetchSellerProducts } from "@/lib/api/index";
import { useUrlPageParam } from "@/lib/hooks/useUrlPageParam";
import { mapProduct, type ProductRecord } from "@/lib/market/listFilters";

/**
 * Storefront column rules. Same contract as the market feed's table (see
 * `marketProductGridBreakpoints.ts`): these declarations are the only place
 * the column count is decided, and the SSR fallback plus the measured pass
 * both resolve it from here, so they cannot disagree (#423).
 *
 * The storefront has no sidebar, so its grid is much wider than the market
 * feed's at the same viewport — hence a larger `auto-fill` minimum (280px)
 * to land on the same four columns this page has always shown on desktop.
 * Mobile and tablet storefronts intentionally keep a stable two-column grid.
 */
const SELLER_PRODUCT_GRID_BREAKPOINTS: MasonryBreakpoint[] = [
	{ maxWidth: 360, track: "repeat(2, minmax(0, 1fr))", gap: 12 },
	{ maxWidth: 680, track: "repeat(2, minmax(0, 1fr))", gap: 12 },
	{ maxWidth: 840, track: "repeat(2, minmax(0, 1fr))", gap: 12 },
	{ maxWidth: 1120, track: "repeat(3, minmax(0, 1fr))", gap: 16 },
	{
		maxWidth: Number.POSITIVE_INFINITY,
		track: "repeat(auto-fill, minmax(280px, 1fr))",
		gap: 18,
	},
];

const PAGE_SIZE = 12;

interface SellerProductsClientProps {
	sellerId: string;
	/** First page products, rendered in the SSR/ISR HTML (SEO). */
	initialProducts: ProductRecord[];
	initialTotalPages: number;
	/** Open the public product detail in the seller-preview drawer. */
	previewMode?: boolean;
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
	previewMode = false,
}: SellerProductsClientProps) {
	// The first page is the ISR/SEO body already in the server HTML; later pages
	// are fetched client-side (below). `useUrlPageParam` reads `?page` WITHOUT
	// useSearchParams so the route is not bailed out to client-side rendering.
	const [currentPage, setPage] = useUrlPageParam();

	const [products, setProducts] = React.useState(initialProducts);
	const [totalPages, setTotalPages] = React.useState(initialTotalPages);

	React.useEffect(() => {
		// Page 1 is already in the server HTML — restore it without a fetch.
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
			.catch(() => {
				// Soft path: keep the current grid on failure / abort.
			});
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
			{items.length > 0 ? (
				<MasonryGrid
					breakpoints={SELLER_PRODUCT_GRID_BREAKPOINTS}
					className="market-product-grid"
					getKey={(product) => product.id}
					items={items}
					renderItem={(product, index) => (
						<MarketProductCard
							index={index}
							priority={index === 0}
							imageLoading="eager"
							product={product}
							storefrontPreviewMode={previewMode}
						/>
					)}
				/>
			) : (
				<div
					className="glass-card glass-card--lg"
					style={{
						marginTop: 24,
						textAlign: "center",
						color: "var(--ink-body)",
					}}
				>
					<p>此賣家目前沒有商品</p>
				</div>
			)}

			<Pagination
				current={currentPage}
				total={totalPages}
				onPageChange={setPage}
			/>
		</>
	);
}
