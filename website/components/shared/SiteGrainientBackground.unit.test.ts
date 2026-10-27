/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	getSiteBackgroundState,
	setSiteBackgroundAvailable,
	toggleSiteBackgroundPaused,
} from "@/lib/animation/site-background-state";
import SiteBackgroundToggle from "./SiteBackgroundToggle";
import SiteGrainientBackground from "./SiteGrainientBackground";

interface RendererOptions {
	colors: readonly [string, string, string, string];
	dark: boolean;
	onError: () => void;
}

const mocks = vi.hoisted(() => {
	const renderer = {
		pause: vi.fn(),
		resume: vi.fn(),
		setDarkMode: vi.fn(),
		destroy: vi.fn(),
	};
	return {
		renderer,
		settled: vi.fn<() => Promise<void>>(),
		moduleReady: vi.fn<() => Promise<void>>(),
		createRenderer:
			vi.fn<
				(
					canvas: HTMLCanvasElement,
					options: RendererOptions,
				) => typeof renderer | null
			>(),
	};
});

vi.mock("@/lib/hydration/document-settled", () => ({
	whenDocumentSettled: mocks.settled,
}));
vi.mock("@/lib/animation/grainient-renderer", async () => {
	await mocks.moduleReady();
	return { createGrainientRenderer: mocks.createRenderer };
});

function deferred() {
	let resolve!: () => void;
	const promise = new Promise<void>((done) => {
		resolve = done;
	});
	return { promise, resolve };
}

class Preference extends EventTarget {
	readonly media: string;
	matches = false;
	onchange = null;
	addListener = vi.fn();
	removeListener = vi.fn();

	constructor(media: string) {
		super();
		this.media = media;
	}

	set(matches: boolean) {
		this.matches = matches;
		this.dispatchEvent(new Event("change"));
	}
}

class VisibilityObserver {
	static instances: VisibilityObserver[] = [];
	readonly root = null;
	readonly rootMargin = "0px";
	readonly scrollMargin = "0px";
	readonly thresholds = [0];
	readonly targets = new Set<Element>();
	readonly disconnect = vi.fn(() => this.targets.clear());
	readonly observe = vi.fn((target: Element) => this.targets.add(target));
	readonly unobserve = vi.fn((target: Element) => this.targets.delete(target));
	readonly takeRecords = () => [];

	constructor(private readonly callback: IntersectionObserverCallback) {
		VisibilityObserver.instances.push(this);
	}

	emit(visible: boolean) {
		const entries = [...this.targets].map((target) => ({
			boundingClientRect: target.getBoundingClientRect(),
			intersectionRatio: visible ? 1 : 0,
			intersectionRect: target.getBoundingClientRect(),
			isIntersecting: visible,
			rootBounds: null,
			target,
			time: 0,
		}));
		if (entries.length) this.callback(entries, this);
	}
}

const REDUCED = "(prefers-reduced-motion: reduce)";
const FORCED = "(forced-colors: active)";
const preferences = new Map<string, Preference>();
const idleCallbacks = new Map<number, IdleRequestCallback>();
let nextIdleId = 0;
let settled = deferred();
let hidden = false;
const connection = Object.assign(new EventTarget(), { saveData: false });
const originalConnection = Object.getOwnPropertyDescriptor(
	navigator,
	"connection",
);
const originalHidden = Object.getOwnPropertyDescriptor(document, "hidden");
const originalVisibility = Object.getOwnPropertyDescriptor(
	document,
	"visibilityState",
);
const originalTheme = document.documentElement.getAttribute("data-theme");
const requestIdle = vi.fn(
	(callback: IdleRequestCallback, _options?: IdleRequestOptions) => {
		const id = ++nextIdleId;
		idleCallbacks.set(id, callback);
		return id;
	},
);
const cancelIdle = vi.fn((id: number) => idleCallbacks.delete(id));

function preference(query: string) {
	let item = preferences.get(query);
	if (!item) {
		item = new Preference(query);
		preferences.set(query, item);
	}
	return item;
}

async function releaseBarrier() {
	await act(async () => {
		settled.resolve();
		await settled.promise;
	});
}

