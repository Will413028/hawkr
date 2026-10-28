import type { Page, Route } from "@playwright/test";
import { expect } from "@playwright/test";
import { seedStoredActiveRole, setupGatewayMocks } from "./mock-gateway";

export const RFQ_PATH = "/account/messages/assistant/demo";
export const RFQ_REQUEST_ID = "1234567890123456788";
// ⚠️ 必須是塞得進 signed int64 的正整數字串：runtime-snapshot 的 `idSchema`
// 會擋掉超出範圍的值，整份快照因此讀不到（發布結果連帶消失）。
export const RFQ_CONVERSATION_ID = "1234567890123456790";
export const RFQ_REVISION_ID = "1234567890123456791";
export const RFQ_RUN_ID = "1234567890123456800";
export const RFQ_PREVIEW_ID = "1234567890123456801";
export const RFQ_CONFIRMED_ID = "1234567890123456802";
export const RFQ_PREVIEW_HASH = "a".repeat(64);
export const RFQ_CONFIRMED_HASH = "b".repeat(64);
export const RFQ_APPROVAL_ID = "1234567890123456803";
export const RFQ_PUBLICATION_ID = "1234567890123456804";
export const RFQ_AUDIENCE_HASH = "c".repeat(64);
export const RFQ_SELLER_PAYLOAD_HASH = "d".repeat(64);

export function pricePreviewData(
	overrides: {
		// 重新試算時 command 帶的是買家剛編輯的區間，不再是預設的 30／42——
		// 呼叫端要驗證「編輯後的區間原樣回送」就傳這個覆寫，不要在呼叫端另起一份
		// preview 物件。
		buyerRequestedRange?: {
			lowerBoundAmount: string;
			upperBoundAmount: string;
		};
	} = {},
) {
	return {
		pricePreviewId: RFQ_PREVIEW_ID,
		version: "1",
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: overrides.buyerRequestedRange?.lowerBoundAmount ?? "30",
			upperBoundAmount: overrides.buyerRequestedRange?.upperBoundAmount ?? "42",
			basisQuantity: "1",
			basisUnit: "KG",
		},
		payloadHash: RFQ_PREVIEW_HASH,
		quoteTotalRange: {
			currency: "TWD",
			lowerBoundAmount: "15000",
			upperBoundAmount: "21000",
			quantity: "500",
			quantityUnit: "KG",
		},
	};
}

export interface RfqMockOptions {
	previewHandler?: (route: Route) => Promise<void> | void;
	confirmationHandler?: (route: Route) => Promise<void> | void;
	/** 停在 searching 階段：不等候選資料出現就回傳。搜尋列的 e2e 用。 */
	stopAtSearching?: boolean;
	/** GET candidate-searches/{runId} 的處理器。預設立刻回 SUCCEEDED（既有行為）。 */
	candidateSearchPollHandler?: (route: Route) => Promise<void> | void;
	publicationPreviewHandler?: (route: Route) => Promise<void> | void;
	publicationExecuteHandler?: (route: Route) => Promise<void> | void;
	/** GET runs/{runId}/events 的處理器。預設 503——那是今天 prod 的真實行為
	 * （capability flag 關著），所以其他 spec 不必知道這個端點就會走降級路徑。 */
	runEventsHandler?: (route: Route) => Promise<void> | void;
	/** GET conversations/{id}/runtime-snapshot 的處理器（稿 R-02 三區版面的
	 * 發布事實來源）。預設帶一筆 `RFQ_PUBLICATION_RESULT`。 */
	runtimeSnapshotHandler?: (route: Route) => Promise<void> | void;
	/** GET requests/{id}/seller-responses 的處理器。預設空清單——「0 筆」是一次
	 * 成功讀取的結論，跟「尚未取得」是兩件事。 */
	sellerResponsesHandler?: (route: Route) => Promise<void> | void;
	/** GET requests/{id}/snapshot 的處理器。重整還原那條路徑要靠它注入
	 * `publishedPublication`。 */
	snapshotHandler?: (route: Route) => Promise<void> | void;
}

/**
 * 重整後 snapshot 帶回來的已發布成果。
 *
 * ⛔ 形狀是 2026-09-19 對 dev 實際擷取的（synthetic buyer 走完 turn→resume→
 * price preview→confirmation→candidate search→selection→publication preview→
 * execute，再 GET snapshot），與 Gateway 的 `ProcurementPublishedPublicationDTO`
 * 逐欄一致：**只有這三個鍵**。不要憑印象多加 state／audienceMode——正規化器會
 * 忽略不認得的鍵，測試照樣綠，但 fixture 會被後來的人當成契約。
 *
 * 同一次擷取另外確認：snapshot 的 top-level **沒有 candidateSet**，所以「重整後
 * 只有 publication、沒有候選」不是假設出來的邊界，是唯一可能的形狀。
 */
