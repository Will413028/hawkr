// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	NOTIFICATION_STREAM_RESYNC_EVENT,
	SELLER_WORKSPACE_EVENT,
} from "@/lib/notifications/events";

const mocks = vi.hoisted(() => ({
	invalidateQueries: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
	useQueryClient: () => ({ invalidateQueries: mocks.invalidateQueries }),
}));
vi.mock("./useQueryIdentity", () => {
	const identity = { accountId: "acc-1", activeRole: "seller" };
	return { useQueryIdentity: () => identity };
});

import {
	SELLER_WORKSPACE_DEBOUNCE_MS,
	useSellerWorkspaceRealtime,
} from "./useSellerWorkspaceRealtime";

function Subscriber() {
	useSellerWorkspaceRealtime();
	return null;
}

const FAMILIES_PER_INVALIDATION = 6;
const workspaceEvent = () =>
	window.dispatchEvent(
		new CustomEvent(SELLER_WORKSPACE_EVENT, {
			detail: { kind: "order", orderNo: "O1" },
		}),
	);
const resync = () =>
	window.dispatchEvent(new CustomEvent(NOTIFICATION_STREAM_RESYNC_EVENT));

describe("useSellerWorkspaceRealtime", () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mocks.invalidateQueries.mockClear();
	});
	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it("連續的賣家事件合併成一次失效（debounce）", () => {
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

	it("resync 立即完整失效，並取消還在等的 debounce", () => {
		render(React.createElement(Subscriber));
		act(() => {
			workspaceEvent();
			resync();
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
			resync();
			vi.advanceTimersByTime(SELLER_WORKSPACE_DEBOUNCE_MS * 2);
		});
		expect(mocks.invalidateQueries).not.toHaveBeenCalled();
	});
});
