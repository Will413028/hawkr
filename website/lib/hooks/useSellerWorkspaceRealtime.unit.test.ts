// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	SELLER_WORKSPACE_SIGNAL_EVENT,
	type SellerWorkspaceSignalEventDetail,
} from "@/lib/notifications/events";
import type { SseConnectionStatus } from "@/lib/sse/client";

const mocks = vi.hoisted(() => ({
	invalidateQueries: vi.fn(),
	status: { value: "idle" as string },
}));

vi.mock("@tanstack/react-query", () => ({
	useQueryClient: () => ({ invalidateQueries: mocks.invalidateQueries }),
}));
vi.mock("./useQueryIdentity", () => {
	const identity = { accountId: "acc-1", activeRole: "seller" };
	return { useQueryIdentity: () => identity };
});
vi.mock("@/lib/notifications/unread-context", () => ({
	useNotificationStreamStatus: () => mocks.status.value,
}));
vi.mock("@/lib/notifications/watch", () => ({
	watchNotifications: () => {
		throw new Error("the hook must not open its own stream");
	},
}));

import {
	SELLER_WORKSPACE_DEBOUNCE_MS,
	useSellerWorkspaceRealtime,
} from "./useSellerWorkspaceRealtime";

let lastStatus: SseConnectionStatus = "idle";
function Subscriber({ enabled = true }: { enabled?: boolean }) {
	lastStatus = useSellerWorkspaceRealtime(enabled);
	return null;
}

const FAMILIES_PER_INVALIDATION = 6;
const emit = (kind: SellerWorkspaceSignalEventDetail["kind"]) =>
	window.dispatchEvent(
		new CustomEvent(SELLER_WORKSPACE_SIGNAL_EVENT, { detail: { kind } }),
	);
const workspaceEvent = () => emit("change");
const reconnected = () => emit("reconnected");

describe("useSellerWorkspaceRealtime", () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mocks.invalidateQueries.mockClear();
		mocks.status.value = "idle";
		lastStatus = "idle";
	});
	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it("不自己開串流，連線狀態取自 provider 的單一串流", () => {
		mocks.status.value = "connected";
		render(React.createElement(Subscriber));
		expect(lastStatus).toBe("connected");
	});

	it("enabled 為 false 時不回應事件，狀態為 idle", () => {
		mocks.status.value = "connected";
		render(React.createElement(Subscriber, { enabled: false }));
		act(() => {
			workspaceEvent();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
		});
		expect(mocks.invalidateQueries).not.toHaveBeenCalled();
		expect(lastStatus).toBe("idle");
	});

	it("連續的賣家工作區訊號合併成一次失效（debounce）", () => {
		render(React.createElement(Subscriber));
		act(() => {
			workspaceEvent();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS - 1);
			workspaceEvent();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS - 1);
			workspaceEvent();
		});
		expect(mocks.invalidateQueries).not.toHaveBeenCalled();
		act(() => {
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS);
		});
		expect(mocks.invalidateQueries).toHaveBeenCalledTimes(
			FAMILIES_PER_INVALIDATION,
		);
	});

	it("失效的是工作區的整組 family，不是單一 query", () => {
		render(React.createElement(Subscriber));
		act(() => {
			workspaceEvent();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS);
		});
		const keys = mocks.invalidateQueries.mock.calls.map(
			([filter]) => (filter as { queryKey: unknown[] }).queryKey,
		);
		const names = keys.map((key) => key[3]);
		expect(names).toEqual(
			expect.arrayContaining([
				"orders",
				"dashboard",
				"order-counts",
				"after-sales",
			]),
		);
		for (const key of keys) {
			expect(key.slice(0, 3)).toEqual(["seller", "acc-1", "seller"]);
		}
	});

	it("重連後立即完整失效，並取消還在等的 debounce", () => {
		render(React.createElement(Subscriber));
		act(() => {
			workspaceEvent();
			reconnected();
		});
		expect(mocks.invalidateQueries).toHaveBeenCalledTimes(
			FAMILIES_PER_INVALIDATION,
		);
		act(() => {
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
		});
		expect(mocks.invalidateQueries).toHaveBeenCalledTimes(
			FAMILIES_PER_INVALIDATION,
		);
	});

	it("卸載後不再回應事件，也不會留下 timer", () => {
		const { unmount } = render(React.createElement(Subscriber));
		act(() => workspaceEvent());
		unmount();
		act(() => {
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
			workspaceEvent();
			reconnected();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
		});
		expect(mocks.invalidateQueries).not.toHaveBeenCalled();
	});
});
