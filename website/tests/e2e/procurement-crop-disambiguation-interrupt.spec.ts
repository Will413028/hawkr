import { expect, type Page, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import {
	ConnectFailure,
	mockProcurementUnary,
	mockWatchRun,
	turnResultJson,
	watchFrame,
} from "./helpers/mock-procurement-connect";
import { gotoStreamed } from "./helpers/streamed-navigation";

const PROCUREMENT_PATH = "/account/messages/assistant/procurement";

const REQUEST_ID = "1234567890123456788";
const REVISION_ID = "1234567890123456791";
const CONVERSATION_ID = "9876543210987654321";
const IDS = {
	requestId: REQUEST_ID,
	conversationId: CONVERSATION_ID,
	revisionId: REVISION_ID,
};

const CROP_QUESTION = {
	slotKey: "SLOT_KEY_ITEM_DISAMBIGUATION",
	prompt: "你說的「高麗菜」是指哪一種？",
	options: [
		{ optionId: "opt-1", label: "甘藍" },
		{ optionId: "opt-2", label: "高麗菜芽（高麗菜）" },
	],
	allowFreeText: false,
	canSkip: false,
};

function snapshotJson(extra: Record<string, unknown> = {}) {
	return {
		requestId: REQUEST_ID,
		revisionId: REVISION_ID,
		status: "REQUEST_STATUS_CLARIFYING",
		slots: { item: "高麗菜" },
		missingRequired: [],
		runs: [],
		activeRunId: "",
		...extra,
	};
}

function openConfirmation(interruptId: string) {
	return {
		interruptId,
		kind: "INTERRUPT_KIND_CLARIFICATION",
		question: CROP_QUESTION,
	};
}

/**
 * 有未回覆的中斷（INTERRUPT_KIND_CLARIFICATION）時的覆蓋：turn_result 帶 interrupt 落地、選項走 ResumeRun
 * 再 WatchRun 取回答結果、重新整理後從 GetSnapshot 還原、SubmitTurn 的 FAILED_PRECONDITION 對 reason 敏感。
 *
 * wire 形狀（proto3 JSON、streaming envelope）見 `helpers/mock-procurement-connect.ts`。
 */
test.describe("procurement assistant — interrupt／resume（Connect）", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName }) => browserName !== "chromium",
		"single representative browser is enough for this mocked-gateway flow",
	);

	async function setupWorkspaceShell(page: Page, appBaseURL: string) {
		await page.route("**/api/v1/**", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			}),
		);
		await seedStoredActiveRole(page, "buyer");
		await setupGatewayMocks(page, {
			identity: { accountId: "300", roles: ["buyer"] },
			session: "identity",
		});
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: appBaseURL }]);
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
	}

	function composer(page: Page) {
		return {
			textarea: page.locator("#procurement-clarify-input"),
			send: page.getByRole("button", { name: /^(送出|傳送訊息中…)$/ }),
		};
	}

	test("turn_result 帶 interrupt 落地：問題卡出現、composer 鎖住，選項走 ResumeRun；回答 run 的結果落地後解鎖", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupWorkspaceShell(page, baseURL ?? "http://localhost:3000");

		await mockProcurementUnary(page, "SubmitTurn", () => ({
			requestId: REQUEST_ID,
			conversationId: CONVERSATION_ID,
			runId: "run-1",
		}));
		await mockProcurementUnary(page, "GetSnapshot", () =>
			snapshotJson({ openConfirmation: openConfirmation("int-1") }),
		);
		const resumeRequests: Record<string, unknown>[] = [];
		await mockProcurementUnary(page, "ResumeRun", (request) => {
			resumeRequests.push(request);
			return { runId: "run-2", runStatus: "RUN_STATUS_RUNNING" };
		});
		await mockWatchRun(page, ({ runId }) =>
			runId === "run-1"
				? [
						watchFrame.accepted("c1", "run-1", CONVERSATION_ID),
						watchFrame.turnResult(
							"c2",
							turnResultJson(IDS, {
								reply: "這個品項有幾種可能，麻煩選一下～",
								question: CROP_QUESTION,
								interrupt: {
									interruptId: "int-1",
									kind: "INTERRUPT_KIND_CLARIFICATION",
								},
							}),
						),
						watchFrame.finished("c3"),
					]
				: [
						watchFrame.accepted("d1", "run-2", CONVERSATION_ID),
						watchFrame.turnResult(
							"d2",
							turnResultJson(IDS, { reply: "高麗菜芽記下了。" }),
						),
						watchFrame.finished("d3"),
					],
		);

		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);
		await expect(page.locator('[data-assistant-mode="live"]')).toBeVisible();

		const { textarea, send } = composer(page);
		await textarea.fill("我要訂高麗菜");
		await send.click();

		await expect(page.getByTestId("clarify-question-card")).toBeVisible({
			timeout: 10_000,
		});
		await expect(textarea).toBeDisabled();
		await expect(send).toBeDisabled();

		await page.getByRole("button", { name: "高麗菜芽（高麗菜）" }).click();

		await expect(page.getByText("高麗菜芽記下了。")).toBeVisible({
			timeout: 10_000,
		});
		await expect(textarea).not.toBeDisabled();
		expect(resumeRequests).toHaveLength(1);
		expect(resumeRequests[0]).toMatchObject({
			interruptId: "int-1",
			clarification: { optionId: "opt-2" },
		});
		expect(resumeRequests[0].clientRequestId).toEqual(expect.any(String));
	});

	test("SubmitTurn 回 APPROVAL_PENDING：顯示伺服器訊息，並靠 GetSnapshot 自我校正回問題卡", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupWorkspaceShell(page, baseURL ?? "http://localhost:3000");
		const APPROVAL_MESSAGE =
			"這筆需求正在等你回覆上一個問題，我先幫你刷新畫面。";

		let submits = 0;
		await mockProcurementUnary(page, "SubmitTurn", () => {
			submits += 1;
			if (submits === 1) {
				return {
					requestId: REQUEST_ID,
					conversationId: CONVERSATION_ID,
					runId: "run-1",
				};
			}
			throw new ConnectFailure(
				"failed_precondition",
				APPROVAL_MESSAGE,
				"APPROVAL_PENDING",
			);
		});
		await mockWatchRun(page, () => [
			watchFrame.accepted("c1", "run-1", CONVERSATION_ID),
			watchFrame.turnResult(
				"c2",
				turnResultJson(IDS, {
					reply: "好的，水果禮盒記下了，還有什麼需要補充的嗎？",
				}),
			),
			watchFrame.finished("c3"),
		]);
		let snapshots = 0;
		await mockProcurementUnary(page, "GetSnapshot", () => {
			snapshots += 1;
			return snapshots === 1
				? snapshotJson()
				: snapshotJson({ openConfirmation: openConfirmation("int-heal") });
		});

		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);
		const { textarea, send } = composer(page);

		await textarea.fill("我要訂水果禮盒");
		await send.click();
		await expect(
			page.getByText("好的，水果禮盒記下了，還有什麼需要補充的嗎？"),
		).toBeVisible({ timeout: 10_000 });

		await textarea.fill("再幫我確認一下");
		await send.click();

		await expect(page.getByText(APPROVAL_MESSAGE)).toBeVisible({
			timeout: 10_000,
		});
		await expect(page.getByTestId("clarify-question-card")).toBeVisible({
			timeout: 10_000,
		});
		await expect(textarea).toBeDisabled();
	});

	test("SubmitTurn 的其他失敗只顯示伺服器訊息，不觸發 GetSnapshot 校正", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupWorkspaceShell(page, baseURL ?? "http://localhost:3000");
		const CONFLICT_MESSAGE = "這則訊息的內容和先前送出的不同，請重新輸入。";

		await mockProcurementUnary(page, "SubmitTurn", () => {
			throw new ConnectFailure("invalid_argument", CONFLICT_MESSAGE);
		});
		let snapshots = 0;
		await mockProcurementUnary(page, "GetSnapshot", () => {
			snapshots += 1;
			return snapshotJson();
		});

		await gotoStreamed(page, PROCUREMENT_PATH);
		const { textarea, send } = composer(page);
		await textarea.fill("我要訂水果禮盒");
		await send.click();

		await expect(page.getByText(CONFLICT_MESSAGE)).toBeVisible({
			timeout: 10_000,
		});
		expect(snapshots).toBe(0);
	});

	test("重新整理一個有未回覆中斷的需求（?requestId= 深連結）：問題卡與 ResumeRun 不需要先跑一輪 live turn", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupWorkspaceShell(page, baseURL ?? "http://localhost:3000");

		await mockProcurementUnary(page, "GetSnapshot", () =>
			snapshotJson({
				slots: { item: "地瓜" },
				openConfirmation: {
					interruptId: "int-restore",
					kind: "INTERRUPT_KIND_CLARIFICATION",
					question: {
						...CROP_QUESTION,
						prompt: "你說的「地瓜」是指哪一種？",
						options: [{ optionId: "opt-sp", label: "地瓜（甘藷）" }],
					},
				},
			}),
		);
		const resumeRequests: Record<string, unknown>[] = [];
		await mockProcurementUnary(page, "ResumeRun", (request) => {
			resumeRequests.push(request);
			return { runId: "run-r", runStatus: "RUN_STATUS_RUNNING" };
		});
		await mockWatchRun(page, () => [
			watchFrame.accepted("c1", "run-r", CONVERSATION_ID),
			watchFrame.turnResult(
				"c2",
				turnResultJson(IDS, { reply: "地瓜記下了。" }),
			),
			watchFrame.finished("c3"),
		]);

		const response = await gotoStreamed(
			page,
			`${PROCUREMENT_PATH}?requestId=${REQUEST_ID}`,
		);
		expect(response?.status()).toBeLessThan(400);

		await expect(page.getByTestId("clarify-question-card")).toBeVisible({
			timeout: 10_000,
		});
		const { textarea } = composer(page);
		await expect(textarea).toBeDisabled();

		await page.getByRole("button", { name: "地瓜（甘藷）" }).click();
		await expect(page.getByText("地瓜記下了。")).toBeVisible({
			timeout: 10_000,
		});
		await expect(textarea).not.toBeDisabled();
		expect(resumeRequests[0]).toMatchObject({
			interruptId: "int-restore",
			clarification: { optionId: "opt-sp" },
		});
	});
});
