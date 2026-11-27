// @vitest-environment jsdom
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	type GetSnapshotResponse,
	InterruptKind,
	OpenConfirmationSchema,
	RequestStatus,
	ResumeRunResponseSchema,
	RunFailureReason,
	RunStatus,
	SlotKey,
	SubmitAnswerResponseSchema,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	acceptedEvent,
	finishedEvent,
	interruptInit,
	snapshotResponse,
	submitTurnResponse,
	turnResultEvent,
} from "./testing/watch-events";
import { useProcurementSession } from "./useProcurementSession";

/**
 * applyTurnResult 從 turn_result 推導 pendingInterrupt、submitTurn 的 FAILED_PRECONDITION 對 reason 敏感、
 * 普通選填問句與分類中斷的 snapshot 還原，以及問題卡答題的傳輸路由：有未回覆的 interrupt 時走
 * ResumeRun（再 WatchRun 取結果），不打 SubmitAnswer。以 in-memory Connect transport 當伺服器。
 */

const { toastErrorMock, rpc } = vi.hoisted(() => ({
	toastErrorMock: vi.fn(),
	rpc: { handlers: {} as Record<string, (request: never) => unknown> },
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		status: "signedIn",
		activeRole: "buyer",
		identity: { accountId: "1001" },
	}),
}));

vi.mock("@/lib/connect/procurement", async () => {
	const { fakeProcurementClient } = await import("./testing/fake-procurement");
	const client = () => fakeProcurementClient(rpc.handlers);
	return {
		buyerProcurementClient: client,
		buyerProcurementStreamClient: client,
	};
});

vi.mock("@/lib/i18n/ui-text", async () => {
	const { uiTextStub } = await import("./testing/fake-procurement");
	return uiTextStub;
});

vi.mock("@/lib/feedback/toast", () => ({
	toast: { error: toastErrorMock, success: vi.fn() },
}));

afterEach(() => {
	rpc.handlers = {};
	toastErrorMock.mockReset();
});

function renderProcurementSession(initialRequestId: string | null = null) {
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	function wrapper({ children }: { children: ReactNode }) {
		return createElement(
			QueryClientProvider,
			{ client: queryClient },
			children,
		);
	}
	return renderHook(() => useProcurementSession(initialRequestId, null), {
		wrapper,
	});
}

const CROP_QUESTION = {
	slotKey: SlotKey.ITEM_DISAMBIGUATION,
	prompt: "找到幾個相符的品項，麻煩選一下你要的是哪一個：",
	allowFreeText: false,
	canSkip: false,
	options: [
		{ optionId: "group-moa-group:f102791d", label: "甘藍（甘藍）" },
		{ optionId: "crop-other", label: "其他（自己輸入）" },
	],
};

const PACKAGING_QUESTION = {
	slotKey: SlotKey.PACKAGING,
	prompt: "這批芭樂需要分裝嗎？",
	options: [{ optionId: "pack-box", label: "分箱包裝" }],
	allowFreeText: true,
	canSkip: true,
};

const INTERRUPT = { interruptId: "int-1", reason: "CLARIFICATION" };

function openConfirmation(
	question: typeof CROP_QUESTION | null = CROP_QUESTION,
	interruptId = "int-1",
) {
	return create(OpenConfirmationSchema, {
		interruptId,
		kind: InterruptKind.CLARIFICATION,
		question: question ?? undefined,
	});
}

/** 依序回傳 snapshot；用完之後一直回最後一個。 */
function snapshots(...responses: Array<GetSnapshotResponse | Error>) {
	const read = vi.fn(() => {
		const next =
			responses[Math.min(read.mock.calls.length - 1, responses.length - 1)];
		if (next instanceof Error) throw next;
		return next;
	});
	rpc.handlers.getSnapshot = read;
	return read;
}

function watchWith(...events: ReturnType<typeof acceptedEvent>[]) {
	return vi.fn(() => events);
}

