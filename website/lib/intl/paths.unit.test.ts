import { afterEach, describe, expect, it } from "vitest";
import { __setFlavorForTest } from "@/lib/config/flavor";
import {
	GENERATED_ROOT_FILES,
	isLocaleRoutableFor,
	isLocalizablePagePath,
	isMetadataImagePath,
	isNonLocalizedPath,
	localePrefix,
	NON_LOCALIZED_EXACT,
	NON_LOCALIZED_PREFIXES,
	PUBLIC_DIRECTORIES,
	routingPathname,
	STRIP_LOCALE_SCRIPT,
	splitLocale,
	stripLocale,
	surfaceOfPath,
	withLocale,
} from "./paths";

const BOTH = ["zh-TW", "en"] as const;
const ONLY_DEFAULT = ["zh-TW"] as const;

describe("splitLocale", () => {
	it("splits a prefixed path into locale, prefix and the locale-free path", () => {
		expect(splitLocale("/en/account", BOTH)).toEqual({
			locale: "en",
			prefix: "/en",
			pathname: "/account",
		});
		expect(splitLocale("/en", BOTH)).toEqual({
			locale: "en",
			prefix: "/en",
			pathname: "/",
		});
		expect(splitLocale("/en/sellers/9/posts", BOTH).pathname).toBe(
			"/sellers/9/posts",
		);
	});

	it("an unprefixed path is the default locale and is returned as is", () => {
		expect(splitLocale("/account", BOTH)).toEqual({
			locale: null,
			prefix: "",
			pathname: "/account",
		});
		expect(splitLocale("/", BOTH).pathname).toBe("/");
	});

	it("an explicit default-locale prefix is split too (next-intl redirects it to the unprefixed URL)", () => {
		expect(splitLocale("/zh-TW/account", BOTH)).toEqual({
			locale: "zh-TW",
			prefix: "/zh-TW",
			pathname: "/account",
		});
		expect(splitLocale("/zh-TW", BOTH).pathname).toBe("/");
	});

	it("matches whole segments only", () => {
		expect(splitLocale("/english", BOTH).locale).toBeNull();
		expect(splitLocale("/entry/x", BOTH).locale).toBeNull();
		expect(splitLocale("/en-US/x", BOTH).locale).toBeNull();
		expect(splitLocale("/zh-TWx", BOTH).locale).toBeNull();
	});

	it("only knows the locales the deployment routes: prod's /en is just a path", () => {
		expect(splitLocale("/en/account", ONLY_DEFAULT)).toEqual({
			locale: null,
			prefix: "",
			pathname: "/en/account",
		});
		expect(stripLocale("/zh-TW/account")).toBe("/account");
	});
});

describe("withLocale / localePrefix", () => {
	it("zh-TW has no prefix, en has /en, and the home page is the bare prefix", () => {
		expect(localePrefix("zh-TW")).toBe("");
		expect(localePrefix("en")).toBe("/en");
		expect(withLocale("/about", "zh-TW")).toBe("/about");
		expect(withLocale("/about", "en")).toBe("/en/about");
		expect(withLocale("/", "zh-TW")).toBe("/");
		expect(withLocale("/", "en")).toBe("/en");
	});

	it("rejects a path without its leading slash", () => {
		expect(() => withLocale("about", "en")).toThrow(/must start with/);
	});

	it("is the inverse of splitLocale", () => {
		for (const path of [
			"/",
			"/about",
			"/sellers/9/posts",
			"/account/orders/12",
		]) {
			for (const locale of BOTH) {
				const split = splitLocale(withLocale(path, locale), BOTH);
				expect(split.pathname, `${locale} ${path}`).toBe(path);
			}
		}
	});
});

