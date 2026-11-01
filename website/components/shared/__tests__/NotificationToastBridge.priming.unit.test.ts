// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({
	push: vi.fn(),
	replace: vi.fn(),
	refresh: vi.fn(),
}));
const repo = vi.hoisted(() => ({
	fetchUnreadCount: vi.fn(),
	fetchNotifications: vi.fn(),
}));
const toast = vi.hoisted(() => ({ show: vi.fn() }));
const stream = vi.hoisted(() => ({ ready: vi.fn() }));

vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
}));
vi.mock("@/lib/intl/navigation", () => ({
	usePathname: () => "/account/notifications",
	useRouter: () => router,
}));
vi.mock("@sentry/nextjs", () => ({ captureMessage: vi.fn() }));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: vi.fn(),
}));
vi.mock("@/lib/notifications/repo", () => ({
	fetchUnreadCount: repo.fetchUnreadCount,
	fetchNotifications: repo.fetchNotifications,
}));
vi.mock("@/lib/notifications/toast-context", () => ({
	useNotificationToast: () => ({ show: toast.show, dismiss: vi.fn() }),
}));
vi.mock("@/lib/chat/ws", () => ({
	getChatSocket: () => ({
		connect: vi.fn(),
		onMessage: () => () => {},
	}),
}));
vi.mock("@/lib/sse/useNotificationStream", () => ({
	useNotificationStream: (options: { handlers: unknown }) => {
		stream.ready(options.handlers);
		return { status: "open" };
	},
}));

import NotificationToastBridge from "@/components/shared/NotificationToastBridge";
import { AuthProvider } from "@/lib/auth/context";
import type { SessionState } from "@/lib/auth/session-state";
import { ZoneIdentityProvider } from "@/lib/auth/zone-context";
import { messagesFor } from "@/lib/intl/config";
import { NotificationUnreadProvider } from "@/lib/notifications/unread-context";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	vi.unstubAllGlobals();
});

const h = React.createElement as unknown as (
	type: unknown,
	props: unknown,
	...children: unknown[]
) => React.ReactElement;

function memoryStorage() {
	const store = new Map<string, string>();
	return {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => void store.set(key, value),
		removeItem: (key: string) => void store.delete(key),
	};
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((done) => {
		resolve = done;
	});
	return { promise, resolve };
}

const flush = () =>
	act(async () => {
		await new Promise((done) => setTimeout(done, 20));
	});

const IDENTITY = { accountId: "1", isSeller: false };
const SIGNED_IN: SessionState = { status: "signedIn", identity: IDENTITY };

function mountBridge(session: Promise<SessionState>, zone: boolean): void {
	const bridge = h(
		NotificationUnreadProvider,
		null,
		h(NotificationToastBridge, null),
	);
	render(
		h(
			QueryClientProvider,
			{ client: new QueryClient() },
			h(
				NextIntlClientProvider,
				{ locale: "zh-TW", messages: messagesFor("zh-TW") },
				h(
					AuthProvider,
					{ session },
					zone
						? h(
								ZoneIdentityProvider,
								{ identity: IDENTITY, workspace: "buyer" },
								bridge,
							)
						: bridge,
				),
			),
		),
	);
}

describe("NotificationToastBridge priming when the unread count settles without an observable loading render", () => {
	for (const zone of [false, true]) {
		it(`sends the pageSize=1 probe and toasts a later row (zone=${zone})`, async () => {
			vi.stubGlobal("localStorage", memoryStorage());
			vi.stubGlobal("sessionStorage", memoryStorage());
			repo.fetchUnreadCount.mockResolvedValue(1);
			repo.fetchNotifications.mockResolvedValue({
				records: [{ id: "10000000000000000001", type: "order" }],
			});
			const session = deferred<SessionState>();
			mountBridge(session.promise, zone);
			await flush();

			(
				globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
			).IS_REACT_ACT_ENVIRONMENT = false;
			session.resolve(SIGNED_IN);
			await new Promise((done) => setTimeout(done, 50));
			(
				globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
			).IS_REACT_ACT_ENVIRONMENT = true;

			expect(repo.fetchNotifications).toHaveBeenCalledWith({ pageSize: 1 });

			const handlers = stream.ready.mock.lastCall?.[0] as {
				onNew: (payload: unknown) => void;
			};
			await act(async () =>
				handlers.onNew({
					item: {
						id: "10000000000000000002",
						type: "order",
						title: "訂單成立通知",
						isRead: false,
						userType: "buyer",
					},
					unreadCount: 2,
				}),
			);
			expect(toast.show).toHaveBeenCalledTimes(1);
		});
	}
});
