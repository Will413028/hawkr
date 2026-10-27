import { expect, type Page, test } from "@playwright/test";
import type { Theme } from "./_helpers";

/**
 * Computed-style gate for the MobileBottomNav CSS → Tailwind migration.
 *
 * Design: removed from the repo; retrieve with
 * `docs/superpowers/specs/2026-05-28-mobile-bottom-nav-tailwind-migration-design.md`（私人封存）
 *
 * The migration deletes every `mobile-bottom-nav*` rule from THREE files
 * (prd-v2-commerce.css base+@media, navigation-final.css icon block,
 * 01-tokens-theme.css dark overrides) and retains no hook class, so there is
 * no unlayered-cascade risk. This gate pins the INTENDED final values:
 *   - sizes / plain beige background / white glass lens: shared with avatar fit
 *   - semantic colours (border / item text / active): normalized to hk-*
 *     tokens, dark-aware (changed on purpose).
 * Run against the showcase BEFORE migration it is RED (dark active text is the
 * one-off #ffeec0, dark border/muted alphas differ, active bg uses the legacy
 * green/gold alphas); after migration it is GREEN.
 *
 * 顯示條件是元件上的 `max-[840px]:grid`＝`not (min-width: 840px)`，**不含** 840，
 * 所以本檔只在 <840 的 viewport 有意義（`--project=mobile`，Pixel 5 / 393px）。
 * 它由 components/shared/RootDocument.tsx 全域渲染，任何路由都行；用 "/" 是因為那會讓「首頁」帶
 * `aria-current="page"`，給我們一個活的 active item 可以斷言。
 *
 * Locators are semantic (role + aria-current) so they survive the BEM → utility
 * swap unchanged: the gate is the same selector pre- and post-migration.
 *
 * ── 為什麼不再 `test.skip(!!process.env.CI)`（web#386）─────────────────────
 * 原本的跳過理由是「computed-style gate needs a running app server」——那不成立：
 * playwright.config.ts 的 `webServer` 在 CI 會 `pnpm start` 起一台真的 server，
 * 同資料夾的其他 spec 都靠它。於是這支從沒在 CI 跑過，也就沒人發現它早就紅了
 * （斷言 `radius: 12px`，元件實際是 `rounded-[16px]`）。現在讓它真的跑：full E2E
 * （main push / PR 進 main）會跑到，dev push 的 chromium smoke 子集不含本檔。
 *
 * 連帶把 `gotoThemed` 換成本檔自己的輕量版：那個 helper 是**截圖**用的，會滾完
 * 整頁強制載入 lazy image 再回頂端（在 dev server 上實測會吃掉 >30s 而 timeout）。
 * 本檔只讀 computed style，不需要那些穩定化步驟。
 */

const THEMES: Theme[] = ["light", "dark"];

/** Mirrors STORAGE_KEY in lib/theme/context.tsx（與 ./_helpers 同一個字面值，
 *  刻意不從 app 匯入：測試 loader 不載入 app 模組）。 */
const THEME_STORAGE_KEY = "hk:theme";

/**
 * 在首次繪製前把主題釘住（THEME_BOOT_SCRIPT 同步讀 localStorage），然後只等字型
 * 就緒——computed style 不需要等 lazy image。
 */
async function gotoWithTheme(
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
	await page.goto(url, { waitUntil: "domcontentloaded" });
	await page.evaluate(() => document.fonts.ready);
}

/** 元件 `max-[840px]:grid` 的斷點；viewport ≥ 此值時導覽列 display:none。
 *  與 tests/mobile-bottom-nav-geometry.spec.ts 同一個判準（寫成 `> 840` 會讓
 *  正好 840px 不 skip 而直接失敗——那正是本檔原本的 bug）。 */
const BOTTOM_NAV_BREAKPOINT = 840;

// Expected resolved values per theme (Chromium rgb()/rgba() comma format).
const EXPECT = {
	light: {
		navBg: "rgb(245, 241, 232)", // hk-surface-muted
		navBorder: "rgba(51, 76, 15, 0.14)", // hk-border-soft
		itemColor: "rgb(114, 121, 110)", // hk-muted (#72796e)
		activeColor: "rgb(16, 16, 16)",
		lensBg: "rgba(255, 255, 255, 0.72)",
		lensBorder: "rgba(255, 255, 255, 0.82)",
	},
	dark: {
		navBg: "rgba(38, 67, 52, 0.62)", // dark hk-surface-muted
		navBorder: "rgba(245, 222, 165, 0.12)", // dark hk-border-soft
		itemColor: "rgba(223, 233, 217, 0.56)", // hk-muted
		activeColor: "rgb(245, 245, 245)",
		lensBg: "rgba(255, 255, 255, 0.14)",
		lensBorder: "rgba(255, 255, 255, 0.18)",
	},
} as const;

// Theme-agnostic faithful values. The shadow is asserted via `toContain` because
// Tailwind v4's box-shadow utility prepends four transparent placeholder layers
// (ring / inset / drop / etc.) before the authored value — they render as
// nothing but show up in the computed string. Pre-migration CSS sets only the
// single layer, so this substring check stays accurate either way.
const LIGHT_SHADOW = "rgba(255, 255, 255, 0.72) 0px 1px 0px 0px inset";

