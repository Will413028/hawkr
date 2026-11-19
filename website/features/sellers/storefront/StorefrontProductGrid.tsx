"use client";

import MasonryGrid, {
	type MasonryBreakpoint,
} from "@/components/shared/MasonryGrid";
import Pagination from "@/components/shared/Pagination";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import type { MarketProductItem } from "@/lib/market/types";

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

interface StorefrontProductGridProps {
	items: MarketProductItem[];
	currentPage: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	/** 賣家預覽：卡片不渲染收藏與購物車操作，點商品開唯讀抽屜。公開店面不傳。 */
	cardPreview?: boolean;
}

/** 店面的商品格與分頁；公開店面與賣家預覽共用的呈現。 */
export default function StorefrontProductGrid({
	items,
	currentPage,
	totalPages,
	onPageChange,
	cardPreview = false,
}: StorefrontProductGridProps) {
	return (
		<>
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
							previewMode={cardPreview}
							storefrontPreviewMode={cardPreview}
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
				onPageChange={onPageChange}
			/>
		</>
	);
}