async function flushIdle() {
	await act(async () => {
		const callbacks = [...idleCallbacks.values()];
		idleCallbacks.clear();
		for (const callback of callbacks) {
			callback({ didTimeout: false, timeRemaining: () => 10 });
		}
	});
}

function setVisible(visible: boolean) {
	act(() => {
		for (const observer of VisibilityObserver.instances) observer.emit(visible);
	});
}

function setHidden(value: boolean) {
	act(() => {
		hidden = value;
		document.dispatchEvent(new Event("visibilitychange"));
	});
}

async function activate() {
	await releaseBarrier();
	setVisible(true);
	await flushIdle();
	await waitFor(() => expect(mocks.createRenderer).toHaveBeenCalledTimes(1));
	setVisible(true);
}

beforeEach(() => {
	vi.clearAllMocks();
	setSiteBackgroundAvailable(false);
	if (getSiteBackgroundState().paused) toggleSiteBackgroundPaused();
	settled = deferred();
	hidden = false;
	connection.saveData = false;
	preferences.clear();
	idleCallbacks.clear();
	VisibilityObserver.instances = [];
	sessionStorage.clear();
	document.documentElement.removeAttribute("data-theme");
	mocks.settled.mockImplementation(() => settled.promise);
	mocks.moduleReady.mockResolvedValue(undefined);
	mocks.createRenderer.mockReturnValue(mocks.renderer);
	vi.stubGlobal("matchMedia", (query: string) => preference(query));
	vi.stubGlobal("IntersectionObserver", VisibilityObserver);
	vi.stubGlobal("requestIdleCallback", requestIdle);
	vi.stubGlobal("cancelIdleCallback", cancelIdle);
	Object.defineProperty(navigator, "connection", {
		configurable: true,
		value: connection,
	});
	Object.defineProperty(document, "hidden", {
		configurable: true,
		get: () => hidden,
	});
	Object.defineProperty(document, "visibilityState", {
		configurable: true,
		get: () => (hidden ? "hidden" : "visible"),
	});
});

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
	for (const [target, key, descriptor] of [
		[navigator, "connection", originalConnection],
		[document, "hidden", originalHidden],
		[document, "visibilityState", originalVisibility],
	] as const) {
		if (descriptor) Object.defineProperty(target, key, descriptor);
		else Reflect.deleteProperty(target, key);
	}
	if (originalTheme === null)
		document.documentElement.removeAttribute("data-theme");
	else document.documentElement.setAttribute("data-theme", originalTheme);
});

