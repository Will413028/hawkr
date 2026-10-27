import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { publishedPublicationSnapshotFact } from "./helpers/mock-procurement-rfq";

/**
 * A8：已發布的採購成果，在重整之後仍然打得開。
 *
 * 為什麼一定要 browser 層：這個缺陷不在掛載條件（那個早就吃了
 * `publishedPublication`，還有一條 source 斷言釘著），而在可見性——`hasResults`
 * 只看 candidateSet，於是面板掛得起來卻永遠 `hidden`、開啟鈕也不 render。
 * 只驗 DOM 有沒有掛載會漏掉整個缺陷。
 *
 * ⛔ snapshot 的 `publishedPublication` 形狀是 2026-09-19 對 dev 實際擷取的
 * （見 helpers 的 publishedPublicationSnapshotFact 註解），不是憑印象編的。
 * 同一次擷取確認 snapshot 的 top-level **沒有 candidateSet**——所以「重整後
 * 只有 publication、沒有候選」不是假想的邊界，是唯一可能的形狀。
 */
const PROCUREMENT_PATH = "/account/messages/assistant/procurement";
const REQUEST_ID = "1234567890123456788";
const REVISION_ID = "1234567890123456791";

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

		for (const path of [
			"**/api/v1/app/chat/conversations",
			"**/api/v1/app/chat/unread",
			"**/api/v1/app/chat/ws-ticket",
		]) {
			await page.route(path, (route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: { records: [], unread: 0 } }),
				}),
			);
		}

		// ⛔ 這兩個端點在這條路徑上一次都不該被呼叫：買家只是重整看成果，
		// 不是要重跑搜尋，也不是要再發布一次。
		await page.route(
			"**/procurement/requests/*/candidate-searches*",
			(route) => {
				calls.candidateSearches += 1;
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: {} }),
				});
			},
		);
		await page.route("**/procurement/requests/*/rfq-publications*", (route) => {
			calls.publications += 1;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			});
		});

		await page.route(
			`**/api/v1/app/buyers/me/procurement/requests/${REQUEST_ID}/snapshot`,
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: REQUEST_ID,
							revisionId: REVISION_ID,
							// dev 實測：發布完成後 status 仍是需求層的狀態，
							// 而且 top-level 沒有 candidateSet。
							status: "REQUIREMENT_READY",
							slots: {
								item: "珍珠芭樂",
								quantity: 50,
								unit: "KG",
								delivery_city: "桃園市",
							},
							missingRequired: [],
							cropResolution: { candidates: [] },
							runs: [],
							publishedPublication: publishedPublicationSnapshotFact(),
						},
					}),
				}),
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

		const response = await page.goto(
			`${PROCUREMENT_PATH}?requestId=${REQUEST_ID}`,
			{
				waitUntil: "domcontentloaded",
			},
		);
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		// 缺陷的本體：開啟鈕只有在 hasResults 為真時才 render。
		const opener = page.getByTestId("agent-panel-open");
		await expect(opener).toBeVisible({ timeout: 10_000 });

		const panel = page.locator("#hawkr-agent-side-panel");
		await opener.click();
		await expect(panel).toBeVisible();

		// 關掉再開一次：一次性的可見不算「回得去」。
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

		const response = await page.goto(
			`${PROCUREMENT_PATH}?requestId=${REQUEST_ID}`,
			{
				waitUntil: "domcontentloaded",
			},
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
