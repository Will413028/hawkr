"use client";

/**
 * 商城列表 Layout（client）— 把 SSR 派生資料與 client 互動分離。
 *
 * 設計目的：消除「server 商城列表」與「seller 預覽商城列表」雙份維護
 * 的問題。所有 sidebar / sort row 的 JSX 全部聚於此；caller 把 SSR 已
 * 算好的 chips / link maps / hidden field 值傳進來即可，本 layout 不
 * 再讀 searchParams。
 *
 * 預覽模式（previewMode=true）下：
 *   - 價格範圍 form 仍 action="/market/products"（與 sidebar 其餘 link 行為一致：
 *     點 chip / 認證 / hashtag 會跳到正式商城）
 *   - 不影響 MarketClientShell（tabs tap 抑制由 caller 傳 previewMode 進 Shell）
 */

import * as React from "react";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import AppliedFilters, {
	type AppliedChip,
} from "@/features/market/components/AppliedFilters";
import CategoryFilterPopover from "@/features/market/components/CategoryFilterPopover";
import MarketClientShell from "@/features/market/components/MarketClientShell";
import MarketProductsList, {
	type MarketProductItem,
} from "@/features/market/components/MarketProductsList";
import { useLocalizedHref } from "@/lib/intl/use-router";
import type { CategoryNode, MarketFetchParams } from "@/lib/market/listFilters";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

export interface SidebarHashtagLink {
	name: string;
	label: string;
	href: string;
	active: boolean;
}

export interface MarketProductsListLayoutProps {
	categories: CategoryNode[];
	sellerStories: StoryItem[];

	// 當前篩選狀態（hidden field 與 active 標記用）
	currentCategories: string[];
	currentPriceMin: string;
	currentPriceMax: string;
	currentHashtags: string[];
	currentSort: string;
	currentQuery: string;
	currentTab: "recommend" | "follow" | "nearby";
	fetchParams?: MarketFetchParams;
	pageSize?: number;

	// SSR 已展開的 chips / link maps
	appliedChips: AppliedChip[];
	clearAllHref: string;
	sortLinks: Array<{ value: string; label: string; href: string }>;
	hashtagLinks: SidebarHashtagLink[];

	/**
	 * The live route passes serialisable product props so this client boundary
	 * owns the grid element. Passing a Server Component as a node slot here can
	 * allocate a second postponed Fizz segment with the root navbar's id after a
	 * Cache Components postpone (historical; cacheComponents is off now). Seller
	 * preview remains fully client-owned and may
	 * keep supplying its existing node.
	 */
	products?: MarketProductItem[];
	initialDone?: boolean;
	/** Whether `products` are the authoritative page-1 for `fetchParams`
	 * (canonical/default), forwarded to the grid's query seed. Defaults to true. */
	seedInitialData?: boolean;
	hasActiveFilters?: boolean;
	productGrid?: React.ReactNode;

	previewMode?: boolean;
}

export default function MarketProductsListLayout({
	categories,
	sellerStories,
	currentCategories,
	currentPriceMin,
	currentPriceMax,
	currentHashtags,
	currentSort,
	currentQuery,
	currentTab,
	fetchParams,
	pageSize,
	appliedChips,
	clearAllHref,
	sortLinks,
	hashtagLinks,
	products,
	initialDone,
	seedInitialData = true,
	hasActiveFilters = false,
	productGrid,
	previewMode = false,
}: MarketProductsListLayoutProps) {
	const sidebar = (
		<>
			<AppliedFilters chips={appliedChips} clearAllHref={clearAllHref} />

			<CategoryFilterPopover tree={categories} selected={currentCategories} />

			<details open className="market-list-sidebar__section">
				<summary>價格範圍</summary>
				<PriceRangeForm
					currentCategories={currentCategories}
					currentPriceMin={currentPriceMin}
					currentPriceMax={currentPriceMax}
					currentHashtags={currentHashtags}
					currentSort={currentSort}
					currentQuery={currentQuery}
					currentTab={currentTab}
				/>
			</details>

			{hashtagLinks.length > 0 ? (
				<details open className="market-list-sidebar__section">
					<summary>熱門 hashtag</summary>
					<div className="market-list-sidebar__hashtags">
						{hashtagLinks.map((tag) => (
							<Link
								key={tag.name}
								href={tag.href}
								scroll={false}
								className="market-list-sidebar__hashtag"
								data-active={tag.active}
								aria-pressed={tag.active}
							>
								{tag.label}
							</Link>
						))}
					</div>
				</details>
			) : null}
		</>
	);

	const resolvedProductGrid =
		productGrid ??
		(products && fetchParams && pageSize && initialDone !== undefined ? (
			<MarketProductsList
				products={products}
				sortLinks={sortLinks}
				currentSort={currentSort}
				isFollowMode={currentTab === "follow"}
				hasActiveFilters={hasActiveFilters}
				fetchParams={fetchParams}
				pageSize={pageSize}
				initialDone={initialDone}
				seedInitialData={seedInitialData}
			/>
		) : null);

	return (
		<MarketClientShell
			stories={sellerStories}
			sidebar={sidebar}
			currentTab={currentTab}
			fetchParams={fetchParams}
			pageSize={pageSize}
			previewMode={previewMode}
			productGrid={resolvedProductGrid}
			sortLinks={sortLinks}
			currentSort={currentSort}
		/>
	);
}

