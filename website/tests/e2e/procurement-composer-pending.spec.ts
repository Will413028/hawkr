import { expect, type Page, test } from "@playwright/test";
import {
	fulfillConnect,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { mockProcurementUnary } from "./helpers/mock-procurement-connect";
import { gotoStreamed } from "./helpers/streamed-navigation";

const PATH = "/account/messages/assistant/demo";
/** 帶 conversationId 的深連結；GetRuntimeSnapshot 以它為 key。 */
const CONVERSATION_ID = "1234567890123456780";
const REQUEST_ID = "1234567890123456781";
const RUN_ID = "1234567890123456801";
const SUBMIT_TURN_URL =
	/\/api\/connect\/hawkr\.procurement\.v1alpha1\.BuyerProcurementAppService\/SubmitTurn(?:\?|$)/;

/** SubmitTurn 停在飛行中：pending 是這個 spec 觀察的狀態，讓它持續夠久才斷言得到，否則會變成跟回應賽跑的 flake。 */
async function holdSubmitTurn(page: Page, ms: number) {
	await page.route(SUBMIT_TURN_URL, async (route) => {
		await new Promise((resolve) => setTimeout(resolve, ms));
		await fulfillConnect(route, {
			requestId: REQUEST_ID,
			conversationId: CONVERSATION_ID,
			runId: RUN_ID,
		}).catch(() => undefined);
	});
}

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

		await mockProcurementUnary(page, "GetSnapshot", () => ({
			requestId: REQUEST_ID,
			revisionId: "1234567890123456790",
			status: "REQUEST_STATUS_CLARIFYING",
			slots: {},
			missingRequired: ["SLOT_KEY_ITEM"],
			runs: [],
			activeRunId: "",
		}));
		await holdSubmitTurn(page, 10_000);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await gotoStreamed(page, PATH);
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

		const stop = page.getByTestId("agent-stop-run");
		await expect(stop).toBeVisible();
		await expect(stop).toBeDisabled();
		await expect(stop).toContainText("停止處理");
		await expect(stop).not.toContainText("規劃中");
		await expect(
			page.getByText("目前無法確認這一輪的執行狀態，暫時不能停止"),
		).toBeAttached();

		const box = await stop.boundingBox();
		expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
		expect(box?.width ?? 0).toBeGreaterThanOrEqual(44);
	});

	test("拿得到 runId 時按得下去，送出的取消命令帶著那個 runId", async ({
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

		await mockProcurementUnary(page, "GetConversationMessages", () => ({
			conversationId: CONVERSATION_ID,
			requestId: REQUEST_ID,
			status: "REQUEST_STATUS_CLARIFYING",
			title: "蘋果 100 公斤",
			messages: [],
		}));
		await mockProcurementUnary(page, "GetSnapshot", () => ({
			requestId: REQUEST_ID,
			revisionId: "1234567890123456790",
			status: "REQUEST_STATUS_CLARIFYING",
			slots: {},
			missingRequired: ["SLOT_KEY_QUANTITY"],
			runs: [],
			activeRunId: "",
		}));

		await mockProcurementUnary(page, "GetRuntimeSnapshot", () => ({
			conversationId: CONVERSATION_ID,
			requestId: REQUEST_ID,
			status: "REQUEST_STATUS_CLARIFYING",
			title: "蘋果 100 公斤",
			messages: [],
			activeRunId: RUN_ID,
			runs: [
				{
					runId: RUN_ID,
					runType: "RUN_TYPE_TURN",
					status: "RUN_STATUS_RUNNING",
					lastCursor: "3",
					terminal: false,
				},
			],
		}));
		await holdSubmitTurn(page, 20_000);

		const cancelBodies: unknown[] = [];
		await mockProcurementUnary(page, "CancelRun", (request) => {
			cancelBodies.push(request);
			return {
				runId: RUN_ID,
				runStatus: "RUN_STATUS_CANCEL_REQUESTED",
				cancelDisposition: "CANCEL_DISPOSITION_REQUESTED",
			};
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await gotoStreamed(
			page,
			`/account/messages/assistant/${CONVERSATION_ID}`,
		);
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒");
		await page.getByRole("button", { name: "送出" }).click();

		const stop = page.getByTestId("agent-stop-run");
		await expect(stop).toBeEnabled();
		await stop.click();

		await expect(page.getByTestId("agent-stop-status")).toContainText(
			"已送出停止，正在收尾這一輪。",
		);
		expect(cancelBodies).toEqual([{ runId: RUN_ID }]);
	});
});
