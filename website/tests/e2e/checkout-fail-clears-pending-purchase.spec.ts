import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

/**
 * GA4 follow-up C: the pending-purchase handoff written at confirm
 * (`lib/analytics/purchase-store.ts`, read by `PurchaseTracker` on
 * /checkout/success) must not survive a FAILED checkout. Only the success
 * landing used to clear it, so a buyer whose payment failed kept the key for
 * the rest of the tab session — reopening an older success URL then fired a
 * phantom `purchase` event for an order that was never paid.
 */

const PENDING_KEY = "hawkr:pending_purchase";
const ORDER_NO = "O-FAIL-GA4-1";

test("landing on /checkout/fail clears the pending purchase carried across the PayUni redirect", {
	// 斷言全在我們自己的 client 行為（頁面掛載時清 sessionStorage），
	// gateway 由 mock 承接 ⇒ hermetic，不加 @live。
	tag: ["@ui"],
}, async ({ page }) => {
	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "300", roles: ["buyer"] },
	});
	await seedStoredActiveRole(page, "buyer");
	await page.addInitScript(
		([key, orderNo]) => {
			try {
				sessionStorage.setItem(key, JSON.stringify([orderNo]));
			} catch {
				// A browser that refuses storage has nothing to clear either.
			}
		},
		[PENDING_KEY, ORDER_NO] as const,
	);

	await page.goto(`/checkout/fail?order=${ORDER_NO}&reason=CARD_DECLINED`);

	// The page itself rendered (not an auth redirect) — otherwise "cleared"
	// would be indistinguishable from "never got there".
	await expect(page.getByRole("heading", { name: "付款未完成" })).toBeVisible();

	const pending = await page.evaluate(
		(key) => sessionStorage.getItem(key),
		PENDING_KEY,
	);
	expect(pending).toBeNull();
});
