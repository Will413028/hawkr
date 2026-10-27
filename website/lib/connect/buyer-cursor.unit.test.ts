import { beforeEach, describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
	listFavorites: vi.fn(),
	listFollows: vi.fn(),
	listVisits: vi.fn(),
	listMemberCards: vi.fn(),
	listSpendingRecords: vi.fn(),
	checkFavorite: vi.fn(),
	checkFollow: vi.fn(),
}));

vi.mock("./buyer", () => ({ buyerAppClient: () => rpc }));

import { checkFavorite, fetchFavorites } from "../favorites/repo";
import { checkFollowed, fetchFollowing } from "../follow/repo";
import { fetchMemberCards, fetchSpendingRecords } from "../member-cards/repo";
import { fetchHistory } from "../visits/repo";

describe("buyer list cursor", () => {
	beforeEach(() => {
		for (const fn of Object.values(rpc)) fn.mockReset();
	});

	it("passes each supplied cursor in one request and returns the next cursor", async () => {
		rpc.listFavorites.mockResolvedValue({ favorites: [], nextPageToken: "f2" });
		rpc.listFollows.mockResolvedValue({ follows: [], nextPageToken: "s2" });
		rpc.listVisits.mockResolvedValue({ visits: [], nextPageToken: "v2" });
		rpc.listMemberCards.mockResolvedValue({
			memberCards: [],
			nextPageToken: "m2",
		});
		rpc.listSpendingRecords.mockResolvedValue({
			records: [],
			totalSpent: 0,
			totalRefunded: 0,
			nextPageToken: "r2",
		});

		expect((await fetchFavorites({ pageToken: "f1" })).nextPageToken).toBe(
			"f2",
		);
		expect((await fetchFollowing({ pageToken: "s1" })).nextPageToken).toBe(
			"s2",
		);
		expect((await fetchHistory({ pageToken: "v1" })).nextPageToken).toBe("v2");
		expect((await fetchMemberCards({ pageToken: "m1" })).nextPageToken).toBe(
			"m2",
		);
		expect(
			(await fetchSpendingRecords({ sellerId: "store", pageToken: "r1" }))
				.nextPageToken,
		).toBe("r2");

		expect(rpc.listFavorites).toHaveBeenCalledExactlyOnceWith({
			pageSize: 20,
			pageToken: "f1",
		});
		expect(rpc.listFollows).toHaveBeenCalledExactlyOnceWith({
			pageSize: 20,
			pageToken: "s1",
		});
		expect(rpc.listVisits).toHaveBeenCalledExactlyOnceWith({
			pageSize: 30,
			pageToken: "v1",
		});
		expect(rpc.listMemberCards).toHaveBeenCalledExactlyOnceWith({
			pageSize: 20,
			pageToken: "m1",
		});
		expect(rpc.listSpendingRecords).toHaveBeenCalledExactlyOnceWith({
			storeId: "store",
			pageSize: 20,
			pageToken: "r1",
		});
	});

	it("reads the generated favorite and follow flags", async () => {
		rpc.checkFavorite.mockResolvedValue({ favorited: true });
		rpc.checkFollow.mockResolvedValue({ following: true });
		expect(await checkFavorite("10011")).toBe(true);
		expect(await checkFollowed("9")).toBe(true);
		expect(rpc.checkFavorite).toHaveBeenCalledWith({ productId: "10011" });
		expect(rpc.checkFollow).toHaveBeenCalledWith({ storeId: "9" });
	});

	it("maps spending rows and preserves both coupon states", async () => {
		rpc.listSpendingRecords.mockResolvedValue({
			records: [
				{
					orderNumber: "O1",
					amount: 1190,
					refundedAmount: 0,
					couponApplied: true,
				},
				{
					orderNumber: "O2",
					amount: 705,
					refundedAmount: 0,
					couponApplied: false,
				},
			],
			totalSpent: 1895,
			totalRefunded: 0,
			nextPageToken: "",
		});
		const result = await fetchSpendingRecords({ sellerId: "9" });
		expect(result.records.map((row) => [row.orderSn, row.hasCoupon])).toEqual([
			["O1", true],
			["O2", false],
		]);
		expect(result.totalSpending).toBe("1895");
	});
});
