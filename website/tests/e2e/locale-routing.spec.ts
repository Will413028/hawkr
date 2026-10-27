import { expect, test } from "@playwright/test";
import { installProviderMocks } from "./helpers/mock-gsi";

/**
 * 語系路由（ADR 2026-10-17 D1）：zh-TW 在 `/…`、en 在 `/en/…`，URL 是唯一的語系來源。
 *
 * 這份 spec 跑在 dev flavor 的建置上（本機與 CI 的 E2E 都是），dev flavor 路由全部支援的語系，
 * 所以 `/en` 在這裡存在；prod flavor 只路由已發佈的語系（`/en` 回 404），那一半由 `lib/intl` 的單元測試
 * 鎖住。登入狀態是 Clerk 的，這裡造不出 session，所以受保護頁只測「被導到哪裡」。
 */
test.describe("locale routing", { tag: ["@ui"] }, () => {
	test.beforeEach(async ({ page }) => {
		await installProviderMocks(page);
	});

	test("zh-TW is the unprefixed URL: <html lang> follows the locale", async ({
		page,
	}) => {
		const response = await page.goto("/faq");
		expect(response?.status()).toBe(200);
		expect(new URL(page.url()).pathname).toBe("/faq");
		await expect(page.locator("html")).toHaveAttribute("lang", "zh-TW");
		await expect(page.locator("link[rel=canonical]")).toHaveAttribute(
			"href",
			/\/faq$/,
		);
	});

	test("/en/<page> is the same page in the en locale: lang, canonical and og:locale follow the URL", async ({
		page,
	}) => {
		const response = await page.goto("/en/faq");
		expect(response?.status()).toBe(200);
		expect(new URL(page.url()).pathname).toBe("/en/faq");
		await expect(page.locator("html")).toHaveAttribute("lang", "en");
		await expect(page.locator("link[rel=canonical]")).toHaveAttribute(
			"href",
			/\/en\/faq$/,
		);
		await expect(page.locator('meta[property="og:locale"]')).toHaveAttribute(
			"content",
			"en_US",
		);
		// the page itself is still the zh-TW text until the English copy exists
		await expect(page.locator("main")).toContainText("常見問題");
	});

	test("the home page is the bare locale: / is zh-TW and /en is en", async ({
		page,
	}) => {
		await page.goto("/");
		await expect(page.locator("html")).toHaveAttribute("lang", "zh-TW");
		await page.goto("/en");
		expect(new URL(page.url()).pathname).toBe("/en");
		await expect(page.locator("html")).toHaveAttribute("lang", "en");
	});

	test("an explicit /zh-TW prefix is redirected to the unprefixed URL", async ({
		request,
	}) => {
		for (const [path, to] of [
			["/zh-TW", "/"],
			["/zh-TW/faq", "/faq"],
			["/zh-TW/account/orders/icon", "/account/orders/icon"],
		] as const) {
			const response = await request.get(path, { maxRedirects: 0 });
			expect([307, 308], path).toContain(response.status());
			expect(
				new URL(response.headers().location ?? "", "http://x").pathname,
			).toBe(to);
		}
	});

	test("the browser's language never routes: no redirect, no cookie, no Link header", async ({
		request,
	}) => {
		for (const path of ["/", "/faq", "/en/faq"]) {
			const response = await request.get(path, {
				maxRedirects: 0,
				headers: { "Accept-Language": "en-US,en;q=0.9" },
			});
			expect(response.status(), path).toBe(200);
			const headers = response.headers();
			expect(headers["set-cookie"], `${path} set-cookie`).toBeUndefined();
			expect(headers.link, `${path} link`).toBeUndefined();
		}
		const home = await (
			await request.get("/", { headers: { "Accept-Language": "en" } })
		).text();
		expect(home).toMatch(/<html[^>]*\blang="zh-TW"/);
	});

	test("the prerendered HTML is computed for the browser's URL: the internal /zh-TW path never leaks into it", async ({
		request,
	}) => {
		for (const path of ["/", "/about", "/faq"]) {
			const html = await (await request.get(path)).text();
			expect(html, `${path} carries the internal locale path`).not.toContain(
				"%2Fzh-TW",
			);
			expect(html, `${path} login link`).toContain(
				`login?redirect=${encodeURIComponent(path)}"`,
			);
		}
	});

	test("a prerendered default-locale page hydrates without a mismatch and keeps the server's links", async ({
		page,
	}) => {
		const problems: string[] = [];
		const hydration =
			/hydrat|react\.dev\/errors\/(?:418|419|422|423|425)|Minified React error #(?:418|419|422|423|425)/i;
		page.on("console", (message) => {
			if (message.type() === "error" && hydration.test(message.text())) {
				problems.push(message.text());
			}
		});
		page.on("pageerror", (error) => {
			if (hydration.test(error.message)) problems.push(error.message);
		});
		for (const path of ["/", "/about"]) {
			await page.goto(path);
			await page.waitForLoadState("networkidle");
			await expect(
				page.locator('a[href^="/login?redirect="]').first(),
			).toHaveAttribute("href", `/login?redirect=${encodeURIComponent(path)}`);
		}
		expect(problems).toEqual([]);
	});

	test("the legacy redirects work under the prefix and keep it", async ({
		request,
	}) => {
		for (const [path, to] of [
			["/products", "/market/products"],
			["/en/products", "/en/market/products"],
			["/en/explore", "/en/feed"],
		] as const) {
			const response = await request.get(path, { maxRedirects: 0 });
			expect(response.status(), path).toBe(308);
			expect(
				new URL(response.headers().location ?? "", "http://x").pathname,
				path,
			).toBe(to);
		}
	});

	test("a guest on a protected page lands on the login of the same locale, with the prefixed way back", async ({
		request,
	}) => {
		for (const [path, login] of [
			["/account/orders", "/login"],
			["/en/account/orders", "/en/login"],
			["/en/seller-center/orders", "/en/login"],
		] as const) {
			const response = await request.get(path, { maxRedirects: 0 });
			expect([302, 307], path).toContain(response.status());
			const to = new URL(response.headers().location ?? "", "http://x");
			expect(to.pathname, path).toBe(login);
			expect(to.searchParams.get("redirect"), path).toBe(path);
		}
	});

	test("another spelling of a protected path meets the same login guard", async ({
		request,
	}) => {
		for (const [path, login, back] of [
			["/%61ccount/orders", "/login", "/%61ccount/orders"],
			["/%65n/account/orders", "/en/login", "/%65n/account/orders"],
		] as const) {
			const response = await request.get(path, { maxRedirects: 0 });
			expect([302, 307, 308], path).toContain(response.status());
			const to = new URL(response.headers().location ?? "", "http://x");
			expect(to.pathname, path).toBe(login);
			expect(to.searchParams.get("redirect"), path).toBe(back);
		}
	});

	test("staff and infrastructure paths are not localized", async ({
		request,
	}) => {
		const health = await request.get("/api/health");
		expect(health.status()).toBe(200);
		// `/en/api/health` is not a route
		expect((await request.get("/en/api/health")).status()).toBe(404);
		// platform files that are not pages
		expect((await request.get("/apple-app-site-association")).status()).toBe(
			200,
		);
		// the delegated workspace has its own zh-TW root layout
		const adm = await request.get("/adm/unavailable");
		expect(adm.status()).toBe(200);
		expect(await adm.text()).toMatch(/<html[^>]*\blang="zh-TW"/);
		expect((await request.get("/en/adm/unavailable")).status()).toBe(404);
	});

	test("a path that only looks like a file is a plain 404 or meets the guard; the files something serves still resolve", async ({
		request,
	}) => {
		for (const path of [
			"/favicon.ico",
			"/probe-a.json",
			"/zzz/products/1.png",
			"/zzz/seller-guide/icon-abc123.svg",
		]) {
			expect((await request.get(path)).status(), path).toBe(404);
		}
		// an asset-looking tail does not take a guarded page out of the guard
		const guarded = await request.get("/account/orders/HK-1.png", {
			maxRedirects: 0,
		});
		expect([302, 307], "guarded").toContain(guarded.status());
		expect(new URL(guarded.headers().location ?? "", "http://x").pathname).toBe(
			"/login",
		);
		// generated root files and public/ files
		for (const path of [
			"/robots.txt",
			"/sitemap.xml",
			"/manifest.webmanifest",
			"/icon.svg",
			"/data/taiwan.geojson",
		]) {
			expect((await request.get(path)).status(), path).toBe(200);
		}
	});

	test("every in-app page link of an English page is an English URL", async ({
		page,
	}) => {
		await page.goto("/en/faq");
		// the footer streams in after the document: wait for it, then read every link
		await expect(
			page.locator('footer a[href="/en/about"]').first(),
		).toBeVisible();
		const hrefs = await page
			.locator('a[href^="/"]')
			.evaluateAll((anchors) =>
				anchors.map((anchor) => anchor.getAttribute("href") ?? ""),
			);
		// Pages only: staff, API and platform paths and files are not localized.
		const notPages =
			/^\/(?:api|adm|_next|\.well-known)(?:\/|$)|\.[A-Za-z0-9]{1,8}$/;
		const pages = hrefs.filter(
			(href) => !href.startsWith("//") && !notPages.test(href.split(/[?#]/)[0]),
		);
		expect(
			pages.filter((href) => !/^\/en(?:[/?#]|$)/.test(href)),
			"links that would drop the reader into the zh-TW site",
		).toEqual([]);
		expect(pages.length, "page links found").toBeGreaterThan(12);
		// the home link is the bare locale, not /
		expect(pages).toContain("/en");
	});

	test("a click on an English page stays in English, and it is a soft navigation", async ({
		page,
	}) => {
		await page.goto("/en/faq");
		await page.evaluate(() => {
			(
				window as unknown as { __softNavigationMarker: number }
			).__softNavigationMarker = 1;
		});
		await page.locator('footer a[href="/en/about"]').first().click();
		await expect(page).toHaveURL(/\/en\/about$/);
		await expect(page.locator("html")).toHaveAttribute("lang", "en");
		expect(
			await page.evaluate(
				() =>
					(window as unknown as { __softNavigationMarker?: number })
						.__softNavigationMarker,
			),
			"the document was reloaded",
		).toBe(1);
	});

	test("the sign-in link of an English page is a hard navigation to the English login", async ({
		page,
	}) => {
		await page.goto("/en/faq");
		const login = page.locator('a[href^="/en/login"]').first();
		await expect(login).toHaveAttribute("data-hk-hard-navigation", "");
		await login.click();
		await expect(page).toHaveURL(/\/en\/login(?:\?|$)/);
		await expect(page.locator("html")).toHaveAttribute("lang", "en");
	});

	test("a search from an English page is an English URL, with or without JavaScript", async ({
		page,
	}) => {
		await page.goto("/en");
		const form = page.locator("form[data-home-navbar-search]").first();
		await expect(form).toHaveAttribute("action", "/en/search");
		await form.locator('input[name="q"]').fill("apple");
		await form.locator('input[name="q"]').press("Enter");
		await expect(page).toHaveURL(/\/en\/search\?q=apple/);
	});

	test("an unknown path is a real 404 with the site chrome, in both locales", async ({
		page,
	}) => {
		for (const path of [
			"/definitely-not-a-page",
			"/en/definitely-not-a-page",
		]) {
			const response = await page.goto(path);
			expect(response?.status(), path).toBe(404);
			await expect(page.locator("html")).toHaveAttribute("lang", /zh-TW|en/);
			await expect(
				page.getByRole("heading", { name: "這頁不在貨架上" }),
			).toBeVisible();
			await expect(page.getByRole("link", { name: "逛逛市集" })).toBeVisible();
		}
	});
});
