// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({ listInboxItems: vi.fn() }));
const toast = vi.hoisted(() => ({ show: vi.fn() }));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		activeRole: "buyer",
		status: "signedIn",
		identity: { accountId: "1" },
	}),
}));
vi.mock("@/lib/notifications/inbox", () => ({
	audienceForRole: () => 1,
	listInboxItems: repo.listInboxItems,
}));
vi.mock("@/lib/chat/useChatStream", () => ({
	useChatStream: () => ({ state: "connected", stream: null }),
}));
vi.mock("@/lib/chat/repo", () => ({ listConversations: vi.fn() }));
vi.mock("@/lib/i18n/ui-text", () => ({ useUiText: () => () => "" }));
vi.mock("@/lib/notifications/toast-context", () => ({
	useNotificationToast: () => toast,
}));

import NotificationToastBridge from "@/components/shared/NotificationToastBridge";
import {
	NOTIFICATION_SIGNAL_EVENT,
	type NotificationSignalEventDetail,
} from "@/lib/notifications/events";

const row = (id: string, extra: Record<string, unknown> = {}) => ({
	id,
	title: `title-${id}`,
	body: `body-${id}`,
	deepLink: `/account/orders/${id}`,
	read: false,
	presentation: 1,
	...extra,
});

function setVisibility(state: "visible" | "hidden") {
	Object.defineProperty(document, "visibilityState", {
		configurable: true,
		get: () => state,
	});
}

async function signal(detail: NotificationSignalEventDetail) {
	await act(async () => {
		window.dispatchEvent(
			new CustomEvent(NOTIFICATION_SIGNAL_EVENT, { detail }),
		);
	});
}

beforeEach(() => {
	setVisibility("visible");
	repo.listInboxItems.mockResolvedValue({
		items: [row("12"), row("11"), row("10")],
	});
});
afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

describe("NotificationToastBridge inbox signal", () => {
	it("toasts exactly the ids the signal names, with the server-composed content", async () => {
		render(React.createElement(NotificationToastBridge));
		await signal({ unreadCount: 3, hints: [], newInboxItemIds: ["12", "10"] });
		expect(repo.listInboxItems).toHaveBeenCalledTimes(1);
		const shown = toast.show.mock.calls.map((c) => c[0]);
		expect(shown.map((t) => t.dedupeKey).sort()).toEqual([
			"notif:10",
			"notif:12",
		]);
		expect(shown.find((t) => t.dedupeKey === "notif:12")).toMatchObject({
			title: "title-12",
			body: "body-12",
			href: "/account/orders/12",
		});
	});

	it("shows an id that repeats across signals under the same dedupeKey so the toast is replaced", async () => {
		render(React.createElement(NotificationToastBridge));
		await signal({ unreadCount: 1, hints: [], newInboxItemIds: ["12"] });
		repo.listInboxItems.mockResolvedValue({
			items: [row("12", { title: "updated" })],
		});
		await signal({ unreadCount: 1, hints: [], newInboxItemIds: ["12"] });
		const calls = toast.show.mock.calls.map((c) => c[0]);
		expect(calls).toHaveLength(2);
		expect(new Set(calls.map((t) => t.dedupeKey))).toEqual(
			new Set(["notif:12"]),
		);
		expect(calls[1].title).toBe("updated");
	});

	it("toasts nothing when the signal is truncated, and does not read the list", async () => {
		render(React.createElement(NotificationToastBridge));
		await signal({
			unreadCount: 30,
			hints: [],
			newInboxItemIds: [],
			newItemsTruncated: true,
		});
		expect(toast.show).not.toHaveBeenCalled();
		expect(repo.listInboxItems).not.toHaveBeenCalled();
	});

	it.each([
		["initial or read/delete signal without ids", { newInboxItemIds: [] }],
		["abnormal-reconnect broadcast", {}],
	])("toasts nothing for a %s", async (_name, extra) => {
		render(React.createElement(NotificationToastBridge));
		await signal({ hints: [1], ...extra });
		expect(toast.show).not.toHaveBeenCalled();
		expect(repo.listInboxItems).not.toHaveBeenCalled();
	});

	it("does not call listInboxItems for an untruncated signal with no ids", async () => {
		render(React.createElement(NotificationToastBridge));
		await signal({
			unreadCount: 3,
			hints: [],
			newInboxItemIds: [],
			newItemsTruncated: false,
		});
		expect(repo.listInboxItems).not.toHaveBeenCalled();
		expect(toast.show).not.toHaveBeenCalled();
	});

	it("toasts nothing in a hidden tab", async () => {
		setVisibility("hidden");
		render(React.createElement(NotificationToastBridge));
		await signal({ unreadCount: 3, hints: [], newInboxItemIds: ["12"] });
		expect(toast.show).not.toHaveBeenCalled();
	});

	it("toasts nothing when the tab is hidden by the time the page arrives", async () => {
		render(React.createElement(NotificationToastBridge));
		repo.listInboxItems.mockImplementationOnce(async () => {
			setVisibility("hidden");
			return { items: [row("12")] };
		});
		await signal({ unreadCount: 3, hints: [], newInboxItemIds: ["12"] });
		expect(toast.show).not.toHaveBeenCalled();
	});

	it("skips message rows, read rows and ids missing from the page", async () => {
		repo.listInboxItems.mockResolvedValue({
			items: [row("12", { presentation: 4 }), row("11", { read: true })],
		});
		render(React.createElement(NotificationToastBridge));
		await signal({
			unreadCount: 3,
			hints: [],
			newInboxItemIds: ["12", "11", "99"],
		});
		expect(toast.show).not.toHaveBeenCalled();
	});
});
