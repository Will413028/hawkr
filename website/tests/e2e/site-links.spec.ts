import {
	type APIRequestContext,
	expect,
	type Page,
	test,
} from "@playwright/test";
import { ACCESS_CLASSES, requiredAccessForPath } from "@/lib/auth/redirect";
import {
	discoverAppPageRoutes,
	discoverPermanentRedirects,
	discoverRedirectOnlyPages,
} from "./helpers/app-route-inventory";
import { canonicalSellerSegment } from "./helpers/canonical-seller";
import {
	DYNAMIC_ROUTE_CATALOG,
	type DynamicRouteCatalogEntry,
	materializeDynamicRoute,
	type PublicDynamicFixtureIds,
} from "./helpers/dynamic-route-catalog";
import { gotoStreamed } from "./helpers/streamed-navigation";

const STATIC_PAGE_ROUTES = discoverAppPageRoutes({ includeDynamic: false });
const DYNAMIC_PAGE_ROUTES = discoverAppPageRoutes().filter((route) =>
	route.includes("["),
);
const PERMANENT_REDIRECTS = discoverPermanentRedirects();
const PERMANENT_REDIRECT_SOURCES = new Set(
	PERMANENT_REDIRECTS.map((redirect) => redirect.source),
);
const PROTECTED_STATIC_PAGE_ROUTES = STATIC_PAGE_ROUTES.flatMap((route) => {
	const access = requiredAccessForPath(route);
	return access ? [{ route, access }] : [];
});

function expectedLoginRole(
	access: NonNullable<ReturnType<typeof requiredAccessForPath>>,
): string | null {
	return ACCESS_CLASSES[access].workspace;
}
const REDIRECT_ONLY_PAGE_ROUTES = new Set(discoverRedirectOnlyPages());
const PUBLIC_STATIC_PAGE_ROUTES = STATIC_PAGE_ROUTES.filter(
	(route) =>
		!requiredAccessForPath(route) &&
		!PERMANENT_REDIRECT_SOURCES.has(route) &&
		!REDIRECT_ONLY_PAGE_ROUTES.has(route),
);
const REDIRECT_ONLY_PUBLIC_PAGE_ROUTES = STATIC_PAGE_ROUTES.filter(
	(route) =>
		REDIRECT_ONLY_PAGE_ROUTES.has(route) &&
		!requiredAccessForPath(route) &&
		!PERMANENT_REDIRECT_SOURCES.has(route),
);
const PROTECTED_DYNAMIC_ROUTES = DYNAMIC_ROUTE_CATALOG.flatMap((route) =>
	route.access === "public" || route.access === "delegated"
		? []
		: [{ ...route, access: route.access }],
);
const DELEGATED_DYNAMIC_ROUTES = DYNAMIC_ROUTE_CATALOG.filter(
	(route) => route.access === "delegated",
);
const PUBLIC_DYNAMIC_ROUTES = DYNAMIC_ROUTE_CATALOG.filter(
	(route) => route.access === "public",
);
const PUBLIC_CALLBACK_DYNAMIC_ROUTES = PUBLIC_DYNAMIC_ROUTES.filter(
	(route) => route.fixture === "order-callback",
);
const PUBLIC_DATA_DYNAMIC_ROUTES = PUBLIC_DYNAMIC_ROUTES.filter(
	(route) =>
		route.fixture !== "order-callback" && route.fixture !== "seller-thread",
);
const PUBLIC_THREAD_DYNAMIC_ROUTES = PUBLIC_DYNAMIC_ROUTES.filter(
	(route) => route.fixture === "seller-thread",
);
const CRAWL_SEEDS = [
	"/",
	"/feed",
	"/market/products",
	"/search",
	"/seller",
	"/about",
	"/faq",
	"/contact",
	"/terms",
	"/privacy",
	"/return-policy",
	"/user-policy",
	"/seller-terms",
	"/login",
] as const;
const RETRYABLE = (status: number) => status >= 500;
const FRAMEWORK_NOT_FOUND =
	/This page could not be found|NEXT_NOT_FOUND|"statusCode"\s*:\s*404/i;