describe("non-localized paths", () => {
	it("staff, infrastructure and root metadata routes are never localized", () => {
		for (const path of [
			"/api",
			"/api/health",
			"/api/connect/hawkr.identity.v1.IdentityAppService/GetSession",
			"/adm",
			"/adm/enter",
			"/adm/2685914682/products",
			"/sentinel",
			"/store-map/return",
			"/.well-known/apple-developer-merchantid-domain-association",
			"/apple-icon",
			"/opengraph-image",
			"/apple-app-site-association",
		]) {
			expect(isNonLocalizedPath(path), path).toBe(true);
		}
	});

	it("matches on a segment boundary: look-alikes are localized pages", () => {
		for (const path of [
			"/",
			"/about",
			"/administrator",
			"/api-docs",
			"/sentinels",
			"/store-map",
			"/apple-iconic",
			"/opengraph-images",
			"/sellers/1/apple-icon",
		]) {
			expect(isNonLocalizedPath(path), path).toBe(false);
		}
	});

	it("a single path is only itself: its subpaths are junk under [locale], not the thing that is served", () => {
		for (const path of [
			"/sentinel/account/orders",
			"/store-map/return/x",
			"/apple-icon/products/1",
			"/opengraph-image/x",
			"/apple-app-site-association/x",
		]) {
			expect(isNonLocalizedPath(path), path).toBe(false);
		}
	});

	it("every listed path is rooted with no trailing slash", () => {
		for (const path of [...NON_LOCALIZED_PREFIXES, ...NON_LOCALIZED_EXACT]) {
			expect(path.startsWith("/")).toBe(true);
			expect(path.endsWith("/")).toBe(false);
		}
	});
});

describe("surfaceOfPath", () => {
	it("the seller center, onboarding and the seller landing pages are the seller surface", () => {
		for (const path of [
			"/seller",
			"/seller/pricing",
			"/seller/measures/m1",
			"/seller/onboarding",
			"/seller-center",
			"/seller-center/orders/12",
			"/seller-product-import-preview/job-1",
		]) {
			expect(surfaceOfPath(path), path).toBe("seller");
		}
	});

	it("everything else, including the public storefronts and the seller guide and terms, is buyer", () => {
		for (const path of [
			"/",
			"/market/products",
			"/sellers/9",
			"/sellers/9/products/5",
			"/seller-guide",
			"/seller-terms",
			"/account/orders",
			"/faq",
		]) {
			expect(surfaceOfPath(path), path).toBe("buyer");
		}
	});
});

describe("isLocaleRoutableFor", () => {
	it("asks the surface of the locale-free path", () => {
		const buyerOnly = (surface: "buyer" | "seller") =>
			surface === "buyer" ? (["zh-TW", "en"] as const) : (["zh-TW"] as const);
		expect(isLocaleRoutableFor("en", "/faq", buyerOnly)).toBe(true);
		expect(isLocaleRoutableFor("en", "/seller-center/orders", buyerOnly)).toBe(
			false,
		);
		expect(
			isLocaleRoutableFor("zh-TW", "/seller-center/orders", buyerOnly),
		).toBe(true);
	});
});

describe("isMetadataImagePath", () => {
	it("recognises the storefront's generated apple-touch-icon, with the group hash Next appends", () => {
		for (const path of [
			"/sellers/9/apple-icon-v673w8",
			"/sellers/2600123456/apple-icon-0a1b2c",
		]) {
			expect(isMetadataImagePath(path), path).toBe(true);
		}
	});

	it("is explicit, not a naming rule: a page whose segment is called icon is not an image route", () => {
		for (const path of [
			"/",
			"/about",
			"/account/orders/icon",
			"/account/orders/apple-icon-v673w8",
			"/products/icon",
			"/icons/leaf",
			"/apple-icon",
			"/opengraph-image",
			"/sellers/9/apple-icon",
			"/sellers/9/apple-icon-big",
			"/sellers/9/apple-icon-v673w8/x",
			"/sellers/9/products/apple-icon-v673w8",
			"/sellers/abc/apple-icon-v673w8",
			"/sellers/12345678901234567890/apple-icon-v673w8",
		]) {
			expect(isMetadataImagePath(path), path).toBe(false);
		}
	});
});

