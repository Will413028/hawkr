// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import type NextLink from "next/link";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	activeRole: "buyer" as "buyer" | "seller",
	switchRole: vi.fn(),
	push: vi.fn(),
	replace: vi.fn(),
	beginNavigationIntent: vi.fn(),
	cancelNavigationIntent: vi.fn(),
	isNavigationIntentCurrent: vi.fn(),
	navigateDocument: vi.fn(),
	endPending: vi.fn(),
	currentIntent: null as string | null,
	nextIntent: 0,
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useRouter: () => ({
		push: mocks.push,
		replace: mocks.replace,
		prefetch: vi.fn(),
	}),
}));
vi.mock("@/lib/auth/context", () => ({
	useOptionalAuth: () => ({
		activeRole: mocks.activeRole,
		isLoggedIn: true,
		hasSeller: true,
		identity: { accountId: "navigation-test" },
		switchRole: mocks.switchRole,
	}),
}));
vi.mock("@/lib/navigation/observability", () => ({
	beginNavigationIntent: mocks.beginNavigationIntent,
	cancelNavigationIntent: mocks.cancelNavigationIntent,
	isNavigationIntentCurrent: mocks.isNavigationIntentCurrent,
	navigateDocument: mocks.navigateDocument,
}));
vi.mock("@/lib/loading/nav-pending", () => ({
	beginNavPending: () => mocks.endPending,
}));
vi.mock("@/lib/navigation/programmatic-route-motion", () => ({
	armProgrammaticRouteMotion: vi.fn(),
}));
vi.mock("@/lib/seller/repo", () => ({
	fetchMyStore: vi.fn(),
}));

// Keep both real Hawkr Link layers. The framework boundary implements
// NextLink's ordered onClick -> cancelable onNavigate -> router dispatch;
// document navigation is suppressed so no test can leave the jsdom document.
vi.mock("next/link", async () => {
	const ReactModule = await import("react");
	return {
		default: (props: React.ComponentProps<typeof NextLink>) => {
			const {
				href,
				as,
				onClick,
				onNavigate,
				prefetch: _prefetch,
				replace,
				scroll: _scroll,
				...anchorProps
			} = props;
			const renderedHref = String(as ?? href);
			return ReactModule.createElement("a", {
				...anchorProps,
				href: renderedHref,
				onClick: (event: React.MouseEvent<HTMLAnchorElement>) => {
					onClick?.(event);
					if (event.defaultPrevented) return;
					let canceled = false;
					onNavigate?.({
						preventDefault() {
							canceled = true;
						},
					});
					if (!canceled) {
						(replace ? mocks.replace : mocks.push)(renderedHref);
					}
					event.preventDefault();
				},
			});
		},
	};
});

import { RoutedLocaleProvider } from "@/lib/intl/routed-locale";
import Link from "./Link";
import { ViewTransitions } from "./view-transitions";

const queryClients: QueryClient[] = [];

function renderLink(
	props: React.ComponentProps<typeof Link>,
	queryClient?: QueryClient,
) {
	const tree = React.createElement(
		ViewTransitions,
		null,
		React.createElement(Link, props, "destination"),
	);
	return render(
		queryClient
			? React.createElement(QueryClientProvider, { client: queryClient }, tree)
			: tree,
	);
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.clearAllMocks();
	mocks.activeRole = "buyer";
	mocks.currentIntent = null;
	mocks.nextIntent = 0;
	mocks.switchRole.mockReturnValue(true);
	mocks.beginNavigationIntent.mockImplementation(() => {
		mocks.currentIntent = `intent-${++mocks.nextIntent}`;
		return mocks.currentIntent;
	});
	mocks.cancelNavigationIntent.mockImplementation((id?: string | null) => {
		if (!id || id === mocks.currentIntent) mocks.currentIntent = null;
	});
	mocks.isNavigationIntentCurrent.mockImplementation(
		(id: string | null) => id !== null && id === mocks.currentIntent,
	);
	mocks.navigateDocument.mockImplementation(() => {
		mocks.currentIntent = null;
		return true;
	});
	window.history.replaceState(null, "", "/source");
	document.documentElement.dataset.vt = "none";
});

