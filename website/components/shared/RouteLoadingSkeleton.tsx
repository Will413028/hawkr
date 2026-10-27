import "@/styles/components/route-loading-skeleton.css";
import "@/styles/components/route-loading-layout.css";
import "@/styles/components/route-loading-variants.css";
import type { CSSProperties } from "react";
import {
	AuthSkeleton,
	CheckoutSkeleton,
	DefaultSkeleton,
	HomeSkeleton,
	SearchSkeleton,
	SellerStorefrontSkeleton,
} from "./RouteLoadingVariants";
import RouteStorySkeleton from "./RouteStorySkeleton";

/**
 * How many placeholder cards a grid reserves. These are HEIGHT reservations,
 * not a promise about the result count — a wide screen renders more real cards
 * than this, so the page still grows a little when the data lands. Raising the
 * default is not free (each card is DOM the browser lays out during a
 * navigation), so callers that know their column count pass `columns`.
 */
const DEFAULT_SKELETON_CARDS = 6;
/** Horizontal shelves (home rail, feed grid) show four across at most. */
const SHELF_CARDS = Array.from({ length: 4 }, (_, index) => index);
const PRODUCT_SECTION_CARDS = Array.from({ length: 3 }, (_, index) => index);
const PRODUCT_THUMBS = Array.from({ length: 5 }, (_, index) => index);
const FILTER_BLOCKS = Array.from({ length: 5 }, (_, index) => index);
const WORKSPACE_ROWS = Array.from({ length: 6 }, (_, index) => index);

export type RouteLoadingVariant =
	| "generic"
	| "auth"
	| "checkout"
	| "feed"
	| "home"
	| "market-list"
	| "product-detail"
	| "search"
	| "seller-storefront"
	| "workspace";

export interface RouteLoadingScope {
	left: number;
	right: number;
	top: number;
	bottom: number;
}

export default function RouteLoadingSkeleton({
	page = false,
	scope,
	variant = "generic",
	columns,
}: {
	page?: boolean;
	scope?: RouteLoadingScope | null;
	variant?: RouteLoadingVariant;
	/**
	 * Columns the destination grid renders at the CURRENT viewport, when the
	 * caller knows it. The card count is derived as two rows of that, so a wide
	 * screen reserves a realistic height instead of the fixed six cards that
	 * under-reserved every desktop navigation. Omit it and the defaults apply.
	 */
	columns?: number;
}) {
	let className = page
		? "route-loading-skeleton route-loading-skeleton--page"
		: "route-loading-skeleton";
	if (scope) className += " route-loading-skeleton--scoped";
	if (variant !== "generic") className += ` route-loading-skeleton--${variant}`;
	const style = scope
		? ({
				"--route-loading-left": `${scope.left}px`,
				"--route-loading-right": `${scope.right}px`,
				"--route-loading-top": `${scope.top}px`,
				"--route-loading-bottom": `${scope.bottom}px`,
			} as CSSProperties)
		: undefined;

	return (
		<div
			aria-label="頁面載入中"
			aria-live="polite"
			className={className}
			role="status"
			style={style}
		>
			<SkeletonByVariant variant={variant} columns={columns} />
			<span className="visually-hidden">載入中</span>
		</div>
	);
}

function skeletonCardCount(columns: number | undefined): number {
	if (!columns || columns < 1) return DEFAULT_SKELETON_CARDS;
	return Math.min(24, Math.max(DEFAULT_SKELETON_CARDS, columns * 2));
}

function SkeletonByVariant({
	variant,
	columns,
}: {
	variant: RouteLoadingVariant;
	columns?: number;
}) {
	const cards = skeletonCardCount(columns);
	switch (variant) {
		case "auth":
			return <AuthSkeleton />;
		case "checkout":
			return <CheckoutSkeleton />;
		case "feed":
			return <FeedSkeleton />;
		case "home":
			return (
				<HomeSkeleton
					shelf={<ProductSkeletonCards variant="market" count={4} />}
				/>
			);
		case "market-list":
			return <MarketListSkeleton cards={cards} />;
		case "product-detail":
			return <ProductDetailSkeleton />;
		case "search":
			return (
				<SearchSkeleton
					cards={<ProductSkeletonCards variant="search" count={cards} />}
				/>
			);
		case "seller-storefront":
			return (
				<SellerStorefrontSkeleton
					cards={<ProductSkeletonCards variant="market" count={cards} />}
				/>
			);
		case "workspace":
			return <WorkspaceSkeleton />;
		default:
			return <DefaultSkeleton cards={<SkeletonCardGrid count={cards} />} />;
	}
}

