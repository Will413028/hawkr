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
	credentials: RequestCredentials | undefined;
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
			credentials: init?.credentials,
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
	delete (globalThis as Record<string, unknown>).localStorage;
});

test("fetchSession returns the account from GetSession", async () => {
	serve(() => connectOk(sessionJson("7")));
	const { fetchSession } = await import("../lib/api/auth");
	const identity = await fetchSession();
	assert.ok(identity && typeof identity === "object");
	assert.equal(identity.accountId, "7");
	assert.equal(seen[0]?.target, "IdentityAppService/GetSession");
	assert.equal(seen[0]?.method, "GET");
	// 憑證是同源的 HttpOnly session cookie，由瀏覽器附上；程式不碰 Authorization。
	assert.equal(seen[0]?.headers.get("Authorization"), null);
	assert.equal(seen[0]?.credentials, "include");
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
