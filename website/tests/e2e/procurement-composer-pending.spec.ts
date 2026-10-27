import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

// V.11.1 對執行中的 composer 有兩條明文要求：停用時要講清楚為什麼不能輸入，
// 以及右下角必須有一顆「停止處理」。稿 01 的像素逐字畫著 placeholder
// 「執行中，暫時無法輸入訊息...」與那顆停止鈕。
//
// 「規劃中」那個標記已經退場：後端把取消觀察器接上 turn pipeline、並把
// `runVersion` 放進 runtime snapshot 之後，這顆按鈕是真的能停的。但它仍然只在
// **拿得到這一輪的 runId ＋ runVersion** 時才按得下去——沒有 CAS token 的取消
// 請求只會換回一個 400，而買家看到的會是「停止失敗」。所以本檔兩支測試分別鎖
// 住那兩半：token 拿不到時看得到但按不下去，拿得到時按下去真的送出命令。
const PATH = "/account/messages/assistant/demo";
/** 帶 conversationId 的深連結；runtime-snapshot 以它為 key。 */
const CONVERSATION_ID = "1234567890123456780";
const REQUEST_ID = "1234567890123456781";
const RUN_ID = "1234567890123456801";

test.describe("procurement composer — 執行中的停用態", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName }) => browserName !== "chromium",
		"single representative browser is enough for this mocked-gateway flow",
	);

	test("送出後 composer 說明為何不能輸入，並顯示不可操作的「停止處理」", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(45_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";

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

		// Messages shell fixtures——形狀照 procurement-assistant-clarify.spec.ts，
		// 那支的 setup 是 CI 上驗證過走得到 live workspace 的那一份。
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
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/snapshot",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456790",
							status: "CLARIFYING",
							slots: {},
							missingRequired: ["item"],
						},
					}),
				}),
		);

		// turn 停在飛行中：pending 是這支 spec 唯一要觀察的狀態，讓它持續夠久
		// 才斷言得到，否則會變成跟回應賽跑的 flake。
		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns",
			async (route) => {
				await new Promise((resolve) => setTimeout(resolve, 10_000));
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: {} }),
				});
			},
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await page.goto(PATH, { waitUntil: "domcontentloaded" });
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(textarea).toBeDisabled();
		await expect(textarea).toHaveAttribute(
			"placeholder",
			"執行中，暫時無法輸入訊息...",
		);

		// 這一份 fixture 沒有 runtime snapshot（catch-all 回空物件），所以拿不到
		// 這一輪的 CAS token。按鈕必須看得到但按不下去：消失等於讓人以為自己漏看
		// 了，按得下去則會送出一個必然被 400 擋下的請求。
		const stop = page.getByTestId("agent-stop-run");
		await expect(stop).toBeVisible();
		await expect(stop).toBeDisabled();
		await expect(stop).toContainText("停止處理");
		await expect(stop).not.toContainText("規劃中");
		await expect(
			page.getByText("目前無法確認這一輪的執行狀態，暫時不能停止"),
		).toBeAttached();

		// V.11.1 A11y 列：觸控區至少 44×44。
		const box = await stop.boundingBox();
		expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
		expect(box?.width ?? 0).toBeGreaterThanOrEqual(44);
	});

	test("拿得到 runId 與 runVersion 時按得下去，送出的命令帶著 CAS token", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(45_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";

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

		// 深連結還原：transcript 給 requestId，snapshot 給需求狀態。
		await page.route(
			`**/api/v1/app/buyers/me/procurement/conversations/${CONVERSATION_ID}/messages`,
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							conversationId: CONVERSATION_ID,
							requestId: REQUEST_ID,
							status: "CLARIFYING",
							title: "蘋果 100 公斤",
							records: [],
						},
					}),
				}),
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
							requestId: REQUEST_ID,
							revisionId: "1234567890123456790",
							status: "CLARIFYING",
							slots: {},
							missingRequired: ["quantity"],
						},
					}),
				}),
		);

		// 停止鈕的 CAS token 唯一來源。
		await page.route(
			"**/api/v1/app/buyers/me/procurement/conversations/*/runtime-snapshot",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							conversationId: CONVERSATION_ID,
							requestId: REQUEST_ID,
							status: "CLARIFYING",
							title: "蘋果 100 公斤",
							messages: [],
							activeRunId: RUN_ID,
							runs: [
								{
									runId: RUN_ID,
									runType: "TURN",
									status: "RUNNING",
									lastSeqId: "3",
									minAvailableSeqId: "1",
									terminal: false,
									runVersion: "7",
								},
							],
							artifacts: [],
						},
					}),
				}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns",
			async (route) => {
				await new Promise((resolve) => setTimeout(resolve, 20_000));
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: {} }),
				});
			},
		);

		const cancelBodies: unknown[] = [];
		await page.route(
			`**/api/v1/app/buyers/me/procurement/runs/${RUN_ID}/cancel`,
			(route) => {
				cancelBodies.push(route.request().postDataJSON());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							cancelDisposition: "REQUESTED",
							runId: RUN_ID,
							runStatus: "CANCELLING",
							runVersion: "8",
							schemaVersion: "1",
						},
					}),
				});
			},
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await page.goto(
			`/account/messages/assistant/${CONVERSATION_ID}`,
			{ waitUntil: "domcontentloaded" },
		);
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒");
		await page.getByRole("button", { name: "送出" }).click();

		const stop = page.getByTestId("agent-stop-run");
		await expect(stop).toBeEnabled();
		await stop.click();

		// 只有 REQUESTED 講「正在停止」——這一句不得出現在三個終態 disposition 上。
		await expect(page.getByTestId("agent-stop-status")).toContainText(
			"已送出停止，正在收尾這一輪。",
		);
		expect(cancelBodies).toEqual([{ expectedRunVersion: "7", reason: "" }]);
	});
});
