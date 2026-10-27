import assert from "node:assert/strict";
import { test } from "node:test";
import { redirectForRole, requiredAccessForPath } from "../lib/auth/redirect";

test("seller import preview is protected as a seller route", () => {
	assert.equal(
		requiredAccessForPath("/seller-product-import-preview/job-1"),
		"seller",
	);
	assert.equal(
		requiredAccessForPath("/seller-product-import-preview/job-1/items/item-1"),
		"seller",
	);
});

test("buyer sessions cannot be redirected into seller import preview", () => {
	assert.equal(
		redirectForRole("/seller-product-import-preview/job-1", "buyer"),
		"/",
	);
	assert.equal(
		redirectForRole("/seller-product-import-preview/job-1", "seller"),
		"/seller-product-import-preview/job-1",
	);
});

test("role redirects classify protected roots before query and hash", () => {
	const cases = [
		{ redirect: "/account?tab=orders#recent", role: "buyer" as const },
		{ redirect: "/checkout?coupon=1", role: "buyer" as const },
		{ redirect: "/seller-center?tab=orders#today", role: "seller" as const },
		{
			redirect: "/seller-product-import-preview?job=1",
			role: "seller" as const,
		},
	];

	for (const { redirect, role } of cases) {
		assert.equal(redirectForRole(redirect, role), redirect);
		assert.equal(
			redirectForRole(redirect, role === "buyer" ? "seller" : "buyer"),
			role === "buyer" ? "/seller-center/dashboard" : "/",
		);
	}
});

test("role redirect falls back when called with an unsafe target", () => {
	assert.equal(redirectForRole("//evil.example", "buyer"), "/");
	assert.equal(
		redirectForRole("/\\evil.example", "seller"),
		"/seller-center/dashboard",
	);
});

test("access classes are resolved from one table: buyer, seller, account, public", () => {
	assert.equal(requiredAccessForPath("/account"), "buyer");
	assert.equal(requiredAccessForPath("/account/orders"), "buyer");
	assert.equal(requiredAccessForPath("/checkout"), "buyer");
	assert.equal(requiredAccessForPath("/seller-center/orders"), "seller");
	assert.equal(requiredAccessForPath("/seller/measures"), "account");
	for (const pathname of [
		"/seller/measures/m1",
		"/seller/measures/m1/appeal",
		"/seller/measures/m1/remediation",
	]) {
		assert.equal(requiredAccessForPath(pathname), "account", pathname);
	}
	// The recruiting pages and onboarding sit under /seller but need no access class.
	for (const pathname of [
		"/seller",
		"/seller/pricing",
		"/seller/onboarding",
		"/seller/measuresx",
		"/",
		"/market",
	]) {
		assert.equal(requiredAccessForPath(pathname), null, pathname);
	}
});

test("an account page is allowed for any workspace slot, workspace pages still are not", () => {
	for (const slot of ["buyer", "seller", null] as const) {
		assert.equal(
			redirectForRole("/seller/measures?from=notice", slot),
			"/seller/measures?from=notice",
			String(slot),
		);
		assert.equal(
			redirectForRole("/seller/measures/m1/appeal", slot),
			"/seller/measures/m1/appeal",
			String(slot),
		);
	}
	assert.equal(redirectForRole("/seller-center/orders", "buyer"), "/");
});
