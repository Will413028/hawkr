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

const PATH = "/account/messages/assistant/demo";
const PROCUREMENT_PATH = "/account/messages/assistant/procurement";
const HANDOFF_STORAGE_KEY = "hawkr:hawkr-agent:search-handoff:v1";
/** doc 55 §8.1 哨兵：raw chain-of-thought 任何一層命中（DOM/a11y tree）都是上線阻斷。
 * Connect 契約的 `TurnResult` 已沒有 reasoning 欄位，所以 mock 不再送它；斷言保留，
 * 守的是「client 不會自己把這類文字畫出來」。 */
const REASONING_SENTINEL =
	"SENTINEL-RAW-COT-品項已知，城市未知且為必填，優先詢問。";

const REQUEST_ID = "1234567890123456788";
const CONVERSATION_ID = "9876543210987654321";

const SLOT = {
	item: "SLOT_KEY_ITEM",
	quantity: "SLOT_KEY_QUANTITY",
	unit: "SLOT_KEY_UNIT",
	packaging: "SLOT_KEY_PACKAGING",
	city: "SLOT_KEY_DELIVERY_CITY",
} as const;

type Wire = Record<string, unknown>;

/** 第 n 次命令（送出一輪或答題）之後的需求版本。 */
function revisionOf(n: number): string {
	return `123456789012345679${n}`;
}

/** 與前面所有 Messages 外殼請求無關的 REST 預設回應、買家身分與 session。 */
async function setupShell(page: Page, appBaseURL: string) {
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
}

interface ClarifyFlow {
	/** SubmitTurn 收到的請求訊息。 */
	submitTurnRequests: Wire[];
	/** SubmitAnswer 收到的請求訊息。 */
	answerRequests: Wire[];
}

/**
 * 釐清對話的 Connect 流程：`SubmitTurn` 受理後由 `WatchRun` 送出 `turn_result`，`SubmitAnswer` 直接回
 * `TurnResult`，`GetSnapshot` 回最近一次結果的需求狀態（必填都齊備才是 REQUIREMENT_READY）。
 * `turn(n)`／`answer(n, request)` 回 `TurnResult` 的欄位（ID 與版本由這裡補）；n 是命令的序號（送出與答題共用）。
 */
async function mockClarifyFlow(
	page: Page,
	handlers: {
		turn?: (n: number, request: Wire) => Wire;
		answer?: (n: number, request: Wire) => Wire;
	},
): Promise<ClarifyFlow> {
	const flow: ClarifyFlow = { submitTurnRequests: [], answerRequests: [] };
	let commands = 0;
	const results = new Map<string, Wire>();
	let latest: Wire = turnResultJson({
		requestId: REQUEST_ID,
		conversationId: CONVERSATION_ID,
		revisionId: revisionOf(1),
	});
	const build = (n: number, extra: Wire): Wire => {
		const result = turnResultJson(
			{
				requestId: REQUEST_ID,
				conversationId: CONVERSATION_ID,
				revisionId: revisionOf(n),
			},
			extra,
		);
		latest = result;
		return result;
	};

	if (handlers.turn) {
		const turn = handlers.turn;
		await mockProcurementUnary(page, "SubmitTurn", (request) => {
			commands += 1;
			flow.submitTurnRequests.push(request);
			const runId = `run-${commands}`;
			results.set(runId, build(commands, turn(commands, request)));
			return { requestId: REQUEST_ID, conversationId: CONVERSATION_ID, runId };
		});
		await mockWatchRun(page, ({ runId }) => [
			watchFrame.accepted(`${runId}-1`, runId, CONVERSATION_ID),
			watchFrame.turnResult(`${runId}-2`, results.get(runId) ?? {}),
			watchFrame.finished(`${runId}-3`),
		]);
	}
	const answer = handlers.answer;
	await mockProcurementUnary(page, "SubmitAnswer", (request) => {
		commands += 1;
		flow.answerRequests.push(request);
		if (!answer) {
			throw new ConnectFailure("invalid_argument", "答題端點沒有預期被呼叫");
		}
		if (!request.expectedRevisionId || !request.clientRequestId) {
			throw new ConnectFailure("invalid_argument", "bad request");
		}
		return { result: build(commands, answer(commands, request)) };
	});
	await mockProcurementUnary(page, "GetSnapshot", () => {
		const missing = (latest.missingRequired as unknown[] | undefined) ?? [];
		return {
			requestId: REQUEST_ID,
			revisionId: latest.revisionId,
			status:
				missing.length === 0
					? "REQUEST_STATUS_REQUIREMENT_READY"
					: "REQUEST_STATUS_CLARIFYING",
			slots: latest.slots ?? {},
			missingRequired: missing,
			runs: [],
			activeRunId: "",
		};
	});
	return flow;
}

