import {
	type APIRequestContext,
	expect,
	type Page,
	test,
} from "@playwright/test";
import { discoverLiveEntity } from "./seo/_helpers";

/**
 * Regression guard: fixed nav must not overlap page content.
 *
 * Root cause (2026-04-16, commit 4883e1a): page-content padding-top was reduced
 * to 80px while the actual nav height is 140px (20px padding + 100px inner +
 * 20px padding), causing content to render behind the frosted-glass nav.
 *
 * ── 為什麼不再 `test.skip(!!process.env.CI)`（web#386）─────────────────────
 * 本檔原本整支跳過 CI，理由寫的是「CI only has an empty Postgres for Better
 * Auth」。那個理由**已經不成立**：`discoverLiveEntity` 直接打
 * `API_BASE_URL`（預設 loopback）的 public endpoint，不經本地 proxy；後端不可達時該測試 skip。同一個 helper 已經被
 * `tests/e2e/public-dynamic-routes.spec.ts` 用在 CI 的 smoke 子集裡。
 *
 * 於是這支從「看起來有覆蓋、實際上零保障」變成真的會跑：full E2E（main push /
 * PR 進 main）兩個 project 都跑，dev push 的 chromium smoke 子集不含本檔。
 *
 * viewport 不由本檔決定，而是由 playwright project 決定（chromium = Desktop
 * Chrome 1280px、mobile = Pixel 5 393px）。因此**不要**在測試名稱寫死
 * `(desktop)` / `(mobile)`——原本那三個 `(desktop)` 其實在 mobile project 下
 * 也照跑（其中兩個本來就會過），只有真的需要桌機寬度的那一個要自己 skip。
 */

/** `.market-list-main__sort` 在 `@media (max-width: 768px)` 是 display:none
 *  （styles/features/market-products-list.css:1308）—— 排序列只在更寬的 viewport
 *  存在，所以引用它的那條斷言必須自己收斂 viewport，不能靠測試名稱暗示。 */
const SORT_NAV_MIN_WIDTH = 769;

async function getNavBottom(page: Page) {
	const navInner = page.locator(".site-nav__inner");
	await expect(navInner).toBeVisible();
	const box = await navInner.boundingBox();
	if (!box) throw new Error("Nav bounding box not found");
	return box.y + box.height;
}

async function getLiveSellerPath(request: APIRequestContext) {
	const sellerId = await discoverLiveEntity(request, "seller");
	expect(
		sellerId,
		"Recommended sellers should provide a live seller id",
	).not.toBeNull();
	return `/sellers/${sellerId}`;
}

function expectSellerCoverAspect(box: { width: number; height: number }) {
	expect(
		box.width / box.height,
		`Seller cover should stay 4:1, received ${box.width}×${box.height}`,
	).toBeCloseTo(4, 2);
}

test("seller cover keeps 4:1 across responsive widths", {
	tag: ["@ui", "@live"],
}, async ({ page, request }) => {
	await page.goto(await getLiveSellerPath(request));
	const cover = page.locator(".hkui-seller-profile__cover");
	await expect(cover).toBeVisible();

	for (const width of [320, 390, 768, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		const coverBox = await cover.boundingBox();
		if (!coverBox) {
			throw new Error(`Seller hero cover missing at ${width}px viewport`);
		}
		expectSellerCoverAspect(coverBox);
	}
});

test.describe("Nav / content overlap regression", {
	tag: ["@ui", "@live"],
}, () => {
	test("seller page: hero cover is below nav bottom", async ({
		page,
		request,
	}) => {
		await page.goto(await getLiveSellerPath(request));

		const navBottom = await getNavBottom(page);
		const cover = page.locator(".hkui-seller-profile__cover");
		await expect(cover).toBeVisible();
		const coverBox = await cover.boundingBox();
		if (!coverBox) throw new Error("Seller hero cover bounding box not found");
		expectSellerCoverAspect(coverBox);

		expect(
			coverBox.y,
			`Seller hero cover top (${coverBox.y}px) should be ≥ nav bottom (${navBottom}px)`,
		).toBeGreaterThanOrEqual(navBottom);
	});

	test("seller page: back button is below nav bottom", async ({
		page,
		request,
	}) => {
		await page.goto(await getLiveSellerPath(request));

		const navBottom = await getNavBottom(page);
		await expect(page.locator(".hkui-seller-profile__cover")).toBeVisible();
		const backBtn = page.locator(".hkui-seller-profile__back");
		await expect(backBtn).toBeVisible();
		const backBox = await backBtn.boundingBox();
		if (!backBox) throw new Error("Back button bounding box not found");

		expect(
			backBox.y,
			`Back button top (${backBox.y}px) should be ≥ nav bottom (${navBottom}px)`,
		).toBeGreaterThanOrEqual(navBottom);
	});

	test("products page: first visible content is below nav bottom", async ({
		page,
		viewport,
	}) => {
		test.skip(
			!viewport || viewport.width < SORT_NAV_MIN_WIDTH,
			`排序列在 <${SORT_NAV_MIN_WIDTH}px 是 display:none（market-products-list.css @media max-width:768px）`,
		);

		await page.goto("/products");

		const navBottom = await getNavBottom(page);
		const listHead = page
			.getByRole("navigation", { name: "排序" })
			.locator("..");
		await expect(listHead).toBeVisible();
		const headBox = await listHead.boundingBox();
		if (!headBox) throw new Error("List head bounding box not found");

		expect(
			headBox.y + headBox.height,
			"List head must have content below the nav",
		).toBeGreaterThan(navBottom);
	});
});
