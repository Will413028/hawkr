// @vitest-environment jsdom

import {
	act,
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	pathname: "/",
	push: vi.fn(),
	clickWasPrevented: vi.fn(),
	prefetchMainNavOnIntent: vi.fn(),
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	usePathname: () => mocks.pathname,
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ push: mocks.push, prefetch: vi.fn() }),
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ isLoggedIn: false, activeRole: "buyer" }),
}));
vi.mock("@/lib/hooks/useAutoHideNav", () => ({
	useAutoHideNav: () => ({ hidden: false }),
}));
vi.mock("@/lib/navigation/main-nav-prefetch", () => ({
	cancelMainNavPointerIntent: vi.fn(),
	prefetchMainNavOnIntent: mocks.prefetchMainNavOnIntent,
	scheduleMainNavIdleWarm: () => () => {},
}));
vi.mock("@/components/shared/Link", async () => {
	const ReactModule = await import("react");
	return {
		default: ReactModule.forwardRef<
			HTMLAnchorElement,
			React.AnchorHTMLAttributes<HTMLAnchorElement> & {
				prefetch?: boolean;
				hardNavigation?: boolean;
			}
		>(
			(
				{
					onClick,
					prefetch: _prefetch,
					hardNavigation: _hardNavigation,
					...props
				},
				ref,
			) =>
				ReactModule.createElement("a", {
					...props,
					ref,
					onClick: (event: React.MouseEvent<HTMLAnchorElement>) => {
						onClick?.(event);
						mocks.clickWasPrevented(event.defaultPrevented);
						if (
							!event.defaultPrevented &&
							!event.ctrlKey &&
							!event.metaKey &&
							!event.shiftKey &&
							!event.altKey
						) {
							mocks.push(props.href);
						}
						event.preventDefault();
					},
				}),
		),
	};
});

import MobileBottomNav from "@/components/shared/MobileBottomNav";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

/**
 * The sliding pill is plain CSS now (framer-motion was removed from this
 * globally mounted component), so its target position is read straight off the
 * inline `transform` instead of a mocked `animate.x`.
 */
function indicatorX() {
	const style = document
		.querySelector("[data-mobile-nav-active-indicator]")
		?.getAttribute("style");
	return style?.match(/translate3d\((-?[\d.]+)px/)?.[1] ?? null;
}

function pointer(target: Element, type: string, x: number) {
	const event = new MouseEvent(type, {
		bubbles: true,
		cancelable: true,
		clientX: x,
		button: 0,
	});
	Object.defineProperties(event, {
		pointerId: { value: 1 },
		pointerType: { value: "touch" },
	});
	fireEvent(target, event);
}

function dragTo(x: number) {
	const nav = screen.getByRole("navigation");
	pointer(screen.getByRole("link", { name: "首頁" }), "pointerdown", 30);
	pointer(nav, "pointermove", x);
	pointer(nav, "pointerup", x);
}

function tap(name: string, x: number) {
	const link = screen.getByRole("link", { name });
	pointer(link, "pointerdown", x);
	pointer(link, "pointerup", x);
	fireEvent.click(link, { detail: 1 });
}

async function pendingPhase(value: string | null) {
	await act(async () => {
		if (value === null)
			document.documentElement.removeAttribute("data-nav-pending");
		else document.documentElement.setAttribute("data-nav-pending", value);
		await Promise.resolve();
	});
}

beforeEach(() => {
	mocks.pathname = "/";
	mocks.push.mockReset();
	mocks.clickWasPrevented.mockReset();
	mocks.prefetchMainNavOnIntent.mockReset();
	window.history.replaceState(null, "", "/");
	document.documentElement.removeAttribute("data-nav-pending");
	vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(60);
	vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(56);
	vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockImplementation(
		function (this: HTMLElement) {
			return this.tagName === "A"
				? Array.from(this.parentElement?.querySelectorAll("a") ?? []).indexOf(
						this as HTMLAnchorElement,
					) * 60
				: 0;
		},
	);
});

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
	document.documentElement.removeAttribute("data-nav-pending");
});

