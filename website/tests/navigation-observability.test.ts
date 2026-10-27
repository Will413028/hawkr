import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { armNavigationWatchdog } from "../lib/loading/navigation-watchdog";
import {
	__dropActiveNavigationForTest,
	__getActiveNavigationForTest,
	__resetNavigationObservabilityForTest,
	beginNavigationIntent,
	cancelNavigationIntent,
	declareRouterNavigationOwner,
	isNavigationIntentCurrent,
	markNavigationCommitted,
	markRouterTransitionStarted,
	navigateDocument,
	navigationActionId,
	reportNavigationWatchdogRecovery,
} from "../lib/navigation/observability";

const realWindow = globalThis.window;
const realDocument = globalThis.document;
const realMutationObserver = globalThis.MutationObserver;

let ready = false;
let revealOnObserve = false;
let observerCallback: MutationCallback | null = null;
let observerOptions: MutationObserverInit | null = null;
const storage = new Map<string, string>();
let assigned: string | null = null;
let replaced: string | null = null;
let visibilityState = "visible";
const windowListeners = new Map<string, Set<EventListener>>();
const documentListeners = new Map<string, Set<EventListener>>();
function listenerMethods(listeners: Map<string, Set<EventListener>>) {
	return {
		addEventListener: (name: string, handler: EventListener) => {
			const entries = listeners.get(name) ?? new Set();
			entries.add(handler);
			listeners.set(name, entries);
		},
		removeEventListener: (name: string, handler: EventListener) =>
			listeners.get(name)?.delete(handler),
	};
}
function emitWindow(type: string, props: Record<string, unknown> = {}) {
	for (const handler of [...(windowListeners.get(type) ?? [])])
		handler({ type, ...props } as unknown as Event);
}
const location = {
	origin: "https://shop.test",
	href: "https://shop.test/market",
	pathname: "/market",
	search: "",
	assign: (url: string) => {
		assigned = url;
	},
	replace: (url: string) => {
		replaced = url;
	},
};

class FakeMutationObserver {
	constructor(callback: MutationCallback) {
		observerCallback = callback;
	}
	observe(_target: Node, options?: MutationObserverInit) {
		observerOptions = options ?? null;
		if (revealOnObserve) ready = true;
	}
	disconnect() {}
}

beforeEach(() => {
	ready = false;
	revealOnObserve = false;
	observerCallback = null;
	observerOptions = null;
	storage.clear();
	assigned = null;
	replaced = null;
	visibilityState = "visible";
	windowListeners.clear();
	documentListeners.clear();
	location.href = "https://shop.test/market";
	location.pathname = "/market";
	location.search = "";
	Object.defineProperty(globalThis, "window", {
		configurable: true,
		value: {
			location,
			...listenerMethods(windowListeners),
			sessionStorage: {
				getItem: (key: string) => storage.get(key) ?? null,
				setItem: (key: string, value: string) => storage.set(key, value),
				removeItem: (key: string) => storage.delete(key),
			},
		},
	});
	Object.defineProperty(globalThis, "document", {
		configurable: true,
		value: {
			...listenerMethods(documentListeners),
			get visibilityState() {
				return visibilityState;
			},
			documentElement: {},
			querySelectorAll: () =>
				ready
					? [
							{
								isConnected: true,
								getAttribute: (name: string) =>
									name === "data-hk-product-detail-id" ? "123" : null,
								closest: () => null,
								getClientRects: () => [{}],
							},
						]
					: [],
		},
	});
	Object.defineProperty(globalThis, "MutationObserver", {
		configurable: true,
		value: FakeMutationObserver,
	});
});

afterEach(() => {
	__resetNavigationObservabilityForTest();
	if (realWindow === undefined)
		delete (globalThis as { window?: unknown }).window;
	else {
		Object.defineProperty(globalThis, "window", {
			configurable: true,
			value: realWindow,
		});
	}
	if (realDocument === undefined) {
		delete (globalThis as { document?: unknown }).document;
	} else {
		Object.defineProperty(globalThis, "document", {
			configurable: true,
			value: realDocument,
		});
	}
	if (realMutationObserver === undefined) {
		delete (globalThis as { MutationObserver?: unknown }).MutationObserver;
	} else {
		Object.defineProperty(globalThis, "MutationObserver", {
			configurable: true,
			value: realMutationObserver,
		});
	}
});

