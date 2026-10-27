import { expect, type Page, test } from "@playwright/test";
import { isGetSessionUrl } from "./helpers/mock-gateway";
import { installProviderMocks } from "./helpers/mock-gsi";

// Mirrors playwright.config.ts's PORT derivation — the server this suite
// exercises doesn't always run on 3000 in CI (see PLAYWRIGHT_PORT there).
const LOCAL_ORIGIN = `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`;

/**
 * Reliability smoke suite — "the page must not break".
 *
 * Every route in the matrix is loaded with health monitors attached and must
 * finish with ZERO collected issues:
 *   - uncaught exceptions (pageerror) — always fatal
 *   - console.error output — fatal unless allowlisted as known environment noise
 *   - unexpected same-origin or Hawkr-media HTTP 4xx/5xx responses — fatal
 *   - a blank document (no rendered text) — fatal
 *
 * On top of the route matrix, the core buyer interactions that orchestrate the
 * most client-side state (zoom transitions, feed viewer, theme toggle, search
 * popover, infinite scroll) are exercised end-to-end under the same monitors.
 *
 * The suite runs against the dev server locally (reuseExistingServer) and the
 * production build in CI, on both the chromium and mobile projects.
 */

interface HealthIssue {
	kind: "pageerror" | "console" | "http4xx" | "http5xx" | "requestfailed";
	detail: string;
}

/* Known noise that does not indicate a broken page. Each entry must have a
 * reason; keep this list short — every addition weakens the suite. */
const CONSOLE_ALLOWLIST: RegExp[] = [
	// React DevTools advertisement in dev.
	/Download the React DevTools/i,
	// Chromium also emits this URL-less console line for a failed resource. The
	// response/requestfailed monitors below inspect the actual URL and make every
	// Hawkr CDN/media failure fatal without misclassifying provider noise.
	/Failed to load resource/i,
	// Next.js dev-mode HMR/fast-refresh chatter.
	/\[Fast Refresh\]/i,
	// Spec-mandated browser notice when `upgrade-insecure-requests` ships in
	// a Content-Security-Policy-Report-Only header (next.config.ts CSP rollout
	// phase). Inert in report-only mode; activates once CSP enforces. Until
	// then the warning is harmless and fires on every page load.
	/Content Security Policy directive 'upgrade-insecure-requests' is ignored when delivered in a report-only policy/i,
];

/* Aborted requests are routine (navigation cancels in-flight fetches, image
 * lazy-load races); only report genuine network-level failures. */
const REQUESTFAILED_IGNORE = /net::ERR_ABORTED|cancelled/i;
// The media host the build under test was configured with (NEXT_PUBLIC_MEDIA_BASE_URL).
const HAWKR_MEDIA_HOSTS = new Set(
	[process.env.NEXT_PUBLIC_MEDIA_BASE_URL]
		.filter((value): value is string => Boolean(value?.trim()))
		.map((value) => new URL(value).hostname),
);

test.beforeEach(async ({ page }) => {
	// CI deliberately bakes a fake OAuth client id. Install the provider fixture
	// instead of suppressing GSI console errors, so unexpected provider/config
	// failures remain visible to the route-health gate.
	await installProviderMocks(page);
});

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

function isExpectedHttpError(
	status: number,
	method: string,
	url: string,
): boolean {
	if (status !== 401 || method !== "GET") return false;

	try {
		// Anonymous pages intentionally probe the session (GetSession) during auth
		// hydration. A 401 there means "guest", not a broken page.
		return isGetSessionUrl(url);
	} catch {
		return false;
	}
}

