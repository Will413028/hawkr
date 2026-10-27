import { expect, test } from "@playwright/test";
import { gotoThemed } from "./_helpers";

/**
 * Computed-style gate for the AutoHideNavShell CSS → Tailwind migration
 * (Navbar group, knife 1).
 *
 * Unlike the colour-normalizing migrations (Pagination / LoginModal /
 * MobileBottomNav), this one is PURELY POSITIONAL — the shell has no
 * theme-dependent paint (background is transparent in both themes, all
 * other props are layout/transform). So this is a refactoring-flavour
 * gate: it locks the existing computed values and must stay GREEN before
 * AND after the migration. A single theme is enough; running both would
 * add cost with zero coverage.
 *
 * What moves to Tailwind:
 *   - wrapper `.auto-nav`: sticky / top / z-index / padding-top(token) /
 *     transform(translate3d) / transition(transform, responsive duration) /
 *     will-change / pointer-events toggle on [data-hidden].
 *   - edge `.auto-nav__edge`: fixed / inset-x / top / height(responsive) /
 *     z-index / pointer-events / transparent bg. The BEM class is dropped.
 * The `.auto-nav` class is RETAINED as a pure hook — three unmigrated
 * consumers still hang off it (descendant `.site-nav`/`.seller-header`
 * position:relative, seller-center.css padding/display overrides, the
 * navigation-final.css dark background reset). It stays until knives 4/5.
 *
 * Responsive: desktop (Chrome 1280) vs mobile (Pixel 5 393, ≤768px) differ
 * on transition duration (320 vs 240ms), edge height (64 vs 24px), padding
 * (16 vs 0px) and hide distance (-120% vs -100%). The gate branches on
 * viewport width and runs on BOTH projects.
 *
 * Locators are structural and survive the BEM → utility swap: the wrapper
 * keeps the `.auto-nav` hook; the edge is its adjacent aria-hidden sibling.
 *
 * ── 為什麼不再 `test.skip(!!process.env.CI)`（web#386 同型殘留）────────────
 * 原本的跳過理由與 `tests/visual/mobile-bottom-nav.spec.ts` 逐字相同：
 * 「computed-style gate needs a running app server」——那個理由在 #386 已被明確
 * 證偽：`playwright.config.ts` 的 `webServer` 在 CI 會 `pnpm start` 起一台真的
 * production server（`process.env.CI ? "pnpm start" : "pnpm exec next dev"`），
 * 同資料夾的其他 spec 全都靠它。#386 修掉了票面點名的兩支，這支（與
 * `seller-header.spec.ts`）帶著同一句已證偽的理由留了下來，於是這道
 * positional 回歸閘門從建立起就零保障。現在讓它真的跑：full E2E（main push /
 * PR 進 main）兩個 project 都跑，dev push 的 chromium smoke 子集不含本檔。
 */

const TIMING = "cubic-bezier(0.22, 1, 0.36, 1)";

test.describe("AutoHideNavShell CSS → Tailwind — computed-style gate", {
	tag: ["@ui", "@live"],
}, () => {
	test("shell + edge positional tokens", async ({ page, viewport }) => {
		const isMobile = !!viewport && viewport.width <= 768;

		await gotoThemed(page, "/", "light");

		const wrapper = page.locator(".auto-nav");
		await expect(wrapper).toBeAttached();
		const edge = page.locator('.auto-nav + [aria-hidden="true"]');
		await expect(edge).toBeAttached();

		// --- wrapper: read transition props BEFORE killing transitions ---
		const w = await wrapper.evaluate((el) => {
			const s = getComputedStyle(el);
			return {
				position: s.position,
				top: s.top,
				zIndex: s.zIndex,
				pad: s.paddingTop,
				bg: s.backgroundColor,
				willChange: s.willChange,
				transProp: s.transitionProperty,
				transDur: s.transitionDuration,
				transTiming: s.transitionTimingFunction,
				pointer: s.pointerEvents,
			};
		});
		expect(w.position).toBe("sticky");
		expect(w.top).toBe("0px");
		expect(w.zIndex).toBe("200");
		expect(w.pad).toBe(isMobile ? "0px" : "16px");
		expect(w.bg).toBe("rgba(0, 0, 0, 0)");
		expect(w.willChange).toBe("transform");
		expect(w.transProp).toBe("transform");
		expect(w.transDur).toBe(isMobile ? "0.2s" : "0.32s");
		expect(w.transTiming).toBe(TIMING);
		expect(w.pointer).toBe("auto");

		// --- edge: fixed reveal strip ---
		const e = await edge.evaluate((el) => {
			const s = getComputedStyle(el);
			return {
				position: s.position,
				top: s.top,
				left: s.left,
				right: s.right,
				height: s.height,
				zIndex: s.zIndex,
				pointer: s.pointerEvents,
				bg: s.backgroundColor,
			};
		});
		expect(e.position).toBe("fixed");
		expect(e.top).toBe("0px");
		expect(e.left).toBe("0px");
		expect(e.right).toBe("0px");
		expect(e.height).toBe(isMobile ? "24px" : "64px");
		expect(e.zIndex).toBe("199");
		// The strip is a layout marker, not a hit target: useAutoHideNav listens
		// for pointermove on `window`, so the reveal works without the strip
		// capturing anything. Pinning "auto" here pinned an implementation the
		// component deliberately moved away from; the capability that actually
		// matters is covered by "a hidden bar is revealed from the top edge".
		expect(e.pointer).toBe("none");
		expect(e.bg).toBe("rgba(0, 0, 0, 0)");

		// --- hidden state: bar translates fully off the top + goes inert. ---
		// Kill transitions first so the transform settles synchronously (a
		// mid-animation getBoundingClientRect would read an interpolated pos).
		await page.addStyleTag({ content: "*{transition:none !important}" });
		const hidden = await wrapper.evaluate((el) => {
			const before = el.getBoundingClientRect().bottom;
			el.setAttribute("data-hidden", "true");
			const after = el.getBoundingClientRect().bottom;
			return { before, after, pointer: getComputedStyle(el).pointerEvents };
		});
		// Visible at top: bottom edge is on-screen.
		expect(hidden.before).toBeGreaterThan(0);
		// Hidden: translated fully above the viewport (≥100% of its own height).
		expect(hidden.after).toBeLessThanOrEqual(1);
		expect(hidden.pointer).toBe("none");
	});

	// The reveal strip is inert, so this is the assertion that keeps the
	// top-edge reveal honest: it exercises the window-level listener the
	// component relies on instead of the strip's computed styles.
	test("a hidden bar is revealed from the top edge", async ({ page }) => {
		await gotoThemed(page, "/", "light");

		const wrapper = page.locator(".auto-nav");
		await expect(wrapper).toBeAttached();

		// Scroll far enough for the hook to hide the bar.
		await page.evaluate(() => window.scrollTo(0, 1200));
		await expect(wrapper).toHaveAttribute("data-hidden", "true");

		// A mouse entering the reveal zone brings it back.
		const width = page.viewportSize()?.width ?? 1280;
		await page.mouse.move(Math.floor(width / 2), 2);
		await expect(wrapper).toHaveAttribute("data-hidden", "false");
	});
});
