import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import {
	__resetNavigationWatchdogForTest,
	armNavigationWatchdog,
	NAVIGATION_WATCHDOG_MS,
} from "../lib/loading/navigation-watchdog";
import {
	__resetNavigationObservabilityForTest,
	beginNavigationIntent,
	markRouterTransitionStarted,
	navigateDocument,
} from "../lib/navigation/observability";

// F2: a hung soft-nav (RSC fetch that never rejects) leaves the user on the
// route skeleton forever. The watchdog arms on transition start and, if the URL
// still hasn't committed when the timer fires, forces a hard navigation.

const realSetTimeout = globalThis.setTimeout;
const realClearTimeout = globalThis.clearTimeout;

let scheduled: (() => void) | null = null;
let scheduledDelay: number | undefined;
let assigned: string | null = null;
let replaced: string | null = null;
let readyMarkers: Array<{
	productId: string;
	hidden?: boolean;
	visible?: boolean;
}> = [];
const location = {
	origin: "https://shop.test",
	href: "https://shop.test/a",
	pathname: "/a",
	search: "",
	assign(url: string) {
		assigned = url;
	},
	replace(url: string) {
		replaced = url;
	},
};
const realDocument = globalThis.document;
const storage = new Map<string, string>();

beforeEach(() => {
	scheduled = null;
	scheduledDelay = undefined;
	assigned = null;
	replaced = null;
	readyMarkers = [];
	storage.clear();
	location.href = "https://shop.test/a";
	location.pathname = "/a";
	location.search = "";
	globalThis.setTimeout = ((fn: () => void, delay?: number) => {
		scheduled = fn;
		scheduledDelay = delay;
		return 1 as unknown as ReturnType<typeof setTimeout>;
	}) as typeof setTimeout;
	globalThis.clearTimeout = (() => {
		scheduled = null;
	}) as typeof clearTimeout;
	(globalThis as { window?: unknown }).window = {
		location,
		sessionStorage: {
			getItem: (key: string) => storage.get(key) ?? null,
			setItem: (key: string, value: string) => storage.set(key, value),
			removeItem: (key: string) => storage.delete(key),
		},
	};
	Object.defineProperty(globalThis, "document", {
		configurable: true,
		value: {
			querySelectorAll: () =>
				readyMarkers.map((marker) => ({
					isConnected: true,
					getAttribute: (name: string) =>
						name === "data-hk-product-detail-id" ? marker.productId : null,
					closest: (selector: string) =>
						selector === "[hidden]" && marker.hidden ? {} : null,
					getClientRects: () => (marker.visible === false ? [] : [{}]),
				})),
		},
	});
});

afterEach(() => {
	__resetNavigationObservabilityForTest();
	globalThis.setTimeout = realSetTimeout;
	globalThis.clearTimeout = realClearTimeout;
	delete (globalThis as { window?: unknown }).window;
	if (realDocument === undefined) {
		Reflect.deleteProperty(globalThis, "document");
	} else {
		Object.defineProperty(globalThis, "document", {
			configurable: true,
			value: realDocument,
		});
	}
	__resetNavigationWatchdogForTest();
});

test("forces a hard navigation when the target never commits", () => {
	armNavigationWatchdog("/b");
	assert.ok(scheduled, "should have armed a timer");
	// Timer fires while the address bar is still stuck on /a → hung soft-nav.
	scheduled?.();
	assert.equal(assigned, "https://shop.test/b");
	assert.equal(replaced, null);
});

test("gives primary tab soft navigations more time for a slow RSC stream", () => {
	armNavigationWatchdog("/market/products");

	assert.ok(scheduled, "should have armed a timer");
	assert.ok(
		(scheduledDelay ?? 0) > NAVIGATION_WATCHDOG_MS,
		"primary tab transitions need more than the ordinary watchdog budget",
	);
});

