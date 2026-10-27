import assert from "node:assert/strict";
import test from "node:test";
import type { PostListItem } from "../lib/api/openapi-types.ts";
import {
	type FeedHeadPage,
	feedHeadRequiresReplacement,
	mergeFeedHeadPages,
} from "../lib/feed/headRefresh.ts";
import {
	type MarketHeadPage,
	marketHeadRequiresReplacement,
	mergeMarketHeadPages,
	preserveMarketBuyerState,
} from "../lib/market/headRefresh.ts";
import type { MarketProductItem } from "../lib/market/types.ts";
import {
	effectiveMainTabStaleMs,
	shouldStartAutomaticMainTabRefresh,
} from "../lib/navigation/main-tab-freshness.ts";

function post(id: string, likeCount: number): PostListItem {
	return {
		id,
		sellerId: "seller",
		storeCode: "2685914682",
		sellerName: "Seller",
		logoKey: "",
		content: id,
		imageKeys: [],
		product: null,
		commentCount: 0,
		likeCount,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 0,
		tags: [],
	};
}

function product(id: string, price: string, stock: number): MarketProductItem {
	return {
		id,
		name: `product-${id}`,
		image: "",
		sellerName: "Seller",
		origin: "Taiwan",
		price,
		isOnSale: false,
		hashtags: [],
		unit: "份",
		rating: null,
		reviews: 0,
		href: `/products/${id}`,
		priceValue: Number(price),
		createdAt: 0,
		salesCount: 0,
		stock,
		isFavorited: true,
		favoriteCount: 1,
	};
}

test("automatic tab refresh respects freshness, retry and network budgets", () => {
	const base = {
		now: 100_000,
		freshAt: 80_000,
		lastAttemptAt: 0,
		refreshing: false,
		baseStaleMs: 15_000,
		online: true,
		visible: true,
	};
	assert.equal(shouldStartAutomaticMainTabRefresh(base), true);
	assert.equal(
		shouldStartAutomaticMainTabRefresh({ ...base, saveData: true }),
		false,
	);
	assert.equal(
		shouldStartAutomaticMainTabRefresh({
			...base,
			effectiveType: "2g",
		}),
		false,
	);
	assert.equal(effectiveMainTabStaleMs(15_000, "3g"), 30_000);
	assert.equal(
		shouldStartAutomaticMainTabRefresh({
			...base,
			effectiveType: "3g",
		}),
		false,
	);
	assert.equal(
		shouldStartAutomaticMainTabRefresh({
			...base,
			lastAttemptAt: 95_000,
		}),
		false,
	);
});

test("feed head refresh updates existing cards and buffers new ids without replaying pages", () => {
	const pages: FeedHeadPage[] = [
		{
			records: [post("a", 1), post("b", 2)],
			page: 1,
			pageSize: 2,
			hasMore: true,
		},
		{
			records: [post("c", 3), post("d", 4)],
			page: 2,
			pageSize: 2,
			hasMore: true,
		},
	];
	const pending = mergeFeedHeadPages(
		pages,
		[post("new", 10), post("a", 99)],
		false,
	);
	assert.deepEqual(
		pending.pages.map((page) => page.records.map((p) => p.id)),
		[
			["a", "b"],
			["c", "d"],
		],
	);
	assert.equal(pending.pages[0].records[0].likeCount, 99);
	assert.deepEqual(
		pending.newRecords.map((record) => record.id),
		["new"],
	);

	const applied = mergeFeedHeadPages(pending.pages, pending.newRecords, true);
	assert.deepEqual(
		applied.pages.map((page) => page.records.map((p) => p.id)),
		[
			["new", "a", "b"],
			["c", "d"],
		],
	);
	assert.equal(applied.pages[1].page, 2, "next-page cursor metadata survives");
	assert.equal(
		applied.pages.flatMap((page) => page.records).length,
		5,
		"prepending new posts must not drop already loaded tail records",
	);
	assert.equal(
		feedHeadRequiresReplacement(pages[0].records, [
			post("a", 99),
			post("b", 2),
		]),
		false,
		"engagement counters update in place without a layout replacement",
	);
	assert.equal(
		feedHeadRequiresReplacement(pages[0].records, [
			{ ...post("a", 99), content: "edited" },
			post("b", 2),
		]),
		true,
	);
});

test("market head refresh reconciles commerce fields without moving cards until accepted", () => {
	const oldA = product("a", "100", 8);
	const oldB = product("b", "200", 4);
	const pages: MarketHeadPage[] = [
		{ products: [oldA, oldB], page: 1, hasMore: true },
	];
	const freshA = {
		...product("a", "80", 0),
		name: "renamed-by-stale-list",
		isFavorited: false,
		favoriteCount: 9,
		isOnSale: true,
		originalPrice: "100",
	};
	const pending = mergeMarketHeadPages(
		pages,
		[product("new", "50", 5), freshA],
		false,
	);
	assert.deepEqual(
		pending.pages[0].products.map((item) => item.id),
		["a", "b"],
	);
	assert.equal(pending.pages[0].products[0].price, "80");
	assert.equal(pending.pages[0].products[0].stock, 0);
	assert.equal(pending.pages[0].products[0].favoriteCount, 9);
	assert.equal(pending.pages[0].products[0].name, oldA.name);
	assert.equal(
		pending.pages[0].products[0].isFavorited,
		true,
		"anonymous refresh cannot overwrite buyer-local favourite state",
	);
	assert.deepEqual(
		pending.newProducts.map((item) => item.id),
		["new"],
	);

	const applied = mergeMarketHeadPages(
		pending.pages,
		pending.newProducts,
		true,
	);
	assert.deepEqual(
		applied.pages[0].products.map((item) => item.id),
		["new", "a", "b"],
	);
	assert.equal(applied.pages[0].hasMore, true);
	assert.equal(
		marketHeadRequiresReplacement(pages[0].products, [
			{ ...oldA, price: "80", stock: 0 },
			oldB,
		]),
		false,
		"fixed-size commerce fields reconcile without replacing the grid",
	);
	assert.equal(
		marketHeadRequiresReplacement(pages[0].products, [oldB]),
		true,
		"membership changes require an authoritative page replacement",
	);
	const buyerSafe = preserveMarketBuyerState(
		[
			{
				products: [{ ...freshA, isFavorited: false }],
				page: 1,
				hasMore: false,
			},
		],
		pages,
	);
	assert.equal(buyerSafe[0].products[0].isFavorited, true);
});
