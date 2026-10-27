import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { fetchOrderStatusCounts, fetchOrders } from "../lib/orders/repo";

const realFetch = globalThis.fetch;
const globals = globalThis as Record<string, unknown>;
let fetchCalls: Array<{ url: string }>;

const fakeLocalStorage = {
	store: { "hkui:auth:active-role": "seller" } as Record<string, string>,
	getItem(k: string) {
		return this.store[k] ?? null;
	},
	setItem(k: string, v: string) {
		this.store[k] = v;
	},
	removeItem(k: string) {
		delete this.store[k];
	},
};

beforeEach(() => {
	fetchCalls = [];
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	globals.window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	};
	globals.localStorage = fakeLocalStorage;
	globalThis.fetch = async (url: string | URL | Request) => {
		fetchCalls.push({ url: String(url) });
		return new Response(
			JSON.stringify({
				code: 0,
				data: {
					page: "1",
					pageSize: "1",
					records: [],
					total: "12",
				},
			}),
			{
				status: 200,
				headers: { "content-type": "application/json" },
			},
		);
	};
});

afterEach(() => {
	globalThis.fetch = realFetch;
	delete globals.window;
	delete globals.localStorage;
});

test("fetchOrders follows OpenAPI pagination params and total shape", async () => {
	const result = await fetchOrders({
		status: "10,20",
		page: 1,
		pageSize: 1,
		view: "seller",
	});
	const url = new URL(fetchCalls[0].url);

	assert.equal(url.searchParams.get("status"), "10,20");
	assert.equal(url.searchParams.get("page"), "1");
	assert.equal(url.searchParams.get("page_size"), "1");
	assert.equal(url.searchParams.get("view"), "seller");
	assert.equal(result.page, 1);
	assert.equal(result.pageSize, 1);
	assert.equal(result.total, 12);
});

test("fetchOrders normalizes seller shipment and buyer receipt photos", async () => {
	globalThis.fetch = async (url: string | URL | Request) => {
		fetchCalls.push({ url: String(url) });
		return new Response(
			JSON.stringify({
				code: 0,
				data: {
					page: 1,
					pageSize: 1,
					records: [
						{
							orderNo: "O-photo",
							status: "50",
							createdAt: "2026-06-25T12:00:00Z",
							totalActualAmount: "999",
							// §231 (#430 D3-S3): the wire sends BOTH — the bare-objectKey
							// SSOT and the Gateway-composed display URL. Keeping both here
							// is what makes this test discriminating: it fails if the
							// normalizer ever goes back to reading the `*Urls` twin.
							shipmentPhotoKeys: ["public/shipment/seller.jpg", ""],
							shipmentPhotoUrls: [
								"https://test-media.example.com/i/original/public/shipment/seller.jpg",
							],
							receiptPhotoKeys: ["public/shipment/buyer.jpg"],
							receiptPhotoUrls: [
								"https://test-media.example.com/i/original/public/shipment/buyer.jpg",
							],
						},
					],
					total: 1,
				},
			}),
			{
				status: 200,
				headers: { "content-type": "application/json" },
			},
		);
	};

	const result = await fetchOrders({ view: "seller" });
	const order = result.records[0];

	assert.deepEqual(order.shipping?.shipmentPhotoUrls, [
		"public/shipment/seller.jpg",
	]);
	assert.deepEqual(order.shipping?.receiptPhotoUrls, [
		"public/shipment/buyer.jpg",
	]);
});