function reasonError(code: Code, reason: string) {
	return new ConnectError("x", code, undefined, [
		{
			desc: ErrorInfoSchema,
			value: create(ErrorInfoSchema, { domain: "hawkr", reason }),
		},
	]);
}

/** 送出一輪並落在「品項消歧待選」的 interrupt 上。 */
async function arriveAtOpenCropInterrupt() {
	rpc.handlers.submitTurn = () => submitTurnResponse();
	rpc.handlers.watchRun = watchWith(
		acceptedEvent("c1"),
		turnResultEvent("c2", {
			reply: CROP_QUESTION.prompt,
			question: CROP_QUESTION,
			interrupt: interruptInit(),
			missingRequired: [SlotKey.DELIVERY_CITY, SlotKey.UNIT],
		}),
		finishedEvent("c3", RunStatus.SUCCEEDED),
	);
	snapshots(snapshotResponse({ openConfirmation: openConfirmation() }));
	const { result } = renderProcurementSession();
	await act(async () => {
		await result.current.submitTurn("甘藍", "client-req-1");
	});
	await waitFor(() => {
		expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
	});
	return result;
}

describe("applyTurnResult 從 turn_result 推導 pendingInterrupt", () => {
	it("turn_result 帶 interrupt 時，question／turns／pendingInterrupt 一起更新", async () => {
		const result = await arriveAtOpenCropInterrupt();
		expect(result.current.question).toMatchObject({
			slotKey: "crop_disambiguation",
			prompt: CROP_QUESTION.prompt,
		});
		expect(result.current.turns.map((turn) => turn.role)).toEqual([
			"buyer",
			"assistant",
		]);
		expect(result.current.pendingTurn).toBe(false);
	});

	it("turn_result 沒有 interrupt 時 pendingInterrupt 維持 null（正常落地，不誤判成 interrupt）", async () => {
		rpc.handlers.submitTurn = () => submitTurnResponse();
		rpc.handlers.watchRun = watchWith(
			acceptedEvent("c1"),
			turnResultEvent("c2"),
			finishedEvent("c3"),
		);
		snapshots(snapshotResponse());
		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("我要買蘋果", "client-req-1");
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.turns.at(-1)?.role).toBe("assistant");
		expect(result.current.pendingInterrupt).toBeNull();
	});
});

describe("submitTurn 的失敗對 reason 敏感", () => {
	it("FAILED_PRECONDITION＋APPROVAL_PENDING：顯示伺服器訊息對應的失敗，並呼叫 refreshCurrentSnapshot 自我校正", async () => {
		rpc.handlers.submitTurn = () => {
			throw reasonError(Code.FailedPrecondition, "APPROVAL_PENDING");
		};
		const read = snapshots(snapshotResponse());
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		read.mockClear();

		await act(async () => {
			await result.current.submitTurn("數量改成 80", "client-req-1");
		});

		expect(result.current.error?.code).toBe("APPROVAL_PENDING");
		expect(result.current.pendingTurn).toBe(false);
		expect(read).toHaveBeenCalledTimes(1);
	});

	it("其他失敗（例如 client_request_id 內容不同的 INVALID_ARGUMENT）只顯示錯誤，不觸發 refreshCurrentSnapshot", async () => {
		rpc.handlers.submitTurn = () => {
			throw new ConnectError("conflict", Code.InvalidArgument);
		};
		const read = snapshots(snapshotResponse());
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		read.mockClear();

		await act(async () => {
			await result.current.submitTurn("數量改成 80", "client-req-1");
		});

		expect(result.current.error?.code).toBe("INVALID_ARGUMENT");
		expect(read).not.toHaveBeenCalled();
	});
});

