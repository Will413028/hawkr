import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import { NextRequest } from "next/server";
import { GET, POST } from "../src/app/api/bff/[...path]/route";
import { adminSession } from "../src/lib/session";

const realFetch = globalThis.fetch;

const ADMIN_TOKEN = "access-admin-jwt";
let token: string | null;

beforeEach(() => {
	token = ADMIN_TOKEN;
	mock.method(adminSession, "token", async () => token);
});

afterEach(() => {
	globalThis.fetch = realFetch;
	mock.restoreAll();
});

function bffRequest(
	path: string[],
	init: { method?: string; headers?: Record<string, string> } = {},
) {
	const request = new NextRequest(
		`http://localhost/api/bff/${path.join("/")}`,
		{ method: init.method ?? "GET", headers: init.headers ?? {} },
	);
	const handler = init.method === "POST" ? POST : GET;
	return handler(request, { params: Promise.resolve({ path }) });
}

/** 記下送往後端的請求；回應可以帶 Set-Cookie。 */
function captureUpstream(setCookies: string[] = []) {
	const seen: { headers?: Headers; calls: number } = { calls: 0 };
	globalThis.fetch = (async (_input, init) => {
		seen.calls++;
		seen.headers = new Headers(init?.headers as HeadersInit);
		const headers = new Headers({ "content-type": "application/json" });
		for (const cookie of setCookies) headers.append("set-cookie", cookie);
		return new Response(JSON.stringify({ code: 0, data: {} }), {
			status: 200,
			headers,
		});
	}) as typeof fetch;
	return seen;
}

/**
 * PAYUni 相關 admin 呼叫走一般 /api/v1/admin/** BFF，不吃 seller token/cookie/
 * delegated session——issue #2806 spec §3.6 把這條列為已驗證為真的不變量，這支
 * 測試把它從「讀碼確認」升級成「回歸鎖」。
 */
test("general BFF rejects a request carrying a reserved delegated-operation header", async () => {
	const upstream = captureUpstream();
	const response = await bffRequest(["api", "v1", "admin", "sellers", "42"], {
		headers: { "X-HK-Operation-Session": "should-be-rejected" },
	});
	assert.equal(response.status, 403);
	const body = await response.json();
	assert.equal(body.message, "請求包含保留的代行營運標頭");
	assert.equal(upstream.calls, 0);
});

/**
 * BFF 轉送 Access JWT；瀏覽器的 Authorization header 與 cookie 都不轉。
 */
test("general BFF sends the Access JWT as Bearer and forwards no browser credential", async () => {
	const upstream = captureUpstream();

	await bffRequest(["api", "v1", "admin", "sellers", "42"], {
		headers: {
			Authorization: "Bearer browser-supplied-token",
			Cookie: "__session=browser-cookie-token; some_other_cookie=x",
		},
	});

	assert.equal(upstream.calls, 1);
	assert.equal(upstream.headers?.get("authorization"), `Bearer ${ADMIN_TOKEN}`);
	assert.equal(upstream.headers?.get("cookie"), null);
});

test("general BFF reads Access assertion from the incoming request", async () => {
	mock.restoreAll();
	const upstream = captureUpstream();
	await bffRequest(["api", "v1", "admin", "sellers", "42"], {
		headers: {
			"Cf-Access-Jwt-Assertion": "signed-access-jwt",
			Authorization: "Bearer browser-supplied-token",
		},
	});
	assert.equal(
		upstream.headers?.get("authorization"),
		"Bearer signed-access-jwt",
	);
});

test("general BFF answers 401 without calling the backend when Access header is absent", async () => {
	token = null;
	const upstream = captureUpstream();

	const response = await bffRequest([
		"hawkr.admin.identity.v1alpha1.IdentityAdminService",
		"GetSession",
	]);

	assert.equal(response.status, 401);
	assert.equal(upstream.calls, 0);
});

test("general BFF never relays the backend's Set-Cookie to the browser", async () => {
	captureUpstream(["tracking=1; Path=/", "__session=x; Path=/; HttpOnly"]);

	const response = await bffRequest(
		["hawkr.admin.identity.v1alpha1.IdentityAdminService", "SignOut"],
		{ method: "POST" },
	);

	assert.equal(response.status, 200);
	assert.deepEqual(response.headers.getSetCookie(), []);
});
