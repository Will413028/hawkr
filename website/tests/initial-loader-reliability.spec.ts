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
			// Visual readiness is deliberately NOT permission for analytics/toast
			// code to imperatively mutate the still-unhydrated document.
			await expect(page.locator("html")).toHaveClass(/initial-loader-active/);
			await expect(page.locator('[data-auth-hydrated="true"]')).toHaveCount(0);
			if (testInfo.project.name === "mobile") {
				await expect(page.locator("html")).toHaveAttribute(
					"data-theme",
					"dark",
				);
			}

			released = true;
			await Promise.all(stalledScripts.map((route) => route.continue()));
			await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
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

		// 預算**從元件的常數推導**，不寫死。
		//
		// ⚠️ 這裡原本寫死 500ms，而 `InitialPageLoader` 的 MIN_SHOWN_MS 是 700 ——
		// 預算低於元件自己宣告的最短顯示時間，於是它只有在**暖導航**（hide()
		// 執行時 elapsed 已超過 MIN_SHOWN_MS，remainingMs 歸零）才碰巧會過。
		// 實測 8 次硬導航的釋放時間是 504/255/161/49/4/73/15/568 ms：兩次超出
		// 500ms，而 loader 的行為完全正確 —— 紅的是預算，不是產品。
		//
		// 正確上限是「最短顯示 ＋ 淡出」再加一點執行期餘裕。它仍然遠低於
		// INITIAL_LOADER_WATCHDOG_MS（6s）那條真正的失效線，所以「loader 卡住
		// 不放開指標輸入」這個真缺陷照樣會被抓到。
		// 8 次 × 約 1.3s 上限 ≈ 10s，仍在本測試 30s 預算內。
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

	// Hydration can begin after DOMContentLoaded on a cold, single-worker CI run.
	// The document-level CSS/watchdog contract is the actual hard bound: the
	// splash must disappear by the hard deadline even if React has not mounted,
	// with 1s allowed
	// here for the browser/expect polling scheduler under load.
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

	// The loader and application tree have independent CSS-only escape
	// animations. Exercise the actual handoff, not only the animation metadata,
	// so a future timing/name mismatch cannot leave users on a blank document
	// when the Next bootstrap fails.
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
		// Gate convergence (#338): the deadline hands the viewport to whatever the
		// server rendered instead of flipping into a blocking retry screen. The
		// page tree stays visible and interactive.
		await expect(loader).toBeHidden({
			timeout: INITIAL_LOADER_WATCHDOG_MS + 1_000,
		});
		await expect(page.locator(".page-wrapper")).toBeVisible();
		expect(await loaderBlocksPointer(page)).toBe(false);

		// whenDocumentSettled() (lib/hydration/document-settled.ts) gates every
		// third-party mount — GA4's <Script>, Sonner's toaster — on the same
		// `initial-loader-active` removal `hide()` just performed above. Assert it
		// directly rather than only inferring it from the splash's own visibility,
		// so a future change that decouples the two (e.g. a new escape hatch that
		// hides the splash without releasing this barrier) fails here first.
		// whenDocumentSettled() 的 trySettle() 要求 `readyState === "complete"`
		// **並且** loader 已釋放（lib/hydration/document-settled.ts）。這裡 splash
		// 走的是 watchdog 逾時路徑，會早於 `load` 就釋放，所以剩下的那個條件是
		// `load` —— 而首頁的動圖原檔（10 MB，eager）會把 `load` 拖到 ~28 秒。
		// 先等 `load` 再斷言，而不是把 3 秒窗口膨脹成 30 秒：這裡要守的是「splash
		// 釋放與這道柵欄不會被拆開」，那是幀等級的交接，把窗口放大會讓它失去鑑別力
		// —— 真的被拆開時，30 秒內多半也還是會通過。
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

		// #1819: the splash latches onto the entry route's content marker for the
		// whole document lifetime. A client-side redirect away from that route (in
		// production, SellerWorkspaceLock bouncing a seller off "/" to
		// /seller-center/dashboard) unmounts the page that would have emitted the
		// marker, so the marker can never arrive and the splash used to sit until
		// the deadline — on a page that had loaded perfectly well. Soft navigations
		// belong to PageTransition, not the splash.
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

		// The loader adds this class from its own effect, so it is the precise
		// "this component has hydrated and latched the entry route" signal. Leaving
		// before it lands would let the redirect become the entry route and make the
		// assertion below pass for the wrong reason.
		await page.waitForFunction(() =>
			document.documentElement.classList.contains("initial-loader-active"),
		);

		// App Router syncs usePathname with native history.pushState, which is how a
		// client-side route change reaches the loader.
		await page.evaluate(() => {
			window.history.pushState({}, "", "/about");
		});

		const loader = page.locator(".initial-loader");
		await expect(loader).toBeHidden({ timeout: 3_000 });
		expect(await loaderBlocksPointer(page)).toBe(false);

		// The deadline must stay quiet afterwards too — a released splash that later
		// reappears is the same user-visible bug, just delayed.
		await page.waitForTimeout(INITIAL_LOADER_WATCHDOG_MS + 1_000);
		await expect(loader).toBeHidden();
		expect(await loaderBlocksPointer(page)).toBe(false);
	},
);
