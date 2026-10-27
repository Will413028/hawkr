import { expect, type Locator, type Page, test } from "@playwright/test";

/** Measure only after the dialog has opened and ancestor motion has finished.
 * Fixed descendants inherit a transformed ancestor's coordinate system. Two
 * equal samples can still be a delayed/frozen intermediate animation pose.
 * Inspect ancestors only: decorative animated descendants may animate forever.
 */
async function settledBoundingBox(page: Page, locator: Locator) {
	await page.evaluate(() => document.fonts.ready);
	await expect
		.poll(() =>
			locator.evaluate((element) => {
				for (
					let node: Element | null = element;
					node;
					node = node.parentElement
				) {
					if (
						node.matches("[data-hk-overlay-root]") &&
						node.getAttribute("data-hk-overlay-state") !== "open"
					)
						return false;
					if (
						node
							.getAnimations()
							.some(
								(animation) =>
									animation.playState !== "finished" &&
									animation.playState !== "idle",
							)
					)
						return false;
				}
				return true;
			}),
		)
		.toBe(true);
	return locator.boundingBox();
}

test("geometry waits for a delayed ancestor animation", {
	tag: ["@ui"],
}, async ({ page }) => {
	await page.setContent(`
  <style>
   body { margin: 0; }
   @keyframes enter { from { transform: translateY(10px); } to { transform: translateY(0); } }
   #motion { animation: enter 100ms linear 300ms both; }
   #hint { position: fixed; top: 16px; width: 20px; height: 20px; }
  </style>
  <div id="motion"><div id="hint"></div></div>
 `);
	const box = await settledBoundingBox(page, page.locator("#hint"));
	expect(box?.y).toBe(16);
});

const IOS_BASE =
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148";

const CASES = [
	{
		name: "Threads iOS",
		userAgent: `${IOS_BASE} Barcelona 388.0.0.0.0`,
		corner: "top-right",
		arrow: "right-up",
		instruction: "點右上角「⋯」",
		actionLabel: "使用外部瀏覽器開啟",
	},
	{
		name: "Threads Android",
		userAgent:
			"Mozilla/5.0 (Linux; Android 14; SM-S921B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36 Barcelona 382.0.0.51.85 Android",
		corner: "top-right",
		arrow: "right-up",
		instruction: "點右上角「⋯」",
		actionLabel: "使用外部瀏覽器開啟",
	},
	{
		name: "Instagram iOS",
		userAgent: `${IOS_BASE} Instagram 388.0.0.0.0`,
		corner: "top-right",
		arrow: "right-up",
		instruction: "點右上角「⋯」",
		actionLabel: "在外部瀏覽器開啟",
	},
	{
		name: "Facebook iOS",
		userAgent: `${IOS_BASE} FBIOS/500.0.0.0.0`,
		corner: "top-right",
		arrow: "right-up",
		instruction: "點右上角「⋯」",
		actionLabel: "使用外部瀏覽器開啟",
	},
	{
		name: "Facebook Android",
		userAgent:
			"Mozilla/5.0 (Linux; Android 15; Pixel 9 Build/AP3A.241105.007; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36 Facebook/500.0.0.0.0",
		corner: "top-right",
		arrow: "right-up",
		instruction: "點右上角「⋯」",
		actionLabel: "使用外部瀏覽器開啟",
	},
	{
		name: "Instagram Android",
		userAgent:
			"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/131.0 Mobile Safari/537.36 Instagram 388.0.0.0.0 Android",
		corner: "top-right",
		arrow: "right-up",
		instruction: "點右上角「⋯」",
		actionLabel: "在外部瀏覽器開啟",
	},
	{
		name: "LINE iOS",
		userAgent: `${IOS_BASE} Line/15.10.1`,
		corner: "bottom-right",
		arrow: "right-down",
		instruction: "點右下角「⋮」",
		actionLabel: "在瀏覽器中開啟",
	},
	{
		name: "LINE Android",
		userAgent:
			"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/131.0 Mobile Safari/537.36 Line/15.10.1",
		corner: "bottom-right",
		arrow: "right-down",
		instruction: "點右下角「⋮」",
		actionLabel: "在瀏覽器中開啟",
	},
] as const;

