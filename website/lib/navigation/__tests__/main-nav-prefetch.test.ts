// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { preloadHomeTabBootstrap, preloadMainTabData } = vi.hoisted(() => ({
	preloadHomeTabBootstrap: vi.fn(() => Promise.resolve()),
	preloadMainTabData: vi.fn((_href: string) => Promise.resolve()),
}));

const { whenDocumentSettled } = vi.hoisted(() => ({
	whenDocumentSettled: vi.fn(() => Promise.resolve()),
}));

vi.mock("@/lib/api/tab-bootstrap", () => ({
	preloadHomeTabBootstrap,
	preloadMainTabData,
}));

vi.mock("@/lib/hydration/document-settled", () => ({ whenDocumentSettled }));

import {
	cancelMainNavPointerIntent,
	prefetchMainNavOnIntent,
	resetMainNavPrefetchCoordinatorForTests,
	scheduleMainNavIdleWarm,
} from "@/lib/navigation/main-nav-prefetch";

const prefetch = vi.fn();
const router = { prefetch };
let idleCallback: (() => void) | null = null;
const requestIdleCallback = vi.fn(
	(
		callback: (deadline: {
			didTimeout: boolean;
			timeRemaining: () => number;
		}) => void,
	) => {
		idleCallback = () =>
			callback({ didTimeout: false, timeRemaining: () => 20 });
		return 7;
	},
);
const cancelIdleCallback = vi.fn();

function deferred() {
	let resolve!: () => void;
	const promise = new Promise<void>((done) => {
		resolve = done;
	});
	return { promise, resolve };
}

function setCurrentTab(pathname = "/feed", ready = true) {
	window.history.replaceState(null, "", pathname);
	const marker =
		pathname === "/"
			? 'data-hk-initial-content-ready="home"'
			: pathname === "/feed"
				? 'data-hk-tab-content-ready="feed"'
				: pathname === "/market/products"
					? 'data-hk-tab-content-ready="market"'
					: 'data-hk-page-content-ready="search"';
	document.body.innerHTML = ready
		? `<main><span ${marker} hidden></span></main>`
		: "";
}

function setConnection(
	overrides: {
		onLine?: boolean;
		effectiveType?: string;
		saveData?: boolean;
	} = {},
) {
	vi.stubGlobal("navigator", {
		onLine: overrides.onLine ?? true,
		connection: {
			effectiveType: overrides.effectiveType ?? "4g",
			saveData: overrides.saveData ?? false,
		},
	});
}

async function flushObservers() {
	await Promise.resolve();
	await Promise.resolve();
}

async function startIdle(currentPathname = "/feed") {
	const cleanup = scheduleMainNavIdleWarm({ router, currentPathname });
	await flushObservers();
	expect(requestIdleCallback).toHaveBeenCalled();
	return cleanup;
}

function intent(
	href: string,
	kind: "pointer" | "focus" | "touch",
	currentPathname = "/",
) {
	prefetchMainNavOnIntent({
		router,
		href,
		currentPathname,
		intent: kind,
	});
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date("2026-07-24T08:00:00.000Z"));
	setConnection();
	vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
	vi.stubGlobal("requestIdleCallback", requestIdleCallback);
	vi.stubGlobal("cancelIdleCallback", cancelIdleCallback);
	document.documentElement.removeAttribute("data-nav-pending");
	setCurrentTab();
	idleCallback = null;
	requestIdleCallback.mockClear();
	cancelIdleCallback.mockClear();
	prefetch.mockReset();
	preloadHomeTabBootstrap.mockClear();
	preloadMainTabData.mockClear();
	whenDocumentSettled.mockReset().mockResolvedValue();
	resetMainNavPrefetchCoordinatorForTests();
});

