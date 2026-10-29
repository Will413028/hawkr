import { create } from "@bufbuild/protobuf";
import {
	StoreCardSchema,
	StoreCardStatus,
} from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import {
	RecommendedSellerSchema,
	SuggestRecommendedSellersResponseSchema,
} from "@hawkr/api/gen/hawkr/discovery/v1alpha1/discovery_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const suggest = vi.hoisted(() => vi.fn());
vi.mock("@/lib/connect/discovery", () => ({
	discoveryPublicClient: () => ({ suggestRecommendedSellers: suggest }),
}));

import { fetchRecommendedStores } from "./recommended";

const seller = (over: Record<string, unknown>, followerCount?: number) =>
	create(RecommendedSellerSchema, {
		store: create(StoreCardSchema, {
			id: "1",
			storeCode: "shop1",
			name: "店",
			status: StoreCardStatus.OPEN,
			townCode: "63000010",
			...over,
		}),
		followerCount,
	});

beforeEach(() => {
	suggest.mockReset();
});

describe("fetchRecommendedStores", () => {
	it("asks for the requested number and keeps the server order", async () => {
		suggest.mockResolvedValue(
			create(SuggestRecommendedSellersResponseSchema, {
				sellers: [
					seller({ id: "2", storeCode: "b", name: "B" }, 5),
					seller({ id: "1", storeCode: "a", name: "A" }),
				],
			}),
		);
		const out = await fetchRecommendedStores(24);
		expect(suggest).toHaveBeenCalledWith(
			{ maxResults: 24 },
			{ signal: undefined },
		);
		expect(out.map((s) => s.id)).toEqual(["2", "1"]);
		expect(out[0].followerCount).toBe(5);
		expect(out[1].followerCount).toBeUndefined();
		expect(out[0].townCode).toBe("63000010");
	});

	it("skips entries that are not open or lack a name or store code", async () => {
		suggest.mockResolvedValue(
			create(SuggestRecommendedSellersResponseSchema, {
				sellers: [
					seller({ id: "1", status: StoreCardStatus.PAUSED }),
					seller({ id: "2", status: StoreCardStatus.UNSPECIFIED }),
					seller({ id: "3", name: undefined }),
					seller({ id: "4", storeCode: "" }),
					seller({ id: "5" }),
				],
			}),
		);
		const out = await fetchRecommendedStores(10);
		expect(out.map((s) => s.id)).toEqual(["5"]);
	});

	it("leaves rating fields unset when the card has none", async () => {
		suggest.mockResolvedValue(
			create(SuggestRecommendedSellersResponseSchema, {
				sellers: [
					seller({ id: "1" }),
					seller({ id: "2", reviewCount: 0 }),
					seller({ id: "3", reviewCount: 4, ratingAverage: 4.5 }),
				],
			}),
		);
		const out = await fetchRecommendedStores(10);
		expect(out[0].reviewCount).toBeUndefined();
		expect(out[0].ratingAverage).toBeUndefined();
		expect(out[1].reviewCount).toBe(0);
		expect(out[1].ratingAverage).toBeUndefined();
		expect(out[2]).toMatchObject({ reviewCount: 4, ratingAverage: 4.5 });
	});
});