describe("WatchRun 的終態", () => {
	it("finished 為 FAILED 且沒有 turn_result：顯示錯誤、收掉進行中狀態，不編造回覆", async () => {
		rpc.handlers.submitTurn = () => submitTurnResponse();
		rpc.handlers.watchRun = watchWith(
			acceptedEvent("c1"),
			finishedEvent("c2", RunStatus.FAILED, RunFailureReason.MODEL_UNAVAILABLE),
		);
		snapshots(snapshotResponse());
		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("我要買蘋果", "client-req-1");
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.error?.code).toBe("MODEL_UNAVAILABLE");
		expect(result.current.turns.map((turn) => turn.role)).toEqual(["buyer"]);
	});

	it("finished 為 SUCCEEDED 卻沒有 turn_result：不套用任何結果，顯示無效回應", async () => {
		rpc.handlers.submitTurn = () => submitTurnResponse();
		rpc.handlers.watchRun = watchWith(acceptedEvent("c1"), finishedEvent("c2"));
		snapshots(snapshotResponse());
		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("我要買蘋果", "client-req-1");
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.error?.code).toBe("INVALID_RESPONSE");
		expect(result.current.turns.map((turn) => turn.role)).toEqual(["buyer"]);
	});

	it("cursor 過期（REPLAY_CURSOR_EXPIRED）：改讀 transcript 與 snapshot 重建畫面", async () => {
		rpc.handlers.submitTurn = () => submitTurnResponse();
		rpc.handlers.watchRun = () => {
			throw reasonError(Code.FailedPrecondition, "REPLAY_CURSOR_EXPIRED");
		};
		const read = snapshots(
			snapshotResponse({
				revisionId: "rev-9",
				openConfirmation: openConfirmation(),
			}),
		);
		rpc.handlers.getConversationMessages = () => ({
			conversationId: "conv-1",
			requestId: "req-1",
			status: RequestStatus.CLARIFYING,
			title: "蘋果",
			messages: [],
		});
		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("我要買蘋果", "client-req-1");
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(read).toHaveBeenCalled();
		expect(result.current.revisionId).toBe("rev-9");
		expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
		expect(result.current.error).toBeNull();
	});
});

describe("普通選填問句的 snapshot 重試", () => {
	const sameRevision = {
		revisionId: "rev-1",
		status: RequestStatus.REQUIREMENT_READY,
		missingRequired: [],
		slots: {
			item: "珍珠芭樂",
			quantity: 80,
			unit: "KG",
			deliveryCity: "桃園市",
		},
	};

	async function arriveAtOptionalQuestion() {
		rpc.handlers.submitTurn = () => submitTurnResponse();
		rpc.handlers.watchRun = watchWith(
			acceptedEvent("c1"),
			turnResultEvent("c2", {
				reply: "已把數量更新為 80 KG。",
				question: PACKAGING_QUESTION,
				missingRequired: [],
			}),
			finishedEvent("c3"),
		);
		const read = snapshots(new Error("snapshot unavailable"));
		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("數量改成 80 KG", "optional-turn");
		});
		expect(result.current.question).toMatchObject({ slotKey: "packaging" });
		expect(result.current.pendingInterrupt).toBeNull();
		return { result, read };
	}

	it("相同 request/revision 的普通選填問題保留，仍可經 SubmitAnswer 作答", async () => {
		const { result } = await arriveAtOptionalQuestion();
		snapshots(snapshotResponse(sameRevision));
		await act(async () => {
			expect(await result.current.refreshCurrentSnapshot()).toBe(true);
		});
		expect(result.current.stage).toBe("ready");
		expect(result.current.question).toMatchObject({ slotKey: "packaging" });
		expect(result.current.pendingInterrupt).toBeNull();

		const submitAnswer = vi.fn((_request: unknown) =>
			create(SubmitAnswerResponseSchema, {
				result: {
					requestId: "req-1",
					conversationId: "conv-1",
					revisionId: "rev-2",
					reply: "已記下分箱包裝。",
				},
			}),
		);
		rpc.handlers.submitAnswer = submitAnswer as never;
		snapshots(snapshotResponse({ ...sameRevision, revisionId: "rev-2" }));
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "packaging",
				optionId: "pack-box",
				optionLabel: "分箱包裝",
			});
		});
		expect(submitAnswer.mock.calls[0][0]).toMatchObject({
			requestId: "req-1",
			slotKey: SlotKey.PACKAGING,
			optionId: "pack-box",
			expectedRevisionId: "rev-1",
		});
		expect(result.current.question).toBeNull();
	});

	it.each([
		["新版 revision", { revisionId: "rev-2" }],
		["已在搜尋候選", { status: RequestStatus.MATCHING }],
		["新中斷未提供問題", { openConfirmation: openConfirmation(null) }],
	])("%s 不沿用普通選填問題", async (_case, overrides) => {
		const { result } = await arriveAtOptionalQuestion();
		snapshots(snapshotResponse({ ...sameRevision, ...overrides }));
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toBeNull();
	});

	it("新權威問題取代普通選填問題", async () => {
		const { result } = await arriveAtOptionalQuestion();
		snapshots(
			snapshotResponse({
				...sameRevision,
				openConfirmation: openConfirmation(),
			}),
		);
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toMatchObject({
			slotKey: "crop_disambiguation",
			prompt: CROP_QUESTION.prompt,
		});
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-1");
	});

	it("已解掉的中斷即使 revision 未變也不保留問題", async () => {
		const result = await arriveAtOpenCropInterrupt();
		snapshots(snapshotResponse({ ...sameRevision }));
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toBeNull();
		expect(result.current.pendingInterrupt).toBeNull();
	});

	it("不同 request 的 snapshot 不套用到目前問題", async () => {
		const { result } = await arriveAtOptionalQuestion();
		snapshots(snapshotResponse({ ...sameRevision, requestId: "req-other" }));
		await act(async () => {
			expect(await result.current.refreshCurrentSnapshot()).toBe(false);
		});
		expect(result.current.requestId).toBe("req-1");
		expect(result.current.question).toMatchObject({ slotKey: "packaging" });
	});
});

