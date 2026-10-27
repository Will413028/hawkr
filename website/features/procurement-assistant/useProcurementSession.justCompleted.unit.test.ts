// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type { AgentTranscript } from "./conversations";
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
		const snapshot = {
			requestId: "req-1",
			revisionId: "rev-1",
			status: "CLARIFYING" as const,
			slots: {},
			missingRequired: ["quantity"],
		};

		const restored = restoredStateFrom(transcript, snapshot);

		// 回傳的 Partial<SessionState> 完全不該提到這個欄位——一旦有人不小心
		// 把最後一則 assistant turn 標成剛完成，merge 進 initialState() 就會讓
		// 已讀取的歷史對話也播交錯淡入動畫。
		expect(restored).not.toHaveProperty("justCompletedTurnId");

		// 對齊掛載時實際的 merge 方式（useState 初始化那行：
		// `{ ...initialState(), ...cached }`）。
		const merged = { ...initialState(), ...restored };
		expect(merged.justCompletedTurnId).toBeNull();
	});
});

// `vi.mock` 的 factory 會被吊起到檔案最上面，早於一般的 `const` 賦值——
// 這幾個 mock function 要用 `vi.hoisted` 宣告，factory 內才存取得到。
const { postMock, getMock, streamProcurementTurnMock } = vi.hoisted(() => ({
	postMock: vi.fn(),
	getMock: vi.fn(),
	streamProcurementTurnMock: vi.fn(),
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

// `streamProcurementTurn` 是 `runTurnStream` 唯一的外部 I/O；本專案目前沒有
// 任何既有測試消費過 `turn-stream.ts` 的 export（`turn-stream.unit.test.ts`
// 測的是它自己內部行為，不是被別的模組 mock 掉），所以這裡是這個 repo 第一個
// 「mock `./turn-stream` 給 `useProcurementSession` 用」的用例，沒有既有慣例
// 可以照抄，寫法對齊上面 `@/lib/api/client` 的 `importOriginal` 局部覆寫模式。
vi.mock("./turn-stream", async (importOriginal) => {
	const actual = await importOriginal<typeof import("./turn-stream")>();
	return { ...actual, streamProcurementTurn: streamProcurementTurnMock };
});

function renderProcurementSession() {
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	// 這個檔案是 `.unit.test.ts`（vitest.config.ts 的 include 只認這個副檔
	// 名），不能用 JSX；改用 createElement，對齊
	// `route-capability-runtime.unit.test.ts` 已有的相同限制與寫法。
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
 * `applyTurnResult` 寫入 justCompletedTurnId — 用 renderHook 真的掛載這個
 * hook，驗證 submitTurn → runTurnStream → applyTurnResult 這條 closure 鏈
 * 實際觀察得到的結果，取代上面純函式測試的 `已知落差` 註記。
 *
 * `getMock`（`/requests/{id}/snapshot`）刻意 resolve 成功但不斷言它——
 * `applyTurnResult` 落地後打的那次是 best-effort 的 stage 校正，跟這裡驗證
 * 的欄位無關；讓它安靜成功比較不會在測試間留下未處理的 rejection 噪音。
 */
describe("applyTurnResult 寫入 justCompletedTurnId（renderHook）", () => {
	it("串流成功落地：justCompletedTurnId 指到新 assistant turn，pendingTurn 回到 false，且沒有退化打同步 /turns 端點", async () => {
		streamProcurementTurnMock.mockResolvedValueOnce({
			requestId: "req-1",
			revisionId: "rev-1",
			reply: "好的，我幫您確認一下數量。",
			degraded: false,
			slots: {},
			missingRequired: ["quantity"],
			rejected: [],
			// 終態仍可能帶 steps；hook 必須剝除，不能把思考過程寫回 turn。
			steps: [
				{
					code: "UNDERSTAND_INPUT",
					status: "SUCCEEDED",
					text: "TERMINAL_ONLY_STEP_SENTINEL",
				},
			],
		});
		getMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				status: "CLARIFYING",
				slots: {},
				missingRequired: ["quantity"],
			},
		});

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
		// 串流路徑成功落地：不該退化打同步 POST /buyers/me/procurement/turns。
		expect(postMock).not.toHaveBeenCalled();
	});
});
