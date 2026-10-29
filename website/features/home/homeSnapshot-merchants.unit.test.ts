import { describe, expect, it } from "vitest";
import type { HomeTabBootstrapV1 } from "@/lib/api/tab-bootstrap";
import type { RecommendedStore } from "@/lib/discovery/recommended";
import { buildHomeSnapshot } from "./homeSnapshot";

const store = (over: Partial<RecommendedStore>): RecommendedStore => ({
	id: "1",
	storeCode: "shop1",
	name: "店",
	logoUrl: "https://img/logo",
	coverUrl: "https://img/cover",
	townCode: "63000010",
	...over,
});

const bootstrap = (
	recommendedSellers: RecommendedStore[],
	hotProducts: HomeTabBootstrapV1["hotProducts"] = [],
): HomeTabBootstrapV1 => ({
	schemaVersion: "home.v2",
	hotProducts,
	products: { data: { records: [] } },
	recommendedSellers,
	posts: { page: 1, pageSize: 8, total: 0, records: [] },
	categoryGroups: [],
});

describe("home recommended merchants", () => {
	it("keeps the server order instead of re-ranking by profile completeness", () => {
		const { mappedSellers } = buildHomeSnapshot(
			bootstrap([
				store({ id: "1", name: "無封面", coverUrl: "" }),
				store({ id: "2", name: "有封面", ratingAverage: 5, reviewCount: 9 }),
			]),
		);
		expect(mappedSellers.map((s) => s.name)).toEqual(["無封面", "有封面"]);
	});

	it("builds the card from the store card: code link, region, rating, followers", () => {
		const { mappedSellers } = buildHomeSnapshot(
			bootstrap([
				store({
					ratingAverage: 4.5,
					reviewCount: 12,
					followerCount: 30,
				}),
			]),
		);
		expect(mappedSellers[0]).toMatchObject({
			id: "1",
			href: "/sellers/shop1",
			logo_url: "https://img/logo",
			image: "https://img/cover",
			rating: 4.5,
			reviewCount: 12,
			followers: 30,
		});
		expect(mappedSellers[0].location).toContain("臺北市");
		expect(mappedSellers[0].location).not.toContain("本週推薦");
		expect(mappedSellers[0]).not.toHaveProperty("certified");
		expect(mappedSellers[0]).not.toHaveProperty("prize");
	});

	it("leaves rating and followers unset when the server did not supply them", () => {
		const { mappedSellers } = buildHomeSnapshot(bootstrap([store({})]));
		expect(mappedSellers[0].rating).toBeUndefined();
		expect(mappedSellers[0].reviewCount).toBeUndefined();
		expect(mappedSellers[0].followers).toBeUndefined();
	});

	it("shows at most 8 merchants", () => {
		const many = Array.from({ length: 12 }, (_, i) => store({ id: String(i) }));
		expect(buildHomeSnapshot(bootstrap(many)).mappedSellers).toHaveLength(8);
	});
});

describe("home hot products", () => {
	it("feeds the editor shelf from hotProducts first", () => {
		const { editorRecommendedProducts } = buildHomeSnapshot(
			bootstrap(
				[],
				[{ id: "hot-1", name: "熱銷", storeId: "9", sellerName: "店" }],
			),
		);
		expect(editorRecommendedProducts[0]?.id).toBe("hot-1");
	});

	it("leaves the location empty when the town code is unknown; the label is the view's job", () => {
		const { mappedSellers } = buildHomeSnapshot(
			bootstrap([store({ townCode: "" })]),
		);
		expect(mappedSellers[0].location).toBe("");
	});
});
