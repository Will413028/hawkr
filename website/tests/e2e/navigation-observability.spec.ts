import { expect, test } from "@playwright/test";
import { canonicalSellerSegment } from "./helpers/canonical-seller";

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

	// 首頁／/feed／/search／/sellers/:id/posts 的貼文卡片會送動圖原檔（10 MB，
	// card preset 的 312 倍），而它在首頁是 eager 載入 ⇒ 直接擋 `load` 與
	// `networkidle`。2026-08-27 main run 33093420954 實測：含動圖的四個面
	// 26～28.5 秒 settle，不含的 9.2～9.7 秒 —— 差距全落在這一顆上。
	// 預設的 15 秒導航預算因此不夠。放寬而不是把動畫關掉：卡片會動是刻意的
	// 產品行為（GIF 逐幀取景是 2026-08-26 的功能），體積的正解是給它一個有界的
	// 動圖 derivative，追在 website#528。**#528 落地後這裡要跟著調回去。**
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

	await page.goto("/market/products", { waitUntil: "domcontentloaded" });
	await expect(
		page.locator('[data-hk-market-products-hydrated="true"]'),
	).toBeAttached();

	// Open the quick-view from a scrolled list. The offset is the whole point:
	// the modal pins it, and that pin used to be replayed onto whatever route
	// the user left for — stranding the storefront mid-content (or clamped to
	// its bottom), which reads as "the tap did nothing".
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
	// Leaving for the storefront stays an in-app navigation — a full document
	// load here would be a reliability patch, not a fix.
	await expect(seller).not.toHaveAttribute("data-hk-hard-navigation", "");
	const sellerPath = new URL(
		(await seller.getAttribute("href")) ?? "/sellers/__missing__",
		page.url(),
	).pathname;
	// The card links by `store_id` while the storefront is canonical on
	// `store_code`, so the destination is whatever middleware settles on — not
	// the href. Asserting the href would be asserting a link-building detail
	// that canonicalization is allowed to change. What this test is actually
	// about survives either way: arriving at the top of the storefront without
	// a document load.
	//
	// Both paths are watched, but only to cover the pre-redirect hop as well:
	// measured under an induced hard navigation, the destination alone already
	// registers it (1 request), and watching both simply also catches the one
	// aimed at the href (2). The `toBe(0)` below is what does the catching —
	// this set does not make it stricter.
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
