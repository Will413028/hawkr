import assert from "node:assert/strict";
import test, { mock } from "node:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";

/**
 * `GET /public/sellers/{id}/reviews` returns the gateway's PublicSellerReviewItem
 * (gateway/internal/handler/order_review.go): buyer identity arrives as
 * `avatarKey`/`avatarUrl` and attached photos as `imageKeys`/`imageUrls`
 * (49 §6.1 paired objectKey + display URL). ReviewList reads `avatar`
 * and `images`, so `fetchSellerReviews` owns the alias mapping — when it
 * lapses the tab silently degrades (issue #411: every buyer avatar fell back to a
 * name initial; the photo array was the same defect one field over, rendering no
 * attached photos at all). Assert against the real wire shape, not the frontend's.
 *
 * The URL twins are NOT a fallback (#430 D4): `ImageURLs` is
 * `buildMediaURLs(base, ImageKeys)` (order_review.go:188-191), so it is derived
 * from the very keys we already read and can never carry a photo the keys lack.
 * The "only the retired names" case below is what keeps a fallback from coming
 * back.
 */
const WIRE_RECORD = {
	id: "7484455658227120601",
	content: "很新鮮",
	rating: 5,
	userName: "林小明",
	createdAt: "2026-07-19T12:00:00Z",
	avatarKey: "public/buyer_avatar/2623900520/20260719/008b97.png",
	avatarUrl:
		"https://test-media.example.com/i/original/public/buyer_avatar/2623900520/20260719/008b97.png",
	imageKeys: [
		"public/order_review/2623900520/20260719/4e36099e.png",
		"public/order_review/2623900520/20260719/52119bb6.png",
	],
	imageUrls: [
		"https://test-media.example.com/i/original/public/order_review/2623900520/20260719/4e36099e.png",
		"https://test-media.example.com/i/original/public/order_review/2623900520/20260719/52119bb6.png",
	],
};

async function fetchWith(record: Record<string, unknown>) {
	mock.reset();
	mock.module("@/lib/api/client", {
		exports: {
			apiClient: {
				public: {
					get: async () => ({
						code: 0,
						data: { records: [record], total: 1 },
					}),
				},
			},
		},
	});
	const { fetchSellerReviews } = await import(
		`../lib/api/index.ts?seller-reviews-media=${Math.random()}`
	);
	return fetchSellerReviews("2685914682");
}

test("fetchSellerReviews folds the gateway's key/url pairs onto the fields ReviewList reads", async () => {
	const { records } = await fetchWith(WIRE_RECORD);
	assert.equal(records.length, 1);

	// Bare objectKey preferred (49 §231); Avatar/buildImageUrl resolve either form.
	assert.equal(records[0].avatar, WIRE_RECORD.avatarKey);
	assert.deepEqual(records[0].images, WIRE_RECORD.imageKeys);
});

test("a review with no attached photos yields no photo array", async () => {
	const { imageKeys, imageUrls, ...withoutPhotos } = WIRE_RECORD;
	const { records } = await fetchWith(withoutPhotos);
	assert.equal(records[0].avatar, WIRE_RECORD.avatarKey);
	assert.equal(records[0].images, undefined);
});

test("the derived imageUrls twin is not a fallback for an empty imageKeys", async () => {
	const { records } = await fetchWith({ ...WIRE_RECORD, imageKeys: [] });
	// `imageUrls` is composed FROM `imageKeys`, so "keys empty, urls populated"
	// is not a shape the gateway can emit — reading it could only ever mask a
	// contract break. Empty keys means no photos, full stop.
	assert.equal(records[0].images, undefined);
});

test("a payload carrying only the retired photo field names yields nothing", async () => {
	const { imageKeys, imageUrls, ...base } = WIRE_RECORD;
	const { records } = await fetchWith({
		...base,
		// Every candidate #430 D4 deleted, all populated at once. Any one of them
		// coming back as a fallback — at any priority — turns this red.
		images: ["public/order_review/legacy-images.png"],
		photos: ["public/order_review/legacy-photos.png"],
		imageUrls: [
			"https://test-media.example.com/i/original/public/order_review/legacy-urls.png",
		],
	});
	assert.equal(records[0].images, undefined);
});

test("ReviewList renders the attached photos and the buyer avatar", async () => {
	const { records } = await fetchWith(WIRE_RECORD);
	// Drop the client stub before pulling ReviewList in — its module graph reaches
	// other real consumers of `lib/api/client` that the stub does not implement.
	mock.reset();
	// Next-coupled leaves stand in as plain tags; this asserts which URLs reach the
	// image/link slots, not how those two components render them.
	mock.module("@/components/shared/AppImage", {
		exports: {
			default: (props: { src?: string; alt?: string }) =>
				React.createElement("img", { src: props.src, alt: props.alt }),
		},
	});
	mock.module("@/components/shared/Link", {
		exports: {
			default: (props: { href?: unknown; children?: React.ReactNode }) =>
				React.createElement(
					"a",
					{ href: String(props.href ?? "") },
					props.children,
				),
		},
	});
	const { default: ReviewList } = await import(
		`../features/sellers/reviews/ReviewList.tsx?render=${Math.random()}`
	);
	const markup = renderToStaticMarkup(
		React.createElement(ReviewList, { reviews: records }),
	);

	for (const key of WIRE_RECORD.imageKeys) {
		assert.ok(
			markup.includes(encodeURI(key)) || markup.includes(key),
			`expected review photo ${key} in markup`,
		);
	}
	// #411's own regression guard: the avatar must be an <img>, not a name initial.
	assert.ok(
		markup.includes(WIRE_RECORD.avatarKey.split("/").pop() as string),
		"expected the buyer avatar image in markup",
	);
});
