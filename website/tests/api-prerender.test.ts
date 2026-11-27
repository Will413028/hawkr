import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

const origPhase = process.env.NEXT_PHASE;
const origNodeEnv = process.env.NODE_ENV;

afterEach(() => {
	process.env.NEXT_PHASE = origPhase;
	(process.env as Record<string, string | undefined>).NODE_ENV = origNodeEnv;
});

test("build phase: swallows error, returns fallback", async () => {
	process.env.NEXT_PHASE = "phase-production-build";
	(process.env as Record<string, string>).NODE_ENV = "production";
	const { prerenderSafe } = await import("../lib/api/prerender");
	const result = prerenderSafe<null>(null)(new Error("backend 502"));
	assert.equal(result, null);
});

test("runtime production: rethrows", async () => {
	delete process.env.NEXT_PHASE;
	(process.env as Record<string, string>).NODE_ENV = "production";
	const { prerenderSafe } = await import("../lib/api/prerender");
	assert.throws(
		() => prerenderSafe<null>(null)(new Error("backend 502")),
		/502/,
	);
});

test("runtime production: definitive 4xx (ApiClientError) still soft-fails", async () => {
	delete process.env.NEXT_PHASE;
	(process.env as Record<string, string>).NODE_ENV = "production";
	const { prerenderSafe } = await import("../lib/api/prerender");
	const { ApiClientError } = await import("../lib/api/client");
	const result = prerenderSafe<null>(null)(
		new ApiClientError("invalid seller id", 400),
	);
	assert.equal(result, null);
});

test("runtime production: a rate-limited read rethrows, it is not a null", async () => {
	delete process.env.NEXT_PHASE;
	(process.env as Record<string, string>).NODE_ENV = "production";
	const { prerenderSafe } = await import("../lib/api/prerender");
	const { ApiClientError } = await import("../lib/api/client");
	assert.throws(
		() => prerenderSafe<null>(null)(new ApiClientError("rate limited", 429)),
		/rate limited/,
	);
});

test("dev: swallows error, returns fallback", async () => {
	delete process.env.NEXT_PHASE;
	(process.env as Record<string, string>).NODE_ENV = "development";
	const { prerenderSafe } = await import("../lib/api/prerender");
	const fallback = { records: [], total: 0 };
	assert.deepEqual(prerenderSafe(fallback)(new Error("x")), fallback);
});