afterEach(() => {
	resetMainNavPrefetchCoordinatorForTests();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

describe("main nav intent prefetch coordinator", () => {
	it("uses a short pointer dwell and cancels fly-by hover work", () => {
		intent("/feed", "pointer");
		vi.advanceTimersByTime(79);
		expect(prefetch).not.toHaveBeenCalled();

		cancelMainNavPointerIntent("/feed");
		vi.advanceTimersByTime(1);
		expect(prefetch).not.toHaveBeenCalled();

		intent("/feed", "pointer");
		vi.advanceTimersByTime(80);
		expect(prefetch).toHaveBeenCalledTimes(1);
		expect(prefetch).toHaveBeenCalledWith("/feed");
	});

	it("single-flights route prefetch while touch starts no same-gesture request", () => {
		intent("/market/products", "pointer");
		vi.advanceTimersByTime(80);
		intent("/market/products", "focus");
		intent("/market/products", "touch");

		expect(prefetch).toHaveBeenCalledTimes(1);
		expect(prefetch).toHaveBeenCalledWith("/market/products");
		expect(preloadMainTabData).not.toHaveBeenCalled();
	});

	it("warms home on focus without adding touch pointer-down work", () => {
		intent("/", "focus", "/feed");
		intent("/", "touch", "/feed");

		expect(prefetch).toHaveBeenCalledTimes(1);
		expect(prefetch).toHaveBeenCalledWith("/", { kind: "full" });
		expect(preloadHomeTabBootstrap).toHaveBeenCalledTimes(1);
		expect(preloadMainTabData).not.toHaveBeenCalled();
	});

	it("never starts a route or data request from touch pointer-down", () => {
		intent("/feed", "touch");

		expect(prefetch).not.toHaveBeenCalled();
		expect(preloadMainTabData).not.toHaveBeenCalled();
	});

	it("spaces route warming across the other public tabs without another data request wave", async () => {
		const cleanup = await startIdle();
		expect(prefetch).not.toHaveBeenCalled();

		idleCallback?.();
		await vi.advanceTimersByTimeAsync(0);
		expect(prefetch.mock.calls).toEqual([["/", { kind: "full" }]]);
		await vi.advanceTimersByTimeAsync(239);
		expect(prefetch).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(prefetch.mock.calls).toEqual([
			["/", { kind: "full" }],
			["/market/products"],
		]);

		await vi.advanceTimersByTimeAsync(240);
		expect(prefetch.mock.calls.map(([href]) => href)).toEqual([
			"/",
			"/market/products",
			"/search",
		]);
		expect(preloadMainTabData).not.toHaveBeenCalled();
		expect(preloadHomeTabBootstrap).not.toHaveBeenCalled();
		cleanup();
	});

	it("gives an explicit intent priority over a pending idle batch", async () => {
		await startIdle();
		idleCallback?.();

		intent("/search", "focus", "/feed");
		await vi.advanceTimersByTimeAsync(1_000);

		expect(prefetch.mock.calls).toEqual([
			["/search"],
			["/", { kind: "full" }],
			["/market/products"],
		]);
		expect(preloadMainTabData).not.toHaveBeenCalled();
	});

	it("continues the idle sequence when intent suppresses its currently scheduled route", async () => {
		await startIdle();
		idleCallback?.();

		// Focus owns the first destination; its pending idle timer must still
		// advance to the remaining routes without issuing a duplicate request.
		intent("/", "focus", "/feed");
		await vi.advanceTimersByTimeAsync(1_000);

		expect(prefetch.mock.calls.map(([href]) => href)).toEqual([
			"/",
			"/market/products",
			"/search",
		]);
		expect(preloadHomeTabBootstrap).toHaveBeenCalledTimes(1);
		expect(preloadMainTabData).not.toHaveBeenCalled();
	});

	it.each([
		"/account",
		"/login",
		"/seller-center",
		"/checkout",
		"/products/example",
		"/feed/post-123",
	] as const)("does not idle-warm from restricted route %s", async (currentPathname) => {
		setCurrentTab(currentPathname);
		scheduleMainNavIdleWarm({ router, currentPathname });
		await flushObservers();

		expect(requestIdleCallback).not.toHaveBeenCalled();
		vi.runAllTimers();
		expect(prefetch).not.toHaveBeenCalled();
	});

	it("does not idle-warm while hidden or on 3G", async () => {
		vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();

		vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
		setConnection({ effectiveType: "3g" });
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		expect(prefetch).not.toHaveBeenCalled();
		expect(preloadHomeTabBootstrap).not.toHaveBeenCalled();
	});

	it("lets the short App Router warm window expire", () => {
		intent("/search", "focus");
		vi.advanceTimersByTime(30_001);
		intent("/search", "focus");

		expect(prefetch).toHaveBeenCalledTimes(2);
	});

	it("never warms auth/private destinations or the already-active tab", () => {
		intent("/login?role=buyer", "focus");
		intent("/account", "touch");
		intent("/feed", "focus", "/feed");
		intent("/feed", "touch", "/feed/post-123");

		expect(prefetch).not.toHaveBeenCalled();
	});

	it.each([
		["offline", { onLine: false, effectiveType: "4g", saveData: false }],
		["Save-Data", { onLine: true, effectiveType: "4g", saveData: true }],
		["slow 2G", { onLine: true, effectiveType: "slow-2g", saveData: false }],
		["2G", { onLine: true, effectiveType: "2g", saveData: false }],
	] as const)("skips every intent while the connection is %s", (_label, state) => {
		vi.stubGlobal("navigator", {
			onLine: state.onLine,
			connection: {
				effectiveType: state.effectiveType,
				saveData: state.saveData,
			},
		});

		intent("/feed", "pointer");
		intent("/market/products", "focus");
		intent("/search", "touch");
		vi.runAllTimers();

		expect(prefetch).not.toHaveBeenCalled();
		expect(preloadHomeTabBootstrap).not.toHaveBeenCalled();
	});

	it("skips speculative hover on 3G but honours explicit focus", () => {
		vi.stubGlobal("navigator", {
			onLine: true,
			connection: { effectiveType: "3g", saveData: false },
		});

		intent("/feed", "pointer");
		vi.runAllTimers();
		expect(prefetch).not.toHaveBeenCalled();

		intent("/feed", "focus");
		expect(prefetch).toHaveBeenCalledTimes(1);
	});

	it("recovers from a synchronous router prefetch failure", () => {
		prefetch.mockImplementationOnce(() => {
			throw new Error("router unavailable");
		});

		expect(() => intent("/feed", "focus")).not.toThrow();
		intent("/feed", "focus");

		expect(prefetch).toHaveBeenCalledTimes(2);
	});
});

describe("main nav idle route readiness and cancellation", () => {
	it("waits for hydration and then the current tab's committed content", async () => {
		const hydration = deferred();
		whenDocumentSettled.mockReturnValueOnce(hydration.promise);
		setCurrentTab("/feed", false);
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();

		hydration.resolve();
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		setCurrentTab();
		await flushObservers();
		expect(requestIdleCallback).toHaveBeenCalledTimes(1);
		expect(prefetch).not.toHaveBeenCalled();
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(0);
		expect(prefetch.mock.calls).toEqual([["/", { kind: "full" }]]);
	});

	it("does not treat a server marker as hydrated content", async () => {
		const hydration = deferred();
		whenDocumentSettled.mockReturnValueOnce(hydration.promise);
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await vi.advanceTimersByTimeAsync(5_000);
		expect(requestIdleCallback).not.toHaveBeenCalled();
		expect(prefetch).not.toHaveBeenCalled();
		hydration.resolve();
		await flushObservers();
		expect(requestIdleCallback).toHaveBeenCalledTimes(1);
	});

	it("rejects a marker inside hidden Flight staging until its tree is revealed", async () => {
		document.body.innerHTML =
			'<div hidden><main data-hk-tab-content-ready="feed"></main></div>';
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		document.querySelector("div")?.removeAttribute("hidden");
		await flushObservers();
		expect(requestIdleCallback).toHaveBeenCalledTimes(1);
	});

	it("waits for the previous route's pending hold to clear after pathname commit", async () => {
		document.documentElement.setAttribute("data-nav-pending", "");
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		document.documentElement.removeAttribute("data-nav-pending");
		await flushObservers();
		expect(requestIdleCallback).toHaveBeenCalledTimes(1);
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(0);
		expect(prefetch.mock.calls).toEqual([["/", { kind: "full" }]]);
	});

	it("fails closed without MutationObserver", async () => {
		vi.stubGlobal("MutationObserver", undefined);
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await vi.advanceTimersByTimeAsync(12_000);
		expect(whenDocumentSettled).not.toHaveBeenCalled();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		expect(prefetch).not.toHaveBeenCalled();
	});

	it.each([
		"missing",
		"terminal",
	] as const)("expires a page with %s readiness without starting speculative work", async (state) => {
		setCurrentTab("/feed", state === "terminal");
		if (state === "terminal") {
			const error = document.createElement("div");
			error.setAttribute("data-hk-route-terminal-ready", "error");
			document.body.append(error);
		}
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		await vi.advanceTimersByTimeAsync(10_001);
		setCurrentTab();
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		expect(prefetch).not.toHaveBeenCalled();
	});

	it("uses the timer fallback when requestIdleCallback is unavailable", async () => {
		vi.stubGlobal("requestIdleCallback", undefined);
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		await vi.advanceTimersByTimeAsync(249);
		expect(prefetch).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(2);
		expect(prefetch.mock.calls).toEqual([["/", { kind: "full" }]]);
	});

	it("cancels the fallback timer when its owner leaves", async () => {
		vi.stubGlobal("requestIdleCallback", undefined);
		const cleanup = scheduleMainNavIdleWarm({
			router,
			currentPathname: "/feed",
		});
		await flushObservers();
		cleanup();
		await vi.advanceTimersByTimeAsync(1_000);
		expect(prefetch).not.toHaveBeenCalled();
	});

	it("cancels queued route calls as soon as navigation starts, before pathname commit", async () => {
		await startIdle();
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(0);
		expect(prefetch).toHaveBeenCalledTimes(1);
		document.documentElement.setAttribute("data-nav-pending", "");
		await flushObservers();
		expect(window.location.pathname).toBe("/feed");
		document.documentElement.removeAttribute("data-nav-pending");
		await vi.advanceTimersByTimeAsync(1_000);
		expect(prefetch).toHaveBeenCalledTimes(1);
	});

	it("stops the entire idle batch on touch without a same-gesture request", async () => {
		await startIdle();
		idleCallback?.();
		intent("/search", "touch", "/feed");
		await vi.advanceTimersByTimeAsync(1_000);
		expect(prefetch).not.toHaveBeenCalled();
		expect(preloadHomeTabBootstrap).not.toHaveBeenCalled();
		expect(preloadMainTabData).not.toHaveBeenCalled();
	});

	it.each([
		"pointerdown",
		"click",
	] as const)("stops remaining speculation when a real link receives %s", async (eventType) => {
		await startIdle();
		idleCallback?.();
		const link = document.createElement("a");
		link.href = "/products/quick-view-target";
		const child = document.createElement("span");
		link.append(child);
		document.body.append(link);
		child.dispatchEvent(new Event(eventType, { bubbles: true }));
		await vi.advanceTimersByTimeAsync(1_000);
		expect(prefetch).not.toHaveBeenCalled();
	});

	it.each([
		"popstate",
		"pagehide",
	] as const)("cancels pending warming on %s", async (eventType) => {
		await startIdle();
		idleCallback?.();
		window.dispatchEvent(new Event(eventType));
		await vi.advanceTimersByTimeAsync(1_000);
		expect(prefetch).not.toHaveBeenCalled();
	});

	it("does not resurrect a canceled batch when deferred hydration resolves", async () => {
		const hydration = deferred();
		whenDocumentSettled.mockReturnValueOnce(hydration.promise);
		const cleanup = scheduleMainNavIdleWarm({
			router,
			currentPathname: "/feed",
		});
		cleanup();
		hydration.resolve();
		await flushObservers();
		await vi.advanceTimersByTimeAsync(1_000);
		expect(requestIdleCallback).not.toHaveBeenCalled();
		expect(prefetch).not.toHaveBeenCalled();
	});

	it("a replaced owner and its stale idle callback cannot cancel the new owner's timer", async () => {
		const oldCleanup = await startIdle();
		const oldIdleCallback = idleCallback;
		await startIdle();
		idleCallback?.();
		oldCleanup();
		oldIdleCallback?.();
		await vi.advanceTimersByTimeAsync(480);
		expect(prefetch.mock.calls).toEqual([
			["/", { kind: "full" }],
			["/market/products"],
			["/search"],
		]);
	});

	it("a canceled owner's late hydration cannot cancel its replacement", async () => {
		const oldHydration = deferred();
		whenDocumentSettled.mockReturnValueOnce(oldHydration.promise);
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await startIdle();
		idleCallback?.();
		oldHydration.resolve();
		await flushObservers();
		await vi.advanceTimersByTimeAsync(480);
		expect(prefetch).toHaveBeenCalledTimes(3);
	});

	it("shares route deduplication across repeated owners and focus until the warm expires", async () => {
		await startIdle();
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(480);
		await startIdle();
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(480);
		intent("/search", "focus", "/feed");
		expect(prefetch).toHaveBeenCalledTimes(3);
		await vi.advanceTimersByTimeAsync(30_001);
		await startIdle();
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(480);
		expect(prefetch).toHaveBeenCalledTimes(6);
	});

	it.each([
		["offline", { onLine: false }],
		["Save-Data", { saveData: true }],
		["slow 2G", { effectiveType: "slow-2g" }],
		["2G", { effectiveType: "2g" }],
		["3G", { effectiveType: "3g" }],
	] as const)("does not start an idle batch while %s", async (_label, network) => {
		setConnection(network);
		scheduleMainNavIdleWarm({ router, currentPathname: "/feed" });
		await flushObservers();
		expect(requestIdleCallback).not.toHaveBeenCalled();
		expect(prefetch).not.toHaveBeenCalled();
	});

	it.each([
		"offline",
		"Save-Data",
		"3G",
		"hidden",
		"new pathname",
	] as const)("stops between routes when conditions change to %s", async (condition) => {
		await startIdle();
		idleCallback?.();
		await vi.advanceTimersByTimeAsync(0);
		if (condition === "hidden") {
			vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
		} else if (condition === "new pathname") {
			window.history.replaceState(null, "", "/account");
		} else {
			setConnection({
				onLine: condition !== "offline",
				saveData: condition === "Save-Data",
				effectiveType: condition === "3G" ? "3g" : "4g",
			});
		}
		await vi.advanceTimersByTimeAsync(1_000);
		expect(prefetch.mock.calls).toEqual([["/", { kind: "full" }]]);
	});
});