describe("site Grainient progressive enhancement", () => {
	it("server-renders a decorative fallback plus a disabled control slot", () => {
		const html = renderToString(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SiteGrainientBackground),
				React.createElement(SiteBackgroundToggle),
			),
		);
		const host = document.createElement("div");
		host.innerHTML = html;
		expect(
			host.querySelector('.site-grainient[aria-hidden="true"]'),
		).not.toBeNull();
		expect(host.querySelector("canvas")).toBeNull();
		const slot = host.querySelector(".site-background-control");
		const button = slot?.querySelector("button");
		expect(slot).not.toBeNull();
		expect(button?.disabled).toBe(true);
		expect(button?.textContent).toBe("靜態背景");
		expect(host.querySelector(".site-grainient button")).toBeNull();
		expect(mocks.settled).not.toHaveBeenCalled();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
	});

	// This is deliberately the first importing case: the real import expression
	// waits for a deferred mocked module, instead of a pretend renderer promise.
	it("does not initialize a late dynamic module after the owner unmounts", async () => {
		const pendingModule = deferred();
		mocks.moduleReady.mockImplementation(() => pendingModule.promise);
		const { unmount } = render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(true);
		await flushIdle();
		await waitFor(() => expect(mocks.moduleReady).toHaveBeenCalledTimes(1));
		unmount();
		await act(async () => {
			pendingModule.resolve();
			await vi.dynamicImportSettled();
		});
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		expect(document.querySelector("canvas")).toBeNull();
	});

	it("keeps the first paint static until document settlement and idle work both run", async () => {
		const { container } = render(React.createElement(SiteGrainientBackground));
		expect(container.querySelector("canvas")).toBeNull();
		expect(requestIdle).not.toHaveBeenCalled();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		await releaseBarrier();
		setVisible(true);
		expect(requestIdle).toHaveBeenCalledWith(expect.any(Function), {
			timeout: 2000,
		});
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		await flushIdle();
		await waitFor(() => expect(mocks.createRenderer).toHaveBeenCalledTimes(1));
		expect(container.querySelectorAll("canvas")).toHaveLength(1);
		expect(mocks.createRenderer.mock.calls[0]?.[0]).toBe(
			container.querySelector("canvas"),
		);
	});

	it("waits for its CSS-owned viewport to become visible", async () => {
		render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(false);
		await flushIdle();
		expect(requestIdle).not.toHaveBeenCalled();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		setVisible(true);
		await flushIdle();
		await waitFor(() => expect(mocks.createRenderer).toHaveBeenCalledTimes(1));
	});

	it("does not initialize while the browser document is hidden", async () => {
		hidden = true;
		render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(true);
		await flushIdle();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		setHidden(false);
		await flushIdle();
		await waitFor(() => expect(mocks.createRenderer).toHaveBeenCalledTimes(1));
	});

	it("passes all four CSS palette tokens and the initial theme to the renderer", async () => {
		document.documentElement.dataset.theme = "dark";
		const { container } = render(React.createElement(SiteGrainientBackground));
		const viewport = container.querySelector<HTMLElement>(
			".site-grainient__viewport",
		);
		expect(viewport).not.toBeNull();
		for (const [index, color] of [
			"#ffffff",
			"#809c7d",
			"#b1ae51",
			"#1a2b3c",
		].entries()) {
			viewport?.style.setProperty(`--site-grainient-color-${index + 1}`, color);
		}
		await activate();
		expect(mocks.createRenderer.mock.calls[0]?.[1].colors).toEqual([
			"#ffffff",
			"#809c7d",
			"#b1ae51",
			"#1a2b3c",
		]);
		expect(mocks.createRenderer.mock.calls[0]?.[1].dark).toBe(true);
	});

	it.each([
		REDUCED,
		FORCED,
	])("does not initialize when %s is already enabled", async (query) => {
		preference(query).matches = true;
		const { container } = render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(true);
		await flushIdle();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		expect(container.querySelector("canvas")).toBeNull();
		expect(getSiteBackgroundState().available).toBe(false);
	});

	it("keeps Save-Data users on the static fallback", async () => {
		connection.saveData = true;
		const { container } = render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(true);
		await flushIdle();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
		expect(container.querySelector("canvas")).toBeNull();
	});

	it("cancels pending initialization when a motion preference changes before idle", async () => {
		render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(true);
		expect(idleCallbacks.size).toBeGreaterThan(0);
		act(() => preference(REDUCED).set(true));
		expect(idleCallbacks.size).toBe(0);
		await flushIdle();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
	});

	it("ignores a late settlement after unmount", async () => {
		const { unmount } = render(React.createElement(SiteGrainientBackground));
		unmount();
		await releaseBarrier();
		expect(requestIdle).not.toHaveBeenCalled();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
	});

	it("cancels queued idle initialization on unmount", async () => {
		const { unmount } = render(React.createElement(SiteGrainientBackground));
		await releaseBarrier();
		setVisible(true);
		expect(idleCallbacks.size).toBeGreaterThan(0);
		unmount();
		expect(cancelIdle).toHaveBeenCalled();
		expect(idleCallbacks.size).toBe(0);
		await flushIdle();
		expect(mocks.createRenderer).not.toHaveBeenCalled();
	});

	it.each([
		REDUCED,
		FORCED,
	])("destroys the enhancement when %s changes at runtime", async (query) => {
		const { container } = render(React.createElement(SiteGrainientBackground));
		await activate();
		act(() => preference(query).set(true));
		expect(mocks.renderer.destroy).toHaveBeenCalledTimes(1);
		expect(container.querySelector("canvas")).toBeNull();
		expect(getSiteBackgroundState().available).toBe(false);
	});

	it("keeps the exact canvas and renderer across repeated visibility changes", async () => {
		const { container } = render(React.createElement(SiteGrainientBackground));
		await activate();
		const canvas = container.querySelector(".site-grainient__canvas");
		expect(canvas).not.toBeNull();
		mocks.renderer.pause.mockClear();
		mocks.renderer.resume.mockClear();

		setVisible(false);
		setVisible(true);
		setVisible(false);
		setVisible(true);

		expect(mocks.renderer.pause).toHaveBeenCalledTimes(2);
		expect(mocks.renderer.resume).toHaveBeenCalledTimes(2);
		expect(mocks.createRenderer).toHaveBeenCalledTimes(1);
		expect(mocks.renderer.destroy).not.toHaveBeenCalled();
		expect(container.querySelector(".site-grainient__canvas")).toBe(canvas);
	});

	it("does not resume while the document is hidden", async () => {
		const { container } = render(React.createElement(SiteGrainientBackground));
		await activate();
		const canvas = container.querySelector(".site-grainient__canvas");
		mocks.renderer.pause.mockClear();
		mocks.renderer.resume.mockClear();
		setVisible(false);
		expect(mocks.renderer.pause).toHaveBeenCalled();
		setHidden(true);
		setVisible(true);
		expect(mocks.renderer.resume).not.toHaveBeenCalled();
		setHidden(false);
		expect(mocks.renderer.resume).toHaveBeenCalledTimes(1);
		expect(container.querySelector(".site-grainient__canvas")).toBe(canvas);
		expect(mocks.createRenderer).toHaveBeenCalledTimes(1);
	});

	it("retains one canvas when route-like sibling content rerenders", async () => {
		function PersistentShell({ route }: { route: string }) {
			return React.createElement(
				React.Fragment,
				null,
				React.createElement(SiteGrainientBackground),
				React.createElement("main", { "data-testid": "route" }, route),
			);
		}
		const view = render(
			React.createElement(PersistentShell, { route: "/feed" }),
		);
		await activate();
		const canvas = view.container.querySelector(".site-grainient__canvas");
		expect(canvas).not.toBeNull();

		for (const route of [
			"/market/products",
			"/faq",
			"/login",
			"/seller/pricing",
			"/account/profile",
			"/seller-center/dashboard",
			"/adm",
			"/search",
		]) {
			view.rerender(React.createElement(PersistentShell, { route }));
		}

		expect(screen.getByTestId("route").textContent).toBe("/search");
		expect(view.container.querySelector(".site-grainient__canvas")).toBe(
			canvas,
		);
		expect(mocks.createRenderer).toHaveBeenCalledTimes(1);
		expect(mocks.renderer.destroy).not.toHaveBeenCalled();
	});

	it("offers the same pause action on standalone page surfaces", async () => {
		const { container } = render(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SiteGrainientBackground),
				React.createElement(SiteBackgroundToggle, { variant: "page" }),
			),
		);
		await activate();
		expect(
			container.querySelector(".site-background-control--page"),
		).not.toBeNull();
		fireEvent.click(screen.getByRole("button", { name: "暫停背景" }));
		expect(screen.getByRole("button", { name: "播放背景" })).toBeDefined();
		expect(mocks.renderer.pause).toHaveBeenCalled();
	});

	it("updates renderer dark mode without recreating its canvas or context", async () => {
		const { container } = render(React.createElement(SiteGrainientBackground));
		await activate();
		const canvas = container.querySelector(".site-grainient__canvas");
		mocks.renderer.setDarkMode.mockClear();

		act(() => {
			document.documentElement.dataset.theme = "dark";
		});
		await waitFor(() =>
			expect(mocks.renderer.setDarkMode).toHaveBeenLastCalledWith(true),
		);
		act(() => {
			document.documentElement.dataset.theme = "light";
		});
		await waitFor(() =>
			expect(mocks.renderer.setDarkMode).toHaveBeenLastCalledWith(false),
		);

		expect(container.querySelector(".site-grainient__canvas")).toBe(canvas);
		expect(mocks.createRenderer).toHaveBeenCalledTimes(1);
		expect(mocks.renderer.destroy).not.toHaveBeenCalled();
	});

	it("keeps one control slot and toggles the shared paused state", async () => {
		const { container } = render(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SiteGrainientBackground),
				React.createElement(SiteBackgroundToggle),
			),
		);
		const slot = container.querySelector(".site-background-control");
		const staticButton = screen.getByRole("button", { name: "靜態背景" });
		expect((staticButton as HTMLButtonElement).disabled).toBe(true);
		await activate();
		const canvas = container.querySelector(".site-grainient__canvas");
		const pauseButton = screen.getByRole("button", { name: "暫停背景" });
		expect(container.querySelector(".site-background-control")).toBe(slot);
		expect(pauseButton).toBe(staticButton);
		expect((pauseButton as HTMLButtonElement).disabled).toBe(false);
		expect(pauseButton.closest('[aria-hidden="true"]')).toBeNull();
		mocks.renderer.pause.mockClear();
		mocks.renderer.resume.mockClear();

		fireEvent.click(pauseButton);
		expect(mocks.renderer.pause).toHaveBeenCalledTimes(1);
		expect(getSiteBackgroundState().paused).toBe(true);
		expect(sessionStorage.getItem("hk:site-background-paused")).toBe("1");
		setVisible(false);
		setVisible(true);
		expect(mocks.renderer.resume).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole("button", { name: "播放背景" }));
		expect(mocks.renderer.resume).toHaveBeenCalledTimes(1);
		expect(getSiteBackgroundState().paused).toBe(false);
		expect(sessionStorage.getItem("hk:site-background-paused")).toBe("0");
		expect(container.querySelector(".site-background-control")).toBe(slot);
		expect(container.querySelector(".site-grainient__canvas")).toBe(canvas);
		expect(mocks.createRenderer).toHaveBeenCalledTimes(1);
	});

	it("releases an active renderer when Save-Data is enabled", async () => {
		const { container } = render(React.createElement(SiteGrainientBackground));
		await activate();
		act(() => {
			connection.saveData = true;
			connection.dispatchEvent(new Event("change"));
		});
		expect(mocks.renderer.destroy).toHaveBeenCalledTimes(1);
		expect(container.querySelector("canvas")).toBeNull();
		expect(getSiteBackgroundState().available).toBe(false);
	});

	it("pauses on pagehide and destroys exactly once when unmounted", async () => {
		const { container, unmount } = render(
			React.createElement(SiteGrainientBackground),
		);
		await activate();
		mocks.renderer.pause.mockClear();
		fireEvent(window, new Event("pagehide"));
		expect(mocks.renderer.pause).toHaveBeenCalled();
		unmount();
		expect(mocks.renderer.destroy).toHaveBeenCalledTimes(1);
		expect(container.querySelector("canvas")).toBeNull();
		expect(
			VisibilityObserver.instances.every(
				(observer) => observer.targets.size === 0,
			),
		).toBe(true);
	});

	it("returns to the static fallback after an unsupported renderer", async () => {
		mocks.createRenderer.mockReturnValue(null);
		const { container } = render(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SiteGrainientBackground),
				React.createElement(SiteBackgroundToggle),
			),
		);
		await activate();
		expect(container.querySelector("canvas")).toBeNull();
		expect(
			(screen.getByRole("button", { name: "靜態背景" }) as HTMLButtonElement)
				.disabled,
		).toBe(true);
	});

	it("contains renderer failure without removing nearby content or controls", async () => {
		const { container } = render(
			React.createElement(
				"section",
				null,
				React.createElement(SiteGrainientBackground),
				React.createElement(SiteBackgroundToggle),
				React.createElement("a", { href: "/market/products" }, "逛市集"),
			),
		);
		await activate();
		act(() => mocks.createRenderer.mock.calls[0]?.[1].onError());
		expect(mocks.renderer.destroy).toHaveBeenCalledTimes(1);
		expect(container.querySelector("canvas")).toBeNull();
		expect(
			(screen.getByRole("button", { name: "靜態背景" }) as HTMLButtonElement)
				.disabled,
		).toBe(true);
		expect(
			screen.getByRole("link", { name: "逛市集" }).getAttribute("href"),
		).toBe("/market/products");
	});
});
