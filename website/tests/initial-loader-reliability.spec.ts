import { expect, test } from "@playwright/test";
import { FADE_MS, MIN_SHOWN_MS } from "../components/shared/InitialPageLoader";
import { INITIAL_LOADER_WATCHDOG_MS } from "../lib/loading/initial-loader-watchdog";
import {
	INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
	INITIAL_PUBLIC_CONTENT_READY_CLASS,
} from "../lib/loading/initial-public-content";

const ROUTES = ["/", "/about", "/faq", "/market/products"];

test(
	"public SSR hands off visually before delayed React, then hydrates without replay",
	{
		tag: ["@ui", "@live"],
	},
	async ({ page }, testInfo) => {
		test.setTimeout(60_000);
		const pageErrors: string[] = [];
		page.on("pageerror", (error) => pageErrors.push(error.message));
		if (testInfo.project.name === "mobile") {
			await page.emulateMedia({ reducedMotion: "reduce" });
			await page.addInitScript(() => localStorage.setItem("hk:theme", "dark"));
		}
		const stalledScripts: import("@playwright/test").Route[] = [];
		let released = false;
		await page.route("**/*", async (route) => {
			if (!released && route.request().resourceType() === "script") {
				stalledScripts.push(route);
				return;
			}
			await route.continue();
		});
		try {
			const response = await page.goto("/", { waitUntil: "domcontentloaded" });
			expect(response?.status()).toBe(200);
			await expect(
				page.locator('[data-hk-initial-content-ready="home"]'),
			).toBeAttached();
			await expect(page.locator("html")).toHaveClass(
				new RegExp(INITIAL_PUBLIC_CONTENT_READY_CLASS),
				{ timeout: 1_000 },
			);
			await expect(page.locator(".initial-loader")).toBeHidden({
				timeout: 1_000,
			});
			await expect(page.locator(".page-wrapper")).toBeVisible();
			expect(stalledScripts.length).toBeGreaterThan(0);
			await expect(page.locator("html")).toHaveClass(/initial-loader-active/);
			await expect(
				page.locator('[data-auth-status]:not([data-auth-status="unknown"])'),
			).toHaveCount(0);
			if (testInfo.project.name === "mobile") {
				await expect(page.locator("html")).toHaveAttribute(
					"data-theme",
					"dark",
				);
			}

			released = true;
			await Promise.all(stalledScripts.map((route) => route.continue()));
			await expect(
				page.locator('[data-auth-status]:not([data-auth-status="unknown"])'),
			).toBeAttached();
			await expect(page.locator(".initial-loader")).toHaveCount(0);
			await expect(page.locator("html")).not.toHaveClass(
				/initial-loader-active/,
			);
			await page.waitForLoadState("load", { timeout: 45_000 });
			await expect(page.locator("html")).toHaveAttribute(
				"data-hk-document-settled",
				"true",
			);
			expect(pageErrors).toEqual([]);
		} finally {
			if (!released) {
				await Promise.all(
					stalledScripts.map((route) => route.abort().catch(() => undefined)),
				);
			}
		}
	},
);

test("a saved workspace keeps the original public loading gate", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	await page.addInitScript(() =>
		localStorage.setItem("hkui:auth:active-role", "seller"),
	);
	await page.route("**/*", async (route) => {
		if (route.request().resourceType() === "script") await route.abort();
		else await route.continue();
	});
	await page.goto("/", { waitUntil: "domcontentloaded" });
	await expect(
		page.locator('[data-hk-initial-content-ready="home"]'),
	).toBeAttached();
	await expect(page.locator("html")).not.toHaveClass(
		new RegExp(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS),
	);
	await expect(page.locator("html")).not.toHaveClass(
		new RegExp(INITIAL_PUBLIC_CONTENT_READY_CLASS),
	);
});

