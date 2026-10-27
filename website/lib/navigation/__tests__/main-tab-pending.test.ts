// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	beginMainTabPending,
	completeMainTabPending,
	getMainTabPendingServerSnapshot,
	getMainTabPendingSnapshot,
	subscribeMainTabPending,
} from "@/lib/navigation/main-tab-pending";

beforeEach(() => {
	vi.useFakeTimers();
	window.dispatchEvent(new Event("pagehide"));
	window.history.replaceState(null, "", "/");
});

afterEach(() => {
	vi.unstubAllGlobals();
	window.dispatchEvent(new Event("pagehide"));
	vi.useRealTimers();
	vi.restoreAllMocks();
});

describe("main-tab pending content store", () => {
	it.each([
		["/feed", "home", "/"],
		["/", "feed", "/feed"],
		["/", "market-list", "/market/products"],
		["/", "search", "/search"],
	])("publishes the %s to %s target synchronously", (source, variant, target) => {
		window.history.replaceState(null, "", source);
		beginMainTabPending(target, "push");
		expect(getMainTabPendingSnapshot()).toEqual({
			id: expect.any(Number),
			sourcePathname: source,
			targetPathname: target,
			variant,
		});
	});

	it("accepts same-origin URLs and target queries without treating queries as new tabs", () => {
		beginMainTabPending(`${window.location.origin}/search?q=fish`, "replace");
		expect(getMainTabPendingSnapshot()?.targetPathname).toBe("/search");

		window.history.replaceState(null, "", "/search?q=fish");
		beginMainTabPending("/search?q=rice", "push");
		expect(getMainTabPendingSnapshot()).toBeNull();
	});

	it("uses the route policy's legacy market source without warming legacy targets", () => {
		window.history.replaceState(null, "", "/market");
		beginMainTabPending("/feed");
		expect(getMainTabPendingSnapshot()?.targetPathname).toBe("/feed");
		beginMainTabPending("/market/products");
		expect(getMainTabPendingSnapshot()).toBeNull();
	});

	it.each([
		"/account",
		"/seller-center",
		"/checkout",
		"/products/product-1",
		"/sellers/seller-1",
		"/feed/post-1",
		"/market",
		"https://other.example/feed",
		"javascript:void(0)",
		"http://",
	])("clears superseded work for an ineligible target: %s", (target) => {
		beginMainTabPending("/feed");
		beginMainTabPending(target);
		expect(getMainTabPendingSnapshot()).toBeNull();
		expect(vi.getTimerCount()).toBe(0);
	});

	it.each([
		"/account",
		"/products/product-1",
		"/feed/post-1",
		"/about",
	])("does not create a main-tab surface from non-tab source %s", (source) => {
		beginMainTabPending("/feed");
		window.history.replaceState(null, "", source);
		beginMainTabPending("/search");
		expect(getMainTabPendingSnapshot()).toBeNull();
	});

	it("clears a pending destination when the source tab is selected again", () => {
		beginMainTabPending("/feed");
		beginMainTabPending("/");
		expect(getMainTabPendingSnapshot()).toBeNull();
		expect(vi.getTimerCount()).toBe(0);
	});

	it("leaves traversals to the router and clears older pending work", () => {
		beginMainTabPending("/feed");
		beginMainTabPending("/search", "traverse");
		expect(getMainTabPendingSnapshot()).toBeNull();
		expect(vi.getTimerCount()).toBe(0);
	});

	it("keeps the latest target through old source effects and the older timer deadline", () => {
		beginMainTabPending("/feed");
		const first = getMainTabPendingSnapshot();
		vi.advanceTimersByTime(10_000);
		beginMainTabPending("/market/products");
		const latest = getMainTabPendingSnapshot();
		expect(latest?.id).toBeGreaterThan(first?.id ?? 0);
		completeMainTabPending("/");
		vi.advanceTimersByTime(12_000);
		expect(getMainTabPendingSnapshot()).toBe(latest);
		expect(latest?.targetPathname).toBe("/market/products");
		expect(vi.getTimerCount()).toBe(1);
	});

	it("uses a new identity for a repeated navigation but stable identity between changes", () => {
		beginMainTabPending("/feed");
		const first = getMainTabPendingSnapshot();
		expect(getMainTabPendingSnapshot()).toBe(first);
		beginMainTabPending("/feed");
		expect(getMainTabPendingSnapshot()).not.toBe(first);
		expect(getMainTabPendingSnapshot()?.id).toBeGreaterThan(first?.id ?? 0);
	});

	it("does not interpret the current source layout effect as completion", () => {
		beginMainTabPending("/feed");
		const pending = getMainTabPendingSnapshot();
		completeMainTabPending("/");
		expect(getMainTabPendingSnapshot()).toBe(pending);
	});

	it.each([
		"/feed",
		"/login",
		"/search",
	])("clears after the target or redirect actually commits to %s", (committedPathname) => {
		beginMainTabPending("/feed");
		window.history.replaceState(null, "", committedPathname);
		completeMainTabPending(committedPathname);
		expect(getMainTabPendingSnapshot()).toBeNull();
		expect(vi.getTimerCount()).toBe(0);
	});

	it.each([
		"popstate",
		"pagehide",
	])("clears immediately on %s and still accepts later navigations", (eventType) => {
		beginMainTabPending("/feed");
		window.dispatchEvent(new Event(eventType));
		expect(getMainTabPendingSnapshot()).toBeNull();
		expect(vi.getTimerCount()).toBe(0);
		beginMainTabPending("/search");
		expect(getMainTabPendingSnapshot()?.targetPathname).toBe("/search");
	});

	it("cleans up after the watchdog budget without claiming a route committed", () => {
		beginMainTabPending("/feed");
		vi.advanceTimersByTime(21_999);
		expect(getMainTabPendingSnapshot()?.targetPathname).toBe("/feed");
		vi.advanceTimersByTime(1);
		expect(getMainTabPendingSnapshot()).toBeNull();
		expect(window.location.pathname).toBe("/");
	});

	it("does not depend on the progress rail's shorter abort signal", () => {
		beginMainTabPending("/feed");
		document.documentElement.setAttribute("data-nav-pending", "abort");
		vi.advanceTimersByTime(4_100);
		expect(getMainTabPendingSnapshot()?.targetPathname).toBe("/feed");
		document.documentElement.removeAttribute("data-nav-pending");
	});

	it("notifies subscribers once per state change and releases subscriptions", () => {
		const listener = vi.fn();
		const unsubscribe = subscribeMainTabPending(listener);
		try {
			beginMainTabPending("/feed");
			beginMainTabPending("/search");
			completeMainTabPending("/");
			expect(listener).toHaveBeenCalledTimes(2);
			window.dispatchEvent(new Event("pagehide"));
			window.dispatchEvent(new Event("pagehide"));
			expect(listener).toHaveBeenCalledTimes(3);
			unsubscribe();
			beginMainTabPending("/market/products");
			expect(listener).toHaveBeenCalledTimes(3);
		} finally {
			unsubscribe();
		}
	});

	it("preserves history, scroll, content and attributes while publishing pending state", () => {
		document.body.innerHTML =
			'<main><button type="button">Source</button></main>';
		const source = document.querySelector("main");
		const originalHref = window.location.href;
		const originalAttributes = document.documentElement.outerHTML;
		const originalScrollY = window.scrollY;
		const push = vi.spyOn(window.history, "pushState");
		const replace = vi.spyOn(window.history, "replaceState");
		const scroll = vi.spyOn(window, "scrollTo");
		beginMainTabPending("/feed");
		expect(getMainTabPendingSnapshot()).not.toBeNull();
		expect(push).not.toHaveBeenCalled();
		expect(replace).not.toHaveBeenCalled();
		expect(scroll).not.toHaveBeenCalled();
		expect(window.location.href).toBe(originalHref);
		expect(window.scrollY).toBe(originalScrollY);
		expect(document.querySelector("main")).toBe(source);
		expect(document.documentElement.outerHTML).toBe(originalAttributes);
	});

	it("has a stable empty server snapshot even during a client navigation", () => {
		beginMainTabPending("/feed");
		expect(getMainTabPendingSnapshot()).not.toBeNull();
		expect(getMainTabPendingServerSnapshot()).toBeNull();
		expect(getMainTabPendingServerSnapshot()).toBeNull();
	});

	it("is safe to call without a browser", () => {
		vi.stubGlobal("window", undefined);
		expect(() => beginMainTabPending("/feed")).not.toThrow();
		expect(() => completeMainTabPending("/feed")).not.toThrow();
		expect(getMainTabPendingServerSnapshot()).toBeNull();
		expect(getMainTabPendingSnapshot()).toBeNull();
	});
});
