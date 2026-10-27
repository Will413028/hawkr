import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ImageSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import {
	BannerActionSchema,
	BannerSchema,
} from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import {
	bannerActionLabel,
	resolveBannerActionHref,
} from "../lib/banners/action";
import { consumeBannerSwipeClick } from "../lib/banners/interaction";
import { fetchBanners, toBanner } from "../lib/banners/repo";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

/** 正式環境由瀏覽器呼叫 fetchBanners()（BannerCarousel）：這支測試補一個最小的 window，走瀏覽器的 transport
 *  （activeLocale() 可用、重試最多兩次並退避），測完還原。 */
const realWindow = Object.getOwnPropertyDescriptor(globalThis, "window");

const realFetch = globalThis.fetch;
let requests: Array<{ url: string; method: string }> = [];
let respond: () => Response | Promise<Response>;

function json(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { "content-type": "application/json" },
	});
}

beforeEach(() => {
	Object.defineProperty(globalThis, "window", {
		value: globalThis,
		configurable: true,
		writable: true,
	});
	requests = [];
	respond = () => json({});
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	// @ts-expect-error deterministic transport stub for the repository contract
	globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
		requests.push({ url: String(input), method: init?.method ?? "GET" });
		return respond();
	};
});

afterEach(() => {
	if (realWindow) Object.defineProperty(globalThis, "window", realWindow);
	else delete (globalThis as { window?: unknown }).window;
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

const image = (key: string) =>
	create(ImageSchema, {
		uriTemplate: `https://media.example.test/i/{preset}/${key}`,
	});

test("maps the Connect banner action oneof to web actions", () => {
	const popup = toBanner(
		create(BannerSchema, {
			id: "0199a1c2-0000-7000-8000-000000000001",
			title: " 會員專屬優惠 ",
			buttonText: " 看更多 ",
			previewImage: image("preview.png"),
			popupImage: image("popup.png"),
			action: create(BannerActionSchema, {
				kind: { case: "popup", value: {} },
			}),
		}),
	);
	assert.deepEqual(popup?.action, { type: "popup" });
	assert.equal(popup?.title, "會員專屬優惠");
	assert.equal(popup?.buttonText, "看更多");
	assert.equal(popup?.popupImage?.uriTemplate.endsWith("popup.png"), true);

	const product = toBanner(
		create(BannerSchema, {
			id: "2",
			previewImage: image("product.png"),
			action: create(BannerActionSchema, {
				kind: { case: "productId", value: "123" },
			}),
		}),
	);
	assert.deepEqual(product?.action, { type: "product", id: "123" });
	assert.equal(product?.title, "");
	assert.equal(product?.buttonText, undefined);

	const url = toBanner(
		create(BannerSchema, {
			id: "3",
			previewImage: image("url.png"),
			action: create(BannerActionSchema, {
				kind: { case: "url", value: "https://example.com/a" },
			}),
		}),
	);
	assert.deepEqual(url?.action, { type: "url", url: "https://example.com/a" });
});

test("a banner without a slide image is not shown", () => {
	assert.equal(toBanner(create(BannerSchema, { id: "4" })), null);
});

test("resolves banner actions to safe web routes", () => {
	// ID 是不透明的字串（ADR 2026-09-25-row-identity-by-data-kind D5）：只編碼，不檢查格式。
	assert.equal(
		resolveBannerActionHref({ type: "product", id: "a/b" }),
		"/products/a%2Fb",
	);
	assert.equal(
		resolveBannerActionHref({ type: "post", id: "42" }),
		"/feed?post=42",
	);
	assert.equal(
		resolveBannerActionHref({ type: "url", url: "https://example.com/path" }),
		"https://example.com/path",
	);
	assert.equal(
		resolveBannerActionHref({ type: "url", url: "//evil.example/path" }),
		undefined,
	);
	assert.equal(resolveBannerActionHref({ type: "popup" }), undefined);
	assert.equal(bannerActionLabel({ type: "product", id: "1" }), "立即購買");
});

test("entity ids are opaque: any non-empty id becomes an encoded link", () => {
	const [product, empty] = [
		{
			case: "productId" as const,
			value: "0199a1c2-0000-7000-8000-00000000abcd",
		},
		{ case: "postId" as const, value: "  " },
	].map((kind, i) =>
		toBanner(
			create(BannerSchema, {
				id: String(i + 1),
				previewImage: image(`${i}.png`),
				action: create(BannerActionSchema, { kind }),
			}),
		),
	);

	assert.deepEqual(product?.action, {
		type: "product",
		id: "0199a1c2-0000-7000-8000-00000000abcd",
	});
	assert.equal(empty?.action, undefined, "空白的 id 沒有連結");
});

test("fetchBanners reads ListLiveBanners with a GET and keeps a valid empty list", async () => {
	assert.deepEqual(await fetchBanners(), []);
	assert.equal(requests.length, 1);
	assert.equal(requests[0].method, "GET");
	assert.match(
		requests[0].url,
		/\/hawkr\.content\.v1alpha1\.ContentPublicService\/ListLiveBanners\?/,
	);
});

test("fetchBanners drops banners without a slide image", async () => {
	respond = () =>
		json({
			banners: [
				{
					id: "1",
					title: "有圖",
					previewImage: { uriTemplate: "https://m.test/i/{preset}/a.png" },
				},
				{ id: "2", title: "沒有圖" },
			],
		});
	assert.deepEqual(
		(await fetchBanners()).map((b) => b.id),
		["1"],
	);
});

// 瀏覽器端：網路錯誤與 unavailable 最多重試兩次、退避等待（共 3 次請求；lib/connect/transport.ts）。
test("fetchBanners rejects a network failure after the browser retries", async () => {
	respond = () => {
		throw new TypeError("fetch failed");
	};
	await assert.rejects(
		() => fetchBanners(),
		(error: unknown) =>
			error instanceof ConnectError && error.code === Code.Unknown,
	);
	assert.equal(requests.length, 3);
});

test("fetchBanners rejects a settled unavailable after the browser retries", async () => {
	respond = () =>
		json({ code: "unavailable", message: "banner service unavailable" }, 503);
	await assert.rejects(
		() => fetchBanners(),
		(error: unknown) =>
			error instanceof ConnectError && error.code === Code.Unavailable,
	);
	assert.equal(requests.length, 3);
});

test("the transport retry recovers from a transient unavailable", async () => {
	respond = () =>
		requests.length === 1
			? json({ code: "unavailable", message: "blip" }, 503)
			: json({});
	assert.deepEqual(await fetchBanners(), []);
	assert.equal(requests.length, 2);
});

test("a client error is not retried", async () => {
	respond = () => json({ code: "invalid_argument", message: "bad" }, 400);
	await assert.rejects(
		() => fetchBanners(),
		(error: unknown) =>
			error instanceof ConnectError && error.code === Code.InvalidArgument,
	);
	assert.equal(requests.length, 1);
});

test("swipe suppression consumes exactly one pointer click", () => {
	const state = { current: true };

	assert.equal(consumeBannerSwipeClick(state, 1), true);
	assert.equal(state.current, false);
	assert.equal(consumeBannerSwipeClick(state, 1), false);
});

test("keyboard clicks are always allowed and clear stale swipe state", () => {
	const state = { current: true };

	assert.equal(consumeBannerSwipeClick(state, 0), false);
	assert.equal(state.current, false);
	assert.equal(consumeBannerSwipeClick(state, 0), false);
});
