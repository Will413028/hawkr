// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentActivityEvent } from "./activity";
import type { RunActivityStreamHandlers } from "./run-events-stream";
import { useRunActivityStream } from "./useRunActivityStream";

const openMock = vi.fn();
const closeMock = vi.fn();

vi.mock("./run-events-stream", () => ({
	openRunActivityStream: (
		runId: string,
		handlers: RunActivityStreamHandlers,
	) => {
		openMock(runId, handlers);
		return { close: closeMock };
	},
}));

/** 最近一次訂閱交出來的 handlers，測試用它模擬伺服器推事件。 */
function latestHandlers(): RunActivityStreamHandlers {
	return openMock.mock.calls.at(-1)?.[1];
}

const parentRow: AgentActivityEvent = {
	rowId: "9001",
	code: "SEARCH_CANDIDATES",
	label: "搜尋候選供應商",
	status: "PENDING",
	text: "",
};

beforeEach(() => {
	openMock.mockClear();
	closeMock.mockClear();
});

describe("useRunActivityStream", () => {
	it("enabled 且有 runId 才開訂閱", () => {
		renderHook(() => useRunActivityStream({ runId: null, enabled: true }));
		expect(openMock).not.toHaveBeenCalled();

		renderHook(() => useRunActivityStream({ runId: "run1", enabled: false }));
		expect(openMock).not.toHaveBeenCalled();

		renderHook(() => useRunActivityStream({ runId: "run1", enabled: true }));
		expect(openMock).toHaveBeenCalledWith("run1", expect.anything());
	});

	it("同一個 rowId 的 PENDING→RUNNING→SUCCEEDED 收斂成一列", () => {
		const { result } = renderHook(() =>
			useRunActivityStream({ runId: "run1", enabled: true }),
		);
		act(() => {
			latestHandlers().onActivity(parentRow);
			latestHandlers().onActivity({ ...parentRow, status: "RUNNING" });
			latestHandlers().onActivity({ ...parentRow, status: "SUCCEEDED" });
		});
		expect(result.current).toHaveLength(1);
		expect(result.current[0].status).toBe("SUCCEEDED");
	});

	it("換 runId 會清掉上一個 run 的列並重開訂閱", () => {
		const { result, rerender } = renderHook(
			({ runId }) => useRunActivityStream({ runId, enabled: true }),
			{ initialProps: { runId: "run1" } },
		);
		act(() => {
			latestHandlers().onActivity(parentRow);
		});
		expect(result.current).toHaveLength(1);

		rerender({ runId: "run2" });
		expect(result.current).toHaveLength(0);
		expect(closeMock).toHaveBeenCalledTimes(1);
		expect(openMock).toHaveBeenLastCalledWith("run2", expect.anything());
	});

	it("enabled 轉 false 時收掉連線（伺服器不會自己關）", () => {
		const { rerender } = renderHook(
			({ enabled }) => useRunActivityStream({ runId: "run1", enabled }),
			{ initialProps: { enabled: true } },
		);
		rerender({ enabled: false });
		expect(closeMock).toHaveBeenCalledTimes(1);
	});

	it("unmount 收掉連線", () => {
		const { unmount } = renderHook(() =>
			useRunActivityStream({ runId: "run1", enabled: true }),
		);
		unmount();
		expect(closeMock).toHaveBeenCalledTimes(1);
	});

	it("收到終態事件時提早收掉連線，不等 enabled／runId 變動", () => {
		renderHook(() => useRunActivityStream({ runId: "run1", enabled: true }));
		act(() => {
			latestHandlers().onTerminal();
		});
		expect(closeMock).toHaveBeenCalledTimes(1);
	});
});
