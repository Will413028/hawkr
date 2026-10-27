import assert from "node:assert/strict";
import test from "node:test";
import { getDelegatedCookieName } from "../features/delegated-operations/server/session";

/**
 * 代行 BFF 的路由表與四道入站檢查（56 號 §4.5 L1041–L1045）。
 *
 * 這份測試同時是**路由表本身的權威副本**：一條 BFF 路徑對一條 gateway 路徑，沒有
 * catch-all，所以「代行到底能打哪些端點」是可以列舉的 —— 下面那張表就是那份列舉。
 * 少一列＝那支端點在工作區裡是 404；多一列＝有人開了一條沒有人審過的上游路徑。
 *
 * 每一支都以**真的呼叫 route handler** 驗證，`fetch` 被換成一支只記錄不連線的替身：
 * 「沒有 cookie 就不打 gateway」這種宣稱，只有在能證明 `fetch` 一次都沒被呼叫時才
 * 算數。
 */

const CREDENTIAL = "dops_test_credential_0123456789abcdef";
const LOCAL_ORIGIN = "http://localhost:3000";

interface RouteCase {
	/** `app/(delegated)/adm/api/` 之下的目錄。 */
	module: string;
	method: "GET" | "POST" | "PATCH" | "DELETE";
	/** route handler 的 export 名。 */
	handler: "GET" | "POST" | "PATCH";
	params: Record<string, string>;
	/** 瀏覽器打的路徑（純顯示用，讓這張表讀起來就是一份端點清單）。 */
	bff: string;
	/** 上游 gateway 路徑（`/api/v1` 之後）。 */
	gateway: string;
}

