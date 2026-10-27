import {
	type APIRequestContext,
	expect,
	type Page,
	test,
} from "@playwright/test";
import { canonicalSellerSegment } from "./helpers/canonical-seller";
import {
	DYNAMIC_ROUTE_CATALOG,
	materializeDynamicRoute,
	type PublicDynamicFixtureIds,
} from "./helpers/dynamic-route-catalog";
import { isGetSessionUrl } from "./helpers/mock-gateway";
import { installProviderMocks } from "./helpers/mock-gsi";
import { assertNoResidualLoading } from "./helpers/page-stability";

const LOCAL_ORIGIN = `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`;
const ERROR_SHELL =
	/Application error|Internal Server Error|NEXT_NOT_FOUND|This page could not be found|商品已下架|糟糕，火箭偏離了航道|暫時迷航|這頁不在貨架上/i;
const ALLOWED_CONSOLE = [
	/Download the React DevTools/i,
	// The URL-less browser line is classified by the response/requestfailed
	// monitors below, which include all Hawkr media origins.
	/Failed to load resource/i,
	/\[Fast Refresh\]/i,
	/Content Security Policy directive 'upgrade-insecure-requests' is ignored/i,
];
// The media host the build under test was configured with (NEXT_PUBLIC_MEDIA_BASE_URL).
const HAWKR_MEDIA_HOSTS = new Set(
	[process.env.NEXT_PUBLIC_MEDIA_BASE_URL]
		.filter((value): value is string => Boolean(value?.trim()))
		.map((value) => new URL(value).hostname),
);

function hasControlCharacter(value: string): boolean {
	for (let index = 0; index < value.length; index++) {
		const code = value.charCodeAt(index);
		if (code <= 0x1f || code === 0x7f) return true;
	}
	return false;
}

interface RuntimeIssue {
	kind: "pageerror" | "console" | "http" | "requestfailed";
	detail: string;
}

function asNumericId(value: unknown): string {
	if (
		(typeof value === "string" || typeof value === "number") &&
		/^\d+$/.test(String(value))
	) {
		return String(value);
	}
	return "";
}

/**
 * Resolve real product/seller ids from the live public catalog, or `null` when
 * that catalog is legitimately empty (a pre-launch prod backend serves 0
 * products — see the order-callback note below). Three outcomes:
 *   (a) 200 + empty records → return null → the fixture-backed tests skip.
 *   (b) 200 + a record that omits numeric ids → contract breakage → throw.
 *   (c) 5xx / network fault → transient → retry, then throw.
 *
 * The seller id is then canonicalized to its `store_code`
 * (`canonicalSellerSegment`): the feed only exposes `store_id`, and
 * `/sellers/{store_id}` is a `308` to the storefront rather than the storefront
 * itself, so entering by it would land these document-health assertions on a
 * different URL than the one they asked for.
 */
