import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	GetRuntimeSnapshotResponseSchema,
	PublishedRfqSchema,
	RequestStatus,
	RunStatus,
	RunSummarySchema,
	RunType,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import type { BuyerQuoteAcceptanceResponse } from "./buyer-quote-contract";
import {
	deriveRfqProgress,
	hasLiveRun,
	runCancelCopy,
	type RuntimeSnapshot,
	runtimeSnapshotFromProto,
	selectCancellableRun,
} from "./runtime-snapshot-contract";
import type { PublishedRfqPublication } from "./types";

const PUBLISHED_AT = "2026-08-20T12:36:00.000Z";

function run(
	init: Partial<{
		runId: string;
		runType: RunType;
		status: RunStatus;
		terminal: boolean;
	}> = {},
) {
	return create(RunSummarySchema, {
		runId: "run-1",
		runType: RunType.TURN,
		status: RunStatus.SUCCEEDED,
		terminal: true,
		...init,
	});
}

function snapshot(
	init: Parameters<typeof create<typeof GetRuntimeSnapshotResponseSchema>>[1] = {},
): RuntimeSnapshot {
	return runtimeSnapshotFromProto(
		create(GetRuntimeSnapshotResponseSchema, {
			conversationId: "conv-1",
			requestId: "req-1",
			status: RequestStatus.REQUIREMENT_READY,
			title: "蘋果 100 公斤",
			runs: [run()],
			...init,
		}),
	);
}

const publication: PublishedRfqPublication = {
	publicationId: "pub-1",
	version: "3",
	publishedAt: PUBLISHED_AT,
	recipientCount: 2,
	queuedCount: 2,
	sellerFacingHash: "b".repeat(64),
};

function acceptance(
	overrides: Partial<BuyerQuoteAcceptanceResponse> = {},
): BuyerQuoteAcceptanceResponse {
	return {
		buyerAcceptanceId: "acc-1",
		sellerResponseId: "resp-1",
		sellerResponseVersion: "v1",
		acceptedDealPrice: "18900",
		currency: "TWD",
		acceptedAt: "2026-08-22T06:00:00Z",
		orderProvisioningState: "CREATING_ORDER",
		orderNo: "",
		paymentDeadline: "",
		replayed: false,
		...overrides,
	};
}

describe("runtimeSnapshotFromProto", () => {
	it("沒有發布時 publication 是 null，不補值", () => {
		expect(snapshot().publication).toBeNull();
	});

	it("發布結果的時間與數量原樣帶出", () => {
		const view = snapshot({
			publishedRfq: create(PublishedRfqSchema, {
				publicationId: "pub-1",
				version: "3",
				publishTime: timestampFromDate(new Date(PUBLISHED_AT)),
				recipientCount: 2,
				queuedCount: 1,
				sellerFacingHash: "b".repeat(64),
			}),
		});
		expect(view.publication).toEqual({
			publicationId: "pub-1",
			version: "3",
			publishedAt: PUBLISHED_AT,
			recipientCount: 2,
			queuedCount: 1,
			sellerFacingHash: "b".repeat(64),
		});
	});

	it("執行中的 run 與需求狀態轉成畫面代碼", () => {
		const view = snapshot({
			status: RequestStatus.MATCHING,
			runs: [
				run({
					runId: "run-2",
					runType: RunType.CANDIDATE_SEARCH,
					status: RunStatus.RUNNING,
					terminal: false,
				}),
			],
		});
		expect(view.status).toBe("MATCHING");
		expect(view.runs).toEqual([
			{
				runId: "run-2",
				runType: "CANDIDATE_SEARCH",
				status: "RUNNING",
				terminal: false,
			},
		]);
	});
});

describe("hasLiveRun", () => {
	it("看 runs[].terminal，不自己用 status 推導終態", () => {
		const view = snapshot({
			runs: [
				run({
					runId: "run-2",
					runType: RunType.CANDIDATE_SEARCH,
					status: RunStatus.QUEUED,
					terminal: false,
				}),
				run(),
			],
		});
		expect(hasLiveRun(view)).toBe(true);
		expect(hasLiveRun(snapshot())).toBe(false);
	});
});

