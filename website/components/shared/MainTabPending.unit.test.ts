// @vitest-environment jsdom

import {
	act,
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import { createElement } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const route = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	usePathname: () => route.pathname,
}));

import MainTabPending, {
	MAIN_TAB_SKELETON_DELAY_MS,
} from "@/components/shared/MainTabPending";
import {
	beginMainTabPending,
	getMainTabPendingSnapshot,
} from "@/lib/navigation/main-tab-pending";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

let content: HTMLElement;
beforeEach(() => {
	vi.useFakeTimers();
	// The pending surface now reuses StoryRail's responsive/collapse geometry.
	vi.stubGlobal(
		"matchMedia",
		vi.fn((media: string) => ({
			media,
			matches: false,
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		})),
	);
	route.pathname = "/";
	window.history.replaceState(null, "", "/");
	document.body.innerHTML =
		'<main class="page-content"><div class="page-transition"><button>來源內容</button></div></main><nav><a href="/feed">動態</a></nav>';
	content = document.querySelector(".page-transition") as HTMLElement;
	content.inert = false;
});

afterEach(() => {
	act(() => window.dispatchEvent(new Event("pagehide")));
	cleanup();
	document.body.innerHTML = "";
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

it("keeps server content untouched before a tab navigation", () => {
	render(createElement(MainTabPending));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(content.inert).toBe(false);
	expect(content.hasAttribute("aria-hidden")).toBe(false);
});

it("publishes navigation intent immediately but never reveals a skeleton for a fast commit", () => {
	const view = render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	expect(getMainTabPendingSnapshot()?.targetPathname).toBe("/feed");
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(content.inert).toBe(false);
	expect(content.hasAttribute("aria-hidden")).toBe(false);
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS - 1));
	route.pathname = "/feed";
	window.history.pushState(null, "", "/feed");
	view.rerender(createElement(MainTabPending));
	act(() => vi.advanceTimersByTime(1000));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(getMainTabPendingSnapshot()).toBeNull();
	expect(content.inert).toBe(false);
});

it("keeps only the latest unrevealed destination and cancels an older reveal deadline", () => {
	render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(70));
	act(() => beginMainTabPending("/search", "push"));
	act(() => vi.advanceTimersByTime(30));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(content.inert).toBe(false);
	act(() => vi.advanceTimersByTime(70));
	expect(screen.getByRole("region", { name: "搜尋內容" })).toBeTruthy();
	expect(screen.queryByRole("region", { name: "動態內容" })).toBeNull();
	expect(content.inert).toBe(true);
});

it.each([
	"popstate",
	"pagehide",
])("cancels an unrevealed skeleton on %s", (eventType) => {
	render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(70));
	act(() => window.dispatchEvent(new Event(eventType)));
	act(() => vi.advanceTimersByTime(1000));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(content.inert).toBe(false);
	expect(vi.getTimerCount()).toBe(0);
});

it("cancels the reveal timer when its surface owner unmounts", () => {
	const view = render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	view.unmount();
	// Only the store's navigation watchdog remains; the local reveal is gone.
	expect(vi.getTimerCount()).toBe(1);
	act(() => vi.advanceTimersByTime(1000));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(content.inert).toBe(false);
});

it("reveals a slow destination after the grace period and preserves the source DOM", () => {
	const button = content.querySelector("button");
	render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS));
	expect(screen.getByRole("region", { name: "動態內容" })).toBeTruthy();
	expect(content.inert).toBe(true);
	expect(content.getAttribute("aria-hidden")).toBe("true");
	expect(content.querySelector("button")).toBe(button);
	expect(content.style.display).toBe("");
	expect(document.querySelector("nav")?.inert).not.toBe(true);
	expect(window.location.pathname).toBe("/");
});

it("keeps slow destination content pending after the progress rail aborts", () => {
	render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS));
	act(() => {
		document.documentElement.setAttribute("data-nav-pending", "abort");
		vi.advanceTimersByTime(5500);
	});
	expect(document.querySelector('[data-main-tab-loading="feed"]')).toBeTruthy();
	expect(content.inert).toBe(true);
	document.documentElement.removeAttribute("data-nav-pending");
});

it("blocks hidden-document keyboard scrolling but keeps navigation and form keys usable", () => {
	render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS));
	const navLink = document.querySelector("nav a") as HTMLElement;
	const press = (target: HTMLElement, key: string) => {
		const event = new KeyboardEvent("keydown", {
			key,
			bubbles: true,
			cancelable: true,
		});
		target.dispatchEvent(event);
		return event.defaultPrevented;
	};
	expect(press(navLink, "PageDown")).toBe(true);
	expect(press(navLink, " ")).toBe(true);
	expect(press(navLink, "Enter")).toBe(false);
	expect(press(navLink, "Tab")).toBe(false);
	const input = document.createElement("input");
	document.querySelector("nav")?.append(input);
	expect(press(input, "Home")).toBe(false);
	act(() => beginMainTabPending("/", "push"));
	expect(press(navLink, "PageDown")).toBe(false);
});

it("shows only the latest selected tab, then restores the original on cancellation", () => {
	render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS));
	act(() => beginMainTabPending("/search", "push"));
	expect(screen.queryByRole("region", { name: "動態內容" })).toBeNull();
	expect(screen.getByRole("region", { name: "搜尋內容" })).toBeTruthy();
	act(() => beginMainTabPending("/", "push"));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(content.inert).toBe(false);
	expect(content.hasAttribute("aria-hidden")).toBe(false);
});

it("removes the skeleton and inertness in the destination commit", () => {
	const view = render(createElement(MainTabPending));
	act(() => beginMainTabPending("/feed", "push"));
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS));
	route.pathname = "/feed";
	window.history.pushState(null, "", "/feed");
	view.rerender(createElement(MainTabPending));
	expect(document.querySelector("[data-main-tab-loading]")).toBeNull();
	expect(document.documentElement.hasAttribute("data-main-tab-pending")).toBe(
		false,
	);
	expect(content.inert).toBe(false);
});

it("restores existing accessibility attributes when the controller unmounts", () => {
	content.setAttribute("aria-hidden", "false");
	const view = render(createElement(MainTabPending));
	act(() => beginMainTabPending("/market/products", "push"));
	act(() => vi.advanceTimersByTime(MAIN_TAB_SKELETON_DELAY_MS));
	view.unmount();
	expect(content.inert).toBe(false);
	expect(content.getAttribute("aria-hidden")).toBe("false");
	expect(document.documentElement.hasAttribute("data-main-tab-pending")).toBe(
		false,
	);
});