const ROUTES: RouteCase[] = [
	// ── session（3 支，10s 檔）─────────────────────────────────────────────
	{
		module: "session/current",
		method: "GET",
		handler: "GET",
		params: {},
		bff: "/adm/api/session/current",
		gateway: "/admin/delegated-operations/current",
	},
	{
		module: "session/summary",
		method: "GET",
		handler: "GET",
		params: {},
		bff: "/adm/api/session/summary",
		gateway: "/admin/delegated-operations/current/summary",
	},
	// session/end 是兩跳（先 GET current 驗 target 再 DELETE），單獨測。

	// ── storefront（4 支）──────────────────────────────────────────────────
	{
		module: "storefront/[storeId]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4" },
		bff: "/adm/api/storefront/4",
		gateway: "/admin/sellers/4/storefront",
	},
	{
		module: "storefront/[storeId]",
		method: "PATCH",
		handler: "PATCH",
		params: { storeId: "4" },
		bff: "/adm/api/storefront/4",
		gateway: "/admin/sellers/4/storefront",
	},
	{
		module: "storefront/[storeId]/logo",
		method: "POST",
		handler: "POST",
		params: { storeId: "4" },
		bff: "/adm/api/storefront/4/logo",
		gateway: "/admin/sellers/4/storefront/logo",
	},
	{
		module: "storefront/[storeId]/cover",
		method: "POST",
		handler: "POST",
		params: { storeId: "4" },
		bff: "/adm/api/storefront/4/cover",
		gateway: "/admin/sellers/4/storefront/cover",
	},

	// ── products（7 支）────────────────────────────────────────────────────
	{
		module: "products/[storeId]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4" },
		bff: "/adm/api/products/4",
		gateway: "/admin/sellers/4/products",
	},
	{
		module: "products/[storeId]",
		method: "POST",
		handler: "POST",
		params: { storeId: "4" },
		bff: "/adm/api/products/4",
		gateway: "/admin/sellers/4/products",
	},
	{
		module: "products/[storeId]/batch-unpublish",
		method: "POST",
		handler: "POST",
		params: { storeId: "4" },
		bff: "/adm/api/products/4/batch-unpublish",
		gateway: "/admin/sellers/4/products/batch-unpublish",
	},
	{
		module: "products/[storeId]/[productId]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4", productId: "77" },
		bff: "/adm/api/products/4/77",
		gateway: "/admin/sellers/4/products/77",
	},
	{
		module: "products/[storeId]/[productId]",
		method: "PATCH",
		handler: "PATCH",
		params: { storeId: "4", productId: "77" },
		bff: "/adm/api/products/4/77",
		gateway: "/admin/sellers/4/products/77",
	},
	{
		module: "products/[storeId]/[productId]/publish",
		method: "POST",
		handler: "POST",
		params: { storeId: "4", productId: "77" },
		bff: "/adm/api/products/4/77/publish",
		gateway: "/admin/sellers/4/products/77/publish",
	},
	{
		module: "products/[storeId]/[productId]/unpublish",
		method: "POST",
		handler: "POST",
		params: { storeId: "4", productId: "77" },
		bff: "/adm/api/products/4/77/unpublish",
		gateway: "/admin/sellers/4/products/77/unpublish",
	},

	// ── posts（6 支）──────────────────────────────────────────────────────
	{
		module: "posts/[storeId]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4" },
		bff: "/adm/api/posts/4",
		gateway: "/admin/sellers/4/posts",
	},
	{
		module: "posts/[storeId]",
		method: "POST",
		handler: "POST",
		params: { storeId: "4" },
		bff: "/adm/api/posts/4",
		gateway: "/admin/sellers/4/posts",
	},
	{
		module: "posts/[storeId]/[postId]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4", postId: "88" },
		bff: "/adm/api/posts/4/88",
		gateway: "/admin/sellers/4/posts/88",
	},
	{
		module: "posts/[storeId]/[postId]",
		method: "PATCH",
		handler: "PATCH",
		params: { storeId: "4", postId: "88" },
		bff: "/adm/api/posts/4/88",
		gateway: "/admin/sellers/4/posts/88",
	},
	{
		module: "posts/[storeId]/[postId]/publish",
		method: "POST",
		handler: "POST",
		params: { storeId: "4", postId: "88" },
		bff: "/adm/api/posts/4/88/publish",
		gateway: "/admin/sellers/4/posts/88/publish",
	},
	{
		module: "posts/[storeId]/[postId]/unpublish",
		method: "POST",
		handler: "POST",
		params: { storeId: "4", postId: "88" },
		bff: "/adm/api/posts/4/88/unpublish",
		gateway: "/admin/sellers/4/posts/88/unpublish",
	},

	// ── orders（3 支，唯讀）───────────────────────────────────────────────
	{
		module: "orders/[storeId]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4" },
		bff: "/adm/api/orders/4",
		gateway: "/admin/sellers/4/orders",
	},
	{
		module: "orders/[storeId]/[orderNo]",
		method: "GET",
		handler: "GET",
		params: { storeId: "4", orderNo: "HK260728001" },
		bff: "/adm/api/orders/4/HK260728001",
		gateway: "/admin/sellers/4/orders/HK260728001",
	},
	{
		module: "orders/[storeId]/[orderNo]/recipient-contact",
		method: "GET",
		handler: "GET",
		params: { storeId: "4", orderNo: "HK260728001" },
		bff: "/adm/api/orders/4/HK260728001/recipient-contact",
		gateway: "/admin/sellers/4/orders/HK260728001/recipient-contact",
	},

	// ── 媒體 upload session（2 支）────────────────────────────────────────
	{
		module: "media/upload_sessions",
		method: "POST",
		handler: "POST",
		params: {},
		bff: "/adm/api/media/upload_sessions",
		gateway: "/admin/delegated-operations/media/upload_sessions",
	},
	{
		module: "media/upload_sessions/[sessionId]/complete",
		method: "POST",
		handler: "POST",
		params: { sessionId: "9f1c0a20-2b6d-4d1e-9a77-8b0a1c2d3e4f" },
		bff: "/adm/api/media/upload_sessions/9f1c0a20-2b6d-4d1e-9a77-8b0a1c2d3e4f/complete",
		gateway:
			"/admin/delegated-operations/media/upload_sessions/9f1c0a20-2b6d-4d1e-9a77-8b0a1c2d3e4f/complete",
	},
];

type Handler = (
	request: Request,
	context: { params: Promise<Record<string, string>> },
) => Promise<Response>;

async function loadHandler(route: RouteCase): Promise<Handler> {
	const mod = (await import(
		`../app/(delegated)/adm/api/${route.module}/route`
	)) as Record<string, Handler>;
	const handler = mod[route.handler];
	assert.equal(
		typeof handler,
		"function",
		`${route.module} 沒有 export ${route.handler}`,
	);
	return handler;
}

interface FetchCall {
	url: string;
	init: RequestInit;
}