async function discoverFixtureIds(
	request: APIRequestContext,
): Promise<PublicDynamicFixtureIds | null> {
	let lastFailure = "not attempted";
	for (let attempt = 1; attempt <= 3; attempt++) {
		try {
			const response = await request.get(
				"/api/v1/public/products?keyword=&page=1&page_size=1",
				{ failOnStatusCode: false, timeout: 15_000 },
			);
			if (response.status() >= 500) {
				lastFailure = `${response.status()} product fixture`;
				continue;
			}
			expect(response.status(), "product fixture response").toBe(200);
			const envelope = (await response.json()) as {
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
			const record = envelope.data?.records?.[0];
			// (a) Empty catalog is a deterministic, legitimate state — signal skip
			// rather than retrying an answer that will not change.
			if (!record) return null;
			const productId = asNumericId(record.id);
			const storeId = asNumericId(
				record.storeId ??
					record.account_id ??
					record.seller?.storeId ??
					record.seller?.store_id ??
					record.seller?.id,
			);
			// (b) A real record that cannot expose numeric ids is a broken contract,
			// not an empty catalog — keep failing.
			if (!productId || !storeId) {
				lastFailure =
					"product fixture record present but omitted numeric product/seller ids";
				continue;
			}
			// `null` here means the backend answered and this seller has no
			// store_code — middleware only redirects a key it could resolve, so it
			// serves its document straight from the store_id path. A lookup that
			// never got an answer throws instead of reaching this `??`, because
			// middleware would still resolve the code server-side and 308 us.
			const sellerId =
				(await canonicalSellerSegment(request, storeId)) ?? storeId;
			return { productId, sellerId };
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}
	throw new Error(`public dynamic fixtures unavailable: ${lastFailure}`);
}

function isMonitoredResource(url: string): boolean {
	try {
		const parsed = new URL(url);
		return (
			parsed.origin === LOCAL_ORIGIN || HAWKR_MEDIA_HOSTS.has(parsed.hostname)
		);
	} catch {
		return false;
	}
}

function attachRuntimeMonitors(page: Page): RuntimeIssue[] {
	const issues: RuntimeIssue[] = [];
	page.on("pageerror", (error) => {
		issues.push({
			kind: "pageerror",
			detail: error.stack || error.message,
		});
	});
	page.on("console", (message) => {
		if (message.type() !== "error") return;
		const text = message.text();
		if (ALLOWED_CONSOLE.some((pattern) => pattern.test(text))) return;
		issues.push({ kind: "console", detail: text.slice(0, 500) });
	});
	page.on("response", (response) => {
		const status = response.status();
		if (status < 400 || !isMonitoredResource(response.url())) return;
		if (
			status === 401 &&
			response.request().method() === "GET" &&
			isGetSessionUrl(response.url())
		) {
			return;
		}
		issues.push({
			kind: "http",
			detail: `${status} ${response.request().method()} ${response.url()}`,
		});
	});
	page.on("requestfailed", (request) => {
		const failure = request.failure()?.errorText ?? "unknown failure";
		if (/net::ERR_ABORTED|cancelled/i.test(failure)) return;
		if (!isMonitoredResource(request.url())) return;
		issues.push({
			kind: "requestfailed",
			detail: `${failure} ${request.url()}`,
		});
	});
	return issues;
}

async function assertNoUnresolvedLinks(page: Page, route: string) {
	const hrefs = await page.locator("a[href]").evaluateAll((anchors) =>
		anchors.flatMap((anchor) => {
			const href = anchor.getAttribute("href");
			return href ? [href] : [];
		}),
	);
	const failures = hrefs.filter((href) => {
		if (
			href.startsWith("//") ||
			href.includes("\\") ||
			hasControlCharacter(href)
		) {
			return true;
		}
		if (/(?:^|[/=?#])(?:undefined|null)(?:$|[/&#])/i.test(href)) {
			return true;
		}
		// 未解析的動態片段（`/products/[id]`）只可能出現在 path 上。整條 href 一起掃
		// 會誤傷第三方分享連結的 query —— Facebook 的 `__cft__[0]=` 就長這樣，而那是
		// 商品描述裡的正常外連，不是渲染失敗。
		const pathname = (() => {
			try {
				return new URL(href, "https://placeholder.invalid").pathname;
			} catch {
				return href;
			}
		})();
		return /\[[^\]]+\]/.test(pathname);
	});
	expect(failures, `${route} rendered unresolved or unsafe hrefs`).toEqual([]);
}

async function assertRenderedRoute(
	page: Page,
	route: string,
	expectedPathname = route,
) {
	const issues = attachRuntimeMonitors(page);
	const response = await page.goto(route, {
		waitUntil: "domcontentloaded",
		timeout: 25_000,
	});
	expect(response, `${route} returned no document response`).toBeTruthy();
	expect(response?.status() ?? 0, `${route} document status`).toBeLessThan(400);

	await expect
		.poll(() => new URL(page.url()).pathname, {
			timeout: 15_000,
			message: `${route} ended on an unexpected page`,
		})
		.toBe(expectedPathname);

	const content = page.locator("main.page-content");
	await expect(content).toBeVisible({ timeout: 15_000 });
	await expect
		.poll(async () => (await content.innerText()).trim().length, {
			timeout: 15_000,
			message: `${route} rendered only shared chrome`,
		})
		.toBeGreaterThan(20);
	await page.waitForTimeout(300);
	const text = await content.innerText();
	expect(text, `${route} rendered an error or unavailable shell`).not.toMatch(
		ERROR_SHELL,
	);
	expect(
		issues,
		`${route} emitted runtime/network failures:\n${issues
			.map((issue) => `[${issue.kind}] ${issue.detail}`)
			.join("\n")}`,
	).toEqual([]);
	await assertNoResidualLoading(page, route);
	await assertNoUnresolvedLinks(page, route);
}

test.describe("public dynamic route browser health", () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the full dynamic document inventory runs once in desktop Chromium",
	);

	let fixturesPromise: Promise<PublicDynamicFixtureIds | null> | undefined;

	for (const entry of DYNAMIC_ROUTE_CATALOG.filter(
		(candidate) =>
			candidate.access === "public" && candidate.fixture !== "order-callback",
	)) {
		test(`${entry.pattern} hard-loads and hydrates`, {
			tag: ["@contract", "@live"],
		}, async ({ page, request }) => {
			// Resolve data-backed IDs only for routes that actually need them.
			// Keeping this out of a describe-level beforeAll lets deterministic
			// order callback tests run even when the current public catalog is
			// legitimately empty (as production was during this audit).
			let route: string;
			if (entry.fixture === "rfq-prototype") {
				route = materializeDynamicRoute(entry);
			} else {
				fixturesPromise ??= discoverFixtureIds(request);
				const fixtures = await fixturesPromise;
				// An empty public catalog (pre-launch prod) leaves nothing to
				// materialize — skip these data-backed routes; they resume
				// automatically once the catalog carries a product. Deterministic
				// preview routes above do not depend on this catalog.
				test.skip(
					fixtures === null,
					"public catalog legitimately empty (pre-launch prod)",
				);
				if (!fixtures) return;
				route = materializeDynamicRoute(entry, fixtures);
			}
			await assertRenderedRoute(page, route);
		});
	}

	test("payment callback without a trade number reaches the protected canonical order", {
		tag: ["@ui", "@live"],
	}, async ({ page }) => {
		const entry = DYNAMIC_ROUTE_CATALOG.find(
			(candidate) => candidate.pattern === "/orders/[orderNo]",
		);
		if (!entry) throw new Error("payment callback route is missing");
		const route = materializeDynamicRoute(entry);
		// This callback intentionally redirects a guest into /login. Install the
		// provider bridge before the document loads so GSI sees a complete local
		// implementation and never requests accounts.google.com with CI's dummy
		// client id. Keep this scoped to the one login-producing route rather than
		// suppressing genuine console errors across the route inventory.
		await installProviderMocks(page);
		const issues = attachRuntimeMonitors(page);
		const response = await page.goto(route, { waitUntil: "domcontentloaded" });
		expect(response?.status() ?? 0).toBeLessThan(400);
		await expect(page).toHaveURL(
			/login\?role=buyer&redirect=%2Faccount%2Forders%2Froute-contract-order/,
		);
		await expect(
			page.getByRole("heading", { name: "登入 / 註冊" }),
		).toBeVisible();
		expect(issues).toEqual([]);
	});

	test("refund callback renders a stable guest result shell", {
		tag: ["@ui", "@live"],
	}, async ({ page }) => {
		await page.route("**/api/v1/app/after_sales**", (route) => {
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: { records: [], total: 0, page: 1, pageSize: 50 },
				}),
			});
		});
		const entry = DYNAMIC_ROUTE_CATALOG.find(
			(candidate) => candidate.pattern === "/orders/[orderNo]/refund",
		);
		if (!entry) throw new Error("refund callback route is missing");
		const route = `${materializeDynamicRoute(entry)}?status=success`;
		await assertRenderedRoute(
			page,
			route,
			"/orders/route-contract-order/refund",
		);
		await expect(
			page.getByRole("heading", { name: "退款資訊已送出" }),
		).toBeVisible();
	});
});