async function loaderBlocksPointer(page: import("@playwright/test").Page) {
	return page.evaluate(() => {
		const loader = document.querySelector<HTMLElement>(".initial-loader");
		if (!loader) return false;
		const style = getComputedStyle(loader);
		return (
			style.pointerEvents !== "none" &&
			style.visibility !== "hidden" &&
			Number(style.opacity) > 0
		);
	});
}

/**
 * 上一支測試的每次導航預算是**從 MIN_SHOWN_MS／FADE_MS 推導**的，那讓它不會因為
 * 常數微調而 flake —— 但也讓它**對那兩個常數本身的退化完全免疫**：把 MIN_SHOWN_MS
 * 改成 3000，預算也跟著變成 3610，測試照樣綠。（實測過，確實如此。）
 *
 * 所以那個上限必須由**這裡**獨立守住：splash 依設計佔用指標輸入的總時間，不得
 * 超過一個與常數無關的天花板。1.5 秒是產品面的宣稱 —— 使用者按不到東西超過這個
 * 長度就不是「避免閃爍」而是「卡住」了。
 *
 * 它與 INITIAL_LOADER_WATCHDOG_MS（6s，真正的失效線）是兩件事：watchdog 管的是
 * 「壞掉時多久強制放開」，本上限管的是「正常時最多佔用多久」。
 */
test("the splash's designed hold never exceeds the interaction ceiling", {
	tag: ["@ui"],
}, () => {
	const DESIGNED_HOLD_CEILING_MS = 1_500;
	expect(
		MIN_SHOWN_MS + FADE_MS,
		`splash 依設計佔用指標輸入 ${MIN_SHOWN_MS + FADE_MS}ms（MIN_SHOWN_MS=${MIN_SHOWN_MS} + FADE_MS=${FADE_MS}），` +
			`超過 ${DESIGNED_HOLD_CEILING_MS}ms 的互動天花板。調大這兩個常數之前先確認那是產品決定，` +
			`不是為了讓某支測試變綠。`,
	).toBeLessThanOrEqual(DESIGNED_HOLD_CEILING_MS);
	expect(
		MIN_SHOWN_MS + FADE_MS,
		"設計上的佔用必須遠低於 watchdog 的強制放開線，否則兩者的語意會混淆",
	).toBeLessThan(INITIAL_LOADER_WATCHDOG_MS);
});

test("initial loader never owns pointer input across repeated hard navigations", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	for (let attempt = 0; attempt < 8; attempt += 1) {
		const route = ROUTES[attempt % ROUTES.length];
		await page.goto(`${route}?loader-attempt=${attempt}`, {
			waitUntil: "domcontentloaded",
		});

		const releaseBudgetMs = MIN_SHOWN_MS + FADE_MS + 250;
		await expect(async () => {
			expect(
				await loaderBlocksPointer(page),
				`loader intercepted pointer input on ${route}, attempt ${attempt + 1}`,
			).toBe(false);
		}).toPass({ timeout: releaseBudgetMs, intervals: [50] });
	}
});

test("slow images cannot keep the visual loader open", {
	tag: ["@ui"],
}, async ({ page }) => {
	const stalledImages: import("@playwright/test").Route[] = [];
	await page.route("**/*", async (route) => {
		if (route.request().resourceType() === "image") {
			stalledImages.push(route);
			return;
		}
		await route.continue();
	});

	await page.goto("/about?loader-slow-images=1", {
		waitUntil: "domcontentloaded",
	});
	const domReadyAt = Date.now();
	const visualDeadline = INITIAL_LOADER_WATCHDOG_MS + 1_000;

	await expect(page.locator(".initial-loader")).toBeHidden({
		timeout: visualDeadline,
	});
	expect(Date.now() - domReadyAt).toBeLessThan(visualDeadline);

	await Promise.all(
		stalledImages.map((route) => route.abort().catch(() => undefined)),
	);
});

