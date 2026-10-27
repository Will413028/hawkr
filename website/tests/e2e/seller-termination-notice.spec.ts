import { expect, type Page, test } from "@playwright/test";
import {
	fulfillConnect,
	type MockMeasure,
	type MockStore,
	mockMyKyc,
	mockMyMeasures,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const SELLER_ID = "90010002";
const LOADER_KEY = "hawkr:initial-loader-seen";

const STORE: MockStore = {
	id: SELLER_ID,
	storeCode: "2600000002",
	storeName: "預告終止測試商店",
	description: "用產地故事介紹每日上架的新鮮食材。",
	contactPhone: "0912345678",
	address: { townCode: "66000290", addressDetail: "東關路一段 100 號" },
	logoUriTemplate: "https://example.com/{preset}/logo.png",
};

// 預告終止的生效時間（伺服器給的 period.startTime）刻意不是 createTime + 30 天：
// createTime 2026-10-05，+30 天是 2026-11-04；生效時間是 2026-11-20 11:30（台北）。
const SCHEDULED_TERMINATION: MockMeasure = {
	id: "t-scheduled",
	kind: "TERMINATION",
	basis: "CONTRACT",
	statement: "預告終止",
	state: "SCHEDULED",
	period: { startTime: "2026-11-20T03:30:00Z" },
	createTime: "2026-10-05T02:00:00Z",
	canAppeal: true,
	terminationGround: "NOTICE",
	appealDeadlineTime: "2027-05-20T03:30:00Z",
};

// 狀態不是 SCHEDULED 的措施：ListMyMeasures(state=SCHEDULED) 不會回它，橫幅不能出現它的日期。
const ACTIVE_DECOY: MockMeasure = {
	...SCHEDULED_TERMINATION,
	id: "t-active",
	state: "ACTIVE",
	period: { startTime: "2026-09-01T02:00:00Z" },
};

async function prepareSeller(page: Page, baseURL: string | undefined) {
	await page.route("**/api/v1/**", (route) => {
		void route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		});
	});
	await page.route("**/api/connect/**", (route) => {
		void fulfillConnect(route, {});
	});
	await setupGatewayMocks(page, {
		identity: { accountId: SELLER_ID, roles: ["seller"] },
		session: "identity",
	});
	await mockMyStore(page, STORE);
	await mockMyKyc(page, { status: "APPROVED" });
	if (baseURL) {
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	}
	await seedStoredActiveRole(page, "seller");
	await page.addInitScript((key) => {
		sessionStorage.setItem(key, "1");
	}, LOADER_KEY);
}

test.describe("預告終止橫幅", { tag: ["@ui"] }, () => {
	test("生效時間讀伺服器的 period.startTime，不是建立時間加 30 天", async ({
		page,
		baseURL,
	}) => {
		await prepareSeller(page, baseURL);
		const measures = await mockMyMeasures(page, [
			SCHEDULED_TERMINATION,
			ACTIVE_DECOY,
		]);

		await page.goto("/seller-center/dashboard");

		const banner = page.getByRole("status").filter({ hasText: "終止合作" });
		await expect(banner).toBeVisible();
		await expect(banner).toContainText("2026/11/20 11:30");
		// createTime + 30 天（2026/11/04）與其他狀態的措施日期都不能出現。
		await expect(banner).not.toContainText("2026/11/04");
		await expect(banner).not.toContainText("2026/09/01");
		await expect(
			banner.getByRole("link", { name: "措施與申覆" }),
		).toHaveAttribute("href", "/seller/measures/t-scheduled");
		expect(measures.requestedStates()).toContain("MEASURE_STATE_SCHEDULED");
	});

	test("沒有預定生效的終止時不顯示橫幅", async ({ page, baseURL }) => {
		await prepareSeller(page, baseURL);
		await mockMyMeasures(page, [ACTIVE_DECOY]);

		await page.goto("/seller-center/dashboard");

		await expect(
			page.getByRole("heading", { name: "賣家總覽", exact: true }),
		).toBeVisible();
		await expect(
			page.getByRole("status").filter({ hasText: "終止合作" }),
		).toHaveCount(0);
	});
});
