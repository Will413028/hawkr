import type { Page } from "@playwright/test";
import { expect } from "@playwright/test";
import { seedStoredActiveRole, setupGatewayMocks } from "./mock-gateway";
import {
	mockProcurementUnaryAbortable,
	mockWatchRun,
	turnResultJson,
	watchFrame,
} from "./mock-procurement-connect";
import { gotoStreamed } from "./streamed-navigation";

/**
 * 「一輪釐清 → 候選搜尋 → 價格試算 → 發布核准 → 發布」的 BuyerProcurementAppService Connect mock。
 * wire 形狀（proto3 JSON、streaming envelope）見 `mock-procurement-connect.ts`；欄位以
 * `api/hawkr/procurement/v1alpha1/procurement.proto` 為準。沒有確認快照這一步的方法（A9-D35）。
 */

export const RFQ_PATH = "/account/messages/assistant/demo";
export const RFQ_REQUEST_ID = "1234567890123456788";
export const RFQ_CONVERSATION_ID = "1234567890123456790";
export const RFQ_REVISION_ID = "1234567890123456791";
/** 候選搜尋的 run。 */
export const RFQ_RUN_ID = "1234567890123456800";
/** 送出需求那一輪的 run。 */
export const RFQ_TURN_RUN_ID = "1234567890123456799";
export const RFQ_CANDIDATE_SET_ID = "candidate-set-e2e";
export const RFQ_APPROVAL_ID = "1234567890123456803";
export const RFQ_PUBLICATION_ID = "1234567890123456804";
export const RFQ_AUDIENCE_HASH = "c".repeat(64);
export const RFQ_SELLER_PAYLOAD_HASH = "d".repeat(64);

export const RFQ_IDS = {
	requestId: RFQ_REQUEST_ID,
	conversationId: RFQ_CONVERSATION_ID,
	revisionId: RFQ_REVISION_ID,
};

const RFQ_SLOTS = {
	item: "珍珠芭樂",
	quantity: 50,
	unit: "KG",
	deliveryCity: "桃園市",
	deliveryDate: { year: 2026, month: 9, day: 1 },
};

/** `PreviewRfqPriceResponse`。 */
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
		buyerRequestedRange: {
			lowerBoundAmount: overrides.buyerRequestedRange?.lowerBoundAmount ?? "30",
			upperBoundAmount: overrides.buyerRequestedRange?.upperBoundAmount ?? "42",
			basisQuantity: 1,
			basisUnit: "KG",
		},
		quoteTotalRange: {
			lowerBoundAmount: "15000",
			upperBoundAmount: "21000",
			quantity: 500,
			quantityUnit: "KG",
		},
	};
}

/** `Candidate`：沒有 `store.id`，所以候選表走單一候選那一列（checkbox id `procurement-candidate-{id}`）。 */
export function candidateJson(
	id: string,
	name: string,
	extra: Record<string, unknown> = {},
) {
	return {
		id,
		store: { name },
		matchedFacts: [{ slotKey: "SLOT_KEY_ITEM", label: "珍珠芭樂" }],
		unverifiableFacts: [],
		factsTime: "2026-08-14T12:00:00Z",
		selected: false,
		...extra,
	};
}

/** `GetCandidateSearchResponse`。 */
export function candidateSearchJson(
	status: "RUNNING" | "SUCCEEDED" | "FAILED",
	candidates: readonly Record<string, unknown>[] = [],
	runId: string = RFQ_RUN_ID,
) {
	return {
		requestId: RFQ_REQUEST_ID,
		runId,
		candidateSetId: RFQ_CANDIDATE_SET_ID,
		status: `RUN_STATUS_${status}`,
		candidates,
	};
}

/** `PreviewRfqPublicationResponse`。 */
export function publicationApprovalJson(
	overrides: Record<string, unknown> = {},
) {
	return {
		approvalId: RFQ_APPROVAL_ID,
		approvalVersion: "1",
		audienceHash: RFQ_AUDIENCE_HASH,
		sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
		expireTime: "2026-08-22T12:36:00Z",
		recipientCount: 1,
		...overrides,
	};
}