test("pairs a declared Link intent with router start and route commit", () => {
	const id = beginNavigationIntent({
		target: "/account",
		trigger: "link",
		componentId: "account_nav",
	});
	assert.ok(id);
	assert.equal(__getActiveNavigationForTest()?.transitionStartedAt, undefined);

	assert.equal(markRouterTransitionStarted("/account", "push"), id);
	location.href = "https://shop.test/account";
	location.pathname = "/account";
	markNavigationCommitted(location.href);

	assert.equal(__getActiveNavigationForTest(), null);
});

test("recovers an intent that never reaches router start without declaring failure at 300ms", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const id = beginNavigationIntent({
		target: "/account?tab=orders",
		trigger: "link",
	});
	t.mock.timers.tick(301);
	assert.equal(__getActiveNavigationForTest()?.id, id);
	assert.equal(assigned, null);
	t.mock.timers.tick(9_699);
	assert.equal(assigned, "https://shop.test/account?tab=orders");
});

test("creates a router transaction when programmatic navigation has no click intent", () => {
	const id = markRouterTransitionStarted("/search?q=apple", "push");
	assert.ok(id);
	assert.equal(__getActiveNavigationForTest()?.trigger, "router");
});

test("normalizes assistant conversations before generic message threads", () => {
	assert.equal(
		navigationActionId("push", "/account/messages/assistant/demo"),
		"navigate_push_to_account_messages_assistant_conversationId",
	);
});

test("normalizes delegated store, order, and product identifiers", () => {
	assert.equal(
		navigationActionId("push", "/adm/2685914682/orders/HK260830001"),
		"navigate_push_to_adm_storeId_orders_orderNo",
	);
	assert.equal(
		navigationActionId(
			"push",
			"/adm/2685914682/products/843856384063471760/edit",
		),
		"navigate_push_to_adm_storeId_products_productId_edit",
	);
});

test("pairs back navigation with the component that declared it", () => {
	declareRouterNavigationOwner("product_modal", navigationActionId("back"));
	markRouterTransitionStarted("/", "traverse");
	assert.equal(__getActiveNavigationForTest()?.componentId, "product_modal");
	assert.equal(__getActiveNavigationForTest()?.actionId, "navigate_back");
});

test("a back transition supersedes an older uncommitted target", () => {
	beginNavigationIntent({
		target: "/products/123",
		trigger: "link",
		mode: "intercepted",
	});
	declareRouterNavigationOwner("product_modal", navigationActionId("back"));
	markRouterTransitionStarted("/", "traverse");
	assert.equal(__getActiveNavigationForTest()?.targetPath, "/");
	assert.equal(__getActiveNavigationForTest()?.componentId, "product_modal");
});

test("does not finish intercepted product navigation until its UI is ready", () => {
	const id = beginNavigationIntent({
		target: "/products/123",
		trigger: "link",
		mode: "intercepted",
	});
	assert.ok(id);
	markRouterTransitionStarted("/products/123", "push");
	location.href = "https://shop.test/products/123";
	location.pathname = "/products/123";
	markNavigationCommitted(location.href);

	assert.equal(__getActiveNavigationForTest()?.id, id);
	ready = true;
	markNavigationCommitted(location.href);
	assert.equal(__getActiveNavigationForTest(), null);
});

test("finishes when React reveals the matching marker from hidden Flight staging", () => {
	const id = beginNavigationIntent({
		target: "/products/123",
		trigger: "link",
		mode: "intercepted",
	});
	assert.ok(id);
	markRouterTransitionStarted("/products/123", "push");
	location.href = "https://shop.test/products/123";
	location.pathname = "/products/123";
	markNavigationCommitted(location.href);

	assert.equal(__getActiveNavigationForTest()?.id, id);
	assert.equal(observerOptions?.attributes, true);
	assert.deepEqual(observerOptions?.attributeFilter, ["hidden"]);

	ready = true;
	assert.ok(observerCallback);
	observerCallback([], {} as MutationObserver);
	assert.equal(__getActiveNavigationForTest(), null);
});