describe("routingPathname", () => {
	it("is the identity for an ordinary path", () => {
		for (const path of [
			"/",
			"/about",
			"/en/products/1",
			"/sellers/2600123456",
		]) {
			expect(routingPathname(path), path).toBe(path);
		}
	});

	it("decodes and collapses the way next-intl does, so an alternate spelling cannot dodge a site rule", () => {
		expect(routingPathname("/%65n/products/1")).toBe("/en/products/1");
		expect(routingPathname("//en/products/1")).toBe("/en/products/1");
		expect(routingPathname("/%61ccount/orders")).toBe("/account/orders");
		expect(routingPathname("/%61pi/health")).toBe("/api/health");
		expect(routingPathname("/en////products")).toBe("/en/products");
		expect(routingPathname("/en/\t/example.org")).toBe("/en/example.org");
	});

	it("leaves reserved characters encoded and neutralises a backslash", () => {
		expect(routingPathname("/a%2Fb")).toBe("/a%2Fb");
		expect(routingPathname("/en/\\example.org")).toBe("/en/%5Cexample.org");
	});

	it("decodes a non-ASCII segment", () => {
		expect(routingPathname("/ja/%E7%B4%84")).toBe("/ja/約");
	});

	it("is null for a malformed escape, which Next answers with a 400", () => {
		expect(routingPathname("/%E0%A4%A")).toBeNull();
	});
});

describe("STRIP_LOCALE_SCRIPT", () => {
	afterEach(() => __setFlavorForTest(null));

	it("is the inline-script twin of stripLocale: the same answer for every path", () => {
		// the inline scripts strip every supported locale's prefix; stripLocale strips the routable ones
		__setFlavorForTest("dev");
		const strip = new Function(`return (${STRIP_LOCALE_SCRIPT})`)() as (
			pathname: string,
		) => string;
		for (const pathname of [
			"/",
			"/en",
			"/en/",
			"/en/about",
			"/en/products/12",
			"/zh-TW",
			"/zh-TW/about",
			"/about",
			"/products/12",
			"/enterprise",
			"/english/x",
			"/zh-TWAN",
			"/en/en/x",
			"/a/en/b",
		]) {
			expect(strip(pathname), pathname).toBe(stripLocale(pathname));
		}
	});
});

describe("isLocalizablePagePath", () => {
	it("pages get a prefix, including those whose last segment has a dot", () => {
		for (const path of [
			"/",
			"/faq",
			"/products/12",
			"/products/1.5",
			"/sellers/9/v1.2",
			"/search",
			"/seller-guide",
		]) {
			expect(isLocalizablePagePath(path), path).toBe(true);
		}
	});

	it("files are decided by the explicit lists, not by an extension rule", () => {
		for (const dir of PUBLIC_DIRECTORIES) {
			expect(isLocalizablePagePath(`/${dir}/anything.xyz`), dir).toBe(false);
			expect(isLocalizablePagePath(`/${dir}/no-extension`), dir).toBe(false);
		}
		for (const file of GENERATED_ROOT_FILES) {
			expect(isLocalizablePagePath(file), file).toBe(false);
		}
		expect(isLocalizablePagePath("/manifest.webmanifest")).toBe(false);
		expect(isLocalizablePagePath("/_next/static/chunks/a.js")).toBe(false);
	});

	it("a directory name is a whole segment: look-alikes are pages", () => {
		expect(isLocalizablePagePath("/images")).toBe(true);
		expect(isLocalizablePagePath("/imagesx/a")).toBe(true);
		expect(isLocalizablePagePath("/data-sheet")).toBe(true);
	});

	it("is false for the non-localized subtrees and single paths too", () => {
		for (const path of [
			"/api/v1/x",
			"/adm/1/products",
			"/.well-known/x",
			"/sentinel",
			"/opengraph-image",
		]) {
			expect(isLocalizablePagePath(path), path).toBe(false);
		}
	});
});
