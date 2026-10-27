// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RoutedLocaleProvider } from "./routed-locale";

const seen = vi.hoisted(() => ({
	props: [] as Array<Record<string, unknown>>,
}));

// Record what NativeLink hands to next/link: the point of the component is to hand over everything
// but the href exactly as the caller wrote it (the viewport prefetch above all).
vi.mock("next/link", async () => {
	const ReactModule = await import("react");
	return {
		default: (props: Record<string, unknown>) => {
			seen.props.push(props);
			return ReactModule.createElement("a", {
				href: String(props.as ?? props.href),
				className: props.className as string | undefined,
			});
		},
	};
});

import NativeLink from "./native-link";

afterEach(() => {
	cleanup();
	seen.props.length = 0;
});

function renderIn(locale: "zh-TW" | "en", props: Record<string, unknown>) {
	return render(
		React.createElement(
			RoutedLocaleProvider,
			{ value: locale },
			React.createElement(NativeLink, props as never),
		),
	);
}

describe("NativeLink", () => {
	it("is next/link with the page's language in the href", () => {
		const english = renderIn("en", { href: "/account/orders?tab=1" });
		expect(english.container.querySelector("a")?.getAttribute("href")).toBe(
			"/en/account/orders?tab=1",
		);
		english.unmount();
		const chinese = renderIn("zh-TW", { href: "/account/orders?tab=1" });
		expect(chinese.container.querySelector("a")?.getAttribute("href")).toBe(
			"/account/orders?tab=1",
		);
	});

	it("hands next/link every other prop untouched, and forces no prefetch setting", () => {
		const onClick = () => undefined;
		renderIn("en", { href: "/sellers/9", className: "x", onClick });
		const [props] = seen.props;
		expect(props.className).toBe("x");
		expect(props.onClick).toBe(onClick);
		expect("prefetch" in props, "next/link's own default prefetch").toBe(false);
	});

	it("passes the caller's prefetch choice through", () => {
		renderIn("en", { href: "/sellers/9", prefetch: false });
		expect(seen.props[0].prefetch).toBe(false);
	});

	it("leaves an external URL and a path that already has a language alone", () => {
		renderIn("en", { href: "https://example.com/a" });
		renderIn("en", { href: "/en/faq" });
		expect(seen.props.map((p) => p.href)).toEqual([
			"https://example.com/a",
			"/en/faq",
		]);
	});
});
