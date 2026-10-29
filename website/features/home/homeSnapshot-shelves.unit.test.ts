import { describe, expect, it } from "vitest";
import type { HomeTabBootstrapV1 } from "@/lib/api/tab-bootstrap";
import { buildHomeSnapshot } from "./homeSnapshot";

/**
 * Discovery shelves file products by the catalogue's own category, not by
 * substrings. The prod defect this pins: a 文旦禮盒 sold by a store named
 * 範例鮮果 showed up under 海鮮 ("鮮" matched the seller name) and under
 * 加工食品 ("禮盒" matched the name) while its breadcrumb said 水果／柚子／文旦.
 */

const product = (
	overrides: Record<string, unknown>,
): Record<string, unknown> => ({
	price: "399",
	priceMin: "399",
	priceMax: "399",
	imageKey: "public/x.png",
	...overrides,
});

const bootstrap = (
	records: Record<string, unknown>[],
	categoryGroups: HomeTabBootstrapV1["categoryGroups"],
): HomeTabBootstrapV1 => ({
	schemaVersion: "home.v2",
	hotProducts: [],
	products: { data: { records } },
	recommendedSellers: [],
	posts: { page: 1, pageSize: 8, total: 0, records: [] },
	categoryGroups,
});

const GROUPS: HomeTabBootstrapV1["categoryGroups"] = [
	{ id: "2", name: "水果", descendantIds: ["154", "763"] },
	{ id: "5", name: "海鮮", descendantIds: ["206", "207"] },
	{ id: "8", name: "加工食品", descendantIds: ["222", "227"] },
];

const shelf = (
	snapshot: ReturnType<typeof buildHomeSnapshot>,
	key: string,
): string[] =>
	(snapshot.discoveryShelves.find((s) => s.key === key)?.products ?? []).map(
		(p) => p.name,
	);

describe("home discovery shelves", () => {
	it("files products by their category, never by words in the name or seller", () => {
		const snapshot = buildHomeSnapshot(
			bootstrap(
				[
					product({
						id: "1",
						name: "「範例」東河文旦 禮盒裝",
						sellerName: "範例鮮果",
						categoryId: "154",
						categoryName: "柚子",
						hashtags: ["禮盒"],
					}),
					product({
						id: "2",
						name: "北海道干貝",
						sellerName: "小舖",
						categoryId: "207",
						categoryName: "蝦類",
					}),
					product({
						id: "3",
						name: "海味零嘴鮮蝦餅",
						sellerName: "小舖",
						categoryId: "222",
						categoryName: "乾貨",
					}),
				],
				GROUPS,
			),
		);
		expect(shelf(snapshot, "fruit")).toEqual(["「範例」東河文旦 禮盒裝"]);
		// No seafood keyword in the name, filed under 海鮮 → on the seafood shelf.
		expect(shelf(snapshot, "seafood")).toEqual(["北海道干貝"]);
		// "鮮蝦" in the name, but the catalogue files it under 加工食品.
		expect(shelf(snapshot, "pantry")).toEqual(["海味零嘴鮮蝦餅"]);
		expect(shelf(snapshot, "vegetables")).toEqual([]);
	});

	it("shows nothing on a category shelf when the tree was unsupplied", () => {
		const snapshot = buildHomeSnapshot(
			bootstrap(
				[
					product({
						id: "1",
						name: "北海道干貝",
						sellerName: "小舖",
						categoryId: "207",
					}),
				],
				[],
			),
		);
		expect(shelf(snapshot, "seafood")).toEqual([]);
	});

	it("keeps the weekend shelf on its hashtag alone", () => {
		const snapshot = buildHomeSnapshot(
			bootstrap(
				[
					product({
						id: "1",
						name: "火鍋肉片",
						sellerName: "小舖",
						categoryId: "198",
						hashtags: ["聚餐"],
					}),
					product({
						id: "2",
						name: "烤肉組合",
						sellerName: "小舖",
						categoryId: "198",
						hashtags: ["烤肉"],
					}),
				],
				GROUPS,
			),
		);
		expect(shelf(snapshot, "weekend")).toEqual(["火鍋肉片"]);
	});

	it("carries the leaf category onto every product card item", () => {
		const snapshot = buildHomeSnapshot(
			bootstrap(
				[
					product({
						id: "1",
						name: "文旦",
						sellerName: "小舖",
						categoryId: 154,
						categoryName: " 柚子 ",
					}),
					product({ id: "2", name: "無分類", sellerName: "小舖" }),
				],
				GROUPS,
			),
		);
		const [withCategory, without] = snapshot.newArrivalProducts;
		expect(withCategory.categoryId).toBe("154");
		expect(withCategory.categoryName).toBe("柚子");
		expect("categoryId" in without).toBe(false);
	});
});
