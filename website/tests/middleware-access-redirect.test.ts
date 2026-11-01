import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { siteMiddleware } from "../middleware";

// 登入與否只看 session cookie 在不在（middleware.ts；值不驗，權威是 GetSession）。
const SIGNED_IN = { cookie: "__Host-hk_session=opaque" };

function request(path: string, headers: Record<string, string> = {}) {
	return new NextRequest(`http://localhost:3000${path}`, { headers });
}

function location(response: Response): URL | null {
	const value = response.headers.get("location");
	return value ? new URL(value) : null;
}

test("a guest on a buyer or seller page is sent to login with that workspace role", async () => {
	for (const [path, role] of [
		["/account/orders", "buyer"],
		["/checkout", "buyer"],
		["/seller-center/orders", "seller"],
	] as const) {
		const to = location(await siteMiddleware(request(path)));
		assert.ok(to, path);
		assert.equal(to.pathname, "/login");
		assert.equal(to.searchParams.get("role"), role, path);
		assert.equal(to.searchParams.get("redirect"), path, path);
	}
});

test("a guest on an account page is sent to login without a role, keeping the return path", async () => {
	for (const path of [
		"/seller/measures",
		"/seller/measures/m1/appeal?from=list",
		"/seller/measures/m1/remediation",
	]) {
		const to = location(await siteMiddleware(request(path)));
		assert.ok(to, path);
		assert.equal(to.pathname, "/login");
		assert.equal(to.searchParams.has("role"), false, path);
		assert.equal(to.searchParams.get("redirect"), path, path);
	}
});

test("a signed-in visitor passes every access class, and public /seller pages never redirect", async () => {
	for (const path of [
		"/account",
		"/seller-center/dashboard",
		"/seller/measures",
		"/seller/measures/m1/appeal",
		"/seller/measures/m1/remediation",
	]) {
		const response = await siteMiddleware(request(path, SIGNED_IN));
		assert.equal(location(response), null, path);
	}
	for (const path of ["/seller", "/seller/pricing", "/seller/onboarding"]) {
		const response = await siteMiddleware(request(path));
		assert.equal(location(response), null, path);
	}
});

test("the login return path is the requested URL byte for byte: an encoded %25 is not decoded to %", async () => {
	const path = "/account/orders/a%25b?tab=open%25";
	const to = location(await siteMiddleware(request(path)));
	assert.ok(to);
	assert.equal(to.pathname, "/login");
	assert.equal(to.searchParams.get("redirect"), path);
});

test("only the __Host-hk_session cookie counts: other cookies and an empty value do not admit a guarded path", async () => {
	for (const cookie of [
		"__session=opaque",
		"hk_session=opaque",
		"__Host-hk_session=",
		"session=__Host-hk_session",
	]) {
		const to = location(
			await siteMiddleware(request("/account/orders", { cookie })),
		);
		assert.ok(to, cookie);
		assert.equal(to.pathname, "/login", cookie);
	}
});

test("the session cookie is checked for presence only: the API is never called", async () => {
	const realFetch = globalThis.fetch;
	globalThis.fetch = (async () => {
		throw new Error("middleware must not call the API to admit a cookie");
	}) as typeof fetch;
	try {
		for (const path of ["/account/orders", "/seller-center/orders"]) {
			const response = await siteMiddleware(request(path, SIGNED_IN));
			assert.equal(location(response), null, path);
		}
	} finally {
		globalThis.fetch = realFetch;
	}
});

// 受保護區的 layout 讀不到 pathname 與 query：middleware 把要求的 URL 放進 request header，
// DAL 判定「cookie 在但單體拒絕」時用它組登入頁的 `redirect`。
function forwardedReturnPath(response: Response): string | null {
	return response.headers.get("x-middleware-request-x-hk-return-path");
}

test("a signed-in visitor on a guarded path forwards the requested URL as a request header", async () => {
	const path = "/account/orders/a%25b?tab=open%25";
	const response = await siteMiddleware(request(path, SIGNED_IN));
	assert.equal(forwardedReturnPath(response), path);
});

test("a client-sent return-path header is overwritten, not trusted", async () => {
	const response = await siteMiddleware(
		request("/account", {
			...SIGNED_IN,
			"x-hk-return-path": "//evil.example/steal",
		}),
	);
	assert.equal(forwardedReturnPath(response), "/account");
});

test("a public path forwards no return path", async () => {
	const response = await siteMiddleware(request("/about", SIGNED_IN));
	assert.equal(forwardedReturnPath(response), null);
});