function attachHealthMonitors(page: Page): HealthIssue[] {
	const issues: HealthIssue[] = [];
	page.on("pageerror", (err) => {
		issues.push({ kind: "pageerror", detail: err.stack ?? err.message });
	});
	page.on("console", (msg) => {
		if (msg.type() !== "error") return;
		const text = msg.text();
		if (CONSOLE_ALLOWLIST.some((re) => re.test(text))) return;
		issues.push({ kind: "console", detail: text.slice(0, 500) });
	});
	page.on("response", (res) => {
		const status = res.status();
		const method = res.request().method();
		const url = res.url();
		if (status < 400 || !isMonitoredResource(url)) return;
		if (isExpectedHttpError(status, method, url)) return;

		issues.push({
			kind: status >= 500 ? "http5xx" : "http4xx",
			detail: `${status} ${method} ${url}`,
		});
	});
	page.on("requestfailed", (req) => {
		const failure = req.failure()?.errorText ?? "";
		if (REQUESTFAILED_IGNORE.test(failure)) return;
		if (!isMonitoredResource(req.url())) return;
		issues.push({ kind: "requestfailed", detail: `${failure} ${req.url()}` });
	});
	return issues;
}

/** Navigate, give hydration a beat to surface errors, then assert health. */
async function expectHealthyPage(
	page: Page,
	issues: HealthIssue[],
	path: string,
) {
	// `networkidle` would hang on SSE/EventSource streams, while `load` can be
	// held open by third-party images. The document and hydrated UI are the
	// health boundary; the explicit rendered-text wait below covers that.
	const response = await page.goto(path, { waitUntil: "domcontentloaded" });
	expect(response, `${path} returned no response`).toBeTruthy();
	expect(response?.status() ?? 0, `${path} document status`).toBeLessThan(400);

	// Wait for real rendered text instead of a fixed beat: pages that stream
	// behind Suspense after live gateway fetches (e.g. /search) can take >1s to flush content under CI load, and
	// their fallback is blank — a fixed 1200ms wait raced that flush and read an
	// empty body. Poll up to a generous ceiling; on timeout the assertion below
	// still fails with the clear "rendered no visible text" message.
	await page
		.waitForFunction(
			() => (document.body?.innerText.trim().length ?? 0) > 20,
			null,
			{ timeout: 15_000 },
		)
		.catch(() => {});
	// Let any post-content hydration errors surface for the issues check below.
	await page.waitForTimeout(300);

	// The page must have actually rendered something.
	const textLength = await page.evaluate(
		() => document.body?.innerText.trim().length ?? 0,
	);
	expect(textLength, `${path} rendered no visible text`).toBeGreaterThan(20);

	expect(
		issues,
		`${path} health issues:\n${issues.map((i) => `[${i.kind}] ${i.detail}`).join("\n")}`,
	).toEqual([]);
}

/* ===== Route matrix ===== */

const STATIC_ROUTES = [
	"/",
	"/feed",
	"/market/products",
	"/seasonal",
	"/topics",
	"/explore",
	"/about",
	"/faq",
	"/terms",
	"/privacy",
	"/return-policy",
	"/contact",
	"/education",
	"/farmers",
	"/user-policy",
	"/seller-terms",
	"/login",
	"/cart",
];

