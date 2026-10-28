/**
 * Pure derivation of the `/market/products` list view from URL params.
 *
 * Extracted from the former server page so the exact same logic runs (a) on the
 * server with empty params — producing the canonical no-filter first page that
 * ships in the ISR HTML (SEO body + ItemList JSON-LD) — and (b) on the client,
 * reactively, as the buyer changes filters via the URL. One source of truth for
 * chips / active states / hrefs / fetchParams guarantees the server default and
 * the client-filtered views stay visually identical.
 *
 * No server-only imports: safe to call in a client component.
 */

import type { AppliedChip } from "@/features/market/components/AppliedFilters";
import type { SidebarHashtagLink } from "@/features/market/products/MarketProductsListLayout";
import {
	type CategoryNode,
	findCategoryId,
	findCategoryNode,
	type MarketFetchParams,
	mapSort,
	normaliseCategorySelections,
} from "@/lib/market/listFilters";

const SORT_OPTIONS: Array<{ value: string; label: string }> = [
	{ value: "推薦", label: "推薦" },
	{ value: "最新上架", label: "最新上架" },
	{ value: "銷量最高", label: "銷量最高" },
	{ value: "價格低到高", label: "價格低到高" },
	{ value: "價格高到低", label: "價格高到低" },
	{ value: "評分最高", label: "評分最高" },
];

/** Raw `/market/products` query params (all strings, as they arrive from the URL). */
export interface MarketListParams {
	q?: string;
	/** Legacy single-value category param, merged with `categories` (csv). */
	category?: string;
	categories?: string;
	priceMin?: string;
	priceMax?: string;
	hashtags?: string;
	sort?: string;
	tab?: string;
}

export interface MarketListData {
	categoryList: CategoryNode[];
	trendingHashtags: Array<{ name: string; label?: string }>;
}

export interface MarketListView {
	currentCategories: string[];
	currentPriceMin: string;
	currentPriceMax: string;
	currentHashtags: string[];
	currentSort: string;
	currentQuery: string;
	currentTab: "recommend" | "follow" | "nearby";
	isFollowTab: boolean;
	fetchParams: MarketFetchParams;
	hasActiveFilters: boolean;
	appliedChips: AppliedChip[];
	clearAllHref: string;
	sortLinks: Array<{ value: string; label: string; href: string }>;
	hashtagLinks: SidebarHashtagLink[];
}

function readCsv(value: string | undefined): string[] {
	return value
		? value
				.split(",")
				.map((v) => v.trim())
				.filter(Boolean)
		: [];
}

function findCategoryParentName(
	nodes: CategoryNode[],
	value: string,
	parent: string | null = null,
): string | null {
	for (const node of nodes) {
		if ((node?.id != null && String(node.id) === value) || node?.name === value)
			return parent;
		const children = Array.isArray(node?.children) ? node.children : [];
		const match = findCategoryParentName(children, value, node?.name ?? null);
		if (match !== null) return match;
	}
	return null;
}

