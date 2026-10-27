import type { AppRouterInstance } from "next/dist/shared/lib/app-router-context.shared-runtime";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

interface CapturedNextLinkProps {
	prefetch?: unknown;
	"data-hk-hard-navigation"?: string;
	onClick?: (event: React.MouseEvent<HTMLAnchorElement>) => void;
	onFocus?: (event: React.FocusEvent<HTMLAnchorElement>) => void;
	onMouseEnter?: (event: React.MouseEvent<HTMLAnchorElement>) => void;
	onPointerDown?: (event: React.PointerEvent<HTMLAnchorElement>) => void;
}

const nextLinkCapture = vi.hoisted(() => ({
	props: null as CapturedNextLinkProps | null,
}));

vi.mock("next/link", () => ({
	default: (props: CapturedNextLinkProps) => {
		nextLinkCapture.props = props;
		return null;
	},
}));

import { Link, ViewTransitions } from "@/components/shared/view-transitions";

const prefetch = vi.fn();
const noop = vi.fn();
const router: AppRouterInstance = {
	bfcacheId: "test",
	back: noop,
	forward: noop,
	refresh: noop,
	push: noop,
	replace: noop,
	prefetch,
};

function renderLink(
	props: Omit<React.ComponentProps<typeof Link>, "children">,
): CapturedNextLinkProps {
	renderToStaticMarkup(
		React.createElement(
			AppRouterContext.Provider,
			{ value: router },
			React.createElement(
				ViewTransitions,
				null,
				React.createElement(Link, props, "destination"),
			),
		),
	);
	if (!nextLinkCapture.props)
		throw new Error("NextLink props were not captured");
	return nextLinkCapture.props;
}

function intentEvent(
	href: string,
	options: {
		download?: boolean;
		target?: string;
		defaultPrevented?: boolean;
	} = {},
) {
	let defaultPrevented = options.defaultPrevented ?? false;
	return {
		get defaultPrevented() {
			return defaultPrevented;
		},
		preventDefault() {
			defaultPrevented = true;
		},
		currentTarget: {
			href,
			target: options.target ?? "",
			hasAttribute(name: string) {
				return name === "download" && options.download === true;
			},
		},
	} as unknown as React.SyntheticEvent<HTMLAnchorElement>;
}

function clickEvent(
	href: string,
	options: { download?: boolean } = {},
): React.MouseEvent<HTMLAnchorElement> & { prevented: () => boolean } {
	let defaultPrevented = false;
	return {
		button: 0,
		metaKey: false,
		ctrlKey: false,
		shiftKey: false,
		altKey: false,
		nativeEvent: { which: 1 },
		get defaultPrevented() {
			return defaultPrevented;
		},
		preventDefault() {
			defaultPrevented = true;
		},
		prevented() {
			return defaultPrevented;
		},
		currentTarget: {
			nodeName: "A",
			href,
			target: "",
			hasAttribute(name: string) {
				return name === "download" && options.download === true;
			},
			getAttribute(name: string) {
				return name === "target" ? "" : null;
			},
		},
	} as unknown as React.MouseEvent<HTMLAnchorElement> & {
		prevented: () => boolean;
	};
}

beforeEach(() => {
	nextLinkCapture.props = null;
	vi.clearAllMocks();
	vi.stubGlobal("window", {
		location: {
			href: "https://example.com/about",
			origin: "https://example.com",
			pathname: "/about",
			search: "",
		},
	});
	vi.stubGlobal("document", {
		documentElement: { dataset: {} },
		startViewTransition: vi.fn(),
	});
});

