// @vitest-environment jsdom
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import { useRfqRuntimeSnapshot } from "./useRfqRuntimeSnapshot";

/**
 * 稿 R-02 資料層的**傳輸契約**（issue #2538 A8 後半）。
 *
 * 推導在 `__tests__/runtime-snapshot-contract.test.ts`，元件行為在
 * `__tests__/QuoteBatchSummary.test.ts`；這裡只釘住四件會在接線當天才看得出
 * 對錯的事：兩支端點的路徑、兩支獨立成敗、解析不過要 fail closed、
 * 沒有 conversationId 時完全不發請求。
 */

const { getMock } = vi.hoisted(() => ({ getMock: vi.fn() }));

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			authed: { ...actual.apiClient.authed, get: getMock },
		},
	};
});

const CONVERSATION_ID = "1234567890123456780";
const REQUEST_ID = "1234567890123456781";
const SNAPSHOT_PATH = `/buyers/me/procurement/conversations/${CONVERSATION_ID}/runtime-snapshot`;
const RESPONSES_PATH = `/buyers/me/procurement/requests/${REQUEST_ID}/seller-responses`;

function snapshotData(overrides: Record<string, unknown> = {}) {
	return {
		conversationId: CONVERSATION_ID,
		requestId: REQUEST_ID,
		status: "REQUIREMENT_READY",
		title: "蘋果 100 公斤",
		messages: [],
		runs: [
			{
				runId: "1234567890123456791",
				runType: "PUBLICATION",
				status: "SUCCEEDED",
				lastSeqId: "1",
				minAvailableSeqId: "1",
				terminal: true,
			},
		],
		artifacts: [
			{
				artifactId: "1234567890123456790",
				runId: "1234567890123456791",
				artifactType: "RFQ_PUBLICATION_RESULT",
				version: "3",
				status: "READY",
				data: {
					publicationId: "1234567890123456788",
					version: "1",
					sellerFacingPayloadHash: "b".repeat(64),
					recipientCount: 0,
					queuedCount: 0,
					notSentCount: 0,
				},
				checksum: "a".repeat(64),
				asOf: "2026-08-20T12:36:00Z",
				createdAt: "2026-08-20T12:36:00Z",
			},
		],
		...overrides,
	};
}

function sellerResponseRow(overrides: Record<string, unknown> = {}) {
	return {
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		status: "SUBMITTED",
		payloadHash: "c".repeat(64),
		submittedAt: "2026-08-21T02:00:00Z",
		seller: {
			displayName: "範例鮮果合作社",
			partyType: "PLATFORM_SELLER",
			storeCode: "greenfield",
		},
		quote: {
			responseType: "MANUAL_QUOTE",
			offeredQuality: { grade: "特級", inspectionNotes: "" },
			unitBasis: { quantity: "1", unit: "KG" },
			offeredQuantity: "100",
			offeredQuantityUnit: "KG",
			conversionBasis: "",
			currency: "TWD",
			goodsAmount: "6000",
			shippingAmount: "0",
			taxAmount: "300",
			taxInclusionBasis: "TAX_EXCLUDED",
			totalAmount: "6300",
			leadTimeDays: "2",
			promisedDeliveryAt: "",
			validUntil: "",
			termsNote: "",
			deviationDisclosure: "",
		},
		outsideBuyerRequestedRange: false,
		shortlisted: false,
		acceptability: "ACCEPTABLE",
		...overrides,
	};
}

function routeGet(snapshot: unknown, responses: unknown) {
	getMock.mockImplementation(async (path: string) => {
		if (path === SNAPSHOT_PATH) {
			if (snapshot instanceof Error) throw snapshot;
			return { code: 0, data: snapshot };
		}
		if (path === RESPONSES_PATH) {
			if (responses instanceof Error) throw responses;
			return { code: 0, data: responses };
		}
		throw new Error(`unexpected path ${path}`);
	});
}

async function mount(snapshot: unknown, responses: unknown) {
	routeGet(snapshot, responses);
	const hook = renderHook(() =>
		useRfqRuntimeSnapshot(CONVERSATION_ID, REQUEST_ID),
	);
	await waitFor(() => {
		expect(hook.result.current.state.status).not.toBe("loading");
	});
	return hook;
}

