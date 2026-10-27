import assert from "node:assert/strict";
import test from "node:test";
import { fetchMyReviews } from "../lib/reviews/repo";

// GET /app/me/reviews — `handler.OrderReviewRecord` (order_review.go:172-198).
//
// The fixture below is a live dev response (buyer 2623900520, 2026-08-05) with
// the field names updated to the post-#2235 contract: the商品封面 pair is
// `productImageKey`/`productImageUrl` (the `product` prefix stays because the
// same object also carries `imageKeys`, which are the buyer's review photos),
// and the envelope's page size is `pageSize`.
//
// The key (not the URL) is what the normalizer reads: `buildImageUrl` /
// `resolveCdnUrl` at each render site resolve a bare objectKey into the right
// preset, while a display URL has to be peeled back first.

/** `data` from GET /app/me/reviews?page=1&size=5 on dev. */
const LIVE_MY_REVIEWS = {
	records: [
		{
			id: "7484455658227120601",
			orderNo: "O556344112",
			orderItemId: "866525219615066919",
			buyerId: "2623900520",
			storeId: "2685914682",
			content:
				"R8-11 評論圖回歸測試：附兩張圖驗證 imageKeys 裸 key 與 productImageKey 快照回填。",
			skuName: "R8-01 媒體回歸測試商品（陳記）",
			productImageKey:
				"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
			productImageUrl:
				"https://test-media.example.com/i/original/public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
			price: "120",
			createdAt: "2026-07-19T03:54:44Z",
			productId: "866353737479521547",
			imageKeys: [
				"public/order_review/2623900520/20260719/4e36099e-f892-4a62-9dd6-af8b47f37eed.png",
				"public/order_review/2623900520/20260719/52119bb6-4664-4362-acc0-d6cf739e6447.png",
			],
			imageUrls: [
				"https://test-media.example.com/i/original/public/order_review/2623900520/20260719/4e36099e-f892-4a62-9dd6-af8b47f37eed.png",
				"https://test-media.example.com/i/original/public/order_review/2623900520/20260719/52119bb6-4664-4362-acc0-d6cf739e6447.png",
			],
			rating: 5,
		},
	],
	total: "1",
	page: "1",
	pageSize: "20",
};

function stubFetch<T>(
	envelopeData: unknown,
	run: () => Promise<T>,
): Promise<T> {
	const original = globalThis.fetch;
	globalThis.fetch = (async () =>
		new Response(JSON.stringify({ code: 0, data: envelopeData }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		})) as typeof fetch;
	return run().finally(() => {
		globalThis.fetch = original;
	});
}

test("fetchMyReviews maps the live wire and prefers the objectKey SSOT", async () => {
	const res = await stubFetch(LIVE_MY_REVIEWS, () => fetchMyReviews());

	assert.equal(res.total, 1);
	assert.equal(res.page, 1);
	assert.equal(res.pageSize, 20);

	const r = res.records[0];
	assert.equal(r.id, "7484455658227120601");
	assert.equal(r.orderNo, "O556344112");
	assert.equal(r.orderItemId, "866525219615066919");
	assert.equal(r.productId, "866353737479521547");
	assert.equal(r.rating, 5);
	assert.equal(r.price, "120");
	assert.equal(r.skuName, "R8-01 媒體回歸測試商品（陳記）");
	assert.ok(r.createdAt > 0);

	// §231: both members of each pair are on the wire — the KEY must win, or the
	// render path pays a peel-and-rebuild round trip and loses the preset.
	assert.equal(
		r.productImageKey,
		"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
	);
	assert.deepEqual(r.imageKeys, [
		"public/order_review/2623900520/20260719/4e36099e-f892-4a62-9dd6-af8b47f37eed.png",
		"public/order_review/2623900520/20260719/52119bb6-4664-4362-acc0-d6cf739e6447.png",
	]);
});

test("fetchMyReviews drops non-string image entries and tolerates a missing orderNo", async () => {
	const res = await stubFetch(
		{
			records: [
				{
					id: "1",
					skuName: "梨",
					imageKeys: ["public/order_review/a.png", 123, null, ""],
				},
			],
			total: "1",
		},
		() => fetchMyReviews(),
	);
	const r = res.records[0];
	assert.equal(r.orderNo, ""); // absent → non-clickable card
	assert.equal(r.skuName, "梨");
	assert.deepEqual(r.imageKeys, ["public/order_review/a.png", ""]);
	assert.equal(r.rating, 0);
	assert.equal(r.productImageKey, "");
});

test("fetchMyReviews ignores the field names the endpoint has never sent", async () => {
	// Guards the #430 D2 deletion (a 13-entry "guess the field name" candidate
	// list, live-probed to zero hits) AND the #2235 rename: `itemImageKey` /
	// `itemImageUrl` / `item_image_url` are the RETIRED names for this pair. If a
	// future edit restores any of them as a fallback the normalizer starts
	// guessing again, and this assertion turns red.
	const res = await stubFetch(
		{
			records: [
				{
					id: "2",
					productImageKey: "public/product/canonical.png",
					itemImageKey: "public/product/retired.png",
					itemImageUrl: "https://cdn/item.jpg",
					item_image_url: "https://cdn/item-snake.jpg",
					productImage: "https://cdn/product.jpg",
					imageUrl: "https://cdn/legacy.jpg",
					images: ["https://cdn/legacy-array.jpg"],
					productImages: ["https://cdn/nested.jpg"],
					orderItem: { imageUrl: "https://cdn/order-item.jpg" },
				},
			],
			total: "1",
		},
		() => fetchMyReviews(),
	);

	assert.equal(res.records[0]?.productImageKey, "public/product/canonical.png");
	assert.deepEqual(res.records[0]?.imageKeys, []);
});

test("fetchMyReviews：只餵舊 snake_case 欄名 → 全部讀不到（禁止雙讀 fallback）", async () => {
	// backend#2235：/app/me/reviews 的 body 已全面 camelCase，且分頁欄從
	// `page_size` 改為 `pageSize`。這裡刻意只餵舊欄名——任何 `?? d.page_size`
	// 之類的 fallback 復活都會讓這條轉紅。
	const res = await stubFetch(
		{
			records: [
				{
					id: "3",
					order_no: "O556344112",
					order_item_id: "866525219615066919",
					product_id: "866353737479521547",
					sku_name: "梨",
					created_at: "2026-07-19T03:54:44Z",
				},
			],
			total: "1",
			page: "1",
			page_size: "99",
		},
		() => fetchMyReviews({ pageSize: 20 }),
	);

	assert.equal(res.pageSize, 20); // 讀不到 page_size → 退回呼叫端傳入的預設
	const r = res.records[0];
	assert.equal(r.orderNo, "");
	assert.equal(r.orderItemId, "");
	assert.equal(r.productId, "");
	assert.equal(r.skuName, "");
	assert.equal(r.createdAt, 0);
});