type PriceRangeFormProps = Pick<
	MarketProductsListLayoutProps,
	| "currentCategories"
	| "currentPriceMin"
	| "currentPriceMax"
	| "currentHashtags"
	| "currentSort"
	| "currentQuery"
	| "currentTab"
>;

function PriceRangeForm({
	currentCategories,
	currentPriceMin,
	currentPriceMax,
	currentHashtags,
	currentSort,
	currentQuery,
	currentTab,
}: PriceRangeFormProps) {
	const router = useObservedRouter("market_products_list");
	const action = useLocalizedHref("/market/products");
	const [minimum, setMinimum] = React.useState(currentPriceMin);
	const [maximum, setMaximum] = React.useState(currentPriceMax);
	const [isPending, startTransition] = React.useTransition();
	const maximumRef = React.useRef<HTMLInputElement>(null);

	React.useEffect(() => {
		setMinimum(currentPriceMin);
		setMaximum(currentPriceMax);
		maximumRef.current?.setCustomValidity("");
	}, [currentPriceMax, currentPriceMin]);

	const handleSubmit = React.useCallback(
		(event: React.FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			const min = minimum.trim();
			const max = maximum.trim();
			const invalidRange = min && max && Number(min) > Number(max);
			maximumRef.current?.setCustomValidity(
				invalidRange ? "最高價不可低於最低價" : "",
			);
			if (invalidRange) {
				maximumRef.current?.reportValidity();
				return;
			}

			const next = new URLSearchParams();
			if (currentCategories.length)
				next.set("categories", currentCategories.join(","));
			if (currentHashtags.length)
				next.set("hashtags", currentHashtags.join(","));
			if (currentSort !== "推薦") next.set("sort", currentSort);
			if (currentQuery) next.set("q", currentQuery);
			if (currentTab !== "recommend") next.set("tab", currentTab);
			if (min) next.set("priceMin", min);
			if (max) next.set("priceMax", max);
			const search = next.toString();

			startTransition(() => {
				router.push(
					search ? `/market/products?${search}` : "/market/products",
					{ scroll: false },
				);
			});
		},
		[
			currentCategories,
			currentHashtags,
			currentQuery,
			currentSort,
			currentTab,
			maximum,
			minimum,
			router,
		],
	);

	return (
		<form
			className="market-list-sidebar__price"
			action={action}
			onSubmit={handleSubmit}
			aria-busy={isPending}
		>
			{currentCategories.length ? (
				<input
					type="hidden"
					name="categories"
					value={currentCategories.join(",")}
				/>
			) : null}
			{currentHashtags.length ? (
				<input
					type="hidden"
					name="hashtags"
					value={currentHashtags.join(",")}
				/>
			) : null}
			{currentSort !== "推薦" ? (
				<input type="hidden" name="sort" value={currentSort} />
			) : null}
			{currentQuery ? (
				<input type="hidden" name="q" value={currentQuery} />
			) : null}
			{currentTab !== "recommend" ? (
				<input type="hidden" name="tab" value={currentTab} />
			) : null}
			<div className="market-list-sidebar__price-fields">
				<div className="market-list-sidebar__price-field">
					<label htmlFor="price-min">最低價</label>
					<div className="market-list-sidebar__price-input">
						<span aria-hidden="true">NT$</span>
						<input
							id="price-min"
							type="number"
							name="priceMin"
							data-slot="control"
							inputMode="numeric"
							min={0}
							placeholder="不限"
							value={minimum}
							onChange={(event) => {
								maximumRef.current?.setCustomValidity("");
								setMinimum(event.currentTarget.value);
							}}
						/>
					</div>
				</div>
				<span
					className="market-list-sidebar__price-separator"
					aria-hidden="true"
				>
					–
				</span>
				<div className="market-list-sidebar__price-field">
					<label htmlFor="price-max">最高價</label>
					<div className="market-list-sidebar__price-input">
						<span aria-hidden="true">NT$</span>
						<input
							ref={maximumRef}
							id="price-max"
							type="number"
							name="priceMax"
							data-slot="control"
							inputMode="numeric"
							min={0}
							placeholder="不限"
							value={maximum}
							onChange={(event) => {
								maximumRef.current?.setCustomValidity("");
								setMaximum(event.currentTarget.value);
							}}
						/>
					</div>
				</div>
			</div>
			<Button
				type="submit"
				disabled={isPending}
				isLoading={isPending}
				loadingLabel="套用價格範圍中…"
				fullWidth
			>
				套用
			</Button>
		</form>
	);
}