afterEach(() => {
	vi.clearAllMocks();
});

describe("端點", () => {
	it("讀 runtime-snapshot 與 seller-responses 兩支", async () => {
		await mount(snapshotData(), [sellerResponseRow()]);
		expect(getMock).toHaveBeenCalledWith(SNAPSHOT_PATH);
		expect(getMock).toHaveBeenCalledWith(RESPONSES_PATH);
	});

	it("沒有 conversationId 時一支都不打", async () => {
		routeGet(snapshotData(), []);
		const { result } = renderHook(() =>
			useRfqRuntimeSnapshot(null, REQUEST_ID),
		);
		await waitFor(() => expect(result.current.state.status).toBe("idle"));
		expect(getMock).not.toHaveBeenCalled();
	});

	it("沒有 requestId 時只讀快照，筆數維持「尚未取得」", async () => {
		routeGet(snapshotData(), []);
		const { result } = renderHook(() =>
			useRfqRuntimeSnapshot(CONVERSATION_ID, null),
		);
		await waitFor(() => expect(result.current.state.status).toBe("success"));
		expect(getMock).toHaveBeenCalledTimes(1);
		expect(result.current.counts.responseCount).toBeNull();
	});
});

describe("解析", () => {
	it("挑得出發布結果 artifact", async () => {
		const { result } = await mount(snapshotData(), []);
		expect(result.current.publication).toMatchObject({
			artifactVersion: "3",
			publishedAt: "2026-08-20T12:36:00Z",
		});
	});

	it("快照解析不過時 fail closed，不呈現半份狀態", async () => {
		const { result } = await mount(snapshotData({ runs: "nope" }), []);
		expect(result.current.state.status).toBe("error");
		if (result.current.state.status === "error") {
			expect(result.current.state.error.code).toBe("INVALID_RESPONSE");
		}
		expect(result.current.publication).toBeNull();
	});

	it("回價清單成功讀到空陣列才算 0 筆", async () => {
		const { result } = await mount(snapshotData(), []);
		await waitFor(() => expect(result.current.counts.responseCount).toBe(0));
		expect(result.current.counts.shortlistCount).toBe(0);
		expect(result.current.counts.error).toBeNull();
	});

	it("數得出 shortlist", async () => {
		const { result } = await mount(snapshotData(), [
			sellerResponseRow({ shortlisted: true }),
			sellerResponseRow({
				sellerResponseVersion: "1234567890123456792",
				shortlisted: false,
			}),
		]);
		await waitFor(() => expect(result.current.counts.responseCount).toBe(2));
		expect(result.current.counts.shortlistCount).toBe(1);
	});

	it("清單有一筆壞掉就不給筆數，而不是只數解析得出來的那幾筆", async () => {
		const { result } = await mount(snapshotData(), [
			sellerResponseRow(),
			sellerResponseRow({ payloadHash: "nope" }),
		]);
		await waitFor(() => expect(result.current.counts.error).not.toBeNull());
		expect(result.current.counts.responseCount).toBeNull();
	});
});

describe("兩支獨立成敗", () => {
	it("清單掛掉不影響已經讀到的發布事實", async () => {
		const { result } = await mount(
			snapshotData(),
			new ApiClientError("upstream exploded", 503, 503, null, ""),
		);
		expect(result.current.state.status).toBe("success");
		expect(result.current.publication?.artifactVersion).toBe("3");
		await waitFor(() =>
			expect(result.current.counts.error?.code).toBe("SERVER_ERROR"),
		);
		expect(result.current.counts.error?.detail).not.toContain("upstream");
		expect(result.current.counts.responseCount).toBeNull();
	});

	it("快照掛掉時筆數照樣讀得到", async () => {
		const { result } = await mount(
			new ApiClientError("gone", 404, 404, null, ""),
			[sellerResponseRow()],
		);
		expect(result.current.state.status).toBe("error");
		await waitFor(() => expect(result.current.counts.responseCount).toBe(1));
	});
});
