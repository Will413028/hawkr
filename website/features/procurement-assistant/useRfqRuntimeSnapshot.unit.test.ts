// @vitest-environment jsdom
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BuyerFacingSellerResponseSchema,
	GetRuntimeSnapshotResponseSchema,
	ListSellerResponsesResponseSchema,
	PublishedRfqSchema,
	RequestStatus,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useRfqRuntimeSnapshot } from "./useRfqRuntimeSnapshot";

/**
 * 稿 R-02 資料層的傳輸契約。推導在 `runtime-snapshot-contract.unit.test.ts`，
 * 元件行為在 `__tests__/QuoteBatchSummary.test.ts`；這裡釘住：兩個讀取的請求形狀、
 * 回價清單讀取依附在發布結果上、失敗不得被當成 0 筆、沒有 conversationId 時完全不發請求。
 */

const rpc = vi.hoisted(() => ({
	handlers: {} as Record<string, (request: never) => unknown>,
}));

vi.mock("@/lib/connect/procurement", async () => {
	const { fakeProcurementClient } = await import("./testing/fake-procurement");
	return { buyerProcurementClient: () => fakeProcurementClient(rpc.handlers) };
});
vi.mock("@/lib/i18n/ui-text", async () => {
	const { uiTextStub } = await import("./testing/fake-procurement");
	return uiTextStub;
});

const CONVERSATION_ID = "conv-1";
const REQUEST_ID = "req-1";

function snapshotResponse(withPublication = true) {
	return create(GetRuntimeSnapshotResponseSchema, {
		conversationId: CONVERSATION_ID,
		requestId: REQUEST_ID,
		status: RequestStatus.REQUIREMENT_READY,
		title: "蘋果 100 公斤",
		publishedRfq: withPublication
			? create(PublishedRfqSchema, {
					publicationId: "pub-1",
					version: "3",
					publishTime: timestampFromDate(new Date("2026-08-20T12:36:00Z")),
					recipientCount: 2,
					queuedCount: 2,
					sellerFacingHash: "b".repeat(64),
				})
			: undefined,
	});
}

function row(shortlisted = false) {
	return create(BuyerFacingSellerResponseSchema, {
		sellerResponseId: "resp-1",
		sellerResponseVersion: "v1",
		shortlisted,
	});
}

const listCalls = vi.fn();

function route(options: {
	snapshot?: ReturnType<typeof snapshotResponse> | Error;
	responses?: ReturnType<typeof row>[] | Error;
}) {
	rpc.handlers.getRuntimeSnapshot = () => {
		const s = options.snapshot ?? snapshotResponse();
		if (s instanceof Error) throw s;
		return s;
	};
	rpc.handlers.listSellerResponses = (request) => {
		listCalls(request);
		const r = options.responses ?? [];
		if (r instanceof Error) throw r;
		return create(ListSellerResponsesResponseSchema, { sellerResponses: r });
	};
}

async function mount(options: Parameters<typeof route>[0]) {
	route(options);
	const hook = renderHook(() =>
		useRfqRuntimeSnapshot(CONVERSATION_ID, REQUEST_ID),
	);
	await waitFor(() => {
		expect(hook.result.current.state.status).not.toBe("loading");
	});
	return hook;
}

afterEach(() => {
	rpc.handlers = {};
	vi.clearAllMocks();
});

describe("請求", () => {
	it("先讀快照，再以快照裡的發布編號讀回價清單", async () => {
		await mount({ responses: [row()] });
		await waitFor(() => expect(listCalls).toHaveBeenCalledTimes(1));
		expect(listCalls.mock.calls[0][0]).toMatchObject({
			requestId: REQUEST_ID,
			publicationId: "pub-1",
		});
	});

	it("沒有 conversationId 時什麼都不發", async () => {
		route({});
		const { result } = renderHook(() =>
			useRfqRuntimeSnapshot(null, REQUEST_ID),
		);
		await waitFor(() => expect(result.current.state.status).toBe("idle"));
		expect(listCalls).not.toHaveBeenCalled();
	});

	it("沒有 requestId 時只讀快照，筆數維持「尚未取得」", async () => {
		route({});
		const { result } = renderHook(() =>
			useRfqRuntimeSnapshot(CONVERSATION_ID, null),
		);
		await waitFor(() => expect(result.current.state.status).toBe("success"));
		expect(listCalls).not.toHaveBeenCalled();
		expect(result.current.counts.responseCount).toBeNull();
	});

	it("還沒有發布時不讀回價清單", async () => {
		const { result } = await mount({ snapshot: snapshotResponse(false) });
		expect(result.current.publication).toBeNull();
		expect(listCalls).not.toHaveBeenCalled();
		expect(result.current.counts.responseCount).toBeNull();
	});
});

describe("轉換", () => {
	it("挑得出發布結果", async () => {
		const { result } = await mount({});
		expect(result.current.publication).toMatchObject({
			version: "3",
			publishedAt: "2026-08-20T12:36:00.000Z",
		});
	});

	it("回價清單成功讀到空陣列才算 0 筆", async () => {
		const { result } = await mount({ responses: [] });
		await waitFor(() => expect(result.current.counts.responseCount).toBe(0));
		expect(result.current.counts.shortlistCount).toBe(0);
		expect(result.current.counts.error).toBeNull();
	});

	it("數得出 shortlist", async () => {
		const { result } = await mount({ responses: [row(true), row(false)] });
		await waitFor(() => expect(result.current.counts.responseCount).toBe(2));
		expect(result.current.counts.shortlistCount).toBe(1);
	});
});

describe("失敗", () => {
	it("快照讀失敗是 error 狀態，不呈現半份狀態，筆數也不猜", async () => {
		const { result } = await mount({
			snapshot: new ConnectError("gone", Code.NotFound),
		});
		expect(result.current.state.status).toBe("error");
		expect(result.current.publication).toBeNull();
		expect(result.current.counts.responseCount).toBeNull();
	});

	it("清單掛掉不影響已經讀到的發布事實，筆數是「尚未取得」而不是 0", async () => {
		const { result } = await mount({
			responses: new ConnectError("upstream exploded", Code.Unavailable),
		});
		expect(result.current.state.status).toBe("success");
		expect(result.current.publication?.version).toBe("3");
		await waitFor(() =>
			expect(result.current.counts.error?.code).toBe("SERVER_ERROR"),
		);
		expect(result.current.counts.error?.detail).not.toContain("upstream");
		expect(result.current.counts.responseCount).toBeNull();
	});
});