test("closes the readiness race when content appears during observer setup", () => {
	revealOnObserve = true;
	const id = beginNavigationIntent({
		target: "/products/123",
		trigger: "link",
		mode: "intercepted",
	});
	assert.ok(id);
	markRouterTransitionStarted("/products/123", "push");
	location.href = "https://shop.test/products/123";
	location.pathname = "/products/123";

	markNavigationCommitted(location.href);

	assert.equal(__getActiveNavigationForTest(), null);
});

test("a stale marker for another product cannot finish the transaction", () => {
	ready = true;
	const id = beginNavigationIntent({
		target: "/products/124",
		trigger: "link",
		mode: "intercepted",
	});
	assert.ok(id);
	markRouterTransitionStarted("/products/124", "push");
	location.href = "https://shop.test/products/124";
	location.pathname = "/products/124";

	markNavigationCommitted(location.href);

	assert.equal(__getActiveNavigationForTest()?.id, id);
});

test("watchdog recovery closes the transaction as a failure before hard navigation", () => {
	beginNavigationIntent({ target: "/account", trigger: "link" });
	markRouterTransitionStarted("/account", "push");
	reportNavigationWatchdogRecovery("https://shop.test/account");
	assert.equal(__getActiveNavigationForTest(), null);
});

test("hard navigation survives the document boundary and commits on the destination", () => {
	const id = beginNavigationIntent({
		target: "/login",
		trigger: "link",
		mode: "hard",
		componentId: "navbar",
	});
	assert.ok(id);
	assert.ok(storage.size > 0);
	__dropActiveNavigationForTest();

	location.href = "https://shop.test/login";
	location.pathname = "/login";
	markNavigationCommitted(location.href);

	assert.equal(__getActiveNavigationForTest(), null);
	assert.equal(storage.size, 0);
});

test("a late router start retains the original intent and cancels pre-start recovery", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const id = beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(500);
	assert.equal(markRouterTransitionStarted("/account", "push"), id);
	t.mock.timers.tick(20_000);
	assert.equal(assigned, null);
	assert.equal(__getActiveNavigationForTest()?.id, id);
});

test("a commit without a start hook closes the pending intent", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	beginNavigationIntent({ target: "/account", trigger: "link" });
	location.href = "https://shop.test/account";
	location.pathname = "/account";
	markNavigationCommitted(location.href);
	t.mock.timers.tick(20_000);
	assert.equal(__getActiveNavigationForTest(), null);
	assert.equal(assigned, null);
});

test("a primary-tab intent gets the existing 20 second budget", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	beginNavigationIntent({ target: "/search?q=apple", trigger: "link" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, null);
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/search?q=apple");
});

test("pre-start recovery retains query, hash, and replace history semantics", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const id = beginNavigationIntent({
		target: "/account?tab=orders#recent",
		trigger: "router",
		replace: true,
	});
	t.mock.timers.tick(10_000);
	assert.equal(replaced, "https://shop.test/account?tab=orders#recent");
	assert.equal(assigned, null);
	assert.equal(isNavigationIntentCurrent(id), false);
	assert.equal(__getActiveNavigationForTest()?.recoveryOf, id);
});

test("a newer intent cannot be interrupted by the older deadline", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const oldId = beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(5_000);
	const newId = beginNavigationIntent({ target: "/seller", trigger: "router" });
	cancelNavigationIntent(oldId);
	assert.equal(isNavigationIntentCurrent(newId), true);
	t.mock.timers.tick(5_000);
	assert.equal(assigned, null);
	t.mock.timers.tick(5_000);
	assert.equal(assigned, "https://shop.test/seller");
});

for (const event of [
	"pointerdown",
	"click",
	"keydown",
	"input",
	"popstate",
	"hashchange",
	"pagehide",
	"offline",
]) {
	test(`${event} cancels an older pending intent and its rescue`, (t) => {
		t.mock.timers.enable({ apis: ["setTimeout"] });
		const id = beginNavigationIntent({ target: "/account", trigger: "link" });
		emitWindow(event, { isTrusted: true, persisted: true });
		t.mock.timers.tick(20_000);
		assert.equal(isNavigationIntentCurrent(id), false);
		assert.equal(assigned, null);
	});
}

