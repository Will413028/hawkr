import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { Code } from "@connectrpc/connect";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { connectError, connectOk, procedureOf } from "./mocks/connect-response";

// The prod flavor requires the browser API base on the web origin; this file's base is http://127.0.0.1:9.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "http://127.0.0.1:9";

interface Seen {
	procedure: string;
	method: string;
	headers: Headers;
}

let seen: Seen[];
const realFetch = globalThis.fetch;

function serve(handler: () => Response | Promise<Response>): void {
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		seen.push({
			procedure: procedureOf(input),
			method: (init?.method ?? "GET").toUpperCase(),
			headers: new Headers(init?.headers),
		});
		return handler();
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

test("sign-out calls the SignOut RPC (POST, same-origin cookie, no Authorization)", async () => {
	serve(() => connectOk({}));
	const { logout } = await import("../lib/api/auth");
	await logout();
	assert.equal(seen.length, 1);
	assert.equal(seen[0]?.procedure, "IdentityAppService/SignOut");
	assert.equal(seen[0]?.method, "POST");
	assert.equal(seen[0]?.headers.get("Authorization"), null);
});

test("a refusal from the monolith keeps sign-out unsuccessful", async () => {
	serve(() => connectError(Code.Internal, "boom"));
	const { logout, LogoutRevokeFailedError } = await import("../lib/api/auth");
	await assert.rejects(logout(), (error: unknown) => {
		assert.ok(error instanceof LogoutRevokeFailedError);
		assert.equal(error.reason, "server");
		return true;
	});
});

test("network failure keeps sign-out unsuccessful", async () => {
	globalThis.fetch = (async () => {
		throw new TypeError("Load failed");
	}) as typeof fetch;
	const { logout, LogoutRevokeFailedError } = await import("../lib/api/auth");
	await assert.rejects(logout(), (error: unknown) => {
		assert.ok(error instanceof LogoutRevokeFailedError);
		assert.equal(error.reason, "network");
		return true;
	});
});

test("a sign-out the monolith answers unauthenticated is a success and broadcasts no expiry", async () => {
	serve(() => connectError(Code.Unauthenticated, "no session"));
	const { AUTH_EXPIRED_EVENT } = await import("../lib/api/client");
	const expired: Event[] = [];
	(
		globalThis as unknown as { dispatchEvent: (e: Event) => boolean }
	).dispatchEvent = (event) => {
		if (event.type === AUTH_EXPIRED_EVENT) expired.push(event);
		return true;
	};
	try {
		const { logout } = await import("../lib/api/auth");
		await logout();
		assert.equal(seen.length, 1);
		assert.deepEqual(expired, []);
	} finally {
		delete (globalThis as Record<string, unknown>).dispatchEvent;
	}
});
