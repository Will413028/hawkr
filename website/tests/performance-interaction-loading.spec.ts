import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import {
	expect,
	type Locator,
	type Page,
	type Route,
	test,
} from "@playwright/test";
import { mockGuestSession } from "./e2e/helpers/mock-gateway";

const REPO_ROOT = process.cwd();
const CHUNK_ROOT = path.join(REPO_ROOT, ".next/static/chunks");
const ROUTE_STATS = path.join(
	REPO_ROOT,
	".next/diagnostics/route-bundle-stats.json",
);

const INTERACTION_MARKERS = {
	menu: "data-mega-menu-content",
	search: "data-home-search-product",
} as const;

type FixtureKey =
	| "menuCategories"
	| "menuHashtags"
	| "menuPosts"
	| "menuTrending"
	| "menuBanners"
	| "searchHashtags"
	| "searchSuggest"
	| "searchProducts";

type FixtureRequests = Record<FixtureKey, string[]>;

function walkJs(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const candidate = path.join(directory, entry.name);
		if (entry.isDirectory()) return walkJs(candidate);
		return entry.isFile() && entry.name.endsWith(".js") ? [candidate] : [];
	});
}

function clientChunksContaining(marker: string): string[] {
	const matches = walkJs(CHUNK_ROOT)
		.filter((file) => readFileSync(file, "utf8").includes(marker))
		.map((file) => path.relative(REPO_ROOT, file).split(path.sep).join("/"));
	expect(
		matches,
		`candidate build has no client chunk containing ${marker}`,
	).not.toHaveLength(0);
	return matches;
}

function deferredChunkPaths(marker: string): string[] {
	const chunks = clientChunksContaining(marker);
	const stats = JSON.parse(readFileSync(ROUTE_STATS, "utf8")) as Array<{
		route: string;
		firstLoadChunkPaths: string[];
	}>;
	for (const route of [
		"/[locale]",
		"/[locale]/market/products",
		"/[locale]/search",
	]) {
		const entry = stats.find((item) => item.route === route);
		expect(entry, `route-bundle-stats is missing ${route}`).toBeDefined();
		for (const chunk of chunks) {
			expect(
				entry?.firstLoadChunkPaths,
				`${chunk} remains in ${route} first load`,
			).not.toContain(chunk);
		}
	}
	return chunks.map((chunk) => `/${chunk.replace(/^\.next\//, "_next/")}`);
}

function trackBrowserJs(page: Page): Set<string> {
	const requested = new Set<string>();
	page.on("request", (request) => {
		const pathname = new URL(request.url()).pathname;
		if (pathname.endsWith(".js")) requested.add(pathname);
	});
	return requested;
}

function requestedAny(requested: Set<string>, candidates: string[]): boolean {
	return candidates.some((candidate) =>
		[...requested].some((pathname) => pathname.endsWith(candidate)),
	);
}

function gatewayData(route: Route, data: unknown) {
	return route.fulfill({
		status: 200,
		contentType: "application/json",
		body: JSON.stringify({ code: 0, data }),
	});
}

function rawData(route: Route, data: unknown) {
	return route.fulfill({
		status: 200,
		contentType: "application/json",
		body: JSON.stringify(data),
	});
}