test("keeps ordinary routes on the default watchdog budget", () => {
	armNavigationWatchdog("/account");

	assert.equal(scheduledDelay, NAVIGATION_WATCHDOG_MS);
});

test("does nothing once the route has committed", () => {
	armNavigationWatchdog("/b");
	// Soft-nav succeeded: the URL is now /b before the timer fires.
	location.pathname = "/b";
	location.href = "https://shop.test/b";
	scheduled?.();
	assert.equal(assigned, null);
	assert.equal(replaced, null);
});

test("does not hard-navigate when a redirect lands on a different URL", () => {
	// User clicks /explore but the server redirect()s to /feed. The address
	// bar leaves the start URL (/a) and settles on /feed — committed, not hung.
	// A target-equality check would misread this and reload /explore mid-read.
	armNavigationWatchdog("/explore");
	location.pathname = "/feed";
	location.href = "https://shop.test/feed";
	scheduled?.();
	assert.equal(assigned, null);
	assert.equal(replaced, null);
});

test("ignores a same-page transition", () => {
	armNavigationWatchdog("/a");
	assert.equal(scheduled, null, "no timer for a same-URL navigation");
});

test("ignores a cross-origin target", () => {
	armNavigationWatchdog("https://elsewhere.test/x");
	assert.equal(scheduled, null);
});

test("hard-replaces a committed product URL whose matching UI never mounts", () => {
	armNavigationWatchdog("/products/42");
	location.pathname = "/products/42";
	location.href = "https://shop.test/products/42";

	scheduled?.();

	assert.equal(assigned, null);
	assert.equal(replaced, "https://shop.test/products/42");
});

test("accepts only a visible ready marker for the target product", () => {
	armNavigationWatchdog("/products/42");
	location.pathname = "/products/42";
	location.href = "https://shop.test/products/42";
	readyMarkers = [{ productId: "42" }];

	scheduled?.();

	assert.equal(assigned, null);
	assert.equal(replaced, null);
});

test("does not accept another product's stale ready marker", () => {
	armNavigationWatchdog("/products/42");
	location.pathname = "/products/42";
	location.href = "https://shop.test/products/42";
	readyMarkers = [{ productId: "41" }];

	scheduled?.();

	assert.equal(replaced, "https://shop.test/products/42");
});

test("does not accept a marker still inside hidden Flight staging", () => {
	armNavigationWatchdog("/products/42");
	location.pathname = "/products/42";
	location.href = "https://shop.test/products/42";
	readyMarkers = [{ productId: "42", hidden: true }];

	scheduled?.();

	assert.equal(replaced, "https://shop.test/products/42");
});

test("a later same-page transition invalidates the captured product timer", () => {
	armNavigationWatchdog("/products/42");
	const staleTimer = scheduled;
	location.pathname = "/products/42";
	location.href = "https://shop.test/products/42";

	armNavigationWatchdog("/products/42");
	staleTimer?.();

	assert.equal(assigned, null);
	assert.equal(replaced, null);
});

test("a login document handoff invalidates a captured soft watchdog", () => {
	beginNavigationIntent({ target: "/account", trigger: "link" });
	markRouterTransitionStarted("/account", "push");
	armNavigationWatchdog("/account");
	const staleTimer = scheduled;
	navigateDocument("/seller?welcome=1", {
		reason: "login_completed",
		replace: true,
	});
	staleTimer?.();
	assert.equal(replaced, "https://shop.test/seller?welcome=1");
	assert.equal(assigned, null);
});

test("in-flight recovery preserves a replace transition and its query", () => {
	beginNavigationIntent({
		target: "/account?tab=orders",
		trigger: "router",
		replace: true,
	});
	markRouterTransitionStarted("/account?tab=orders", "replace");
	armNavigationWatchdog("/account?tab=orders");
	scheduled?.();
	assert.equal(replaced, "https://shop.test/account?tab=orders");
	assert.equal(assigned, null);
});
