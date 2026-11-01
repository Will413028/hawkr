// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({ fetchNotifications: vi.fn() }));
const auth = vi.hoisted(() => ({
	value: {
		activeRole: "buyer",
		status: "signedIn",
		identity: { accountId: "1" },
	} as unknown,
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => auth.value }));
vi.mock("@/lib/notifications/repo", () => ({
	fetchNotifications: repo.fetchNotifications,
}));
vi.mock("@/lib/chat/ws", () => ({
	getChatSocket: () => ({ connect: vi.fn(), onMessage: () => () => {} }),
}));
vi.mock("@/lib/notifications/unread-context", () => ({
	useNotificationUnread: () => ({
		count: 1,
		loading: false,
		baselineReady: true,
	}),
}));

const toastApi = vi.hoisted(() => ({ current: { show: vi.fn() } }));
vi.mock("@/lib/notifications/toast-context", () => ({
	useNotificationToast: () => toastApi.current,
}));

import NotificationToastBridge from "@/components/shared/NotificationToastBridge";
import { NOTIFICATION_NEW_EVENT } from "@/lib/notifications/events";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((done) => {
		resolve = done;
	});
	return { promise, resolve };
}

function setAuth(accountId: string) {
	auth.value = {
		activeRole: "buyer",
		status: "signedIn",
		identity: { accountId },
	};
}

function setToast(): ReturnType<typeof vi.fn> {
	const show = vi.fn();
	toastApi.current = { show };
	return show;
}

function pushNew(id: string, show: ReturnType<typeof vi.fn>): number {
	const before = show.mock.calls.length;
	act(() => {
		window.dispatchEvent(
			new CustomEvent(NOTIFICATION_NEW_EVENT, {
				detail: {
					item: { id, type: "order", title: "t", isRead: false },
					unreadCount: 2,
				},
			}),
		);
	});
	return show.mock.calls.length - before;
}

const h = React.createElement as unknown as (
	type: unknown,
	props: unknown,
) => React.ReactElement;

describe("NotificationToastBridge priming high-water mark", () => {
	it("records the mark when an unrelated dependency changes mid-request", async () => {
		setAuth("1");
		let show = setToast();
		const probe = deferred<{ records: Array<{ id: string }> }>();
		repo.fetchNotifications.mockReturnValueOnce(probe.promise);
		const view = render(h(NotificationToastBridge, null));
		expect(repo.fetchNotifications).toHaveBeenCalledTimes(1);

		show = setToast();
		view.rerender(h(NotificationToastBridge, null));
		await act(async () => {
			probe.resolve({ records: [{ id: "10" }] });
			await Promise.resolve();
		});

		expect(pushNew("5", show)).toBe(0);
		expect(pushNew("11", show)).toBe(1);
	});

	it("discards the result when the account switches mid-request", async () => {
		setAuth("1");
		let show = setToast();
		const probe = deferred<{ records: Array<{ id: string }> }>();
		repo.fetchNotifications
			.mockReturnValueOnce(probe.promise)
			.mockResolvedValue({ records: [] });
		const view = render(h(NotificationToastBridge, null));

		setAuth("2");
		show = setToast();
		view.rerender(h(NotificationToastBridge, null));
		await act(async () => {
			probe.resolve({ records: [{ id: "10" }] });
			await Promise.resolve();
		});

		expect(pushNew("5", show)).toBe(1);
	});
});
