import type { Page } from "@playwright/test";

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
			}
		},
		[THEME_STORAGE_KEY, theme] as const,
	);

	await page.goto(url, { waitUntil: "networkidle", timeout: 45_000 });

	await page.waitForFunction(
		() => document.documentElement.scrollHeight > 1000,
		{ timeout: 20_000, polling: 200 },
	);

	await page.evaluate(() => document.fonts.ready);

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