/** `ExecuteRfqPublicationResponse`。 */
export function publicationResultJson(overrides: Record<string, unknown> = {}) {
	return {
		publicationId: RFQ_PUBLICATION_ID,
		publicationVersion: "1",
		state: "PUBLICATION_STATE_PUBLISHED",
		audienceHash: RFQ_AUDIENCE_HASH,
		sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
		publishTime: "2026-08-22T12:40:00Z",
		replayed: false,
		...overrides,
	};
}

/**
 * `PublishedRfq`：`GetSnapshot` 與 `GetRuntimeSnapshot` 的 `publishedRfq`。三個 count 是發布當下的意圖分布。
 */
export function publishedRfqJson(
	counts: { recipientCount?: number; queuedCount?: number } = {},
) {
	return {
		publicationId: RFQ_PUBLICATION_ID,
		version: "1",
		publishTime: "2026-08-22T12:40:00Z",
		recipientCount: counts.recipientCount ?? 1,
		queuedCount: counts.queuedCount ?? 1,
		sellerFacingHash: RFQ_SELLER_PAYLOAD_HASH,
	};
}

/** `GetSnapshotResponse`：需求已整理好、可以開始搜尋。 */
export function snapshotReadyJson(extra: Record<string, unknown> = {}) {
	return {
		requestId: RFQ_REQUEST_ID,
		revisionId: RFQ_REVISION_ID,
		status: "REQUEST_STATUS_REQUIREMENT_READY",
		slots: RFQ_SLOTS,
		missingRequired: [],
		runs: [],
		activeRunId: "",
		...extra,
	};
}

/** `GetRuntimeSnapshotResponse`：預設帶一筆已發布的結果（R-02 三區版面的事實來源）。 */
export function runtimeSnapshotJson(extra: Record<string, unknown> = {}) {
	return {
		conversationId: RFQ_CONVERSATION_ID,
		requestId: RFQ_REQUEST_ID,
		status: "REQUEST_STATUS_MATCHING",
		title: "珍珠芭樂 50 KG",
		messages: [],
		runs: [],
		activeRunId: "",
		publishedRfq: publishedRfqJson(),
		...extra,
	};
}

/**
 * 每個處理器收到請求訊息（proto3 JSON），回應訊息；丟 `ConnectFailure` 回 Connect 錯誤，
 * 回 `ABORT_REQUEST` 讓請求在網路層失敗。
 */
type Handler = (request: Record<string, unknown>) => unknown;

export interface RfqMockOptions {
	/** 停在 searching 階段：不等候選資料出現就回傳。搜尋列的 e2e 用。 */
	stopAtSearching?: boolean;
	/** `PreviewRfqPrice`。 */
	previewHandler?: Handler;
	/** `GetCandidateSearch`。預設立刻回 SUCCEEDED 與一筆候選。 */
	candidateSearchPollHandler?: Handler;
	/** `PreviewRfqPublication`。 */
	publicationPreviewHandler?: Handler;
	/** `ExecuteRfqPublication`。 */
	publicationExecuteHandler?: Handler;
	/** 搜尋 run 的 `WatchRun` 事件。預設只有 `run_accepted` 與 `finished`（A9-D29）；有 `finished` 之後串流正常結束。 */
	searchWatchFrames?: () => readonly Record<string, unknown>[];
	/** `GetRuntimeSnapshot`（R-02 三區版面的發布事實來源）。預設帶一筆 `publishedRfq`。 */
	runtimeSnapshotHandler?: Handler;
	/** `ListSellerResponses`。預設空清單——「0 筆」是一次成功讀取的結論，跟「尚未取得」是兩件事。 */
	sellerResponsesHandler?: Handler;
	/** `GetSnapshot`。重整還原那條路徑要靠它注入 `publishedRfq`。 */
	snapshotHandler?: Handler;
}

