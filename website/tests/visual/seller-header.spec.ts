import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { expect, type Page, test } from "@playwright/test";
import { mockMyStore, setupGatewayMocks } from "../e2e/helpers/mock-gateway";
import type { Theme } from "./_helpers";

/**
 * Computed-style gate for the seller SellerHeader Tailwind migration (子刀 5).
 *
 * The seller header only renders on /seller-center/* (RoleAwareNavbar branches
 * on pathname) and its content layout is guarded by RoleZoneGuard required=
 * "seller", so — unlike the buyer navbar gate — there is no guest route that
 * shows it. This gate fakes a logged-in seller: it route-mocks the gateway
 * (setupGatewayMocks with a seller-bit identity + session probe) and pins the
 * stored active-role to "seller" so RoleZoneGuard passes. SellerCenterShell
 * then mounts `.seller-shell`, which is what activates the route-scoped
 * `body .page-wrapper:has(.seller-shell) .seller-header__inner` surface
 * overrides in seller-center.css — the values this gate must preserve.
 *
 * The header's live styling is split: navigation-shell.css (`body .seller-
 * header*`, loaded everywhere) supplies the layout + children, and
 * seller-center.css (the `:has(.seller-shell)` overrides + dark block) supplies
 * the pill surface and dark colours. The migration folds the RESOLVED values
 * into inline Tailwind on RoleAwareNavbar.tsx and deletes the matching CSS;
 * every value here must stay byte-identical (toEqual).
 *
 * Out of scope (子刀 6, untouched): the shared `.site-nav-user__menu*` dropdown
 * the SellerAccountMenu opens. Only the header chrome is gated.
 *
 * Regenerate the baseline (only when the header intentionally changes) by
 * running with CAPTURE=1 against the pre-change tree and copying the printed
 * JSON into seller-header.baseline.json.
 *
 * ── 為什麼不再 `test.skip(!!process.env.CI)`（web#386 同型殘留）────────────
 * 原本檔頭寫「Local-only (needs a running app + the mocked gateway); skipped in
 * CI」，skip 理由字串則與 `tests/visual/mobile-bottom-nav.spec.ts` 逐字相同：
 * 「computed-style gate needs a running app server」。兩個前提都不成立：
 *   1. app server —— `playwright.config.ts` 的 `webServer` 在 CI 會 `pnpm start`
 *      起一台真的 production server，同資料夾的其他 spec 全都靠它（#386 已證偽
 *      同一句理由）。
 *   2. mocked gateway —— `setupGatewayMocks` 是 Playwright 的 `page.route()`
 *      攔截，跑在瀏覽器層，本來就不需要任何後端；`tests/e2e/` 的多支 spec 已
 *      在 CI 用同一個 helper。
 * #386 修掉了票面點名的兩支，這支（與 `auto-hide-nav-shell.spec.ts`）帶著同一
 * 句已證偽的理由留了下來，於是這道 computed-style 閘門從建立起就零保障。現在
 * 讓它真的跑：full E2E（main push / PR 進 main）跑到，並由檔內既有的
 * `testInfo.project.name !== "chromium"` 收斂成 mouse-only 的 chromium 專屬；
 * dev push 的 chromium smoke 子集不含本檔。
 */

const baseline = JSON.parse(
	readFileSync(
		fileURLToPath(new URL("./seller-header.baseline.json", import.meta.url)),
		"utf8",
	),
) as Record<
	string,
	{
		base: Record<string, Record<string, string> | null>;
		hover: Record<string, Record<string, string>>;
	}
>;

const THEME_STORAGE_KEY = "hk:theme";
const ACTIVE_ROLE_KEY = "hkui:auth:active-role";

/**
 * Land on the seller workspace as a logged-in seller: mock the gateway session
 * probe (seller-bit identity), pin theme + active-role before first paint, then
 * wait for both the header and `.seller-shell` (the latter activates the
 * `:has(.seller-shell)` surface overrides) plus fonts.
 */
