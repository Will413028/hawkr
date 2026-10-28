import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";
import { allRoutableLocales, defaultLocale } from "../lib/intl/locales";
import { PRIVATE_PREFIXES } from "../lib/seo/indexability";
import nextConfig from "../next.config.ts";

// 公開頁：禁止未驗證重用（擋部署 skew），但允許 bfcache。
const NO_CACHE = "private, no-cache, max-age=0, must-revalidate";
// 私有路由：連瀏覽器磁碟與 bfcache 都不留。
const NO_STORE = "private, no-cache, no-store, max-age=0, must-revalidate";

type HeaderEntry = { key: string; value: string };
type HeaderMatcher = { type: string; key: string; value?: string };
type HeaderRule = {
	source: string;
	has?: HeaderMatcher[];
	missing?: HeaderMatcher[];
	headers: HeaderEntry[];
};

async function loadRules(): Promise<HeaderRule[]> {
	const headers = nextConfig.headers;
	assert.ok(typeof headers === "function", "next.config must define headers()");
	return (await headers()) as unknown as HeaderRule[];
}

function cacheControlOf(rule: HeaderRule): string | undefined {
	return rule.headers.find((header) => header.key === "Cache-Control")?.value;
}

const require = createRequire(import.meta.url);
const matchHas =
	require("next/dist/shared/lib/router/utils/prepare-destination.js")
		.matchHas as (
		req: { headers: Record<string, string> },
		query: Record<string, string>,
		has?: HeaderMatcher[],
		missing?: HeaderMatcher[],
	) => false | Record<string, string>;
// Next 自己的 source 比對器：規則的 `source` 走 path-to-regexp，不能用字串相等
// 去猜。用它，測試量到的才是 runtime 真正的匹配結果。
const getPathMatch = require("next/dist/shared/lib/router/utils/path-match.js")
	.getPathMatch as (
	path: string,
	options?: { strict?: boolean; sensitive?: boolean },
) => (pathname: string) => false | Record<string, unknown>;

function effectiveCacheControl(
	rules: HeaderRule[],
	path: string,
	headers: Record<string, string>,
): string | undefined {
	let value: string | undefined;
	for (const rule of rules) {
		if (getPathMatch(rule.source)(path) === false) continue;
		if (matchHas({ headers }, {}, rule.has, rule.missing) === false) continue;
		value = cacheControlOf(rule) ?? value;
	}
	return value;
}

const REQUEST_SHAPES: Array<[string, Record<string, string>]> = [
	[
		"browser document",
		{
			accept:
				"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,*/*;q=0.8",
		},
	],
	["RSC payload", { accept: "text/x-component", rsc: "1" }],
	["wildcard accept", { accept: "*/*" }],
	["JSON accept", { accept: "application/json" }],
	["image accept", { accept: "image/avif,image/webp,*/*;q=0.8" }],
	["no Accept header", {}],
	["unknown RSC value", { accept: "*/*", rsc: "2" }],
	["unknown RSC value on a document", { accept: "text/html", rsc: "2" }],
];

const PUBLIC_PATHS = [
	"/",
	"/products/1",
	"/sellers/abc",
	"/sellers/abc/reviews",
	"/market/products",
	"/feed",
	"/faq",
	"/seller-guide",
	// 公開行銷頁：與 /seller/onboarding 同前綴，但必須留在 no-cache 這邊。
	"/seller",
	"/seller/pricing",
	// 前綴只在完整路徑段才算：`/accounting` 不是 `/account`、`/apis` 不是 `/api`。
	"/accounting",
	"/cartridge",
	"/apis",
];