describe("shared Link intent prefetch", () => {
	it("always disables NextLink viewport prefetch", () => {
		expect(renderLink({ href: "/faq" }).prefetch).toBe(false);
		expect(renderLink({ href: "/faq", prefetch: true }).prefetch).toBe(false);
		expect(renderLink({ href: "/login", hardNavigation: true }).prefetch).toBe(
			false,
		);
	});

	for (const eventName of [
		"onMouseEnter",
		"onFocus",
		"onPointerDown",
	] as const) {
		it(`${eventName} prefetches a same-origin route once per target`, () => {
			const props = renderLink({ href: "/faq" });
			const event = intentEvent("https://example.com/faq");
			props[eventName]?.(event as never);
			props[eventName]?.(event as never);

			expect(prefetch).toHaveBeenCalledTimes(1);
			expect(prefetch).toHaveBeenCalledWith("/faq");
		});
	}

	for (const eventName of [
		"onMouseEnter",
		"onFocus",
		"onPointerDown",
	] as const) {
		it(`${eventName} does not race an intercepted product quick view`, () => {
			const props = renderLink({ href: "/products/product-123" });
			const event = intentEvent("https://example.com/products/product-123");
			props[eventName]?.(event as never);

			expect(prefetch).not.toHaveBeenCalled();
		});
	}

	it("deduplicates each destination even if a reusable link changes targets", () => {
		const props = renderLink({ href: "/faq" });
		for (const href of ["/faq", "/contact", "/faq", "/contact"]) {
			props.onMouseEnter?.(
				intentEvent(
					new URL(href, "https://example.com").href,
				) as React.MouseEvent<HTMLAnchorElement>,
			);
		}

		expect(prefetch.mock.calls).toEqual([["/faq"], ["/contact"]]);
	});

	it("honours a user handler that prevents the intent", () => {
		const props = renderLink({
			href: "/faq",
			onMouseEnter: (event) => event.preventDefault(),
		});
		props.onMouseEnter?.(
			intentEvent(
				"https://example.com/faq",
			) as React.MouseEvent<HTMLAnchorElement>,
		);

		expect(prefetch).not.toHaveBeenCalled();
	});

	it.each([
		["hard navigation", { href: "/login", hardNavigation: true }, "/login", {}],
		["prefetch opt-out", { href: "/faq", prefetch: false }, "/faq", {}],
		[
			"external URL",
			{ href: "https://example.org/file" },
			"https://example.org/file",
			{},
		],
		[
			"non-HTTP URL",
			{ href: "mailto:help@example.com" },
			"mailto:help@example.com",
			{},
		],
		[
			"download",
			{ href: "/files/report.pdf" },
			"/files/report.pdf",
			{ download: true },
		],
		["new tab", { href: "/faq" }, "/faq", { target: "_blank" }],
		["hash only", { href: "/about#team" }, "/about#team", {}],
	] as const)("does not prefetch %s", (_label, linkProps, href, eventOptions) => {
		const props = renderLink(linkProps);
		props.onPointerDown?.(
			intentEvent(
				new URL(href, "https://example.com").href,
				eventOptions,
			) as React.PointerEvent<HTMLAnchorElement>,
		);

		expect(prefetch).not.toHaveBeenCalled();
	});

	it("marks hard navigation on the rendered anchor for capture listeners", () => {
		expect(
			renderLink({ href: "/login", hardNavigation: true })[
				"data-hk-hard-navigation"
			],
		).toBe("");
	});

	it.each([
		["external navigation", "https://example.org/file", {}],
		[
			"native file download",
			"https://example.com/files/report.pdf",
			{ download: true },
		],
	] as const)("leaves %s to the browser", (_label, href, options) => {
		const props = renderLink({ href });
		const event = clickEvent(href, options);
		props.onClick?.(event);

		expect(event.prevented()).toBe(false);
		expect(router.push).not.toHaveBeenCalled();
		expect(router.replace).not.toHaveBeenCalled();
	});

	describe("network-aware intent prefetch", () => {
		it("does not prefetch when offline", () => {
			vi.stubGlobal("navigator", { onLine: false });
			const props = renderLink({ href: "/faq" });
			props.onMouseEnter?.(
				intentEvent(
					"https://example.com/faq",
				) as React.MouseEvent<HTMLAnchorElement>,
			);
			expect(prefetch).not.toHaveBeenCalled();
		});

		it("does not prefetch when Save-Data is enabled", () => {
			vi.stubGlobal("navigator", {
				onLine: true,
				connection: { saveData: true, effectiveType: "4g" },
			});
			const props = renderLink({ href: "/faq" });
			props.onMouseEnter?.(
				intentEvent(
					"https://example.com/faq",
				) as React.MouseEvent<HTMLAnchorElement>,
			);
			expect(prefetch).not.toHaveBeenCalled();
		});

		it("does not prefetch on 2G connections", () => {
			vi.stubGlobal("navigator", {
				onLine: true,
				connection: { saveData: false, effectiveType: "2g" },
			});
			const props = renderLink({ href: "/faq" });
			props.onPointerDown?.(
				intentEvent(
					"https://example.com/faq",
				) as React.PointerEvent<HTMLAnchorElement>,
			);
			expect(prefetch).not.toHaveBeenCalled();
		});

		it("suppresses hover (pointer) on 3G but allows explicit focus and pointerDown", () => {
			vi.stubGlobal("navigator", {
				onLine: true,
				connection: { saveData: false, effectiveType: "3g" },
			});
			const props = renderLink({ href: "/faq" });

			// onMouseEnter (pointer) is suppressed on 3G
			props.onMouseEnter?.(
				intentEvent(
					"https://example.com/faq",
				) as React.MouseEvent<HTMLAnchorElement>,
			);
			expect(prefetch).not.toHaveBeenCalled();

			// onFocus is allowed on 3G
			props.onFocus?.(
				intentEvent(
					"https://example.com/faq",
				) as React.FocusEvent<HTMLAnchorElement>,
			);
			expect(prefetch).toHaveBeenCalledTimes(1);
			expect(prefetch).toHaveBeenCalledWith("/faq");
		});
	});
});