async function gotoSellerHeader(
	page: Page,
	theme: Theme,
	baseURL: string,
): Promise<void> {
	await setupGatewayMocks(page, {
		identity: { accountId: "1", roles: ["seller"] },
		session: "identity",
	});
	// SellerCenterShell treats a store without a completed onboarding as an
	// incomplete shop and redirects to onboarding, so provide the one store
	// fixture this header gate actually needs. Registering it after the
	// catch-all gives this exact route precedence in Playwright.
	await mockMyStore(page, { id: "1", storeName: "測試商店" });
	// The /seller-center server layer reads the gateway session cookie and
	// redirects to /login when it is absent. The cookie is HttpOnly (no JS
	// set-cookie path), so seed it directly — the e2e suite gets it from the
	// social-login mock's Set-Cookie instead.
	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await page.addInitScript(
		([themeKey, themeVal, roleKey, roleVal, loaderKey]) => {
			try {
				localStorage.setItem(themeKey, themeVal);
				localStorage.setItem(roleKey, roleVal);
				// Skip the full-screen splash so the header paints immediately.
				sessionStorage.setItem(loaderKey, "1");
			} catch {
				/* private mode — boot script falls back to light/guest */
			}
		},
		[
			THEME_STORAGE_KEY,
			theme,
			ACTIVE_ROLE_KEY,
			"seller",
			"hawkr:initial-loader-seen",
		] as const,
	);
	await page.goto("/seller-center/dashboard", {
		waitUntil: "domcontentloaded",
	});
	await page.locator(".seller-header").first().waitFor({ state: "attached" });
	// `.seller-shell` only mounts once RoleZoneGuard resolves the seller role —
	// it is the `:has()` anchor for the pill surface overrides, so the read must
	// wait for it or it captures the un-overridden navigation-shell surface.
	await page.locator(".seller-shell").first().waitFor({ state: "attached" });
	await page.evaluate(() => document.fonts.ready);
}

const VIEWPORTS = [
	{ label: "desktop-1440", width: 1440, height: 900 },
	// ≤960: seller-center.css hides the pill (SellerCenterShell renders its own
	// compact mobile bar). Locks the hide.
	{ label: "tablet-900", width: 900, height: 900 },
] as const;

const THEMES: Theme[] = ["light", "dark"];

async function readEl(
	page: Page,
	selector: string,
	props: string[],
): Promise<Record<string, string> | null> {
	const loc = page.locator(selector).first();
	if ((await loc.count()) === 0) return null;
	return loc.evaluate((el, ps) => {
		const s = getComputedStyle(el as Element);
		const out: Record<string, string> = {};
		for (const p of ps) out[p] = s.getPropertyValue(p) || (s as never)[p];
		return out;
	}, props);
}

const HOST = [
	"position",
	"display",
	"width",
	"height",
	"background-color",
	"padding",
];
const INNER = [
	"display",
	"align-items",
	"column-gap",
	"max-width",
	"height",
	"min-height",
	"margin-left",
	"margin-right",
	"padding",
	"background-color",
	"border-top-width",
	"border-top-color",
	"border-radius",
	"box-shadow",
	"backdrop-filter",
	"color",
];
const BRAND = [
	"display",
	"align-items",
	"column-gap",
	"color",
	"text-decoration-line",
];
const NAV = ["display", "align-items", "column-gap"];
const NAVITEM = [
	"position",
	"display",
	"align-items",
	"justify-content",
	"min-height",
	"padding",
	"border-radius",
	"color",
	"font-size",
	"font-weight",
	"text-decoration-line",
	"white-space",
	"background-color",
];
const ACTIONS = [
	"display",
	"align-items",
	"justify-content",
	"column-gap",
	"margin-left",
];
const CTA = [
	"display",
	"align-items",
	"column-gap",
	"height",
	"padding",
	"border-radius",
	"background-color",
	"color",
	"font-size",
	"font-weight",
	"text-decoration-line",
	"white-space",
	"box-shadow",
];
const ICONBTN = [
	"display",
	"width",
	"height",
	"align-items",
	"justify-items",
	"border-top-width",
	"border-radius",
	"background-color",
	"color",
];
const LOCKUP = ["width", "height"];
const AVATAR = [
	"display",
	"width",
	"height",
	"align-items",
	"justify-items",
	"border-top-width",
	"border-top-color",
	"border-radius",
	"background-color",
	"color",
	"box-shadow",
];