// 手寫、不從 next.config 或 PRIVATE_PREFIXES 反推的敏感面清單。這份清單的
// 存在理由就是「設定漏了 app/ 裡的某個敏感頁時要紅」——如果從設定生成期望值，
// 測試只能證明設定跟自己一致。新增敏感路由時要同時補這裡。
const PRIVATE_PATHS = [
	"/account",
	"/account/orders",
	"/account/orders/HK123",
	"/account/messages/abc",
	"/seller-center",
	"/seller-center/orders/HK123",
	"/seller/measures",
	"/seller/measures/m1",
	"/seller/measures/m1/appeal",
	"/seller/measures/m1/remediation",
	"/seller-product-import-preview/job1",
	"/checkout",
	"/checkout/success",
	"/cart",
	"/payment/return",
	"/store-map/return",
	"/login",
	"/login/sso-callback",
	"/search",
	"/orders/HK123",
	"/orders/HK123/refund",
	"/adm",
	"/adm/123/dashboard",
	"/adm/enter",
	// 一次性 token 驅動的報價確認頁。
	"/quote-confirm",
	// 開店流程：承接已驗證手機與表單狀態。
	"/seller/onboarding",
	// route handler 自己宣告 no-store 是契約，全域規則不得把它降級。
	"/api/health",
	"/api/public/tab-bootstrap/v1/home",
	"/api/v1/app/orders",
	"/api/revalidate",
	// 快取金絲雀探針。
	"/sentinel",
];