describe("分類中斷的權威問句還原", () => {
	const categoryQuestion = {
		...CROP_QUESTION,
		prompt: "請選擇豬肉品項，或補充名稱：",
		allowFreeText: true,
		options: [
			{ optionId: "category-102", label: "豬五花" },
			{ optionId: "crop-other", label: "自行描述品項" },
		],
	};

	it("reload 還原問題並保留 opaque optionId 與 interrupt", async () => {
		snapshots(
			snapshotResponse({
				openConfirmation: openConfirmation(categoryQuestion),
			}),
		);
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		expect(result.current.question).toMatchObject({
			prompt: categoryQuestion.prompt,
			options: categoryQuestion.options,
		});
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-1");
	});

	it("resume 失敗後以最新 snapshot 還原下一層問題及 interrupt", async () => {
		snapshots(
			snapshotResponse({ openConfirmation: openConfirmation() }),
			snapshotResponse({
				openConfirmation: openConfirmation(categoryQuestion, "int-2"),
			}),
		);
		rpc.handlers.resumeRun = () => {
			throw new ConnectError("updated", Code.FailedPrecondition);
		};
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		await act(async () => {
			await result.current.resume({ freeText: "豬五花" });
		});
		expect(result.current.question).toMatchObject({
			prompt: categoryQuestion.prompt,
		});
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-2");
	});
});

