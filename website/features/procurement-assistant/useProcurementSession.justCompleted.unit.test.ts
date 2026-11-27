// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type { AgentTranscript } from "./conversations";
import {
	acceptedEvent,
	finishedEvent,
	snapshotResponse,
	submitTurnResponse,
	turnResultEvent,
} from "./testing/watch-events";
import type { ProcurementSnapshot } from "./types";
import {
	initialState,
	restoredStateFrom,
	useProcurementSession,
} from "./useProcurementSession";

/**
 * #2549 Phase 3 — justCompletedTurnId。
 *
 * 範圍說明（歷史，2026-08 之前）：`useProcurementSession` 這個 hook 本身在
 * 這個 repo 從未有 render-based 單元測試——`@testing-library/react`／
 * `jsdom`／`react-test-renderer` 都不是本專案的 dependency。`applyTurnResult`
 * 把 `justCompletedTurnId` 設成新 assistant turn id、`submitTurn` 樂觀更新把它
 * 清成 `null`，這兩處寫入點是 hook 內部的 `useCallback` closure，當時無法在
 * 沒有 hook render harness 的情況下獨立驗證，只對 module-level 的純函式
 * `initialState`／`restoredStateFrom` 做直接單元測試。
 *
 * `chore/testing-library-jsdom-setup` 補上 `@testing-library/react`／
 * `jsdom`（per-file `@vitest-environment jsdom`，全域仍是 `node`），下面
 * `applyTurnResult 寫入 justCompletedTurnId` 這個 describe 用 renderHook 真的
 * 掛載這個 hook，把上述已知落差關掉；純函式測試維持不動。
 */
describe("justCompletedTurnId", () => {
	it("initialState() 預設為 null（全新對話，尚未有任何一輪完成）", () => {
		expect(initialState().justCompletedTurnId).toBeNull();
	});

	it("restoredStateFrom 不寫入 justCompletedTurnId，reload path 維持 null（歷史 turn 不得被標成「剛完成」）", () => {
		const transcript: AgentTranscript = {
			conversationId: "conv-1",
			requestId: "req-1",
			status: "CLARIFYING",
			title: "測試對話",
			records: [
				{
					id: "req-1-buyer",
					role: "buyer",
					text: "我要買蘋果",
					createdAt: "2026-08-23T00:00:00Z",
				},
				{
					id: "req-1-assistant",
					role: "assistant",
					text: "好的，請問數量？",
					createdAt: "2026-08-23T00:00:01Z",
				},
			],
		};
		const snapshot: ProcurementSnapshot = {
			requestId: "req-1",
			revisionId: "rev-1",
			status: "CLARIFYING",
			slots: {},
			missingRequired: ["quantity"],
			runs: [],
			activeRunId: "",
			openConfirmation: null,
			publishedPublication: null,
		};

		const restored = restoredStateFrom(transcript, snapshot);

		expect(restored).not.toHaveProperty("justCompletedTurnId");

		const merged = { ...initialState(), ...restored };
		expect(merged.justCompletedTurnId).toBeNull();
	});
});

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
	return renderHook(() => useProcurementSession(null, null), { wrapper });
}

/**
 * `applyTurnResult` 寫入 justCompletedTurnId：用 renderHook 掛載 hook，驗證
 * submitTurn → WatchRun → turn_result → applyTurnResult 這條鏈實際觀察得到的結果。
 */
describe("applyTurnResult 寫入 justCompletedTurnId（renderHook）", () => {
	it("WatchRun 帶回 turn_result：justCompletedTurnId 指到新 assistant turn，pendingTurn 回到 false，即時步驟清空", async () => {
		const submit = vi.fn(() => submitTurnResponse());
		rpc.handlers.submitTurn = submit;
		rpc.handlers.watchRun = () => [
			acceptedEvent("c1"),
			turnResultEvent("c2", {
				reply: "好的，我幫您確認一下數量。",
				steps: [
					{
						code: "UNDERSTAND_INPUT",
						status: 4,
						text: "TERMINAL_ONLY_STEP_SENTINEL",
					},
				],
			}),
			finishedEvent("c3"),
		];
		rpc.handlers.getSnapshot = () => snapshotResponse();

		const { result } = renderProcurementSession();
		expect(result.current.justCompletedTurnId).toBeNull();

		await act(async () => {
			await result.current.submitTurn("我要買蘋果", "client-req-1");
		});

		await waitFor(() => {
			expect(result.current.justCompletedTurnId).toBe("client-req-1-assistant");
		});
		expect(result.current.pendingTurn).toBe(false);
		const assistantTurn = result.current.turns.find(
			(turn) => turn.id === "client-req-1-assistant",
		);
		expect(assistantTurn).toMatchObject({
			role: "assistant",
			text: "好的，我幫您確認一下數量。",
		});
		expect(assistantTurn).not.toHaveProperty("steps");
		expect(result.current.liveSteps).toEqual([]);
		expect(result.current.liveActivity).toEqual([]);
		expect(submit).toHaveBeenCalledTimes(1);
	});
});