/**
 * The card grid every list-shaped variant reserves. It was written out three
 * times (default / market+search / storefront) with the same four spans, so a
 * change to the card shape had to be made three times or the surfaces drifted.
 */
function SkeletonCardGrid({
	className = "route-loading-skeleton__grid",
	count = DEFAULT_SKELETON_CARDS,
	lines = 3,
}: {
	className?: string;
	/** Cards to reserve. Pass the column count × visible rows when known. */
	count?: number;
	/** Text lines under each card's media block. */
	lines?: 2 | 3;
}) {
	return (
		<div aria-hidden="true" className={className}>
			{Array.from({ length: count }, (_, index) => (
				// biome-ignore lint/suspicious/noArrayIndexKey: inert placeholders, never reordered
				<div className="route-loading-skeleton__card" key={index}>
					<span className="route-loading-skeleton__media" />
					<span className="route-loading-skeleton__line route-loading-skeleton__line--wide" />
					<span className="route-loading-skeleton__line" />
					{lines === 3 ? (
						<span className="route-loading-skeleton__line route-loading-skeleton__line--narrow" />
					) : null}
				</div>
			))}
		</div>
	);
}

function ProductSkeletonCards({
	variant,
	count,
}: {
	variant: "market" | "search";
	count?: number;
}) {
	return (
		<div
			aria-hidden="true"
			className={`route-loading-skeleton__grid route-loading-skeleton__product-grid route-loading-skeleton__product-grid--${variant}`}
		>
			{Array.from({ length: count ?? DEFAULT_SKELETON_CARDS }, (_, index) => (
				// biome-ignore lint/suspicious/noArrayIndexKey: inert placeholders, never reordered
				<div className="route-loading-skeleton__product-card" key={index}>
					<div className="route-loading-skeleton__product-media">
						<span className="route-loading-skeleton__media" />
					</div>
					<div className="route-loading-skeleton__product-body">
						<div className="route-loading-skeleton__product-name">
							<span className="route-loading-skeleton__line route-loading-skeleton__line--wide" />
							<span className="route-loading-skeleton__line" />
						</div>
						<span className="route-loading-skeleton__line route-loading-skeleton__product-seller" />
						<span className="route-loading-skeleton__line route-loading-skeleton__product-rating" />
						<span className="route-loading-skeleton__line route-loading-skeleton__product-price" />
						<div className="route-loading-skeleton__product-controls">
							<span className="route-loading-skeleton__line" />
							<span className="route-loading-skeleton__line" />
						</div>
					</div>
				</div>
			))}
		</div>
	);
}

/** Data-only fallback: keep the real search form, filters and navigation usable. */
export function ProductGridSkeleton({
	variant = "market",
	announce = true,
}: {
	variant?: "market" | "search";
	/** Search already announces its pending state in the result header. */
	announce?: boolean;
}) {
	if (!announce) {
		return (
			<div className="route-product-grid-skeleton" aria-hidden="true">
				<ProductSkeletonCards variant={variant} />
			</div>
		);
	}
	return (
		<div
			className="route-product-grid-skeleton"
			role="status"
			aria-label="商品載入中"
			aria-live="polite"
		>
			<ProductSkeletonCards variant={variant} />
			<span className="visually-hidden">載入中</span>
		</div>
	);
}

