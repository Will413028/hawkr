import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

const realFetch = globalThis.fetch;

function hangUnlessAborted() {
	globalThis.fetch = ((_url: string | URL | Request, init: RequestInit = {}) =>
		new Promise((_resolve, reject) => {
			const signal = init.signal;
			if (signal?.aborted) {
				reject(signal.reason ?? new Error("aborted"));
				return;
			}
			signal?.addEventListener(
				"abort",
				() => reject(signal.reason ?? new Error("aborted")),
				{ once: true },
			);
		})) as typeof fetch;
}

beforeEach(() => {
	hangUnlessAborted();
});
afterEach(() => {
	globalThis.fetch = realFetch;
});

test("a caller signal is composed with the timeout (a hung upstream still aborts)", async () => {
	const { apiClient } = await import("../lib/api/client");
	const controller = new AbortController();
	const started = Date.now();
	await assert.rejects(() =>
		apiClient.public.get("/x", { signal: controller.signal, timeoutMs: 200 }),
	);
	assert.ok(Date.now() - started < 2000, "should abort near the 200ms timeout");
});

test("a caller abort still aborts the request when composed", async () => {
	const { apiClient } = await import("../lib/api/client");
	const controller = new AbortController();
	const pending = apiClient.public.get("/x", {
		signal: controller.signal,
		timeoutMs: 60_000,
	});
	controller.abort(new Error("caller-abort"));
	await assert.rejects(() => pending);
});
