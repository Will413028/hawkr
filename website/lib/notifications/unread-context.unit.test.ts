// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	NOTIFICATION_STREAM_RESYNC_EVENT,
	SELLER_WORKSPACE_EVENT,
	type SellerWorkspaceEventDetail,
} from "./events";

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
	useAuth: () => ({ activeRole: "seller", isLoggedIn: true }),
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

const removers: Array<() => void> = [];
function collect<T>(name: string): T[] {
	const seen: T[] = [];
	const listener = (event: Event) =>
		seen.push((event as CustomEvent<T>).detail);
	window.addEventListener(name, listener);
	removers.push(() => window.removeEventListener(name, listener));
	return seen;
}

describe("NotificationUnreadProvider 持有唯一一條串流並分派全部事件", () => {
	beforeEach(() => {
		mocks.clients.length = 0;
		mocks.fetchUnreadCount.mockClear();
	});
	afterEach(() => {
		cleanup();
		for (const remove of removers.splice(0)) remove();
	});

	it("整個 provider 只開一條連線", () => {
		mount();
		expect(mocks.clients).toHaveLength(1);
	});

	it("seller.order.updated 與 seller.after_sale.updated 發 window 事件", () => {
		const seen = collect<SellerWorkspaceEventDetail>(SELLER_WORKSPACE_EVENT);
		const stream = mount();
		act(() => {
			stream.onEvent(
				"seller.order.updated",
				{ orderNo: "O1", status: "PAID", source: "payment" },
				"e1",
			);
			stream.onEvent(
				"seller.after_sale.updated",
				{
					orderNo: "O2",
					afterSaleCaseNo: "A1",
					status: "OPEN",
					source: "buyer",
				},
				"e2",
			);
		});
		expect(seen).toEqual([
			{ kind: "order", orderNo: "O1", status: "PAID" },
			{
				kind: "after_sale",
				orderNo: "O2",
				afterSaleCaseNo: "A1",
				status: "OPEN",
			},
		]);
	});

	it("包在 notification.batch 裡的賣家事件也會發", () => {
		const seen = collect<SellerWorkspaceEventDetail>(SELLER_WORKSPACE_EVENT);
		const stream = mount();
		act(() => {
			stream.onEvent(
				"notification.batch",
				{
					events: [
						{
							event: "seller.order.updated",
							data: { orderNo: "O9", status: "PAID" },
							id: "b1",
						},
						{
							event: "seller.after_sale.updated",
							data: { orderNo: "O9", afterSaleCaseNo: "A9" },
							id: "b2",
						},
					],
				},
				null,
			);
		});
		expect(seen.map((detail) => detail.kind)).toEqual(["order", "after_sale"]);
	});

	it("ready 帶 bufferMiss 發一次 resync，並重新讀未讀數", async () => {
		const resyncs = collect<unknown>(NOTIFICATION_STREAM_RESYNC_EVENT);
		const stream = mount();
		const readsBefore = mocks.fetchUnreadCount.mock.calls.length;
		act(() => stream.onReady({ bufferMiss: false }));
		expect(resyncs).toHaveLength(0);
		await act(async () => stream.onReady({ bufferMiss: true }));
		expect(resyncs).toHaveLength(1);
		expect(mocks.fetchUnreadCount.mock.calls.length).toBeGreaterThan(
			readsBefore,
		);
	});

	it("連線狀態變成 unavailable 時發一次 resync；狀態 hook 讀得到", () => {
		const resyncs = collect<unknown>(NOTIFICATION_STREAM_RESYNC_EVENT);
		const stream = mount();
		act(() => stream.onStatusChange("connected"));
		expect(status).toBe("connected");
		expect(resyncs).toHaveLength(0);
		act(() => stream.onStatusChange("unavailable"));
		expect(status).toBe("unavailable");
		expect(resyncs).toHaveLength(1);
		act(() => stream.onStatusChange("unavailable"));
		expect(resyncs).toHaveLength(1);
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
