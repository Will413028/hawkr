import { expect, test } from "@playwright/test";
import { proxyPublicGatewayReads } from "./e2e/helpers/proxy-public-gateway";

test("desktop product click keeps the source page until the complete quick view opens", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	const stopProxying = await proxyPublicGatewayReads(page);
	try {
		await page.goto("/", { waitUntil: "domcontentloaded", timeout: 30_000 });
		await expect(page.locator(".initial-loader")).toBeHidden();

		const recommendations = page.locator('section[aria-label="小編推薦"]');
		const productLink = recommendations
			.locator('a[href^="/products/"]')
			.first();
		const emptyState = recommendations.getByText("目前沒有可顯示的推薦商品", {
			exact: true,
		});

		// Production is allowed to have an explicitly empty pre-launch catalogue.
		// Keep this interaction coverage live-data-backed when a product exists,
		// but do not turn the product-domain empty state into a navigation failure.
		// Await the data-backed branch before deciding; missing both still fails.
		await expect(productLink.or(emptyState)).toBeVisible();
		if (await emptyState.isVisible()) {
			await expect(productLink).toHaveCount(0);
			test.skip(
				true,
				"public catalogue is explicitly empty; no product quick view fixture exists",
			);
		}

		await expect(productLink).toBeVisible();
		await productLink.scrollIntoViewIfNeeded();
		const box = await productLink.boundingBox();
		if (!box) throw new Error("product link has no mouse target");

		const pendingShell = page.locator('.product-modal[data-pending="true"]');
		await expect(pendingShell).toHaveCount(0);

		await page.mouse.click(box.x + 20, box.y + 20);
		await page.waitForTimeout(100);
		await expect(productLink).toBeVisible();
		await expect(pendingShell).toHaveCount(0);

		await expect(page).toHaveURL(/\/products\/[^/]+/);
		await expect(
			page.locator('.product-modal:not([data-pending="true"])'),
		).toBeVisible();
	} finally {
		await stopProxying();
	}
});
