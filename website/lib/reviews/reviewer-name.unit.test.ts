import { create } from "@bufbuild/protobuf";
import { ReviewSchema } from "@hawkr/api/gen/hawkr/review/v1alpha1/review_pb";
import { describe, expect, it } from "vitest";
import { mapLiveReviews } from "@/features/product/deriveDetailLayoutProps";
import { sellerReviewItem } from "@/features/sellers/reviews/reviewItem";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { type Review, reviewFromWire } from "./repo";
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
		const ut = (m: { key: string }) => `[${m.key}]`;
		expect(reviewerName(ut, undefined)).toBe(
			"[review.ReviewAuthor.anonymous_name]",
		);
		expect(reviewerName(ut, "  ")).toBe("[review.ReviewAuthor.anonymous_name]");
		expect(reviewerName(ut, " 小明 ")).toBe("小明");
		expect(reviewerName(uiTextFor("zh-TW"), undefined)).toBe("匿名會員");
	});
});

describe("viewer's own review", () => {
	it("carries is_mine from the wire to every view model", () => {
		const wire = create(ReviewSchema, { id: "r", isMine: true });
		const r = reviewFromWire(wire);
		expect(r.isMine).toBe(true);
		expect(reviewFromWire(create(ReviewSchema, { id: "r" })).isMine).toBe(
			false,
		);
		expect(mapLiveReviews({ reviews: [r], total: 1 }).reviews[0].isMine).toBe(
			true,
		);
		expect(sellerReviewItem(r).isMine).toBe(true);
	});
});