/** 換掉 `fetch`，記錄呼叫並回一份可指定的 envelope。 */
function stubFetch(
	responder: (call: FetchCall) => Response | Promise<Response> = () =>
		Response.json({ code: 0, message: "success", data: { ok: true } }),
): { calls: FetchCall[]; restore(): void } {
	const calls: FetchCall[] = [];
	const original = globalThis.fetch;
	globalThis.fetch = (async (input: unknown, init: RequestInit = {}) => {
		const call = { url: String(input), init };
		calls.push(call);
		return responder(call);
	}) as typeof fetch;
	return {
		calls,
		restore() {
			globalThis.fetch = original;
		},
	};
}

function buildRequest(
	route: RouteCase,
	overrides: {
		cookie?: boolean;
		xhr?: boolean;
		origin?: string | null;
		headers?: Record<string, string>;
		query?: string;
		body?: string;
	} = {},
): Request {
	const headers = new Headers(overrides.headers ?? {});
	if (overrides.cookie !== false) {
		headers.set("cookie", `${getDelegatedCookieName()}=${CREDENTIAL}`);
	}
	if (route.method !== "GET") {
		if (overrides.xhr !== false)
			headers.set("x-requested-with", "XMLHttpRequest");
		const origin =
			overrides.origin === undefined ? LOCAL_ORIGIN : overrides.origin;
		if (origin !== null) headers.set("origin", origin);
	}
	return new Request(`${LOCAL_ORIGIN}${route.bff}${overrides.query ?? ""}`, {
		method: route.method,
		headers,
		body: route.method === "GET" ? undefined : (overrides.body ?? "{}"),
	});
}

function contextOf(route: RouteCase) {
	return { params: Promise.resolve(route.params) };
}

async function codeOf(response: Response): Promise<unknown> {
	return ((await response.json()) as { code: unknown }).code;
}

test("每一支 BFF 都轉到它自己那一條 gateway 路徑（禁 catch-all 的可列舉性）", async () => {
	for (const route of ROUTES) {
		const handler = await loadHandler(route);
		const fetchStub = stubFetch();
		try {
			const response = await handler(buildRequest(route), contextOf(route));
			assert.equal(response.status, 200, route.bff);
			assert.equal(fetchStub.calls.length, 1, `${route.bff} 應該只打一次上游`);
			const [call] = fetchStub.calls;
			assert.equal(call.init.method, route.method, route.bff);
			assert.ok(
				call.url.endsWith(route.gateway),
				`${route.bff} → ${call.url}，預期結尾 ${route.gateway}`,
			);
			// credential 只走這一個 header，且絕不出現在 URL 裡。
			const sent = new Headers(call.init.headers);
			assert.equal(sent.get("X-HK-Operation-Session"), CREDENTIAL, route.bff);
			assert.ok(!call.url.includes(CREDENTIAL), route.bff);
		} finally {
			fetchStub.restore();
		}
	}
});

test("沒有 credential cookie：401 且一次都不打 gateway", async () => {
	for (const route of ROUTES) {
		const handler = await loadHandler(route);
		const fetchStub = stubFetch(() => {
			throw new Error(`${route.bff} 在沒有 credential 時打了 gateway`);
		});
		try {
			const response = await handler(
				buildRequest(route, { cookie: false }),
				contextOf(route),
			);
			assert.equal(response.status, 401, route.bff);
			assert.equal(await codeOf(response), "CREDENTIAL_MISSING", route.bff);
			assert.equal(fetchStub.calls.length, 0, route.bff);
		} finally {
			fetchStub.restore();
		}
	}
});

test("非 GET 缺 X-Requested-With 或 Origin 不符：403，且不打 gateway", async () => {
	const mutations = ROUTES.filter((route) => route.method !== "GET");
	assert.ok(mutations.length >= 12, "mutation 路由數不對，這張表可能被改壞了");
	for (const route of mutations) {
		const handler = await loadHandler(route);
		for (const overrides of [
			{ xhr: false },
			{ origin: null },
			{ origin: "https://evil.example" },
			// 同名前綴不算同源：`…food.evil` 與 `…food` 是兩個 host。
			{ origin: `${LOCAL_ORIGIN}.evil.example` },
		]) {
			const fetchStub = stubFetch(() => {
				throw new Error(`${route.bff} 在 CSRF 檢查失敗時打了 gateway`);
			});
			try {
				const response = await handler(
					buildRequest(route, overrides),
					contextOf(route),
				);
				assert.equal(
					response.status,
					403,
					`${route.bff} ${JSON.stringify(overrides)}`,
				);
				assert.equal(await codeOf(response), "CSRF_REJECTED", route.bff);
				assert.equal(fetchStub.calls.length, 0, route.bff);
			} finally {
				fetchStub.restore();
			}
		}
	}
});