async function installInteractionFixtures(
	page: Page,
): Promise<FixtureRequests> {
	const requests: FixtureRequests = {
		menuCategories: [],
		menuHashtags: [],
		menuPosts: [],
		menuTrending: [],
		menuBanners: [],
		searchHashtags: [],
		searchSuggest: [],
		searchProducts: [],
	};
	const note = (key: FixtureKey, route: Route) => {
		requests[key].push(route.request().url());
	};

	await mockGuestSession(page);
	await page.route(/\/api\/v1\/public\/categories\/tree(?:\?|$)/, (route) => {
		note("menuCategories", route);
		return gatewayData(route, [{ id: "12", name: "葉菜類", children: [] }]);
	});
	await page.route(/\/api\/v1\/public\/hashtags\/popular(?:\?|$)/, (route) => {
		const limit = new URL(route.request().url()).searchParams.get("limit");
		if (limit === "5") note("menuHashtags", route);
		else if (limit === "12") note("searchHashtags", route);
		else return route.fallback();
		return gatewayData(route, {
			records: [{ hashtag: "當季", count: "3" }],
		});
	});
	await page.route(/\/api\/v1\/public\/posts(?:\?|$)/, (route) => {
		const url = new URL(route.request().url());
		if (url.searchParams.get("page_size") !== "30") return route.fallback();
		note("menuPosts", route);
		return gatewayData(route, {
			page: 1,
			pageSize: 30,
			total: 0,
			records: [],
		});
	});
	await page.route(/\/api\/v1\/public\/tags\/trending(?:\?|$)/, (route) => {
		const limit = new URL(route.request().url()).searchParams.get("limit");
		if (limit !== "5") return route.fallback();
		note("menuTrending", route);
		return gatewayData(route, {
			items: [{ hashtag: "產地", count: "4" }],
		});
	});
	// content 走 Connect（gateway 根目錄的 /{service}/{method}，回應是訊息本身的 JSON）。
	await page.route(
		/\/ContentPublicService\/ListLiveBanners(?:\?|$)/,
		(route) => {
			note("menuBanners", route);
			return rawData(route, {});
		},
	);

	await page.route(/\/api\/public\/search\/suggest(?:\?|$)/, (route) => {
		note("searchSuggest", route);
		return rawData(route, {
			suggestions: [{ keyword: "小白菜", category: "葉菜類" }],
		});
	});
	await page.route(/\/api\/public\/search\/products(?:\?|$)/, (route) => {
		note("searchProducts", route);
		return rawData(route, {
			records: [{ id: "1001", name: "小白菜", price: "45" }],
			total: 1,
		});
	});

	return requests;
}

function homeUrl(baseURL: string | undefined): string {
	return baseURL ? new URL("/", baseURL).toString() : "/";
}

async function openHome(page: Page, baseURL?: string) {
	await page.goto(homeUrl(baseURL), { waitUntil: "domcontentloaded" });
	await expect(page.locator(".initial-loader")).toBeHidden({ timeout: 30_000 });
	await expect(
		page.locator("form[data-home-navbar-search]:visible"),
	).toBeVisible();
}

function navbar(page: Page): Locator {
	return page.locator('nav.site-nav[aria-label="主要導覽"]');
}

function menuTrigger(page: Page, label = "Hawkr 市集"): Locator {
	return navbar(page).getByRole("link", { name: label, exact: true });
}

function searchInput(page: Page): Locator {
	return page.locator(
		'form[data-home-navbar-search]:visible input[aria-label="搜尋商品"]',
	);
}

function allInteractionRequests(requests: FixtureRequests): string[] {
	return Object.values(requests).flat();
}

function regexForPaths(paths: string[]): RegExp {
	const escaped = paths.map((value) =>
		value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"),
	);
	return new RegExp(`(?:${escaped.join("|")})(?:\\?.*)?$`);
}

