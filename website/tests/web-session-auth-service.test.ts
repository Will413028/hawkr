import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { Code } from "@connectrpc/connect";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	connectError,
	connectOk,
	procedureOf,
	sessionJson,
} from "./mocks/connect-response";

interface Seen {
	target: string;
	method: string;
	headers: Headers;
}

let seen: Seen[];
const realFetch = globalThis.fetch;

function serve(
	handler: (target: string) => Response | Promise<Response>,
): void {
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		const target = url.includes("/api/v1/")
			? new URL(url).pathname.replace(/^\/api\/v1/, "")
			: procedureOf(input);
		seen.push({
			target,
			method: (init?.method ?? "GET").toUpperCase(),
			headers: new Headers(init?.headers),
		});
		return handler(target);
	}) as typeof fetch;
}

beforeEach(() => {
	seen = [];
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	getEnv();
	(globalThis as Record<string, unknown>).window = globalThis;
	(globalThis as Record<string, unknown>).Clerk = {
		loaded: true,
		session: { getToken: async () => "clerk-session-token" },
	};
	(globalThis as Record<string, unknown>).localStorage = {
		getItem: () => null,
		setItem: () => {},
		removeItem: () => {},
	};
});

afterEach(() => {
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete (globalThis as Record<string, unknown>).window;
	delete (globalThis as Record<string, unknown>).Clerk;
	delete (globalThis as Record<string, unknown>).localStorage;
});

test("completeSignIn reads GetSession and GetMyStore with the Clerk token", async () => {
	serve((target) =>
		target.endsWith("/GetSession")
			? connectOk(sessionJson("5", { seller: true }))
			: connectOk({ store: { id: "s1", onboardingCompleted: true } }),
	);
	const { completeSignIn } = await import("../lib/api/auth");
	const account = await completeSignIn("google");
	assert.deepEqual(
		seen.map((r) => r.target),
		["IdentityAppService/GetSession", "SellerAppService/GetMyStore"],
	);
	assert.equal(seen[0]?.method, "GET");
	assert.equal(seen[1]?.method, "GET");
	assert.equal(
		seen[0]?.headers.get("Authorization"),
		"Bearer clerk-session-token",
	);
	assert.equal(
		seen[1]?.headers.get("Authorization"),
		"Bearer clerk-session-token",
	);
	assert.equal(account.accountId, "5");
	assert.equal(account.isSeller, true);
	assert.equal("perspective" in account, false);
	assert.equal(account.needsOnboarding, false);
});

test("buyer-only sign-in does not request seller onboarding", async () => {
	serve(() => connectOk(sessionJson("8")));
	const { completeSignIn } = await import("../lib/api/auth");
	const account = await completeSignIn("line");
	assert.deepEqual(
		seen.map((r) => r.target),
		["IdentityAppService/GetSession"],
	);
	assert.equal(account.needsOnboarding, true);
});

test("seller sign-in reports the seller role and onboarding state", async () => {
	serve((target) =>
		target.endsWith("/GetSession")
			? connectOk(sessionJson("9", { seller: true }))
			: connectOk({ store: { id: "s9", onboardingCompleted: false } }),
	);
	const { completeSignIn } = await import("../lib/api/auth");
	const account = await completeSignIn("apple");
	assert.equal(account.isSeller, true);
	assert.equal(account.needsOnboarding, true);
});

test("fetchSession returns the account from GetSession", async () => {
	serve(() => connectOk(sessionJson("7")));
	const { fetchSession } = await import("../lib/api/auth");
	const identity = await fetchSession();
	assert.ok(identity && typeof identity === "object");
	assert.equal(identity.accountId, "7");
	assert.equal(seen[0]?.target, "IdentityAppService/GetSession");
	assert.equal(seen[0]?.method, "GET");
});

test("backend outage and a dead network remain retryable", async () => {
	const { fetchSession, SESSION_UNAVAILABLE } = await import("../lib/api/auth");
	serve(() => connectError(Code.Unavailable, "down"));
	assert.equal(await fetchSession(), SESSION_UNAVAILABLE);
	globalThis.fetch = (async () => {
		throw new TypeError("Load failed");
	}) as typeof fetch;
	assert.equal(await fetchSession(), SESSION_UNAVAILABLE);
});

test("unavailable Clerk token prevents the GetSession request", async () => {
	(globalThis as Record<string, unknown>).Clerk = {
		loaded: true,
		session: { getToken: async () => null },
	};
	serve(() => connectOk(sessionJson("7")));
	const { fetchSession, SESSION_UNAVAILABLE } = await import("../lib/api/auth");
	assert.equal(await fetchSession(), SESSION_UNAVAILABLE);
	assert.equal(seen.length, 0);
});

test("backend unauthenticated resolves guest; permission denied remains distinct", async () => {
	const { fetchSession } = await import("../lib/api/auth");
	serve(() => connectError(Code.Unauthenticated));
	assert.equal(await fetchSession(), null);
	serve(() => connectError(Code.PermissionDenied));
	await assert.rejects(fetchSession(), (error: unknown) => {
		assert.equal((error as { code?: Code }).code, Code.PermissionDenied);
		return true;
	});
});

test("completeSignIn preserves the backend rejection code", async () => {
	serve(() => connectError(Code.Unauthenticated));
	const { completeSignIn } = await import("../lib/api/auth");
	await assert.rejects(completeSignIn("google"), (error: unknown) => {
		assert.equal((error as { code?: Code }).code, Code.Unauthenticated);
		return true;
	});
});

for (const code of [
	Code.FailedPrecondition,
	Code.Aborted,
	Code.InvalidArgument,
]) {
	test(`fetchSession keeps session unresolved for ${Code[code]}`, async () => {
		serve(() => connectError(code));
		const { fetchSession, SESSION_UNAVAILABLE } = await import(
			"../lib/api/auth"
		);
		assert.equal(await fetchSession(), SESSION_UNAVAILABLE);
	});
}
test("fetchSession keeps malformed responses unresolved", async () => {
	serve(() => connectOk({}));
	const { fetchSession, SESSION_UNAVAILABLE } = await import("../lib/api/auth");
	assert.equal(await fetchSession(), SESSION_UNAVAILABLE);
});