test.describe("MobileBottomNav CSS → Tailwind — computed-style gate", {
	tag: ["@ui", "@live"],
}, () => {
	for (const theme of THEMES) {
		test(`structure + tokens (${theme})`, async ({ page, viewport }) => {
			// Bottom nav is display:none at ≥840px — only meaningful on a phone vp.
			test.skip(
				!viewport || viewport.width >= BOTTOM_NAV_BREAKPOINT,
				`mobile-only：底部導覽列在 ≥${BOTTOM_NAV_BREAKPOINT}px 是 display:none`,
			);

			await gotoWithTheme(page, "/", theme);

			const nav = page.getByRole("navigation", { name: "手機底部導覽" });
			await expect(nav).toBeVisible();

			const links = nav.getByRole("link");
			await expect(links).toHaveCount(5);

			// "/" route → "首頁" (href="/") is the active item.
			const activeLink = nav.locator('a[aria-current="page"]');
			await expect(activeLink).toHaveCount(1);
			const inactiveLink = nav.locator('a:not([aria-current="page"])').first();
			await expect(inactiveLink).toBeVisible();

			const exp = EXPECT[theme];

			// --- nav container: faithful sizes/glass + normalized border ---
			const cont = await nav.evaluate((el) => {
				const s = getComputedStyle(el);
				return {
					display: s.display,
					position: s.position,
					bottom: s.bottom,
					left: s.left,
					right: s.right,
					zIndex: s.zIndex,
					tracks: s.gridTemplateColumns.split(" ").length,
					gap: s.columnGap,
					pad: s.paddingTop,
					radius: s.borderTopLeftRadius,
					bw: s.borderTopWidth,
					border: s.borderTopColor,
					bg: s.backgroundColor,
					backgroundImage: s.backgroundImage,
					shadow: s.boxShadow,
					backdrop: s.backdropFilter,
				};
			});
			expect(cont.display).toBe("grid");
			expect(cont.position).toBe("fixed");
			expect(cont.bottom).toBe("10px");
			expect(cont.left).toBe("10px");
			expect(cont.right).toBe("10px");
			expect(cont.zIndex).toBe("200");
			expect(cont.tracks).toBe(5);
			expect(cont.gap).toBe("2px");
			expect(cont.pad).toBe("8px");
			expect(cont.radius).toBe("26px");
			expect(cont.bw).toBe("1px");
			expect(cont.border).toBe(exp.navBorder);
			expect(cont.bg).toBe(exp.navBg);
			expect(cont.backgroundImage).toBe("none");
			expect(cont.backdrop).toBe("none");
			if (theme === "light") expect(cont.shadow).toContain(LIGHT_SHADOW);

			// --- inactive item: faithful sizes + normalized muted text ---
			const item = await inactiveLink.evaluate((el) => {
				const s = getComputedStyle(el);
				return {
					display: s.display,
					alignItems: s.alignItems,
					justifyItems: s.justifyItems,
					gap: s.columnGap,
					minH: s.minHeight,
					radius: s.borderTopLeftRadius,
					fs: s.fontSize,
					fw: s.fontWeight,
					deco: s.textDecorationLine,
					touch: s.touchAction,
					color: s.color,
				};
			});
			expect(item.display).toBe("grid");
			expect(item.alignItems).toBe("center");
			expect(item.justifyItems).toBe("center");
			expect(item.gap).toBe("3px");
			expect(item.minH).toBe("48px");
			// `MobileBottomNav.tsx` 的 item 是 `rounded-[16px]`（與 active indicator
			// 的 lens 同值，見下方 `lens.borderRadius`）。本檔原本寫 12px，是遷移前的
			// 舊值 —— 因為整支從沒在 CI 跑過，紅了兩個月沒人知道（web#386）。
			expect(item.radius).toBe("16px");
			expect(item.fs).toBe("11px");
			expect(item.fw).toBe("800");
			expect(item.deco).toBe("none");
			expect(item.touch).toBe("manipulation");
			expect(item.color).toBe(exp.itemColor);

			// --- active item + sliding indicator: primary text + white glass lens ---
			// The draggable-nav interaction paints the active background on one
			// independently animated pill, not on the link itself. Keep the link
			// semantic for aria-current/text and assert the actual painted layer.
			const activeIndicator = nav.locator(
				'[data-mobile-nav-active-indicator="true"]',
			);
			await expect(activeIndicator).toHaveCount(1);
			const [activeColor, lens] = await Promise.all([
				activeLink.evaluate((el) => getComputedStyle(el).color),
				activeIndicator.evaluate((el) => {
					const s = getComputedStyle(el);
					return {
						backgroundColor: s.backgroundColor,
						backgroundImage: s.backgroundImage,
						backdrop: s.backdropFilter,
						borderColor: s.borderTopColor,
						borderRadius: s.borderTopLeftRadius,
					};
				}),
			]);
			expect(activeColor).toBe(exp.activeColor);
			expect(lens.backgroundColor).toBe(exp.lensBg);
			expect(lens.backgroundImage).toBe("none");
			expect(lens.backdrop).toContain("blur(10px)");
			expect(lens.backdrop).toContain("saturate(1.15)");
			expect(lens.borderColor).toBe(exp.lensBorder);
			expect(lens.borderRadius).toBe("16px");

			// --- icon wrapper (first span in a link): faithful 30px box ---
			const icon = await inactiveLink
				.locator("span")
				.first()
				.evaluate((el) => {
					const s = getComputedStyle(el);
					return {
						display: s.display,
						w: s.width,
						h: s.height,
						overflow: s.overflow,
						lh: s.lineHeight,
					};
				});
			expect(icon.display).toBe("grid");
			expect(icon.w).toBe("30px");
			expect(icon.h).toBe("30px");
			expect(icon.overflow).toBe("hidden");
			expect(icon.lh).toBe("0px");
		});
	}
});
