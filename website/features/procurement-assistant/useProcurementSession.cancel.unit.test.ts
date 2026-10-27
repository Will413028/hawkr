// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import { useProcurementSession } from "./useProcurementSession";

/**
 * 「停止任務」的資料層。
 *
 * 這支測試存在的理由是**取消命令帶不帶 CAS token 沒有任何型別會擋**：
 * `expectedRunVersion` 是一個 string，漏帶、帶空字串、或帶上一次的舊版本，
 * 編譯與 lint 都不會有反應，症狀只會在真流量上變成一個 400 或一次無效的取消。
 * 所以這裡直接斷言送出去的 body。
 *
 * 沿用 `useProcurementSession.interrupt.unit.test.ts` 的 renderHook 慣例：
 * mock `@/lib/api/client`（get／post）、`./turn-stream`、`@/lib/auth/context`。
 */

const { postMock, getMock, streamProcurementTurnMock, toastErrorMock } =
	vi.hoisted(() => ({
		postMock: vi.fn(),
		getMock: vi.fn(),
		streamProcurementTurnMock: vi.fn(),
		toastErrorMock: vi.fn(),
	}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		isLoggedIn: true,
		activeRole: "buyer",
		identity: { accountId: "1001" },
	}),
}));

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			authed: { ...actual.apiClient.authed, post: postMock, get: getMock },
		},
	};
});

vi.mock("./turn-stream", async (importOriginal) => {
	const actual = await importOriginal<typeof import("./turn-stream")>();
	return { ...actual, streamProcurementTurn: streamProcurementTurnMock };
});

vi.mock("@/lib/feedback/toast", () => ({
	toast: { error: toastErrorMock, success: vi.fn() },
}));

afterEach(() => {
	postMock.mockReset();
	getMock.mockReset();
	streamProcurementTurnMock.mockReset();
	toastErrorMock.mockReset();
});

const CONVERSATION_ID = "1234567890123456780";
const REQUEST_ID = "1234567890123456781";
const RUN_ID = "1234567890123456801";

const SNAPSHOT = {
	requestId: REQUEST_ID,
	revisionId: "1234567890123456790",
	status: "CLARIFYING",
	slots: {},
	missingRequired: ["quantity"],
};

const TRANSCRIPT = {
	conversationId: CONVERSATION_ID,
	requestId: REQUEST_ID,
	status: "CLARIFYING",
	title: "蘋果 100 公斤",
	records: [],
};

function runtimeSnapshot(runVersion: string, overrides: object = {}) {
	return {
		conversationId: CONVERSATION_ID,
		requestId: REQUEST_ID,
		status: "CLARIFYING",
		title: "蘋果 100 公斤",
		messages: [],
		activeRunId: RUN_ID,
		runs: [
			{
				runId: RUN_ID,
				runType: "TURN",
				status: "RUNNING",
				lastSeqId: "3",
				minAvailableSeqId: "1",
				terminal: false,
				runVersion,
			},
		],
		artifacts: [],
		...overrides,
	};
}

/** 依路徑分派的 GET：restore 會讀 snapshot 與 transcript，停止鈕讀 runtime-snapshot。 */
function routeGet(runtime: () => unknown) {
	getMock.mockImplementation(async (path: string) => {
		if (path.includes("/runtime-snapshot")) return { data: runtime() };
		if (path.includes("/messages")) return { data: TRANSCRIPT };
		if (path.includes("/snapshot")) return { data: SNAPSHOT };
		throw new Error(`unexpected GET ${path}`);
	});
}

function renderProcurementSession() {
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
	return renderHook(() => useProcurementSession(REQUEST_ID, CONVERSATION_ID), {
		wrapper,
	});
}

/** 送出一輪並讓它停在飛行中：pendingTurn 為 true 的期間才有東西可以停。 */
async function renderWithPendingTurn() {
	streamProcurementTurnMock.mockImplementation(
		() => new Promise(() => undefined),
	);
	const harness = renderProcurementSession();
	await waitFor(() => {
		expect(harness.result.current.conversationId).toBe(CONVERSATION_ID);
	});
	act(() => {
		void harness.result.current.submitTurn("我要買蘋果", "client-req-1");
	});
	await waitFor(() => {
		expect(harness.result.current.pendingTurn).toBe(true);
	});
	return harness;
}

interface CancelCall {
	json: { expectedRunVersion: string; reason: string };
}

function cancelCalls(): Array<[string, CancelCall]> {
	return postMock.mock.calls.filter((call) =>
		String(call[0]).endsWith("/cancel"),
	) as Array<[string, CancelCall]>;
}

