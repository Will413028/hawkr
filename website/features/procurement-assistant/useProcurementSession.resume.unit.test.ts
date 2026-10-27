// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import {
	type ProcurementResumeOutcome,
	useProcurementSession,
} from "./useProcurementSession";

/**
 * Task 17 — reload 恢復 pending interrupt ＋ resume() 呼叫。
 *
 * `useProcurementSession.unit.test.ts` 的 `pendingInterrupt` describe 已經對
 * `restoredStateFrom`（純函式）做直接單元測試；這個檔案補上兩段那邊測不到
 * 的東西，比照 `useProcurementSession.justCompleted.unit.test.ts` 的
 * renderHook 慣例：
 *
 * 1. 真的掛載 hook、走 `initialRequestId` → fetchSnapshot 那條 restore
 *    effect（`useProcurementSession.unit.test.ts` 只測了 `restoredStateFrom`
 *    這個純函式本身，沒驗證 effect 真的把它的回傳值 merge 進 state）。
 * 2. `resume()` 這個 useCallback closure 本身——guard、fresh clientRequestId、
 *    呼叫的 URL／body 形狀、成功路徑清空 pendingInterrupt、失敗路徑改用
 *    `refreshCurrentSnapshot()` 讓後端 snapshot 校正 pendingInterrupt 該
 *    清空還是維持（不是本地假設失敗＝還可以重試）。
 */

const { postMock, getMock, toastErrorMock } = vi.hoisted(() => ({
	postMock: vi.fn(),
	getMock: vi.fn(),
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

// 不 mock sonner 本身，避免真的排隊一個 `whenDocumentSettled` 之後的動態
// import——那條路跟這裡要驗證的行為無關，只會留下測試間的噪音。
vi.mock("@/lib/feedback/toast", () => ({
	toast: { error: toastErrorMock, success: vi.fn() },
}));

afterEach(() => {
	postMock.mockReset();
	getMock.mockReset();
	toastErrorMock.mockReset();
});

function renderProcurementSession(initialRequestId: string | null) {
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	// `.unit.test.ts`（vitest.config.ts 的 include 只認這個副檔名）不能用
	// JSX，跟 justCompleted 那個檔案一樣改用 createElement。
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

const SNAPSHOT_WITH_OPEN_CONFIRMATION = {
	requestId: "req-1",
	revisionId: "rev-1",
	status: "CLARIFYING",
	slots: {},
	missingRequired: ["quantity"],
	openConfirmation: {
		interruptId: "int-1",
		confirmationKind: "CLARIFICATION",
		expiresAt: "2026-09-01T00:00:00Z",
	},
};

describe("reload 恢復 pendingInterrupt（renderHook，走 initialRequestId 深連結那條 restore 路徑）", () => {
	it("mount 時打 GET snapshot，回應帶 openConfirmation 就把同一個 pendingInterrupt 寫進 state", async () => {
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

		const { result } = renderProcurementSession("req-1");

		await waitFor(() => {
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			});
		});
		expect(result.current.restoring).toBe(false);
		expect(result.current.requestId).toBe("req-1");
		expect(getMock).toHaveBeenCalledWith(
			"/buyers/me/procurement/requests/req-1/snapshot",
		);
	});

	it("snapshot 沒有 openConfirmation 時 pendingInterrupt 維持 null（沒有卡住的 run 不該無中生有一個 interrupt）", async () => {
		getMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				status: "CLARIFYING",
				slots: {},
				missingRequired: ["quantity"],
			},
		});

		const { result } = renderProcurementSession("req-1");

		await waitFor(() => {
			expect(result.current.restoring).toBe(false);
		});
		expect(result.current.pendingInterrupt).toBeNull();
	});
});

