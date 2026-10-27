import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const PATH = "/account/messages/assistant/demo";

test.describe("procurement assistant — crop disambiguation card", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName }) => browserName !== "chromium",
		"single representative browser is enough for this mocked-gateway flow",
	);

	test("品項有多個候選作物時，買家能看到消歧卡並選擇", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		let turnCount = 0;

		// Lowest-priority response for unrelated Messages shell requests.
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

		// Messages shell fixtures — registered after the generic gateway mock so
		// their typed response wins Playwright's last-route-first dispatch.
		await page.route("**/api/v1/app/chat/conversations", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: { records: [] } }),
			}),
		);
		await page.route("**/api/v1/app/chat/unread", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: { unread: 0 } }),
			}),
		);
		await page.route("**/api/v1/app/chat/ws-ticket", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			}),
		);

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			turnCount += 1;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1",
						revisionId: "1",
						slots: { item: "高麗菜" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "這個品項有幾種可能，麻煩選一下～",
						cropDisambiguation: {
							prompt: "你說的「高麗菜」是指哪一種？",
							options: [
								{
									optionId: "opt-1",
									label: "甘藍",
									cropCode: null,
									groupCode: "N04-001",
								},
								{
									optionId: "opt-2",
									label: "高麗菜芽（高麗菜）",
									cropCode: null,
									groupCode: "N04-002",
								},
							],
						},
					},
				}),
			});
		});
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/crop-disambiguation",
			(route) => {
				const body = route.request().postDataJSON() as Record<string, unknown>;
				expect(body.groupCode).toBe("N04-002");
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1",
							revisionId: "2",
							slots: { item: "高麗菜" },
							missingRequired: [],
							rejected: [],
							degraded: false,
							cropDisambiguation: null,
							reply: "好，記下了。",
						},
					}),
				});
			},
		);
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/snapshot",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1",
							revisionId: "2",
							status: "CLARIFYING",
							slots: { item: "高麗菜" },
							missingRequired: [],
						},
					}),
				}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await page.goto(PATH, { waitUntil: "domcontentloaded" });
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		const sendButton = page.getByRole("button", { name: "送出" });

		await textarea.fill("我要訂高麗菜");
		await sendButton.click();

		await expect(page.getByTestId("crop-disambiguation-card")).toBeVisible({
			timeout: 10_000,
		});
		await page.getByRole("button", { name: "高麗菜芽（高麗菜）" }).click();
		await expect(
			page.getByTestId("crop-disambiguation-card"),
		).not.toBeVisible();
		expect(turnCount).toBe(1);
	});
});