test("fetchOrders keeps receiver name separate from buyer/customer names", async () => {
	globalThis.fetch = async (url: string | URL | Request) => {
		fetchCalls.push({ url: String(url) });
		return new Response(
			JSON.stringify({
				code: 0,
				data: {
					page: 1,
					pageSize: 3,
					records: [
						{
							orderNo: "O-recipient",
							status: "10",
							createdAt: "2026-06-25T12:00:00Z",
							totalActualAmount: "999",
							buyerName: "卡秋山林",
							shipping: {
								recipientName: "林小明",
								receiverPhone: "0900123123",
							},
						},
						{
							orderNo: "O-recipient-only",
							status: "10",
							createdAt: "2026-06-25T12:00:00Z",
							totalActualAmount: "999",
							receiverName: "只應顯示於詳情",
							receiverPhone: "0900123123",
						},
						{
							orderNo: "O-buyer-only",
							status: "10",
							createdAt: "2026-06-25T12:00:00Z",
							totalActualAmount: "999",
							buyerName: "卡秋山林",
							customerName: "會員名稱",
							shipping: {
								buyerName: "卡秋山林",
								customerName: "會員名稱",
								receiverPhone: "0900123123",
							},
						},
					],
					total: 3,
				},
			}),
			{
				status: 200,
				headers: { "content-type": "application/json" },
			},
		);
	};

	const result = await fetchOrders({ view: "seller" });

	assert.equal(result.records[0].buyerName, "卡秋山林");
	assert.equal(result.records[0].shipping?.receiverName, "林小明");
	assert.equal(result.records[1].buyerName, undefined);
	assert.equal(result.records[1].shipping?.receiverName, "只應顯示於詳情");
	assert.equal(result.records[2].buyerName, "卡秋山林");
	assert.equal(result.records[2].shipping?.receiverName, undefined);
});

test("fetchOrders reads the buyer avatar from the objectKey SSOT, not the URL twin", async () => {
	globalThis.fetch = async (url: string | URL | Request) => {
		fetchCalls.push({ url: String(url) });
		return new Response(
			JSON.stringify({
				code: 0,
				data: {
					records: [
						{
							orderNo: "O-buyer-profile",
							status: "10",
							nickname: "Hawkr 買家",
							// §231 (#430 D3-S2, renamed backend#2216): seller-view orders
							// send the flat pair `avatarKey` + `avatarUrl`
							// (order.go:47-48). Both present → the key must win.
							avatarKey: "public/buyer_avatar/2623900520/a.png",
							avatarUrl:
								"https://test-media.example.com/i/original/public/buyer_avatar/2623900520/a.png",
						},
					],
					total: 1,
				},
			}),
			{
				status: 200,
				headers: { "content-type": "application/json" },
			},
		);
	};

	const result = await fetchOrders({ view: "seller" });

	assert.equal(result.records[0].buyerName, "Hawkr 買家");
	assert.equal(
		result.records[0].buyerAvatar,
		"public/buyer_avatar/2623900520/a.png",
	);
});

test("fetchOrderStatusCounts hits the rollup endpoint and folds counts by raw status", async () => {
	globalThis.fetch = async (url: string | URL | Request) => {
		fetchCalls.push({ url: String(url) });
		return new Response(
			JSON.stringify({
				code: 0,
				data: {
					counts: [
						{ status: "10", count: "3" },
						{ status: "20", count: "2" },
						{ status: "50", count: "5" },
					],
					total: "10",
				},
			}),
			{ status: 200, headers: { "content-type": "application/json" } },
		);
	};

	const result = await fetchOrderStatusCounts();
	const url = new URL(fetchCalls[0].url);

	assert.ok(url.pathname.endsWith("/app/orders/status_counts"));
	// No args at all — the handler reads only `view` (to reject the seller
	// view) and derives the buyer from the JWT, so nothing is sent.
	assert.equal(url.searchParams.get("status"), null);
	assert.equal(url.searchParams.get("page"), null);
	assert.equal(url.searchParams.get("view"), null);
	assert.equal(url.search, "");
	assert.equal(result.total, 10);
	assert.equal(result.countsByStatus["10"], 3);
	assert.equal(result.countsByStatus["20"], 2);
	assert.equal(result.countsByStatus["50"], 5);
	// Absent buckets fold to 0 at the call site (no key present here).
	assert.equal(result.countsByStatus["80"], undefined);
});
