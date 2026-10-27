import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { siteMiddleware } from "../middleware";

const signedOut = async () => ({ userId: null });
const signedIn = async () => ({ userId: "user_1" });

function request(path: string) {
	return new NextRequest(`http://localhost:3000${path}`);
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
		const to = location(await siteMiddleware(request(path), signedOut));
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
		const to = location(await siteMiddleware(request(path), signedOut));
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
		const response = await siteMiddleware(request(path), signedIn);
		assert.equal(location(response), null, path);
	}
	for (const path of ["/seller", "/seller/pricing", "/seller/onboarding"]) {
		const response = await siteMiddleware(request(path), signedOut);
		assert.equal(location(response), null, path);
	}
});

test("the login return path is the requested URL byte for byte: an encoded %25 is not decoded to %", async () => {
	const path = "/account/orders/a%25b?tab=open%25";
	const to = location(await siteMiddleware(request(path), signedOut));
	assert.ok(to);
	assert.equal(to.pathname, "/login");
	assert.equal(to.searchParams.get("redirect"), path);
});