test("page responses use three exhaustive no-cache header rules", async () => {
	const rules = await loadRules();

	assert.deepEqual(rules.slice(0, 3), [
		{
			source: "/(.*)",
			has: [{ type: "header", key: "accept", value: ".*text/html.*" }],
			headers: [{ key: "Cache-Control", value: NO_CACHE }],
		},
		{
			source: "/(.*)",
			has: [{ type: "header", key: "rsc", value: "1" }],
			headers: [{ key: "Cache-Control", value: NO_CACHE }],
		},
		{
			source: "/(.*)",
			missing: [
				{ type: "header", key: "accept", value: ".*text/html.*" },
				{ type: "header", key: "rsc", value: "1" },
			],
			headers: [{ key: "Cache-Control", value: NO_CACHE }],
		},
	]);

	const source = await readFile(
		new URL("../next.config.ts", import.meta.url),
		"utf8",
	);
	assert.match(source, /const PAGE_RESPONSE_CACHE_HEADERS = \[/);
	assert.match(source, /const PRIVATE_RESPONSE_CACHE_HEADERS = \[/);
	const forbiddenSourceTerms = [
		["EDGE", "CACHEABLE", "ROUTES"].join("_"),
		["EDGE", "CACHE", "STATIC", "SENTINEL", "SECONDS"].join("_"),
		["s", "maxage="].join("-"),
		["Workers", "Cache"].join(" "),
	];
	assert.doesNotMatch(source, new RegExp(forbiddenSourceTerms.join("|")));
});

test("every public page request shape is private, revalidated, and bfcache-eligible", async () => {
	const rules = await loadRules();

	for (const path of PUBLIC_PATHS) {
		for (const [label, headers] of REQUEST_SHAPES) {
			assert.equal(
				effectiveCacheControl(rules, path, headers),
				NO_CACHE,
				`${path} must return the no-cache policy for ${label}`,
			);
		}
	}
});

test("every private route request shape is no-store", async () => {
	const rules = await loadRules();

	for (const path of PRIVATE_PATHS) {
		for (const [label, headers] of REQUEST_SHAPES) {
			assert.equal(
				effectiveCacheControl(rules, path, headers),
				NO_STORE,
				`${path} must return the no-store policy for ${label}`,
			);
		}
	}
});

test("the no-store rule still covers every SEO-private prefix", async () => {
	const rules = await loadRules();
	const privateRule = rules.find((rule) => cacheControlOf(rule) === NO_STORE);
	assert.ok(privateRule, "a no-store rule must exist");
	for (const prefix of PRIVATE_PREFIXES) {
		for (const [label, headers] of REQUEST_SHAPES) {
			assert.equal(
				effectiveCacheControl(rules, prefix, headers),
				NO_STORE,
				`${prefix} (from PRIVATE_PREFIXES) must be no-store for ${label}`,
			);
		}
	}
});

test("no page header rule opts out of the two cache policies", async () => {
	const rules = await loadRules();
	const cacheRules = rules.filter((rule) => cacheControlOf(rule) !== undefined);

	// The three public rules, the no-store rule, and one more no-store rule per
	// routed non-default locale (prod routes none, so there it is exactly four).
	assert.equal(cacheRules.length, 4 + NON_DEFAULT_LOCALES.length);
	for (const rule of cacheRules.slice(0, 3)) {
		assert.equal(cacheControlOf(rule), NO_CACHE);
	}
	for (const rule of cacheRules.slice(3)) {
		assert.equal(cacheControlOf(rule), NO_STORE);
	}
});

// 語系前綴不能讓一個頁面脫離它的快取政策：只認無前綴路徑的 source 在 `/en/…` 底下不會命中，
// no-store 會悄悄失效。手寫的清單，同上面的理由。
const NON_DEFAULT_LOCALES = allRoutableLocales().filter(
	(locale) => locale !== defaultLocale,
);
const LOCALIZED_PUBLIC_PATHS = [
	"",
	"/products/1",
	"/sellers/abc/reviews",
	"/market/products",
	"/faq",
	"/seller",
	"/seller/pricing",
	"/accounting",
	"/apis",
];
const LOCALIZED_PRIVATE_PATHS = [
	"/account",
	"/account/orders/HK123",
	"/seller-center",
	"/seller-center/orders/HK123",
	"/seller/measures/m1/appeal",
	"/seller-product-import-preview/job1",
	"/checkout/success",
	"/cart",
	"/payment/return",
	"/login",
	"/login/sso-callback",
	"/search",
	"/orders/HK123/refund",
	"/seller/onboarding",
];

test("a routed non-default locale keeps every page's cache policy: public stays no-cache, private stays no-store", async () => {
	const rules = await loadRules();
	for (const locale of NON_DEFAULT_LOCALES) {
		for (const path of LOCALIZED_PUBLIC_PATHS) {
			for (const [label, headers] of REQUEST_SHAPES) {
				assert.equal(
					effectiveCacheControl(rules, `/${locale}${path}`, headers),
					NO_CACHE,
					`/${locale}${path} must return the no-cache policy for ${label}`,
				);
			}
		}
		for (const path of LOCALIZED_PRIVATE_PATHS) {
			for (const [label, headers] of REQUEST_SHAPES) {
				assert.equal(
					effectiveCacheControl(rules, `/${locale}${path}`, headers),
					NO_STORE,
					`/${locale}${path} must return the no-store policy for ${label}`,
				);
			}
		}
	}
});

test("the localized no-store rules are not applied to the staff and infrastructure paths that have no locale", async () => {
	const rules = await loadRules();
	for (const locale of NON_DEFAULT_LOCALES) {
		for (const path of ["/adm", "/api/health", "/sentinel"]) {
			// `/en/adm` is not a route: it gets the ordinary page policy, not a private one.
			assert.equal(
				effectiveCacheControl(rules, `/${locale}${path}`, {
					accept: "text/html",
				}),
				NO_CACHE,
				`/${locale}${path}`,
			);
		}
	}
});

test("security headers cover every route", async () => {
	const rules = await loadRules();
	const security = rules.findLast(
		(rule) =>
			rule.source === "/(.*)" &&
			rule.headers.some((header) => header.key === "X-Content-Type-Options"),
	);

	assert.equal(security?.source, "/(.*)");
	assert.ok(
		security?.headers.some(
			(header) =>
				header.key === "X-Content-Type-Options" && header.value === "nosniff",
		),
	);
	assert.ok(
		security?.headers.some(
			(header) =>
				header.key === "Referrer-Policy" &&
				header.value === "strict-origin-when-cross-origin",
		),
	);
});

test("Permissions-Policy allows geolocation only for same-origin pages", async () => {
	const rules = await loadRules();
	const security = rules.findLast((rule) =>
		rule.headers.some((header) => header.key === "Permissions-Policy"),
	);
	const permissionsPolicy = security?.headers.find(
		(header) => header.key === "Permissions-Policy",
	);

	assert.equal(security?.source, "/(.*)");
	assert.equal(
		permissionsPolicy?.value,
		"camera=(), microphone=(), geolocation=(self)",
	);
});
