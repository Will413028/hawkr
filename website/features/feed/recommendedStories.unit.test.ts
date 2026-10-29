import { describe, expect, it } from "vitest";
import type { RecommendedStore } from "@/lib/discovery/recommended";
import { recommendedStoriesFromStores } from "./recommendedStories";

const store = (over: Partial<RecommendedStore>): RecommendedStore => ({
	id: "1",
	storeCode: "shop1",
	name: "店",
	logoUrl: "https://img/avatar",
	coverUrl: "",
	townCode: "",
	...over,
});

describe("recommendedStoriesFromStores", () => {
	it("carries the store code and takes city and district from the town code", () => {
		const [item] = recommendedStoriesFromStores([
			store({ townCode: "63000010" }),
		]);
		expect(item.storeCode).toBe("shop1");
		expect(item.id).toBe("1");
		expect(item.city).toBe("臺北市");
		expect(item.district).toBeTruthy();
	});

	it("leaves the region empty for an unset or unknown town code", () => {
		const items = recommendedStoriesFromStores([
			store({ id: "1", townCode: "" }),
			store({ id: "2", townCode: "00000000" }),
		]);
		expect(items.map((i) => i.city)).toEqual([undefined, undefined]);
	});

	it("keeps the server order and caps the rail at 12", () => {
		const stores = Array.from({ length: 15 }, (_, i) =>
			store({ id: String(i + 1) }),
		);
		const items = recommendedStoriesFromStores(stores);
		expect(items).toHaveLength(12);
		expect(items.map((i) => i.id)).toEqual(
			stores.slice(0, 12).map((s) => s.id),
		);
	});
});
