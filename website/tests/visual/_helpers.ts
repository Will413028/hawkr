import { expect, type Locator, type Page } from "@playwright/test";

/**
 * Visual-regression harness for the CSS → Tailwind v4 migration.
 *
 * Strategy (see AGENTS.md "CSS 現狀 & 遷移方向"): migrating a CSS class to
 * utilities should be pixel-faithful. We capture a baseline per file BEFORE
 * touching it (`pnpm test:e2e --update-snapshots`), migrate element-by-element,
 * then re-run to pixel-diff. Zero diff = computed styles preserved.
 *
 * Determinism: these pages are Server Components fetching from API_BASE_URL, so
 * Playwright route mocks can't stub their data. Instead baseline + compare run
 * the same session against the same dev backend — the only variable between the
 * two is our edits. Don't reseed backend data mid-file.
 */

/** Mirrors STORAGE_KEY in lib/theme/context.tsx (hardcoded to avoid app imports in the test loader). */
const THEME_STORAGE_KEY = "hk:theme";

export type Theme = "light" | "dark";

/**
 * Navigate with a theme pinned before first paint (THEME_BOOT_SCRIPT reads
 * localStorage synchronously), then settle fonts + lazy images so a full-page
 * screenshot is stable.
 */
export async function gotoThemed(
	page: Page,
	url: string,
	theme: Theme,
): Promise<void> {
	await page.addInitScript(
		([key, value]) => {
			try {
				localStorage.setItem(key, value);
			} catch {
				/* Safari private mode etc. — boot script falls back to light */
			}
		},
		[THEME_STORAGE_KEY, theme] as const,
	);

	// 首頁／/feed／/search／/sellers/:id/posts 的貼文卡片會送動圖原檔（10 MB，
	// card preset 的 312 倍），而它在首頁是 eager 載入 ⇒ 直接擋 `load` 與
	// `networkidle`。2026-08-27 main run 33093420954 實測：含動圖的四個面
	// 26～28.5 秒 settle，不含的 9.2～9.7 秒 —— 差距全落在這一顆上。
	// 預設的 15 秒導航預算因此不夠。放寬而不是把動畫關掉：卡片會動是刻意的
	// 產品行為（GIF 逐幀取景是 2026-08-26 的功能），體積的正解是給它一個有界的
	// 動圖 derivative，追在 website#528。**#528 落地後這裡要跟著調回去。**
	await page.goto(url, { waitUntil: "networkidle", timeout: 45_000 });

	// Server Components stream in behind a route-level Suspense skeleton, and
	// networkidle can fire while that skeleton (viewport height, ~720px) is
	// still up — capturing a stub. Wait until the real content has expanded the
	// document well past the viewport. Every covered route is >1300px tall once
	// loaded, so 1000px cleanly separates content from skeleton; if content
	// genuinely never loads this fails loudly instead of snapshotting the stub.
	await page.waitForFunction(
		() => document.documentElement.scrollHeight > 1000,
		{ timeout: 20_000, polling: 200 },
	);

	await page.evaluate(() => document.fonts.ready);

	// Next <Image> lazy-loads on scroll; walk the page to force every image in,
	// then return to the top so the full-page capture starts from the same place.
	await page.evaluate(async () => {
		await new Promise<void>((resolve) => {
			let y = 0;
			const step = () => {
				window.scrollTo(0, y);
				y += Math.round(window.innerHeight * 0.8);
				if (y < document.body.scrollHeight) {
					setTimeout(step, 60);
				} else {
					window.scrollTo(0, 0);
					setTimeout(resolve, 250);
				}
			};
			step();
		});
	});
	await page.waitForLoadState("networkidle");
}

/**
 * Full-page screenshot assertion. Name gets the project (chromium/mobile) +
 * platform suffix automatically, so light/dark and desktop/mobile never clash.
 * Animation/caret freezing comes from the global `expect.toHaveScreenshot` config.
 *
 * `mask` covers regions whose content rotates on the shared backend (e.g.
 * recommendation rails) so they don't cause false diffs — Playwright overlays
 * them with a solid box in both baseline and comparison.
 */
export async function snap(
	page: Page,
	name: string,
	opts: { mask?: Locator[] } = {},
): Promise<void> {
	await expect(page).toHaveScreenshot(`${name}.png`, {
		fullPage: true,
		mask: opts.mask,
	});
}