test("GET 也受同源保護以外的那道：保留 header 出現即拒絕", async () => {
	// 瀏覽器自己送 x-hk-operation-session ＝ 想繞過 httpOnly 直接指定憑證；
	// x-hk-origin-auth ＝ 想指定這次請求要計給誰的限流桶。
	for (const header of ["x-hk-operation-session", "x-hk-origin-auth"]) {
		const route = ROUTES[0];
		const handler = await loadHandler(route);
		const fetchStub = stubFetch(() => {
			throw new Error("保留 header 被放行到 gateway");
		});
		try {
			const response = await handler(
				buildRequest(route, { headers: { [header]: "spoofed" } }),
				contextOf(route),
			);
			assert.equal(response.status, 400, header);
			assert.equal(await codeOf(response), "RESERVED_HEADER", header);
			assert.equal(fetchStub.calls.length, 0, header);
		} finally {
			fetchStub.restore();
		}
	}
});

test("path param 值域：不是數字／訂單編號的一律 400，且不組成上游 URL", async () => {
	const cases: Array<{ route: RouteCase; params: Record<string, string> }> = [
		{
			route: ROUTES.find((r) => r.module === "products/[storeId]") as RouteCase,
			params: { storeId: "4/../../app/sellers/me" },
		},
		{
			route: ROUTES.find(
				(r) => r.module === "orders/[storeId]/[orderNo]",
			) as RouteCase,
			params: { storeId: "4", orderNo: "HK1?x=1" },
		},
		{
			route: ROUTES.find(
				(r) => r.module === "media/upload_sessions/[sessionId]/complete",
			) as RouteCase,
			params: { sessionId: "../../current" },
		},
	];
	for (const { route, params } of cases) {
		const handler = await loadHandler(route);
		const fetchStub = stubFetch(() => {
			throw new Error(`${route.bff} 讓非法 path param 進到上游`);
		});
		try {
			const response = await handler(buildRequest(route), {
				params: Promise.resolve(params),
			});
			assert.equal(response.status, 400, route.bff);
			assert.equal(await codeOf(response), "INVALID_PATH_PARAM", route.bff);
			assert.equal(fetchStub.calls.length, 0, route.bff);
		} finally {
			fetchStub.restore();
		}
	}
});

test("query allow-list：列表鍵原樣轉送，未登記的鍵 400", async () => {
	const products = ROUTES.find(
		(r) => r.module === "products/[storeId]" && r.method === "GET",
	) as RouteCase;

	const allowed = stubFetch();
	try {
		const response = await (await loadHandler(products))(
			buildRequest(products, { query: "?page=2&page_size=50&status=4004" }),
			contextOf(products),
		);
		assert.equal(response.status, 200);
		assert.ok(
			allowed.calls[0].url.endsWith(
				"/admin/sellers/4/products?page=2&page_size=50&status=4004",
			),
			allowed.calls[0].url,
		);
	} finally {
		allowed.restore();
	}

	// 訂單列表的六個鍵（§6.1.10 的 Query 契約全集）原樣轉送。日期是 ISO 8601 UTC
	// 不是台北曆日——展開在 repo 層，這一層一個字都不折。
	const orders = ROUTES.find(
		(r) => r.module === "orders/[storeId]",
	) as RouteCase;
	const orderQuery =
		"?page=2&page_size=50&status=20&keyword=%E7%95%AA%E8%8C%84" +
		"&date_from=2026-08-17T16%3A00%3A00Z&date_to=2026-08-18T15%3A59%3A59Z";
	const forwardedOrders = stubFetch();
	try {
		const response = await (await loadHandler(orders))(
			buildRequest(orders, { query: orderQuery }),
			contextOf(orders),
		);
		assert.equal(response.status, 200);
		const forwarded = new URL(forwardedOrders.calls[0].url).searchParams;
		assert.deepEqual(
			[...forwarded.entries()],
			[
				["page", "2"],
				["page_size", "50"],
				["status", "20"],
				["keyword", "番茄"],
				["date_from", "2026-08-17T16:00:00Z"],
				["date_to", "2026-08-18T15:59:59Z"],
			],
			forwardedOrders.calls[0].url,
		);
	} finally {
		forwardedOrders.restore();
	}

	// 白名單之外的鍵仍當場 400：靜默丟棄會讓使用者看到「條件送出去了、清單完全沒篩、
	// 也沒有任何錯誤」。`view` 是最容易誤送的一個（賣家側的 createOrderRepo 會送它，
	// 見 tests/delegated-orders-view-trap.test.ts）。
	const rejected = stubFetch(() => {
		throw new Error("未登記的 query 被轉到 gateway");
	});
	try {
		const response = await (await loadHandler(orders))(
			buildRequest(orders, { query: "?view=seller" }),
			contextOf(orders),
		);
		assert.equal(response.status, 400);
		assert.equal(await codeOf(response), "INVALID_QUERY");
		assert.equal(rejected.calls.length, 0);
	} finally {
		rejected.restore();
	}
});