describe("停止任務", () => {
	it("執行中會去讀 runtime-snapshot 取得 runId 與 runVersion", async () => {
		routeGet(() => runtimeSnapshot("7"));
		const { result } = await renderWithPendingTurn();

		await waitFor(() => {
			expect(result.current.activeRun).toEqual({
				runId: RUN_ID,
				runVersion: "7",
				runType: "TURN",
				status: "RUNNING",
			});
		});
		expect(
			getMock.mock.calls.some((call) =>
				String(call[0]).includes(
					`/conversations/${CONVERSATION_ID}/runtime-snapshot`,
				),
			),
		).toBe(true);
	});

	it("送出的取消命令必須帶著快照給的 expectedRunVersion", async () => {
		routeGet(() => runtimeSnapshot("7"));
		postMock.mockResolvedValue({
			data: {
				cancelDisposition: "REQUESTED",
				runId: RUN_ID,
				runStatus: "CANCELLING",
				runVersion: "8",
				schemaVersion: "1",
			},
		});
		const { result } = await renderWithPendingTurn();
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		const calls = cancelCalls();
		expect(calls).toHaveLength(1);
		const [path, options] = calls[0];
		expect(path).toBe(`/buyers/me/procurement/runs/${RUN_ID}/cancel`);
		// ⛔ 這一行就是本檔存在的理由：漏掉 version 的實作在這裡才會紅。
		expect(options.json).toEqual({ expectedRunVersion: "7", reason: "" });
		expect(result.current.cancelState).toEqual({
			status: "settled",
			disposition: "REQUESTED",
		});
	});

	it("拿不到 activeRun 時不送出——沒有 token 的請求只會換回 400", async () => {
		// 舊版後端：runs[] 沒有 runVersion 欄位。
		routeGet(() => {
			const wire = runtimeSnapshot("0") as {
				runs: Array<Record<string, unknown>>;
			};
			for (const row of wire.runs) delete row.runVersion;
			return wire;
		});
		const { result } = await renderWithPendingTurn();

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.activeRun).toBeNull();
		expect(cancelCalls()).toHaveLength(0);
		expect(result.current.cancelState).toEqual({ status: "idle" });
	});

	it("CAS 沒過不是取消失敗：重讀快照換新版本再送一次", async () => {
		let version = "7";
		routeGet(() => runtimeSnapshot(version));
		postMock.mockImplementation(
			async (
				_path: string,
				options: {
					json: { expectedRunVersion: string };
				},
			) => {
				if (options.json.expectedRunVersion !== version) {
					throw new ApiClientError("run version conflict", 409);
				}
				return {
					data: {
						cancelDisposition: "REQUESTED",
						runId: RUN_ID,
						runStatus: "CANCELLING",
						runVersion: "12",
						schemaVersion: "1",
					},
				};
			},
		);
		const { result } = await renderWithPendingTurn();
		await waitFor(() => {
			expect(result.current.activeRun?.runVersion).toBe("7");
		});
		// 伺服器端版本在瀏覽器手上那一把之後又前進了一次。
		version = "11";

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		const calls = cancelCalls();
		expect(calls).toHaveLength(2);
		expect(calls[0][1].json.expectedRunVersion).toBe("7");
		expect(calls[1][1].json.expectedRunVersion).toBe("11");
		// 買家看到的是「正在停止」，不是一句「停止失敗」。
		expect(result.current.cancelState).toEqual({
			status: "settled",
			disposition: "REQUESTED",
		});
	});

	it("CAS 沒過且重讀之後拿不到新 token：收斂成 stale，不編一個 disposition", async () => {
		let live = true;
		routeGet(() =>
			live ? runtimeSnapshot("7") : runtimeSnapshot("7", { activeRunId: "" }),
		);
		postMock.mockImplementation(async () => {
			throw new ApiClientError("run version conflict", 409);
		});
		const { result } = await renderWithPendingTurn();
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});
		live = false;

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(cancelCalls()).toHaveLength(1);
		expect(result.current.cancelState).toEqual({ status: "stale" });
		expect(result.current.activeRun).toBeNull();
	});

	it("TOO_LATE 原樣留在狀態裡——它是成功回應，不得被塌成錯誤", async () => {
		routeGet(() => runtimeSnapshot("7"));
		postMock.mockResolvedValue({
			data: {
				cancelDisposition: "TOO_LATE",
				runId: RUN_ID,
				runStatus: "SUCCEEDED",
				runVersion: "9",
				schemaVersion: "1",
			},
		});
		const { result } = await renderWithPendingTurn();
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.cancelState).toEqual({
			status: "settled",
			disposition: "TOO_LATE",
		});
	});

	it("沒登記的 disposition 解析不過，畫面說無法確認而不是猜一個結果", async () => {
		routeGet(() => runtimeSnapshot("7"));
		postMock.mockResolvedValue({
			data: {
				cancelDisposition: "MAYBE_CANCELLED",
				runId: RUN_ID,
				runStatus: "RUNNING",
				runVersion: "8",
			},
		});
		const { result } = await renderWithPendingTurn();
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.cancelState.status).toBe("error");
	});
});
