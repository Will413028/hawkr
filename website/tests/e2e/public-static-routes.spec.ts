import { expect, type Page, test } from "@playwright/test";
import { requiredAccessForPath } from "@/lib/auth/redirect";
import {
	discoverAppPageRoutes,
	discoverPermanentRedirects,
} from "./helpers/app-route-inventory";
import {
	fulfillConnectError,
	GET_SESSION_URL,
	isGetSessionUrl,
} from "./helpers/mock-gateway";
import { installProviderMocks } from "./helpers/mock-gsi";
import { assertNoResidualLoading } from "./helpers/page-stability";

const LOADER_KEY = "hawkr:initial-loader-seen";
const PERMANENT_REDIRECT_SOURCES = new Set(
	discoverPermanentRedirects().map(({ source }) => source),
);
const PUBLIC_STATIC_ROUTES = discoverAppPageRoutes({
	includeDynamic: false,
}).filter(
	(route) =>
		!requiredAccessForPath(route) && !PERMANENT_REDIRECT_SOURCES.has(route),
);

const EXPECTED_PUBLIC_STATIC_ROUTES = [
	"/",
	"/about",
	// 代行交接失敗的落地頁（56 號 §4.5）。它必須匿名可達——兌換失敗的人正好就是
	// 沒有工作階段的人，擋掉他等於讓失敗無處落地。頁面本身只有兩段固定文案、
	// 不透傳 gateway 的 message，也不帶任何賣家資料。
	"/adm/unavailable",
	"/cart",
	"/contact",
	"/faq",
	"/feed",
	"/hawkr-agent-rfq-demo",
	"/login",
	"/market/products",
	"/payment/return",
	"/privacy",
	"/return-policy",
	"/search",
	"/seller",
	"/seller-guide",
	"/seller-terms",
	"/seller/onboarding",
	"/seller/pricing",
	"/terms",
	"/user-policy",
	// 站外業者的邀請信落地頁（#2539 E1）。匿名可達是刻意的——收到邀請的人正好
	// 就是沒有平台帳號的人。沒有 fragment 裡的 secret 時它只顯示「連結無效」，
	// 不呼叫任何端點，也不帶任何買家或賣家資料。
	"/vendor/rfq",
] as const;

/**
 * These pages are canonical App Router entries, but a direct guest visit cannot
 * render their guarded workflow without state created on the previous step.
 * Keep each intentional client redirect explicit so a new surprise redirect
 * cannot silently weaken the smoke test.
 */
const EXPECTED_FINAL_PATHNAME: Partial<Record<string, string>> = {
	"/payment/return": "/login",
	"/seller/onboarding": "/login",
};

const READY_SELECTOR: Partial<Record<string, string>> = {
	"/": '[data-hk-initial-content-ready="home"]',
	"/feed": '[data-hk-tab-content-ready="feed"]',
	"/market/products": '[data-hk-tab-content-ready="market"]',
};

const ERROR_SHELL =
	/Application error|Internal Server Error|NEXT_NOT_FOUND|This page could not be found|糟糕，火箭偏離了航道|暫時迷航|這頁不在貨架上/i;
