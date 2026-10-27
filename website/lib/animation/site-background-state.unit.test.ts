/** @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const PAUSE_KEY = "hk:site-background-paused";
const LEGACY_PAUSE_KEY = "hk:home-grainient-paused";

async function loadFreshState() {
	vi.resetModules();
	return import("./site-background-state");
}

beforeEach(() => {
	vi.restoreAllMocks();
	sessionStorage.clear();
});

afterEach(() => {
	vi.restoreAllMocks();
	sessionStorage.clear();
});

describe("site background external store", () => {
	it("starts with one stable server-safe snapshot", async () => {
		const store = await loadFreshState();
		expect(store.getServerSiteBackgroundState()).toEqual({
			available: false,
			paused: false,
		});
		expect(store.getSiteBackgroundState()).toBe(
			store.getServerSiteBackgroundState(),
		);
	});

	it("notifies subscribers only for state changes and stops after unsubscribe", async () => {
		const store = await loadFreshState();
		const listener = vi.fn();
		const unsubscribe = store.subscribeSiteBackground(listener);

		store.setSiteBackgroundAvailable(false);
		expect(listener).not.toHaveBeenCalled();
		store.setSiteBackgroundAvailable(true);
		expect(listener).toHaveBeenCalledTimes(1);
		store.setSiteBackgroundAvailable(true);
		expect(listener).toHaveBeenCalledTimes(1);

		store.toggleSiteBackgroundPaused();
		expect(listener).toHaveBeenCalledTimes(2);
		expect(store.getSiteBackgroundState()).toEqual({
			available: true,
			paused: true,
		});
		expect(sessionStorage.getItem(PAUSE_KEY)).toBe("1");

		unsubscribe();
		store.toggleSiteBackgroundPaused();
		expect(listener).toHaveBeenCalledTimes(2);
		expect(sessionStorage.getItem(PAUSE_KEY)).toBe("0");
	});

	it("hydrates the current pause key once without dropping availability", async () => {
		sessionStorage.setItem(PAUSE_KEY, "1");
		const store = await loadFreshState();
		const listener = vi.fn();
		store.subscribeSiteBackground(listener);
		store.setSiteBackgroundAvailable(true);

		store.hydrateSiteBackgroundState();
		expect(store.getSiteBackgroundState()).toEqual({
			available: true,
			paused: true,
		});
		expect(listener).toHaveBeenCalledTimes(2);

		sessionStorage.setItem(PAUSE_KEY, "0");
		store.hydrateSiteBackgroundState();
		expect(store.getSiteBackgroundState().paused).toBe(true);
		expect(listener).toHaveBeenCalledTimes(2);
	});

	it("migrates the legacy home preference only when the current key is absent", async () => {
		sessionStorage.setItem(LEGACY_PAUSE_KEY, "1");
		let store = await loadFreshState();
		store.hydrateSiteBackgroundState();
		expect(store.getSiteBackgroundState().paused).toBe(true);

		sessionStorage.setItem(PAUSE_KEY, "0");
		store = await loadFreshState();
		store.hydrateSiteBackgroundState();
		expect(store.getSiteBackgroundState().paused).toBe(false);
	});

	it("keeps toggling when optional storage access is blocked", async () => {
		vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
			throw new Error("fixture storage read blocked");
		});
		const store = await loadFreshState();
		expect(() => store.hydrateSiteBackgroundState()).not.toThrow();
		expect(store.getSiteBackgroundState().paused).toBe(false);

		vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
			throw new Error("fixture storage write blocked");
		});
		expect(() => store.toggleSiteBackgroundPaused()).not.toThrow();
		expect(store.getSiteBackgroundState().paused).toBe(true);
	});
});
