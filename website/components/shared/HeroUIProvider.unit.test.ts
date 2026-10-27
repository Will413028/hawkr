// @vitest-environment jsdom

import { Link as HeroLink } from "@heroui/react";
import { cleanup, fireEvent, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RoutedLocaleProvider } from "@/lib/intl/routed-locale";

const router = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
}));

import { UIProvider } from "./HeroUIProvider";

afterEach(() => {
	cleanup();
	router.push.mockClear();
});

function renderLink(locale: "zh-TW" | "en", href: string) {
	return render(
		React.createElement(
			RoutedLocaleProvider,
			{ value: locale },
			React.createElement(UIProvider, {
				children: React.createElement(HeroLink, { href }, "go"),
			}),
		),
	);
}

describe("UIProvider (react-aria's router bridge)", () => {
	it("a HeroUI link renders the page's language in its href, and navigates with the path as written", () => {
		const view = renderLink("en", "/account/orders?tab=1");
		const anchor = view.container.querySelector("a");
		expect(anchor?.getAttribute("href")).toBe("/en/account/orders?tab=1");
		fireEvent.click(anchor as HTMLAnchorElement);
		expect(router.push).toHaveBeenCalledTimes(1);
		expect(router.push.mock.calls[0][0]).toBe("/account/orders?tab=1");
	});

	it("the default language is unprefixed, an external link is untouched", () => {
		const chinese = renderLink("zh-TW", "/account");
		expect(chinese.container.querySelector("a")?.getAttribute("href")).toBe(
			"/account",
		);
		chinese.unmount();
		const external = renderLink("en", "https://example.com/x");
		expect(external.container.querySelector("a")?.getAttribute("href")).toBe(
			"https://example.com/x",
		);
	});
});
