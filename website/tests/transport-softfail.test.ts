import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";

const realFetch = globalThis.fetch;
let lastInit: RequestInit & { next?: unknown };
let respond: () => Response | Promise<Response>;

beforeEach(() => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	delete (globalThis as { window?: unknown }).window;
	respond = () =>
		new Response(JSON.stringify({ code: 0, data: { ok: true } }), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	// @ts-expect-error test stub
	globalThis.fetch = async (_url: string, init: RequestInit) => {
		lastInit = init;
		return respond();
	};
});

afterEach(() => {
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("softFail GET on 5xx → {code,data:null}, no throw", async () => {
	respond = () => new Response("bad gateway", { status: 502 });
	const { apiClient } = await import("../lib/api/client");
	const { code, data } = await apiClient.public.get("/marketplace", {
		softFail: true,
	});
	assert.equal(code, 502);
	assert.equal(data, null);
});

test("softFail GET on 404 → {code,data:null}", async () => {
	respond = () => new Response("not found", { status: 404 });
	const { apiClient } = await import("../lib/api/client");
	const { code, data } = await apiClient.public.get("/sellers/1", {
		softFail: true,
	});
	assert.equal(code, 404);
	assert.equal(data, null);
});

test("softFail GET on a hung/unreachable upstream → {code:503,data:null}", async () => {
	respond = () => {
		throw new Error("ETIMEDOUT");
	};
	const { apiClient } = await import("../lib/api/client");
	const { code, data } = await apiClient.public.get("/categories/tree", {
		softFail: true,
	});
	assert.equal(code, 503);
	assert.equal(data, null);
});

test("softFail does NOT soften a mutation (POST still throws)", async () => {
	respond = () => new Response(JSON.stringify({ code: 503 }), { status: 503 });
	const { apiClient, ApiClientError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.public.post("/posts/1/view", { softFail: true }),
		(e: unknown) => e instanceof ApiClientError,
	);
});

test("softFail GET still throws on a real 4xx client error (400)", async () => {
	respond = () => new Response(JSON.stringify({ code: 400 }), { status: 400 });
	const { apiClient, ApiClientError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.public.get("/products", { softFail: true }),
		(e: unknown) =>
			e instanceof ApiClientError && (e as { status: number }).status === 400,
	);
});

test("softFailNotFound GET on 404 → {code:404,data:null}", async () => {
	respond = () => new Response("not found", { status: 404 });
	const { apiClient } = await import("../lib/api/client");
	const { code, data } = await apiClient.public.get("/sellers/1", {
		softFailNotFound: true,
	});
	assert.equal(code, 404);
	assert.equal(data, null);
});

test("softFailNotFound GET on 5xx THROWS (cache keeps stale)", async () => {
	respond = () => new Response("bad gateway", { status: 502 });
	const { apiClient, ApiClientError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.public.get("/sellers/1", { softFailNotFound: true }),
		(e: unknown) =>
			e instanceof ApiClientError && (e as { status: number }).status === 502,
	);
});

test("softFailNotFound GET on a hung/unreachable upstream THROWS", async () => {
	respond = () => {
		throw new Error("ETIMEDOUT");
	};
	const { apiClient } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.public.get("/sellers/1", { softFailNotFound: true }),
		(e: unknown) =>
			e instanceof Error && /ETIMEDOUT/.test((e as Error).message),
	);
});

test("softFailNotFound GET on an unparseable 200 body THROWS", async () => {
	respond = () =>
		new Response("<html>not json</html>", {
			status: 200,
			headers: { "content-type": "text/html" },
		});
	const { apiClient, ApiClientError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.public.get("/sellers/1", { softFailNotFound: true }),
		(e: unknown) => e instanceof ApiClientError,
	);
});

test("softFail wins over softFailNotFound when both are set (5xx → null)", async () => {
	respond = () => new Response("bad gateway", { status: 502 });
	const { apiClient } = await import("../lib/api/client");
	const { code, data } = await apiClient.public.get("/sellers/1", {
		softFail: true,
		softFailNotFound: true,
	});
	assert.equal(code, 502);
	assert.equal(data, null);
});

test("server-side GET passes next:{revalidate} through to fetch init", async () => {
	const { apiClient } = await import("../lib/api/client");
	await apiClient.public.get("/marketplace", { next: { revalidate: 300 } });
	assert.deepEqual(lastInit.next, { revalidate: 300 });
});
