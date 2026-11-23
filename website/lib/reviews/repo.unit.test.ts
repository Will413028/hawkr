import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	ReviewReplySchema,
	ReviewSchema,
} from "@hawkr/api/gen/hawkr/review/v1alpha1/review_pb";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	listStoreReviews: vi.fn(),
	getStoreReviewStats: vi.fn(),
	getProductReviewStats: vi.fn(),
}));
vi.mock("../connect/review", () => ({
	reviewPublicClient: () => ({
		listStoreReviews: mocks.listStoreReviews,
		getStoreReviewStats: mocks.getStoreReviewStats,
		getProductReviewStats: mocks.getProductReviewStats,
	}),
	reviewAppClient: () => ({}),
}));

import {
	fetchProductReviewStats,
	fetchStoreReviewsAtPage,
	reviewFromWire,
	statsFromWire,
} from "./repo";

describe("statsFromWire", () => {
	it("takes count, average and distribution from the server; missing stars are 0", () => {
		expect(
			statsFromWire({
				count: 7,
				average: 4.5,
				distribution: [
					{ rating: 5, count: 5 },
					{ rating: 3, count: 2 },
				],
			}),
		).toEqual({
			totalCount: 7,
			averageRating: 4.5,
			rating1Count: 0,
			rating2Count: 0,
			rating3Count: 2,
			rating4Count: 0,
			rating5Count: 5,
		});
	});

	it("fetchProductReviewStats returns the server totals", async () => {
		mocks.getProductReviewStats.mockResolvedValue({
			count: 120,
			average: 4.2,
			distribution: [],
		});
		const stats = await fetchProductReviewStats("p1");
		expect(stats.totalCount).toBe(120);
		expect(mocks.getProductReviewStats).toHaveBeenCalledWith(
			{ productId: "p1" },
			expect.anything(),
		);
	});
});

describe("reviewFromWire", () => {
	it("maps reply and image templates", () => {
		const r = reviewFromWire(
			create(ReviewSchema, {
				id: "r1",
				rating: 5,
				price: BigInt(120),
				createTime: timestampFromDate(new Date("2026-01-02T00:00:00Z")),
				reply: create(ReviewReplySchema, {
					content: "thanks",
					replyTime: timestampFromDate(new Date("2026-01-03T00:00:00Z")),
				}),
				images: [
					{
						$typeName: "hawkr.common.v1alpha1.Image",
						uriTemplate: "https://x/{preset}/a",
					},
				] as never,
			}),
		);
		expect(r.reply).toEqual({
			content: "thanks",
			repliedAt: "2026-01-03T00:00:00.000Z",
		});
		expect(r.imageUrls).toEqual(["https://x/card/a"]);
		expect(r.price).toBe(120);
		expect(r.username).toBeUndefined();
	});
});

describe("fetchStoreReviewsAtPage", () => {
	it("walks page tokens to the requested page", async () => {
		mocks.listStoreReviews
			.mockResolvedValueOnce({ reviews: [], nextPageToken: "t2" })
			.mockResolvedValueOnce({ reviews: [], nextPageToken: "t3" })
			.mockResolvedValueOnce({ reviews: [], nextPageToken: "" });
		await fetchStoreReviewsAtPage("s1", 3);
		expect(
			mocks.listStoreReviews.mock.calls.map((c) => c[0].pageToken),
		).toEqual(["", "t2", "t3"]);
	});

	it("a page beyond the end is an empty page, but a failing page throws", async () => {
		mocks.listStoreReviews.mockReset();
		mocks.listStoreReviews.mockResolvedValueOnce({
			reviews: [],
			nextPageToken: "",
		});
		expect(await fetchStoreReviewsAtPage("s1", 4)).toEqual({
			reviews: [],
			nextPageToken: "",
		});
		mocks.listStoreReviews.mockRejectedValueOnce(new Error("503"));
		await expect(fetchStoreReviewsAtPage("s1", 2)).rejects.toThrow("503");
	});
});
