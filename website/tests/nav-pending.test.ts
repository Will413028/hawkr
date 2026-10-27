import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import {
	__resetNavPendingForTest,
	beginNavPending,
} from "../lib/loading/nav-pending";

// The top progress rail is driven purely by html[data-nav-pending]; these
// cases pin the ref-count, reveal-window, and done/abort semantics both
// producers rely on (PageTransition's setLoading mirror and Link's
// atomic-ensure + commit-poll window).

const realSetTimeout = globalThis.setTimeout;
const realClearTimeout = globalThis.clearTimeout;
const realDateNow = Date.now;

let scheduled: (() => void) | null = null;
let now = 0;

const attributes = new Map<string, string>();
const documentElement = {
	setAttribute(name: string, value: string) {
		attributes.set(name, value);
	},
	removeAttribute(name: string) {
		attributes.delete(name);
	},
	getAttribute(name: string) {
		return attributes.has(name) ? (attributes.get(name) as string) : null;
	},
	hasAttribute(name: string) {
		return attributes.has(name);
	},
};

function pendingAttr(): string | null {
	return documentElement.getAttribute("data-nav-pending");
}

/** Advance the fake clock past the reveal window so end() plays a sweep. */
function elapseRevealWindow() {
	now += 400;
}

beforeEach(() => {
	attributes.clear();
	scheduled = null;
	now = 10_000;
	Date.now = () => now;
	globalThis.setTimeout = ((fn: () => void) => {
		scheduled = fn;
		return 1 as unknown as ReturnType<typeof setTimeout>;
	}) as typeof setTimeout;
	globalThis.clearTimeout = (() => {
		scheduled = null;
	}) as typeof clearTimeout;
	(globalThis as { document?: unknown }).document = { documentElement };
});

afterEach(() => {
	__resetNavPendingForTest();
	Date.now = realDateNow;
	globalThis.setTimeout = realSetTimeout;
	globalThis.clearTimeout = realClearTimeout;
	delete (globalThis as { document?: unknown }).document;
});

test("a wait longer than the reveal window sweeps done then clears", () => {
	const end = beginNavPending();
	assert.equal(pendingAttr(), "");
	elapseRevealWindow();
	end();
	assert.equal(pendingAttr(), "done");
	scheduled?.();
	assert.equal(pendingAttr(), null);
});

test("a fast wait clears silently — no done flash inside the reveal delay", () => {
	// Quick-view open/close style navigation: ends in well under 250ms.
	const end = beginNavPending();
	now += 120;
	end();
	assert.equal(pendingAttr(), null, "no sweep for a rail nobody saw");
});

test("abort fades out instead of claiming completion", () => {
	const end = beginNavPending();
	elapseRevealWindow();
	end({ abort: true });
	assert.equal(pendingAttr(), "abort");
	scheduled?.();
	assert.equal(pendingAttr(), null);
});

test("end is idempotent — double invoke cannot underflow a later wait", () => {
	const first = beginNavPending();
	elapseRevealWindow();
	first();
	first();
	scheduled?.();
	const second = beginNavPending();
	assert.equal(pendingAttr(), "");
	elapseRevealWindow();
	second();
	assert.equal(pendingAttr(), "done");
});

test("overlapping producers keep the rail until the last end", () => {
	// Link's ensure window hands over to PageTransition's signal mid-nav.
	const ensureWindow = beginNavPending();
	const pageTransition = beginNavPending();
	elapseRevealWindow();
	ensureWindow();
	assert.equal(pendingAttr(), "", "still pending while one producer active");
	pageTransition();
	assert.equal(pendingAttr(), "done");
});

test("a new wait during the done sweep restarts the trickle", () => {
	const first = beginNavPending();
	elapseRevealWindow();
	first();
	assert.equal(pendingAttr(), "done");
	// Next navigation starts before the sweep timer fires.
	const second = beginNavPending();
	assert.equal(pendingAttr(), "", "sweep interrupted, trickle restarted");
	// The stale sweep timer must not clear the new wait's rail.
	scheduled?.();
	assert.equal(pendingAttr(), "");
	elapseRevealWindow();
	second();
	assert.equal(pendingAttr(), "done");
});

test("restart during the sweep resets the reveal clock", () => {
	const first = beginNavPending();
	elapseRevealWindow();
	first();
	assert.equal(pendingAttr(), "done");
	// New wait restarts the trickle; ending it quickly must clear silently —
	// the restarted animation begins from scaleX(0) again.
	const second = beginNavPending();
	now += 100;
	second();
	assert.equal(pendingAttr(), null);
});

test("SSR (no document) degrades to a no-op", () => {
	delete (globalThis as { document?: unknown }).document;
	const end = beginNavPending();
	assert.doesNotThrow(() => end());
});
