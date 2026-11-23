import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import { NextRequest } from "next/server";
import { PUT } from "../src/app/api/bff/[...path]/route";
import { adminSession } from "../src/lib/session";
import { BACKEND_BASE_URL } from "../src/services/backend";

// 代行營運的機敏檔段上傳（B1-D16）：admin 的瀏覽器打 /api/bff/api/files/admin/secure_uploads/{token}/chunks/{index}，
// 一般 BFF 把 PUT 的原始位元組原樣轉到單體根目錄的 /api/files/admin/…，帶 Access 換來的 Bearer。

const realFetch = globalThis.fetch;

beforeEach(() => {
	mock.method(adminSession, "token", async () => "access-admin-jwt");
});

afterEach(() => {
	globalThis.fetch = realFetch;
	mock.restoreAll();
});

test("BFF forwards a chunk PUT body byte-for-byte to the monolith's admin files route", async () => {
	const seen: {
		url?: string;
		method?: string;
		headers?: Headers;
		body?: Uint8Array;
	} = {};
	globalThis.fetch = (async (input, init) => {
		seen.url = String(input);
		seen.method = init?.method;
		seen.headers = new Headers(init?.headers as HeadersInit);
		seen.body = new Uint8Array(init?.body as ArrayBuffer);
		return new Response(null, { status: 204 });
	}) as typeof fetch;

	// 不是合法的 UTF-8：任何文字轉碼都會改掉這些位元組。
	const body = new Uint8Array(4096);
	for (let i = 0; i < body.length; i++) body[i] = (i * 31 + 7) % 256;
	body[0] = 0xff;
	body[1] = 0xfe;
	const path = [
		"api",
		"files",
		"admin",
		"secure_uploads",
		"tok123",
		"chunks",
		"3",
	];
	const request = new NextRequest(
		`http://localhost/api/bff/${path.join("/")}`,
		{
			method: "PUT",
			headers: { "content-type": "application/octet-stream" },
			body,
		},
	);
	const response = await PUT(request, { params: Promise.resolve({ path }) });

	assert.equal(response.status, 204);
	assert.equal(seen.method, "PUT");
	assert.equal(
		seen.url,
		`${BACKEND_BASE_URL}/api/files/admin/secure_uploads/tok123/chunks/3`,
	);
	assert.equal(seen.headers?.get("authorization"), "Bearer access-admin-jwt");
	assert.ok(seen.body, "上游收到 body");
	assert.deepEqual(Buffer.from(seen.body), Buffer.from(body));
});

test("BFF passes the monolith's 409 and Hawkr-Next-Index back to the browser", async () => {
	globalThis.fetch = (async () =>
		new Response("conflict", {
			status: 409,
			headers: { "Hawkr-Next-Index": "2" },
		})) as typeof fetch;
	const path = [
		"api",
		"files",
		"admin",
		"secure_uploads",
		"tok",
		"chunks",
		"5",
	];
	const response = await PUT(
		new NextRequest(`http://localhost/api/bff/${path.join("/")}`, {
			method: "PUT",
			body: new Uint8Array([1, 2, 3]),
		}),
		{ params: Promise.resolve({ path }) },
	);
	assert.equal(response.status, 409);
	assert.equal(response.headers.get("hawkr-next-index"), "2");
});
