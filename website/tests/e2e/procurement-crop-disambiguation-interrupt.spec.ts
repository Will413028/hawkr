import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const DEMO_PATH = "/account/messages/assistant/demo";
const PROCUREMENT_PATH = "/account/messages/assistant/procurement";

/**
 * doc57 run ledger 的 interrupt／resume 覆蓋（backend-payuni#2861 開放
 * interrupt guard；hawkr-website#548 接上 UI）。這裡每個情境都要組 SSE frame／sync-fallback／reload
 * 深連結三種不同 transport 的 mock，setup 發散到值得獨立成檔——precedent 是
 * procurement-quote-batch-preview.spec.ts 之於 procurement-quote-batch-send.spec.ts。
 *
 * PROCUREMENT_DURABLE_RUN_ENABLED 目前 dev 全域為 false，這支只能靠 mock
 * gateway 驗證前端邏輯本身；wire 形狀全部照
 * features/procurement-assistant/session-logic.ts（normalizeProcurementTurnResult）
 * 與 lib/ai-chat/stream-client.ts（EventSchemas.safeParse）核對過，不是憑印象編的。
 */
test.describe("procurement assistant — interrupt／resume（doc57 run ledger）", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName }) => browserName !== "chromium",
		"single representative browser is enough for this mocked-gateway flow",
	);

	async function setupWorkspaceShell(
		page: import("@playwright/test").Page,
		appBaseURL: string,
	) {
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

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
	}

	/** AG-UI SSE frame encoder — 同 procurement-assistant-clarify.spec.ts 既有的
	 * `aguiFrame` 慣例，`RUN_FINISHED`/`RUN_ERROR` 都要通過
	 * `lib/ai-chat/stream-client.ts` 的 `EventSchemas.safeParse`，形狀不對會落到
	 * 「AI 回應格式不符 AG-UI 契約」而不是我們要測的訊息。 */
	function aguiFrame(event: Record<string, unknown>): string {
		return `data: ${JSON.stringify(event)}\n\n`;
	}

	test("live-stream 落地帶 interrupt：消歧卡出現、composer 鎖住，選項呼叫 resume() 而非 chooseCropDisambiguation；resume 成功後解鎖", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		const REQUEST_ID = "1234567890123456788";
		const REVISION_ID = "1234567890123456791";
		const INTERRUPT_ID = "run-int-001";

		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "text/event-stream",
					// ⛔ 2026-09-03：這個 mock 必須維持 **production 真正的 wire 形狀**。
					// 舊版把 interruptId／interruptKind 塞在 result 裡——實測 dev 證明
					// production 從不那樣送：串流 RUN_FINISHED 的 interrupt **只**存在於
					// 頂層 outcome（`{type:"interrupt",interrupts:[{id,reason}]}`，AG-UI
					// 官方 schema），result 裡沒有這兩個欄位；unary /turns 才有（見本檔
					// 下方 sync-fallback 那支，那裡的 result.interruptId 是對的）。
					// 舊 mock 讓這支 e2e 在 production 壞掉時照樣全綠——它驗的是一個
					// 不會發生的情境。詳 buyer-flow-verify flow 48「品項消歧 interrupt
					// 終端契約」。
					body: aguiFrame({
						type: "RUN_FINISHED",
						threadId: "",
						runId: "1",
						outcome: {
							type: "interrupt",
							interrupts: [{ id: INTERRUPT_ID, reason: "CROP_DISAMBIGUATION" }],
						},
						result: {
							requestId: REQUEST_ID,
							revisionId: REVISION_ID,
							conversationId: "9876543210987654321",
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
									},
									{
										optionId: "opt-2",
										label: "高麗菜芽（高麗菜）",
									},
								],
							},
							question: null,
						},
					}),
				}),
		);

		// applyTurnResult 落地後會 best-effort 打一次 snapshot 校正 stage。
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
							revisionId: REVISION_ID,
							status: "CLARIFYING",
							slots: { item: "高麗菜" },
							missingRequired: [],
						},
					}),
				}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/runs/*/resume",
			(route) => {
				const body = route.request().postDataJSON() as Record<string, unknown>;
				expect(body.interruptId).toBe(INTERRUPT_ID);
				expect(body.payload).toEqual({ optionId: "opt-2" });
				expect(typeof body.clientRequestId).toBe("string");
				expect((body.clientRequestId as string).length).toBeGreaterThan(0);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: {} }),
				});
			},
		);

		const response = await page.goto(DEMO_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		// aria-label 在 pending（loading）時改成「傳送訊息中…」（見
		// features/messages/components/SendButton.tsx），固定文字的 locator 在
		// 那段期間會查無元素——用 regex 涵蓋兩種 label，同一顆按鈕全程可查。
		const sendButton = page.getByRole("button", {
			name: /^(送出|傳送訊息中…)$/,
		});

		await textarea.fill("我要訂高麗菜");
		await sendButton.click();

		const card = page.getByTestId("crop-disambiguation-card");
		await expect(card).toBeVisible({ timeout: 10_000 });

		// interrupt 期間鎖住訊息輸入框——composer 的 pending 現在同時看
		// pendingTurn 與 pendingInterrupt（AssistantWorkspace.tsx Fix）。
		await expect(textarea).toBeDisabled();
		await expect(sendButton).toBeDisabled();

		await page.getByRole("button", { name: "高麗菜芽（高麗菜）" }).click();

		// resume 成功 → pendingInterrupt 清空，composer 解鎖（上面的 resume
		// route handler 已經斷言過實際送出的 body）。
		await expect(textarea).not.toBeDisabled();
		await textarea.fill("之後想再訂別的");
		await expect(sendButton).toBeEnabled();
	});

	test("in-band RUN_ERROR reason=APPROVAL_PENDING：顯示正確訊息（不是身分衝突文案），並靠 refreshCurrentSnapshot() 自我校正回消歧卡", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		const REQUEST_ID = "1234567890123456788";
		const REVISION_ID_1 = "1234567890123456791";
		const INTERRUPT_ID = "run-heal-1";
		const APPROVAL_MESSAGE =
			"這筆需求正在等你回覆上一個問題，我先幫你刷新畫面。";

		let turnCount = 0;
		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) => {
				turnCount += 1;
				if (turnCount === 1) {
					return route.fulfill({
						status: 200,
						contentType: "text/event-stream",
						body: aguiFrame({
							type: "RUN_FINISHED",
							threadId: "",
							runId: "1",
							result: {
								requestId: REQUEST_ID,
								revisionId: REVISION_ID_1,
								conversationId: "9876543210987654321",
								slots: { item: "水果禮盒" },
								missingRequired: [],
								rejected: [],
								degraded: false,
								reply: "好的，水果禮盒記下了，還有什麼需要補充的嗎？",
								question: null,
							},
						}),
					});
				}
				return route.fulfill({
					status: 200,
					contentType: "text/event-stream",
					body: aguiFrame({
						type: "RUN_ERROR",
						message: APPROVAL_MESSAGE,
						code: "409",
						reason: "APPROVAL_PENDING",
					}),
				});
			},
		);

		let snapshotCalls = 0;
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/snapshot",
			(route) => {
				snapshotCalls += 1;
				if (snapshotCalls === 1) {
					return route.fulfill({
						status: 200,
						contentType: "application/json",
						body: JSON.stringify({
							code: 0,
							data: {
								requestId: REQUEST_ID,
								revisionId: REVISION_ID_1,
								status: "CLARIFYING",
								slots: { item: "水果禮盒" },
								missingRequired: [],
							},
						}),
					});
				}
				// refreshCurrentSnapshot() 的自我校正：權威 snapshot 帶著另一個
				// request 早已卡住的 interrupt（doc57 run ledger 的持久狀態）。
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: REQUEST_ID,
							revisionId: REVISION_ID_1,
							status: "CLARIFYING",
							slots: { item: "水果禮盒" },
							missingRequired: [],
							cropDisambiguation: {
								prompt: "你說的「水果禮盒」符合多種分類，麻煩選一下～",
								options: [
									{
										optionId: "opt-a",
										label: "綜合水果禮盒",
									},
								],
							},
							openConfirmation: {
								interruptId: INTERRUPT_ID,
								confirmationKind: "CROP_DISAMBIGUATION",
							},
						},
					}),
				});
			},
		);

		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		const sendButton = page.getByRole("button", {
			name: /^(送出|傳送訊息中…)$/,
		});

		await textarea.fill("我要訂水果禮盒");
		await sendButton.click();
		await expect(
			page.getByText("好的，水果禮盒記下了，還有什麼需要補充的嗎？"),
		).toBeVisible({ timeout: 10_000 });

		await textarea.fill("再幫我確認一下");
		await sendButton.click();

		await expect(page.getByText(APPROVAL_MESSAGE)).toBeVisible({
			timeout: 10_000,
		});
		await expect(
			page.getByText("APPROVAL_PENDING", { exact: true }),
		).toBeVisible();
		// 舊文案不得出現——這正是這個 reason-aware 分支要修掉的誤導訊息。
		await expect(page.getByText("識別碼已綁定其他內容")).toHaveCount(0);

		// 自我校正：refreshCurrentSnapshot() 打回權威 snapshot，消歧卡出現、
		// composer 繼續鎖住（現在是被 snapshot 校正出的 pendingInterrupt 鎖住，
		// 不是被 pendingTurn 鎖住）。
		await expect(page.getByTestId("crop-disambiguation-card")).toBeVisible({
			timeout: 10_000,
		});
		await expect(textarea).toBeDisabled();
		expect(snapshotCalls).toBe(2);
	});

	test("in-band RUN_ERROR 409 但 reason 不是 APPROVAL_PENDING（含空字串）：維持既有 TURN_IDEMPOTENCY_CONFLICT 文案（regression）", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "text/event-stream",
					body: aguiFrame({
						type: "RUN_ERROR",
						message: "duplicate client_request_id（不應直接顯示）",
						code: "409",
						// 刻意不帶 reason——既有 RUN_ERROR／HTTP 409 多數情況本來就沒有
						// 業務 reason，這條分支必須維持既有行為。
					}),
				}),
		);

		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂高麗菜");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(
			page.getByText(
				"這則訊息的識別碼已綁定其他內容，系統沒有建立新的操作。請修改文字後再送出。",
			),
		).toBeVisible({ timeout: 10_000 });
		await expect(
			page.getByText("IDEMPOTENCY_CONFLICT", { exact: true }),
		).toBeVisible();
	});

	test("串流開場前 HTTP 409（AiStreamHttpError）reason=APPROVAL_PENDING：同一套 reason-aware 分流，不是只有 in-band RUN_ERROR 才認得", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 409,
					contentType: "application/json",
					body: JSON.stringify({
						code: 409,
						reason: "APPROVAL_PENDING",
						message: "這筆需求正在等你回覆上一個問題。",
					}),
				}),
		);

		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂高麗菜");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(
			page.getByText("這筆需求正在等你回覆上一個問題。"),
		).toBeVisible({ timeout: 10_000 });
		await expect(
			page.getByText("APPROVAL_PENDING", { exact: true }),
		).toBeVisible();
	});

	test("串流開場前 HTTP 409（AiStreamHttpError）reason 非 APPROVAL_PENDING：維持既有 TURN_IDEMPOTENCY_CONFLICT 文案", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 409,
					contentType: "application/json",
					body: JSON.stringify({
						code: 409,
						message: "duplicate client_request_id",
					}),
				}),
		);

		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂高麗菜");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(
			page.getByText(
				"這則訊息的識別碼已綁定其他內容，系統沒有建立新的操作。請修改文字後再送出。",
			),
		).toBeVisible({ timeout: 10_000 });
	});

	test("串流端點探測到不存在（404）→ 改走同步 /turns：TurnResult 帶 interruptId 時不再被 sync-fallback 路徑無條件吞掉（existing bug 1 regression）", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		const REQUEST_ID = "1234567890123456788";
		const REVISION_ID = "1234567890123456791";
		const INTERRUPT_ID = "run-sync-fallback-1";

		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 404,
					contentType: "application/json",
					body: JSON.stringify({ code: 404, message: "not found" }),
				}),
		);

		let turnsCalls = 0;
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			turnsCalls += 1;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: REQUEST_ID,
						revisionId: REVISION_ID,
						slots: { item: "高麗菜" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "這個品項有幾種可能，麻煩選一下～",
						interruptId: INTERRUPT_ID,
						interruptKind: "CROP_DISAMBIGUATION",
						cropDisambiguation: {
							prompt: "你說的「高麗菜」是指哪一種？",
							options: [
								{
									optionId: "opt-1",
									label: "甘藍",
								},
							],
						},
					},
				}),
			});
		});

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
							revisionId: REVISION_ID,
							status: "CLARIFYING",
							slots: { item: "高麗菜" },
							missingRequired: [],
						},
					}),
				}),
		);

		const response = await page.goto(DEMO_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂高麗菜");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(page.getByTestId("crop-disambiguation-card")).toBeVisible({
			timeout: 10_000,
		});
		await expect(textarea).toBeDisabled();
		expect(turnsCalls).toBe(1);
	});

	test("reload 已有 pendingInterrupt 的需求（?requestId= 深連結）：消歧卡與 resume 不需要先跑一輪 live turn", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		const REQUEST_ID = "1234567890123456799";
		const INTERRUPT_ID = "run-restore-1";

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
							revisionId: "1234567890123456791",
							status: "CLARIFYING",
							slots: { item: "地瓜" },
							missingRequired: [],
							cropDisambiguation: {
								prompt: "你說的「地瓜」是指哪一種？",
								options: [
									{
										optionId: "opt-sp",
										label: "地瓜（甘藷）",
									},
								],
							},
							openConfirmation: {
								interruptId: INTERRUPT_ID,
								confirmationKind: "CROP_DISAMBIGUATION",
							},
						},
					}),
				}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/runs/*/resume",
			(route) => {
				const body = route.request().postDataJSON() as Record<string, unknown>;
				expect(body.interruptId).toBe(INTERRUPT_ID);
				expect(body.payload).toEqual({ optionId: "opt-sp" });
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: {} }),
				});
			},
		);

		const response = await page.goto(
			`${PROCUREMENT_PATH}?requestId=${REQUEST_ID}`,
			{ waitUntil: "domcontentloaded" },
		);
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const card = page.getByTestId("crop-disambiguation-card");
		await expect(card).toBeVisible({ timeout: 10_000 });
		const textarea = page.locator("#procurement-clarify-input");
		await expect(textarea).toBeDisabled();

		await page.getByRole("button", { name: "地瓜（甘藷）" }).click();
		await expect(textarea).not.toBeDisabled();
	});

	test("問題卡（ClarifyQuestionCard）的選項在有未回覆中斷時走 resume，完全不打 /answers", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await setupWorkspaceShell(page, appBaseURL);

		const REQUEST_ID = "1234567890123456792";
		const REVISION_ID = "1234567890123456793";
		const INTERRUPT_ID = "7501080536165646336";

		// 2026-09-03 dev 實測形狀：品項消歧現在由後端當成 **question** 下發
		// （dialogueActCode ITEM_NEEDS_DISAMBIGUATION，slotKey
		// "crop_disambiguation"，最後一個選項固定是 crop-other），
		// cropDisambiguation 欄位是 null，而 interrupt 一定同時存在於頂層
		// outcome。上面那支測的是 legacy CropDisambiguationCard，這支測的是
		// 後端今天真正驅動的問題卡——正是 APPROVAL_PENDING 那個 bug 的所在。
		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "text/event-stream",
					body: aguiFrame({
						type: "RUN_FINISHED",
						threadId: "",
						runId: "1",
						outcome: {
							type: "interrupt",
							interrupts: [{ id: INTERRUPT_ID, reason: "CLARIFICATION" }],
						},
						result: {
							requestId: REQUEST_ID,
							revisionId: REVISION_ID,
							conversationId: "9876543210987654322",
							slots: { item: "甘藍" },
							missingRequired: ["delivery_city", "unit"],
							rejected: [],
							degraded: false,
							reply: "找到幾個相符的品項，麻煩選一下你要的是哪一個：",
							cropDisambiguation: null,
							question: {
								slotKey: "crop_disambiguation",
								prompt: "找到幾個相符的品項，麻煩選一下你要的是哪一個：",
								allowFreeText: false,
								canSkip: false,
								options: [
									{
										optionId: "group-moa-group:f102791d6f6f4949",
										label: "甘藍（甘藍）",
										value: "甘藍（甘藍）",
									},
									{
										optionId: "crop-LA1",
										label: "甘藍-初秋",
										value: "甘藍-初秋",
									},
									{
										optionId: "crop-other",
										label: "其他（自己輸入）",
										value: "OTHER",
									},
								],
							},
						},
					}),
				}),
		);

		let snapshotCalls = 0;
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/snapshot",
			(route) => {
				snapshotCalls += 1;
				// 第一次是 applyTurnResult 的 stage 校正（中斷仍在）；resume 成功
				// 之後那次才回沒有 openConfirmation 的快照。
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: REQUEST_ID,
							revisionId: REVISION_ID,
							status: "CLARIFYING",
							slots: { item: "甘藍" },
							missingRequired: ["delivery_city", "unit"],
							...(snapshotCalls === 1
								? {
										openConfirmation: {
											interruptId: INTERRUPT_ID,
											confirmationKind: "CLARIFICATION",
											expiresAt: "",
										},
									}
								: {}),
						},
					}),
				});
			},
		);

		// 這個 bug 的本體：答案跑去打 /answers，被後端 GuardNoOpenInterrupt
		// 擋成 409。這裡照 production 真的會回的錯誤回，讓「路由錯了」在這支
		// 測試裡表現成使用者看得到的失敗，而不是靜靜通過。
		let answersCalls = 0;
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answersCalls += 1;
				return route.fulfill({
					status: 409,
					contentType: "application/json",
					body: JSON.stringify({
						code: 409,
						reason: "APPROVAL_PENDING",
						message: "這個需求有一個尚未回覆的中斷，請改用 resume 回覆",
					}),
				});
			},
		);

		// 用陣列收集而不是可為 null 的 let：TS 的控制流分析看不到 route callback
		// 會被呼叫，`let` 會在斷言處被窄化成 never。
		const resumeBodies: Record<string, unknown>[] = [];
		await page.route(
			"**/api/v1/app/buyers/me/procurement/runs/*/resume",
			(route) => {
				resumeBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: { runId: "run-child-1", runStatus: "SUCCEEDED" },
					}),
				});
			},
		);

		const response = await page.goto(DEMO_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();

		const textarea = page.locator("#procurement-clarify-input");
		const sendButton = page.getByRole("button", {
			name: /^(送出|傳送訊息中…)$/,
		});

		await textarea.fill("甘藍");
		await sendButton.click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible({ timeout: 10_000 });

		await page.getByRole("button", { name: "甘藍（甘藍）" }).click();

		// 打的是 resume，payload 是後端 cropDisambiguationResponseV1 的形狀。
		await expect.poll(() => resumeBodies.length, { timeout: 10_000 }).toBe(1);
		expect(resumeBodies[0].interruptId).toBe(INTERRUPT_ID);
		expect(resumeBodies[0].payload).toMatchObject({
			optionId: "group-moa-group:f102791d6f6f4949",
		});

		// 已回覆的問題卡收起，且全程沒有碰過 /answers。
		await expect(questionCard).toBeHidden();
		expect(answersCalls).toBe(0);
		await expect(
			page.getByText("這個需求有一個尚未回覆的中斷，請改用 resume 回覆"),
		).toHaveCount(0);
	});
});