describe("mobile bottom navigation gesture ownership", () => {
	it.each([
		50, 90,
	])("does not swallow the first real tap after a drag ending at x=%i without a synthetic click", (x) => {
		render(React.createElement(MobileBottomNav));
		dragTo(x);
		mocks.push.mockClear();
		tap("市集", 150);
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/market/products");
	});

	it("suppresses only the drag-release click and accepts the next gesture", () => {
		render(React.createElement(MobileBottomNav));
		dragTo(90);
		fireEvent.click(screen.getByRole("link", { name: "動態" }), { detail: 1 });
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/feed");
		tap("市集", 150);
		expect(mocks.push.mock.calls).toEqual([["/feed"], ["/market/products"]]);
	});

	it("does not swallow keyboard activation after a drag without a click", () => {
		render(React.createElement(MobileBottomNav));
		dragTo(50);
		fireEvent.click(screen.getByRole("link", { name: "動態" }), { detail: 0 });
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/feed");
	});

	it("clears drag suppression on pointer cancellation", () => {
		render(React.createElement(MobileBottomNav));
		dragTo(50);
		pointer(screen.getByRole("navigation"), "pointercancel", 50);
		fireEvent.click(screen.getByRole("link", { name: "動態" }), { detail: 1 });
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/feed");
	});

	it("consumes a release click targeted at the nav without changing the selected target", () => {
		render(React.createElement(MobileBottomNav));
		dragTo(90);
		fireEvent.click(screen.getByRole("navigation"), { detail: 1 });
		expect(indicatorX()).toBe("60");
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/feed");
		fireEvent.click(screen.getByRole("link", { name: "市集" }), { detail: 0 });
		expect(mocks.push.mock.calls).toEqual([["/feed"], ["/market/products"]]);
	});
});

describe("mobile bottom navigation optimistic selection", () => {
	it.each([
		"ctrlKey",
		"metaKey",
		"shiftKey",
		"altKey",
	] as const)("leaves native %s activation untouched without changing the current pill", (modifier) => {
		render(React.createElement(MobileBottomNav));
		fireEvent.click(screen.getByRole("link", { name: "動態" }), {
			detail: 1,
			[modifier]: true,
		});
		expect(indicatorX()).toBe("0");
		expect(mocks.clickWasPrevented).toHaveBeenCalledExactlyOnceWith(false);
		expect(mocks.push).not.toHaveBeenCalled();
	});

	it("moves the pill immediately while aria-current keeps the actual route", () => {
		render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		expect(indicatorX()).toBe("60");
		expect(
			screen.getByRole("link", { name: "首頁" }).getAttribute("aria-current"),
		).toBe("page");
	});

	it.each([
		"abort",
		null,
	])("returns to the real tab after pending ends with %s and no commit", async (phase) => {
		render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		await pendingPhase("");
		await pendingPhase(phase);
		expect(indicatorX()).toBe("0");
	});

	it("keeps the newest target when an old abort and a new begin arrive in one observer batch", async () => {
		render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		await pendingPhase("");
		await act(async () => {
			document.documentElement.setAttribute("data-nav-pending", "abort");
			tap("市集", 150);
			document.documentElement.setAttribute("data-nav-pending", "");
			await Promise.resolve();
		});
		expect(indicatorX()).toBe("120");
		await pendingPhase("abort");
		expect(indicatorX()).toBe("0");
	});

	it("handles a pending phase that begins and ends in the same observer batch", async () => {
		render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		await act(async () => {
			document.documentElement.setAttribute("data-nav-pending", "");
			document.documentElement.removeAttribute("data-nav-pending");
			await Promise.resolve();
		});
		expect(indicatorX()).toBe("0");
	});

	it("does not erase an immediate selection before its navigation has started", async () => {
		render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		await pendingPhase("done");
		expect(indicatorX()).toBe("60");
	});

	it("keeps the newly committed tab selected when its pending lifecycle settles", async () => {
		const view = render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		await pendingPhase("");
		mocks.pathname = "/feed";
		window.history.replaceState(null, "", "/feed");
		view.rerender(React.createElement(MobileBottomNav));
		await pendingPhase("done");
		expect(indicatorX()).toBe("60");
		expect(
			screen.getByRole("link", { name: "動態" }).getAttribute("aria-current"),
		).toBe("page");
	});

	it.each([
		"popstate",
		"pagehide",
	] as const)("clears an uncommitted selection on %s", async (eventName) => {
		render(React.createElement(MobileBottomNav));
		tap("動態", 90);
		await pendingPhase("");
		fireEvent(window, new Event(eventName));
		expect(indicatorX()).toBe("0");
	});
});