const HANDOFF_ITEM_ONLY = { slots: { item: "水果禮盒" } } as const;

test.describe("procurement assistant — reply/reasoning + no snapshot UI", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName }) => browserName !== "chromium",
		"single representative browser is enough for this mocked-gateway flow",
	);

	test("renders backend reply text, keeps raw reasoning out of the DOM, and reaches ready without a snapshot card", async ({
		page,
		baseURL,
		isMobile,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒" },
				missingRequired: [SLOT.city],
				reply: "好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？",
				steps: [
					{
						code: "UNDERSTAND_INPUT",
						status: "STEP_STATUS_SUCCEEDED",
						label: "正在理解你的需求",
						durationMs: "120",
					},
					{
						code: "EXTRACT_SLOTS",
						status: "STEP_STATUS_SUCCEEDED",
						label: "正在整理採購條件",
						durationMs: "800",
					},
				],
				recordedThisTurn: [
					{ key: SLOT.item, label: "品項", value: "水果禮盒" },
				],
				question: {
					slotKey: SLOT.city,
					prompt: "要送到哪個城市呢？",
					options: [
						{ optionId: "city-taoyuan", label: "桃園市" },
						{ optionId: "city-taipei", label: "台北市" },
					],
					allowFreeText: true,
				},
			}),
			answer: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，那我先幫您找桃園能送水果禮盒的賣家，馬上開始搜尋！",
				steps: [
					{
						code: "VALIDATE_SLOTS",
						status: "STEP_STATUS_SUCCEEDED",
						label: "正在檢查條件",
						durationMs: "40",
					},
					{
						code: "REQUIREMENT_READY",
						status: "STEP_STATUS_SUCCEEDED",
						label: "必填條件已齊備",
						durationMs: "10",
					},
				],
				recordedThisTurn: [
					{ key: SLOT.city, label: "配送城市", value: "桃園市" },
				],
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await gotoStreamed(page, PATH);
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		const sendButton = page.getByRole("button", { name: "送出" });

		await textarea.fill("我要訂水果禮盒");
		await sendButton.click();
		const buyerBubble = page
			.locator(".hawkr-agent-message-row.is-mine .thread-page__bubble")
			.first();
		await expect(buyerBubble.getByText("你", { exact: true })).toHaveCount(0);
		await expect(
			page.getByText("好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？", {
				exact: true,
			}),
		).toBeVisible({ timeout: 10_000 });

		await expect(page.getByText(REASONING_SENTINEL)).toHaveCount(0);
		await expect(page.getByText("查看思考過程")).toHaveCount(0);
		const domHasSentinel = await page.evaluate(
			(sentinel) => document.documentElement.outerHTML.includes(sentinel),
			REASONING_SENTINEL,
		);
		expect(domHasSentinel).toBe(false);

		await expect(page.getByTestId("agent-activity-rail")).toHaveCount(0);
		await expect(page.getByTestId("agent-processing")).toHaveCount(0);
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);
		await expect(page.getByTestId("agent-step-summary")).toHaveCount(0);
		await expect(page.getByText("正在理解你的需求")).toHaveCount(0);

		const recordedFacts = page.getByRole("list", { name: "已記下的條件" });
		await expect(recordedFacts.getByText("品項：水果禮盒")).toBeVisible();
		await expect(
			recordedFacts
				.locator('[data-slot-key="item"]')
				.locator('[data-icon-name="goods"]'),
		).toBeVisible();
		expect(
			await page.evaluate(
				() => document.documentElement.scrollWidth <= window.innerWidth,
			),
		).toBe(true);

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(
			questionCard.getByRole("button", { name: "台北市" }),
		).toBeVisible();
		const taipeiOption = questionCard.getByRole("button", { name: "台北市" });
		await taipeiOption.hover();
		const optionMark = taipeiOption.locator(
			".hawkr-agent-question-card__option-mark",
		);
		const checkMark = optionMark.locator('[data-icon-name="check"]');
		await expect(checkMark).toHaveCSS("opacity", "1");
		await expect(checkMark.locator("path")).toHaveCount(1);
		await expect(checkMark.locator("rect, circle")).toHaveCount(0);
		await expect(
			questionCard.getByRole("button", { name: "跳過" }),
		).toHaveCount(0);
		await questionCard.getByRole("button", { name: "其他…" }).click();
		const customCity = questionCard.getByRole("textbox", {
			name: "要送到哪個城市呢？的答案",
		});
		await expect(customCity).toBeVisible();
		await expect(customCity).toHaveAttribute("placeholder", "例如：新竹市");

		await expect(page.getByText("需求快照")).toHaveCount(0);
		await expect(page.locator(".assistant-snapshot")).toHaveCount(0);

		await questionCard.getByRole("button", { name: "桃園市" }).click();
		await expect(
			page.getByText(
				"好的，那我先幫您找桃園能送水果禮盒的賣家，馬上開始搜尋！",
				{ exact: true },
			),
		).toBeVisible({ timeout: 10_000 });
		const cityFact = recordedFacts.locator('[data-slot-key="delivery_city"]');
		await expect(cityFact.getByText("配送城市：桃園市")).toBeVisible();
		await expect(cityFact.locator('[data-icon-name="location"]')).toBeVisible();
		await expect(
			page.getByRole("button", { name: "交給 Hawkr Agent 搜尋" }),
		).toBeVisible();
		await expect(page.getByText("必要條件都補齊了")).toHaveCount(0);
		await expect(page).toHaveURL(
			/\/account\/messages\/assistant\/9876543210987654321$/,
		);

		if (isMobile) {
			await page.setViewportSize({ width: 320, height: 800 });
			await page.evaluate(() => {
				document.documentElement.style.fontSize = "200%";
			});
			const taskCard = page.getByRole("region", {
				name: "搜尋候選供應商",
			});
			const assignButton = page.getByRole("button", {
				name: "交給 Hawkr Agent 搜尋",
			});
			await expect(taskCard).toBeVisible();
			await expect(assignButton).toBeVisible();
			for (const target of [
				...(await recordedFacts.all()),
				taskCard,
				assignButton,
			]) {
				expect(
					await target.evaluate(
						(element) => element.scrollWidth <= element.clientWidth,
					),
				).toBe(true);
			}
		}
	});

	test("ready 後 question 仍非 null 時，先問完再派工：問題卡在、搜尋 CTA 不在", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		const flow = await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply:
					"好的，那我先幫您找桃園能送水果禮盒的賣家！方便說一下大概數量嗎？",
				steps: [
					{
						code: "REQUIREMENT_READY",
						status: "STEP_STATUS_SUCCEEDED",
						label: "必填條件已齊備",
						durationMs: "10",
					},
				],
				recordedThisTurn: [
					{ key: SLOT.city, label: "配送城市", value: "桃園市" },
				],
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					allowFreeText: true,
				},
			}),
			answer: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市", quantity: 30 },
				reply: "收到，數量先記為 30。",
				recordedThisTurn: [{ key: SLOT.quantity, label: "數量", value: "30" }],
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(page.getByTestId("clarify-question-card")).toBeVisible();
		await expect(
			page.getByRole("button", { name: "交給 Hawkr Agent 搜尋" }),
		).toHaveCount(0);

		const questionCard = page.getByTestId("clarify-question-card");
		const inlineAnswer = questionCard.getByRole("textbox", {
			name: "大概需要多少數量？的答案",
		});
		await inlineAnswer.fill("30 箱");
		await questionCard.getByRole("button", { name: "送出答案" }).click();
		await expect(page.getByText("數量請只輸入數字，例如 50。")).toBeVisible();
		expect(flow.answerRequests).toHaveLength(0);
		await inlineAnswer.fill("30");
		await questionCard.getByRole("button", { name: "送出答案" }).click();
		await expect(
			page.getByText("收到，數量先記為 30。", { exact: true }),
		).toBeVisible();
		expect(flow.answerRequests).toHaveLength(1);
		const answerBody = flow.answerRequests[0];
		expect(answerBody.slotKey).toBe(SLOT.quantity);
		expect(answerBody.freeText).toBe("30");
		expect(answerBody.optionId).toBeFalsy();

		await expect(page.getByTestId("clarify-question-card")).toHaveCount(0);
		await expect(
			page.getByRole("button", { name: "交給 Hawkr Agent 搜尋" }),
		).toBeVisible();
	});

	test("選填欄位問題卡可點跳過；送出的 skip:true 不與 optionId/freeText 同時出現，泡泡顯示「略過」", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		const flow = await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply:
					"好的，那我先幫您找桃園能送水果禮盒的賣家！方便說一下大概數量嗎？",
				recordedThisTurn: [
					{ key: SLOT.city, label: "配送城市", value: "桃園市" },
				],
				question: {
					slotKey: SLOT.packaging,
					prompt: "包裝上有什麼需求嗎？",
					options: [],
					allowFreeText: true,
					canSkip: true,
				},
			}),
			answer: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "了解，那我先不記包裝，其他資訊都齊了。",
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		const skipButton = questionCard.getByRole("button", { name: "跳過" });
		await expect(skipButton).toBeVisible();
		await skipButton.click();

		await expect(page.getByText("略過", { exact: true })).toBeVisible();
		await expect(
			page.getByText("了解，那我先不記包裝，其他資訊都齊了。", {
				exact: true,
			}),
		).toBeVisible({ timeout: 10_000 });

		expect(flow.answerRequests).toHaveLength(1);
		const answerBody = flow.answerRequests[0];
		expect(answerBody.skip).toBe(true);
		expect(answerBody.slotKey).toBe(SLOT.packaging);
		expect(answerBody.optionId).toBeFalsy();
		expect(answerBody.freeText).toBeFalsy();
	});

	test("報價單前提欄位（quantity／delivery_date）不得渲染跳過按鈕——後端對它們的 skip 一律 422", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，方便說一下大概數量嗎？",
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					allowFreeText: true,
				},
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(questionCard).toContainText("大概需要多少數量？");
		await expect(
			questionCard.getByRole("button", { name: "跳過" }),
		).toHaveCount(0);
		await expect(questionCard).not.toContainText("或選擇跳過");
	});

	test("quantity 複合問題卡——單位 chip 在數字欄有效前 disabled，點 chip 一次送出 freeText+unitOptionId", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		const flow = await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，方便說一下大概數量嗎？",
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					unitOptions: [
						{ optionId: "unit-basket", label: "籃" },
						{ optionId: "unit-box", label: "箱" },
					],
					allowFreeText: true,
				},
			}),
			answer: () => ({
				slots: {
					item: "水果禮盒",
					deliveryCity: "桃園市",
					quantity: 50,
					unit: "箱",
				},
				reply: "收到，50 箱先記下了。",
				recordedThisTurn: [
					{ key: SLOT.quantity, label: "數量", value: "50" },
					{ key: SLOT.unit, label: "單位", value: "箱" },
				],
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(questionCard).toHaveAttribute(
			"data-question-kind",
			"quantity-unit",
		);

		const boxChip = questionCard.getByRole("button", { name: "箱" });
		await expect(boxChip).toBeDisabled();
		await expect(
			questionCard.getByRole("button", { name: "送出答案" }),
		).toHaveCount(0);

		const inlineAnswer = questionCard.getByRole("textbox", {
			name: "大概需要多少數量？的答案",
		});
		await inlineAnswer.fill("50");
		await expect(boxChip).toBeEnabled();
		await boxChip.click();

		await expect(
			page.getByText("收到，50 箱先記下了。", { exact: true }),
		).toBeVisible();
		await expect(page.getByText("50 箱", { exact: true })).toBeVisible();

		expect(flow.answerRequests).toHaveLength(1);
		const answerBody = flow.answerRequests[0];
		expect(answerBody.slotKey).toBe(SLOT.quantity);
		expect(answerBody.freeText).toBe("50");
		expect(answerBody.unitOptionId).toBe("unit-box");
		expect(answerBody.optionId).toBeFalsy();

		await expect(page.getByTestId("clarify-question-card")).toHaveCount(0);
	});

	test("quantity 單位追問——沒有數字欄，只有單位 chip，點擊直接送出 unitOptionId（不帶 freeText）", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		const flow = await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市", quantity: 50 },
				reply: "50 先記下了，這批貨要用什麼單位計算？",
				recordedThisTurn: [{ key: SLOT.quantity, label: "數量", value: "50" }],
				question: {
					slotKey: SLOT.quantity,
					prompt: "這批貨要用什麼單位計算？",
					options: [],
					unitOptions: [
						{ optionId: "unit-basket", label: "籃" },
						{ optionId: "unit-box", label: "箱" },
					],
					allowFreeText: false,
				},
			}),
			answer: () => ({
				slots: {
					item: "水果禮盒",
					deliveryCity: "桃園市",
					quantity: 50,
					unit: "箱",
				},
				reply: "收到，單位記為箱。",
				recordedThisTurn: [{ key: SLOT.unit, label: "單位", value: "箱" }],
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒 50 個，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(questionCard).toHaveAttribute("data-question-kind", "unit");
		await expect(
			questionCard.getByRole("textbox", {
				name: "這批貨要用什麼單位計算？的答案",
			}),
		).toHaveCount(0);

		const boxChip = questionCard.getByRole("button", { name: "箱" });
		await expect(boxChip).toBeEnabled();
		await boxChip.click();

		await expect(
			page.getByText("收到，單位記為箱。", { exact: true }),
		).toBeVisible();
		const latestRecordedConditions = page
			.getByRole("list", { name: "已記下的條件" })
			.last();
		await expect(
			latestRecordedConditions.getByText("箱", { exact: true }),
		).toBeVisible();

		expect(flow.answerRequests).toHaveLength(1);
		const answerBody = flow.answerRequests[0];
		expect(answerBody.slotKey).toBe(SLOT.quantity);
		expect(answerBody.unitOptionId).toBe("unit-box");
		expect(answerBody.freeText).toBeFalsy();
		expect(answerBody.optionId).toBeFalsy();
	});

	test("複合問題卡活躍時，底部 composer 打字送出走一般 SubmitTurn（不是 SubmitAnswer）——避免缺單位被拒", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		const flow = await mockClarifyFlow(page, {
			turn: (n) =>
				n >= 2
					? {
							slots: {
								item: "水果禮盒",
								deliveryCity: "桃園市",
								quantity: 50,
								unit: "箱",
							},
							reply: "收到，50 箱先記下了。",
							recordedThisTurn: [
								{ key: SLOT.quantity, label: "數量", value: "50" },
								{ key: SLOT.unit, label: "單位", value: "箱" },
							],
						}
					: {
							slots: { item: "水果禮盒", deliveryCity: "桃園市" },
							reply: "好的，方便說一下大概數量嗎？",
							question: {
								slotKey: SLOT.quantity,
								prompt: "大概需要多少數量？",
								options: [],
								unitOptions: [{ optionId: "unit-box", label: "箱" }],
								allowFreeText: true,
							},
						},
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();

		await textarea.fill("50箱");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(
			page.getByText("收到，50 箱先記下了。", { exact: true }),
		).toBeVisible();
		expect(flow.submitTurnRequests).toHaveLength(2);
		expect(flow.answerRequests).toHaveLength(0);
	});

	test("複合問題卡／單位追問卡的單位 chips 群組不會長出「其他」按鈕——後端沒有自由輸入單位的路徑", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，方便說一下大概數量嗎？",
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					unitOptions: [{ optionId: "unit-box", label: "箱" }],
					allowFreeText: true,
				},
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(
			questionCard.getByRole("button", { name: /其他|自行輸入/ }),
		).toHaveCount(0);
	});

	test("單位追問卡掛載時，鍵盤焦點落在第一顆單位 chip——不落回 <body>", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒" },
				missingRequired: [SLOT.city],
				reply: "要送到哪個城市呢？",
				question: {
					slotKey: SLOT.city,
					prompt: "要送到哪個城市呢？",
					options: [{ optionId: "city-taoyuan", label: "桃園市" }],
					allowFreeText: true,
				},
			}),
			answer: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市", quantity: 50 },
				reply: "50 先記下了，這批貨要用什麼單位計算？",
				recordedThisTurn: [
					{ key: SLOT.city, label: "配送城市", value: "桃園市" },
				],
				question: {
					slotKey: SLOT.quantity,
					prompt: "這批貨要用什麼單位計算？",
					options: [],
					unitOptions: [
						{ optionId: "unit-box", label: "箱" },
						{ optionId: "unit-kg", label: "公斤" },
					],
					allowFreeText: false,
				},
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒");
		await page.getByRole("button", { name: "送出" }).click();

		const cityQuestionCard = page.getByTestId("clarify-question-card");
		await expect(cityQuestionCard).toBeVisible();
		await cityQuestionCard.getByRole("button", { name: "桃園市" }).click();

		const unitQuestionCard = page.getByTestId("clarify-question-card");
		await expect(unitQuestionCard).toHaveAttribute(
			"data-question-kind",
			"unit",
		);
		await expect(
			unitQuestionCard.getByRole("button", { name: "箱" }),
		).toBeFocused();
	});

	test("複合問題卡：數字欄輸入超過上限（>1,000,000）時單位 chip 維持 disabled", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，方便說一下大概數量嗎？",
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					unitOptions: [{ optionId: "unit-box", label: "箱" }],
					allowFreeText: true,
				},
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		const boxChip = questionCard.getByRole("button", { name: "箱" });
		await expect(boxChip).toBeDisabled();

		const inlineAnswer = questionCard.getByRole("textbox", {
			name: "大概需要多少數量？的答案",
		});
		await inlineAnswer.fill("99999999");
		await expect(boxChip).toBeDisabled();

		await inlineAnswer.fill("50");
		await expect(boxChip).toBeEnabled();
	});

	test("複合問題卡：點選非第一顆單位 chip 也能正確送出對應的 unitOptionId／unitLabel", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		const flow = await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，方便說一下大概數量嗎？",
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					unitOptions: [
						{ optionId: "unit-box", label: "箱" },
						{ optionId: "unit-kg", label: "公斤" },
						{ optionId: "unit-catty", label: "台斤" },
					],
					allowFreeText: true,
				},
			}),
			answer: () => ({
				slots: {
					item: "水果禮盒",
					deliveryCity: "桃園市",
					quantity: 50,
					unit: "台斤",
				},
				reply: "收到，50 台斤先記下了。",
				recordedThisTurn: [
					{ key: SLOT.quantity, label: "數量", value: "50" },
					{ key: SLOT.unit, label: "單位", value: "台斤" },
				],
			}),
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		const inlineAnswer = questionCard.getByRole("textbox", {
			name: "大概需要多少數量？的答案",
		});
		await inlineAnswer.fill("50");

		const cattyChip = questionCard.getByRole("button", { name: "台斤" });
		await expect(cattyChip).toBeEnabled();
		await cattyChip.click();

		await expect(
			page.getByText("收到，50 台斤先記下了。", { exact: true }),
		).toBeVisible();

		expect(flow.answerRequests).toHaveLength(1);
		const answerBody = flow.answerRequests[0];
		expect(answerBody.unitOptionId).toBe("unit-catty");
		expect(answerBody.freeText).toBe("50");
	});

	test("複合問題卡送出後端回 INVALID_ARGUMENT——錯誤顯示在對話尾端，問題卡不會憑空消失", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");

		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒", deliveryCity: "桃園市" },
				reply: "好的，方便說一下大概數量嗎？",
				question: {
					slotKey: SLOT.quantity,
					prompt: "大概需要多少數量？",
					options: [],
					unitOptions: [{ optionId: "unit-box", label: "箱" }],
					allowFreeText: true,
				},
			}),
			answer: () => {
				throw new ConnectFailure(
					"invalid_argument",
					"答案不符合這個欄位的格式",
				);
			},
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		const inlineAnswer = questionCard.getByRole("textbox", {
			name: "大概需要多少數量？的答案",
		});
		await inlineAnswer.fill("50");
		await questionCard.getByRole("button", { name: "箱" }).click();

		await expect(
			page.getByText("答案不符合這個欄位的格式", { exact: false }),
		).toBeVisible();
		const questionCardAfterError = page.getByTestId("clarify-question-card");
		await expect(questionCardAfterError).toBeVisible();
		await expect(
			questionCardAfterError.getByRole("button", { name: "箱" }),
		).toBeDisabled();
		await questionCardAfterError
			.getByRole("textbox", { name: "大概需要多少數量？的答案" })
			.fill("50");
		await expect(
			questionCardAfterError.getByRole("button", { name: "箱" }),
		).toBeEnabled();
	});

	test("a search draft handed off from the entry link prefills the composer, focuses only on desktop, and never auto-sends", async ({
		page,
		baseURL,
		isMobile,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");
		const flow = await mockClarifyFlow(page, {
			turn: () => ({ ...HANDOFF_ITEM_ONLY }),
		});

		await page.addInitScript(
			([key, query]) => {
				sessionStorage.setItem("hawkr:initial-loader-seen", "1");
				sessionStorage.setItem(
					key,
					JSON.stringify({
						version: 1,
						id: "handoff-e2e-procurement",
						query,
						source: "search-page",
						createdAt: Date.now(),
					}),
				);
			},
			[HANDOFF_STORAGE_KEY, "企業送禮水果盒"] as const,
		);

		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await expect(textarea).toHaveValue("企業送禮水果盒");
		if (isMobile) {
			await expect(textarea).not.toBeFocused();
		} else {
			await expect(textarea).toBeFocused();
		}
		expect(flow.submitTurnRequests).toHaveLength(0);
	});

	test("「開新對話」在 procurement 入口對話開始（URL 已被 replaceState 改寫）後真正開出新對話", async ({
		page,
		baseURL,
		isMobile,
	}) => {
		test.skip(!!isMobile, "側欄控制項在手機的 thread 視圖不存在");
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");
		await mockClarifyFlow(page, {
			turn: () => ({
				slots: { item: "水果禮盒" },
				missingRequired: [SLOT.city],
				reply: "好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？",
			}),
		});
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		await gotoStreamed(page, PROCUREMENT_PATH);
		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒");
		await page.getByRole("button", { name: "送出" }).click();
		await expect(
			page.getByText("好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？", {
				exact: true,
			}),
		).toBeVisible({ timeout: 10_000 });
		await expect(page).toHaveURL(
			/\/account\/messages\/assistant\/9876543210987654321$/,
		);

		await page.getByRole("button", { name: "開新對話" }).click();
		await expect(page).toHaveURL(
			/\/account\/messages\/assistant\/procurement$/,
		);
		await expect(page.locator('[data-assistant-mode="live"]')).toHaveAttribute(
			"data-assistant-view",
			"entry",
		);
		await expect(
			page.getByRole("heading", { name: "今天想請 Hawkr 幫你完成什麼？" }),
		).toBeVisible();
		await expect(page.locator("#procurement-clarify-input")).toHaveCount(1);
		await expect(textarea).toHaveValue("");
	});

	test("對話 composer 支援換行與中文組字；44–120px 自動高度且長文不水平溢出", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		await setupShell(page, baseURL ?? "http://localhost:3000");
		const flow = await mockClarifyFlow(page, {
			turn: (n) => ({
				slots: { item: "番茄" },
				reply: n === 1 ? "已記下番茄需求。" : "已更新採購需求。",
			}),
		});
		const turnBodies = flow.submitTurnRequests;
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		await gotoStreamed(page, PROCUREMENT_PATH);
		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要番茄");
		await page.getByRole("button", { name: "送出", exact: true }).click();
		await expect(
			page.getByText("已記下番茄需求。", { exact: true }),
		).toBeVisible();
		await expect(textarea).toBeEnabled();
		await expect(textarea).toHaveValue("");
		const height = () =>
			textarea.evaluate((node) => node.getBoundingClientRect().height);
		await expect.poll(height).toBeGreaterThanOrEqual(44);
		await expect.poll(height).toBeLessThanOrEqual(120);
		const singleLineHeight = await height();

		await textarea.fill("明天配送");
		await textarea.press("Shift+Enter");
		await expect(textarea).toHaveValue("明天配送\n");
		expect(turnBodies).toHaveLength(1);
		await textarea.dispatchEvent("keydown", {
			key: "Enter",
			code: "Enter",
			isComposing: true,
			keyCode: 229,
			bubbles: true,
		});
		await expect(textarea).toHaveValue("明天配送\n");
		expect(turnBodies).toHaveLength(1);

		const longDraft = `${"連續的長段採購需求".repeat(30)}\n${"每箱分開包裝\n".repeat(10)}`;
		await textarea.fill(longDraft);
		await expect.poll(height).toBeGreaterThan(singleLineHeight);
		await expect.poll(height).toBeLessThanOrEqual(120);
		expect(
			await textarea.evaluate(
				(node) => node.scrollWidth <= node.clientWidth + 1,
			),
		).toBe(true);
		expect(
			await page.evaluate(
				() => document.documentElement.scrollWidth <= window.innerWidth,
			),
		).toBe(true);
		expect(turnBodies).toHaveLength(1);
		await textarea.fill("明天配送到桃園");
		await textarea.press("Enter");
		await expect(
			page.getByText("已更新採購需求。", { exact: true }),
		).toBeVisible();
		expect(turnBodies).toHaveLength(2);
		expect(turnBodies[1].message).toBe("明天配送到桃園");
	});

	test("主對話獨立處理區顯示 N 列；heartbeat 與 step 完成不清軌，terminal 不留快照", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
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

		await mockProcurementUnary(page, "SubmitTurn", () => ({
			requestId: "1234567890123456788",
			conversationId: "9876543210987654321",
			runId: "1",
		}));
		await mockProcurementUnary(page, "GetSnapshot", () => ({
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			status: "REQUEST_STATUS_CLARIFYING",
			slots: { item: "番茄" },
			missingRequired: ["SLOT_KEY_DELIVERY_CITY"],
			runs: [],
			activeRunId: "",
		}));

		await page.addInitScript(() => {
			const nativeFetch = window.fetch.bind(window);
			window.fetch = async (input, init) => {
				const url =
					typeof input === "string"
						? input
						: input instanceof URL
							? input.href
							: input.url;
				if (!url.includes("BuyerProcurementAppService/WatchRun"))
					return nativeFetch(input as RequestInfo, init);
				const encoder = new TextEncoder();
				const envelope = (flags: number, json: unknown) => {
					const payload = encoder.encode(JSON.stringify(json));
					const frame = new Uint8Array(5 + payload.length);
					frame[0] = flags;
					new DataView(frame.buffer).setUint32(1, payload.length);
					frame.set(payload, 5);
					return frame;
				};
				const step = (
					cursor: string,
					seq: number,
					code: string,
					status: string,
				) =>
					envelope(0, {
						cursor,
						step: { seq, step: { code, status: `STEP_STATUS_${status}` } },
					});
				const frames: Uint8Array[][] = [
					[step("c1", 1, "UNDERSTAND_INPUT", "RUNNING")],
					[step("c2", 2, "UNDERSTAND_INPUT", "RUNNING")],
					[step("c3", 3, "UNDERSTAND_INPUT", "SUCCEEDED")],
					[step("c4", 4, "EXTRACT_SLOTS", "RUNNING")],
					[
						envelope(0, {
							cursor: "c5",
							turnResult: {
								requestId: "1234567890123456788",
								revisionId: "1234567890123456791",
								conversationId: "9876543210987654321",
								slots: { item: "番茄" },
								missingRequired: ["SLOT_KEY_DELIVERY_CITY"],
								reply: "好的，番茄記下了。請問要配送到哪個城市呢？",
								steps: [
									{
										code: "VALIDATE_SLOTS",
										status: "STEP_STATUS_SUCCEEDED",
										text: "TERMINAL_ONLY_STEP_SENTINEL",
										durationMs: "120",
									},
								],
								recordedThisTurn: [
									{ key: "SLOT_KEY_ITEM", label: "品項", value: "番茄" },
								],
							},
						}),
						envelope(0, {
							cursor: "c6",
							finished: { status: "RUN_STATUS_SUCCEEDED" },
						}),
					],
				];
				const body = new ReadableStream<Uint8Array>({
					start(controller) {
						controller.enqueue(
							envelope(0, {
								cursor: "c0",
								runAccepted: {
									runId: "1",
									conversationId: "9876543210987654321",
								},
							}),
						);
						let nextFrame = 0;
						Object.assign(window, {
							__advanceProcurementFrame: () => {
								for (const chunk of frames[nextFrame++]) {
									controller.enqueue(chunk);
								}
								if (nextFrame === frames.length) {
									controller.enqueue(envelope(2, {}));
									controller.close();
								}
							},
						});
					},
				});
				return new Response(body, {
					status: 200,
					headers: { "content-type": "application/connect+json" },
				});
			};
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await gotoStreamed(page, PROCUREMENT_PATH);
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要番茄");
		await page.getByRole("button", { name: "送出" }).click();

		const processing = page.getByTestId("agent-processing");
		await expect(processing).toBeVisible();
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);
		await expect(page.getByTestId("agent-panel-open")).toHaveCount(0);
		await expect(textarea).toBeVisible();
		await expect(textarea).toBeDisabled();
		await expect(textarea).toHaveCount(1);
		expect(
			await processing.evaluate(
				(node) =>
					node.closest(".thread-page") !== null &&
					node.closest(".thread-page__bubble") === null,
			),
		).toBe(true);
		const advanceFrame = () =>
			page.evaluate(() => {
				(
					window as unknown as { __advanceProcurementFrame: () => void }
				).__advanceProcurementFrame();
			});
		await expect
			.poll(() =>
				page.evaluate(
					() =>
						typeof (
							window as Window & { __advanceProcurementFrame?: () => void }
						).__advanceProcurementFrame,
				),
			)
			.toBe("function");
		const rail = processing.getByTestId("agent-activity-rail");
		await expect(rail).toHaveCount(0);
		await advanceFrame();
		await expect(rail).toBeVisible();
		await expect(rail).toContainText("讀你寫的需求");
		await expect(page.locator(".thread-page__typing")).toHaveCount(0);
		await expect(rail.locator(".hawkr-agent-rail__row")).toHaveCount(1);
		await advanceFrame();
		await advanceFrame();
		await expect(rail.locator(".hawkr-agent-rail__row")).toHaveCount(1);
		await expect(rail.locator(".hawkr-agent-rail__row")).toHaveAttribute(
			"data-status",
			"SUCCEEDED",
		);
		await expect(rail).toContainText("讀你寫的需求");
		await expect(processing).toBeVisible();
		await advanceFrame();
		await expect(rail).toContainText("抓出品項、數量和交期");
		await expect(rail).toContainText("讀你寫的需求");
		await expect(rail.locator(".hawkr-agent-rail__row")).toHaveCount(2);

		await advanceFrame();
		const finalReply = page.getByText(
			"好的，番茄記下了。請問要配送到哪個城市呢？",
			{ exact: true },
		);
		await expect(finalReply).toBeVisible();
		const finalNarrative = page
			.locator("li.hawkr-agent-narrative")
			.filter({ has: finalReply });
		await expect(finalNarrative).toHaveCount(1);
		expect(
			await finalReply.evaluate(
				(node) => node.closest(".thread-page__bubble") === null,
			),
		).toBe(true);
		await expect(finalNarrative).toHaveCSS(
			"background-color",
			"rgba(0, 0, 0, 0)",
		);
		await expect(finalNarrative).toHaveCSS("border-top-width", "0px");
		await expect(finalNarrative).toHaveCSS("box-shadow", "none");
		await expect(processing).toHaveCount(0);
		await expect(page.getByTestId("agent-activity-rail")).toHaveCount(0);
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);
		await expect(textarea).toBeEnabled();
		await expect(page.getByTestId("agent-step-summary")).toHaveCount(0);
		await expect(page.locator(".hawkr-agent-message__steps")).toHaveCount(0);
		await expect(page.locator("body")).not.toContainText(
			"TERMINAL_ONLY_STEP_SENTINEL",
		);
	});
});
