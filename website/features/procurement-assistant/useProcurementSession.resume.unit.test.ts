// @vitest-environment jsdom
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	type GetSnapshotResponse,
	InterruptKind,
	OpenConfirmationSchema,
	RequestStatus,
	ResumeRunResponseSchema,
	RunStatus,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { snapshotResponse } from "./testing/watch-events";
import {
	type ProcurementResumeOutcome,
	useProcurementSession,
} from "./useProcurementSession";

/**
 * reload 恢復 pending interrupt 與 `resume()`（ResumeRun）。
 *
 * 1. 真的掛載 hook、走 `initialRequestId` → fetchSnapshot 那條 restore effect。
 * 2. `resume()` 這個 closure 本身：guard、全新的 client_request_id、請求形狀、成功路徑清空 pendingInterrupt 並交出 run_id、
 *    失敗路徑改用 `refreshCurrentSnapshot()` 讓伺服器的 snapshot 決定 pendingInterrupt 該清空還是維持。
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

function renderProcurementSession(initialRequestId: string | null) {
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

const snapshotWithOpenConfirmation = () =>
	snapshotResponse({
		openConfirmation: create(OpenConfirmationSchema, {
			interruptId: "int-1",
			kind: InterruptKind.CLARIFICATION,
		}),
	});

const INTERRUPT = { interruptId: "int-1", reason: "CLARIFICATION" };

/** 依序回傳 snapshot；用完之後一直回最後一個。 */
function snapshots(...responses: GetSnapshotResponse[]) {
	const read = vi.fn(() => {
		const index = Math.min(read.mock.calls.length - 1, responses.length - 1);
		return responses[index];
	});
	rpc.handlers.getSnapshot = read;
	return read;
}

describe("reload 恢復 pendingInterrupt（renderHook，走 initialRequestId 深連結那條 restore 路徑）", () => {
	it("mount 時讀 snapshot，回應帶 openConfirmation 就把同一個 pendingInterrupt 寫進 state", async () => {
		const read = snapshots(snapshotWithOpenConfirmation());

		const { result } = renderProcurementSession("req-1");

		await waitFor(() => {
			expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
		});
		expect(result.current.restoring).toBe(false);
		expect(result.current.requestId).toBe("req-1");
		expect(read).toHaveBeenCalledWith(
			expect.objectContaining({ requestId: "req-1" }),
		);
	});

	it("snapshot 沒有 openConfirmation 時 pendingInterrupt 維持 null（沒有卡住的 run 不該無中生有一個 interrupt）", async () => {
		snapshots(snapshotResponse());

		const { result } = renderProcurementSession("req-1");

		await waitFor(() => {
			expect(result.current.restoring).toBe(false);
		});
		expect(result.current.pendingInterrupt).toBeNull();
	});
});

describe("resume()（renderHook）", () => {
	it("沒有 pendingInterrupt 時是 no-op：不打 API、回傳 false", async () => {
		snapshots(snapshotResponse());
		const resumeRun = vi.fn();
		rpc.handlers.resumeRun = resumeRun;
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		expect(result.current.pendingInterrupt).toBeNull();

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed?.resolved).toBe(false);
		expect(resumeRun).not.toHaveBeenCalled();
	});

	it("成功：以 interrupt_id 打 ResumeRun，帶全新的 client_request_id 與回覆內容，成功後清空 pendingInterrupt 並交出 run_id", async () => {
		snapshots(snapshotWithOpenConfirmation());
		const resumeRun = vi.fn((_request: unknown) =>
			create(ResumeRunResponseSchema, {
				runId: "run-2",
				runStatus: RunStatus.RUNNING,
			}),
		);
		rpc.handlers.resumeRun = resumeRun as never;

		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual(INTERRUPT),
		);

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed).toEqual({ resolved: true, runId: "run-2" });
		expect(resumeRun).toHaveBeenCalledTimes(1);
		const request = resumeRun.mock.calls[0][0] as {
			interruptId: string;
			clientRequestId: string;
			response: { case: string; value: { optionId: string } };
		};
		expect(request.interruptId).toBe("int-1");
		expect(request.response.case).toBe("clarification");
		expect(request.response.value.optionId).toBe("opt-a");
		expect(request.clientRequestId.length).toBeGreaterThan(0);
		expect(request.clientRequestId).not.toBe("int-1");

		await waitFor(() => expect(result.current.pendingInterrupt).toBeNull());
	});

	/**
	 * resume 失敗時不能假設 pendingInterrupt 還有效可以重試：FAILED_PRECONDITION 的 APPROVAL_ALREADY_RESOLVED 代表
	 * 這個 interrupt 已被另一個請求解掉，重試永遠不會成功。修法是失敗後一律呼叫 refreshCurrentSnapshot()，
	 * 讓伺服器的 snapshot 決定 pendingInterrupt 該清空還是維持。
	 */
	it("失敗且 interrupt 其實已經被解掉（APPROVAL_ALREADY_RESOLVED）：snapshot 校正出 pendingInterrupt 該清空", async () => {
		const read = snapshots(
			snapshotWithOpenConfirmation(),
			snapshotResponse({
				revisionId: "rev-2",
				status: RequestStatus.REQUIREMENT_READY,
				missingRequired: [],
			}),
		);
		rpc.handlers.resumeRun = () => {
			throw new ConnectError("resolved", Code.FailedPrecondition);
		};

		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual(INTERRUPT),
		);

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed?.resolved).toBe(false);
		expect(toastErrorMock).toHaveBeenCalledTimes(1);
		expect(read).toHaveBeenCalledTimes(2);
		await waitFor(() => expect(result.current.pendingInterrupt).toBeNull());
	});

	it("失敗但 interrupt 其實還沒解掉（例如單純網路錯誤）：snapshot 校正後同一個 pendingInterrupt 維持不變、可以重試", async () => {
		const read = snapshots(snapshotWithOpenConfirmation());
		rpc.handlers.resumeRun = () => {
			throw new ConnectError("down", Code.Unavailable);
		};

		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual(INTERRUPT),
		);

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed?.resolved).toBe(false);
		expect(toastErrorMock).toHaveBeenCalledTimes(1);
		expect(read).toHaveBeenCalledTimes(2);
		expect(result.current.pendingInterrupt).toEqual(INTERRUPT);
	});
});
