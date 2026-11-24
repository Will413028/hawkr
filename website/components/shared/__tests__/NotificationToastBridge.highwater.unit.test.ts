// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({ listInboxItems: vi.fn() }));
const auth = vi.hoisted(() => ({
	value: {
		activeRole: "buyer",
		status: "signedIn",
		identity: { accountId: "1" },
	} as unknown,
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => auth.value }));
vi.mock("@/lib/notifications/inbox", () => ({
	audienceForRole: () => 1,
	listInboxItems: repo.listInboxItems,
}));
vi.mock("@/lib/chat/useChatStream", () => ({
	useChatStream: () => ({ state: "connected", stream: null }),
}));
vi.mock("@/lib/chat/repo", () => ({ listConversations: vi.fn() }));
vi.mock("@/lib/i18n/ui-text", () => ({ useUiText: () => () => "" }));
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
import { NOTIFICATION_SIGNAL_EVENT } from "@/lib/notifications/events";

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

const row = (id: string) => ({
	id,
	title: "t",
	body: "",
	deepLink: "",
	read: false,
	presentation: 1,
});

/** One inbox signal; the signal handler reads the newest page, which holds `ids`. */
async function pushNew(
	ids: string[],
	show: ReturnType<typeof vi.fn>,
): Promise<number> {
	repo.listInboxItems.mockResolvedValueOnce({ items: ids.map(row) });
	const before = show.mock.calls.length;
	await act(async () => {
		window.dispatchEvent(
			new CustomEvent(NOTIFICATION_SIGNAL_EVENT, {
				detail: { unreadCount: 2, hints: [] },
			}),
		);
		await Promise.resolve();
	});
	return show.mock.calls.length - before;
}

const h = React.createElement as unknown as (
	type: unknown,
	props: unknown,
) => React.ReactElement;

describe("NotificationToastBridge priming baseline", () => {
	it("records the baseline when an unrelated dependency changes mid-request", async () => {
		setAuth("1");
		let show = setToast();
		const probe = deferred<{ items: Array<{ id: string }> }>();
		repo.listInboxItems.mockReturnValueOnce(probe.promise);
		const view = render(h(NotificationToastBridge, null));
		expect(repo.listInboxItems).toHaveBeenCalledTimes(1);

		show = setToast();
		view.rerender(h(NotificationToastBridge, null));
		await act(async () => {
			probe.resolve({ items: [{ id: "10" }] });
			await Promise.resolve();
		});

		expect(await pushNew(["10"], show)).toBe(0);
		expect(await pushNew(["11", "10"], show)).toBe(1);
	});

	it("discards the result when the account switches mid-request", async () => {
		setAuth("1");
		let show = setToast();
		const probe = deferred<{ items: Array<{ id: string }> }>();
		repo.listInboxItems
			.mockReturnValueOnce(probe.promise)
			.mockResolvedValue({ items: [] });
		const view = render(h(NotificationToastBridge, null));

		setAuth("2");
		show = setToast();
		view.rerender(h(NotificationToastBridge, null));
		await act(async () => {
			probe.resolve({ items: [{ id: "10" }] });
			await Promise.resolve();
		});

		expect(await pushNew(["10"], show)).toBe(1);
	});
});