test(
	"seller header computed styles unchanged by Tailwind migration",
	{ tag: ["@ui"] },
	async ({ page, baseURL }, testInfo) => {
		// The gate asserts mouse-hover affordances (lift transform, highlight) that
		// touch devices never apply — so it is meaningful on chromium only. One
		// baseline file can't encode both the hovered and the no-hover state, and
		// the mobile project would always diff on `transform: none`.
		test.skip(
			testInfo.project.name !== "chromium",
			"hover-state gate is mouse-only; chromium covers it",
		);
		test.setTimeout(180_000);
		const origin = baseURL ?? "http://localhost:3000";
		const capture = !!process.env.CAPTURE;
		const captured: Record<string, unknown> = {};
		for (const vp of VIEWPORTS) {
			await page.setViewportSize({ width: vp.width, height: vp.height });
			for (const theme of THEMES) {
				await gotoSellerHeader(page, theme, origin);
				await page.addStyleTag({
					// Keep the computed-style fixture independent from dashboard
					// content height. A classic vertical scrollbar otherwise changes
					// the 1440px viewport into a 1425px layout viewport even though
					// the header CSS is unchanged.
					content:
						"html{overflow-y:hidden!important;scrollbar-gutter:auto!important}*{transition:none!important;animation:none!important}",
				});
				const key = `${vp.label}__${theme}`;
				const base = {
					header: await readEl(page, ".seller-header", HOST),
					inner: await readEl(page, ".seller-header__inner", INNER),
					brand: await readEl(page, ".seller-header__brand", BRAND),
					nav: await readEl(page, ".seller-header__nav", NAV),
					// /seller-center/dashboard → 賣家總覽 is the active item.
					navItemActive: await readEl(
						page,
						".seller-header__nav-item.is-active",
						NAVITEM,
					),
					navItemPlain: await readEl(
						page,
						".seller-header__nav-item:not(.is-active)",
						NAVITEM,
					),
					actions: await readEl(page, ".seller-header__actions", ACTIONS),
					cta: await readEl(page, ".seller-header__cta", CTA),
					iconBtn: await readEl(page, ".seller-header__icon-btn", ICONBTN),
					themeBtn: await readEl(page, ".seller-header__theme-btn", LOCKUP),
					avatar: await readEl(page, ".seller-header__avatar", AVATAR),
				};

				// hover carries the translateY(-1px) lift + bg/colour shift on the
				// interactive children. nav-item hover read uses a plain (non-active)
				// item so the hover vs is-active states do not overlap.
				const hover: Record<string, unknown> = {};
				for (const [name, sel] of [
					["navItem", ".seller-header__nav-item:not(.is-active)"],
					["cta", ".seller-header__cta"],
					["iconBtn", ".seller-header__icon-btn"],
				] as const) {
					const loc = page.locator(sel).first();
					if ((await loc.count()) === 0) continue;
					await loc.hover().catch(() => {});
					hover[name] = await loc.evaluate((el) => {
						const s = getComputedStyle(el as Element);
						return {
							backgroundColor: s.backgroundColor,
							color: s.color,
							transform: s.transform,
						};
					});
					await page.mouse.move(0, 0);
				}

				if (capture) {
					captured[key] = { base, hover };
				} else {
					expect(base, `base @ ${key}`).toEqual(baseline[key].base);
					expect(hover, `hover @ ${key}`).toEqual(baseline[key].hover);
				}
			}
		}
		if (capture) {
			console.log(
				`\n@@SELLER_CAPTURE_START@@\n${JSON.stringify(captured, null, 2)}\n@@SELLER_CAPTURE_END@@\n`,
			);
		}
	},
);