test("Idempotency-Key：合法轉送、畸形當場 400、缺席不代為生成", async () => {
	const route = ROUTES.find(
		(r) => r.module === "products/[storeId]/[productId]/publish",
	) as RouteCase;
	const handler = await loadHandler(route);

	const forwarded = stubFetch();
	try {
		await handler(
			buildRequest(route, {
				headers: { "Idempotency-Key": "pub-2026-08-14-01" },
			}),
			contextOf(route),
		);
		assert.equal(
			new Headers(forwarded.calls[0].init.headers).get("Idempotency-Key"),
			"pub-2026-08-14-01",
		);
	} finally {
		forwarded.restore();
	}

	const malformed = stubFetch(() => {
		throw new Error("畸形的 Idempotency-Key 被轉到 gateway");
	});
	try {
		const response = await handler(
			buildRequest(route, { headers: { "Idempotency-Key": "short" } }),
			contextOf(route),
		);
		assert.equal(response.status, 400);
		assert.equal(await codeOf(response), "INVALID_IDEMPOTENCY_KEY");
		assert.equal(malformed.calls.length, 0);
	} finally {
		malformed.restore();
	}

	const absent = stubFetch();
	try {
		await handler(buildRequest(route), contextOf(route));
		assert.equal(
			new Headers(absent.calls[0].init.headers).has("Idempotency-Key"),
			false,
			"BFF 不得代為生成 —— 生成的那把每次重送都不一樣，冪等就只剩名字",
		);
	} finally {
		absent.restore();
	}
});

test("上游業務碼原碼原狀態轉出；message 一律不透傳", async () => {
	const route = ROUTES[0];
	const handler = await loadHandler(route);
	const fetchStub = stubFetch(() =>
		Response.json(
			{
				code: 9271,
				message: "代行工作階段已撤銷",
				traceId: "trace-abc",
			},
			{ status: 410 },
		),
	);
	try {
		const response = await handler(buildRequest(route), contextOf(route));
		assert.equal(response.status, 410);
		const body = (await response.json()) as Record<string, unknown>;
		// 9271 是終態碼：塌成單一錯誤碼會讓工作區顯示「暫時性錯誤，請重試」。
		assert.equal(body.code, 9271);
		assert.equal(body.traceId, "trace-abc");
		assert.ok(!("message" in body), "message 不得透傳");
	} finally {
		fetchStub.restore();
	}
});

test("上游逾時 → 504 UPSTREAM_FAILURE（不是業務失敗）", async () => {
	const route = ROUTES[0];
	const handler = await loadHandler(route);
	const fetchStub = stubFetch(() => {
		const error = new Error("timed out");
		error.name = "TimeoutError";
		throw error;
	});
	try {
		const response = await handler(buildRequest(route), contextOf(route));
		assert.equal(response.status, 504);
		assert.equal(await codeOf(response), "UPSTREAM_FAILURE");
	} finally {
		fetchStub.restore();
	}
});

