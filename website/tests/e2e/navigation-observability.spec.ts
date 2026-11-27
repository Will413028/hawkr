import { expect, test } from "@playwright/test";
import { canonicalSellerSegment } from "./helpers/canonical-seller";
import { gotoStreamed } from "./helpers/streamed-navigation";

async function publicCatalogHasProducts(
	request: import("@playwright/test").APIRequestContext,
): Promise<boolean> {
	const response = await request.get(
		"/api/v1/public/products?page=1&pageSize=1",
		{ failOnStatusCode: false },
	);
	expect(response.status()).toBeLessThan(500);
	if (!response.ok()) return false;
	const body = await response.json();
	const records = body?.data?.records ?? body?.data?.list;
	return Array.isArray(records) && records.length > 0;
}

test("guest login leaves the document without a route or splash transition", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	const loginDocumentRequests: string[] = [];
	page.on("request", (request) => {
		if (
			request.resourceType() === "document" &&
			new URL(request.url()).pathname === "/login"
		) {
			loginDocumentRequests.push(request.url());
		}
	});

	await page.goto("/", { timeout: 45_000 });
	await expect(page.locator(".initial-loader")).toBeHidden({ timeout: 15_000 });
	const login = page.getByRole("link", { name: /登入/ }).first();
	await expect(login).toHaveAttribute("data-hk-hard-navigation", "");

	await Promise.all([page.waitForURL(/\/login(?:\?|$)/), login.click()]);

	expect(loginDocumentRequests).toHaveLength(1);
	await expect(page.locator("html")).toHaveClass(/hk-skip-initial-loader/);
	await expect(page.locator(".initial-loader")).toBeHidden();
	await expect(
		page.getByRole("heading", { name: "登入 / 註冊" }),
	).toBeVisible();
});

test("product quick-view exits to the top of the seller storefront", {
	tag: ["@ui", "@live"],
}, async ({ page, request }) => {
	test.slow();
	test.skip(
		!(await publicCatalogHasProducts(request)),
		"public catalog legitimately empty",
	);

	await gotoStreamed(page, "/market/products");
	await expect(
		page.locator('[data-hk-market-products-hydrated="true"]'),
	).toBeAttached();

	await page.evaluate(() => window.scrollTo(0, 800));
	const listScrollY = await page.evaluate(() => window.scrollY);
	expect(listScrollY).toBeGreaterThan(0);

	const card = page
		.locator("article.market-product-card")
		.first()
		.locator("a[data-market-product-primary-link]")
		.first();
	await expect(card).toBeVisible();
	await card.click({ noWaitAfter: true, position: { x: 20, y: 20 } });
	await expect(page).toHaveURL(/\/products\/[^/?#]+$/);

	const dialog = page.getByRole("dialog", { name: "商品詳情" });
	await expect(dialog).toBeVisible();
	const seller = dialog.getByRole("link", { name: /查看 .+ 的店家頁/ });
	await expect(seller).not.toHaveAttribute("data-hk-hard-navigation", "");
	const sellerPath = new URL(
		(await seller.getAttribute("href")) ?? "/sellers/__missing__",
		page.url(),
	).pathname;
	const linkedSegment = sellerPath.split("/")[2] ?? "";
	const canonicalSegment = /^\d+$/.test(linkedSegment)
		? ((await canonicalSellerSegment(request, linkedSegment)) ?? linkedSegment)
		: linkedSegment;
	const destinationPath = sellerPath.replace(
		`/sellers/${linkedSegment}`,
		`/sellers/${canonicalSegment}`,
	);
	const storefrontPaths = new Set([sellerPath, destinationPath]);

	let sellerDocumentRequests = 0;
	page.on("request", (candidate) => {
		if (
			candidate.resourceType() === "document" &&
			storefrontPaths.has(new URL(candidate.url()).pathname)
		) {
			sellerDocumentRequests += 1;
		}
	});

	await Promise.all([
		page.waitForURL((url) => url.pathname === destinationPath),
		seller.click(),
	]);

	await expect(
		page.locator('[data-hk-page-content-ready="seller-storefront"]'),
	).toBeAttached();
	await expect(page.locator(".initial-loader")).toBeHidden();
	await expect(page.locator(".page-wrapper")).toBeVisible();
	expect(sellerDocumentRequests).toBe(0);
	await expect
		.poll(() => page.evaluate(() => window.scrollY), { timeout: 5_000 })
		.toBeLessThan(8);
});
