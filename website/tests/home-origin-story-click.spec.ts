import { expect, test } from "@playwright/test";
import { proxyPublicGatewayReads } from "./e2e/helpers/proxy-public-gateway";

test("clicking the active 產地大小事 card opens its post", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	const stopProxying = await proxyPublicGatewayReads(page);
	try {
		await page.goto("/", { waitUntil: "domcontentloaded" });
		await expect(page.locator(".initial-loader")).toBeHidden();

		const stage = page.locator('section[aria-label="產地大小事"]');
		await expect(stage).toBeVisible();

		const openPost = stage.locator(
			'[data-home-feed-active="true"] [data-home-feed-open]',
		);
		await expect(openPost).toBeVisible();
		await openPost.click();

		await expect(page.getByRole("dialog", { name: "貼文詳情" })).toBeVisible();
	} finally {
		await stopProxying();
	}
});
