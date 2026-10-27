import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

// F1: React Query passes its own AbortSignal (aborts on unmount / key change),
// which carries NO time budget. The transport must COMPOSE it with the timeout
// so a hung upstream still aborts — not drop the timeout because a signal was
// supplied. These drive apiClient with a fetch that only settles on abort.

const realFetch = globalThis.fetch;

// A fetch that never resolves on its own — it rejects only when the request's
// (composed) signal aborts, mirroring a hung upstream. Already-aborted signals
// reject immediately so the transport's one GET retry can't hang the test.
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
	const controller = new AbortController(); // caller signal, never aborted
	const started = Date.now();
	await assert.rejects(() =>
		apiClient.public.get("/x", { signal: controller.signal, timeoutMs: 200 }),
	);
	// The composed timeout — not the (idle) caller signal — has to fire here.
	assert.ok(Date.now() - started < 2000, "should abort near the 200ms timeout");
});

test("a caller abort still aborts the request when composed", async () => {
	const { apiClient } = await import("../lib/api/client");
	const controller = new AbortController();
	const pending = apiClient.public.get("/x", {
		signal: controller.signal,
		timeoutMs: 60_000, // far away — only the caller abort can end this
	});
	controller.abort(new Error("caller-abort"));
	await assert.rejects(() => pending);
});
