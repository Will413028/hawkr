import { expect, type Page, test } from "@playwright/test";
import {
	fulfillConnect,
	type MockMeasure,
	type MockStore,
	mockMyKyc,
	mockMyMeasures,
	mockMyStore,
	mockStoreOpening,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

// 伺服器對「不能做」給的句子（LocalizedMessage）：client 原樣顯示，不依 reason 自己組字。
// 句子取自設計 step10e-Lprime-design.md §7(a)；mock 的形狀是 proto3 JSON 的 `*Unavailability { reason, message }`。
const SELLER_ID = "90010003";
const LOADER_KEY = "hawkr:initial-loader-seen";

const STORE: MockStore = {
	id: SELLER_ID,
	storeCode: "2600000003",
	storeName: "不能做的原因測試商店",
	description: "用產地故事介紹每日上架的新鮮食材。",
	contactPhone: "0912345678",
	address: { townCode: "66000290", addressDetail: "東關路一段 100 號" },
	logoUriTemplate: "https://example.com/{preset}/logo.png",
};

const APPEAL_PENDING_SENTENCE =
	"這筆措施有待處理的解除請求，審核完成後才能申覆";
const COOLDOWN_SENTENCE = "解除請求目前在冷卻期內，2026/10/20 10:06 後可再提出";
const TERMINATED_SENTENCE =
	"此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。";

const RISK_SUSPENSION: MockMeasure = {
	id: "m-risk",
	kind: "SUSPENSION",
	basis: "RISK",
	riskGround: "FRAUD",
	statement: "商品說明不實",
	state: "ACTIVE",
	period: { startTime: "2026-10-01T02:00:00Z" },
	createTime: "2026-10-01T02:00:00Z",
	canAppeal: false,
	appealUnavailabilities: [
		{ reason: "REQUEST_PENDING", message: APPEAL_PENDING_SENTENCE },
	],
	canRequestRemediation: false,
	remediationUnavailabilities: [
		{ reason: "COOLDOWN", message: COOLDOWN_SENTENCE },
	],
	nextRemediationTime: "2026-10-20T02:05:01Z",
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

test.describe("伺服器給的不能做的原因句", { tag: ["@ui"] }, () => {
	test("措施列表：申覆與解除請求入口附伺服器的句子，冷卻時間不是 client 用 nextRemediationTime 組的", async ({
		page,
		baseURL,
	}) => {
		await prepareSeller(page, baseURL);
		await mockMyMeasures(page, [RISK_SUSPENSION]);

		await page.goto("/seller/measures");

		await expect(page.getByText(APPEAL_PENDING_SENTENCE).first()).toBeVisible();
		// 句子裡的時間（10:06）與 nextRemediationTime（10:05:01）進位後相同只是巧合；
		// 換掉句子就換掉畫面，證明畫面讀的是句子而不是欄位。
		await expect(page.getByText(COOLDOWN_SENTENCE).first()).toBeVisible();
		await expect(
			page.getByRole("button", { name: "提出申覆" }).first(),
		).toBeDisabled();
		await expect(
			page.getByRole("button", { name: "已改善，請求解除" }).first(),
		).toBeDisabled();
	});

	test("措施列表：伺服器改了句子，畫面跟著變（不依 reason 對照）", async ({
		page,
		baseURL,
	}) => {
		await prepareSeller(page, baseURL);
		await mockMyMeasures(page, [
			{
				...RISK_SUSPENSION,
				appealUnavailabilities: [
					{ reason: "REQUEST_PENDING", message: "伺服器改過的申覆說明" },
				],
				remediationUnavailabilities: [
					{ reason: "COOLDOWN", message: "伺服器改過的解除請求說明" },
				],
			},
		]);

		await page.goto("/seller/measures");

		await expect(page.getByText("伺服器改過的申覆說明").first()).toBeVisible();
		await expect(
			page.getByText("伺服器改過的解除請求說明").first(),
		).toBeVisible();
		await expect(page.getByText(APPEAL_PENDING_SENTENCE)).toHaveCount(0);
		await expect(page.getByText(COOLDOWN_SENTENCE)).toHaveCount(0);
	});

	test("開店：不能開店時顯示伺服器的句子與措施與申覆連結", async ({
		page,
		baseURL,
	}) => {
		await page.route("**/api/v1/**", (route) => {
			void route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			});
		});
		await setupGatewayMocks(page, {
			identity: { accountId: SELLER_ID, roles: ["buyer"] },
			session: "identity",
		});
		await mockStoreOpening(page, [
			{ reason: "TERMINATED", message: TERMINATED_SENTENCE },
		]);
		if (baseURL) {
			await page
				.context()
				.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
		}
		await page.addInitScript((key) => {
			sessionStorage.setItem(key, "1");
		}, LOADER_KEY);

		await page.goto("/seller/onboarding");

		const alert = page.getByRole("alert").filter({ hasText: "第 21.3 條" });
		await expect(alert).toContainText(TERMINATED_SENTENCE);
		await expect(
			alert.getByRole("link", { name: "措施與申覆" }),
		).toHaveAttribute("href", "/seller/measures");
	});
});