/**
 * This is hermetic: embedded-browser mode deliberately skips GSI bootstrap and
 * turns the Google surface into a local guide trigger. Keeping it in the
 * hermetic tier ensures every dev push guards that trigger against becoming
 * disabled by unrelated SDK loading state.
 */
for (const browserCase of CASES) {
	test.describe(browserCase.name, { tag: ["@ui"] }, () => {
		test.use({
			userAgent: browserCase.userAgent,
			viewport: { width: 390, height: 844 },
			isMobile: true,
			hasTouch: true,
		});

		test("positions the native-browser guide only after Google is selected", async ({
			page,
		}) => {
			await page.goto("/login?webviewGuide=sheet", {
				waitUntil: "domcontentloaded",
			});

			const guide = page.locator('[data-webview-guide="sheet"]');
			const googleTrigger = page.locator(
				'[data-webview-guide-trigger="google"]',
			);
			await expect(googleTrigger).toBeVisible();
			await expect(googleTrigger).toBeEnabled();
			await expect(googleTrigger).toHaveAttribute("aria-expanded", "false");
			await expect(googleTrigger.locator("..")).not.toHaveAttribute(
				"aria-busy",
				"true",
			);
			await expect(guide).toBeHidden();

			await googleTrigger.click();
			await expect(guide).toBeVisible();

			const menuHint = guide.locator('[data-webview-native-menu-hint="true"]');
			await expect(menuHint).toHaveAttribute(
				"data-webview-guide-corner",
				browserCase.corner,
			);
			await expect(menuHint).toHaveAttribute(
				"data-webview-guide-arrow",
				browserCase.arrow,
			);
			await expect(menuHint).toContainText(browserCase.instruction);
			await expect(menuHint).toContainText(`選「${browserCase.actionLabel}」`);
			await expect(menuHint.locator("svg")).toHaveCount(1);

			// 🔴 序列取樣、且等版面定下來——見 `settledBoundingBox` 的檔頭。
			// 三個框不並行量：`Promise.all` 會讓三次量測落在不同的取樣時點上，
			// 而下面的 `intersectionArea` 斷言比較的正是它們之間的相對位置。
			const hintBox = await settledBoundingBox(page, menuHint);
			const spotlightBox = await settledBoundingBox(
				page,
				guide.locator('[data-webview-guide-spotlight="external-browser"]'),
			);
			const closeBox = await settledBoundingBox(
				page,
				guide.getByRole("button", { name: "關閉開啟方式" }),
			);
			expect(hintBox).not.toBeNull();
			expect(spotlightBox).not.toBeNull();
			expect(closeBox).not.toBeNull();
			if (!hintBox || !spotlightBox || !closeBox) return;

			const viewport = page.viewportSize();
			expect(viewport).not.toBeNull();
			if (!viewport) return;

			const hintRight = hintBox.x + hintBox.width;
			const hintBottom = hintBox.y + hintBox.height;
			const intersectionArea = (
				first: typeof hintBox,
				second: typeof hintBox,
			) =>
				Math.max(
					0,
					Math.min(first.x + first.width, second.x + second.width) -
						Math.max(first.x, second.x),
				) *
				Math.max(
					0,
					Math.min(first.y + first.height, second.y + second.height) -
						Math.max(first.y, second.y),
				);

			expect(hintBox.x).toBeGreaterThanOrEqual(-1);
			expect(hintBox.y).toBeGreaterThanOrEqual(-1);
			expect(hintRight).toBeLessThanOrEqual(viewport.width + 1);
			expect(hintBottom).toBeLessThanOrEqual(viewport.height + 1);
			expect(viewport.width - hintRight).toBeGreaterThanOrEqual(0);
			expect(viewport.width - hintRight).toBeLessThanOrEqual(24);
			expect(intersectionArea(hintBox, spotlightBox)).toBe(0);
			expect(intersectionArea(hintBox, closeBox)).toBe(0);

			if (browserCase.corner.startsWith("top")) {
				expect(hintBox.y).toBeLessThanOrEqual(24);
			} else {
				expect(viewport.height - hintBottom).toBeGreaterThanOrEqual(0);
				expect(viewport.height - hintBottom).toBeLessThanOrEqual(24);
			}
		});
	});
}
