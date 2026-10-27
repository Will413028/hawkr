import { describe, expect, it } from "vitest";
import type { BuyerQuoteAcceptanceResponse } from "../buyer-quote-contract";
import {
	deriveRfqProgress,
	hasLiveRun,
	type PublicationResultView,
	publicationResultDataSchema,
	RUN_CANCEL_COPY,
	runCancelResultSchema,
	runtimeSnapshotSchema,
	selectCancellableRun,
	selectPublicationResult,
} from "../runtime-snapshot-contract";

/**
 * runtime-snapshot 的 wire 契約與三區版面的推導（issue #2538 A8 後半）。
 *
 * 這支端點的形狀來自 gateway `ProcurementRuntimeSnapshotResponseDTO` 與
 * ai-service `publicationResultData`；本檔用線上真的會出現的 payload 形狀鎖住
 * 三件在接線當天才看得出對錯的事：開放值域不得誤擋、缺欄位不得補值、
 * 「不知道」不得被推導成「確定沒有」。
 */

const CHECKSUM = "a".repeat(64);
const PAYLOAD_HASH = "b".repeat(64);

function publicationData(overrides: Record<string, unknown> = {}) {
	return {
		publicationId: "1234567890123456788",
		version: "1",
		sellerFacingPayloadHash: PAYLOAD_HASH,
		recipientCount: 0,
		queuedCount: 0,
		notSentCount: 0,
		...overrides,
	};
}

function artifact(overrides: Record<string, unknown> = {}) {
	return {
		artifactId: "1234567890123456790",
		runId: "1234567890123456791",
		artifactType: "RFQ_PUBLICATION_RESULT",
		version: "3",
		status: "READY",
		data: publicationData(),
		checksum: CHECKSUM,
		asOf: "2026-08-20T12:36:00Z",
		createdAt: "2026-08-20T12:36:00Z",
		...overrides,
	};
}

function run(overrides: Record<string, unknown> = {}) {
	return {
		runId: "1234567890123456791",
		runType: "PUBLICATION",
		status: "SUCCEEDED",
		lastSeqId: "12",
		minAvailableSeqId: "1",
		terminal: true,
		...overrides,
	};
}

function snapshotWire(overrides: Record<string, unknown> = {}) {
	return {
		conversationId: "1234567890123456780",
		requestId: "1234567890123456781",
		status: "REQUIREMENT_READY",
		title: "蘋果 100 公斤",
		messages: [],
		runs: [run()],
		artifacts: [artifact()],
		...overrides,
	};
}

function acceptance(
	overrides: Partial<BuyerQuoteAcceptanceResponse> = {},
): BuyerQuoteAcceptanceResponse {
	return {
		buyerAcceptanceId: "1234567890123456805",
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
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

describe("runtime-snapshot wire 解析", () => {
	it("收下 gateway 的完整形狀", () => {
		const parsed = runtimeSnapshotSchema.safeParse(snapshotWire());
		expect(parsed.success).toBe(true);
	});

	it("activeRunId 與 openConfirmation 是 omitempty，缺席不算壞掉", () => {
		const parsed = runtimeSnapshotSchema.safeParse(snapshotWire());
		expect(parsed.success).toBe(true);
		if (parsed.success) {
			expect(parsed.data.activeRunId).toBe("");
			expect(parsed.data.openConfirmation).toBeUndefined();
		}
	});

	it("runType／status 是開放值域，沒見過的值不得整份擋掉", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			snapshotWire({
				runs: [run({ runType: "SOMETHING_NEW", status: "BRAND_NEW" })],
			}),
		);
		expect(parsed.success).toBe(true);
	});

	it("artifact 的 data 形狀由各型自己解，外層不預先押一個猜測", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			snapshotWire({
				artifacts: [artifact({ artifactType: "CANDIDATE_RESULTS", data: 42 })],
			}),
		);
		expect(parsed.success).toBe(true);
	});

	it("terminal 缺席就是壞掉的 wire，不得預設成 false", () => {
		const wire = snapshotWire();
		const runs = wire.runs as Array<Record<string, unknown>>;
		delete runs[0].terminal;
		expect(runtimeSnapshotSchema.safeParse(wire).success).toBe(false);
	});

	it("checksum 不是 64 hex 就整筆不採用", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			snapshotWire({ artifacts: [artifact({ checksum: "nope" })] }),
		);
		expect(parsed.success).toBe(false);
	});
});

