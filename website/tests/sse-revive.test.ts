import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

// F4: a stream that exhausts its downtime budget lands in "unavailable" and
// stops retrying. It must be revived when the tab becomes visible again (the
// commonest case: notifications froze while the tab was backgrounded), and stay
// dead while still hidden. We force "unavailable" with maxDowntimeMs=0 and a
// fetch that always rejects, then drive the visibilitychange listener.

const realFetch = globalThis.fetch;

let fetchCalls = 0;
const winListeners = new Map<string, () => void>();
const docListeners = new Map<string, () => void>();
const documentStub = {
	visibilityState: "hidden" as DocumentVisibilityState,
	addEventListener: (type: string, fn: () => void) => {
		docListeners.set(type, fn);
	},
	removeEventListener: (type: string) => docListeners.delete(type),
};

const flush = () =>
	new Promise((resolve) => realFetch && setTimeout(resolve, 0));

beforeEach(() => {
	fetchCalls = 0;
	winListeners.clear();
	docListeners.clear();
	documentStub.visibilityState = "hidden";
	globalThis.fetch = (async () => {
		fetchCalls += 1;
		throw new Error("network down");
	}) as typeof fetch;
	(globalThis as { window?: unknown }).window = {
		addEventListener: (type: string, fn: () => void) => {
			winListeners.set(type, fn);
		},
		removeEventListener: (type: string) => winListeners.delete(type),
	};
	(globalThis as { document?: unknown }).document = documentStub;
});

afterEach(() => {
	globalThis.fetch = realFetch;
	delete (globalThis as { window?: unknown }).window;
	delete (globalThis as { document?: unknown }).document;
});

async function openDeadStream() {
	const { SseClient } = await import("../lib/sse/client");
	const statuses: string[] = [];
	const client = new SseClient({
		path: "/notifications/stream",
		mode: "public",
		maxDowntimeMs: 0, // first failed connect goes straight to "unavailable"
		handlers: {
			onEvent: () => {},
			onReady: () => {},
			onStatusChange: (s) => statuses.push(s),
		},
	});
	client.open();
	await flush();
	return { client, statuses };
}

test("becoming visible again revives an unavailable stream", async () => {
	const { client, statuses } = await openDeadStream();
	assert.equal(fetchCalls, 1);
	assert.equal(statuses.at(-1), "unavailable");

	documentStub.visibilityState = "visible";
	docListeners.get("visibilitychange")?.();
	await flush();

	assert.equal(fetchCalls, 2, "a visible tab should retry the dead stream");
	client.close();
});

test("staying hidden does not revive the stream", async () => {
	const { client } = await openDeadStream();
	assert.equal(fetchCalls, 1);

	documentStub.visibilityState = "hidden";
	docListeners.get("visibilitychange")?.();
	await flush();

	assert.equal(fetchCalls, 1, "a hidden tab must not reconnect");
	client.close();
});