export function publishedPublicationSnapshotFact() {
	return {
		publicationId: RFQ_PUBLICATION_ID,
		version: "1",
		publishedAt: "2026-08-22T12:40:00Z",
	};
}

/** 稿 R-02 三區版面讀到的發布結果 artifact。三個 count 是發布當下的意圖分布。 */
export function publicationResultArtifact(
	counts: {
		recipientCount?: number;
		queuedCount?: number;
		notSentCount?: number;
	} = {},
) {
	return {
		artifactId: "1234567890123456810",
		runId: RFQ_RUN_ID,
		artifactType: "RFQ_PUBLICATION_RESULT",
		version: "3",
		status: "READY",
		data: {
			publicationId: RFQ_PUBLICATION_ID,
			version: "1",
			sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
			recipientCount: counts.recipientCount ?? 1,
			queuedCount: counts.queuedCount ?? 1,
			notSentCount: counts.notSentCount ?? 0,
		},
		checksum: "e".repeat(64),
		domainRefType: "",
		domainRefId: "",
		asOf: "",
		createdAt: "2026-08-22T12:40:00Z",
	};
}

export async function setupRfqPriceReview(
	page: Page,
	options: RfqMockOptions = {},
) {
	const legacy = { calls: 0 };
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
	const testOrigin =
		process.env.RFQ_TEST_ORIGIN ??
		`http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`;
	await page.context().addCookies([
		{
			name: "hk_session",
			value: "opaque",
			url: testOrigin,
		},
	]);

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
		"**/api/v1/app/buyers/me/procurement/turns/stream",
		(route) =>
			route.fulfill({
				status: 503,
				contentType: "application/json",
				body: JSON.stringify({ code: 503, message: "stream unavailable" }),
			}),
	);
	await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: {
					requestId: RFQ_REQUEST_ID,
					revisionId: RFQ_REVISION_ID,
					conversationId: RFQ_CONVERSATION_ID,
					slots: {
						item: "珍珠芭樂",
						quantity: 50,
						unit: "KG",
						delivery_city: "桃園市",
						delivery_date: "2026-09-01",
					},
					missingRequired: [],
					rejected: [],
					degraded: false,
					reply: "需求已整理好，可以開始搜尋符合條件的商家。",
					recordedThisTurn: [],
					question: null,
				},
			}),
		}),
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/snapshot`,
		(route) => {
			if (options.snapshotHandler) return options.snapshotHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: RFQ_REQUEST_ID,
						revisionId: RFQ_REVISION_ID,
						status: "REQUIREMENT_READY",
						slots: {
							item: "珍珠芭樂",
							quantity: 50,
							unit: "KG",
							delivery_city: "桃園市",
							delivery_date: "2026-09-01",
						},
						missingRequired: [],
					},
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/candidate-searches`,
		(route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: RFQ_REQUEST_ID,
						runId: RFQ_RUN_ID,
						status: "RUNNING",
					},
				}),
			}),
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/candidate-searches/${RFQ_RUN_ID}`,
		(route) => {
			if (options.candidateSearchPollHandler)
				return options.candidateSearchPollHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: RFQ_REQUEST_ID,
						runId: RFQ_RUN_ID,
						candidateSetId: "candidate-set-e2e",
						status: "SUCCEEDED",
						platformLaneStatus: "SUCCEEDED",
						externalLaneStatus: "SUCCEEDED",
						failureReason: "",
						candidates: {
							platform: [
								{
									candidateId: "seller-green",
									sourceId: "product-green",
									lane: "PLATFORM",
									displayName: "範例鮮果合作社",
									score: 94,
									matchedFacts: [{ key: "item", label: "珍珠芭樂" }],
									unverifiableFacts: [],
									factsAsOf: "2026-08-14T12:00:00Z",
									selected: false,
								},
							],
							external: [],
						},
					},
				}),
			});
		},
	);
	await page.route(
		"**/api/v1/app/buyers/me/procurement/runs/*/events",
		(route) => {
			if (options.runEventsHandler) return options.runEventsHandler(route);
			return route.fulfill({
				status: 503,
				contentType: "application/json",
				body: JSON.stringify({
					code: 503,
					reason: "CAPABILITY_UNAVAILABLE",
					message: "replay 端點尚未開放",
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/selections`,
		(route) => {
			const body = route.request().postDataJSON() as {
				candidateId: string;
				selected: boolean;
			};
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: { candidateId: body.candidateId, selected: body.selected },
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/rfq-price-previews`,
		(route) => {
			expect(route.request().method()).toBe("POST");
			if (options.previewHandler) return options.previewHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: pricePreviewData() }),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/rfq-confirmations`,
		(route) => {
			expect(route.request().method()).toBe("POST");
			if (options.confirmationHandler)
				return options.confirmationHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						confirmedRfqSnapshotId: RFQ_CONFIRMED_ID,
						payloadHash: RFQ_CONFIRMED_HASH,
					},
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/rfq-publications`,
		(route) => {
			expect(route.request().method()).toBe("POST");
			if (options.publicationExecuteHandler)
				return options.publicationExecuteHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						publicationId: RFQ_PUBLICATION_ID,
						publicationVersion: "1",
						state: "PUBLISHED",
						audienceMode: "WISH_POOL",
						audienceHash: RFQ_AUDIENCE_HASH,
						sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
						publishedAt: "2026-08-22T12:40:00Z",
						replayed: false,
					},
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/rfq-publications/preview`,
		(route) => {
			expect(route.request().method()).toBe("POST");
			if (options.publicationPreviewHandler)
				return options.publicationPreviewHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						approvalId: RFQ_APPROVAL_ID,
						approvalVersion: "1",
						audienceHash: RFQ_AUDIENCE_HASH,
						sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
						sellerFacingPayloadJson: '{"item":"蘋果"}',
						expiresAt: "2026-08-22T12:36:00Z",
						audiencePolicyId: "WISH_POOL_ALL_ACTIVE_SELLERS_V1",
						audiencePolicyVersion: "1",
						recipientCount: "0",
					},
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/conversations/${RFQ_CONVERSATION_ID}/runtime-snapshot`,
		(route) => {
			if (options.runtimeSnapshotHandler)
				return options.runtimeSnapshotHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						conversationId: RFQ_CONVERSATION_ID,
						requestId: RFQ_REQUEST_ID,
						status: "PUBLISHED",
						title: "珍珠芭樂 50 KG",
						messages: [],
						runs: [],
						activeRunId: "",
						artifacts: [publicationResultArtifact()],
					},
				}),
			});
		},
	);
	await page.route(
		`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/seller-responses`,
		(route) => {
			if (options.sellerResponsesHandler)
				return options.sellerResponsesHandler(route);
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: [] }),
			});
		},
	);
	await page.route(
		"**/api/v1/app/buyers/me/procurement/requests/*/quote-batches**",
		(route) => {
			legacy.calls += 1;
			return route.fulfill({
				status: 500,
				contentType: "application/json",
				body: JSON.stringify({ code: 500, message: "legacy route forbidden" }),
			});
		},
	);

	await page.addInitScript(() => {
		sessionStorage.setItem("hawkr:initial-loader-seen", "1");
	});
	await page.goto(RFQ_PATH, { waitUntil: "domcontentloaded" });
	await page
		.locator("#procurement-clarify-input")
		.fill("我要採購 50 KG 珍珠芭樂");
	await page.getByRole("button", { name: "送出" }).click();
	await page.getByRole("button", { name: "交給 Hawkr Agent 搜尋" }).click();

	if (options.stopAtSearching) return { legacy };

	const openCanvas = page.getByRole("button", { name: "查看候選資料" });
	if ((page.viewportSize()?.width ?? 1200) < 1200) {
		await expect(openCanvas).toBeVisible({ timeout: 15_000 });
		await openCanvas.click();
	}
	await expect(page.getByText("範例鮮果合作社", { exact: true })).toBeVisible({
		timeout: 15_000,
	});
	await page.locator('label[for="procurement-candidate-seller-green"]').click();
	await expect(
		page.getByRole("checkbox", { name: "選取 範例鮮果合作社" }),
	).toBeChecked();
	await page.getByRole("button", { name: "建立詢價草稿" }).click();
	await expect(
		page.getByRole("heading", { name: "核對需求與價格" }),
	).toBeVisible();
	await page.getByRole("button", { name: "查看預計名單" }).click();
	await expect(
		page.getByRole("heading", { name: "預計邀請 1 筆候選" }),
	).toBeVisible();
	await page.getByRole("button", { name: "取得價格範圍" }).click();
	// 買家自己設定希望詢價區間後才取得試算（伺服器不再預填）。
	await page.getByLabel("希望下限").fill("30");
	await page.getByLabel("希望上限").fill("42");
	await page.getByRole("button", { name: "取得無參考價試算" }).click();

	return { legacy };
}