const UNRESOLVED_HREF =
	/(?:^|[/=?&#])(?:undefined|null)(?=$|[/=?&#])|\[[^\]]+\]/i;

interface GuestPageState {
	sessionProbeHits: number;
}

function hasControlCharacter(value: string): boolean {
	for (let index = 0; index < value.length; index++) {
		const code = value.charCodeAt(index);
		if (code <= 0x1f || code === 0x7f) return true;
	}
	return false;
}

function attachRuntimeMonitors(page: Page, baseURL: string): string[] {
	const issues: string[] = [];
	const appOrigin = new URL(baseURL).origin;

	page.on("pageerror", (error) => {
		issues.push(`pageerror: ${error.stack || error.message}`);
	});
	page.on("console", (message) => {
		if (message.type() !== "error") return;
		// The browser logs a failed fetch's status as a console error; the guest
		// session probe's 401 is the answer "no session", not a broken page.
		if (
			message.text().includes("status of 401") &&
			isGetSessionUrl(message.location().url)
		) {
			return;
		}
		issues.push(`console.error: ${message.text()}`);
	});
	page.on("response", (response) => {
		if (response.status() < 400) return;
		// A guest's GetSession probe answers 401 `unauthenticated` (guest, not broken).
		if (
			response.status() === 401 &&
			response.request().method() === "GET" &&
			isGetSessionUrl(response.url())
		) {
			return;
		}
		try {
			if (new URL(response.url()).origin !== appOrigin) return;
		} catch {
			return;
		}
		issues.push(
			`HTTP ${response.status()} ${response.request().method()} ${response.url()}`,
		);
	});
	page.on("requestfailed", (request) => {
		const failure = request.failure()?.errorText ?? "unknown failure";
		let requestUrl: URL;
		try {
			requestUrl = new URL(request.url());
		} catch {
			return;
		}
		// Chromium aborts superseded fetches when a guard redirects or a query
		// observer is disposed during navigation. The destination document is
		// still verified below; only completed HTTP failures are actionable.
		if (/net::ERR_ABORTED|cancelled/i.test(failure)) {
			return;
		}
		if (requestUrl.origin === appOrigin) {
			issues.push(
				`requestfailed: ${request.method()} ${request.url()} (${failure})`,
			);
		}
	});

	return issues;
}

async function prepareGuestPage(page: Page): Promise<GuestPageState> {
	const state: GuestPageState = { sessionProbeHits: 0 };
	await installProviderMocks(page);
	await page.addInitScript((loaderKey) => {
		sessionStorage.setItem(loaderKey, "1");
	}, LOADER_KEY);
	// The session cookie is HttpOnly, so every page load asks GetSession; a guest
	// gets `unauthenticated`. The monitors above exempt exactly this probe.
	await page.route(GET_SESSION_URL, (route) => {
		if (route.request().method() !== "GET") {
			void route.fallback();
			return;
		}
		state.sessionProbeHits += 1;
		void fulfillConnectError(route, "unauthenticated");
	});
	return state;
}

async function waitForRenderedAnchors(page: Page) {
	let previous = "";
	let stableSamples = 0;

	for (let sample = 0; sample < 12; sample++) {
		const current = await page.locator("a[href]").evaluateAll((anchors) =>
			anchors
				.map((anchor) => anchor.getAttribute("href") ?? "")
				.sort()
				.join("\n"),
		);
		if (current === previous) stableSamples += 1;
		else stableSamples = 0;
		if (stableSamples >= 2) return;
		previous = current;
		await page.waitForTimeout(150);
	}
}

async function assertRenderedLinksSafe(page: Page, route: string) {
	const hrefs = await page
		.locator("a[href]")
		.evaluateAll((anchors) =>
			anchors.map((anchor) => anchor.getAttribute("href") ?? ""),
		);
	const current = new URL(page.url());
	const issues: string[] = [];

	for (const href of hrefs) {
		if (
			href.trimStart().startsWith("//") ||
			href.includes("\\") ||
			hasControlCharacter(href)
		) {
			issues.push(`unsafe href: ${href}`);
			continue;
		}

		let decodedHref: string;
		try {
			decodedHref = decodeURIComponent(href);
		} catch {
			issues.push(`invalid percent-encoding: ${href}`);
			continue;
		}
		if (UNRESOLVED_HREF.test(href) || UNRESOLVED_HREF.test(decodedHref)) {
			issues.push(`unresolved href: ${href}`);
			continue;
		}

		let target: URL;
		try {
			target = new URL(href, current);
		} catch {
			issues.push(`invalid href: ${href}`);
			continue;
		}

		if (target.origin === current.origin) {
			if (!["http:", "https:"].includes(target.protocol)) {
				issues.push(`unsafe internal protocol: ${href}`);
				continue;
			}
		} else if (!["https:", "mailto:", "tel:"].includes(target.protocol)) {
			issues.push(`unsafe external protocol: ${href}`);
			continue;
		}

		const linksToCurrentDocument =
			target.origin === current.origin &&
			target.pathname === current.pathname &&
			target.search === current.search;
		if (!linksToCurrentDocument || !href.includes("#")) continue;

		if (!target.hash) {
			issues.push(`missing hash target: ${href}`);
			continue;
		}
		let targetName: string;
		try {
			targetName = decodeURIComponent(target.hash.slice(1));
		} catch {
			issues.push(`invalid hash encoding: ${href}`);
			continue;
		}
		const targetExists = await page.evaluate(
			(name) =>
				document.getElementById(name) !== null ||
				document.getElementsByName(name).length > 0,
			targetName,
		);
		if (!targetExists) {
			issues.push(`missing hash target: ${href}`);
		}
	}

	expect(issues, `${route} rendered unsafe or unresolved links`).toEqual([]);
}

async function assertPublicStaticRoute(
	page: Page,
	route: string,
	baseURL: string,
	guestState: GuestPageState,
) {
	const runtimeIssues = attachRuntimeMonitors(page, baseURL);
	const response = await page.goto(route, {
		waitUntil: "domcontentloaded",
		timeout: 25_000,
	});
	expect(response, `${route} returned no document response`).toBeTruthy();
	expect(
		response?.status() ?? 0,
		`${route} document response must be successful`,
	).toBeLessThan(400);

	try {
		await expect(
			page.locator('[data-auth-hydrated="true"]'),
			`${route} never hydrated its guest shell`,
		).toBeAttached({ timeout: 15_000 });
	} catch (error) {
		const markerValues = await page
			.locator("[data-auth-hydrated]")
			.evaluateAll((nodes) =>
				nodes.map((node) => node.getAttribute("data-auth-hydrated")),
			);
		throw new Error(
			`${route} guest hydration diagnosis: session mock hits=${guestState.sessionProbeHits}, marker values=${JSON.stringify(markerValues)}, runtime issues=${JSON.stringify(runtimeIssues)}\n${error instanceof Error ? error.message : String(error)}`,
		);
	}

	const expectedPathname = EXPECTED_FINAL_PATHNAME[route] ?? route;
	await expect
		.poll(() => new URL(page.url()).pathname, {
			timeout: 15_000,
			message: `${route} ended on an unexpected page`,
		})
		.toBe(expectedPathname);

	// Scope marker lookups to the visible route tree. While React streams a
	// route, the same subtree transiently exists twice: once relocated into
	// place under <main> and once inside the body-level `<div hidden id="S:…">`
	// staging container it streamed into (~200ms until cleanup). An unscoped
	// strict locator that polls inside that window fails instantly with a
	// strict-mode violation. The app filters the same way — PageTransition's
	// isCurrentTabContentReady ignores markers under a [hidden] ancestor.
	if (expectedPathname === route && READY_SELECTOR[route]) {
		await expect(
			page.locator(`main.page-content ${READY_SELECTOR[route]}`),
			`${route} never published its page-ready marker`,
		).toBeAttached({ timeout: 15_000 });
	}

	const content =
		expectedPathname === "/feed"
			? page.locator('main.page-content [data-hk-tab-content-ready="feed"]')
			: page.locator("main").first();
	await expect(
		content,
		`${route} has no visible meaningful-content region`,
	).toBeVisible({ timeout: 15_000 });
	await expect
		.poll(async () => (await content.innerText()).trim().length, {
			timeout: 15_000,
			message: `${route} rendered an empty meaningful-content region`,
		})
		.toBeGreaterThan(10);

	await waitForRenderedAnchors(page);
	const bodyText = await page.locator("body").innerText();
	expect(bodyText, `${route} rendered an error/not-found shell`).not.toMatch(
		ERROR_SHELL,
	);
	expect(runtimeIssues, `${route} emitted runtime/network failures`).toEqual(
		[],
	);
	await assertNoResidualLoading(page, route);
	await assertRenderedLinksSafe(page, route);
}

test.describe("canonical public static route browser health", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the public static inventory runs once in desktop Chromium",
	);

	test("public static inventory stays explicit and complete", () => {
		expect(PUBLIC_STATIC_ROUTES).toEqual([...EXPECTED_PUBLIC_STATIC_ROUTES]);
		expect(PUBLIC_STATIC_ROUTES).toHaveLength(22);
	});

	// EVERY route here is `@live`, including the ones that render nothing but
	// static copy (`/terms`, `/privacy`, `/faq`…). Measured, not assumed: with
	// API_BASE_URL pointed at a closed port, all 21 fail — the shared guest
	// chrome client-fetches `/public/categories/tree`, `/public/hashtags/popular`,
	// `/public/tags/trending` and `/public/posts` on every guest-facing document,
	// and this suite counts any same-origin >=400 as a page-health failure. So a
	// legal page's "does it load cleanly" check is coupled to the shared dev
	// backend just as tightly as the storefront's. That coupling — UI intent,
	// live dependency — is the thing recorded fixtures are meant to cut
	// (hawkr-website#369), and it is four endpoints wide, not three specs wide.
	for (const route of PUBLIC_STATIC_ROUTES) {
		test(`${route} hard-loads, hydrates, and renders safe links`, {
			tag: ["@live"],
		}, async ({ page, baseURL }) => {
			test.setTimeout(45_000);
			const appBaseURL = baseURL ?? "http://localhost:3000";
			const guestState = await prepareGuestPage(page);
			await assertPublicStaticRoute(page, route, appBaseURL, guestState);
		});
	}
});