test.describe("route health", { tag: ["@ui"] }, () => {
	for (const route of STATIC_ROUTES) {
		test(`loads without breaking: ${route}`, { tag: ["@live"] }, async ({
			page,
		}) => {
			const issues = attachHealthMonitors(page);
			await expectHealthyPage(page, issues, route);
		});
	}

	test("loads without breaking: product detail (dynamic)", {
		tag: ["@live"],
	}, async ({ page, request }) => {
		const id = await firstProductId(request);
		test.skip(
			id === null,
			"public catalog legitimately empty (pre-launch prod)",
		);
		if (id === null) return;
		const issues = attachHealthMonitors(page);
		await expectHealthyPage(page, issues, `/products/${id}`);
		// A product page that renders the error/empty state still passes the
		// generic checks — pin the purchase box as proof of real content.
		await expect(page.getByText("數量").first()).toBeVisible();
	});

	test("loads without breaking: seller storefront (dynamic)", {
		tag: ["@live"],
	}, async ({ page, request }) => {
		const sellerId = await firstSellerId(request);
		test.skip(
			sellerId === null,
			"no public seller fixture available (pre-launch prod)",
		);
		if (sellerId === null) return;
		const issues = attachHealthMonitors(page);
		await expectHealthyPage(page, issues, `/sellers/${sellerId}`);
	});

	test("unknown route renders a graceful 404", async ({ page }) => {
		const issues = attachHealthMonitors(page);
		const response = await page.goto("/this-route-does-not-exist-xyz", {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBe(404);

		// The branded splash intentionally owns at least the first 700ms and then
		// fades. A fixed 600ms read races that contract and sees zero *visible*
		// body text even though the complete 404 is already mounted underneath.
		// Wait for the route's terminal marker and the loader's explicit handoff
		// instead of weakening either first-paint ownership or this health gate.
		const notFound = page.locator('[data-hk-route-terminal-ready="not-found"]');
		await expect(notFound).toBeAttached();
		await expect(page.locator("html")).not.toHaveClass(
			/\binitial-loader-active\b/,
		);
		await expect(
			page.getByRole("heading", { name: "這頁不在貨架上" }),
		).toBeVisible();
		const textLength = await page.evaluate(
			() => document.body.innerText.trim().length,
		);
		expect(textLength, "404 page rendered no visible text").toBeGreaterThan(10);
		expect(
			issues.filter((i) => i.kind === "pageerror"),
			"404 page threw uncaught exceptions",
		).toEqual([]);
	});
});

test.describe("PPR stream integrity", { tag: ["@ui", "@live"] }, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"one raw-document check is sufficient",
	);

	test("cached main-tab documents never reuse a Fizz segment id", async ({
		request,
	}) => {
		for (const route of ["/market/products", "/search"]) {
			// The first request fills Cache Components; the second is the path
			// that previously emitted both navbar and route content as `S:2`.
			for (let attempt = 1; attempt <= 2; attempt += 1) {
				const response = await request.get(route, {
					headers: { accept: "text/html" },
				});
				expect(
					response.ok(),
					`${route} document attempt ${attempt} failed`,
				).toBe(true);
				const html = await response.text();
				const counts = new Map<string, number>();
				for (const match of html.matchAll(/\bid="([BPS]:[^"]+)"/g)) {
					const id = match[1];
					counts.set(id, (counts.get(id) ?? 0) + 1);
				}
				const duplicates = [...counts.entries()]
					.filter(([, count]) => count > 1)
					.map(([id, count]) => `${id}×${count}`);
				expect(
					duplicates,
					`${route} document attempt ${attempt} reused a PPR/Fizz id`,
				).toEqual([]);
			}
		}
	});
});

/* ===== Core interaction flows (desktop chromium only — they drive the
 * desktop navbar / hover surfaces; the mobile project still covers every
 * route above) ===== */