test("上游回 3xx（§6.1.0 明令不得發生）→ 502，不跟隨重導", async () => {
	const route = ROUTES[0];
	const handler = await loadHandler(route);
	const fetchStub = stubFetch(
		() => new Response(null, { status: 302, headers: { location: "/login" } }),
	);
	try {
		const response = await handler(buildRequest(route), contextOf(route));
		assert.equal(response.status, 502);
		assert.equal(await codeOf(response), "UPSTREAM_FAILURE");
		assert.equal(fetchStub.calls[0].init.redirect, "manual");
	} finally {
		fetchStub.restore();
	}
});

test("session/end：target 不符 → 409 且根本不 DELETE", async () => {
	const { POST } = (await import(
		"../app/(delegated)/adm/api/session/end/route"
	)) as { POST: (request: Request) => Promise<Response> };
	const fetchStub = stubFetch((call) => {
		assert.equal(
			call.init.method,
			"GET",
			"target 不符時不得送出 DELETE —— 那會結束掉另一家店的工作階段",
		);
		return Response.json({
			code: 0,
			message: "success",
			data: { target: { storeId: "9" } },
		});
	});
	try {
		const response = await POST(
			new Request(`${LOCAL_ORIGIN}/adm/api/session/end`, {
				method: "POST",
				headers: {
					cookie: `${getDelegatedCookieName()}=${CREDENTIAL}`,
					"x-requested-with": "XMLHttpRequest",
					origin: LOCAL_ORIGIN,
					"content-type": "application/json",
				},
				body: JSON.stringify({ storeId: "4" }),
			}),
		);
		assert.equal(response.status, 409);
		assert.equal(await codeOf(response), "TARGET_MISMATCH");
		assert.equal(fetchStub.calls.length, 1);
		assert.equal(
			response.headers.get("set-cookie"),
			null,
			"沒有結束就不得清 cookie",
		);
	} finally {
		fetchStub.restore();
	}
});

test("session/end：target 相符 → DELETE，且只有拿到終止證明才清 cookie", async () => {
	const { POST } = (await import(
		"../app/(delegated)/adm/api/session/end/route"
	)) as { POST: (request: Request) => Promise<Response> };

	function endRequest(): Request {
		return new Request(`${LOCAL_ORIGIN}/adm/api/session/end`, {
			method: "POST",
			headers: {
				cookie: `${getDelegatedCookieName()}=${CREDENTIAL}`,
				"x-requested-with": "XMLHttpRequest",
				origin: LOCAL_ORIGIN,
				"content-type": "application/json",
			},
			body: JSON.stringify({ storeId: "4" }),
		});
	}

	const proven = stubFetch((call) =>
		call.init.method === "GET"
			? Response.json({
					code: 0,
					message: "success",
					data: { target: { storeId: "4" } },
				})
			: Response.json({ code: 0, message: "success", data: { ended: true } }),
	);
	try {
		const response = await POST(endRequest());
		assert.equal(response.status, 200);
		assert.equal(proven.calls.length, 2);
		assert.equal(proven.calls[1].init.method, "DELETE");
		assert.ok(
			proven.calls[1].url.endsWith("/admin/delegated-operations/current"),
			proven.calls[1].url,
		);
		const cookie = response.headers.get("set-cookie") ?? "";
		assert.match(cookie, /Max-Age=0/);
		assert.match(cookie, /Path=\/adm/);
	} finally {
		proven.restore();
	}

	// 上游沒有明確證明終止（9280「服務暫時不可用」）→ 不清 cookie。狀態不明時丟掉一把
	// 可能還活著的憑證，等於讓管理員以為已經結束、而代行仍在有效期內。
	//
	// ⚠️ status 在這一條只是佈景（判定看的是 `isDelegatedEndProven`，與狀態碼無關），
	// 但仍寫成 **424** ——9280 的 HTTP 鏡射已由 502 改為 424，而這裡是全 repo 最後一處
	// 把兩者綁在一起的地方；留著 502 不會紅，卻會讓日後拿它當契約參考的人接錯。
	const unproven = stubFetch((call) =>
		call.init.method === "GET"
			? Response.json({
					code: 0,
					message: "success",
					data: { target: { storeId: "4" } },
				})
			: Response.json({ code: 9280, message: "boom" }, { status: 424 }),
	);
	try {
		const response = await POST(endRequest());
		assert.equal(response.status, 424);
		assert.equal(await codeOf(response), 9280);
		assert.equal(response.headers.get("set-cookie"), null);
	} finally {
		unproven.restore();
	}
});