function MarketListSkeleton({ cards }: { cards: number }) {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--market"
		>
			<RouteStorySkeleton market />
			<div className="route-loading-skeleton__market-layout">
				<aside className="route-loading-skeleton__market-sidebar">
					<span className="route-loading-skeleton__line route-loading-skeleton__line--wide" />
					<span className="route-loading-skeleton__search-bar" />
					{FILTER_BLOCKS.map((block) => (
						<div className="route-loading-skeleton__filter-block" key={block}>
							<span />
							<span />
							<span />
						</div>
					))}
				</aside>
				<section className="route-loading-skeleton__market-main">
					<div className="route-loading-skeleton__market-toolbar">
						<span className="route-loading-skeleton__title" />
						<div>
							<span />
							<span />
							<span />
						</div>
					</div>
					<ProductSkeletonCards variant="market" count={cards} />
				</section>
			</div>
		</div>
	);
}

function ProductDetailSkeleton() {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--pdp"
		>
			<div className="route-loading-skeleton__breadcrumb">
				<span className="route-loading-skeleton__line route-loading-skeleton__line--short" />
				<span className="route-loading-skeleton__line route-loading-skeleton__line--narrow" />
				<span className="route-loading-skeleton__line route-loading-skeleton__line--short" />
			</div>
			<div className="route-loading-skeleton__pdp-grid">
				<div className="route-loading-skeleton__gallery">
					<span className="route-loading-skeleton__hero-media" />
					<div className="route-loading-skeleton__thumb-row">
						{PRODUCT_THUMBS.map((thumb) => (
							<span key={thumb} />
						))}
					</div>
				</div>
				<div className="route-loading-skeleton__purchase">
					<span className="route-loading-skeleton__title" />
					<span className="route-loading-skeleton__pill" />
					<span className="route-loading-skeleton__copy" />
					<span className="route-loading-skeleton__copy route-loading-skeleton__copy--short" />
					<div className="route-loading-skeleton__price-row">
						<span />
						<span />
					</div>
					<div className="route-loading-skeleton__actions">
						<span />
						<span />
					</div>
				</div>
			</div>
			<div className="route-loading-skeleton__section-grid">
				{PRODUCT_SECTION_CARDS.map((card) => (
					<div className="route-loading-skeleton__card" key={card}>
						<span className="route-loading-skeleton__line route-loading-skeleton__line--wide" />
						<span className="route-loading-skeleton__line" />
						<span className="route-loading-skeleton__line route-loading-skeleton__line--narrow" />
					</div>
				))}
			</div>
		</div>
	);
}

function FeedSkeleton() {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--feed"
		>
			<RouteStorySkeleton />
			<FeedSkeletonCards />
		</div>
	);
}

function FeedSkeletonCards() {
	return (
		<div
			aria-hidden="true"
			className="hkui-feed-grid route-loading-skeleton__feed-grid"
		>
			{SHELF_CARDS.map((card) => (
				<article
					className="hkui-feed-card route-loading-skeleton__feed-card"
					key={card}
				>
					<span className="hkui-feed-card__media route-loading-skeleton__media" />
					<div className="hkui-feed-card__body">
						<span className="route-loading-skeleton__line route-loading-skeleton__line--wide" />
						<span className="route-loading-skeleton__line" />
					</div>
				</article>
			))}
		</div>
	);
}

/** Inline feed recovery inherits the existing rail and content container. */
export function FeedGridSkeleton() {
	return (
		<div
			className="route-feed-grid-skeleton"
			role="status"
			aria-label="動態載入中"
			aria-live="polite"
		>
			<FeedSkeletonCards />
			<span className="visually-hidden">載入中</span>
		</div>
	);
}

function WorkspaceSkeleton() {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--workspace"
		>
			<div className="route-loading-skeleton__workspace-head">
				<div>
					<span className="route-loading-skeleton__pill" />
					<span className="route-loading-skeleton__title" />
				</div>
				<span />
			</div>
			<div className="route-loading-skeleton__workspace-stats">
				<span />
				<span />
				<span />
			</div>
			<div className="route-loading-skeleton__workspace-table">
				{WORKSPACE_ROWS.map((row) => (
					<div className="route-loading-skeleton__workspace-row" key={row}>
						<span />
						<span />
						<span />
						<span />
					</div>
				))}
			</div>
		</div>
	);
}