describe("問題卡答題的傳輸路由：有未回覆的 interrupt 時走 ResumeRun，不打 SubmitAnswer", () => {
	async function arriveWithInterrupt() {
		snapshots(snapshotResponse({ openConfirmation: openConfirmation() }));
		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual(INTERRUPT),
		);
		return result;
	}

	it("選項送出打 ResumeRun（帶 option_id），完全不碰 SubmitAnswer", async () => {
		const result = await arriveWithInterrupt();
		const resumeRun = vi.fn((_request: unknown) =>
			create(ResumeRunResponseSchema, { runId: "run-2" }),
		);
		const submitAnswer = vi.fn();
		rpc.handlers.resumeRun = resumeRun as never;
		rpc.handlers.submitAnswer = submitAnswer;
		rpc.handlers.watchRun = watchWith(finishedEvent("c1"));
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});
		expect(resumeRun.mock.calls[0][0]).toMatchObject({
			interruptId: "int-1",
			response: {
				case: "clarification",
				value: { optionId: "group-moa-group:f102791d" },
			},
		});
		expect(submitAnswer).not.toHaveBeenCalled();
	});

	it("自由描述只送 free_text；回答 run 的 turn_result 帶回下一層問題與新 interrupt，助理回覆一次落地", async () => {
		const result = await arriveWithInterrupt();
		rpc.handlers.resumeRun = () =>
			create(ResumeRunResponseSchema, { runId: "run-2" });
		rpc.handlers.watchRun = watchWith(
			acceptedEvent("c1"),
			turnResultEvent("c2", {
				reply: "請再選一次豬肉的細項。",
				question: CROP_QUESTION,
				interrupt: interruptInit("int-2"),
			}),
			finishedEvent("c3"),
		);
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				freeText: "豬五花",
			});
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-2");
		expect(result.current.question).toMatchObject({
			slotKey: "crop_disambiguation",
		});
		expect(result.current.turns.at(-1)).toMatchObject({
			role: "assistant",
			text: "請再選一次豬肉的細項。",
		});
	});

	it("ResumeRun 失敗時撤掉樂觀買家泡泡，pendingInterrupt 交給 snapshot 校正後仍在", async () => {
		const result = await arriveWithInterrupt();
		rpc.handlers.resumeRun = () => {
			throw new ConnectError("down", Code.Unavailable);
		};
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "crop-other",
				optionLabel: "其他（自己輸入）",
			});
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.turns).toEqual([]);
		expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
	});

	it("回答 run 以 FAILED（ANSWER_REJECTED）結束：撤掉買家泡泡、顯示錯誤，snapshot 校正後中斷仍待回答", async () => {
		const result = await arriveWithInterrupt();
		rpc.handlers.resumeRun = () =>
			create(ResumeRunResponseSchema, { runId: "run-2" });
		rpc.handlers.watchRun = watchWith(
			acceptedEvent("c1"),
			finishedEvent("c2", RunStatus.FAILED, RunFailureReason.ANSWER_REJECTED),
		);
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				freeText: "不合格式",
			});
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.error?.code).toBe("ANSWER_REJECTED");
		expect(result.current.turns).toEqual([]);
		expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
	});

	it("沒有未回覆的 interrupt 時維持 SubmitAnswer 路徑（一般引導式問句不受影響）", async () => {
		snapshots(snapshotResponse());
		const submitAnswer = vi.fn((_request: unknown) =>
			create(SubmitAnswerResponseSchema, {
				result: {
					requestId: "req-1",
					conversationId: "conv-1",
					revisionId: "rev-2",
					reply: "已記下。",
				},
			}),
		);
		const resumeRun = vi.fn();
		rpc.handlers.submitAnswer = submitAnswer as never;
		rpc.handlers.resumeRun = resumeRun;
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "quantity",
				freeText: "80",
			});
		});
		expect(submitAnswer).toHaveBeenCalledTimes(1);
		expect(resumeRun).not.toHaveBeenCalled();
	});
});

describe("submitAnswer 不樂觀清空 pendingInterrupt", () => {
	it("ResumeRun 失敗且快照校正也失敗時，pendingInterrupt 仍在——否則下一次作答會回頭打 SubmitAnswer 再撞一次 FAILED_PRECONDITION", async () => {
		snapshots(
			snapshotResponse({ openConfirmation: openConfirmation() }),
			new Error("snapshot down"),
		);
		rpc.handlers.resumeRun = () => {
			throw new ConnectError("down", Code.Unavailable);
		};
		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual(INTERRUPT),
		);
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "crop-other",
				optionLabel: "其他（自己輸入）",
			});
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
	});
});