describe("RFQ_PUBLICATION_RESULT 的 allowlist", () => {
	it("恰好六欄都在時解析成功", () => {
		expect(
			publicationResultDataSchema.safeParse(publicationData()).success,
		).toBe(true);
	});

	it("WISH_POOL 的三個 count 都是 0，不是缺欄位", () => {
		const parsed = publicationResultDataSchema.safeParse(publicationData());
		expect(parsed.success).toBe(true);
		if (parsed.success) {
			expect(parsed.data.recipientCount).toBe(0);
			expect(parsed.data.queuedCount).toBe(0);
			expect(parsed.data.notSentCount).toBe(0);
		}
	});

	it("count 缺席不得補 0", () => {
		const wire = publicationData();
		delete (wire as Record<string, unknown>).queuedCount;
		expect(publicationResultDataSchema.safeParse(wire).success).toBe(false);
	});
});

describe("selectPublicationResult", () => {
	it("data 讀不懂就整筆跳過，不退化成「有發布但內容未知」", () => {
		const parsed = runtimeSnapshotSchema.parse(
			snapshotWire({ artifacts: [artifact({ data: { publicationId: "1" } })] }),
		);
		expect(selectPublicationResult(parsed)).toBeNull();
	});

	it("同型多筆時挑 createdAt 最新的那一筆，不倚賴陣列順序", () => {
		const parsed = runtimeSnapshotSchema.parse(
			snapshotWire({
				artifacts: [
					artifact({
						artifactId: "1234567890123456792",
						version: "1",
						createdAt: "2026-08-19T01:00:00Z",
					}),
					artifact({ version: "4", createdAt: "2026-08-21T01:00:00Z" }),
				],
			}),
		);
		expect(selectPublicationResult(parsed)?.artifactVersion).toBe("4");
	});

	it("只有別型 artifact 時回 null", () => {
		const parsed = runtimeSnapshotSchema.parse(
			snapshotWire({
				artifacts: [artifact({ artifactType: "CANDIDATE_RESULTS", data: {} })],
			}),
		);
		expect(selectPublicationResult(parsed)).toBeNull();
	});
});

describe("hasLiveRun", () => {
	it("看 runs[].terminal 而不是 activeRunId——後者刻意排除 CANDIDATE_SEARCH", () => {
		const parsed = runtimeSnapshotSchema.parse(
			snapshotWire({
				runs: [
					run({
						runId: "1234567890123456795",
						runType: "CANDIDATE_SEARCH",
						status: "QUEUED",
						terminal: false,
					}),
					run(),
				],
				activeRunId: undefined,
			}),
		);
		expect(parsed.activeRunId).toBe("");
		expect(hasLiveRun(parsed)).toBe(true);
	});
});

