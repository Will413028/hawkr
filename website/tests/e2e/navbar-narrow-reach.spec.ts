import { expect, test } from "@playwright/test";
import { type MockIdentity, setupGatewayMocks } from "./helpers/mock-gateway";
import { installProviderMocks } from "./helpers/mock-gsi";

/**
 * The navbar's right-hand cluster must fit the narrowest phone we support.
 *
 * When it does not, the failure is silent and total: the nav clips its own
 * overflow and the document has no horizontal scroll, so the account menu sits
 * outside the viewport with no way to reach it. A signed-in buyer on a 393px
 * phone cannot open their account menu, which is also the only way to log out.
 *
 * The viewport is pinned here instead of inherited from the project so this
 * runs in the chromium project too. Mobile-only coverage would have left this
 * to the main-push run alone, which is exactly how the 2026-09-11 regression
 * (six 44px actions plus an 8px gap = 54px of overflow) reached main.
 */

const BUYER: MockIdentity = { accountId: "1", roles: ["buyer"] };

test.describe("navbar reach on the narrowest phone", { tag: ["@ui"] }, () => {
	test.use({ viewport: { width: 393, height: 727 } });

	test.beforeEach(async ({ page }) => {
		await page.addInitScript(() => {
			try {
				sessionStorage.setItem("hawkr:initial-loader-seen", "1");
			} catch {
			}
		});
		await installProviderMocks(page);
	});

	test("a signed-in buyer can reach the account menu", async ({
		page,
		isMobile,
	}) => {
		test.skip(!isMobile, "只量手機（浮動捲軸）的寬度");
		await setupGatewayMocks(page, { identity: BUYER, session: "identity" });

		await page.goto("/");

		const trigger = page.getByTestId("account-menu-trigger");
		await expect(trigger).toBeVisible();

		const box = await trigger.boundingBox();
		expect(box).not.toBeNull();
		const viewport = page.viewportSize();
		expect(
			box?.x ?? Number.NaN,
			"account menu starts left of the viewport",
		).toBeGreaterThanOrEqual(0);
		expect(
			(box?.x ?? Number.NaN) + (box?.width ?? Number.NaN),
			"account menu extends past the right edge of the viewport",
		).toBeLessThanOrEqual(viewport?.width ?? Number.NaN);

		const navOverflow = await page.evaluate(() => {
			const nav = document
				.querySelector('[data-testid="account-menu-trigger"]')
				?.closest("header, nav");
			return nav ? nav.scrollWidth - nav.clientWidth : -1;
		});
		const overflowing = await page.evaluate(() => {
			const nav = document
				.querySelector('[data-testid="account-menu-trigger"]')
				?.closest("header, nav");
			if (!nav) return [];
			const edge = nav.getBoundingClientRect().right;
			return Array.from(nav.querySelectorAll("*"))
				.filter((el) => el.getBoundingClientRect().right > edge + 0.5)
				.slice(0, 8)
				.map((el) => {
					const r = el.getBoundingClientRect();
					return `${el.tagName.toLowerCase()}.${String(el.className).slice(0, 80)} right=${r.right.toFixed(1)} width=${r.width.toFixed(1)}`;
				});
		});
		expect(
			navOverflow,
			`navbar overflows horizontally: ${overflowing.join(" | ")}`,
		).toBe(0);

		await trigger.click();
		await expect(page.getByTestId("logout-button")).toBeVisible();
	});
});