export async function setupRfqPriceReview(
	page: Page,
	options: RfqMockOptions = {},
) {
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

	await mockProcurementUnaryAbortable(page, "ListConversations", () => ({
		conversations: [],
		nextPageToken: "",
	}));
	await mockProcurementUnaryAbortable(page, "SubmitTurn", () => ({
		requestId: RFQ_REQUEST_ID,
		conversationId: RFQ_CONVERSATION_ID,
		runId: RFQ_TURN_RUN_ID,
	}));
	await mockWatchRun(page, ({ runId }) => {
		if (runId === RFQ_TURN_RUN_ID) {
			return [
				watchFrame.accepted("t1", RFQ_TURN_RUN_ID, RFQ_CONVERSATION_ID),
				watchFrame.turnResult(
					"t2",
					turnResultJson(RFQ_IDS, {
						slots: RFQ_SLOTS,
						reply: "需求已整理好，可以開始搜尋符合條件的商家。",
					}),
				),
				watchFrame.finished("t3"),
			];
		}
		return (
			options.searchWatchFrames?.() ?? [
				watchFrame.accepted("s1", RFQ_RUN_ID, RFQ_CONVERSATION_ID),
				watchFrame.finished("s2"),
			]
		);
	});
	await mockProcurementUnaryAbortable(page, "GetSnapshot", (request) =>
		options.snapshotHandler
			? options.snapshotHandler(request)
			: snapshotReadyJson(),
	);
	await mockProcurementUnaryAbortable(page, "StartCandidateSearch", () => ({
		requestId: RFQ_REQUEST_ID,
		runId: RFQ_RUN_ID,
		status: "RUN_STATUS_RUNNING",
		candidateSetId: RFQ_CANDIDATE_SET_ID,
	}));
	await mockProcurementUnaryAbortable(page, "GetCandidateSearch", (request) =>
		options.candidateSearchPollHandler
			? options.candidateSearchPollHandler(request)
			: candidateSearchJson("SUCCEEDED", [
					candidateJson("seller-green", "範例鮮果合作社"),
				]),
	);
	await mockProcurementUnaryAbortable(page, "ToggleSelection", (request) => ({
		requestId: RFQ_REQUEST_ID,
		candidateSetId: request.candidateSetId,
		candidateId: request.candidateId,
		selected: request.selected,
	}));
	await mockProcurementUnaryAbortable(page, "PreviewRfqPrice", (request) =>
		options.previewHandler
			? options.previewHandler(request)
			: pricePreviewData(),
	);
	await mockProcurementUnaryAbortable(
		page,
		"PreviewRfqPublication",
		(request) =>
			options.publicationPreviewHandler
				? options.publicationPreviewHandler(request)
				: publicationApprovalJson(),
	);
	await mockProcurementUnaryAbortable(
		page,
		"ExecuteRfqPublication",
		(request) =>
			options.publicationExecuteHandler
				? options.publicationExecuteHandler(request)
				: publicationResultJson(),
	);
	await mockProcurementUnaryAbortable(page, "GetRuntimeSnapshot", (request) =>
		options.runtimeSnapshotHandler
			? options.runtimeSnapshotHandler(request)
			: runtimeSnapshotJson(),
	);
	await mockProcurementUnaryAbortable(page, "ListSellerResponses", (request) =>
		options.sellerResponsesHandler
			? options.sellerResponsesHandler(request)
			: { sellerResponses: [], nextPageToken: "" },
	);

	await page.addInitScript(() => {
		sessionStorage.setItem("hawkr:initial-loader-seen", "1");
	});
	await gotoStreamed(page, RFQ_PATH);
	await page
		.locator("#procurement-clarify-input")
		.fill("我要採購 50 KG 珍珠芭樂");
	await page.getByRole("button", { name: "送出" }).click();
	await page.getByRole("button", { name: "交給 Hawkr Agent 搜尋" }).click();

	if (options.stopAtSearching) return;

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
}
