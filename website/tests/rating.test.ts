import assert from "node:assert/strict";
import { test } from "node:test";
import {
	formatRatingSummary,
	parseRating,
	storeRatingClause,
} from "../lib/products/rating";

test("missing and zero aggregate ratings are described as no buyer reviews", () => {
	for (const value of [undefined, null, "", 0, "0", Number.NaN]) {
		assert.equal(parseRating(value), null);
		assert.equal(formatRatingSummary(value), "尚無買家評價");
	}
});

test("a real aggregate rating is formatted without inventing a fallback", () => {
	assert.equal(formatRatingSummary(4.75), "評分 4.8");
	assert.equal(formatRatingSummary("5"), "評分 5.0");
});

test("storeRatingClause says no reviews only for a real zero, and omits unreadable stats", () => {
	assert.equal(storeRatingClause({ reviewCount: 0 }), "，尚無買家評價");
	assert.equal(
		storeRatingClause({ reviewCount: 3, ratingAverage: 4.5 }),
		"，評分 4.5",
	);
	assert.equal(storeRatingClause({}), "");
	assert.equal(storeRatingClause({ ratingAverage: 4.5 }), "");
	assert.equal(storeRatingClause({ reviewCount: 3 }), "");
});