test("a new click is captured before its own navigation intent is created", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	markNavigationCommitted(location.href); // mount lifecycle listeners
	emitWindow("click", { isTrusted: true });
	const id = beginNavigationIntent({ target: "/account", trigger: "link" });
	assert.equal(isNavigationIntentCurrent(id), true);
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/account");
});

test("a swipe's pointerup navigation survives its own release click", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	markNavigationCommitted(location.href);
	emitWindow("pointerdown", { isTrusted: true });
	emitWindow("pointerup", { isTrusted: true });
	const id = beginNavigationIntent({
		target: "/search?q=apple",
		trigger: "router",
	});
	markRouterTransitionStarted("/search?q=apple", "push");
	armNavigationWatchdog("/search?q=apple");
	emitWindow("click", { isTrusted: true, detail: 1 });
	assert.equal(isNavigationIntentCurrent(id), true);
	t.mock.timers.tick(20_000);
	assert.equal(assigned, "https://shop.test/search?q=apple");
});

test("keyboard activation and its generated click/submit are one navigation action", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	markNavigationCommitted(location.href);
	emitWindow("keydown", { isTrusted: true, key: "Enter" });
	const id = beginNavigationIntent({ target: "/account", trigger: "router" });
	emitWindow("click", { isTrusted: true, detail: 0 });
	emitWindow("submit", { isTrusted: true });
	assert.equal(isNavigationIntentCurrent(id), true);
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/account");
});

test("a standalone assistive-technology click still cancels the older navigation", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const id = beginNavigationIntent({ target: "/account", trigger: "router" });
	emitWindow("click", { isTrusted: true, detail: 0 });
	assert.equal(isNavigationIntentCurrent(id), false);
	t.mock.timers.tick(10_000);
	assert.equal(assigned, null);
});

test("Enter keyup does not recreate an already consumed activation-click exemption", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	markNavigationCommitted(location.href);
	emitWindow("keydown", { isTrusted: true, key: "Enter" });
	const id = beginNavigationIntent({ target: "/account", trigger: "router" });
	emitWindow("click", { isTrusted: true, detail: 0 });
	assert.equal(isNavigationIntentCurrent(id), true);
	emitWindow("keyup", { isTrusted: true, key: "Enter" });
	emitWindow("click", { isTrusted: true, detail: 0 });
	assert.equal(isNavigationIntentCurrent(id), false);
	t.mock.timers.tick(10_000);
	assert.equal(assigned, null);
});

test("a hidden page cannot resume old recovery when it becomes visible", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const id = beginNavigationIntent({ target: "/account", trigger: "link" });
	visibilityState = "hidden";
	for (const handler of documentListeners.get("visibilitychange") ?? [])
		handler(new Event("visibilitychange"));
	visibilityState = "visible";
	t.mock.timers.tick(20_000);
	assert.equal(isNavigationIntentCurrent(id), false);
	assert.equal(assigned, null);
});

test("an intent created while hidden is cancelled immediately", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	visibilityState = "hidden";
	const id = beginNavigationIntent({ target: "/account", trigger: "router" });
	visibilityState = "visible";
	t.mock.timers.tick(20_000);
	assert.equal(isNavigationIntentCurrent(id), false);
	assert.equal(assigned, null);
});

test("hash-only navigation and a no-op Back both invalidate old recovery", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	beginNavigationIntent({ target: "/account", trigger: "link" });
	beginNavigationIntent({ target: "/market#offers", trigger: "link" });
	t.mock.timers.tick(20_000);
	assert.equal(assigned, null);
	beginNavigationIntent({ target: "/account", trigger: "link" });
	declareRouterNavigationOwner("back_button", navigationActionId("back"));
	t.mock.timers.tick(20_000);
	assert.equal(assigned, null);
});

