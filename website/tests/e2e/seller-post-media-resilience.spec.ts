import { expect, type Page, test } from "@playwright/test";
import type { PostListItem } from "@/lib/api/openapi-types";
import {
	type MockStore,
	mockMyKyc,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const SELLER_ID = "90010001";
const IMAGE_KEY = "public/post/e2e/seller-post-media.png";
const IMAGE_BASENAME = "seller-post-media.png";
const LOADER_KEY = "hawkr:initial-loader-seen";

const ONE_PIXEL_PNG = Buffer.from(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	"base64",
);

const SELLER_STORE: MockStore = {
	id: SELLER_ID,
	storeName: "動態圖片測試商店",
};

const POST: PostListItem = {
	id: "9007199254740993",
	sellerId: SELLER_ID,
	storeCode: "2609000001",
	sellerName: SELLER_STORE.storeName ?? "",
	logoKey: "",
	content: "E2E 圖片掛載契約",
	imageKeys: [IMAGE_KEY],
	tags: [],
	product: null,
	commentCount: 2,
	likeCount: 3,
	shareCount: 1,
	viewCount: 12,
	isLiked: false,
	createdAt: 1_725_000_000,
	publishStatus: "4402",
};

async function prepareSellerPosts(
	page: Page,
	baseURL: string,
	imageStatus: 200 | 404,
): Promise<void> {
	// Lowest-priority catch-all for secondary seller-shell requests. The gateway
	// helper and exact fixtures below are registered later, so they win.
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		}),
	);

	await setupGatewayMocks(page, {
		identity: {
			accountId: SELLER_ID,
			roles: ["seller"],
		},
		session: "identity",
	});

	await mockMyStore(page, SELLER_STORE);
	await mockMyKyc(page, { status: "APPROVED" });
	await page.route("**/api/v1/app/sellers/me/posts?**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: { records: [POST], total: 1, page: 1, pageSize: 50 },
			}),
		}),
	);

	await page.route(
		(url) => url.pathname.endsWith(`/${IMAGE_BASENAME}`),
		(route) =>
			route.fulfill(
				imageStatus === 200
					? {
							status: 200,
							contentType: "image/png",
							body: ONE_PIXEL_PNG,
							headers: { "cache-control": "no-store" },
						}
					: {
							status: 404,
							contentType: "text/plain",
							body: "missing fixture",
							headers: { "cache-control": "no-store" },
						},
			),
	);

	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await seedStoredActiveRole(page, "seller");
	await page.addInitScript((loaderKey) => {
		sessionStorage.setItem(loaderKey, "1");
	}, LOADER_KEY);

	await page.goto("/seller-center/posts", { waitUntil: "domcontentloaded" });
	await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
	await expect(
		page.getByRole("heading", { name: "動態管理", exact: true }),
	).toBeVisible();
}

function postImages(page: Page) {
	return page.locator(`img[src*="${IMAGE_BASENAME}"]`);
}

function postImageFallbacks(page: Page) {
	return page.locator('span[role="img"][aria-label$="圖片無法顯示"]');
}

test.describe("seller post media resilience", { tag: ["@ui"] }, () => {
	test("mounts only the active breakpoint image tree and defers mobile details", async ({
		page,
		baseURL,
		isMobile,
	}) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		await prepareSellerPosts(page, baseURL, 200);

		const allImages = postImages(page);
		await expect(allImages).toHaveCount(1);
		await expect
			.poll(() =>
				allImages.evaluateAll((nodes) =>
					nodes.every((node) => (node as HTMLImageElement).complete),
				),
			)
			.toBe(true);

		if (!isMobile) {
			await expect(
				page.locator(`.seller-posts__cover img[src*="${IMAGE_BASENAME}"]`),
			).toHaveCount(1);
			await expect(
				page.locator(
					`.seller-mobile-disclosure-list img[src*="${IMAGE_BASENAME}"]`,
				),
			).toHaveCount(0);
			return;
		}

		await expect(
			page.locator(`.seller-posts__cover img[src*="${IMAGE_BASENAME}"]`),
		).toHaveCount(0);
		const disclosureImages = page.locator(
			`.seller-mobile-disclosure-list img[src*="${IMAGE_BASENAME}"]`,
		);
		await expect(disclosureImages).toHaveCount(1);

		await page
			.getByRole("button", {
				name: "展開動態：E2E 圖片掛載契約",
				exact: true,
			})
			.click();
		await expect(disclosureImages).toHaveCount(2);
		await expect(allImages).toHaveCount(2);
	});

	test("replaces a 404 image with AppImage fallback instead of a broken img", async ({
		page,
		baseURL,
		isMobile,
	}) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		await prepareSellerPosts(page, baseURL, 404);

		const allImages = postImages(page);
		const fallbacks = postImageFallbacks(page);
		await expect(fallbacks).toHaveCount(1);
		await expect(allImages).toHaveCount(0);

		if (!isMobile) return;

		await page
			.getByRole("button", {
				name: "展開動態：E2E 圖片掛載契約",
				exact: true,
			})
			.click();
		await expect(fallbacks).toHaveCount(2);
		await expect(allImages).toHaveCount(0);
	});
});