test("script failure cannot make the visual loader intercept pointer input", {
	tag: ["@ui"],
}, async ({ page }) => {
	await page.route("**/*", async (route) => {
		if (route.request().resourceType() === "script") {
			await route.abort();
			return;
		}
		await route.continue();
	});

	await page.goto("/about?loader-script-failure=1", {
		waitUntil: "domcontentloaded",
	});
	await expect(page.locator(".initial-loader")).toBeAttached();
	expect(await loaderBlocksPointer(page)).toBe(false);
	const fallbackAnimation = await page
		.locator(".initial-loader")
		.evaluate((node) => {
			const style = getComputedStyle(node);
			return { name: style.animationName, delay: style.animationDelay };
		});
	expect(fallbackAnimation).toEqual({
		name: "initial-loader-hard-timeout",
		delay: `${INITIAL_LOADER_WATCHDOG_MS / 1_000}s`,
	});

	await expect(page.locator(".initial-loader")).toBeHidden({
		timeout: INITIAL_LOADER_WATCHDOG_MS + 1_000,
	});
	await expect(page.locator(".page-wrapper")).toBeVisible();
	await expect(
		page.getByText("認識 Hawkr · ABOUT", { exact: true }),
	).toBeVisible();
});

test(
	"a missing page-ready marker releases the splash at the deadline, never a lock",
	{ tag: ["@ui", "@live"] },
	async ({ page }, testInfo) => {
		test.skip(
			testInfo.project.name === "mobile",
			"one browser covers the timeout",
		);
		await page.addInitScript(() => {
			const target = window as Window & {
				__hkReadyMarkerBlocker?: MutationObserver;
			};
			const removeHomeMarker = () => {
				document
					.querySelectorAll('[data-hk-initial-content-ready="home"]')
					.forEach((node) => {
						node.remove();
					});
			};
			target.__hkReadyMarkerBlocker = new MutationObserver(removeHomeMarker);
			target.__hkReadyMarkerBlocker.observe(document, {
				childList: true,
				subtree: true,
			});
		});

		await page.goto("/", { waitUntil: "domcontentloaded" });
		const loader = page.locator(".initial-loader");
		await expect(loader).toBeHidden({
			timeout: INITIAL_LOADER_WATCHDOG_MS + 1_000,
		});
		await expect(page.locator(".page-wrapper")).toBeVisible();
		expect(await loaderBlocksPointer(page)).toBe(false);

		await page.waitForLoadState("load", { timeout: 45_000 });
		await expect(page.locator("html")).toHaveAttribute(
			"data-hk-document-settled",
			"true",
			{ timeout: 3_000 },
		);
	},
);

test(
	"a soft navigation off the gated route releases the splash instead of timing out",
	{ tag: ["@ui", "@live"] },
	async ({ page }, testInfo) => {
		test.skip(
			testInfo.project.name === "mobile",
			"one browser covers the timeout",
		);

		await page.addInitScript(() => {
			const target = window as Window & {
				__hkReadyMarkerBlocker?: MutationObserver;
			};
			const removeHomeMarker = () => {
				document
					.querySelectorAll('[data-hk-initial-content-ready="home"]')
					.forEach((node) => {
						node.remove();
					});
			};
			target.__hkReadyMarkerBlocker = new MutationObserver(removeHomeMarker);
			target.__hkReadyMarkerBlocker.observe(document, {
				childList: true,
				subtree: true,
			});
		});

		await page.goto("/", { waitUntil: "domcontentloaded" });

		await page.waitForFunction(() =>
			document.documentElement.classList.contains("initial-loader-active"),
		);

		await page.evaluate(() => {
			window.history.pushState({}, "", "/about");
		});

		const loader = page.locator(".initial-loader");
		await expect(loader).toBeHidden({ timeout: 3_000 });
		expect(await loaderBlocksPointer(page)).toBe(false);

		await page.waitForTimeout(INITIAL_LOADER_WATCHDOG_MS + 1_000);
		await expect(loader).toBeHidden();
		expect(await loaderBlocksPointer(page)).toBe(false);
	},
);
