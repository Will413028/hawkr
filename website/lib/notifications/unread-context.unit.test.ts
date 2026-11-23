// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

interface FakeHandlers {
	onEvent: (name: string, data: unknown, id: string | null) => void;
	onReady: (info: { bufferMiss: boolean }) => void;
	onStatusChange: (status: string) => void;
}

const mocks = vi.hoisted(() => ({
	clients: [] as Array<{ handlers: unknown; closed: boolean }>,
	fetchUnreadCount: vi.fn(async () => 0),
}));

vi.mock("@/lib/sse/client", () => ({
	SseClient: class {
		handlers: unknown;
		closed = false;
		constructor(options: { handlers: unknown }) {
			this.handlers = options.handlers;
			mocks.clients.push(this);
		}
		open() {}
		close() {
			this.closed = true;
		}
	},
}));
vi.mock("@/lib/sse/lastEventIdCache", () => ({
	getLastEventId: () => undefined,
	setLastEventId: () => {},
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
	invalidateNotificationLists: vi.fn(async () => {}),
}));
vi.mock("@/lib/notifications/repo", () => ({
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
function UnreadRenderProbe() {
	setUnreadCount = useNotificationUnread().setCount;
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

function mount() {
	render(
		React.createElement(
			NotificationUnreadProvider,
			null,
			React.createElement(StatusProbe),
		),
	);
	const client = mocks.clients.at(-1);
	if (!client) throw new Error("the provider opened no stream");
	return client.handlers as FakeHandlers;
}

describe("NotificationUnreadProvider 持有通知串流並分派事件", () => {
	beforeEach(() => {
		mocks.clients.length = 0;
		mocks.fetchUnreadCount.mockClear();
	});
	afterEach(() => {
		cleanup();
	});

	it("整個 provider 只開一條通知連線", () => {
		mount();
		expect(mocks.clients).toHaveLength(1);
	});

	it("ready 帶 bufferMiss 時重新讀未讀數", async () => {
		const stream = mount();
		await act(async () => stream.onReady({ bufferMiss: false }));
		const readsBefore = mocks.fetchUnreadCount.mock.calls.length;
		await act(async () => stream.onReady({ bufferMiss: true }));
		expect(mocks.fetchUnreadCount.mock.calls.length).toBeGreaterThan(
			readsBefore,
		);
	});

	it("連線狀態變成 unavailable 時重新讀一次未讀數；狀態 hook 讀得到", async () => {
		const stream = mount();
		act(() => stream.onStatusChange("connected"));
		expect(status).toBe("connected");
		const readsBefore = mocks.fetchUnreadCount.mock.calls.length;
		await act(async () => stream.onStatusChange("unavailable"));
		expect(status).toBe("unavailable");
		const readsAfterFirst = mocks.fetchUnreadCount.mock.calls.length;
		expect(readsAfterFirst).toBeGreaterThan(readsBefore);
		await act(async () => stream.onStatusChange("unavailable"));
		expect(mocks.fetchUnreadCount.mock.calls.length).toBe(readsAfterFirst);
	});
});

describe("連線狀態與未讀數各自的 context", () => {
	afterEach(cleanup);

	it("狀態變化不重繪未讀數的消費端，未讀數變化不重繪狀態的消費端", async () => {
		unreadRenders = 0;
		statusRenders = 0;
		render(
			React.createElement(
				NotificationUnreadProvider,
				null,
				React.createElement(UnreadRenderProbe),
				React.createElement(StatusRenderProbe),
			),
		);
		const client = mocks.clients.at(-1);
		if (!client) throw new Error("the provider opened no stream");
		const stream = client.handlers as FakeHandlers;
		await act(async () => {});

		const unreadBefore = unreadRenders;
		const statusBefore = statusRenders;
		act(() => stream.onStatusChange("connected"));
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