export function deriveMarketListView(
	params: MarketListParams,
	data: MarketListData,
): MarketListView {
	const { categoryList, trendingHashtags } = data;

	// Merge legacy `category` + multi-select `categories` (csv), dedupe. Values
	// may be stable ids or names from older links; canonicalisation happens once
	// the category tree has loaded.
	const requestedCategories: string[] = (() => {
		const list: string[] = [];
		const seen = new Set<string>();
		const push = (name: string) => {
			const trimmed = name.trim();
			if (!trimmed || seen.has(trimmed)) return;
			seen.add(trimmed);
			list.push(trimmed);
		};
		if (params.category) push(params.category);
		if (params.categories) {
			for (const name of params.categories.split(",")) push(name);
		}
		return list;
	})();
	const currentPriceMin = params.priceMin ?? "";
	const currentPriceMax = params.priceMax ?? "";
	const currentHashtags = readCsv(params.hashtags);
	const currentSort = params.sort ?? "推薦";
	const currentQuery = params.q ?? "";
	const rawTab = (params.tab ?? "").toLowerCase();
	const currentTab: "recommend" | "follow" | "nearby" =
		rawTab === "follow" || rawTab === "nearby" ? rawTab : "recommend";
	const isFollowTab = currentTab === "follow";

	// Preserve other query params when changing one filter dimension.
	const buildHref = (overrides: Record<string, string | undefined>) => {
		const next = new URLSearchParams();
		const merged = { ...params, ...overrides } as Record<
			string,
			string | undefined
		>;
		for (const [k, v] of Object.entries(merged)) {
			if (v !== undefined && v !== "") next.set(k, v);
		}
		const qs = next.toString();
		return qs ? `/market/products?${qs}` : "/market/products";
	};

	const toggleHashtagHref = (tag: string) => {
		const has = currentHashtags.includes(tag);
		const next = has
			? currentHashtags.filter((t) => t !== tag)
			: [...currentHashtags, tag];
		return buildHref({ hashtags: next.length ? next.join(",") : undefined });
	};

	// New category-picker writes stable ids. Older links that carry names are
	// canonicalised here after the tree arrives, so both schemas keep working.
	const currentCategories = normaliseCategorySelections(
		categoryList,
		requestedCategories,
	);
	const hasActiveFilters = Boolean(
		currentCategories.length ||
			currentPriceMin ||
			currentPriceMax ||
			currentHashtags.length ||
			currentQuery,
	);
	// Resolve every selection token to a backend id, drop nulls, join as CSV.
	const categoryIdCsv: string | undefined = (() => {
		if (!currentCategories.length) return undefined;
		const ids: string[] = [];
		for (const token of currentCategories) {
			const id = findCategoryId(categoryList, token);
			if (id) ids.push(id);
		}
		return ids.length ? ids.join(",") : undefined;
	})();
	const categoryParentByToken = new Map<string, string | null>();
	for (const token of currentCategories) {
		categoryParentByToken.set(
			token,
			findCategoryParentName(categoryList, token),
		);
	}
	const removeCategoryHref = (token: string) => {
		const remaining = currentCategories.filter((c) => c !== token);
		return buildHref({
			category: undefined,
			categories: remaining.length ? remaining.join(",") : undefined,
		});
	};
	const priceMinNum = currentPriceMin ? Number(currentPriceMin) : undefined;
	const priceMaxNum = currentPriceMax ? Number(currentPriceMax) : undefined;
	const priceMinNumOpt =
		priceMinNum !== undefined && Number.isFinite(priceMinNum)
			? priceMinNum
			: undefined;
	const priceMaxNumOpt =
		priceMaxNum !== undefined && Number.isFinite(priceMaxNum)
			? priceMaxNum
			: undefined;
	const fetchParams: MarketFetchParams = {
		keyword: currentQuery,
		categoryId: categoryIdCsv,
		sort: mapSort(currentSort),
		priceMin: priceMinNumOpt,
		priceMax: priceMaxNumOpt,
		hashtags: currentHashtags.length ? currentHashtags : undefined,
		followedSellerOnly: isFollowTab || undefined,
	};

	const hashtagLinks: SidebarHashtagLink[] = trendingHashtags.map((tag) => ({
		name: tag.name,
		label: tag.label || `#${tag.name}`,
		href: toggleHashtagHref(tag.name),
		active: currentHashtags.includes(tag.name),
	}));

	// Applied chips — order preserved: keyword → category → price → hashtags.
	const appliedChips: AppliedChip[] = [];
	if (currentQuery) {
		appliedChips.push({
			key: "keyword",
			kind: "keyword",
			label: "關鍵字",
			value: currentQuery,
			href: buildHref({ q: undefined }),
			ariaLabel: `移除關鍵字 ${currentQuery}`,
		});
	}
	for (const token of currentCategories) {
		const node = findCategoryNode(categoryList, token);
		const name = node?.name ?? token;
		const parent = categoryParentByToken.get(token) ?? null;
		appliedChips.push({
			key: `cat-${token}`,
			kind: "category",
			label: "分類",
			value: name,
			crumb: parent ?? undefined,
			href: removeCategoryHref(token),
			ariaLabel: `移除分類 ${parent ? `${parent} > ${name}` : name}`,
		});
	}
	if (currentPriceMin || currentPriceMax) {
		const value = `${currentPriceMin ? `NT$ ${currentPriceMin}` : "NT$ 0"} – ${
			currentPriceMax ? `NT$ ${currentPriceMax}` : "不限"
		}`;
		appliedChips.push({
			key: "price",
			kind: "price",
			label: "價格",
			value,
			href: buildHref({ priceMin: undefined, priceMax: undefined }),
			ariaLabel: "移除價格範圍",
		});
	}
	for (const tag of currentHashtags) {
		appliedChips.push({
			key: `hashtag-${tag}`,
			kind: "hashtag",
			value: tag,
			href: toggleHashtagHref(tag),
			ariaLabel: `移除標籤 ${tag}`,
		});
	}
	const clearAllHref = (() => {
		const next = new URLSearchParams();
		if (currentSort !== "推薦") next.set("sort", currentSort);
		if (currentTab !== "recommend") next.set("tab", currentTab);
		const qs = next.toString();
		return qs ? `/market/products?${qs}` : "/market/products";
	})();

	const sortLinks = SORT_OPTIONS.map((opt) => ({
		value: opt.value,
		label: opt.label,
		href: buildHref({ sort: opt.value === "推薦" ? undefined : opt.value }),
	}));

	return {
		currentCategories,
		currentPriceMin,
		currentPriceMax,
		currentHashtags,
		currentSort,
		currentQuery,
		currentTab,
		isFollowTab,
		fetchParams,
		hasActiveFilters,
		appliedChips,
		clearAllHref,
		sortLinks,
		hashtagLinks,
	};
}