test.describe("interaction flows", { tag: ["@ui", "@live"] }, () => {
	// A first-attempt route race already reached a user. Do not let a clean retry
	// erase quick-view, navigation, or hydration failures from the CI result.
	test.describe.configure({ retries: 0 });

	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"desktop chromium only",
	);

	test("market → product detail → back (zoom transitions)", async ({
		page,
		request,
	}) => {
		// The flow clicks a real catalog card; with a legitimately empty public
		// catalog (pre-launch prod) there is no card to click. Probe the same
		// public endpoint the grid renders from and skip deterministically —
		// the interaction contract resumes as soon as a product exists.
		const probeId = await firstProductId(request);
		test.skip(
			probeId === null,
			"public catalog legitimately empty (pre-launch prod)",
		);
		if (probeId === null) return;
		const issues = attachHealthMonitors(page);

		await page.goto("/market/products", { waitUntil: "domcontentloaded" });
		// Do not click a server-rendered anchor before React owns it: that would
		// exercise a native document navigation instead of the intercepted
		// product quick-view route.
		await expect(
			page.locator('[data-hk-market-products-hydrated="true"]'),
		).toBeAttached();
		const productCard = page
			.locator("article.market-product-card")
			.filter({
				has: page.locator("a[data-market-product-primary-link]"),
			})
			.first();
		const card = productCard
			.locator("a[data-market-product-primary-link]")
			.first();
		await expect(card).toBeVisible();
		const productName = (await productCard.locator("h2").innerText()).trim();
		await expect(card).toHaveAccessibleName(`查看商品：${productName}`);
		const hitMap = await productCard.evaluate((element) => {
			const primary = element.querySelector<HTMLAnchorElement>(
				"a[data-market-product-primary-link]",
			);
			const hitsPrimary = (selector: string) => {
				const candidate = element.querySelector<HTMLElement>(selector);
				if (!candidate || !primary) return null;
				candidate.scrollIntoView({ block: "center", inline: "center" });
				const rect = candidate.getBoundingClientRect();
				const hit = document.elementFromPoint(
					rect.left + rect.width / 2,
					rect.top + rect.height / 2,
				);
				return hit?.closest("a[data-market-product-primary-link]") === primary;
			};
			return {
				productAreas: [
					hitsPrimary(".market-product-card__media"),
					hitsPrimary("h2"),
					hitsPrimary("small"),
					hitsPrimary(".market-product-card__price-row"),
				],
				controls: [
					hitsPrimary('button[aria-label^="收藏"]'),
					hitsPrimary('button[aria-label^="取消收藏"]'),
					hitsPrimary('a[href^="/sellers/"]'),
					hitsPrimary("fieldset.market-product-card__qty"),
					hitsPrimary(".market-product-card__cart-btn"),
				].filter((value) => value !== null),
			};
		});
		expect(
			hitMap.productAreas,
			"media/title/rating/price must all resolve to the product link",
		).toEqual([true, true, true, true]);
		expect(
			hitMap.controls.length,
			"the card should expose independent controls",
		).toBeGreaterThan(0);
		expect(
			hitMap.controls,
			"favorite/seller/quantity/cart controls must stay independent",
		).not.toContain(true);
		// 數量預設 0 ⇒ 加入購物車按鈕預設就是 disabled。它必須「看起來也按不動」，
		// 否則使用者只會覺得按鈕壞了（web#531）。
		//
		// 這條斷言守的是 `styles/foundation/02-reset-layout.css` 的
		// `button:where(:not(:disabled)) { cursor: pointer }`。那條規則是 UNLAYERED，
		// 而 unlayered 宣告會贏過任何 @layer —— 所以只要有人把它改回裸的
		// `button { cursor: pointer }`，全站 40 個 `disabled:cursor-not-allowed`
		// （它們編譯進 @layer utilities）會同時靜默失效，而**不會有任何編譯或型別錯誤**。
		// 這裡是唯一會轉紅的地方。
		const cartButtonAffordance = await productCard.evaluate((element) => {
			const button = element.querySelector<HTMLButtonElement>(
				".market-product-card__cart-btn",
			);
			if (!button) return null;
			return {
				disabled: button.disabled,
				cursor: getComputedStyle(button).cursor,
			};
		});
		expect(
			cartButtonAffordance,
			"the market card must expose a cart button",
		).not.toBeNull();
		expect(
			cartButtonAffordance?.disabled,
			"quantity defaults to 0, so the cart button starts disabled",
		).toBe(true);
		expect(
			cartButtonAffordance?.cursor,
			"a disabled cart button must not render a pointer cursor",
		).toBe("not-allowed");
		const documentNavigations: string[] = [];
		const productPrefetchesBeforeClick: string[] = [];
		let productClickStarted = false;
		const productPath = new URL(
			(await card.getAttribute("href")) ?? "/products/__missing__",
			page.url(),
		).pathname;
		page.on("request", (request) => {
			if (
				request.isNavigationRequest() &&
				request.resourceType() === "document" &&
				request.frame() === page.mainFrame()
			) {
				documentNavigations.push(request.url());
			}
			const url = new URL(request.url());
			if (
				!productClickStarted &&
				url.pathname === productPath &&
				url.searchParams.has("_rsc")
			) {
				productPrefetchesBeforeClick.push(request.url());
			}
		});
		// Product quick-view must own a single Flight request. A manual hover
		// prefetch overlapping the intercepted navigation can strand the real RSC
		// stream after its 200 headers, leaving the URL changed with no dialog.
		await card.hover();
		await page.waitForTimeout(250);
		expect(
			productPrefetchesBeforeClick,
			"product hover must not start an overlapping RSC prefetch",
		).toEqual([]);
		// The quick-view route intentionally leaves remote product media loading
		// in the background. Do not let Playwright's action auto-wait turn that
		// into a false timeout; the URL + dialog assertions below are the real
		// navigation contract and still fail if this regresses to a hard page.
		productClickStarted = true;
		await card.click({
			noWaitAfter: true,
			position: { x: 20, y: 20 },
		});
		await expect(page).toHaveURL(/\/products\/[^/?#]+$/);
		const productDialog = page.getByRole("dialog", { name: "商品詳情" });
		await expect(productDialog).toBeVisible();
		await expect(
			productDialog.getByRole("button", { name: "關閉商品詳情" }),
		).toHaveCount(0);
		const back = productDialog.getByRole("button", { name: "返回" });
		await expect(back).toBeVisible();
		const visibleReadyMarker = productDialog
			.locator('[data-hk-page-content-ready="product-detail"]:visible')
			.first();
		await expect(visibleReadyMarker).toBeVisible();
		await expect(visibleReadyMarker).toHaveAttribute(
			"data-hk-product-detail-id",
			decodeURIComponent(productPath.split("/").at(-1) ?? ""),
		);
		await expect
			.poll(() =>
				page.evaluate(() => ({
					focusInsideModal:
						document.activeElement?.closest(".product-modal") !== null,
					backgroundInert:
						document.querySelector<HTMLElement>(".page-wrapper")?.inert ??
						false,
				})),
			)
			.toEqual({ focusInsideModal: true, backgroundInert: true });
		await productDialog.focus();
		await page.keyboard.press("Shift+Tab");
		await expect
			.poll(() =>
				page.evaluate(
					() => document.activeElement?.closest(".product-modal") !== null,
				),
			)
			.toBe(true);
		await productDialog.focus();
		await page.keyboard.press("Tab");
		await expect(back).toBeFocused();
		// Let the zoom-in transition fully settle (300ms tween + buffer).
		await page.waitForTimeout(900);

		await back.click();
		await expect(page).toHaveURL(/\/market\/products(?:[?#]|$)/);
		await expect(productDialog).toHaveCount(0);
		await expect(card).toBeFocused();
		// Zoom-out + enter animation settle.
		await page.waitForTimeout(900);

		// Scroll must be unlocked after the transition chain.
		const overflow = await page.evaluate(
			() => document.body.style.overflow || "(empty)",
		);
		expect(overflow, "body scroll still locked after transitions").not.toBe(
			"hidden",
		);
		const position = await page.evaluate(
			() => document.body.style.position || "(empty)",
		);
		expect(position, "body position still locked after product modal").not.toBe(
			"fixed",
		);
		expect(
			issues,
			issues.map((i) => `[${i.kind}] ${i.detail}`).join("\n"),
		).toEqual([]);
		expect(
			documentNavigations,
			"product quick-view and back must stay inside the App Router",
		).toEqual([]);
	});

	test("hard market hydration restores the list snapshot before reveal", async ({
		page,
	}) => {
		const pageErrors: string[] = [];
		page.on("pageerror", (error) => {
			pageErrors.push(error.stack ?? error.message);
		});
		await page.addInitScript(() => {
			const products = Array.from({ length: 18 }, (_, index) => ({
				id: `snapshot-${index}`,
				name: `Snapshot restore ${index + 1}`,
				image: "data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=",
				sellerName: "Snapshot seller",
				origin: "Taiwan",
				price: "NT$ 100",
				isOnSale: false,
				hashtags: [],
				unit: "份",
				rating: null,
				reviews: 0,
				href: `/products/snapshot-${index}`,
				priceValue: 100,
				createdAt: index,
				salesCount: 0,
				isFavorited: false,
			}));
			window.sessionStorage.setItem(
				"hk-market-list-snapshot",
				JSON.stringify({
					key: "/market/products",
					products,
					page: 3,
					done: true,
					scrollY: 720,
					savedAt: Date.now(),
				}),
			);
			(
				window as typeof window & { __hkMarketRestoreEvents?: number }
			).__hkMarketRestoreEvents = 0;
			window.addEventListener("hk:market-list-restored", () => {
				const target = window as typeof window & {
					__hkMarketRestoreEvents?: number;
				};
				target.__hkMarketRestoreEvents =
					(target.__hkMarketRestoreEvents ?? 0) + 1;
			});
		});

		await page.goto("/market/products", { waitUntil: "domcontentloaded" });
		await expect(
			page.locator('[data-hk-market-products-hydrated="true"]'),
		).toBeAttached();
		await expect(
			page.getByRole("heading", { name: "Snapshot restore 18" }),
		).toBeVisible();
		await expect
			.poll(() =>
				page.evaluate(
					() =>
						(
							window as typeof window & {
								__hkMarketRestoreEvents?: number;
							}
						).__hkMarketRestoreEvents ?? 0,
				),
			)
			.toBe(1);
		expect(await page.evaluate(() => window.scrollY)).toBeGreaterThanOrEqual(
			718,
		);
		expect(
			await page.evaluate(() =>
				window.sessionStorage.getItem("hk-market-list-snapshot"),
			),
		).toBeNull();
		expect(pageErrors).toEqual([]);
	});

	test("feed viewer opens and closes cleanly", async ({ page }) => {
		const issues = attachHealthMonitors(page);
		await page.goto("/feed", { waitUntil: "domcontentloaded" });
		await page.waitForTimeout(800);

		const card = page.locator(".hkui-feed-card").first();
		test.skip(!(await card.count()), "no feed posts available");
		await card.click();
		const dialog = page.getByRole("dialog", { name: "貼文詳情" });
		await expect(dialog).toBeVisible();
		// Open zoom settle.
		await page.waitForTimeout(700);

		await page.keyboard.press("Escape");
		await expect(dialog).toHaveCount(0);
		await page.waitForTimeout(700);

		const overflow = await page.evaluate(
			() => document.body.style.overflow || "(empty)",
		);
		expect(overflow, "body scroll still locked after viewer close").not.toBe(
			"hidden",
		);
		expect(
			issues,
			issues.map((i) => `[${i.kind}] ${i.detail}`).join("\n"),
		).toEqual([]);
	});

	test("theme toggle round-trips without breaking", async ({ page }) => {
		const issues = attachHealthMonitors(page);
		await page.goto("/feed", { waitUntil: "domcontentloaded" });
		await page.waitForTimeout(600);

		// ThemeToggle 是 APG toggle：固定名稱「主題切換」+ aria-pressed 表狀態
		// （2026-09-10 前是「切換成夜間模式 / 切換成日間模式」動作名）。
		const toggle = page.locator('button[aria-label="主題切換"]').first();
		test.skip(!(await toggle.count()), "theme toggle not found in navbar");

		await toggle.click();
		await page.waitForTimeout(400);
		const dark = await page.evaluate(
			() => document.documentElement.getAttribute("data-theme") === "dark",
		);
		expect(dark, "data-theme did not flip to dark").toBe(true);

		await toggle.click();
		await page.waitForTimeout(400);
		expect(
			issues,
			issues.map((i) => `[${i.kind}] ${i.detail}`).join("\n"),
		).toEqual([]);
	});

	test("feed infinite scroll loads more without breaking", async ({ page }) => {
		const issues = attachHealthMonitors(page);
		await page.goto("/feed", { waitUntil: "domcontentloaded" });
		await page.waitForTimeout(800);
		const before = await page.locator(".hkui-feed-card").count();
		test.skip(before === 0, "no feed posts available");

		for (let i = 0; i < 3; i++) {
			await page.mouse.wheel(0, 2400);
			await page.waitForTimeout(700);
		}
		const after = await page.locator(".hkui-feed-card").count();
		// Either more posts streamed in or the list ended — both are healthy;
		// the assertion that matters is zero collected issues.
		expect(after).toBeGreaterThanOrEqual(before);
		expect(
			issues,
			issues.map((i) => `[${i.kind}] ${i.detail}`).join("\n"),
		).toEqual([]);
	});
});

/* ===== Dynamic-id discovery helpers ===== */

// Returns a real product id, or `null` when the public catalog is legitimately
// empty (a pre-launch prod backend serves 0 products). A present record that
// omits its id, or a 5xx/network fault, still fails.
async function firstProductId(
	request: import("@playwright/test").APIRequestContext,
): Promise<string | null> {
	let lastFailure = "request was not attempted";
	for (let attempt = 1; attempt <= 3; attempt++) {
		try {
			const res = await request.get(
				`${LOCAL_ORIGIN}/api/v1/public/products?page=1&pageSize=1`,
				{ failOnStatusCode: false, timeout: 15_000 },
			);
			if (res.status() >= 500) {
				lastFailure = `${res.status()} products fixture`;
				continue;
			}
			if (!res.ok()) throw new Error(`${res.status()} products fixture`);
			const body = await res.json();
			const records = body?.data?.records ?? body?.data?.list;
			// Empty catalog is deterministic and legitimate — signal skip.
			if (Array.isArray(records) && records.length === 0) return null;
			const rec = records?.[0];
			if (rec?.id) return String(rec.id);
			lastFailure = "products fixture record present but returned no id";
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}
	throw new Error(
		`dynamic product reliability fixture unavailable after 3 attempts: ${lastFailure}`,
	);
}

// Returns a real seller id derived from public posts, or `null` when there are
// no public posts to derive one from (a pre-launch prod backend). Posts present
// but carrying no seller id, or a 5xx/network fault, still fails.
async function firstSellerId(
	request: import("@playwright/test").APIRequestContext,
): Promise<string | null> {
	let lastFailure = "request was not attempted";
	for (let attempt = 1; attempt <= 3; attempt++) {
		try {
			const res = await request.get(
				`${LOCAL_ORIGIN}/api/v1/public/posts?page=1&pageSize=10`,
				{ failOnStatusCode: false, timeout: 15_000 },
			);
			if (res.status() >= 500) {
				lastFailure = `${res.status()} seller fixture`;
				continue;
			}
			if (!res.ok()) throw new Error(`${res.status()} seller fixture`);
			const body = await res.json();
			const recs: Array<Record<string, unknown>> = body?.data?.records ?? [];
			// No public posts is deterministic and legitimate — signal skip.
			if (recs.length === 0) return null;
			for (const rec of recs) {
				// camelCase, matching what `/public/posts` actually emits (`sellerId`
				// — verified against test-api). The retired `seller_id` /
				// `seller_account_id` read as undefined for every record, so the loop
				// fell through all ten and the helper threw "returned no seller id":
				// a contract rename reported as a missing fixture.
				const id = rec.sellerId;
				if (id) return String(id);
			}
			lastFailure = "seller fixture returned no seller id";
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}
	throw new Error(
		`dynamic seller reliability fixture unavailable after 3 attempts: ${lastFailure}`,
	);
}
