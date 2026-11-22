import { describe, expect, it } from "vitest";
import { mapLiveReviews } from "@/features/product/deriveDetailLayoutProps";
import { sellerReviewItem } from "@/features/sellers/reviews/reviewItem";
import type { Review } from "./repo";
import { reviewerName } from "./reviewer-name";

const review = (username?: string) =>
	({
		id: "r1",
		rating: 5,
		content: "x",
		imageUrls: [],
		createdAt: 0,
		username,
	}) as unknown as Review;

describe("anonymous reviewer", () => {
	it("keeps the data layer free of display text: no name stays undefined", () => {
		expect(
			mapLiveReviews({ reviews: [review()], total: 1 }).reviews[0].author,
		).toBeUndefined();
		expect(sellerReviewItem(review()).author).toBeUndefined();
		expect(sellerReviewItem(review("小明")).author).toBe("小明");
	});

	it("renders the fallback in one view-level function", () => {
		expect(reviewerName(undefined)).toBe("匿名");
		expect(reviewerName("  ")).toBe("匿名");
		expect(reviewerName(" 小明 ")).toBe("小明");
	});
});