afterEach(() => {
	cleanup();
	for (const client of queryClients.splice(0)) client.clear();
	vi.clearAllTimers();
	vi.useRealTimers();
	delete document.documentElement.dataset.vt;
});

describe("shared Link navigation recovery integration", () => {
	it("records the rendered as target, full query/hash, and replace semantics", () => {
		renderLink({
			href: "/account",
			as: "/account?tab=orders#recent",
			replace: true,
		});

		fireEvent.click(screen.getByRole("link"));

		expect(mocks.beginNavigationIntent).toHaveBeenCalledExactlyOnceWith(
			expect.objectContaining({
				target: `${window.location.origin}/account?tab=orders#recent`,
				replace: true,
			}),
		);
		expect(mocks.replace).toHaveBeenCalledExactlyOnceWith(
			"/account?tab=orders#recent",
		);
	});

	it.each([
		"/account",
		"/products/demo",
	])("cancels the declared intent when onNavigate prevents %s, including intercepted routing", (href) => {
		const onNavigate = vi.fn((event: { preventDefault(): void }) =>
			event.preventDefault(),
		);
		renderLink({ href, onNavigate });

		fireEvent.click(screen.getByRole("link"));

		expect(onNavigate).toHaveBeenCalledTimes(1);
		expect(mocks.beginNavigationIntent).toHaveBeenCalledTimes(1);
		expect(mocks.cancelNavigationIntent).toHaveBeenCalledWith("intent-1");
		expect(mocks.currentIntent).toBeNull();
		expect(mocks.push).not.toHaveBeenCalled();
	});

	it("calls onNavigate once for a permitted route and leaves dispatch enabled", () => {
		const onNavigate = vi.fn();
		renderLink({ href: "/account", onNavigate });

		fireEvent.click(screen.getByRole("link"));

		expect(onNavigate).toHaveBeenCalledTimes(1);
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/account");
	});

	it("does not create an intent when the caller cancels its click", () => {
		renderLink({
			href: "/account",
			onClick: (event) => event.preventDefault(),
		});

		fireEvent.click(screen.getByRole("link"));

		expect(mocks.beginNavigationIntent).not.toHaveBeenCalled();
		expect(mocks.push).not.toHaveBeenCalled();
	});

	it("hands a role switch to document navigation with the rendered destination", () => {
		renderLink({
			href: "/seller-center/products",
			as: "/seller-center/products?status=draft#latest",
			replace: true,
		});

		fireEvent.click(screen.getByRole("link"));

		expect(mocks.switchRole).toHaveBeenCalledExactlyOnceWith("seller");
		expect(mocks.navigateDocument).toHaveBeenCalledExactlyOnceWith(
			`${window.location.origin}/seller-center/products?status=draft#latest`,
			expect.objectContaining({ replace: true }),
		);
		expect(mocks.push).not.toHaveBeenCalled();
		expect(mocks.replace).not.toHaveBeenCalled();
	});

	async function startDeferredSellerLink() {
		mocks.activeRole = "seller";
		let resolveData: (value: object) => void = () => {};
		const pendingData = new Promise<object>((resolve) => {
			resolveData = resolve;
		});
		const queryClient = new QueryClient({
			defaultOptions: { queries: { retry: false, gcTime: 0 } },
		});
		queryClients.push(queryClient);
		const ensure = vi
			.spyOn(queryClient, "ensureQueryData")
			.mockImplementation(() => pendingData as Promise<never>);
		renderLink({ href: "/seller-center/products", replace: true }, queryClient);
		await act(async () => {
			fireEvent.click(screen.getByRole("link"));
			await vi.dynamicImportSettled();
		});
		expect(ensure).toHaveBeenCalledTimes(1);
		expect(screen.getByRole("link").getAttribute("aria-busy")).toBe("true");
		expect(mocks.replace).not.toHaveBeenCalled();
		return async () => {
			await act(async () => resolveData({ id: "seller-demo" }));
		};
	}

	it("still dispatches the active deferred destination when its data arrives", async () => {
		const settle = await startDeferredSellerLink();

		await settle();

		expect(mocks.replace).toHaveBeenCalledExactlyOnceWith(
			`${window.location.origin}/seller-center/products`,
			{ scroll: true },
		);
	});

	it("keeps one deferred data wave navigable after the same pending link is tapped again", async () => {
		const settle = await startDeferredSellerLink();
		// The shared recovery capture listener cancels the old intent before a
		// trusted repeat click reaches React. Model that boundary explicitly;
		// jsdom-generated events cannot be isTrusted.
		mocks.cancelNavigationIntent();
		fireEvent.click(screen.getByRole("link"));
		expect(mocks.replace).not.toHaveBeenCalled();

		await settle();

		expect(mocks.replace).toHaveBeenCalledExactlyOnceWith(
			`${window.location.origin}/seller-center/products`,
			{ scroll: true },
		);
	});

	it.each([
		"login document handoff",
		"new programmatic intent",
	])("does not let delayed link data override %s while the source URL is unchanged", async (replacement) => {
		const settle = await startDeferredSellerLink();
		if (replacement === "login document handoff") {
			mocks.navigateDocument("/products/after-login", {
				componentId: "login",
				actionId: "login_completed",
			});
		} else {
			mocks.beginNavigationIntent({ target: "/account", trigger: "router" });
		}

		await settle();

		expect(window.location.pathname).toBe("/source");
		expect(mocks.replace).not.toHaveBeenCalled();
		expect(mocks.push).not.toHaveBeenCalled();
		expect(mocks.endPending).toHaveBeenCalled();
		expect(screen.getByRole("link").getAttribute("aria-busy")).not.toBe("true");
		expect(screen.getByRole("link").hasAttribute("data-pending")).toBe(false);
	});
});