describe("resume()（renderHook）", () => {
	it("沒有 pendingInterrupt 時是 no-op：不打 API、回傳 false", async () => {
		getMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				status: "CLARIFYING",
				slots: {},
				missingRequired: ["quantity"],
			},
		});
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		expect(result.current.pendingInterrupt).toBeNull();

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ candidateId: "c-1" });
		});

		expect(resumed?.resolved).toBe(false);
		expect(postMock).not.toHaveBeenCalled();
	});

	it("成功：打 POST .../runs/{interruptId}/resume（interruptId 當 path 佔位），body 帶全新 clientRequestId＋interruptId＋payload，成功後清空 pendingInterrupt", async () => {
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
		postMock.mockResolvedValueOnce({
			data: { runId: "run-1", runStatus: "RUNNING" },
		});

		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			}),
		);

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed?.resolved).toBe(true);
		expect(postMock).toHaveBeenCalledTimes(1);
		const [path, options] = postMock.mock.calls[0];
		expect(path).toBe("/buyers/me/procurement/runs/int-1/resume");
		expect(options.json).toMatchObject({
			interruptId: "int-1",
			payload: { optionId: "opt-a" },
		});
		expect(typeof options.json.clientRequestId).toBe("string");
		expect(options.json.clientRequestId.length).toBeGreaterThan(0);
		// resume 自己的 clientRequestId 必須是全新一個，不能沿用觸發這個
		// interrupt 那一輪的（這裡沒有任何「原始 clientRequestId」流進
		// restoredStateFrom／state，所以無從沿用——這行順便斷言了這件事：
		// 這支呼叫的識別碼完全獨立於 interruptId 本身）。
		expect(options.json.clientRequestId).not.toBe("int-1");

		await waitFor(() => expect(result.current.pendingInterrupt).toBeNull());
	});

	/**
	 * resume 失敗時不能假設 pendingInterrupt 還有效可以重試——resume 端點
	 * 唯一有文件的 409 是 APPROVAL_ALREADY_RESOLVED（這個 interrupt 已經被
	 * 另一個請求解掉了），這種失敗重試永遠不會成功。修法不是對這個錯誤
	 * 原因字串特判，而是失敗後一律呼叫既有的 refreshCurrentSnapshot()，讓
	 * 後端 snapshot（權威來源）決定 pendingInterrupt 該清空還是維持——下面
	 * 兩個測試分別驗證這兩種後端裁決結果都被正確套用。
	 */
	it("失敗且 interrupt 其實已經被解掉（409 APPROVAL_ALREADY_RESOLVED）：refreshCurrentSnapshot 從最新 snapshot 校正出 pendingInterrupt 該清空，不留一個後端已不認得的死結", async () => {
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
		postMock.mockRejectedValueOnce(
			new ApiClientError(
				"resume failed",
				409,
				undefined,
				undefined,
				"APPROVAL_ALREADY_RESOLVED",
			),
		);

		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			}),
		);

		// refreshCurrentSnapshot 校正用的第二次 GET：最新 snapshot 已經沒有
		// openConfirmation——這個 interrupt 已經被別的請求解掉了。
		getMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-2",
				status: "REQUIREMENT_READY",
				slots: {},
				missingRequired: [],
			},
		});

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed?.resolved).toBe(false);
		expect(toastErrorMock).toHaveBeenCalledTimes(1);
		expect(getMock).toHaveBeenCalledTimes(2);
		await waitFor(() => expect(result.current.pendingInterrupt).toBeNull());
	});

	it("失敗但 interrupt 其實還沒解掉（例如單純網路錯誤）：refreshCurrentSnapshot 從最新 snapshot 校正出同一個 pendingInterrupt 仍然有效，維持不變、可以重試", async () => {
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
		postMock.mockRejectedValueOnce(new Error("network down"));

		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			}),
		);

		// refreshCurrentSnapshot 校正用的第二次 GET：同一個 openConfirmation
		// 還在——這個 interrupt 沒有被解掉，只是這次 resume 呼叫本身網路失敗。
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

		let resumed: ProcurementResumeOutcome | undefined;
		await act(async () => {
			resumed = await result.current.resume({ optionId: "opt-a" });
		});

		expect(resumed?.resolved).toBe(false);
		expect(toastErrorMock).toHaveBeenCalledTimes(1);
		expect(getMock).toHaveBeenCalledTimes(2);
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});
});
