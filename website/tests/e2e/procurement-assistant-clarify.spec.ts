import { expect, type Request, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const PATH = "/account/messages/assistant/demo";
const PROCUREMENT_PATH = "/account/messages/assistant/procurement";
const HANDOFF_STORAGE_KEY = "hawkr:hawkr-agent:search-handoff:v1";
/** doc 55 §8.1 哨兵：後端過渡期仍會送 reasoning，這串字任何一層命中
 * （DOM/a11y tree）都是上線阻斷。 */
const REASONING_SENTINEL =
	"SENTINEL-RAW-COT-品項已知，城市未知且為必填，優先詢問。";

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
			const ready = turnCount >= 2;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					// B-244 契約：conversationId/steps/recordedThisTurn/question 為
					// 新欄位；reasoning 是過渡期後端仍會送的舊欄位，前端必須
					// 完全不渲染（哨兵斷言在下面）。
					data: {
						requestId: "1234567890123456788",
						revisionId: `123456789012345679${turnCount}`,
						conversationId: "9876543210987654321",
						slots: ready
							? { item: "水果禮盒", delivery_city: "桃園市" }
							: { item: "水果禮盒" },
						missingRequired: ready ? [] : ["delivery_city"],
						rejected: [],
						degraded: false,
						reply: ready
							? "好的，那我先幫您找桃園能送水果禮盒的賣家，馬上開始搜尋！"
							: "好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？",
						reasoning: REASONING_SENTINEL,
						steps: [
							{
								code: "UNDERSTAND_INPUT",
								status: "SUCCEEDED",
								label: "正在理解你的需求",
								durationMs: 120,
							},
							{
								code: "EXTRACT_SLOTS",
								status: "SUCCEEDED",
								label: "正在整理採購條件",
								durationMs: 800,
							},
						],
						recordedThisTurn: ready
							? [{ key: "delivery_city", label: "配送城市", value: "桃園市" }]
							: [{ key: "item", label: "品項", value: "水果禮盒" }],
						question: ready
							? null
							: {
									slotKey: "delivery_city",
									prompt: "要送到哪個城市呢？",
									options: [
										{
											optionId: "city-taoyuan",
											label: "桃園市",
											value: "桃園市",
										},
										{
											optionId: "city-taipei",
											label: "台北市",
											value: "台北市",
										},
									],
									allowFreeText: true,
									source: "FIXED",
								},
					},
				}),
			});
		});

		// B-244 Ask 3：問題卡答題端點——回應與 turns 完全同構。
		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				turnCount += 1;
				const body = route.request().postDataJSON() as Record<string, unknown>;
				if (!body.expectedRevisionId || !body.clientRequestId) {
					return route.fulfill({
						status: 400,
						contentType: "application/json",
						body: JSON.stringify({ code: 400, message: "bad request" }),
					});
				}
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: `123456789012345679${turnCount}`,
							conversationId: "9876543210987654321",
							slots: { item: "水果禮盒", delivery_city: "桃園市" },
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "好的，那我先幫您找桃園能送水果禮盒的賣家，馬上開始搜尋！",
							reasoning: REASONING_SENTINEL,
							steps: [
								{
									code: "VALIDATE_SLOTS",
									status: "SUCCEEDED",
									label: "正在檢查條件",
									durationMs: 40,
								},
								{
									code: "REQUIREMENT_READY",
									status: "SUCCEEDED",
									label: "必填條件已齊備",
									durationMs: 10,
								},
							],
							recordedThisTurn: [
								{ key: "delivery_city", label: "配送城市", value: "桃園市" },
							],
							question: null,
						},
					}),
				});
			},
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/snapshot",
			(route) => {
				const ready = turnCount >= 2;
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: `123456789012345679${turnCount}`,
							status: ready ? "REQUIREMENT_READY" : "CLARIFYING",
							slots: ready
								? { item: "水果禮盒", delivery_city: "桃園市" }
								: { item: "水果禮盒" },
							missingRequired: ready ? [] : ["delivery_city"],
						},
					}),
				});
			},
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

		// Turn 1 — 只講品項，後端回缺城市。
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

		// §8.1 哨兵：raw chain-of-thought 不得出現在 DOM 的任何角落，
		// 「查看思考過程」展示已下架。
		await expect(page.getByText(REASONING_SENTINEL)).toHaveCount(0);
		await expect(page.getByText("查看思考過程")).toHaveCount(0);
		const domHasSentinel = await page.evaluate(
			(sentinel) => document.documentElement.outerHTML.includes(sentinel),
			REASONING_SENTINEL,
		);
		expect(domHasSentinel).toBe(false);

		// 思考步驟只在等待期間顯示：落地後跑條卸載，且不留步驟摘要或回放清單。
		//
		// 這裡刻意不寫「畫面上不得有任何 <details>」——那條在合併 #488 之前是
		// 對的（當時 <details> 只有步驟摘要在用），但推理過程展開現在也用同一個
		// 元素，一竿子打翻會連它一起擋掉。要擋的是步驟摘要這個東西，就對著它的
		// testid 擋。
		// 進度只屬於 active operation；落地後對話與成果面板都不能保留思考快照。
		await expect(page.getByTestId("agent-activity-rail")).toHaveCount(0);
		await expect(page.getByTestId("agent-processing")).toHaveCount(0);
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);
		await expect(page.getByTestId("agent-step-summary")).toHaveCount(0);
		await expect(page.getByText("正在理解你的需求")).toHaveCount(0);

		// 三段式①確認段：「已記下」chips。
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

		// 引導式問題卡：選項 chips＋自由輸入。
		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(
			questionCard.getByRole("button", { name: "台北市" }),
		).toBeVisible();
		// 選項本身已用圓形狀態容器表達選中；共享 check glyph 必須維持
		// 純勾號，不能再疊一層方框形成「圓中有方框」。
		const taipeiOption = questionCard.getByRole("button", { name: "台北市" });
		await taipeiOption.hover();
		const optionMark = taipeiOption.locator(
			".hawkr-agent-question-card__option-mark",
		);
		const checkMark = optionMark.locator('[data-icon-name="check"]');
		await expect(checkMark).toHaveCSS("opacity", "1");
		await expect(checkMark.locator("path")).toHaveCount(1);
		await expect(checkMark.locator("rect, circle")).toHaveCount(0);
		// delivery_city 是必填欄位（在 missingRequired 裡）——跳過對它是無效
		// 動作（backend 只把 skip 套用在選填欄位優先序上，必填缺項的判定不看
		// skipped set，下一輪還是同一題），前端不得顯示跳過按鈕誤導買家。
		await expect(
			questionCard.getByRole("button", { name: "跳過" }),
		).toHaveCount(0);
		// 「其他」在原問題卡內展開對應 freeText 欄位，不把使用者丟到底部
		// composer，回答仍會走同一支 /answers contract。
		await questionCard.getByRole("button", { name: "其他…" }).click();
		const customCity = questionCard.getByRole("textbox", {
			name: "要送到哪個城市呢？的答案",
		});
		await expect(customCity).toBeVisible();
		await expect(customCity).toHaveAttribute("placeholder", "例如：新竹市");

		// 需求快照相關 UI 完全不存在。
		await expect(page.getByText("需求快照")).toHaveCount(0);
		await expect(page.locator(".assistant-snapshot")).toHaveCount(0);

		// Turn 2 — 點選項答題（走 answers 端點；mock 在下方登記）。
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
		// 答題後 URL 已寫回對話位址（第一輪 conversationId 即寫入）。
		await expect(page).toHaveURL(
			/\/account\/messages\/assistant\/9876543210987654321$/,
		);

		// 320px + 200% 文字縮放不能把 task CTA 裁掉；facts 與 task card 都要
		// 在自己的可見寬度內完整 reflow。只在 mobile project 做一次即可。
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
		const appBaseURL = baseURL ?? "http://localhost:3000";
		const answerRequests: Request[] = [];

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

		// B-244 Ask 3/5 設計核心：REQUIREMENT_READY 之後 question 仍可非
		// null——pickNextSlot 可以繼續問選填欄位，同時「開始搜尋候選」CTA
		// 也已可見，兩者不再互斥。
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply:
							"好的，那我先幫您找桃園能送水果禮盒的賣家！方便說一下大概數量嗎？",
						reasoning: REASONING_SENTINEL,
						steps: [
							{
								code: "REQUIREMENT_READY",
								status: "SUCCEEDED",
								label: "必填條件已齊備",
								durationMs: 10,
							},
						],
						recordedThisTurn: [
							{ key: "delivery_city", label: "配送城市", value: "桃園市" },
						],
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							allowFreeText: true,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answerRequests.push(route.request());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456792",
							conversationId: "9876543210987654321",
							slots: {
								item: "水果禮盒",
								delivery_city: "桃園市",
								quantity: 30,
							},
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "收到，數量先記為 30。",
							recordedThisTurn: [
								{ key: "quantity", label: "數量", value: "30" },
							],
							question: null,
						},
					}),
				});
			},
		);

		// stage 的權威來源是 snapshot（不是 turns 回應的 missingRequired——見
		// useProcurementSession.ts applyTurnResult 的註解），沿用第一個 test
		// 的模式明確 mock REQUIREMENT_READY，讓這個 case 真的驗證到
		// 「stage===ready 時 question 與 CTA 並存」，而不是巧合命中
		// deriveStageFromStatus 對未知 status 的預設 fallthrough。
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
							revisionId: "1234567890123456791",
							status: "REQUIREMENT_READY",
							slots: { item: "水果禮盒", delivery_city: "桃園市" },
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

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		// REQUIREMENT_READY 之後 question 仍可非 null（後端繼續問選填欄位）。
		// 此時只出問題卡——派工卡要等問題清空才出現，不在問題未答完時就誘導
		// 使用者跳過剩下的補充。
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
		expect(answerRequests).toHaveLength(0);
		await inlineAnswer.fill("30");
		await questionCard.getByRole("button", { name: "送出答案" }).click();
		await expect(
			page.getByText("收到，數量先記為 30。", { exact: true }),
		).toBeVisible();
		expect(answerRequests).toHaveLength(1);
		const answerBody = answerRequests[0].postDataJSON();
		expect(answerBody.slotKey).toBe("quantity");
		expect(answerBody.freeText).toBe("30");
		expect(answerBody.optionId).toBeUndefined();

		// 問完了（後端回 question: null）→ 派工卡這時候才出現。
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
		const appBaseURL = baseURL ?? "http://localhost:3000";
		const answerRequests: Request[] = [];

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

		// 必填已齊備、question 是選填欄位 quantity（無 options，純自由輸入型——
		// 6 個選填欄位裡的多數屬此類，正是「fieldset gate 只認 options.length」
		// 那個 bug 會讓跳過按鈕整個不出現的情境）。
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply:
							"好的，那我先幫您找桃園能送水果禮盒的賣家！方便說一下大概數量嗎？",
						recordedThisTurn: [
							{ key: "delivery_city", label: "配送城市", value: "桃園市" },
						],
						question: {
							// 用 packaging 而不是 quantity：quantity／delivery_date 是後端的
							// QuoteRequiredOptionalSlots，canSkip 恆為 false，所以前端本來就
							// 不該渲染跳過按鈕（另一支測試釘住那件事）。原本這裡用 quantity，
							// 等於把「按了必定失敗的按鈕」當成正常行為釘住。
							slotKey: "packaging",
							prompt: "包裝上有什麼需求嗎？",
							options: [],
							allowFreeText: true,
							canSkip: true,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answerRequests.push(route.request());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456792",
							conversationId: "9876543210987654321",
							slots: { item: "水果禮盒", delivery_city: "桃園市" },
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "了解，那我先不記包裝，其他資訊都齊了。",
							recordedThisTurn: [],
							question: null,
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
							requestId: "1234567890123456788",
							revisionId: "1234567890123456791",
							status: "REQUIREMENT_READY",
							slots: { item: "水果禮盒", delivery_city: "桃園市" },
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

		expect(answerRequests).toHaveLength(1);
		const answerBody = answerRequests[0].postDataJSON();
		expect(answerBody.skip).toBe(true);
		expect(answerBody.slotKey).toBe("packaging");
		expect(answerBody.optionId).toBeUndefined();
		expect(answerBody.freeText).toBeUndefined();
	});

	test("報價單前提欄位（quantity／delivery_date）不得渲染跳過按鈕——後端對它們的 skip 一律 422", async ({
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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: [],
						// canSkip:false——quantity 是後端 QuoteRequiredOptionalSlots，
						// /answers 對它的 skip:true 一律回 422 SLOT_NOT_SKIPPABLE。
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(questionCard).toContainText("大概需要多少數量？");
		// 按鈕不存在——不是 disabled，是根本不渲染（呼叫端不傳 onSkip）。
		await expect(
			questionCard.getByRole("button", { name: "跳過" }),
		).toHaveCount(0);
		// 說明文案也要跟著改口，不能還寫著「或選擇跳過」。
		await expect(questionCard).not.toContainText("或選擇跳過");
	});

	test("quantity 複合問題卡——單位 chip 在數字欄有效前 disabled，點 chip 一次送出 freeText+unitOptionId", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		const answerRequests: Request[] = [];

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

		// quantity 未填時（Unit 也未填）：backend-payuni#2442 §4 的複合問題卡
		// 型態，unitOptions 帶單位選項，allowFreeText:true（數字欄仍要收）。
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: [],
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							unitOptions: [
								{ optionId: "unit-basket", label: "籃", value: "BASKET" },
								{ optionId: "unit-box", label: "箱", value: "BOX" },
							],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answerRequests.push(route.request());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456792",
							conversationId: "9876543210987654321",
							slots: {
								item: "水果禮盒",
								delivery_city: "桃園市",
								quantity: 50,
								unit: "箱",
							},
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "收到，50 箱先記下了。",
							recordedThisTurn: [
								{ key: "quantity", label: "數量", value: "50" },
								{ key: "unit", label: "單位", value: "箱" },
							],
							question: null,
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
							requestId: "1234567890123456788",
							revisionId: "1234567890123456791",
							status: "REQUIREMENT_READY",
							slots: { item: "水果禮盒", delivery_city: "桃園市" },
							missingRequired: [],
						},
					}),
				}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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
		// 沒有獨立的送出按鈕——這格靠點單位 chip 一次送出，不是打完數字按
		// Enter／送出。
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

		expect(answerRequests).toHaveLength(1);
		const answerBody = answerRequests[0].postDataJSON();
		expect(answerBody.slotKey).toBe("quantity");
		expect(answerBody.freeText).toBe("50");
		expect(answerBody.unitOptionId).toBe("unit-box");
		expect(answerBody.optionId).toBeUndefined();

		await expect(page.getByTestId("clarify-question-card")).toHaveCount(0);
	});

	test("quantity 單位追問——沒有數字欄，只有單位 chip，點擊直接送出 unitOptionId（不帶 freeText）", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		const answerRequests: Request[] = [];

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

		// 買家自由對話讓 LLM Extract 只抓到數字、沒抓到單位（backend-payuni#2442
		// §3 邊界情況）：Quantity 已填、Unit 未填，backend 回「單位追問」型態
		// （allowFreeText:false，只問單位）。
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市", quantity: 50 },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "50 先記下了，這批貨要用什麼單位計算？",
						recordedThisTurn: [{ key: "quantity", label: "數量", value: "50" }],
						question: {
							slotKey: "quantity",
							prompt: "這批貨要用什麼單位計算？",
							options: [],
							unitOptions: [
								{ optionId: "unit-basket", label: "籃", value: "BASKET" },
								{ optionId: "unit-box", label: "箱", value: "BOX" },
							],
							allowFreeText: false,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answerRequests.push(route.request());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456792",
							conversationId: "9876543210987654321",
							slots: {
								item: "水果禮盒",
								delivery_city: "桃園市",
								quantity: 50,
								unit: "箱",
							},
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "收到，單位記為箱。",
							recordedThisTurn: [{ key: "unit", label: "單位", value: "箱" }],
							question: null,
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
							requestId: "1234567890123456788",
							revisionId: "1234567890123456791",
							status: "REQUIREMENT_READY",
							slots: {
								item: "水果禮盒",
								delivery_city: "桃園市",
								quantity: 50,
							},
							missingRequired: [],
						},
					}),
				}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒 50 個，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();
		await expect(questionCard).toHaveAttribute("data-question-kind", "unit");
		// 沒有數字輸入欄——這題只問單位。
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

		expect(answerRequests).toHaveLength(1);
		const answerBody = answerRequests[0].postDataJSON();
		expect(answerBody.slotKey).toBe("quantity");
		expect(answerBody.unitOptionId).toBe("unit-box");
		expect(answerBody.freeText).toBeUndefined();
		expect(answerBody.optionId).toBeUndefined();
	});

	test("複合問題卡活躍時，底部 composer 打字送出走一般 /turns（不是 /answers）——避免缺單位觸發 422", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(30_000);
		const appBaseURL = baseURL ?? "http://localhost:3000";
		const answerRequests: Request[] = [];
		let turnCount = 0;

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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			turnCount += 1;
			const answered = turnCount >= 2;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: `123456789012345679${turnCount}`,
						conversationId: "9876543210987654321",
						slots: answered
							? {
									item: "水果禮盒",
									delivery_city: "桃園市",
									quantity: 50,
									unit: "箱",
								}
							: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: answered
							? "收到，50 箱先記下了。"
							: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: answered
							? [
									{ key: "quantity", label: "數量", value: "50" },
									{ key: "unit", label: "單位", value: "箱" },
								]
							: [],
						question: answered
							? null
							: {
									slotKey: "quantity",
									prompt: "大概需要多少數量？",
									options: [],
									unitOptions: [
										{ optionId: "unit-box", label: "箱", value: "BOX" },
									],
									allowFreeText: true,
									canSkip: false,
									source: "FIXED",
								},
					},
				}),
			});
		});

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answerRequests.push(route.request());
				return route.fulfill({
					status: 422,
					contentType: "application/json",
					body: JSON.stringify({
						code: "SLOT_NOT_SKIPPABLE",
						message: "unit missing",
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
							requestId: "1234567890123456788",
							revisionId: "1234567890123456791",
							status: "REQUIREMENT_READY",
							slots: { item: "水果禮盒", delivery_city: "桃園市" },
							missingRequired: [],
						},
					}),
				}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒，送桃園");
		await page.getByRole("button", { name: "送出" }).click();

		const questionCard = page.getByTestId("clarify-question-card");
		await expect(questionCard).toBeVisible();

		// 底部 composer 直接打完整句子（不透過卡片內的兩段式流程）。
		await textarea.fill("50箱");
		await page.getByRole("button", { name: "送出" }).click();

		await expect(
			page.getByText("收到，50 箱先記下了。", { exact: true }),
		).toBeVisible();
		expect(turnCount).toBe(2);
		expect(answerRequests).toHaveLength(0);
	});

	test("複合問題卡／單位追問卡的單位 chips 群組不會長出「其他」按鈕——後端沒有自由輸入單位的路徑", async ({
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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: [],
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							unitOptions: [
								{ optionId: "unit-box", label: "箱", value: "BOX" },
							],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "要送到哪個城市呢？",
						recordedThisTurn: [],
						question: {
							slotKey: "delivery_city",
							prompt: "要送到哪個城市呢？",
							options: [
								{ optionId: "city-taoyuan", label: "桃園市", value: "桃園市" },
							],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			});
		});

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456792",
							conversationId: "9876543210987654321",
							slots: {
								item: "水果禮盒",
								delivery_city: "桃園市",
								quantity: 50,
							},
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "50 先記下了，這批貨要用什麼單位計算？",
							recordedThisTurn: [
								{ key: "delivery_city", label: "配送城市", value: "桃園市" },
							],
							question: {
								slotKey: "quantity",
								prompt: "這批貨要用什麼單位計算？",
								options: [],
								unitOptions: [
									{ optionId: "unit-box", label: "箱", value: "BOX" },
									{ optionId: "unit-kg", label: "公斤", value: "KG" },
								],
								allowFreeText: false,
								canSkip: false,
								source: "FIXED",
							},
						},
					}),
				}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: [],
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							unitOptions: [
								{ optionId: "unit-box", label: "箱", value: "BOX" },
							],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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
		const appBaseURL = baseURL ?? "http://localhost:3000";
		const answerRequests: Request[] = [];

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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: [],
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							unitOptions: [
								{ optionId: "unit-box", label: "箱", value: "BOX" },
								{ optionId: "unit-kg", label: "公斤", value: "KG" },
								{ optionId: "unit-catty", label: "台斤", value: "CATTY" },
							],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) => {
				answerRequests.push(route.request());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456792",
							conversationId: "9876543210987654321",
							slots: {
								item: "水果禮盒",
								delivery_city: "桃園市",
								quantity: 50,
								unit: "台斤",
							},
							missingRequired: [],
							rejected: [],
							degraded: false,
							reply: "收到，50 台斤先記下了。",
							recordedThisTurn: [
								{ key: "quantity", label: "數量", value: "50" },
								{ key: "unit", label: "單位", value: "台斤" },
							],
							question: null,
						},
					}),
				});
			},
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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

		expect(answerRequests).toHaveLength(1);
		const answerBody = answerRequests[0].postDataJSON();
		expect(answerBody.unitOptionId).toBe("unit-catty");
		expect(answerBody.freeText).toBe("50");
	});

	test("複合問題卡送出後端回 422 ANSWER_SCHEMA_INVALID——錯誤顯示在對話尾端，問題卡不會憑空消失", async ({
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

		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456791",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒", delivery_city: "桃園市" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "好的，方便說一下大概數量嗎？",
						recordedThisTurn: [],
						question: {
							slotKey: "quantity",
							prompt: "大概需要多少數量？",
							options: [],
							unitOptions: [
								{ optionId: "unit-box", label: "箱", value: "BOX" },
							],
							allowFreeText: true,
							canSkip: false,
							source: "FIXED",
						},
					},
				}),
			}),
		);

		await page.route(
			"**/api/v1/app/buyers/me/procurement/requests/*/answers",
			(route) =>
				route.fulfill({
					status: 422,
					contentType: "application/json",
					body: JSON.stringify({
						code: 422,
						message: "ANSWER_SCHEMA_INVALID: 答案不符合這個欄位的格式",
					}),
				}),
		);

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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
			page.getByText("ANSWER_SCHEMA_INVALID", { exact: false }),
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
		const appBaseURL = baseURL ?? "http://localhost:3000";
		let turnRequests = 0;

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
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			turnRequests += 1;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			});
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

		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		const textarea = page.locator("#procurement-clarify-input");
		await expect(textarea).toHaveValue("企業送禮水果盒");
		if (isMobile) {
			await expect(textarea).not.toBeFocused();
		} else {
			await expect(textarea).toBeFocused();
		}
		expect(turnRequests).toBe(0);
	});

	test("「開新對話」在 procurement 入口對話開始（URL 已被 replaceState 改寫）後真正開出新對話", async ({
		page,
		baseURL,
		isMobile,
	}) => {
		// 這一支驅動的是側欄的控制項。手機在對話視圖是全螢幕 thread，側欄只存
		// 在於列表視圖（list ↔ thread），所以這個互動在該版型不成立——不是有
		// bug 被藏起來，是版型本來就沒有這個位置。
		//
		// describe 上那句 `browserName !== "chromium"` 擋不到：mobile project 用
		// 的是 Pixel 5，browserName 一樣是 "chromium"。dev push 只跑 chromium 所
		// 以看不出來，main 會連 mobile 一起跑。
		test.skip(!!isMobile, "側欄控制項在手機的 thread 視圖不存在");
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
			identity: { accountId: "300", roles: ["buyer"] },
			session: "identity",
		});
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: appBaseURL }]);
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: "1234567890123456790",
						conversationId: "9876543210987654321",
						slots: { item: "水果禮盒" },
						missingRequired: ["delivery_city"],
						rejected: [],
						degraded: false,
						reply: "好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？",
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
							requestId: "1234567890123456788",
							revisionId: "1234567890123456790",
							status: "CLARIFYING",
							slots: { item: "水果禮盒" },
							missingRequired: ["delivery_city"],
						},
					}),
				}),
		);
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		await page.goto(PROCUREMENT_PATH, { waitUntil: "domcontentloaded" });
		const textarea = page.locator("#procurement-clarify-input");
		await textarea.fill("我要訂水果禮盒");
		await page.getByRole("button", { name: "送出" }).click();
		await expect(
			page.getByText("好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？", {
				exact: true,
			}),
		).toBeVisible({ timeout: 10_000 });
		// 第一輪後 URL 被 replaceState 改寫成對話位址。
		await expect(page).toHaveURL(
			/\/account\/messages\/assistant\/9876543210987654321$/,
		);

		// 開新對話：同 segment push 不 remount——靠 workspace 的 pathname
		// 轉場監聽把 session 歸零（審查 CONFIRMED-high 的修復）。
		// 入口原本是 header 的方框 ＋，已改成「Hawkr Agent」分頁列表頂端的常駐列。
		await page.getByRole("button", { name: "開新對話" }).click();
		await expect(page).toHaveURL(
			/\/account\/messages\/assistant\/procurement$/,
		);
		// 歸零後沒有既有 conversation → 回到 entry state（設計書 v6 §V.11.0），
		// 不是 thread 的空狀態泡泡。
		await expect(page.locator('[data-assistant-mode="live"]')).toHaveAttribute(
			"data-assistant-view",
			"entry",
		);
		await expect(
			page.getByRole("heading", { name: "今天想請 Hawkr 幫你完成什麼？" }),
		).toBeVisible();
		// 同一個 composer 跟著移回中央，草稿是空的。
		await expect(page.locator("#procurement-clarify-input")).toHaveCount(1);
		await expect(textarea).toHaveValue("");
	});
	test("對話 composer 支援換行與中文組字；44–120px 自動高度且長文不水平溢出", async ({
		page,
	}) => {
		test.setTimeout(30_000);
		const turnBodies: Record<string, unknown>[] = [];
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
		await page.route(
			"**/api/v1/app/buyers/me/procurement/turns/stream",
			(route) =>
				route.fulfill({
					status: 503,
					contentType: "application/json",
					body: JSON.stringify({ code: 503, message: "stream unavailable" }),
				}),
		);
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			turnBodies.push(route.request().postDataJSON());
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: "1234567890123456788",
						revisionId: `123456789012345679${turnBodies.length}`,
						conversationId: "9876543210987654321",
						slots: { item: "番茄" },
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply:
							turnBodies.length === 1 ? "已記下番茄需求。" : "已更新採購需求。",
						recordedThisTurn: [],
						question: null,
					},
				}),
			});
		});
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		await page.goto(PROCUREMENT_PATH, { waitUntil: "domcontentloaded" });
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

		// Playwright 的 route.fulfill 一次送完整個 body，無法逐塊推送——而這支
		// 測的正是「一塊一塊到達時畫面有沒有跟著換行」。所以在瀏覽器端包一層
		// fetch，對串流端點回一個自己控制節奏的 ReadableStream；其餘請求原樣
		// 放行給上面的 route mock。
		await page.addInitScript(() => {
			const nativeFetch = window.fetch.bind(window);
			window.fetch = async (input, init) => {
				const url =
					typeof input === "string"
						? input
						: input instanceof URL
							? input.href
							: input.url;
				if (!url.includes("/procurement/turns/stream"))
					return nativeFetch(input as RequestInfo, init);
				const encoder = new TextEncoder();
				const aguiFrame = (event: Record<string, unknown>) =>
					`data: ${JSON.stringify(event)}\n\n`;
				const frames = [
					aguiFrame({
						type: "CUSTOM",
						name: "step_detail",
						value: {
							turnId: "1",
							seq: 1,
							code: "UNDERSTAND_INPUT",
							status: "RUNNING",
							label: "讀你寫的需求",
						},
					}),
					aguiFrame({
						type: "CUSTOM",
						name: "step_detail",
						value: {
							turnId: "1",
							seq: 2,
							code: "UNDERSTAND_INPUT",
							status: "RUNNING",
							label: "讀你寫的需求",
						},
					}),
					aguiFrame({
						type: "CUSTOM",
						name: "step_detail",
						value: {
							turnId: "1",
							seq: 3,
							code: "UNDERSTAND_INPUT",
							status: "SUCCEEDED",
							label: "讀你寫的需求",
						},
					}),
					aguiFrame({
						type: "CUSTOM",
						name: "step_detail",
						value: {
							turnId: "1",
							seq: 4,
							code: "EXTRACT_SLOTS",
							status: "RUNNING",
							label: "抓出品項、數量和交期",
						},
					}),
					aguiFrame({
						type: "RUN_FINISHED",
						threadId: "",
						runId: "1",
						result: {
							requestId: "1234567890123456788",
							revisionId: "1234567890123456791",
							conversationId: "9876543210987654321",
							slots: { item: "番茄" },
							missingRequired: ["delivery_city"],
							rejected: [],
							degraded: false,
							reply: "好的，番茄記下了。請問要配送到哪個城市呢？",
							steps: [
								{
									code: "UNDERSTAND_INPUT",
									status: "SUCCEEDED",
									label: "讀你寫的需求",
									durationMs: 200,
								},
								{
									code: "EXTRACT_SLOTS",
									status: "SUCCEEDED",
									label: "抓出品項、數量和交期",
									durationMs: 5000,
								},
								{
									code: "VALIDATE_SLOTS",
									status: "SUCCEEDED",
									text: "TERMINAL_ONLY_STEP_SENTINEL",
									durationMs: 120,
								},
							],
							recordedThisTurn: [{ key: "item", label: "品項", value: "番茄" }],
							question: null,
						},
					}),
				];
				// 測試逐塊放行，不靠 600ms 計時搶在下一個 frame 前觀察 DOM。
				const body = new ReadableStream<Uint8Array>({
					start(controller) {
						let nextFrame = 0;
						Object.assign(window, {
							__advanceProcurementFrame: () => {
								controller.enqueue(encoder.encode(frames[nextFrame++]));
								if (nextFrame === frames.length) controller.close();
							},
						});
					},
				});
				return new Response(body, {
					status: 200,
					headers: { "content-type": "text/event-stream" },
				});
			};
		});

		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});
		const response = await page.goto(PROCUREMENT_PATH, {
			waitUntil: "domcontentloaded",
		});
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
		await advanceFrame(); // 相同步驟 heartbeat。
		await advanceFrame(); // step SUCCEEDED 不等於 run terminal。
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
		// 落地：即時狀態軌由父層在同一個 commit 換成無泡泡 Public Narrative。
		// 終態 payload 即使仍帶 steps，也不得在 transcript 再長成一組氣泡。
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
