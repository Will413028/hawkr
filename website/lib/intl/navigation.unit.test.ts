// @vitest-environment jsdom

import { renderHook } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { createNavigation } from "next-intl/navigation";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import { routing } from "@/i18n/routing";
import type { AppLocale } from "./locales";

const mocks = vi.hoisted(() => ({ pathname: "/" }));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	usePathname: () => mocks.pathname,
}));

import { localizeHref, usePathname } from "./navigation";

function pathnameAt(locale: AppLocale, browserPathname: string): string {
	mocks.pathname = browserPathname;
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(NextIntlClientProvider, {
			locale,
			messages: {},
			timeZone: "Asia/Taipei",
			children,
		});
	return renderHook(() => usePathname(), { wrapper }).result.current;
}

describe("usePathname (next-intl's, locale-free)", () => {
	it("reads the same path in the browser and in the prerender: the internal /zh-TW of a rewritten default-locale page is dropped", () => {
		for (const [browser, prerender, expected] of [
			["/", "/zh-TW", "/"],
			["/about", "/zh-TW/about", "/about"],
			[
				"/account/orders/HK-1",
				"/zh-TW/account/orders/HK-1",
				"/account/orders/HK-1",
			],
			["/products/12", "/zh-TW/products/12", "/products/12"],
		] as const) {
			expect(pathnameAt("zh-TW", browser), browser).toBe(expected);
			expect(pathnameAt("zh-TW", prerender), prerender).toBe(expected);
		}
	});

	it("drops the English prefix, so every language of a page has one set of pathname rules", () => {
		expect(pathnameAt("en", "/en")).toBe("/");
		expect(pathnameAt("en", "/en/about")).toBe("/about");
		expect(pathnameAt("en", "/en/sellers/9/posts")).toBe("/sellers/9/posts");
	});

	it("only strips the prefix of the page's own language, and only at a segment boundary", () => {
		expect(pathnameAt("zh-TW", "/en/about")).toBe("/en/about");
		expect(pathnameAt("en", "/zh-TW/about")).toBe("/zh-TW/about");
		expect(pathnameAt("zh-TW", "/zh-TWAN/x")).toBe("/zh-TWAN/x");
		expect(pathnameAt("en", "/enterprise")).toBe("/enterprise");
	});
});

describe("localizeHref", () => {
	it("zh-TW is the unprefixed site; English gets /en, and the home page is the bare prefix", () => {
		expect(localizeHref("/account/orders", "zh-TW")).toBe("/account/orders");
		expect(localizeHref("/", "zh-TW")).toBe("/");
		expect(localizeHref("/account/orders", "en")).toBe("/en/account/orders");
		expect(localizeHref("/", "en")).toBe("/en");
	});

	it("a hash goes straight after the prefixed path, with no slash in between", () => {
		expect(localizeHref("/#top", "en")).toBe("/en#top");
		expect(localizeHref("/en#top", "en")).toBe("/en#top");
		expect(localizeHref("/en#top", "zh-TW")).toBe("/en#top");
		expect(localizeHref("/zh-TW#top", "en")).toBe("/#top");
		expect(localizeHref("/?a=1#top", "en")).toBe("/en?a=1#top");
		expect(localizeHref("/faq#shipping", "en")).toBe("/en/faq#shipping");
		expect(localizeHref("/#top", "zh-TW")).toBe("/#top");
	});

	it("the default language leaves every href exactly as it was written", () => {
		// zh-TW hrefs pass through untouched: no normalizing.
		for (const href of [
			"/",
			"/faq/",
			"/a//b",
			"/a/b/../c",
			"/search?",
			"/search?q",
			"/a%20b",
			"/訂單",
			"/products/1/",
			"/market//products",
			"/a b",
			"/a?b#c",
			"/a#",
			"/?",
			"/#",
			"/products/1.5",
			"/login?role=seller&redirect=%2Fseller-center%2Forders",
			"/search?q=%E7%95%AA%E8%8C%84",
		]) {
			expect(localizeHref(href, "zh-TW"), href).toBe(href);
		}
	});

	it("keeps the query and the hash", () => {
		expect(localizeHref("/search?q=a%20b&sort=1", "en")).toBe(
			"/en/search?q=a%20b&sort=1",
		);
		expect(localizeHref("/faq#shipping", "en")).toBe("/en/faq#shipping");
		expect(localizeHref("/market/products?tab=1#top", "en")).toBe(
			"/en/market/products?tab=1#top",
		);
		expect(localizeHref("/?ref=x", "en")).toBe("/en?ref=x");
	});

	it("can be applied twice: an already-prefixed path keeps its own language", () => {
		expect(localizeHref("/en/account", "en")).toBe("/en/account");
		expect(localizeHref("/en", "en")).toBe("/en");
		expect(localizeHref("/en/account?tab=1", "zh-TW")).toBe(
			"/en/account?tab=1",
		);
		expect(localizeHref(localizeHref("/cart", "en"), "en")).toBe("/en/cart");
	});

	it("an explicit /zh-TW prefix resolves to the canonical unprefixed URL", () => {
		expect(localizeHref("/zh-TW/account", "en")).toBe("/account");
		expect(localizeHref("/zh-TW", "en")).toBe("/");
	});

	it("a page whose last segment has a dot is still a page", () => {
		expect(localizeHref("/products/1.5", "en")).toBe("/en/products/1.5");
		expect(localizeHref("/sellers/2600123456/v1.2", "en")).toBe(
			"/en/sellers/2600123456/v1.2",
		);
		expect(localizeHref("/search?q=a.b", "en")).toBe("/en/search?q=a.b");
	});

	it("an absolute URL is never localized, same-origin or not: whoever wrote it chose the address", () => {
		for (const href of [
			"https://hawkr.example/account",
			"http://localhost:3000/faq",
			"https://example.com/en/account",
		]) {
			expect(localizeHref(href, "en"), href).toBe(href);
		}
	});

	it("leaves everything that is not an app page alone", () => {
		for (const href of [
			"https://example.com/account",
			"//cdn.example.com/x",
			"mailto:hello@example.com",
			"tel:+886000000000",
			"#section",
			"?page=2",
			"",
			"account",
			"/api/v1/orders",
			"/api/connect/x.Y/Z",
			"/adm/12/products",
			"/.well-known/apple-developer-merchantid-domain-association",
			"/sitemap.xml",
			"/robots.txt",
			"/manifest.webmanifest",
			"/icon.svg",
			"/placeholders/avatar.svg",
			"/brand/wordmark.svg",
			"/templates/hawkr_import_template.csv",
			"/_next/static/chunks/main.js",
		]) {
			expect(localizeHref(href, "en"), href).toBe(href);
		}
	});

	it("matches next-intl's own path building for the pages it does localize", () => {
		const { getPathname } = createNavigation(routing);
		for (const path of ["/", "/about", "/products/1", "/sellers/9/reviews"]) {
			expect(localizeHref(path, "en")).toBe(
				getPathname({ href: path, locale: "en" }),
			);
			expect(localizeHref(path, "zh-TW")).toBe(
				getPathname({ href: path, locale: "zh-TW" }),
			);
		}
	});
});