const publication: PublicationResultView = {
	artifactVersion: "3",
	publishedAt: "2026-08-20T12:36:00Z",
	data: publicationResultDataSchema.parse(publicationData()),
};

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

	it("有發布結果就蘊含快照已確認，但那一步沒有時間戳可寫", () => {
		const steps = deriveRfqProgress({
			publication,
			responseCount: 0,
			acceptance: null,
		});
		expect(steps[0]).toMatchObject({ state: "done", at: "" });
		expect(steps[1]).toMatchObject({
			state: "done",
			at: "2026-08-20T12:36:00Z",
		});
		expect(steps[2].state).toBe("current");
	});

	it("回價筆數還沒讀到時不得當成 0——第三步仍停在等待", () => {
		const unknown = deriveRfqProgress({
			publication,
			responseCount: null,
			acceptance: null,
		});
		const zero = deriveRfqProgress({
			publication,
			responseCount: 0,
			acceptance: null,
		});
		expect(unknown[2].state).toBe("current");
		expect(unknown[3].state).toBe("pending");
		expect(zero[2].state).toBe("current");
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
			acceptance: acceptance({
				orderProvisioningState: "ORDER_FAILED",
			}),
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

describe("停止任務：可取消的 run 與它的 CAS token", () => {
	const ACTIVE_RUN_ID = "1234567890123456801";

	function liveSnapshot(overrides: Record<string, unknown> = {}) {
		return snapshotWire({
			activeRunId: ACTIVE_RUN_ID,
			runs: [
				run({
					runId: ACTIVE_RUN_ID,
					runType: "TURN",
					status: "RUNNING",
					terminal: false,
					runVersion: "7",
				}),
				run(),
			],
			...overrides,
		});
	}

	it("runVersion 原樣讀出來——它是取消命令唯一的 CAS token 來源", () => {
		const parsed = runtimeSnapshotSchema.safeParse(liveSnapshot());
		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(selectCancellableRun(parsed.data)).toEqual({
			runId: ACTIVE_RUN_ID,
			runVersion: "7",
			runType: "TURN",
			status: "RUNNING",
		});
	});

	it("舊版後端不回 runVersion 時整份快照仍要解得開，但那一輪不可取消", () => {
		const wire = liveSnapshot();
		for (const row of wire.runs as Array<Record<string, unknown>>) {
			delete row.runVersion;
		}
		const parsed = runtimeSnapshotSchema.safeParse(wire);
		// 缺欄位不得讓發布結果與進度一起讀不到。
		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(parsed.data.runs[0].runVersion).toBe("");
		// 空字串是「伺服器沒給」，不是一個可以送出去的版本號。
		expect(selectCancellableRun(parsed.data)).toBeNull();
		// 但快照的其他事實照常可用。
		expect(selectPublicationResult(parsed.data)).not.toBeNull();
	});

	it("runVersion 不是十進位字串時不得送出——送出去只會換回 400", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			liveSnapshot({
				runs: [
					run({
						runId: ACTIVE_RUN_ID,
						terminal: false,
						runVersion: "v7",
					}),
				],
			}),
		);
		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(selectCancellableRun(parsed.data)).toBeNull();
	});

	it("沒有 activeRunId 就沒有可取消的 run，即使 runs[] 裡有非終態的列", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			snapshotWire({
				runs: [
					// candidate search 那兩條刻意跳過 TerminalizeRun 的分支留下的
					// zombie：永遠 QUEUED，但伺服器不把它算進 activeRunId。
					run({
						runId: "1234567890123456802",
						runType: "CANDIDATE_SEARCH",
						status: "QUEUED",
						terminal: false,
						runVersion: "1",
					}),
				],
			}),
		);
		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		// 「還有事情在跑」仍然成立⋯⋯
		expect(hasLiveRun(parsed.data)).toBe(true);
		// ⋯⋯但沒有一個該對它下取消命令的 run。
		expect(selectCancellableRun(parsed.data)).toBeNull();
	});

	it("activeRunId 指到的那一列已經終態時不可取消", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			liveSnapshot({
				runs: [run({ runId: ACTIVE_RUN_ID, terminal: true, runVersion: "9" })],
			}),
		);
		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(selectCancellableRun(parsed.data)).toBeNull();
	});

	it("activeRunId 在 runs[] 裡找不到對應列時不可取消（拿不到 token）", () => {
		const parsed = runtimeSnapshotSchema.safeParse(
			liveSnapshot({ runs: [run()] }),
		);
		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(selectCancellableRun(parsed.data)).toBeNull();
	});
});

describe("取消回執", () => {
	function ack(overrides: Record<string, unknown> = {}) {
		return {
			cancelDisposition: "REQUESTED",
			runId: "1234567890123456801",
			runStatus: "CANCELLING",
			runVersion: "8",
			schemaVersion: "1",
			...overrides,
		};
	}

	it("四個 disposition 都是成功回應，四個都解得開", () => {
		for (const disposition of [
			"REQUESTED",
			"ALREADY_CANCELLED",
			"ALREADY_TERMINAL",
			"TOO_LATE",
		]) {
			const parsed = runCancelResultSchema.safeParse(
				ack({ cancelDisposition: disposition }),
			);
			expect(parsed.success).toBe(true);
		}
	});

	it("沒登記的 disposition 是解析失敗，不挑一句最像的講", () => {
		const parsed = runCancelResultSchema.safeParse(
			ack({ cancelDisposition: "MAYBE_CANCELLED" }),
		);
		expect(parsed.success).toBe(false);
	});

	it("只有 REQUESTED 講「正在停止」；另外三句都不得宣稱已完成的結果被作廢", () => {
		expect(RUN_CANCEL_COPY.REQUESTED).toContain("正在收尾");
		for (const disposition of [
			"ALREADY_CANCELLED",
			"ALREADY_TERMINAL",
			"TOO_LATE",
		] as const) {
			const copy = RUN_CANCEL_COPY[disposition];
			expect(copy).not.toContain("已為您取消");
			expect(copy).not.toContain("正在停止");
		}
		// 已經到終點的兩種必須明說結果仍然有效。
		expect(RUN_CANCEL_COPY.ALREADY_TERMINAL).toContain("仍然有效");
		expect(RUN_CANCEL_COPY.TOO_LATE).toContain("仍然有效");
	});
});
