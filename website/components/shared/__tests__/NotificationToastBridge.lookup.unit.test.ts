// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ChatStreamEvent } from "@/lib/chat/stream";

const chat = vi.hoisted(() => ({
	handler: undefined as undefined | ((e: ChatStreamEvent) => void),
	listConversations: vi.fn(),
}));
const toast = vi.hoisted(() => ({ show: vi.fn() }));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		activeRole: "buyer",
		status: "signedIn",
		identity: { accountId: "1" },
	}),
}));
vi.mock("@/lib/chat/useChatStream", () => ({
	useChatStream: (
		_side: string,
		_enabled: boolean,
		onEvent: (e: ChatStreamEvent) => void,
	) => {
		chat.handler = onEvent;
		return { state: "connected", stream: null };
	},
}));
vi.mock("@/lib/chat/repo", () => ({
	listConversations: chat.listConversations,
}));
vi.mock("@/lib/notifications/repo", () => ({
	fetchNotifications: async () => ({ records: [] }),
	fetchUnreadCount: async () => 0,
}));
vi.mock("@/lib/i18n/ui-text", () => ({ useUiText: () => () => "" }));
vi.mock("@/lib/notifications/unread-context", () => ({
	useNotificationUnread: () => ({
		count: 0,
		loading: false,
		baselineReady: true,
	}),
}));
vi.mock("@/lib/notifications/toast-context", () => ({
	useNotificationToast: () => ({ show: toast.show }),
}));

import NotificationToastBridge from "@/components/shared/NotificationToastBridge";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

const incoming = (id: string, conversationId: string): ChatStreamEvent => ({
	type: "message",
	updated: false,
	message: {
		id,
		conversationId,
		seq: 1,
		sender: "store",
		kind: "text",
		text: "hi",
		hidden: false,
		erased: false,
		timeMs: 1,
	},
});

describe("NotificationToastBridge conversation lookup", () => {
	it("shares one in-flight lookup across a burst of messages and links each to its peer", async () => {
		let resolve: (v: unknown) => void = () => {};
		chat.listConversations.mockReturnValue(new Promise((r) => (resolve = r)));
		render(React.createElement(NotificationToastBridge));
		act(() => {
			chat.handler?.(incoming("m1", "c1"));
			chat.handler?.(incoming("m2", "c2"));
			chat.handler?.(incoming("m3", "c1"));
		});
		expect(chat.listConversations).toHaveBeenCalledTimes(1);
		await act(async () => {
			resolve([
				{ id: "c1", targetUserId: "s1" },
				{ id: "c2", targetUserId: "s2" },
			]);
			await new Promise((r) => setTimeout(r, 10));
		});
		const hrefs = toast.show.mock.calls.map((c) => c[0].href).sort();
		expect(hrefs).toEqual([
			"/account/messages/s1",
			"/account/messages/s1",
			"/account/messages/s2",
		]);
		expect(chat.listConversations).toHaveBeenCalledTimes(1);
	});
});