interface LinkReference {
	href: string;
	sources: Set<string>;
}

interface JsonResponse {
	status: number;
	body: unknown;
}

interface DocumentResponse {
	status: number;
	contentType: string;
	body: string;
}

function asFixtureId(value: unknown): string {
	if (
		(typeof value === "string" || typeof value === "number") &&
		/^\d+$/.test(String(value))
	) {
		return String(value);
	}
	return "";
}

async function getJsonWithRetry(
	request: APIRequestContext,
	url: string,
): Promise<JsonResponse> {
	let lastFailure = "request was not attempted";
	for (let attempt = 1; attempt <= 3; attempt++) {
		try {
			const response = await request.get(url, {
				failOnStatusCode: false,
				timeout: 15_000,
			});
			const status = response.status();
			if (status >= 400 && status < 500) {
				throw new Error(`${status} ${url}`);
			}
			if (RETRYABLE(status)) {
				lastFailure = `${status} ${url}`;
				continue;
			}
			const body = await response.json().catch((error: unknown) => {
				throw new Error(
					`${url} returned invalid JSON: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			});
			return { status, body };
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
			if (/^4\d\d /.test(lastFailure)) throw error;
			if (attempt === 3) break;
		}
	}
	throw new Error(`test gateway unavailable after 3 attempts: ${lastFailure}`);
}

/**
 * Resolve fixture ids from the SAME backend the server under test renders
 * against: the local origin's /api/v1 proxy follows API_BASE_URL, so a
 * prod-flavored build is probed against the formal gateway instead of being
 * handed dev-only ids it can only 404 on. Returns `null` when that catalog is
 * legitimately empty (pre-launch prod) so data-backed checks can skip; a
 * record that omits its numeric ids, or a persistent 5xx, still fails.
 *
 * The seller id is canonicalized to its `store_code` (`canonicalSellerSegment`)
 * because the feed only exposes `store_id`, and `/sellers/{store_id}` answers
 * the `308` that consolidates the subtree — not the document these checks are
 * about. The redirect itself is asserted separately, below.
 */
async function discoverPublicDynamicFixtures(
	request: APIRequestContext,
): Promise<PublicDynamicFixtureIds | null> {
	const { body } = await getJsonWithRetry(
		request,
		"/api/v1/public/products?keyword=&page=1&page_size=1",
	);
	const data = body as {
		data?: {
			records?: Array<{
				id?: unknown;
				storeId?: unknown;
				account_id?: unknown;
				seller?: {
					id?: unknown;
					storeId?: unknown;
					store_id?: unknown;
				};
			}>;
		};
	};
	const record = data.data?.records?.[0];
	if (!record) return null;
	const productId = asFixtureId(record.id);
	const storeId = asFixtureId(
		record.storeId ??
			record.account_id ??
			record.seller?.storeId ??
			record.seller?.store_id ??
			record.seller?.id,
	);

	expect(
		productId,
		"gateway must expose a numeric id on its first product fixture",
	).not.toBe("");
	expect(
		storeId,
		"the first product fixture must expose its numeric seller account id",
	).not.toBe("");

	const sellerId = (await canonicalSellerSegment(request, storeId)) ?? storeId;
	return { productId, sellerId, sellerStoreId: storeId };
}

async function getDocumentWithRetry(
	request: APIRequestContext,
	route: string,
): Promise<DocumentResponse> {
	let lastFailure = "request was not attempted";
	for (let attempt = 1; attempt <= 2; attempt++) {
		try {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
				timeout: 15_000,
			});
			const status = response.status();
			const result = {
				status,
				contentType: response.headers()["content-type"] ?? "",
				body: await response.text(),
			};
			if (!RETRYABLE(status)) return result;
			lastFailure = `${status} ${route}`;
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}
	throw new Error(`${route} failed after 2 attempts: ${lastFailure}`);
}

function publicDocumentFailure(
	entry: DynamicRouteCatalogEntry,
	route: string,
	response: DocumentResponse,
): string | null {
	if (response.status < 200 || response.status >= 300) {
		return `${response.status} ${route}`;
	}
	if (!response.contentType.toLowerCase().includes("text/html")) {
		return `${response.status} ${route} returned ${response.contentType || "no content-type"}`;
	}
	if (FRAMEWORK_NOT_FOUND.test(response.body)) {
		return `${response.status} ${route} rendered the framework not-found document`;
	}
	if (entry.fixture === "product" && response.body.includes("商品已下架")) {
		return `${response.status} ${route} rendered the unavailable-product shell`;
	}
	return null;
}

function normalizeInternalHref(
	rawHref: string,
	currentUrl: string,
): string | null {
	try {
		const current = new URL(currentUrl);
		const target = new URL(rawHref, current);
		if (target.origin !== current.origin) return null;
		if (!["http:", "https:"].includes(target.protocol)) return null;
		if (
			target.pathname.startsWith("/_next/") ||
			target.pathname.startsWith("/api/")
		) {
			return null;
		}
		target.hash = "";
		for (const key of [
			"_rsc",
			"dpl",
			"utm_source",
			"utm_medium",
			"utm_campaign",
		]) {
			target.searchParams.delete(key);
		}
		target.searchParams.sort();
		return `${target.pathname}${target.search}`;
	} catch {
		return null;
	}
}

async function collectRenderedLinks(
	page: Page,
	seed: string,
	links: Map<string, LinkReference>,
) {
	const response = await gotoStreamed(page, seed);
	expect(response, `${seed} returned no document response`).toBeTruthy();
	expect(
		response?.status() ?? 0,
		`${seed} failed before links could be collected`,
	).toBeLessThan(400);
	await page
		.waitForFunction(
			() => (document.body?.innerText.trim().length ?? 0) > 20,
			null,
			{ timeout: 15_000 },
		)
		.catch(() => {});
	await page.waitForTimeout(250);

	const hrefs = await page
		.locator("a[href]")
		.evaluateAll((anchors) =>
			anchors
				.map((anchor) => anchor.getAttribute("href"))
				.filter((href): href is string => Boolean(href)),
		);
	for (const rawHref of hrefs) {
		const href = normalizeInternalHref(rawHref, page.url());
		if (!href) continue;
		const existing = links.get(href);
		if (existing) existing.sources.add(seed);
		else links.set(href, { href, sources: new Set([seed]) });
	}
}

async function renderedLinkFailure(
	request: APIRequestContext,
	link: LinkReference,
): Promise<string | null> {
	const parsedLink = new URL(link.href, "http://route-contract.local");
	const requiredAccess = requiredAccessForPath(parsedLink.pathname);
	let lastFailure = "request was not attempted";

	for (let attempt = 1; attempt <= 2; attempt++) {
		try {
			const response = await request.get(link.href, {
				failOnStatusCode: false,
				maxRedirects: requiredAccess ? 0 : 5,
				timeout: 15_000,
			});
			if (RETRYABLE(response.status()) && attempt < 2) continue;

			if (requiredAccess) {
				const location = response.headers().location ?? "";
				const target = new URL(location, "http://route-contract.local");
				if (
					response.status() === 307 &&
					target.pathname === "/login" &&
					target.searchParams.get("role") ===
						expectedLoginRole(requiredAccess) &&
					target.searchParams.get("redirect") === link.href
				) {
					return null;
				}
				return `${response.status()} ${link.href} -> ${location || "(no location)"}`;
			}

			if (response.status() !== 200) {
				return `${response.status()} ${link.href}`;
			}
			const finalUrl = new URL(response.url());
			if (parsedLink.pathname !== "/login" && finalUrl.pathname === "/login") {
				return `${link.href} unexpectedly ended at ${finalUrl.pathname}${finalUrl.search}`;
			}
			return null;
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}

	return `REQUEST_FAILED ${link.href}: ${lastFailure}`;
}

test.describe("whole-site route and link contract", () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"HTTP route/link inventory only needs one browser project",
	);

	test("every canonical public static page serves 200", {
		tag: ["@ui", "@live"],
	}, async ({ request }) => {
		test.setTimeout(180_000);
		const failures: string[] = [];
		for (const route of PUBLIC_STATIC_PAGE_ROUTES) {
			try {
				const response = await request.get(route, {
					failOnStatusCode: false,
					maxRedirects: 0,
					timeout: 15_000,
				});
				if (response.status() !== 200) {
					failures.push(`${response.status()} ${route}`);
				}
			} catch (error) {
				failures.push(
					`REQUEST_FAILED ${route}: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			}
		}
		expect(
			failures,
			`${PUBLIC_STATIC_PAGE_ROUTES.length} canonical public static routes checked:\n${failures.join("\n")}`,
		).toEqual([]);
	});

	test("every protected static page sends guests to login", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const { route, access } of PROTECTED_STATIC_PAGE_ROUTES) {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (
				response.status() !== 307 ||
				parsed.pathname !== "/login" ||
				parsed.searchParams.get("redirect") !== route ||
				parsed.searchParams.get("role") !== expectedLoginRole(access)
			) {
				failures.push(
					`${response.status()} ${route} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(failures).toEqual([]);
	});

	test("every redirect-only public page redirects instead of rendering", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const route of REDIRECT_ONLY_PUBLIC_PAGE_ROUTES) {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (response.status() !== 307 || !location || parsed.pathname === route) {
				failures.push(
					`${response.status()} ${route} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(
			failures,
			`${REDIRECT_ONLY_PUBLIC_PAGE_ROUTES.length} redirect-only public routes checked`,
		).toEqual([]);
	});

	test("every static page belongs to one explicit response class", {
		tag: ["@ui"],
	}, () => {
		const classified = [
			...PUBLIC_STATIC_PAGE_ROUTES,
			...PROTECTED_STATIC_PAGE_ROUTES.map(({ route }) => route),
			...STATIC_PAGE_ROUTES.filter((route) =>
				PERMANENT_REDIRECT_SOURCES.has(route),
			),
			...REDIRECT_ONLY_PUBLIC_PAGE_ROUTES,
		].sort();
		expect(classified).toEqual(STATIC_PAGE_ROUTES);
	});

	test("dynamic route catalog exactly matches the App Router inventory", {
		tag: ["@ui"],
	}, () => {
		const catalogPatterns = DYNAMIC_ROUTE_CATALOG.map(
			(route) => route.pattern,
		).sort();
		expect(
			catalogPatterns,
			"every addressable dynamic page needs exactly one deliberate catalog entry",
		).toEqual(DYNAMIC_PAGE_ROUTES);
	});

	test("every protected dynamic page sends guests to login", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const entry of PROTECTED_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry);
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (
				response.status() !== 307 ||
				parsed.pathname !== "/login" ||
				parsed.searchParams.get("redirect") !== route ||
				parsed.searchParams.get("role") !== expectedLoginRole(entry.access)
			) {
				failures.push(
					`${response.status()} ${route} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(failures).toEqual([]);
	});

	test("delegated workspace routes stay guest-safe and unindexed", {
		tag: ["@ui", "@live"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const entry of DELEGATED_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry);
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			if (response.status() >= 500) {
				failures.push(`${response.status()} ${route}`);
				continue;
			}
			const html = await response.text();
			if (!/<meta[^>]+name="robots"[^>]+noindex/i.test(html)) {
				failures.push(`${route} 缺 noindex`);
			}
			if (/data-(seller|store)-name/i.test(html)) {
				failures.push(`${route} 骨架含賣家資料屬性`);
			}
			if (/data-hk-seller-workspace-ready/i.test(html)) {
				failures.push(`${route} 骨架已渲染工作區外框（工作階段閘門被繞過）`);
			}
		}
		expect(failures).toEqual([]);
	});

	test("工作區索引在 HTTP 層轉址，且不吃掉同層的具名路由", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const index = await request.get("/adm/1", {
			failOnStatusCode: false,
			maxRedirects: 0,
		});
		expect(index.status()).toBe(307);
		expect(index.headers().location).toMatch(/\/adm\/1\/products$/);

		const siblings = await Promise.all(
			["/adm/enter", "/adm/unavailable"].map(async (path) => {
				const res = await request.get(path, {
					failOnStatusCode: false,
					maxRedirects: 0,
				});
				return `${path} -> ${res.status()} ${res.headers().location ?? ""}`.trim();
			}),
		);
		expect(siblings.filter((s) => /\/products$/.test(s))).toEqual([]);
	});

	test("every callback public dynamic page serves a real guest document", {
		tag: ["@ui", "@live"],
	}, async ({ request }) => {
		const failures: string[] = [];

		for (const entry of PUBLIC_CALLBACK_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry);
			try {
				const response = await getDocumentWithRetry(request, route);
				const failure = publicDocumentFailure(entry, route, response);
				if (failure) failures.push(failure);
			} catch (error) {
				failures.push(
					`REQUEST_FAILED ${route}: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			}
		}

		expect(PUBLIC_DYNAMIC_ROUTES).toHaveLength(10);
		expect(PUBLIC_CALLBACK_DYNAMIC_ROUTES).toHaveLength(2);
		expect(
			failures,
			`${PUBLIC_CALLBACK_DYNAMIC_ROUTES.length} callback public dynamic routes checked:\n${failures.join("\n")}`,
		).toEqual([]);
	});

	test("every data-backed public dynamic page serves a real guest document", {
		tag: ["@contract", "@live"],
	}, async ({ request }) => {
		test.setTimeout(120_000);
		const fixtures = await discoverPublicDynamicFixtures(request);
		test.skip(
			fixtures === null,
			"public catalog legitimately empty (pre-launch prod)",
		);
		if (!fixtures) return;
		const failures: string[] = [];

		for (const entry of PUBLIC_DATA_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry, fixtures);
			try {
				const response = await getDocumentWithRetry(request, route);
				const failure = publicDocumentFailure(entry, route, response);
				if (failure) failures.push(failure);
			} catch (error) {
				failures.push(
					`REQUEST_FAILED ${route}: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			}
		}

		expect(PUBLIC_DATA_DYNAMIC_ROUTES).toHaveLength(7);
		expect(
			failures,
			`${PUBLIC_DATA_DYNAMIC_ROUTES.length} data-backed public dynamic routes checked with product ${fixtures.productId} / seller ${fixtures.sellerId}:\n${failures.join("\n")}`,
		).toEqual([]);
	});

	test("an unknown seller forum thread answers 404 instead of a document", {
		tag: ["@contract", "@live"],
	}, async ({ request }) => {
		const fixtures = await discoverPublicDynamicFixtures(request);
		test.skip(
			fixtures === null,
			"public catalog legitimately empty (pre-launch prod)",
		);
		if (!fixtures) return;
		expect(PUBLIC_THREAD_DYNAMIC_ROUTES).toHaveLength(1);
		for (const entry of PUBLIC_THREAD_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry, fixtures);
			const response = await getDocumentWithRetry(request, route);
			expect(response.status, route).toBe(404);
		}
	});

	test("every permanent redirect preserves its destination and query", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const redirect of PERMANENT_REDIRECTS) {
			const response = await request.get(
				`${redirect.source}?route_contract=1`,
				{
					failOnStatusCode: false,
					maxRedirects: 0,
				},
			);
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (
				response.status() !== 308 ||
				parsed.pathname !== redirect.destination ||
				parsed.searchParams.get("route_contract") !== "1"
			) {
				failures.push(
					`${response.status()} ${redirect.source} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(PERMANENT_REDIRECTS.length).toBeGreaterThan(0);
		expect(failures).toEqual([]);
	});

	test("every rendered internal link reaches a valid document", {
		tag: ["@contract", "@live"],
	}, async ({ page, request }) => {
		test.setTimeout(180_000);
		const links = new Map<string, LinkReference>();
		for (const seed of CRAWL_SEEDS) {
			await collectRenderedLinks(page, seed, links);
		}

		const failures: string[] = [];
		for (const link of links.values()) {
			const failure = await renderedLinkFailure(request, link);
			if (failure) {
				failures.push(
					`${failure} (rendered on ${[...link.sources].join(", ")})`,
				);
			}
		}

		expect(links.size, "crawler found no internal links").toBeGreaterThan(0);
		expect(
			failures,
			`${links.size} rendered internal links checked:\n${failures.join("\n")}`,
		).toEqual([]);
	});
});
