// @vitest-environment jsdom
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	CancelDisposition,
	CancelRunResponseSchema,
	GetRuntimeSnapshotResponseSchema,
	RequestStatus,
	RunStatus,
	RunType,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	CONVERSATION_ID,
	REQUEST_ID,
	RUN_ID,
	snapshotResponse,
	submitTurnResponse,
	transcriptResponse,
} from "./testing/watch-events";
import { useProcurementSession } from "./useProcurementSession";

/**
 * 「停止任務」的資料層：取消命令只帶 run_id，回 OK 不等於「已取消」，`cancel_disposition` 原樣帶到畫面層。
 * 沿用 `useProcurementSession.interrupt.unit.test.ts` 的 renderHook 慣例，以 in-memory Connect transport 當伺服器。
 */

const rpc = vi.hoisted(() => ({
	handlers: {} as Record<string, (request: never) => unknown>,
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
	toast: { error: vi.fn(), success: vi.fn() },
}));

afterEach(() => {
	rpc.handlers = {};
});

function runtimeSnapshot(live = true) {
	return create(GetRuntimeSnapshotResponseSchema, {
		conversationId: CONVERSATION_ID,
		requestId: REQUEST_ID,
		status: RequestStatus.CLARIFYING,
		title: "蘋果 100 公斤",
		activeRunId: live ? RUN_ID : "",
		runs: [
			{
				runId: RUN_ID,
				runType: RunType.TURN,
				status: RunStatus.RUNNING,
				lastCursor: "c3",
				terminal: !live,
			},
		],
	});
}

function cancelResponse(
	disposition: CancelDisposition,
	status: RunStatus = RunStatus.RUNNING,
) {
	return create(CancelRunResponseSchema, {
		runId: RUN_ID,
		runStatus: status,
		cancelDisposition: disposition,
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

/** 送出一輪並讓 WatchRun 停在飛行中：pendingTurn 為 true 的期間才有東西可以停。 */
async function renderWithPendingTurn(live = true) {
	rpc.handlers.getSnapshot = () => snapshotResponse();
	rpc.handlers.getConversationMessages = () => transcriptResponse();
	rpc.handlers.getRuntimeSnapshot = () => runtimeSnapshot(live);
	rpc.handlers.submitTurn = () => submitTurnResponse();
	rpc.handlers.watchRun = () =>
		(async function* () {
			await new Promise<never>(() => undefined);
		})();
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

describe("停止任務", () => {
	it("執行中會去讀 runtime snapshot 取得可停止的 run", async () => {
		const { result } = await renderWithPendingTurn();

		await waitFor(() => {
			expect(result.current.activeRun).toEqual({ runId: RUN_ID });
		});
	});

	it("送出的取消命令只帶 run_id，回應的處置原樣留在狀態裡", async () => {
		const cancel = vi.fn((_request: { runId: string }) =>
			cancelResponse(CancelDisposition.REQUESTED, RunStatus.CANCEL_REQUESTED),
		);
		const { result } = await renderWithPendingTurn();
		rpc.handlers.cancelRun = cancel as never;
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(cancel).toHaveBeenCalledTimes(1);
		expect(cancel.mock.calls[0][0]).toMatchObject({ runId: RUN_ID });
		expect(result.current.cancelState).toEqual({
			status: "settled",
			disposition: "REQUESTED",
		});
	});

	it("拿不到可停止的 run 時不送出", async () => {
		const cancel = vi.fn();
		const { result } = await renderWithPendingTurn(false);
		rpc.handlers.cancelRun = cancel;

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.activeRun).toBeNull();
		expect(cancel).not.toHaveBeenCalled();
		expect(result.current.cancelState).toEqual({ status: "idle" });
	});

	it.each([
		[CancelDisposition.ALREADY_CANCELLED, "ALREADY_CANCELLED"],
		[CancelDisposition.ALREADY_TERMINAL, "ALREADY_TERMINAL"],
		[CancelDisposition.TOO_LATE, "TOO_LATE"],
	])("處置 %s 是成功回應，原樣留在狀態裡，不得被塌成錯誤", async (disposition, expected) => {
		const { result } = await renderWithPendingTurn();
		rpc.handlers.cancelRun = () => cancelResponse(disposition);
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.cancelState).toEqual({
			status: "settled",
			disposition: expected,
		});
	});

	it("沒有可辨識的處置，畫面說無法確認而不是猜一個結果", async () => {
		const { result } = await renderWithPendingTurn();
		rpc.handlers.cancelRun = () => cancelResponse(CancelDisposition.UNSPECIFIED);
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.cancelState.status).toBe("error");
	});

	it("取消呼叫失敗時收斂成 error，帶伺服器或預設的錯誤文字", async () => {
		const { result } = await renderWithPendingTurn();
		rpc.handlers.cancelRun = () => {
			throw new ConnectError("down", Code.Unavailable);
		};
		await waitFor(() => {
			expect(result.current.activeRun).not.toBeNull();
		});

		await act(async () => {
			await result.current.cancelActiveRun();
		});

		expect(result.current.cancelState).toEqual({
			status: "error",
			detail: `default-${Code.Unavailable}`,
		});
	});
});