test.describe("interaction-only navbar resources", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"bounded candidate gate runs once in desktop Chromium; touch cases create explicit cold contexts",
	);

	test("homepage first load excludes menu/search requests and compiled interaction chunks", async ({
		page,
	}) => {
		test.setTimeout(75_000);
		const menuChunks = deferredChunkPaths(INTERACTION_MARKERS.menu);
		const searchChunks = deferredChunkPaths(INTERACTION_MARKERS.search);
		const requestedJs = trackBrowserJs(page);
		const fixtureRequests = await installInteractionFixtures(page);

		await openHome(page);
		// Include the full idle W30, so a delayed timer cannot hide startup work.
		await page.waitForTimeout(30_000);

		expect(allInteractionRequests(fixtureRequests)).toEqual([]);
		expect(requestedAny(requestedJs, menuChunks)).toBe(false);
		expect(requestedAny(requestedJs, searchChunks)).toBe(false);
		await expect(page.locator("[data-mega-menu-content]")).toHaveCount(0);
		await expect(page.getByRole("dialog", { name: "搜尋建議" })).toHaveCount(0);
	});

	test("keyboard focus loads the menu once and Escape restores its trigger", async ({
		page,
	}) => {
		const menuChunks = deferredChunkPaths(INTERACTION_MARKERS.menu);
		const requestedJs = trackBrowserJs(page);
		const fixtureRequests = await installInteractionFixtures(page);
		await openHome(page);
		expect(requestedAny(requestedJs, menuChunks)).toBe(false);

		// 2026-09-10：觸發鈕不再宣告 aria-haspopup（面板不是 menu），改以
		// aria-expanded + aria-controls 表達關係。
		const firstTrigger = navbar(page).locator("a[aria-expanded]").first();
		await navbar(page).getByRole("link", { name: "回到 Hawkr 首頁" }).focus();
		await page.keyboard.press("Tab");
		await expect(firstTrigger).toBeFocused();
		await expect(page.locator("[data-mega-menu-content]")).toBeAttached();
		await expect(
			page.locator('section[aria-label="Hawkr 動態 子選單"] > div'),
		).toBeVisible();
		expect(requestedAny(requestedJs, menuChunks)).toBe(true);
		await expect.poll(() => fixtureRequests.menuCategories.length).toBe(1);
		await expect.poll(() => fixtureRequests.menuHashtags.length).toBe(1);
		await expect.poll(() => fixtureRequests.menuPosts.length).toBe(1);
		await expect.poll(() => fixtureRequests.menuTrending.length).toBe(1);

		await page.keyboard.press("Escape");
		await expect(page.locator("[data-mega-menu-content]")).toHaveCount(0);
		await expect(firstTrigger).toHaveAttribute("aria-expanded", "false");
		await expect(firstTrigger).toBeFocused();
	});

	test("soft route navigation closes an open menu", async ({ page }) => {
		await installInteractionFixtures(page);
		await openHome(page);

		const trigger = menuTrigger(page);
		await trigger.hover();
		await expect(page.locator("[data-mega-menu-content]")).toBeAttached();
		await Promise.all([
			page.waitForURL((url) => url.pathname === "/market/products"),
			trigger.click(),
		]);
		await expect(page.locator("[data-mega-menu-content]")).toHaveCount(0);
	});

	test("a cold touch opens the menu on the first tap without navigating", async ({
		browser,
		baseURL,
	}) => {
		const context = await browser.newContext({
			baseURL,
			viewport: { width: 841, height: 900 },
			hasTouch: true,
			isMobile: true,
			serviceWorkers: "block",
		});
		const touchPage = await context.newPage();
		try {
			const menuChunks = deferredChunkPaths(INTERACTION_MARKERS.menu);
			const requestedJs = trackBrowserJs(touchPage);
			await installInteractionFixtures(touchPage);
			await openHome(touchPage, baseURL);
			expect(requestedAny(requestedJs, menuChunks)).toBe(false);

			await menuTrigger(touchPage).tap();
			await expect(
				touchPage.locator("[data-mega-menu-content]"),
			).toBeAttached();
			expect(new URL(touchPage.url()).pathname).toBe("/");
			expect(requestedAny(requestedJs, menuChunks)).toBe(true);
		} finally {
			await context.close();
		}
	});

	test("open menu stays inside 841, 1024, and 1440 pixel viewports", async ({
		page,
	}) => {
		await installInteractionFixtures(page);
		for (const width of [841, 1024, 1440]) {
			await page.setViewportSize({ width, height: 900 });
			await openHome(page);
			// A full navigation retains the mouse position. Start outside the nav
			// so every viewport exercises an actual pointer entry after hydration.
			await page.mouse.move(0, 899);
			await menuTrigger(page).hover();
			const surface = page.locator(
				'section[aria-label="Hawkr 市集 子選單"] > div',
			);
			await expect(surface).toBeVisible();
			const geometry = await surface.evaluate((element) => {
				const rect = element.getBoundingClientRect();
				return {
					left: rect.left,
					right: rect.right,
					viewport: window.innerWidth,
					documentWidth: document.documentElement.scrollWidth,
				};
			});
			expect
				.soft(geometry.left, `${width}px left edge`)
				.toBeGreaterThanOrEqual(-1);
			expect
				.soft(geometry.right, `${width}px right edge`)
				.toBeLessThanOrEqual(geometry.viewport + 1);
			expect
				.soft(geometry.documentWidth, `${width}px document overflow`)
				.toBeLessThanOrEqual(geometry.viewport + 1);
			await page.keyboard.press("Escape");
		}
	});

	test("typing survives a delayed cold search chunk and IME Enter does not navigate", async ({
		page,
	}) => {
		const searchChunks = deferredChunkPaths(INTERACTION_MARKERS.search);
		const requestedJs = trackBrowserJs(page);
		const fixtureRequests = await installInteractionFixtures(page);
		await openHome(page);
		expect(requestedAny(requestedJs, searchChunks)).toBe(false);

		let releaseChunk = () => {};
		const chunkGate = new Promise<void>((resolve) => {
			releaseChunk = resolve;
		});
		let interceptedChunks = 0;
		await page.route(regexForPaths(searchChunks), async (route) => {
			interceptedChunks += 1;
			await chunkGate;
			await route.continue();
		});

		const input = searchInput(page);
		try {
			await input.click();
			await input.fill("小白菜");
			await expect(input).toHaveValue("小白菜");
			await expect
				.poll(() => interceptedChunks, { timeout: 5_000 })
				.toBeGreaterThan(0);
			await expect(
				page.getByRole("dialog", { name: "搜尋建議" }),
			).toBeVisible();
		} finally {
			releaseChunk();
		}

		await expect(page.locator("[data-home-search-product]")).toContainText(
			"小白菜",
		);
		await expect(input).toHaveValue("小白菜");
		expect(requestedAny(requestedJs, searchChunks)).toBe(true);
		expect(fixtureRequests.searchSuggest).toHaveLength(1);
		expect(fixtureRequests.searchProducts).toHaveLength(1);
		expect(
			new URL(fixtureRequests.searchSuggest[0]).searchParams.get("q"),
		).toBe("小白菜");
		expect(
			new URL(fixtureRequests.searchProducts[0]).searchParams.get("q"),
		).toBe("小白菜");

		const beforeImeEnter = page.url();
		await input.dispatchEvent("compositionstart", { data: "" });
		await input.fill("小白菜心");
		await page.waitForTimeout(350);
		expect(fixtureRequests.searchSuggest).toHaveLength(1);
		expect(fixtureRequests.searchProducts).toHaveLength(1);
		await page.keyboard.press("Enter");
		await page.waitForTimeout(150);
		expect(page.url()).toBe(beforeImeEnter);
		await expect(input).toHaveValue("小白菜心");
		await input.dispatchEvent("compositionend", { data: "小白菜心" });
		await expect.poll(() => fixtureRequests.searchSuggest.length).toBe(2);
		await expect.poll(() => fixtureRequests.searchProducts.length).toBe(2);
		expect(
			new URL(fixtureRequests.searchProducts[1]).searchParams.get("q"),
		).toBe("小白菜心");
		await input.press("ArrowDown");
		await expect(input).not.toBeFocused();
		await page.keyboard.press("Escape");
		await expect(input).toBeFocused();
		await expect(input).toHaveAttribute("aria-expanded", "false");
		await expect(input).toHaveValue("小白菜心");
	});

	test("the first mobile search tap opens the cold experience without a second tap", async ({
		browser,
		baseURL,
	}) => {
		const context = await browser.newContext({
			baseURL,
			viewport: { width: 390, height: 844 },
			hasTouch: true,
			isMobile: true,
			serviceWorkers: "block",
		});
		const touchPage = await context.newPage();
		try {
			const searchChunks = deferredChunkPaths(INTERACTION_MARKERS.search);
			const requestedJs = trackBrowserJs(touchPage);
			const fixtureRequests = await installInteractionFixtures(touchPage);
			await openHome(touchPage, baseURL);
			expect(requestedAny(requestedJs, searchChunks)).toBe(false);

			const input = searchInput(touchPage);
			await input.tap();
			await expect(
				touchPage.getByRole("dialog", { name: "搜尋建議" }),
			).toBeVisible();
			await expect(input).toBeFocused();
			await expect.poll(() => fixtureRequests.searchHashtags.length).toBe(1);
			expect(new URL(touchPage.url()).pathname).toBe("/");
			expect(requestedAny(requestedJs, searchChunks)).toBe(true);
		} finally {
			await context.close();
		}
	});

	for (const kind of ["menu", "search"] as const) {
		test(`${kind} recovers from a failed cold chunk without losing the entry`, async ({
			page,
		}) => {
			await installInteractionFixtures(page);
			const chunks = deferredChunkPaths(INTERACTION_MARKERS[kind]);
			await openHome(page);
			// Exercise the local fallback after the existing one-shot recovery has
			// already been used. Production listeners stay installed and unchanged.
			await page.evaluate(() =>
				sessionStorage.setItem("__chunk_reload_at", String(Date.now())),
			);
			let requests = 0;
			let available = false;
			let documentNavigations = 0;
			page.on("request", (request) => {
				if (
					request.resourceType() === "document" &&
					request.isNavigationRequest() &&
					request.frame() === page.mainFrame()
				)
					documentNavigations += 1;
			});
			await page.route(regexForPaths(chunks), async (route) => {
				requests += 1;
				if (!available) await route.abort("failed");
				else await route.continue();
			});
			if (kind === "menu") {
				await menuTrigger(page).focus();
				const retry = page.getByRole("button", { name: "重新載入選單" });
				await expect(retry).toBeVisible({ timeout: 8_000 });
				available = true;
				await retry.click();
				await expect(page.locator("[data-mega-menu-content]")).toBeAttached();
			} else {
				const input = searchInput(page);
				await input.click();
				await input.fill("小白菜");
				const retry = page.locator("[data-home-search-import-retry]");
				await expect(retry).toBeVisible({ timeout: 8_000 });
				available = true;
				await retry.click();
				await expect(page.locator("[data-home-search-product]")).toBeVisible();
				await expect(input).toHaveValue("小白菜");
				await expect(input).toBeFocused();
			}
			expect(requests).toBeGreaterThanOrEqual(3);
			expect(documentNavigations).toBe(0);
			expect(new URL(page.url()).pathname).toBe("/");
		});
	}

	test("search draft survives the existing one-shot stale-chunk refresh", async ({
		page,
	}) => {
		await installInteractionFixtures(page);
		const chunks = deferredChunkPaths(INTERACTION_MARKERS.search);
		await openHome(page);
		let releaseFailure = () => {};
		const gate = new Promise<void>((resolve) => {
			releaseFailure = resolve;
		});
		let failed = false;
		let reloads = 0;
		page.on("request", (request) => {
			if (
				request.resourceType() === "document" &&
				request.isNavigationRequest() &&
				request.frame() === page.mainFrame()
			)
				reloads += 1;
		});
		await page.route(regexForPaths(chunks), async (route) => {
			if (!failed) {
				failed = true;
				await gate;
				await route.abort("failed");
			} else await route.continue();
		});
		try {
			await searchInput(page).click();
			await searchInput(page).fill("刷新也保留的小白菜");
			releaseFailure();
			await expect.poll(() => reloads).toBe(1);
			await expect(searchInput(page)).toHaveValue("刷新也保留的小白菜");
			await expect(searchInput(page)).toBeFocused();
			await expect(page.locator("[data-home-search-product]")).toBeVisible();
			expect(reloads).toBe(1);
		} finally {
			releaseFailure();
		}
	});
});
