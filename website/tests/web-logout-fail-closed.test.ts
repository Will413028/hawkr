import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { Code } from "@connectrpc/connect";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { connectError, connectOk, procedureOf } from "./mocks/connect-response";

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
