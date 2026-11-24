// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NotificationSignal } from "@/lib/notifications/watch";

const mocks = vi.hoisted(() => ({
	watches: [] as Array<{
		audience: number;
		onSignal: (s: unknown) => void;
		onStatus: (s: string) => void;
		stop: ReturnType<typeof vi.fn>;
	}>,
	fetchUnreadCount: vi.fn(async () => 0),
	invalidateNotificationLists: vi.fn(async (..._args: unknown[]) => {}),
}));

vi.mock("@/lib/notifications/watch", () => ({
	watchNotifications: (
		audience: number,
		onSignal: (s: unknown) => void,
		onStatus: (s: string) => void,
	) => {
		const handle = { audience, onSignal, onStatus, stop: vi.fn() };
		mocks.watches.push(handle);
		return { stop: handle.stop, resume: vi.fn() };
	},
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({}) }));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ activeRole: "seller", status: "signedIn" }),
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => {
	const identity = { accountId: "acc-1", activeRole: "seller" };
	return { useQueryIdentity: () => identity };
});
vi.mock("@/lib/notifications/cache", () => ({
	invalidateNotificationLists: mocks.invalidateNotificationLists,
}));
vi.mock("@/lib/notifications/inbox", () => ({
	audienceForRole: () => 2,
	fetchUnreadCount: mocks.fetchUnreadCount,
}));

import {
	NotificationUnreadProvider,
	useNotificationStreamStatus,
	useNotificationUnread,
} from "./unread-context";

let unreadRenders = 0;
let statusRenders = 0;
let setUnreadCount: (next: number) => void = () => {};
let unreadCount = 0;
function UnreadRenderProbe() {
	const unread = useNotificationUnread();
	setUnreadCount = unread.setCount;
	unreadCount = unread.count;
	unreadRenders += 1;
	return null;
}
function StatusRenderProbe() {
	useNotificationStreamStatus();
	statusRenders += 1;
	return null;
}

let status = "";
function StatusProbe() {
	status = useNotificationStreamStatus();
	return null;
}

function mount(...children: React.ReactElement[]) {
	render(
		React.createElement(
			NotificationUnreadProvider,
			null,
			React.createElement(StatusProbe),
			...children,
		),
	);
	const watch = mocks.watches.at(-1);
	if (!watch) throw new Error("the provider opened no stream");
	return watch;
}

const inbox = (unread: number): NotificationSignal => ({
	type: "inbox",
	unreadCount: unread,
	hints: [],
});
const reconnected = (afterAbnormalEnd: boolean): NotificationSignal => ({
	type: "reconnected",
	afterAbnormalEnd,
});

describe("NotificationUnreadProvider 持有通知串流並分派訊號", () => {
	beforeEach(() => {
		mocks.watches.length = 0;
		mocks.fetchUnreadCount.mockClear();
		mocks.invalidateNotificationLists.mockClear();
	});
	afterEach(() => {
		cleanup();
	});

	it("整個 provider 只開一條通知連線，受眾跟著工作區", () => {
		const watch = mount();
		expect(mocks.watches).toHaveLength(1);
		expect(watch.audience).toBe(2);
	});

	it("收到收件匣訊號後重拉作用中的列表，並以訊號的未讀數為準", async () => {
		const watch = mount(React.createElement(UnreadRenderProbe));
		await act(async () => {});
		expect(mocks.invalidateNotificationLists).not.toHaveBeenCalled();

		await act(async () => watch.onSignal(inbox(3)));
		expect(mocks.invalidateNotificationLists).toHaveBeenCalledTimes(1);
		expect(mocks.invalidateNotificationLists.mock.calls[0]?.[2]).toBe("active");
		expect(unreadCount).toBe(3);
	});

	it("異常斷線後重連，補讀一次列表（連同第一個 frame 也只拉一次）", async () => {
		const watch = mount();
		await act(async () => {});
		const readsBefore = mocks.fetchUnreadCount.mock.calls.length;

		await act(async () => watch.onSignal(reconnected(true)));
		expect(mocks.invalidateNotificationLists).toHaveBeenCalledTimes(1);
		expect(mocks.fetchUnreadCount.mock.calls.length).toBeGreaterThan(
			readsBefore,
		);

		mocks.invalidateNotificationLists.mockClear();
		await act(async () => {
			watch.onSignal(reconnected(true));
			watch.onSignal(inbox(1));
		});
		expect(mocks.invalidateNotificationLists).toHaveBeenCalledTimes(1);
	});

	it("壽命到期的正常結束再重連，不補讀", async () => {
		const watch = mount();
		await act(async () => {});
		const readsBefore = mocks.fetchUnreadCount.mock.calls.length;

		await act(async () => watch.onSignal(reconnected(false)));
		expect(mocks.invalidateNotificationLists).not.toHaveBeenCalled();
		expect(mocks.fetchUnreadCount.mock.calls.length).toBe(readsBefore);
	});

	it("連線狀態變成 unavailable 時補讀一次；狀態 hook 讀得到", async () => {
		const watch = mount();
		act(() => watch.onStatus("connected"));
		expect(status).toBe("connected");
		const readsBefore = mocks.fetchUnreadCount.mock.calls.length;
		await act(async () => watch.onStatus("unavailable"));
		expect(status).toBe("unavailable");
		const readsAfterFirst = mocks.fetchUnreadCount.mock.calls.length;
		expect(readsAfterFirst).toBeGreaterThan(readsBefore);
		expect(mocks.invalidateNotificationLists).toHaveBeenCalledTimes(1);
		await act(async () => watch.onStatus("unavailable"));
		expect(mocks.fetchUnreadCount.mock.calls.length).toBe(readsAfterFirst);
	});
});

describe("連線狀態與未讀數各自的 context", () => {
	afterEach(cleanup);

	it("狀態變化不重繪未讀數的消費端，未讀數變化不重繪狀態的消費端", async () => {
		unreadRenders = 0;
		statusRenders = 0;
		const watch = mount(
			React.createElement(UnreadRenderProbe),
			React.createElement(StatusRenderProbe),
		);
		await act(async () => {});

		const unreadBefore = unreadRenders;
		const statusBefore = statusRenders;
		act(() => watch.onStatus("connected"));
		expect(statusRenders).toBeGreaterThan(statusBefore);
		expect(unreadRenders).toBe(unreadBefore);

		const statusMid = statusRenders;
		act(() => setUnreadCount(7));
		expect(unreadRenders).toBeGreaterThan(unreadBefore);
		expect(statusRenders).toBe(statusMid);
	});
});

describe("useNotificationStreamStatus 在 provider 之外", () => {
	afterEach(cleanup);

	it("是 idle", () => {
		render(React.createElement(StatusProbe));
		expect(status).toBe("idle");
	});
});