test("a hard handoff cancels pre-start rescue and persists pagehide evidence", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const softId = beginNavigationIntent({ target: "/account", trigger: "link" });
	assert.equal(
		navigateDocument("/seller?welcome=1", {
			reason: "login_completed",
			componentId: "login_page",
		}),
		true,
	);
	const hard = __getActiveNavigationForTest();
	assert.ok(hard?.id);
	assert.equal(isNavigationIntentCurrent(softId), false);
	emitWindow("pagehide", { persisted: false });
	const persisted = JSON.parse(
		storage.get("__hk_navigation_transaction") ?? "{}",
	);
	assert.equal(persisted.id, hard.id);
	assert.equal(persisted.reason, "login_completed");
	assert.equal(persisted.pagehidePersisted, false);
	assert.equal(typeof persisted.pagehideAt, "number");
	t.mock.timers.tick(20_000);
	assert.equal(assigned, "https://shop.test/seller?welcome=1");
	__dropActiveNavigationForTest();
	location.href = "https://shop.test/seller?welcome=1";
	location.pathname = "/seller";
	location.search = "?welcome=1";
	markNavigationCommitted(location.href);
	assert.equal(storage.has("__hk_navigation_transaction"), false);
});

test("a BFCache restore cannot revive the previous page's recovery timer", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	beginNavigationIntent({ target: "/account", trigger: "link" });
	emitWindow("pagehide", { persisted: true });
	emitWindow("pageshow", { persisted: true });
	t.mock.timers.tick(20_000);
	assert.equal(assigned, null);
});

test("the automatic recovery allowance survives a document boundary until a new trusted action", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	beginNavigationIntent({ target: "/account", trigger: "router" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/account");
	// Model a new document or server redirect back to the source, preserving storage.
	__resetNavigationObservabilityForTest();
	assigned = null;
	beginNavigationIntent({ target: "/account", trigger: "router" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, null);
	emitWindow("click", { isTrusted: true });
	beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/account");
});

test("blocked storage disables automatic loops but preserves explicit document navigation", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	Object.defineProperty(window, "sessionStorage", {
		configurable: true,
		get: () => {
			throw new Error("blocked");
		},
	});
	beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, null);
	assert.equal(navigateDocument("/account?retry=1"), true);
	assert.equal(assigned, "https://shop.test/account?retry=1");
});

test("blocked storage does not discard a deferred Link whose route starts after the rescue deadline", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	Object.defineProperty(window, "sessionStorage", {
		configurable: true,
		get: () => {
			throw new Error("blocked");
		},
	});
	const id = beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(15_000);
	assert.equal(assigned, null);
	assert.equal(isNavigationIntentCurrent(id), true);
	assert.equal(markRouterTransitionStarted("/account", "push"), id);
	location.href = "https://shop.test/account";
	location.pathname = "/account";
	markNavigationCommitted(location.href);
	assert.equal(__getActiveNavigationForTest(), null);
});

test("a document I/O exception releases the latch so the next navigation can recover", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	const assign = location.assign;
	location.assign = () => {
		throw new Error("navigation rejected");
	};
	assert.throws(() => navigateDocument("/seller"), /navigation rejected/);
	location.assign = assign;
	beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/account");
});

test("a visible user action can navigate again after a cancelled beforeunload prompt", (t) => {
	t.mock.timers.enable({ apis: ["setTimeout"] });
	navigateDocument("/seller");
	assigned = null; // browser kept the current document after beforeunload was cancelled
	emitWindow("click", { isTrusted: true });
	beginNavigationIntent({ target: "/account", trigger: "link" });
	t.mock.timers.tick(10_000);
	assert.equal(assigned, "https://shop.test/account");
});

test("document navigation rejects external, non-http, and credential-bearing targets", () => {
	assert.equal(navigateDocument("https://elsewhere.test/account"), false);
	assert.equal(navigateDocument("javascript:alert(1)"), false);
	assert.equal(
		navigateDocument("https://user:password@shop.test/account"),
		false,
	);
	assert.equal(assigned, null);
});

test("a same-URL document replacement is only completed after crossing the document boundary", () => {
	navigateDocument(location.href, {
		replace: true,
		reason: "watchdog_recovery",
	});
	const id = __getActiveNavigationForTest()?.id;
	markNavigationCommitted(location.href);
	assert.equal(__getActiveNavigationForTest()?.id, id);
	__dropActiveNavigationForTest();
	markNavigationCommitted(location.href);
	assert.equal(__getActiveNavigationForTest(), null);
	assert.equal(storage.has("__hk_navigation_transaction"), false);
});
