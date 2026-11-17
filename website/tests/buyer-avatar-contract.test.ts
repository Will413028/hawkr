/**
 * Buyer-avatar wire contract — backend#2216 (G1 + C2).
 *
 * The backend collapsed NINE names for the same buyer avatar (`avatar`,
 * `object_key`, `user_avatar_key`, `avatarKey`, `targetAvatarKey`,
 * `buyerAvatarKey`, `userAvatarKey`, `userAvatar` on SSE, `senderAvatar` on WS)
 * into one flat pair: `avatarKey` (bare objectKey, 49 §6.1 SSOT) + `avatarUrl`
 * (the URL the Gateway composed FROM that key). It was a hard cut — the old
 * names are physically gone, so a frontend still reading one gets `undefined`
 * and renders a blank avatar.
 *
 * `GET/POST/PUT /app/buyers/me` moved further still: it used to return the raw
 * proto (snake_case tags, `created_at` as a Unix-second string) and now returns
 * a DTO whose keys are camelCase and whose timestamps are ISO 8601 UTC.
 *
 * Every fixture below is a VERBATIM slice of a live response captured on
 * 2026-08-05 from the backend at `697c989bd` (a locally-run Gateway wired to the
 * dev microservices), never a hand-written wire shape — same rule as
 * tests/media-objectkey-contract.test.ts, and for the same reason.
 *
 * Two things are asserted per surface:
 *
 *   1. **The new name resolves.** Both pair members are present in the fixture,
 *      so a normalizer that reverts to the URL twin turns this red.
 *   2. **The retired names do not.** A payload carrying ONLY the deleted names
 *      must produce nothing. Reviving any of them as a fallback — at any
 *      priority — turns this red, which is the property a simple rename test
 *      cannot give you.
 */

import assert from "node:assert/strict";
import test, { mock } from "node:test";

/* ------------------------------------- GET /public/products/{id}/reviews */

// Verbatim records[0] of GET /public/products/866353737479521547/reviews.
const LIVE_PRODUCT_REVIEW = {
	reviewId: "7484455658227120601",
	buyerId: "2623900520",
	productId: "866353737479521547",
	username: "陳**",
	avatarKey:
		"public/buyer_avatar/2623900520/20260719/008b97af-544d-4efb-8bc4-8f11e2b4e027.png",
	avatarUrl:
		"https://test-media.example.com/i/original/public/buyer_avatar/2623900520/20260719/008b97af-544d-4efb-8bc4-8f11e2b4e027.png",
	rating: "5",
	content: "R8-11 評論圖回歸測試",
	createdAt: "2026-07-19T03:54:44Z",
	imageKeys: ["public/order_review/2623900520/20260719/4e36099e.png"],
	imageUrls: [
		"https://test-media.example.com/i/original/public/order_review/2623900520/20260719/4e36099e.png",
	],
};

async function reviewsWith(record: Record<string, unknown>) {
	mock.reset();
	mock.module("@/lib/api/client", {
		exports: {
			authedFetch: async () => ({ code: 0, data: null }),
			apiClient: {
				public: {
					get: async () => ({
						code: 0,
						data: { records: [record], total: "1", page: "1", pageSize: "10" },
					}),
				},
			},
		},
	});
	const { fetchProductReviews } = await import(
		`../lib/reviews/repo.ts?buyer-avatar=${Math.random()}`
	);
	return fetchProductReviews("866353737479521547");
}

test("product review avatar reads avatarKey, not the URL twin", async () => {
	const { reviews } = await reviewsWith(LIVE_PRODUCT_REVIEW);
	assert.equal(reviews[0].avatarKey, LIVE_PRODUCT_REVIEW.avatarKey);
});

test("a review carrying only the retired avatar names yields nothing", async () => {
	const { reviews } = await reviewsWith({
		...LIVE_PRODUCT_REVIEW,
		avatarKey: undefined,
		avatarUrl: undefined,
		userAvatarKey: "public/buyer_avatar/2623900520/retired.png",
		userAvatarUrl: "https://cdn/retired.png",
		userAvatar: "https://cdn/retired-flat.png",
		avatar: "https://cdn/retired-bare.png",
	});
	assert.equal(reviews[0].avatarKey, undefined);
});