describe("shared Link in an English page", () => {
	function renderEnglishLink(props: React.ComponentProps<typeof Link>) {
		return render(
			React.createElement(
				RoutedLocaleProvider,
				{ value: "en" },
				React.createElement(
					ViewTransitions,
					null,
					React.createElement(Link, props, "destination"),
				),
			),
		);
	}

	it("renders the English URL and navigates to it, with the query and hash kept", () => {
		renderEnglishLink({ href: "/account/orders?tab=1#recent" });

		const anchor = screen.getByRole("link");
		expect(anchor.getAttribute("href")).toBe("/en/account/orders?tab=1#recent");
		fireEvent.click(anchor);

		expect(mocks.beginNavigationIntent).toHaveBeenCalledExactlyOnceWith(
			expect.objectContaining({
				target: `${window.location.origin}/en/account/orders?tab=1#recent`,
			}),
		);
		expect(mocks.push).toHaveBeenCalledExactlyOnceWith(
			"/en/account/orders?tab=1#recent",
		);
	});

	it("localizes the as target too, and the home page is /en", () => {
		renderEnglishLink({ href: "/", as: "/?ref=nav" });
		expect(screen.getByRole("link").getAttribute("href")).toBe("/en?ref=nav");
	});

	it("does not prefix a path that already carries a language, an external URL or an API path", () => {
		for (const [href, rendered] of [
			["/en/faq", "/en/faq"],
			["https://example.com/x", "https://example.com/x"],
			["/api/v1/orders/export.csv", "/api/v1/orders/export.csv"],
			["mailto:hello@example.com", "mailto:hello@example.com"],
		] as const) {
			const view = renderEnglishLink({ href });
			expect(screen.getByRole("link").getAttribute("href"), href).toBe(
				rendered,
			);
			view.unmount();
		}
	});

	it("a hard navigation (the sign-in boundary) keeps the language too", () => {
		renderEnglishLink({
			href: "/login?role=seller",
			hardNavigation: true,
		});
		const anchor = screen.getByRole("link");
		expect(anchor.getAttribute("href")).toBe("/en/login?role=seller");
		expect(anchor.hasAttribute("data-hk-hard-navigation")).toBe(true);
	});

	it("without a provider (isolated renders) it is the default-language site", () => {
		renderLink({ href: "/account" });
		expect(screen.getByRole("link").getAttribute("href")).toBe("/account");
	});
});
