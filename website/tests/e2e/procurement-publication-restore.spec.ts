import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { mockProcurementUnary } from "./helpers/mock-procurement-connect";
import {
	publishedRfqJson,
	snapshotReadyJson,
} from "./helpers/mock-procurement-rfq";
import { gotoStreamed } from "./helpers/streamed-navigation";

/**
 * A8：已發布的採購成果，在重整之後仍然打得開。
 *
 * 為什麼一定要 browser 層：這個缺陷不在掛載條件（那個早就吃了
 * `publishedPublication`，還有一條 source 斷言釘著），而在可見性——`hasResults`
 * 只看 candidateSet，於是面板掛得起來卻永遠 `hidden`、開啟鈕也不 render。
 * 只驗 DOM 有沒有掛載會漏掉整個缺陷。
 *
 * ⛔ GetSnapshot 的 `publishedRfq` 形狀以 procurement.proto 的 `PublishedRfq` 為準。
 * `GetSnapshotResponse` 沒有候選集欄位——所以「重整後只有 publication、沒有候選」
 * 不是假想的邊界，是唯一可能的形狀。
 */
const PROCUREMENT_PATH = "/account/messages/assistant/procurement";
const REQUEST_ID = "1234567890123456788";

test.describe("procurement assistant — 已發布成果的重整還原", {
	tag: ["@ui"],
}, () => {
	async function setupRestoredWorkspace(
		page: import("@playwright/test").Page,
		appBaseURL: string,
	) {
		const calls = { candidateSearches: 0, publications: 0 };

		await page.route("**/api/v1/**", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			}),
		);
		await seedStoredActiveRole(page, "buyer");
		await setupGatewayMocks(page, {
			identity: {
				accountId: "300",
				roles: ["buyer"],
			},
			session: "identity",
		});
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: appBaseURL }]);

		await mockProcurementUnary(page, "StartCandidateSearch", () => {
			calls.candidateSearches += 1;
			return {};
		});
		await mockProcurementUnary(page, "ExecuteRfqPublication", () => {
			calls.publications += 1;
			return {};
		});
		await mockProcurementUnary(page, "ListConversations", () => ({
			conversations: [],
			nextPageToken: "",
		}));

		await mockProcurementUnary(page, "GetSnapshot", () =>
			snapshotReadyJson({ publishedRfq: publishedRfqJson() }),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		return calls;
	}

	test("桌面：重整後只有 publication、沒有候選，仍能開啟成果面板並重複開關", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await page.setViewportSize({ width: 1440, height: 900 });
		const calls = await setupRestoredWorkspace(
			page,
			baseURL ?? "http://localhost:3000",
		);

		const response = await gotoStreamed(
			page,
			`${PROCUREMENT_PATH}?requestId=${REQUEST_ID}`,
		);
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		const opener = page.getByTestId("agent-panel-open");
		await expect(opener).toBeVisible({ timeout: 10_000 });

		const panel = page.locator("#hawkr-agent-side-panel");
		await opener.click();
		await expect(panel).toBeVisible();

		await page.getByRole("button", { name: "回到對話" }).click();
		await expect(panel).toBeHidden();
		await opener.click();
		await expect(panel).toBeVisible();

		expect(calls.candidateSearches).toBe(0);
		expect(calls.publications).toBe(0);
	});

	test("手機：同一份還原狀態也能開啟成果面板", async ({ page, baseURL }) => {
		test.setTimeout(30_000);
		await page.setViewportSize({ width: 390, height: 844 });
		const calls = await setupRestoredWorkspace(
			page,
			baseURL ?? "http://localhost:3000",
		);

		const response = await gotoStreamed(
			page,
			`${PROCUREMENT_PATH}?requestId=${REQUEST_ID}`,
		);
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		const opener = page.getByTestId("agent-panel-open");
		await expect(opener).toBeVisible({ timeout: 10_000 });
		await opener.click();
		await expect(page.locator("#hawkr-agent-side-panel")).toBeVisible();

		expect(calls.candidateSearches).toBe(0);
		expect(calls.publications).toBe(0);
	});
});
