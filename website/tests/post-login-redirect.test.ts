import assert from "node:assert/strict";
import test from "node:test";
import {
	resolvePostLoginTarget,
	sellerPostLoginRedirect,
	sellerWorkspaceRedirect,
} from "../lib/auth/post-login-redirect";

test("completed seller continues directly to the requested workspace", () => {
	assert.equal(
		sellerPostLoginRedirect("/seller-center/orders", false, true),
		"/seller-center/orders",
	);
	assert.equal(
		sellerPostLoginRedirect("/seller-center/orders?tab=pending", false, true),
		"/seller-center/orders?tab=pending",
	);
});

test("seller login never lands on buyer or public pages first", () => {
	for (const redirect of ["", "/", "/market", "/account/orders", "/checkout"]) {
		assert.equal(
			sellerPostLoginRedirect(redirect, false, true),
			"/seller-center/dashboard",
			redirect,
		);
	}
	assert.equal(
		sellerWorkspaceRedirect("//evil.example"),
		"/seller-center/dashboard",
	);
});

test("incomplete seller enters onboarding and preserves the requested workspace", () => {
	assert.equal(
		sellerPostLoginRedirect(
			"/seller-center/orders?tab=after-sales",
			true,
			true,
		),
		"/seller/onboarding?redirect=%2Fseller-center%2Forders%3Ftab%3Dafter-sales",
	);
	assert.equal(sellerPostLoginRedirect("/", true, true), "/seller/onboarding");
});

test("missing contract field fails closed through the onboarding gate", () => {
	assert.equal(
		sellerPostLoginRedirect("/seller-center/dashboard", undefined, true),
		"/seller/onboarding?redirect=%2Fseller-center%2Fdashboard",
	);
});

test("a seller-selected login without the seller role fails closed to onboarding", () => {
	assert.equal(
		sellerPostLoginRedirect("/seller-center/orders", false, false),
		"/seller/onboarding?redirect=%2Fseller-center%2Forders",
	);
});

test("dual-role login honors the explicitly selected workspace", () => {
	assert.deepEqual(
		resolvePostLoginTarget({
			requestedRole: "buyer",
			redirectAfter: "/",
			account: { isSeller: true },
			needsOnboarding: false,
		}),
		{ activeRole: "buyer", destination: "/" },
	);
	assert.deepEqual(
		resolvePostLoginTarget({
			requestedRole: "seller",
			redirectAfter: "/",
			account: { isSeller: true },
			needsOnboarding: false,
		}),
		{
			activeRole: "seller",
			destination: "/seller-center/dashboard",
		},
	);
});

test("buyer-only seller intent keeps the buyer role while entering onboarding", () => {
	assert.deepEqual(
		resolvePostLoginTarget({
			requestedRole: "seller",
			redirectAfter: "/seller-center/orders",
			account: { isSeller: false },
			needsOnboarding: false,
		}),
		{
			activeRole: "buyer",
			destination: "/seller/onboarding?redirect=%2Fseller-center%2Forders",
		},
	);
});

test("without a requested workspace the default is client-side: stored seller workspace for a seller, else buyer", () => {
	// The default rule lives in the client (lib/auth/active-role.ts); GetSession
	// does not return a perspective (ADR 2026-10-07-connect-without-perspective).
	assert.deepEqual(
		resolvePostLoginTarget({
			requestedRole: null,
			redirectAfter: "/",
			account: { isSeller: true },
			needsOnboarding: false,
			storedRole: "seller",
		}),
		{ activeRole: "seller", destination: "/seller-center/dashboard" },
	);
	// Stored seller workspace but the account no longer has the seller role.
	assert.deepEqual(
		resolvePostLoginTarget({
			requestedRole: null,
			redirectAfter: "/",
			account: { isSeller: false },
			needsOnboarding: undefined,
			storedRole: "seller",
		}),
		{ activeRole: "buyer", destination: "/" },
	);
	// Nothing stored: buyer, even for a seller account.
	assert.deepEqual(
		resolvePostLoginTarget({
			requestedRole: null,
			redirectAfter: "/",
			account: { isSeller: true },
			needsOnboarding: false,
			storedRole: null,
		}),
		{ activeRole: "buyer", destination: "/" },
	);
});

test("an account page is the destination whatever the seller state or requested workspace", () => {
	const redirectAfter = "/seller/measures/m1/appeal?x=1";
	const cases = [
		{ requestedRole: null, account: { isSeller: false }, storedRole: null },
		{ requestedRole: "buyer", account: { isSeller: true }, storedRole: null },
		{ requestedRole: "seller", account: { isSeller: true }, storedRole: null },
		// 店家全關的帳號沒有賣家角色：選賣家分頁也不能被送去開店頁。
		{ requestedRole: "seller", account: { isSeller: false }, storedRole: null },
		{ requestedRole: null, account: { isSeller: true }, storedRole: "seller" },
	] as const;
	for (const input of cases) {
		for (const needsOnboarding of [true, false, undefined]) {
			const target = resolvePostLoginTarget({
				...input,
				redirectAfter,
				needsOnboarding,
			});
			assert.equal(
				target.destination,
				redirectAfter,
				JSON.stringify({ input, needsOnboarding }),
			);
		}
	}
});

test("an unsafe account-looking redirect is not trusted", () => {
	assert.equal(
		resolvePostLoginTarget({
			requestedRole: "seller",
			redirectAfter: "//evil.example/seller/measures",
			account: { isSeller: true },
			needsOnboarding: false,
		}).destination,
		"/seller-center/dashboard",
	);
});
