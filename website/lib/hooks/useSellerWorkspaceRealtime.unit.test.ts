// @vitest-environment jsdom

import { NotificationAudience } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NotificationSignal } from "@/lib/notifications/watch";
import type { SseConnectionStatus } from "@/lib/sse/client";

const mocks = vi.hoisted(() => ({
	invalidateQueries: vi.fn(),
	watches: [] as Array<{
		audience: number;
		onSignal: (s: unknown) => void;
		onStatus: (s: string) => void;
		stop: ReturnType<typeof vi.fn>;
		resume: ReturnType<typeof vi.fn>;
	}>,
}));

vi.mock("@tanstack/react-query", () => ({
	useQueryClient: () => ({ invalidateQueries: mocks.invalidateQueries }),
}));
vi.mock("./useQueryIdentity", () => {
	const identity = { accountId: "acc-1", activeRole: "seller" };
	return { useQueryIdentity: () => identity };
});
vi.mock("@/lib/notifications/watch", () => ({
	watchNotifications: (
		audience: number,
		onSignal: (s: unknown) => void,
		onStatus: (s: string) => void,
	) => {
		const handle = {
			audience,
			onSignal,
			onStatus,
			stop: vi.fn(),
			resume: vi.fn(),
		};
		mocks.watches.push(handle);
		return handle;
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
const signal = (s: NotificationSignal) => mocks.watches.at(-1)?.onSignal(s);
const workspaceEvent = () =>
	signal({ type: "sellerWorkspace", change: "order" });
const reconnected = () => signal({ type: "reconnected" });

describe("useSellerWorkspaceRealtime", () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mocks.invalidateQueries.mockClear();
		mocks.watches.length = 0;
		lastStatus = "idle";
	});
	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it("開一條 STORE 受眾的串流，連線狀態由回傳值暴露", () => {
		render(React.createElement(Subscriber));
		expect(mocks.watches).toHaveLength(1);
		expect(mocks.watches[0]?.audience).toBe(NotificationAudience.STORE);
		act(() => mocks.watches[0]?.onStatus("connected"));
		expect(lastStatus).toBe("connected");
	});

	it("enabled 為 false 時不開連線", () => {
		render(React.createElement(Subscriber, { enabled: false }));
		expect(mocks.watches).toHaveLength(0);
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

	it("inbox 訊號不讓工作區失效", () => {
		render(React.createElement(Subscriber));
		act(() => {
			signal({ type: "inbox", unreadCount: 1, hints: [] });
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
		});
		expect(mocks.invalidateQueries).not.toHaveBeenCalled();
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
		const stop = mocks.watches[0]?.stop;
		unmount();
		expect(stop).toHaveBeenCalledTimes(1);
		act(() => {
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
			workspaceEvent();
			reconnected();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
		});
		expect(mocks.invalidateQueries).not.toHaveBeenCalled();
	});
});
