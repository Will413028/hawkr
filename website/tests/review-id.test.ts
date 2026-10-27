import assert from "node:assert/strict";
import test from "node:test";
import { submitOrderItemReview } from "../lib/reviews/repo";

// Regression: orderItemId / productId are 64-bit TSIDs. The backend sends
// the order detail with these as JSON *strings* precisely because the values
// exceed Number.MAX_SAFE_INTEGER (2^53-1). Coercing them to a JS number before
// POSTing the review rounds the low digits, so review-service can no longer
// match the item against the order and replies 400 "item not in order".
const ORDER_ITEM_ID = "853917025239746789";
const PRODUCT_ID = "843856384063471789";

type FetchCall = { url: string; body: Record<string, unknown> | undefined };

async function withStubbedFetch(
	run: () => Promise<void>,
): Promise<FetchCall[]> {
	const calls: FetchCall[] = [];
	const original = globalThis.fetch;
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		calls.push({
			url: String(input),
			body: init?.body ? JSON.parse(String(init.body)) : undefined,
		});
		return new Response(JSON.stringify({ code: 0, message: "success" }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		});
	}) as typeof fetch;
	try {
		await run();
	} finally {
		globalThis.fetch = original;
	}
	return calls;
}

test("submitOrderItemReview keeps the orderItemId TSID as an intact string", async () => {
	const calls = await withStubbedFetch(async () => {
		await submitOrderItemReview({
			orderNo: "O214578729",
			orderItemId: ORDER_ITEM_ID,
			productId: PRODUCT_ID,
			skuName: "有機番茄 1kg",
			productImageUrl: "https://test-media.example.com/products/tomato.jpg",
			price: "180",
			rating: 5,
			content: "good",
		});
	});
	assert.equal(calls.length, 1);
	const body = calls[0]?.body ?? {};
	assert.equal(body.orderItemId, ORDER_ITEM_ID);
	assert.equal(typeof body.orderItemId, "string");
	assert.equal(body.productId, PRODUCT_ID);
	assert.equal(typeof body.productId, "string");
	assert.equal(body.skuName, "有機番茄 1kg");
	assert.equal(
		body.productImageUrl,
		"https://test-media.example.com/products/tomato.jpg",
	);
	// backend#2235：請求 body 已全面 camelCase。舊 snake 欄名一旦復活，
	// 後端的 binding 會直接收不到值（400 或靜默落空），所以在這裡釘死。
	for (const retired of [
		"order_no",
		"order_item_id",
		"product_id",
		"sku_name",
		"item_image_url",
		"staged_tokens",
	]) {
		assert.ok(!(retired in body), `retired request key ${retired} came back`);
	}
	assert.equal(body.price, 180);
	// Guard the actual failure mode: Number() silently rounds the tail.
	assert.notEqual(String(Number(ORDER_ITEM_ID)), ORDER_ITEM_ID);
});

test("submitOrderItemReview omits productId when it is absent", async () => {
	const calls = await withStubbedFetch(async () => {
		await submitOrderItemReview({
			orderNo: "O214578729",
			orderItemId: ORDER_ITEM_ID,
			rating: 4,
		});
	});
	const body = calls[0]?.body ?? {};
	assert.equal(body.orderItemId, ORDER_ITEM_ID);
	assert.ok(!("productId" in body));
});