describe("六步驟進度", () => {
	it("沒有發布事實時第一步只是「進行中」，不得標成已確認", () => {
		const steps = deriveRfqProgress({
			publication: null,
			responseCount: null,
			acceptance: null,
		});
		expect(steps.map((s) => s.state)).toEqual([
			"current",
			"pending",
			"pending",
			"pending",
			"pending",
			"pending",
		]);
	});

	it("有發布結果就蘊含核准存在，但那一步沒有時間戳可寫", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 0,
			acceptance: null,
		});
		expect(steps[0]).toMatchObject({ state: "done", at: "" });
		expect(steps[1]).toMatchObject({ state: "done", at: PUBLISHED_AT });
		expect(steps[2].state).toBe("current");
	});

	it("回價筆數還沒讀到時不得當成 0，第三步仍停在等待", () => {
		const unknown = deriveRfqProgress({
			publication,
			responseCount: null,
			acceptance: null,
		});
		expect(unknown[2].state).toBe("current");
		expect(unknown[3].state).toBe("pending");
	});

	it("收到回價後等待那一步收線，換成買家比較", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 2,
			acceptance: null,
		});
		expect(steps[2].state).toBe("done");
		expect(steps[3].state).toBe("current");
	});

	it("CREATING_ORDER 只到「建立中」，第六步永遠不會標成已完成", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 2,
			acceptance: acceptance(),
		});
		expect(steps[3]).toMatchObject({
			state: "done",
			at: "2026-08-22T06:00:00Z",
		});
		expect(steps[4].state).toBe("current");
		expect(steps[5].state).toBe("pending");
	});

	it("ORDER_FAILED 是明確失敗，不是待進行", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 2,
			acceptance: acceptance({ orderProvisioningState: "ORDER_FAILED" }),
		});
		expect(steps[4].state).toBe("failed");
	});

	it("ORDER_CREATED 才算建單完成，付款那一步仍是待進行", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 2,
			acceptance: acceptance({
				orderProvisioningState: "ORDER_CREATED",
				orderNo: "HK20260822001",
				paymentDeadline: "2026-08-23T06:00:00Z",
			}),
		});
		expect(steps[4].state).toBe("done");
		expect(steps[5].state).toBe("pending");
	});

	it("六步驟的標籤與順序逐字固定", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 0,
			acceptance: null,
		});
		expect(steps.map((s) => s.label)).toEqual([
			"已確認 RFQ 快照",
			"已發布詢價",
			"等待供應商正式回價",
			"買家比較並接受指定報價版本",
			"建立待付款訂單",
			"付款成功後正式成立",
		]);
	});
});

describe("停止任務：可取消的 run", () => {
	it("伺服器指的 activeRunId 且未終態才可取消", () => {
		const view = snapshot({
			activeRunId: "run-2",
			runs: [
				run({
					runId: "run-2",
					status: RunStatus.RUNNING,
					terminal: false,
				}),
			],
		});
		expect(selectCancellableRun(view)).toEqual({ runId: "run-2" });
	});

	it("沒有 activeRunId 就沒有可取消的 run，即使 runs 裡有非終態的列", () => {
		const view = snapshot({
			runs: [
				run({
					runId: "run-2",
					runType: RunType.CANDIDATE_SEARCH,
					status: RunStatus.QUEUED,
					terminal: false,
				}),
			],
		});
		expect(selectCancellableRun(view)).toBeNull();
	});

	it("activeRunId 指到的那一列已經終態時不可取消", () => {
		const view = snapshot({ activeRunId: "run-1" });
		expect(selectCancellableRun(view)).toBeNull();
	});

	it("activeRunId 在 runs 裡找不到對應列時不可取消", () => {
		const view = snapshot({ activeRunId: "run-missing" });
		expect(selectCancellableRun(view)).toBeNull();
	});
});

const ut = uiTextFor("zh-TW");
const RUN_CANCEL_COPY = {
	REQUESTED: runCancelCopy(ut, "REQUESTED"),
	ALREADY_CANCELLED: runCancelCopy(ut, "ALREADY_CANCELLED"),
	ALREADY_TERMINAL: runCancelCopy(ut, "ALREADY_TERMINAL"),
	TOO_LATE: runCancelCopy(ut, "TOO_LATE"),
};

describe("取消文案", () => {
	it("只有 REQUESTED 講「正在停止」；另外三句都不得宣稱已完成的結果被作廢", () => {
		expect(RUN_CANCEL_COPY.REQUESTED).toContain("停止");
		for (const key of [
			"ALREADY_CANCELLED",
			"ALREADY_TERMINAL",
			"TOO_LATE",
		] as const) {
			expect(RUN_CANCEL_COPY[key]).not.toContain("已為您取消");
		}
		expect(RUN_CANCEL_COPY.ALREADY_TERMINAL).toContain("沒有被這次停止作廢");
		expect(RUN_CANCEL_COPY.TOO_LATE).toContain("來不及停止");
	});
});
