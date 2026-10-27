import { expect, test } from "@playwright/test";
import { setupGatewayMocks } from "./helpers/mock-gateway";

const ACTIVE_ROLE_KEY = "hkui:auth:active-role";
const LOADER_KEY = "hawkr:initial-loader-seen";

test.describe("seller import preview authorization", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"role-zone authorization only needs one browser project",
	);

	test("a buyer session cannot enter the seller-only preview", async ({
		page,
		baseURL,
	}) => {
		await setupGatewayMocks(page, {
			identity: { accountId: "1", roles: ["buyer"] },
			session: "identity",
		});
		await page.context().addCookies([
			{
				name: "hk_session",
				value: "opaque",
				url: baseURL ?? "http://localhost:3000",
			},
		]);
		await page.addInitScript(
			([roleKey, loaderKey]) => {
				localStorage.setItem(roleKey, "buyer");
				sessionStorage.setItem(loaderKey, "1");
			},
			[ACTIVE_ROLE_KEY, LOADER_KEY] as const,
		);

		await page.goto("/seller-product-import-preview/route-contract-job", {
			waitUntil: "domcontentloaded",
		});

		await expect(page).toHaveURL("/", { timeout: 15_000 });
	});
});
