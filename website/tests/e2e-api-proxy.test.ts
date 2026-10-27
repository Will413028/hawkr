import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { POST as connectPOST } from "../app/api/connect/[...path]/route";
import { GET, POST } from "../app/api/v1/[...path]/route";
import {
	apiProxyTarget,
	connectProxyTarget,
	isLocalApiProxyEnabled,
} from "../lib/api/local-proxy";

test("same-origin API proxy is limited to development and explicit Playwright builds", () => {
	assert.equal(
		isLocalApiProxyEnabled("http://localhost:3000/api/v1/test", {
			NODE_ENV: "development",
		}),
		true,
	);
	assert.equal(
		isLocalApiProxyEnabled("http://localhost:3213/api/v1/test", {
			NODE_ENV: "production",
		}),
		false,
	);
	assert.equal(
		isLocalApiProxyEnabled("http://localhost:3213/api/v1/test", {
			NODE_ENV: "test",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		false,
	);
	assert.equal(
		isLocalApiProxyEnabled("http://localhost:3213/api/v1/test", {
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		true,
	);
	assert.equal(
		isLocalApiProxyEnabled("http://127.0.0.1:3213/api/v1/test", {
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		true,
	);
	assert.equal(
		isLocalApiProxyEnabled("http://[::1]:3213/api/v1/test", {
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		true,
	);
	assert.equal(
		isLocalApiProxyEnabled("https://example.com/api/v1/test", {
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		false,
	);
	assert.equal(
		isLocalApiProxyEnabled("not a URL", {
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		false,
	);
});

test("API proxy preserves the configured gateway base, path, and query", () => {
	assert.equal(
		apiProxyTarget(
			"http://localhost:3213/api/v1/public/products?page=2&page_size=12",
			"https://test-api.example.com/api/v1",
		),
		"https://test-api.example.com/api/v1/public/products?page=2&page_size=12",
	);
	assert.equal(
		apiProxyTarget(
			"http://localhost:3213/api/v1/app/auth/session",
			"https://gateway.example/internal/api/v1/",
		),
		"https://gateway.example/internal/api/v1/app/auth/session",
	);
});

test("Connect proxy targets the gateway root, keeping a base path prefix", () => {
	assert.equal(
		connectProxyTarget(
			"http://localhost:3213/api/connect/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners?connect=v1&encoding=json&message=%7B%7D",
			"https://gateway.example/api/v1",
		),
		"https://gateway.example/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners?connect=v1&encoding=json&message=%7B%7D",
	);
	assert.equal(
		connectProxyTarget(
			"http://localhost:3213/api/connect/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion",
			"https://gateway.example/internal/api/v1/",
		),
		"https://gateway.example/internal/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion",
	);
});

test("API proxy handler preserves its HTTP contract", async (t) => {
	const originalFetch = globalThis.fetch;
	const originalNodeEnv = process.env.NODE_ENV;
	const originalProxyFlag = process.env.PLAYWRIGHT_E2E_API_PROXY;
	const originalApiBase = process.env.API_BASE_URL;

	process.env.NODE_ENV = "production";
	process.env.PLAYWRIGHT_E2E_API_PROXY = "1";
	process.env.API_BASE_URL = "https://gateway.example/api/v1";

	try {
		await t.test(
			"returns 404 while the production proxy flag is disabled",
			async () => {
				delete process.env.PLAYWRIGHT_E2E_API_PROXY;
				let fetchCalled = false;
				globalThis.fetch = (async () => {
					fetchCalled = true;
					throw new Error("disabled proxy must not call upstream");
				}) as typeof fetch;

				const response = await GET(
					new NextRequest("http://localhost:3213/api/v1/public/products"),
				);

				assert.equal(response.status, 404);
				assert.equal(fetchCalled, false);
				process.env.PLAYWRIGHT_E2E_API_PROXY = "1";
			},
		);

		await t.test(
			"rejects a non-loopback host even with the flag enabled",
			async () => {
				let fetchCalled = false;
				globalThis.fetch = (async () => {
					fetchCalled = true;
					throw new Error("host-gated proxy must not call upstream");
				}) as typeof fetch;

				const response = await GET(
					new NextRequest("https://example.com/api/v1/public/products"),
				);

				assert.equal(response.status, 404);
				assert.equal(fetchCalled, false);
			},
		);

		await t.test(
			"forwards a GET path, query, headers, and 401 response",
			async () => {
				globalThis.fetch = (async (input, init) => {
					assert.equal(
						String(input),
						"https://gateway.example/api/v1/app/auth/session?probe=1",
					);
					assert.equal(init?.method, "GET");
					assert.equal(new Headers(init?.headers).get("x-test-request"), "get");
					assert.equal(init?.body, undefined);
					return new Response('{"code":401}', {
						status: 401,
						headers: {
							"content-type": "application/json",
							"x-upstream": "auth",
						},
					});
				}) as typeof fetch;

				const response = await GET(
					new NextRequest(
						"http://127.0.0.1:3213/api/v1/app/auth/session?probe=1",
						{ headers: { "x-test-request": "get" } },
					),
				);

				assert.equal(response.status, 401);
				assert.equal(response.headers.get("x-upstream"), "auth");
				assert.equal(await response.text(), '{"code":401}');
			},
		);

		await t.test(
			"coalesces and caches repeated anonymous public JSON GETs",
			async () => {
				let fetchCalls = 0;
				let releaseUpstream!: () => void;
				const upstreamGate = new Promise<void>((resolve) => {
					releaseUpstream = resolve;
				});
				globalThis.fetch = (async () => {
					fetchCalls += 1;
					await upstreamGate;
					return new Response('{"code":0,"data":["cached"]}', {
						status: 200,
						headers: {
							"content-type": "application/json; charset=utf-8",
							"x-upstream": "public-cache",
						},
					});
				}) as typeof fetch;

				const request = () =>
					new NextRequest(
						"http://localhost:3213/api/v1/public/categories/tree?cache-test=1",
						{
							headers: {
								// Playwright APIRequestContext uses */* for fixture
								// discovery, so it must share the same JSON snapshot.
								accept: "*/*",
								"accept-language": "zh-TW",
							},
						},
					);
				const first = GET(request());
				const second = GET(request());
				await Promise.resolve();
				assert.equal(fetchCalls, 1);
				releaseUpstream();

				const [firstResponse, secondResponse] = await Promise.all([
					first,
					second,
				]);
				assert.equal(
					await firstResponse.text(),
					'{"code":0,"data":["cached"]}',
				);
				assert.equal(
					await secondResponse.text(),
					'{"code":0,"data":["cached"]}',
				);

				const cachedResponse = await GET(request());
				assert.equal(cachedResponse.headers.get("x-upstream"), "public-cache");
				assert.equal(
					await cachedResponse.text(),
					'{"code":0,"data":["cached"]}',
				);
				assert.equal(fetchCalls, 1);
			},
		);

		await t.test(
			"does not cache credentialed or user-specific public GETs",
			async () => {
				let fetchCalls = 0;
				globalThis.fetch = (async () => {
					fetchCalls += 1;
					return new Response(`{"call":${fetchCalls}}`, {
						status: 200,
						headers: { "content-type": "application/json" },
					});
				}) as typeof fetch;

				const credentialedRequest = () =>
					new NextRequest(
						"http://localhost:3213/api/v1/public/products?cache-test=2",
						{
							headers: {
								accept: "application/json",
								cookie: "__session=session-token",
							},
						},
					);
				assert.equal(
					await (await GET(credentialedRequest())).text(),
					'{"call":1}',
				);
				assert.equal(
					await (await GET(credentialedRequest())).text(),
					'{"call":2}',
				);

				const cartRequest = () =>
					new NextRequest(
						"http://localhost:3213/api/v1/public/carts/test-cart",
						{ headers: { accept: "application/json" } },
					);
				assert.equal(await (await GET(cartRequest())).text(), '{"call":3}');
				assert.equal(await (await GET(cartRequest())).text(), '{"call":4}');
				assert.equal(fetchCalls, 4);
			},
		);

		await t.test("does not retain a rate-limited public GET", async () => {
			let fetchCalls = 0;
			globalThis.fetch = (async () => {
				fetchCalls += 1;
				return new Response('{"code":429}', {
					status: 429,
					headers: { "content-type": "application/json" },
				});
			}) as typeof fetch;

			const request = () =>
				new NextRequest(
					"http://localhost:3213/api/v1/public/tags/trending?cache-test=3",
					{ headers: { accept: "application/json" } },
				);
			assert.equal((await GET(request())).status, 429);
			assert.equal((await GET(request())).status, 429);
			assert.equal(fetchCalls, 2);
		});

		await t.test(
			"forwards a POST body and preserves multiple Set-Cookie",
			async () => {
				globalThis.fetch = (async (input, init) => {
					assert.equal(
						String(input),
						"https://gateway.example/api/v1/public/auth/web/google",
					);
					assert.equal(init?.method, "POST");
					assert.equal(
						new Headers(init?.headers).get("content-type"),
						"application/json",
					);
					assert.ok(init?.body instanceof ArrayBuffer);
					assert.equal(
						new TextDecoder().decode(init.body),
						'{"credential":"token"}',
					);

					const headers = new Headers({
						"content-type": "application/json",
					});
					headers.append(
						"set-cookie",
						"legacy_session=; Path=/; HttpOnly; Max-Age=0",
					);
					headers.append(
						"set-cookie",
						"hk_session=opaque; Path=/; HttpOnly; SameSite=Lax",
					);
					return new Response('{"code":0}', { status: 200, headers });
				}) as typeof fetch;

				const response = await POST(
					new NextRequest("http://[::1]:3213/api/v1/public/auth/web/google", {
						method: "POST",
						headers: { "content-type": "application/json" },
						body: '{"credential":"token"}',
					}),
				);

				assert.equal(response.status, 200);
				assert.deepEqual(response.headers.getSetCookie(), [
					"legacy_session=; Path=/; HttpOnly; Max-Age=0",
					"hk_session=opaque; Path=/; HttpOnly; SameSite=Lax",
				]);
				assert.equal(await response.text(), '{"code":0}');
			},
		);

		await t.test("preserves a bodyless 204 response", async () => {
			globalThis.fetch = (async () => {
				return new Response(null, {
					status: 204,
					headers: { "x-upstream": "no-content" },
				});
			}) as typeof fetch;

			const response = await POST(
				new NextRequest("http://localhost:3213/api/v1/app/no-content", {
					method: "POST",
					body: "{}",
				}),
			);

			assert.equal(response.status, 204);
			assert.equal(response.headers.get("x-upstream"), "no-content");
			assert.equal(await response.text(), "");
		});
	} finally {
		globalThis.fetch = originalFetch;
		if (originalNodeEnv === undefined) delete process.env.NODE_ENV;
		else process.env.NODE_ENV = originalNodeEnv;
		if (originalProxyFlag === undefined) {
			delete process.env.PLAYWRIGHT_E2E_API_PROXY;
		} else {
			process.env.PLAYWRIGHT_E2E_API_PROXY = originalProxyFlag;
		}
		if (originalApiBase === undefined) delete process.env.API_BASE_URL;
		else process.env.API_BASE_URL = originalApiBase;
	}
});

test("Connect proxy forwards the credentials and every Set-Cookie", async () => {
	// 登入的憑證是 Clerk 的 session token（Authorization: Bearer，或同源的 `__session` cookie）：本機代理要
	// 原樣轉給單體；回應的多個 Set-Cookie 逐筆傳回（new Headers 會把它們併成一個）。
	const originalFetch = globalThis.fetch;
	const originalNodeEnv = process.env.NODE_ENV;
	const originalProxyFlag = process.env.PLAYWRIGHT_E2E_API_PROXY;
	const originalApiBase = process.env.API_BASE_URL;
	process.env.NODE_ENV = "production";
	process.env.PLAYWRIGHT_E2E_API_PROXY = "1";
	process.env.API_BASE_URL = "https://monolith.example/api/v1";
	try {
		globalThis.fetch = (async (input, init) => {
			assert.equal(
				String(input),
				"https://monolith.example/hawkr.identity.v1alpha1.IdentityAppService/GetSession",
			);
			const forwarded = new Headers(init?.headers);
			assert.equal(forwarded.get("authorization"), "Bearer session-token");
			assert.equal(forwarded.get("cookie"), "__session=session-token");
			const headers = new Headers({ "content-type": "application/json" });
			headers.append("set-cookie", "a=1; Path=/; HttpOnly");
			headers.append("set-cookie", "b=2; Path=/; HttpOnly; SameSite=Lax");
			return new Response("{}", { status: 200, headers });
		}) as typeof fetch;

		const response = await connectPOST(
			new NextRequest(
				"http://localhost:3213/api/connect/hawkr.identity.v1alpha1.IdentityAppService/GetSession",
				{
					method: "POST",
					headers: {
						"content-type": "application/json",
						authorization: "Bearer session-token",
						cookie: "__session=session-token",
					},
					body: "{}",
				},
			),
		);

		assert.equal(response.status, 200);
		assert.deepEqual(response.headers.getSetCookie(), [
			"a=1; Path=/; HttpOnly",
			"b=2; Path=/; HttpOnly; SameSite=Lax",
		]);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalNodeEnv === undefined) delete process.env.NODE_ENV;
		else process.env.NODE_ENV = originalNodeEnv;
		if (originalProxyFlag === undefined) {
			delete process.env.PLAYWRIGHT_E2E_API_PROXY;
		} else {
			process.env.PLAYWRIGHT_E2E_API_PROXY = originalProxyFlag;
		}
		if (originalApiBase === undefined) delete process.env.API_BASE_URL;
		else process.env.API_BASE_URL = originalApiBase;
	}
});
