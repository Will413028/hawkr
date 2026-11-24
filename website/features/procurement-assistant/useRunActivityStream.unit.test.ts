// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import {
	type Activity,
	RunStatus,
	StepStatus,
	type WatchRunResponse,
	WatchRunResponseSchema,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useRunActivityStream } from "./useRunActivityStream";

interface OpenedStream {
	request: { runId: string; cursor: string };
	signal: AbortSignal;
	push: (event: WatchRunResponse) => void;
}

const opened: OpenedStream[] = [];

vi.mock("@/lib/connect/procurement", () => ({
	buyerProcurementStreamClient: () => ({
		watchRun: (
			request: { runId: string; cursor: string },
			{ signal }: { signal: AbortSignal },
		) => {
			const queue: WatchRunResponse[] = [];
			let wake: (() => void) | null = null;
			signal.addEventListener("abort", () => wake?.());
			opened.push({
				request,
				signal,
				push: (event) => {
					queue.push(event);
					wake?.();
				},
			});
			return (async function* () {
				while (!signal.aborted) {
					const next = queue.shift();
					if (next) {
						yield next;
						continue;
					}
					await new Promise<void>((resolve) => {
						wake = resolve;
					});
					wake = null;
				}
			})();
		},
	}),
}));

function activityEvent(
	cursor: string,
	activity: Partial<Activity>,
): WatchRunResponse {
	return create(WatchRunResponseSchema, {
		cursor,
		event: {
			case: "activity",
			value: {
				rowId: "9001",
				code: "SEARCH_CANDIDATES",
				label: "搜尋候選供應商",
				status: StepStatus.PENDING,
				...activity,
			},
		},
	});
}

beforeEach(() => {
	opened.length = 0;
});

describe("useRunActivityStream", () => {
	it("enabled 且有 runId 才開訂閱", () => {
		renderHook(() => useRunActivityStream({ runId: null, enabled: true }));
		renderHook(() => useRunActivityStream({ runId: "run1", enabled: false }));
		expect(opened).toHaveLength(0);

		renderHook(() => useRunActivityStream({ runId: "run1", enabled: true }));
		expect(opened).toHaveLength(1);
		expect(opened[0].request).toEqual({ runId: "run1", cursor: "" });
	});

	it("同一個 rowId 的 PENDING→RUNNING→SUCCEEDED 收斂成一列", async () => {
		const { result } = renderHook(() =>
			useRunActivityStream({ runId: "run1", enabled: true }),
		);
		act(() => {
			opened[0].push(activityEvent("c1", {}));
			opened[0].push(activityEvent("c2", { status: StepStatus.RUNNING }));
			opened[0].push(activityEvent("c3", { status: StepStatus.SUCCEEDED }));
		});
		await waitFor(() => expect(result.current[0]?.status).toBe("SUCCEEDED"));
		expect(result.current).toHaveLength(1);
	});

	it("伺服器重播同一個 cursor 的事件不重複處理", async () => {
		const { result } = renderHook(() =>
			useRunActivityStream({ runId: "run1", enabled: true }),
		);
		act(() => {
			opened[0].push(activityEvent("c1", { status: StepStatus.SUCCEEDED }));
			opened[0].push(activityEvent("c1", { status: StepStatus.RUNNING }));
			opened[0].push(activityEvent("c2", { rowId: "9002" }));
		});
		await waitFor(() => expect(result.current).toHaveLength(2));
		expect(result.current[0].status).toBe("SUCCEEDED");
	});

	it("換 runId 會清掉上一個 run 的列、中止舊連線並重開訂閱", async () => {
		const { result, rerender } = renderHook(
			({ runId }) => useRunActivityStream({ runId, enabled: true }),
			{ initialProps: { runId: "run1" } },
		);
		act(() => {
			opened[0].push(activityEvent("c1", {}));
		});
		await waitFor(() => expect(result.current).toHaveLength(1));

		rerender({ runId: "run2" });
		expect(result.current).toHaveLength(0);
		expect(opened[0].signal.aborted).toBe(true);
		expect(opened[1].request.runId).toBe("run2");
	});

	it("enabled 轉 false 時中止連線", () => {
		const { rerender } = renderHook(
			({ enabled }) => useRunActivityStream({ runId: "run1", enabled }),
			{ initialProps: { enabled: true } },
		);
		rerender({ enabled: false });
		expect(opened[0].signal.aborted).toBe(true);
	});

	it("unmount 中止連線", () => {
		const { unmount } = renderHook(() =>
			useRunActivityStream({ runId: "run1", enabled: true }),
		);
		unmount();
		expect(opened[0].signal.aborted).toBe(true);
	});

	it("收到 finished 事件後不再重開訂閱，已收到的列保留", async () => {
		const { result } = renderHook(() =>
			useRunActivityStream({ runId: "run1", enabled: true }),
		);
		act(() => {
			opened[0].push(activityEvent("c1", { status: StepStatus.SUCCEEDED }));
			opened[0].push(
				create(WatchRunResponseSchema, {
					cursor: "c2",
					event: {
						case: "finished",
						value: { status: RunStatus.SUCCEEDED },
					},
				}),
			);
		});
		await waitFor(() => expect(result.current).toHaveLength(1));
		await waitFor(() => expect(opened[0].signal.aborted).toBe(true));
		expect(opened).toHaveLength(1);
	});
});
